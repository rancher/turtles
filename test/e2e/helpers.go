//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/yaml"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	provisioningv1 "github.com/rancher/turtles/api/rancher/provisioning/v1"
	turtlesframework "github.com/rancher/turtles/test/framework"
	networkingv1 "k8s.io/api/networking/v1"
)

func SetupSpecNamespace(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string) (*corev1.Namespace, context.CancelFunc) {
	turtlesframework.Byf("Creating a namespace for hosting the %q test spec", specName)
	namespace, cancelWatches := framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
		Creator:   clusterProxy.GetClient(),
		ClientSet: clusterProxy.GetClientSet(),
		Name:      fmt.Sprintf("%s-%s", specName, util.RandomString(6)),
		LogFolder: filepath.Join(artifactFolder, "clusters", clusterProxy.GetName()),
	})

	return namespace, cancelWatches
}

func CreateRepoName(specName string) string {
	return fmt.Sprintf("repo-%s-%s", specName, util.RandomString(6))
}

func DumpSpecResourcesAndCleanup(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string, namespace *corev1.Namespace, cancelWatches context.CancelFunc, capiCluster *types.NamespacedName, intervalsGetter func(spec, key string) []interface{}, skipCleanup bool) {
	if !skipCleanup {
		turtlesframework.Byf("Deleting cluster %s", capiCluster)
		// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
		// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
		// instead of DeleteClusterAndWait
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			Client:    clusterProxy.GetClient(),
			Namespace: namespace.Name,
		}, intervalsGetter(specName, "wait-delete-cluster")...)

		turtlesframework.Byf("Deleting namespace used for hosting the %q test spec", specName)
		framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
			Deleter: clusterProxy.GetClient(),
			Name:    namespace.Name,
		})
	}
	cancelWatches()
}

func InitScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	framework.TryAddDefaultSchemes(scheme)
	Expect(operatorv1.AddToScheme(scheme)).To(Succeed())
	Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
	Expect(provisioningv1.AddToScheme(scheme)).To(Succeed())
	Expect(managementv3.AddToScheme(scheme)).To(Succeed())
	Expect(networkingv1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func LoadE2EConfig(configPath string) *clusterctl.E2EConfig {
	configData, err := os.ReadFile(configPath)
	Expect(err).ToNot(HaveOccurred(), "Failed to read the e2e test config file")
	Expect(configData).ToNot(BeEmpty(), "The e2e test config file should not be empty")

	config := &clusterctl.E2EConfig{}
	Expect(yaml.UnmarshalStrict(configData, config)).To(Succeed(), "Failed to convert the e2e test config file to yaml")

	config.Defaults()
	config.AbsPaths(filepath.Dir(configPath))

	return config
}

func CreateClusterctlLocalRepository(ctx context.Context, config *clusterctl.E2EConfig, repositoryFolder string) string {
	createRepositoryInput := clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: repositoryFolder,
	}

	clusterctlConfig := clusterctl.CreateRepository(ctx, createRepositoryInput)
	Expect(clusterctlConfig).To(BeAnExistingFile(), "The clusterctl config file does not exists in the local repository %s", repositoryFolder)
	return clusterctlConfig
}

func ValidateE2EConfig(config *clusterctl.E2EConfig) {
	Expect(os.MkdirAll(config.GetVariable(ArtifactsFolderVar), 0o755)).To(Succeed(), "Invalid test suite argument. Can't create artifacts folder %q", config.GetVariable(ArtifactsFolderVar))
	Expect(config.GetVariable(HelmBinaryPathVar)).To(BeAnExistingFile(), "Invalid test suite argument. HELM_BINARY_PATH should be an existing file.")
	Expect(config.GetVariable(TurtlesPathVar)).To(BeAnExistingFile(), "Invalid test suite argument. TURTLES_PATH should be an existing file.")

	_, err := strconv.ParseBool(config.GetVariable(UseExistingClusterVar))
	Expect(err).ToNot(HaveOccurred(), "Invalid test suite argument. Can't parse USE_EXISTING_CLUSTER %q", config.GetVariable(UseExistingClusterVar))

	_, err = strconv.ParseBool(config.GetVariable(SkipResourceCleanupVar))
	Expect(err).ToNot(HaveOccurred(), "Invalid test suite argument. Can't parse SKIP_RESOURCE_CLEANUP %q", config.GetVariable(SkipResourceCleanupVar))
}
