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

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
)

// EtcdSnapshotSyncReconciler reconciles a EtcdSnapshotSync object.
type EtcdSnapshotSyncReconciler struct {
	Client           client.Client
	WatchFilterValue string

	controller controller.Controller
	Tracker    *remote.ClusterCacheTracker
}

func (r *EtcdSnapshotSyncReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, _ controller.Options) error {
	// TODO: Setup predicates for the controller.
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating etcdSnapshotSync controller: %w", err)
	}

	r.controller = c

	return nil
}

func (r *EtcdSnapshotSyncReconciler) Reconcile(_ context.Context, _ ctrl.Request) (res ctrl.Result, reterr error) {
	return ctrl.Result{}, nil
}
