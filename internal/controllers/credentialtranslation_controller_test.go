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
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Credential Translation", func() {
	var (
		r                           *RancherCredentialReconciler
		awsClusterStaticIdentityCRD *apiextensionsv1.CustomResourceDefinition
		provider                    *turtlesv1.CAPIProvider
		rancherSecret               *corev1.Secret
		translatedSecret            *corev1.Secret
		translatedIdentity          *unstructured.Unstructured
	)

	BeforeEach(func() {
		SetClient(testEnv)
		SetContext(ctx)

		r = &RancherCredentialReconciler{
			Client:              cl,
			CAPASystemNamespace: capaProviderNamespace,
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
											"secretName": {Type: "string"},
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

		capaNs := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: capaProviderNamespace,
			},
		}
		if apierrors.IsNotFound(testEnv.Get(ctx, client.ObjectKeyFromObject(capaNs), &corev1.Namespace{})) {
			Expect(testEnv.Create(ctx, capaNs)).ToNot(HaveOccurred())
		}

		provider = &turtlesv1.CAPIProvider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      capaProviderSpecName,
				Namespace: capaNs.Name,
			},
			Spec: turtlesv1.CAPIProviderSpec{
				Type: turtlesv1.Infrastructure,
			},
		}

		rancherSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:         "cctest",
				GenerateName: "cc-",
				Namespace:    rancherCredentialsNamespace,
			},
		}

		translatedSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rancherSecret.GetName(),
				Namespace: capaNs.Name,
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
		capaProvider := &turtlesv1.CAPIProvider{
			ObjectMeta: metav1.ObjectMeta{
				Name:      capaProviderSpecName,
				Namespace: capaProviderNamespace,
			},
		}

		err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			if err := cl.Get(ctx, client.ObjectKeyFromObject(capaProvider), capaProvider); err != nil {
				return client.IgnoreNotFound(err)
			}

			capaProvider.SetFinalizers(nil)

			return cl.Update(ctx, capaProvider)
		})
		Expect(err).NotTo(HaveOccurred())

		err = cl.Delete(ctx, capaProvider)
		Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

		clientObjs := []client.Object{
			rancherSecret,
			translatedSecret,
			translatedIdentity,
		}
		Expect(test.CleanupAndWait(ctx, cl, clientObjs...)).To(Succeed())
	})

	It("Should reconcile and translate Rancher Cloud Credential secret when CAPA is installed", func() {
		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())
		patchBase := client.MergeFrom(provider.DeepCopy())
		provider.Status = turtlesv1.CAPIProviderStatus{
			Phase: turtlesv1.Ready,
		}

		err := cl.Status().Patch(ctx, provider, patchBase)
		Expect(err).NotTo(HaveOccurred())

		rancherSecret.Annotations = map[string]string{
			"field.cattle.io/name": "aws-cred",
			driverNameAnnotation:   "aws",
		}
		rancherSecret.StringData = map[string]string{
			"amazonec2credentialConfig-accessKey":     "access-key",
			"amazonec2credentialConfig-secretKey":     "secret-key",
			"amazonec2credentialConfig-defaultRegion": "us-east-1",
		}
		Expect(cl.Create(ctx, rancherSecret)).ToNot(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherSecret.Namespace,
					Name:      rancherSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name: rancherSecret.GetName(),
			}, translatedIdentity)).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      rancherSecret.GetName(),
				Namespace: capaProviderNamespace,
			}, translatedSecret)).ToNot(HaveOccurred())
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should requeue and not translate when CAPA CAPIProvider is installed but not yet ready", func() {
		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())

		rancherSecret.Annotations = map[string]string{
			"field.cattle.io/name": "aws-cred",
			driverNameAnnotation:   "aws",
		}
		rancherSecret.StringData = map[string]string{
			"amazonec2credentialConfig-accessKey":     "access-key",
			"amazonec2credentialConfig-secretKey":     "secret-key",
			"amazonec2credentialConfig-defaultRegion": "us-east-1",
		}
		Expect(cl.Create(ctx, rancherSecret)).ToNot(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherSecret.Namespace,
					Name:      rancherSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.RequeueAfter).ToNot(BeZero())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      rancherSecret.GetName(),
				Namespace: capaProviderNamespace,
			}, translatedSecret))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should not error when CAPA is not installed (Cloud Credentials are not translated) and not requeue", func() {
		capaProvider := &turtlesv1.CAPIProvider{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      capaProviderSpecName,
				Namespace: capaProviderNamespace,
			}, capaProvider))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		rancherSecret.Annotations = map[string]string{
			"field.cattle.io/name": "aws-cred",
			driverNameAnnotation:   "aws",
		}
		rancherSecret.StringData = map[string]string{
			"amazonec2credentialConfig-accessKey":     "access-key",
			"amazonec2credentialConfig-secretKey":     "secret-key",
			"amazonec2credentialConfig-defaultRegion": "us-east-1",
		}
		Expect(cl.Create(ctx, rancherSecret)).ToNot(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherSecret.Namespace,
					Name:      rancherSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.RequeueAfter).To(BeZero())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      rancherSecret.GetName(),
				Namespace: capaProviderNamespace,
			}, translatedSecret))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should translate existing Cloud Credentials after CAPA CAPIProvider is installed", func() {
		// first we check that no translation happens when CAPA is not installed.
		// then we install CAPA and expect existing Cloud Credentials to be translated.
		capaProvider := &turtlesv1.CAPIProvider{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      capaProviderSpecName,
				Namespace: capaProviderNamespace,
			}, capaProvider))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())

		rancherSecret.Annotations = map[string]string{
			"field.cattle.io/name": "aws-cred",
			driverNameAnnotation:   "aws",
		}
		rancherSecret.StringData = map[string]string{
			"amazonec2credentialConfig-accessKey":     "access-key",
			"amazonec2credentialConfig-secretKey":     "secret-key",
			"amazonec2credentialConfig-defaultRegion": "us-east-1",
		}
		Expect(cl.Create(ctx, rancherSecret)).ToNot(HaveOccurred())

		translatedSecret := &corev1.Secret{}

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherSecret.Namespace,
					Name:      rancherSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name:      rancherSecret.GetName(),
				Namespace: capaProviderNamespace,
			}, translatedSecret))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())

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
					Namespace: rancherSecret.Namespace,
					Name:      rancherSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name: rancherSecret.GetName(),
			}, translatedIdentity)).ToNot(HaveOccurred())

			g.Expect(cl.Get(ctx, types.NamespacedName{
				Name:      rancherSecret.GetName(),
				Namespace: capaProviderNamespace,
			}, translatedSecret)).ToNot(HaveOccurred())
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})

	It("Should not translate non-AWS Rancher Cloud Credential", func() {
		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())
		patchBase := client.MergeFrom(provider.DeepCopy())
		provider.Status = turtlesv1.CAPIProviderStatus{
			Phase: turtlesv1.Ready,
		}

		err := cl.Status().Patch(ctx, provider, patchBase)
		Expect(err).NotTo(HaveOccurred())

		rancherSecret.Annotations = map[string]string{
			"field.cattle.io/name": "azure-cred",
			driverNameAnnotation:   "azure",
		}
		rancherSecret.StringData = map[string]string{
			"sampleazure": "sample-azure-key",
		}
		Expect(cl.Create(ctx, rancherSecret)).ToNot(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: rancherSecret.Namespace,
					Name:      rancherSecret.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(apierrors.IsNotFound(cl.Get(ctx, types.NamespacedName{
				Name: rancherSecret.GetName(),
			}, translatedIdentity))).To(BeTrue())
		}).WithTimeout(10 * time.Second).Should(Succeed())
	})
})
