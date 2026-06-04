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
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"

	turtlesframework "github.com/rancher/turtles/test/framework"
)

// CustomClusterProvider is a function type that represents a custom cluster provider.
// It takes in a context, an E2EConfig, a cluster name, and a Kubernetes version as parameters.
// It returns a bootstrap.ClusterProvider.
type CustomClusterProvider func(ctx context.Context, config *clusterctl.E2EConfig, clusterName, eksManagementVersion string, kubernetesManagementVersion string, kubernetesDownstreamVersion string) bootstrap.ClusterProvider

// EKSBootstrapCluster is a function that creates a new EKS bootstrap cluster.
func EKSBootstrapCluster(ctx context.Context, config *clusterctl.E2EConfig, clusterName, eksManagementVersion string, _ string, _ string) bootstrap.ClusterProvider {
	By("Creating a new EKS bootstrap cluster")

	region := config.Variables["KUBERNETES_MANAGEMENT_AWS_REGION"]
	Expect(region).ToNot(BeEmpty(), "KUBERNETES_MANAGEMENT_AWS_REGION must be set in the e2e config")

	eksCreateResult := &CreateEKSBootstrapClusterAndValidateImagesInputResult{}
	CreateEKSBootstrapClusterAndValidateImages(ctx, CreateEKSBootstrapClusterAndValidateImagesInput{
		Name:       clusterName,
		Version:    eksManagementVersion,
		Region:     region,
		NumWorkers: 1,
		Images:     config.Images,
	}, eksCreateResult)

	return eksCreateResult.BootstrapClusterProvider
}

// KindBootstrapCluster is a function that creates a new kind bootstrap cluster with extra port mappings. This is useful for forwarding requests from the host.
func KindWithExtraPortMappingsBootstrapCluster(ctx context.Context, config *clusterctl.E2EConfig, clusterName, _ string, kubernetesManagementVersion string, kubernetesDownstreamVersion string) bootstrap.ClusterProvider {

	Expect(kubernetesManagementVersion).ShouldNot(BeEmpty(), "kubernetesManagementVersion is required")

	By(fmt.Sprintf("Building kindest/node:%s for management Cluster", kubernetesManagementVersion))
	BuildKindImage(ctx, kubernetesManagementVersion)

	if len(kubernetesDownstreamVersion) > 0 {
		By(fmt.Sprintf("Building kindest/node:%s for downstream Clusters", kubernetesDownstreamVersion))
		BuildKindImage(ctx, kubernetesDownstreamVersion)
	}

	By("Creating a new kind bootstrap cluster with extra port mappings")

	return bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
		Name:               clusterName,
		KubernetesVersion:  kubernetesManagementVersion,
		RequiresDockerSock: true,
		Images:             config.Images,
		ExtraPortMappings: []v1alpha4.PortMapping{
			{ContainerPort: 80, HostPort: 80, Protocol: v1alpha4.PortMappingProtocolTCP},
			{ContainerPort: 443, HostPort: 443, Protocol: v1alpha4.PortMappingProtocolTCP},
			{ContainerPort: 30002, HostPort: 30002, Protocol: v1alpha4.PortMappingProtocolTCP}, // etcd nodeport
		},
	})
}

// BuildKindImage is a function that builds a local kindest/node image for an arbitrary k8s version.
func BuildKindImage(ctx context.Context, version string) {
	Eventually(func() error {
		kindBuildImage := turtlesframework.RunCommand(ctx, turtlesframework.RunCommandInput{
			Command: "kind",
			Args: []string{
				"build",
				"node-image",
				"--type", "release",
				"--image", fmt.Sprintf("kindest/node:%s", version),
				version,
			},
		})
		if kindBuildImage.Error != nil || kindBuildImage.ExitCode != 0 {
			// FIXME: https://github.com/rancher/turtles/issues/2449
			GinkgoWriter.Printf("\nFailed to build kindest/node image\n")
			GinkgoWriter.Printf("kind build stdout: \n%s\n", string(kindBuildImage.Stdout))
			GinkgoWriter.Printf("kind build stderr: \n%s\n", string(kindBuildImage.Stderr))
			cleanBuildKindImageCache()
			return fmt.Errorf("Failed to build kindest/node image. Exit Code: %d. Error: %w",
				kindBuildImage.ExitCode, kindBuildImage.Error)
		}
		return nil
	}).WithPolling(5 * time.Minute).WithTimeout(30 * time.Minute).ShouldNot(HaveOccurred())
}

// cleanBuildKindImageCache is a helper function that deletes '/tmp/k8s-tar-extract-*' folders.
// These are created by kind when downloading the k8s tarball that can be ~350MB in size.
func cleanBuildKindImageCache() {
	const cleanTarget = "/tmp/k8s-tar-extract-*"

	matches, err := filepath.Glob(cleanTarget)
	Expect(err).ShouldNot(HaveOccurred())

	for _, match := range matches {
		GinkgoWriter.Printf("Cleaning kind build image cache %s\n", match)
		Expect(os.RemoveAll(match)).Should(Succeed())
	}
}
