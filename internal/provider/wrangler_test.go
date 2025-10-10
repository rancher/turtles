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
	"errors"

	admissionv1 "k8s.io/api/admissionregistration/v1beta1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
