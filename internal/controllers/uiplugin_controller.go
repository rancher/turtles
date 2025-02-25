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
	"os"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// UIPluginReconciler reconciles a UIPlugin object.
type UIPluginReconciler struct {
	client.Client
	*runtime.Scheme
	UncachedClient client.Client
}

// SetupWithManager sets up the controller with the Manager.
func (r *UIPluginReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, _ controller.Options) error {
	uiPlugin := &metav1.PartialObjectMetadata{}
	uiPlugin.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "catalog.cattle.io",
		Version: "v1",
		Kind:    "UIPlugin",
	})

	if err := ctrl.NewControllerManagedBy(mgr).
		Named("ui-plugin").
		For(uiPlugin).
		WithEventFilter(predicate.NewPredicateFuncs(func(plugin client.Object) bool {
			return plugin.GetNamespace() == os.Getenv("POD_NAMESPACE")
		})).
		Complete(r); err != nil {
		return fmt.Errorf("creating UIPlugin controller: %w", err)
	}

	return nil
}

//+kubebuilder:rbac:groups=catalog.cattle.io,resources=uiplugins,verbs=get;list;watch;create;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resourceNames=rancher-turtles-manager-role,resources=clusterroles,verbs=get;list

// Reconcile moves the UIPlugin into cattle-ui-plugin-system namespace.
func (r *UIPluginReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	plugin := &unstructured.Unstructured{}
	plugin.SetKind("UIPlugin")
	plugin.SetAPIVersion("catalog.cattle.io/v1")

	if err := r.Client.Get(ctx, req.NamespacedName, plugin); err != nil {
		log.Error(err, "Unable to get UIPlugin")

		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if plugin.GetDeletionTimestamp() != nil {
		return ctrl.Result{}, nil
	}

	role := &rbacv1.ClusterRole{}
	if err := r.UncachedClient.Get(ctx, types.NamespacedName{
		Name: "rancher-turtles-manager-role",
	}, role); err != nil {
		log.Error(err, "Unable to get turtles clusterRole")

		return ctrl.Result{}, err
	}

	destination := &unstructured.Unstructured{}
	destination.SetGroupVersionKind(plugin.GroupVersionKind())
	destination.SetName(plugin.GetName())
	destination.SetNamespace("cattle-ui-plugin-system")
	destination.Object["spec"] = plugin.Object["spec"]

	if err := controllerutil.SetOwnerReference(role, destination, r.Scheme); err != nil {
		log.Error(err, "Unable to set ClusterRole owner on UIPlugin")

		return ctrl.Result{}, err
	}

	if err := r.Patch(ctx, destination, client.Apply, []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner("ui-plugin-controller"),
	}...); err != nil {
		log.Error(err, "Unable to patch UIPlugin")

		return ctrl.Result{}, err
	}

	if err := r.Delete(ctx, plugin); err != nil {
		log.Error(err, "Unable to cleanup source UIPlugin")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
