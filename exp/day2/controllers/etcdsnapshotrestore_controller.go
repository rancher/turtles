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

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	controlplanev1 "github.com/rancher/cluster-api-provider-rke2/controlplane/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/collections"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	snapshotrestorev1 "github.com/rancher/turtles/exp/day2/api/v1alpha1"
)

// initMachine is a filter matching on init machine of the ETCD snapshot
func initMachine(etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshotFile) collections.Func {
	return func(machine *clusterv1.Machine) bool {
		return machine.Name == etcdMachineSnapshot.MachineName
	}
}

// ETCDSnapshotRestoreReconciler reconciles a ETCDSnapshotRestore object
type ETCDSnapshotRestoreReconciler struct {
	client.Client
}

func clusterToRestore(c client.Client) handler.MapFunc {
	return func(ctx context.Context, cluster client.Object) []ctrl.Request {
		restoreList := &snapshotrestorev1.ETCDSnapshotRestoreList{}
		if err := c.List(ctx, restoreList); err != nil {
			return nil
		}

		requests := []ctrl.Request{}
		for _, restore := range restoreList.Items {
			if restore.Spec.ClusterName == cluster.GetName() {
				requests = append(requests, ctrl.Request{
					NamespacedName: client.ObjectKey{
						Namespace: cluster.GetNamespace(),
						Name:      restore.Name,
					},
				})
			}
		}

		return requests
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *ETCDSnapshotRestoreReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, options controller.Options) error {
	return ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(&snapshotrestorev1.ETCDSnapshotRestore{}).
		Watches(&clusterv1.Cluster{}, handler.EnqueueRequestsFromMapFunc(clusterToRestore(r.Client))).
		Complete(reconcile.AsReconciler(r.Client, r))
}

// scope holds the different objects that are read and used during the reconcile.
type scope struct {
	// cluster is the Cluster object the Machine belongs to.
	// It is set at the beginning of the reconcile function.
	cluster *clusterv1.Cluster

	// machine is the Machine object. It is set at the beginning
	// of the reconcile function.
	machines collections.Machines

	// etcdMachineSnapshot is the EtcdMachineSnapshot object.
	etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshotFile
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdsnapshotrestores,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdsnapshotrestores/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=etcdsnapshotrestores/finalizers,verbs=update
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters/status,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=secrets;events;configmaps;serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=serviceaccounts/token,verbs=create
//+kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles;rolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="management.cattle.io",resources=*,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=bootstrap.cluster.x-k8s.io,resources=rke2configs;rke2configs/status;rke2configs/finalizers,verbs=get;list;watch;create;update;patch;delete

func (r *ETCDSnapshotRestoreReconciler) Reconcile(ctx context.Context, restore *snapshotrestorev1.ETCDSnapshotRestore) (_ ctrl.Result, reterr error) {
	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(restore, r.Client)
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
		if err := patchHelper.Patch(ctx, restore, patchOpts...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deleted etcdSnapshot
	if !restore.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, restore)
	}

	var errs []error

	result, err := r.reconcileNormal(ctx, restore)
	if err != nil {
		errs = append(errs, fmt.Errorf("error reconciling etcd snapshot restore: %w", err))
	}

	return result, kerrors.NewAggregate(errs)
}

func (r *ETCDSnapshotRestoreReconciler) reconcileNormal(ctx context.Context, etcdSnapshotRestore *snapshotrestorev1.ETCDSnapshotRestore) (_ ctrl.Result, reterr error) {
	log := log.FromContext(ctx)

	scope, err := initScope(ctx, r.Client, etcdSnapshotRestore)
	if err != nil {
		return ctrl.Result{}, err
	}

	running := func(machine *clusterv1.Machine) bool {
		return machine.Status.Phase == string(clusterv1.MachinePhaseRunning)
	}

	if scope.machines.Filter(running).Len() != scope.machines.Len() {
		log.Info("Not all machines are running yet, requeuing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if scope.machines.Filter(initMachine(scope.etcdMachineSnapshot)).Len() != 1 {
		return ctrl.Result{}, fmt.Errorf(
			"init machine %s for snapshot %s is not found",
			scope.etcdMachineSnapshot.MachineName,
			etcdSnapshotRestore.Spec.ETCDMachineSnapshotName)
	}

	patchHelper, err := patch.NewHelper(scope.cluster, r.Client)
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
		if err := patchHelper.Patch(ctx, scope.cluster, patchOpts...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	switch etcdSnapshotRestore.Status.Phase {
	case snapshotrestorev1.ETCDSnapshotRestorePhasePending:
		// Pause CAPI cluster.
		scope.cluster.Spec.Paused = true
		etcdSnapshotRestore.Status.Phase = snapshotrestorev1.ETCDSnapshotRestorePhaseStarted

		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotRestorePhaseStarted:
		return r.preparePlanPermissions(ctx, scope, etcdSnapshotRestore)
	case snapshotrestorev1.ETCDSnapshotRestorePhaseShutdown:
		// Stop RKE2 on all the machines.
		return r.stopRKE2OnAllMachines(ctx, scope, etcdSnapshotRestore)
	case snapshotrestorev1.ETCDSnapshotRestorePhaseRunning:
		// Restore the etcd snapshot on the init machine.
		return r.restoreSnapshotOnInitMachine(ctx, scope, etcdSnapshotRestore)
	case snapshotrestorev1.ETCDSnapshotRestorePhaseAgentRestart:
		// Start RKE2 on all the machines.
		return r.startRKE2OnAllMachines(ctx, scope, etcdSnapshotRestore)
	case snapshotrestorev1.ETCDSnapshotRestoreUnpauseCluster:
		// Unpause the cluster to reconcile machine agent conditions
		scope.cluster.Spec.Paused = false
		etcdSnapshotRestore.Status.Phase = snapshotrestorev1.ETCDSnapshotRestorePhaseJoinAgents

		return ctrl.Result{}, nil
	case snapshotrestorev1.ETCDSnapshotRestorePhaseJoinAgents:
		return r.waitForMachinesToJoin(ctx, scope, etcdSnapshotRestore)
	case snapshotrestorev1.ETCDSnapshotRestorePhaseFinished, snapshotrestorev1.ETCDSnapshotRestorePhaseFailed:
		return r.revokePlanPermissions(ctx, scope, etcdSnapshotRestore)
	}

	return ctrl.Result{}, nil
}

func (r *ETCDSnapshotRestoreReconciler) reconcileDelete(_ context.Context, _ *snapshotrestorev1.ETCDSnapshotRestore) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func initScope(ctx context.Context, c client.Client, etcdSnapshotRestore *snapshotrestorev1.ETCDSnapshotRestore) (*scope, error) {
	// Get the cluster object.
	cluster := &clusterv1.Cluster{}

	if err := c.Get(ctx, client.ObjectKey{Namespace: etcdSnapshotRestore.Namespace, Name: etcdSnapshotRestore.Spec.ClusterName}, cluster); err != nil {
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	machines, err := collections.GetFilteredMachinesForCluster(ctx, c, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to collect machines for cluster: %w", err)
	}

	// Get etcd machine backup object.
	etcdMachineSnapshot := &snapshotrestorev1.ETCDMachineSnapshot{}
	if err := c.Get(ctx, client.ObjectKey{
		Namespace: etcdSnapshotRestore.Namespace,
		Name:      etcdSnapshotRestore.Spec.ClusterName,
	}, etcdMachineSnapshot); err != nil {
		return nil, fmt.Errorf("failed to get etcd machine backup: %w", err)
	}

	var machineSnapshot *snapshotrestorev1.ETCDMachineSnapshotFile

	for _, snapshot := range etcdMachineSnapshot.Status.Snapshots {
		if snapshot.Name != etcdSnapshotRestore.Spec.ETCDMachineSnapshotName {
			continue
		}

		machineSnapshot = &snapshot
	}

	if machineSnapshot == nil {
		return nil, fmt.Errorf("failed to locate snapshot named %s", etcdSnapshotRestore.Spec.ETCDMachineSnapshotName)
	}

	return &scope{
		cluster:             cluster,
		machines:            machines.Filter(collections.ControlPlaneMachines(cluster.Name)),
		etcdMachineSnapshot: machineSnapshot,
	}, nil
}

func (r *ETCDSnapshotRestoreReconciler) preparePlanPermissions(ctx context.Context, scope *scope, etcdSnapshotRestore *snapshotrestorev1.ETCDSnapshotRestore) (ctrl.Result, error) {
	if err := Plan(ctx, r.Client, "restore"+etcdSnapshotRestore.Name, scope.machines.Newest(), scope.machines).Permit(ctx); err != nil {
		return ctrl.Result{}, err
	}

	etcdSnapshotRestore.Status.Phase = snapshotrestorev1.ETCDSnapshotRestorePhaseShutdown

	return ctrl.Result{}, nil
}

func (r *ETCDSnapshotRestoreReconciler) revokePlanPermissions(ctx context.Context, scope *scope, etcdSnapshotRestore *snapshotrestorev1.ETCDSnapshotRestore) (ctrl.Result, error) {
	if err := Plan(ctx, r.Client, "restore"+etcdSnapshotRestore.Name, scope.machines.Newest(), scope.machines).Revoke(ctx); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *ETCDSnapshotRestoreReconciler) stopRKE2OnAllMachines(ctx context.Context, scope *scope, etcdSnapshotRestore *snapshotrestorev1.ETCDSnapshotRestore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	results := []Output{}
	for _, machine := range scope.machines {
		log.Info("Stopping RKE2 on machine", "machine", machine.Name)

		// Get the plan secret for the machine.
		applied, err := Plan(ctx, r.Client, "restore"+etcdSnapshotRestore.Name, machine, scope.machines).Apply(ctx, RKE2KillAll())
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to get plan secret for machine: %w", err)
		}

		results = append(results, applied)
	}

	for _, result := range results {
		if !result.Finished {
			log.Info("Plan is not yet applied, requeuing", "machine", result.Machine.Name)

			// Requeue after 30 seconds if not ready.
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		log.Info(fmt.Sprintf("Decompressed plan output: %s", result.Result), "machine", result.Machine.Name)

	}

	log.Info("All machines are ready to proceed to the next phase, setting phase to running")

	etcdSnapshotRestore.Status.Phase = snapshotrestorev1.ETCDSnapshotRestorePhaseRunning

	return ctrl.Result{}, nil
}

func (r *ETCDSnapshotRestoreReconciler) restoreSnapshotOnInitMachine(ctx context.Context, scope *scope, etcdSnapshotRestore *snapshotrestorev1.ETCDSnapshotRestore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	initMachine := scope.machines.Filter(initMachine(scope.etcdMachineSnapshot)).UnsortedList()[0]

	log.Info("Filling plan secret with etcd restore instructions", "machine", initMachine.Name)

	// Get the plan secret for the machine.
	applied, err := Plan(ctx, r.Client, "restore"+etcdSnapshotRestore.Name, initMachine, scope.machines).Apply(
		ctx,
		RemoveServerURL(),
		ManifestRemoval(),
		ETCDRestore(scope.etcdMachineSnapshot),
	)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get plan secret for machine: %w", err)
	} else if !applied.Finished {
		log.Info("Plan not applied yet", "machine", initMachine.Name)

		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	log.Info(fmt.Sprintf("Decompressed plan output: %s", applied.Result), "machine", initMachine.Name)

	etcdSnapshotRestore.Status.Phase = snapshotrestorev1.ETCDSnapshotRestorePhaseAgentRestart

	return ctrl.Result{}, nil
}

func (r *ETCDSnapshotRestoreReconciler) startRKE2OnAllMachines(ctx context.Context, scope *scope, etcdSnapshotRestore *snapshotrestorev1.ETCDSnapshotRestore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	initMachine := scope.machines.Filter(initMachine(scope.etcdMachineSnapshot)).UnsortedList()[0]

	// TODO: other registration methods
	initMachineIP := getInternalMachineIP(initMachine)
	if initMachineIP == "" {
		log.Info("failed to get internal machine IP, field is empty")

		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// Start from the init machine.
	sortedMachines := append(
		[]*clusterv1.Machine{initMachine},
		scope.machines.UnsortedList()...,
	)

	for _, machine := range sortedMachines {
		instructions := Instructions{}
		if machine.Name == initMachine.Name {
			log.Info("Starting RKE2 on init machine", "machine", initMachine.Name)

			instructions = append(instructions, StartRKE2())
		} else {
			log.Info("Starting RKE2 on machine", "machine", machine.Name)

			instructions = append(instructions, RemoveServerURL(),
				SetServerURL(initMachineIP),
				RemoveETCDData(),
				ManifestRemoval(),
				StartRKE2())
		}

		applied, err := Plan(ctx, r.Client, "restore"+etcdSnapshotRestore.Name, machine, scope.machines).Apply(ctx, instructions...)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to patch plan secret: %w", err)
		} else if !applied.Finished {
			log.Info("Plan is not yet applied, requeuing", "machine", machine.Name)

			// Requeue after 30 seconds if not ready.
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		log.Info(fmt.Sprintf("Decompressed plan output: %s", applied.Result), "machine", machine.Name)
	}

	log.Info("All machines are ready and have RKE2 started")

	etcdSnapshotRestore.Status.Phase = snapshotrestorev1.ETCDSnapshotRestoreUnpauseCluster

	return ctrl.Result{}, nil
}

func (r *ETCDSnapshotRestoreReconciler) waitForMachinesToJoin(ctx context.Context, scope *scope, etcdSnapshotRestore *snapshotrestorev1.ETCDSnapshotRestore) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	for _, machine := range scope.machines.ConditionGetters() {
		if !conditions.IsTrue(machine, controlplanev1.MachineAgentHealthyCondition) {
			log.Info("Machine does not have AgentHealthyCondition yet", "machine", klog.KObj(machine))

			return ctrl.Result{RequeueAfter: snapshotPhaseRequeueDuration}, nil
		}
	}

	log.Info("All machines joined the RKE2 cluster")

	etcdSnapshotRestore.Status.Phase = snapshotrestorev1.ETCDSnapshotRestorePhaseFinished

	return ctrl.Result{}, nil
}

// getInternalMachineIP collects internal machine IP for the init machine
func getInternalMachineIP(machine *clusterv1.Machine) string {
	for _, address := range machine.Status.Addresses {
		if address.Type == clusterv1.MachineInternalIP {
			return address.Address
		}
	}
	return ""
}
