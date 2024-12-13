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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/sync"
)

// CAPIProviderReconciler reconciles a CAPIProvider object.
type CAPIProviderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders/finalizers,verbs=update
//+kubebuilder:rbac:groups=operator.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles the CAPIProvider object.
func (r *CAPIProviderReconciler) Reconcile(ctx context.Context, capiProvider *turtlesv1.CAPIProvider) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPIProvider")

	if !capiProvider.DeletionTimestamp.IsZero() {
		log.Info("Provider is in the process of deletion, skipping reconcile...")

		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, capiProvider)
}

func (r *CAPIProviderReconciler) reconcileNormal(ctx context.Context, capiProvider *turtlesv1.CAPIProvider) (_ ctrl.Result, err error) {
	return r.sync(ctx, capiProvider)
}

func (r *CAPIProviderReconciler) sync(ctx context.Context, capiProvider *turtlesv1.CAPIProvider) (_ ctrl.Result, err error) {
	s := sync.NewList(
		sync.NewSecretSync(r.Client, capiProvider),
		sync.NewSecretMapperSync(ctx, r.Client, capiProvider),
	)

	switch cmp.Or(capiProvider.Spec.Name, capiProvider.GetName()) {
	case "azure":
		s = append(s, sync.NewAzureProviderSync(r.Client, capiProvider))
	case "gcp":
		s = append(s, sync.NewGCPProviderSync(r.Client, capiProvider))
	default:
		s = append(s, sync.NewProviderSync(r.Client, capiProvider))
	}

	defer r.patchStatus(ctx, capiProvider, &err)

	if err := s.Sync(ctx); client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}
	defer s.Apply(ctx, &err)

	return ctrl.Result{}, nil
}

func (r *CAPIProviderReconciler) patchStatus(ctx context.Context, capiProvider *turtlesv1.CAPIProvider, err *error) {
	*err = sync.PatchStatus(ctx, r.Client, capiProvider)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CAPIProviderReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, options controller.Options) (err error) {
	b := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&turtlesv1.CAPIProvider{})

	resources := []client.Object{
		&operatorv1.CoreProvider{},
		&operatorv1.ControlPlaneProvider{},
		&operatorv1.InfrastructureProvider{},
		&operatorv1.BootstrapProvider{},
		&operatorv1.AddonProvider{},
		&corev1.Secret{},
	}

	for _, resource := range resources {
		b = b.Owns(resource)
	}

	return b.Complete(reconcile.AsReconciler(r.Client, r))
}
