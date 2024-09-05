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
	"errors"
	"fmt"
	"time"

	k3sv1 "github.com/rancher/turtles/api/rancher/k3s/v1"
	snapshotrestorev1 "github.com/rancher/turtles/exp/etcdrestore/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const snapshotPhaseRequeueDuration = 1 * time.Minute

// ETCDMachineSnapshotReconciler reconciles an EtcdMachineSnapshot object.
type ETCDMachineSnapshotReconciler struct {
	client.Client
	WatchFilterValue string

	controller controller.Controller
	Tracker    *remote.ClusterCacheTracker
	Scheme     *runtime.Scheme
}

// SetupWithManager sets up the controller with the Manager.
func (r *ETCDMachineSnapshotReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, _ controller.Options) error {
	// TODO: Setup predicates for the controller.
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&snapshotrestorev1.ETCDMachineSnapshot{}).
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
func (r *ETCDMachineSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := log.FromContext(ctx)

	etcdMachineSnapshot := &snapshotrestorev1.ETCDMachineSnapshot{}
	if err := r.Client.Get(ctx, req.NamespacedName, etcdMachineSnapshot); apierrors.IsNotFound(err) {
		// Object not found, return. Created objects are automatically garbage collected.
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, fmt.Sprintf("Unable to get etcdMachineSnapshot resource: %s", req.String()))
		return ctrl.Result{}, err
	}

	if !etcdMachineSnapshot.Spec.Manual {
		log.V(5).Info("Skipping snapshot creation for non-manual EtcdMachineSnapshot")
		return ctrl.Result{}, nil
	}

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
	if !controllerutil.ContainsFinalizer(etcdMachineSnapshot, snapshotrestorev1.ETCDMachineSnapshotFinalizer) {
		controllerutil.AddFinalizer(etcdMachineSnapshot, snapshotrestorev1.ETCDMachineSnapshotFinalizer)
		if err := r.Client.Update(ctx, etcdMachineSnapshot); err != nil {
			log.Error(err, "Failed to add finalizer to EtcdMachineSnapshot")
			return ctrl.Result{}, err
		}
	}

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
		etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhaseRunning
		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotPhaseRunning:
		// Check the status of the snapshot creation process
		// List ETCDSnapshotFile resources to determine if the snapshot is complete
		if err := checkSnapshotStatus(ctx, r, etcdMachineSnapshot); err != nil {
			// If no matching ready snapshot is found, requeue the request
			return ctrl.Result{RequeueAfter: snapshotPhaseRequeueDuration}, nil
		}
		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotPhaseFailed:
		// If the snapshot creation failed, requeue the request
		return ctrl.Result{RequeueAfter: snapshotPhaseRequeueDuration}, nil
	case snapshotrestorev1.ETCDSnapshotPhaseDone:
		// Snapshot creation is complete, no further action needed
		return ctrl.Result{}, nil
	}

	// Requeue the request if necessary
	if etcdMachineSnapshot.Status.Phase != snapshotrestorev1.ETCDSnapshotPhaseDone {
		return ctrl.Result{RequeueAfter: snapshotPhaseRequeueDuration}, nil
	}

	return ctrl.Result{}, nil
}

func (r *ETCDMachineSnapshotReconciler) reconcileDelete(
	ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshot,
) error {
	log := log.FromContext(ctx)

	// Log the start of the deletion process
	log.Info("Starting deletion of EtcdMachineSnapshot", "name", etcdMachineSnapshot.Name)

	// Perform any necessary cleanup of associated resources here
	// Example: Delete associated snapshot resources
	snapshotList := &snapshotrestorev1.ETCDMachineSnapshotList{}
	if err := r.Client.List(ctx, snapshotList, client.InNamespace(etcdMachineSnapshot.Namespace)); err != nil {
		log.Error(err, "Failed to list associated EtcdMachineSnapshots")
		return err
	}

	for _, snapshot := range snapshotList.Items {
		if snapshot.Spec.MachineName == etcdMachineSnapshot.Spec.MachineName {
			if err := r.Client.Delete(ctx, &snapshot); err != nil {
				log.Error(err, "Failed to delete associated EtcdMachineSnapshot", "snapshotName", snapshot.Name)
				return err
			}
			log.Info("Deleted associated EtcdMachineSnapshot", "snapshotName", snapshot.Name)
		}
	}

	// Remove the finalizer so the EtcdMachineSnapshot can be garbage collected by Kubernetes.
	controllerutil.RemoveFinalizer(etcdMachineSnapshot, snapshotrestorev1.ETCDMachineSnapshotFinalizer)

	// Log the completion of the deletion process
	log.Info("Completed deletion of EtcdMachineSnapshot", "name", etcdMachineSnapshot.Name)

	return nil
}

// checkSnapshotStatus checks the status of the snapshot creation process.
func checkSnapshotStatus(ctx context.Context, r *ETCDMachineSnapshotReconciler, etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshot) error {
	log := log.FromContext(ctx)

	etcdSnapshotFileList := &k3sv1.ETCDSnapshotFileList{}

	if err := r.Client.List(ctx, etcdSnapshotFileList); err != nil {
		log.Error(err, "Failed to list ETCDSnapshotFile resources")
		return err
	}

	// Iterate through the list of ETCDSnapshotFile resources
	for _, snapshotFile := range etcdSnapshotFileList.Items {
		// Validate the snapshotFile fields
		if err := validateETCDSnapshotFile(snapshotFile); err != nil {
			// Log the error and continue to the next snapshotFile
			log.Error(err, "Failed to validate ETCDSnapshotFile")
			continue
		}

		// Extract fields directly after validation
		snapshotName := snapshotFile.Spec.SnapshotName
		readyToUse := *snapshotFile.Status.ReadyToUse

		// Check if the snapshot is ready to use and matches the machine snapshot name
		if readyToUse && snapshotName == etcdMachineSnapshot.Name {
			// Update the status to Done
			etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhaseDone
			return nil
		}
	}

	// If no matching ready snapshot is found, return an error to retry
	return errors.New("snapshot not ready")
}

// validateETCDSnapshotFile validates the fields of an ETCDSnapshotFile resource.
func validateETCDSnapshotFile(snapshotFile k3sv1.ETCDSnapshotFile) error {
	if snapshotFile.Spec.SnapshotName == "" {
		return fmt.Errorf("snapshotName is empty for etcdsnapshotfile %s", snapshotFile.Name)
	}

	if snapshotFile.Spec.Location == "" {
		return fmt.Errorf("location is empty for etcdsnapshotfile %s", snapshotFile.Name)
	}

	if snapshotFile.Spec.NodeName == "" {
		return fmt.Errorf("node name is empty for etcdsnapshotfile %s", snapshotFile.Name)
	}

	if snapshotFile.Status.ReadyToUse == nil {
		return fmt.Errorf("readyToUse field is nil for etcdsnapshotfile %s", snapshotFile.Name)
	}

	return nil
}
