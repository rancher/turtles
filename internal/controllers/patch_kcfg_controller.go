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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util/predicates"

	provisioningv1 "github.com/rancher/turtles/api/rancher/provisioning/v1"
	turtlespredicates "github.com/rancher/turtles/util/predicates"
)

// RancherKubeconfigSecretReconciler is a controller that will reconcile secrets created by Rancher as
// part of provisioning v2. Its job is to add the label required by Cluster API v1.5.0 and higher.
type RancherKubeconfigSecretReconciler struct {
	Client           client.Client
	recorder         record.EventRecorder
	WatchFilterValue string
	Scheme           *runtime.Scheme

	controller      controller.Controller
	externalTracker external.ObjectTracker
}

// SetupWithManager will setup the controller.
func (r *RancherKubeconfigSecretReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	capiPredicates := predicates.All(r.Scheme, log,
		turtlespredicates.V2ProvClusterOwned(log),
		turtlespredicates.NameHasSuffix(log, "-kubeconfig"),
	)

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Secret{}).
		WithOptions(options).
		WithEventFilter(capiPredicates).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating new controller: %w", err)
	}

	r.recorder = mgr.GetEventRecorderFor("rancher-turtles-v2prov")
	r.controller = c
	r.externalTracker = external.ObjectTracker{
		Controller: c,
	}

	return nil
}

// +kubebuilder:rbac:groups="",resources=secrets;events,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;create;update
// +kubebuilder:rbac:groups=provisioning.cattle.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile will patch v2prov created kubeconfig secrets to add the required owner label if its missing.
func (r *RancherKubeconfigSecretReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling v2prov cluster")

	secret := &corev1.Secret{}
	if err := r.Client.Get(ctx, req.NamespacedName, secret); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{Requeue: true}, err
	}

	if _, ok := secret.Labels[clusterv1.ClusterNameLabel]; ok {
		log.V(4).Info("kubeconfig secret %s/%s already has the capi cluster label", secret.Name, secret.Name)

		return ctrl.Result{}, nil
	}

	clusterName, err := r.getClusterName(ctx, secret)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("getting cluster name from secret: %w", err)
	}

	if clusterName == "" {
		log.Info("Could not determine cluster name from kubeconfig secret")

		return ctrl.Result{}, nil
	}

	secretCopy := secret.DeepCopy()
	if secretCopy.Labels == nil {
		secretCopy.Labels = map[string]string{}
	}

	secretCopy.Labels[clusterv1.ClusterNameLabel] = clusterName

	patchBase := client.MergeFromWithOptions(secret, client.MergeFromWithOptimisticLock{})

	if err := r.Client.Patch(ctx, secretCopy, patchBase); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to patch secret: %w", err)
	}

	log.V(4).Info("patched kubeconfig secret", "name", secret.Name, "namespace", secret.Namespace, "cluster", clusterName)

	return ctrl.Result{}, nil
}

func (r *RancherKubeconfigSecretReconciler) getClusterName(ctx context.Context, secret *corev1.Secret) (string, error) {
	v2ProvClusterName := ""

	for _, ref := range secret.OwnerReferences {
		if ref.APIVersion == provisioningv1.GroupVersion.Identifier() {
			if ref.Kind == "Cluster" {
				v2ProvClusterName = ref.Name

				break
			}
		}
	}

	if v2ProvClusterName == "" {
		return "", nil
	}

	v2ProvCluster := &provisioningv1.Cluster{}

	if err := r.Client.Get(ctx, types.NamespacedName{Name: v2ProvClusterName, Namespace: secret.Namespace}, v2ProvCluster); err != nil {
		return "", fmt.Errorf("getting rancher cluster: %w", err)
	}

	if v2ProvCluster.Spec.RKEConfig == nil {
		return "", nil
	}

	return v2ProvCluster.Name, nil
}
