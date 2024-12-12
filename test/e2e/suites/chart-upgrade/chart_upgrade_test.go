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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/rancher/turtles/test/e2e"
	"github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	capiframework "sigs.k8s.io/cluster-api/test/framework"
)

var _ = Describe("Chart upgrade functionality should work", Label(e2e.ShortTestLabel), func() {
	BeforeEach(func() {
		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)

	})

	It("Should perform upgrade from GA version to latest", func() {
		rtInput := testenv.DeployRancherTurtlesInput{
			BootstrapClusterProxy:        bootstrapClusterProxy,
			HelmBinaryPath:               e2eConfig.GetVariable(e2e.HelmBinaryPathVar),
			TurtlesChartPath:             "https://rancher.github.io/turtles",
			CAPIProvidersYAML:            e2e.CapiProvidersLegacy,
			Namespace:                    framework.DefaultRancherTurtlesNamespace,
			Version:                      "v0.6.0",
			WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers"),
			AdditionalValues:             map[string]string{},
		}
		testenv.DeployRancherTurtles(ctx, rtInput)

		chartMuseumDeployInput := testenv.DeployChartMuseumInput{
			HelmBinaryPath:        e2eConfig.GetVariable(e2e.HelmBinaryPathVar),
			ChartsPath:            e2eConfig.GetVariable(e2e.TurtlesPathVar),
			ChartVersion:          e2eConfig.GetVariable(e2e.TurtlesVersionVar),
			BootstrapClusterProxy: bootstrapClusterProxy,
			WaitInterval:          e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers"),
			Variables:             e2eConfig.Variables,
		}

		testenv.PreChartMuseumInstallHook(&chartMuseumDeployInput, e2eConfig)

		testenv.DeployChartMuseum(ctx, chartMuseumDeployInput)

		upgradeInput := testenv.UpgradeRancherTurtlesInput{
			BootstrapClusterProxy:        bootstrapClusterProxy,
			HelmBinaryPath:               e2eConfig.GetVariable(e2e.HelmBinaryPathVar),
			Namespace:                    framework.DefaultRancherTurtlesNamespace,
			Image:                        "ghcr.io/rancher/turtles-e2e",
			Tag:                          e2eConfig.GetVariable(e2e.TurtlesVersionVar),
			WaitDeploymentsReadyInterval: e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers"),
			AdditionalValues:             rtInput.AdditionalValues,
			PostUpgradeSteps:             []func(){},
		}

		testenv.PreRancherTurtlesInstallHook(&rtInput, e2eConfig)

		rtInput.AdditionalValues["rancherTurtles.features.addon-provider-fleet.enabled"] = "true"
		rtInput.AdditionalValues["rancherTurtles.features.managementv3-cluster.enabled"] = "false" // disable the default management.cattle.io/v3 controller

		upgradeInput.PostUpgradeSteps = append(upgradeInput.PostUpgradeSteps, func() {
			By("Waiting for CAAPF deployment to be available")
			capiframework.WaitForDeploymentsAvailable(ctx, capiframework.WaitForDeploymentsAvailableInput{
				Getter: bootstrapClusterProxy.GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      "caapf-controller-manager",
					Namespace: e2e.RancherTurtlesNamespace,
				}},
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

			By("Setting the CAAPF config to use hostNetwork")
			Expect(bootstrapClusterProxy.Apply(ctx, e2e.AddonProviderFleetHostNetworkPatch)).To(Succeed())
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
				Getter:    bootstrapClusterProxy.GetClient(),
				Version:   e2e.CAPIVersion,
				Name:      "docker",
				Namespace: "capd-system",
			}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)
		})

		testenv.UpgradeRancherTurtles(ctx, upgradeInput)
	})
})
