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

package v2prov

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/drone/envsubst/v2"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	provisioningv1 "github.com/rancher/turtles/api/rancher/provisioning/v1"
	"github.com/rancher/turtles/test/e2e"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"
)

var _ = Describe("[v2prov] [Azure] Creating a cluster with v2prov should still work", Label(e2e.FullTestLabel), func() {
	var (
		specName          = "v2prov"
		rancherKubeconfig *turtlesframework.RancherGetClusterKubeconfigResult
		clusterName       string
		rancherCluster    *provisioningv1.Cluster
	)

	BeforeEach(func() {
		komega.SetClient(bootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)

		rancherKubeconfig = new(turtlesframework.RancherGetClusterKubeconfigResult)
		clusterName = "az-cluster1-v2prov"
	})

	It("Should create a RKE2 cluster in Azure", func() {
		azSubId := e2eConfig.GetVariableOrEmpty(e2e.AzureSubIDVar)
		Expect(azSubId).ToNot(BeEmpty(), "Azure Subscription ID is required")
		azClientId := e2eConfig.GetVariableOrEmpty(e2e.AzureClientIDVar)
		Expect(azSubId).ToNot(BeEmpty(), "Azure Client ID is required")
		azClientSecret := e2eConfig.GetVariableOrEmpty(e2e.AzureClientSecretVar)
		Expect(azSubId).ToNot(BeEmpty(), "Azure Client Secret is required")

		rke2Version := e2eConfig.GetVariableOrEmpty(e2e.RKE2V2ProvVersionVar)
		Expect(rke2Version).ToNot(BeEmpty(), "RKE2 version is required")

		credsSecretName := "cc-test99"
		credsName := "az-ecm"
		poolName := "az-test-pool"

		lookupResult := &turtlesframework.RancherLookupUserResult{}
		turtlesframework.RancherLookupUser(ctx, turtlesframework.RancherLookupUserInput{
			Username:     "admin",
			ClusterProxy: bootstrapClusterProxy,
		}, lookupResult)

		turtlesframework.CreateSecret(ctx, turtlesframework.CreateSecretInput{
			Creator:   bootstrapClusterProxy.GetClient(),
			Name:      credsSecretName,
			Namespace: "cattle-global-data",
			Type:      corev1.SecretTypeOpaque,
			Data: map[string]string{
				"azurecredentialConfig-clientId":       azClientId,
				"azurecredentialConfig-clientSecret":   azClientSecret,
				"azurecredentialConfig-environment":    "AzurePublicCloud",
				"azurecredentialConfig-subscriptionId": azSubId,
				"azurecredentialConfig-tenantId":       "",
			},
			Annotations: map[string]string{
				"field.cattle.io/name":          credsName,
				"provisioning.cattle.io/driver": "azure",
				"field.cattle.io/creatorId":     lookupResult.User,
			},
			Labels: map[string]string{
				"cattle.io/creator": "norman",
			},
		})

		rkeConfig, err := envsubst.Eval(string(e2e.V2ProvAzureRkeConfig), func(s string) string {
			switch s {
			case "POOL_NAME":
				return poolName
			case "USER":
				return lookupResult.User
			default:
				return os.Getenv(s)
			}
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(turtlesframework.Apply(ctx, bootstrapClusterProxy, []byte(rkeConfig))).To(Succeed(), "Failed apply Digital Ocean RKE config")

		cluster, err := envsubst.Eval(string(e2e.V2ProvAzureCluster), func(s string) string {
			switch s {
			case "CLUSTER_NAME":
				return clusterName
			case "USER":
				return lookupResult.User
			case "CREDENTIAL_SECRET":
				return fmt.Sprintf("cattle-global-data:%s", credsSecretName)
			case "KUBERNETES_VERSION":
				return rke2Version
			case "AZ_CONFIG_NAME":
				return poolName
			default:
				return os.Getenv(s)
			}
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(turtlesframework.Apply(ctx, bootstrapClusterProxy, []byte(cluster))).To(Succeed(), "Failed apply Digital Ocean cluster config")

		By("Waiting for the rancher cluster record to appear")
		rancherCluster = &provisioningv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Namespace: "fleet-default",
			Name:      clusterName,
		}}
		Eventually(komega.Get(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())

		By("Waiting for the rancher cluster to have a deployed agent")
		Eventually(komega.Object(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-v2prov-create")...).Should(HaveField("Status.AgentDeployed", BeTrue()))

		By("Waiting for the rancher cluster to be ready")
		Eventually(komega.Object(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(HaveField("Status.Ready", BeTrue()))

		By("Waiting for the CAPI cluster to be connectable using Rancher kubeconfig")
		turtlesframework.RancherGetClusterKubeconfig(ctx, turtlesframework.RancherGetClusterKubeconfigInput{
			ClusterProxy:     bootstrapClusterProxy,
			SecretName:       fmt.Sprintf("%s-kubeconfig", rancherCluster.Name),
			Namespace:        rancherCluster.Namespace,
			RancherServerURL: hostName,
			WriteToTempFile:  true,
			WaitInterval:     e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher"),
		}, rancherKubeconfig)

		rancherConnectRes := &turtlesframework.RunCommandResult{}
		turtlesframework.RunCommand(ctx, turtlesframework.RunCommandInput{
			Command: "kubectl",
			Args: []string{
				"--kubeconfig",
				rancherKubeconfig.TempFilePath,
				"get",
				"nodes",
				"--insecure-skip-tls-verify",
			},
		}, rancherConnectRes)
		Expect(rancherConnectRes.Error).NotTo(HaveOccurred(), "Failed getting nodes with Rancher Kubeconfig")
		Expect(rancherConnectRes.ExitCode).To(Equal(0), "Getting nodes return non-zero exit code")
	})

	AfterEach(func() {
		By(fmt.Sprintf("Collecting artifacts for %s/%s", clusterName, specName))

		testenv.TryCollectArtifacts(ctx, testenv.CollectArtifactsInput{
			Path:                    clusterName + "-bootstrap-" + specName,
			BootstrapKubeconfigPath: bootstrapClusterProxy.GetKubeconfigPath(),
		})

		testenv.TryCollectArtifacts(ctx, testenv.CollectArtifactsInput{
			KubeconfigPath: rancherKubeconfig.TempFilePath,
			Path:           clusterName + "-workload-" + specName,
		})

		By("Deleting cluster from Rancher")
		err := bootstrapClusterProxy.GetClient().Delete(ctx, rancherCluster)
		Expect(err).NotTo(HaveOccurred(), "Failed to delete rancher cluster")

		By("Waiting for the rancher cluster record to be removed")
		Eventually(komega.Get(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-azure-delete")...).Should(MatchError(ContainSubstring("not found")), "Rancher cluster should be deleted")
	})
})
