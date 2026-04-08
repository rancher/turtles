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

package rancher_managed_fleet

import (
	. "github.com/onsi/ginkgo/v2"
	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("[RancherManagedFleet] [Docker] [RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.ShortTestLabel, e2e.Rke2TestLabel, e2e.RancherManagedFleetLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-ranchermanagedfleet-docker-rke2"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIDockerRKE2Topology,
			ClusterName:                    "cluster-docker-rke2",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			LabelNamespace:                 true,
			TestClusterReimport:            false,
			RancherManagedFleet:            true,
			ValidateFleetAgentWasInstalled: true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-rancher",
			DeleteClusterWaitName:          "wait-controllers",
			TopologyNamespace:              topologyNamespace,
			AdditionalTemplateVariables: map[string]string{
				"RKE2_CNI": `""`,
			},
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "docker-cluster-classes-regular",
					Paths:           []string{"examples/clusterclasses/docker/rke2"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:                   "lb-configmap",
					Paths:                  []string{"examples/applications/lb/docker"},
					ClusterProxy:           bootstrapClusterProxy,
					TargetClusterNamespace: true,
				},
			},
		}
	})
})
