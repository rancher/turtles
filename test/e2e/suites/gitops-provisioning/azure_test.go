//go:build e2e
// +build e2e

/*
Copyright © 2023 - 2026 SUSE LLC

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

package gitopsprov

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("[RancherManagedFleet] [Azure] [RKE2] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel, e2e.Rke2TestLabel, e2e.RancherManagedFleetLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-azure-rke2"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAzureRKE2Topology,
			ClusterName:                    "cluster-azure-rke2",
			ControlPlaneMachineCount:       new(1),
			WorkerMachineCount:             new(1),
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			ValidateFleetAgentWasInstalled: true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capz-create-cluster",
			DeleteClusterWaitName:          "wait-aks-delete",
			TopologyNamespace:              topologyNamespace,
			VerifyETCDSize:                 true,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "azure-cluster-class-rke2",
					Paths:           []string{"examples/clusterclasses/azure/rke2"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
		}
	})
})

var _ = Describe("[RancherManagedFleet] [Azure] [Kubeadm] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel, e2e.KubeadmTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-azure-kubeadm"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAzureKubeadmTopology,
			ClusterName:                    "cluster-azure-kubeadm",
			ControlPlaneMachineCount:       new(1),
			WorkerMachineCount:             new(1),
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capz-create-cluster",
			DeleteClusterWaitName:          "wait-aks-delete",
			TopologyNamespace:              topologyNamespace,
			VerifyETCDSize:                 true,
			ValidateFleetAgentWasInstalled: true,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "azure-cluster-class-kubeadm",
					Paths:           []string{"examples/clusterclasses/azure/kubeadm"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
			AdditionalDownstreamTemplates: [][]byte{
				e2e.CalicoManifest,
				e2e.CloudProviderAzureManifest,
			},
			AdditionalTemplateVariables: map[string]string{
				e2e.ClusterCIDRVar: "192.168.0.0/16",
			},
		}
	})
})

var _ = Describe("[Azure] [AKS] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-azure-aks"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAzureAKSTopology,
			ClusterName:                    "cluster-aks",
			ControlPlaneMachineCount:       new(1),
			WorkerMachineCount:             new(1),
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capz-create-cluster",
			DeleteClusterWaitName:          "wait-aks-delete",
			TopologyNamespace:              topologyNamespace,
			VerifyETCDSize:                 true,
			ValidateFleetAgentWasInstalled: true,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "azure-cluster-classes-aks",
					Paths:           []string{"examples/clusterclasses/azure/aks"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
		}
	})
})
