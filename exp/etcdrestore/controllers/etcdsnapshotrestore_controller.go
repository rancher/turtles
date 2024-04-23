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

	"sigs.k8s.io/cluster-api/controllers/remote"

	snapshotrestorev1 "github.com/rancher/turtles/exp/etcdrestore/api/v1alpha1"
)

// EtcdSnapshotRestoreReconciler reconciles a EtcdSnapshotRestore object.
type EtcdSnapshotRestoreReconciler struct {
	client.Client
	WatchFilterValue string

	controller controller.Controller
	Tracker    *remote.ClusterCacheTracker
	Scheme     *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *EtcdSnapshotRestoreReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, _ controller.Options) error {
	// TODO: Setup predicates for the controller.
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&snapshotrestorev1.EtcdSnapshotRestore{}).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating etcdSnapshotRestore controller: %w", err)
	}

	r.controller = c

	return nil
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdsnapshotrestores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdsnapshotrestores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdsnapshotrestores/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets;events;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="management.cattle.io",resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=rke2configs;rke2configs/status;rke2configs/finalizers,verbs=get;list;watch;create;update;patch;delete

// Reconcile reconciles the EtcdSnapshotRestore object.
func (r *EtcdSnapshotRestoreReconciler) Reconcile(_ context.Context, _ ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *EtcdSnapshotRestoreReconciler) reconcileNormal(
	_ context.Context, _ *snapshotrestorev1.EtcdSnapshotRestore,
) (_ ctrl.Result, err error) {
	return ctrl.Result{}, nil
}

func (r *EtcdSnapshotRestoreReconciler) reconcileDelete(
	_ context.Context, _ *snapshotrestorev1.EtcdSnapshotRestore,
) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
