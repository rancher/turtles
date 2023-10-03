//go:build e2e
// +build e2e

/*
Copyright 2023 SUSE.

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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/rancher-sandbox/rancher-turtles/test/e2e"
	turtlesframework "github.com/rancher-sandbox/rancher-turtles/test/framework"
	"github.com/rancher-sandbox/rancher-turtles/test/testenv"
)

// Test suite flags.
var (
	// configPath is the path to the e2e config file.
	configPath string

	// useExistingCluster instructs the test to use the current cluster instead of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool

	// helmBinaryPath is the path to the helm binary.
	helmBinaryPath string

	// chartPath is the path to the operator chart.
	chartPath string

	// isolatedMode instructs the test to run without ngrok and exposing the cluster to the internet. This setup will only work with CAPD
	// or other providers that run in the same network as the bootstrap cluster.
	isolatedMode bool

	// clusterctlBinaryPath is the path to the clusterctl binary to use.
	clusterctlBinaryPath string
)

// Test suite global vars.
var (
	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// clusterctlConfigPath to be used for this test, created by generating a clusterctl local repository
	// with the providers specified in the configPath.
	clusterctlConfigPath string

	// hostName is the host name for the Rancher Manager server.
	hostName string

	ctx = context.Background()

	setupClusterResult *testenv.SetupTestClusterResult
	giteaResult        *testenv.DeployGiteaResult
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "config/operator.yaml", "path to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "_artifacts", "folder where e2e test artifact should be stored")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false, "if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
	flag.StringVar(&helmBinaryPath, "e2e.helm-binary-path", "helm", "path to the helm binary")
	flag.StringVar(&clusterctlBinaryPath, "e2e.clusterctl-binary-path", "helm", "path to the clusterctl binary")
	flag.StringVar(&chartPath, "e2e.chart-path", "", "path to the operator chart")
	flag.BoolVar(&isolatedMode, "e2e.isolated-mode", false, "if true, the test will run without ngrok and exposing the cluster to the internet. This setup will only work with CAPD or other providers that run in the same network as the bootstrap cluster.")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	ctrl.SetLogger(klog.Background())

	RunSpecs(t, "rancher-turtles-e2e")
}

var _ = BeforeSuite(func() {
	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder)
	Expect(helmBinaryPath).To(BeAnExistingFile(), "Invalid test suite argument. helm-binary-path should be an existing file.")
	Expect(chartPath).To(BeAnExistingFile(), "Invalid test suite argument. chart-path should be an existing file.")

	By(fmt.Sprintf("Loading the e2e test configuration from %q", configPath))
	e2eConfig = e2e.LoadE2EConfig(configPath)

	By(fmt.Sprintf("Creating a clusterctl config into %q", artifactFolder))
	clusterctlConfigPath = e2e.CreateClusterctlLocalRepository(ctx, e2eConfig, filepath.Join(artifactFolder, "repository"))

	hostName = e2eConfig.GetVariable(e2e.RancherHostnameVar)

	setupClusterResult = testenv.SetupTestCluster(ctx, testenv.SetupTestClusterInput{
		UseExistingCluster:   useExistingCluster,
		E2EConfig:            e2eConfig,
		ClusterctlConfigPath: clusterctlConfigPath,
		Scheme:               e2e.InitScheme(),
		ArtifactFolder:       artifactFolder,
		Hostname:             hostName,
		KubernetesVersion:    e2eConfig.GetVariable(e2e.KubernetesVersionVar),
		IsolatedMode:         isolatedMode,
		HelmBinaryPath:       helmBinaryPath,
	})

	if isolatedMode {
		hostName = setupClusterResult.IsolatedHostName
	}

	testenv.DeployRancherTurtles(ctx, testenv.DeployRancherTurtlesInput{
		BootstrapClusterProxy:        setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:               helmBinaryPath,
		ChartPath:                    chartPath,
		CAPIProvidersSecretYAML:      e2e.CapiProvidersSecret,
		CAPIProvidersYAML:            e2e.CapiProviders,
		Namespace:                    turtlesframework.DefaultRancherTurtlesNamespace,
		Image:                        "ghcr.io/rancher-sandbox/rancher-turtles-amd64",
		Tag:                          "v0.0.1",
		WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers"),
	})

	if Label(e2e.FullTestLabel).MatchesLabelFilter(GinkgoLabelFilter()) {
		By("Running full tests, deploying additional infrastructure providers")
		awsCreds := e2eConfig.GetVariable(e2e.CapaEncodedCredentialsVar)
		Expect(awsCreds).ToNot(BeEmpty(), "AWS creds required for full test")

		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy:   setupClusterResult.BootstrapClusterProxy,
			CAPIProvidersSecretYAML: e2e.FullProvidersSecret,
			CAPIProvidersYAML:       e2e.FullProviders,
			Data: map[string]string{
				"AWSEncodedCredentials": e2eConfig.GetVariable(e2e.CapaEncodedCredentialsVar),
			},
			WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers"),
			WaitForDeployments: []testenv.NamespaceName{
				{
					Name:      "capa-controller-manager",
					Namespace: "capa-system",
				},
			},
		})
	}

	testenv.RancherDeployIngress(ctx, testenv.RancherDeployIngressInput{
		BootstrapClusterProxy:    setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:           helmBinaryPath,
		IsolatedMode:             isolatedMode,
		NginxIngress:             e2e.NginxIngress,
		NginxIngressNamespace:    e2e.NginxIngressNamespace,
		IngressWaitInterval:      e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher"),
		NgrokApiKey:              e2eConfig.GetVariable(e2e.NgrokApiKeyVar),
		NgrokAuthToken:           e2eConfig.GetVariable(e2e.NgrokAuthTokenVar),
		NgrokPath:                e2eConfig.GetVariable(e2e.NgrokPathVar),
		NgrokRepoName:            e2eConfig.GetVariable(e2e.NgrokRepoNameVar),
		NgrokRepoURL:             e2eConfig.GetVariable(e2e.NgrokUrlVar),
		DefaultIngressClassPatch: e2e.IngressClassPatch,
	})

	testenv.DeployRancher(ctx, testenv.DeployRancherInput{
		BootstrapClusterProxy:  setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:         helmBinaryPath,
		RancherChartRepoName:   e2eConfig.GetVariable(e2e.RancherRepoNameVar),
		RancherChartURL:        e2eConfig.GetVariable(e2e.RancherUrlVar),
		RancherChartPath:       e2eConfig.GetVariable(e2e.RancherPathVar),
		RancherVersion:         e2eConfig.GetVariable(e2e.RancherVersionVar),
		RancherHost:            hostName,
		RancherNamespace:       e2e.RancherNamespace,
		RancherPassword:        e2eConfig.GetVariable(e2e.RancherPasswordVar),
		RancherFeatures:        e2eConfig.GetVariable(e2e.RancherFeaturesVar),
		RancherSettingsPatch:   e2e.RancherSettingPatch,
		RancherWaitInterval:    e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher"),
		ControllerWaitInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers"),
		IsolatedMode:           isolatedMode,
		RancherIngressConfig:   e2e.IngressConfig,
		RancherServicePatch:    e2e.RancherServicePatch,
	})

	giteaResult = testenv.DeployGitea(ctx, testenv.DeployGiteaInput{
		BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:        helmBinaryPath,
		ChartRepoName:         e2eConfig.GetVariable(e2e.GiteaRepoNameVar),
		ChartRepoURL:          e2eConfig.GetVariable(e2e.GiteaRepoURLVar),
		ChartName:             e2eConfig.GetVariable(e2e.GiteaChartNameVar),
		ChartVersion:          e2eConfig.GetVariable(e2e.GiteaChartVersionVar),
		ValuesFilePath:        "../../data/gitea/values.yaml",
		Values: map[string]string{
			"gitea.admin.username": e2eConfig.GetVariable(e2e.GiteaUserNameVar),
			"gitea.admin.password": e2eConfig.GetVariable(e2e.GiteaUserPasswordVar),
		},
		RolloutWaitInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-gitea"),
		ServiceWaitInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-getservice"),
		AuthSecretName:      e2e.AuthSecretName,
		Username:            e2eConfig.GetVariable(e2e.GiteaUserNameVar),
		Password:            e2eConfig.GetVariable(e2e.GiteaUserPasswordVar),
	})
})

var _ = AfterSuite(func() {
	testenv.CleanupTestCluster(ctx, testenv.CleanupTestClusterInput{
		SetupTestClusterResult: *setupClusterResult,
		SkipCleanup:            skipCleanup,
		ArtifactFolder:         artifactFolder,
	})
})
