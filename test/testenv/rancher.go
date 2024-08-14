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
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesframework "github.com/rancher/turtles/test/framework"

	"github.com/drone/envsubst/v2"
	"github.com/rancher/turtles/test/e2e"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opframework "sigs.k8s.io/cluster-api-operator/test/framework"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/yaml"
)

// DeployRancherInput represents the input parameters for deploying Rancher.
type DeployRancherInput struct {
	// BootstrapClusterProxy is the cluster proxy for bootstrapping.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string

	// HelmExtraValuesPath is the path to the Helm extra values file.
	HelmExtraValuesPath string

	// InstallCertManager is the flag indicating whether to install Cert Manager.
	InstallCertManager bool

	// CertManagerChartPath is the path to the Cert Manager chart.
	CertManagerChartPath string

	// CertManagerUrl is the URL for Cert Manager.
	CertManagerUrl string

	// CertManagerRepoName is the repository name for Cert Manager.
	CertManagerRepoName string

	// RancherChartRepoName is the repository name for Rancher chart.
	RancherChartRepoName string

	// RancherChartURL is the URL for Rancher chart.
	RancherChartURL string

	// RancherChartPath is the path to the Rancher chart.
	RancherChartPath string

	// RancherVersion is the version of Rancher.
	RancherVersion string

	// RancherImageTag is the image tag for Rancher.
	RancherImageTag string

	// RancherNamespace is the namespace for Rancher.
	RancherNamespace string

	// RancherHost is the host for Rancher.
	RancherHost string

	// RancherPassword is the password for Rancher.
	RancherPassword string

	// RancherFeatures are the features for Rancher.
	RancherFeatures string

	// RancherPatches are the patches for Rancher.
	RancherPatches [][]byte

	// RancherWaitInterval is the wait interval for Rancher.
	RancherWaitInterval []interface{}

	// ControllerWaitInterval is the wait interval for the controller.
	ControllerWaitInterval []interface{}

	// RancherIngressConfig is the ingress configuration for Rancher.
	RancherIngressConfig []byte

	// RancherServicePatch is the service patch for Rancher.
	RancherServicePatch []byte

	// RancherIngressClassName is the ingress class name for Rancher.
	RancherIngressClassName string

	// Development is the flag indicating whether it is a development environment.
	Development bool

	// Variables is the collection of variables.
	Variables turtlesframework.VariableCollection
}

type deployRancherValuesFile struct {
	BootstrapPassword string `json:"bootstrapPassword"`
	Hostname          string `json:"hostname"`
}

type ngrokCredentials struct {
	NgrokAPIKey    string `json:"apiKey"`
	NgrokAuthToken string `json:"authtoken"`
}
type deployRancherIngressValuesFile struct {
	Credentials ngrokCredentials `json:"credentials"`
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
func DeployRancher(ctx context.Context, input DeployRancherInput) {
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

	if input.RancherVersion == "" && input.RancherImageTag == "" {
		Fail("RancherVersion or RancherImageTag is required")
	}
	if input.RancherVersion != "" && input.RancherImageTag != "" {
		Fail("Only one of RancherVersion or RancherImageTag cen be used")
	}

	if input.InstallCertManager {
		Expect(input.CertManagerRepoName).ToNot(BeEmpty(), "CertManagerRepoName is required for DeployRancher")
		Expect(input.CertManagerUrl).ToNot(BeEmpty(), "CertManagerUrl is required for DeployRancher")
		Expect(input.CertManagerChartPath).ToNot(BeEmpty(), "CertManagerChartPath is required for DeployRancher")

		By("Add cert manager chart repo")
		certChart := &opframework.HelmChart{
			BinaryPath:      input.HelmBinaryPath,
			Name:            input.CertManagerRepoName,
			Path:            input.CertManagerUrl,
			Commands:        opframework.Commands(opframework.Repo, opframework.Add),
			AdditionalFlags: opframework.Flags("--force-update"),
			Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
		}
		_, certErr := certChart.Run(nil)
		Expect(certErr).ToNot(HaveOccurred())
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

	if input.InstallCertManager {
		By("Installing cert-manager")
		certManagerChart := &opframework.HelmChart{
			BinaryPath: input.HelmBinaryPath,
			Path:       input.CertManagerChartPath,
			Name:       "cert-manager",
			Kubeconfig: input.BootstrapClusterProxy.GetKubeconfigPath(),
			AdditionalFlags: opframework.Flags(
				"--namespace", "cert-manager",
				"--version", "v1.12.0",
				"--create-namespace",
			),
			Wait: true,
		}
		_, err = certManagerChart.Run(map[string]string{
			"installCRDs": "true",
		})
		Expect(err).ToNot(HaveOccurred())
	}

	yamlExtraValues, err := yaml.Marshal(deployRancherValuesFile{
		BootstrapPassword: input.RancherPassword,
		Hostname:          input.RancherHost,
	})
	Expect(err).ToNot(HaveOccurred())
	err = ioutil.WriteFile(input.HelmExtraValuesPath, yamlExtraValues, 0644)
	Expect(err).ToNot(HaveOccurred())

	By("Installing Rancher")
	installFlags := opframework.Flags(
		"--namespace", input.RancherNamespace,
		"--create-namespace",
		"--values", input.HelmExtraValuesPath,
	)
	if input.RancherVersion != "" {
		installFlags = append(installFlags, "--version", input.RancherVersion)
	}
	if input.Development {
		installFlags = append(installFlags, "--devel")
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
	if input.RancherFeatures != "" {
		values["CATTLE_FEATURES"] = input.RancherFeatures
	}
	if input.RancherImageTag != "" {
		values["rancherImageTag"] = input.RancherImageTag
	}
	if input.RancherIngressClassName != "" {
		values["ingress.ingressClassName"] = input.RancherIngressClassName
	}

	_, err = chart.Run(values)
	Expect(err).ToNot(HaveOccurred())

	By("Updating rancher configuration")
	variableGetter := turtlesframework.GetVariable(input.Variables)
	for _, patch := range input.RancherPatches {
		Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
			Proxy:    input.BootstrapClusterProxy,
			Template: patch,
			Getter:   variableGetter,
			AddtionalEnvironmentVariables: map[string]string{
				e2e.RancherHostnameVar: input.RancherHost,
			},
		})).To(Succeed())
	}

	if len(input.RancherIngressConfig) > 0 {
		By("Setting up ingress")
		ingress, err := envsubst.Eval(string(input.RancherIngressConfig), os.Getenv)
		Expect(err).ToNot(HaveOccurred())
		Expect(input.BootstrapClusterProxy.Apply(ctx, []byte(ingress))).To(Succeed())
	}
	if len(input.RancherServicePatch) > 0 {
		By("Updating rancher svc")
		Expect(input.BootstrapClusterProxy.Apply(ctx, input.RancherServicePatch, "--server-side")).To(Succeed())
	}

	By("Waiting for rancher webhook rollout")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "rancher-webhook", Namespace: input.RancherNamespace}},
	}, input.RancherWaitInterval...)

	// hack: fleet controller needs to be restarted first to pickup config change with a valid API url.
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "fleet-controller", Namespace: "cattle-fleet-system"}},
	}, input.ControllerWaitInterval...)

	By("Bouncing the fleet")
	Eventually(func() error {
		return input.BootstrapClusterProxy.GetClient().DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace("cattle-fleet-system"), client.MatchingLabels{"app": "fleet-controller"})
	}, input.ControllerWaitInterval...).ShouldNot(HaveOccurred())
}

// RestartRancherInput represents the input parameters for restarting Rancher.
type RestartRancherInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// RancherNamespace is the namespace where Rancher is deployed.
	RancherNamespace string

	// RancherWaitInterval is the wait interval for Rancher restart.
	RancherWaitInterval []interface{}
}

// RestartRancher restarts the Rancher application by killing its pods.
// It expects the required input parameters to be non-nil.
func RestartRancher(ctx context.Context, input RestartRancherInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for RestartRancher")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for RestartRancher")
	Expect(input.RancherNamespace).ToNot(BeEmpty(), "RancherNamespace is required for RestartRancher")
	Expect(input.RancherWaitInterval).ToNot(BeNil(), "RancherWaitInterval is required for RestartRancher")

	By("Restarting Rancher by killing its pods")

	Eventually(func() error {
		return input.BootstrapClusterProxy.GetClient().DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(input.RancherNamespace), client.MatchingLabels{"app": "rancher"})
	}, input.RancherWaitInterval...).ShouldNot(HaveOccurred())
}

type IngressType string

const (
	// CustomIngress represents a custom ingress type.
	CustomIngress IngressType = "custom"

	// NgrokIngress represents an ngrok ingress type.
	NgrokIngress IngressType = "ngrok"

	// EKSNginxIngress represents an EKS nginx ingress type.
	EKSNginxIngress IngressType = "eks"
)

// RancherDeployIngressInput represents the input parameters for deploying an ingress in Rancher.
type RancherDeployIngressInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string

	// HelmExtraValuesPath is the path to the Helm extra values file.
	HelmExtraValuesPath string

	// CustomIngress is the custom ingress to be deployed.
	CustomIngress []byte

	// CustomIngressNamespace is the namespace for the custom ingress.
	CustomIngressNamespace string

	// CustomIngressDeployment is the deployment name for the custom ingress.
	CustomIngressDeployment string

	// IngressWaitInterval is the wait interval for the ingress deployment.
	IngressWaitInterval []interface{}

	// DefaultIngressClassPatch is the default ingress class patch.
	DefaultIngressClassPatch []byte

	// IngressType is the type of ingress to be deployed.
	IngressType IngressType

	// NgrokApiKey is the API key for Ngrok.
	NgrokApiKey string

	// NgrokAuthToken is the authentication token for Ngrok.
	NgrokAuthToken string

	// NgrokPath is the path to the Ngrok binary.
	NgrokPath string

	// NgrokRepoName is the name of the Ngrok repository.
	NgrokRepoName string

	// NgrokRepoURL is the URL of the Ngrok repository.
	NgrokRepoURL string
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
	Expect(input.IngressType).ToNot(BeEmpty(), "IngressType is required for RancherDeployIngress")

	komega.SetClient(input.BootstrapClusterProxy.GetClient())
	komega.SetContext(ctx)

	switch input.IngressType {
	case CustomIngress:
		Expect(input.CustomIngress).ToNot(BeEmpty(), "CustomIngress is required when using custom ingress")
		Expect(input.CustomIngressNamespace).ToNot(BeEmpty(), "CustomIngressNamespace is required when using custom ingress")
		Expect(input.CustomIngressDeployment).ToNot(BeEmpty(), "CustomIngressDeployment is required when using custom ingress")
		Expect(input.IngressWaitInterval).ToNot(BeNil(), "IngressWaitInterval is required when using custom ingress")
		deployIsolatedModeIngress(ctx, input)
	case NgrokIngress:
		Expect(input.NgrokApiKey).ToNot(BeEmpty(), "NgrokApiKey is required when using ngrok ingress")
		Expect(input.NgrokAuthToken).ToNot(BeEmpty(), "NgrokAuthToken is required when using ngrok ingress")
		Expect(input.NgrokPath).ToNot(BeEmpty(), "NgrokPath is required  when using ngrok ingress")
		Expect(input.NgrokRepoName).ToNot(BeEmpty(), "NgrokRepoName is required when using ngrok ingress")
		Expect(input.NgrokRepoURL).ToNot(BeEmpty(), "NgrokRepoURL is required when using ngrok ingress")
		Expect(input.HelmExtraValuesPath).ToNot(BeNil(), "HelmExtraValuesPath is when using ngrok ingress")
		deployNgrokIngress(ctx, input)
	case EKSNginxIngress:
		Expect(input.IngressWaitInterval).ToNot(BeNil(), "IngressWaitInterval is required when using eks ingress")
		deployEKSIngress(input)
	}
}

func deployIsolatedModeIngress(ctx context.Context, input RancherDeployIngressInput) {
	By("Deploying custom ingress")
	Expect(input.BootstrapClusterProxy.Apply(ctx, []byte(input.CustomIngress))).To(Succeed())

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
	By("Add nginx ingress chart repo")
	certChart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Name:            "ingress-nginx",
		Path:            "https://kubernetes.github.io/ingress-nginx",
		Commands:        opframework.Commands(opframework.Repo, opframework.Add),
		AdditionalFlags: opframework.Flags("--force-update"),
		Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
	}
	_, err := certChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	By("Installing nginx ingress")
	certManagerChart := &opframework.HelmChart{
		BinaryPath: input.HelmBinaryPath,
		Path:       "ingress-nginx/ingress-nginx",
		Name:       "ingress-nginx",
		Kubeconfig: input.BootstrapClusterProxy.GetKubeconfigPath(),
		AdditionalFlags: opframework.Flags(
			"--namespace", "ingress-nginx",
			"--version", "v4.9.0",
			"--create-namespace",
		),
		Wait: true,
	}
	_, err = certManagerChart.Run(map[string]string{
		"controller.service.type": "LoadBalancer",
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
	err = ioutil.WriteFile(input.HelmExtraValuesPath, yamlExtraValues, 0644)
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
	Expect(input.BootstrapClusterProxy.Apply(ctx, input.DefaultIngressClassPatch, "--server-side")).To(Succeed())
}

// PreRancherInstallHookInput represents the input parameters for the pre-Rancher install hook.
type PreRancherInstallHookInput struct {
	// Ctx is the context for the hook execution.
	Ctx context.Context

	// RancherInput is the input parameters for deploying Rancher.
	RancherInput *DeployRancherInput

	// PreSetupOutput is the output of the pre-management cluster setup.
	PreSetupOutput PreManagementClusterSetupResult

	// SetupClusterResult is the result of setting up the test cluster.
	SetupClusterResult *SetupTestClusterResult

	// E2EConfig is the E2E configuration for the cluster.
	E2EConfig *clusterctl.E2EConfig
}

// PreRancherInstallHookResult represents the result of a pre-Rancher install hook.
type PreRancherInstallHookResult struct {
	// HostName is the hostname of the Rancher installation.
	HostName string
}

// PreRancherInstallHook is a function that performs pre-installation tasks for Rancher.
// The function retrieves the infrastructure type from the input and performs different actions based on the type.
// If the infrastructure type is e2e.ManagementClusterEnvironmentEKS, it retrieves the ingress hostname and sets it as the Rancher host.
// It also deploys ghcr details by creating a Docker registry secret.
// If the infrastructure type is e2e.ManagementClusterEnvironmentIsolatedKind, it sets the isolated host name as the Rancher host.
// If the infrastructure type is e2e.ManagementClusterEnvironmentKind, it sets the Rancher ingress config and service patch based on the provided values.
// The function returns the host name as part of the PreRancherInstallHookResult.
func PreRancherInstallHook(input *PreRancherInstallHookInput) PreRancherInstallHookResult {
	hostName := ""

	infrastructureType := e2e.ManagementClusterEnvironmentType(input.E2EConfig.GetVariable(e2e.ManagementClusterEnvironmentVar))

	switch e2e.ManagementClusterEnvironmentType(infrastructureType) {
	case e2e.ManagementClusterEnvironmentEKS:
		By("Getting ingress hostname")
		svcRes := &WaitForServiceIngressHostnameResult{}
		WaitForServiceIngressHostname(input.Ctx, WaitForServiceIngressHostnameInput{
			BootstrapClusterProxy: input.SetupClusterResult.BootstrapClusterProxy,
			ServiceName:           "ingress-nginx-controller",
			ServiceNamespace:      "ingress-nginx",
			IngressWaitInterval:   input.E2EConfig.GetIntervals(input.SetupClusterResult.BootstrapClusterProxy.GetName(), "wait-rancher"),
		}, svcRes)

		hostName = svcRes.Hostname
		input.RancherInput.RancherHost = hostName

		By("Deploying ghcr details")
		turtlesframework.CreateDockerRegistrySecret(input.Ctx, turtlesframework.CreateDockerRegistrySecretInput{
			Name:                  "regcred",
			BootstrapClusterProxy: input.SetupClusterResult.BootstrapClusterProxy,
			Namespace:             "rancher-turtles-system",
			DockerServer:          "https://ghcr.io",
			DockerUsername:        input.PreSetupOutput.DockerUsername,
			DockerPassword:        input.PreSetupOutput.DockerPassword,
		})

		input.RancherInput.RancherIngressClassName = "nginx"
	case e2e.ManagementClusterEnvironmentIsolatedKind:
		hostName = input.SetupClusterResult.IsolatedHostName
		input.RancherInput.RancherHost = hostName
	case e2e.ManagementClusterEnvironmentKind:
		// i.e. we are using ngrok locally
		input.RancherInput.RancherIngressConfig = e2e.IngressConfig
		input.RancherInput.RancherServicePatch = e2e.RancherServicePatch
		hostName = input.E2EConfig.GetVariable(e2e.RancherHostnameVar)
		input.RancherInput.RancherHost = hostName
	default:
		Fail(fmt.Sprintf("Invalid management cluster infrastructure type %q", infrastructureType))
	}

	return PreRancherInstallHookResult{
		HostName: hostName,
	}
}
