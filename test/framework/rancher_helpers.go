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
	"fmt"
	"net/url"
	"os"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/conditions"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/infrastructure/container"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/log"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
)

const (
	CapiClusterOwnerLabel          = "cluster-api.cattle.io/capi-cluster-owner"
	CapiClusterOwnerNamespaceLabel = "cluster-api.cattle.io/capi-cluster-owner-ns"
	OwnedLabelName                 = "cluster-api.cattle.io/owned"
)

// RancherGetClusterKubeconfigInput represents the input parameters for getting the kubeconfig of a cluster in Rancher.
type RancherGetClusterKubeconfigInput struct {
	// ClusterProxy is the framework cluster proxy used to retrieve the kubeconfig.
	ClusterProxy framework.ClusterProxy

	// SecretName is the name of the secret containing the kubeconfig.
	SecretName string

	// Namespace is the namespace of the secret containing the kubeconfig.
	Namespace string

	// ClusterName is the name of the cluster.
	ClusterName string

	// RancherServerURL is the URL of the Rancher server.
	RancherServerURL string

	// WriteToTempFile indicates whether to write the kubeconfig to a temporary file.
	WriteToTempFile bool

	// WaitInterval is the interval to wait for the secret to be available.
	WaitInterval []interface{}
}

// RancherGetClusterKubeconfigResult represents the result of getting the kubeconfig for a Rancher cluster.
type RancherGetClusterKubeconfigResult struct {
	// KubeconfigData contains the kubeconfig data as a byte array.
	KubeconfigData []byte

	// TempFilePath is the temporary file path where the kubeconfig is stored.
	TempFilePath string
}

// RancherGetClusterKubeconfig will get the Kubeconfig for a cluster from Rancher.
func RancherGetClusterKubeconfig(ctx context.Context, input RancherGetClusterKubeconfigInput) *RancherGetClusterKubeconfigResult {
	Expect(ctx).NotTo(BeNil(), "ctx is required for RancherGetClusterKubeconfig")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.ClusterProxy can't be nil when calling RancherGetClusterKubeconfig")
	Expect(input.SecretName).ToNot(BeEmpty(), "Invalid argument. input.SecretName can't be nil when calling RancherGetClusterKubeconfig")
	Expect(input.RancherServerURL).ToNot(BeEmpty(), "Invalid argument. input.RancherServerURL can't be nil when calling RancherGetClusterKubeconfig")
	Expect(input.WaitInterval).ToNot(BeEmpty(), "Invalid argument. input.WaitInterval can't be nil when calling RancherGetClusterKubeconfig")

	if input.Namespace == "" {
		input.Namespace = DefaultNamespace
	}

	result := &RancherGetClusterKubeconfigResult{}

	Byf("Getting Rancher kubeconfig secret: %s/%s", input.Namespace, input.SecretName)
	Byf("Using Cluster %s with kubeconfig path: %s", input.ClusterProxy.GetName(), input.ClusterProxy.GetKubeconfigPath())
	secret := &corev1.Secret{}
	Eventually(func() error {
		return input.ClusterProxy.GetClient().Get(ctx, types.NamespacedName{Namespace: input.Namespace, Name: input.SecretName}, secret)
	}, input.WaitInterval...).ShouldNot(HaveOccurred(), "Getting Rancher kubeconfig secret for %s", input.SecretName)

	content, ok := secret.Data["value"]
	Expect(ok).To(BeTrue(), "Failed to find expected key in kubeconfig secret")

	By("Loading secret data into kubeconfig")

	cfg, err := clientcmd.Load(content)
	Expect(err).ShouldNot(HaveOccurred(), "Failed to unmarshall data into kubeconfig")

	clusterName := cfg.Contexts[cfg.CurrentContext].Cluster
	cluster := cfg.Clusters[clusterName]

	serverURL, err := url.Parse(cluster.Server)
	Expect(err).ShouldNot(HaveOccurred(), "Failed to parse server URL")

	serverURL.Host = input.RancherServerURL
	cluster.Server = serverURL.String()

	content, err = clientcmd.Write(*cfg)
	Expect(err).NotTo(HaveOccurred(), "Failed to save updated kubeconfig")

	result.KubeconfigData = content

	if !input.WriteToTempFile {
		return result
	}

	tempFile, err := os.CreateTemp("", "kubeconfig")
	Expect(err).NotTo(HaveOccurred(), "Failed to create temp file for kubeconfig")

	Byf("Writing updated kubeconfig to temp file %s", tempFile.Name())

	err = clientcmd.WriteToFile(*cfg, tempFile.Name())
	Expect(err).ShouldNot(HaveOccurred(), "Failed to write kubeconfig to file %s", tempFile.Name())

	result.TempFilePath = tempFile.Name()
	return result
}

// RancherGetOriginalKubeconfig will get the unmodified Kubeconfig for a cluster from Rancher.
func RancherGetOriginalKubeconfig(ctx context.Context, input RancherGetClusterKubeconfigInput) *RancherGetClusterKubeconfigResult {
	Expect(ctx).NotTo(BeNil(), "ctx is required for RancherGetOriginalKubeconfig")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.ClusterProxy can't be nil when calling RancherGetOriginalKubeconfig")
	Expect(input.SecretName).ToNot(BeEmpty(), "Invalid argument. input.SecretName can't be nil when calling RancherGetOriginalKubeconfig")

	if input.Namespace == "" {
		input.Namespace = DefaultNamespace
	}

	result := &RancherGetClusterKubeconfigResult{}

	Byf("Getting Original Rancher kubeconfig secret: %s/%s", input.Namespace, input.SecretName)
	secret := &corev1.Secret{}

	err := input.ClusterProxy.GetClient().Get(ctx, types.NamespacedName{Namespace: input.Namespace, Name: input.SecretName}, secret)
	Expect(err).ShouldNot(HaveOccurred(), "Getting Rancher kubeconfig secret for %s", input.SecretName)

	content, ok := secret.Data["value"]
	Expect(ok).To(BeTrue(), "Failed to find expected key in kubeconfig secret")

	By("Loading secret data into kubeconfig")

	cfg, err := clientcmd.Load(content)
	Expect(err).ShouldNot(HaveOccurred(), "Failed to unmarshall data into kubeconfig")

	// if we are on mac and the cluster is a DockerCluster, it is required to fix the control plane address
	// by using localhost:load-balancer-host-port instead of the address used in the docker network.
	if runtime.GOOS == "darwin" && input.isDockerCluster(ctx) {
		fixConfig(ctx, input.SecretName, cfg)
	}

	content, err = clientcmd.Write(*cfg)
	Expect(err).NotTo(HaveOccurred(), "Failed to save original kubeconfig")

	result.KubeconfigData = content

	if !input.WriteToTempFile {
		return result
	}

	tempFile, err := os.CreateTemp("", "kubeconfig-original")
	Expect(err).NotTo(HaveOccurred(), "Failed to create temp file for original kubeconfig")

	Byf("Writing original kubeconfig to temp file %s", tempFile.Name())

	err = clientcmd.WriteToFile(*cfg, tempFile.Name())
	Expect(err).ShouldNot(HaveOccurred(), "Failed to write kubeconfig to file %s", tempFile.Name())

	result.TempFilePath = tempFile.Name()
	return result
}

func (i *RancherGetClusterKubeconfigInput) isDockerCluster(ctx context.Context) bool {
	cluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Name:      i.ClusterName,
		Namespace: i.Namespace,
	}

	Eventually(func() error {
		return i.ClusterProxy.GetClient().Get(ctx, key, cluster)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to get %s", key)

	return cluster.Spec.InfrastructureRef.Kind == "DockerCluster"
}

func fixConfig(ctx context.Context, name string, config *api.Config) {
	containerRuntime, err := container.NewDockerClient()
	Expect(err).ToNot(HaveOccurred(), "Failed to get Docker runtime client")
	ctx = container.RuntimeInto(ctx, containerRuntime)

	lbContainerName := name + "-lb"

	// Check if the container exists locally.
	filters := container.FilterBuilder{}
	filters.AddKeyValue("name", lbContainerName)
	containers, err := containerRuntime.ListContainers(ctx, filters)
	Expect(err).ToNot(HaveOccurred())
	if len(containers) == 0 {
		// Return without changing the config if the container does not exist locally.
		// Note: This is necessary when running the tests with Tilt and a remote Docker
		// engine as the lb container running on the remote Docker engine is accessible
		// under its normal address but not via 127.0.0.1.
		return
	}

	port, err := containerRuntime.GetHostPort(ctx, lbContainerName, "6443/tcp")
	Expect(err).ToNot(HaveOccurred(), "Failed to get load balancer port")

	controlPlaneURL := &url.URL{
		Scheme: "https",
		Host:   "127.0.0.1:" + port,
	}
	currentCluster := config.Contexts[config.CurrentContext].Cluster
	config.Clusters[currentCluster].Server = controlPlaneURL.String()
}

// RancherLookupUserInput represents the input for looking up a user in Rancher.
type RancherLookupUserInput struct {
	// ClusterProxy is the cluster proxy used for communication with Rancher.
	ClusterProxy framework.ClusterProxy

	// Username is the username of the user to look up.
	Username string
}

// RancherLookupUserResult represents the result of a user lookup in Rancher.
type RancherLookupUserResult struct {
	// User is the username of the user found in Rancher.
	User string
}

func RancherLookupUser(ctx context.Context, input RancherLookupUserInput, result *RancherLookupUserResult) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for RancherLookupUser")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.ClusterProxy can't be nil when calling RancherLookupUser")
	Expect(input.Username).ToNot(BeEmpty(), "Invalid argument. input.Username can't be nil when calling RancherLookupUser")

	gvkUser := schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "User"}

	usersList := &unstructured.Unstructured{}
	usersList.SetGroupVersionKind(gvkUser)
	err := input.ClusterProxy.GetClient().List(ctx, usersList)
	Expect(err).NotTo(HaveOccurred(), "Failed to list users")

	field, ok := usersList.Object["items"]
	Expect(ok).To(BeTrue(), "Returned content is not a list")

	items, ok := field.([]interface{})
	Expect(ok).To(BeTrue(), "Returned content is not a list")
	foundUser := ""
	for _, item := range items {
		child, ok := item.(map[string]interface{})
		Expect(ok).To(BeTrue(), "items member is not an object")

		username, ok := child["username"].(string)
		if !ok {
			continue
		}

		if username != input.Username {
			continue
		}

		obj := &unstructured.Unstructured{Object: child}
		foundUser = obj.GetName()
		break
	}

	Expect(foundUser).ToNot(BeEmpty(), "Failed to find user for %s", input.Username)

	result.User = foundUser
}

// ValidateRancherClusterInput is the input for ValidateRancherCluster.
type ValidateRancherClusterInput struct {
	// BootstrapClusterProxy is the management cluster proxy.
	BootstrapClusterProxy framework.ClusterProxy

	// CAPIClusterKey is the key of the CAPI Cluster to be validated.
	CAPIClusterKey types.NamespacedName

	// CAPIClusterAnnotations are the annotations belonging to the CAPI Cluster.
	CAPIClusterAnnotations map[string]string

	// WaitRancherIntervals defines the interval used to wait on Rancher checks.
	WaitRancherIntervals []any

	// WaitKubeconfigIntervals defines the interval used to wait for the Rancher kubeconfig to be ready.
	WaitKubeconfigIntervals []any

	// RancherServerURL is the Rancher server-url setting.
	RancherServerURL string

	// SkipLatestFeatureChecks can be used to skip tests that have not been released yet and can not be tested
	// with stable versions of Turtles, for example during the chart upgrade test.
	SkipLatestFeatureChecks bool

	// NotUsingCAAPF is used to determine whether the `provisioning.cattle.io/externally-managed`
	// annotation should be present or not in an imported test cluster.
	NotUsingCAAPF bool
}

// ValidateRancherCluster performs all checks to validate the CAPI Cluster import into Rancher.
func ValidateRancherCluster(ctx context.Context, input ValidateRancherClusterInput) *managementv3.Cluster {
	By("Waiting for the rancher cluster record to appear")
	rancherClusters := &managementv3.ClusterList{}
	selectors := []client.ListOption{
		client.MatchingLabels{
			CapiClusterOwnerLabel:          input.CAPIClusterKey.Name,
			CapiClusterOwnerNamespaceLabel: input.CAPIClusterKey.Namespace,
			OwnedLabelName:                 "",
		},
	}
	Eventually(func() bool {
		Eventually(komega.List(rancherClusters, selectors...)).Should(Succeed())
		return len(rancherClusters.Items) == 1
	}, input.WaitRancherIntervals...).Should(BeTrue(), "No more than 1 Rancher Cluster should be found")
	rancherCluster := &rancherClusters.Items[0]
	Eventually(komega.Get(rancherCluster), input.WaitRancherIntervals...).Should(Succeed())

	By("Rancher cluster should have the custom description if it is provided")
	description := input.CAPIClusterAnnotations[turtlesannotations.ClusterDescriptionAnnotation]
	if description == "" {
		description = "CAPI cluster imported to Rancher"
	}
	Expect(rancherCluster.Spec.Description).To(Equal(description))

	By("Waiting for the rancher cluster to have a deployed agent")
	Eventually(func() bool {
		Eventually(komega.Get(rancherCluster), input.WaitRancherIntervals...).Should(Succeed())
		return conditions.IsTrue(rancherCluster, managementv3.ClusterConditionAgentDeployed)
	}, input.WaitRancherIntervals...).Should(BeTrue())

	By("Waiting for the rancher cluster to be ready")
	Eventually(func() bool {
		Eventually(komega.Get(rancherCluster), input.WaitRancherIntervals...).Should(Succeed())
		return conditions.IsTrue(rancherCluster, managementv3.ClusterConditionReady)
	}, input.WaitRancherIntervals...).Should(BeTrue())

	By("Rancher cluster should have the 'NoCreatorRBAC' annotation")
	Eventually(func() bool {
		Eventually(komega.Get(rancherCluster), input.WaitRancherIntervals...).Should(Succeed())
		_, found := rancherCluster.Annotations[turtlesannotations.NoCreatorRBACAnnotation]
		return found
	}, input.WaitRancherIntervals...).Should(BeTrue())

	if input.NotUsingCAAPF {
		By("Rancher cluster should not have the 'provisioning.cattle.io/externally-managed' annotation")
		Eventually(func() bool {
			Eventually(komega.Get(rancherCluster), input.WaitRancherIntervals...).Should(Succeed())
			_, found := rancherCluster.Annotations[turtlesannotations.ExternalFleetAnnotation]
			return found
		}, input.WaitRancherIntervals...).Should(BeFalse())
	} else {
		By("Rancher cluster should have the 'provisioning.cattle.io/externally-managed' annotation")
		Eventually(func() bool {
			Eventually(komega.Get(rancherCluster), input.WaitRancherIntervals...).Should(Succeed())
			_, found := rancherCluster.Annotations[turtlesannotations.ExternalFleetAnnotation]
			return found
		}, input.WaitRancherIntervals...).Should(BeTrue())
	}

	if !input.SkipLatestFeatureChecks { // Can be removed after Turtles v0.26 is used a starter for the chart upgrade test.
		By("Rancher cluster should have the 'rancher.io/imported-cluster-version-management' annotation")
		Eventually(func() bool {
			Eventually(komega.Get(rancherCluster), input.WaitRancherIntervals...).Should(Succeed())
			value, found := rancherCluster.Annotations[turtlesannotations.ImportedClusterVersionManagementAnnotation]
			return found && value == "false"
		}, input.WaitRancherIntervals...).Should(BeTrue())
	}

	By("Waiting for the CAPI cluster to be connectable using Rancher kubeconfig")
	rancherKubeconfig := RancherGetClusterKubeconfig(ctx, RancherGetClusterKubeconfigInput{
		ClusterProxy:     input.BootstrapClusterProxy,
		SecretName:       fmt.Sprintf("%s-kubeconfig", rancherCluster.Name),
		Namespace:        rancherCluster.Spec.FleetWorkspaceName,
		RancherServerURL: input.RancherServerURL,
		WriteToTempFile:  true,
		WaitInterval:     input.WaitRancherIntervals,
	})

	Eventually(func() bool {
		rancherConnectRes := &RunCommandResult{}
		RunCommand(ctx, RunCommandInput{
			Command: "kubectl",
			Args: []string{
				"--kubeconfig",
				rancherKubeconfig.TempFilePath,
				"get",
				"nodes",
				"--insecure-skip-tls-verify",
			},
		}, rancherConnectRes)

		log.FromContext(ctx).Info("kubectl stdout", "output", string(rancherConnectRes.Stdout))

		if rancherConnectRes.Error != nil || rancherConnectRes.ExitCode != 0 {
			log.FromContext(ctx).Info("kubectl error", "error", rancherConnectRes.Error, "exitCode", rancherConnectRes.ExitCode)
			log.FromContext(ctx).Info("Failed to connect to cluster using Rancher kubeconfig, retrying...")

			return false
		}

		return true
	}, input.WaitKubeconfigIntervals...).Should(BeTrue())

	return rancherCluster
}
