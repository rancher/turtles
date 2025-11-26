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
	"bytes"
	_ "embed"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	"github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	capiframework "sigs.k8s.io/cluster-api/test/framework"
)

var _ = Describe("Chart upgrade functionality should work", Ordered, Label(e2e.ShortTestLabel), func() {
	var (
		clusterName       string
		topologyNamespace = "creategitops-docker-rke2"
	)

	BeforeAll(func() {
		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)

		clusterName = fmt.Sprintf("cluster-docker-rke2")
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
		testenv.UpgradeRancherWithGitea(ctx, testenv.UpgradeRancherWithGiteaInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			// at the time of adding this test, there's no stable chart in `rancher-latest` for v2.13.0, so we use a release candidate
			RancherVersion:      "2.13.0-rc2",
			ChartRepoURL:        chartsResult.ChartRepoHTTPURL,
			ChartRepoBranch:     chartsResult.Branch,
			ChartVersion:        chartsResult.ChartVersion,
			RancherWaitInterval: e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher"),
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
		clusterctlConfigYAML := bytes.Replace(e2e.ClusterctlConfig, []byte("rancher-turtles-system"), []byte("cattle-turtles-system"), 1)
		framework.Apply(ctx, bootstrapClusterProxy, clusterctlConfigYAML)

		By("Installing CAPI providers via providers chart (post-upgrade, uses cattle-capi-system namespace)")
		testenv.DeployRancherTurtlesProviders(ctx, testenv.DeployRancherTurtlesProvidersInput{
			BootstrapClusterProxy:        bootstrapClusterProxy,
			WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers"),
			UseLegacyCAPINamespace:       false, // v0.25.x uses new cattle-capi-system namespace
			RancherTurtlesNamespace:      e2e.NewRancherTurtlesNamespace,
			ProviderList:                 "docker,aws,gcp",
			AdditionalValues: map[string]string{
				"providers.infrastructureDocker.enabled": "true",
				"providers.infrastructureAWS.enabled":    "true",
				"providers.infrastructureGCP.enabled":    "true",
			},
		})

		By("Verifying all CAPI providers are running after upgrade")
		framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
			Getter:    bootstrapClusterProxy.GetClient(),
			Name:      "cluster-api",
			Namespace: "cattle-capi-system",
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
			Getter:    bootstrapClusterProxy.GetClient(),
			Name:      "docker",
			Namespace: "cattle-capi-system",
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
			Getter:    bootstrapClusterProxy.GetClient(),
			Name:      "aws",
			Namespace: "cattle-capi-system",
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
			Getter:    bootstrapClusterProxy.GetClient(),
			Name:      "gcp",
			Namespace: "cattle-capi-system",
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
})
