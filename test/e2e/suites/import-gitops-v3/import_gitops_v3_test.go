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
	turtlesframework "github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"
)

var _ = Describe("[Docker] [Kubeadm]  Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.ShortTestLabel, e2e.KubeadmTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-docker-kubeadm"
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersYAML: [][]byte{
				e2e.CapiProviders,
			},
			WaitForDeployments: testenv.DefaultDeployments,
		})

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIDockerKubeadmTopology,
			ClusterName:                    "clusterv3-auto-import-kubeadm",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
			LabelNamespace:                 true,
			TestClusterReimport:            true,
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
					Paths:           []string{"examples/clusterclasses/docker/kubeadm"},
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

var _ = Describe("[Docker] [RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.ShortTestLabel, e2e.Rke2TestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-docker-rke2"
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersYAML: [][]byte{
				e2e.CapiProviders,
			},
			WaitForDeployments: testenv.DefaultDeployments,
		})

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIDockerRKE2Topology,
			ClusterName:                    "clusterv3-auto-import-rke2",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
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

		topologyNamespace = "creategitops-azure-aks"
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
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
		})

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAzureAKSTopology,
			ClusterName:                    "cluster-aks",
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

var _ = Describe("[Azure] [Kubeadm] - [management.cattle.io/v3] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel, e2e.KubeadmTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-azure-kubeadm"
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.AzureIdentitySecret,
			},
			CAPIProvidersYAML: [][]byte{
				e2e.AzureProvider,
				e2e.CapiProviders,
			},
			WaitForDeployments: append([]testenv.NamespaceName{
				{
					Name:      "capz-controller-manager",
					Namespace: "capz-system",
				},
			}, testenv.DefaultDeployments...),
		})

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAzureKubeadmTopology,
			ClusterName:                    "cluster-azure-kubeadm",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capz-create-cluster",
			DeleteClusterWaitName:          "wait-aks-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
			TopologyNamespace:              topologyNamespace,
			AdditionalTemplateVariables: map[string]string{
				e2e.KubernetesVersionVar: e2e.LoadE2EConfig().GetVariable(e2e.AzureKubernetesVersionVar), // override the default k8s version
			},
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "azure-cluster-class-kubeadm",
					Paths:           []string{"examples/clusterclasses/azure/kubeadm"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "azure-ccm-regular",
					Paths:           []string{"examples/applications/ccm/azure"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "azure-cni",
					Paths:           []string{"examples/applications/cni/calico"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
		}
	})

})

var _ = Describe("[Azure] [RKE2] - [management.cattle.io/v3] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel, e2e.Rke2TestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-azure-rke2"
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
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
		})

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAzureRKE2Topology,
			ClusterName:                    "cluster-azure-rke2",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			SkipDeletionTest:               false,
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
					Name:            "azure-cluster-class-rke2",
					Paths:           []string{"examples/clusterclasses/azure/rke2"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "azure-ccm-regular",
					Paths:           []string{"examples/applications/ccm/azure"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "azure-cni",
					Paths:           []string{"examples/applications/cni/calico"},
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

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.AWSProviderSecret,
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

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAwsEKSMMP,
			ClusterName:                    "cluster-eks",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
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

var _ = Describe("[AWS] [EC2 Kubeadm] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel, e2e.KubeadmTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-aws-kubeadm"
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.AWSProviderSecret,
			},
			CAPIProvidersYAML: [][]byte{
				e2e.AWSProvider,
				e2e.CapiProviders,
			},
			WaitForDeployments: append([]testenv.NamespaceName{
				{
					Name:      "capa-controller-manager",
					Namespace: "capa-system",
				},
			}, testenv.DefaultDeployments...),
		})

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAwsKubeadmTopology,
			ClusterName:                    "cluster-aws-kubeadm",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capa-create-cluster",
			DeleteClusterWaitName:          "wait-eks-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
			AdditionalTemplateVariables: map[string]string{
				e2e.KubernetesVersionVar: e2e.LoadE2EConfig().GetVariable(e2e.AWSKubernetesVersionVar), // override the default k8s version
			},
			TopologyNamespace: topologyNamespace,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "aws-cluster-classes-regular",
					Paths:           []string{"examples/clusterclasses/aws/kubeadm"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "aws-cni",
					Paths:           []string{"examples/applications/cni/aws/calico"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "aws-ccm",
					Paths:           []string{"examples/applications/ccm/aws"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "aws-csi",
					Paths:           []string{"examples/applications/csi/aws"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
		}
	})
})

var _ = Describe("[AWS] [EC2 RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel, e2e.Rke2TestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-aws-rke2"
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.AWSProviderSecret,
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

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAwsEC2RKE2Topology,
			ClusterName:                    "cluster-ec2-rke2",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capa-create-cluster",
			DeleteClusterWaitName:          "wait-eks-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
			TopologyNamespace:              topologyNamespace,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "aws-cluster-class-rke2",
					Paths:           []string{"examples/clusterclasses/aws/rke2"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "aws-ccm-ec2-rke2",
					Paths:           []string{"examples/applications/ccm/aws"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "aws-csi-ec2-rke2",
					Paths:           []string{"examples/applications/csi/aws"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
				{
					Name:            "aws-cni",
					Paths:           []string{"examples/applications/cni/aws/calico"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
		}
	})
})

var _ = Describe("[GCP] [GKE] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
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

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIGCPGKE,
			ClusterName:                    "cluster-gke",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
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

var _ = Describe("[vSphere] [Kubeadm] Create and delete CAPI cluster from cluster class", Ordered, Label(e2e.VsphereTestLabel, e2e.KubeadmTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-vsphere-kubeadm"
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		By("Running local vSphere tests, deploying vSphere infrastructure provider")

		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.VSphereProviderSecret,
			},
			CAPIProvidersYAML: [][]byte{
				e2e.CapvProvider,
				e2e.CapiProviders,
			},
			WaitForDeployments: append([]testenv.NamespaceName{
				{
					Name:      "capv-controller-manager",
					Namespace: "capv-system",
				},
			}, testenv.DefaultDeployments...),
		})

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIvSphereKubeadmTopology,
			TopologyNamespace:              topologyNamespace,
			ClusterName:                    "cluster-vsphere-kubeadm",
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
					Name:            "vsphere-cluster-classes-kubeadm",
					TargetNamespace: topologyNamespace,
					Paths:           []string{"examples/clusterclasses/vsphere/kubeadm"},
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

var _ = Describe("[vSphere] [RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Ordered, Label(e2e.VsphereTestLabel, e2e.Rke2TestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-vsphere-rke2"
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
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

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIvSphereRKE2Topology,
			TopologyNamespace:              topologyNamespace,
			ClusterName:                    "cluster-vsphere-rke2",
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
