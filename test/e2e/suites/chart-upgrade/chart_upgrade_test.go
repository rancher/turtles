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

package chart_upgrade

import (
	_ "embed"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	apimachineryversion "k8s.io/apimachinery/pkg/util/version"
	"k8s.io/utils/ptr"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/test/e2e/specs"
	corev1 "k8s.io/api/core/v1"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
)

const (
	// Turtles-specific constants
	turtlesDeploymentName = "rancher-turtles-controller-manager"

	// CAPI-specific constants
	capiDeploymentName = "capi-controller-manager"
	capiNamespace      = "cattle-capi-system"
	capiProviderName   = "cluster-api"

	// Upstream CAPI image URL is used for verifying controller image after update
	upstreamCAPIImageURL = "registry.k8s.io/cluster-api/cluster-api-controller"

	enabledProviders = "docker,rke2"
)

// This is the updated version of the chart-upgrade test for verifying updating from a Rancher version that
// uses Turtles with CAPI v1.10 to a Rancher version that uses Turtles with CAPI v1.11.
//   - Users that are bumping to v2.14 (CAPI v1.11) will always be on v2.13 as Rancher does not support skipping a minor.
//     1. Install Rancher v2.13.2 which includes Turtles as system chart.
//     2. Validate Rancher and Turtles are installed successfully.
//     3. Install CAPI providers: for this test, only `docker,rke2`
//     4. Provisions and runs checks on workload cluster using `v1beta1` clients: `CreateUsingGitOpsV1Beta1Spec`.
//     5. `UpgradeInstallRancherWithGitea` and configure current version of Turtles -> this uses CAPI v1.11.
//     6. Confirm that Turtles is rolled-out.
//     7. Check providers after upgrade.
//     8. Verify the workload cluster is still available and active.
var _ = Describe("Chart upgrade functionality should work", Ordered, Label(e2e.ShortTestLabel), func() {
	var (
		clusterName       string
		topologyNamespace = "creategitops-docker-rke2"

		toBeUpdatedTurtlesPod *corev1.Pod
	)

	BeforeAll(func() {
		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)

		clusterName = "cluster-docker-rke2"
	})

	It("Should install Rancher 2.13.2, including Turtles v0.25.2 as system chart, and provision a workload cluster", func() {
		By("Installing Rancher 2.13.2 (simulating existing Rancher installation)")
		rancherHookResult := testenv.DeployRancher(ctx, testenv.DeployRancherInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			RancherVersion:        "2.13.2",
			RancherPatches:        [][]byte{e2e.RancherSettingPatch},
		})
		hostName = rancherHookResult.Hostname

		By("Waiting for Turtles controller to be installed by system chart controller")
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      turtlesDeploymentName,
				Namespace: e2e.NewRancherTurtlesNamespace,
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		By("Fetching the Turtles controller Pod for later")
		podList := &corev1.PodList{}
		Expect(bootstrapClusterProxy.GetClient().List(ctx, podList, &client.ListOptions{Namespace: e2e.NewRancherTurtlesNamespace}))
		Expect(podList.Items).ShouldNot(BeEmpty(), "Turtles namespace must have at least one pod running")
		for i, pod := range podList.Items {
			if strings.HasPrefix(pod.Name, turtlesDeploymentName) {
				toBeUpdatedTurtlesPod = &podList.Items[i]
			}
		}
		Expect(toBeUpdatedTurtlesPod).ShouldNot(BeNil(), "Turtles must have a controller manager pod running")
		GinkgoWriter.Printf("Found Turtles controller pod: %s\n", toBeUpdatedTurtlesPod.GetName())

		By("Applying test ClusterctlConfig to configure Docker provider version")
		Expect(framework.Apply(ctx, setupClusterResult.BootstrapClusterProxy, e2e.ClusterctlConfigChartUpgrade)).To(Succeed())

		By("Installing CAPI providers via providers chart")
		testenv.DeployRancherTurtlesProviders(ctx, testenv.DeployRancherTurtlesProvidersInput{
			BootstrapClusterProxy:        bootstrapClusterProxy,
			WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers"),
			UseLegacyCAPINamespace:       false, // >=v0.25.0 uses `cattle-capi-system`
			RancherTurtlesNamespace:      e2e.NewRancherTurtlesNamespace,
			ProviderList:                 enabledProviders,
		})
	})

	specs.CreateUsingGitOpsV1Beta1Spec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		By("Provisioning workload cluster (validates zero-downtime requirement)")

		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIDockerRKE2V1Beta1Topology,
			ClusterName:                    clusterName,
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			LabelNamespace:                 true,
			TestClusterReimport:            false,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-rancher",
			DeleteClusterWaitName:          "wait-controllers",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
			TopologyNamespace:              topologyNamespace,
			SkipCleanup:                    true, // Keep cluster running during upgrade
			SkipDeletionTest:               true,
			SkipLatestFeatureChecks:        true,
			AdditionalFleetGitRepos: []framework.FleetCreateGitRepoInput{
				{
					Name:            "docker-cluster-classes-regular",
					Paths:           []string{"examples/clusterclasses/docker/rke2-v1beta1"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "docker-cni",
					Paths:           []string{"examples/applications/cni/calico"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
		}
	})

	It("Should migrate to Rancher 2.14.x with zero-downtime", func() {
		By("Upgrading Rancher to 2.14.x with Gitea chart repository")
		testenv.UpgradeInstallRancherWithGitea(ctx, testenv.UpgradeInstallRancherWithGiteaInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			ChartRepoURL:          chartsResult.ChartRepoHTTPURL,
			ChartRepoBranch:       chartsResult.Branch,
			ChartVersion:          chartsResult.ChartVersion,
			TurtlesImageRepo:      "ghcr.io/rancher/turtles-e2e",
			TurtlesImageTag:       "v0.0.1",
			RancherHostname:       hostName,
			RancherWaitInterval:   e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher"),
		})

		By("Waiting for Rancher to be ready after upgrade")
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "rancher",
				Namespace: e2e.RancherNamespace,
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...)

		By("Verifying the Turtles controller Pod was rolled out")
		Eventually(func() bool {
			err := bootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(toBeUpdatedTurtlesPod), toBeUpdatedTurtlesPod)
			return apierrors.IsNotFound(err)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).
			Should(BeTrue(), "Previously running Turtles pod should have been rolled out")

		By("Waiting for Turtles controller to be installed by system chart controller")
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      turtlesDeploymentName,
				Namespace: e2e.NewRancherTurtlesNamespace,
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		By("Verifying core CAPIProvider version is updated after Rancher upgrade")
		Eventually(func() bool {
			capiProvider := &turtlesv1.CAPIProvider{}
			Expect(bootstrapClusterProxy.GetClient().Get(ctx,
				types.NamespacedName{
					Namespace: capiNamespace,
					Name:      capiProviderName,
				}, capiProvider)).Should(Succeed())

			semVer, err := apimachineryversion.ParseSemantic(capiProvider.GetSpec().Version)
			if err != nil {
				return false
			}

			refSemVer, err := apimachineryversion.ParseSemantic("v1.11.0")
			if err != nil {
				return false
			}

			return semVer.AtLeast(refSemVer)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).
			Should(BeTrue(), "Core CAPIProvider must be updated to at least v1.11.0")

		By("Waiting for core provider controller to be ready")
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      capiDeploymentName,
				Namespace: capiNamespace,
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		By("Verifying workload cluster survived the upgrade (zero-downtime validated)")
		// This is the critical validation: the workload cluster provisioned before the upgrade
		// should still be healthy and operational, proving zero-downtime migration
		framework.VerifyCluster(ctx, framework.VerifyClusterInput{
			BootstrapClusterProxy:   bootstrapClusterProxy,
			Name:                    clusterName,
			DeleteAfterVerification: true,
		})
	})

	// NOTE: this test is temporarily disabled as Turtles stays on the latest CAPI patch version.
	XIt("Should bump core CAPI when a new version of Turtles ships with a newer version of CAPI", func() {
		By("Upgrading Turtles to 2.13.x from Gitea with newer core CAPI version")
		testenv.UpgradeInstallRancherWithGitea(ctx, testenv.UpgradeInstallRancherWithGiteaInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			ChartRepoURL:          chartsResult.ChartRepoHTTPURL,
			ChartRepoBranch:       chartsResult.Branch,
			ChartVersion:          fmt.Sprintf("%s.1", chartsResult.ChartVersion),
			TurtlesImageRepo:      "ghcr.io/rancher/turtles-e2e",
			TurtlesImageTag:       "v0.0.1-capi",
			RancherHostname:       hostName,
			RancherWaitInterval:   e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher"),
		})

		By("Waiting for core CAPI Provider to be updated")
		Eventually(func() bool {
			capiProvider := &turtlesv1.CAPIProvider{}
			err := bootstrapClusterProxy.GetClient().Get(ctx,
				types.NamespacedName{
					Namespace: capiNamespace,
					Name:      capiProviderName,
				}, capiProvider)
			if err != nil {
				return false
			}

			version := capiProvider.GetSpec().Version
			expected := e2eConfig.GetVariableOrEmpty(e2e.CAPIVersionBump)

			return version == expected
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).
			Should(BeTrue(), "Failed to verify CAPIProvider version after upgrade")

		By("Verifying core CAPI controller pod runs the expected version after upgrade")
		Eventually(func() bool {
			podList := corev1.PodList{}
			podLabels := map[string]string{
				"cluster.x-k8s.io/provider": capiProviderName,
				"control-plane":             "controller-manager",
			}
			err := bootstrapClusterProxy.GetClient().List(ctx, &podList,
				&client.ListOptions{
					Namespace:     capiNamespace,
					LabelSelector: labels.SelectorFromSet(podLabels),
				})
			if err != nil {
				return false
			}

			if len(podList.Items) == 0 {
				return false
			}

			image := podList.Items[0].Spec.Containers[0].Image
			expected := fmt.Sprintf("%s:%s", upstreamCAPIImageURL, e2eConfig.GetVariableOrEmpty(e2e.CAPIVersionBump))

			return image == expected
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).
			Should(BeTrue(), "Failed to verify CAPI controller pod image after upgrade")
	})
})
