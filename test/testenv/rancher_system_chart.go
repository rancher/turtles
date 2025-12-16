/*
Copyright © 2024 - 2025 SUSE LLC

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
	"os"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesframework "github.com/rancher/turtles/test/framework"

	"github.com/rancher/turtles/test/e2e"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	opframework "sigs.k8s.io/cluster-api-operator/test/framework"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateRancherDeploymentWithChartConfigInput represents the input for UpdateRancherDeploymentWithChartConfig.
type UpdateRancherDeploymentWithChartConfigInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// ChartRepoURL is the URL of the chart repository (e.g., http://gitea.address/git/charts).
	ChartRepoURL string

	// ChartRepoBranch is the branch to use from the chart repository.
	ChartRepoBranch string

	// ChartVersion is the version of the chart to install (e.g., "108.0.0+up99.99.99").
	ChartVersion string
}

// UpdateRancherDeploymentWithChartConfig updates the Rancher deployment to add environment variables
// for the system chart controller. This is used to test migration from helm-based installation
// to system chart controller based installation.
func UpdateRancherDeploymentWithChartConfig(ctx context.Context, input UpdateRancherDeploymentWithChartConfigInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for UpdateRancherDeploymentWithChartConfig")
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy is required")
	Expect(input.ChartRepoURL).NotTo(BeEmpty(), "ChartRepoURL is required")
	Expect(input.ChartRepoBranch).NotTo(BeEmpty(), "ChartRepoBranch is required")
	Expect(input.ChartVersion).NotTo(BeEmpty(), "ChartVersion is required")

	By("Patching Rancher deployment to enable system chart controller")

	// Get the Rancher deployment
	deployment := &appsv1.Deployment{}
	Expect(input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKey{
		Name:      "rancher",
		Namespace: e2e.RancherNamespace,
	}, deployment)).To(Succeed())

	// Add environment variables for system chart controller
	envVars := []corev1.EnvVar{
		{
			Name:  "CATTLE_CHART_DEFAULT_URL",
			Value: input.ChartRepoURL,
		},
		{
			Name:  "CATTLE_CHART_DEFAULT_BRANCH",
			Value: input.ChartRepoBranch,
		},
		{
			Name:  "CATTLE_RANCHER_TURTLES_VERSION",
			Value: input.ChartVersion,
		},
	}

	// Find the rancher container and add/update env vars
	for i := range deployment.Spec.Template.Spec.Containers {
		if deployment.Spec.Template.Spec.Containers[i].Name == "rancher" {
			existingEnv := deployment.Spec.Template.Spec.Containers[i].Env
			// Remove existing chart config env vars if any
			filteredEnv := []corev1.EnvVar{}
			for _, env := range existingEnv {
				if env.Name != "CATTLE_CHART_DEFAULT_URL" &&
					env.Name != "CATTLE_CHART_DEFAULT_BRANCH" &&
					env.Name != "CATTLE_RANCHER_TURTLES_VERSION" {
					filteredEnv = append(filteredEnv, env)
				}
			}
			// Add the new env vars
			deployment.Spec.Template.Spec.Containers[i].Env = append(filteredEnv, envVars...)
			break
		}
	}

	// Update the deployment
	Expect(input.BootstrapClusterProxy.GetClient().Update(ctx, deployment)).To(Succeed())
}

// UpgradeInstallRancherWithGiteaInput represents the input for UpgradeInstallRancherWithGitea.
type UpgradeInstallRancherWithGiteaInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// EnvironmentType is the environment type
	EnvironmentType e2e.ManagementClusterEnvironmentType `env:"MANAGEMENT_CLUSTER_ENVIRONMENT"`

	// HelmBinaryPath is the path to the Helm binary.
	HelmBinaryPath string `env:"HELM_BINARY_PATH"`

	// RancherChartRepoName is the repository name for Rancher chart.
	RancherChartRepoName string `env:"RANCHER_REPO_NAME"`

	// RancherChartURL is the URL for Rancher chart.
	RancherChartURL string `env:"RANCHER_URL"`

	// RancherHostname is the hostname to be used by Rancher. This depends on the result of PreRancherInstallHook and it's based on MANAGEMENT_CLUSTER_ENVIRONMENT.
	RancherHostname string `env:"RANCHER_HOSTNAME"`

	// RancherReplicas is the number of replicas for Rancher.
	RancherReplicas int `env:"RANCHER_REPLICAS" envDefault:"1"`

	// RancherIngressClassName is the class name of the Ingress used by Rancher.
	RancherIngressClassName string

	// RancherNamespace is the namespace for Rancher.
	RancherNamespace string `env:"RANCHER_NAMESPACE" envDefault:"cattle-system"`

	// RancherPatches are the patches for Rancher.
	RancherPatches [][]byte

	// RancherVersion is the version to upgrade to.
	RancherVersion string `env:"RANCHER_VERSION"`

	// ChartRepoURL is the URL of the Gitea chart repository (e.g., http://gitea.address/git/charts).
	ChartRepoURL string

	// ChartRepoBranch is the branch to use from the chart repository.
	ChartRepoBranch string

	// ChartVersion is the Turtles chart version (e.g., "108.0.0+up99.99.99").
	ChartVersion string

	// TurtlesImageRepo is the image repository to override in rancher-config ConfigMap (optional, for e2e with preloaded images).
	TurtlesImageRepo string

	// TurtlesImageTag is the image tag to override in rancher-config ConfigMap (optional, for e2e with preloaded images).
	TurtlesImageTag string

	// RancherWaitInterval is the wait interval for Rancher.
	RancherWaitInterval []interface{} `envDefault:"15m,30s"`
}

// UpgradeInstallRancherWithGitea upgrades Rancher to a new version and configures it with Gitea chart repository
// environment variables to enable the system chart controller.
func UpgradeInstallRancherWithGitea(ctx context.Context, input UpgradeInstallRancherWithGiteaInput) PreRancherInstallHookResult {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for UpgradeInstallRancherWithGitea")
	Expect(input.BootstrapClusterProxy).NotTo(BeNil(), "BootstrapClusterProxy is required")
	Expect(input.HelmBinaryPath).NotTo(BeEmpty(), "HelmBinaryPath is required")
	Expect(input.RancherChartRepoName).NotTo(BeEmpty(), "RancherChartRepoName is required")
	Expect(input.RancherChartURL).NotTo(BeEmpty(), "RancherChartURL is required")
	Expect(input.RancherHostname).NotTo(BeEmpty(), "RancherHostname is required")
	Expect(input.RancherNamespace).NotTo(BeEmpty(), "RancherNamespace is required")
	Expect(input.RancherVersion).NotTo(BeEmpty(), "RancherVersion is required")
	Expect(input.ChartRepoURL).NotTo(BeEmpty(), "ChartRepoURL is required")
	Expect(input.ChartRepoBranch).NotTo(BeEmpty(), "ChartRepoBranch is required")
	Expect(input.ChartVersion).NotTo(BeEmpty(), "ChartVersion is required")
	Expect(input.RancherWaitInterval).NotTo(BeNil(), "RancherWaitInterval is required")

	By("Adding Rancher chart repo")
	addChart := &opframework.HelmChart{
		BinaryPath:      input.HelmBinaryPath,
		Name:            input.RancherChartRepoName,
		Path:            input.RancherChartURL,
		Commands:        opframework.Commands(opframework.Repo, opframework.Add),
		AdditionalFlags: opframework.Flags("--force-update"),
		Kubeconfig:      input.BootstrapClusterProxy.GetKubeconfigPath(),
	}
	_, err := addChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	updateChart := &opframework.HelmChart{
		BinaryPath: input.HelmBinaryPath,
		Commands:   opframework.Commands(opframework.Repo, opframework.Update),
		Kubeconfig: input.BootstrapClusterProxy.GetKubeconfigPath(),
	}
	_, err = updateChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	By(fmt.Sprintf("Upgrading/Installing Rancher to version %s with Gitea chart repository", input.RancherVersion))

	By("Running rancher pre-install hook")
	rancherHookResult := PreRancherInstallHook(PreRancherInstallHookInput{
		Ctx:                     ctx,
		BootstrapClusterProxy:   input.BootstrapClusterProxy,
		RancherIngressClassName: input.RancherIngressClassName,
		RancherHostname:         input.RancherHostname,
	})
	input.RancherPatches = append(input.RancherPatches, rancherHookResult.ConfigPatches...)

	// Run helm upgrade with environment variables for system chart controller
	args := []string{
		"upgrade", "rancher",
		fmt.Sprintf("%s/rancher", input.RancherChartRepoName),
		"--install",
		"--create-namespace",
		"--namespace", input.RancherNamespace,
		"--version", input.RancherVersion,
		"--reuse-values",
		"--set", fmt.Sprintf("hostname=%s", rancherHookResult.Hostname),
		"--set", fmt.Sprintf("replicas=%v", input.RancherReplicas),
		"--set", "extraEnv[0].name=CATTLE_CHART_DEFAULT_URL",
		"--set", fmt.Sprintf("extraEnv[0].value=%s", input.ChartRepoURL),
		"--set", "extraEnv[1].name=CATTLE_CHART_DEFAULT_BRANCH",
		"--set", fmt.Sprintf("extraEnv[1].value=%s", input.ChartRepoBranch),
		"--set", "extraEnv[2].name=CATTLE_RANCHER_TURTLES_VERSION",
		"--set", fmt.Sprintf("extraEnv[2].value=%s", input.ChartVersion),
		"--wait",
	}

	upgradeCmd := exec.Command(input.HelmBinaryPath, args...)
	upgradeCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", input.BootstrapClusterProxy.GetKubeconfigPath()))

	output, err := upgradeCmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred(), "Failed to upgrade Rancher: %s", string(output))

	By("Waiting for Rancher deployment to be ready")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter: input.BootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "rancher",
				Namespace: input.RancherNamespace,
			},
		},
	}, input.RancherWaitInterval...)

	// Optionally configure image overrides via rancher-config ConfigMap for e2e tests with preloaded images
	if input.TurtlesImageRepo != "" && input.TurtlesImageTag != "" {
		patch := fmt.Sprintf(`{"data":{"rancher-turtles": "image:\n  repository: %s\n  tag: %s\n"}}`, input.TurtlesImageRepo, input.TurtlesImageTag)

		// regcred is needed on EKS to pull the e2e test image.
		// See: framework.CreateDockerRegistrySecret
		if input.EnvironmentType == e2e.ManagementClusterEnvironmentEKS {
			patch = fmt.Sprintf(`{"data":{"rancher-turtles": "image:\n  repository: %s\n  tag: %s\nimagePullSecrets:\n- regcred\n"}}`, input.TurtlesImageRepo, input.TurtlesImageTag)
		}

		By("Patching rancher-config ConfigMap to override Turtles image")
		patchCmd := exec.Command(
			"kubectl",
			"patch", "configmap", "rancher-config",
			"-n", e2e.RancherNamespace,
			"--type", "merge",
			"--patch", patch,
		)
		patchCmd.Env = append(os.Environ(), fmt.Sprintf("KUBECONFIG=%s", input.BootstrapClusterProxy.GetKubeconfigPath()))
		patchOutput, patchErr := patchCmd.CombinedOutput()
		Expect(patchErr).ToNot(HaveOccurred(), "Failed to patch rancher-config ConfigMap: %s", string(patchOutput))
	}

	By("Applying additional patches")
	for _, patch := range input.RancherPatches {
		Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
			Proxy:    input.BootstrapClusterProxy,
			Template: patch,
			AddtionalEnvironmentVariables: map[string]string{
				e2e.RancherHostnameVar: rancherHookResult.Hostname,
			},
		})).To(Succeed())
	}

	return rancherHookResult
}

// PushRancherChartsToGiteaInput represents the input parameters for building and pushing Rancher charts to Gitea.
type PushRancherChartsToGiteaInput struct {
	// BootstrapClusterProxy is the cluster proxy for the bootstrap cluster.
	BootstrapClusterProxy framework.ClusterProxy

	// RancherChartsRepoDir is the directory where the rancher/charts repo will be created by the Makefile.
	RancherChartsRepoDir string `env:"RANCHER_CHARTS_REPO_DIR"`

	// RancherChartsBaseBranch is the branch name to use in the charts repo (e.g., dev-v2.13).
	// Turtles integration with Rancher system chart controller starts from Rancher v2.13.0.
	RancherChartsBaseBranch string `env:"RANCHER_CHARTS_BASE_BRANCH" envDefault:"dev-v2.13"`

	// GiteaServerAddress is the address of the Gitea server.
	GiteaServerAddress string

	// GiteaRepoName is the name of the repository in Gitea.
	GiteaRepoName string `envDefault:"charts"`

	// GiteaUsername is the username for Gitea authentication.
	GiteaUsername string `env:"GITEA_USER_NAME"`

	// GiteaPassword is the password for Gitea authentication.
	GiteaPassword string `env:"GITEA_USER_PWD"`

	// ChartVersion is the version string for the Turtles chart.
	// If not provided, will use RANCHER_CHART_DEV_VERSION from environment or Makefile default.
	ChartVersion string `env:"RANCHER_CHART_DEV_VERSION" envDefault:"108.0.0+up99.99.99"`
}

// PushRancherChartsToGiteaResult represents the result of building and pushing charts.
type PushRancherChartsToGiteaResult struct {
	// ChartRepoURL is the URL of the chart repository in Gitea (for git clone).
	ChartRepoURL string

	// ChartRepoHTTPURL is the HTTP URL for Rancher to use (format: http://host:port/git/charts).
	ChartRepoHTTPURL string

	// LocalRepoDir is the local directory where the chart repo was built.
	LocalRepoDir string

	// Branch is the branch name used in the charts repo.
	Branch string

	// ChartVersion is the version of the Turtles chart (from RANCHER_CHART_DEV_VERSION or Makefile default).
	ChartVersion string
}

// PushRancherChartsToGitea pushes the Turtles chart to a Gitea server.
// This is used for testing the system chart controller integration with Rancher.
// The chart is expected to be prepared before the tests run, using `make build-local-rancher-charts`.`
//
// The result can be used to configure Rancher with environment variables:
//   - CATTLE_CHART_DEFAULT_URL: result.ChartRepoHTTPURL
//   - CATTLE_CHART_DEFAULT_BRANCH: result.Branch
//   - CATTLE_RANCHER_TURTLES_VERSION: input.ChartVersion
func PushRancherChartsToGitea(ctx context.Context, input PushRancherChartsToGiteaInput) *PushRancherChartsToGiteaResult {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for PushRancherChartsToGitea")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for PushRancherChartsToGitea")
	Expect(input.RancherChartsRepoDir).ToNot(BeEmpty(), "RancherChartsRepoDir is required for PushRancherChartsToGitea")
	Expect(input.GiteaServerAddress).ToNot(BeEmpty(), "GiteaServerAddress is required for PushRancherChartsToGitea")
	Expect(input.GiteaRepoName).ToNot(BeEmpty(), "GiteaRepoName is required for PushRancherChartsToGitea")
	Expect(input.GiteaUsername).ToNot(BeEmpty(), "GiteaUsername is required for PushRancherChartsToGitea")
	Expect(input.GiteaPassword).ToNot(BeEmpty(), "GiteaPassword is required for PushRancherChartsToGitea")
	Expect(input.ChartVersion).ToNot(BeEmpty(), "ChartVersion is required for PushRancherChartsToGitea")

	By("Creating Gitea repository for Rancher charts")
	repoURL := turtlesframework.GiteaCreateRepo(ctx, turtlesframework.GiteaCreateRepoInput{
		ServerAddr: input.GiteaServerAddress,
		RepoName:   input.GiteaRepoName,
		Username:   input.GiteaUsername,
		Password:   input.GiteaPassword,
	})

	// Use random remote name.
	// This is needed since different suites may be adding remotes to different gitea instances concurrently.
	remoteName := util.RandomString(6)

	By("Configuring git remote to point to Gitea")
	// Strip protocol prefix from server address
	serverAddr := strings.TrimPrefix(input.GiteaServerAddress, "http://")
	serverAddr = strings.TrimPrefix(serverAddr, "https://")

	// Construct Gitea remote URL
	giteaRemoteURL := fmt.Sprintf("http://%s/%s/%s.git",
		serverAddr,
		input.GiteaUsername,
		input.GiteaRepoName)

	turtlesframework.GitSetRemote(ctx, turtlesframework.GitSetRemoteInput{
		RepoLocation: input.RancherChartsRepoDir,
		RemoteName:   remoteName,
		RemoteURL:    giteaRemoteURL,
		Username:     input.GiteaUsername,
		Password:     input.GiteaPassword,
	})

	By("Pushing changes to Gitea")
	// The Makefile already created a commit, so we just need to push
	// Force push is needed because Gitea creates an initial commit with README
	turtlesframework.GitPush(ctx, turtlesframework.GitPushInput{
		RepoLocation: input.RancherChartsRepoDir,
		RemoteName:   remoteName,
		Username:     input.GiteaUsername,
		Password:     input.GiteaPassword,
		Force:        true,
	})

	// Construct the HTTP URL for Rancher to use (plain HTTP without auth in URL)
	// Format: http://gitea-address/username/repo.git (matches Gitea's repository URL structure)
	httpURL := fmt.Sprintf("http://%s/%s/%s.git", serverAddr, input.GiteaUsername, input.GiteaRepoName)

	return &PushRancherChartsToGiteaResult{
		ChartRepoURL:     repoURL,
		ChartRepoHTTPURL: httpURL,
		LocalRepoDir:     input.RancherChartsRepoDir,
		Branch:           input.RancherChartsBaseBranch,
		ChartVersion:     input.ChartVersion,
	}
}
