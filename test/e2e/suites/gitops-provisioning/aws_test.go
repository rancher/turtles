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
	"os"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	turtlesframework "github.com/rancher/turtles/test/framework"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("[RancherManagedFleet] [AWS] [RKE2] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel, e2e.Rke2TestLabel), func() {
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
		e2eConfig := e2e.LoadE2EConfig()
		validateInPlaceUpdate := e2eConfig.GetVariableOrEmpty(e2e.RKE2InPlaceUpdatesVar) == "true"

		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:             e2eConfig,
			BootstrapClusterProxy: bootstrapClusterProxy,
			ClusterTemplate:       e2e.CAPIAwsEC2RKE2Topology,
			ClusterName:           "cluster-aws-rke2",
			Namespace:             capiClusterNamespace,
			// minimum 3 replicas for CSI controller and CAPRKE2 in-place updates
			ControlPlaneMachineCount:       ptr.To(3),
			WorkerMachineCount:             ptr.To(1),
			LabelNamespace:                 true,
			ValidateFleetAgentWasInstalled: true,
			ValidateInPlaceUpdate:          validateInPlaceUpdate,
			RancherServerURL:               hostName,
			CAPIClusterCreateWaitName:      "wait-capa-create-cluster",
			DeleteClusterWaitName:          "wait-eks-delete",
			TopologyNamespace:              topologyNamespace,
			VerifyETCDSize:                 true,
			AdditionalTemplateVariables: map[string]string{
				"AWS_CLUSTER_IDENTITY_NAME": credentialName,
				"NAMESPACE":                 capiClusterNamespace,
				"IN_PLACE_UPDATES":          strconv.FormatBool(validateInPlaceUpdate),
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

var _ = Describe("[RancherManagedFleet] [AWS] [Kubeadm] Create and delete CAPI cluster functionality should work with namespace auto-import", Label(e2e.FullTestLabel, e2e.KubeadmTestLabel), func() {
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
			SkipClusterAvailableWait:       true, // EKS kubelet reports version with -eks-<build> suffix, so TopologyReconciled/MachinesUpToDate never report an empty message
			VerifyETCDSize:                 true,
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
