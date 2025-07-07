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
	"sigs.k8s.io/controller-runtime/pkg/log"

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

	By("Tearing down the management cluster")
	if input.BootstrapClusterProxy != nil {
		input.BootstrapClusterProxy.Dispose(ctx)
	}
	if input.BootstrapClusterProvider != nil {
		input.BootstrapClusterProvider.Dispose(ctx)
	}
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

	// Secrets is the set of secret keys to exclude from output
	Secrets []string

	// SecretKeyList is the list of secret keys to exclude from output separated with ","
	SecretKeyList []string `env:"SECRET_KEYS"`
}

// CollectArtifacts collects artifacts using the provided kubeconfig path, name, and additional arguments.
// It returns an error if the kubeconfig path is empty or if there is an error running the kubectl command.
func CollectArtifacts(ctx context.Context, input CollectArtifactsInput) error {
	log := log.FromContext(ctx)

	if err := turtlesframework.Parse(&input); err != nil {
		return err
	}

	kubeconfig := cmp.Or(input.KubeconfigPath, input.BootstrapKubeconfigPath)
	if kubeconfig == "" {
		log.Info("No kubeconfig provided, skipping artifacts collection")
		return nil
	}

	path := path.Join(input.ArtifactsFolder, input.BootstrapClusterName, input.Path)
	aargs := append([]string{"crust-gather", "collect", "--kubeconfig", kubeconfig, "-f", path, "-v", "ERROR"}, input.Args...)
	for _, secret := range input.Secrets {
		aargs = append(aargs, "-s", secret)
	}
	for _, secret := range input.SecretKeyList {
		aargs = append(aargs, "-s", secret)
	}

	cmd := exec.Command("kubectl", aargs...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = time.Minute

	log.Info("Running kubectl:", "command", strings.Join(aargs, " "))
	err := cmd.Run()
	log.Info("stderr:", "stderr", string(stderr.Bytes()))
	log.Info("stdout:", "stdout", string(stdout.Bytes()))
	return err
}

func DumpBootstrapCluster(ctx context.Context) {
	err := CollectArtifacts(ctx, CollectArtifactsInput{})
	if err != nil {
		fmt.Printf("Failed to artifacts for the bootstrap cluster: %v\n", err)
		return
	}
}
