/*
Copyright © 2023 - 2025 SUSE LLC

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
	"strings"
	"time"

	k3sv1 "github.com/rancher/turtles/api/rancher/k3s/v1"
	snapshotrestorev1 "github.com/rancher/turtles/exp/day2/api/v1alpha1"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/clustercache"
	"sigs.k8s.io/cluster-api/util/collections"
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
const snapshotRequestRequeueDuration = 5 * time.Second

// ETCDMachineSnapshotReconciler reconciles an EtcdMachineSnapshot object.
type ETCDMachineSnapshotReconciler struct {
	client.Client
	WatchFilterValue string

	controller controller.Controller
	Tracker    clustercache.ClusterCache
	Scheme     *runtime.Scheme
}

// snapshotScope holds the different objects that are read and used for the snapshot execution.
type snapshotScope struct {
	// cluster is the Cluster object the Machine belongs to.
	// It is set at the beginning of the reconcile function.
	cluster *clusterv1.Cluster

	// machine is the Machine object. It is set at the beginning
	// of the reconcile function.
	machines collections.Machines

	// machine for the snapshot execution
	machine *clusterv1.Machine

	// snapshot is the snapshot object which is used for reconcile
	snapshot *snapshotrestorev1.ETCDMachineSnapshot
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

func (r *ETCDMachineSnapshotReconciler) newScope(ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshot) (*snapshotScope, error) {
	// Get the cluster object.
	cluster := &clusterv1.Cluster{}

	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: etcdMachineSnapshot.Namespace, Name: etcdMachineSnapshot.Spec.ClusterName}, cluster); err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	machines, err := collections.GetFilteredMachinesForCluster(ctx, r.Client, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to collect machines for cluster: %w", err)
	}

	controlPlaneMachines := machines.Filter(collections.ControlPlaneMachines(cluster.Name))
	targetMachineCandidates := controlPlaneMachines.Filter(func(machine *clusterv1.Machine) bool {
		return machine.Name == etcdMachineSnapshot.Spec.MachineName
	}).UnsortedList()

	if len(targetMachineCandidates) < 1 {
		return nil, fmt.Errorf(
			"failed to found machine %s for cluster %s",
			etcdMachineSnapshot.Spec.MachineName,
			client.ObjectKeyFromObject(cluster).String())
	}

	return &snapshotScope{
		cluster:  cluster,
		machines: controlPlaneMachines,
		machine:  targetMachineCandidates[0],
		snapshot: etcdMachineSnapshot,
	}, nil
}

func (r *ETCDMachineSnapshotReconciler) reconcileNormal(
	ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshot,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	scope, err := r.newScope(ctx, etcdMachineSnapshot)
	if err != nil {
		log.Error(err, "Unable to intialize scope")
		return ctrl.Result{}, err
	}

	if scope.machine.Status.NodeRef == nil {
		log.Info("Machine has no node ref yet", "machine", client.ObjectKeyFromObject(scope.machine).String())

		return ctrl.Result{RequeueAfter: snapshotPhaseRequeueDuration}, nil
	}

	// Handle different phases of the etcdmachinesnapshot creation process
	switch etcdMachineSnapshot.Status.Phase {
	case "":
		if err := r.permit(ctx, scope); err != nil {
			return ctrl.Result{}, err
		}

		// Initial phase, set to Pending
		etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhasePending

		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotPhasePending, snapshotrestorev1.ETCDSnapshotPhasePlanning:
		// Transition to Running
		if finished, err := r.createMachineSnapshot(ctx, scope); err != nil {
			return ctrl.Result{}, err
		} else if !finished {
			etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhasePlanning

			return ctrl.Result{RequeueAfter: snapshotRequestRequeueDuration}, nil
		}

		etcdMachineSnapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhaseRunning

		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotPhaseRunning:
		// Check the status of the snapshot creation process
		// Fetch ETCDSnapshotFile resource to determine if the snapshot is complete
		if finished, err := r.checkSnapshotStatus(ctx, scope); err != nil {
			return ctrl.Result{}, err
		} else if !finished {
			return ctrl.Result{RequeueAfter: snapshotPhaseRequeueDuration}, nil
		}

		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotPhaseFailed, snapshotrestorev1.ETCDSnapshotPhaseDone:
		// If the snapshot creation failed or completed, do nothing
		return ctrl.Result{}, r.revoke(ctx, scope)
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

func (r *ETCDMachineSnapshotReconciler) permit(ctx context.Context, scope *snapshotScope) error {
	return Plan(ctx, r.Client, "snapshot"+scope.snapshot.Name, scope.machine, scope.machines).Permit(ctx)
}

func (r *ETCDMachineSnapshotReconciler) revoke(ctx context.Context, scope *snapshotScope) error {
	return Plan(ctx, r.Client, "snapshot"+scope.snapshot.Name, scope.machine, scope.machines).Revoke(ctx)
}

// snapshot creates an RKE2 snapshot
func snapshot(snapshot *snapshotrestorev1.ETCDMachineSnapshot) Instruction {
	ins := Instruction{
		Name:    "snapshot",
		Command: "/bin/sh",
		Args: []string{
			"-c",
		},
		SaveOutput: true,
	}

	command := []string{
		"rke2 etcd-snapshot save",
		"--name", snapshot.Name,
	}

	if snapshot.Spec.Location != "" {
		command = append(command, "--dir", snapshot.Spec.Location)
	}

	ins.Args = append(ins.Args, strings.Join(command, " "))

	return ins
}

// createMachineSnapshot generates ETCDSnapshotFile on the child cluster.
func (r *ETCDMachineSnapshotReconciler) createMachineSnapshot(ctx context.Context, scope *snapshotScope) (bool, error) {
	log := log.FromContext(ctx)

	clusterKey := client.ObjectKeyFromObject(scope.cluster)

	plan := Plan(ctx, r.Client, "snapshot"+scope.snapshot.Name, scope.machine, scope.machines)

	if result, err := plan.Apply(ctx, snapshot(scope.snapshot)); err != nil {
		log.Error(err, "Failed to perform snapshot on a cluster",
			"cluster", clusterKey.String(),
			"machine", client.ObjectKeyFromObject(scope.machine),
			"snapshot", client.ObjectKeyFromObject(scope.snapshot).String())

		return false, err
	} else if !result.Finished {
		log.Info("Plan is not yet applied, requeuing", "machine", result.Machine.Name)

		return false, nil
	} else {
		log.Info(fmt.Sprintf("Decompressed plan output: %s", result.Result), "machine", result.Machine.Name)
	}

	return true, nil
}

// checkSnapshotStatus checks the status of the snapshot creation process.
func (r *ETCDMachineSnapshotReconciler) checkSnapshotStatus(ctx context.Context, scope *snapshotScope) (bool, error) {
	log := log.FromContext(ctx)

	clusterKey := client.ObjectKeyFromObject(scope.cluster)

	remoteClient, err := r.Tracker.GetClient(ctx, clusterKey)
	if err != nil {
		log.Error(err, "Failed to open remote client to cluster", "cluster", clusterKey.String())

		return false, err
	}

	etcdSnapshotFiles := &k3sv1.ETCDSnapshotFileList{}
	if err := remoteClient.List(ctx, etcdSnapshotFiles); err != nil {
		log.Error(err, "Failed to list ETCDSnapshotFiles", "snapshot", scope.snapshot.Name)

		return false, err
	}

	var etcdSnapshotFile *k3sv1.ETCDSnapshotFile

	for _, snapshot := range etcdSnapshotFiles.Items {
		snapshotName := fmt.Sprintf("%s-%s", scope.snapshot.Name, scope.snapshot.Spec.MachineName)
		if strings.Contains(snapshot.Name, snapshotName) {
			etcdSnapshotFile = &snapshot
			break
		}
	}

	if etcdSnapshotFile == nil {
		log.Info("ETCDSnapshotFile is not found yet", "snapshot", scope.snapshot.Name)

		return false, nil
	}

	scope.snapshot.Status.SnapshotFileName = &etcdSnapshotFile.Name

	// Check if the snapshot is ready to use and matches the machine snapshot name
	if etcdSnapshotFile.Status.ReadyToUse != nil && *etcdSnapshotFile.Status.ReadyToUse {
		// Update the status to Done
		scope.snapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhaseDone
		return true, nil
	}

	// Otherwise fail with reason
	if etcdSnapshotFile.Status.Error != nil {
		scope.snapshot.Status.Error = etcdSnapshotFile.Status.Error.Message
		scope.snapshot.Status.Phase = snapshotrestorev1.ETCDSnapshotPhaseFailed

		return true, nil
	}

	// If no matching ready snapshot is found, return an error to retry
	return false, nil
}
