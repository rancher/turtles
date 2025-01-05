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
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesframework "github.com/rancher/turtles/test/framework"
)

// CleanupTestClusterInput represents the input parameters for cleaning up a test cluster.
type CleanupTestClusterInput struct {
	// SetupTestClusterResult contains the result of setting up the test cluster.
	SetupTestClusterResult

	// SkipCleanup indicates whether to skip the cleanup process.
	SkipCleanup bool `env:"SKIP_RESOURCE_CLEANUP"`

	// ArtifactFolder specifies the folder where artifacts are stored.
	ArtifactFolder string `env:"ARTIFACTS_FOLDER"`
}

// CleanupTestCluster is a function that cleans up the test cluster.
// It expects the required input parameters to be non-nil.
func CleanupTestCluster(ctx context.Context, input CleanupTestClusterInput) {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for CleanupTestCluster")
	Expect(input.SetupTestClusterResult).ToNot(BeNil(), "SetupTestClusterResult is required for CleanupTestCluster")
	Expect(input.ArtifactFolder).ToNot(BeEmpty(), "ArtifactFolder is required for CleanupTestCluster")

	By("Dumping artifacts from the bootstrap cluster")
	dumpBootstrapCluster()

	if input.SkipCleanup {
		return
	}

	By("Tearing down the management cluster")
	if input.BootstrapClusterProxy != nil {
		input.BootstrapClusterProxy.Dispose(ctx)
	}
	if input.BootstrapClusterProvider != nil {
		input.BootstrapClusterProvider.Dispose(ctx)
	}
}

var secrets = []string{
	"NGROK_AUTHTOKEN",
	"NGROK_API_KEY",
	"RANCHER_HOSTNAME",
	"RANCHER_PASSWORD",
	"CAPA_ENCODED_CREDS",
	"CAPG_ENCODED_CREDS",
	"AZURE_SUBSCRIPTION_ID",
	"AZURE_CLIENT_ID",
	"AZURE_CLIENT_SECRET",
	"AZURE_TENANT_ID",
}

type CollectArtifactsInput struct {
	// BootstrapKubeconfigPath is a path to the bootstrap cluster kubeconfig
	BootstrapKubeconfigPath string `env:"BOOTSTRAP_CLUSTER_KUBECONFIG_PATH"`

	// KubeconfigPath is a path to the cluster kubeconfig
	KubeconfigPath string

	// Path parts to the collected archive
	Path string `envDefault:"bootstrap"`

	// ArtifactsFolder is the root path for the artifacts
	ArtifactsFolder string `env:"ARTIFACTS_FOLDER"`

	// BootstrapClusterName is the name of the bootstrap cluster
	BootstrapClusterName string `env:"BOOTSTRAP_CLUSTER_NAME" envDefault:"bootstrap"`

	// Args are additional args for the artifacts collection
	Args []string

	// SecretKeys is the set of secret keys to exclude from output
	SecretKeys []string
}

// CollectArtifacts collects artifacts using the provided kubeconfig path, name, and additional arguments.
// It returns an error if the kubeconfig path is empty or if there is an error running the kubectl command.
func CollectArtifacts(input CollectArtifactsInput) error {
	if err := turtlesframework.Parse(&input); err != nil {
		return err
	}

	kubeconfig := cmp.Or(input.KubeconfigPath, input.BootstrapKubeconfigPath)
	if kubeconfig == "" {
		return nil
	}

	path := path.Join(input.ArtifactsFolder, input.BootstrapClusterName, input.Path)
	aargs := append([]string{"crust-gather", "collect", "--kubeconfig", kubeconfig, "-f", path, "-v", "ERROR"}, input.Args...)
	for _, secret := range secrets {
		aargs = append(aargs, "-s", secret)
	}
	for _, secret := range input.SecretKeys {
		aargs = append(aargs, "-s", secret)
	}

	cmd := exec.Command("kubectl", aargs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = time.Minute

	fmt.Printf("Running kubectl %s\n", strings.Join(aargs, " "))
	err := cmd.Run()
	fmt.Printf("stderr:\n%s\n", string(stderr.Bytes()))
	fmt.Printf("stdout:\n%s\n", string(stdout.Bytes()))
	return err
}

func dumpBootstrapCluster() {
	err := CollectArtifacts(CollectArtifactsInput{})
	if err != nil {
		fmt.Printf("Failed to artifacts for the bootstrap cluster: %v\n", err)
		return
	}
}
