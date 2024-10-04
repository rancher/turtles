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

package webhooks

import (
	"context"
	"fmt"

	snapshotrestorev1 "github.com/rancher/turtles/exp/etcdrestore/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-turtles-capi-cattle-io-v1alpha1-etcdsnapshotrestore,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=turtles-capi.cattle.io,resources=etcdsnapshotrestores,verbs=create;update,versions=v1alpha1,name=etcdsnapshotrestore.kb.io,admissionReviewVersions=v1

// EtcdSnapshotRestoreWebhook defines a webhook for EtcdSnapshotRestore.
type EtcdSnapshotRestoreWebhook struct {
	client.Client
}

var _ webhook.CustomValidator = &EtcdSnapshotRestoreWebhook{}

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (r *EtcdSnapshotRestoreWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&snapshotrestorev1.ETCDSnapshotRestore{}).
		WithValidator(r).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *EtcdSnapshotRestoreWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logger := log.FromContext(ctx)

	logger.Info("Validating EtcdSnapshotRestore")

	etcdSnapshotRestore, ok := obj.(*snapshotrestorev1.ETCDSnapshotRestore)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a EtcdSnapshotRestore but got a %T", obj))
	}

	return r.validateSpec(ctx, etcdSnapshotRestore)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *EtcdSnapshotRestoreWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *EtcdSnapshotRestoreWebhook) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (r *EtcdSnapshotRestoreWebhook) validateSpec(ctx context.Context, etcdSnapshotRestore *snapshotrestorev1.ETCDSnapshotRestore) (admission.Warnings, error) {
	if err := validateRBAC(ctx, r.Client, etcdSnapshotRestore.Spec.ClusterName, etcdSnapshotRestore.Namespace); err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("failed to validate RBAC: %v", err))
	}

	return nil, nil
}
