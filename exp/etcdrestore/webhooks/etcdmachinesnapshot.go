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
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-turtles-capi-cattle-io-v1alpha1-etcdmachinesnapshot,mutating=false,failurePolicy=fail,matchPolicy=Equivalent,sideEffects=None,groups=turtles-capi.cattle.io,resources=etcdmachinesnapshots,verbs=create;update,versions=v1alpha1,name=etcdmachinesnapshot.kb.io,admissionReviewVersions=v1
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=get;create

// EtcdMachineSnapshotWebhook defines a webhook for EtcdMachineSnapshot.
type EtcdMachineSnapshotWebhook struct {
	client.Client
}

var _ webhook.CustomValidator = &EtcdMachineSnapshotWebhook{}

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (r *EtcdMachineSnapshotWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&snapshotrestorev1.ETCDMachineSnapshot{}).
		WithValidator(r).
		Complete()
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *EtcdMachineSnapshotWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	logger := log.FromContext(ctx)

	logger.Info("Validating EtcdMachineSnapshot")

	etcdMachineSnapshot, ok := obj.(*snapshotrestorev1.ETCDMachineSnapshot)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a EtcdMachineSnapshot but got a %T", obj))
	}

	return r.validateSpec(ctx, etcdMachineSnapshot)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *EtcdMachineSnapshotWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	logger := log.FromContext(ctx)

	logger.Info("Validating EtcdMachineSnapshot")

	etcdMachineSnapshot, ok := newObj.(*snapshotrestorev1.ETCDMachineSnapshot)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a EtcdMachineSnapshot but got a %T", newObj))
	}

	return r.validateSpec(ctx, etcdMachineSnapshot)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *EtcdMachineSnapshotWebhook) ValidateDelete(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (r *EtcdMachineSnapshotWebhook) validateSpec(ctx context.Context, etcdMachineSnapshot *snapshotrestorev1.ETCDMachineSnapshot) (admission.Warnings, error) {
	var allErrs field.ErrorList

	if etcdMachineSnapshot.Spec.ClusterName == "" {
		allErrs = append(allErrs, field.Required(field.NewPath("spec.clusterName"), "clusterName is required"))
	}

	if len(allErrs) > 0 {
		return nil, apierrors.NewInvalid(snapshotrestorev1.GroupVersion.WithKind("EtcdMachineSnapshot").GroupKind(), etcdMachineSnapshot.Name, allErrs)
	}

	if err := validateRBAC(ctx, r.Client, etcdMachineSnapshot.Spec.ClusterName, etcdMachineSnapshot.Namespace); err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("failed to validate RBAC: %v", err))
	}

	return nil, nil
}
