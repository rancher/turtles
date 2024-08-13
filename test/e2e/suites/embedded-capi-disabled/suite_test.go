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

package embedded_capi_disabled

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/framework"
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

	artifactsFolder string

	ctx = context.Background()

	setupClusterResult *testenv.SetupTestClusterResult
	giteaResult        *testenv.DeployGiteaResult
)

func init() {
	flagVals = &e2e.FlagValues{}
	e2e.InitFlags(flagVals)
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	ctrl.SetLogger(klog.Background())

	RunSpecs(t, "rancher-turtles-e2e-embedded-capi-disabled")
}

var _ = BeforeSuite(func() {
	By(fmt.Sprintf("Loading the e2e test configuration from %q", flagVals.ConfigPath))
	Expect(flagVals.ConfigPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	e2eConfig = e2e.LoadE2EConfig(flagVals.ConfigPath)
	e2e.ValidateE2EConfig(e2eConfig)

	artifactsFolder = e2eConfig.GetVariable(e2e.ArtifactsFolderVar)

	preSetupOutput := testenv.PreManagementClusterSetupHook(e2eConfig)

	By(fmt.Sprintf("Creating a clusterctl config into %q", artifactsFolder))
	clusterctlConfigPath = e2e.CreateClusterctlLocalRepository(ctx, e2eConfig, filepath.Join(artifactsFolder, "repository"))

	useExistingCluter, err := strconv.ParseBool(e2eConfig.GetVariable(e2e.UseExistingClusterVar))
	Expect(err).ToNot(HaveOccurred(), "Failed to parse the USE_EXISTING_CLUSTER variable")

	setupClusterResult = testenv.SetupTestCluster(ctx, testenv.SetupTestClusterInput{
		UseExistingCluster:    useExistingCluter,
		E2EConfig:             e2eConfig,
		ClusterctlConfigPath:  clusterctlConfigPath,
		Scheme:                e2e.InitScheme(),
		ArtifactFolder:        artifactsFolder,
		KubernetesVersion:     e2eConfig.GetVariable(e2e.KubernetesManagementVersionVar),
		HelmBinaryPath:        e2eConfig.GetVariable(e2e.HelmBinaryPathVar),
		CustomClusterProvider: preSetupOutput.CustomClusterProvider,
	})

	testenv.RancherDeployIngress(ctx, testenv.RancherDeployIngressInput{
		BootstrapClusterProxy:    setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:           e2eConfig.GetVariable(e2e.HelmBinaryPathVar),
		HelmExtraValuesPath:      filepath.Join(e2eConfig.GetVariable(e2e.HelmExtraValuesFolderVar), "deploy-rancher-ingress.yaml"),
		IngressType:              preSetupOutput.IngressType,
		CustomIngress:            e2e.NginxIngress,
		CustomIngressNamespace:   e2e.NginxIngressNamespace,
		CustomIngressDeployment:  e2e.NginxIngressDeployment,
		IngressWaitInterval:      e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher"),
		NgrokApiKey:              e2eConfig.GetVariable(e2e.NgrokApiKeyVar),
		NgrokAuthToken:           e2eConfig.GetVariable(e2e.NgrokAuthTokenVar),
		NgrokPath:                e2eConfig.GetVariable(e2e.NgrokPathVar),
		NgrokRepoName:            e2eConfig.GetVariable(e2e.NgrokRepoNameVar),
		NgrokRepoURL:             e2eConfig.GetVariable(e2e.NgrokUrlVar),
		DefaultIngressClassPatch: e2e.IngressClassPatch,
	})

	// NOTE: deploy Rancher first with the embedded-cluster-api feature disabled.
	// and the deploy Rancher Turtles.
	rancherInput := testenv.DeployRancherInput{
		BootstrapClusterProxy:  setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:         e2eConfig.GetVariable(e2e.HelmBinaryPathVar),
		HelmExtraValuesPath:    filepath.Join(e2eConfig.GetVariable(e2e.HelmExtraValuesFolderVar), "deploy-rancher.yaml"),
		InstallCertManager:     true,
		CertManagerChartPath:   e2eConfig.GetVariable(e2e.CertManagerPathVar),
		CertManagerUrl:         e2eConfig.GetVariable(e2e.CertManagerUrlVar),
		CertManagerRepoName:    e2eConfig.GetVariable(e2e.CertManagerRepoNameVar),
		RancherChartRepoName:   e2eConfig.GetVariable(e2e.RancherRepoNameVar),
		RancherChartURL:        e2eConfig.GetVariable(e2e.RancherUrlVar),
		RancherChartPath:       e2eConfig.GetVariable(e2e.RancherPathVar),
		RancherVersion:         e2eConfig.GetVariable(e2e.RancherVersionVar),
		RancherNamespace:       e2e.RancherNamespace,
		RancherPassword:        e2eConfig.GetVariable(e2e.RancherPasswordVar),
		RancherFeatures:        "embedded-cluster-api=false",
		RancherPatches:         [][]byte{e2e.RancherSettingPatch},
		RancherWaitInterval:    e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher"),
		ControllerWaitInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers"),
		Variables:              e2eConfig.Variables,
	}

	rancherHookResult := testenv.PreRancherInstallHook(
		&testenv.PreRancherInstallHookInput{
			Ctx:                ctx,
			RancherInput:       &rancherInput,
			E2EConfig:          e2eConfig,
			SetupClusterResult: setupClusterResult,
			PreSetupOutput:     preSetupOutput,
		})

	hostName = rancherHookResult.HostName

	testenv.DeployRancher(ctx, rancherInput)

	rtInput := testenv.DeployRancherTurtlesInput{
		BootstrapClusterProxy:        setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:               e2eConfig.GetVariable(e2e.HelmBinaryPathVar),
		TurtlesChartPath:             e2eConfig.GetVariable(e2e.TurtlesPathVar),
		CAPIProvidersYAML:            e2e.CapiProviders,
		Namespace:                    framework.DefaultRancherTurtlesNamespace,
		Image:                        fmt.Sprintf("ghcr.io/rancher/turtles-e2e-%s", runtime.GOARCH),
		Tag:                          e2eConfig.GetVariable(e2e.TurtlesVersionVar),
		WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers"),
		AdditionalValues: map[string]string{
			"cluster-api-operator.cert-manager.enabled":            "false",
			"rancherTurtles.features.embedded-capi.disabled":       "false",
			"rancherTurtles.features.managementv3-cluster.enabled": "false",
		},
	}

	testenv.PreRancherTurtlesInstallHook(&rtInput, e2eConfig)

	testenv.DeployRancherTurtles(ctx, rtInput)

	// NOTE: there are no short or local tests in this suite
	By("Deploying additional infrastructure providers")
	awsCreds := e2eConfig.GetVariable(e2e.CapaEncodedCredentialsVar)
	Expect(awsCreds).ToNot(BeEmpty(), "AWS creds required for full test")

	testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
		BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
		CAPIProvidersSecretsYAML: [][]byte{
			e2e.AWSProviderSecret,
			e2e.AzureIdentitySecret,
		},
		CAPIProvidersYAML: e2e.FullProviders,
		TemplateData: map[string]string{
			"AWSEncodedCredentials": awsCreds,
		},
		WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-controllers"),
		WaitForDeployments: []testenv.NamespaceName{
			{
				Name:      "capa-controller-manager",
				Namespace: "capa-system",
			},
			{
				Name:      "capz-controller-manager",
				Namespace: "capz-system",
			},
		},
	})

	giteaInput := testenv.DeployGiteaInput{
		BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:        e2eConfig.GetVariable(e2e.HelmBinaryPathVar),
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
		ServiceWaitInterval: e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-gitea-service"),
		AuthSecretName:      e2e.AuthSecretName,
		Username:            e2eConfig.GetVariable(e2e.GiteaUserNameVar),
		Password:            e2eConfig.GetVariable(e2e.GiteaUserPasswordVar),
		CustomIngressConfig: e2e.GiteaIngress,
		Variables:           e2eConfig.Variables,
	}

	testenv.PreGiteaInstallHook(&giteaInput, e2eConfig)

	giteaResult = testenv.DeployGitea(ctx, giteaInput)
})

var _ = AfterSuite(func() {
	testenv.UninstallGitea(ctx, testenv.UninstallGiteaInput{
		BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
		HelmBinaryPath:        e2eConfig.GetVariable(e2e.HelmBinaryPathVar),
		DeleteWaitInterval:    e2eConfig.GetIntervals(setupClusterResult.BootstrapClusterProxy.GetName(), "wait-gitea-uninstall"),
	})

	skipCleanup, err := strconv.ParseBool(e2eConfig.GetVariable(e2e.SkipResourceCleanupVar))
	Expect(err).ToNot(HaveOccurred(), "Failed to parse the SKIP_RESOURCE_CLEANUP variable")

	testenv.CleanupTestCluster(ctx, testenv.CleanupTestClusterInput{
		SetupTestClusterResult: *setupClusterResult,
		SkipCleanup:            skipCleanup,
		ArtifactFolder:         artifactsFolder,
	})
})
