/*
Copyright © 2023 - 2024 SUSE LLC

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

package sync_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesv1 "github.com/rancher-sandbox/rancher-turtles/api/v1alpha1"
	"github.com/rancher-sandbox/rancher-turtles/internal/sync"
	corev1 "k8s.io/api/core/v1"

	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Core provider", func() {
	var (
		err          error
		ns           *corev1.Namespace
		capiProvider *turtlesv1.CAPIProvider
	)

	BeforeEach(func() {
		SetClient(testEnv)
		SetContext(ctx)

		ns, err = testEnv.CreateNamespace(ctx, "ns")
		Expect(err).ToNot(HaveOccurred())

		capiProvider = &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Name: "docker",
			Type: turtlesv1.Core,
		}}

		Expect(testEnv.Client.Create(ctx, capiProvider)).To(Succeed())
	})

	AfterEach(func() {
		testEnv.Cleanup(ctx, ns)
	})

	It("Status patch only updates the status of the resource", func() {
		capiProvider.Spec.Name = "rke2"
		capiProvider.Status.Phase = turtlesv1.Ready

		Expect(sync.PatchStatus(ctx, testEnv, capiProvider)).To(Succeed())
		Eventually(Object(capiProvider)).Should(HaveField("Status.Phase", Equal(turtlesv1.Ready)))
		Eventually(Object(capiProvider)).Should(HaveField("Spec.Name", Equal("docker")))
	})

	It("Regular patch only updates the spec of the resource", func() {
		capiProvider.Spec.Name = "rke2"
		capiProvider.Status.Phase = turtlesv1.Ready

		Expect(sync.Patch(ctx, testEnv, capiProvider)).To(Succeed())
		Eventually(Object(capiProvider)).Should(HaveField("Spec.Name", Equal("rke2")))
		Eventually(Object(capiProvider)).Should(HaveField("Status.Phase", BeEmpty()))
	})
})
