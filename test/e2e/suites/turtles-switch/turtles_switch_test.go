//go:build e2e
// +build e2e

/*
Copyright © 2023 - 2025 SUSE LLC

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

package turtles_switch

import (
	"fmt"
	"os"

	"github.com/drone/envsubst/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/e2e/specs"
	turtlesframework "github.com/rancher/turtles/test/framework"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

var _ = Describe("Switch from Turtles to embedded CAPI", Ordered, Label(e2e.ShortTestLabel), func() {
	var turtlesHelmApp, capiHelmApp turtlesframework.GetHelmAppInput
	const (
		oldCAPINamespace = "cattle-provisioning-capi-system"
		turtlesNamespace = e2e.NewRancherTurtlesNamespace

		clusterName = "cluster-docker-rke2-switch"
	)
	BeforeAll(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
		e2eConfig = e2e.LoadE2EConfig()
		turtlesHelmApp = turtlesframework.GetHelmAppInput{
			GetLister:        bootstrapClusterProxy.GetClient(),
			HelmAppName:      "rancher-turtles",
			HelmAppNamespace: turtlesNamespace,
		}

		capiHelmApp = turtlesframework.GetHelmAppInput{
			GetLister:        bootstrapClusterProxy.GetClient(),
			HelmAppName:      "rancher-provisioning-capi",
			HelmAppNamespace: oldCAPINamespace,
		}
	})

	It("Should use Turtles as the source of truth", func() {
		By("Verifying turtles helm app is installed", func() {
			app, err := turtlesframework.GetHelmApp(ctx, turtlesHelmApp)
			Expect(err).NotTo(HaveOccurred())
			Expect(app).ToNot(BeNil())
		})

		By("Verifying cluster-api helm app is not installed", func() {
			_, err := turtlesframework.GetHelmApp(ctx, capiHelmApp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		By("Verifying capi-controller-manager is managed by turtles", func() {
			capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
				Getter: bootstrapClusterProxy.GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      "capi-controller-manager",
					Namespace: "cattle-capi-system",
				}},
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
		})

	})

	specs.CreateUsingGitOpsSpec(ctx, func() specs.CreateUsingGitOpsSpecInput {
		By("Provisioning workload cluster")

		const topologyNamespace = "switch-docker-rke2"

		return specs.CreateUsingGitOpsSpecInput{
			E2EConfig:                      e2e.LoadE2EConfig(),
			BootstrapClusterProxy:          bootstrapClusterProxy,
			ClusterTemplate:                e2e.CAPIDockerRKE2Topology,
			ClusterName:                    clusterName,
			ControlPlaneMachineCount:       ptr.To(1),
			WorkerMachineCount:             ptr.To(1),
			LabelNamespace:                 true,
			TestClusterReimport:            false,
			SkipDeletionTest:               true, // Delete the cluster later
			SkipCleanup:                    true, // Delete the cluster later
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

	It("Should disable turtles feature with zero-downtime", func() {
		By("Enabling embedded-cluster-api and disabling turtles feature", func() {
			enableEmbeddedCAPIFeature, err := envsubst.Eval(string(e2e.TurtlesEmbeddedCAPIFeature), func(s string) string {
				switch s {
				case "ENABLE_TURTLES_FEATURE":
					return "false"
				case "ENABLE_EMBEDDED_CAPI_FEATURE":
					return "true"
				default:
					return os.Getenv(s)
				}
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(turtlesframework.Apply(ctx, bootstrapClusterProxy, []byte(enableEmbeddedCAPIFeature))).To(Succeed(), "Failed to enable embedded-cluster-api feature")
		})

		By("Verifying cluster-api helm app is installed", func() {
			capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
				Getter: bootstrapClusterProxy.GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      "capi-controller-manager",
					Namespace: oldCAPINamespace,
				}},
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

			host, err := turtlesframework.GetHelmApp(ctx, capiHelmApp)
			Expect(err).ToNot(HaveOccurred())
			Expect(host).ToNot(BeNil())
		})

		By("Verifying rancher-turtles is disabled", func() {
			_, err := turtlesframework.GetHelmApp(ctx, turtlesHelmApp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		By("Verifying turtlesNamespace is deleted", func() {
			turtlesframework.WaitForNamespaceToBeDeleted(ctx, turtlesframework.WaitNamespaceInput{
				Name:      turtlesNamespace,
				GetLister: bootstrapClusterProxy.GetClient(),
			})
		})

		By("Verifying the cluster still exists", func() {
			turtlesframework.VerifyCluster(ctx, turtlesframework.VerifyClusterInput{
				BootstrapClusterProxy:   bootstrapClusterProxy,
				Name:                    clusterName,
				DeleteAfterVerification: false,
			})
		})

	})

	It("Should re-enable turtles feature with zero-downtime", func() {
		By("Disabling embedded-cluster-api and enabling turtles feature", func() {
			enableTurtlesFeature, err := envsubst.Eval(string(e2e.TurtlesEmbeddedCAPIFeature), func(s string) string {
				switch s {
				case "ENABLE_TURTLES_FEATURE":
					return "true"
				case "ENABLE_EMBEDDED_CAPI_FEATURE":
					return "false"
				default:
					return os.Getenv(s)
				}
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(turtlesframework.Apply(ctx, bootstrapClusterProxy, []byte(enableTurtlesFeature))).To(Succeed(), "Failed to enable turtles feature")
		})

		By("Verifying rancher-turtles is installed", func() {
			capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
				Getter: bootstrapClusterProxy.GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      "rancher-turtles-controller-manager",
					Namespace: turtlesNamespace,
				}},
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

			host, err := turtlesframework.GetHelmApp(ctx, turtlesHelmApp)
			Expect(err).ToNot(HaveOccurred())
			Expect(host).ToNot(BeNil())
		})

		By("Verifying cluster-api helm app is uninstalled", func() {
			_, err := turtlesframework.GetHelmApp(ctx, capiHelmApp)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		By("Verifying capiNamespace is deleted", func() {
			turtlesframework.WaitForNamespaceToBeDeleted(ctx, turtlesframework.WaitNamespaceInput{
				Name:      oldCAPINamespace,
				GetLister: bootstrapClusterProxy.GetClient(),
			})
		})

		By("Re-applying Clusterctl Config", func() {
			// This configmap is deployed in ns cattle-turtles-system which is deleted when turtles is disabled;
			// hence the need to re-apply it
			Expect(turtlesframework.Apply(ctx, setupClusterResult.BootstrapClusterProxy, e2e.ClusterctlConfig)).To(Succeed())
		})

		By("Verifying providers deployments are active", func() {
			for _, provider := range []string{"capd", "rke2-bootstrap", "rke2-control-plane"} {
				capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
					Getter: bootstrapClusterProxy.GetClient(),
					Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-controller-manager", provider),
						Namespace: fmt.Sprintf("%s-system", provider),
					}},
				}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
			}
		})

		By("Verifying workload cluster survived the switch(zero-downtime validated)", func() {
			// This is the critical validation: the workload cluster provisioned before the turtles feature disabling
			// should still be healthy and operational, proving zero-downtime migration
			turtlesframework.VerifyCluster(ctx, turtlesframework.VerifyClusterInput{
				BootstrapClusterProxy:   bootstrapClusterProxy,
				Name:                    clusterName,
				DeleteAfterVerification: true,
			})
		})

	})
})
