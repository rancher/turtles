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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
)

var _ = Describe("Rancher Cloud Credentials Translation", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
		scheme     *runtime.Scheme

		credentialReconciler RancherCredentialReconciler
		request              ctrl.Request
		provider             *turtlesv1.CAPIProvider
		rancherAWSSecret     *corev1.Secret
		translatedSecret     *corev1.Secret
		existingSecret       *corev1.Secret
		translatedIdentity   *unstructured.Unstructured
	)

	BeforeEach(func() {
		ctx = context.TODO()
		scheme = runtime.NewScheme()

		Expect(corev1.AddToScheme(scheme)).To(Succeed())
		Expect(turtlesv1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		aws := &AWSTranslator{}
		credentialReconciler = RancherCredentialReconciler{
			Client:      fakeClient,
			Translators: map[string]CredentialTranslator{"aws": aws},
		}

		provider = &turtlesv1.CAPIProvider{
			Name:      aws.ProviderName(),
			Namespace: aws.ProviderNamespace(),
			Spec: turtlesv1.CAPIProviderSpec{
				Type: turtlesv1.Infrastructure,
			},
			Status: turtlesv1.CAPIProviderStatus{
				Phase: turtlesv1.Ready,
			},
		}

		// The fake client neither assigns UIDs nor converts StringData, so both are set explicitly.
		rancherAWSSecret = &corev1.Secret{
			Name:      "cctest",
			Namespace: rancherCredentialsNamespace,
			UID:       "cctest-uid",
			Annotations: map[string]string{
				"field.cattle.io/name": "aws-cred",
				driverNameAnnotation:   "aws",
			},
			Data: map[string][]byte{
				rancherAWSAccessKeyField: []byte("access-key"),
				rancherAWSSecretKeyField: []byte("secret-key"),
			},
		}
		Expect(fakeClient.Create(ctx, rancherAWSSecret)).Should(Succeed())

		request = ctrl.Request{NamespacedName: client.ObjectKeyFromObject(rancherAWSSecret)}

		translatedSecret = &corev1.Secret{
			Name:      rancherAWSSecret.Name,
			Namespace: aws.ProviderNamespace(),
		}

		existingSecret = &corev1.Secret{
			Name:      rancherAWSSecret.Name,
			Namespace: aws.ProviderNamespace(),
			Data: map[string][]byte{
				"testData": []byte("test-unchanged-data"),
			},
		}

		// Built independently of the translator so a wrong GVK in the controller code is caught.
		translatedIdentity = &unstructured.Unstructured{}
		translatedIdentity.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "infrastructure.cluster.x-k8s.io",
			Version: "v1beta2",
			Kind:    "AWSClusterStaticIdentity",
		})
		translatedIdentity.SetName(rancherAWSSecret.Name)
	})

	It("Should reconcile and translate Rancher Cloud Credential secret when CAPA is installed and becomes ready", func() {
		provider.Status.Phase = ""
		Expect(fakeClient.Create(ctx, provider)).Should(Succeed())

		// provider is not yet ready, controller should requeue.
		result, err := credentialReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result.RequeueAfter).ShouldNot(BeZero())

		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedSecret), translatedSecret)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

		provider.Status.Phase = turtlesv1.Ready
		Expect(fakeClient.Update(ctx, provider)).Should(Succeed())

		// provider is now ready, controller should translate credentials.
		result, err = credentialReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result.RequeueAfter).Should(BeZero())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedIdentity), translatedIdentity)).Should(Succeed())
		Expect(translatedIdentity.GetAnnotations()).Should(HaveKeyWithValue(cloudCredentialSecretAnnotation, string(rancherAWSSecret.UID)))

		secretRef, found, err := unstructured.NestedString(translatedIdentity.Object, "spec", "secretRef")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(found).Should(BeTrue(), "secretRef field should exist")
		Expect(secretRef).Should(Equal(rancherAWSSecret.Name))

		allowedNs, found, err := unstructured.NestedStringSlice(translatedIdentity.Object, "spec", "allowedNamespaces", "list")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(found).Should(BeTrue(), "allowedNamespaces field should exist")
		Expect(allowedNs).Should(ContainElement("fleet-default"))

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedSecret), translatedSecret)).Should(Succeed())
		Expect(translatedSecret.Data).Should(HaveKeyWithValue("AccessKeyID", rancherAWSSecret.Data[rancherAWSAccessKeyField]))
		Expect(translatedSecret.Data).Should(HaveKeyWithValue("SecretAccessKey", rancherAWSSecret.Data[rancherAWSSecretKeyField]))
		Expect(translatedSecret.GetAnnotations()).Should(HaveKeyWithValue(cloudCredentialSecretAnnotation, string(rancherAWSSecret.UID)))

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rancherAWSSecret), rancherAWSSecret)).Should(Succeed())
		Expect(rancherAWSSecret.GetAnnotations()[turtlesannotations.CAPIIdentityRefAnnotation]).Should(Equal(translatedIdentity.GetName()))
	})

	It("Should delete translated CAPI identity when Rancher Cloud Credential is removed", func() {
		Expect(fakeClient.Create(ctx, provider)).Should(Succeed())

		_, err := credentialReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedIdentity), translatedIdentity)).Should(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedSecret), translatedSecret)).Should(Succeed())

		Expect(fakeClient.Delete(ctx, rancherAWSSecret)).Should(Succeed())

		_, err = credentialReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())

		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedIdentity), translatedIdentity)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedSecret), translatedSecret)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
	})

	It("Should not delete existing secret when a collision is detected and Rancher Cloud Credential is removed", func() {
		Expect(fakeClient.Create(ctx, provider)).Should(Succeed())
		Expect(fakeClient.Create(ctx, existingSecret)).Should(Succeed())

		_, err := credentialReconciler.Reconcile(ctx, request)
		Expect(err).Should(MatchError(ContainSubstring("collision detected")))

		Expect(fakeClient.Delete(ctx, rancherAWSSecret)).Should(Succeed())

		_, err = credentialReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())

		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(rancherAWSSecret), rancherAWSSecret)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(existingSecret), existingSecret)).Should(Succeed())
		Expect(existingSecret.Data).Should(HaveKeyWithValue("testData", []byte("test-unchanged-data")))
		Expect(existingSecret.GetAnnotations()).ShouldNot(HaveKey(cloudCredentialSecretAnnotation))
	})

	It("Should update AWSClusterStaticIdentity when AWS Cloud Credential changes", func() {
		Expect(fakeClient.Create(ctx, provider)).Should(Succeed())

		_, err := credentialReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedIdentity), translatedIdentity)).Should(Succeed())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedSecret), translatedSecret)).Should(Succeed())
		Expect(translatedSecret.Data).Should(HaveKeyWithValue("AccessKeyID", rancherAWSSecret.Data[rancherAWSAccessKeyField]))
		Expect(translatedSecret.Data).Should(HaveKeyWithValue("SecretAccessKey", rancherAWSSecret.Data[rancherAWSSecretKeyField]))

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rancherAWSSecret), rancherAWSSecret)).Should(Succeed())
		Expect(rancherAWSSecret.GetAnnotations()[turtlesannotations.CAPIIdentityRefAnnotation]).Should(Equal(translatedIdentity.GetName()))

		// update existing Cloud Credentials Secret
		rancherAWSSecret.Data = map[string][]byte{
			rancherAWSAccessKeyField: []byte("new-access-key"),
			rancherAWSSecretKeyField: []byte("new-secret-key"),
		}
		Expect(fakeClient.Update(ctx, rancherAWSSecret)).Should(Succeed())

		_, err = credentialReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedSecret), translatedSecret)).Should(Succeed())
		Expect(translatedSecret.Data).Should(HaveKeyWithValue("AccessKeyID", []byte("new-access-key")))
		Expect(translatedSecret.Data).Should(HaveKeyWithValue("SecretAccessKey", []byte("new-secret-key")))
	})

	It("Should not translate Rancher Cloud Credential secret when a collision is detected (a secret with the same name already exists)", func() {
		Expect(fakeClient.Create(ctx, provider)).Should(Succeed())
		Expect(fakeClient.Create(ctx, existingSecret)).Should(Succeed())

		_, err := credentialReconciler.Reconcile(ctx, request)
		Expect(err).Should(MatchError(ContainSubstring("collision detected")))

		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedIdentity), translatedIdentity)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(existingSecret), existingSecret)).Should(Succeed())
		Expect(existingSecret.Data).Should(HaveKeyWithValue("testData", []byte("test-unchanged-data")))
		Expect(existingSecret.GetAnnotations()).ShouldNot(HaveKey(cloudCredentialSecretAnnotation))
	})

	It("Should not error when CAPA is not installed (Cloud Credentials are not translated) and not requeue", func() {
		Expect(credentialReconciler.Reconcile(ctx, request)).Should(BeZero())

		err := fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedSecret), translatedSecret)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedIdentity), translatedIdentity)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(rancherAWSSecret), rancherAWSSecret)).Should(Succeed())
		Expect(rancherAWSSecret.Finalizers).Should(BeEmpty())
	})

	It("Should translate existing Cloud Credentials after CAPA CAPIProvider is installed", func() {
		// first we check that no translation happens when CAPA is not installed.
		// then we install CAPA and expect existing Cloud Credentials to be translated.
		_, err := credentialReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())

		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedSecret), translatedSecret)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

		Expect(fakeClient.Create(ctx, provider)).Should(Succeed())

		_, err = credentialReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedIdentity), translatedIdentity)).Should(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedSecret), translatedSecret)).Should(Succeed())
	})

	It("Should not translate non-AWS Rancher Cloud Credential", func() {
		Expect(fakeClient.Create(ctx, provider)).Should(Succeed())

		rancherAzureSecret := &corev1.Secret{
			Name:      "cctest-azure",
			Namespace: rancherCredentialsNamespace,
			Annotations: map[string]string{
				"field.cattle.io/name": "azure-cred",
				driverNameAnnotation:   "azure",
			},
			Data: map[string][]byte{
				"sampleazure": []byte("sample-azure-key"),
			},
		}
		Expect(fakeClient.Create(ctx, rancherAzureSecret)).Should(Succeed())

		_, err := credentialReconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(rancherAzureSecret)})
		Expect(err).ShouldNot(HaveOccurred())

		translatedSecret.Name = rancherAzureSecret.Name
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedSecret), translatedSecret)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

		translatedIdentity.SetName(rancherAzureSecret.Name)
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(translatedIdentity), translatedIdentity)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
	})
})
