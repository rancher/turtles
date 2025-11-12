/*
Copyright © 2025 SUSE LLC

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
	"maps"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/turtles/test/e2e"
	turtlesframework "github.com/rancher/turtles/test/framework"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	opframework "sigs.k8s.io/cluster-api-operator/test/framework"
	"sigs.k8s.io/cluster-api/test/framework"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	providerEnabledKey   = "enabled"
	providerVerbosityKey = "manager.verbosity"
	debugVerbosityValue  = "5"

	bootstrapRKE2Path       = "providers.bootstrapRKE2."
	controlplaneRKE2Path    = "providers.controlplaneRKE2."
	bootstrapKubeadmPath    = "providers.bootstrapKubeadm."
	controlplaneKubeadmPath = "providers.controlplaneKubeadm."
	dockerPath              = "providers.infrastructureDocker."
	awsPath                 = "providers.infrastructureAWS."
	azurePath               = "providers.infrastructureAzure."
	gcpPath                 = "providers.infrastructureGCP."
	vspherePath             = "providers.infrastructureVSphere."

	defaultOCIRegistry = "registry.rancher.com/rancher/cluster-api-controller-components"

	providerKubeadmBootstrap    = "kubeadm-bootstrap"
	providerKubeadmControlPlane = "kubeadm-control-plane"
	providerDocker              = "docker"
	providerAWS                 = "aws"
	providerAzure               = "azure"
	providerGCP                 = "gcp"
	providerVSphere             = "vsphere"

	deployCAPIControllerManager = "capi-controller-manager"
	namespaceCAPISystem         = "cattle-capi-system"

	deployKubeadmBootstrapControllerManager = "capi-kubeadm-bootstrap-controller-manager"
	namespaceKubeadmBootstrapSystem         = "capi-kubeadm-bootstrap-system"

	deployKubeadmControlPlaneControllerManager = "capi-kubeadm-control-plane-controller-manager"
	namespaceKubeadmControlPlaneSystem         = "capi-kubeadm-control-plane-system"

	deployCAPDControllerManager = "capd-controller-manager"
	namespaceCAPDSystem         = "capd-system"

	deployCAPAControllerManager = "capa-controller-manager"
	namespaceCAPASystem         = "capa-system"

	deployCAPZControllerManager = "capz-controller-manager"
	namespaceCAPZSystem         = "capz-system"

	deployCAPGControllerManager = "capg-controller-manager"
	namespaceCAPGSystem         = "capg-system"

	deployCAPVControllerManager = "capv-controller-manager"
	namespaceCAPVSystem         = "capv-system"
)

// DeployRancherTurtlesProvidersInput represents the input parameters for installing the
// rancher-turtles-providers chart.
type DeployRancherTurtlesProvidersInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string `env:"HELM_BINARY_PATH"`

	// ProvidersChartUrl is the URL of the rancher-turtles-providers chart repository.
	ProvidersChartUrl string `env:"TURTLES_PROVIDERS_URL"`

	// ProvidersChartPath is the path to the rancher-turtles-providers chart.
	ProvidersChartPath string `env:"TURTLES_PROVIDERS_PATH"`

	// ProvidersChartRepoName is the name of the Helm repo.
	ProvidersChartRepoName string `env:"TURTLES_PROVIDERS_REPO_NAME" envDefault:"turtles"`

	// Version is the chart version to install.
	Version string

	// AdditionalValues are additional Helm values to pass to the chart (for example to
	// enable specific infrastructure providers).
	AdditionalValues map[string]string

	// WaitDeploymentsReadyInterval is the interval used when waiting for provider
	// deployments to become available (e.g. "15m,10s").
	WaitDeploymentsReadyInterval []interface{} `envDefault:"15m,10s"`

	// ProviderList is an optional comma-separated list of providers to enable on first install.
	// Examples: "all", "azure,aws".
	ProviderList string `env:"TURTLES_PROVIDERS" envDefault:"all"`

	// MigrationScriptPath is the path to the providers ownership migration script.
	MigrationScriptPath string `env:"TURTLES_MIGRATION_SCRIPT_PATH"`
}

// DeployRancherTurtlesProviders installs the rancher-turtles-providers chart with provided values and waits for deployments.
func DeployRancherTurtlesProviders(ctx context.Context, input DeployRancherTurtlesProvidersInput) {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for DeployRancherTurtlesProviders")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for DeployRancherTurtlesProviders")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for DeployRancherTurtlesProviders")

	chartPath := input.ProvidersChartPath

	if chartPath != "" {
		By(fmt.Sprintf("Using local chart path: %s", chartPath))
	} else if input.Version != "" && input.ProvidersChartUrl == "" {
		chartPath = "rancher-turtles-providers-external/rancher-turtles-providers"

		By("Adding external rancher-turtles-providers chart repo")
		addChart := &opframework.HelmChart{
			BinaryPath:      input.HelmBinaryPath,
			Name:            "rancher-turtles-providers-external",
			Path:            input.ProvidersChartPath,
			Commands:        opframework.Commands(opframework.Repo, opframework.Add),
			AdditionalFlags: opframework.Flags("--force-update"),
			Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
		}
		_, err := addChart.Run(nil)
		Expect(err).ToNot(HaveOccurred())
	} else if input.ProvidersChartUrl != "" {
		By("Adding rancher-turtles-providers chart repo")
		addChart := &opframework.HelmChart{
			BinaryPath:      input.HelmBinaryPath,
			Name:            input.ProvidersChartRepoName,
			Path:            input.ProvidersChartUrl,
			Commands:        opframework.Commands(opframework.Repo, opframework.Add),
			AdditionalFlags: opframework.Flags("--force-update"),
			Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
		}
		_, err := addChart.Run(nil)
		Expect(err).ToNot(HaveOccurred())
		chartPath = fmt.Sprintf("%s/%s", input.ProvidersChartRepoName, e2e.ProvidersChartName)
	}

	values := map[string]string{}

	selectedList := strings.TrimSpace(strings.ToLower(input.ProviderList))
	if selectedList == "all" {
		enableAllProviders(values)
	} else if selectedList != "" {
		providerList := strings.Split(selectedList, ",")
		for _, p := range providerList {
			provider := strings.TrimSpace(strings.ToLower(p))
			switch provider {
			case "rke2":
				values[bootstrapRKE2Path+providerVerbosityKey] = debugVerbosityValue
				values[controlplaneRKE2Path+providerVerbosityKey] = debugVerbosityValue
			case "kubeadm":
				values[bootstrapKubeadmPath+providerEnabledKey] = "true"
				values[bootstrapKubeadmPath+providerVerbosityKey] = debugVerbosityValue
				values[controlplaneKubeadmPath+providerEnabledKey] = "true"
				values[controlplaneKubeadmPath+providerVerbosityKey] = debugVerbosityValue
			case "docker", "capd":
				values[dockerPath+providerEnabledKey] = "true"
				values[dockerPath+providerVerbosityKey] = debugVerbosityValue
			case "aws", "capa":
				values[awsPath+providerEnabledKey] = "true"
				values[awsPath+providerVerbosityKey] = debugVerbosityValue
			case "azure", "capz":
				values[azurePath+providerEnabledKey] = "true"
				values[azurePath+providerVerbosityKey] = debugVerbosityValue
			case "gcp", "capg":
				values[gcpPath+providerEnabledKey] = "true"
				values[gcpPath+providerVerbosityKey] = debugVerbosityValue
			case "vsphere", "capv":
				values[vspherePath+providerEnabledKey] = "true"
				values[vspherePath+providerVerbosityKey] = debugVerbosityValue
			case "", "all":
			default:
				log.FromContext(ctx).Info("Unknown provider in TURTLES_PROVIDERS, ignoring", "provider", provider)
			}
		}
	}

	By("Waiting for rancher webhook rollout")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "rancher-webhook", Namespace: e2e.RancherNamespace}},
	}, e2e.LoadE2EConfig().GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...)

	By("Installing rancher-turtles-providers chart")

	maps.Copy(values, input.AdditionalValues) // Merge additional values into the values map

	enabledProviders := getEnabledCAPIProviders(values)

	applyProviderSecrets(ctx, input, enabledProviders)

	providerWaiters := []func(ctx context.Context){}
	configureProviderDefaults(ctx, input, values, enabledProviders)
	configureProviderWaiters(input, enabledProviders, &providerWaiters)
	adoptArgs := getAdoptArgsForEnabledProviders(enabledProviders, values)
	log.FromContext(ctx).Info("Providers adoption args prepared", "args", adoptArgs, "enabled", enabledProviders)
	runProviderMigration(ctx, input.MigrationScriptPath, input.BootstrapClusterProxy.GetKubeconfigPath(), adoptArgs...)

	command := []string{
		"upgrade", e2e.ProvidersChartName, chartPath,
		"--install",
		"--dependency-update",
		"-n", e2e.RancherTurtlesNamespace,
		"--create-namespace",
		"--wait",
		"--reuse-values",
		"--timeout", "10m",
		"--kubeconfig", input.BootstrapClusterProxy.GetKubeconfigPath(),
	}

	if input.Version != "" {
		command = append(command, "--version", input.Version)
	}

	for k, v := range values {
		if strings.Contains(k, ".variables.") { // Provider variables must be strings
			command = append(command, "--set-string", fmt.Sprintf("%s=%s", k, v))
			continue
		}
		command = append(command, "--set", fmt.Sprintf("%s=%s", k, v))
	}

	fullCommand := append([]string{input.HelmBinaryPath}, command...)
	log.FromContext(ctx).Info("Executing:", "install", strings.Join(fullCommand, " "))

	By("Installing providers chart...")
	cmd := exec.Command(input.HelmBinaryPath, command...)
	cmd.WaitDelay = 10 * time.Minute
	out, err := cmd.CombinedOutput()
	if err != nil {
		Expect(fmt.Errorf("Unable to install providers chart: %w\nOutput: %s, Command: %s", err, out, strings.Join(fullCommand, " "))).ToNot(HaveOccurred())
	}

	deploymentsToWait := getDeploymentsForEnabledProviders(enabledProviders)
	if len(deploymentsToWait) > 0 {
		By("Waiting for provider deployments to be ready")
		Expect(input.WaitDeploymentsReadyInterval).ToNot(BeNil(), "WaitDeploymentsReadyInterval is required when waiting for deployments")
		for _, nn := range deploymentsToWait {
			turtlesframework.Byf("Waiting for deployment %s/%s to be available", nn.Namespace, nn.Name)
			framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
				Getter: input.BootstrapClusterProxy.GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      nn.Name,
					Namespace: nn.Namespace,
				}},
			}, input.WaitDeploymentsReadyInterval...)
		}
	}

	for _, waiter := range providerWaiters {
		By("Executing provider-specific custom waiter function")
		waiter(ctx)
	}
}

func runProviderMigration(ctx context.Context, scriptPath, kubeconfigPath string, extraArgs ...string) {
	if _, err := os.Stat(scriptPath); err != nil {
		Expect(fmt.Errorf("migration script not found: %s", scriptPath)).ToNot(HaveOccurred())
	}

	By("Running providers ownership migration script")
	args := []string{"--kubeconfig", kubeconfigPath}
	if len(extraArgs) > 0 {
		args = append(args, extraArgs...)
	}
	cmd := exec.Command(scriptPath, args...)
	cmd.Env = append(os.Environ(),
		"RELEASE_NAME="+e2e.ProvidersChartName,
		"RELEASE_NAMESPACE="+e2e.RancherTurtlesNamespace,
		"TURTLES_CHART_NAMESPACE="+e2e.RancherTurtlesNamespace,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		Expect(fmt.Errorf("migration script failed: %w\nOutput: %s", err, out)).ToNot(HaveOccurred())
	}

	log.FromContext(ctx).Info("migration completed", "output", string(out))
}

func enableAllProviders(values map[string]string) {
	values[bootstrapRKE2Path+providerVerbosityKey] = debugVerbosityValue
	values[controlplaneRKE2Path+providerVerbosityKey] = debugVerbosityValue
	values[bootstrapKubeadmPath+providerEnabledKey] = "true"
	values[bootstrapKubeadmPath+providerVerbosityKey] = debugVerbosityValue
	values[controlplaneKubeadmPath+providerEnabledKey] = "true"
	values[controlplaneKubeadmPath+providerVerbosityKey] = debugVerbosityValue
	values[dockerPath+providerEnabledKey] = "true"
	values[dockerPath+providerVerbosityKey] = debugVerbosityValue
	values[awsPath+providerEnabledKey] = "true"
	values[awsPath+providerVerbosityKey] = debugVerbosityValue
	values[azurePath+providerEnabledKey] = "true"
	values[azurePath+providerVerbosityKey] = debugVerbosityValue
	values[gcpPath+providerEnabledKey] = "true"
	values[gcpPath+providerVerbosityKey] = debugVerbosityValue
	values[vspherePath+providerEnabledKey] = "true"
	values[vspherePath+providerVerbosityKey] = debugVerbosityValue
}

func getAdoptArgsForEnabledProviders(enabled []string, values map[string]string) []string {
	adopt := []string{}

	getNamespaceWithDefault := func(key, defaultNamespace string) string { // Try to get namespace from values or use default
		if namespace, ok := values[key]; ok && strings.TrimSpace(namespace) != "" {
			return strings.TrimSpace(namespace)
		}
		return defaultNamespace
	}

	namespaces := map[string]string{
		providerKubeadmBootstrap:    getNamespaceWithDefault("providers.bootstrapKubeadm.namespace", namespaceKubeadmBootstrapSystem),
		providerKubeadmControlPlane: getNamespaceWithDefault("providers.controlplaneKubeadm.namespace", namespaceKubeadmControlPlaneSystem),
		providerDocker:              getNamespaceWithDefault("providers.infrastructureDocker.namespace", namespaceCAPDSystem),
		providerAWS:                 getNamespaceWithDefault("providers.infrastructureAWS.namespace", namespaceCAPASystem),
		providerAzure:               getNamespaceWithDefault("providers.infrastructureAzure.namespace", namespaceCAPZSystem),
		providerGCP:                 getNamespaceWithDefault("providers.infrastructureGCP.namespace", namespaceCAPGSystem),
		providerVSphere:             getNamespaceWithDefault("providers.infrastructureVSphere.namespace", namespaceCAPVSystem),
	}

	for _, name := range enabled {
		if namespace, ok := namespaces[name]; ok && namespace != "" {
			adopt = append(adopt, "--adopt", fmt.Sprintf("%s:%s", name, namespace))
		}
	}

	return adopt
}

func getEnabledCAPIProviders(values map[string]string) []string {
	out := []string{}
	if values[bootstrapKubeadmPath+providerEnabledKey] == "true" {
		out = append(out, providerKubeadmBootstrap)
	}
	if values[controlplaneKubeadmPath+providerEnabledKey] == "true" {
		out = append(out, providerKubeadmControlPlane)
	}
	if values[dockerPath+providerEnabledKey] == "true" {
		out = append(out, providerDocker)
	}
	if values[awsPath+providerEnabledKey] == "true" {
		out = append(out, providerAWS)
	}
	if values[azurePath+providerEnabledKey] == "true" {
		out = append(out, providerAzure)
	}
	if values[gcpPath+providerEnabledKey] == "true" {
		out = append(out, providerGCP)
	}
	if values[vspherePath+providerEnabledKey] == "true" {
		out = append(out, providerVSphere)
	}
	return out
}

func getDeploymentsForEnabledProviders(enabled []string) []NamespaceName {
	deployments := []NamespaceName{
		{Name: deployCAPIControllerManager, Namespace: namespaceCAPISystem},
	}

	for _, name := range enabled {
		switch name {
		case providerKubeadmBootstrap:
			deployments = append(deployments, NamespaceName{Name: deployKubeadmBootstrapControllerManager, Namespace: namespaceKubeadmBootstrapSystem})
		case providerKubeadmControlPlane:
			deployments = append(deployments, NamespaceName{Name: deployKubeadmControlPlaneControllerManager, Namespace: namespaceKubeadmControlPlaneSystem})
		case providerDocker:
			deployments = append(deployments, NamespaceName{Name: deployCAPDControllerManager, Namespace: namespaceCAPDSystem})
		case providerAWS:
			deployments = append(deployments, NamespaceName{Name: deployCAPAControllerManager, Namespace: namespaceCAPASystem})
		case providerAzure:
			deployments = append(deployments, NamespaceName{Name: deployCAPZControllerManager, Namespace: namespaceCAPZSystem})
		case providerGCP:
			deployments = append(deployments, NamespaceName{Name: deployCAPGControllerManager, Namespace: namespaceCAPGSystem})
		case providerVSphere:
			deployments = append(deployments, NamespaceName{Name: deployCAPVControllerManager, Namespace: namespaceCAPVSystem})
		}
	}

	return deployments
}

func configureProviderDefaults(ctx context.Context, input DeployRancherTurtlesProvidersInput, values map[string]string, enabled []string) {
	for _, name := range enabled {
		switch name {
		case providerAWS:
			values["providers.infrastructureAWS.variables.EXP_MACHINE_POOL"] = "true"
			values["providers.infrastructureAWS.variables.EXP_EXTERNAL_RESOURCE_GC"] = "true"
			values["providers.infrastructureAWS.variables.CAPA_LOGLEVEL"] = "5"
			values["providers.infrastructureAWS.manager.syncPeriod"] = "5m"
		case providerDocker:
			By("Configuring Docker provider with OCI registry")
			clusterctl := turtlesframework.GetClusterctl(ctx, turtlesframework.GetClusterctlInput{
				GetLister:          input.BootstrapClusterProxy.GetClient(),
				ConfigMapNamespace: e2e.RancherTurtlesNamespace,
				ConfigMapName:      "clusterctl-config",
			})
			dockerVersion := getProviderVersion(clusterctl, "docker")
			Expect(dockerVersion).ToNot(BeEmpty(), "Docker provider version must be available in clusterctl config")

			values["providers.infrastructureDocker.fetchConfig.oci"] = fmt.Sprintf("%s:%s", defaultOCIRegistry, dockerVersion)
			By("Using Docker provider version " + dockerVersion + " from OCI registry")
		}
	}
}

func configureProviderWaiters(input DeployRancherTurtlesProvidersInput, enabled []string, customWaiters *[]func(ctx context.Context)) {
	for _, name := range enabled {
		switch name {
		case providerAzure:
			*customWaiters = append(*customWaiters, azureServiceOperatorWaiter(input.BootstrapClusterProxy))
		}
	}
}

func applyProviderSecrets(ctx context.Context, input DeployRancherTurtlesProvidersInput, enabled []string) {
	for _, name := range enabled {
		switch name {
		case providerAzure:
			By("Applying Azure provider secret")
			Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
				Proxy:    input.BootstrapClusterProxy,
				Template: e2e.AzureIdentitySecret,
			})).To(Succeed(), "Failed to apply Azure provider secret")
		case providerAWS:
			By("Applying AWS provider secret")
			Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
				Proxy:    input.BootstrapClusterProxy,
				Template: e2e.AWSIdentitySecret,
			})).To(Succeed(), "Failed to apply AWS provider secret")
		case providerGCP:
			By("Applying GCP provider secret")
			Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
				Proxy:    input.BootstrapClusterProxy,
				Template: e2e.GCPProviderSecret,
			})).To(Succeed(), "Failed to apply GCP provider secret")
		case providerVSphere:
			By("Applying vSphere provider secret")
			Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
				Proxy:    input.BootstrapClusterProxy,
				Template: e2e.VSphereProviderSecret,
			})).To(Succeed(), "Failed to apply vSphere provider secret")
		}
	}
}

// azureServiceOperatorWaiter returns a custom waiter function for Azure service operator
// Workaround for https://github.com/rancher/turtles/issues/1584 - should be removed when fixed
func azureServiceOperatorWaiter(bootstrapClusterProxy framework.ClusterProxy) func(ctx context.Context) {
	return func(ctx context.Context) {
		overallTimeout := 10 * time.Minute
		pollInterval := 5 * time.Second
		overallDeadline := time.Now().Add(overallTimeout)
		podLabels := map[string]string{
			"app.kubernetes.io/name": "azure-service-operator",
			"control-plane":          "controller-manager",
		}
		lastPod := &corev1.Pod{}

		for time.Now().Before(overallDeadline) {
			var podList corev1.PodList
			err := bootstrapClusterProxy.GetClient().List(ctx, &podList, &crclient.ListOptions{
				Namespace:     namespaceCAPZSystem,
				LabelSelector: labels.SelectorFromSet(podLabels),
			})
			Expect(err).ToNot(HaveOccurred(), "Failed to list azure-service-operator pods")

			if len(podList.Items) == 0 {
				By("Waiting for azure-service-operator pod to be created")
				time.Sleep(pollInterval)
				continue
			}

			pod := &podList.Items[0]
			lastPod = pod

			crashloop := false
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.State.Waiting != nil && cs.State.Waiting.Reason == "CrashLoopBackOff" {
					crashloop = true
					break
				}
			}
			if crashloop {
				By("Restarting azure-service-operator pod due to CrashLoopBackOff")
				err := bootstrapClusterProxy.GetClient().Delete(ctx, pod)
				Expect(err).ToNot(HaveOccurred(), "Failed to delete azure-service-operator pod for restart")
				time.Sleep(pollInterval)
				continue
			}

			ready := false
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Ready {
					ready = true
					break
				}
			}
			if ready && pod.Status.Phase == corev1.PodRunning {
				By("azure-service-operator pod is running and ready, continuing to monitor...")
			}

			time.Sleep(pollInterval)
		}

		Expect(lastPod).ToNot(BeNil(), "azure-service-operator pod should exist after 10 minutes of monitoring")

		By("Performing final azure-service-operator pod status check")
		Expect(lastPod.Status.Phase).To(Equal(corev1.PodRunning), "azure-service-operator pod should be in Running phase after 10 minutes")

		finalReady := false
		for _, cs := range lastPod.Status.ContainerStatuses {
			if cs.Ready {
				finalReady = true
				break
			}
		}
		Expect(lastPod.Status.Phase == corev1.PodRunning && finalReady).To(BeTrue(), "azure-service-operator pod should be both running and ready after 10 minutes")
		By("azure-service-operator pod monitoring completed successfully - pod is running and ready")
	}
}
