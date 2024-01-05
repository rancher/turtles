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
	"os/exec"

	. "github.com/onsi/gomega"
)

// RunCommandInput is the input to RunCommand.
type RunCommandInput struct {
	Command              string
	Args                 []string
	EnvironmentVariables map[string]string
}

// RunCommandResult is the result of RunCommand.
type RunCommandResult struct {
	ExitCode int
	Stdout   []byte
	Stderr   []byte
	Error    error
}

// RunCommand will run a command with the given args and environment variables.
func RunCommand(ctx context.Context, input RunCommandInput, result *RunCommandResult) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for RunCommand")
	Expect(input.Command).ToNot(BeEmpty(), "Invalid argument. input.Command can't be empty when calling RunCommand")

	cmd := exec.Command(input.Command, input.Args...)

	for name, val := range input.EnvironmentVariables {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", name, val))
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result.Error = err
	result.Stdout = stdout.Bytes()
	result.Stderr = stdout.Bytes()
	result.ExitCode = 0

	if exitError, ok := err.(*exec.ExitError); ok {
		result.ExitCode = exitError.ExitCode()
	}
}
