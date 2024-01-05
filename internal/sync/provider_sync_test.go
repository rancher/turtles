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
	"k8s.io/utils/pointer"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
)

var _ = Describe("Provider sync", func() {
	var (
		err            error
		ns             *corev1.Namespace
		capiProvider   *turtlesv1.CAPIProvider
		infrastructure *operatorv1.InfrastructureProvider
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
			Type: turtlesv1.Infrastructure,
		}}

		infrastructure = &operatorv1.InfrastructureProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      string(capiProvider.Spec.Name),
			Namespace: capiProvider.Namespace,
		}}

		Expect(testEnv.Client.Create(ctx, capiProvider)).To(Succeed())
	})

	AfterEach(func() {
		testEnv.Cleanup(ctx, ns)
	})

	It("Should sync spec down", func() {
		s := sync.NewProviderSync(testEnv, capiProvider.DeepCopy())
		Eventually(s.Get(ctx)).Should(Succeed())

		Expect(s.Sync(ctx)).To(Succeed())

		var err error
		s.Apply(ctx, &err)
		Expect(err).To(Succeed())

		Eventually(Object(infrastructure)).Should(
			HaveField("Spec.ProviderSpec", Equal(capiProvider.Spec.ProviderSpec)))
	})

	It("Should sync status up", func() {
		Expect(testEnv.Client.Create(ctx, infrastructure.DeepCopy())).To(Succeed())
		Eventually(UpdateStatus(infrastructure, func() {
			infrastructure.Status = operatorv1.InfrastructureProviderStatus{
				ProviderStatus: operatorv1.ProviderStatus{
					InstalledVersion: pointer.String("v1.2.3"),
				},
			}
		})).Should(Succeed())

		s := sync.NewProviderSync(testEnv, capiProvider)

		Eventually(func() (err error) {
			err = s.Get(ctx)
			if err != nil {
				return
			}
			err = s.Sync(ctx)
			s.Apply(ctx, &err)
			return
		}).Should(Succeed())

		Expect(capiProvider).To(HaveField("Status.ProviderStatus", Equal(infrastructure.Status.ProviderStatus)))
		Expect(capiProvider).To(HaveField("Status.Phase", Equal(turtlesv1.Provisioning)))
	})
})
