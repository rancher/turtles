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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/sync"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
)

var _ = Describe("Provider sync", func() {
	var (
		err                          error
		ns                           *corev1.Namespace
		otherNs                      *corev1.Namespace
		capiProvider                 *turtlesv1.CAPIProvider
		capiProviderFollowLatest     *turtlesv1.CAPIProvider
		capiProviderAzure            *turtlesv1.CAPIProvider
		capiProviderDuplicate        *turtlesv1.CAPIProvider
		infrastructure               *operatorv1.InfrastructureProvider
		infrastructureFollowLatest   *operatorv1.InfrastructureProvider
		infrastructureStatusOutdated operatorv1.ProviderStatus
		infrastructureDuplicate      *operatorv1.InfrastructureProvider
		infrastructureAzure          *operatorv1.InfrastructureProvider
	)

	BeforeEach(func() {
		SetClient(testEnv)
		SetContext(ctx)

		ns, err = testEnv.CreateNamespace(ctx, "ns")
		Expect(err).ToNot(HaveOccurred())

		otherNs, err = testEnv.CreateNamespace(ctx, "other")
		Expect(err).ToNot(HaveOccurred())

		capiProvider = &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Name: "docker",
			Type: turtlesv1.Infrastructure,
		}}

		capiProviderAzure = capiProvider.DeepCopy()
		capiProviderAzure.Spec.Name = "azure"
		capiProviderAzure.Name = "azure"

		capiProviderDuplicate = capiProvider.DeepCopy()
		capiProviderDuplicate.Namespace = otherNs.Name

		capiProviderFollowLatest = capiProvider.DeepCopy()
		capiProviderFollowLatest.Name = "latest"
		capiProviderFollowLatest.Spec.FollowLatest = true

		infrastructure = &operatorv1.InfrastructureProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      string(capiProvider.Spec.Name),
			Namespace: ns.Name,
		}}

		infrastructureAzure = &operatorv1.InfrastructureProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      string(capiProviderAzure.Spec.Name),
			Namespace: ns.Name,
		}}

		infrastructureDuplicate = &operatorv1.InfrastructureProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      string(capiProvider.Spec.Name),
			Namespace: otherNs.Name,
		}}

		infrastructureFollowLatest = &operatorv1.InfrastructureProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      string(capiProviderFollowLatest.Spec.Name),
			Namespace: otherNs.Name,
		}}

		infrastructureStatusOutdated = operatorv1.ProviderStatus{
			Conditions: clusterv1.Conditions{
				{
					Type:               turtlesv1.CheckLatestVersionTime,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second).Add(-23 * time.Hour)),
				},
				{
					Type:               turtlesv1.LastAppliedConfigurationTime,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: metav1.NewTime(time.Now().UTC().Truncate(time.Second).Add(-24 * 100 * time.Hour)),
				},
			},
		}

		Expect(testEnv.Client.Create(ctx, capiProviderFollowLatest)).To(Succeed())
		Expect(testEnv.Client.Create(ctx, capiProvider)).To(Succeed())
		Expect(testEnv.Client.Create(ctx, capiProviderDuplicate)).To(Succeed())
		Expect(testEnv.Client.Create(ctx, capiProviderAzure)).To(Succeed())
	})

	AfterEach(func() {
		testEnv.Cleanup(ctx, ns, otherNs)
	})

	It("Should sync spec down", func() {
		s := sync.NewProviderSync(testEnv, capiProvider.DeepCopy())

		Eventually(func(g Gomega) {
			g.Expect(s.Get(ctx)).To(Succeed())
			g.Expect(s.Sync(ctx)).To(Succeed())
			var err error = nil
			s.Apply(ctx, &err)
			g.Expect(err).To(Succeed())
		}).Should(Succeed())

		Eventually(Object(infrastructure)).Should(
			HaveField("Spec.ProviderSpec", Equal(capiProvider.Spec.ProviderSpec)))
	})

	It("Should sync azure spec", func() {
		s := sync.NewAzureProviderSync(testEnv, capiProviderAzure)

		Eventually(func(g Gomega) {
			g.Expect(s.Get(ctx)).To(Succeed())
			g.Expect(s.Sync(ctx)).To(Succeed())
			var err error = nil
			s.Apply(ctx, &err)
			Expect(err).To(Succeed())
		}).Should(Succeed())

		capiProviderAzure.Spec.Deployment = &operatorv1.DeploymentSpec{
			Containers: []operatorv1.ContainerSpec{{
				Name: "manager",
				Args: map[string]string{
					"--bootstrap-config-gvk": "RKE2Config.v1beta1.bootstrap.cluster.x-k8s.io",
				},
			}},
		}

		Eventually(Object(infrastructureAzure)).Should(
			HaveField("Spec.ProviderSpec", Equal(capiProviderAzure.Spec.ProviderSpec)))
	})

	It("Should sync status up and set provisioning state", func() {
		Expect(testEnv.Client.Create(ctx, infrastructure.DeepCopy())).To(Succeed())
		Eventually(UpdateStatus(infrastructure, func() {
			infrastructure.Status = operatorv1.InfrastructureProviderStatus{
				ProviderStatus: operatorv1.ProviderStatus{
					InstalledVersion: ptr.To("v1.2.3"),
				},
			}
		})).Should(Succeed())

		s := sync.NewProviderSync(testEnv, capiProvider)

		Eventually(func(g Gomega) {
			err = nil
			g.Expect(s.Get(ctx)).To(Succeed())
			g.Expect(s.Sync(ctx)).To(Succeed())
			s.Apply(ctx, &err)
			g.Expect(conditions.IsTrue(capiProvider, turtlesv1.LastAppliedConfigurationTime)).To(BeTrue())
			g.Expect(capiProvider.Status.Conditions).To(HaveLen(1))
			g.Expect(capiProvider).To(HaveField("Status.Phase", Equal(turtlesv1.Provisioning)))
		}).Should(Succeed())
	})

	It("Should update outdated condition, maintain last applied time and empty the hash annotation", func() {
		capiProviderFollowLatest.Status.ProviderStatus = infrastructureStatusOutdated

		appliedCondition := conditions.Get(capiProviderFollowLatest, turtlesv1.LastAppliedConfigurationTime)
		lastVersionCheckCondition := conditions.Get(capiProviderFollowLatest, turtlesv1.CheckLatestVersionTime)

		Eventually(testEnv.Status().Update(ctx, capiProviderFollowLatest)).Should(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(capiProviderFollowLatest), capiProviderFollowLatest)).To(Succeed())
			g.Expect(conditions.Get(capiProviderFollowLatest, turtlesv1.LastAppliedConfigurationTime)).ToNot(BeNil())
			g.Expect(conditions.Get(capiProviderFollowLatest, turtlesv1.LastAppliedConfigurationTime).LastTransitionTime.Second()).To(Equal(appliedCondition.LastTransitionTime.Second()))
		}).Should(Succeed())

		s := sync.NewProviderSync(testEnv, capiProviderFollowLatest)

		dest := &operatorv1.InfrastructureProvider{}
		Eventually(func(g Gomega) {
			err = nil
			g.Expect(s.Get(ctx)).To(Succeed())
			g.Expect(s.Sync(ctx)).To(Succeed())
			s.Apply(ctx, &err)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(infrastructure), dest)).To(Succeed())
			g.Expect(dest.GetAnnotations()).To(HaveKeyWithValue(sync.AppliedSpecHashAnnotation, ""))
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(infrastructure), dest)).To(Succeed())
			g.Expect(capiProviderFollowLatest.Status.Conditions).To(HaveLen(2))
			g.Expect(conditions.IsTrue(capiProviderFollowLatest, turtlesv1.LastAppliedConfigurationTime)).To(BeTrue())
			g.Expect(conditions.IsTrue(capiProviderFollowLatest, turtlesv1.CheckLatestVersionTime)).To(BeTrue())
			g.Expect(conditions.Get(capiProviderFollowLatest, turtlesv1.CheckLatestVersionTime).LastTransitionTime.Equal(
				&lastVersionCheckCondition.LastTransitionTime)).To(BeTrue())
			g.Expect(conditions.Get(capiProviderFollowLatest, turtlesv1.LastAppliedConfigurationTime).LastTransitionTime.After(
				appliedCondition.LastTransitionTime.Time)).To(BeTrue())
		}).Should(Succeed())

		Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(capiProviderFollowLatest), capiProviderFollowLatest)).To(Succeed())
		condition := conditions.Get(capiProviderFollowLatest, turtlesv1.LastAppliedConfigurationTime)
		lastVersionCheckCondition = conditions.Get(capiProviderFollowLatest, turtlesv1.CheckLatestVersionTime)

		Consistently(func(g Gomega) {
			err = nil
			g.Expect(s.Get(ctx)).To(Succeed())
			g.Expect(s.Sync(ctx)).To(Succeed())
			s.Apply(ctx, &err)
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(capiProviderFollowLatest), capiProviderFollowLatest)).To(Succeed())
			g.Expect(conditions.Get(capiProviderFollowLatest, turtlesv1.LastAppliedConfigurationTime)).To(Equal(condition))
			g.Expect(conditions.Get(capiProviderFollowLatest, turtlesv1.CheckLatestVersionTime)).To(Equal(lastVersionCheckCondition))
		}, 5*time.Second).Should(Succeed())
	})

	It("Should set the last applied version check condition and empty the version field", func() {
		s := sync.NewProviderSync(testEnv, capiProviderFollowLatest)

		infrastructureFollowLatest.Spec.Version = "v1.2.3"

		Expect(testEnv.Create(ctx, infrastructureFollowLatest)).To(Succeed())

		Eventually(func(g Gomega) {
			err = nil
			g.Expect(s.Get(ctx)).To(Succeed())
			g.Expect(s.Sync(ctx)).To(Succeed())
			s.Apply(ctx, &err)
			g.Expect(conditions.IsTrue(capiProviderFollowLatest, turtlesv1.LastAppliedConfigurationTime)).To(BeTrue())
			g.Expect(conditions.IsTrue(capiProviderFollowLatest, turtlesv1.CheckLatestVersionTime)).To(BeTrue())
			g.Expect(capiProviderFollowLatest.Status.Conditions).To(HaveLen(2))
		}, 5*time.Second).Should(Succeed())
	})

	It("Should not set the last applied version check, if the FollowLatest is not set", func() {
		s := sync.NewProviderSync(testEnv, capiProvider)

		Consistently(func(g Gomega) {
			err = nil
			g.Expect(s.Get(ctx)).To(Succeed())
			g.Expect(s.Sync(ctx)).To(Succeed())
			s.Apply(ctx, &err)
			g.Expect(conditions.IsTrue(capiProvider, turtlesv1.LastAppliedConfigurationTime)).To(BeTrue())
			g.Expect(capiProvider.Status.Conditions).To(HaveLen(1))
		}).Should(Succeed())
	})

	It("Should individually sync every provider", func() {
		Expect(testEnv.Client.Create(ctx, infrastructure.DeepCopy())).To(Succeed())
		Eventually(UpdateStatus(infrastructure, func() {
			infrastructure.Status = operatorv1.InfrastructureProviderStatus{
				ProviderStatus: operatorv1.ProviderStatus{
					InstalledVersion: ptr.To("v1.2.3"),
				},
			}
		})).Should(Succeed())

		s := sync.NewProviderSync(testEnv, capiProvider)

		dest := &operatorv1.InfrastructureProvider{}
		Eventually(func(g Gomega) {
			err = nil
			g.Expect(s.Get(ctx)).To(Succeed())
			g.Expect(s.Sync(ctx)).To(Succeed())
			s.Apply(ctx, &err)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(capiProvider.Status.Conditions).To(HaveLen(1))
			g.Expect(conditions.IsTrue(capiProvider, turtlesv1.LastAppliedConfigurationTime)).To(BeTrue())
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(infrastructure), dest)).To(Succeed())
		}).Should(Succeed())

		s = sync.NewProviderSync(testEnv, capiProviderDuplicate)

		Eventually(func(g Gomega) {
			err = nil
			g.Expect(s.Get(ctx)).To(Succeed())
			g.Expect(s.Sync(ctx)).To(Succeed())
			s.Apply(ctx, &err)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(infrastructureDuplicate), dest)).To(Succeed())
			g.Expect(dest.GetAnnotations()).To(HaveKeyWithValue(sync.AppliedSpecHashAnnotation, ""))
			g.Expect(capiProviderDuplicate.Status.Conditions).To(HaveLen(1))
			g.Expect(conditions.IsTrue(capiProviderDuplicate, turtlesv1.LastAppliedConfigurationTime)).To(BeTrue())
			g.Expect(conditions.Get(capiProviderDuplicate, turtlesv1.LastAppliedConfigurationTime).LastTransitionTime.Minute()).To(Equal(time.Now().UTC().Minute()))
		}).Should(Succeed())

		// Provider manifest should be created and phase set to provisioning
		Eventually(func(g Gomega) {
			err = nil
			g.Expect(s.Get(ctx)).To(Succeed())
			g.Expect(s.Sync(ctx)).To(Succeed())
			s.Apply(ctx, &err)
			g.Expect(err).ToNot(HaveOccurred())

			g.Expect(capiProviderDuplicate).To(HaveField("Status.Phase", Equal(turtlesv1.Provisioning)))
			g.Expect(capiProviderDuplicate).To(HaveField("Status.ProviderStatus.InstalledVersion", BeNil()))
		}).Should(Succeed())
	})
})
