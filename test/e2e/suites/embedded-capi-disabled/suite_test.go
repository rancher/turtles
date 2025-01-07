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
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/testenv"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// Test suite global vars.
var (
	// hostName is the host name for the Rancher Manager server.
	hostName string

	ctx = context.Background()

	setupClusterResult    *testenv.SetupTestClusterResult
	bootstrapClusterProxy capiframework.ClusterProxy
	gitAddress            string
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	RunSpecs(t, "rancher-turtles-e2e-embedded-capi-disabled")
}

var _ = SynchronizedBeforeSuite(
	func() []byte {
		setupClusterResult = testenv.SetupTestCluster(ctx, testenv.SetupTestClusterInput{
			E2EConfig: e2e.LoadE2EConfig(),
			Scheme:    e2e.InitScheme(),
		})

		testenv.RancherDeployIngress(ctx, testenv.RancherDeployIngressInput{
			BootstrapClusterProxy:    setupClusterResult.BootstrapClusterProxy,
			CustomIngress:            e2e.NginxIngress,
			DefaultIngressClassPatch: e2e.IngressClassPatch,
		})

		// NOTE: deploy Rancher first with the embedded-cluster-api feature disabled.
		// and the deploy Rancher Turtles.
		rancherHookResult := testenv.DeployRancher(ctx, testenv.DeployRancherInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			RancherFeatures:       "embedded-cluster-api=false",
			RancherPatches:        [][]byte{e2e.RancherSettingPatch},
		})

		rtInput := testenv.DeployRancherTurtlesInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			CAPIProvidersYAML:     e2e.CapiProviders,
			AdditionalValues: map[string]string{
				"rancherTurtles.features.embedded-capi.disabled":       "false",
				"rancherTurtles.features.managementv3-cluster.enabled": "false",
			},
			WaitForDeployments: testenv.DefaultDeployments,
		}

		testenv.DeployRancherTurtles(ctx, rtInput)

		// NOTE: there are no short or local tests in this suite
		By("Deploying additional infrastructure providers")

		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.AWSProviderSecret,
				e2e.AzureIdentitySecret,
				e2e.GCPProviderSecret,
			},
			CAPIProvidersYAML: e2e.FullProviders,
			WaitForDeployments: []testenv.NamespaceName{
				{
					Name:      "capa-controller-manager",
					Namespace: "capa-system",
				},
				{
					Name:      "capz-controller-manager",
					Namespace: "capz-system",
				},
				{
					Name:      "capg-controller-manager",
					Namespace: "capg-system",
				},
			},
		})

		giteaResult := testenv.DeployGitea(ctx, testenv.DeployGiteaInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			ValuesFile:            e2e.GiteaValues,
			CustomIngressConfig:   e2e.GiteaIngress,
		})

		data, err := json.Marshal(e2e.Setup{
			ClusterName:     setupClusterResult.ClusterName,
			KubeconfigPath:  setupClusterResult.KubeconfigPath,
			GitAddress:      giteaResult.GitAddress,
			RancherHostname: rancherHookResult.Hostname,
		})
		Expect(err).ToNot(HaveOccurred())
		return data
	},
	func(sharedData []byte) {
		setup := e2e.Setup{}
		Expect(json.Unmarshal(sharedData, &setup)).To(Succeed())

		gitAddress = setup.GitAddress
		hostName = setup.RancherHostname

		bootstrapClusterProxy = capiframework.NewClusterProxy(setup.ClusterName, setup.KubeconfigPath, e2e.InitScheme(), capiframework.WithMachineLogCollector(capiframework.DockerLogCollector{}))
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "cluster proxy should not be nil")
	},
)

var _ = SynchronizedAfterSuite(
	func() {
	},
	func() {
		testenv.UninstallGitea(ctx, testenv.UninstallGiteaInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
		})

		testenv.CleanupTestCluster(ctx, testenv.CleanupTestClusterInput{
			SetupTestClusterResult: *setupClusterResult,
		})
	},
)
