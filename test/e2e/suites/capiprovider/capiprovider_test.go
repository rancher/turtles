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
	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/conditions"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
)

const (
	// vSphere is used as a sample certified provider.
	capvDeploymentName = "capv-controller-manager"
	capvNamespace      = "capv-system"
	capvProviderName   = "vsphere"

	// vCluster is used as an uknown provider.
	// Uknown in this context means this provider is not certified and not known by clusterctl upstream.
	// See: https://github.com/kubernetes-sigs/cluster-api/blob/main/cmd/clusterctl/client/config/providers_client.go
	vclusterDeploymentName = "cluster-api-provider-vcluster-controller-manager"
	vclusterNamespace      = "cluster-api-provider-vcluster-system"
	vclusterProviderName   = "vcluster"
)

var _ = Describe("CAPIProvider lifecycle", Ordered, Label(e2e.ShortTestLabel), func() {
	BeforeAll(func() {
		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)
	})

	It("Should apply initial ClusterctlConfig", func() {
		Expect(framework.Apply(ctx, bootstrapClusterProxy, e2e.ClusterctlConfigInitial)).Should(Succeed())
	})

	It("Should install latest available provider version from ClusterctlConfig when version is empty", func() {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersYAML: [][]byte{
				e2e.CAPVProviderNoVersion,
			},
			WaitForDeployments: []types.NamespacedName{
				{
					Name:      capvDeploymentName,
					Namespace: capvNamespace,
				},
			},
		})

		verifyManagerImage(ctx, types.NamespacedName{Name: capvDeploymentName, Namespace: capvNamespace}, "registry.k8s.io/cluster-api-vsphere/cluster-api-vsphere-controller:v1.12.0")
	})

	It("Should have pinned latest installed version", func() {
		provider := &turtlesv1.CAPIProvider{}
		Expect(bootstrapClusterProxy.GetClient().
			Get(ctx, types.NamespacedName{Namespace: capvNamespace, Name: capvProviderName}, provider)).
			Should(Succeed())
		Expect(provider.Spec.Version).Should(Equal("v1.12.0"))
	})

	It("Should notify of available update when bumping ClusterctlConfig", func() {
		Expect(framework.Apply(ctx, bootstrapClusterProxy, e2e.ClusterctlConfigUpdated)).Should(Succeed())
		provider := &turtlesv1.CAPIProvider{}
		checkLastVersionCondition := &metav1.Condition{}
		Eventually(func() bool {
			Expect(bootstrapClusterProxy.GetClient().
				Get(ctx, types.NamespacedName{Namespace: capvNamespace, Name: capvProviderName}, provider)).
				Should(Succeed())
			checkLastVersionCondition = conditions.Get(provider, string(turtlesv1.CheckLatestVersionTime))
			if checkLastVersionCondition == nil {
				return false
			}
			if checkLastVersionCondition.Reason != turtlesv1.CheckLatestUpdateAvailableReason {
				return false
			}
			return true
		}, e2e.LoadE2EConfig().GetIntervals("default", "wait-capiprovider-update")...).Should(BeTrue(), "CAPIProvider must have CheckLatestVersionTime condition with UpdateAvailable reason")
		Expect(checkLastVersionCondition.Status).Should(Equal(metav1.ConditionFalse), "UpdateAvailable status must be False")
		Expect(checkLastVersionCondition.Message).Should(Equal("Provider version update available. Current latest is v1.13.0"))
	})

	It("Should not automatically update provider version", func() {
		provider := &turtlesv1.CAPIProvider{}
		Expect(bootstrapClusterProxy.GetClient().
			Get(ctx, types.NamespacedName{Namespace: capvNamespace, Name: capvProviderName}, provider)).
			Should(Succeed())
		Expect(provider.Spec.Version).Should(Equal("v1.12.0"))
		consistentlyVerifyManagerImage(ctx, types.NamespacedName{Name: capvDeploymentName, Namespace: capvNamespace}, "registry.k8s.io/cluster-api-vsphere/cluster-api-vsphere-controller:v1.12.0")
	})

	It("Should automatically update provider version if EnabledAutomaticUpdate", func() {
		provider := &turtlesv1.CAPIProvider{}
		Expect(bootstrapClusterProxy.GetClient().
			Get(ctx, types.NamespacedName{Namespace: capvNamespace, Name: capvProviderName}, provider)).
			Should(Succeed())
		provider.Spec.EnableAutomaticUpdate = true
		Expect(bootstrapClusterProxy.GetClient().Update(ctx, provider)).Should(Succeed())

		Eventually(func() bool {
			Expect(bootstrapClusterProxy.GetClient().
				Get(ctx, types.NamespacedName{Namespace: capvNamespace, Name: capvProviderName}, provider)).
				Should(Succeed())
			checkLastVersionCondition := conditions.Get(provider, string(turtlesv1.CheckLatestVersionTime))
			if checkLastVersionCondition == nil {
				return false
			}
			if checkLastVersionCondition.Status != metav1.ConditionTrue {
				return false
			}
			return true
		}, e2e.LoadE2EConfig().GetIntervals("default", "wait-capiprovider-update")...).
			Should(BeTrue(), "CAPIProvider must have CheckLatestVersionTime condition True")

		Expect(provider.Spec.Version).Should(Equal("v1.13.0"))
		verifyManagerImage(ctx, types.NamespacedName{Name: capvDeploymentName, Namespace: capvNamespace}, "registry.k8s.io/cluster-api-vsphere/cluster-api-vsphere-controller:v1.13.0")
	})

	It("Should delete vSphere provider and namespace", func() {
		Expect(framework.Delete(ctx, bootstrapClusterProxy, e2e.CAPVProviderNoVersion)).Should(Succeed())
	})

	It("Should install but not automatically update Unknown provider", func() {
		Expect(framework.Apply(ctx, bootstrapClusterProxy, e2e.UnknownProvider)).Should(Succeed())
		provider := &turtlesv1.CAPIProvider{}
		checkLastVersionCondition := &metav1.Condition{}
		Eventually(func() bool {
			Expect(bootstrapClusterProxy.GetClient().
				Get(ctx, types.NamespacedName{Namespace: vclusterNamespace, Name: vclusterProviderName}, provider)).
				Should(Succeed())
			checkLastVersionCondition = conditions.Get(provider, string(turtlesv1.CheckLatestVersionTime))
			if checkLastVersionCondition == nil {
				return false
			}
			return true
		}, e2e.LoadE2EConfig().GetIntervals("default", "wait-capiprovider-update")...).
			Should(BeTrue(), "CAPIProvider must have CheckLatestVersionTime condition")
		Expect(checkLastVersionCondition.Status).Should(Equal(metav1.ConditionUnknown), "UpdateAvailable status must be Unknown")
		Expect(checkLastVersionCondition.Message).Should(Equal("Provider is unknown"))
		Expect(checkLastVersionCondition.Reason).Should(Equal(turtlesv1.CheckLatestProviderUnknownReason))

		Expect(provider.Spec.Version).Should(Equal("v0.2.1"))
		consistentlyVerifyManagerImage(ctx, types.NamespacedName{Name: vclusterDeploymentName, Namespace: vclusterNamespace}, "docker.io/loftsh/cluster-api-provider-vcluster:0.2.1")
	})

	It("Should delete Unknown provider and namespace", func() {
		Expect(framework.Delete(ctx, bootstrapClusterProxy, e2e.UnknownProvider)).Should(Succeed())
	})

	It("Should delete ClusterctlConfig", func() {
		Expect(framework.Delete(ctx, bootstrapClusterProxy, e2e.ClusterctlConfigUpdated)).Should(Succeed())
	})
})

var _ = Describe("no-cert-manager feature verification", Ordered, Label(e2e.ShortTestLabel), func() {
	BeforeAll(func() {
		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)
	})

	It("Should install provider", func() {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersYAML: [][]byte{
				e2e.CAPVProviderNoVersion,
			},
			WaitForDeployments: []types.NamespacedName{
				{
					Name:      capvDeploymentName,
					Namespace: capvNamespace,
				},
			},
		})
	})

	It("Should test no-cert-manager conversion", func() {
		framework.VerifyCertificatesInNamespace(ctx, bootstrapClusterProxy.GetClient(), capvNamespace)
		framework.VerifyIssuersInNamespace(ctx, bootstrapClusterProxy.GetClient(), capvNamespace)
		framework.VerifyCertManagerAnnotationsForProvider(ctx, bootstrapClusterProxy.GetClient(), "infrastructure-vsphere")
		framework.VerifyWranglerAnnotationsInNamespace(ctx, bootstrapClusterProxy.GetClient(), capvNamespace)
	})

	It("Should verify WranglerManagedCertificates Condition", func() {
		Eventually(func() bool {
			capiProvider := &turtlesv1.CAPIProvider{}
			Expect(bootstrapClusterProxy.GetClient().Get(ctx,
				types.NamespacedName{
					Namespace: capvNamespace,
					Name:      capvProviderName,
				}, capiProvider)).Should(Succeed())
			condition := conditions.Get(capiProvider, turtlesv1.CAPIProviderWranglerManagedCertificatesCondition)
			if condition == nil || condition.Status != metav1.ConditionTrue {
				return false
			}
			return true
		}, e2e.LoadE2EConfig().GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).
			Should(BeTrue(), "WranglerManagedCertificates condition must be True")
	})

	It("Should wait for Deployment to be available", func() {
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      capvDeploymentName,
				Namespace: capvNamespace,
			}},
		}, e2e.LoadE2EConfig().GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers")...)
	})

	It("Should apply dummy resource that uses webhooks", func() {
		Expect(framework.Apply(ctx, bootstrapClusterProxy, e2e.CAPVDummyMachineTemplate)).Should(Succeed())
		// Early cleanup since it's not used
		Expect(framework.Delete(ctx, bootstrapClusterProxy, e2e.CAPVDummyMachineTemplate)).Should(Succeed())
	})

	It("Should uninstall provider", func() {
		Expect(framework.Delete(ctx, bootstrapClusterProxy, e2e.CAPVProviderNoVersion)).Should(Succeed())
	})
})

func verifyManagerImage(ctx context.Context, deploymentKey types.NamespacedName, desiredImage string) {
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
