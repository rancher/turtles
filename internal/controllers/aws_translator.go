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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// rancherAWSAccessKeyField is the field name in Rancher Cloud Credential secrets for the AWS access key ID.
	rancherAWSAccessKeyField = "amazonec2credentialConfig-accessKey"

	// rancherAWSSecretKeyField is the field name in Rancher Cloud Credential secrets for the AWS secret key.
	//nolint:gosec  // This is not a hardcoded secret, just a key name.
	rancherAWSSecretKeyField = "amazonec2credentialConfig-secretKey"
)

// allowedAWSClusterNamespace is the only namespace where AWSClusterStaticIdentities will be allowed to be used.
// for now it only supports `fleet-default`.
var allowedAWSClusterNamespaceList = []string{
	"fleet-default",
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=awsclusterstaticidentities,verbs=get;list;watch;create;update;patch;delete

// AWSTranslator implements the Translator interface for AWS credentials.
type AWSTranslator struct{}

// Translate is the main method for translation Cloud Credentials into CAPA identities.
func (a *AWSTranslator) Translate(ctx context.Context, cl client.Client, credential *corev1.Secret) (string, error) {
	accessKeyID := string(credential.Data[rancherAWSAccessKeyField])
	secretAccessKey := string(credential.Data[rancherAWSSecretKeyField])

	if accessKeyID == "" || secretAccessKey == "" {
		return "", nil
	}

	if err := createOrUpdateCredentialSecret(ctx, cl, credential, accessKeyID, secretAccessKey, a.ProviderNamespace()); err != nil {
		return "", err
	}

	if err := createOrUpdateAWSClusterStaticIdentity(ctx, cl, credential); err != nil {
		return "", err
	}

	return credential.Name, nil
}

// Cleanup removes the translated CAPA identity and referenced secret for the given Cloud Credential.
func (a *AWSTranslator) Cleanup(ctx context.Context, cl client.Client, credential *corev1.Secret) error {
	log := log.FromContext(ctx)

	identityName := credential.Name

	awsIdentity := awsClusterStaticIdentity(identityName)

	if err := cl.Get(ctx, types.NamespacedName{Name: identityName}, awsIdentity); err == nil {
		if isObjectOwnedBySecret(awsIdentity, credential) {
			if err := cl.Delete(ctx, awsIdentity); err != nil && !apierrors.IsNotFound(err) {
				return err
			}

			log.Info("Deleted AWSClusterStaticIdentity", "name", identityName)
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	credSecret := &corev1.Secret{}

	if err := cl.Get(ctx, types.NamespacedName{Name: identityName, Namespace: a.ProviderNamespace()}, credSecret); err == nil {
		if isObjectOwnedBySecret(credSecret, credential) {
			if err := cl.Delete(ctx, credSecret); err != nil && !apierrors.IsNotFound(err) {
				return err
			}

			log.Info("Deleted credentials secret", "namespace", a.ProviderNamespace(), "name", identityName)
		}
	} else if !apierrors.IsNotFound(err) {
		return err
	}

	return nil
}

// DriverName is the name of the provider in the Rancher Cloud Credential Secret.
func (a *AWSTranslator) DriverName() string { return "aws" }

// ProviderName is the name of the CAPI provider.
func (a *AWSTranslator) ProviderName() string { return "aws" }

// ProviderNamespace is the namespace where the `CAPIProvider` is installed.
func (a *AWSTranslator) ProviderNamespace() string { return "capa-system" }

// Finalizer is the finalizer set on the original Rancher Cloud Credential to control garbage collection of translated resources.
func (a *AWSTranslator) Finalizer() string { return "cloudcredential.cattle.io/aws-identity-finalizer" }

// awsClusterStaticIdentity returns an unstructured AWSClusterStaticIdentity with the given name.
func awsClusterStaticIdentity(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Version: "v1beta2",
		Kind:    "AWSClusterStaticIdentity",
	})
	obj.SetName(name)

	return obj
}

// createOrUpdateCredentialSecret creates or updates the credentials Secret in the CAPA system namespace.
func createOrUpdateCredentialSecret(ctx context.Context, cl client.Client, sourceSecret *corev1.Secret, accessKey, secretKey, ns string) error {
	credSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sourceSecret.Name,
			Namespace: ns,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, cl, credSecret, func() error {
		if err := verifySecretOwnership(credSecret, sourceSecret); err != nil {
			return err
		}

		credSecret.Data = map[string][]byte{
			"AccessKeyID":     []byte(accessKey),
			"SecretAccessKey": []byte(secretKey),
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("creating/updating credentials secret %s/%s: %w", ns, sourceSecret.Name, err)
	}

	return nil
}

// createOrUpdateAWSClusterStaticIdentity creates or updates the AWSClusterStaticIdentity referencing
// the credentials secret.
func createOrUpdateAWSClusterStaticIdentity(ctx context.Context, cl client.Client, sourceSecret *corev1.Secret) error {
	awsIdentity := awsClusterStaticIdentity(sourceSecret.Name)

	identitySpec := map[string]any{
		"secretRef": sourceSecret.Name,
		"allowedNamespaces": map[string]any{
			"list": allowedAWSClusterNamespaceList,
		},
	}

	_, err := controllerutil.CreateOrUpdate(ctx, cl, awsIdentity, func() error {
		if err := verifySecretOwnership(awsIdentity, sourceSecret); err != nil {
			return err
		}

		awsIdentity.Object["spec"] = identitySpec

		return nil
	})
	if err != nil {
		return fmt.Errorf("creating/updating AWSClusterStaticIdentity %s: %w", sourceSecret.Name, err)
	}

	return nil
}
