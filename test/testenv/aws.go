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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/cluster-api/test/framework"

	turtlesframework "github.com/rancher/turtles/test/framework"
)

// CreateECRCredsInput represents the input parameters for creating ECR credentials.
type CreateECRCredsInput struct {
	// BootstrapClusterProxy is the cluster proxy used for bootstrapping.
	BootstrapClusterProxy framework.ClusterProxy

	// Name is the name of the ECR credentials.
	Name string

	// Account is the AWS account associated with the ECR credentials.
	Account string

	// Region is the AWS region where the ECR credentials are created.
	Region string

	// Namespace is the Kubernetes namespace where the ECR credentials are stored.
	Namespace string
}

// CreateECRCreds is a function that creates ECR credentials for a given input. It expects the required input parameters to be non-nil.
func CreateECRCreds(ctx context.Context, input CreateECRCredsInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for CreateECRCreds")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for CreateECRCreds")
	Expect(input.Name).ToNot(BeEmpty(), "Name is required for CreateECRCreds")
	Expect(input.Namespace).ToNot(BeEmpty(), "Namespace is required for CreateECRCreds")
	Expect(input.Account).ToNot(BeEmpty(), "Account is required for CreateECRCreds")
	Expect(input.Region).ToNot(BeEmpty(), "Region is required for CreateECRCreds")

	By("Getting password for ECR")
	cmdPwdRes := &turtlesframework.RunCommandResult{}
	turtlesframework.RunCommand(ctx, turtlesframework.RunCommandInput{
		Command: "aws",
		Args: []string{
			"ecr",
			"get-login-password",
		},
	}, cmdPwdRes)
	Expect(cmdPwdRes.Error).NotTo(HaveOccurred(), "Failed getting ecr password")
	Expect(cmdPwdRes.ExitCode).To(Equal(0), "Getting password return non-zero exit code")
	ecrPassword := string(cmdPwdRes.Stdout)

	server := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", input.Account, input.Region)
	turtlesframework.Byf("Creating ECR image pull secret for %s", server)

	turtlesframework.CreateDockerRegistrySecret(ctx, turtlesframework.CreateDockerRegistrySecretInput{
		Name:                  input.Name,
		Namespace:             input.Namespace,
		BootstrapClusterProxy: input.BootstrapClusterProxy,
		DockerServer:          server,
		DockerUsername:        "AWS",
		DockerPassword:        ecrPassword,
	})
}
