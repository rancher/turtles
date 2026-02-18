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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"golang.org/x/mod/semver"

	"github.com/rancher/turtles/test/e2e"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opframework "sigs.k8s.io/cluster-api-operator/test/framework"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

// DeployRancherInput represents the input parameters for deploying Rancher.
type DeployRancherInput struct {
	// BootstrapClusterProxy is the cluster proxy for bootstrapping.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string `env:"HELM_BINARY_PATH"`

	// HelmExtraValuesPath is the path to the Helm extra values file.
	HelmExtraValuesPath string `env:",expand" envDefault:"${HELM_EXTRA_VALUES_FOLDER}/deploy-rancher.yaml"`

	// RancherChartRepoName is the repository name for Rancher chart.
	RancherChartRepoName string `env:"RANCHER_REPO_NAME"`

	// RancherChartURL is the URL for Rancher chart.
	RancherChartURL string `env:"RANCHER_URL"`

	// RancherChartPath is the path to the Rancher chart.
	RancherChartPath string `env:"RANCHER_PATH"`

	// RancherVersion is the version of Rancher.
	RancherVersion string `env:"RANCHER_VERSION"`

	// RancherImageTag is the image tag for Rancher.
	RancherImageTag string

	// RancherNamespace is the namespace for Rancher.
	RancherNamespace string `env:"RANCHER_NAMESPACE" envDefault:"cattle-system"`

	// RancherHost is the host for Rancher.
	RancherHost string `env:"RANCHER_HOSTNAME"`

	// RancherPassword is the password for Rancher.
	RancherPassword string `env:"RANCHER_PASSWORD"`

	// RancherPatches are the patches for Rancher.
	RancherPatches [][]byte

	// AdditionalValues are additional helm values to pass to the Rancher chart.
	AdditionalValues map[string]string

	// RancherWaitInterval is the wait interval for Rancher.
	RancherWaitInterval []interface{} `envDefault:"15m,30s"`

	// ControllerWaitInterval is the wait interval for the controller.
	ControllerWaitInterval []interface{} `envDefault:"15m,10s"`

	// RancherIngressClassName is the class name of the Ingress used by Rancher.
	RancherIngressClassName string

	// Development is the flag indicating whether it is a development environment.
	Development bool

	// RancherInstallationTimeout is the timeout for Rancher installation.
	RancherInstallationTimeout string `env:"RANCHER_INSTALLATION_TIMEOUT" envDefault:"5m"`

	// RancherDebug enables the `debug` chart value
	RancherDebug bool `env:"RANCHER_DEBUG"`
}

type deployRancherValuesFile struct {
	BootstrapPassword string `json:"bootstrapPassword"`
	Hostname          string `json:"hostname"`
}

type ngrokCredentials struct {
	NgrokAPIKey    string `json:"apiKey" yaml:"apiKey"`
	NgrokAuthToken string `json:"authtoken" yaml:"authtoken"`
}
type deployRancherIngressValuesFile struct {
	Credentials ngrokCredentials `json:"credentials" yaml:"credentials"`
}

// DeployRancher deploys Rancher using the provided input parameters.
// It expects the required input parameters to be non-nil.
// If InstallCertManager is true, the function will install cert-manager.
// The function adds the cert-manager chart repository and the Rancher chart repository.
// It then updates the Rancher chart repository.
// The function generates the extra values file for Rancher and writes it to the Helm extra values path.//
// If RancherIngressConfig is provided, the function sets up the ingress for Rancher.
// If RancherServicePatch is provided, the function updates the Rancher service.
// The function waits for the Rancher webhook rollout and the fleet controller rollout.
//
// Note: Use UpgradeInstallRancherWithGitea instead to bootstrap Rancher using a custom Turtles system chart.
func DeployRancher(ctx context.Context, input DeployRancherInput) PreRancherInstallHookResult {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for DeployRancher")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for DeployRancher")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for DeployRancher")
	Expect(input.HelmExtraValuesPath).ToNot(BeEmpty(), "HelmExtraValuesPath is required for DeployRancher")
	Expect(input.RancherChartRepoName).ToNot(BeEmpty(), "RancherChartRepoName is required for DeployRancher")
	Expect(input.RancherChartURL).ToNot(BeEmpty(), "RancherChartURL is required for DeployRancher")
	Expect(input.RancherChartPath).ToNot(BeEmpty(), "RancherChartPath is required for DeployRancher")
	Expect(input.RancherNamespace).ToNot(BeEmpty(), "RancherNamespace is required for DeployRancher")
	Expect(input.RancherHost).ToNot(BeEmpty(), "RancherHost is required for DeployRancher")
	Expect(input.RancherPassword).ToNot(BeEmpty(), "RancherPassword is required for DeployRancher")
	Expect(input.RancherWaitInterval).ToNot(BeNil(), "RancherWaitInterval is required for DeployRancher")
	Expect(input.ControllerWaitInterval).ToNot(BeNil(), "ControllerWaitInterval is required for DeployRancher")

	By("Running rancher pre-install hook")
	rancherHookResult := PreRancherInstallHook(PreRancherInstallHookInput{
		Ctx:                     ctx,
		BootstrapClusterProxy:   input.BootstrapClusterProxy,
		RancherIngressClassName: input.RancherIngressClassName,
		RancherHostname:         input.RancherHost,
	})

	if input.RancherVersion == "" && input.RancherImageTag == "" {
		Fail("RancherVersion or RancherImageTag is required")
	}
	if input.RancherVersion != "" && input.RancherImageTag != "" {
		Fail("Only one of RancherVersion or RancherImageTag cen be used")
	}

	By("Adding Rancher chart repo")
	addChart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Name:            input.RancherChartRepoName,
		Path:            input.RancherChartURL,
		Commands:        opframework.Commands(opframework.Repo, opframework.Add),
		AdditionalFlags: opframework.Flags("--force-update"),
		Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
	}
	_, err := addChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	updateChart := &opframework.HelmChart{
		BinaryPath: input.HelmBinaryPath,
		Commands:   opframework.Commands(opframework.Repo, opframework.Update),
		Kubeconfig: input.BootstrapClusterProxy.GetKubeconfigPath(),
	}
	_, err = updateChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	yamlExtraValues, err := yaml.Marshal(deployRancherValuesFile{
		BootstrapPassword: input.RancherPassword,
		Hostname:          rancherHookResult.Hostname,
	})
	Expect(err).ToNot(HaveOccurred())
	err = os.WriteFile(input.HelmExtraValuesPath, yamlExtraValues, 0644)
	Expect(err).ToNot(HaveOccurred())

	By("Installing Rancher")
	installFlags := opframework.Flags(
		"--namespace", input.RancherNamespace,
		"--create-namespace",
		"--values", input.HelmExtraValuesPath,
	)
	if input.RancherDebug {
		installFlags = append(installFlags, "--set", "debug=true")
	}
	if input.RancherVersion != "" {
		installFlags = append(installFlags, "--version", input.RancherVersion)
	}
	if input.Development {
		installFlags = append(installFlags, "--devel")
	}
	if input.RancherInstallationTimeout != "" {
		installFlags = append(installFlags, "--timeout", input.RancherInstallationTimeout)
	}

	chart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Path:            input.RancherChartPath,
		Name:            "rancher",
		Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
		AdditionalFlags: installFlags,
		Wait:            true,
	}
	values := map[string]string{
		"global.cattle.psp.enabled": "false",
		"replicas":                  "1",
	}

	if input.RancherImageTag != "" {
		values["rancherImageTag"] = input.RancherImageTag
	}

	if semver.Compare(input.RancherVersion, "v2.14.0") >= 0 {
		values["networkExposure.type"] = "ingress"
	}

	if rancherHookResult.IngressClassName != "" {
		values["ingress.ingressClassName"] = rancherHookResult.IngressClassName
	}
	// Merge additional values
	for k, v := range input.AdditionalValues {
		values[k] = v
	}

	_, err = chart.Run(values)
	Expect(err).ToNot(HaveOccurred())

	if len(rancherHookResult.ConfigPatches) > 0 {
		input.RancherPatches = append(input.RancherPatches, rancherHookResult.ConfigPatches...)
	}

	By("Updating rancher configuration")
	for _, patch := range input.RancherPatches {
		Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
			Proxy:    input.BootstrapClusterProxy,
			Template: patch,
			AddtionalEnvironmentVariables: map[string]string{
				e2e.RancherHostnameVar: rancherHookResult.Hostname,
			},
		})).To(Succeed())
	}

	// hack: fleet controller needs to be restarted first to pickup config change with a valid API url.
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "fleet-controller", Namespace: "cattle-fleet-system"}},
	}, input.ControllerWaitInterval...)

	By("Bouncing the fleet")
	Eventually(func() error {
		return input.BootstrapClusterProxy.GetClient().DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace("cattle-fleet-system"), client.MatchingLabels{"app": "fleet-controller"})
	}, input.ControllerWaitInterval...).ShouldNot(HaveOccurred())

	return rancherHookResult
}

// RestartRancherInput represents the input parameters for restarting Rancher.
type RestartRancherInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// RancherNamespace is the namespace where Rancher is deployed.
	RancherNamespace string `envDefault:"cattle-system"`

	// RancherWaitInterval is the wait interval for Rancher restart.
	RancherWaitInterval []interface{} `envDefault:"15m,10s"`
}

// RestartRancher restarts the Rancher application by killing its pods.
// It expects the required input parameters to be non-nil.
func RestartRancher(ctx context.Context, input RestartRancherInput) {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for RestartRancher")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for RestartRancher")
	Expect(input.RancherNamespace).ToNot(BeEmpty(), "RancherNamespace is required for RestartRancher")
	Expect(input.RancherWaitInterval).ToNot(BeNil(), "RancherWaitInterval is required for RestartRancher")

	By("Restarting Rancher by killing its pods")

	Eventually(func() error {
		return input.BootstrapClusterProxy.GetClient().DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(input.RancherNamespace), client.MatchingLabels{"app": "rancher"})
	}, input.RancherWaitInterval...).ShouldNot(HaveOccurred())
}

// RancherDeployIngressInput represents the input parameters for deploying an ingress in Rancher.
type RancherDeployIngressInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string `env:"HELM_BINARY_PATH"`

	// HelmExtraValuesPath is the path to the Helm extra values file.
	HelmExtraValuesPath string `env:",expand" envDefault:"${HELM_EXTRA_VALUES_FOLDER}/deploy-rancher-ingress.yaml"`

	// CustomIngress is the custom ingress to be deployed.
	CustomIngress []byte

	// CustomIngressLoadBalancer is the custom ingress to be deployed that has a type of LoadBalancer.
	CustomIngressLoadBalancer []byte

	// CustomIngressNamespace is the namespace for the custom ingress.
	CustomIngressNamespace string `envDefault:"traefik"`

	// CustomIngressDeployment is the deployment name for the custom ingress.
	CustomIngressDeployment string `envDefault:"traefik"`

	// IngressWaitInterval is the wait interval for the ingress deployment.
	IngressWaitInterval []interface{} `envDefault:"15m,30s"`

	// DefaultIngressClassPatch is the default ingress class patch.
	DefaultIngressClassPatch []byte

	// EnvironmentType is the type of the invironment to select ingress to be deployed.
	EnvironmentType e2e.ManagementClusterEnvironmentType `env:"MANAGEMENT_CLUSTER_ENVIRONMENT"`

	// NgrokApiKey is the API key for Ngrok.
	NgrokApiKey string `env:"NGROK_API_KEY"`

	// NgrokAuthToken is the authentication token for Ngrok.
	NgrokAuthToken string `env:"NGROK_AUTHTOKEN"`

	// NgrokPath is the path to the Ngrok binary.
	NgrokPath string `env:"NGROK_PATH"`

	// NgrokRepoName is the name of the Ngrok repository.
	NgrokRepoName string `env:"NGROK_REPO_NAME"`

	// NgrokRepoURL is the URL of the Ngrok repository.
	NgrokRepoURL string `env:"NGROK_URL"`
}

// RancherDeployIngress deploys an ingress based on the provided input.
// It expects the required input parameters to be non-nil.
// - If the IngressType is CustomIngress:
//   - CustomIngress, CustomIngressNamespace, CustomIngressDeployment, and IngressWaitInterval must not be empty.
//   - deployIsolatedModeIngress is called with the provided context and input.
//
// - If the IngressType is NgrokIngress:
//   - NgrokApiKey, NgrokAuthToken, NgrokPath, NgrokRepoName, NgrokRepoURL, and HelmExtraValuesPath must not be empty.
//   - deployNgrokIngress is called with the provided context and input.
//
// - If the IngressType is EKSNginxIngress:
//   - IngressWaitInterval must not be nil.
//   - deployEKSIngress is called with the provided input.
func RancherDeployIngress(ctx context.Context, input RancherDeployIngressInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for RancherDeployIngress")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for RancherDeployIngress")

	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	komega.SetClient(input.BootstrapClusterProxy.GetClient())
	komega.SetContext(ctx)

	switch input.EnvironmentType {
	case e2e.ManagementClusterEnvironmentIsolatedKind:
		Expect(input.CustomIngress).ToNot(BeEmpty(), "CustomIngress is required when using custom ingress")
		deployIsolatedModeIngress(ctx, input)
	case e2e.ManagementClusterEnvironmentKind:
		Expect(input.NgrokApiKey).ToNot(BeEmpty(), "NgrokApiKey is required when using ngrok ingress")
		Expect(input.NgrokAuthToken).ToNot(BeEmpty(), "NgrokAuthToken is required when using ngrok ingress")
		Expect(input.NgrokPath).ToNot(BeEmpty(), "NgrokPath is required  when using ngrok ingress")
		Expect(input.NgrokRepoName).ToNot(BeEmpty(), "NgrokRepoName is required when using ngrok ingress")
		Expect(input.NgrokRepoURL).ToNot(BeEmpty(), "NgrokRepoURL is required when using ngrok ingress")
		Expect(input.HelmExtraValuesPath).ToNot(BeNil(), "HelmExtraValuesPath is when using ngrok ingress")
		deployNgrokIngress(ctx, input)
	case e2e.ManagementClusterEnvironmentEKS:
		deployEKSIngress(input)
	case e2e.ManagementClusterEnvironmentInternalKind:
		Expect(input.CustomIngressLoadBalancer).ToNot(BeEmpty(), "CustomIngressLoadBalancer is required when using custom ingress with a load balancer")
		deployTraefikIngressLoadBalancer(ctx, input)
	}
}

func deployIsolatedModeIngress(ctx context.Context, input RancherDeployIngressInput) {
	By("Deploying custom ingress")
	Expect(turtlesframework.Apply(ctx, input.BootstrapClusterProxy, []byte(input.CustomIngress))).To(Succeed())

	By("Getting custom ingress deployment")
	ingressDeployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: input.CustomIngressDeployment, Namespace: input.CustomIngressNamespace}}
	Eventually(
		komega.Get(ingressDeployment),
		input.IngressWaitInterval...,
	).Should(Succeed(), "Failed to get custom ingress controller")

	turtlesframework.Byf("Waiting for %s deployment to be available", input.CustomIngressDeployment)
	Eventually(komega.Object(ingressDeployment), input.IngressWaitInterval...).Should(HaveField("Status.AvailableReplicas", Equal(int32(1))))
}

func deployEKSIngress(input RancherDeployIngressInput) {
	By("Adding Traefik chart repo")
	repoChart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Name:            "traefik",
		Path:            "https://traefik.github.io/charts",
		Commands:        opframework.Commands(opframework.Repo, opframework.Add),
		AdditionalFlags: opframework.Flags("--force-update"),
		Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
	}
	_, err := repoChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	By("Installing Traefik ingress")
	traefikChart := &opframework.HelmChart{
		BinaryPath: input.HelmBinaryPath,
		Path:       "traefik/traefik",
		Name:       "traefik",
		Kubeconfig: input.BootstrapClusterProxy.GetKubeconfigPath(),
		AdditionalFlags: opframework.Flags(
			"--namespace", "traefik",
			"--version", "v39.0.1",
			"--create-namespace",
		),
		Wait: true,
	}
	_, err = traefikChart.Run(map[string]string{
		"service.type": "LoadBalancer",
	})
	Expect(err).ToNot(HaveOccurred())
}

func deployNgrokIngress(ctx context.Context, input RancherDeployIngressInput) {
	By("Setting up ngrok-ingress-controller")
	addChart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Name:            input.NgrokRepoName,
		Path:            input.NgrokRepoURL,
		Commands:        opframework.Commands(opframework.Repo, opframework.Add),
		AdditionalFlags: opframework.Flags("--force-update"),
		Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
	}
	_, err := addChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	updateChart := &opframework.HelmChart{
		BinaryPath: input.HelmBinaryPath,
		Commands:   opframework.Commands(opframework.Repo, opframework.Update),
		Kubeconfig: input.BootstrapClusterProxy.GetKubeconfigPath(),
	}
	_, err = updateChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	yamlExtraValues, err := yaml.Marshal(deployRancherIngressValuesFile{
		Credentials: ngrokCredentials{
			NgrokAPIKey:    input.NgrokApiKey,
			NgrokAuthToken: input.NgrokAuthToken,
		},
	})
	Expect(err).ToNot(HaveOccurred())
	err = os.WriteFile(input.HelmExtraValuesPath, yamlExtraValues, 0644)
	Expect(err).ToNot(HaveOccurred())

	installFlags := opframework.Flags(
		"--timeout", "5m",
		"--values", input.HelmExtraValuesPath,
	)

	installChart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Name:            input.NgrokRepoName,
		Path:            input.NgrokPath,
		Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
		Wait:            true,
		AdditionalFlags: installFlags,
	}
	_, err = installChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	By("Setting up default ingress class")
	Expect(turtlesframework.Apply(ctx, input.BootstrapClusterProxy, input.DefaultIngressClassPatch)).To(Succeed())
}

func deployTraefikIngressLoadBalancer(ctx context.Context, input RancherDeployIngressInput) {
	By("Deploying custom ingress controller of type LoadBalancer")
	Expect(turtlesframework.Apply(ctx, input.BootstrapClusterProxy, []byte(input.CustomIngressLoadBalancer))).To(Succeed())

	By("Getting custom ingress deployment")
	ingressDeployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: input.CustomIngressDeployment, Namespace: input.CustomIngressNamespace}}
	Eventually(
		komega.Get(ingressDeployment),
		input.IngressWaitInterval...,
	).Should(Succeed(), "Failed to get custom ingress controller")

	turtlesframework.Byf("Waiting for %s deployment to be available", input.CustomIngressDeployment)
	Eventually(komega.Object(ingressDeployment), input.IngressWaitInterval...).Should(HaveField("Status.AvailableReplicas", Equal(int32(1))))
}

// PreRancherInstallHookInput represents the input parameters for the pre-Rancher install hook.
type PreRancherInstallHookInput struct {
	// Ctx is the context for the hook execution.
	Ctx context.Context

	// BootstrapClusterProxy is the cluster proxy for bootstrapping.
	BootstrapClusterProxy framework.ClusterProxy

	// EnvironmentType is the environment type
	EnvironmentType e2e.ManagementClusterEnvironmentType `env:"MANAGEMENT_CLUSTER_ENVIRONMENT"`

	// RancherIngressClassName is the class name of the Ingress used by Rancher.
	RancherIngressClassName string

	// IngressWaitInterval is the interval to wait between ingress checks.
	IngressWaitInterval []interface{} `envDefault:"15m,10s"`

	// RancherHostname is a maunally specified RancherHostname value
	RancherHostname string `env:"RANCHER_HOSTNAME"`
}

// PreRancherInstallHookResult represents the result of a pre-Rancher install hook.
type PreRancherInstallHookResult struct {
	// Hostname is the hostname of the Rancher installation.
	Hostname string
	// IngressClassName is the class name of the Ingress used by Rancher.
	IngressClassName string
	// ConfigPatches is an optional list of additional patches that need to be applied to configure Rancher.
	ConfigPatches [][]byte
}

// PreRancherInstallHook is a function that performs pre-installation tasks for Rancher.
// The function retrieves the infrastructure type from the input and performs different actions based on the type.
// If the infrastructure type is e2e.ManagementClusterEnvironmentEKS, it retrieves the ingress hostname and sets it as the Rancher host.
// It also deploys ghcr details by creating a Docker registry secret.
// If the infrastructure type is e2e.ManagementClusterEnvironmentIsolatedKind, it sets the isolated host name as the Rancher host.
// If the infrastructure type is e2e.ManagementClusterEnvironmentKind, it sets the Rancher ingress config and service patch based on the provided values.
func PreRancherInstallHook(input PreRancherInstallHookInput) PreRancherInstallHookResult {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	switch input.EnvironmentType {
	case e2e.ManagementClusterEnvironmentEKS:
		By("Getting ingress hostname")
		svcRes := &WaitForServiceIngressHostnameResult{}
		WaitForServiceIngressHostname(input.Ctx, WaitForServiceIngressHostnameInput{
			BootstrapClusterProxy: input.BootstrapClusterProxy,
			ServiceName:           "traefik",
			ServiceNamespace:      "traefik",
			IngressWaitInterval:   input.IngressWaitInterval,
		}, svcRes)

		By("Deploying ghcr details")
		turtlesframework.CreateDockerRegistrySecret(input.Ctx, turtlesframework.CreateDockerRegistrySecretInput{
			BootstrapClusterProxy: input.BootstrapClusterProxy,
		})

		return PreRancherInstallHookResult{
			Hostname:         svcRes.Hostname,
			IngressClassName: "traefik",
		}
	case e2e.ManagementClusterEnvironmentIsolatedKind:
		By("Getting internal cluster hostname")
		hostname := getInternalClusterHostname(input.Ctx, input.BootstrapClusterProxy)
		return PreRancherInstallHookResult{
			Hostname:         hostname,
			IngressClassName: input.RancherIngressClassName,
		}
	case e2e.ManagementClusterEnvironmentKind:
		By("Using RANCHER_HOSTNAME")
		// i.e. we are using ngrok locally
		return PreRancherInstallHookResult{
			Hostname:         input.RancherHostname,
			IngressClassName: input.RancherIngressClassName,
			ConfigPatches:    [][]byte{e2e.RancherServicePatch, e2e.IngressConfig, e2e.SystemStoreSettingPatch},
		}

	case e2e.ManagementClusterEnvironmentInternalKind:
		By("Using RANCHER_HOSTNAME for internal kind")
		return PreRancherInstallHookResult{
			Hostname: input.RancherHostname,
		}

	default:
		Fail(fmt.Sprintf("Unknown MANAGEMENT_CLUSTER_ENVIRONMENT: %s", input.EnvironmentType))
		return PreRancherInstallHookResult{}
	}
}
