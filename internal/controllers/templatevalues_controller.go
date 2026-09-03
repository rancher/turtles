/*
Copyright © 2023 - 2026 SUSE LLC

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/predicates"

	turtlespredicates "github.com/rancher/turtles/util/predicates"
)

// TemplateValuesReconciler represents a reconciler for propagating the CAPI cluster's
// `spec.topology.variables` into the corresponding Fleet cluster's `spec.templateValues`.
type TemplateValuesReconciler struct {
	Client           client.Client
	Scheme           *runtime.Scheme
	WatchFilterValue string
}

// SetupWithManager sets up reconciler with manager.
func (r *TemplateValuesReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	capiPredicates := predicates.All(r.Scheme, log,
		predicates.ResourceHasFilterLabel(r.Scheme, log, r.WatchFilterValue),
		turtlespredicates.ClustersWithTopologyVariables(log),
	)

	if err := ctrl.NewControllerManagedBy(mgr).
		Named("template-values").
		WithOptions(options).
		For(&clusterv1.Cluster{}, builder.WithPredicates(capiPredicates)).
		Complete(r); err != nil {
		return fmt.Errorf("creating templateValues controller: %w", err)
	}

	return nil
}

//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=management.cattle.io,resources=clusters,verbs=get;list;watch
//+kubebuilder:rbac:groups=fleet.cattle.io,resources=clusters,verbs=get;list;watch;patch

// Reconcile propagates a Cluster's cluster's `spec.topology.variables` into its
// corresponding Fleet cluster's `spec.templateValues`.
func (r *TemplateValuesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPI cluster")

	capiCluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, capiCluster); err != nil {
		if apierrors.IsNotFound(err) {
			// These may be requests enqueued from ManagementV3 Cluster deletion,
			// for a no longer existing CAPI Cluster.
			// Safe to ignore.
			return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
		}

		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, err
	}

	log = log.WithValues("cluster", capiCluster.Name)

	if !capiCluster.Spec.Topology.IsDefined() || len(capiCluster.Spec.Topology.Variables) == 0 {
		log.V(5).Info("Cluster was not created from a topology with variables, skipping")

		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}
