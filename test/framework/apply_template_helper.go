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
	"os"

	"github.com/drone/envsubst/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/cluster-api/test/framework"
)

// ApplyFromTemplateInput is the input to ApplyFromTemplate.
type ApplyFromTemplateInput struct {
	Getter                        func(key string) string
	Template                      []byte
	AddtionalEnvironmentVariables map[string]string

	Proxy          framework.ClusterProxy
	OutputFilePath string
}

// ApplyFromTemplate will generate a yaml definition from a given template and apply it in the cluster.
func ApplyFromTemplate(ctx context.Context, input ApplyFromTemplateInput) error {
	Expect(ctx).NotTo(BeNil(), "ctx is required for ApplyFromTemplate.")
	Expect(input.Template).ToNot(BeEmpty(), "Invalid argument. input.Template must be an existing byte array.")
	Expect(input.Getter).NotTo(BeNil(), "Getter method is required for ApplyFromTemplate. Typically an os.Getenv is enough.")
	if input.OutputFilePath == "" {
		Expect(input.Proxy).NotTo(BeNil(), "Cluster proxy is required for ApplyFromTemplate.")
	}

	// Apply environment variables in the folowing order of the precedence:
	//	1. input.AddtionalEnvironmentVariables
	//	2. input.Getter - in case of using cluster-api proxy GetVariable:
	//		1. os.Getenv
	//		2. test/e2e/config/operator.yaml variables content
	overrides := input.AddtionalEnvironmentVariables
	if overrides == nil {
		overrides = map[string]string{}
	}

	getter := func(key string) string {
		if val, ok := overrides[key]; ok {
			return val
		}
		return input.Getter(key)
	}

	template, err := envsubst.Eval(string(input.Template), getter)
	Expect(err).NotTo(HaveOccurred(), "Failed executing template generate")

	if input.OutputFilePath != "" {
		return os.WriteFile(input.OutputFilePath, []byte(template), os.ModePerm)
	}

	return input.Proxy.Apply(ctx, []byte(template))
}
