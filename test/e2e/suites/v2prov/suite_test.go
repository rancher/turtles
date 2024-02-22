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

package v2prov

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/framework"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"
)

// Test suite flags.
var (
	flagVals *e2e.FlagValues
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
)

func init() {
	flagVals = &e2e.FlagValues{}
	e2e.InitFlags(flagVals)
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	ctrl.SetLogger(klog.Background())

	RunSpecs(t, "rancher-turtles-e2e-v2prov")
}

var _ = BeforeSuite(func() {
	Expect(flagVals.ConfigPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(flagVals.ArtifactFolder, 0o755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", flagVals.ArtifactFolder)
	Expect(flagVals.HelmBinaryPath).To(BeAnExistingFile(), "Invalid test suite argument. helm-binary-path should be an existing file.")
	Expect(flagVals.ChartPath).To(BeAnExistingFile(), "Invalid test suite argument. chart-path should be an existing file.")

	By(fmt.Sprintf("Loading the e2e test configuration from %q", flagVals.ConfigPath))
	e2eConfig = e2e.LoadE2EConfig(flagVals.ConfigPath)

	dockerUsername := ""
	dockerPassword := ""
	if flagVals.UseEKS {
		Expect(flagVals.IsolatedMode).To(BeFalse(), "You cannot use eks with isolated")
		dockerUsername = os.Getenv("GITHUB_USERNAME")
		Expect(dockerUsername).NotTo(BeEmpty(), "Github username is required")
		dockerPassword = os.Getenv("GITHUB_TOKEN")
		Expect(dockerPassword).NotTo(BeEmpty(), "Github token is required")
	}

	By(fmt.Sprintf("Creating a clusterctl config into %q", flagVals.ArtifactFolder))
	clusterctlConfigPath = e2e.CreateClusterctlLocalRepository(ctx, e2eConfig, filepath.Join(flagVals.ArtifactFolder, "repository"))

	hostName = e2eConfig.GetVariable(e2e.RancherHostnameVar)

	setupClusterResult = testenv.SetupTestCluster(ctx, testenv.SetupTestClusterInput{
		UseExistingCluster:   flagVals.UseExistingCluster,
		E2EConfig:            e2eConfig,
		ClusterctlConfigPath: clusterctlConfigPath,
		Scheme:               e2e.InitScheme(),
		ArtifactFolder:       flagVals.ArtifactFolder,
		KubernetesVersion:    e2eConfig.GetVariable(e2e.KubernetesManagementVersionVar),
		IsolatedMode:         flagVals.IsolatedMode,
		HelmBinaryPath:       flagVals.HelmBinaryPath,
		UseEKS:               flagVals.UseEKS,
	})

	if flagVals.IsolatedMode {
		hostName = setupClusterResult.IsolatedHostName
	}

	testenv.RancherDeployIngress(ctx, testenv.RancherDeployIngressInput{
		BootstrapClusterProxy:    setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:           flagVals.HelmBinaryPath,
		HelmExtraValuesPath:      filepath.Join(flagVals.HelmExtraValuesDir, "deploy-rancher-ingress.yaml"),
		IsolatedMode:             flagVals.IsolatedMode,
		UseEKS:                   flagVals.UseEKS,
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

	if flagVals.UseEKS {
		By("Getting ingress hostname")
		svcRes := &testenv.WaitForServiceIngressHostnameResult{}
		testenv.WaitForServiceIngressHostname(ctx, testenv.WaitForServiceIngressHostnameInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			ServiceName:           "ingress-nginx-controller",
			ServiceNamespace:      "ingress-nginx",
			IngressWaitInterval:   e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher"),
		}, svcRes)
		hostName = svcRes.Hostname

		By("Deploying ghcr details")
		framework.CreateDockerRegistrySecret(ctx, framework.CreateDockerRegistrySecretInput{
			Name:                  "regcred",
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			Namespace:             "rancher-turtles-system",
			DockerServer:          "https://ghcr.io",
			DockerUsername:        dockerUsername,
			DockerPassword:        dockerPassword,
		})
	}

	rancherInput := testenv.DeployRancherInput{
		BootstrapClusterProxy:  setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:         flagVals.HelmBinaryPath,
		HelmExtraValuesPath:    filepath.Join(flagVals.HelmExtraValuesDir, "deploy-rancher.yaml"),
		InstallCertManager:     true,
		CertManagerChartPath:   e2eConfig.GetVariable(e2e.CertManagerPathVar),
		CertManagerUrl:         e2eConfig.GetVariable(e2e.CertManagerUrlVar),
		CertManagerRepoName:    e2eConfig.GetVariable(e2e.CertManagerRepoNameVar),
		RancherChartRepoName:   e2eConfig.GetVariable(e2e.RancherRepoNameVar),
		RancherChartURL:        e2eConfig.GetVariable(e2e.RancherUrlVar),
		RancherChartPath:       e2eConfig.GetVariable(e2e.RancherPathVar),
		RancherVersion:         e2eConfig.GetVariable(e2e.RancherVersionVar),
		Development:            true,
		RancherHost:            hostName,
		RancherNamespace:       e2e.RancherNamespace,
		RancherPassword:        e2eConfig.GetVariable(e2e.RancherPasswordVar),
		RancherFeatures:        e2eConfig.GetVariable(e2e.RancherFeaturesVar),
		RancherPatches:         [][]byte{e2e.RancherSettingPatch},
		RancherWaitInterval:    e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher"),
		ControllerWaitInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers"),
		Variables:              e2eConfig.Variables,
	}
	if !flagVals.IsolatedMode && !flagVals.UseEKS {
		// i.e. we are using ngrok locally
		rancherInput.RancherIngressConfig = e2e.IngressConfig
		rancherInput.RancherServicePatch = e2e.RancherServicePatch
	}
	if flagVals.UseEKS {
		rancherInput.RancherIngressClassName = "nginx"
	}
	testenv.DeployRancher(ctx, rancherInput)

	rtInput := testenv.DeployRancherTurtlesInput{
		BootstrapClusterProxy:        setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:               flagVals.HelmBinaryPath,
		ChartPath:                    flagVals.ChartPath,
		CAPIProvidersYAML:            e2e.CapiProviders,
		Namespace:                    turtlesframework.DefaultRancherTurtlesNamespace,
		Image:                        fmt.Sprintf("ghcr.io/rancher/turtles-e2e-%s", runtime.GOARCH),
		Tag:                          "v0.0.1",
		WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers"),
		AdditionalValues:             map[string]string{},
	}
	if flagVals.UseEKS {
		rtInput.AdditionalValues["rancherTurtles.imagePullSecrets"] = "{regcred}"
		rtInput.AdditionalValues["rancherTurtles.imagePullPolicy"] = "IfNotPresent"
	} else {
		// NOTE: this was the default previously in the chart locally and ok as
		// we where loading the image into kind manually.
		rtInput.AdditionalValues["rancherTurtles.imagePullPolicy"] = "Never"
	}
	testenv.DeployRancherTurtles(ctx, rtInput)

	testenv.RestartRancher(ctx, testenv.RestartRancherInput{
		BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
		RancherNamespace:      e2e.RancherNamespace,
		RancherWaitInterval:   e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher"),
	})
})

var _ = AfterSuite(func() {
	testenv.CleanupTestCluster(ctx, testenv.CleanupTestClusterInput{
		SetupTestClusterResult: *setupClusterResult,
		SkipCleanup:            flagVals.SkipCleanup,
		ArtifactFolder:         flagVals.ArtifactFolder,
	})
})
