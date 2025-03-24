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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
)

// CustomClusterProvider is a function type that represents a custom cluster provider.
// It takes in a context, an E2EConfig, a cluster name, and a Kubernetes version as parameters.
// It returns a bootstrap.ClusterProvider.
type CustomClusterProvider func(ctx context.Context, config *clusterctl.E2EConfig, clusterName, kubernetesVersion string) bootstrap.ClusterProvider

// EKSBootstrapCluster is a function that creates a new EKS bootstrap cluster.
func EKSBootstrapCluster(ctx context.Context, config *clusterctl.E2EConfig, clusterName, kubernetesVersion string) bootstrap.ClusterProvider {
	By("Creating a new EKS bootstrap cluster")

	region := config.Variables["KUBERNETES_MANAGEMENT_AWS_REGION"]
	Expect(region).ToNot(BeEmpty(), "KUBERNETES_MANAGEMENT_AWS_REGION must be set in the e2e config")

	eksCreateResult := &CreateEKSBootstrapClusterAndValidateImagesInputResult{}
	CreateEKSBootstrapClusterAndValidateImages(ctx, CreateEKSBootstrapClusterAndValidateImagesInput{
		Name:       clusterName,
		Version:    kubernetesVersion,
		Region:     region,
		NumWorkers: 1,
		Images:     config.Images,
	}, eksCreateResult)

	return eksCreateResult.BootstrapClusterProvider
}

// KindBootstrapCluster is a function that creates a new kind bootstrap cluster with extra port mappings. This is useful for forwarding requests from the host.
func KindWithExtraPortMappingsBootstrapCluster(ctx context.Context, config *clusterctl.E2EConfig, clusterName, kubernetesVersion string) bootstrap.ClusterProvider {
	By("Creating a new kind bootstrap cluster with extra port mappings")

	return bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
		Name:               clusterName,
		KubernetesVersion:  kubernetesVersion,
		RequiresDockerSock: false,
		Images:             config.Images,
		ExtraPortMappings: []v1alpha4.PortMapping{
			{ContainerPort: 80, HostPort: 80, Protocol: v1alpha4.PortMappingProtocolTCP},
			{ContainerPort: 443, HostPort: 443, Protocol: v1alpha4.PortMappingProtocolTCP},
		},
	})
}
