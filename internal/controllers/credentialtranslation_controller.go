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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

const (
	// awsDriverAnnotationValue is the value of the driver annotation that identifies AWS credentials.
	awsDriverAnnotationValue = "aws"

	// awsRancherCCFinalizer is the finalizer added to Rancher Cloud Credentials to ensure
	// cleanup of the derived AWSClusterStaticIdentity when the credential is deleted.
	awsRancherCCFinalizer = "cloudcredential.cattle.io/aws-identity-finalizer"

	// capaProviderSpecName is CAPA's CAPIProvider `spec.name`.
	capaProviderSpecName = "aws"

	// capaProviderNamespace is the default namespace where CAPA is installed and where the credentials secret will be created.
	// defaults to `capa-system`.
	capaProviderNamespace = "capa-system"

	// rancherAWSAccessKeyField is the field name in Rancher Cloud Credential secrets for the AWS access key ID.
	rancherAWSAccessKeyField = "amazonec2credentialConfig-accessKey"

	// rancherAWSSecretKeyField is the field name in Rancher Cloud Credential secrets for the AWS secret key.
	//nolint:gosec  // This is not a hardcoded secret, just a key name.
	rancherAWSSecretKeyField = "amazonec2credentialConfig-secretKey"
)

// RancherCredentialReconciler reconciles Rancher Cloud Credentials in the cattle-global-data
// namespace into CAPA-specific AWSClusterStaticIdentity resources. This enables users to reuse
// Rancher Cloud Credentials when provisioning AWS clusters with CAPI/CAPA.
type RancherCredentialReconciler struct {
	Client client.Client

	// CAPASystemNamespace is the namespace where the CAPA controller is installed and where
	// the credentials secret will be created. Defaults to "capa-system".
	CAPASystemNamespace string
}

// SetupWithManager sets up the controller with the Manager.
func (r *RancherCredentialReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	if r.CAPASystemNamespace == "" {
		r.CAPASystemNamespace = capaProviderNamespace
	}

	log := log.FromContext(ctx)

	log.Info("Watching AWS Cloud Credential secrets")

	secretPredicate := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == rancherCredentialsNamespace &&
			obj.GetAnnotations()[driverNameAnnotation] == awsDriverAnnotationValue
	})

	capiProviderPredicate := predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			providerObj, ok := e.Object.(*turtlesv1.CAPIProvider)
			if !ok {
				return false
			}

			return providerObj.Spec.Type == turtlesv1.Infrastructure &&
				providerObj.Spec.Name == capaProviderSpecName
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
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusterstaticidentities,verbs=get;list;watch;create;update;patch;delete

// Reconcile watches Rancher Cloud Credentials for AWS and translates them into
// AWSClusterStaticIdentity resources for use with CAPA.
func (r *RancherCredentialReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	credential := &corev1.Secret{}
	if err := r.Client.Get(ctx, req.NamespacedName, credential); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// verify that the CAPA CAPIProvider is available and ready
	var provider turtlesv1.CAPIProvider

	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      capaProviderSpecName,
		Namespace: capaProviderNamespace,
	}, &provider)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	if provider.Status.Phase != turtlesv1.Ready {
		return ctrl.Result{RequeueAfter: defaultRequeueDuration}, nil
	}

	if err := r.Client.Get(ctx, req.NamespacedName, credential); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if credential.GetAnnotations()[driverNameAnnotation] != awsDriverAnnotationValue {
		return ctrl.Result{}, nil
	}

	if !credential.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, credential)
	}

	return r.reconcileNormal(ctx, credential)
}

// reconcileNormal handles creation and updates of the AWSClusterStaticIdentity for a given
// Rancher Cloud Credential.
func (r *RancherCredentialReconciler) reconcileNormal(ctx context.Context, credential *corev1.Secret) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Reconciling AWS Cloud Credentials to CAPA translation", "credential", client.ObjectKeyFromObject(credential))

	// add finalizer to the credential so we can clean up derived resources on deletion.
	if !controllerutil.ContainsFinalizer(credential, awsRancherCCFinalizer) {
		patch := client.MergeFrom(credential.DeepCopy())
		controllerutil.AddFinalizer(credential, awsRancherCCFinalizer)

		if err := r.Client.Patch(ctx, credential, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding finalizer to credential %s: %w", client.ObjectKeyFromObject(credential), err)
		}

		log.Info("Added finalizer to AWS credential", "credential", client.ObjectKeyFromObject(credential))
	}

	accessKeyID := string(credential.Data[rancherAWSAccessKeyField])
	secretAccessKey := string(credential.Data[rancherAWSSecretKeyField])

	if accessKeyID == "" || secretAccessKey == "" {
		log.Info("AWS credential secret is missing required keys, skipping",
			"credential", client.ObjectKeyFromObject(credential),
			"missingKeys", fmt.Sprintf("%s or %s", rancherAWSAccessKeyField, rancherAWSSecretKeyField))

		return ctrl.Result{}, nil
	}

	identityName := credential.Name

	if err := r.reconcileCredentialSecret(ctx, identityName, accessKeyID, secretAccessKey); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.reconcileAWSClusterStaticIdentity(ctx, identityName); err != nil {
		return ctrl.Result{}, err
	}

	// annotate the original Rancher credential with a reference to the AWSClusterStaticIdentity.
	if credential.GetAnnotations()[turtlesannotations.AWSClusterStaticIdentityRefAnnotation] != identityName {
		patch := client.MergeFrom(credential.DeepCopy())

		annotations := credential.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		annotations[turtlesannotations.AWSClusterStaticIdentityRefAnnotation] = identityName
		credential.SetAnnotations(annotations)

		if err := r.Client.Patch(ctx, credential, patch); err != nil {
			return ctrl.Result{}, fmt.Errorf("annotating credential %s with identity reference: %w", client.ObjectKeyFromObject(credential), err)
		}

		log.Info("Annotated AWS credential with AWSClusterStaticIdentity reference",
			"credential", client.ObjectKeyFromObject(credential),
			"identity", identityName)
	}

	log.Info("Successfully reconciled AWS credential translation",
		"credential", client.ObjectKeyFromObject(credential),
		"identity", identityName)

	return ctrl.Result{}, nil
}

// reconcileDelete cleans up the AWSClusterStaticIdentity and its credentials secret when
// the Rancher Cloud Credential is deleted.
func (r *RancherCredentialReconciler) reconcileDelete(ctx context.Context, credential *corev1.Secret) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	if !controllerutil.ContainsFinalizer(credential, awsRancherCCFinalizer) {
		return ctrl.Result{}, nil
	}

	identityName := credential.Name

	awsIdentity := r.awsClusterStaticIdentity(identityName)
	if err := r.Client.Delete(ctx, awsIdentity); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("deleting AWSClusterStaticIdentity %s: %w", identityName, err)
	}

	log.Info("Deleted AWSClusterStaticIdentity", "name", identityName)

	credSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      identityName,
			Namespace: r.CAPASystemNamespace,
		},
	}

	if err := r.Client.Delete(ctx, credSecret); err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("deleting credentials secret %s/%s: %w", r.CAPASystemNamespace, identityName, err)
	}

	log.Info("Deleted credentials secret", "namespace", r.CAPASystemNamespace, "name", identityName)

	// Remove the finalizer so the credential can be garbage-collected.
	patch := client.MergeFrom(credential.DeepCopy())
	controllerutil.RemoveFinalizer(credential, awsRancherCCFinalizer)

	if err := r.Client.Patch(ctx, credential, patch); err != nil {
		return ctrl.Result{}, fmt.Errorf("removing finalizer from credential %s: %w", client.ObjectKeyFromObject(credential), err)
	}

	return ctrl.Result{}, nil
}

// reconcileCredentialSecret creates or updates the credentials Secret in the CAPA system namespace.
func (r *RancherCredentialReconciler) reconcileCredentialSecret(ctx context.Context, name, accessKeyID, secretAccessKey string) error {
	credSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: r.CAPASystemNamespace,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, credSecret, func() error {
		credSecret.Data = map[string][]byte{
			"AccessKeyID":     []byte(accessKeyID),
			"SecretAccessKey": []byte(secretAccessKey),
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("creating/updating credentials secret %s/%s: %w", r.CAPASystemNamespace, name, err)
	}

	return nil
}

// reconcileAWSClusterStaticIdentity creates or updates the AWSClusterStaticIdentity referencing
// the credentials secret.
func (r *RancherCredentialReconciler) reconcileAWSClusterStaticIdentity(ctx context.Context, name string) error {
	awsIdentity := r.awsClusterStaticIdentity(name)

	identitySpec := map[string]any{
		"secretRef":         name,
		"allowedNamespaces": map[string]any{},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, awsIdentity, func() error {
		awsIdentity.Object["spec"] = identitySpec

		return nil
	})
	if err != nil {
		return fmt.Errorf("creating/updating AWSClusterStaticIdentity %s: %w", name, err)
	}

	return nil
}

// enqueueAllMatchingSecrets runs when CAPA is installed and the `CAPIProvider` is ready.
func (r *RancherCredentialReconciler) enqueueAllMatchingSecrets(ctx context.Context, _ client.Object) []ctrl.Request {
	var secretList corev1.SecretList

	err := r.Client.List(ctx, &secretList, client.InNamespace(rancherCredentialsNamespace))
	if err != nil {
		return nil
	}

	var requests []ctrl.Request

	for _, secret := range secretList.Items {
		if secret.GetAnnotations()[driverNameAnnotation] == awsDriverAnnotationValue {
			requests = append(requests, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			})
		}
	}

	return requests
}

// awsClusterStaticIdentity returns an unstructured AWSClusterStaticIdentity with the given name.
func (r *RancherCredentialReconciler) awsClusterStaticIdentity(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Version: "v1beta2",
		Kind:    "AWSClusterStaticIdentity",
	})
	obj.SetName(name)

	return obj
}
