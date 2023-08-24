//go:build e2e
// +build e2e

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

package e2e

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drone/envsubst/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	. "sigs.k8s.io/cluster-api-operator/test/framework"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
)

const (
	operatorPackage    = "CAPI_OPERATOR"
	kubernetesVersion  = "KUBERNETES_VERSION"
	rancherFeatures    = "RANCHER_FEATURES"
	rancherHostname    = "RANCHER_HOSTNAME"
	rancherVersion     = "RANCHER_VERSION"
	rancherPath        = "RANCHER_PATH"
	rancherUrl         = "RANCHER_URL"
	rancherRepoName    = "RANCHER_REPO_NAME"
	rancherPassword    = "RANCHER_PASSWORD"
	capiInfrastructure = "CAPI_INFRASTRUCTURE"

	ngrokRepoName  = "NGROK_REPO_NAME"
	ngrokUrl       = "NGROK_URL"
	ngrokPath      = "NGROK_PATH"
	ngrokApiKey    = "NGROK_API_KEY"
	ngrokAuthToken = "NGROK_AUTHTOKEN"
)

// Test suite flags.
var (
	// configPath is the path to the e2e config file.
	configPath string

	// useExistingCluster instructs the test to use the current cluster instead of creating a new one (default discovery rules apply).
	useExistingCluster bool

	// artifactFolder is the folder to store e2e test artifacts.
	artifactFolder string

	// skipCleanup prevents cleanup of test resources e.g. for debug purposes.
	skipCleanup bool

	// helmBinaryPath is the path to the helm binary.
	helmBinaryPath string

	// chartPath is the path to the operator chart.
	chartPath string
)

// Test suite global vars.
var (
	// e2eConfig to be used for this test, read from configPath.
	e2eConfig *clusterctl.E2EConfig

	// clusterctlConfigPath to be used for this test, created by generating a clusterctl local repository
	// with the providers specified in the configPath.
	clusterctlConfigPath string

	// bootstrapClusterProvider manages provisioning of the the bootstrap cluster to be used for the e2e tests.
	// Please note that provisioning will be skipped if e2e.use-existing-cluster is provided.
	bootstrapClusterProvider bootstrap.ClusterProvider

	// bootstrapClusterProxy allows to interact with the bootstrap cluster to be used for the e2e tests.
	bootstrapClusterProxy framework.ClusterProxy

	// helmChart is the helm chart helper to be used for the e2e tests.
	helmChart *HelmChart
)

func init() {
	flag.StringVar(&configPath, "e2e.config", "config/operator.yaml", "path to the e2e config file")
	flag.StringVar(&artifactFolder, "e2e.artifacts-folder", "_artifacts", "folder where e2e test artifact should be stored")
	flag.BoolVar(&skipCleanup, "e2e.skip-resource-cleanup", false, "if true, the resource cleanup after tests will be skipped")
	flag.BoolVar(&useExistingCluster, "e2e.use-existing-cluster", false, "if true, the test uses the current cluster instead of creating a new one (default discovery rules apply)")
	flag.StringVar(&helmBinaryPath, "e2e.helm-binary-path", "helm", "path to the helm binary")
	flag.StringVar(&chartPath, "e2e.chart-path", "", "path to the operator chart")
}

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)

	ctrl.SetLogger(klog.Background())

	RunSpecs(t, "capi-operator-e2e")
}

// Using a SynchronizedBeforeSuite for controlling how to create resources shared across ParallelNodes (~ginkgo threads).
// The bootstrap cluster is created once and shared across all the tests.
var _ = SynchronizedBeforeSuite(func() []byte {
	// Before all ParallelNodes.

	Expect(configPath).To(BeAnExistingFile(), "Invalid test suite argument. e2e.config should be an existing file.")
	Expect(os.MkdirAll(artifactFolder, 0755)).To(Succeed(), "Invalid test suite argument. Can't create e2e.artifacts-folder %q", artifactFolder)
	Expect(helmBinaryPath).To(BeAnExistingFile(), "Invalid test suite argument. helm-binary-path should be an existing file.")
	Expect(chartPath).To(BeAnExistingFile(), "Invalid test suite argument. chart-path should be an existing file.")

	By("Initializing a runtime.Scheme with all the GVK relevant for this test")
	scheme := initScheme()

	By(fmt.Sprintf("Loading the e2e test configuration from %q", configPath))
	e2eConfig = loadE2EConfig(configPath)

	By(fmt.Sprintf("Creating a clusterctl config into %q", artifactFolder))
	clusterctlConfigPath = createClusterctlLocalRepository(e2eConfig, filepath.Join(artifactFolder, "repository"))

	By("Setting up the bootstrap cluster")
	bootstrapClusterProvider, bootstrapClusterProxy = setupCluster(e2eConfig, scheme, useExistingCluster, "bootstrap")

	By("Initializing the bootstrap cluster")
	initBootstrapCluster(bootstrapClusterProxy, e2eConfig, artifactFolder, useExistingCluster)

	return []byte(
		strings.Join([]string{
			artifactFolder,
			configPath,
			clusterctlConfigPath,
			bootstrapClusterProxy.GetKubeconfigPath(),
		}, ","),
	)
}, func(data []byte) {
	// Before each ParallelNode.

	parts := strings.Split(string(data), ",")
	Expect(parts).To(HaveLen(4))

	artifactFolder = parts[0]
	configPath = parts[1]
	clusterctlConfigPath = parts[2]
	bootstrapKubeconfigPath := parts[3]

	e2eConfig = loadE2EConfig(configPath)
	bootstrapProxy := framework.NewClusterProxy("bootstrap", bootstrapKubeconfigPath, initScheme(), framework.WithMachineLogCollector(framework.DockerLogCollector{}))

	bootstrapClusterProxy = bootstrapProxy
})

func initScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	framework.TryAddDefaultSchemes(scheme)
	Expect(operatorv1.AddToScheme(scheme)).To(Succeed())
	return scheme
}

func loadE2EConfig(configPath string) *clusterctl.E2EConfig {
	configData, err := os.ReadFile(configPath)
	Expect(err).ToNot(HaveOccurred(), "Failed to read the e2e test config file")
	Expect(configData).ToNot(BeEmpty(), "The e2e test config file should not be empty")

	config := &clusterctl.E2EConfig{}
	Expect(yaml.UnmarshalStrict(configData, config)).To(Succeed(), "Failed to convert the e2e test config file to yaml")

	config.Defaults()
	config.AbsPaths(filepath.Dir(configPath))

	return config
}

func createClusterctlLocalRepository(config *clusterctl.E2EConfig, repositoryFolder string) string {
	createRepositoryInput := clusterctl.CreateRepositoryInput{
		E2EConfig:        config,
		RepositoryFolder: repositoryFolder,
	}

	clusterctlConfig := clusterctl.CreateRepository(ctx, createRepositoryInput)
	Expect(clusterctlConfig).To(BeAnExistingFile(), "The clusterctl config file does not exists in the local repository %s", repositoryFolder)
	return clusterctlConfig
}

func setupCluster(config *clusterctl.E2EConfig, scheme *runtime.Scheme, useExistingCluster bool, clusterProxyName string) (bootstrap.ClusterProvider, framework.ClusterProxy) {
	var clusterProvider bootstrap.ClusterProvider
	kubeconfigPath := ""
	if !useExistingCluster {
		clusterProvider = bootstrap.CreateKindBootstrapClusterAndLoadImages(ctx, bootstrap.CreateKindBootstrapClusterAndLoadImagesInput{
			Name:               config.ManagementClusterName,
			KubernetesVersion:  config.GetVariable(kubernetesVersion),
			RequiresDockerSock: true,
			Images:             config.Images,
		})
		Expect(clusterProvider).ToNot(BeNil(), "Failed to create a bootstrap cluster")

		kubeconfigPath = clusterProvider.GetKubeconfigPath()
		Expect(kubeconfigPath).To(BeAnExistingFile(), "Failed to get the kubeconfig file for the bootstrap cluster")
	}

	proxy := framework.NewClusterProxy(clusterProxyName, kubeconfigPath, scheme, framework.WithMachineLogCollector(framework.DockerLogCollector{}))

	return clusterProvider, proxy
}

func initBootstrapCluster(bootstrapClusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig, artifactFolder string, useExistingCluster bool) {
	if useExistingCluster {
		return
	}

	Expect(bootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. bootstrapClusterProxy can't be nil when calling initBootstrapCluster")
	Expect(config.GetVariable(operatorPackage)).To(BeAnExistingFile(), "Invalid path to operator package. Please specify a valid one")
	logFolder := filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName())
	Expect(os.MkdirAll(logFolder, 0750)).To(Succeed(), "Invalid argument. Log folder can't be created for initBootstrapCluster")

	initCAPIOperator(bootstrapClusterProxy, config)
	initRancherTurtles(bootstrapClusterProxy, config)
	initRancher(bootstrapClusterProxy, config)
}

func initRancherTurtles(clusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig) {
	By("Installing rancher-turtles chart")
	chart := &HelmChart{
		BinaryPath:      helmBinaryPath,
		Path:            chartPath,
		Name:            "rancher-turtles",
		Kubeconfig:      clusterProxy.GetKubeconfigPath(),
		AdditionalFlags: Flags("-n", rancherTurtlesNamespace, "--create-namespace", "--wait"),
	}
	_, err := chart.Run(nil)
	Expect(err).ToNot(HaveOccurred())
}

func initCAPIOperator(clusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig) {
	By("Adding docker variables secret")
	Expect(clusterProxy.Apply(ctx, dockerVariablesSecret)).To(Succeed())

	By("Installing CAPI operator chart")
	chart := &HelmChart{
		BinaryPath:      helmBinaryPath,
		Path:            config.GetVariable(operatorPackage),
		Name:            "capi-operator",
		Kubeconfig:      clusterProxy.GetKubeconfigPath(),
		Output:          Full,
		AdditionalFlags: Flags("-n", operatorNamespace, "--create-namespace", "--wait"),
	}
	_, err := chart.Run(map[string]string{
		"cert-manager.enabled": "true",
		"infrastructure":       config.GetVariable(capiInfrastructure),
		"secretName":           "variables",
		"secretNamespace":      "default",
	})
	Expect(err).ToNot(HaveOccurred())
}

func initRancher(clusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig) {
	By("Installing rancher chart")
	addChart := &HelmChart{
		BinaryPath:      helmBinaryPath,
		Name:            config.GetVariable(rancherRepoName),
		Path:            config.GetVariable(rancherUrl),
		Commands:        Commands(Repo, Add),
		AdditionalFlags: Flags("--force-update"),
		Kubeconfig:      clusterProxy.GetKubeconfigPath(),
	}
	_, err := addChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	updateChart := &HelmChart{
		BinaryPath: helmBinaryPath,
		Commands:   Commands(Repo, Update),
		Kubeconfig: clusterProxy.GetKubeconfigPath(),
	}
	_, err = updateChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	chart := &HelmChart{
		BinaryPath: helmBinaryPath,
		Path:       config.GetVariable(rancherPath),
		Name:       "rancher",
		Kubeconfig: clusterProxy.GetKubeconfigPath(),
		AdditionalFlags: Flags(
			"--version", config.GetVariable(rancherVersion),
			"--namespace", rancherNamespace,
			"--create-namespace",
			"--wait",
		),
	}
	_, err = chart.Run(map[string]string{
		"bootstrapPassword":         config.GetVariable(rancherPassword),
		"features":                  config.GetVariable(rancherFeatures),
		"global.cattle.psp.enabled": "false",
		"hostname":                  config.GetVariable(rancherHostname),
		"replicas":                  "1",
	})
	Expect(err).ToNot(HaveOccurred())

	By("Updating rancher settings")
	settingPatch, err := envsubst.Eval(string(rancherSettingPatch), os.Getenv)
	Expect(err).ToNot(HaveOccurred())
	Expect(clusterProxy.Apply(ctx, []byte(settingPatch))).To(Succeed())

	initNgrokIngress(clusterProxy, config)

	By("Setting up ingress")
	ingress, err := envsubst.Eval(string(ingressConfig), os.Getenv)
	Expect(err).ToNot(HaveOccurred())
	Expect(clusterProxy.Apply(ctx, []byte(ingress))).To(Succeed())

	By("Waiting for rancher webhook rollout")
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     bootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "rancher-webhook", Namespace: rancherNamespace}},
	}, config.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...)

	// hack: fleet controller needs to be restarted first to pickup config change with a valid API url.
	framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
		Getter:     bootstrapClusterProxy.GetClient(),
		Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "fleet-controller", Namespace: "cattle-fleet-system"}},
	}, config.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...)

	By("Bouncing the fleet")
	Eventually(func() error {
		return bootstrapClusterProxy.GetClient().DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace("cattle-fleet-system"), client.MatchingLabels{"app": "fleet-controller"})
	}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).ShouldNot(HaveOccurred())
}

func initNgrokIngress(bootstrapClusterProxy framework.ClusterProxy, config *clusterctl.E2EConfig) {
	if config.GetVariable(ngrokApiKey) == "" || config.GetVariable(ngrokAuthToken) == "" {
		return
	}

	By("Setting up ngrok-ingress-controller")
	addChart := &HelmChart{
		BinaryPath:      helmBinaryPath,
		Name:            config.GetVariable(ngrokRepoName),
		Path:            config.GetVariable(ngrokUrl),
		Commands:        Commands(Repo, Add),
		AdditionalFlags: Flags("--force-update"),
		Kubeconfig:      bootstrapClusterProxy.GetKubeconfigPath(),
	}
	_, err := addChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	updateChart := &HelmChart{
		BinaryPath: helmBinaryPath,
		Commands:   Commands(Repo, Update),
		Kubeconfig: bootstrapClusterProxy.GetKubeconfigPath(),
	}
	_, err = updateChart.Run(nil)
	Expect(err).ToNot(HaveOccurred())

	installChart := &HelmChart{
		BinaryPath:      helmBinaryPath,
		Name:            config.GetVariable(ngrokRepoName),
		Path:            config.GetVariable(ngrokPath),
		Kubeconfig:      bootstrapClusterProxy.GetKubeconfigPath(),
		Wait:            true,
		AdditionalFlags: Flags("--timeout", "5m"),
	}
	_, err = installChart.Run(map[string]string{
		"credentials.apiKey":    config.GetVariable(ngrokApiKey),
		"credentials.authtoken": config.GetVariable(ngrokAuthToken),
	})
	Expect(err).ToNot(HaveOccurred())

	By("Updating rancher svc")
	Expect(bootstrapClusterProxy.Apply(ctx, rancherServicePatch, "--server-side")).To(Succeed())

	By("Setting up default ingress class")
	Expect(bootstrapClusterProxy.Apply(ctx, ingressClassPatch, "--server-side")).To(Succeed())
}

func dumpBootstrapClusterLogs(bootstrapClusterProxy framework.ClusterProxy) {
	if bootstrapClusterProxy == nil {
		return
	}

	clusterLogCollector := bootstrapClusterProxy.GetLogCollector()
	if clusterLogCollector == nil {
		return
	}

	nodes, err := bootstrapClusterProxy.GetClientSet().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Failed to get nodes for the bootstrap cluster: %v\n", err)
		return
	}

	for i := range nodes.Items {
		nodeName := nodes.Items[i].GetName()
		err = clusterLogCollector.CollectMachineLog(
			ctx,
			bootstrapClusterProxy.GetClient(),
			&clusterv1.Machine{
				Spec:       clusterv1.MachineSpec{ClusterName: nodeName},
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
			},
			filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName(), "machines", nodeName),
		)
		if err != nil {
			fmt.Printf("Failed to get logs for the bootstrap cluster node %s: %v\n", nodeName, err)
		}
	}
}

// Using a SynchronizedAfterSuite for controlling how to delete resources shared across ParallelNodes (~ginkgo threads).
// The bootstrap cluster is shared across all the tests, so it should be deleted only after all ParallelNodes completes.
var _ = SynchronizedAfterSuite(func() {
	// After each ParallelNode.
}, func() {
	By("Dumping logs from the bootstrap cluster")
	dumpBootstrapClusterLogs(bootstrapClusterProxy)

	By("Tearing down the management clusters")
	if !skipCleanup {
		tearDown(bootstrapClusterProvider, bootstrapClusterProxy)
	}
})

func tearDown(clusterProvider bootstrap.ClusterProvider, clusterProxy framework.ClusterProxy) {
	if clusterProxy != nil {
		clusterProxy.Dispose(ctx)
	}
	if clusterProvider != nil {
		clusterProvider.Dispose(ctx)
	}
}
