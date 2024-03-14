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
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"

	managementv3 "github.com/rancher/turtles/internal/rancher/management/v3"
	"github.com/rancher/turtles/util"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
	turtlespredicates "github.com/rancher/turtles/util/predicates"
)

// CAPIImportManagementV3Reconciler represents a reconciler for importing CAPI clusters in Rancher.
type CAPIImportManagementV3Reconciler struct {
	Client             client.Client
	RancherClient      client.Client
	recorder           record.EventRecorder
	WatchFilterValue   string
	Scheme             *runtime.Scheme
	InsecureSkipVerify bool

	controller         controller.Controller
	externalTracker    external.ObjectTracker
	remoteClientGetter remote.ClusterClientGetter
}

// SetupWithManager sets up reconciler with manager.
func (r *CAPIImportManagementV3Reconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	if r.remoteClientGetter == nil {
		r.remoteClientGetter = remote.NewClusterClient
	}

	capiPredicates := predicates.All(log,
		predicates.ResourceHasFilterLabel(log, r.WatchFilterValue),
		turtlespredicates.ClusterWithoutImportedAnnotation(log),
		turtlespredicates.ClusterWithReadyControlPlane(log),
		turtlespredicates.ClusterOrNamespaceWithImportLabel(ctx, log, r.Client, importLabelName),
	)

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		WithOptions(options).
		WithEventFilter(capiPredicates).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating new controller: %w", err)
	}

	// Watch Rancher managementv3 clusters
	if err := c.Watch(
		source.Kind(mgr.GetCache(), &managementv3.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(r.rancherClusterToCapiCluster(ctx, capiPredicates)),
	); err != nil {
		return fmt.Errorf("adding watch for Rancher cluster: %w", err)
	}

	ns := &corev1.Namespace{}
	if err = c.Watch(
		source.Kind(mgr.GetCache(), ns),
		handler.EnqueueRequestsFromMapFunc(namespaceToCapiClusters(ctx, capiPredicates, r.Client)),
	); err != nil {
		return fmt.Errorf("adding watch for namespaces: %w", err)
	}

	r.recorder = mgr.GetEventRecorderFor("rancher-turtles")
	r.controller = c
	r.externalTracker = external.ObjectTracker{
		Controller: c,
	}

	return nil
}

// +kubebuilder:rbac:groups="",resources=secrets;events;configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=*,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=management.cattle.io,resources=clusters;clusters/status,verbs=get;list;watch;create;update;delete;deletecollection;patch
// +kubebuilder:rbac:groups=management.cattle.io,resources=clusters;clusterregistrationtokens;clusterregistrationtokens/status,verbs=get;list;watch

// Reconcile reconciles a CAPI cluster, creating a Rancher cluster if needed and applying the import manifests.
func (r *CAPIImportManagementV3Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPI cluster")

	capiCluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, capiCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	if capiCluster.ObjectMeta.DeletionTimestamp.IsZero() && !turtlesannotations.HasClusterImportAnnotation(capiCluster) &&
		!controllerutil.ContainsFinalizer(capiCluster, managementv3.CapiClusterFinalizer) {
		log.Info("capi cluster is imported, adding finalizer")
		controllerutil.AddFinalizer(capiCluster, managementv3.CapiClusterFinalizer)

		if err := r.Client.Update(ctx, capiCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("error adding finalizer: %w", err)
		}
	}

	log = log.WithValues("cluster", capiCluster.Name)

	// Wait for controlplane to be ready. This should never be false as the predicates
	// do the filtering.
	if !capiCluster.Status.ControlPlaneReady && !conditions.IsTrue(capiCluster, clusterv1.ControlPlaneReadyCondition) {
		log.Info("clusters control plane is not ready, requeue")
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	// Collect errors as an aggregate to return together after all patches have been performed.
	var errs []error

	result, err := r.reconcile(ctx, capiCluster)
	if err != nil {
		errs = append(errs, fmt.Errorf("error reconciling cluster: %w", err))
	}

	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		capiClusterCopy := capiCluster.DeepCopy()

		patchBase := client.MergeFromWithOptions(capiCluster, client.MergeFromWithOptimisticLock{})

		if err := r.Client.Patch(ctx, capiClusterCopy, patchBase); err != nil {
			errs = append(errs, fmt.Errorf("failed to patch cluster: %w", err))
		}
		return nil
	}); err != nil {
		return ctrl.Result{}, err
	}

	if len(errs) > 0 {
		return ctrl.Result{}, errorutils.NewAggregate(errs)
	}

	return result, nil
}

func (r *CAPIImportManagementV3Reconciler) reconcile(ctx context.Context, capiCluster *clusterv1.Cluster) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	rancherCluster := &managementv3.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				capiClusterOwner:          capiCluster.Name,
				capiClusterOwnerNamespace: capiCluster.Namespace,
			},
		},
	}

	rancherClusterList := &managementv3.ClusterList{}
	selectors := []client.ListOption{
		client.MatchingLabels{
			capiClusterOwner:          capiCluster.Name,
			capiClusterOwnerNamespace: capiCluster.Namespace,
			ownedLabelName:            "",
		},
	}
	err := r.RancherClient.List(ctx, rancherClusterList, selectors...)

	if client.IgnoreNotFound(err) != nil {
		log.Error(err, fmt.Sprintf("Unable to fetch rancher cluster %s", client.ObjectKeyFromObject(rancherCluster)))
		return ctrl.Result{Requeue: true}, err
	}

	if len(rancherClusterList.Items) != 0 {
		if len(rancherClusterList.Items) > 1 {
			log.Info("More than one rancher cluster found. Will default to using the first one.")
		}

		rancherCluster = &rancherClusterList.Items[0]
	}

	if !capiCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		if err := r.deleteDependentRancherCluster(ctx, capiCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("error deleting associated managementv3.Cluster resources: %w", err)
		}
	}

	if !rancherCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, capiCluster)
	}

	return r.reconcileNormal(ctx, capiCluster, rancherCluster)
}

func (r *CAPIImportManagementV3Reconciler) reconcileNormal(ctx context.Context, capiCluster *clusterv1.Cluster,
	rancherCluster *managementv3.Cluster,
) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	err := r.RancherClient.Get(ctx, client.ObjectKeyFromObject(rancherCluster), rancherCluster)
	if apierrors.IsNotFound(err) {
		shouldImport, err := util.ShouldAutoImport(ctx, log, r.Client, capiCluster, importLabelName)
		if err != nil {
			return ctrl.Result{}, err
		}

		if !shouldImport {
			log.Info("not auto importing cluster as namespace or cluster isn't marked auto import")
			return ctrl.Result{}, nil
		}

		if err := r.RancherClient.Create(ctx, &managementv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    capiCluster.Namespace,
				GenerateName: "c-",
				Labels: map[string]string{
					capiClusterOwner:          capiCluster.Name,
					capiClusterOwnerNamespace: capiCluster.Namespace,
					ownedLabelName:            "",
				},
			},
			Spec: managementv3.ClusterSpec{
				DisplayName: capiCluster.Name,
				Description: "CAPI cluster imported to Rancher",
			},
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating rancher cluster: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to fetch rancher cluster %s", client.ObjectKeyFromObject(rancherCluster)))

		return ctrl.Result{}, err
	}

	if conditions.IsTrue(rancherCluster, managementv3.ClusterConditionReady) {
		log.Info("cluster is ready, no action needed")
		return ctrl.Result{}, nil
	}

	// We have to ensure the agent deployment has correct nodeAffinity settings at all times
	remoteClient, err := r.remoteClientGetter(ctx, capiCluster.Name, r.Client, client.ObjectKeyFromObject(capiCluster))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting remote cluster client: %w", err)
	}

	if conditions.IsTrue(rancherCluster, managementv3.ClusterConditionAgentDeployed) {
		log.Info("updating agent node affinity settings")

		agent := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      "cattle-cluster-agent",
			Namespace: "cattle-system",
		}}

		if err := remoteClient.Get(ctx, client.ObjectKeyFromObject(agent), agent); err != nil {
			log.Error(err, "unable to get existing agent deployment")
			return ctrl.Result{}, err
		}

		setDeploymentAffinity(agent)
		agent.SetManagedFields(nil)
		agent.TypeMeta = metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       deploymentKind,
		}

		if err := remoteClient.Patch(ctx, agent, client.Apply, []client.PatchOption{
			client.ForceOwnership,
			client.FieldOwner(fieldOwner),
		}...); err != nil {
			log.Error(err, "unable to update existing agent deployment")
			return ctrl.Result{}, err
		}

		// During the provisioning after registration the initial deployment gets
		// updated by the rancher. We must not miss it.
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	// get the registration manifest
	manifest, err := getClusterRegistrationManifest(ctx, rancherCluster.Name, rancherCluster.Name, r.RancherClient, r.InsecureSkipVerify)
	if err != nil {
		return ctrl.Result{}, err
	}

	if manifest == "" {
		log.Info("Import manifest URL not set yet, requeue")
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("Creating import manifest")

	if err := createImportManifest(ctx, remoteClient, strings.NewReader(manifest)); err != nil {
		return ctrl.Result{}, fmt.Errorf("creating import manifest: %w", err)
	}

	log.Info("Successfully applied import manifest")

	return ctrl.Result{}, nil
}

func (r *CAPIImportManagementV3Reconciler) rancherClusterToCapiCluster(ctx context.Context, clusterPredicate predicate.Funcs) handler.MapFunc {
	log := log.FromContext(ctx)

	return func(_ context.Context, cluster client.Object) []ctrl.Request {
		labels := cluster.GetLabels()
		if _, ok := labels[capiClusterOwner]; !ok {
			log.Error(fmt.Errorf("missing label %s", capiClusterOwner), "getting rancher cluster labels")
			return nil
		}

		if _, ok := labels[capiClusterOwnerNamespace]; !ok {
			log.Error(fmt.Errorf("missing label %s", capiClusterOwnerNamespace), "getting rancher cluster labels")
			return nil
		}

		capiCluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name:      labels[capiClusterOwner],
			Namespace: labels[capiClusterOwnerNamespace],
		}}

		if err := r.Client.Get(ctx, client.ObjectKeyFromObject(capiCluster), capiCluster); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "getting capi cluster")
			}

			return nil
		}

		if !clusterPredicate.Generic(event.GenericEvent{Object: capiCluster}) {
			return nil
		}

		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: capiCluster.Namespace, Name: capiCluster.Name}}}
	}
}

func (r *CAPIImportManagementV3Reconciler) reconcileDelete(ctx context.Context, capiCluster *clusterv1.Cluster) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling rancher cluster deletion")

	// If the Rancher Cluster was already imported, then annotate the CAPI cluster so that we don't auto-import again.
	log.Info(fmt.Sprintf("Rancher cluster is being removed, annotating CAPI cluster %s with %s",
		capiCluster.Name,
		turtlesannotations.ClusterImportedAnnotation))

	annotations := capiCluster.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[turtlesannotations.ClusterImportedAnnotation] = "true"
	capiCluster.SetAnnotations(annotations)

	if controllerutil.ContainsFinalizer(capiCluster, managementv3.CapiClusterFinalizer) {
		controllerutil.RemoveFinalizer(capiCluster, managementv3.CapiClusterFinalizer)

		if err := r.Client.Update(ctx, capiCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("error removing finalizer: %w", err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *CAPIImportManagementV3Reconciler) deleteDependentRancherCluster(ctx context.Context, capiCluster *clusterv1.Cluster) error {
	log := log.FromContext(ctx)
	log.Info("capi cluster is being deleted, deleting dependent rancher cluster")

	selectors := []client.DeleteAllOfOption{
		client.MatchingLabels{
			capiClusterOwner:          capiCluster.Name,
			capiClusterOwnerNamespace: capiCluster.Namespace,
			ownedLabelName:            "",
		},
	}

	return r.RancherClient.DeleteAllOf(ctx, &managementv3.Cluster{}, selectors...)
}
