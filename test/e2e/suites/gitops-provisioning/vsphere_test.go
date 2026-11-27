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

var _ = Describe("[RancherManagedFleet] [vSphere] [RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.VsphereTestLabel, e2e.Rke2TestLabel, e2e.RancherManagedFleetLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-vsphere-rke2"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIvSphereRKE2Topology,
			TopologyNamespace:              topologyNamespace,
			ClusterName:                    "cluster-vsphere-rke2",
			ControlPlaneMachineCount:       new(3), // minimum 3 replicas for CSI controller
			WorkerMachineCount:             new(1),
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			ValidateFleetAgentWasInstalled: true,
			CAPIClusterCreateWaitName:      "wait-capv-create-cluster",
			DeleteClusterWaitName:          "wait-vsphere-delete",
			VerifyETCDSize:                 true,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "vsphere-cluster-classes-rke2",
					TargetNamespace: topologyNamespace,
					Paths:           []string{"examples/clusterclasses/vsphere/rke2"},
					ClusterProxy:    bootstrapClusterProxy,
				},
			},
		}
	})
})

var _ = Describe("[RancherManagedFleet] [vSphere] [Kubeadm] Create and delete CAPI cluster from cluster class", Label(e2e.VsphereTestLabel, e2e.KubeadmTestLabel, e2e.RancherManagedFleetLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-vsphere-kubeadm"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIvSphereKubeadmTopology,
			TopologyNamespace:              topologyNamespace,
			ClusterName:                    "cluster-vsphere-kubeadm",
			ControlPlaneMachineCount:       new(3), // minimum 3 replicas for CSI controller
			WorkerMachineCount:             new(1),
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capv-create-cluster",
			DeleteClusterWaitName:          "wait-vsphere-delete",
			VerifyETCDSize:                 true,
			ValidateFleetAgentWasInstalled: true,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "vsphere-cluster-classes-kubeadm",
					TargetNamespace: topologyNamespace,
					Paths:           []string{"examples/clusterclasses/vsphere/kubeadm"},
					ClusterProxy:    bootstrapClusterProxy,
				},
			},
			AdditionalDownstreamTemplates: [][]byte{
				e2e.CalicoManifest,
				e2e.CloudProvidervSphereManifest,
				e2e.CSIvSphereManifest,
			},
		}
	})
})
