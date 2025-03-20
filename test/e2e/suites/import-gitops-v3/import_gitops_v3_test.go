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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"k8s.io/utils/ptr"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	"github.com/rancher/turtles/test/testenv"

	turtlesframework "github.com/rancher/turtles/test/framework"
)

var _ = Describe("[Docker] [Kubeadm]  Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.ShortTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
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
			ClusterTemplate:                e2e.CAPIDockerKubeadm,
			AdditionalTemplates:            [][]byte{e2e.CAPIKindnet},
			ClusterName:                    "clusterv3-auto-import-kubeadm",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
			GitAddr:                        gitAddress,
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

var _ = Describe("[Docker] [RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.ShortTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
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
			ClusterTemplate:                e2e.CAPIDockerRKE2,
			AdditionalTemplates:            [][]byte{e2e.CAPIKindnet},
			ClusterName:                    "clusterv3-auto-import-rke2",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
			GitAddr:                        gitAddress,
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

var _ = Describe("[Azure] [AKS] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
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

		// Add the needed ClusterClass
		fixedNamespace := "creategitops-azure-aks"
		Expect(turtlesframework.CreateNamespace(ctx, bootstrapClusterProxy, fixedNamespace)).Should(Succeed())
		clusterClass, err := os.ReadFile("../../../../examples/clusterclasses/azure/clusterclass-aks-example.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(turtlesframework.Apply(ctx, bootstrapClusterProxy, clusterClass, "-n", fixedNamespace)).Should(Succeed())

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAzureAKSTopology,
			ClusterName:                    "cluster-aks",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			GitAddr:                        gitAddress,
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capz-create-cluster",
			DeleteClusterWaitName:          "wait-aks-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
			FixedNamespace:                 fixedNamespace,
		}
	})
})

var _ = Describe("[Azure] [RKE2] - [management.cattle.io/v3] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
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

		// Add the needed ClusterClass and ClusterResourceSet
		fixedNamespace := "creategitops-azure-rke2"
		Expect(turtlesframework.CreateNamespace(ctx, bootstrapClusterProxy, fixedNamespace)).Should(Succeed())
		clusterClass, err := os.ReadFile("../../../../examples/clusterclasses/azure/clusterclass-example.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(turtlesframework.Apply(ctx, bootstrapClusterProxy, clusterClass, "-n", fixedNamespace)).Should(Succeed())
		cloudProvider, err := os.ReadFile("../../../../examples/applications/azure/clusterresourceset-cloud-provider.yaml")
		Expect(err).ShouldNot(HaveOccurred())
		Expect(turtlesframework.Apply(ctx, bootstrapClusterProxy, cloudProvider, "-n", fixedNamespace)).Should(Succeed())

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAzureRKE2Topology,
			ClusterName:                    "cluster-azure-rke2",
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			GitAddr:                        gitAddress,
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capz-create-cluster",
			DeleteClusterWaitName:          "wait-aks-delete",
			CapiClusterOwnerLabel:          e2e.CapiClusterOwnerLabel,
			CapiClusterOwnerNamespaceLabel: e2e.CapiClusterOwnerNamespaceLabel,
			OwnedLabelName:                 e2e.OwnedLabelName,
			FixedNamespace:                 fixedNamespace,
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
			GitAddr:                        gitAddress,
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

var _ = Describe("[AWS] [EC2 Kubeadm] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel), func() {
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
			ClusterTemplate:                e2e.CAPIAwsEC2Kubeadm,
			AdditionalTemplates:            [][]byte{e2e.CAPICalico, e2e.CAPIAWSCPICSI},
			ClusterName:                    "cluster-ec2",
			ControlPlaneMachineCount:       ptr.To[int](1),
			WorkerMachineCount:             ptr.To[int](1),
			GitAddr:                        gitAddress,
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
			GitAddr:                        gitAddress,
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

var _ = Describe("[vSphere] [Kubeadm] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.LocalTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		By("Running local vSphere tests, deploying vSphere infrastructure provider")

		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.VSphereProviderSecret,
			},
			CAPIProvidersYAML: [][]byte{e2e.CapvProvider, e2e.CapiProviders},
			WaitForDeployments: []testenv.NamespaceName{
				{
					Name:      "capv-controller-manager",
					Namespace: "capv-system",
				},
			},
		})

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                 e2e.LoadE2EConfig(),
			BootstrapClusterProxy:     bootstrapClusterProxy,
			ClusterTemplate:           e2e.CAPIvSphereKubeadm,
			ClusterName:               "cluster-vsphere-kubeadm",
			ControlPlaneMachineCount:  ptr.To[int](1),
			WorkerMachineCount:        ptr.To[int](1),
			GitAddr:                   gitAddress,
			LabelNamespace:            true,
			RancherServerURL:          hostName,
			CAPIClusterCreateWaitName: "wait-capv-create-cluster",
			DeleteClusterWaitName:     "wait-vsphere-delete",
			AdditionalTemplateVariables: map[string]string{
				"NAMESPACE":             "default",
				"VIP_NETWORK_INTERFACE": "",
			},
		}
	})
})

var _ = Describe("[vSphere] [RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.LocalTestLabel), func() {
	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	specs.CreateMgmtV3UsingGitOpsSpec(ctx, func() specs.CreateMgmtV3UsingGitOpsSpecInput {
		By("Running local vSphere tests, deploying vSphere infrastructure provider")

		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: setupClusterResult.BootstrapClusterProxy,
			CAPIProvidersSecretsYAML: [][]byte{
				e2e.VSphereProviderSecret,
			},
			CAPIProvidersYAML: [][]byte{e2e.CapvProvider},
			WaitForDeployments: []testenv.NamespaceName{
				{
					Name:      "capv-controller-manager",
					Namespace: "capv-system",
				},
			},
		})

		return specs.CreateMgmtV3UsingGitOpsSpecInput{
			E2EConfig:                 e2e.LoadE2EConfig(),
			BootstrapClusterProxy:     bootstrapClusterProxy,
			ClusterTemplate:           e2e.CAPIvSphereRKE2,
			ClusterName:               "cluster-vsphere-rke2",
			ControlPlaneMachineCount:  ptr.To[int](1),
			WorkerMachineCount:        ptr.To[int](1),
			GitAddr:                   gitAddress,
			LabelNamespace:            true,
			RancherServerURL:          hostName,
			CAPIClusterCreateWaitName: "wait-capv-create-cluster",
			DeleteClusterWaitName:     "wait-vsphere-delete",
			AdditionalTemplateVariables: map[string]string{
				"NAMESPACE":             "default",
				"VIP_NETWORK_INTERFACE": "",
			},
		}
	})
})
