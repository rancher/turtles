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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("AddClusterIndexedLabelFn function", func() {
	It("Should add cluster indexed label to CRDs", func() {
		crd1 := unstructured.Unstructured{}
		crd1.SetKind("CustomResourceDefinition")
		crd1.SetName("test")

		crd2 := unstructured.Unstructured{}
		crd2.SetKind("CustomResourceDefinition")
		crd2.SetName("test")

		svc := unstructured.Unstructured{}
		svc.SetKind("Service")
		svc.SetName("ignored")

		alteredComponents, err := AddClusterIndexedLabelFn([]unstructured.Unstructured{crd1, crd2, svc})
		Expect(err).ToNot(HaveOccurred())
		Expect(alteredComponents).To(HaveLen(3))
		Expect(alteredComponents[0].GetLabels()[clusterIndexedLabelKey]).To(Equal("true"))
		Expect(alteredComponents[1].GetLabels()[clusterIndexedLabelKey]).To(Equal("true"))
		Expect(alteredComponents[2].GetLabels()).NotTo(HaveKey(clusterIndexedLabelKey))
	})

	It("Should handle empty input slices", func() {
		alteredComponents, err := AddClusterIndexedLabelFn(nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(alteredComponents).To(BeNil())
	})
})
