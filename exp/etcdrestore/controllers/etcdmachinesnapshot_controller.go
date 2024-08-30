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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

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
func (r *EtcdMachineSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	etcdMachineSnapshot := &snapshotrestorev1.EtcdMachineSnapshot{ObjectMeta: metav1.ObjectMeta{
		Name:      req.Name,
		Namespace: req.Namespace,
	}}
	if err := r.Client.Get(ctx, req.NamespacedName, etcdMachineSnapshot); apierrors.IsNotFound(err) {
		// Object not found, return. Created objects are automatically garbage collected.
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, fmt.Sprintf("Unable to get etcdMachineSnapshot resource: %s", req.String()))
		return ctrl.Result{}, err
	}

	// Handle deleted etcdMachineSnapshot
	if !etcdMachineSnapshot.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, etcdMachineSnapshot)
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

func (r *EtcdMachineSnapshotReconciler) reconcileNormal(
	ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.EtcdMachineSnapshot,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Handle different phases of the etcdmachinesnapshot creation process
	switch etcdMachineSnapshot.Status.Phase {
	case "":
		// Initial phase, set to Pending
		etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhasePending
		if err := r.Client.Status().Update(ctx, etcdMachineSnapshot); err != nil {
			log.Error(err, "Failed to update EtcdMachineSnapshot status to Pending")
			return ctrl.Result{}, err
		}
	case snapshotrestorev1.ETCDSnapshotPhasePending:
		// Transition to Running
		etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhaseRunning
		if err := r.Client.Status().Update(ctx, etcdMachineSnapshot); err != nil {
			log.Error(err, "Failed to update EtcdMachineSnapshot status to Running")
			return ctrl.Result{}, err
		}
	case snapshotrestorev1.ETCDSnapshotPhaseRunning:
		// Check the status of the snapshot creation process
		// List ETCDSnapshotFile resources to determine if the snapshot is complete
		if err := checkSnapshotStatus(ctx, r, etcdMachineSnapshot); err != nil {
			// If no matching ready snapshot is found, requeue the request
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}
		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotPhaseFailed:
		// Handle snapshot creation failure
		log.Error(nil, "Snapshot creation failed")
		// Perform retry logic with exponential backoff
		initialDelay := 1 * time.Second
		maxRetries := 5

		err := retryWithExponentialBackoff(ctx, initialDelay, maxRetries, func(ctx context.Context) error {
			return checkSnapshotStatus(ctx, r, etcdMachineSnapshot)
		})
		if err != nil {
			log.Error(err, "Snapshot creation failed after retries")
			return ctrl.Result{}, err
		}
	case snapshotrestorev1.ETCDSnapshotPhaseDone:
		// Snapshot creation is complete, no further action needed
		return ctrl.Result{}, nil
	}

	// Requeue the request if necessary
	if etcdMachineSnapshot.Status.Phase != snapshotrestorev1.ETCDSnapshotPhaseDone {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{}, nil
}

func (r *EtcdMachineSnapshotReconciler) reconcileDelete(
	ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.EtcdMachineSnapshot,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Log the start of the deletion process
	log.Info("Starting deletion of EtcdMachineSnapshot", "name", etcdMachineSnapshot.Name)

	// Perform any necessary cleanup of associated resources here
	// Example: Delete associated snapshot resources
	snapshotList := &snapshotrestorev1.EtcdMachineSnapshotList{}
	if err := r.Client.List(ctx, snapshotList, client.InNamespace(etcdMachineSnapshot.Namespace)); err != nil {
		log.Error(err, "Failed to list associated EtcdMachineSnapshots")
		return ctrl.Result{}, err
	}

	for _, snapshot := range snapshotList.Items {
		if snapshot.Spec.MachineName == etcdMachineSnapshot.Spec.MachineName {
			if err := r.Client.Delete(ctx, &snapshot); err != nil {
				log.Error(err, "Failed to delete associated EtcdMachineSnapshot", "snapshotName", snapshot.Name)
				return ctrl.Result{}, err
			}
			log.Info("Deleted associated EtcdMachineSnapshot", "snapshotName", snapshot.Name)
		}
	}

	// Remove the finalizer to allow deletion of the EtcdMachineSnapshot
	controllerutil.RemoveFinalizer(etcdMachineSnapshot, snapshotrestorev1.ETCDMachineSnapshotFinalizer)
	if err := r.Client.Update(ctx, etcdMachineSnapshot); err != nil {
		log.Error(err, "Failed to remove finalizer from EtcdMachineSnapshot")
		return ctrl.Result{}, err
	}

	// Log the completion of the deletion process
	log.Info("Completed deletion of EtcdMachineSnapshot", "name", etcdMachineSnapshot.Name)

	return ctrl.Result{}, nil
}

// retryWithExponentialBackoff retries an operation with exponential backoff.
func retryWithExponentialBackoff(ctx context.Context, initialDelay time.Duration, maxRetries int, operation func(ctx context.Context) error) error {
	delay := initialDelay
	for i := 0; i < maxRetries; i++ {
		err := operation(ctx)
		if err == nil {
			return nil
		}
		fmt.Printf("Retry %d/%d failed: %v. Retrying in %v...", i+1, maxRetries, err, delay)
		select {
		case <-time.After(delay):
			delay *= 2
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return errors.New("operation failed after maximum retries")
}

// checkSnapshotStatus checks the status of the snapshot creation process.
func checkSnapshotStatus(ctx context.Context, r *EtcdMachineSnapshotReconciler, etcdMachineSnapshot *snapshotrestorev1.EtcdMachineSnapshot) error {
	log := log.FromContext(ctx)
	etcdSnapshotFileList := &unstructured.UnstructuredList{}
	etcdSnapshotFileList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "k3s.cattle.io",
		Kind:    "ETCDSnapshotFile",
		Version: "v1",
	})

	if err := r.Client.List(ctx, etcdSnapshotFileList); err != nil {
		log.Error(err, "Failed to list ETCDSnapshotFile resources")
		return err
	}

	for _, snapshotFile := range etcdSnapshotFileList.Items {
		_, _, snapshotName, readyToUse, err := extractFieldsFromETCDSnapshotFile(snapshotFile)
		if err != nil {
			log.Error(err, "Failed to extract fields from ETCDSnapshotFile")
			continue
		}

		if readyToUse && snapshotName == etcdMachineSnapshot.Name {
			etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhaseDone
			if err := r.Client.Status().Update(ctx, etcdMachineSnapshot); err != nil {
				log.Error(err, "Failed to update EtcdMachineSnapshot status to Done")
				return err
			}
			return nil
		}
	}

	// If no matching ready snapshot is found, return an error to retry
	return errors.New("snapshot not ready")
}

// extractFieldsFromETCDSnapshotFile extracts the location, nodeName, snapshotName, and readyToUse fields from an ETCDSnapshotFile resource.
func extractFieldsFromETCDSnapshotFile(snapshotFile unstructured.Unstructured) (string, string, string, bool, error) {
	metadata, ok := snapshotFile.Object["metadata"].(map[string]interface{})
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get metadata from etcd snapshot file, expected a map[string]interface{}, got %T", snapshotFile.Object["metadata"])
	}

	snapshotName, ok := metadata["name"].(string)
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get snapshotName from etcd snapshot file spec, expected a string, got %T", metadata["name"])
	}

	spec, ok := snapshotFile.Object["spec"].(map[string]interface{})
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get spec from etcd snapshot file expected a map[string]interface{}, got %T", snapshotFile.Object["spec"])
	}

	location, ok := spec["location"].(string)
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get location from etcd snapshot file spec, expected a string, got %T", spec["location"])
	}

	nodeName, ok := spec["nodeName"].(string)
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get nodeName from etcd snapshot file spec, expected a string, got %T", spec["nodeName"])
	}

	status, ok := snapshotFile.Object["status"].(map[string]interface{})
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get status from etcd snapshot file expected a map[string]interface{}, got %T", snapshotFile.Object["status"])
	}

	readyToUse, ok := status["readyToUse"].(bool)
	if !ok {
		return "", "", "", false, fmt.Errorf("failed to get readyToUse from etcd snapshot file status, expected a bool, got %T", status["readyToUse"])
	}

	return location, nodeName, snapshotName, readyToUse, nil
}
