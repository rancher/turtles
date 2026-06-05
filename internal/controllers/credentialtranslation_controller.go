/*
Copyright © 2023 - 2026 SUSE LLC

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
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
)

// CredentialTranslator defines the interface for cloud-specific translation logic.
type CredentialTranslator interface {
	// DriverName returns the Rancher driver annotation value (e.g. "aws", "azure").
	DriverName() string
	// ProviderName returns the CAPA/CAPZ CAPIProvider spec name (e.g. "aws", "azure").
	ProviderName() string
	// ProviderNamespace returns the namespace where the CAPIProvider is installed (e.g. "capa-system").
	ProviderNamespace() string
	// Finalizer returns the specific finalizer to attach to the Rancher Secret.
	Finalizer() string

	// Translate handles creating/updating the CAPI-specific identity resources.
	// It returns the name of the identity resource created so the core controller can annotate the secret.
	Translate(ctx context.Context, cl client.Client, credential *corev1.Secret) (string, error)

	// Cleanup removes the CAPI-specific identity resources when the Rancher secret is deleted.
	Cleanup(ctx context.Context, cl client.Client, credential *corev1.Secret) error
}

// RancherCredentialReconciler reconciles Rancher Cloud Credentials in the cattle-global-data
// namespace into CAPI-specific identity resources. This enables users to reuse
// Rancher Cloud Credentials when provisioning.
type RancherCredentialReconciler struct {
	Client client.Client
	// Translators holds provider-specific logic referenced by driver name (e.g. "aws" -> AWSTranslator)
	Translators map[string]CredentialTranslator
}

// SetupWithManager sets up the controller with the Manager.
func (r *RancherCredentialReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	log.Info("Watching Rancher Cloud Credential secrets")

	secretPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		if obj.GetNamespace() != rancherCredentialsNamespace {
			return false
		}

		driver := obj.GetAnnotations()[driverNameAnnotation]
		_, supported := r.Translators[driver]

		return supported
	})

	capiProviderPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			providerObj, ok := e.Object.(*turtlesv1.CAPIProvider)
			if !ok || providerObj.Spec.Type != turtlesv1.Infrastructure {
				return false
			}

			for _, t := range r.Translators {
				if providerObj.Spec.Name == t.ProviderName() {
					return true
				}
			}

			return false
		},

		UpdateFunc: func(_ event.UpdateEvent) bool {
			return false
		},

		DeleteFunc: func(_ event.DeleteEvent) bool {
			return false
		},

		GenericFunc: func(_ event.GenericEvent) bool {
			return false
		},
	}

	if err := ctrl.NewControllerManagedBy(mgr).
		Named("rancher-credential-translation").
		WithOptions(options).
		For(&corev1.Secret{}, builder.WithPredicates(secretPredicate)).
		Watches(
			&turtlesv1.CAPIProvider{},
			handler.EnqueueRequestsFromMapFunc(r.enqueueAllMatchingSecrets),
			builder.WithPredicates(capiProviderPredicate),
		).
		Complete(r); err != nil {
		return fmt.Errorf("creating RancherCredential translation controller: %w", err)
	}

	return nil
}

//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch

// Reconcile watches Rancher Cloud Credentials for AWS and translates them into
// AWSClusterStaticIdentity resources for use with CAPA.
func (r *RancherCredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	credential := &corev1.Secret{}
	if err := r.Client.Get(ctx, req.NamespacedName, credential); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	driverName := credential.GetAnnotations()[driverNameAnnotation]

	translator, supported := r.Translators[driverName]
	if !supported {
		return ctrl.Result{}, nil
	}

	// verify that the CAPA CAPIProvider is available and ready
	var provider turtlesv1.CAPIProvider

	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      translator.ProviderName(),
		Namespace: translator.ProviderNamespace(),
	}, &provider)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if provider.Status.Phase != turtlesv1.Ready {
		log.Info("CAPIProvider is installed but not Ready.", "provider", translator.ProviderName(), "namespace", translator.ProviderNamespace())
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if !credential.DeletionTimestamp.IsZero() {
		log.Info("Rancher Cloud Credential is marked for deletion, reconciling delete", "credential", client.ObjectKeyFromObject(credential))
		return r.reconcileDelete(ctx, credential, translator)
	}

	return r.reconcileNormal(ctx, credential, translator)
}

// reconcileNormal handles creation and updates of the AWSClusterStaticIdentity for a given
// Rancher Cloud Credential.
func (r *RancherCredentialReconciler) reconcileNormal(ctx context.Context, credential *corev1.Secret, t CredentialTranslator) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling Cloud Credentials to CAPI translation", "credential", client.ObjectKeyFromObject(credential))

	// add finalizer so we can cleanup resources on deletion.
	if !controllerutil.ContainsFinalizer(credential, t.Finalizer()) {
		patch := client.MergeFrom(credential.DeepCopy())
		controllerutil.AddFinalizer(credential, t.Finalizer())

		if err := r.Client.Patch(ctx, credential, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer to credential %s: %w", client.ObjectKeyFromObject(credential), err)
		}
	}

	identityName, err := t.Translate(ctx, r.Client, credential)
	if err != nil {
		return ctrl.Result{}, err
	}

	if identityName == "" {
		return ctrl.Result{}, nil
	}

	// annotate the original Rancher credential with a reference to the CAPI identity translated object.
	if credential.GetAnnotations()[turtlesannotations.CAPIIdentityRefAnnotation] != identityName {
		patch := client.MergeFrom(credential.DeepCopy())

		annotations := credential.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		annotations[turtlesannotations.CAPIIdentityRefAnnotation] = identityName
		credential.SetAnnotations(annotations)

		if err := r.Client.Patch(ctx, credential, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("annotating credential %s with identity reference: %w", client.ObjectKeyFromObject(credential), err)
		}
	}

	return ctrl.Result{}, nil
}

// reconcileDelete cleans up the CAPI identity and its credentials secret when
// the Rancher Cloud Credential is deleted.
func (r *RancherCredentialReconciler) reconcileDelete(ctx context.Context, credential *corev1.Secret, t CredentialTranslator) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(credential, t.Finalizer()) {
		return ctrl.Result{}, nil
	}

	// first removed the CAPI identity objects
	if err := t.Cleanup(ctx, r.Client, credential); err != nil {
		return ctrl.Result{}, err
	}

	// remove the finalizer so the original cloud credential can be garbage-collected.
	patch := client.MergeFrom(credential.DeepCopy())
	controllerutil.RemoveFinalizer(credential, t.Finalizer())

	if err := r.Client.Patch(ctx, credential, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer from credential %s: %w", client.ObjectKeyFromObject(credential), err)
	}

	return ctrl.Result{}, nil
}

// enqueueAllMatchingSecrets runs when `CAPIProvider` is installed and ready.
func (r *RancherCredentialReconciler) enqueueAllMatchingSecrets(ctx context.Context, obj client.Object) []ctrl.Request {
	provider, ok := obj.(*turtlesv1.CAPIProvider)
	if !ok {
		return nil
	}

	var targetProvider string

	for _, t := range r.Translators {
		if t.ProviderName() == provider.Spec.Name {
			targetProvider = t.DriverName()
			break
		}
	}

	if targetProvider == "" {
		return nil
	}

	// List and enqueue only secrets for this specific provider
	var secretList corev1.SecretList

	err := r.Client.List(ctx, &secretList, client.InNamespace(rancherCredentialsNamespace))
	if err != nil {
		return nil
	}

	var requests []ctrl.Request

	for _, secret := range secretList.Items {
		if secret.GetAnnotations()[driverNameAnnotation] == targetProvider {
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(&secret),
			})
		}
	}

	return requests
}
