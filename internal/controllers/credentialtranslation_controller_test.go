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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/test"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Rancher Cloud Credentials Translation", func() {
	var (
		r                           *RancherCredentialReconciler
		awsClusterStaticIdentityCRD *apiextensionsv1.CustomResourceDefinition
		providerTemplate            *turtlesv1.CAPIProvider
		rancherAWSSecret            *corev1.Secret
		rancherAzureSecret          *corev1.Secret
		translatedSecret            *corev1.Secret
		existingSecret              *corev1.Secret
		capaNs                      *corev1.Namespace
		translatedIdentity          *unstructured.Unstructured
	)

	BeforeEach(func() {
		SetClient(testEnv)
		SetContext(ctx)

		r = &RancherCredentialReconciler{
			Client: cl,
			Translators: map[string]CredentialTranslator{
				"aws": &AWSTranslator{},
			},
		}

		awsClusterStaticIdentityCRD = &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "awsclusterstaticidentities.infrastructure.cluster.x-k8s.io",
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "infrastructure.cluster.x-k8s.io",
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{
						Name:    "v1beta2",
						Served:  true,
						Storage: true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"secretRef": {Type: "string"},
											"allowedNamespaces": {
												Type: "object",
												Properties: map[string]apiextensionsv1.JSONSchemaProps{
													"list": {
														Type: "array",
														Items: &apiextensionsv1.JSONSchemaPropsOrArray{
															Schema: &apiextensionsv1.JSONSchemaProps{
																Type: "string",
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Scope: apiextensionsv1.ClusterScoped,
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Plural:   "awsclusterstaticidentities",
					Singular: "awsclusterstaticidentity",
					Kind:     "AWSClusterStaticIdentity",
					ListKind: "AWSClusterStaticIdentityList",
				},
			},
		}
		if apierrors.IsNotFound(testEnv.Get(ctx, client.ObjectKeyFromObject(awsClusterStaticIdentityCRD), &apiextensionsv1.CustomResourceDefinition{})) {
			Expect(testEnv.Create(ctx, awsClusterStaticIdentityCRD)).ToNot(HaveOccurred())
		}

		globalDataNs := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: rancherCredentialsNamespace,
			},
		}
		if apierrors.IsNotFound(testEnv.Get(ctx, client.ObjectKeyFromObject(globalDataNs), &corev1.Namespace{})) {
			Expect(testEnv.Create(ctx, globalDataNs)).ToNot(HaveOccurred())
		}

		capaNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: r.Translators["aws"].ProviderNamespace(),
			},
		}
		if apierrors.IsNotFound(testEnv.Get(ctx, client.ObjectKeyFromObject(capaNs), &corev1.Namespace{})) {
			Expect(testEnv.Create(ctx, capaNs)).ToNot(HaveOccurred())
		}

		providerTemplate = &turtlesv1.CAPIProvider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.Translators["aws"].ProviderName(),
				Namespace: capaNs.Name,
			},
			Spec: turtlesv1.CAPIProviderSpec{
				Type: turtlesv1.Infrastructure,
			},
		}

		rancherAWSSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:         "cctest",
				GenerateName: "cc-",
				Namespace:    rancherCredentialsNamespace,
				Annotations: map[string]string{
					"field.cattle.io/name": "aws-cred",
					driverNameAnnotation:   "aws",
				},
			},
			StringData: map[string]string{
				rancherAWSAccessKeyField: "access-key",
				rancherAWSSecretKeyField: "secret-key",
			},
		}
		if apierrors.IsNotFound(testEnv.Get(ctx, client.ObjectKeyFromObject(rancherAWSSecret), &corev1.Secret{})) {
			Expect(testEnv.Create(ctx, rancherAWSSecret)).ToNot(HaveOccurred())
		}

		rancherAzureSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:         "cctest-azure",
				GenerateName: "cc-",
				Namespace:    rancherCredentialsNamespace,
			},
		}

		translatedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rancherAWSSecret.GetName(),
				Namespace: capaNs.Name,
			},
		}

		existingSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rancherAWSSecret.GetName(),
				Namespace: capaNs.Name,
			},
			StringData: map[string]string{
				"testData": "test-unchanged-data",
			},
		}

		translatedIdentity = &unstructured.Unstructured{}
		translatedIdentity.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "infrastructure.cluster.x-k8s.io",
			Version: "v1beta2",
			Kind:    "AWSClusterStaticIdentity",
		})
	})

	AfterEach(func() {
		providerKey := client.ObjectKey{
			Name:      r.Translators["aws"].ProviderName(),
			Namespace: r.Translators["aws"].ProviderNamespace(),
		}

		capaProvider := &turtlesv1.CAPIProvider{}
		Expect(client.IgnoreNotFound(cl.Get(ctx, providerKey, capaProvider))).NotTo(HaveOccurred())

		clientObjs := []client.Object{
			capaProvider,
			rancherAWSSecret,
			translatedSecret,
			existingSecret,
			translatedIdentity,
		}
		Expect(test.CleanupAndWait(ctx, cl, clientObjs...)).To(Succeed())
	})

	It("Should reconcile and translate Rancher Cloud Credential secret when CAPA is installed and becomes ready", func() {
		provider := providerTemplate.DeepCopy()

		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())

		// provider is not yet ready, controller should requeue.
		Eventually(ctx, func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.RequeueAfter).ToNot(BeZero())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, translatedSecret))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		patchBase := client.MergeFrom(provider.DeepCopy())
		provider.Status = turtlesv1.CAPIProviderStatus{
			Phase: turtlesv1.Ready,
		}

		err := cl.Status().Patch(ctx, provider, patchBase)
		Expect(err).NotTo(HaveOccurred())

		// provider is now ready, controller should translate credentials.
		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name: rancherAWSSecret.GetName(),
			}, translatedIdentity)).ToNot(HaveOccurred())

			g.Expect(translatedIdentity.GetAnnotations()).To(HaveKeyWithValue(
				cloudCredentialSecretAnnotation, string(rancherAWSSecret.UID)))

			secretRef, found, err := unstructured.NestedString(translatedIdentity.Object, "spec", "secretRef")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue(), "secretRef field should exist")

			g.Expect(secretRef).To(Equal(rancherAWSSecret.GetName()))

			allowedNs, found, err := unstructured.NestedMap(translatedIdentity.Object, "spec", "allowedNamespaces")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue(), "allowedNamespaces field should exist")

			Expect(allowedNs).To(HaveKeyWithValue(
				"list",
				ContainElement("fleet-default"),
			))

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, translatedSecret)).ToNot(HaveOccurred())

			g.Expect(translatedSecret.Data).To(HaveKeyWithValue(
				"AccessKeyID", rancherAWSSecret.Data[rancherAWSAccessKeyField]))
			g.Expect(translatedSecret.Data).To(HaveKeyWithValue(
				"SecretAccessKey", rancherAWSSecret.Data[rancherAWSSecretKeyField]))
			g.Expect(translatedSecret.GetAnnotations()).To(HaveKeyWithValue(
				cloudCredentialSecretAnnotation, string(rancherAWSSecret.UID)))

			updatedRancherSecret := &corev1.Secret{}
			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: rancherAWSSecret.GetNamespace(),
			}, updatedRancherSecret)).ToNot(HaveOccurred())

			g.Expect(updatedRancherSecret.GetAnnotations()[turtlesannotations.CAPIIdentityRefAnnotation]).To(Equal(translatedIdentity.GetName()))
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should delete translated CAPI identity when Rancher Cloud Credential is removed", func() {
		provider := providerTemplate.DeepCopy()

		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())
		patchBase := client.MergeFrom(provider.DeepCopy())
		provider.Status = turtlesv1.CAPIProviderStatus{
			Phase: turtlesv1.Ready,
		}

		err := cl.Status().Patch(ctx, provider, patchBase)
		Expect(err).NotTo(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name: rancherAWSSecret.GetName(),
			}, translatedIdentity)).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, translatedSecret)).ToNot(HaveOccurred())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		err = cl.Delete(ctx, rancherAWSSecret)
		Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name: rancherAWSSecret.GetName(),
			}, translatedIdentity))).To(BeTrue())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, translatedSecret))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should not delete existing secret when a collision is detected and Rancher Cloud Credential is removed", func() {
		provider := providerTemplate.DeepCopy()

		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())
		patchBase := client.MergeFrom(provider.DeepCopy())
		provider.Status = turtlesv1.CAPIProviderStatus{
			Phase: turtlesv1.Ready,
		}

		err := cl.Status().Patch(ctx, provider, patchBase)
		Expect(err).NotTo(HaveOccurred())

		Expect(cl.Create(ctx, existingSecret)).ToNot(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("collision detected"))
		}).WithTimeout(10 * time.Second).Should(Succeed())

		err = cl.Delete(ctx, rancherAWSSecret)
		Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			updatedRancherSecret := &corev1.Secret{}

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Namespace: rancherAWSSecret.Namespace,
				Name:      rancherAWSSecret.Name,
			}, updatedRancherSecret))).To(BeTrue())

			updatedExistingSecret := &corev1.Secret{}

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      existingSecret.GetName(),
				Namespace: existingSecret.GetNamespace(),
			}, updatedExistingSecret)).ToNot(HaveOccurred())
			g.Expect(updatedExistingSecret.Data).To(HaveKeyWithValue(
				"testData", []byte("test-unchanged-data")))

			g.Expect(updatedExistingSecret.GetAnnotations()).ToNot(HaveKey(cloudCredentialSecretAnnotation))
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should update AWSClusterStaticIdentity when AWS Cloud Credential changes", func() {
		provider := providerTemplate.DeepCopy()

		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())
		patchBase := client.MergeFrom(provider.DeepCopy())
		provider.Status = turtlesv1.CAPIProviderStatus{
			Phase: turtlesv1.Ready,
		}

		err := cl.Status().Patch(ctx, provider, patchBase)
		Expect(err).NotTo(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name: rancherAWSSecret.GetName(),
			}, translatedIdentity)).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, translatedSecret)).ToNot(HaveOccurred())

			g.Expect(translatedSecret.Data).To(HaveKeyWithValue(
				"AccessKeyID", rancherAWSSecret.Data[rancherAWSAccessKeyField]))
			g.Expect(translatedSecret.Data).To(HaveKeyWithValue(
				"SecretAccessKey", rancherAWSSecret.Data[rancherAWSSecretKeyField]))

			updatedRancherSecret := &corev1.Secret{}
			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: rancherAWSSecret.GetNamespace(),
			}, updatedRancherSecret)).ToNot(HaveOccurred())

			g.Expect(updatedRancherSecret.GetAnnotations()[turtlesannotations.CAPIIdentityRefAnnotation]).To(Equal(translatedIdentity.GetName()))
		}).WithTimeout(10 * time.Second).Should(Succeed())

		// update existing Cloud Credentials Secret
		updatedRancherSecret := &corev1.Secret{}
		Expect(cl.Get(ctx, types.NamespacedName{
			Name:      rancherAWSSecret.GetName(),
			Namespace: rancherAWSSecret.GetNamespace(),
		}, updatedRancherSecret)).ToNot(HaveOccurred())

		patchRancherSecret := client.MergeFrom(updatedRancherSecret.DeepCopy())

		updatedRancherSecret.StringData = map[string]string{
			rancherAWSAccessKeyField: "new-access-key",
			rancherAWSSecretKeyField: "new-secret-key",
		}

		Expect(cl.Patch(ctx, updatedRancherSecret, patchRancherSecret)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, translatedSecret)).ToNot(HaveOccurred())

			g.Expect(translatedSecret.Data).To(HaveKeyWithValue(
				"AccessKeyID", updatedRancherSecret.Data[rancherAWSAccessKeyField]))
			g.Expect(translatedSecret.Data).To(HaveKeyWithValue(
				"SecretAccessKey", updatedRancherSecret.Data[rancherAWSSecretKeyField]))
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should not translate Rancher Cloud Credential secret when a collision is detected (a secret with the same name already exists)", func() {
		provider := providerTemplate.DeepCopy()

		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())
		patchBase := client.MergeFrom(provider.DeepCopy())
		provider.Status = turtlesv1.CAPIProviderStatus{
			Phase: turtlesv1.Ready,
		}

		err := cl.Status().Patch(ctx, provider, patchBase)
		Expect(err).NotTo(HaveOccurred())

		Expect(cl.Create(ctx, existingSecret)).ToNot(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("collision detected"))

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name: rancherAWSSecret.GetName(),
			}, translatedIdentity))).To(BeTrue())

			updatedExistingSecret := &corev1.Secret{}

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      existingSecret.GetName(),
				Namespace: existingSecret.GetNamespace(),
			}, updatedExistingSecret)).ToNot(HaveOccurred())

			g.Expect(updatedExistingSecret.Data).To(HaveKeyWithValue(
				"testData", []byte("test-unchanged-data")))

			g.Expect(updatedExistingSecret.GetAnnotations()).ToNot(HaveKey(cloudCredentialSecretAnnotation))
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should not error when CAPA is not installed (Cloud Credentials are not translated) and not requeue", func() {
		capaProvider := &turtlesv1.CAPIProvider{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      r.Translators["aws"].ProviderName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, capaProvider))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		Eventually(ctx, func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.RequeueAfter).To(BeZero())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, translatedSecret))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should translate existing Cloud Credentials after CAPA CAPIProvider is installed", func() {
		// first we check that no translation happens when CAPA is not installed.
		// then we install CAPA and expect existing Cloud Credentials to be translated.
		capaProvider := &turtlesv1.CAPIProvider{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      r.Translators["aws"].ProviderName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, capaProvider))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		translatedSecret := &corev1.Secret{}

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, translatedSecret))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		provider := providerTemplate.DeepCopy()

		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())
		patchBase := client.MergeFrom(provider.DeepCopy())
		provider.Status = turtlesv1.CAPIProviderStatus{
			Phase: turtlesv1.Ready,
		}

		err := cl.Status().Patch(ctx, provider, patchBase)
		Expect(err).NotTo(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAWSSecret.Namespace,
					Name:      rancherAWSSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name: rancherAWSSecret.GetName(),
			}, translatedIdentity)).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAWSSecret.GetName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, translatedSecret)).ToNot(HaveOccurred())
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should not translate non-AWS Rancher Cloud Credential", func() {
		provider := providerTemplate.DeepCopy()

		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())
		patchBase := client.MergeFrom(provider.DeepCopy())
		provider.Status = turtlesv1.CAPIProviderStatus{
			Phase: turtlesv1.Ready,
		}

		err := cl.Status().Patch(ctx, provider, patchBase)
		Expect(err).NotTo(HaveOccurred())

		rancherAzureSecret.Annotations = map[string]string{
			"field.cattle.io/name": "azure-cred",
			driverNameAnnotation:   "azure",
		}
		rancherAzureSecret.StringData = map[string]string{
			"sampleazure": "sample-azure-key",
		}
		Expect(cl.Create(ctx, rancherAzureSecret)).ToNot(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherAzureSecret.Namespace,
					Name:      rancherAzureSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      rancherAzureSecret.GetName(),
				Namespace: r.Translators["aws"].ProviderNamespace(),
			}, translatedSecret))).To(BeTrue())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name: rancherAzureSecret.GetName(),
			}, translatedIdentity))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})
})
