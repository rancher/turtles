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
	"bytes"
	"context"
	"html/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"

	turtlesframework "github.com/rancher/turtles/test/framework"
)

// CAPIOperatorDeployProviderInput represents the input parameters for deploying a CAPI operator provider.
type CAPIOperatorDeployProviderInput struct {
	// E2EConfig is the configuration for end-to-end testing.
	E2EConfig *clusterctl.E2EConfig

	// BootstrapClusterProxy is the proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// CAPIProvidersSecretsYAML is the YAML representation of the secrets for the CAPI providers.
	CAPIProvidersSecretsYAML [][]byte

	// CAPIProvidersYAML is the YAML representation of the CAPI providers.
	CAPIProvidersYAML []byte

	// TemplateData is the data used for templating.
	TemplateData TemplateData

	// WaitDeploymentsReadyInterval is the interval for waiting for deployments to be ready.
	WaitDeploymentsReadyInterval []interface{} `envDefault:"15m,10s"`

	// WaitForDeployments is the list of deployments to wait for.
	WaitForDeployments []NamespaceName
}

// TemplateData contains secret variables used for templating
type TemplateData struct {
	// AWSEncodedCredentials is the AWS credentials
	AWSEncodedCredentials string `env:"CAPA_ENCODED_CREDS"`
	// GCPEncodedCredentials is the GCP credentials
	GCPEncodedCredentials string `env:"CAPG_ENCODED_CREDS"`
}

type NamespaceName struct {
	Name      string
	Namespace string
}

// CAPIOperatorDeployProvider deploys the CAPI operator providers.
// It expects the required input parameters to be non-nil.
// It iterates over the CAPIProvidersSecretsYAML and applies them. Then, it applies the CAPI operator providers.
// If there are no deployments to wait for, the function returns. Otherwise, it waits for the provider deployments to be ready.
func CAPIOperatorDeployProvider(ctx context.Context, input CAPIOperatorDeployProviderInput) {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for CAPIOperatorDeployProvider")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for CAPIOperatorDeployProvider")
	Expect(input.CAPIProvidersYAML).ToNot(BeNil(), "CAPIProvidersYAML is required for CAPIOperatorDeployProvider")

	for _, secret := range input.CAPIProvidersSecretsYAML {
		secret := secret
		By("Adding CAPI Operator variables secret")

		Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
			Proxy:    input.BootstrapClusterProxy,
			Template: getFullProviderVariables(string(secret), input.TemplateData),
		})).To(Succeed(), "Failed to apply secret for capi providers")
	}

	By("Adding CAPI Operator providers")
	Expect(turtlesframework.Apply(ctx, input.BootstrapClusterProxy, input.CAPIProvidersYAML)).To(Succeed(), "Failed to add CAPI operator providers")

	if len(input.WaitForDeployments) == 0 {
		By("No deployments to wait for")

		return
	}

	By("Waiting for provider deployments to be ready")
	Expect(input.WaitDeploymentsReadyInterval).ToNot(BeNil(), "WaitDeploymentsReadyInterval is required when waiting for deployments")

	for _, nn := range input.WaitForDeployments {
		turtlesframework.Byf("Waiting for CAPI deployment %s/%s to be available", nn.Namespace, nn.Name)
		framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
			Getter: input.BootstrapClusterProxy.GetClient(),
			Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      nn.Name,
				Namespace: nn.Namespace,
			}},
		}, input.WaitDeploymentsReadyInterval...)
	}
}

func getFullProviderVariables(operatorTemplate string, data TemplateData) []byte {
	Expect(turtlesframework.Parse(&data)).To(Succeed(), "Failed to parse environment variables")

	t := template.New("capi-operator")
	t, err := t.Parse(operatorTemplate)
	Expect(err).ShouldNot(HaveOccurred(), "Failed to parse template")

	var renderedTemplate bytes.Buffer
	err = t.Execute(&renderedTemplate, data)
	Expect(err).NotTo(HaveOccurred(), "Failed to execute template")

	return renderedTemplate.Bytes()
}
