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

	bootstrapv1 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/rancher/turtles/exp/etcdrestore/webhooks"
)

// ETCDSnapshotSecretReconciler reconciles an EtcdMachineSnapshot object.
type ETCDSnapshotSecretReconciler struct {
	client.Client
}

func secretFilter(obj client.Object) bool {
	rke2ConfigName, found := obj.GetLabels()[webhooks.RKE2ConfigNameLabel]

	owned := false
	for _, owner := range obj.GetOwnerReferences() {
		if owner.Name == rke2ConfigName {
			owned = true
			break
		}
	}

	return found && !owned
}

func setKind(cl client.Client, obj client.Object) error {
	kinds, _, err := cl.Scheme().ObjectKinds(obj)
	if err != nil {
		return err
	} else if len(kinds) > 0 {
		obj.GetObjectKind().SetGroupVersionKind(kinds[0])
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ETCDSnapshotSecretReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager, _ controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		Named("etcdsnapshot-secret").
		For(&corev1.Secret{}).
		WithEventFilter(predicate.NewPredicateFuncs(secretFilter)).
		Complete(reconcile.AsReconciler(mgr.GetClient(), r))
	if err != nil {
		return fmt.Errorf("creating ETCDSnapshotSecretReconciler controller: %w", err)
	}

	return nil
}

// Reconcile reconciles the EtcdMachineSnapshot object.
func (r *ETCDSnapshotSecretReconciler) Reconcile(ctx context.Context, secret *corev1.Secret) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	rke2ConfigName, found := secret.GetLabels()[webhooks.RKE2ConfigNameLabel]
	if !found {
		return ctrl.Result{}, nil
	}

	rke2Config := &bootstrapv1.RKE2Config{}

	if err := r.Client.Get(ctx, client.ObjectKey{
		Name:      rke2ConfigName,
		Namespace: secret.Namespace,
	}, rke2Config); apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		log.Error(err, "Unable to fetch RKE2Config instance")

		return ctrl.Result{}, err
	}

	if len(secret.OwnerReferences) == 0 {
		secret.OwnerReferences = []metav1.OwnerReference{}
	}

	secret.OwnerReferences = append(secret.OwnerReferences, metav1.OwnerReference{
		APIVersion: rke2Config.APIVersion,
		Kind:       rke2Config.Kind,
		Name:       rke2Config.Name,
		UID:        rke2Config.UID,
	})

	if err := setKind(r, secret); err != nil {
		log.Error(err, "Unable to set default secret kind")

		return ctrl.Result{}, err
	}

	secret.SetManagedFields(nil)

	if err := r.Client.Patch(ctx, secret, client.Apply, []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner("clusterctl-controller"),
	}...); err != nil {
		log.Error(err, "Unable to patch Secret with RKE2Config ownership")

		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
