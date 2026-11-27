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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("[RancherManagedFleet] [GCP] [Kubeadm] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel, e2e.KubeadmTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-gcp-kubeadm"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		const gcpImageFormat = "https://www.googleapis.com/compute/v1/projects/%s/global/images/%s"
		gcpImageFormatted := fmt.Sprintf(gcpImageFormat, e2e.LoadE2EConfig().MustGetVariable(e2e.GCPProjectIDVar), e2e.LoadE2EConfig().MustGetVariable(e2e.GCPImageIDVar))
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIGCPKubeadmTopology,
			ClusterName:                    "cluster-gcp-kubeadm",
			ControlPlaneMachineCount:       new(1),
			WorkerMachineCount:             new(1),
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capg-create-cluster",
			DeleteClusterWaitName:          "wait-gke-delete",
			TopologyNamespace:              topologyNamespace,
			VerifyETCDSize:                 true,
			ValidateFleetAgentWasInstalled: true,
			AdditionalTemplateVariables: map[string]string{
				e2e.GCPImageIDFormattedVar: gcpImageFormatted,
				e2e.ClusterCIDRVar:         "192.168.0.0/16",
				e2e.GCPCCMVersionVar:       "v35.1.3",
			},
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "gcp-cluster-class-kubeadm",
					Paths:           []string{"examples/clusterclasses/gcp/kubeadm"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
			AdditionalDownstreamTemplates: [][]byte{
				e2e.CalicoManifest,
				e2e.CloudProviderGCPManifest,
			},
		}
	})
})

var _ = Describe("[RancherManagedFleet] [GCP] [RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel, e2e.Rke2TestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-gcp-rke2"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIGCPRKE2Topology,
			ClusterName:                    "cluster-gcp-rke2",
			ControlPlaneMachineCount:       new(1),
			WorkerMachineCount:             new(1),
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capg-create-cluster",
			DeleteClusterWaitName:          "wait-gke-delete",
			TopologyNamespace:              topologyNamespace,
			VerifyETCDSize:                 true,
			ValidateFleetAgentWasInstalled: true,
			AdditionalTemplateVariables: map[string]string{
				e2e.ClusterCIDRVar:   "192.168.0.0/16",
				e2e.GCPCCMVersionVar: "v35.1.3",
			},
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "gcp-cluster-class-rke2",
					Paths:           []string{"examples/clusterclasses/gcp/rke2"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
			AdditionalDownstreamTemplates: [][]byte{
				e2e.CloudProviderGCPManifest,
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
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIGCPGKETopology,
			ClusterName:                    "cluster-gke",
			ControlPlaneMachineCount:       new(1),
			WorkerMachineCount:             new(3), // GKE regional clusters (us-west1 has 3 zones) require machine pool replicas to be a multiple of the zone count (1 node per zone × 3 zones = 3 replicas minimum).
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capg-create-cluster",
			DeleteClusterWaitName:          "wait-gke-delete",
			TopologyNamespace:              topologyNamespace,
			SkipClusterAvailableWait:       true, // GKE auto-upgrades cause non-empty Available condition message
			VerifyETCDSize:                 true,
			ValidateFleetAgentWasInstalled: true,
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
