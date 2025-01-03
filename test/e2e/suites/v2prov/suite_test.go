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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/testenv"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
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

	artifactsFolder string

	setupClusterResult    *testenv.SetupTestClusterResult
	bootstrapClusterProxy capiframework.ClusterProxy
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

var _ = SynchronizedBeforeSuite(
	func() []byte {
		By(fmt.Sprintf("Loading the e2e test configuration from %q", flagVals.ConfigPath))
		Expect(flagVals.ConfigPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
		e2eConfig = e2e.LoadE2EConfig(flagVals.ConfigPath)
		e2e.ValidateE2EConfig(e2eConfig)

		artifactsFolder = e2eConfig.GetVariable(e2e.ArtifactsFolderVar)

		preSetupOutput := testenv.PreManagementClusterSetupHook(&testenv.PreRancherInstallHookInput{})

		By(fmt.Sprintf("Creating a clusterctl config into %q", artifactsFolder))
		clusterctlConfigPath = e2e.CreateClusterctlLocalRepository(ctx, e2eConfig, filepath.Join(artifactsFolder, "repository"))

		setupClusterResult = testenv.SetupTestCluster(ctx, testenv.SetupTestClusterInput{
			E2EConfig:             e2eConfig,
			ClusterctlConfigPath:  clusterctlConfigPath,
			Scheme:                e2e.InitScheme(),
			CustomClusterProvider: preSetupOutput.CustomClusterProvider,
		})

		testenv.RancherDeployIngress(ctx, testenv.RancherDeployIngressInput{
			BootstrapClusterProxy:    setupClusterResult.BootstrapClusterProxy,
			CustomIngress:            e2e.NginxIngress,
			DefaultIngressClassPatch: e2e.IngressClassPatch,
		})

		rancherInput := testenv.DeployRancherInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			Development:           true,
			RancherPatches:        [][]byte{e2e.RancherSettingPatch},
		}

		rancherHookResult := testenv.DeployRancher(ctx, rancherInput)

		rtInput := testenv.DeployRancherTurtlesInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			CAPIProvidersYAML:     e2e.CapiProviders,
			Image:                 "ghcr.io/rancher/turtles-e2e",
			Tag:                   e2eConfig.GetVariable(e2e.TurtlesVersionVar),
		}

		testenv.DeployRancherTurtles(ctx, rtInput)

		testenv.RestartRancher(ctx, testenv.RestartRancherInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
		})

		// encode the e2e config into the byte array.
		var configBuf bytes.Buffer
		enc := gob.NewEncoder(&configBuf)
		Expect(enc.Encode(e2eConfig)).To(Succeed())
		configStr := base64.StdEncoding.EncodeToString(configBuf.Bytes())

		return []byte(
			strings.Join([]string{
				setupClusterResult.ClusterName,
				setupClusterResult.KubeconfigPath,
				configStr,
				rancherHookResult.HostName,
			}, ","),
		)
	},
	func(sharedData []byte) {
		parts := strings.Split(string(sharedData), ",")
		Expect(parts).To(HaveLen(4))

		clusterName := parts[0]
		kubeconfigPath := parts[1]

		configBytes, err := base64.StdEncoding.DecodeString(parts[2])
		Expect(err).NotTo(HaveOccurred())
		buf := bytes.NewBuffer(configBytes)
		dec := gob.NewDecoder(buf)
		Expect(dec.Decode(&e2eConfig)).To(Succeed())

		bootstrapClusterProxy = capiframework.NewClusterProxy(string(clusterName), string(kubeconfigPath), e2e.InitScheme(), capiframework.WithMachineLogCollector(capiframework.DockerLogCollector{}))
		Expect(bootstrapClusterProxy).ToNot(BeNil(), "cluster proxy should not be nil")

		hostName = parts[3]
	},
)

var _ = SynchronizedAfterSuite(
	func() {
	},
	func() {
		testenv.CleanupTestCluster(ctx, testenv.CleanupTestClusterInput{
			SetupTestClusterResult: *setupClusterResult,
		})
	},
)
