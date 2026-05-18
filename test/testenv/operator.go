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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
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
	CAPIProvidersYAML [][]byte

	// WaitDeploymentsReadyInterval is the interval for waiting for deployments to be ready.
	WaitDeploymentsReadyInterval []interface{} `envDefault:"15m,10s"`

	// WaitForDeployments is the list of deployments to wait for.
	WaitForDeployments []types.NamespacedName

	// CustomWaiter is a slice of functions for custom waiting logic.
	CustomWaiter []func(ctx context.Context)
}

// ProviderTemplateData contains variables used for templating
type ProviderTemplateData struct {
	// ProviderVersion is the version of the provider
	ProviderVersion string
}

// Provider represents a cluster-api provider with version
type Provider struct {
	Name    string `yaml:"name"`
	Type    string `yaml:"type"`
	URL     string `yaml:"url"`
	Version string // Parsed from URL
}

// ClusterctlConfig represents the structure of clusterctl.yaml
type ClusterctlConfig struct {
	Images    map[string]interface{} `yaml:"images"`
	Providers []Provider             `yaml:"providers"`
}

// CAPIOperatorDeployProvider deploys the CAPI operator providers.
// It expects the required input parameters to be non-nil.
// It iterates over the CAPIProvidersSecretsYAML and applies them. Then, it applies the CAPI operator providers.
// If there are no deployments to wait for, the function returns. Otherwise, it waits for the provider deployments to be ready.
func CAPIOperatorDeployProvider(ctx context.Context, input CAPIOperatorDeployProviderInput) {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for CAPIOperatorDeployProvider")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for CAPIOperatorDeployProvider")
	Expect(len(input.CAPIProvidersYAML)).ShouldNot(BeZero(), "CAPIProvidersYAML must be provided")

	for _, secret := range input.CAPIProvidersSecretsYAML {
		By("Adding CAPI Operator variables secret")

		Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
			Proxy:    input.BootstrapClusterProxy,
			Template: secret,
		})).To(Succeed(), "Failed to apply secret for capi providers")
	}

	for _, provider := range input.CAPIProvidersYAML {
		By("Adding CAPI Operator provider")
		Expect(turtlesframework.Apply(ctx, input.BootstrapClusterProxy, provider)).To(Succeed(), "Failed to add CAPI operator providers")
	}

	if len(input.WaitForDeployments) == 0 {
		By("No deployments to wait for")
	} else {
		By("Waiting for provider deployments to be ready")
		Expect(input.WaitDeploymentsReadyInterval).ToNot(BeNil(), "WaitDeploymentsReadyInterval is required when waiting for deployments")

		for _, nn := range input.WaitForDeployments {
			turtlesframework.Byf("Waiting for CAPI deployment %s/%s to be available", nn.Namespace, nn.Name)
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
				Name:      nn.Name,
				Namespace: nn.Namespace,
			}}
			framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
				Getter:     input.BootstrapClusterProxy.GetClient(),
				Deployment: deployment,
			}, input.WaitDeploymentsReadyInterval...)
			Expect(input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).Should(Succeed())
			wantRepository := "registry.suse.com/rancher"
			for _, container := range deployment.Spec.Template.Spec.Containers {
				Expect(strings.HasPrefix(container.Image, wantRepository), fmt.Sprintf("Container image %s does not use expected repository %s", container.Image, wantRepository))
			}
		}
	}

	if len(input.CustomWaiter) == 0 {
		By("No custom waiters to run")
	} else {
		By("Running custom waiters")
		for _, waiter := range input.CustomWaiter {
			if waiter != nil {
				waiter(ctx)
			}
		}
	}
}

type RemoveCAPIProviderInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	ProviderName          string
	ProviderNamespace     string
}

func RemoveCAPIProvider(ctx context.Context, input RemoveCAPIProviderInput) {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for RemoveCAPIProvider")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for RemoveCAPIProvider")
	Expect(input.ProviderName).ToNot(BeEmpty(), "ProviderName is required for RemoveCAPIProvider")
	Expect(input.ProviderNamespace).ToNot(BeEmpty(), "ProviderNamespace is required for RemoveCAPIProvider")

	By("Removing CAPI Operator provider: " + input.ProviderName)

	err := input.BootstrapClusterProxy.GetClient().Delete(ctx, &turtlesv1.CAPIProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.ProviderName,
			Namespace: input.ProviderNamespace,
		},
	})

	Expect(err).ToNot(HaveOccurred(), "Failed to delete CAPI operator provider: "+input.ProviderName)
}
