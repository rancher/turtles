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
	"time"

	k3sv1 "github.com/rancher/turtles/api/rancher/k3s/v1"
	"github.com/rancher/turtles/exp/etcdrestore/controllers/snapshotters"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	capiutil "sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	RKE2ControlPlaneKind = "RKE2ControlPlane"
)

// EtcdSnapshotSyncReconciler reconciles a EtcdSnapshotSync object.
type EtcdSnapshotSyncReconciler struct {
	client.Client
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

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=rke2etcdmachinesnapshotconfigs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=rke2etcdmachinesnapshotconfigs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=rke2etcdmachinesnapshotconfigs/finalizers,verbs=update

func (r *EtcdSnapshotSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling CAPI cluster and syncing etcd snapshots")

	cluster := &clusterv1.Cluster{}
	if err := r.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	if cluster.Spec.Paused {
		log.Info("Cluster is paused, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Only reconcile RKE2 clusters
	if cluster.Spec.ControlPlaneRef.Kind != RKE2ControlPlaneKind { // TODO: Move to predicate
		log.Info("Cluster is not an RKE2 cluster, skipping reconciliation")
		return ctrl.Result{RequeueAfter: 3 * time.Minute}, nil
	}

	if !cluster.Status.ControlPlaneReady {
		log.Info("Control plane is not ready, skipping reconciliation")
		return ctrl.Result{RequeueAfter: 3 * time.Minute}, nil
	}

	if err := r.watchEtcdSnapshotFiles(ctx, cluster); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to start watch on ETCDSnapshotFile: %w", err)
	}

	return r.reconcileNormal(ctx, cluster)
}

func (r *EtcdSnapshotSyncReconciler) reconcileNormal(ctx context.Context, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	remoteClient, err := r.Tracker.GetClient(ctx, capiutil.ObjectKey(cluster))
	if err != nil {
		return ctrl.Result{}, err
	}

	var snapshotter snapshotters.Snapshotter

	switch cluster.Spec.ControlPlaneRef.Kind {
	case RKE2ControlPlaneKind:
		snapshotter = snapshotters.NewRKE2Snapshotter(r.Client, remoteClient, cluster)
	default:
		return ctrl.Result{}, fmt.Errorf("unsupported control plane kind: %s", cluster.Spec.ControlPlaneRef.Kind)
	}

	if err := snapshotter.Sync(ctx); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to sync etcd snapshots: %w", err)
	}

	return ctrl.Result{}, nil
}

func (r *EtcdSnapshotSyncReconciler) watchEtcdSnapshotFiles(ctx context.Context, cluster *clusterv1.Cluster) error {
	log := ctrl.LoggerFrom(ctx)

	log.V(5).Info("Setting up watch on ETCDSnapshotFile")

	return r.Tracker.Watch(ctx, remote.WatchInput{
		Name:         "ETCDSnapshotFiles-watcher",
		Cluster:      capiutil.ObjectKey(cluster),
		Watcher:      r.controller,
		Kind:         &k3sv1.ETCDSnapshotFile{},
		EventHandler: handler.EnqueueRequestsFromMapFunc(r.etcdSnapshotFile(ctx, cluster)),
	})
}

func (r *EtcdSnapshotSyncReconciler) etcdSnapshotFile(ctx context.Context, cluster *clusterv1.Cluster) handler.MapFunc {
	log := log.FromContext(ctx)

	return func(_ context.Context, o client.Object) []ctrl.Request {
		log.Info("Cluster name", "name", cluster.GetName())

		gvk := schema.GroupVersionKind{
			Group:   k3sv1.GroupVersion.Group,
			Kind:    "ETCDSnapshotFile",
			Version: k3sv1.GroupVersion.Version,
		}

		if o.GetObjectKind().GroupVersionKind() != gvk {
			log.Error(fmt.Errorf("got a different object"), "objectGVK", o.GetObjectKind().GroupVersionKind().String())
			return nil
		}

		return []reconcile.Request{{NamespacedName: capiutil.ObjectKey(cluster)}}
	}
}
