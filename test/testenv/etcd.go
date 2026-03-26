/*
Copyright © 2026 SUSE LLC

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
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/turtles/test/e2e"
	turtlesframework "github.com/rancher/turtles/test/framework"
)

// VerifyETCDSizeInput is the input for VerifyETCDSize
type VerifyETCDSizeInput struct {
	// ClusterName is the cluster for which the data is collected.
	// Beware that launching the script multiple times for the same cluster will override the data.
	ClusterName string

	// ContainerName is the container to use to fetch etcd credentials.
	ContainerName string

	// ETCDEndpointAddress is the address used by the etcd client
	ETCDEndpointAddress string

	// VerifyETCDDataScriptPath is the file path to the certs creation script.
	VerifyETCDDataScriptPath string `env:"ETCD_VERIFY_SCRIPT_PATH"`

	// ArtifactsFolder is the root path for the artifacts folder.
	ArtifactsFolder string `env:"ARTIFACTS_FOLDER"`

	// ManagementClusterEnvironment is the management cluster environment type.
	ManagementClusterEnvironment e2e.ManagementClusterEnvironmentType `env:"MANAGEMENT_CLUSTER_ENVIRONMENT"`
}

// VerifyETCDSize runs etcd data collection and size verification for a given cluster.
func VerifyETCDSize(ctx context.Context, input VerifyETCDSizeInput) {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).ShouldNot(BeNil(), "ctx is required for VerifyETCDSize")
	Expect(input.ClusterName).ShouldNot(BeEmpty(), "ClusterName is required for VerifyETCDSize")
	Expect(input.ArtifactsFolder).ShouldNot(BeEmpty(), "ArtifactsFolder is required for VerifyETCDSize")
	Expect(input.ManagementClusterEnvironment).ShouldNot(BeEmpty(), "ManagementClusterEnvironment is required for VerifyETCDSize")

	switch input.ManagementClusterEnvironment {
	case e2e.ManagementClusterEnvironmentEKS:
	default:
		Expect(input.ContainerName).ShouldNot(BeEmpty(), "ContainerName is required for VerifyETCDSize")
		Expect(input.ETCDEndpointAddress).ShouldNot(BeEmpty(), "ETCDEndpointAddress is required for VerifyETCDSize")
		Expect(input.VerifyETCDDataScriptPath).ShouldNot(BeEmpty(), "VerifyETCDDataScriptPath is required for VerifyETCDSize")
	}

	switch input.ManagementClusterEnvironment {
	case e2e.ManagementClusterEnvironmentEKS:
		GinkgoWriter.Println("ETCD size verification not yet supported for EKS")
		return
	default:
		verityETCDCmd := exec.Command(input.VerifyETCDDataScriptPath)
		verityETCDCmd.Env = append(os.Environ(),
			"OUTPUT_DIR="+input.ArtifactsFolder+"/etcd",
			"CLUSTER_NAME="+input.ClusterName,
			"CONTROL_PLANE_CONTAINER_NAME="+input.ContainerName,
			"ETCD_ENDPOINT_ADDRESS="+input.ETCDEndpointAddress,
		)
		out, err := verityETCDCmd.CombinedOutput()
		GinkgoWriter.Printf("%s\n", out)
		Expect(err).ShouldNot(HaveOccurred(), "ETCD verification failed")
	}
}
