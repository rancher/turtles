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

type DeployRancherInput struct {
	BootstrapClusterProxy   framework.ClusterProxy
	HelmBinaryPath          string
	HelmExtraValuesPath     string
	InstallCertManager      bool
	CertManagerChartPath    string
	CertManagerUrl          string
	CertManagerRepoName     string
	RancherChartRepoName    string
	RancherChartURL         string
	RancherChartPath        string
	RancherVersion          string
	RancherImageTag         string
	RancherNamespace        string
	RancherHost             string
	RancherPassword         string
	RancherFeatures         string
	RancherPatches          [][]byte
	RancherWaitInterval     []interface{}
	ControllerWaitInterval  []interface{}
	RancherIngressConfig    []byte
	RancherServicePatch     []byte
	RancherIngressClassName string
	Development             bool
	Variables               turtlesframework.VariableCollection
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

type RestartRancherInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	RancherNamespace      string
	RancherWaitInterval   []interface{}
}

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
	CustomIngress   IngressType = "custom"
	NgrokIngress    IngressType = "ngrok"
	EKSNginxIngress IngressType = "eks"
)

type RancherDeployIngressInput struct {
	BootstrapClusterProxy    framework.ClusterProxy
	HelmBinaryPath           string
	HelmExtraValuesPath      string
	CustomIngress            []byte // TODO: add ability to pass a function that deploys the custom ingress
	CustomIngressNamespace   string
	CustomIngressDeployment  string
	IngressWaitInterval      []interface{}
	DefaultIngressClassPatch []byte
	IngressType              IngressType
	NgrokApiKey              string
	NgrokAuthToken           string
	NgrokPath                string
	NgrokRepoName            string
	NgrokRepoURL             string
}

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

type PreRancherInstallHookInput struct {
	Ctx                context.Context
	RancherInput       *DeployRancherInput
	PreSetupOutput     PreManagementClusterSetupResult
	SetupClusterResult *SetupTestClusterResult
	E2EConfig          *clusterctl.E2EConfig
}

type PreRancherInstallHookResult struct {
	HostName string
}

// PreRancherInstallHook is a hook that can be used to perform actions before Rancher is installed.
func PreRancherInstallHook(input *PreRancherInstallHookInput) PreRancherInstallHookResult {
	hostName := ""

	switch e2e.ManagementClusterInfrastuctureType(input.E2EConfig.GetVariable(e2e.ManagementClusterInfrastucture)) {
	case e2e.ManagementClusterInfrastuctureEKS:
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
	case e2e.ManagementClusterInfrastuctureIsolatedKind:
		hostName = input.SetupClusterResult.IsolatedHostName
		input.RancherInput.RancherHost = hostName
	case e2e.ManagementClusterInfrastuctureKind:
		// i.e. we are using ngrok locally
		input.RancherInput.RancherIngressConfig = e2e.IngressConfig
		input.RancherInput.RancherServicePatch = e2e.RancherServicePatch
		hostName = input.E2EConfig.GetVariable(e2e.RancherHostnameVar)
		input.RancherInput.RancherHost = hostName
	default:
		Fail(fmt.Sprintf("Invalid management cluster infrastructure type %q", input.E2EConfig.GetVariable(e2e.ManagementClusterInfrastucture)))
	}

	return PreRancherInstallHookResult{
		HostName: hostName,
	}
}
