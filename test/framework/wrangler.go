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

package framework

import (
	"context"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func VerifyCertificatesInNamespace(ctx context.Context, cl client.Client, namespace string) {
	Byf("Verifying no Certificates are used in namespace: %s", namespace)
	certs := schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Certificate",
	}

	certList := &unstructured.UnstructuredList{}
	certList.SetGroupVersionKind(certs)

	Expect(cl.List(ctx, certList, &client.ListOptions{Namespace: namespace})).Should(Succeed())
	Expect(certList.Items).Should(BeEmpty(), "cert-manager Certificates should not have been deployed")
}

func VerifyIssuersInNamespace(ctx context.Context, cl client.Client, namespace string) {
	Byf("Should verify no Issuers are used in namespace: %s", namespace)
	issuers := schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Issuer",
	}

	issuerList := &unstructured.UnstructuredList{}
	issuerList.SetGroupVersionKind(issuers)

	Expect(cl.List(ctx, issuerList, &client.ListOptions{Namespace: namespace})).Should(Succeed())
	Expect(issuerList.Items).Should(BeEmpty(), "cert-manager Issuers should not have been deployed")
}

func VerifyCertManagerAnnotationsForProvider(ctx context.Context, cl client.Client, providerName string) {
	Byf("Should verify cert-manager annotations have been removed for provider: %s", providerName)
	requirement, err := labels.NewRequirement("cluster.x-k8s.io/provider", selection.In, []string{providerName})
	Expect(err).ShouldNot(HaveOccurred())
	selector := client.MatchingLabelsSelector{
		Selector: labels.NewSelector().
			Add(*requirement),
	}

	cleanupKinds := []schema.GroupVersionKind{
		{
			Group:   "apiextensions.k8s.io",
			Version: "v1",
			Kind:    "CustomResourceDefinition",
		},
		{
			Group:   "admissionregistration.k8s.io",
			Version: "v1",
			Kind:    "MutatingWebhookConfiguration",
		},
		{
			Group:   "admissionregistration.k8s.io",
			Version: "v1",
			Kind:    "ValidatingWebhookConfiguration",
		},
	}

	for _, cleanupKind := range cleanupKinds {
		resourcesList := &unstructured.UnstructuredList{}
		resourcesList.SetGroupVersionKind(cleanupKind)

		Byf("Verifying %s resources do not contain cert-manager annotations", cleanupKind.Kind)
		Expect(cl.List(ctx, resourcesList, &client.ListOptions{LabelSelector: selector})).Should(Succeed())
		Expect(resourcesList.Items).ShouldNot(BeEmpty(), "Could not find any "+cleanupKind.Kind+" for the provider")

		for i := range resourcesList.Items {
			_, found := resourcesList.Items[i].GetAnnotations()["cert-manager.io/inject-ca-from"]
			Expect(found).Should(BeFalse(), "cert-manager annotation must be not found on: "+resourcesList.Items[i].GetName())
		}
	}
}

func VerifyWranglerAnnotationsInNamespace(ctx context.Context, cl client.Client, namespace string) {
	Byf("Should verify wrangler annotations have been added to Services in namespace: %s", namespace)
	servicesList := &corev1.ServiceList{}
	Expect(cl.List(ctx, servicesList, &client.ListOptions{Namespace: namespace})).Should(Succeed())
	Expect(servicesList.Items).ShouldNot(BeEmpty())

	for _, service := range servicesList.Items {
		Expect(service.GetAnnotations()).ShouldNot(BeEmpty())
		_, found := service.GetAnnotations()["need-a-cert.cattle.io/secret-name"]
		Expect(found).Should(BeTrue(), "need-a-cert.cattle.io/secret-name annotation must be found on Service: "+service.GetName())
	}
}
