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
	"cmp"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/log"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	"github.com/rancher/turtles/test/e2e"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
)

type CreateMgmtV3UsingGitOpsSpecInput struct {
	E2EConfig             *clusterctl.E2EConfig
	BootstrapClusterProxy framework.ClusterProxy
	ArtifactFolder        string `env:"ARTIFACTS_FOLDER"`
	RancherServerURL      string

	ClusterctlBinaryPath        string `env:"CLUSTERCTL_BINARY_PATH"`
	ClusterTemplate             []byte
	AdditionalTemplates         [][]byte
	ClusterName                 string
	AdditionalTemplateVariables map[string]string

	CAPIClusterCreateWaitName string
	DeleteClusterWaitName     string

	// ControlPlaneMachineCount defines the number of control plane machines to be added to the workload cluster.
	// If not specified, 1 will be used.
	ControlPlaneMachineCount *int

	// WorkerMachineCount defines number of worker machines to be added to the workload cluster.
	// If not specified, 1 will be used.
	WorkerMachineCount *int

	GitAddr string

	SkipCleanup      bool `env:"SKIP_RESOURCE_CLEANUP"`
	SkipDeletionTest bool `env:"SKIP_DELETION_TEST"`

	LabelNamespace bool

	// TestClusterReimport defines whether to test un-importing and re-importing the cluster after initial test.
	TestClusterReimport bool

	// management.cattle.io specifc
	CapiClusterOwnerLabel          string
	CapiClusterOwnerNamespaceLabel string
	OwnedLabelName                 string

	// TopologyNamespace is the namespace to use for topology-related resources (e.g., cluster classes).
	TopologyNamespace string

	// AdditionalFleetGitRepos specifies additional FleetGitRepos to be created before the main GitRepo.
	// This is useful for setting up resources like cluster classes/cni/cpi that some tests require.
	AdditionalFleetGitRepos []turtlesframework.FleetCreateGitRepoInput
}

// CreateMgmtV3UsingGitOpsSpec implements a spec that will create a cluster via Fleet and test that it
// automatically imports into Rancher Manager.
func CreateMgmtV3UsingGitOpsSpec(ctx context.Context, inputGetter func() CreateMgmtV3UsingGitOpsSpecInput) {
	var (
		specName              = "creategitops"
		input                 CreateMgmtV3UsingGitOpsSpecInput
		namespace             *corev1.Namespace
		repoName              string
		cancelWatches         context.CancelFunc
		capiCluster           *types.NamespacedName
		rancherKubeconfig     *turtlesframework.RancherGetClusterKubeconfigResult
		originalKubeconfig    *turtlesframework.RancherGetClusterKubeconfigResult
		rancherConnectRes     *turtlesframework.RunCommandResult
		rancherCluster        *managementv3.Cluster
		capiClusterCreateWait []interface{}
		deleteClusterWait     []interface{}
	)

	validateRancherCluster := func() {
		By("Waiting for the rancher cluster record to appear")
		rancherClusters := &managementv3.ClusterList{}
		selectors := []client.ListOption{
			client.MatchingLabels{
				input.CapiClusterOwnerLabel:          capiCluster.Name,
				input.CapiClusterOwnerNamespaceLabel: capiCluster.Namespace,
				input.OwnedLabelName:                 "",
			},
		}
		Eventually(func() bool {
			Eventually(komega.List(rancherClusters, selectors...)).Should(Succeed())
			return len(rancherClusters.Items) == 1
		}, input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(BeTrue())
		rancherCluster = &rancherClusters.Items[0]
		Eventually(komega.Get(rancherCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())

		By("Waiting for the rancher cluster to have a deployed agent")
		Eventually(func() bool {
			Eventually(komega.Get(rancherCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
			return conditions.IsTrue(rancherCluster, managementv3.ClusterConditionAgentDeployed)
		}, input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(BeTrue())

		By("Waiting for the rancher cluster to be ready")
		Eventually(func() bool {
			Eventually(komega.Get(rancherCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
			return conditions.IsTrue(rancherCluster, managementv3.ClusterConditionReady)
		}, input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(BeTrue())
		By("Waiting for the rancher cluster to be ready")

		By("Rancher cluster should have the 'NoCreatorRBAC' annotation")
		Eventually(func() bool {
			Eventually(komega.Get(rancherCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
			_, found := rancherCluster.Annotations[turtlesannotations.NoCreatorRBACAnnotation]
			return found
		}, input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(BeTrue())

		By("Waiting for the CAPI cluster to be connectable using Rancher kubeconfig")
		turtlesframework.RancherGetClusterKubeconfig(ctx, turtlesframework.RancherGetClusterKubeconfigInput{
			Getter:           input.BootstrapClusterProxy.GetClient(),
			SecretName:       fmt.Sprintf("%s-kubeconfig", rancherCluster.Name),
			Namespace:        rancherCluster.Spec.FleetWorkspaceName,
			RancherServerURL: input.RancherServerURL,
			WriteToTempFile:  true,
			WaitInterval:     input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher"),
		}, rancherKubeconfig)

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
	}

	BeforeEach(func() {
		Expect(ctx).NotTo(BeNil(), "ctx is required for %s spec", specName)
		input = inputGetter()
		Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

		Expect(input.GitAddr).ToNot(BeEmpty(), "Invalid argument. input.GitAddr can't be empty when calling %s spec", specName)
		Expect(input.E2EConfig).ToNot(BeNil(), "Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName)
		Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(input.ArtifactFolder, 0750)).To(Succeed(), "Invalid argument. input.ArtifactFolder can't be created for %s spec", specName)

		Expect(input.E2EConfig.Variables).To(HaveKey(e2e.KubernetesManagementVersionVar))
		namespace, cancelWatches = e2e.SetupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)
		repoName = e2e.CreateRepoName(specName)

		capiClusterCreateWait = input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), input.CAPIClusterCreateWaitName)
		Expect(capiClusterCreateWait).ToNot(BeNil(), "Failed to get wait intervals %s", input.CAPIClusterCreateWaitName)

		deleteClusterWait = input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), input.DeleteClusterWaitName)
		Expect(capiClusterCreateWait).ToNot(BeNil(), "Failed to get wait intervals %s", input.CAPIClusterCreateWaitName)

		capiCluster = &types.NamespacedName{
			Namespace: namespace.Name,
			Name:      input.ClusterName,
		}

		rancherKubeconfig = new(turtlesframework.RancherGetClusterKubeconfigResult)
		originalKubeconfig = new(turtlesframework.RancherGetClusterKubeconfigResult)
		rancherConnectRes = new(turtlesframework.RunCommandResult)

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

		if input.LabelNamespace {
			turtlesframework.AddLabelsToNamespace(ctx, turtlesframework.AddLabelsToNamespaceInput{
				ClusterProxy: input.BootstrapClusterProxy,
				Name:         namespace.Name,
				Labels: map[string]string{
					"cluster-api.cattle.io/rancher-auto-import": "true",
				},
			})
		}

		if input.TopologyNamespace != "" {
			Expect(turtlesframework.CreateNamespace(ctx, input.BootstrapClusterProxy, input.TopologyNamespace)).To(Succeed())
		}

		for _, additionalRepo := range input.AdditionalFleetGitRepos {
			if additionalRepo.TargetClusterNamespace {
				additionalRepo.TargetNamespace = namespace.Name
			}

			turtlesframework.FleetCreateGitRepo(ctx, additionalRepo)
		}

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
			"TOPOLOGY_NAMESPACE":          cmp.Or(input.TopologyNamespace, namespace.Name),
			"CLUSTER_NAME":                input.ClusterName,
			"CLUSTER_CLASS_NAME":          fmt.Sprintf("%s-class", input.ClusterName),
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
		Eventually(komega.Object(capiCluster), capiClusterCreateWait...).Should(HaveField("Status.ControlPlaneReady", BeTrue()))

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

		By("Running checks on Rancher cluster")
		validateRancherCluster()

		By("Waiting for the CAPI Cluster to be Ready")
		Eventually(func() error {
			if err := input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(capiCluster), capiCluster); err != nil {
				return fmt.Errorf("getting Cluster: %w", err)
			}

			readyCondition := conditions.Get(capiCluster, clusterv1.ReadyCondition)
			if readyCondition == nil {
				return fmt.Errorf("Cluster Ready condition is not found")
			}

			switch readyCondition.Status {
			case corev1.ConditionTrue:
				//Cluster is ready
				return nil
			case corev1.ConditionFalse:
				return fmt.Errorf("Cluster is not Ready")
			default:
				return fmt.Errorf("Cluster Ready condition is unknown")
			}
		}, capiClusterCreateWait...).Should(Succeed(), "CAPI Cluster should be Ready")

		if input.TestClusterReimport {
			By("Deleting Rancher cluster record to simulate unimporting the cluster")
			err := input.BootstrapClusterProxy.GetClient().Delete(ctx, rancherCluster)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete rancher cluster")

			By("CAPI cluster should have the 'imported' annotation")
			Eventually(func() bool {
				Eventually(komega.Get(capiCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
				annotations := capiCluster.GetAnnotations()

				return annotations["imported"] == "true"
			}, capiClusterCreateWait...).Should(BeTrue(), "Failed to detect 'imported' annotation on CAPI cluster")

			By("Waiting for the Rancher cluster record to be removed")
			Eventually(komega.Get(rancherCluster), deleteClusterWait...).Should(MatchError(ContainSubstring("not found")), "Rancher cluster should be unimported (deleted)")

			By("Removing 'imported' annotation from CAPI cluster")
			Eventually(komega.Get(capiCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
			annotations := capiCluster.GetAnnotations()
			delete(annotations, "imported")
			capiCluster.SetAnnotations(annotations)
			err = input.BootstrapClusterProxy.GetClient().Update(ctx, capiCluster)
			Expect(err).NotTo(HaveOccurred(), "Failed to remove 'imported' annotation from CAPI cluster")

			By("Validating annotation is removed from CAPI cluster")
			Eventually(func() bool {
				Eventually(komega.Get(capiCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
				annotations := capiCluster.GetAnnotations()

				return annotations["imported"] != "true"
			}, capiClusterCreateWait...).Should(BeTrue(), "CAPI cluster still contains the 'imported' annotation")

			By("Rancher should be available after removing 'imported' annotation")
			validateRancherCluster()
		}
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

			By("Waiting for the CAPI cluster to be deleted")
			Eventually(func() error {
				cl := input.BootstrapClusterProxy.GetClient()

				cluster := &clusterv1.Cluster{}
				err := cl.Get(ctx, *capiCluster, cluster)
				if err != nil {
					if apierrors.IsNotFound(err) {
						return nil
					}
					return fmt.Errorf("getting CAPI Cluster: %w", err)
				}

				// (FIXME upstream)
				// Check if InfaCluster is deleted
				//
				// This is to bypass a race condition where the InfraCluster is deleted,
				// before some other resources (ex. InfraMachinePool) are deleted.
				if cluster.Spec.InfrastructureRef != nil &&
					(cluster.Spec.InfrastructureRef.Kind == "AWSCluster" ||
						cluster.Spec.InfrastructureRef.Kind == "GCPManagedCluster") {

					infraCluster := &unstructured.Unstructured{}
					infraCluster.SetGroupVersionKind(schema.GroupVersionKind{
						Group:   cluster.Spec.InfrastructureRef.GroupVersionKind().Group,
						Kind:    cluster.Spec.InfrastructureRef.GroupVersionKind().Kind,
						Version: cluster.Spec.InfrastructureRef.GroupVersionKind().Version,
					})
					infraClusterKey := types.NamespacedName{
						Namespace: cluster.Namespace,
						Name:      cluster.Spec.InfrastructureRef.Name,
					}
					if err := cl.Get(ctx, infraClusterKey, infraCluster); err != nil {
						if apierrors.IsNotFound(err) {
							// If the InfraCluster is deleted, ignore Cluster deletion (may hang indefinitely)
							return nil
						}
						return fmt.Errorf("getting %s %s/%s: %w", cluster.Spec.InfrastructureRef.Kind, infraClusterKey.Namespace, infraClusterKey.Name, err)
					}
					return fmt.Errorf("%s %s/%s is still present", cluster.Spec.InfrastructureRef.Kind, infraClusterKey.Namespace, infraClusterKey.Name)
				}

				return fmt.Errorf("CAPI Cluster %s/%s is still present", cluster.Namespace, cluster.Name)
			}, deleteClusterWait...).Should(Succeed(), "CAPI cluster deletion should complete")

			By("Waiting for the rancher cluster record to be removed")
			Eventually(komega.Get(rancherCluster), deleteClusterWait...).Should(MatchError(ContainSubstring("not found")), "Rancher cluster should be deleted")
		}
		e2e.DumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, namespace, cancelWatches, capiCluster, input.E2EConfig.GetIntervals, input.SkipCleanup)
	})
}
