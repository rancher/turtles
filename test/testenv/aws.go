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

type CreateECRCredsInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	Name                  string
	Account               string
	Region                string
	Namespace             string
}

func CreateECRCreds(ctx context.Context, input CreateECRCredsInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for CreateECRCreds")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for CreateECRCreds")
	Expect(input.Name).ToNot(BeEmpty(), "Name is required for CreatECRCreds")
	Expect(input.Namespace).ToNot(BeEmpty(), "Namespace is required for CreatECRCreds")
	Expect(input.Account).ToNot(BeEmpty(), "Account is required for CreatECRCreds")
	Expect(input.Region).ToNot(BeEmpty(), "Region is required for CreatECRCreds")

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
