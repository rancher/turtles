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

	"github.com/rancher/turtles/test/e2e"
	turtlesframework "github.com/rancher/turtles/test/framework"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opframework "sigs.k8s.io/cluster-api-operator/test/framework"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
)

// DeployRancherTurtlesInput represents the input parameters for deploying Rancher Turtles.
type DeployRancherTurtlesInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string

	// TurtlesChartUrl is the URL of the Turtles chart.
	TurtlesChartUrl string

	// TurtlesChartPath is the path to the Turtles chart.
	TurtlesChartPath string

	// TurtlesChartRepoName is the name of the Turtles chart repository.
	TurtlesChartRepoName string

	// CAPIProvidersSecretYAML is the YAML content of the CAPI providers secret.
	CAPIProvidersSecretYAML []byte

	// CAPIProvidersYAML is the YAML content of the CAPI providers.
	CAPIProvidersYAML []byte

	// Namespace is the namespace for deploying Rancher Turtles.
	Namespace string

	// Image is the image for Rancher Turtles.
	Image string

	// Tag is the tag for Rancher Turtles.
	Tag string

	// Version is the version of Rancher Turtles.
	Version string

	// WaitDeploymentsReadyInterval is the interval for waiting for deployments to be ready.
	WaitDeploymentsReadyInterval []interface{}

	// AdditionalValues are the additional values for Rancher Turtles.
	AdditionalValues map[string]string
}

// UninstallRancherTurtlesInput represents the input parameters for uninstalling Rancher Turtles.
type UninstallRancherTurtlesInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string

	// Namespace is the namespace where Rancher Turtles are installed.
	Namespace string

	// DeleteWaitInterval is the wait interval for deleting resources.
	DeleteWaitInterval []interface{}
}

// DeployRancherTurtles deploys Rancher Turtles to the specified Kubernetes cluster.
// It expects the required input parameters to be non-nil.
// If the version is specified but the TurtlesChartUrl is empty, it adds an external rancher turtles chart repo for chartmuseum use-case. If the TurtlesChartUrl is specified, it adds the Rancher chart repo.
// After adding the necessary chart repos, the function installs the rancher-turtles chart. It sets the additional values for the chart based on the input parameters.
// If the image and tag are specified, it sets the corresponding values in the chart. If only the version is specified, it adds the version flag to the chart's additional flags.
// The function then adds the CAPI infrastructure providers and waits for the CAPI deployments to be available. It waits for the capi-controller-manager, capi-kubeadm-bootstrap-controller-manager,
// capi-kubeadm-control-plane-controller-manager, capd-controller-manager, rke2-bootstrap-controller-manager, and rke2-control-plane-controller-manager deployments to be available.
func DeployRancherTurtles(ctx context.Context, input DeployRancherTurtlesInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DeployRancherTurtles")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for DeployRancherTurtles")
	Expect(input.CAPIProvidersYAML).ToNot(BeNil(), "CAPIProvidersYAML is required for DeployRancherTurtles")
	Expect(input.TurtlesChartPath).ToNot(BeEmpty(), "ChartPath is required for DeployRancherTurtles")
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

	chartPath := input.TurtlesChartPath
	if input.Version != "" && input.TurtlesChartUrl == "" {
		chartPath = "rancher-turtles-external/rancher-turtles"

		By("Adding external rancher turtles chart repo")
		addChart := &opframework.HelmChart{
			BinaryPath:      input.HelmBinaryPath,
			Name:            "rancher-turtles-external",
			Path:            input.TurtlesChartPath,
			Commands:        opframework.Commands(opframework.Repo, opframework.Add),
			AdditionalFlags: opframework.Flags("--force-update"),
			Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
		}
		_, err := addChart.Run(nil)
		Expect(err).ToNot(HaveOccurred())
	}

	if input.TurtlesChartUrl != "" {
		By("Adding Rancher chart repo")
		addChart := &opframework.HelmChart{
			BinaryPath:      input.HelmBinaryPath,
			Name:            input.TurtlesChartRepoName,
			Path:            input.TurtlesChartUrl,
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

// UpgradeRancherTurtlesInput represents the input parameters for upgrading Rancher Turtles.
type UpgradeRancherTurtlesInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string

	// Namespace is the namespace for the deployment.
	Namespace string

	// WaitDeploymentsReadyInterval is the interval for waiting until deployments are ready.
	WaitDeploymentsReadyInterval []interface{}

	// AdditionalValues are the additional values for the Helm chart.
	AdditionalValues map[string]string

	// Image is the image for the deployment.
	Image string

	// Tag is the tag for the deployment.
	Tag string

	// PostUpgradeSteps are the post-upgrade steps to be executed.
	PostUpgradeSteps []func()

	// SkipCleanup indicates whether to skip the cleanup after the upgrade.
	SkipCleanup bool
}

// UpgradeRancherTurtles upgrades the rancher-turtles chart.
// It expects the required input parameters to be non-nil.
// The function performs the following steps:
// 1. Validates the input parameters to ensure they are not empty or nil.
// 2. Upgrades the rancher-turtles chart by executing the necessary helm commands.
// 3. Executes any post-upgrade steps provided in the input.
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

	for _, step := range input.PostUpgradeSteps {
		step()
	}
}

// UninstallRancherTurtles uninstalls the Rancher Turtles chart.
// It expects the required input parameters to be non-nil.
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

// PreRancherTurtlesInstallHook is a function that sets additional values for the Rancher Turtles installation based on the management cluster environment type.
// If the infrastructure type is e2e.ManagementClusterEnvironmentEKS, the image pull secrets are set to "{regcred}" and the image pull policy is set to "IfNotPresent".
// If the infrastructure type is e2e.ManagementClusterEnvironmentIsolatedKind or e2e.ManagementClusterEnvironmentKind, the image pull policy is set to "Never".
// If the infrastructure type is not recognized, the function fails with an error message indicating the invalid infrastructure type.
func PreRancherTurtlesInstallHook(rtInput *DeployRancherTurtlesInput, e2eConfig *clusterctl.E2EConfig) {
	infrastructureType := e2e.ManagementClusterEnvironmentType(e2eConfig.GetVariable(e2e.ManagementClusterEnvironmentVar))

	switch infrastructureType {
	case e2e.ManagementClusterEnvironmentEKS:
		rtInput.AdditionalValues["rancherTurtles.imagePullSecrets"] = "{regcred}"
		rtInput.AdditionalValues["rancherTurtles.imagePullPolicy"] = "IfNotPresent"
	case e2e.ManagementClusterEnvironmentIsolatedKind:
		// NOTE: rancher turtles image is loaded into kind manually, we can set the imagePullPolicy to Never
		rtInput.AdditionalValues["rancherTurtles.imagePullPolicy"] = "Never"
	case e2e.ManagementClusterEnvironmentKind:
		rtInput.AdditionalValues["rancherTurtles.imagePullPolicy"] = "Never"
	default:
		Fail(fmt.Sprintf("Invalid management cluster infrastructure type %q", infrastructureType))
	}
}

// PreRancherTurtlesUpgradelHook is a function that handles the pre-upgrade hook for Rancher Turtles.
// If the infrastructure type is e2e.ManagementClusterEnvironmentEKS, it sets the imagePullSecrets and imagePullPolicy values in rtUpgradeInput.
// If the infrastructure type is e2e.ManagementClusterEnvironmentIsolatedKind, it sets the imagePullPolicy value in rtUpgradeInput to "Never".
// If the infrastructure type is e2e.ManagementClusterEnvironmentKind, it sets the imagePullPolicy value in rtUpgradeInput to "Never".
// If the infrastructure type is not recognized, it fails with an error message indicating the invalid infrastructure type.
func PreRancherTurtlesUpgradelHook(rtUpgradeInput *UpgradeRancherTurtlesInput, e2eConfig *clusterctl.E2EConfig) {
	infrastructureType := e2e.ManagementClusterEnvironmentType(e2eConfig.GetVariable(e2e.ManagementClusterEnvironmentVar))

	switch infrastructureType {
	case e2e.ManagementClusterEnvironmentEKS:
		rtUpgradeInput.AdditionalValues["rancherTurtles.imagePullSecrets"] = "{regcred}"
		rtUpgradeInput.AdditionalValues["rancherTurtles.imagePullPolicy"] = "IfNotPresent"
	case e2e.ManagementClusterEnvironmentIsolatedKind:
		// NOTE: rancher turtles image is loaded into kind manually, we can set the imagePullPolicy to Never
		rtUpgradeInput.AdditionalValues["rancherTurtles.imagePullPolicy"] = "Never"
	case e2e.ManagementClusterEnvironmentKind:
		rtUpgradeInput.AdditionalValues["rancherTurtles.imagePullPolicy"] = "Never"
	default:
		Fail(fmt.Sprintf("Invalid management cluster infrastructure type %q", infrastructureType))
	}
}
