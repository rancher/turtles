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
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/sync"
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

	It("Should repetively sync any object to the cluster", func() {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
		}}
		s := sync.NewDefaultSynchronizer(testEnv, capiProvider, secret)
		Expect(s.Get(ctx)).To(Succeed())

		secret.StringData = map[string]string{
			"something": "added",
		}

		var err error
		s.Apply(ctx, &err)
		Expect(err).To(Succeed())

		Eventually(Object(secret)).Should(
			HaveField("Data", HaveKey("something")))
		Eventually(Object(secret)).Should(
			HaveField("OwnerReferences", HaveLen(1)))

		Expect(s.Get(ctx)).To(Succeed())
		secret.StringData = map[string]string{
			"removed": "something",
		}
		s.Apply(ctx, &err)
		Expect(err).To(Succeed())

		Eventually(Object(secret)).Should(
			HaveField("Data", HaveKey("removed")))
		Eventually(Object(secret)).Should(
			HaveField("OwnerReferences", HaveLen(1)))
	})
})
