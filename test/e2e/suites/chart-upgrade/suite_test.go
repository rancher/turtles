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
	"context"
	"encoding/json"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/testenv"

	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Test suite global vars.
var (
	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// hostName is the host name for the Rancher Manager server.
	hostName string

	// giteaResult stores the result of Gitea deployment
	giteaResult *testenv.DeployGiteaResult

	// chartsResult stores the result of building and pushing charts to Gitea
	chartsResult *testenv.PushRancherChartsToGiteaResult

	ctx = context.Background()

	setupClusterResult    *testenv.SetupTestClusterResult
	bootstrapClusterProxy capiframework.ClusterProxy
)

// setupData is the data structure shared between SynchronizedBeforeSuite parallel runs
type setupData struct {
	e2e.Setup
	GitAddress       string
	ChartRepoURL     string
	ChartRepoHTTPURL string
	ChartBranch      string
	ChartVersion     string
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	RunSpecs(t, "rancher-turtles-e2e-chart-upgrade")
}

var _ = SynchronizedBeforeSuite(
	func() []byte {
		e2eConfig := e2e.LoadE2EConfig()

		e2eConfig.ManagementClusterName = e2eConfig.ManagementClusterName + "-chart-upgrade"
		setupClusterResult = testenv.SetupTestCluster(ctx, testenv.SetupTestClusterInput{
			E2EConfig: e2eConfig,
			Scheme:    e2e.InitScheme(),
			// Use v1.32.0 for Rancher 2.12.3 compatibility (requires < v1.34.0) and v1.33 causes issues with CAAPF
			KubernetesVersion: "v1.32.0",
		})

		testenv.DeployCertManager(ctx, testenv.DeployCertManagerInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
		})

		testenv.RancherDeployIngress(ctx, testenv.RancherDeployIngressInput{
			BootstrapClusterProxy:    setupClusterResult.BootstrapClusterProxy,
			CustomIngress:            e2e.TraefikIngress,
			DefaultIngressClassPatch: e2e.IngressClassPatch,
		})

		By("Deploying Gitea for chart repository")
		giteaResult = testenv.DeployGitea(ctx, testenv.DeployGiteaInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			ValuesFile:            e2e.GiteaValues,
			CustomIngressConfig:   e2e.GiteaIngress,
		})

		By("Building and pushing Rancher charts to Gitea for later upgrade")
		chartsResult = testenv.PushRancherChartsToGitea(ctx, testenv.PushRancherChartsToGiteaInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			GiteaServerAddress:    giteaResult.GitAddress,
			GiteaRepoName:         "charts",
			// ChartVersion will be auto-populated from RANCHER_CHART_DEV_VERSION env var or Makefile default
		})

		data, err := json.Marshal(setupData{
			Setup: e2e.Setup{
				ClusterName:    setupClusterResult.ClusterName,
				KubeconfigPath: setupClusterResult.KubeconfigPath,
			},
			GitAddress:       giteaResult.GitAddress,
			ChartRepoURL:     chartsResult.ChartRepoURL,
			ChartRepoHTTPURL: chartsResult.ChartRepoHTTPURL,
			ChartBranch:      chartsResult.Branch,
			ChartVersion:     chartsResult.ChartVersion,
		})
		Expect(err).ToNot(HaveOccurred())
		return data
	},
	func(sharedData []byte) {
		setup := setupData{}
		Expect(json.Unmarshal(sharedData, &setup)).To(Succeed())

		e2eConfig = e2e.LoadE2EConfig()

		giteaResult = &testenv.DeployGiteaResult{
			GitAddress: setup.GitAddress,
		}

		chartsResult = &testenv.PushRancherChartsToGiteaResult{
			ChartRepoURL:     setup.ChartRepoURL,
			ChartRepoHTTPURL: setup.ChartRepoHTTPURL,
			Branch:           setup.ChartBranch,
			ChartVersion:     setup.ChartVersion,
		}

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
