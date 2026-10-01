/*
Copyright © 2023 - 2025 SUSE LLC

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

package provider

import (
	"encoding/base64"
	"errors"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	runtimev1 "sigs.k8s.io/cluster-api/api/runtime/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
)

var _ = Describe("Alter component functions", func() {
	It("Should patch provider manifest with certificate secret annotation on service", func() {
		cert := unstructured.Unstructured{}
		cert.SetKind("Certificate")
		cert.SetName("test")
		cert.SetNamespace("test")
		Expect(unstructured.SetNestedField(cert.Object, "my-cert-secret", "spec", "secretName")).ToNot(HaveOccurred())

		svc := unstructured.Unstructured{}
		svc.SetKind("Service")
		svc.SetName("test")
		svc.SetNamespace("test")

		webhook := &admissionv1.MutatingWebhookConfiguration{
			ObjectMeta: v1.ObjectMeta{
				Name:        "test",
				Namespace:   "test",
				Annotations: map[string]string{CertManagerInjectAnnotationKey: "test/test"},
			},
			Webhooks: []admissionv1.MutatingWebhook{
				{
					Name: "test",
					ClientConfig: admissionv1.WebhookClientConfig{
						Service: &admissionv1.ServiceReference{
							Name:      svc.GetName(),
							Namespace: svc.GetNamespace(),
						},
					},
				},
			},
		}

		var err error
		unstructuredWebhook := &unstructured.Unstructured{}
		unstructuredWebhook.Object, err = runtime.DefaultUnstructuredConverter.ToUnstructured(webhook)
		Expect(err).ShouldNot(HaveOccurred())
		unstructuredWebhook.SetKind("MutatingWebhookConfiguration")

		alteredComponents, err := WranglerPatcher([]unstructured.Unstructured{svc, cert, *unstructuredWebhook})
		Expect(err).ToNot(HaveOccurred())
		Expect(alteredComponents).To(HaveLen(2))
		Expect(alteredComponents[0].GetKind()).To(Equal("Service"))
		needACertAnnotation, found := alteredComponents[0].GetAnnotations()[CertificateAnnotationKey]
		Expect(found).Should(BeTrue(), "need-a-cert annotation must be set on Service")
		Expect(needACertAnnotation).To(Equal("my-cert-secret"))
	})

	It("Should patch the Service referenced by an ExtensionConfig and drop the runtime inject annotation", func() {
		cert := unstructured.Unstructured{}
		cert.SetKind("Certificate")
		cert.SetName("serving-cert")
		cert.SetNamespace("test")
		Expect(unstructured.SetNestedField(cert.Object, "webhook-service-cert", "spec", "secretName")).ToNot(HaveOccurred())

		svc := unstructured.Unstructured{}
		svc.SetKind("Service")
		svc.SetName("webhook-service")
		svc.SetNamespace("test")

		extensionConfig := unstructured.Unstructured{}
		extensionConfig.SetKind(ExtensionConfigKind)
		extensionConfig.SetName("test")
		extensionConfig.SetAnnotations(map[string]string{RuntimeInjectCAFromSecretAnnotationKey: "test/webhook-service-cert"})
		Expect(unstructured.SetNestedMap(extensionConfig.Object, map[string]interface{}{
			"name":      "webhook-service",
			"namespace": "test",
		}, "spec", "clientConfig", "service")).ToNot(HaveOccurred())

		alteredComponents, err := WranglerPatcher([]unstructured.Unstructured{svc, cert, extensionConfig})
		Expect(err).ToNot(HaveOccurred())
		Expect(alteredComponents).To(HaveLen(2))
		Expect(alteredComponents[0].GetKind()).To(Equal("Service"))
		Expect(alteredComponents[0].GetAnnotations()).To(HaveKeyWithValue(CertificateAnnotationKey, "webhook-service-cert"))
		Expect(alteredComponents[1].GetKind()).To(Equal(ExtensionConfigKind))
		Expect(alteredComponents[1].GetAnnotations()).NotTo(HaveKey(RuntimeInjectCAFromSecretAnnotationKey))
	})

	It("Should fail when the ExtensionConfig inject-ca-from-secret reference is not <namespace>/<name>", func() {
		svc := unstructured.Unstructured{}
		svc.SetKind("Service")
		svc.SetName("webhook-service")
		svc.SetNamespace("test")

		for _, reference := range []string{"/webhook-service-cert", "test/", "webhook-service-cert", "one/two/three/four/five"} {
			extensionConfig := unstructured.Unstructured{}
			extensionConfig.SetKind(ExtensionConfigKind)
			extensionConfig.SetName("test")
			extensionConfig.SetAnnotations(map[string]string{RuntimeInjectCAFromSecretAnnotationKey: reference})
			Expect(unstructured.SetNestedMap(extensionConfig.Object, map[string]interface{}{
				"name":      "webhook-service",
				"namespace": "test",
			}, "spec", "clientConfig", "service")).ToNot(HaveOccurred())

			_, err := WranglerPatcher([]unstructured.Unstructured{svc, extensionConfig})
			Expect(err).To(MatchError(ContainSubstring(RuntimeInjectCAFromSecretAnnotationKey)), "reference %q", reference)
		}
	})

	It("Should fail when Certificate secretName is missing", func() {
		cert := unstructured.Unstructured{}
		cert.SetKind("Certificate")
		cert.SetName("test")

		_, err := WranglerPatcher([]unstructured.Unstructured{cert})
		Expect(err).Should(HaveOccurred())
		Expect(errors.Is(err, ErrNoCertificateSecret)).Should(BeTrue())
	})

	It("Should remove cert-manager resources and annotations", func() {
		cert := unstructured.Unstructured{}
		cert.SetKind("Certificate")
		cert.SetName("test")
		cert.SetNamespace("test")
		Expect(unstructured.SetNestedField(cert.Object, "my-cert-secret", "spec", "secretName")).ToNot(HaveOccurred())
		issuer := unstructured.Unstructured{}
		issuer.SetKind("Issuer")
		issuer.SetName("test")
		issuer.SetNamespace("test")
		deploy := unstructured.Unstructured{}
		deploy.SetKind("Deployment")
		deploy.SetName("test")
		deploy.SetNamespace("test")
		deploy.SetAnnotations(map[string]string{CertManagerInjectAnnotationKey: "test/test"})
		svc := unstructured.Unstructured{}
		svc.SetKind("Service")
		svc.SetName("test")
		svc.SetNamespace("test")
		svc.SetAnnotations(map[string]string{CertManagerInjectAnnotationKey: "test/test"})

		alteredComponents, err := WranglerPatcher([]unstructured.Unstructured{cert, issuer, deploy, svc})
		Expect(err).ToNot(HaveOccurred())
		Expect(alteredComponents).To(HaveLen(2))
		Expect(alteredComponents[0].GetKind()).ToNot(Or(Equal("Certificate"), Equal("Issuer")))
		Expect(alteredComponents[1].GetKind()).ToNot(Or(Equal("Certificate"), Equal("Issuer")))
		if alteredComponents[0].GetAnnotations() != nil {
			Expect(alteredComponents[0].GetAnnotations()).NotTo(HaveKey(CertManagerInjectAnnotationKey))
		}
		if alteredComponents[1].GetAnnotations() != nil {
			Expect(alteredComponents[1].GetAnnotations()).NotTo(HaveKey(CertManagerInjectAnnotationKey))
		}
	})

	It("Should handle empty input slices", func() {
		alteredComponents, err := WranglerPatcher(nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(alteredComponents).To(BeNil())

		alteredComponents, err = WranglerPatcher(nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(alteredComponents).To(BeNil())
	})

	It("Should ignore manifests without Service or Certificate", func() {
		pod := unstructured.Unstructured{}
		pod.SetKind("Pod")
		pod.SetName("test")

		alteredComponents, err := WranglerPatcher([]unstructured.Unstructured{pod})
		Expect(err).ToNot(HaveOccurred())
		Expect(alteredComponents).To(HaveLen(1))
	})
})

var _ = Describe("Inject ExtensionConfig CA bundle", func() {
	var (
		extensionScheme *runtime.Scheme
		provider        *turtlesv1.CAPIProvider
		extensionConfig *unstructured.Unstructured
		service         *corev1.Service
		secret          *corev1.Secret
	)

	newExtensionConfig := func(name, providerLabel string) *unstructured.Unstructured {
		extensionConfig := &unstructured.Unstructured{}
		extensionConfig.SetGroupVersionKind(runtimev1.GroupVersion.WithKind(ExtensionConfigKind))
		extensionConfig.SetName(name)
		extensionConfig.SetLabels(map[string]string{CAPIProviderLabel: providerLabel})
		Expect(unstructured.SetNestedMap(extensionConfig.Object, map[string]interface{}{
			"name":      "webhook-service",
			"namespace": "rke2-extension-system",
		}, "spec", "clientConfig", "service")).ToNot(HaveOccurred())

		return extensionConfig
	}

	getExtensionConfig := func(cl client.Client, name string) *unstructured.Unstructured {
		extensionConfig := &unstructured.Unstructured{}
		extensionConfig.SetGroupVersionKind(runtimev1.GroupVersion.WithKind(ExtensionConfigKind))
		Expect(cl.Get(ctx, client.ObjectKey{Name: name}, extensionConfig)).ToNot(HaveOccurred())

		return extensionConfig
	}

	getCABundle := func(cl client.Client, name string) string {
		caBundle, _, err := unstructured.NestedString(getExtensionConfig(cl, name).Object, "spec", "clientConfig", "caBundle")
		Expect(err).ToNot(HaveOccurred())

		return caBundle
	}

	BeforeEach(func() {
		extensionScheme = runtime.NewScheme()
		Expect(clientgoscheme.AddToScheme(extensionScheme)).ToNot(HaveOccurred())
		Expect(runtimev1.AddToScheme(extensionScheme)).ToNot(HaveOccurred())

		provider = &turtlesv1.CAPIProvider{
			ObjectMeta: v1.ObjectMeta{Name: "rke2-extension", Namespace: "rke2-extension-system"},
			Spec:       turtlesv1.CAPIProviderSpec{Name: "rke2", Type: turtlesv1.RuntimeExtension},
		}

		extensionConfig = newExtensionConfig("rke2-extension-provider", "runtime-extension-rke2")

		service = &corev1.Service{ObjectMeta: v1.ObjectMeta{
			Name:        "webhook-service",
			Namespace:   "rke2-extension-system",
			Annotations: map[string]string{CertificateAnnotationKey: "webhook-service-cert"},
		}}

		secret = &corev1.Secret{
			ObjectMeta: v1.ObjectMeta{Name: "webhook-service-cert", Namespace: "rke2-extension-system"},
			Data:       map[string][]byte{corev1.TLSCertKey: []byte("test-certificate")},
		}
	})

	It("Should inject the base64 encoded wrangler certificate into the ExtensionConfig caBundle", func() {
		cl := fake.NewClientBuilder().WithScheme(extensionScheme).WithObjects(extensionConfig, service, secret).Build()

		res, err := InjectExtensionConfigCABundle(ctx, cl, provider)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeZero())
		Expect(getCABundle(cl, extensionConfig.GetName())).To(Equal(base64.StdEncoding.EncodeToString([]byte("test-certificate"))))
	})

	It("Should requeue until wrangler generates the certificate Secret", func() {
		cl := fake.NewClientBuilder().WithScheme(extensionScheme).WithObjects(extensionConfig, service).Build()

		res, err := InjectExtensionConfigCABundle(ctx, cl, provider)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(Equal(10 * time.Second))
		Expect(getCABundle(cl, extensionConfig.GetName())).To(BeEmpty())
	})

	It("Should not update the ExtensionConfig when the caBundle is up to date", func() {
		cl := fake.NewClientBuilder().WithScheme(extensionScheme).WithObjects(extensionConfig, service, secret).Build()

		_, err := InjectExtensionConfigCABundle(ctx, cl, provider)
		Expect(err).ToNot(HaveOccurred())
		resourceVersion := getExtensionConfig(cl, extensionConfig.GetName()).GetResourceVersion()

		_, err = InjectExtensionConfigCABundle(ctx, cl, provider)
		Expect(err).ToNot(HaveOccurred())
		Expect(getExtensionConfig(cl, extensionConfig.GetName()).GetResourceVersion()).To(Equal(resourceVersion))
	})

	It("Should ignore ExtensionConfigs that belong to other providers", func() {
		otherExtensionConfig := newExtensionConfig("other-extension-provider", "runtime-extension-other")
		cl := fake.NewClientBuilder().WithScheme(extensionScheme).WithObjects(extensionConfig, otherExtensionConfig, service, secret).Build()

		_, err := InjectExtensionConfigCABundle(ctx, cl, provider)
		Expect(err).ToNot(HaveOccurred())
		Expect(getCABundle(cl, extensionConfig.GetName())).ToNot(BeEmpty())
		Expect(getCABundle(cl, otherExtensionConfig.GetName())).To(BeEmpty())
	})

	It("Should skip providers that are not runtime extensions", func() {
		provider.Spec.Type = turtlesv1.ControlPlane
		cl := fake.NewClientBuilder().WithScheme(extensionScheme).WithObjects(extensionConfig, service, secret).Build()

		res, err := InjectExtensionConfigCABundle(ctx, cl, provider)
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(BeZero())
		Expect(getCABundle(cl, extensionConfig.GetName())).To(BeEmpty())
	})
})
