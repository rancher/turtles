/*
Copyright 2024.

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

package controller

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	clusterclassv1 "github.com/rancher/turtles/exp/clusterclass/api/v1alpha1"

	"github.com/rancher/turtles/exp/clusterclass/internal/matcher"
)

// ClusterUpgradeReconciler reconciles a ClusterUpgrade object
type ClusterUpgradeReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	controller      controller.Controller
	externalTracker external.ObjectTracker
}

var setupLog = ctrl.Log.WithName("setup")

// SetupWithManager sets up the controller with the Manager.
func (r *ClusterUpgradeReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterclassv1.ClusterUpgradeGroup{}).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating new controller: %w", err)
	}

	// TODO: watch CAPI clusters and ClusterClass
	err = c.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(r.capiClusterToClusterUpgradeGroup(ctx)),
	)
	if err != nil {
		return fmt.Errorf("adding watch for cluster upgrade group: %w", err)
	}

	err = c.Watch(
		source.Kind(mgr.GetCache(), &clusterv1.ClusterClass{}),
		handler.EnqueueRequestsFromMapFunc(r.clusterClassToClusterUpgradeGroup(ctx)),
	)
	if err != nil {
		return fmt.Errorf("adding watch for cluster class: %w", err)
	}

	r.controller = c
	r.externalTracker = external.ObjectTracker{
		Controller: c,
	}

	return nil
}

//+kubebuilder:rbac:groups=rollout.turtles-capi.cattle.io,resources=clusterupgradegroups,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rollout.turtles-capi.cattle.io,resources=clusterupgradegroups/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=rollout.turtles-capi.cattle.io,resources=clusterupgradegroupss/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;update;patch
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusterclasses,verbs=get;list;watch;update;patch

// Reconcile reconciles the ClusterUpgradeGroup object.
func (r *ClusterUpgradeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling ClusterUpgradeGroup")

	upgradeGroup := &clusterclassv1.ClusterUpgradeGroup{}
	if err := r.Client.Get(ctx, req.NamespacedName, upgradeGroup); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	timeRaw := upgradeGroup.Annotations["cc-time"]
	if len(timeRaw) == 0 {
		return ctrl.Result{}, nil
	}
	timeParsed, err := time.Parse(time.RFC3339, timeRaw)
	if err != nil {
		return ctrl.Result{}, err
	}

	if timeParsed.Before(time.Now()) {
		fmt.Println("time from annotation is after current time, executing rebase")
	} else {
		fmt.Println("time from annotation is before current time, not executing rebase")
		return ctrl.Result{Requeue: true}, nil
	}

	patchBase := client.MergeFromWithOptions(upgradeGroup.DeepCopy(), client.MergeFromWithOptimisticLock{})
	defer func() {
		if err := r.Client.Patch(ctx, upgradeGroup, patchBase); err != nil {
			reterr = err
		}
	}()

	log = log.WithValues("group", upgradeGroup.Name)

	if !upgradeGroup.DeletionTimestamp.IsZero() {
		// this won't be the case as there is no finalizer yet
		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, upgradeGroup, log)
}

func (r *ClusterUpgradeReconciler) reconcileNormal(ctx context.Context, upgradeGroup *clusterclassv1.ClusterUpgradeGroup, log logr.Logger) (ctrl.Result, error) {
	clusters := &clusterv1.ClusterList{}
	if err := r.Client.List(ctx, clusters); err != nil {
		return ctrl.Result{}, fmt.Errorf("listing clusters: %w", err)
	}

	requeue := false
	for _, target := range upgradeGroup.Spec.Targets {
		log = log.WithValues("target", target.Name)

		matched, err := r.getTargetClusters(ctx, &target, clusters.Items)
		if err != nil {
			setupLog.Info("reconcileNormal error getting target clusters: ", "error", err)
			return ctrl.Result{}, err
		}
		if len(matched) == 0 {
			log.V(4).Info("target has no matched clusters", "target", target.Name)
			continue
		}

		log.V(4).Info("matched cluster", "num", len(matched))
		needRequeue, err := r.rollout(ctx, upgradeGroup, matched, log)
		if err != nil {
			return ctrl.Result{Requeue: requeue}, err
		}
		if needRequeue {
			requeue = true
		}

	}

	return ctrl.Result{Requeue: requeue}, nil
}

func (r *ClusterUpgradeReconciler) getTargetClusters(ctx context.Context, target *clusterclassv1.ClusterTargets, clusters []clusterv1.Cluster) ([]*clusterv1.Cluster, error) {
	clusterMatcher, err := matcher.NewClusterMatcher(target.ClusterName, target.ClusterGroup, target.ClusterGroupSelector, target.ClusterSelector)
	if err != nil {
		return nil, fmt.Errorf("created cluster match for target %s: %w", target.Name, err)
	}

	matched := []*clusterv1.Cluster{}

	for _, cluster := range clusters {
		if cluster.Spec.Topology == nil {
			// Cluster no using clusterclass
			continue
		}

		if clusterMatcher.Match(cluster.Name, "", map[string]string{}, cluster.Labels) {
			current := cluster
			matched = append(matched, &current)
		}
	}
	return matched, nil
}

func (r *ClusterUpgradeReconciler) rollout(ctx context.Context, upgradeGroup *clusterclassv1.ClusterUpgradeGroup, clusters []*clusterv1.Cluster, log logr.Logger) (bool, error) {
	numReady := 0
	numNotReady := 0
	numNeedUpdate := 0

	// TODO: should we look at RollingUpdateInProgressReason = "RollingUpdateInProgress"
	// TODO: look for doNotDeploy

	// Summary first
	for _, cluster := range clusters {
		if cluster.Spec.Topology.Class != upgradeGroup.Spec.ClassName {
			numNeedUpdate++
		}

		readyCondition := conditions.Get(cluster, clusterv1.ReadyCondition)
		if readyCondition == nil {
			numNotReady++
			continue
		}
		if readyCondition.Status == v1.ConditionTrue {
			numReady++
			continue
		}

		numNotReady++
	}

	if numNeedUpdate == 0 {
		log.Info("no clusters need a rebase")

		return false, nil
	}

	maxUpdates := -1
	if upgradeGroup.Spec.RolloutStrategy != nil {
		// TODO: add other scenarios: for now only RollingUpdate is valid
		maxUnavailable := upgradeGroup.Spec.RolloutStrategy.RollingUpdate.MaxRollouts.IntVal
		if numNotReady >= int(maxUnavailable) {
			log.Info("maximum clusters unavailable, no rollout allowed", "maxunavialable", maxUnavailable, "needrebase", numNeedUpdate, "notready", numNotReady)

			return true, nil
		}
		maxUpdates = int(maxUnavailable) - numNotReady
	}

	numUpdated := 0
	for _, cluster := range clusters {
		if cluster.Spec.Topology.Class == upgradeGroup.Spec.ClassName {
			continue
		}

		if maxUpdates != -1 {
			if numUpdated >= maxUpdates {
				log.Info("maximum number of updates done, requeue")

				return true, nil
			}
		}

		updatedCluster := cluster.DeepCopy()
		updatedCluster.Spec.Topology.Class = upgradeGroup.Spec.ClassName

		if err := r.Client.Update(ctx, updatedCluster); err != nil {
			return true, fmt.Errorf("rebasing cluster: %w", err)
		}

		numUpdated++
	}

	return false, nil
}

func (r *ClusterUpgradeReconciler) capiClusterToClusterUpgradeGroup(ctx context.Context) handler.MapFunc {
	log := log.FromContext(ctx)
	return func(_ context.Context, o client.Object) []ctrl.Request {
		cluster, ok := o.(*clusterv1.Cluster)
		if !ok {
			log.Error(nil, fmt.Sprintf("Expected a capi cluster but got a %T", o))
			return nil
		}

		upgradeGroupList := &clusterclassv1.ClusterUpgradeGroupList{}
		if err := r.Client.List(ctx, upgradeGroupList, client.InNamespace(o.GetNamespace())); err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "UpgradeGroup list not found")
			}

			return nil
		}

		requests := make([]ctrl.Request, len(upgradeGroupList.Items))
		for _, item := range upgradeGroupList.Items {
			if item.Spec.Targets == nil {
				continue
			}
			for _, target := range item.Spec.Targets {
				if target.ClusterName == cluster.ObjectMeta.Name {
					requests = append(requests, reconcile.Request{
						NamespacedName: types.NamespacedName{
							Name:      item.GetName(),
							Namespace: item.GetNamespace(),
						},
					})
				}
			}
		}

		return requests
	}
}

func (r *ClusterUpgradeReconciler) clusterClassToClusterUpgradeGroup(ctx context.Context) handler.MapFunc {
	log := log.FromContext(ctx)
	return func(_ context.Context, o client.Object) []ctrl.Request {
		clusterClass, ok := o.(*clusterv1.ClusterClass)
		if !ok {
			log.Error(nil, fmt.Sprintf("Expected a cluster class but got a %T", o))
			return nil
		}

		upgradeGroupList := &clusterclassv1.ClusterUpgradeGroupList{}
		if err := r.Client.List(ctx, upgradeGroupList, client.InNamespace(clusterClass.ObjectMeta.Namespace)); err != nil {
			if apierrors.IsNotFound(err) {
				log.Error(err, "UpgradeGroup list not found")
			}

			return nil
		}

		requests := make([]ctrl.Request, len(upgradeGroupList.Items))
		for _, item := range upgradeGroupList.Items {
			if item.Spec.Targets == nil {
				continue
			}
			if item.Spec.ClassName == clusterClass.ObjectMeta.Name {
				requests = append(requests, reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      item.GetName(),
						Namespace: item.GetNamespace(),
					},
				})
			}
		}

		return requests
	}
}
