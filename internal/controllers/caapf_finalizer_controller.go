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
	"errors"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/predicates"

	fleetv1 "github.com/rancher/turtles/api/fleet/v1alpha1"
)

// FleetAddonFinalizer is the finalizer added by CAAPF to guard cleanup.
const FleetAddonFinalizer = "fleet.addons.cluster.x-k8s.io"

// errCAAPFMigrationIncomplete is logged while a CAPI cluster still has a CAAPF-owned Fleet cluster.
var errCAAPFMigrationIncomplete = errors.New("CAAPF migration is not complete for this cluster")

// CAAPFFinalizerReconciler removes the finalizer left by CAAPF on CAPI clusters once their CAAPF
// migration is complete, or when they are being deleted.
//
// The post phase of scripts/migrate-caapf.sh deletes the CAAPF-owned Fleet cluster (a Fleet cluster
// owned by the CAPI cluster) only after all of its bundles are present on the Rancher-managed Fleet
// cluster, so the absence of such a Fleet cluster is the per-cluster migration-complete signal. Until
// then the finalizer is kept, as the pre phase uses it to discover clusters to migrate.
//
// Deletion is never blocked. The import controller deletes the Rancher cluster of a CAPI cluster being
// deleted, after which the post phase skips it, so waiting for the migration would block forever.
//
// This runs independently of the import controller, so it also covers CAPI clusters that controller
// skips, such as clusters with the imported=true annotation, clusters not marked for import, clusters
// without an available control plane and clusters being deleted.
type CAAPFFinalizerReconciler struct {
	Client           client.Client
	Scheme           *runtime.Scheme
	WatchFilterValue string
}

// SetupWithManager sets up reconciler with manager.
func (r *CAAPFFinalizerReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	hasCAAPFFinalizer := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return controllerutil.ContainsFinalizer(obj, FleetAddonFinalizer)
	})

	if err := ctrl.NewControllerManagedBy(mgr).
		Named("caapf-finalizer").
		WithOptions(options).
		For(&clusterv1.Cluster{}, builder.WithPredicates(
			predicates.ResourceHasFilterLabel(r.Scheme, log, r.WatchFilterValue),
			hasCAAPFFinalizer,
		)).
		// Re-check the owning CAPI cluster as soon as its CAAPF-owned Fleet cluster is removed.
		Watches(
			&fleetv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(fleetClusterToCAPIOwner),
			builder.WithPredicates(fleetClusterOwnerChanged()),
		).
		Complete(r); err != nil {
		return fmt.Errorf("initializing CAAPFFinalizerReconciler builder: %w", err)
	}

	return nil
}

// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=fleet.cattle.io,resources=clusters,verbs=get;list;watch

// Reconcile removes the CAAPF finalizer from a CAPI cluster whose CAAPF migration is complete or that is
// being deleted.
func (r *CAAPFFinalizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	capiCluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, capiCluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !controllerutil.ContainsFinalizer(capiCluster, FleetAddonFinalizer) {
		return ctrl.Result{}, nil
	}

	// The Fleet cluster watch maps to owners without applying the watch filter.
	if r.WatchFilterValue != "" && capiCluster.Labels[clusterv1.WatchLabel] != r.WatchFilterValue {
		return ctrl.Result{}, nil
	}

	// A cluster being deleted is not blocked: its Fleet resources go away with it, and the migration
	// cannot complete anyway once the import controller has deleted its Rancher cluster.
	if capiCluster.DeletionTimestamp.IsZero() {
		fleetCluster, found, err := r.caapfOwnedFleetCluster(ctx, capiCluster)
		if err != nil {
			return ctrl.Result{}, err
		}

		if found {
			log.Error(errCAAPFMigrationIncomplete,
				"Keeping CAAPF finalizer on CAPI cluster, run scripts/migrate-caapf.sh to migrate its Fleet resources",
				"fleetCluster", fleetCluster)

			// The Fleet cluster watch triggers a new reconcile once it is removed.
			return ctrl.Result{}, nil
		}
	}

	removed := false

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Client.Get(ctx, req.NamespacedName, capiCluster); err != nil {
			return err
		}

		if !controllerutil.RemoveFinalizer(capiCluster, FleetAddonFinalizer) {
			return nil
		}

		if err := r.Client.Update(ctx, capiCluster); err != nil {
			return err
		}

		removed = true

		return nil
	})
	// Removing the last finalizer of a cluster being deleted lets it go away.
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("removing CAAPF finalizer from CAPI cluster: %w", err)
	}

	if removed {
		log.Info("Removed CAAPF finalizer from CAPI cluster")
	}

	return ctrl.Result{}, nil
}

// caapfOwnedFleetCluster returns the key of a Fleet cluster owned by the CAPI cluster and whether one
// still exists. Fleet clusters registered by Rancher are owned by the Rancher cluster instead, so they
// are not matched.
func (r *CAAPFFinalizerReconciler) caapfOwnedFleetCluster(
	ctx context.Context, capiCluster *clusterv1.Cluster,
) (client.ObjectKey, bool, error) {
	fleetClusters := &fleetv1.ClusterList{}
	if err := r.Client.List(ctx, fleetClusters, client.InNamespace(capiCluster.Namespace)); err != nil {
		return client.ObjectKey{}, false, fmt.Errorf("listing Fleet clusters: %w", err)
	}

	for i := range fleetClusters.Items {
		owner, ok := capiClusterOwner(&fleetClusters.Items[i])
		if ok && owner.Name == capiCluster.Name && owner.UID == capiCluster.UID {
			return client.ObjectKeyFromObject(&fleetClusters.Items[i]), true, nil
		}
	}

	return client.ObjectKey{}, false, nil
}

// fleetClusterOwnerChanged filters out Fleet cluster updates that leave its CAPI cluster owner as it
// is, such as status updates. Only creating or deleting a Fleet cluster, or changing its CAPI cluster
// owner, can change whether a CAPI cluster still has a CAAPF-owned Fleet cluster.
func fleetClusterOwnerChanged() predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldOwner, oldFound := capiClusterOwner(e.ObjectOld)
			newOwner, newFound := capiClusterOwner(e.ObjectNew)

			return oldFound != newFound || oldOwner.Name != newOwner.Name || oldOwner.UID != newOwner.UID
		},
	}
}

// fleetClusterToCAPIOwner enqueues the CAPI cluster owning a Fleet cluster.
func fleetClusterToCAPIOwner(_ context.Context, obj client.Object) []ctrl.Request {
	owner, ok := capiClusterOwner(obj)
	if !ok {
		return nil
	}

	return []ctrl.Request{{Namespace: obj.GetNamespace(), Name: owner.Name}}
}

// capiClusterOwner returns the CAPI cluster owner reference from the object's owner references.
func capiClusterOwner(obj metav1.Object) (metav1.OwnerReference, bool) {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Kind == "Cluster" && strings.HasPrefix(ref.APIVersion, clusterv1.GroupVersion.Group+"/") {
			return ref, true
		}
	}

	return metav1.OwnerReference{}, false
}
