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
	"net/url"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cluster-api/test/framework"
)

// RancherGetClusterKubeconfigInput is the input to RancherGetClusterKubeconfig.
type RancherGetClusterKubeconfigInput struct {
	Getter           framework.Getter
	SecretName       string
	Namespace        string
	RancherServerURL string
	WriteToTempFile  bool
}

// RancherGetClusterKubeconfigResult is the result of RancherGetClusterKubeconfig.
type RancherGetClusterKubeconfigResult struct {
	KubeconfigData []byte
	TempFilePath   string
}

// RancherGetClusterKubeconfig will get the Kubeconfig for a cluster from Rancher.
func RancherGetClusterKubeconfig(ctx context.Context, input RancherGetClusterKubeconfigInput, result *RancherGetClusterKubeconfigResult) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for RancherGetClusterKubeconfig")
	Expect(input.Getter).ToNot(BeNil(), "Invalid argument. input.Getter can't be nil when calling RancherGetClusterKubeconfig")
	Expect(input.SecretName).ToNot(BeEmpty(), "Invalid argument. input.SecretName can't be nil when calling RancherGetClusterKubeconfig")
	Expect(input.RancherServerURL).ToNot(BeEmpty(), "Invalid argument. input.RancherServerURL can't be nil when calling RancherGetClusterKubeconfig")

	if input.Namespace == "" {
		input.Namespace = DefaultNamespace
	}

	By("Getting Rancher kubeconfig secret")
	secret := &corev1.Secret{}

	err := input.Getter.Get(ctx, types.NamespacedName{Namespace: input.Namespace, Name: input.SecretName}, secret)
	Expect(err).ShouldNot(HaveOccurred(), "Getting Rancher kubeconfig secret %s", input.SecretName)

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
		return
	}

	tempFile, err := os.CreateTemp("", "kubeconfig")
	Expect(err).NotTo(HaveOccurred(), "Failed to create temp file for kubeconfig")

	Byf("Writing updated kubeconfig to temp file %s", tempFile.Name())

	err = clientcmd.WriteToFile(*cfg, tempFile.Name())
	Expect(err).ShouldNot(HaveOccurred(), "Failed to write kubeconfig to file %s", tempFile.Name())

	result.TempFilePath = tempFile.Name()
}

type RancherLookupUserInput struct {
	ClusterProxy framework.ClusterProxy
	Username     string
}

type RancherLookupUserResult struct {
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
