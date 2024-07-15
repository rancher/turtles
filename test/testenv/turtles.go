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

package testenv

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opframework "sigs.k8s.io/cluster-api-operator/test/framework"
	"sigs.k8s.io/cluster-api/test/framework"

	turtlesframework "github.com/rancher/turtles/test/framework"
)

type DeployRancherTurtlesInput struct {
	BootstrapClusterProxy        framework.ClusterProxy
	HelmBinaryPath               string
	ChartPath                    string
	CAPIProvidersSecretYAML      []byte
	CAPIProvidersYAML            []byte
	Namespace                    string
	Image                        string
	Tag                          string
	Version                      string
	WaitDeploymentsReadyInterval []interface{}
	AdditionalValues             map[string]string
}

type UninstallRancherTurtlesInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	HelmBinaryPath        string
	Namespace             string
	DeleteWaitInterval    []interface{}
}

func DeployRancherTurtles(ctx context.Context, input DeployRancherTurtlesInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DeployRancherTurtles")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for DeployRancherTurtles")
	Expect(input.CAPIProvidersYAML).ToNot(BeNil(), "CAPIProvidersYAML is required for DeployRancherTurtles")
	Expect(input.ChartPath).ToNot(BeEmpty(), "ChartPath is required for DeployRancherTurtles")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for DeployRancherTurtles")
	Expect(input.WaitDeploymentsReadyInterval).ToNot(BeNil(), "WaitDeploymentsReadyInterval is required for DeployRancherTurtles")

	namespace := input.Namespace
	if namespace == "" {
		namespace = turtlesframework.DefaultRancherTurtlesNamespace
	}

	if input.CAPIProvidersSecretYAML != nil {
		By("Adding CAPI variables secret")
		Expect(input.BootstrapClusterProxy.Apply(ctx, input.CAPIProvidersSecretYAML)).To(Succeed())
	}

	chartPath := input.ChartPath
	if input.Version != "" {
		chartPath = "rancher-turtles-external/rancher-turtles"

		By("Adding external rancher turtles chart repo")
		addChart := &opframework.HelmChart{
			BinaryPath:      input.HelmBinaryPath,
			Name:            "rancher-turtles-external",
			Path:            input.ChartPath,
			Commands:        opframework.Commands(opframework.Repo, opframework.Add),
			AdditionalFlags: opframework.Flags("--force-update"),
			Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
		}
		_, err := addChart.Run(nil)
		Expect(err).ToNot(HaveOccurred())
	}

	By("Installing rancher-turtles chart")
	chart := &opframework.HelmChart{
		BinaryPath: input.HelmBinaryPath,
		Path:       chartPath,
		Name:       "rancher-turtles",
		Kubeconfig: input.BootstrapClusterProxy.GetKubeconfigPath(),
		AdditionalFlags: opframework.Flags(
			"--dependency-update",
			"-n", namespace,
			"--create-namespace", "--wait"),
	}

	values := map[string]string{
		"rancherTurtles.managerArguments[0]":                 "--insecure-skip-verify=true",
		"cluster-api-operator.cluster-api.configSecret.name": "variables",
	}

	if input.Image != "" && input.Tag != "" {
		values["rancherTurtles.image"] = input.Image
		values["rancherTurtles.imageVersion"] = input.Tag
		values["rancherTurtles.tag"] = input.Tag
	} else if input.Version != "" {
		chart.AdditionalFlags = append(chart.AdditionalFlags, opframework.Flags(
			"--version", input.Version,
		)...)
	}

	for name, val := range input.AdditionalValues {
		values[name] = val
	}

	_, err := chart.Run(values)
	Expect(err).ToNot(HaveOccurred())

	// TODO: this can probably be covered by the Operator helper

	By("Adding CAPI infrastructure providers")
	Expect(input.BootstrapClusterProxy.Apply(ctx, input.CAPIProvidersYAML)).To(Succeed())

	By("Waiting for CAPI deployment to be available")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "capi-controller-manager",
				Namespace: "capi-system",
			},
		},
	}, input.WaitDeploymentsReadyInterval...)

	By("Waiting for CAPI kubeadm bootstrap deployment to be available")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-kubeadm-bootstrap-controller-manager",
			Namespace: "capi-kubeadm-bootstrap-system",
		}},
	}, input.WaitDeploymentsReadyInterval...)

	By("Waiting for CAPI kubeadm control plane deployment to be available")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      "capi-kubeadm-control-plane-controller-manager",
			Namespace: "capi-kubeadm-control-plane-system",
		}},
	}, input.WaitDeploymentsReadyInterval...)

	By("Waiting for CAPI docker provider deployment to be available")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      "capd-controller-manager",
			Namespace: "capd-system",
		}},
	}, input.WaitDeploymentsReadyInterval...)

	By("Waiting for CAPI RKE2 bootstrap deployment to be available")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      "rke2-bootstrap-controller-manager",
			Namespace: "rke2-bootstrap-system",
		}},
	}, input.WaitDeploymentsReadyInterval...)

	By("Waiting for CAPI RKE2 control plane deployment to be available")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      "rke2-control-plane-controller-manager",
			Namespace: "rke2-control-plane-system",
		}},
	}, input.WaitDeploymentsReadyInterval...)
}

type UpgradeRancherTurtlesInput struct {
	BootstrapClusterProxy        framework.ClusterProxy
	HelmBinaryPath               string
	Namespace                    string
	WaitDeploymentsReadyInterval []interface{}
	AdditionalValues             map[string]string
	Image                        string
	Tag                          string
	SkipCleanup                  bool
}

func UpgradeRancherTurtles(ctx context.Context, input UpgradeRancherTurtlesInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for UpgradeRancherTurtles")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for UpgradeRancherTurtles")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for UpgradeRancherTurtles")
	Expect(input.Image).ToNot(BeEmpty(), "Image is required for UpgradeRancherTurtles")
	Expect(input.Tag).ToNot(BeEmpty(), "Tag is required for UpgradeRancherTurtles")
	Expect(input.WaitDeploymentsReadyInterval).ToNot(BeNil(), "WaitDeploymentsReadyInterval is required for UpgradeRancherTurtles")

	By("Upgrading rancher-turtles chart")

	additionalValues := []string{}
	for name, val := range input.AdditionalValues {
		additionalValues = append(additionalValues, "--set", fmt.Sprintf("%s=%s", name, val))
	}

	if !input.SkipCleanup {
		defer func() {
			values := []string{"repo", "remove", "rancher-turtles-local"}
			cmd := exec.Command(
				input.HelmBinaryPath,
				values...,
			)
			cmd.WaitDelay = time.Minute
			out, err := cmd.CombinedOutput()
			if err != nil {
				Expect(fmt.Errorf("Unable to perform chart removal: %w\nOutput: %s, Command: %s", err, out, strings.Join(append(values, additionalValues...), " "))).ToNot(HaveOccurred())
			}
		}()
	}

	values := []string{"repo", "update"}
	cmd := exec.Command(
		input.HelmBinaryPath,
		values...,
	)
	cmd.WaitDelay = time.Minute
	out, err := cmd.CombinedOutput()
	if err != nil {
		Expect(fmt.Errorf("Unable to perform chart index update: %w\nOutput: %s, Command: %s", err, out, strings.Join(append(values, additionalValues...), " "))).ToNot(HaveOccurred())
	}

	values = []string{
		"upgrade", "rancher-turtles", "rancher-turtles-local/rancher-turtles",
		"-n", input.Namespace,
		"--wait",
		"--kubeconfig", input.BootstrapClusterProxy.GetKubeconfigPath(),
		"--set", "rancherTurtles.managerArguments[0]=--insecure-skip-verify=true",
		"--set", fmt.Sprintf("rancherTurtles.image=%s", input.Image),
		"--set", fmt.Sprintf("rancherTurtles.imageVersion=%s", input.Tag),
		"--set", fmt.Sprintf("rancherTurtles.tag=%s", input.Tag),
	}

	cmd = exec.Command(
		input.HelmBinaryPath,
		append(values, additionalValues...)...,
	)
	cmd.WaitDelay = time.Minute
	out, err = cmd.CombinedOutput()

	if err != nil {
		Expect(fmt.Errorf("Unable to perform chart upgrade: %w\nOutput: %s, Command: %s", err, out, strings.Join(append(values, additionalValues...), " "))).ToNot(HaveOccurred())
	}
}

func UninstallRancherTurtles(ctx context.Context, input UninstallRancherTurtlesInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for UninstallRancherTurtles")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for UninstallRancherTurtles")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for UninstallRancherTurtles")
	Expect(input.DeleteWaitInterval).ToNot(BeNil(), "DeleteWaitInterval is required for UninstallRancherTurtles")

	namespace := input.Namespace
	if namespace == "" {
		namespace = turtlesframework.DefaultRancherTurtlesNamespace
	}

	By("Removing Turtles chart")
	removeChart := &opframework.HelmChart{
		BinaryPath: input.HelmBinaryPath,
		Name:       "rancher-turtles",
		Commands:   opframework.HelmCommands{opframework.Uninstall},
		Kubeconfig: input.BootstrapClusterProxy.GetKubeconfigPath(),
		AdditionalFlags: opframework.Flags(
			"-n", namespace,
			"--cascade", "foreground",
			"--wait"),
	}

	_, err := removeChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())
}
