/*
Copyright © 2023 - 2024 SUSE LLC

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

import (
	"cmp"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	provisioningv1 "github.com/rancher/turtles/api/rancher/provisioning/v1"
	"github.com/rancher/turtles/feature"
	"github.com/rancher/turtles/util"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
	turtlesnaming "github.com/rancher/turtles/util/naming"
	turtlespredicates "github.com/rancher/turtles/util/predicates"
)

const (
	missingLabelMsg = "missing label"
)

// CAPIImportManagementV3Reconciler represents a reconciler for importing CAPI clusters in Rancher.
type CAPIImportManagementV3Reconciler struct {
	Client             client.Client
	UncachedClient     client.Client
	RancherClient      client.Client
	recorder           record.EventRecorder
	WatchFilterValue   string
	Scheme             *runtime.Scheme
	InsecureSkipVerify bool

	controller         controller.Controller
	externalTracker    external.ObjectTracker
	remoteClientGetter remote.ClusterClientGetter
}

// SetupWithManager sets up reconciler with manager.
func (r *CAPIImportManagementV3Reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	if r.remoteClientGetter == nil {
		r.remoteClientGetter = remote.NewClusterClient
	}

	capiPredicates := predicates.All(r.Scheme, log,
		predicates.ResourceHasFilterLabel(r.Scheme, log, r.WatchFilterValue),
		turtlespredicates.ClusterWithoutImportedAnnotation(log),
		turtlespredicates.ClusterWithReadyControlPlane(log),
		turtlespredicates.ClusterOrNamespaceWithImportLabel(ctx, log, r.Client, importLabelName),
	)

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		WithOptions(options).
		WithEventFilter(capiPredicates).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating new controller: %w", err)
	}

	// Watch Rancher managementv3 clusters
	if err := c.Watch(
		source.Kind[client.Object](mgr.GetCache(), &managementv3.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.rancherV3ClusterToCapiCluster(ctx, capiPredicates)),
		)); err != nil {
		return fmt.Errorf("adding watch for Rancher cluster: %w", err)
	}

	// Watch Rancher provisioningv1 clusters that don't have the migrated annotation and are related to a CAPI cluster
	if err := c.Watch(
		source.Kind[client.Object](mgr.GetCache(), &provisioningv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.rancherV1ClusterToCapiCluster(ctx, capiPredicates)),
		)); err != nil {
		return fmt.Errorf("adding watch for Rancher cluster: %w", err)
	}

	ns := &corev1.Namespace{}
	if err = c.Watch(
		source.Kind[client.Object](mgr.GetCache(), ns,
			handler.EnqueueRequestsFromMapFunc(namespaceToCapiClusters(ctx, capiPredicates, r.Client)),
		)); err != nil {
		return fmt.Errorf("adding watch for namespaces: %w", err)
	}

	r.recorder = mgr.GetEventRecorderFor("rancher-turtles")
	r.controller = c
	r.externalTracker = external.ObjectTracker{
		Controller: c,
	}

	return nil
}

// +kubebuilder:rbac:groups="",resources=secrets;events;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=management.cattle.io,resources=clusters;clusters/status;clusterregistrationtokens,verbs=get;list;watch;create;update;delete;deletecollection;patch
// +kubebuilder:rbac:groups=management.cattle.io,resources=clusterregistrationtokens/status;settings,verbs=get;list;watch
// +kubebuilder:rbac:groups=provisioning.cattle.io,resources=clusters;clusters/status,verbs=get;list;watch
//
//nolint:lll

// Reconcile reconciles a CAPI cluster, creating a Rancher cluster if needed and applying the import manifests.
func (r *CAPIImportManagementV3Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPI cluster")

	capiCluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, capiCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	log = log.WithValues("cluster", capiCluster.Name)

	if capiCluster.DeletionTimestamp.IsZero() && !turtlesannotations.HasClusterImportAnnotation(capiCluster) &&
		controllerutil.AddFinalizer(capiCluster, managementv3.CapiClusterFinalizer) {
		log.Info("CAPI cluster is marked for import, adding finalizer")

		if err := r.Client.Update(ctx, capiCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("error adding finalizer: %w", err)
		}
	}

	// Wait for controlplane to be ready. This should never be false as the predicates
	// do the filtering.
	if !capiCluster.Status.ControlPlaneReady && !conditions.IsTrue(capiCluster, clusterv1.ControlPlaneReadyCondition) {
		log.Info("clusters control plane is not ready, requeue")
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if turtlesannotations.HasClusterImportAnnotation(capiCluster) {
		log.Info("cluster was imported already and has imported=true annotation set, skipping re-import")
		return ctrl.Result{}, nil
	}

	// Collect errors as an aggregate to return together after all patches have been performed.
	var errs []error

	patchBase := client.MergeFromWithOptions(capiCluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

	result, err := r.reconcile(ctx, capiCluster)
	if err != nil {
		errs = append(errs, fmt.Errorf("error reconciling cluster: %w", err))
	}

	if err := r.Client.Patch(ctx, capiCluster, patchBase); err != nil {
		errs = append(errs, fmt.Errorf("failed to patch cluster: %w", err))
	}

	if len(errs) > 0 {
		return ctrl.Result{}, errorutils.NewAggregate(errs)
	}

	return result, nil
}

func (r *CAPIImportManagementV3Reconciler) reconcile(ctx context.Context, capiCluster *clusterv1.Cluster) (res ctrl.Result, reterr error) {
	log := log.FromContext(ctx)

	migrated, err := r.verifyV1ClusterMigration(ctx, capiCluster)
	if err != nil || !migrated {
		return ctrl.Result{Requeue: true}, err
	}

	labels := map[string]string{
		capiClusterOwner:          capiCluster.Name,
		capiClusterOwnerNamespace: capiCluster.Namespace,
		ownedLabelName:            "",
	}

	var rancherCluster *managementv3.Cluster

	rancherClusterList := &managementv3.ClusterList{}
	selectors := []client.ListOption{
		client.MatchingLabels(labels),
	}

	if err := r.RancherClient.List(ctx, rancherClusterList, selectors...); client.IgnoreNotFound(err) != nil {
		log.Error(err, fmt.Sprintf("Unable to fetch rancher cluster %s", client.ObjectKeyFromObject(rancherCluster)))
		return ctrl.Result{Requeue: true}, err
	}

	if len(rancherClusterList.Items) != 0 {
		if len(rancherClusterList.Items) > 1 {
			log.Info("More than one rancher cluster found. Will default to using the first one.")
		}

		rancherCluster = &rancherClusterList.Items[0]
	}

	if rancherCluster != nil && !rancherCluster.DeletionTimestamp.IsZero() {
		if err := r.reconcileDelete(ctx, capiCluster); err != nil {
			log.Error(err, "Removing CAPI Cluster failed, retrying")
			return ctrl.Result{}, err
		}

		if controllerutil.RemoveFinalizer(rancherCluster, managementv3.CapiClusterFinalizer) {
			if err := r.Client.Update(ctx, rancherCluster); err != nil {
				return ctrl.Result{}, fmt.Errorf("error removing rancher cluster finalizer: %w", err)
			}
		}
	}

	if !capiCluster.DeletionTimestamp.IsZero() {
		if err := r.deleteDependentRancherCluster(ctx, capiCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting associated managementv3.Cluster resources: %w", err)
		}
	}

	patchBase := client.MergeFromWithOptions(rancherCluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

	defer func() {
		// As the rancherCluster is created inside reconcileNormal, we can only patch existing object
		// Skipping non-existent cluster or returned error
		if reterr != nil || rancherCluster == nil {
			return
		}

		if err := r.Client.Patch(ctx, rancherCluster, patchBase); err != nil {
			reterr = fmt.Errorf("failed to patch Rancher cluster: %w", err)
		}
	}()

	res, reterr = r.reconcileNormal(ctx, capiCluster, rancherCluster)

	return res, reterr
}

func (r *CAPIImportManagementV3Reconciler) reconcileNormal(ctx context.Context, capiCluster *clusterv1.Cluster,
	rancherCluster *managementv3.Cluster,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	clusterMissing := rancherCluster == nil

	updatedCluster := &managementv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    capiCluster.Namespace,
			GenerateName: "c-",
			Labels: map[string]string{
				capiClusterOwner:          capiCluster.Name,
				capiClusterOwnerNamespace: capiCluster.Namespace,
				ownedLabelName:            "",
			},
			Annotations: map[string]string{
				fleetNamespaceMigrated: "cattle-fleet-system",
			},
			Finalizers: []string{
				managementv3.CapiClusterFinalizer,
			},
		},
		Spec: managementv3.ClusterSpec{
			DisplayName: capiCluster.Name,
			Description: "CAPI cluster imported to Rancher",
		},
	}

	rancherCluster = cmp.Or(rancherCluster, updatedCluster)

	r.optOutOfClusterOwner(ctx, rancherCluster)
	r.optOutOfFleetManagement(ctx, rancherCluster)

	addedFinalizer := controllerutil.AddFinalizer(rancherCluster, managementv3.CapiClusterFinalizer)
	if addedFinalizer {
		log.Info("Successfully added capicluster.turtles.cattle.io finalizer to Rancher cluster")
	}

	if clusterMissing {
		if autoImport, err := r.shouldAutoImportUncached(ctx, capiCluster); err != nil || !autoImport {
			return ctrl.Result{}, err
		}

		if err := r.RancherClient.Create(ctx, rancherCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating rancher cluster: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	fleetMigrated := false
	if labels := capiCluster.GetLabels(); labels != nil {
		_, fleetMigrated = labels[fleetDisabledLabel]
	}

	annotations := rancherCluster.GetAnnotations()
	fleetMigrated = annotations[fleetNamespaceMigrated] == "cattle-fleet-system" || fleetMigrated

	if conditions.IsTrue(rancherCluster, managementv3.ClusterConditionReady) && fleetMigrated {
		log.Info("agent is ready, no action needed")

		return ctrl.Result{}, nil
	} else if conditions.IsTrue(rancherCluster, managementv3.ClusterConditionReady) {
		// Delete old agent namespace on the downstream cluster
		remoteClient, err := r.remoteClientGetter(ctx, capiCluster.Name, r.Client, client.ObjectKeyFromObject(capiCluster))
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("getting remote cluster client: %w", err)
		}

		if requeue, err := removeFleetNamespace(ctx, remoteClient, rancherCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("cleaning up fleet namespace: %w", err)
		} else if requeue {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, nil
	}

	// Get custom CAcert if agentTLSMode feature is enabled
	caCert, err := getTrustedCAcert(ctx, r.Client, feature.Gates.Enabled(feature.AgentTLSMode))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error getting CA cert: %w", err)
	}

	// get the registration manifest
	manifest, err := getClusterRegistrationManifest(ctx, rancherCluster.Name, rancherCluster.Name, r.RancherClient, caCert, r.InsecureSkipVerify)
	if err != nil {
		return ctrl.Result{}, err
	}

	if manifest == "" {
		log.Info("Import manifest URL not set yet, requeue")
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("Creating import manifest")

	remoteClient, err := r.remoteClientGetter(ctx, capiCluster.Name, r.Client, client.ObjectKeyFromObject(capiCluster))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting remote cluster client: %w", err)
	}

	if requeue, err := validateImportReadiness(ctx, remoteClient, strings.NewReader(manifest)); err != nil {
		return ctrl.Result{}, fmt.Errorf("verifying import manifest: %w", err)
	} else if requeue {
		log.Info("Import manifests are being deleted, not ready to be applied yet, requeue")
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if err := createImportManifest(ctx, remoteClient, strings.NewReader(manifest)); err != nil {
		return ctrl.Result{}, fmt.Errorf("creating import manifest: %w", err)
	}

	log.Info("Successfully applied import manifest")

	return ctrl.Result{}, nil
}

func (r *CAPIImportManagementV3Reconciler) shouldAutoImportUncached(ctx context.Context, capiCluster *clusterv1.Cluster) (bool, error) {
	log := log.FromContext(ctx)

	if err := r.UncachedClient.Get(ctx, client.ObjectKeyFromObject(capiCluster), capiCluster); err != nil {
		return false, client.IgnoreNotFound(err)
	}

	if shouldImport, err := util.ShouldAutoImport(ctx, log, r.Client, capiCluster, importLabelName); err != nil {
		return false, err
	} else if !shouldImport {
		log.Info("not auto importing cluster as namespace or cluster isn't marked auto import")

		return false, nil
	}

	return true, nil
}

func (r *CAPIImportManagementV3Reconciler) rancherV3ClusterToCapiCluster(ctx context.Context, clusterPredicate predicate.Funcs) handler.MapFunc {
	log := log.FromContext(ctx)

	return func(_ context.Context, cluster client.Object) []ctrl.Request {
		labels := cluster.GetLabels()
		if _, ok := labels[ownedLabelName]; !ok { // Ignore clusters that are not owned by turtles
			log.V(5).Info(missingLabelMsg+ownedLabelName, "cluster", cluster.GetName())
			return nil
		}

		if _, ok := labels[capiClusterOwner]; !ok {
			log.V(5).Info(missingLabelMsg+capiClusterOwner, "cluster", cluster.GetName())
			return nil
		}

		if _, ok := labels[capiClusterOwnerNamespace]; !ok {
			log.V(5).Info(missingLabelMsg+capiClusterOwnerNamespace, "cluster", cluster.GetName())
			return nil
		}

		capiCluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name:      labels[capiClusterOwner],
			Namespace: labels[capiClusterOwnerNamespace],
		}}

		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(capiCluster), capiCluster); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "getting capi cluster")
			}

			return nil
		}

		if !clusterPredicate.Generic(event.GenericEvent{Object: capiCluster}) {
			return nil
		}

		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: capiCluster.Namespace, Name: capiCluster.Name}}}
	}
}

func (r *CAPIImportManagementV3Reconciler) rancherV1ClusterToCapiCluster(ctx context.Context, clusterPredicate predicate.Funcs) handler.MapFunc {
	log := log.FromContext(ctx)

	return func(_ context.Context, cluster client.Object) []ctrl.Request {
		labels := cluster.GetLabels()
		if _, ok := labels[ownedLabelName]; !ok { // Ignore clusters that are not owned by turtles
			log.V(5).Info(missingLabelMsg+ownedLabelName, "cluster", cluster.GetName())
			return nil
		}

		annotations := cluster.GetAnnotations()
		if _, ok := annotations[v1ClusterMigrated]; ok { // Ignore watching clusters that are already migrated
			log.V(5).Info("migrated annotation is present"+v1ClusterMigrated, "cluster", cluster.GetName())
			return nil
		}

		capiCluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name:      turtlesnaming.Name(cluster.GetName()).ToCapiName(),
			Namespace: cluster.GetNamespace(),
		}}

		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(capiCluster), capiCluster); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "getting capi cluster")
			}

			return nil
		}

		if !clusterPredicate.Generic(event.GenericEvent{Object: capiCluster}) {
			return nil
		}

		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: capiCluster.Namespace, Name: capiCluster.Name}}}
	}
}

func (r *CAPIImportManagementV3Reconciler) reconcileDelete(ctx context.Context, capiCluster *clusterv1.Cluster) error {
	log := log.FromContext(ctx)
	log.Info("Reconciling rancher cluster deletion")

	// If the Rancher Cluster was already imported, then annotate the CAPI cluster so that we don't auto-import again.
	log.Info(fmt.Sprintf("Rancher cluster is being removed, annotating CAPI cluster %s with %s",
		capiCluster.Name,
		turtlesannotations.ClusterImportedAnnotation))

	patchBase := client.MergeFromWithOptions(capiCluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

	annotations := capiCluster.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[turtlesannotations.ClusterImportedAnnotation] = trueAnnotationValue
	capiCluster.SetAnnotations(annotations)
	controllerutil.RemoveFinalizer(capiCluster, managementv3.CapiClusterFinalizer)

	if err := r.Client.Patch(ctx, capiCluster, patchBase); err != nil {
		return fmt.Errorf("error removing finalizer: %w", err)
	}

	return nil
}

func (r *CAPIImportManagementV3Reconciler) deleteDependentRancherCluster(ctx context.Context, capiCluster *clusterv1.Cluster) error {
	log := log.FromContext(ctx)
	log.Info("capi cluster is being deleted, deleting dependent rancher cluster")

	selectors := []client.DeleteAllOfOption{
		client.MatchingLabels{
			capiClusterOwner:          capiCluster.Name,
			capiClusterOwnerNamespace: capiCluster.Namespace,
			ownedLabelName:            "",
		},
	}

	return client.IgnoreNotFound(r.RancherClient.DeleteAllOf(ctx, &managementv3.Cluster{}, selectors...))
}

// verifyV1ClusterMigration verifies if a v1 cluster has been successfully migrated.
// It checks if the v1 cluster exists for a v3 cluster and if it has the "cluster-api.cattle.io/migrated" annotation.
// If the cluster is not migrated yet, it returns false and requeues the reconciliation.
func (r *CAPIImportManagementV3Reconciler) verifyV1ClusterMigration(ctx context.Context, capiCluster *clusterv1.Cluster) (bool, error) {
	log := log.FromContext(ctx)

	v1rancherCluster := &provisioningv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: capiCluster.Namespace,
			Name:      turtlesnaming.Name(capiCluster.Name).ToRancherName(),
		},
	}

	err := r.RancherClient.Get(ctx, client.ObjectKeyFromObject(v1rancherCluster), v1rancherCluster)
	if client.IgnoreNotFound(err) != nil {
		log.Error(err, fmt.Sprintf("Unable to fetch rancher cluster %s", client.ObjectKeyFromObject(v1rancherCluster)))
		return false, err
	}

	if apierrors.IsNotFound(err) {
		log.V(5).Info("V1 Cluster is migrated or doesn't exist, continuing with v3 reconciliation")

		return true, nil
	}

	if _, present := v1rancherCluster.Annotations[v1ClusterMigrated]; !present {
		log.Info("Cluster is not migrated yet, requeue, name", "name", v1rancherCluster.Name)

		return false, nil
	}

	return true, nil
}

// optOutOfClusterOwner annotates the cluster with the opt-out annotation.
// Rancher will detect this annotation and it won't create ProjectOwner or ClusterOwner roles.
func (r *CAPIImportManagementV3Reconciler) optOutOfClusterOwner(ctx context.Context, rancherCluster *managementv3.Cluster) {
	log := log.FromContext(ctx)

	annotations := rancherCluster.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	if _, ok := annotations[turtlesannotations.NoCreatorRBACAnnotation]; !ok {
		log.Info(fmt.Sprintf("Rancher cluster '%s' does not have the NoCreatorRBACAnnotation annotation: '%s', annotating it",
			rancherCluster.Name,
			turtlesannotations.ClusterImportedAnnotation))

		annotations[turtlesannotations.NoCreatorRBACAnnotation] = trueAnnotationValue
		rancherCluster.SetAnnotations(annotations)
	}
}

// optOutOfFleetManagement annotates the cluster with the fleet provisioning opt-out annotation,
// allowing external fleet cluster management.
func (r *CAPIImportManagementV3Reconciler) optOutOfFleetManagement(ctx context.Context, rancherCluster *managementv3.Cluster) {
	log := log.FromContext(ctx)

	annotations := rancherCluster.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	if _, found := annotations[externalFleetAnnotation]; !found {
		annotations[externalFleetAnnotation] = trueAnnotationValue
		rancherCluster.SetAnnotations(annotations)

		log.Info("Added fleet annotation to Rancher cluster")
	}
}
