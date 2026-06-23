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

package import_gitops

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("[Azure] [AKS] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-azure-aks"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                 e2e.LoadE2EConfig(),
			BootstrapClusterProxy:     bootstrapClusterProxy,
			ClusterTemplate:           e2e.CAPIAzureAKSTopology,
			ClusterName:               "cluster-aks",
			ControlPlaneMachineCount:  ptr.To(1),
			WorkerMachineCount:        ptr.To(1),
			LabelNamespace:            true,
			RancherServerURL:          hostName,
			CAPIClusterCreateWaitName: "wait-capz-create-cluster",
			DeleteClusterWaitName:     "wait-aks-delete",
			TopologyNamespace:         topologyNamespace,
			VerifyETCDSize:            true,
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

var _ = Describe("[AWS] [EKS] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-aws-eks"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                 e2e.LoadE2EConfig(),
			BootstrapClusterProxy:     bootstrapClusterProxy,
			ClusterTemplate:           e2e.CAPIAwsEKSTopology,
			ClusterName:               "cluster-eks",
			ControlPlaneMachineCount:  ptr.To(1),
			WorkerMachineCount:        ptr.To(1),
			LabelNamespace:            true,
			RancherServerURL:          hostName,
			CAPIClusterCreateWaitName: "wait-capa-create-cluster",
			DeleteClusterWaitName:     "wait-eks-delete",
			TopologyNamespace:         topologyNamespace,
			VerifyETCDSize:            true,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "aws-cluster-classes-eks",
					Paths:           []string{"examples/clusterclasses/aws/eks"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
		}
	})
})

var _ = Describe("[GCP] [GKE] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-gcp-gke"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                 e2e.LoadE2EConfig(),
			BootstrapClusterProxy:     bootstrapClusterProxy,
			ClusterTemplate:           e2e.CAPIGCPGKETopology,
			ClusterName:               "cluster-gke",
			ControlPlaneMachineCount:  ptr.To(1),
			WorkerMachineCount:        ptr.To(3), // GKE regional clusters (us-west1 has 3 zones) require machine pool replicas to be a multiple of the zone count (1 node per zone × 3 zones = 3 replicas minimum).
			LabelNamespace:            true,
			RancherServerURL:          hostName,
			CAPIClusterCreateWaitName: "wait-capg-create-cluster",
			DeleteClusterWaitName:     "wait-gke-delete",
			TopologyNamespace:         topologyNamespace,
			SkipClusterAvailableWait:  true, // GKE auto-upgrades cause non-empty Available condition message
			VerifyETCDSize:            true,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "gcp-cluster-classes-gke",
					Paths:           []string{"examples/clusterclasses/gcp/gke"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
		}
	})
})
