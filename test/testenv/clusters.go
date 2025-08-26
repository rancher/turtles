/*
Copyright © 2023 - 2025 SUSE LLC

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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesframework "github.com/rancher/turtles/test/framework"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

// CreateClusterInput represents the input parameters for creating a cluster.
type CreateClusterInput struct {
	// BootstrapClusterProxy is the cluster proxy for bootstrapping.
	BootstrapClusterProxy framework.ClusterProxy
	// Namespace is the namespace in which the cluster will be created.
	Namespace string
	// ClusterName is the name of the cluster to be deployed.
	ClusterName string
	// ClusterTemplate is the template of the cluster to be deployed.
	ClusterTemplate []byte
	// AdditionalTemplateVariables is a map of additional variables to be used in the cluster template.
	AdditionalTemplateVariables map[string]string
	// AdditionalFleetGitRepos is a list of additional Fleet Git repositories to be created before applying the cluster template.
	// This is useful for deploying additional resources or configurations that the cluster template depends on.
	AdditionalFleetGitRepos []turtlesframework.FleetCreateGitRepoInput
	// WaitForCreatedCluster is the duration to wait for the cluster to be created and control plane to be ready.
	WaitForCreatedCluster []interface{}
}

func CreateCluster(ctx context.Context, input CreateClusterInput) {
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for CreateCluster")
	Expect(input.Namespace).ToNot(BeNil(), "Namespace is required for CreateCluster")
	Expect(input.ClusterName).ToNot(BeNil(), "ClusterName is required for CreateCluster")
	Expect(input.ClusterTemplate).ToNot(BeNil(), "ClusterTemplate is required for CreateCluster")
	Expect(input.WaitForCreatedCluster).ToNot(BeNil(), "WaitForCreatedCluster is required for CreateCluster")

	By("Creating cluster")
	for _, gitRepo := range input.AdditionalFleetGitRepos {
		if gitRepo.ClusterProxy == nil {
			gitRepo.ClusterProxy = input.BootstrapClusterProxy
		}
		if gitRepo.TargetNamespace == "" {
			gitRepo.TargetNamespace = input.Namespace
		}
		turtlesframework.FleetCreateGitRepo(ctx, gitRepo)
	}

	Eventually(func() error {
		envVars := map[string]string{
			"CLUSTER_NAME":                input.ClusterName,
			"NAMESPACE":                   input.Namespace,
			"TOPOLOGY_NAMESPACE":          input.Namespace,
			"CONTROL_PLANE_MACHINE_COUNT": "1",
			"WORKER_MACHINE_COUNT":        "1",
		}
		for k, v := range input.AdditionalTemplateVariables {
			envVars[k] = v
		}

		return turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
			Template:                      input.ClusterTemplate,
			AddtionalEnvironmentVariables: envVars,
			Proxy:                         input.BootstrapClusterProxy,
		})
	}).WithTimeout(2 * time.Minute).WithPolling(2 * time.Second).To(Succeed())

	By("Waiting for cluster control plane to be ready")
	capiCluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{
		Namespace: input.Namespace,
		Name:      input.ClusterName,
	}}
	Eventually(komega.Object(capiCluster), input.WaitForCreatedCluster...).Should(HaveField("Status.ControlPlaneReady", BeTrue()))

	By("Waiting for cluster machines to be ready")
	capiframework.WaitForClusterMachinesReady(ctx, capiframework.WaitForClusterMachinesReadyInput{
		GetLister:  input.BootstrapClusterProxy.GetClient(),
		NodeGetter: input.BootstrapClusterProxy.GetWorkloadCluster(ctx, capiCluster.Namespace, capiCluster.Name).GetClient(),
		Cluster:    capiCluster,
	}, input.WaitForCreatedCluster...)
	By(fmt.Sprintf("Cluster %s/%s is ready", capiCluster.Namespace, capiCluster.Name))
}
