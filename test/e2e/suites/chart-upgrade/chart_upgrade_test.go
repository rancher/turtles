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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	"github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"

	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/conditions"
)

const (
	// CAPI-specific constants
	capiDeploymentName = "capi-controller-manager"
	capiNamespace      = "cattle-capi-system"
	capiProviderName   = "cluster-api"

	// vSphere is used as a sample certified provider.
	capvDeploymentName = "capv-controller-manager"
	capvNamespace      = "capv-system"
	capvProviderName   = "vsphere"

	// Upstream CAPI image URL is used for verifying controller image after update
	upstreamCAPIImageURL = "registry.k8s.io/cluster-api/cluster-api-controller"
)

var _ = Describe("Chart upgrade functionality should work", Ordered, Label(e2e.ShortTestLabel), func() {
	var (
		clusterName       string
		topologyNamespace = "creategitops-docker-rke2"

		toBeWranglerConvertedPod *corev1.Pod
	)

	BeforeAll(func() {
		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)

		clusterName = "cluster-docker-rke2"
	})

	// This test suite validates the ZERO-DOWNTIME migration from Rancher 2.12.x/Turtles v0.24.x
	// to Rancher 2.13.x with system chart controller architecture.
	//
	// This tests the realistic production scenario where:
	// - Users have existing CAPI providers installed
	// - Workload clusters are running and must remain operational during upgrade
	// - Provider resources need to be migrated (not destroyed and recreated)
	//
	// Migration steps following the official migration guide (https://turtles.docs.rancher.com/turtles/next/en/tutorials/migration.html):
	// 1. Install Rancher 2.12.3 (simulating existing installation)
	// 2. Install Turtles v0.24.3 via Helm
	// 3. Install CAPI providers (simulating existing production setup)
	// 4. Provision workload cluster (validates zero-downtime requirement)
	// 5. Uninstall Rancher Turtles (providers and clusters keep running)
	// 6. Patch CRDs with cattle-turtles-system namespace
	// 7. Upgrade Rancher to 2.13.x (enables system chart controller)
	// 8. Run migration script to adopt provider resources into new Helm release
	// 9. Install additional CAPI providers via providers Helm chart
	// 10. Verify workload cluster survived the upgrade (zero-downtime validated)
	It("Should install Rancher 2.12.3/Turtles v0.24.3 and provision a workload cluster", func() {
		By("Installing Rancher 2.12.3 (simulating existing Rancher installation)")
		rancherHookResult := testenv.DeployRancher(ctx, testenv.DeployRancherInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			RancherVersion:        "2.12.3",
			RancherPatches:        [][]byte{e2e.RancherSettingPatch},
		})
		hostName = rancherHookResult.Hostname

		By("Installing Turtles v0.24.3 via Helm (Step 1 of migration guide: upgrade to v0.24.3)")
		rtInput := testenv.DeployRancherTurtlesInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersYAML:     e2e.CapiProviders,
			TurtlesChartRepoName:  "rancher-turtles",
			TurtlesChartUrl:       "https://rancher.github.io/turtles",
			Version:               "v0.24.3",
			AdditionalValues: map[string]string{
				"rancherTurtles.namespace": e2e.RancherTurtlesNamespace,
			},
		}
		testenv.DeployRancherTurtles(ctx, rtInput)

		By("Waiting for Turtles controller Deployment to be ready")
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "rancher-turtles-controller-manager",
				Namespace: e2e.RancherTurtlesNamespace,
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		By("Provisioning workload cluster (validates zero-downtime requirement)")

		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIDockerRKE2Topology,
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
					Paths:           []string{"examples/clusterclasses/docker/rke2"},
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

	It("Should install a CAPIProvider to test cert-manager migration", func() {
		By("Installing vSphere CAPIProvider")
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
		By("Consistently verifying wrangler is not in use for the provider")
		Consistently(func() {
			capiProvider := &turtlesv1.CAPIProvider{}
			Expect(bootstrapClusterProxy.GetClient().Get(ctx,
				types.NamespacedName{
					Namespace: capvNamespace,
					Name:      capvProviderName,
				}, capiProvider)).Should(Succeed())

			condition := conditions.Get(capiProvider, turtlesv1.CAPIProviderWranglerManagedCertificatesCondition)
			Expect(condition).Should(BeNil(), "WranglerManagedCertificates condition should not be present")
		}).WithTimeout(2 * time.Minute).WithPolling(10 * time.Second)

		By("Applying and deleting a dummy resource that uses webhooks")
		Expect(framework.Apply(ctx, bootstrapClusterProxy, e2e.CAPVDummyMachineTemplate)).Should(Succeed())
		Expect(framework.Delete(ctx, bootstrapClusterProxy, e2e.CAPVDummyMachineTemplate)).Should(Succeed())

		By("Fetching the provider manager Pod for later")
		podList := &corev1.PodList{}
		Expect(bootstrapClusterProxy.GetClient().List(ctx, podList, &client.ListOptions{Namespace: capvNamespace}))
		Expect(podList.Items).ShouldNot(BeEmpty(), "Provider must have at least one pod running")
		for i, pod := range podList.Items {
			if strings.HasPrefix(pod.Name, capvDeploymentName) {
				toBeWranglerConvertedPod = &podList.Items[i]
			}
		}
		Expect(toBeWranglerConvertedPod).ShouldNot(BeNil(), "Provider must have a controller manager pod running")
		GinkgoWriter.Printf("Found manager pod: %s\n", toBeWranglerConvertedPod.GetName())
	})

	It("Should migrate to Rancher 2.13.x with zero-downtime", func() {
		By("Uninstalling Turtles v0.24.3 (providers and workload cluster keep running)")
		testenv.UninstallRancherTurtles(ctx, testenv.UninstallRancherTurtlesInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			DeleteWaitInterval:    e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers"),
		})

		By("Patching Turtles CRDs with Helm release annotations for cattle-turtles-system (Step 3 of migration guide)")
		framework.RunCommand(ctx, framework.RunCommandInput{
			Command: "kubectl",
			Args: []string{
				"--kubeconfig", bootstrapClusterProxy.GetKubeconfigPath(),
				"patch", "crd", "capiproviders.turtles-capi.cattle.io",
				"--type=json",
				"-p=[{\"op\": \"add\", \"path\": \"/metadata/annotations/meta.helm.sh~1release-namespace\", \"value\": \"cattle-turtles-system\"}]",
			},
		}, &framework.RunCommandResult{})

		framework.RunCommand(ctx, framework.RunCommandInput{
			Command: "kubectl",
			Args: []string{
				"--kubeconfig", bootstrapClusterProxy.GetKubeconfigPath(),
				"patch", "crd", "clusterctlconfigs.turtles-capi.cattle.io",
				"--type=json",
				"-p=[{\"op\": \"add\", \"path\": \"/metadata/annotations/meta.helm.sh~1release-namespace\", \"value\": \"cattle-turtles-system\"}]",
			},
		}, &framework.RunCommandResult{})

		By("Upgrading Rancher to 2.13.x with Gitea chart repository (enables system chart controller)")
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

		By("Waiting for Turtles controller to be installed by system chart controller")
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "rancher-turtles-controller-manager",
				Namespace: e2e.NewRancherTurtlesNamespace,
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		By("Creating ClusterctlConfig for CAPD registry override in cattle-turtles-system namespace")
		Expect(framework.Apply(ctx, setupClusterResult.BootstrapClusterProxy, e2e.ClusterctlConfig)).To(Succeed())

		By("Installing CAPI providers via providers chart (post-upgrade, uses cattle-capi-system namespace)")
		testenv.DeployRancherTurtlesProviders(ctx, testenv.DeployRancherTurtlesProvidersInput{
			BootstrapClusterProxy:        bootstrapClusterProxy,
			WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers"),
			UseLegacyCAPINamespace:       false, // v0.25.x uses new cattle-capi-system namespace
			RancherTurtlesNamespace:      e2e.NewRancherTurtlesNamespace,
			ProviderList:                 "docker,aws,gcp",
		})

		By("Verifying all CAPI providers are running after upgrade")
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      capiDeploymentName,
				Namespace: capiNamespace,
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "capd-controller-manager",
				Namespace: "capd-system",
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "capa-controller-manager",
				Namespace: "capa-system",
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "capg-controller-manager",
				Namespace: "capg-system",
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

	It("Should verify CAPIProvider was converted to wrangler certificates", func() {
		By("Verifying CAPIProvider has the WranglerManagedCertificates condition")
		Eventually(func() bool {
			capiProvider := &turtlesv1.CAPIProvider{}
			Expect(bootstrapClusterProxy.GetClient().Get(ctx,
				types.NamespacedName{
					Namespace: capvNamespace,
					Name:      capvProviderName,
				}, capiProvider)).Should(Succeed())
			condition := conditions.Get(capiProvider, turtlesv1.CAPIProviderWranglerManagedCertificatesCondition)
			if condition == nil || condition.Status != corev1.ConditionTrue {
				return false
			}
			return true
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).
			Should(BeTrue(), "WranglerManagedCertificates condition must be True")

		By("Verifying cert-manager resources are deleted and Services are wrangler annotated")
		framework.VerifyCertificatesInNamespace(ctx, bootstrapClusterProxy.GetClient(), capvNamespace)
		framework.VerifyIssuersInNamespace(ctx, bootstrapClusterProxy.GetClient(), capvNamespace)
		framework.VerifyCertManagerAnnotationsForProvider(ctx, bootstrapClusterProxy.GetClient(), "infrastructure-vsphere")
		framework.VerifyWranglerAnnotationsInNamespace(ctx, bootstrapClusterProxy.GetClient(), capvNamespace)

		By("Verifying the provider manager Pod was rolled out")
		Eventually(func() bool {
			err := bootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(toBeWranglerConvertedPod), toBeWranglerConvertedPod)
			return apierrors.IsNotFound(err)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).
			Should(BeTrue(), "Previously running pod should have been rolled out")

		By("Applying and deleting a dummy resource that uses webhooks")
		Expect(framework.Apply(ctx, bootstrapClusterProxy, e2e.CAPVDummyMachineTemplate)).Should(Succeed())
		Expect(framework.Delete(ctx, bootstrapClusterProxy, e2e.CAPVDummyMachineTemplate)).Should(Succeed())
	})

	It("Should bump core CAPI when a new version of Turtles ships with a newer version of CAPI", func() {
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
