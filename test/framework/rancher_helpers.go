/*
Copyright 2023 SUSE.

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/cluster-api/test/framework"
)

// RancherGetClusterKubeconfigInput is the input to RancherGetClusterKubeconfig.
type RancherGetClusterKubeconfigInput struct {
	Getter           framework.Getter
	ClusterName      string
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
	Expect(input.ClusterName).ToNot(BeEmpty(), "Invalid argument. input.ClusterName can't be nil when calling RancherGetClusterKubeconfig")
	Expect(input.RancherServerURL).ToNot(BeEmpty(), "Invalid argument. input.RancherServerURL can't be nil when calling RancherGetClusterKubeconfig")

	if input.Namespace == "" {
		input.Namespace = DefaultNamespace
	}

	By("Getting Rancher kubeconfig secret")

	kubeConfigSecretName := fmt.Sprintf("%s-capi-kubeconfig", input.ClusterName)

	secret := &corev1.Secret{}

	err := input.Getter.Get(ctx, types.NamespacedName{Namespace: input.Namespace, Name: kubeConfigSecretName}, secret)
	Expect(err).ShouldNot(HaveOccurred(), "Getting Rancher kubeconfig secret %s", kubeConfigSecretName)

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

	Byf("Updated kubeconfig with new server-url of %s", cluster.Server)

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
