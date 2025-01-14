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
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var DefaultDeployments = []NamespaceName{
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
}

// DeployRancherTurtlesInput represents the input parameters for deploying Rancher Turtles.
type DeployRancherTurtlesInput struct {
	// EnvironmentType is the environment type
	EnvironmentType e2e.ManagementClusterEnvironmentType `env:"MANAGEMENT_CLUSTER_ENVIRONMENT"`

	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string `env:"HELM_BINARY_PATH"`

	// TurtlesChartUrl is the URL of the Turtles chart.
	TurtlesChartUrl string

	// TurtlesChartPath is the path to the Turtles chart.
	TurtlesChartPath string `env:"TURTLES_PATH"`

	// TurtlesChartRepoName is the name of the Turtles chart repository.
	TurtlesChartRepoName string

	// CAPIProvidersSecretYAML is the YAML content of the CAPI providers secret.
	CAPIProvidersSecretYAML []byte

	// CAPIProvidersYAML is the YAML content of the CAPI providers.
	CAPIProvidersYAML []byte

	// ConfigurationPatches is a list of additional patches to apply after turtles install
	ConfigurationPatches [][]byte

	// Namespace is the namespace for deploying Rancher Turtles.
	Namespace string `envDefault:"rancher-turtles-system"`

	// Image is the image for Rancher Turtles.
	Image string `env:"TURTLES_IMAGE"`

	// Tag is the tag for Rancher Turtles.
	Tag string `env:"TURTLES_VERSION"`

	// Version is the version of Rancher Turtles.
	Version string

	// WaitDeploymentsReadyInterval is the interval for waiting for deployments to be ready.
	WaitDeploymentsReadyInterval []interface{} `envDefault:"15m,10s"`

	// WaitForDeployments is the list of deployments to wait for.
	WaitForDeployments []NamespaceName

	// AdditionalValues are the additional values for Rancher Turtles.
	AdditionalValues map[string]string
}

// UninstallRancherTurtlesInput represents the input parameters for uninstalling Rancher Turtles.
type UninstallRancherTurtlesInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string `env:"HELM_BINARY_PATH"`

	// Namespace is the namespace where Rancher Turtles are installed.
	Namespace string `envDefault:"rancher-turtles-system"`

	// DeleteWaitInterval is the wait interval for deleting resources.
	DeleteWaitInterval []interface{} `envDefault:"10m,10s"`
}

// DeployRancherTurtles deploys Rancher Turtles to the specified Kubernetes cluster.
// It expects the required input parameters to be non-nil.
// If the version is specified but the TurtlesChartUrl is empty, it adds an external rancher turtles chart repo for chartmuseum use-case. If the TurtlesChartUrl is specified, it adds the Rancher chart repo.
// After adding the necessary chart repos, the function installs the rancher-turtles chart. It sets the additional values for the chart based on the input parameters.
// If the image and tag are specified, it sets the corresponding values in the chart. If only the version is specified, it adds the version flag to the chart's additional flags.
// The function then adds the CAPI infrastructure providers and waits for the CAPI deployments to be available. It waits for the capi-controller-manager, capi-kubeadm-bootstrap-controller-manager,
// capi-kubeadm-control-plane-controller-manager, capd-controller-manager, rke2-bootstrap-controller-manager, and rke2-control-plane-controller-manager deployments to be available.
func DeployRancherTurtles(ctx context.Context, input DeployRancherTurtlesInput) {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")
	PreRancherTurtlesInstallHook(&input)

	Expect(ctx).NotTo(BeNil(), "ctx is required for DeployRancherTurtles")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for DeployRancherTurtles")
	Expect(input.CAPIProvidersYAML).ToNot(BeNil(), "CAPIProvidersYAML is required for DeployRancherTurtles")
	Expect(input.TurtlesChartPath).ToNot(BeEmpty(), "ChartPath is required for DeployRancherTurtles")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for DeployRancherTurtles")
	Expect(input.WaitDeploymentsReadyInterval).ToNot(BeNil(), "WaitDeploymentsReadyInterval is required for DeployRancherTurtles")

	namespace := input.Namespace

	if input.CAPIProvidersSecretYAML != nil {
		By("Adding CAPI variables secret")
		Expect(turtlesframework.Apply(ctx, input.BootstrapClusterProxy, input.CAPIProvidersSecretYAML)).To(Succeed())
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
		By("Adding Rancher turtles chart repo")
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
	values := map[string]string{
		"rancherTurtles.managerArguments[0]":                 "--insecure-skip-verify=true",
		"cluster-api-operator.cluster-api.configSecret.name": "variables",
	}

	if input.Version == "" {
		values["rancherTurtles.image"] = input.Image
		values["rancherTurtles.imageVersion"] = input.Tag
		values["rancherTurtles.tag"] = input.Tag
	}

	for name, val := range input.AdditionalValues {
		values[name] = val
	}

	command := []string{
		"upgrade", "rancher-turtles", chartPath,
		"--install",
		"--dependency-update",
		"-n", namespace,
		"--create-namespace",
		"--wait",
		"--timeout", "10m",
		"--kubeconfig", input.BootstrapClusterProxy.GetKubeconfigPath(),
	}

	if input.Version != "" {
		command = append(command, "--version", input.Version)
	}

	for k, v := range values {
		command = append(command, "--set", fmt.Sprintf("%s=%s", k, v))
	}

	fullCommand := append([]string{input.HelmBinaryPath}, command...)
	log.FromContext(ctx).Info("Executing:", "install", strings.Join(fullCommand, " "))

	cmd := exec.Command(
		input.HelmBinaryPath,
		command...,
	)
	cmd.WaitDelay = 10 * time.Minute

	out, err := cmd.CombinedOutput()
	if err != nil {
		Expect(fmt.Errorf("Unable to perform chart upgrade: %w\nOutput: %s, Command: %s", err, out, strings.Join(fullCommand, " "))).ToNot(HaveOccurred())
	}

	By("Adding CAPI infrastructure providers")
	Expect(turtlesframework.Apply(ctx, input.BootstrapClusterProxy, input.CAPIProvidersYAML)).To(Succeed())

	if input.WaitForDeployments != nil {
		By("Waiting for provider deployments to be ready")
	}

	for _, nn := range input.WaitForDeployments {
		turtlesframework.Byf("Waiting for CAPI deployment %s/%s to be available", nn.Namespace, nn.Name)
		framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
			Getter: input.BootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      nn.Name,
				Namespace: nn.Namespace,
			}},
		}, input.WaitDeploymentsReadyInterval...)
	}

	if input.ConfigurationPatches != nil {
		By("Applying configuration patches")
	}

	for _, patch := range input.ConfigurationPatches {
		Expect(turtlesframework.Apply(ctx, input.BootstrapClusterProxy, patch)).To(Succeed(), "Failed to apply configuration patch")
	}
}

// UpgradeRancherTurtlesInput represents the input parameters for upgrading Rancher Turtles.
type UpgradeRancherTurtlesInput struct {
	// EnvironmentType is the environment type
	EnvironmentType e2e.ManagementClusterEnvironmentType `env:"MANAGEMENT_CLUSTER_ENVIRONMENT"`

	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string `env:"HELM_BINARY_PATH"`

	// Namespace is the namespace for the deployment.
	Namespace string `envDefault:"rancher-turtles-system"`

	// WaitDeploymentsReadyInterval is the interval for waiting until deployments are ready.
	WaitDeploymentsReadyInterval []interface{} `envDefault:"15m,10s"`

	// AdditionalValues are the additional values for the Helm chart.
	AdditionalValues map[string]string

	// Image is the image for the deployment.
	Image string `env:"TURTLES_IMAGE"`

	// Tag is the tag for the deployment.
	Tag string `env:"TURTLES_VERSION"`

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
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for UpgradeRancherTurtles")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for UpgradeRancherTurtles")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for UpgradeRancherTurtles")
	Expect(input.Image).ToNot(BeEmpty(), "Image is required for UpgradeRancherTurtles")
	Expect(input.Tag).ToNot(BeEmpty(), "Tag is required for UpgradeRancherTurtles")
	Expect(input.WaitDeploymentsReadyInterval).ToNot(BeNil(), "WaitDeploymentsReadyInterval is required for UpgradeRancherTurtles")

	By("Upgrading rancher-turtles chart")

	PreRancherTurtlesUpgradelHook(&input)

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

			fullCommand := append([]string{input.HelmBinaryPath}, values...)
			log.FromContext(ctx).Info("Executing:", "cleanup", strings.Join(fullCommand, " "))
			out, err := cmd.CombinedOutput()
			if err != nil {
				Expect(fmt.Errorf("Unable to perform chart removal: %w\nOutput: %s, Command: %s", err, out, strings.Join(append(values, additionalValues...), " "))).ToNot(HaveOccurred())
			}
		}()
	}

	By("Updating the chart index")
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
		"--install",
		"--wait",
		"--timeout", "10m",
		"--kubeconfig", input.BootstrapClusterProxy.GetKubeconfigPath(),
		"--set", "rancherTurtles.managerArguments[0]=--insecure-skip-verify=true",
		"--set", fmt.Sprintf("rancherTurtles.image=%s", input.Image),
		"--set", fmt.Sprintf("rancherTurtles.imageVersion=%s", input.Tag),
		"--set", fmt.Sprintf("rancherTurtles.tag=%s", input.Tag),
	}

	fullCommand := append([]string{input.HelmBinaryPath}, values...)
	log.FromContext(ctx).Info("Executing:", "upgrade", strings.Join(fullCommand, " "))

	cmd = exec.Command(
		input.HelmBinaryPath,
		append(values, additionalValues...)...,
	)
	cmd.WaitDelay = 10 * time.Minute
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
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for UninstallRancherTurtles")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for UninstallRancherTurtles")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for UninstallRancherTurtles")
	Expect(input.DeleteWaitInterval).ToNot(BeNil(), "DeleteWaitInterval is required for UninstallRancherTurtles")

	By("Removing Turtles chart")
	removeChart := &opframework.HelmChart{
		BinaryPath: input.HelmBinaryPath,
		Name:       "rancher-turtles",
		Commands:   opframework.HelmCommands{opframework.Uninstall},
		Kubeconfig: input.BootstrapClusterProxy.GetKubeconfigPath(),
		AdditionalFlags: opframework.Flags(
			"-n", input.Namespace,
			"--cascade", "foreground",
			"--wait"),
	}

	_, err := removeChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())
}

// PreRancherTurtlesInstallHook is a function that sets additional values for the Rancher Turtles installation based on the management cluster environment type.
// If the infrastructure type is e2e.ManagementClusterEnvironmentEKS, the image pull secrets are set to "{regcred}".
func PreRancherTurtlesInstallHook(rtInput *DeployRancherTurtlesInput) {
	Expect(turtlesframework.Parse(rtInput)).To(Succeed(), "Failed to parse environment variables")
	if rtInput.AdditionalValues == nil {
		rtInput.AdditionalValues = map[string]string{}
	}
	switch rtInput.EnvironmentType {
	case e2e.ManagementClusterEnvironmentEKS:
		rtInput.AdditionalValues["rancherTurtles.imagePullSecrets"] = "{regcred}"
	}
}

// PreRancherTurtlesUpgradelHook is a function that handles the pre-upgrade hook for Rancher Turtles.
// If the infrastructure type is e2e.ManagementClusterEnvironmentEKS, it sets the imagePullSecrets and imagePullPolicy values in rtUpgradeInput.
func PreRancherTurtlesUpgradelHook(rtUpgradeInput *UpgradeRancherTurtlesInput) {
	Expect(turtlesframework.Parse(rtUpgradeInput)).To(Succeed(), "Failed to parse environment variables")
	if rtUpgradeInput.AdditionalValues == nil {
		rtUpgradeInput.AdditionalValues = map[string]string{}
	}
	switch rtUpgradeInput.EnvironmentType {
	case e2e.ManagementClusterEnvironmentEKS:
		rtUpgradeInput.AdditionalValues["rancherTurtles.imagePullSecrets"] = "{regcred}"
	}
}
