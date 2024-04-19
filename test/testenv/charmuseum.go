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

	turtlesframework "github.com/rancher/turtles/test/framework"
)

type DeployChartMuseumInput struct {
	HelmBinaryPath        string
	BootstrapClusterProxy framework.ClusterProxy
	ChartsPath            string
	WaitInterval          []interface{}
}

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
		Proxy:                input.BootstrapClusterProxy,
		WaitInterval:         input.WaitInterval,
		ChartMuseumManifests: e2e.ChartMuseum,
		DeploymentName:       "chartmuseum",
		ServiceName:          "chartmuseum-service",
		PortName:             "http",
	})
}
