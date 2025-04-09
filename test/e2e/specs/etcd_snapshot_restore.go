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

package specs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo/v2"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1beta2conditions "sigs.k8s.io/cluster-api/util/conditions/v1beta2"

	etcdrestorev1 "github.com/rancher/turtles/exp/day2/api/v1alpha1"
	"github.com/rancher/turtles/test/e2e"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"
)

type ETCDSnapshotRestoreInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	ArtifactFolder        string `env:"ARTIFACTS_FOLDER"`
	RancherServerURL      string

	ClusterctlBinaryPath        string `env:"CLUSTERCTL_BINARY_PATH"`
	ClusterTemplate             []byte
	AdditionalTemplates         [][]byte
	ClusterName                 string
	AdditionalTemplateVariables map[string]string

	CAPIClusterCreateWaitName   string
	CAPIClusterSnapshotWaitName string
	DeleteClusterWaitName       string

	// ControlPlaneMachineCount defines the number of control plane machines to be added to the workload cluster.
	// If not specified, 1 will be used.
	ControlPlaneMachineCount *int

	// WorkerMachineCount defines number of worker machines to be added to the workload cluster.
	// If not specified, 1 will be used.
	WorkerMachineCount *int

	GitAddr string

	SkipCleanup      bool `env:"SKIP_RESOURCE_CLEANUP"`
	SkipDeletionTest bool `env:"SKIP_DELETION_TEST"`

	// AdditionalFleetGitRepos specifies additional FleetGitRepos to be created before the main GitRepo.
	// This is useful for setting up resources like cluster classes/cni/cpi that some tests require.
	AdditionalFleetGitRepos []turtlesframework.FleetCreateGitRepoInput
}

// CreateUsingGitOpsSpec implements a spec that will create a cluster via Fleet and test that it
// automatically imports into Rancher Manager.
func ETCDSnapshotRestore(ctx context.Context, inputGetter func() ETCDSnapshotRestoreInput) {
	var (
		specName              = "etcdsnapshotrestore"
		input                 ETCDSnapshotRestoreInput
		namespace             *corev1.Namespace
		repoName              string
		cancelWatches         context.CancelFunc
		capiCluster           *types.NamespacedName
		originalKubeconfig    *turtlesframework.RancherGetClusterKubeconfigResult
		capiClusterCreateWait []interface{}
		capiSnapshotWait      []interface{}
	)

	validateETCDSnapshot := func() {
		machineList := &clusterv1.MachineList{}
		Expect(input.BootstrapClusterProxy.GetClient().List(ctx, machineList, client.InNamespace(capiCluster.Namespace))).To(Succeed())

		snapshot := &etcdrestorev1.ETCDMachineSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "snapshot",
				Namespace: capiCluster.Namespace,
			},
			Spec: etcdrestorev1.ETCDMachineSnapshotSpec{
				ClusterName: capiCluster.Name,
				MachineName: machineList.Items[0].Name,
			},
		}
		Expect(input.BootstrapClusterProxy.GetClient().Create(ctx, snapshot)).To(Succeed())

		By("Waiting for the snapshot to succeed")
		Eventually(func(g Gomega) {
			g.Expect(input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(snapshot), snapshot)).To(Succeed())
			g.Expect(snapshot.Status.Phase == etcdrestorev1.ETCDSnapshotPhaseDone).To(BeTrue())
		}, capiSnapshotWait...).Should(Succeed(), "Snapshot didn't finish", snapshot.Status.Phase)

		restore := &etcdrestorev1.ETCDSnapshotRestore{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "restore",
				Namespace: capiCluster.Namespace,
			},
			Spec: etcdrestorev1.ETCDSnapshotRestoreSpec{
				ClusterName:             capiCluster.Name,
				ETCDMachineSnapshotName: *snapshot.Status.SnapshotFileName,
			},
		}
		Expect(input.BootstrapClusterProxy.GetClient().Create(ctx, restore)).To(Succeed())

		By("Waiting for the restore to succeed")
		Eventually(func(g Gomega) {
			g.Expect(input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(restore), restore)).To(Succeed())
			g.Expect(restore.Status.Phase == etcdrestorev1.ETCDSnapshotRestorePhaseFinished).To(BeTrue())
		}, capiSnapshotWait...).Should(Succeed(), "Restore didn't finish", restore.Status.Phase)
	}

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

		Expect(input.E2EConfig).ToNot(BeNil(), "Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName)
		Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(input.ArtifactFolder, 0750)).To(Succeed(), "Invalid argument. input.ArtifactFolder can't be created for %s spec", specName)

		Expect(input.E2EConfig.Variables).To(HaveKey(e2e.KubernetesManagementVersionVar))
		namespace, cancelWatches = e2e.SetupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)
		repoName = e2e.CreateRepoName(specName)

		capiClusterCreateWait = input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), input.CAPIClusterCreateWaitName)
		Expect(capiClusterCreateWait).ToNot(BeNil(), "Failed to get wait intervals %s", input.CAPIClusterCreateWaitName)
		capiSnapshotWait = input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), input.CAPIClusterSnapshotWaitName)
		Expect(capiSnapshotWait).ToNot(BeNil(), "Failed to get wait intervals %s", input.CAPIClusterSnapshotWaitName)

		capiCluster = &types.NamespacedName{
			Namespace: namespace.Name,
			Name:      input.ClusterName,
		}

		originalKubeconfig = new(turtlesframework.RancherGetClusterKubeconfigResult)

		komega.SetClient(input.BootstrapClusterProxy.GetClient())
		komega.SetContext(ctx)
	})

	It("Should import a cluster using gitops", func() {
		controlPlaneMachineCount := 1
		if input.ControlPlaneMachineCount != nil {
			controlPlaneMachineCount = *input.ControlPlaneMachineCount
		}

		workerMachineCount := 1
		if input.WorkerMachineCount != nil {
			workerMachineCount = *input.WorkerMachineCount
		}

		for _, additionalRepo := range input.AdditionalFleetGitRepos {
			if additionalRepo.TargetClusterNamespace {
				additionalRepo.TargetNamespace = namespace.Name
			}

			turtlesframework.FleetCreateGitRepo(ctx, additionalRepo)
		}

		turtlesframework.AddLabelsToNamespace(ctx, turtlesframework.AddLabelsToNamespaceInput{ // Set import label to trigger fleet installation
			ClusterProxy: input.BootstrapClusterProxy,
			Name:         namespace.Name,
			Labels: map[string]string{
				"cluster-api.cattle.io/rancher-auto-import": "true",
			},
		})

		By("Create Git repository")

		repoCloneAddr := turtlesframework.GiteaCreateRepo(ctx, turtlesframework.GiteaCreateRepoInput{
			ServerAddr: input.GitAddr,
			RepoName:   repoName,
		})
		repoDir := turtlesframework.GitCloneRepo(ctx, turtlesframework.GitCloneRepoInput{
			Address: repoCloneAddr,
		})

		By("Create fleet repository structure")

		clustersDir := filepath.Join(repoDir, "clusters")
		os.MkdirAll(clustersDir, os.ModePerm)

		additionalVars := map[string]string{
			"CLUSTER_NAME":                input.ClusterName,
			"WORKER_MACHINE_COUNT":        strconv.Itoa(workerMachineCount),
			"CONTROL_PLANE_MACHINE_COUNT": strconv.Itoa(controlPlaneMachineCount),
		}
		for k, v := range input.AdditionalTemplateVariables {
			additionalVars[k] = v
		}

		clusterPath := filepath.Join(clustersDir, fmt.Sprintf("%s.yaml", input.ClusterName))
		Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
			Template:                      input.ClusterTemplate,
			OutputFilePath:                clusterPath,
			AddtionalEnvironmentVariables: additionalVars,
		})).To(Succeed())

		for n, template := range input.AdditionalTemplates {
			templatePath := filepath.Join(clustersDir, fmt.Sprintf("%s-template-%d.yaml", input.ClusterName, n))
			Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
				Template:                      template,
				OutputFilePath:                templatePath,
				AddtionalEnvironmentVariables: additionalVars,
			})).To(Succeed())
		}

		fleetPath := filepath.Join(clustersDir, "fleet.yaml")
		turtlesframework.FleetCreateFleetFile(ctx, turtlesframework.FleetCreateFleetFileInput{
			Namespace: namespace.Name,
			FilePath:  fleetPath,
		})

		By("Committing changes to fleet repo and pushing")

		turtlesframework.GitCommitAndPush(ctx, turtlesframework.GitCommitAndPushInput{
			CloneLocation: repoDir,
			CommitMessage: "ci: add clusters bundle",
		})

		By("Applying GitRepo")

		turtlesframework.FleetCreateGitRepo(ctx, turtlesframework.FleetCreateGitRepoInput{
			Name:            repoName,
			Repo:            repoCloneAddr,
			FleetGeneration: 1,
			Paths:           []string{"clusters"},
			ClusterProxy:    input.BootstrapClusterProxy,
		})

		By("Waiting for the CAPI cluster to appear")
		capiCluster := &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace.Name,
			Name:      input.ClusterName,
		}}
		Eventually(
			komega.Get(capiCluster),
			input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).
			Should(Succeed(), "Failed to apply CAPI cluster definition to cluster via Fleet")

		By("Waiting for cluster control plane to be Ready")
		Eventually(func(g Gomega) {
			g.Expect(input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(capiCluster), capiCluster)).To(Succeed())
			g.Expect(capiCluster.Status.ControlPlaneReady).To(BeTrue())
		}, capiClusterCreateWait...).Should(Succeed(), "Failed to connect to workload cluster using CAPI kubeconfig")

		By("Waiting for the CAPI cluster to be connectable")
		Eventually(func() error {
			remoteClient := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, capiCluster.Namespace, capiCluster.Name).GetClient()
			namespaces := &corev1.NamespaceList{}

			return remoteClient.List(ctx, namespaces)
		}, capiClusterCreateWait...).Should(Succeed(), "Failed to connect to workload cluster using CAPI kubeconfig")

		By("Storing the original CAPI cluster kubeconfig")
		turtlesframework.RancherGetOriginalKubeconfig(ctx, turtlesframework.RancherGetClusterKubeconfigInput{
			Getter:          input.BootstrapClusterProxy.GetClient(),
			SecretName:      fmt.Sprintf("%s-kubeconfig", capiCluster.Name),
			ClusterName:     capiCluster.Name,
			Namespace:       capiCluster.Namespace,
			WriteToTempFile: true,
		}, originalKubeconfig)

		By("Waiting for the Nodes to be Ready")
		Eventually(func(g Gomega) {
			machineList := &clusterv1.MachineList{}

			listOptions := []client.ListOption{
				client.InNamespace(capiCluster.Namespace),
				client.MatchingLabels(map[string]string{clusterv1.ClusterNameLabel: capiCluster.Name}),
			}
			g.Expect(input.BootstrapClusterProxy.GetClient().List(ctx, machineList, listOptions...)).To(Succeed())

			for _, machine := range machineList.Items {
				g.Expect(v1beta2conditions.IsTrue(&machine, clusterv1.MachineNodeReadyV1Beta2Condition)).To(BeTrue())
			}

		}, capiClusterCreateWait...).Should(Succeed(), "Failed to connect to workload cluster using CAPI kubeconfig")

		By("Creating snapshot on Rancher cluster")
		validateETCDSnapshot()
	})

	AfterEach(func() {
		err := testenv.CollectArtifacts(ctx, testenv.CollectArtifactsInput{
			Path: input.ClusterName + "bootstrap" + specName,
		})
		if err != nil {
			log.FromContext(ctx).Error(err, "failed to collect artifacts for the bootstrap cluster")
		}

		err = testenv.CollectArtifacts(ctx, testenv.CollectArtifactsInput{
			KubeconfigPath: originalKubeconfig.TempFilePath,
			Path:           input.ClusterName + specName,
		})
		if err != nil {
			log.FromContext(ctx).Error(err, "failed to collect artifacts for the child cluster")
		}

		// If SKIP_RESOURCE_CLEANUP=true & if the SkipDeletionTest is true, all the resources should stay as they are,
		// nothing should be deleted. If SkipDeletionTest is true, deleting the git repo will delete the clusters too.
		// If SKIP_RESOURCE_CLEANUP=false, everything must be cleaned up.
		if input.SkipCleanup && input.SkipDeletionTest {
			log.FromContext(ctx).Info("Skipping GitRepo and Cluster deletion from Rancher")
		} else {
			By("Deleting GitRepo from Rancher")
			turtlesframework.FleetDeleteGitRepo(ctx, turtlesframework.FleetDeleteGitRepoInput{
				Name:         repoName,
				ClusterProxy: input.BootstrapClusterProxy,
			})
		}
		e2e.DumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, namespace, cancelWatches, capiCluster, input.E2EConfig.GetIntervals, input.SkipCleanup)
	})
}
