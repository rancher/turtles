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
	_ "embed"

	. "github.com/onsi/ginkgo/v2"
	"k8s.io/utils/ptr"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	turtlesframework "github.com/rancher/turtles/test/framework"
)

var _ = Describe("[Docker] [RKE2] Perform an ETCD backup and restore of the cluster", Label(e2e.ShortTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)

		topologyNamespace = "creategitops-docker-rke2"
	})

	specs.ETCDSnapshotRestore(ctx, func() specs.ETCDSnapshotRestoreInput {
		return specs.ETCDSnapshotRestoreInput{
			E2EConfig:                   e2e.LoadE2EConfig(),
			BootstrapClusterProxy:       bootstrapClusterProxy,
			ClusterTemplate:             e2e.CAPIDockerRKE2Topology,
			ClusterName:                 "etcd-snapshot-restore",
			ControlPlaneMachineCount:    ptr.To[int](1),
			WorkerMachineCount:          ptr.To[int](0),
			RancherServerURL:            hostName,
			CAPIClusterSnapshotWaitName: "wait-snapshot",
			CAPIClusterCreateWaitName:   "wait-snapshot",
			DeleteClusterWaitName:       "wait-controllers",
			TopologyNamespace:           topologyNamespace,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "docker-cluster-class-rke2",
					Paths:           []string{"examples/clusterclasses/docker/rke2"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "docker-cni",
					Paths:           []string{"examples/applications/cni/calico"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
		}
	})
})
