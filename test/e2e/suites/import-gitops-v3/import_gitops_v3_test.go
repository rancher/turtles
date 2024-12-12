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

package import_gitops_v3

import (
	. "github.com/onsi/ginkgo/v2"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"k8s.io/utils/ptr"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
)

var _ = Describe("[Docker] [Kubeadm] - [management.cattle.io/v3] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.ShortTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2eConfig,
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterctlConfigPath:           flagVals.ConfigPath,
			ClusterctlBinaryPath:           e2eConfig.GetVariable(e2e.ClusterctlBinaryPathVar),
			ArtifactFolder:                 artifactsFolder,
			ClusterTemplate:                e2e.CAPIDockerKubeadm,
			ClusterName:                    "clusterv3-auto-import-kubeadm",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
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

var _ = Describe("[Docker] [RKE2] - [management.cattle.io/v3] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.LocalTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2eConfig,
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterctlConfigPath:           flagVals.ConfigPath,
			ClusterctlBinaryPath:           e2eConfig.GetVariable(e2e.ClusterctlBinaryPathVar),
			ArtifactFolder:                 artifactsFolder,
			ClusterTemplate:                e2e.CAPIDockerRKE2,
			ClusterName:                    "clusterv3-auto-import-rke2",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
			GitAddr:                        gitAddress,
			GitAuthSecretName:              e2e.AuthSecretName,
			SkipCleanup:                    false,
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			TestClusterReimport:            false,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-rancher",
			DeleteClusterWaitName:          "wait-controllers",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
		}
	})
})

var _ = Describe("[Azure] [AKS] - [management.cattle.io/v3] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2eConfig,
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterctlConfigPath:           flagVals.ConfigPath,
			ArtifactFolder:                 artifactsFolder,
			ClusterTemplate:                e2e.CAPIAzureAKSTopology,
			ClusterName:                    "highlander-e2e-topology",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
			GitAddr:                        gitAddress,
			GitAuthSecretName:              e2e.AuthSecretName,
			SkipCleanup:                    false,
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capz-create-cluster",
			DeleteClusterWaitName:          "wait-aks-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
		}
	})
})

var _ = Describe("[AWS] [EKS] - [management.cattle.io/v3] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2eConfig,
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterctlConfigPath:           flagVals.ConfigPath,
			ClusterctlBinaryPath:           e2eConfig.GetVariable(e2e.ClusterctlBinaryPathVar),
			ArtifactFolder:                 artifactsFolder,
			ClusterTemplate:                e2e.CAPIAwsEKSMMP,
			ClusterName:                    "clusterv3-eks",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
			GitAddr:                        gitAddress,
			GitAuthSecretName:              e2e.AuthSecretName,
			SkipCleanup:                    false,
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capa-create-cluster",
			DeleteClusterWaitName:          "wait-eks-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
		}
	})
})

var _ = Describe("[GCP] [GKE] - [management.cattle.io/v3] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2eConfig,
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterctlConfigPath:           flagVals.ConfigPath,
			ClusterctlBinaryPath:           e2eConfig.GetVariable(e2e.ClusterctlBinaryPathVar),
			ArtifactFolder:                 artifactsFolder,
			ClusterTemplate:                e2e.CAPIGCPGKE,
			ClusterName:                    "clusterv3-gke",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
			GitAddr:                        gitAddress,
			GitAuthSecretName:              e2e.AuthSecretName,
			SkipCleanup:                    false,
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capg-create-cluster",
			DeleteClusterWaitName:          "wait-gke-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
			IsGCPCluster:                   true,
		}
	})
})
