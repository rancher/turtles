//go:build e2e
// +build e2e

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

package capiprovider

import (
	"context"
	_ "embed"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/conditions"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
)

const (
	deploymentName      = "capv-controller-manager"
	deploymentNamespace = "capv-system"
)

var (
	deploymentKey = types.NamespacedName{Namespace: deploymentNamespace, Name: deploymentName}
)

var _ = Describe("CAPIProvider lifecycle", Ordered, Label(e2e.ShortTestLabel), func() {
	BeforeEach(func() {
		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)
	})

	It("Should apply initial ClusterctlConfig", func() {
		Expect(framework.Apply(ctx, bootstrapClusterProxy, e2e.ClusterctlConfigInitial)).Should(Succeed())
	})

	It("Should install latest available provider version from ClusterctlConfig when version is empty", func() {
		Expect(framework.Apply(ctx, bootstrapClusterProxy, e2e.CAPVProviderNamespace)).Should(Succeed())

		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersYAML: [][]byte{
				e2e.CAPVProviderNoVersion,
			},
			WaitForDeployments: []testenv.NamespaceName{
				{
					Name:      deploymentName,
					Namespace: deploymentNamespace,
				},
			},
		})

		verifyManagerImage(ctx, deploymentKey, "registry.k8s.io/cluster-api-vsphere/cluster-api-vsphere-controller:v1.12.0")
	})

	It("Should have pinned latest installed version", func() {
		provider := &turtlesv1.CAPIProvider{}
		Expect(bootstrapClusterProxy.GetClient().
			Get(ctx, types.NamespacedName{Namespace: "capv-system", Name: "vsphere"}, provider)).
			Should(Succeed())
		Expect(provider.Spec.Version).Should(Equal("v1.12.0"))
	})

	It("Should notify of available update when bumping ClusterctlConfig", func() {
		Expect(framework.Apply(ctx, bootstrapClusterProxy, e2e.ClusterctlConfigUpdated)).Should(Succeed())
		provider := &turtlesv1.CAPIProvider{}
		checkLastVersionCondition := &clusterv1.Condition{}
		Eventually(func() bool {
			Expect(bootstrapClusterProxy.GetClient().
				Get(ctx, types.NamespacedName{Namespace: "capv-system", Name: "vsphere"}, provider)).
				Should(Succeed())
			checkLastVersionCondition = conditions.Get(provider, turtlesv1.CheckLatestVersionTime)
			if checkLastVersionCondition == nil {
				return false
			}
			if checkLastVersionCondition.Reason != turtlesv1.CheckLatestUpdateAvailableReason {
				return false
			}
			return true
		}, e2e.LoadE2EConfig().GetIntervals("default", "wait-capiprovider-update")...).Should(BeTrue(), "CAPIProvider must have CheckLatestVersionTime condition with UpdateAvailable reason")
		Expect(checkLastVersionCondition.Severity).Should(Equal(clusterv1.ConditionSeverityInfo), "UpdateAvailable severity must be Info")
		Expect(checkLastVersionCondition.Status).Should(Equal(corev1.ConditionFalse), "UpdateAvailable status must be False")
		Expect(checkLastVersionCondition.Message).Should(Equal("Provider version update available. Current latest is v1.13.0"))
	})

	It("Should not automatically update provider version", func() {
		provider := &turtlesv1.CAPIProvider{}
		Expect(bootstrapClusterProxy.GetClient().
			Get(ctx, types.NamespacedName{Namespace: "capv-system", Name: "vsphere"}, provider)).
			Should(Succeed())
		Expect(provider.Spec.Version).Should(Equal("v1.12.0"))
		consistentlyVerifyManagerImage(ctx, deploymentKey, "registry.k8s.io/cluster-api-vsphere/cluster-api-vsphere-controller:v1.12.0")
	})

	It("Should automatically update provider version if EnabledAutomaticUpdate", func() {
		provider := &turtlesv1.CAPIProvider{}
		Expect(bootstrapClusterProxy.GetClient().
			Get(ctx, types.NamespacedName{Namespace: "capv-system", Name: "vsphere"}, provider)).
			Should(Succeed())
		provider.Spec.EnableAutomaticUpdate = true
		Expect(bootstrapClusterProxy.GetClient().Update(ctx, provider)).Should(Succeed())

		Eventually(func() bool {
			Expect(bootstrapClusterProxy.GetClient().
				Get(ctx, types.NamespacedName{Namespace: "capv-system", Name: "vsphere"}, provider)).
				Should(Succeed())
			checkLastVersionCondition := conditions.Get(provider, turtlesv1.CheckLatestVersionTime)
			if checkLastVersionCondition == nil {
				return false
			}
			if checkLastVersionCondition.Status != corev1.ConditionTrue {
				return false
			}
			return true
		}, e2e.LoadE2EConfig().GetIntervals("default", "wait-capiprovider-update")...).
			Should(BeTrue(), "CAPIProvider must have CheckLatestVersionTime condition True")

		Expect(provider.Spec.Version).Should(Equal("v1.13.0"))
		verifyManagerImage(ctx, deploymentKey, "registry.k8s.io/cluster-api-vsphere/cluster-api-vsphere-controller:v1.13.0")
	})

	It("Should not automatically update Unknown provider", func() {
		Expect(framework.Apply(ctx, bootstrapClusterProxy, e2e.UnknownProvider)).Should(Succeed())
		provider := &turtlesv1.CAPIProvider{}
		checkLastVersionCondition := &clusterv1.Condition{}
		Eventually(func() bool {
			Expect(bootstrapClusterProxy.GetClient().
				Get(ctx, types.NamespacedName{Namespace: "unknown-provider", Name: "vcluster"}, provider)).
				Should(Succeed())
			checkLastVersionCondition = conditions.Get(provider, turtlesv1.CheckLatestVersionTime)
			if checkLastVersionCondition == nil {
				return false
			}
			return true
		}, e2e.LoadE2EConfig().GetIntervals("default", "wait-capiprovider-update")...).
			Should(BeTrue(), "CAPIProvider must have CheckLatestVersionTime condition")
		Expect(checkLastVersionCondition.Status).Should(Equal(corev1.ConditionUnknown), "UpdateAvailable status must be Unknown")
		Expect(checkLastVersionCondition.Message).Should(Equal("Provider is unknown"))
		Expect(checkLastVersionCondition.Reason).Should(Equal(turtlesv1.CheckLatestProviderUnknownReason))

		Expect(provider.Spec.Version).Should(Equal("v0.2.1"))
		consistentlyVerifyManagerImage(ctx, deploymentKey, "docker.io/loftsh/cluster-api-provider-vcluster:0.2.1")
	})
})

func verifyManagerImage(ctx context.Context, deploymentKey types.NamespacedName, desiredImage string) {
	GinkgoHelper()
	deployment := &appsv1.Deployment{}
	Expect(bootstrapClusterProxy.GetClient().Get(ctx, deploymentKey, deployment)).Should(Succeed())
	foundImage := ""
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name == "manager" {
			foundImage = container.Image
		}
	}
	Expect(foundImage).ShouldNot(BeEmpty(), "Could not find any manager container image")

	Expect(foundImage).Should(Equal(desiredImage))
}

func consistentlyVerifyManagerImage(ctx context.Context, deploymentKey types.NamespacedName, desiredImage string) {
	Consistently(func() {
		verifyManagerImage(ctx, deploymentKey, desiredImage)
	}).WithTimeout(2 * time.Minute).WithPolling(10 * time.Second)
}
