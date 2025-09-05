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

package import_no_cert_manager

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("[Docker] [RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.ShortTestLabel, e2e.Rke2TestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "no-cert-manager-docker-rke2"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersYAML: [][]byte{
				e2e.CapiProviders,
			},
			WaitForDeployments: testenv.DefaultDeployments,
		})

		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIDockerRKE2Topology,
			ClusterName:                    "no-cert-manager-docker-rke2",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			LabelNamespace:                 true,
			TestClusterReimport:            false,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-rancher",
			DeleteClusterWaitName:          "wait-controllers",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
			TopologyNamespace:              topologyNamespace,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "docker-cluster-classes-regular",
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

var _ = Describe("[Azure] [AKS] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "no-cert-manager-azure-aks"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.AzureIdentitySecret,
			},
			CAPIProvidersYAML: [][]byte{
				e2e.AzureProvider,
			},
			WaitForDeployments: []testenv.NamespaceName{
				{
					Name:      "capz-controller-manager",
					Namespace: "capz-system",
				},
			},
			CustomWaiter: []func(ctx context.Context){
				e2e.AzureServiceOperatorWaiter(bootstrapClusterProxy), // workaround for https://github.com/rancher/turtles/issues/1584, remove when fixed
			},
		})

		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAzureAKSTopology,
			ClusterName:                    "no-cert-manager-aks",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capz-create-cluster",
			DeleteClusterWaitName:          "wait-aks-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
			TopologyNamespace:              topologyNamespace,
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

var _ = Describe("[AWS] [EKS] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.AWSIdentitySecret,
			},
			CAPIProvidersYAML: [][]byte{
				e2e.AWSProvider,
			},
			WaitForDeployments: []testenv.NamespaceName{
				{
					Name:      "capa-controller-manager",
					Namespace: "capa-system",
				},
			},
		})

		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAwsEKSMMP,
			ClusterName:                    "no-cert-manager-eks",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
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

var _ = Describe("[GCP] [GKE] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.GCPProviderSecret,
			},
			CAPIProvidersYAML: [][]byte{
				e2e.GCPProvider,
			},
			WaitForDeployments: []testenv.NamespaceName{
				{
					Name:      "capg-controller-manager",
					Namespace: "capg-system",
				},
			},
		})

		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIGCPGKE,
			ClusterName:                    "no-cert-manager-gke",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capg-create-cluster",
			DeleteClusterWaitName:          "wait-gke-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
		}
	})
})

var _ = Describe("[vSphere] [RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.VsphereTestLabel, e2e.Rke2TestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "no-cert-manager-vsphere-rke2"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		By("Running local vSphere tests, deploying vSphere infrastructure provider")

		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.VSphereProviderSecret,
			},
			CAPIProvidersYAML: [][]byte{
				e2e.CapvProvider,
			},
			WaitForDeployments: []testenv.NamespaceName{
				{
					Name:      "capv-controller-manager",
					Namespace: "capv-system",
				},
			},
		})

		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIvSphereRKE2Topology,
			TopologyNamespace:              topologyNamespace,
			ClusterName:                    "no-cert-manager-vsphere-rke2",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capv-create-cluster",
			DeleteClusterWaitName:          "wait-vsphere-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "vsphere-cluster-classes-rke2",
					TargetNamespace: topologyNamespace,
					Paths:           []string{"examples/clusterclasses/vsphere/rke2"},
					ClusterProxy:    bootstrapClusterProxy,
				},
				{
					Name:            "vsphere-cni",
					Paths:           []string{"examples/applications/cni/calico"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "vsphere-cpi",
					Paths:           []string{"examples/applications/ccm/vsphere"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "vsphere-csi",
					Paths:           []string{"examples/applications/csi/vsphere"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
		}
	})
})
