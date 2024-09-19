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
	"sigs.k8s.io/yaml"

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
func (r *ClusterctlConfigReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, _ controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&turtlesv1.ClusterctlConfig{}).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(configMapMapper)).
		Complete(r)
	if err != nil {
		return fmt.Errorf("creating ClusterctlConfigReconciler controller: %w", err)
	}

	// This needs to be created in the empty state, so that controller can sync the changes
	// and have the initial resource to receive reconcile request from
	configMapTemplate := clusterctl.Config()
	configMapTemplate.Data = nil

	err = r.Client.Patch(ctx, configMapTemplate, client.Apply, []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner("clusterctl-controller"),
	}...)
	if err != nil {
		return fmt.Errorf("creating ClusterctlConfig default ConfigMap: %w", err)
	}

	return nil
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=clusterctlconfigs,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=clusterctlconfigs/status,verbs=get;list;watch;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=clusterctlconfigs/finalizers,verbs=get;list;watch;patch;update
//+kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;patch

// Reconcile reconciles the EtcdMachineSnapshot object.
func (r *ClusterctlConfigReconciler) Reconcile(ctx context.Context, _ reconcile.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	clusterctlConfig, err := clusterctl.ClusterConfig(ctx, r.Client)
	if err != nil {
		log.Error(err, "Unable to serialize updated clusterctl config")

		return ctrl.Result{}, err
	}

	clusterctlYaml, err := yaml.Marshal(clusterctlConfig)
	if err != nil {
		log.Error(err, "Unable to serialize updated clusterctl config")

		return ctrl.Result{}, err
	}

	configMap := clusterctl.Config()
	configMap.Data["clusterctl.yaml"] = string(clusterctlYaml)

	if err := r.Client.Patch(ctx, configMap, client.Apply, []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner("clusterctlconfig-controller"),
	}...); err != nil {
		log.Error(err, "Unable to patch clusterctl ConfigMap")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
