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
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	turtlesframework "github.com/rancher/turtles/test/framework"
	corev1 "k8s.io/api/core/v1"
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
			RancherManagedFleet:            true,
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

var _ = Describe("[RancherManagedFleet] [Docker] [Kubeadm]  Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.ShortTestLabel, e2e.KubeadmTestLabel, e2e.RancherManagedFleetLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-docker-kubeadm"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIDockerKubeadmTopology,
			ClusterName:                    "cluster-docker-kubeadm",
			ControlPlaneMachineCount:       new(1),
			WorkerMachineCount:             new(1),
			LabelNamespace:                 true,
			TestClusterReimport:            true,
			RancherManagedFleet:            true,
			ValidateFleetAgentWasInstalled: true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-rancher",
			DeleteClusterWaitName:          "wait-controllers",
			TopologyNamespace:              topologyNamespace,
			VerifyETCDSize:                 true,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "docker-cluster-classes-regular",
					Paths:           []string{"examples/clusterclasses/docker/kubeadm"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
			AdditionalDownstreamTemplates: [][]byte{
				e2e.CalicoManifest,
			},
		}
	})
})

var _ = Describe("[AWS] [EC2 RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel, e2e.Rke2TestLabel), func() {
	var topologyNamespace, capiClusterNamespace, credentialName string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-aws-rke2"
		// AWSClusterStaticIdentity only allows provisioning clusters in "fleet-default"
		capiClusterNamespace = "fleet-default"
		credentialName = "rancher-cloud-credential-aws"

		By("Creating Rancher AWS Cloud Credential which will be translated into `AWSClusterStaticIdentity`")
		lookupResult := &turtlesframework.RancherLookupUserResult{}
		turtlesframework.RancherLookupUser(ctx, turtlesframework.RancherLookupUserInput{
			Username:     "admin",
			ClusterProxy: bootstrapClusterProxy,
		}, lookupResult)

		turtlesframework.CreateSecret(ctx, turtlesframework.CreateSecretInput{
			Creator:   bootstrapClusterProxy.GetClient(),
			Name:      credentialName,
			Namespace: "cattle-global-data",
			Type:      corev1.SecretTypeOpaque,
			Data: map[string]string{
				"amazonec2credentialConfig-accessKey": os.Getenv("AWS_ACCESS_KEY_ID"),
				"amazonec2credentialConfig-secretKey": os.Getenv("AWS_SECRET_ACCESS_KEY"),
			},
			Annotations: map[string]string{
				"field.cattle.io/name":          credentialName,
				"provisioning.cattle.io/driver": "aws",
				"field.cattle.io/creatorId":     lookupResult.User,
			},
			Labels: map[string]string{
				"cattle.io/creator": "norman",
			},
		})
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAwsEC2RKE2Topology,
			ClusterName:                    "cluster-aws-rke2",
			Namespace:                      capiClusterNamespace,
			ControlPlaneMachineCount:       ptr.To(3), // minimum 3 replicas for CSI controller
			WorkerMachineCount:             ptr.To(1),
			LabelNamespace:                 true,
			RancherManagedFleet:            true,
			ValidateFleetAgentWasInstalled: true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capa-create-cluster",
			DeleteClusterWaitName:          "wait-eks-delete",
			TopologyNamespace:              topologyNamespace,
			VerifyETCDSize:                 true,
			AdditionalTemplateVariables: map[string]string{
				"AWS_CLUSTER_IDENTITY_NAME": credentialName,
				"NAMESPACE":                 capiClusterNamespace,
			},
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "aws-cluster-class-rke2",
					Paths:           []string{"examples/clusterclasses/aws/rke2"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
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

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAwsKubeadmTopology,
			ClusterName:                    "cluster-aws-kubeadm",
			ControlPlaneMachineCount:       ptr.To(3), // minimum 3 replicas for CSI controller
			WorkerMachineCount:             ptr.To(1),
			SkipDeletionTest:               false,
			LabelNamespace:                 true,
			RancherManagedFleet:            true,
			ValidateFleetAgentWasInstalled: true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capa-create-cluster",
			DeleteClusterWaitName:          "wait-eks-delete",
			TopologyNamespace:              topologyNamespace,
			VerifyETCDSize:                 true,
			AdditionalFleetGitRepos: []turtlesframework.FleetCreateGitRepoInput{
				{
					Name:            "aws-cluster-classes-regular",
					Paths:           []string{"examples/clusterclasses/aws/kubeadm"},
					ClusterProxy:    bootstrapClusterProxy,
					TargetNamespace: topologyNamespace,
				},
			},
			AdditionalDownstreamTemplates: [][]byte{
				e2e.CalicoManifest,
				e2e.CloudProviderAWSManifest,
				e2e.CSIAWSEBSManifest,
			},
		}
	})
})

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
			RancherManagedFleet:            true,
			ValidateFleetAgentWasInstalled: true,
			AdditionalTemplateVariables: map[string]string{
				e2e.GCPImageIDFormattedVar: gcpImageFormatted,
				e2e.ClusterCIDRVar:         "192.168.0.0/16",
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
			RancherManagedFleet:            true,
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
			RancherManagedFleet:            true,
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
			RancherManagedFleet:            true,
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
			RancherManagedFleet:            true,
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

var _ = Describe("[AWS] [EKS] Create and delete CAPI cluster from cluster class", Label(e2e.FullTestLabel), func() {
	var topologyNamespace string

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		topologyNamespace = "creategitops-aws-eks"
	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIAwsEKSTopology,
			ClusterName:                    "cluster-eks",
			ControlPlaneMachineCount:       new(1),
			WorkerMachineCount:             new(1),
			LabelNamespace:                 true,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capa-create-cluster",
			DeleteClusterWaitName:          "wait-eks-delete",
			TopologyNamespace:              topologyNamespace,
			VerifyETCDSize:                 true,
			RancherManagedFleet:            true,
			ValidateFleetAgentWasInstalled: true,
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
			RancherManagedFleet:            true,
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
