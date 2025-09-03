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

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"

	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
)

var _ = Describe("Chart upgrade functionality should work", Label(e2e.ShortTestLabel), func() {
	var (
		namespace   *corev1.Namespace
		clusterName string
	)

	BeforeEach(func() {
		namespace = capiframework.CreateNamespace(ctx, capiframework.CreateNamespaceInput{
			Creator: bootstrapClusterProxy.GetClient(),
			Name:    fmt.Sprintf("chartupgrade-%s", util.RandomString(6)),
		})
		clusterName = fmt.Sprintf("docker-rke2-%s", util.RandomString(6))

		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)
	})

	It("Should perform upgrade from latest N-1 version to latest", func() {
		rtInput := testenv.DeployRancherTurtlesInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			TurtlesChartPath:      "https://rancher.github.io/turtles",
			CAPIProvidersYAML:     e2e.CapiProvidersLegacy,
			Version:               "v0.17.0",
			AdditionalValues:      map[string]string{},
			WaitForDeployments:    testenv.DefaultDeployments,
		}
		testenv.DeployRancherTurtles(ctx, rtInput)

		testenv.CAPIOperatorDeployProvider(ctx, testenv.CAPIOperatorDeployProviderInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			CAPIProvidersYAML: [][]byte{
				e2e.CapiProviders,
			},
			WaitForDeployments: testenv.DefaultDeployments,
		})

		ccInput := testenv.CreateClusterInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			Namespace:             namespace.Name,
			ClusterTemplate:       e2e.CAPIDockerRKE2Topology,
			ClusterName:           clusterName,
			AdditionalTemplateVariables: map[string]string{
				"RKE2_CNI":     "calico",
				"RKE2_VERSION": e2eConfig.GetVariableOrEmpty(e2e.RKE2VersionVar),
			},
			AdditionalFleetGitRepos: []framework.FleetCreateGitRepoInput{
				{
					Name:  "docker-cluster-classes-regular",
					Paths: []string{"examples/clusterclasses/docker/rke2"},
				},
				{
					Name:  "docker-cni",
					Paths: []string{"examples/applications/cni/calico"},
				},
			},
			WaitForCreatedCluster: e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher"),
		}
		testenv.CreateCluster(ctx, ccInput)

		chartMuseumDeployInput := testenv.DeployChartMuseumInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
		}

		testenv.DeployChartMuseum(ctx, chartMuseumDeployInput)

		upgradeInput := testenv.UpgradeRancherTurtlesInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			AdditionalValues:      rtInput.AdditionalValues,
			PostUpgradeSteps:      []func(){},
		}

		upgradeInput.PostUpgradeSteps = append(upgradeInput.PostUpgradeSteps, func() {
			By("Waiting for the upstream CAPI operator deployment to be removed")
			framework.WaitForDeploymentsRemoved(ctx, framework.WaitForDeploymentsRemovedInput{
				Getter: bootstrapClusterProxy.GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      "rancher-turtles-cluster-api-operator",
					Namespace: e2e.RancherTurtlesNamespace,
				}},
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
		})

		upgradeInput.PostUpgradeSteps = append(upgradeInput.PostUpgradeSteps, func() {
			By("Waiting for CAAPF deployment to be available")
			capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
				Getter: bootstrapClusterProxy.GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      "caapf-controller-manager",
					Namespace: e2e.RancherTurtlesNamespace,
				}},
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
		})

		upgradeInput.PostUpgradeSteps = append(upgradeInput.PostUpgradeSteps, func() {
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
		}, func() {
			framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
				Getter:    bootstrapClusterProxy.GetClient(),
				Version:   e2e.CAPIVersion,
				Name:      "kubeadm-control-plane",
				Namespace: "capi-kubeadm-control-plane-system",
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
		}, func() {
			framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
				Getter: bootstrapClusterProxy.GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      "rke2-bootstrap-controller-manager",
					Namespace: "rke2-bootstrap-system",
				}},
				Image:     "registry.suse.com/rancher/cluster-api-provider-rke2-bootstrap:",
				Name:      "rke2-bootstrap",
				Namespace: "rke2-bootstrap-system",
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
		}, func() {
			framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
				Getter: bootstrapClusterProxy.GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      "rke2-control-plane-controller-manager",
					Namespace: "rke2-control-plane-system",
				}},
				Image:     "registry.suse.com/rancher/cluster-api-provider-rke2-controlplane:",
				Name:      "rke2-control-plane",
				Namespace: "rke2-control-plane-system",
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
		}, func() {
			framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
				Getter: bootstrapClusterProxy.GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      "caapf-controller-manager",
					Namespace: "rancher-turtles-system",
				}},
				Image:     "registry.suse.com/rancher/cluster-api-addon-provider-fleet:",
				Name:      "fleet",
				Namespace: "rancher-turtles-system",
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
		}, func() {
			framework.WaitForCAPIProviderRollout(ctx, framework.WaitForCAPIProviderRolloutInput{
				Getter:    bootstrapClusterProxy.GetClient(),
				Version:   e2e.CAPIVersion,
				Name:      "docker",
				Namespace: "capd-system",
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
		})

		upgradeInput.PostUpgradeSteps = append(upgradeInput.PostUpgradeSteps, func() {
			framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
				Lister: bootstrapClusterProxy.GetClient(),
				GroupVersionKind: schema.GroupVersionKind{
					Group:   "operator.cluster.x-k8s.io",
					Version: "v1alpha2",
					Kind:    "AddonProvider",
				},
			})
		}, func() {
			framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
				Lister: bootstrapClusterProxy.GetClient(),
				GroupVersionKind: schema.GroupVersionKind{
					Group:   "operator.cluster.x-k8s.io",
					Version: "v1alpha2",
					Kind:    "BootstrapProvider",
				},
			})
		}, func() {
			framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
				Lister: bootstrapClusterProxy.GetClient(),
				GroupVersionKind: schema.GroupVersionKind{
					Group:   "operator.cluster.x-k8s.io",
					Version: "v1alpha2",
					Kind:    "ControlPlaneProvider",
				},
			})
		}, func() {
			framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
				Lister: bootstrapClusterProxy.GetClient(),
				GroupVersionKind: schema.GroupVersionKind{
					Group:   "operator.cluster.x-k8s.io",
					Version: "v1alpha2",
					Kind:    "CoreProvider",
				},
			})
		}, func() {
			framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
				Lister: bootstrapClusterProxy.GetClient(),
				GroupVersionKind: schema.GroupVersionKind{
					Group:   "operator.cluster.x-k8s.io",
					Version: "v1alpha2",
					Kind:    "InfrastructureProvider",
				},
			})
		}, func() {
			framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
				Lister: bootstrapClusterProxy.GetClient(),
				GroupVersionKind: schema.GroupVersionKind{
					Group:   "operator.cluster.x-k8s.io",
					Version: "v1alpha2",
					Kind:    "IPAMProvider",
				},
			})
		}, func() {
			framework.VerifyCustomResourceHasBeenRemoved(ctx, framework.VerifyCustomResourceHasBeenRemovedInput{
				Lister: bootstrapClusterProxy.GetClient(),
				GroupVersionKind: schema.GroupVersionKind{
					Group:   "operator.cluster.x-k8s.io",
					Version: "v1alpha2",
					Kind:    "RuntimeExtensionProvider",
				},
			})
		}, func() {
			framework.VerifyCluster(ctx, framework.VerifyClusterInput{
				GetLister: bootstrapClusterProxy.GetClient(),
				Name:      clusterName,
				Namespace: namespace.Name,
			})
		})

		testenv.UpgradeRancherTurtles(ctx, upgradeInput)

		testenv.DeployRancherTurtlesProviders(ctx, testenv.DeployRancherTurtlesProvidersInput{
			BootstrapClusterProxy: bootstrapClusterProxy,
			AdditionalValues: map[string]string{
				// CAAPF and CAPRKE2 are enabled by default in the providers chart
				"providers.bootstrapKubeadm.enabled":     "true",
				"providers.controlplaneKubeadm.enabled":  "true",
				"providers.infrastructureDocker.enabled": "true",
				"providers.addonFleet.namespace":         e2e.RancherTurtlesNamespace,
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

		verifyAdopted("fleet", e2e.RancherTurtlesNamespace)
		verifyAdopted("rke2-bootstrap", "rke2-bootstrap-system")
		verifyAdopted("rke2-control-plane", "rke2-control-plane-system")
	})
})
