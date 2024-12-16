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
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/rancher/turtles/test/e2e"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"

	turtlesframework "github.com/rancher/turtles/test/framework"
)

// SetupTestClusterInput represents the input parameters for setting up a test cluster.
type SetupTestClusterInput struct {
	// UseExistingCluster specifies whether to use an existing cluster or create a new one.
	UseExistingCluster bool

	// E2EConfig is the configuration for end-to-end testing.
	E2EConfig *clusterctl.E2EConfig

	// ClusterctlConfigPath is the path to the clusterctl configuration file.
	ClusterctlConfigPath string

	// Scheme is the runtime scheme.
	Scheme *runtime.Scheme

	// ArtifactFolder is the folder where artifacts are stored.
	ArtifactFolder string

	// KubernetesVersion is the version of Kubernetes to use.
	KubernetesVersion string

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string

	// CustomClusterProvider is a custom cluster provider.
	CustomClusterProvider CustomClusterProvider
}

type SetupTestClusterResult struct {
	// BootstrapClusterProvider manages provisioning of the the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	BootstrapClusterProvider bootstrap.ClusterProvider

	// BootstrapClusterProxy allows to interact with the bootstrap cluster to be used for the e2e tests.
	BootstrapClusterProxy framework.ClusterProxy

	// BootstrapClusterLogFolder is the log folder for the cluster
	BootstrapClusterLogFolder string

	// IsolatedHostName is the hostname to use for Rancher in isolated mode
	IsolatedHostName string

	ClusterName    string
	KubeconfigPath string
}

// SetupTestCluster sets up a test cluster for running tests.
// It expects the required input parameters to be non-nil.
func SetupTestCluster(ctx context.Context, input SetupTestClusterInput) *SetupTestClusterResult {
	Expect(ctx).NotTo(BeNil(), "ctx is required for setupTestCluster")
	Expect(input.E2EConfig).ToNot(BeNil(), "E2EConfig is required for setupTestCluster")
	Expect(input.ClusterctlConfigPath).ToNot(BeEmpty(), "ClusterctlConfigPath is required for setupTestCluster")
	Expect(input.Scheme).ToNot(BeNil(), "Scheme is required for setupTestCluster")
	Expect(input.ArtifactFolder).ToNot(BeEmpty(), "ArtifactFolder is required for setupTestCluster")
	Expect(input.KubernetesVersion).ToNot(BeEmpty(), "KubernetesVersion is required for SetupTestCluster")

	clusterName := createClusterName(input.E2EConfig.ManagementClusterName)
	result := &SetupTestClusterResult{}

	By("Setting up the bootstrap cluster")
	result.setupCluster(ctx, input.E2EConfig, input.Scheme, clusterName, input.UseExistingCluster, input.KubernetesVersion, input.CustomClusterProvider)

	if input.UseExistingCluster {
		return result
	}

	By("Create log folder for cluster")

	result.BootstrapClusterLogFolder = filepath.Join(input.ArtifactFolder, "clusters", result.BootstrapClusterProxy.GetName())
	Expect(os.MkdirAll(result.BootstrapClusterLogFolder, 0o750)).To(Succeed(), "Invalid argument. Log folder can't be created %s", result.BootstrapClusterLogFolder)

	result.IsolatedHostName = getInternalClusterHostname(ctx, result.BootstrapClusterProxy)

	return result
}

func (r *SetupTestClusterResult) setupCluster(ctx context.Context, config *clusterctl.E2EConfig, scheme *runtime.Scheme, clusterName string, useExistingCluster bool, kubernetesVersion string, customClusterProvider CustomClusterProvider) {
	var clusterProvider bootstrap.ClusterProvider
	kubeconfigPath := ""

	if !useExistingCluster {
		if customClusterProvider != nil { // if customClusterProvider is provided, use it to create the bootstrap cluster instead of kind
			clusterProvider = customClusterProvider(ctx, config, clusterName, kubernetesVersion)
		} else {
			clusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
				Name:               clusterName,
				KubernetesVersion:  kubernetesVersion,
				RequiresDockerSock: true,
				Images:             config.Images,
			})
		}

		Expect(clusterProvider).ToNot(BeNil(), "Failed to create a bootstrap cluster")

		kubeconfigPath = clusterProvider.GetKubeconfigPath()
		Expect(kubeconfigPath).To(BeAnExistingFile(), "Failed to get the kubeconfig file for the bootstrap cluster")
	}

	proxy := framework.NewClusterProxy(clusterName, kubeconfigPath, scheme, framework.WithMachineLogCollector(framework.DockerLogCollector{}))
	Expect(proxy).ToNot(BeNil(), "Cluster proxy should not be nil")

	r.ClusterName = clusterName
	r.BootstrapClusterProxy = proxy
	r.BootstrapClusterProvider = clusterProvider
	r.KubeconfigPath = kubeconfigPath
}

// getInternalClusterHostname gets the internal by setting it to the IP of the first and only node in the boostrap cluster. Labels the node with
// "ingress-ready" so that the nginx ingress controller can pick it up, required by kind. See: https://kind.sigs.k8s.io/docs/user/ingress/#create-cluster
// This hostname can be used in an environment where the cluster is isolated from the outside world and a Rancher hostname is required.
func getInternalClusterHostname(ctx context.Context, clusterProxy framework.ClusterProxy) string {
	cpNodeList := corev1.NodeList{}
	Expect(clusterProxy.GetClient().List(ctx, &cpNodeList)).To(Succeed())
	Expect(cpNodeList.Items).To(HaveLen(1))
	Expect(cpNodeList.Items[0].Status.Addresses).ToNot(BeEmpty())

	cpNode := cpNodeList.Items[0]
	Expect(cpNode.Status.Addresses).ToNot(BeEmpty())

	for _, address := range cpNode.Status.Addresses {
		if address.Type == corev1.NodeInternalIP {
			return address.Address + "." + turtlesframework.MagicDNS
		}
	}

	Fail("Expected to find IP address of the first node with ingress-ready")
	return ""
}

func createClusterName(baseName string) string {
	return fmt.Sprintf("%s-%s", baseName, util.RandomString(6))
}

// PreManagementClusterSetupResult represents the result of pre-management cluster setup.
type PreManagementClusterSetupResult struct {
	// IngressType specifies the type of ingress for the cluster.
	IngressType IngressType

	// DockerUsername is the username for accessing the Docker registry.
	DockerUsername string

	// DockerPassword is the password for accessing the Docker registry.
	DockerPassword string

	// CustomClusterProvider represents the custom cluster provider for the cluster.
	CustomClusterProvider CustomClusterProvider
}

// PreManagementClusterSetupHook is a function that performs pre-setup tasks for the management cluster.
// It expects the required input parameters to be non-nil.
// It checks the environment type and sets the Docker username, Docker password, custom cluster provider, and ingress type accordingly.
// If the environment type is e2e.ManagementClusterEnvironmentEKS, it expects the GITHUB_USERNAME and GITHUB_TOKEN environment variables to be set.
// If the environment type is e2e.ManagementClusterEnvironmentIsolatedKind, it sets the ingress type to CustomIngress.
// If the environment type is e2e.ManagementClusterEnvironmentKind, it sets the ingress type to NgrokIngress.
// If the environment type is not recognized, it fails with an error message indicating the invalid infrastructure type.
func PreManagementClusterSetupHook(e2eConfig *clusterctl.E2EConfig) PreManagementClusterSetupResult {
	output := PreManagementClusterSetupResult{}

	infrastructureType := e2e.ManagementClusterEnvironmentType(e2eConfig.GetVariable(e2e.ManagementClusterEnvironmentVar))

	switch infrastructureType {
	case e2e.ManagementClusterEnvironmentEKS:
		output.DockerUsername = os.Getenv("GITHUB_USERNAME")
		Expect(output.DockerUsername).NotTo(BeEmpty(), "Github username is required")
		output.DockerPassword = os.Getenv("GITHUB_TOKEN")
		Expect(output.DockerPassword).NotTo(BeEmpty(), "Github token is required")
		output.CustomClusterProvider = EKSBootsrapCluster
		Expect(output.CustomClusterProvider).NotTo(BeNil(), "EKS custom cluster provider is required")
		output.IngressType = EKSNginxIngress
	case e2e.ManagementClusterEnvironmentIsolatedKind:
		output.IngressType = CustomIngress
	case e2e.ManagementClusterEnvironmentKind:
		output.IngressType = NgrokIngress
	default:
		Fail(fmt.Sprintf("Invalid management cluster infrastructure type %q", infrastructureType))
	}

	return output
}
