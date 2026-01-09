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

	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"

	turtlesframework "github.com/rancher/turtles/test/framework"
)

// CreateEKSBootstrapClusterAndValidateImagesInput represents the input parameters for creating an EKS bootstrap cluster and validating images.
type CreateEKSBootstrapClusterAndValidateImagesInput struct {
	// Name is the name of the bootstrap cluster.
	Name string
	// Version is the version of the bootstrap cluster.
	Version string
	// Region is the AWS region where the bootstrap cluster will be created.
	Region string
	// NumWorkers is the number of worker nodes in the bootstrap cluster.
	NumWorkers int
	// Images is a list of container images to be validated.
	Images []clusterctl.ContainerImage
}

type CreateEKSBootstrapClusterAndValidateImagesInputResult struct {
	// BootstrapClusterProvider manages provisioning of the the bootstrap cluster to be used for the e2e tests.
	BootstrapClusterProvider bootstrap.ClusterProvider
}

// CreateEKSBootstrapClusterAndValidateImages is a function that creates an EKS bootstrap cluster and validates images.
// It expects the required input parameters to be non-nil.
func CreateEKSBootstrapClusterAndValidateImages(ctx context.Context, input CreateEKSBootstrapClusterAndValidateImagesInput, res *CreateEKSBootstrapClusterAndValidateImagesInputResult) {
	Expect(ctx).ToNot(BeNil(), "Context is required for CreateEKSBootstrapClusterAndValidateImages")
	Expect(input.Name).ToNot(BeEmpty(), "Name is required for CreateEKSBootstrapClusterAndValidateImages")
	Expect(input.Version).ToNot(BeEmpty(), "Version is required for CreateEKSBootstrapClusterAndValidateImages")
	Expect(res).ToNot(BeNil(), "Result should not be nil")

	By("Checking images are present in registry")
	for _, image := range input.Images {
		turtlesframework.Byf("Checking image: %s", image.Name)
		cmdImgRes := &turtlesframework.RunCommandResult{}
		turtlesframework.RunCommand(ctx, turtlesframework.RunCommandInput{
			Command: "docker",
			Args: []string{
				"manifest",
				"inspect",
				image.Name,
			},
		}, cmdImgRes)

		Expect(cmdImgRes.Error).NotTo(HaveOccurred(), "Failed checking if image is available %s error", image.Name)
		Expect(cmdImgRes.ExitCode).To(Equal(0), "Image not found %s", image.Name)
	}

	if input.NumWorkers == 0 {
		By("Defaulting the bootstrap cluster to 1 worker node")
		input.NumWorkers = 1
	}

	By("Creating EKS bootstrap cluster")

	clusterProvider := NewEKSClusterProvider(input.Name, input.Version, input.Region, input.NumWorkers)
	clusterProvider.Create(ctx)

	res.BootstrapClusterProvider = clusterProvider
}
