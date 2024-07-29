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
)

type CustomClusterProvider func(ctx context.Context, config *clusterctl.E2EConfig, clusterName, kubernetesVersion string) bootstrap.ClusterProvider

// EKSBootsrapCluster creates a new EKS bootstrap cluster and returns a ClusterProvider
func EKSBootsrapCluster(ctx context.Context, config *clusterctl.E2EConfig, clusterName, kubernetesVersion string) bootstrap.ClusterProvider {
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
