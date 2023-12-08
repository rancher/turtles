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

package update_labels

import (
	"context"
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

	RunSpecs(t, "rancher-turtles-e2e-import-gitops")
}

var _ = BeforeSuite(func() {
	Expect(flagVals.ConfigPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(flagVals.ArtifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", flagVals.ArtifactFolder)
	Expect(flagVals.HelmBinaryPath).To(BeAnExistingFile(), "Invalid test suite argument. helm-binary-path should be an existing file.")
	Expect(flagVals.ChartPath).To(BeAnExistingFile(), "Invalid test suite argument. chart-path should be an existing file.")

	By(fmt.Sprintf("Loading the e2e test configuration from %q", flagVals.ConfigPath))
	e2eConfig = e2e.LoadE2EConfig(flagVals.ConfigPath)

	By(fmt.Sprintf("Creating a clusterctl config into %q", flagVals.ArtifactFolder))
	clusterctlConfigPath = e2e.CreateClusterctlLocalRepository(ctx, e2eConfig, filepath.Join(flagVals.ArtifactFolder, "repository"))

	hostName = e2eConfig.GetVariable(e2e.RancherHostnameVar)

	setupClusterResult = testenv.SetupTestCluster(ctx, testenv.SetupTestClusterInput{
		UseExistingCluster:   flagVals.UseExistingCluster,
		E2EConfig:            e2eConfig,
		ClusterctlConfigPath: clusterctlConfigPath,
		Scheme:               e2e.InitScheme(),
		ArtifactFolder:       flagVals.ArtifactFolder,
		Hostname:             hostName,
		KubernetesVersion:    e2eConfig.GetVariable(e2e.KubernetesVersionVar),
		IsolatedMode:         flagVals.IsolatedMode,
		HelmBinaryPath:       flagVals.HelmBinaryPath,
	})

	if flagVals.IsolatedMode {
		hostName = setupClusterResult.IsolatedHostName
	}

	testenv.DeployRancherTurtles(ctx, testenv.DeployRancherTurtlesInput{
		BootstrapClusterProxy:        setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:               flagVals.HelmBinaryPath,
		ChartPath:                    flagVals.ChartPath,
		CAPIProvidersYAML:            e2e.CapiProviders,
		Namespace:                    turtlesframework.DefaultRancherTurtlesNamespace,
		Image:                        "ghcr.io/rancher-sandbox/rancher-turtles-amd64",
		Tag:                          "v0.0.1",
		WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers"),
		AdditionalValues: map[string]string{
			"cluster-api-operator.cluster-api.version":          "v1.5.2",
			"cluster-api-operator.cert-manager.enabled":         "true",
			"rancherTurtles.features.embedded-capi.disabled":    "false",
			"rancherTurtles.features.rancher-webhook.cleanup":   "false",
			"rancherTurtles.features.rancher-kubeconfigs.label": "true", // force to be true even if the default in teh chart changes
		},
	})

	testenv.RancherDeployIngress(ctx, testenv.RancherDeployIngressInput{
		BootstrapClusterProxy:    setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:           flagVals.HelmBinaryPath,
		HelmExtraValuesPath:      filepath.Join(flagVals.HelmExtraValuesDir, "deploy-rancher-ingress.yaml"),
		IsolatedMode:             flagVals.IsolatedMode,
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
		HelmBinaryPath:         flagVals.HelmBinaryPath,
		HelmExtraValuesPath:    filepath.Join(flagVals.HelmExtraValuesDir, "deploy-rancher.yaml"),
		InstallCertManager:     false,
		RancherChartRepoName:   "rancher-latest",
		RancherChartURL:        "https://releases.rancher.com/server-charts/latest",
		RancherChartPath:       "rancher-latest/rancher",
		RancherVersion:         "2.7.7",
		RancherHost:            hostName,
		RancherNamespace:       e2e.RancherNamespace,
		RancherPassword:        e2eConfig.GetVariable(e2e.RancherPasswordVar),
		RancherFeatures:        "embedded-cluster-api=false",
		RancherPatches:         [][]byte{e2e.RancherSettingPatch},
		RancherWaitInterval:    e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher"),
		ControllerWaitInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers"),
		IsolatedMode:           flagVals.IsolatedMode,
		RancherIngressConfig:   e2e.IngressConfig,
		RancherServicePatch:    e2e.RancherServicePatch,
		Variables:              e2eConfig.Variables,
	})
})

var _ = AfterSuite(func() {
	testenv.CleanupTestCluster(ctx, testenv.CleanupTestClusterInput{
		SetupTestClusterResult: *setupClusterResult,
		SkipCleanup:            flagVals.SkipCleanup,
		ArtifactFolder:         flagVals.ArtifactFolder,
	})
})
