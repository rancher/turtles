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
	snapshotrestorev1 "github.com/rancher/turtles/exp/etcdrestore/api/v1alpha1"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const snapshotPhaseRequeueDuration = 30 * time.Second

// ETCDMachineSnapshotReconciler reconciles an EtcdMachineSnapshot object.
type ETCDMachineSnapshotReconciler struct {
	client.Client
	WatchFilterValue string

	controller controller.Controller
	Tracker    *remote.ClusterCacheTracker
	Scheme     *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ETCDMachineSnapshotReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, _ controller.Options) error {
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&snapshotrestorev1.ETCDMachineSnapshot{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			log := log.FromContext(ctx)

			if turtlesannotations.HasAnnotation(object, turtlesannotations.EtcdAutomaticSnapshot) {
				log.V(5).Info("Skipping snapshot creation for non-manual EtcdMachineSnapshot")

				return false
			}

			return true
		})).
		Build(reconcile.AsReconciler(r.Client, r))
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
func (r *ETCDMachineSnapshotReconciler) Reconcile(ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshot) (_ ctrl.Result, reterr error) {
	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(etcdMachineSnapshot, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		// Always attempt to patch the object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully.
		patchOpts := []patch.Option{}
		if reterr == nil {
			patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
		}
		if err := patchHelper.Patch(ctx, etcdMachineSnapshot, patchOpts...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deleted etcdMachineSnapshot
	if !etcdMachineSnapshot.ObjectMeta.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.reconcileDelete(ctx, etcdMachineSnapshot)
	}

	// Ensure the finalizer is present
	controllerutil.AddFinalizer(etcdMachineSnapshot, snapshotrestorev1.ETCDMachineSnapshotFinalizer)

	return r.reconcileNormal(ctx, etcdMachineSnapshot)
}

func (r *ETCDMachineSnapshotReconciler) reconcileNormal(
	ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshot,
) (ctrl.Result, error) {
	// Handle different phases of the etcdmachinesnapshot creation process
	switch etcdMachineSnapshot.Status.Phase {
	case "":
		// Initial phase, set to Pending
		etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhasePending
		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotPhasePending:
		// Transition to Running
		if finished, err := r.createMachineSnapshot(ctx, etcdMachineSnapshot); err != nil {
			return ctrl.Result{}, err
		} else if !finished {
			return ctrl.Result{RequeueAfter: snapshotPhaseRequeueDuration}, nil
		}

		etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhaseRunning

		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotPhaseRunning:
		// Check the status of the snapshot creation process
		// Fetch ETCDSnapshotFile resource to determine if the snapshot is complete
		if finished, err := r.checkSnapshotStatus(ctx, etcdMachineSnapshot); err != nil {
			return ctrl.Result{}, err
		} else if !finished {
			return ctrl.Result{RequeueAfter: snapshotPhaseRequeueDuration}, nil
		}

		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotPhaseFailed, snapshotrestorev1.ETCDSnapshotPhaseDone:
		// If the snapshot creation failed or completed, do nothing
		return ctrl.Result{}, nil
	default:
		return ctrl.Result{}, nil
	}
}

func (r *ETCDMachineSnapshotReconciler) reconcileDelete(
	ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshot,
) error {
	log := log.FromContext(ctx)

	// Log the start of the deletion process
	log.Info("Starting deletion of EtcdMachineSnapshot", "name", etcdMachineSnapshot.Name)

	// Remove the finalizer so the EtcdMachineSnapshot can be garbage collected by Kubernetes.
	controllerutil.RemoveFinalizer(etcdMachineSnapshot, snapshotrestorev1.ETCDMachineSnapshotFinalizer)

	// Log the completion of the deletion process
	log.Info("Completed deletion of EtcdMachineSnapshot", "name", etcdMachineSnapshot.Name)

	return nil
}

// createMachineSnapshot generates ETCDSnapshotFile on the child cluster.
func (r *ETCDMachineSnapshotReconciler) createMachineSnapshot(ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshot) (bool, error) {
	log := log.FromContext(ctx)

	machineKey := client.ObjectKey{
		Name:      etcdMachineSnapshot.Spec.MachineName,
		Namespace: etcdMachineSnapshot.Namespace,
	}

	machine := &clusterv1.Machine{}
	if err := r.Client.Get(ctx, machineKey, machine); err != nil {
		log.Error(err, "Failed to find machine", "machine", machineKey.String())

		return false, err
	} else if machine.Status.NodeRef == nil {
		log.Info("Machine has no node ref yet", "machine", machineKey.String())

		return false, nil
	}

	clusterKey := client.ObjectKey{
		Name:      etcdMachineSnapshot.Spec.ClusterName,
		Namespace: etcdMachineSnapshot.Namespace,
	}

	remoteClient, err := r.Tracker.GetClient(ctx, clusterKey)
	if err != nil {
		log.Error(err, "Failed to open remote client to cluster", "cluster", clusterKey.String())

		return false, err
	}

	etcdSnapshotFile := &k3sv1.ETCDSnapshotFile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      etcdMachineSnapshot.Name,
			Namespace: "default",
		},
		Spec: k3sv1.ETCDSnapshotSpec{
			SnapshotName: etcdMachineSnapshot.Name,
			NodeName:     machine.Status.NodeRef.Name,
			Location:     etcdMachineSnapshot.Spec.Location,
		},
	}

	if err := remoteClient.Create(ctx, etcdSnapshotFile); err != nil {
		log.Error(err, "Failed to create ETCDSnapshotFile", "snapshot", client.ObjectKeyFromObject(etcdSnapshotFile))

		return false, err
	}

	return true, nil
}

// checkSnapshotStatus checks the status of the snapshot creation process.
func (r *ETCDMachineSnapshotReconciler) checkSnapshotStatus(ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshot) (bool, error) {
	log := log.FromContext(ctx)

	etcdSnapshotFile := &k3sv1.ETCDSnapshotFile{}

	clusterKey := client.ObjectKey{
		Name:      etcdMachineSnapshot.Spec.ClusterName,
		Namespace: etcdMachineSnapshot.Namespace,
	}

	remoteClient, err := r.Tracker.GetClient(ctx, clusterKey)
	if err != nil {
		log.Error(err, "Failed to open remote client to cluster", "cluster", clusterKey.String())

		return false, err
	}

	snapshotKey := client.ObjectKey{
		Name:      etcdMachineSnapshot.Name,
		Namespace: "default",
	}
	if err := remoteClient.Get(ctx, snapshotKey, etcdSnapshotFile); err != nil {
		log.Error(err, "Failed to get ETCDSnapshotFile", "snapshot", snapshotKey.String())

		return false, err
	}

	// Check if the snapshot is ready to use and matches the machine snapshot name
	if *etcdSnapshotFile.Status.ReadyToUse {
		// Update the status to Done
		etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhaseDone
		return true, nil
	}

	if etcdSnapshotFile.Status.Error != nil {
		etcdMachineSnapshot.Status.Error = etcdSnapshotFile.Status.Error.Message
		etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhaseFailed

		return true, nil
	}

	// If no matching ready snapshot is found, return an error to retry
	return false, nil
}
