/*
Copyright 2023 SUSE.

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
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesframework "github.com/rancher-sandbox/rancher-turtles/test/framework"

	"github.com/drone/envsubst/v2"
	"github.com/rancher-sandbox/rancher-turtles/test/e2e"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opframework "sigs.k8s.io/cluster-api-operator/test/framework"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/yaml"
)

type DeployRancherInput struct {
	BootstrapClusterProxy  framework.ClusterProxy
	HelmBinaryPath         string
	HelmExtraValuesPath    string
	InstallCertManager     bool
	CertManagerChartPath   string
	CertManagerUrl         string
	CertManagerRepoName    string
	RancherChartRepoName   string
	RancherChartURL        string
	RancherChartPath       string
	RancherVersion         string
	RancherImageTag        string
	RancherNamespace       string
	RancherHost            string
	RancherPassword        string
	RancherFeatures        string
	RancherPatches         [][]byte
	RancherWaitInterval    []interface{}
	ControllerWaitInterval []interface{}
	IsolatedMode           bool
	RancherIngressConfig   []byte
	RancherServicePatch    []byte
	Development            bool
	Variables              turtlesframework.VariableCollection
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

	if !input.IsolatedMode {
		By("Setting up ingress")

		ingress, err := envsubst.Eval(string(input.RancherIngressConfig), os.Getenv)
		Expect(err).ToNot(HaveOccurred())
		Expect(input.BootstrapClusterProxy.Apply(ctx, []byte(ingress))).To(Succeed())

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

type RancherDeployIngressInput struct {
	BootstrapClusterProxy    framework.ClusterProxy
	HelmBinaryPath           string
	HelmExtraValuesPath      string
	IsolatedMode             bool
	NginxIngress             []byte
	NginxIngressNamespace    string
	IngressWaitInterval      []interface{}
	NgrokApiKey              string
	NgrokAuthToken           string
	NgrokPath                string
	NgrokRepoName            string
	NgrokRepoURL             string
	DefaultIngressClassPatch []byte
}

func RancherDeployIngress(ctx context.Context, input RancherDeployIngressInput) {

	Expect(ctx).NotTo(BeNil(), "ctx is required for RancherDeployIngress")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for RancherDeployIngress")
	if input.IsolatedMode {
		Expect(input.NginxIngress).ToNot(BeEmpty(), "NginxIngress is required when running in isolated mode")
		Expect(input.NginxIngressNamespace).ToNot(BeEmpty(), "NginxIngressNamespace is required when running in isolated mode")
		Expect(input.IngressWaitInterval).ToNot(BeNil(), "IngressWaitInterval is required when running in isolated mode")
	} else {
		Expect(input.NgrokApiKey).ToNot(BeEmpty(), "NgrokApiKey is required when not running in isolated mode")
		Expect(input.NgrokAuthToken).ToNot(BeEmpty(), "NgrokAuthToken is required when not running in isolated mode")
		Expect(input.NgrokPath).ToNot(BeEmpty(), "NgrokPath is required when not running in isolated mode")
		Expect(input.NgrokRepoName).ToNot(BeEmpty(), "NgrokRepoName is required when not running in isolated mode")
		Expect(input.NgrokRepoURL).ToNot(BeEmpty(), "NgrokRepoURL is required when not running in isolated mode")
		Expect(input.HelmExtraValuesPath).ToNot(BeNil(), "HelmExtraValuesPath is when not running in isolated mode")
	}

	komega.SetClient(input.BootstrapClusterProxy.GetClient())
	komega.SetContext(ctx)

	if input.IsolatedMode {
		By("Deploying nginx ingress")
		Expect(input.BootstrapClusterProxy.Apply(ctx, []byte(input.NginxIngress))).To(Succeed())

		By("Getting nginx ingress deployment")
		ngixDeployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "ingress-nginx-controller", Namespace: input.NginxIngressNamespace}}
		Eventually(
			komega.Get(ngixDeployment),
			input.IngressWaitInterval...,
		).Should(Succeed(), "Failed to get nginx ingress controller")

		By("Waiting for ingress-nginx-controller deployment to be available")
		Eventually(komega.Object(ngixDeployment), input.IngressWaitInterval...).Should(HaveField("Status.AvailableReplicas", Equal(int32(1))))

		return
	}

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
