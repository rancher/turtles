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
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
)

// CAPICleanupReconciler is a reconciler for cleanup of managementv3 clusters.
type CAPICleanupReconciler struct {
	RancherClient client.Client
	Scheme        *runtime.Scheme
}

// SetupWithManager sets up reconciler with manager.
func (r *CAPICleanupReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, options controller.Options) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		Named("cleanup").
		For(&managementv3.Cluster{}).
		WithOptions(options).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			_, exist := object.GetLabels()[ownedLabelName]
			return exist
		})).
		Complete(reconcile.AsReconciler(r.RancherClient, r)); err != nil {
		return fmt.Errorf("creating new downgrade controller: %w", err)
	}

	return nil
}

// Reconcile performs check for clusters and removes finalizer on the clusters in deleteion
// still containing the turtles finalizer.
func (r *CAPICleanupReconciler) Reconcile(ctx context.Context, cluster *managementv3.Cluster) (res ctrl.Result, err error) {
	log := log.FromContext(ctx)

	patchBase := client.MergeFromWithOptions(cluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

	if cluster.DeletionTimestamp.IsZero() || !controllerutil.RemoveFinalizer(cluster, managementv3.CapiClusterFinalizer) {
		return
	}

	if err = r.RancherClient.Patch(ctx, cluster, patchBase); err != nil {
		log.Error(err, "Unable to remove turtles finalizer from cluster"+cluster.GetName())
	}

	return
}
