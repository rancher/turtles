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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	provisioningv1 "github.com/rancher/turtles/api/rancher/provisioning/v1"
	"github.com/rancher/turtles/feature"
	"github.com/rancher/turtles/util"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
	turtlesnaming "github.com/rancher/turtles/util/naming"
	turtlespredicates "github.com/rancher/turtles/util/predicates"
)

// CAPIImportReconciler represents a reconciler for importing CAPI clusters in Rancher.
type CAPIImportReconciler struct {
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
func (r *CAPIImportReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
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

	// Watch Rancher provisioningv1 clusters
	// NOTE: we will import the types from rancher in the future
	err = c.Watch(
		source.Kind(mgr.GetCache(), &provisioningv1.Cluster{}),
		handler.EnqueueRequestsFromMapFunc(r.rancherClusterToCapiCluster(ctx, capiPredicates)),
	)
	if err != nil {
		return fmt.Errorf("adding watch for Rancher cluster: %w", err)
	}

	ns := &corev1.Namespace{}

	err = c.Watch(
		source.Kind(mgr.GetCache(), ns),
		handler.EnqueueRequestsFromMapFunc(namespaceToCapiClusters(ctx, capiPredicates, r.Client)),
	)
	if err != nil {
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
// +kubebuilder:rbac:groups=provisioning.cattle.io,resources=clusters;clusters/status,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups=management.cattle.io,resources=clusterregistrationtokens;clusterregistrationtokens/status,verbs=get;list;watch

// Reconcile reconciles a CAPI cluster, creating a Rancher cluster if needed and applying the import manifests.
func (r *CAPIImportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPI cluster")

	capiCluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, capiCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	patchBase := client.MergeFromWithOptions(capiCluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

	log = log.WithValues("cluster", capiCluster.Name)

	// Collect errors as an aggregate to return together after all patches have been performed.
	var errs []error

	if !capiCluster.ObjectMeta.DeletionTimestamp.IsZero() && controllerutil.RemoveFinalizer(capiCluster, managementv3.CapiClusterFinalizer) {
		if err := r.Client.Patch(ctx, capiCluster, patchBase); err != nil {
			log.Error(err, "failed to remove CAPI cluster finalizer "+managementv3.CapiClusterFinalizer)
			errs = append(errs, err)
		}
	}

	// Wait for controlplane to be ready. This should never be false as the predicates
	// do the filtering.
	if !capiCluster.Status.ControlPlaneReady && !conditions.IsTrue(capiCluster, clusterv1.ControlPlaneReadyCondition) {
		log.Info("clusters control plane is not ready, requeue")
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	result, err := r.reconcile(ctx, capiCluster)
	if err != nil {
		errs = append(errs, fmt.Errorf("error reconciling cluster: %w", err))
	}

	if err := r.Client.Patch(ctx, capiCluster, patchBase); err != nil {
		errs = append(errs, fmt.Errorf("failed to patch cluster: %w", err))
	}

	if len(errs) > 0 {
		return ctrl.Result{}, errorutils.NewAggregate(errs)
	}

	return result, nil
}

func (r *CAPIImportReconciler) reconcile(ctx context.Context, capiCluster *clusterv1.Cluster) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// fetch the rancher cluster
	rancherCluster := &provisioningv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace: capiCluster.Namespace,
		Name:      turtlesnaming.Name(capiCluster.Name).ToRancherName(),
	}}

	err := r.RancherClient.Get(ctx, client.ObjectKeyFromObject(rancherCluster), rancherCluster)
	if client.IgnoreNotFound(err) != nil {
		log.Error(err, fmt.Sprintf("Unable to fetch rancher cluster %s", client.ObjectKeyFromObject(rancherCluster)))
		return ctrl.Result{Requeue: true}, err
	}

	if !rancherCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, capiCluster)
	}

	return r.reconcileNormal(ctx, capiCluster, rancherCluster)
}

func (r *CAPIImportReconciler) reconcileNormal(ctx context.Context, capiCluster *clusterv1.Cluster,
	rancherCluster *provisioningv1.Cluster,
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

		newCluster := &provisioningv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      turtlesnaming.Name(capiCluster.Name).ToRancherName(),
				Namespace: capiCluster.Namespace,
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: clusterv1.GroupVersion.String(),
					Kind:       clusterv1.ClusterKind,
					Name:       capiCluster.Name,
					UID:        capiCluster.UID,
				}},
				Labels: map[string]string{
					ownedLabelName: "",
				},
			},
		}

		if feature.Gates.Enabled(feature.PropagateLabels) {
			for labelKey, labelVal := range capiCluster.Labels {
				newCluster.Labels[labelKey] = labelVal
			}
		}

		if err := r.RancherClient.Create(ctx, newCluster); err != nil {
			return ctrl.Result{}, fmt.Errorf("error creating rancher cluster: %w", err)
		}

		return ctrl.Result{Requeue: true}, nil
	}

	if err != nil {
		log.Error(err, fmt.Sprintf("Unable to fetch rancher cluster %s", client.ObjectKeyFromObject(rancherCluster)))

		return ctrl.Result{}, err
	}

	if rancherCluster.Status.ClusterName == "" {
		log.Info("cluster name not set yet, requeue")
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("found cluster name", "name", rancherCluster.Status.ClusterName)

	if rancherCluster.Status.AgentDeployed {
		log.Info("agent is deployed, no action needed")
		return ctrl.Result{}, nil
	}

	// get the registration manifest
	manifest, err := getClusterRegistrationManifest(ctx, rancherCluster.Status.ClusterName, capiCluster.Namespace, r.RancherClient, r.InsecureSkipVerify)
	if err != nil {
		return ctrl.Result{}, err
	}

	if manifest == "" {
		log.Info("Import manifest URL not set yet, requeue")
		return ctrl.Result{Requeue: true}, nil
	}

	log.Info("Creating import manifest")

	remoteClient, err := r.remoteClientGetter(ctx, capiCluster.Name, r.Client, client.ObjectKeyFromObject(capiCluster))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting remote cluster client: %w", err)
	}

	if requeue, err := validateImportReadiness(ctx, remoteClient, strings.NewReader(manifest)); err != nil {
		return ctrl.Result{}, fmt.Errorf("verifying import manifest: %w", err)
	} else if requeue {
		log.Info("Import manifests are being deleted, not ready to be applied yet, requeue")
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if err := createImportManifest(ctx, remoteClient, strings.NewReader(manifest)); err != nil {
		return ctrl.Result{}, fmt.Errorf("creating import manifest: %w", err)
	}

	log.Info("Successfully applied import manifest")

	if feature.Gates.Enabled(feature.PropagateLabels) {
		patchBase := client.MergeFromWithOptions(rancherCluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

		if rancherCluster.Labels == nil {
			rancherCluster.Labels = map[string]string{}
		}

		for labelKey, labelVal := range capiCluster.Labels {
			rancherCluster.Labels[labelKey] = labelVal
		}

		if err := r.Client.Patch(ctx, rancherCluster, patchBase); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to patch Rancher cluster: %w", err)
		}

		log.Info("Successfully propagated labels to Rancher cluster")
	}

	return ctrl.Result{}, nil
}

func (r *CAPIImportReconciler) rancherClusterToCapiCluster(ctx context.Context, clusterPredicate predicate.Funcs) handler.MapFunc {
	log := log.FromContext(ctx)

	return func(_ context.Context, o client.Object) []ctrl.Request {
		capiCluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Name:      turtlesnaming.Name(o.GetName()).ToCapiName(),
			Namespace: o.GetNamespace(),
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

func (r *CAPIImportReconciler) reconcileDelete(ctx context.Context, capiCluster *clusterv1.Cluster) (ctrl.Result, error) {
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

	annotations[turtlesannotations.ClusterImportedAnnotation] = trueAnnotationValue
	capiCluster.SetAnnotations(annotations)

	return ctrl.Result{}, nil
}

// CAPICleanupReconciler is a reconciler for cleanup of managementv3 clusters.
type CAPICleanupReconciler struct {
	RancherClient client.Client
	Scheme        *runtime.Scheme
}

// SetupWithManager sets up reconciler with manager.
func (r *CAPICleanupReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, options controller.Options) error {
	if err := ctrl.NewControllerManagedBy(mgr).
		For(&managementv3.Cluster{}).
		WithOptions(options).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			_, exist := object.GetLabels()[ownedLabelName]
			return exist
		})).
		Complete(reconcile.AsReconciler(r.RancherClient, r)); err != nil {
		return fmt.Errorf("creating new downgrade controller: %w", err)
	}

	return nil
}

// Reconcile performs check for clusters and removes finalizer on the clusters in deleteion
// still containing the turtles finalizer.
func (r *CAPICleanupReconciler) Reconcile(ctx context.Context, cluster *managementv3.Cluster) (res ctrl.Result, err error) {
	log := log.FromContext(ctx)

	patchBase := client.MergeFromWithOptions(cluster.DeepCopy(), client.MergeFromWithOptimisticLock{})

	if cluster.DeletionTimestamp.IsZero() || !controllerutil.RemoveFinalizer(cluster, managementv3.CapiClusterFinalizer) {
		return
	}

	if err = r.RancherClient.Patch(ctx, cluster, patchBase); err != nil {
		log.Error(err, "Unable to remove turtles finalizer from cluster"+cluster.GetName())
	}

	return
}
