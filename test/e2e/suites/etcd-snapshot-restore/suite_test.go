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

package etcd_snapshot_restore

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/rancher/turtles/test/e2e"
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

	RunSpecs(t, "rancher-turtles-e2e-etcd-restore")
}

var _ = SynchronizedBeforeSuite(
	func() []byte {
		e2eConfig := e2e.LoadE2EConfig()
		e2eConfig.ManagementClusterName = e2eConfig.ManagementClusterName + "-etcd-restore"
		setupClusterResult = testenv.SetupTestCluster(ctx, testenv.SetupTestClusterInput{
			E2EConfig: e2eConfig,
			Scheme:    e2e.InitScheme(),
		})

		testenv.RancherDeployIngress(ctx, testenv.RancherDeployIngressInput{
			BootstrapClusterProxy:    setupClusterResult.BootstrapClusterProxy,
			CustomIngress:            e2e.NginxIngress,
			DefaultIngressClassPatch: e2e.IngressClassPatch,
		})

		rancherHookResult := testenv.DeployRancher(ctx, testenv.DeployRancherInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			RancherPatches:        [][]byte{e2e.RancherSettingPatch},
		})

		testenv.DeployRancherTurtles(ctx, testenv.DeployRancherTurtlesInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			CAPIProvidersYAML:     e2e.CapiProviders,
			AdditionalValues: map[string]string{
				"rancherTurtles.features.day2operations.enabled":                   "true", // enable day2operations feature
				"rancherTurtles.features.day2operations.etcdBackupRestore.enabled": "true", // enable etcdBackupRestore feature
			},
			WaitForDeployments: testenv.DefaultDeployments,
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
		testenv.DumpBootstrapCluster(ctx)

		config := e2e.LoadE2EConfig()
		// skipping error check since it is already done at the beginning of the test in e2e.ValidateE2EConfig()
		skipCleanup, _ := strconv.ParseBool(config.MustGetVariable(e2e.SkipResourceCleanupVar))
		if skipCleanup {
			// add a log line about skipping charts uninstallation and cluster cleanup
			return
		}

		testenv.UninstallRancherTurtles(ctx, testenv.UninstallRancherTurtlesInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
		})

		testenv.CleanupTestCluster(ctx, testenv.CleanupTestClusterInput{
			SetupTestClusterResult: *setupClusterResult,
		})
	},
)
