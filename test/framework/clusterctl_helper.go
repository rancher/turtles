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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"

	. "github.com/onsi/gomega"
)

// ClusterctlGenerateFromTemplateInput is the input to ClusterctlGenerateFromTemplate.
type ClusterctlGenerateFromTemplateInput struct {
	ClusterName          string
	TemplatePath         string
	OutputFilePath       string
	ClusterCtlBinaryPath string
	EnvironmentVariables map[string]string
}

// ClusterctlGenerateFromTemplate will generate a cluster definition from a given template.
func ClusterctlGenerateFromTemplate(ctx context.Context, input ClusterctlGenerateFromTemplateInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for ClusterctlGenerateFromTemplate")
	Expect(input.TemplatePath).To(BeAnExistingFile(), "Invalid argument. input.Template must be an existing file when calling ClusterctlGenerateFromTemplate")
	Expect(input.ClusterCtlBinaryPath).To(BeAnExistingFile(), "Invalid argument. input.ClusterCtlBinaryPath must be an existing file when calling ClusterctlGenerateFromTemplate")
	Expect(input.OutputFilePath).ToNot(BeEmpty(), "Invalid argument. input.OutputFilePath must not be empty when calling ClusterctlGenerateFromTemplate")
	Expect(input.ClusterName).ToNot(BeEmpty(), "Invalid argument. input.ClusterName must not be empty when calling ClusterctlGenerateFromTemplate")

	args := []string{
		"generate",
		"yaml",
		"--from",
		input.TemplatePath,
	}

	cmd := exec.Command(input.ClusterCtlBinaryPath, args...)

	if _, ok := input.EnvironmentVariables["CLUSTER_NAME"]; !ok {
		input.EnvironmentVariables["CLUSTER_NAME"] = input.ClusterName
	}

	for name, val := range input.EnvironmentVariables {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", name, val))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed executing clusterctl generate: %s", stderr.String()))

	err = os.WriteFile(input.OutputFilePath, stdout.Bytes(), os.ModePerm)
	Expect(err).NotTo(HaveOccurred(), "Failed writing template to file")
}
