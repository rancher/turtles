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

package framework

import (
	"context"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opframework "sigs.k8s.io/cluster-api-operator/test/framework"
	"sigs.k8s.io/cluster-api/test/framework"
)

// ChartMuseumInput represents the input parameters for interacting with ChartMuseum.
type ChartMuseumInput struct {
	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string

	// ChartsPath is the path to the charts.
	ChartsPath string

	// ChartVersion is the version of the chart.
	ChartVersion string

	// ChartMuseumManifests are the ChartMuseum manifests.
	ChartMuseumManifests []byte

	// DeploymentName is the name of the deployment.
	DeploymentName string

	// ServiceName is the name of the service.
	ServiceName string

	// PortName is the name of the port.
	PortName string

	// Proxy is the cluster proxy.
	Proxy framework.ClusterProxy

	// WaitInterval is the wait interval.
	WaitInterval []interface{}
}

// DeployChartMuseum will create a new repo in the Gitea server.
func DeployChartMuseum(ctx context.Context, input ChartMuseumInput) string {
	Expect(ctx).NotTo(BeNil(), "ctx is required for DeployChartMuseum")

	Expect(input.ServiceName).ToNot(BeEmpty(), "Invalid argument. input.ServiceName can't be empty for calling DeployChartMuseum")
	Expect(input.DeploymentName).ToNot(BeEmpty(), "Invalid argument. input.DeploymentName can't be empty when calling DeployChartMuseum")
	Expect(input.HelmBinaryPath).ToNot(BeEmpty(), "HelmBinaryPath is required for DeployChartMuseum")
	Expect(input.ChartsPath).ToNot(BeEmpty(), "ChartsPath is required for DeployChartMuseum")
	Expect(input.ChartMuseumManifests).ToNot(BeEmpty(), "Invalid argument. input.ChartMuseumManifests must be an existing set of manifests")
	Expect(input.Proxy).NotTo(BeNil(), "Cluster proxy is required for DeployChartMuseum.")
	Expect(input.WaitInterval).ToNot(BeNil(), "WaitInterval is required for DeployGitea")

	By("Installing chartmuseum push plugin")
	exec.Command(
		input.HelmBinaryPath,
		"plugin", "install",
		"https://github.com/chartmuseum/helm-push.git",
		"--kubeconfig", input.Proxy.GetKubeconfigPath(),
	).CombinedOutput()

	By("Creating chartmuseum manifests")
	Expect(input.Proxy.Apply(ctx, input.ChartMuseumManifests)).ShouldNot(HaveOccurred())

	By("Waiting for chartmuseum rollout")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     input.Proxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: input.DeploymentName, Namespace: "default"}},
	}, input.WaitInterval...)

	By("Waiting for the chartmuseum service address")
	port := GetServicePortByName(ctx, GetServicePortByNameInput{
		GetLister:        input.Proxy.GetClient(),
		ServiceName:      input.ServiceName,
		PortName:         input.PortName,
		ServiceNamespace: "default",
	}, input.WaitInterval...)

	addr := GetNodeAddress(ctx, GetNodeAddressInput{
		Lister:       input.Proxy.GetClient(),
		NodeIndex:    0,
		AddressIndex: 0,
	})

	By("Adding local rancher turtles chart repo")
	addChart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Name:            "rancher-turtles-local",
		Path:            fmt.Sprintf("http://%s:%d", addr, port.NodePort),
		Commands:        opframework.Commands(opframework.Repo, opframework.Add),
		AdditionalFlags: opframework.Flags("--force-update"),
		Kubeconfig:      input.Proxy.GetKubeconfigPath(),
	}
	_, err := addChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	By("Pushing local chart to chartmuseum")
	exec.Command(
		input.HelmBinaryPath,
		"cm-push", input.ChartsPath,
		"rancher-turtles-local", "-a", input.ChartVersion,
		"--kubeconfig", input.Proxy.GetKubeconfigPath(),
	).CombinedOutput()

	return fmt.Sprintf("http://%s:%d", addr, port.NodePort)
}
