/*
Copyright © 2024 - 2025 SUSE LLC

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
	"net/http"
	"os"

	"github.com/drone/envsubst/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/turtles/test/e2e"
	turtlesframework "github.com/rancher/turtles/test/framework"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opframework "sigs.k8s.io/cluster-api-operator/test/framework"
	"sigs.k8s.io/cluster-api/test/framework"
)

// DeployGiteaInput represents the input parameters for deploying Gitea.
type DeployGiteaInput struct {
	// EnvironmentType is the environment type
	EnvironmentType e2e.ManagementClusterEnvironmentType `env:"MANAGEMENT_CLUSTER_ENVIRONMENT"`

	// BootstrapClusterProxy is the cluster proxy for bootstrapping.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string `env:"HELM_BINARY_PATH"`

	// ChartRepoName is the name of the chart repository.
	ChartRepoName string `env:"GITEA_REPO_NAME"`

	// ChartRepoURL is the URL of the chart repository.
	ChartRepoURL string `env:"GITEA_REPO_URL"`

	// ChartName is the name of the chart.
	ChartName string `env:"GITEA_CHART_NAME"`

	// ChartVersion is the version of the chart.
	ChartVersion string `env:"GITEA_CHART_VERSION"`

	// ValuesFile is the data the values file.
	ValuesFile []byte

	// Values are the values for the chart.
	Values map[string]string

	// RolloutWaitInterval is the interval to wait between rollouts.
	RolloutWaitInterval []interface{} `envDefault:"3m,10s"`

	// ServiceWaitInterval is the interval to wait for the service.
	ServiceWaitInterval []interface{} `envDefault:"5m,10s"`

	// Username is the username for authentication.
	Username string `env:"GITEA_USER_NAME"`

	// Password is the password for authentication.
	Password string `env:"GITEA_USER_PWD"`

	// AuthSecretName is the name of the authentication secret.
	AuthSecretName string `envDefault:"basic-auth-secret"`

	// CustomIngressConfig is the custom ingress configuration.
	CustomIngressConfig []byte

	// ServiceType is the type of the service.
	ServiceType corev1.ServiceType
}

// DeployGiteaResult represents the result of deploying Gitea.
type DeployGiteaResult struct {
	// GitAddress is the address of the deployed Gitea instance.
	GitAddress string
}

// DeployGitea deploys Gitea using the provided input parameters.
// It expects the required input parameters to be non-nil.
// If the service type is ClusterIP, it checks that the custom ingress config is not empty.
// The function then proceeds to install the Gitea chart using Helm.
// It adds the chart repository, updates the chart, and installs the chart with the specified version and flags.
// After the installation, it waits for the Gitea deployment to be available.
// Depending on the service type, it retrieves the Git server address using the node port, load balancer, or custom ingress.
// If a username is provided, it waits for the Gitea endpoint to be available and creates a Gitea secret with the username and password.
func DeployGitea(ctx context.Context, input DeployGiteaInput) *DeployGiteaResult {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")
	PreGiteaInstallHook(&input)

	Expect(ctx).NotTo(BeNil(), "ctx is required for DeployGitea")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for DeployGitea")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for DeployGitea")
	Expect(input.ChartRepoName).ToNot(BeEmpty(), "ChartRepoName is required for DeployGitea")
	Expect(input.ChartRepoURL).ToNot(BeEmpty(), "ChartRepoURL is required for DeployGitea")
	Expect(input.ChartName).ToNot(BeEmpty(), "ChartName is required for DeployGitea")
	Expect(input.ChartVersion).ToNot(BeEmpty(), "Chartversion is required for DeployGitea")
	Expect(input.RolloutWaitInterval).ToNot(BeNil(), "RolloutWaitInterval is required for DeployGitea")
	Expect(input.ServiceWaitInterval).ToNot(BeNil(), "ServiceWaitInterval is required for DeployGitea")
	Expect(input.ServiceType).ToNot(BeEmpty(), "ServiceType is required for DeployGitea")

	if input.Username != "" {
		Expect(input.Password).ToNot(BeEmpty(), "Password is required for DeployGitea if a username is supplied")
		Expect(input.AuthSecretName).ToNot(BeEmpty(), "AuthSecretName is required for DeployGitea if a username is supplied")
	}

	if input.ServiceType == corev1.ServiceTypeClusterIP {
		Expect(input.CustomIngressConfig).ToNot(BeEmpty(), "CustomIngressConfig is required for DeployGitea if service type is ClusterIP")
	}

	if input.Values == nil {
		input.Values = map[string]string{}
	}

	if input.Values["service.http.type"] == "" {
		input.Values["service.http.type"] = string(input.ServiceType)
	}
	if input.Values["gitea.admin.username"] == "" {
		input.Values["gitea.admin.username"] = input.Username
	}
	if input.Values["gitea.admin.password"] == "" {
		input.Values["gitea.admin.password"] = input.Password
	}

	result := &DeployGiteaResult{}

	By("Installing gitea chart")
	addChart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Name:            input.ChartRepoName,
		Path:            input.ChartRepoURL,
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

	flags := opframework.Flags(
		"--version", input.ChartVersion,
		"--namespace", "gitea",
		"--create-namespace",
		"--wait",
	)

	if input.ValuesFile != nil {
		giteaValues, err := os.CreateTemp("", "gitea-values.yaml")
		Expect(err).NotTo(HaveOccurred(), "Failed to create temp file for gitea values")
		Expect(os.WriteFile(giteaValues.Name(), input.ValuesFile, os.ModePerm)).To(Succeed(), "Failed to write gitea values to tmp file")
		flags = append(flags, "-f", giteaValues.Name())
	}

	chart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Path:            fmt.Sprintf("%s/%s", input.ChartRepoName, input.ChartName),
		Name:            "gitea",
		Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
		AdditionalFlags: flags,
	}

	// Gitea values can be found gitea_values.yaml file as well. For a list of the values
	// available look here: https://gitea.com/gitea/helm-chart/src/branch/main/values.yaml
	_, err = chart.Run(input.Values)
	Expect(err).ToNot(HaveOccurred())

	By("Waiting for gitea rollout")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gitea", Namespace: "gitea"}},
	}, input.RolloutWaitInterval...)

	port := turtlesframework.GetServicePortByName(ctx, turtlesframework.GetServicePortByNameInput{
		GetLister:        input.BootstrapClusterProxy.GetClient(),
		ServiceName:      "gitea-http",
		ServiceNamespace: "gitea",
		PortName:         "http",
	}, input.ServiceWaitInterval...)
	Expect(port.NodePort).ToNot(Equal(0), "Node port for Gitea service is not set")

	switch input.Values["service.http.type"] {
	case string(corev1.ServiceTypeNodePort):
		By("Get Git server node port")
		addr := turtlesframework.GetNodeAddress(ctx, turtlesframework.GetNodeAddressInput{
			Lister:       input.BootstrapClusterProxy.GetClient(),
			NodeIndex:    0,
			AddressIndex: 0,
		})

		result.GitAddress = fmt.Sprintf("http://%s:%d", addr, port.NodePort)
	case string(corev1.ServiceTypeLoadBalancer):
		By("Getting git server ingress address")
		svcRes := &WaitForServiceIngressHostnameResult{}
		WaitForServiceIngressHostname(ctx, WaitForServiceIngressHostnameInput{
			BootstrapClusterProxy: input.BootstrapClusterProxy,
			ServiceName:           "gitea-http",
			ServiceNamespace:      "gitea",
			IngressWaitInterval:   input.ServiceWaitInterval,
		}, svcRes)
		result.GitAddress = fmt.Sprintf("http://%s:%d", svcRes.Hostname, port.Port)
	case string(corev1.ServiceTypeClusterIP):
		By("Creating custom ingress for gitea")
		var ingress string
		var err error
		if input.EnvironmentType == e2e.ManagementClusterEnvironmentInternalKind {
			ingress, err = envsubst.Eval(string(input.CustomIngressConfig), func(s string) string {
				if s == "GITEA_INGRESS_CLASS_NAME" {
					return "traefik"
				}
				return os.Getenv(s)
			})
		} else {
			ingress, err = envsubst.Eval(string(input.CustomIngressConfig), os.Getenv)
		}

		Expect(err).ToNot(HaveOccurred())
		Expect(turtlesframework.Apply(ctx, input.BootstrapClusterProxy, []byte(ingress))).To(Succeed())

		By("Getting git server ingress address")
		host := turtlesframework.GetIngressHost(ctx, turtlesframework.GetIngressHostInput{
			GetLister:        input.BootstrapClusterProxy.GetClient(),
			IngressRuleIndex: 0,
			IngressName:      "gitea-http",
			IngressNamespace: "gitea",
		})

		if input.EnvironmentType == e2e.ManagementClusterEnvironmentInternalKind {
			// Fleet fails TLS verification when using a self-signed certificate, hence we use http
			result.GitAddress = fmt.Sprintf("http://%s", host)
		} else {
			result.GitAddress = fmt.Sprintf("https://%s", host)
		}
	}

	if input.Username == "" {
		By("No gitea username, skipping creation of auth secret")
		return result
	}

	// Only check HTTP endpoint for environments where it's accessible from the test runner
	// In isolated-kind, the service uses NodePort but is only accessible from within Docker network
	if input.EnvironmentType != e2e.ManagementClusterEnvironmentIsolatedKind {
		By("Waiting for Gitea endpoint to be available")
		url := fmt.Sprintf("%s/api/v1/version", result.GitAddress)
		Eventually(func() error {
			resp, err := http.Get(url)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("expected status OK, got %v", resp.Status)
			}

			return nil
		}, input.ServiceWaitInterval...).Should(Succeed())
	}

	return result
}

// UninstallGiteaInput represents the input parameters for uninstalling Gitea.
type UninstallGiteaInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string `env:"HELM_BINARY_PATH"`

	// DeleteWaitInterval is the interval to wait between deleting resources.
	DeleteWaitInterval []interface{} `envDefault:"10m,10s"`
}

// UninstallGitea uninstalls Gitea by removing the Gitea Helm Chart.
// It expects the required input parameters to be non-nil.
func UninstallGitea(ctx context.Context, input UninstallGiteaInput) {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for UninstallGitea")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for UninstallGitea")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for UninstallGitea")
	Expect(input.DeleteWaitInterval).ToNot(BeNil(), "DeleteWaitInterval is required for UninstallGitea")

	By("Removing Gitea Helm Chart")
	removeChart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Name:            "gitea",
		Commands:        opframework.Commands(opframework.Uninstall),
		Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
		AdditionalFlags: opframework.Flags("--wait"),
	}
	_, err := removeChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())
}

// PreGiteaInstallHook is a function that sets the service type for the Gitea input based on the management cluster environment type.
// It expects the required input parameters to be non-nil.
func PreGiteaInstallHook(giteaInput *DeployGiteaInput) {
	Expect(turtlesframework.Parse(giteaInput)).To(Succeed(), "Failed to parse environment variables")

	switch giteaInput.EnvironmentType {
	case e2e.ManagementClusterEnvironmentEKS:
		giteaInput.ServiceType = corev1.ServiceTypeLoadBalancer
	case e2e.ManagementClusterEnvironmentIsolatedKind:
		giteaInput.ServiceType = corev1.ServiceTypeNodePort
	case e2e.ManagementClusterEnvironmentKind, e2e.ManagementClusterEnvironmentInternalKind:
		giteaInput.ServiceType = corev1.ServiceTypeClusterIP
	}
}
