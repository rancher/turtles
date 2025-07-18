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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/sync"
)

// SyncReconciler reconciles a CAPIProvider dependent objects.
type SyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders/finalizers,verbs=update

// Reconcile reconciles the CAPIProvider object.
func (r *SyncReconciler) Reconcile(ctx context.Context, capiProvider *turtlesv1.CAPIProvider) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPIProvider")

	if !capiProvider.DeletionTimestamp.IsZero() {
		log.Info("Provider is in the process of deletion, skipping reconcile...")

		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, capiProvider)
}

// SetupWithManager sets up the controller with the Manager.
func (r *SyncReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, options controller.Options) (err error) {
	b := ctrl.NewControllerManagedBy(mgr).
		Named("CAPIProviderSync").
		WithOptions(options).
		For(&turtlesv1.CAPIProvider{})

	return b.Complete(reconcile.AsReconciler(r.Client, r))
}

func (r *SyncReconciler) reconcileNormal(ctx context.Context, capiProvider *turtlesv1.CAPIProvider) (_ ctrl.Result, err error) {
	return r.sync(ctx, capiProvider)
}

func (r *SyncReconciler) sync(ctx context.Context, capiProvider *turtlesv1.CAPIProvider) (_ ctrl.Result, err error) {
	s := sync.NewList(
		sync.NewSecretSync(r.Client, capiProvider),
		sync.NewSecretMapperSync(ctx, r.Client, capiProvider),
	)

	defer r.patchStatus(ctx, capiProvider, &err)

	if err := s.Sync(ctx); client.IgnoreNotFound(err) != nil {
		return ctrl.Result{}, err
	}
	defer s.Apply(ctx, &err)

	return ctrl.Result{}, nil
}

func (r *SyncReconciler) patchStatus(ctx context.Context, capiProvider *turtlesv1.CAPIProvider, err *error) {
	*err = sync.PatchStatus(ctx, r.Client, capiProvider)
}
