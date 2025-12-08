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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesframework "github.com/rancher/turtles/test/framework"

	opframework "sigs.k8s.io/cluster-api-operator/test/framework"
	"sigs.k8s.io/cluster-api/test/framework"
)

// DeployCertManagerInput represents the input parameters for deploying Cert Manager.
type DeployCertManagerInput struct {
	// BootstrapClusterProxy is the cluster proxy for bootstrapping.
	BootstrapClusterProxy framework.ClusterProxy

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string `env:"HELM_BINARY_PATH"`

	// CertManagerChartPath is the path to the Cert Manager chart.
	CertManagerChartPath string `env:"CERT_MANAGER_PATH"`

	// CertManagerUrl is the URL for Cert Manager.
	CertManagerUrl string `env:"CERT_MANAGER_URL"`

	// CertManagerRepoName is the repository name for Cert Manager.
	CertManagerRepoName string `env:"CERT_MANAGER_REPO_NAME"`
}

// DeployCertManager deploys Cert Manager using the provided input parameters.
func DeployCertManager(ctx context.Context, input DeployCertManagerInput) {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for DeployRancher")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for DeployCertManager")

	Expect(input.CertManagerRepoName).ToNot(BeEmpty(), "CertManagerRepoName is required for DeployRancher")
	Expect(input.CertManagerUrl).ToNot(BeEmpty(), "CertManagerUrl is required for DeployRancher")
	Expect(input.CertManagerChartPath).ToNot(BeEmpty(), "CertManagerChartPath is required for DeployRancher")

	By("Adding cert-manager chart repo")
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

	By("Installing cert-manager")
	certManagerChart := &opframework.HelmChart{
		BinaryPath: input.HelmBinaryPath,
		Path:       input.CertManagerChartPath,
		Name:       "cert-manager",
		Kubeconfig: input.BootstrapClusterProxy.GetKubeconfigPath(),
		AdditionalFlags: opframework.Flags(
			"--namespace", "cert-manager",
			"--version", "v1.16.3",
			"--create-namespace",
		),
		Wait: true,
	}
	_, err := certManagerChart.Run(map[string]string{
		"crds.enabled": "true",
		"crds.keep":    "true",
	})
	Expect(err).ToNot(HaveOccurred())
}
