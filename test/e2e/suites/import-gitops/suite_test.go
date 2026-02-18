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

package import_gitops

import (
	"context"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"
)

// Test suite global vars.
var (
	// hostName is the host name for the Rancher Manager server.
	hostName string

	ctx = context.Background()

	setupClusterResult    *testenv.SetupTestClusterResult
	bootstrapClusterProxy capiframework.ClusterProxy
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	RunSpecs(t, "rancher-turtles-e2e-import-gitops")
}

var _ = SynchronizedBeforeSuite(
	func() []byte {
		e2eConfig := e2e.LoadE2EConfig()
		e2eConfig.ManagementClusterName = e2eConfig.ManagementClusterName + "-import-gitops"
		setupClusterResult = testenv.SetupTestCluster(ctx, testenv.SetupTestClusterInput{
			E2EConfig: e2eConfig,
			Scheme:    e2e.InitScheme(),
		})

		testenv.DeployCertManager(ctx, testenv.DeployCertManagerInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
		})

		testenv.RancherDeployIngress(ctx, testenv.RancherDeployIngressInput{
			BootstrapClusterProxy:     setupClusterResult.BootstrapClusterProxy,
			CustomIngress:             e2e.TraefikIngress,
			CustomIngressLoadBalancer: e2e.TraefikIngressLoadBalancer,
			DefaultIngressClassPatch:  e2e.IngressClassPatch,
		})

		By("Deploying Gitea for chart repository")
		giteaResult := testenv.DeployGitea(ctx, testenv.DeployGiteaInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			ValuesFile:            e2e.GiteaValues,
			CustomIngressConfig:   e2e.GiteaIngress,
		})

		By("Pushing Rancher charts to Gitea for Turtles installation")
		chartsResult := testenv.PushRancherChartsToGitea(ctx, testenv.PushRancherChartsToGiteaInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			GiteaServerAddress:    giteaResult.GitAddress,
			GiteaRepoName:         "charts",
			// ChartVersion will be auto-populated from RANCHER_CHART_DEV_VERSION env var or Makefile default
		})

		By("Installing Rancher to 2.13.x with Gitea chart repository (enables system chart controller)")
		rancherHookResult := testenv.UpgradeInstallRancherWithGitea(ctx, testenv.UpgradeInstallRancherWithGiteaInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			ChartRepoURL:          chartsResult.ChartRepoHTTPURL,
			ChartRepoBranch:       chartsResult.Branch,
			ChartVersion:          chartsResult.ChartVersion,
			TurtlesImageRepo:      "ghcr.io/rancher/turtles-e2e",
			TurtlesImageTag:       "v0.0.1",
			RancherWaitInterval:   e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher"),
			RancherPatches:        [][]byte{e2e.RancherSettingPatch},
		})

		By("Waiting for Rancher to be ready")
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: setupClusterResult.BootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "rancher",
				Namespace: e2e.RancherNamespace,
			}},
		}, e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher")...)

		By("Waiting for Turtles controller to be installed by system chart controller")
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: setupClusterResult.BootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "rancher-turtles-controller-manager",
				Namespace: e2e.NewRancherTurtlesNamespace,
			}},
		}, e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers")...)

		By("Applying test ClusterctlConfig")
		Expect(framework.Apply(ctx, setupClusterResult.BootstrapClusterProxy, e2e.ClusterctlConfig)).To(Succeed())

		testenv.DeployRancherTurtlesProviders(ctx, testenv.DeployRancherTurtlesProvidersInput{
			BootstrapClusterProxy:   setupClusterResult.BootstrapClusterProxy,
			UseLegacyCAPINamespace:  false,
			RancherTurtlesNamespace: e2e.NewRancherTurtlesNamespace,
		})

		data, err := json.Marshal(e2e.Setup{
			ClusterName:     setupClusterResult.ClusterName,
			KubeconfigPath:  setupClusterResult.KubeconfigPath,
			RancherHostname: rancherHookResult.Hostname,
		})
		Expect(err).ToNot(HaveOccurred())
		return data
	},
	func(sharedData []byte) {
		setup := e2e.Setup{}
		Expect(json.Unmarshal(sharedData, &setup)).To(Succeed())

		hostName = setup.RancherHostname

		bootstrapClusterProxy = capiframework.NewClusterProxy(setup.ClusterName, setup.KubeconfigPath, e2e.InitScheme(), capiframework.WithMachineLogCollector(capiframework.DockerLogCollector{}))
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "cluster proxy should not be nil")
	},
)

var _ = SynchronizedAfterSuite(
	func() {
	},
	func() {
		By("Dumping artifacts from the bootstrap cluster")
		testenv.DumpBootstrapCluster(ctx, bootstrapClusterProxy.GetKubeconfigPath())

		config := e2e.LoadE2EConfig()
		// skipping error check since it is already done at the beginning of the test in e2e.ValidateE2EConfig()
		skipCleanup, _ := strconv.ParseBool(config.GetVariableOrEmpty(e2e.SkipResourceCleanupVar))
		if skipCleanup {
			// add a log line about skipping charts uninstallation and cluster cleanup
			return
		}

		testenv.CleanupTestCluster(ctx, testenv.CleanupTestClusterInput{
			SetupTestClusterResult: *setupClusterResult,
		})
	},
)
