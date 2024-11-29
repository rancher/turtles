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

	"github.com/rancher/turtles/test/e2e"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"

	turtlesframework "github.com/rancher/turtles/test/framework"
)

// DeployChartMuseumInput represents the input parameters for deploying ChartMuseum.
type DeployChartMuseumInput struct {
	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string

	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// ChartsPath is the path to the charts.
	ChartsPath string

	// ChartVersion is the version of the chart.
	ChartVersion string

	// WaitInterval is the interval to wait for.
	WaitInterval []interface{}

	// CustomIngressConfig is the custom ingress configuration.
	CustomIngressConfig []byte

	// Variables is the collection of variables.
	Variables turtlesframework.VariableCollection
}

// DeployChartMuseum installs ChartMuseum to the Kubernetes cluster using the provided input parameters.
// It expects the required input parameters to be non-nil.
func DeployChartMuseum(ctx context.Context, input DeployChartMuseumInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DeployChartMuseum")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for DeployChartMuseum")
	Expect(input.ChartsPath).ToNot(BeNil(), "ChartsPath is required for DeployChartMuseum")
	Expect(input.WaitInterval).ToNot(BeNil(), "WaitInterval is required for DeployChartMuseum")
	Expect(input.HelmBinaryPath).To(BeAnExistingFile(), "Invalid test suite argument. helm-binary-path should be an existing file.")

	By("Installing ChartMuseum")
	turtlesframework.DeployChartMuseum(ctx, turtlesframework.ChartMuseumInput{
		HelmBinaryPath:       input.HelmBinaryPath,
		ChartsPath:           input.ChartsPath,
		ChartVersion:         input.ChartVersion,
		Proxy:                input.BootstrapClusterProxy,
		WaitInterval:         input.WaitInterval,
		ChartMuseumManifests: e2e.ChartMuseum,
		DeploymentName:       "chartmuseum",
		ServiceName:          "chartmuseum-service",
		PortName:             "http",
		CustomIngressConfig:  input.CustomIngressConfig,
		Variables:            input.Variables,
	})
}

// PreChartMuseumInstallHook is a pre-install hook for ChartMuseum.
func PreChartMuseumInstallHook(chartMuseumInput *DeployChartMuseumInput, e2eConfig *clusterctl.E2EConfig) {
	infrastructureType := e2e.ManagementClusterEnvironmentType(e2eConfig.GetVariable(e2e.ManagementClusterEnvironmentVar))

	switch infrastructureType {
	case e2e.ManagementClusterEnvironmentKind:
		chartMuseumInput.CustomIngressConfig = e2e.ChartMuseumIngress
	}
}
