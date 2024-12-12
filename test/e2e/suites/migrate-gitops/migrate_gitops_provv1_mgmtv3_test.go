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

package migrate_gitops

import (
	_ "embed"

	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"k8s.io/utils/ptr"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
)

var _ = Describe("[Docker] [Kubeadm] - [management.cattle.io/v3] Migrate v1 to management v3 cluster functionality should work", Label(e2e.ShortTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.MigrateToV3UsingGitOpsSpec(ctx, func() specs.MigrateToV3UsingGitOpsSpecInput {
		return specs.MigrateToV3UsingGitOpsSpecInput{
			HelmBinaryPath:                 e2eConfig.GetVariable(e2e.HelmBinaryPathVar),
			ChartPath:                      e2eConfig.GetVariable(e2e.TurtlesPathVar),
			E2EConfig:                      e2eConfig,
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterctlConfigPath:           flagVals.ConfigPath,
			ClusterctlBinaryPath:           e2eConfig.GetVariable(e2e.ClusterctlBinaryPathVar),
			ArtifactFolder:                 artifactsFolder,
			ClusterTemplate:                e2e.CAPIDockerKubeadm,
			ClusterName:                    "clusterv3-migrated",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			GitAddr:                        gitAddress,
			GitAuthSecretName:              e2e.AuthSecretName,
			SkipCleanup:                    false,
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			TestClusterReimport:            true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-rancher",
			DeleteClusterWaitName:          "wait-controllers",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
		}
	})
})
