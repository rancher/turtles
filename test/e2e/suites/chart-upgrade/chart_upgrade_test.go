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

package chart_upgrade

import (
	_ "embed"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	"github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"

	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("Chart upgrade functionality should work", Ordered, Label(e2e.ShortTestLabel), func() {
	var (
		clusterName       string
		topologyNamespace = "creategitops-docker-rke2"
	)

	BeforeAll(func() {
		clusterName = fmt.Sprintf("docker-rke2-%s", util.RandomString(6))

		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)
	})

	// Note that this test suite requires an older installation of Turtles
	// where the separate cluster-api-operator is still deployed.
	// The embedded cluster-api-operator was shipped in Turtles v0.22.0,
	// so any version installed here should be lower.
	//
	// Consider reworking this suite in the future to test latest --> head/main instead,
	// once testing the successful embedding of cluster-api-operator will no longer be required.
	It("Should install old version of Turtles", func() {
		rtInput := testenv.DeployRancherTurtlesInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			TurtlesChartRepoName:  "rancher-turtles",
			TurtlesChartUrl:       "https://rancher.github.io/turtles",
			Version:               "v0.21.0",
			AdditionalValues: map[string]string{
				"rancherTurtles.namespace": e2e.RancherTurtlesNamespace,
			},
		}
		testenv.DeployRancherTurtles(ctx, rtInput)

		By("Waiting for cluster-api-operator Deployment to be ready")
		capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "rancher-turtles-cluster-api-operator",
				Namespace: e2e.RancherTurtlesNamespace,
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersYAML: [][]byte{
				e2e.CapiProviders,
			},
			WaitForDeployments: []testenv.NamespaceName{
				{
					Name:      "capi-controller-manager",
					Namespace: "capi-system",
				}, {
					Name:      "capi-kubeadm-bootstrap-controller-manager",
					Namespace: "capi-kubeadm-bootstrap-system",
				}, {
					Name:      "capi-kubeadm-control-plane-controller-manager",
					Namespace: "capi-kubeadm-control-plane-system",
				}, {
					Name:      "capd-controller-manager",
					Namespace: "capd-system",
				}, {
					Name:      "rke2-bootstrap-controller-manager",
					Namespace: "rke2-bootstrap-system",
				}, {
					Name:      "rke2-control-plane-controller-manager",
					Namespace: "rke2-control-plane-system",
				},
			},
		})
	})

	Context("Provisioning a workload Cluster", func() {
		// Provision a workload Cluster.
		// This ensures that upgrading the chart will not unexpectedly lead to unready Cluster or Machines.
		specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
			return specs.CreateUsingGitOpsSpecInput{
				E2EConfig:                      e2e.LoadE2EConfig(),
				BootstrapClusterProxy:          bootstrapClusterProxy,
				ClusterTemplate:                e2e.CAPIDockerRKE2Topology,
				ClusterName:                    clusterName,
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
				SkipCleanup:                    true,
				SkipDeletionTest:               true,
				AdditionalFleetGitRepos: []framework.FleetCreateGitRepoInput{
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

	It("Should upgrade Turtles to head/main and validate providers", func() {
		// Upgrade Turtles chart to locally built one
		testenv.DeployRancherTurtles(ctx, testenv.DeployRancherTurtlesInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			AdditionalValues:      map[string]string{},
		})

		By("Waiting for the upstream CAPI operator deployment to be removed")
		framework.WaitForDeploymentsRemoved(ctx, framework.WaitForDeploymentsRemovedInput{
			Getter: bootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "rancher-turtles-cluster-api-operator",
				Namespace: e2e.RancherTurtlesNamespace,
			}},
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
			Getter:  bootstrapClusterProxy.GetClient(),
			Version: e2e.CAPIVersion,
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      "capi-controller-manager",
				Namespace: "capi-system",
			}},
			Image:     "registry.suse.com/rancher/cluster-api-controller:",
			Name:      "cluster-api",
			Namespace: "capi-system",
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		By("Verifying Kubeadm CAPIProvider version has not changed after upgrade")
		framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
			Getter:    bootstrapClusterProxy.GetClient(),
			Version:   "v1.9.5", // provider version that was installed previously
			Name:      "kubeadm-bootstrap",
			Namespace: "capi-kubeadm-bootstrap-system",
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
			Getter:    bootstrapClusterProxy.GetClient(),
			Version:   "v1.9.5", // provider version that was installed previously
			Name:      "kubeadm-control-plane",
			Namespace: "capi-kubeadm-control-plane-system",
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		By("Verifying CAPD CAPIProvider version has not changed after upgrade")
		framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
			Getter:    bootstrapClusterProxy.GetClient(),
			Version:   "v1.9.5", // provider version that was installed previously
			Name:      "docker",
			Namespace: "capd-system",
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

		framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
			Lister: bootstrapClusterProxy.GetClient(),
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "operator.cluster.x-k8s.io",
				Version: "v1alpha2",
				Kind:    "AddonProvider",
			},
		})

		framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
			Lister: bootstrapClusterProxy.GetClient(),
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "operator.cluster.x-k8s.io",
				Version: "v1alpha2",
				Kind:    "BootstrapProvider",
			},
		})

		framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
			Lister: bootstrapClusterProxy.GetClient(),
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "operator.cluster.x-k8s.io",
				Version: "v1alpha2",
				Kind:    "ControlPlaneProvider",
			},
		})

		framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
			Lister: bootstrapClusterProxy.GetClient(),
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "operator.cluster.x-k8s.io",
				Version: "v1alpha2",
				Kind:    "CoreProvider",
			},
		})

		framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
			Lister: bootstrapClusterProxy.GetClient(),
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "operator.cluster.x-k8s.io",
				Version: "v1alpha2",
				Kind:    "InfrastructureProvider",
			},
		})

		framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
			Lister: bootstrapClusterProxy.GetClient(),
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "operator.cluster.x-k8s.io",
				Version: "v1alpha2",
				Kind:    "IPAMProvider",
			},
		})

		framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
			Lister: bootstrapClusterProxy.GetClient(),
			GroupVersionKind: schema.GroupVersionKind{
				Group:   "operator.cluster.x-k8s.io",
				Version: "v1alpha2",
				Kind:    "RuntimeExtensionProvider",
			},
		})

		framework.VerifyCluster(ctx, framework.VerifyClusterInput{
			BootstrapClusterProxy:   bootstrapClusterProxy,
			Name:                    clusterName,
			DeleteAfterVerification: true,
		})

		testenv.DeployRancherTurtlesProviders(ctx, testenv.DeployRancherTurtlesProvidersInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			AdditionalValues: map[string]string{
				// CAAPF and CAPRKE2 are enabled by default in the providers chart
				"providers.bootstrapKubeadm.enabled":     "true",
				"providers.controlplaneKubeadm.enabled":  "true",
				"providers.infrastructureDocker.enabled": "true",
			},
			WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers"),
		})

		By("Verifying providers chart adopted pre-existing CAPIProvider resources with Helm ownership")
		verifyAdopted := func(name, namespace string) {
			provider := &turtlesv1.CAPIProvider{}
			key := types.NamespacedName{Name: name, Namespace: namespace}
			Eventually(func(g Gomega) {
				g.Expect(bootstrapClusterProxy.GetClient().Get(ctx, key, provider)).To(Succeed())
				g.Expect(provider.GetLabels()).To(HaveKeyWithValue("app.kubernetes.io/managed-by", "Helm"))
				g.Expect(provider.GetAnnotations()).To(HaveKeyWithValue("meta.helm.sh/release-name", e2e.ProvidersChartName))
				g.Expect(provider.GetAnnotations()).To(HaveKeyWithValue("meta.helm.sh/release-namespace", e2e.RancherTurtlesNamespace))
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).Should(Succeed(),
				"CAPIProvider %s/%s should be adopted by Helm release %s in %s",
				namespace, name, e2e.ProvidersChartName, e2e.RancherTurtlesNamespace,
			)
		}

		verifyAdopted("fleet", "fleet-addon-system")
		verifyAdopted("rke2-bootstrap", "rke2-bootstrap-system")
		verifyAdopted("rke2-control-plane", "rke2-control-plane-system")
	})
})
