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
	"fmt"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	turtlespredicates "github.com/rancher/turtles/util/predicates"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
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
		For(&managementv3.Cluster{}). // TODO: Filter /owned and other labels.
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.CAPIClusterToRancherManagementCluster),
			builder.WithPredicates(turtlespredicates.TurtlesManagedClusterPredicates(ctx, log, r.Client, r.Scheme, r.WatchFilterValue)),
		).
		Complete(r); err != nil {
		return fmt.Errorf("initializing FleetReconciler builder: %w", err)
	}
	return nil
}

func (r *FleetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info(fmt.Sprintf("Reconciling Rancher Management Cluster %s", req.NamespacedName))

	// 1. Fetch the Management Cluster

	// 2. Fetch the CAPI Cluster via `cluster-api.cattle.io/capi-cluster-owner`
	// and `cluster-api.cattle.io/capi-cluster-owner-ns` labels

	// 3. Fetch the Fleet Cluster using managementCluster.name and managementCluster.spec.fleetWorkspaceName

	// 4. Propagate CAPI Cluster in Fleet Cluster templateValues

	// 5. Add clusterClassName and clusterClassNamespace to Fleet Cluster

	// 6. Create Fleet BundleNamespaceMapping to link ClusterClass namespace to Cluster namespace

	// 7. (Optional) Create ClusterClass related Fleet ClusterGroups

	// 8. Add a finalizer to Fleet Cluster to allow cleanup of above resources.

	// 9. Patch the Fleet Cluster

	return ctrl.Result{}, nil
}

func (r *FleetReconciler) CAPIClusterToRancherManagementCluster(ctx context.Context, obj client.Object) []ctrl.Request {
	logger := log.FromContext(ctx).
		WithValues("clusterNamespace", obj.GetNamespace()).
		WithValues("clusterName", obj.GetName())
	logger.Info("Enqueueing Rancher Cluster reconciliation from CAPI Cluster")

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

	// Find the associated Rancher Management Cluster.
	rancherClusterLabels := map[string]string{ // TODO: generalize this
		turtlesv1.LabelCAPIClusterOwnerName:      capiCluster.Name,
		turtlesv1.LabelCAPIClusterOwnerNamespace: capiCluster.Namespace,
		turtlesv1.LabelCAPIClusterOwned:          "",
	}

	rancherClusterList := &managementv3.ClusterList{}
	selectors := []client.ListOption{
		client.MatchingLabels(rancherClusterLabels),
	}

	if err := r.Client.List(ctx, rancherClusterList, selectors...); client.IgnoreNotFound(err) != nil {
		logger.Error(err, "Unable to list Rancher Management Clusters")
		return []ctrl.Request{}
	}

	rancherCluster := resolveMultipleRancherManagementClusters(logger, *rancherClusterList)
	if rancherCluster == nil {
		return []ctrl.Request{}
	}

	logger.Info("Adding Rancher Management Cluster to reconciliation request", "rancherClusterName", rancherCluster.Name)
	return []ctrl.Request{{NamespacedName: client.ObjectKeyFromObject(rancherCluster)}}
}
