/*
Copyright © 2023 - 2025 SUSE LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

//nolint:gci
import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"maps"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctr "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	"sigs.k8s.io/cluster-api-operator/controller"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	clusterctlv1 "sigs.k8s.io/cluster-api/cmd/clusterctl/api/v1alpha3"
	configclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/controllers/clusterctl"
	"github.com/rancher/turtles/internal/sync"
)

const (
	configSecretNameField      = "spec.configSecret.name"      //nolint:gosec
	configSecretNamespaceField = "spec.configSecret.namespace" //nolint:gosec
	providerTypeField          = "spec.type"                   //nolint:gosec
	providerNameField          = "spec.name"                   //nolint:gosec

	azureProvider = "azure"
	gcpProvider   = "gcp"
)

// OperatorReconciler is a mapping wrapper for CAPIProvider -> operator provider resources.
type OperatorReconciler struct{}

// SetupWithManager is a mapping wrapper for CAPIProvider -> operator provider resources.
func (r *OperatorReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options ctr.Options) error {
	if err := (&CAPIProviderReconciler{
		Client: mgr.GetClient(),
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:     &turtlesv1.CAPIProvider{},
			ProviderList: &turtlesv1.CAPIProviderList{},
			Client:       &ClientWrapper{Client: mgr.GetClient()},
			Config:       mgr.GetConfig(),
		},
	}).SetupWithManager(ctx, mgr, options); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "unable to create controller", "controller", "CAPIProvider")

		return err
	}

	if err := (&controller.GenericProviderHealthCheckReconciler{
		Client:   mgr.GetClient(),
		Provider: &turtlesv1.CAPIProvider{},
	}).SetupWithManager(mgr, options); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "unable to create controller", "controller", "GenericProviderHealthCheck")

		return err
	}

	return nil
}

// ClientWrapper wraps the upstream client, preventing CAPIProvider spec patch. Status patch is performed as usual.
type ClientWrapper struct {
	client.Client
}

// Patch shadows the upstream patch method, ignoring CAPIProvider spec patch.
func (c *ClientWrapper) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	// Ignore CAPIProvider spec patch
	if _, ok := obj.(*turtlesv1.CAPIProvider); ok && obj.GetDeletionTimestamp().IsZero() {
		return nil
	}

	return c.Client.Patch(ctx, obj, patch, opts...)
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// CAPIProviderReconciler wraps the upstream CAPIProviderReconciler.
type CAPIProviderReconciler struct {
	controller.GenericProviderReconciler
	client.Client
}

// BuildWithManager builds the CAPIProviderReconciler.
func (r *CAPIProviderReconciler) BuildWithManager(ctx context.Context, mgr ctrl.Manager) (*ctrl.Builder, error) {
	builder, err := r.GenericProviderReconciler.BuildWithManager(ctx, mgr)
	if err != nil {
		return nil, err
	}

	builder = builder.Named("ProviderReconciler")

	if err := indexFields(ctx, &turtlesv1.CAPIProvider{}, mgr); err != nil {
		return nil, err
	}

	builder.Watches(
		&corev1.Secret{},
		handler.EnqueueRequestsFromMapFunc(newSecretToProviderFuncMapForProviderList(mgr.GetClient())),
	)

	builder = builder.Watches(
		&turtlesv1.CAPIProvider{},
		handler.EnqueueRequestsFromMapFunc(newCoreProviderToProviderFuncMapForProviderList(mgr.GetClient())),
	)

	rec := controller.NewPhaseReconciler(
		r.GenericProviderReconciler, r.Provider, r.ProviderList,
		controller.WithProviderConverter(getProvider),
		controller.WithProviderLister(r.listProviders),
		controller.WithProviderMapper(r.getGenericProvider),
		controller.WithProviderTypeMapper(toClusterctlType),
	)

	r.ReconcilePhases = []controller.PhaseFn{
		r.waitForClusterctlConfigUpdate,
		r.setProviderSpec,
		r.syncSecrets,
		rec.ApplyFromCache,
		rec.PreflightChecks,
		rec.InitializePhaseReconciler,
		rec.DownloadManifests,
		rec.Load,
		rec.Fetch,
		rec.Store,
		rec.Upgrade,
		rec.Install,
		rec.ReportStatus,
		r.setConditions,
		rec.Finalize,
	}

	r.DeletePhases = []controller.PhaseFn{
		r.waitForClusterctlConfigUpdate,
		r.setProviderSpec,
		rec.Delete,
	}

	for i, phase := range r.ReconcilePhases {
		r.ReconcilePhases[i] = finalizePhase(phase, r.setConditions)
	}

	return builder, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CAPIProviderReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options ctr.Options) error {
	builder, err := r.BuildWithManager(ctx, mgr)
	if err != nil {
		return err
	}

	return builder.WithOptions(options).Complete(reconcile.AsReconciler(r.Client, r))
}

// newSecretToProviderFuncMapForProviderList maps a Kubernetes secret to all the providers that reference it.
// It lists all the providers matching spec.configSecret.name values with the secret name querying by index.
// If the provider references a secret without a namespace, it will assume the secret is in the same namespace as the provider.
func newSecretToProviderFuncMapForProviderList(cl client.Client) handler.MapFunc {
	return func(ctx context.Context, secret client.Object) []reconcile.Request {
		log := ctrl.LoggerFrom(ctx).WithValues("secret", map[string]string{"name": secret.GetName(), "namespace": secret.GetNamespace()})

		var requests []reconcile.Request

		configSecretMatcher := client.MatchingFields{
			configSecretNameField:      secret.GetName(),
			configSecretNamespaceField: secret.GetNamespace(),
		}

		providerList := &turtlesv1.CAPIProviderList{}
		if err := cl.List(ctx, providerList, configSecretMatcher); err != nil {
			log.Error(err, "failed to list providers")
			return nil
		}

		for _, provider := range providerList.GetItems() {
			log = log.WithValues("provider", map[string]string{"name": provider.GetName(), "namespace": provider.GetNamespace()})
			requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(provider)})
		}

		return requests
	}
}

// newCoreProviderToProviderFuncMapForProviderList maps a ready CoreProvider object to all other provider objects.
// It lists all the providers and if its PreflightCheckCondition is not True, this object will be added to the resulting request.
// This means that notifications will only be sent to those objects that have not pass PreflightCheck.
func newCoreProviderToProviderFuncMapForProviderList(cl client.Client) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		log := ctrl.LoggerFrom(ctx).WithValues("provider", map[string]string{"name": obj.GetName(), "namespace": obj.GetNamespace()})

		coreProvider, ok := obj.(*turtlesv1.CAPIProvider)
		if !ok {
			log.Error(fmt.Errorf("expected a %T but got a %T", turtlesv1.CAPIProvider{}, obj), "unable to cast object")
			return nil
		}

		if coreProvider.Spec.Type != turtlesv1.Core {
			return nil
		}

		// We don't want to raise events if CoreProvider is not ready yet.
		if !conditions.IsTrue(coreProvider, clusterv1.ReadyCondition) {
			return nil
		}

		var requests []reconcile.Request

		providerList := &turtlesv1.CAPIProviderList{}
		if err := cl.List(ctx, providerList); err != nil {
			log.Error(err, "failed to list providers")

			return nil
		}

		for _, provider := range providerList.Items {
			if provider.Spec.Type == turtlesv1.Core {
				continue
			}

			if !conditions.IsTrue(&provider, operatorv1.PreflightCheckCondition) {
				// Raise secondary events for the providers that fail PreflightCheck.
				requests = append(requests, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(&provider)})
			}
		}

		return requests
	}
}

// Reconcile wraps the upstream Reconcile method.
func (r *CAPIProviderReconciler) Reconcile(ctx context.Context, provider *turtlesv1.CAPIProvider) (_ reconcile.Result, reterr error) {
	if !controllerutil.ContainsFinalizer(provider, operatorv1.ProviderFinalizer) && provider.DeletionTimestamp.IsZero() {
		patchHelper, err := patch.NewHelper(provider, r.Client)
		if err != nil {
			return ctrl.Result{}, err
		}

		controllerutil.AddFinalizer(provider, operatorv1.ProviderFinalizer)

		return reconcile.Result{}, patchHelper.Patch(ctx, provider)
	}

	return r.GenericProviderReconciler.Reconcile(ctx, reconcile.Request{
		NamespacedName: client.ObjectKeyFromObject(provider),
	})
}

func setProviderSpec(ctx context.Context, cl client.Client, provider *turtlesv1.CAPIProvider) error {
	setDefaultProviderSpec(provider)

	if err := setLatestVersion(ctx, cl, provider); err != nil {
		return err
	}

	switch provider.ProviderName() {
	case azureProvider:
		if provider.Status.Variables == nil {
			provider.Status.Variables = map[string]string{}
		}

		provider.Status.Variables["EXP_AKS_RESOURCE_HEALTH"] = "true"
	case gcpProvider:
		if provider.Status.Variables == nil {
			provider.Status.Variables = map[string]string{}
		}

		provider.Status.Variables["EXP_CAPG_GKE"] = "true"
	}

	return nil
}

func (r *CAPIProviderReconciler) setProviderSpec(ctx context.Context) (*controller.Result, error) {
	if capiProvider, ok := r.Provider.(*turtlesv1.CAPIProvider); ok {
		return &controller.Result{}, setProviderSpec(ctx, r.Client, capiProvider)
	}

	return &controller.Result{}, nil
}

func (r *CAPIProviderReconciler) setConditions(_ context.Context) (*controller.Result, error) {
	if capiProvider, ok := r.Provider.(*turtlesv1.CAPIProvider); ok {
		setConditions(capiProvider)
	}

	return &controller.Result{}, nil
}

// setDefaultProviderSpec sets the default values for the provider spec.
func setDefaultProviderSpec(o operatorv1.GenericProvider) {
	providerSpec := o.GetSpec()
	providerNamespace := o.GetNamespace()

	if providerSpec.ConfigSecret != nil && providerSpec.ConfigSecret.Namespace == "" {
		providerSpec.ConfigSecret.Namespace = providerNamespace
	}

	if providerSpec.AdditionalManifestsRef != nil && providerSpec.AdditionalManifestsRef.Namespace == "" {
		providerSpec.AdditionalManifestsRef.Namespace = providerNamespace
	}

	if provider, ok := o.(*turtlesv1.CAPIProvider); ok {
		setVariables(provider)
		setFeatures(provider)
	}

	providerSpec.ConfigSecret = cmp.Or(providerSpec.ConfigSecret, &operatorv1.SecretReference{
		Name: o.GetName(),
	})

	providerSpec.ConfigSecret.Namespace = cmp.Or(providerSpec.ConfigSecret.Namespace, providerNamespace)

	o.SetSpec(providerSpec)
}

// finalizePhase performs a finalization step after a phase is completed or failed.
func finalizePhase(phase controller.PhaseFn, finalizePhase controller.PhaseFn) controller.PhaseFn {
	return func(ctx context.Context) (*controller.Result, error) {
		res, err := phase(ctx)
		if err != nil {
			// Perform finalization step after failed phase
			_, finalizeErr := finalizePhase(ctx)
			return res, kerrors.NewAggregate([]error{err, finalizeErr})
		}

		// Perform finalization step after early completion
		if res != nil && res.Completed {
			return finalizePhase(ctx)
		}

		return res, nil
	}
}

func getProvider(provider operatorv1.GenericProvider) clusterctlv1.Provider {
	clusterctlProvider := &clusterctlv1.Provider{}
	if p, ok := provider.(*turtlesv1.CAPIProvider); ok {
		clusterctlProvider.Name = p.Spec.Type.ToName() + cmp.Or(p.Spec.Name, p.GetName())
		clusterctlProvider.Namespace = provider.GetNamespace()
		clusterctlProvider.Type = string(toClusterctlType(p))
		clusterctlProvider.ProviderName = p.ProviderName()
	}

	if provider.GetStatus().InstalledVersion != nil {
		clusterctlProvider.Version = *provider.GetStatus().InstalledVersion
	}

	return *clusterctlProvider
}

func toClusterctlType(p operatorv1.GenericProvider) clusterctlv1.ProviderType {
	provider, ok := p.(*turtlesv1.CAPIProvider)
	if !ok {
		return clusterctlv1.ProviderTypeUnknown
	}

	switch provider.Spec.Type {
	case turtlesv1.Infrastructure:
		return clusterctlv1.InfrastructureProviderType
	case turtlesv1.Core:
		return clusterctlv1.CoreProviderType
	case turtlesv1.ControlPlane:
		return clusterctlv1.ControlPlaneProviderType
	case turtlesv1.Bootstrap:
		return clusterctlv1.BootstrapProviderType
	case turtlesv1.Addon:
		return clusterctlv1.AddonProviderType
	case turtlesv1.IPAM:
		return clusterctlv1.IPAMProviderType
	case turtlesv1.RuntimeExtension:
		return clusterctlv1.RuntimeExtensionProviderType
	default:
		return clusterctlv1.ProviderTypeUnknown
	}
}

func toProviderType(p clusterctlv1.ProviderType) turtlesv1.Type {
	switch p {
	case clusterctlv1.CoreProviderType:
		return turtlesv1.Core
	case clusterctlv1.BootstrapProviderType:
		return turtlesv1.Bootstrap
	case clusterctlv1.InfrastructureProviderType:
		return turtlesv1.Infrastructure
	case clusterctlv1.ControlPlaneProviderType:
		return turtlesv1.ControlPlane
	case clusterctlv1.IPAMProviderType:
		return turtlesv1.IPAM
	case clusterctlv1.RuntimeExtensionProviderType:
		return turtlesv1.RuntimeExtension
	case clusterctlv1.AddonProviderType:
		return turtlesv1.Addon
	default:
		return ""
	}
}

func (r *CAPIProviderReconciler) listProviders(ctx context.Context, list *clusterctlv1.ProviderList, ops ...controller.ProviderOperation) error {
	group := &turtlesv1.CAPIProviderList{}
	if err := r.List(ctx, group); err != nil {
		return err
	}

	for _, p := range group.GetItems() {
		for _, op := range ops {
			err := op(p)
			if err != nil {
				return err
			}
		}

		list.Items = append(list.Items, getProvider(p))
	}

	return nil
}

// GetGenericProvider returns the first of generic providers matching the type and the name from the configclient.Provider.
func (r *CAPIProviderReconciler) getGenericProvider(ctx context.Context, provider configclient.Provider) (operatorv1.GenericProvider, error) {
	list := &turtlesv1.CAPIProviderList{}
	if err := r.List(ctx, list, client.MatchingFields{
		providerTypeField: string(toProviderType(provider.Type())),
		providerNameField: provider.Name(),
	}); err != nil {
		return nil, err
	}

	if len(list.Items) == 0 {
		return nil, fmt.Errorf("unable to find provider manifest with name %s and type %s", provider.Name(), toProviderType(provider.Type()))
	}

	pr := list.Items[0]
	// We need to default provider spec here, otherwise vesion and other required fields may be empty
	if err := setProviderSpec(ctx, r.Client, &pr); err != nil {
		return nil, err
	}

	return &pr, nil
}

func indexFields(ctx context.Context, provider client.Object, mgr ctrl.Manager) error {
	return cmp.Or(
		mgr.GetFieldIndexer().IndexField(ctx, provider, configSecretNameField, configSecretNameIndexFunc),
		mgr.GetFieldIndexer().IndexField(ctx, provider, configSecretNamespaceField, configSecretNamespaceIndexFunc),
		mgr.GetFieldIndexer().IndexField(ctx, provider, providerTypeField, typeIndexFunc),
		mgr.GetFieldIndexer().IndexField(ctx, provider, providerNameField, nameIndexFunc),
	)
}

// configSecretNameIndexFunc is indexing config Secret name field.
func configSecretNameIndexFunc(obj client.Object) []string {
	provider, ok := obj.(operatorv1.GenericProvider)
	if !ok {
		return nil
	}

	setDefaultProviderSpec(provider)

	if provider.GetSpec().ConfigSecret == nil {
		return nil
	}

	return []string{provider.GetSpec().ConfigSecret.Name}
}

// configSecretNamespaceIndexFunc is indexing config Secret namespace field.
func configSecretNamespaceIndexFunc(obj client.Object) []string {
	provider, ok := obj.(operatorv1.GenericProvider)
	if !ok {
		return nil
	}

	if provider.GetSpec().ConfigSecret == nil {
		return nil
	}

	return []string{cmp.Or(provider.GetSpec().ConfigSecret.Namespace, provider.GetNamespace())}
}

// typeIndexFunc is indexing the provider type field.
func typeIndexFunc(obj client.Object) []string {
	provider, ok := obj.(*turtlesv1.CAPIProvider)
	if !ok {
		return nil
	}

	return []string{string(provider.Spec.Type)}
}

// nameIndexFunc is indexing the provider name field.
func nameIndexFunc(obj client.Object) []string {
	provider, ok := obj.(*turtlesv1.CAPIProvider)
	if !ok {
		return nil
	}

	return []string{provider.ProviderName()}
}

func setVariables(capiProvider *turtlesv1.CAPIProvider) {
	if capiProvider.Spec.Variables != nil {
		maps.Copy(capiProvider.Status.Variables, capiProvider.Spec.Variables)
	}
}

func setFeatures(capiProvider *turtlesv1.CAPIProvider) {
	features := capiProvider.Spec.Features
	variables := capiProvider.Status.Variables

	if features != nil {
		variables["EXP_CLUSTER_RESOURCE_SET"] = strconv.FormatBool(features.ClusterResourceSet)
		variables["CLUSTER_TOPOLOGY"] = strconv.FormatBool(features.ClusterTopology)
		variables["EXP_MACHINE_POOL"] = strconv.FormatBool(features.MachinePool)
	}
}

func setConditions(provider *turtlesv1.CAPIProvider) {
	provider.SetProviderName()

	switch {
	case conditions.IsTrue(provider, operatorv1.ProviderInstalledCondition):
		provider.SetPhase(turtlesv1.Ready)
	case conditions.IsFalse(provider, operatorv1.PreflightCheckCondition):
		provider.SetPhase(turtlesv1.Failed)
	default:
		provider.SetPhase(turtlesv1.Provisioning)
	}
}

func (r *CAPIProviderReconciler) syncSecrets(ctx context.Context) (*controller.Result, error) {
	var err error

	if capiProvider, ok := r.Provider.(*turtlesv1.CAPIProvider); ok {
		s := sync.NewList(
			sync.NewSecretSync(r.Client, capiProvider),
			sync.NewSecretMapperSync(ctx, r.Client, capiProvider),
		)

		if err := s.Sync(ctx); client.IgnoreNotFound(err) != nil {
			return &controller.Result{}, err
		}

		s.Apply(ctx, &err)
	}

	return &controller.Result{}, err
}

func setLatestVersion(ctx context.Context, cl client.Client, provider *turtlesv1.CAPIProvider) error {
	log := log.FromContext(ctx)

	config, err := clusterctl.ClusterConfig(ctx, cl)
	if err != nil {
		return err
	}

	providerVersion, knownProvider := config.GetProviderVersion(ctx, provider.ProviderName(), provider.Spec.Type.ToKind())

	latest, err := config.IsLatestVersion(providerVersion, provider.Spec.Version)
	if err != nil {
		return err
	}

	switch {
	case !knownProvider:
		conditions.MarkUnknown(provider, turtlesv1.CheckLatestVersionTime, turtlesv1.CheckLatestProviderUnknownReason, "Provider is unknown")
	case provider.Spec.Version != "" && latest:
		conditions.MarkTrue(provider, turtlesv1.CheckLatestVersionTime)
	case provider.Spec.Version != "" && !latest:
		conditions.MarkFalse(
			provider,
			turtlesv1.CheckLatestVersionTime,
			turtlesv1.CheckLatestUpdateAvailableReason,
			clusterv1.ConditionSeverityInfo,
			"Provider version update available. Current latest is %s", providerVersion,
		)
	case !latest:
		lastCheck := conditions.Get(provider, turtlesv1.CheckLatestVersionTime)
		updatedMessage := fmt.Sprintf("Updated to latest %s version", providerVersion)

		if lastCheck == nil || lastCheck.Message != updatedMessage {
			log.Info(fmt.Sprintf("Version %s is beyond current latest, updated to %s", cmp.Or(provider.Spec.Version, "latest"), providerVersion))

			lastCheck = conditions.TrueCondition(turtlesv1.CheckLatestVersionTime)
			lastCheck.Message = updatedMessage

			conditions.Set(provider, lastCheck)
		}

		provider.Spec.Version = providerVersion
	}

	return nil
}

// waitForClusterctlConfigUpdate is a phase that waits for the clusterctl-config Configmap
// mounted in `/config/clusterctl.yaml` to be updated with the intended content.
// This should contain the base embedded in-memory ConfigMap, with overrides
// from the user defined ClusterctlConfig, if any.
// It may take a few minutes for the changes to take effect.
// We need to wait since the cluster-api-operator library is going to use the mounted file
// to deploy providers, therefore we need it to be synced with embedded and user overrides.
func (r *CAPIProviderReconciler) waitForClusterctlConfigUpdate(ctx context.Context) (*controller.Result, error) {
	logger := log.FromContext(ctx)

	// Load the mounted config from filesystem
	configBytes, err := os.ReadFile(clusterctl.ConfigPath)
	if os.IsNotExist(err) {
		logger.Info("ClusterctlConfig is not initialized yet, waiting for mounted ConfigMap to be updated.")
		return &controller.Result{RequeueAfter: defaultRequeueDuration}, nil
	} else if err != nil {
		return &controller.Result{}, fmt.Errorf("reading %s file: %w", clusterctl.ConfigPath, err)
	}

	// Get the expected config with user overrides
	config, err := clusterctl.ClusterConfig(ctx, r.Client)
	if err != nil {
		return &controller.Result{}, fmt.Errorf("getting updated ClusterctlConfig: %w", err)
	}

	// Compare the filesystem config with the expected one
	clusterctlYaml, err := yaml.Marshal(config)
	if err != nil {
		return &controller.Result{}, fmt.Errorf("serializing updated ClusterctlConfig: %w", err)
	}

	synced := bytes.Equal(clusterctlYaml, configBytes)

	if !synced {
		logger.Info("ClusterctlConfig is not synced yet, waiting for mounted ConfigMap to be updated.")
		return &controller.Result{RequeueAfter: 5 * time.Second}, nil
	}

	return &controller.Result{}, nil
}
