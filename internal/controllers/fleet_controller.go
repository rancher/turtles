/*
Copyright © 2026 SUSE LLC

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
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	fleetv1 "github.com/rancher/turtles/api/fleet/v1alpha1"
	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/util"
	turtlespredicates "github.com/rancher/turtles/util/predicates"
)

// FleetReconciler reconciles the fleet.cattle.io Cluster and other Fleet resources.
type FleetReconciler struct {
	Client           client.Client
	Scheme           *runtime.Scheme
	WatchFilterValue string
}

// SetupWithManager sets up reconciler with manager.
func (r *FleetReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	if err := ctrl.NewControllerManagedBy(mgr).
		Named("fleet-cluster").
		WithOptions(options).
		For(&fleetv1.Cluster{}, builder.WithPredicates(turtlespredicates.FleetClusterOwnedByRancherCluster(log))).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.CAPIClusterToFleetCluster),
			builder.WithPredicates(turtlespredicates.TurtlesManagedClusterPredicates(ctx, log, r.Client, r.Scheme, r.WatchFilterValue)),
		).
		Complete(r); err != nil {
		return fmt.Errorf("initializing FleetReconciler builder: %w", err)
	}

	return nil
}

// CAPIClusterToFleetCluster enqueues Fleet Cluster requests from associated CAPI Clusters.
func (r *FleetReconciler) CAPIClusterToFleetCluster(ctx context.Context, obj client.Object) []ctrl.Request {
	logger := log.FromContext(ctx).
		WithValues("clusterNamespace", obj.GetNamespace()).
		WithValues("clusterName", obj.GetName())
	logger.V(5).Info("Enqueueing Fleet Cluster reconciliation from CAPI Cluster.")

	// Verify we are actually handling a CAPI Cluster object.
	capiCluster, ok := obj.(*clusterv1.Cluster)
	if !ok {
		logger.Error(ErrEnqueueing, fmt.Sprintf("Expected a CAPI Cluster object, but got %T", obj))
		return []ctrl.Request{}
	}

	// Fetch the CAPI Cluster.
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(capiCluster), capiCluster); err != nil {
		if apierrors.IsNotFound(err) {
			logger.V(5).Info("CAPI Cluster not found. Nothing to do.")
			return []ctrl.Request{}
		}

		logger.Error(ErrEnqueueing, fmt.Errorf("getting CAPI Cluster: %w", err).Error())

		return []ctrl.Request{}
	}

	rancherClusterList := &managementv3.ClusterList{}
	selectors := []client.ListOption{
		client.MatchingLabels(util.RancherClusterManagedLabels(*capiCluster)),
	}

	if err := r.Client.List(ctx, rancherClusterList, selectors...); client.IgnoreNotFound(err) != nil {
		logger.Error(err, "Unable to list Fleet Clusters.")
		return []ctrl.Request{}
	}

	rancherCluster := resolveMultipleRancherManagementClusters(logger, rancherClusterList)
	if rancherCluster == nil {
		logger.V(5).Info("CAPI Cluster has not been imported yet.")
		return []ctrl.Request{}
	}

	if len(rancherCluster.Spec.FleetWorkspaceName) == 0 {
		logger.V(5).Info("FleetWorkspace is not yet initialized.")
		return []ctrl.Request{}
	}

	fleetClusterKey := types.NamespacedName{
		Name:      rancherCluster.Name,
		Namespace: rancherCluster.Spec.FleetWorkspaceName,
	}

	logger.V(5).Info("Adding Fleet Cluster to reconciliation request", "fleetCluster", fleetClusterKey.String())

	return []ctrl.Request{{NamespacedName: fleetClusterKey}}
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=management.cattle.io,resources=clusters,verbs=get;list
// +kubebuilder:rbac:groups=fleet.cattle.io,resources=clusters,verbs=get;patch;list;watch
// +kubebuilder:rbac:groups=fleet.cattle.io,resources=bundlenamespacemappings,verbs=get;create;patch;delete

// Reconcile reconciles the Fleet Cluster.
func (r *FleetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info(fmt.Sprintf("Reconciling Fleet Cluster %s", req.NamespacedName))

	// Fetch the Fleet Cluster
	fleetCluster := &fleetv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: req.Namespace,
		},
	}

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(fleetCluster), fleetCluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Fleet Cluster not yet initialized.")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("getting Fleet Cluster: %w", err)
	}

	fleetClusterPatchBase := client.MergeFromWithOptions(fleetCluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

	// Set the finalizer if not up for deletion already and not done yet
	if fleetCluster.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(fleetCluster, fleetv1.FleetClusterFinalizer) {
		log.V(5).Info("Setting finalizer on Fleet Cluster")

		if err := r.Client.Patch(ctx, fleetCluster, fleetClusterPatchBase); err != nil {
			return ctrl.Result{}, fmt.Errorf("patching FleetCluster: %w", err)
		}
	}

	// Reconcile Delete
	if !fleetCluster.DeletionTimestamp.IsZero() {
		log.Info("Reconciling Fleet Cluster Deletion")
		return r.ReconcileDelete(ctx, fleetCluster)
	}

	// Reconcile Normal
	return r.ReconcileNormal(ctx, fleetCluster)
}

// ReconcileNormal reconciles Fleet Cluster.
func (r *FleetReconciler) ReconcileNormal(ctx context.Context, fleetCluster *fleetv1.Cluster) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the Rancher Management Cluster
	if fleetCluster.Labels == nil {
		log.V(5).Info("No labels found. Can not determine Fleet Cluster owner.")
		return ctrl.Result{}, nil
	}

	rancherClusterName, found := fleetCluster.Labels[managementv3.LabelRancherOwnerName]
	if !found || len(rancherClusterName) == 0 {
		log.V(5).Info(managementv3.LabelRancherOwnerName + " label not found. Can not determine Fleet Cluster owner.")
		return ctrl.Result{}, nil
	}

	rancherCluster := managementv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: rancherClusterName,
		},
	}

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(&rancherCluster), &rancherCluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(5).Info("Rancher Management Cluster has been deleted. Nothing to do.")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("getting Rancher Management Cluster: %w", err)
	}

	// Fetch the CAPI Cluster via `cluster-api.cattle.io/capi-cluster-owner`
	// and `cluster-api.cattle.io/capi-cluster-owner-ns` labels
	capiClusterName, found := rancherCluster.Labels[turtlesv1.LabelCAPIClusterOwnerName]
	if !found {
		log.V(5).Info(
			fmt.Sprintf("Could not find label %s on Rancher Management Cluster. Skipping.",
				turtlesv1.LabelCAPIClusterOwnerName))

		return ctrl.Result{}, nil
	}

	capiClusterNamespace, found := rancherCluster.Labels[turtlesv1.LabelCAPIClusterOwnerNamespace]
	if !found {
		log.V(5).Info(
			fmt.Sprintf("Could not find label %s on Rancher Management Cluster. Skipping.",
				turtlesv1.LabelCAPIClusterOwnerNamespace))

		return ctrl.Result{}, nil
	}

	log = log.WithValues("capiClusterName", capiClusterName, "capiClusterNamespace", capiClusterNamespace)
	log.V(5).Info("Fetching associated CAPI Cluster.")

	capiCluster := clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      capiClusterName,
			Namespace: capiClusterNamespace,
		},
	}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(&capiCluster), &capiCluster); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("CAPI Cluster has been deleted. Nothing to do.")
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("getting CAPI Cluster: %w", err)
	}

	fleetClusterPatchBase := client.MergeFromWithOptions(fleetCluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

	// Reconcile template values
	if err := r.ReconcileTemplateValues(&capiCluster, fleetCluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("reconciling Template Values: %w", err)
	}

	// Reconcile CAPI ClusterClass
	if capiCluster.Spec.Topology.IsDefined() && len(capiCluster.Spec.Topology.ClassRef.Name) > 0 {
		if err := r.ReconcileClusterClass(ctx, &capiCluster, fleetCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("reconciling CAPI ClusterClass: %w", err)
		}
	}

	// Patch the Fleet Cluster
	if err := r.Client.Patch(ctx, fleetCluster, fleetClusterPatchBase); err != nil {
		return ctrl.Result{}, fmt.Errorf("patching FleetCluster: %w", err)
	}

	return ctrl.Result{}, nil
}

// ReconcileDelete reconciles Fleet Cluster deletion.
func (r *FleetReconciler) ReconcileDelete(ctx context.Context, fleetCluster *fleetv1.Cluster) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	clusterClassNamespace, usingClusterClass := fleetCluster.Labels[turtlesv1.LabelCAPIClusterClassNamespace]
	if usingClusterClass && clusterClassNamespace != fleetCluster.Namespace {
		log.V(5).Info("Cross-namespace ClusterClass detected. Cleaning up BundleNamespaceMapping.")

		sameClusterClassClustersList := &fleetv1.ClusterList{}
		sameClusterClassSelector := []client.ListOption{
			client.MatchingLabels(map[string]string{
				turtlesv1.LabelCAPIClusterClassNamespace: clusterClassNamespace,
			}),
		}

		if err := r.Client.List(ctx, sameClusterClassClustersList, sameClusterClassSelector...); client.IgnoreNotFound(err) != nil {
			log.Error(err, "Unable to list Fleet Management Clusters")

			return ctrl.Result{}, err
		}

		if len(sameClusterClassClustersList.Items) == 1 && sameClusterClassClustersList.Items[0].UID == fleetCluster.UID {
			log.V(5).Info("Removing orphan BundleNamespaceMapping")

			bundleNamespaceMapping := &fleetv1.BundleNamespaceMapping{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fleetCluster.Namespace,
					Namespace: clusterClassNamespace,
				},
			}

			if err := r.Client.Delete(ctx, bundleNamespaceMapping); err != nil {
				return ctrl.Result{}, fmt.Errorf("deleting BundleNamespaceMapping %s/%s: %w",
					clusterClassNamespace, fleetCluster.Namespace, err)
			}
		}
	}

	fleetClusterPatchBase := client.MergeFromWithOptions(fleetCluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

	if controllerutil.RemoveFinalizer(fleetCluster, fleetv1.FleetClusterFinalizer) {
		log.V(5).Info("Removing finalizer on Fleet Cluster")

		if err := r.Client.Patch(ctx, fleetCluster, fleetClusterPatchBase); err != nil {
			return ctrl.Result{}, fmt.Errorf("patching FleetCluster: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

// ReconcileClusterClass reconciles the CAPI ClusterClass.
func (r *FleetReconciler) ReconcileClusterClass(ctx context.Context, capiCluster *clusterv1.Cluster, fleetCluster *fleetv1.Cluster) error {
	log := log.FromContext(ctx)

	// Add clusterClassName and clusterClassNamespace to Fleet Cluster
	log.V(5).Info("Propagating ClusterClass name and namespace labels to Fleet Cluster.")

	if fleetCluster.Labels == nil {
		fleetCluster.Labels = map[string]string{}
	}

	fleetCluster.Labels[turtlesv1.LabelCAPIClusterClassName] = capiCluster.Spec.Topology.ClassRef.Name

	clusterClassNamespace := capiCluster.Spec.Topology.ClassRef.Namespace
	if len(clusterClassNamespace) == 0 {
		// Default to CAPI Cluster namespace if ClusterClass Namespace is not defined.
		clusterClassNamespace = capiCluster.Namespace
	}

	fleetCluster.Labels[turtlesv1.LabelCAPIClusterClassNamespace] = clusterClassNamespace

	// Create Fleet BundleNamespaceMapping to link ClusterClass namespace to Fleet Cluster namespace
	if clusterClassNamespace != fleetCluster.Namespace {
		log.V(5).Info("Cross-namespace ClusterClass detected. Reconciling Fleet BundleNamespaceMapping.")

		if err := r.ReconcileBundleNamespaceMapping(ctx, clusterClassNamespace, fleetCluster.Namespace); err != nil {
			return fmt.Errorf("reconciling BundleNamespaceMapping: %w", err)
		}
	}

	return nil
}

// ReconcileBundleNamespaceMapping reconciles the CAPI ClusterClass.
func (r *FleetReconciler) ReconcileBundleNamespaceMapping(ctx context.Context, sourceNamespace string, targetNamespace string) error {
	bundleNamespaceMapping := &fleetv1.BundleNamespaceMapping{
		ObjectMeta: metav1.ObjectMeta{
			Name:      targetNamespace,
			Namespace: sourceNamespace,
		},
	}

	if _, err := controllerutil.CreateOrPatch(ctx, r.Client, bundleNamespaceMapping, func() error {
		// Reconcile BundleNamespaceMapping
		if bundleNamespaceMapping.NamespaceSelector == nil {
			bundleNamespaceMapping.NamespaceSelector = &metav1.LabelSelector{}
		}

		if bundleNamespaceMapping.NamespaceSelector.MatchLabels == nil {
			bundleNamespaceMapping.NamespaceSelector.MatchLabels = map[string]string{}
		}

		bundleNamespaceMapping.NamespaceSelector.MatchLabels[corev1.LabelMetadataName] = targetNamespace

		return nil
	}); err != nil {
		return fmt.Errorf("patching BundleNamespaceMapping: %w", err)
	}

	return nil
}

// ReconcileTemplateValues propagates the CAPI Cluster in Fleet Cluster .spec.templateValues.
func (*FleetReconciler) ReconcileTemplateValues(capiCluster *clusterv1.Cluster, fleetCluster *fleetv1.Cluster) error {
	cluster := capiCluster.DeepCopy()
	// Strip .status, managed fields and resource version
	cluster.Status = clusterv1.ClusterStatus{}
	cluster.SetManagedFields(nil)
	cluster.SetResourceVersion("")

	clusterJSON, err := json.Marshal(&cluster)
	if err != nil {
		return fmt.Errorf("json encoding CAPI Cluster: %w", err)
	}

	fleetCluster.Spec.TemplateValues = map[string]apiextensionsv1.JSON{
		"Cluster": {Raw: clusterJSON},
	}

	return nil
}
