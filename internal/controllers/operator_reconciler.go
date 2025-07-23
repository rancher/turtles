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
	"context"
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	"sigs.k8s.io/cluster-api-operator/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctr "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/yaml"

	"github.com/rancher/turtles/internal/controllers/clusterctl"
)

// OperatorReconciler is a mapping wrapper for CAPIProvider -> operator provider resources.
type OperatorReconciler struct{}

// SetupWithManager is a mapping wrapper for CAPIProvider -> operator provider resources.
func (r *OperatorReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options ctr.Options) error {
	log := log.FromContext(ctx)

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.CoreProvider{},
			ProviderList:             &operatorv1.CoreProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
		},
	}).SetupWithManager(ctx, mgr, options); err != nil {
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.InfrastructureProvider{},
			ProviderList:             &operatorv1.InfrastructureProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		},
	}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "InfrastructureProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.BootstrapProvider{},
			ProviderList:             &operatorv1.BootstrapProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		},
	}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "BootstrapProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.ControlPlaneProvider{},
			ProviderList:             &operatorv1.ControlPlaneProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		},
	}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "ControlPlaneProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.AddonProvider{},
			ProviderList:             &operatorv1.AddonProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		},
	}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "AddonProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.IPAMProvider{},
			ProviderList:             &operatorv1.IPAMProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		},
	}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "IPAMProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.RuntimeExtensionProvider{},
			ProviderList:             &operatorv1.RuntimeExtensionProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		},
	}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "RuntimeExtensionProvider")
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders/finalizers,verbs=update

// CAPIProviderReconcilerWrapper wraps the upstream CAPIProviderReconciler.
type CAPIProviderReconcilerWrapper struct {
	controller.GenericProviderReconciler
}

// BuildWithManager builds the CAPIProviderReconciler.
func (r *CAPIProviderReconcilerWrapper) BuildWithManager(ctx context.Context, mgr ctrl.Manager) (*ctrl.Builder, error) {
	builder, err := r.GenericProviderReconciler.BuildWithManager(ctx, mgr)
	if err != nil {
		return nil, err
	}

	reconciler := controller.NewPhaseReconciler(r.GenericProviderReconciler, r.Provider, r.ProviderList)

	r.ReconcilePhases = []controller.PhaseFn{
		r.waitForClusterctlConfigUpdate,
		r.setDefaultProviderSpec,
		reconciler.ApplyFromCache,
		reconciler.PreflightChecks,
		reconciler.InitializePhaseReconciler,
		reconciler.DownloadManifests,
		reconciler.Load,
		reconciler.Fetch,
		r.patchProviderManifest,
		reconciler.Store,
		reconciler.Upgrade,
		reconciler.Install,
		reconciler.ReportStatus,
		reconciler.Finalize,
	}

	return builder, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *CAPIProviderReconcilerWrapper) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options ctr.Options) error {
	builder, err := r.BuildWithManager(ctx, mgr)
	if err != nil {
		return err
	}

	return builder.WithOptions(options).Complete(r)
}

// Reconcile wraps the upstream Reconcile method.
func (r *CAPIProviderReconcilerWrapper) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	return r.GenericProviderReconciler.Reconcile(ctx, req)
}

func (r *CAPIProviderReconcilerWrapper) patchProviderManifest(ctx context.Context) (*controller.Result, error) {
	// TODO: patch provider manifest to annotate webhook service
	log := log.FromContext(ctx)

	providerSpec := r.Provider.GetSpec()

	// TODO: need to fetch the name of the secret for each provider: e.g. "capi-webhook-service" | "capd-webhook-service"
	// could only find this name in the provider manifest ConfigMap
	// Manifest ConfigMap is looked up based on labels:
	//      - "managed-by.operator.cluster.x-k8s.io":"true",
	//      - "provider.cluster.x-k8s.io/name":"cluster-api",
	//      - "provider.cluster.x-k8s.io/type":"core",
	//      - "provider.cluster.x-k8s.io/version":"v1.9.5"
	labels := map[string]string{
		operatorv1.ConfigMapNameLabel:        r.Provider.GetName(),
		operatorv1.ConfigMapTypeLabel:        r.Provider.GetType(),
		operatorv1.ConfigMapVersionLabelName: providerSpec.Version,
	}

	var configMapList corev1.ConfigMapList
	selectors := []client.ListOption{
		client.MatchingLabels(labels),
	}

	if err := r.Client.List(ctx, &configMapList, selectors...); err != nil {
		return nil, fmt.Errorf("listing ConfigMaps with labels %v: %w", labels, err)
	}

	// log.Info("Patching provider manifest", "ProviderType", r.Provider.GetType(), "ProviderName", r.Provider.GetName(), "ProviderVersion", providerSpec.Version)
	if len(configMapList.Items) != 1 {
		return nil, fmt.Errorf("expected exactly one ConfigMap with labels %v, got %d", labels, len(configMapList.Items))
	}
	manifestConfigMap := configMapList.Items[0]
	log.Info("Patching provider manifest", "ConfigMap.Name", manifestConfigMap.Name)
	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(manifestConfigMap.Data["components"]), &result); err != nil {
	}
	log.Info("Patching provider manifest", "ConfigMap.Data.metadata", result)

	// TODO: apply patch on manifest to add annotation
	/*
			newManifestPatches := []string{
				`---
		apiVersion: v1
		kind: Service
		metadata:
		  annotations:
		    need-a-cert.cattle.io/secret-name: test-value`,
			}
			//newManifestPatches := []string{
			//	"apiVersion: v1",
			//	"kind: Service",
			//	"metadata:",
			//	"  labels:",
			//	"      test-label: test-value",
			//}
			providerSpec.ManifestPatches = append(providerSpec.ManifestPatches, newManifestPatches...)
	*/

	return &controller.Result{}, nil
}

func (r *CAPIProviderReconcilerWrapper) setDefaultProviderSpec(_ context.Context) (*controller.Result, error) {
	setDefaultProviderSpec(r.Provider)

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

	o.SetSpec(providerSpec)
}

// waitForClusterctlConfigUpdate is a phase that waits for the clusterctl-config Configmap
// mounted in `/config/clusterctl.yaml` to be updated with the intended content.
// This should contain the base embedded in-memory ConfigMap, with overrides
// from the user defined ClusterctlConfig, if any.
// It may take a few minutes for the changes to take effect.
// We need to wait since the cluster-api-operator library is going to use the mounted file
// to deploy providers, therefore we need it to be synced with embedded and user overrides.
func (r *CAPIProviderReconcilerWrapper) waitForClusterctlConfigUpdate(ctx context.Context) (*controller.Result, error) {
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
		return &controller.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	return &controller.Result{}, nil
}
