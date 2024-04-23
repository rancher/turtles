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

// EtcdMachineSnapshotReconciler reconciles an EtcdMachineSnapshot object.
type EtcdMachineSnapshotReconciler struct {
	client.Client
	WatchFilterValue string

	controller controller.Controller
	Tracker    *remote.ClusterCacheTracker
	Scheme     *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *EtcdMachineSnapshotReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, _ controller.Options) error {
	// TODO: Setup predicates for the controller.
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&snapshotrestorev1.EtcdMachineSnapshot{}).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating etcdMachineSnapshot controller: %w", err)
	}

	r.controller = c

	return nil
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdmachinesnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdmachinesnapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdmachinesnapshots/finalizers,verbs=update

// Reconcile reconciles the EtcdMachineSnapshot object.
func (r *EtcdMachineSnapshotReconciler) Reconcile(_ context.Context, _ ctrl.Request) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *EtcdMachineSnapshotReconciler) reconcileNormal(
	_ context.Context, _ *snapshotrestorev1.EtcdMachineSnapshot,
) (_ ctrl.Result, err error) {
	return ctrl.Result{}, nil
}

func (r *EtcdMachineSnapshotReconciler) reconcileDelete(
	_ context.Context, _ *snapshotrestorev1.EtcdMachineSnapshot,
) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}
