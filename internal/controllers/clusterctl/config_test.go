/*
Copyright © 2024 - 2025 SUSE LLC

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

package clusterctl

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/turtles/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("ClusterConfig Tests", func() {
	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client
	)

	BeforeEach(func() {
		ctx = context.TODO()

		// Create a new runtime scheme
		scheme = runtime.NewScheme()

		// Register corev1 types
		Expect(corev1.AddToScheme(scheme)).To(Succeed())

		// Register the v1alpha1.ClusterctlConfig type
		Expect(v1alpha1.AddToScheme(scheme)).To(Succeed())

		// Override the configDefault variable to provide custom data for the test
		configDefault = []byte(`
data:
  clusterctl.yaml: |
    providers:
      - name: core
        type: CoreProvider
        url: https://github.com/rancher-sandbox/cluster-api/releases/v1.9.4/core-components.yaml
      - name: gcp
        type: InfrastructureProvider
        url: https://github.com/rancher-sandbox/cluster-api-provider-gcp/releases/v1.8.1/infrastructure-components.yaml
      - name: rke2
        type: ControlPlaneProvider
        url: https://github.com/rancher/cluster-api-provider-rke2/releases/v0.11.0/control-plane-components.yaml
    images:
      image1:
        repository: repo1
        tag: v1
`)

		// Reinitialize the config variable
		utilruntime.Must(yaml.UnmarshalStrict(configDefault, &config))

		// Initialize the fake Kubernetes client with the scheme and initial objects
		fakeClient = fake.NewClientBuilder().
			WithScheme(scheme).
			WithObjects(
				&v1alpha1.ClusterctlConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "clusterctl-config",
						Namespace: "rancher-turtles-system",
					},
					Spec: v1alpha1.ClusterctlConfigSpec{
						Providers: []v1alpha1.Provider{
							{
								Name: "gcp",
								Type: "InfrastructureProvider",
								URL:  "https://github.com/rancher-sandbox/cluster-api-provider-gcp/releases/v1.99.99/updated-infrastructure-components.yaml",
							},
							{
								Name: "rke2",
								Type: "BootstrapProvider",
								URL:  "https://github.com/rancher/cluster-api-provider-rke2/releases/v0.11.0/bootstrap-components.yaml",
							},
							{
								Name: "fleet",
								Type: "AddonProvider",
								URL:  "https://github.com/rancher-sandbox/cluster-api-addon-provider-fleet/releases/v0.4.0/addon-components.yaml",
							},
						},
						Images: []v1alpha1.Image{
							{
								Name:       "image3",
								Repository: "repo3",
								Tag:        "v3",
							},
						},
					},
				},
			).
			Build()
	})

	It("should leave unchanged, deduplicate and add new providers correctly", func() {
		configRepo, err := ClusterConfig(ctx, fakeClient)
		Expect(err).ToNot(HaveOccurred())

		Expect(configRepo.Providers).To(HaveLen(5))

		// Ensure core provider remains unchanged
		Expect(configRepo.Providers).To(ContainElement(v1alpha1.Provider{
			Name: "core",
			Type: "CoreProvider",
			URL:  "https://github.com/rancher-sandbox/cluster-api/releases/v1.9.4/core-components.yaml",
		}))

		// Ensure gcp infrastructure provider is updated
		Expect(configRepo.Providers).To(ContainElement(v1alpha1.Provider{
			Name: "gcp",
			Type: "InfrastructureProvider",
			URL:  "https://github.com/rancher-sandbox/cluster-api-provider-gcp/releases/v1.99.99/updated-infrastructure-components.yaml",
		}))

		// Ensure rke2 controlplane provider remains unchanged
		Expect(configRepo.Providers).To(ContainElement(v1alpha1.Provider{
			Name: "rke2",
			Type: "ControlPlaneProvider",
			URL:  "https://github.com/rancher/cluster-api-provider-rke2/releases/v0.11.0/control-plane-components.yaml",
		}))

		// Ensure a new rke2 bootstrap provider is added
		Expect(configRepo.Providers).To(ContainElement(v1alpha1.Provider{
			Name: "rke2",
			Type: "BootstrapProvider",
			URL:  "https://github.com/rancher/cluster-api-provider-rke2/releases/v0.11.0/bootstrap-components.yaml",
		}))

		// Ensure a new fleet addon provider is added
		Expect(configRepo.Providers).To(ContainElement(v1alpha1.Provider{
			Name: "fleet",
			Type: "AddonProvider",
			URL:  "https://github.com/rancher-sandbox/cluster-api-addon-provider-fleet/releases/v0.4.0/addon-components.yaml",
		}))
	})
})

func TestClusterctl(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Clusterctl Suite")
}
