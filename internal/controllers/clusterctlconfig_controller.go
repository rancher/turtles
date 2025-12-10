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

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/controllers/clusterctl"
)

// ClusterctlConfigReconciler reconciles a ClusterctlConfig object.
type ClusterctlConfigReconciler struct {
	client.Client
}

// Config is a direct clusterctl config representation.
type Config struct {
	Providers turtlesv1.ProviderList `json:"providers"`
	Images    map[string]ConfigImage `json:"images"`
}

// ConfigImage is a direct clusterctl representation of image config value.
type ConfigImage struct {
	// Repository sets the container registry override to pull images from.
	Repository string `json:"repository,omitempty"`

	// Tag allows to specify a tag for the images.
	Tag string `json:"tag,omitempty"`
}

func configMapMapper(_ context.Context, obj client.Object) []reconcile.Request {
	if obj.GetName() != turtlesv1.ClusterctlConfigName {
		return []reconcile.Request{}
	}

	return []reconcile.Request{{NamespacedName: client.ObjectKeyFromObject(obj)}}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterctlConfigReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, _ controller.Options) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&turtlesv1.ClusterctlConfig{}).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(configMapMapper)).
		Complete(r); err != nil {
		return fmt.Errorf("creating ClusterctlConfigReconciler controller: %w", err)
	}

	return nil
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=clusterctlconfigs,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=clusterctlconfigs/status,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=clusterctlconfigs/finalizers,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch
//+kubebuilder:rbac:groups="management.cattle.io",resources=settings,verbs=get;list;watch

// Reconcile reconciles the ClusterctlConfig object.
func (r *ClusterctlConfigReconciler) Reconcile(ctx context.Context, _ reconcile.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if err := clusterctl.SyncConfigMap(ctx, r.Client, "clusterctlconfig-controller"); err != nil {
		log.Error(err, "Unable to sync clusterctl ConfigMap")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
