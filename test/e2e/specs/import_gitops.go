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
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
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

type CreateUsingGitOpsSpecInput struct {
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

	// SkipLatestFeatureChecks can be used to skip tests that have not been released yet and can not be tested
	// with stable versions of Turtles, for example during the chart upgrade test.
	SkipLatestFeatureChecks bool
}

// CreateUsingGitOpsSpec implements a spec that will create a cluster via Fleet and test that it
// automatically imports into Rancher Manager.
func CreateUsingGitOpsSpec(ctx context.Context, inputGetter func() CreateUsingGitOpsSpecInput) {
	var (
		specName              = "creategitops"
		input                 CreateUsingGitOpsSpecInput
		namespace             *corev1.Namespace
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

		By("Rancher cluster should have the custom description if it is provided")
		capiClusterObject := framework.GetClusterByName(
			ctx,
			framework.GetClusterByNameInput{
				Getter:    input.BootstrapClusterProxy.GetClient(),
				Name:      input.ClusterName,
				Namespace: namespace.Name,
			},
		)
		annotations := capiClusterObject.GetAnnotations()
		description := annotations[turtlesannotations.ClusterDescriptionAnnotation]
		if description == "" {
			description = "CAPI cluster imported to Rancher"
		}
		Expect(rancherCluster.Spec.Description).To(Equal(description))

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

		By("Rancher cluster should have the 'provisioning.cattle.io/externally-managed' annotation")
		Eventually(func() bool {
			Eventually(komega.Get(rancherCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
			_, found := rancherCluster.Annotations[turtlesannotations.ExternalFleetAnnotation]
			return found
		}, input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(BeTrue())

		if !input.SkipLatestFeatureChecks { // Can be removed after Turtles v0.26 is used a starter for the chart upgrade test.
			By("Rancher cluster should have the 'rancher.io/imported-cluster-version-management' annotation")
			Eventually(func() bool {
				Eventually(komega.Get(rancherCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
				value, found := rancherCluster.Annotations[turtlesannotations.ImportedClusterVersionManagementAnnotation]
				return found && value == "false"
			}, input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(BeTrue())
		}

		By("Waiting for the CAPI cluster to be connectable using Rancher kubeconfig")
		turtlesframework.RancherGetClusterKubeconfig(ctx, turtlesframework.RancherGetClusterKubeconfigInput{
			ClusterProxy:     input.BootstrapClusterProxy,
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

		input.ClusterName = fmt.Sprintf("%s-%s", input.ClusterName, util.RandomString(6))
		turtlesframework.Byf("Testing with cluster %s", input.ClusterName)

		Expect(input.E2EConfig).ToNot(BeNil(), "Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName)
		Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(input.ArtifactFolder, 0o750)).To(Succeed(), "Invalid argument. input.ArtifactFolder can't be created for %s spec", specName)

		Expect(input.E2EConfig.Variables).To(HaveKey(e2e.KubernetesManagementVersionVar))
		namespace, cancelWatches = e2e.SetupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)

		capiClusterCreateWait = input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), input.CAPIClusterCreateWaitName)
		Expect(capiClusterCreateWait).ToNot(BeNil(), "Failed to get wait intervals %s", input.CAPIClusterCreateWaitName)

		deleteClusterWait = input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), input.DeleteClusterWaitName)
		Expect(deleteClusterWait).ToNot(BeNil(), "Failed to get wait intervals %s", input.DeleteClusterWaitName)

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

		additionalVars := map[string]string{
			"NAMESPACE":                   namespace.Name,
			"TOPOLOGY_NAMESPACE":          cmp.Or(input.TopologyNamespace, namespace.Name),
			"CLUSTER_NAME":                input.ClusterName,
			"CLUSTER_CLASS_NAME":          fmt.Sprintf("%s-class", input.ClusterName),
			"WORKER_MACHINE_COUNT":        strconv.Itoa(workerMachineCount),
			"CONTROL_PLANE_MACHINE_COUNT": strconv.Itoa(controlPlaneMachineCount),
		}

		for k, v := range input.AdditionalTemplateVariables {
			additionalVars[k] = v
		}

		Eventually(func() error {
			return turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
				Template:                      input.ClusterTemplate,
				AddtionalEnvironmentVariables: additionalVars,
				Proxy:                         input.BootstrapClusterProxy,
			})
		}).WithTimeout(2*time.Minute).WithPolling(2*time.Second).To(Succeed(), "Cluster must be created")

		for _, template := range input.AdditionalTemplates {
			Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
				Template:                      template,
				AddtionalEnvironmentVariables: additionalVars,
				Proxy:                         input.BootstrapClusterProxy,
			})).To(Succeed())
		}

		By("Waiting for the CAPI cluster to appear")

		capiCluster := framework.GetClusterByName(
			ctx,
			framework.GetClusterByNameInput{
				Getter:    input.BootstrapClusterProxy.GetClient(),
				Name:      input.ClusterName,
				Namespace: namespace.Name,
			},
		)

		By("Waiting for the CAPI cluster to be connectable")
		Eventually(func() error {
			secret := &corev1.Secret{}
			key := client.ObjectKey{
				Name:      fmt.Sprintf("%s-kubeconfig", capiCluster.Name),
				Namespace: capiCluster.Namespace,
			}

			if err := input.BootstrapClusterProxy.GetClient().Get(ctx, key, secret); err != nil {
				return err
			}

			remoteClient := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, capiCluster.Namespace, capiCluster.Name).GetClient()
			namespaces := &corev1.NamespaceList{}

			return remoteClient.List(ctx, namespaces)
		}, capiClusterCreateWait...).Should(Succeed(), "Failed to connect to workload cluster using CAPI kubeconfig")

		By("Storing the original CAPI cluster kubeconfig")
		turtlesframework.RancherGetOriginalKubeconfig(ctx, turtlesframework.RancherGetClusterKubeconfigInput{
			ClusterProxy:    input.BootstrapClusterProxy,
			SecretName:      fmt.Sprintf("%s-kubeconfig", capiCluster.Name),
			ClusterName:     capiCluster.Name,
			Namespace:       capiCluster.Namespace,
			WriteToTempFile: true,
		}, originalKubeconfig)

		By("Waiting for cluster control plane to be Ready")
		Eventually(func() bool {
			capiCluster := framework.GetClusterByName(
				ctx,
				framework.GetClusterByNameInput{
					Getter:    input.BootstrapClusterProxy.GetClient(),
					Name:      input.ClusterName,
					Namespace: namespace.Name,
				},
			)

			return ptr.Deref(capiCluster.Status.Initialization.ControlPlaneInitialized, false)
		}, capiClusterCreateWait...).Should(BeTrue())

		By("Running checks on Rancher cluster")
		validateRancherCluster()

		By("Waiting for the CAPI Cluster to be Available")
		framework.VerifyClusterAvailable(
			ctx,
			framework.VerifyClusterAvailableInput{
				Getter:    input.BootstrapClusterProxy.GetClient(),
				Namespace: capiCluster.Namespace,
				Name:      capiCluster.Name,
			})

		if input.TestClusterReimport {
			By("Deleting Rancher cluster record to simulate unimporting the cluster")
			err := input.BootstrapClusterProxy.GetClient().Delete(ctx, rancherCluster)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete rancher cluster")

			By("CAPI cluster should have the 'imported' annotation")
			Eventually(func() bool {
				capiCluster := framework.GetClusterByName(
					ctx,
					framework.GetClusterByNameInput{
						Getter:    input.BootstrapClusterProxy.GetClient(),
						Name:      input.ClusterName,
						Namespace: namespace.Name,
					},
				)
				annotations := capiCluster.GetAnnotations()

				return annotations["imported"] == "true"
			}, capiClusterCreateWait...).Should(BeTrue(), "Failed to detect 'imported' annotation on CAPI cluster")

			By("Waiting for the Rancher cluster record to be removed")
			Eventually(komega.Get(rancherCluster), deleteClusterWait...).Should(MatchError(ContainSubstring("not found")), "Rancher cluster should be unimported (deleted)")

			By("Removing 'imported' annotation from CAPI cluster")
			Eventually(func() error {
				capiCluster := framework.GetClusterByName(
					ctx,
					framework.GetClusterByNameInput{
						Getter:    input.BootstrapClusterProxy.GetClient(),
						Name:      input.ClusterName,
						Namespace: namespace.Name,
					},
				)
				annotations := capiCluster.GetAnnotations()
				delete(annotations, "imported")
				capiCluster.SetAnnotations(annotations)

				return input.BootstrapClusterProxy.GetClient().Update(ctx, capiCluster)
			}).ShouldNot(HaveOccurred(), "Failed to remove 'imported' annotation from CAPI cluster")

			By("Validating annotation is removed from CAPI cluster")
			Eventually(func() bool {
				capiCluster := framework.GetClusterByName(
					ctx,
					framework.GetClusterByNameInput{
						Getter:    input.BootstrapClusterProxy.GetClient(),
						Name:      input.ClusterName,
						Namespace: namespace.Name,
					},
				)
				annotations := capiCluster.GetAnnotations()
				return annotations["imported"] != "true"
			}, capiClusterCreateWait...).Should(BeTrue(), "CAPI cluster still contains the 'imported' annotation")

			By("Rancher should be available after removing 'imported' annotation")
			validateRancherCluster()
		}
	})

	AfterEach(func() {
		By(fmt.Sprintf("Collecting artifacts for %s/%s", input.ClusterName, specName))

		testenv.TryCollectArtifacts(ctx, testenv.CollectArtifactsInput{
			Path:                    input.ClusterName + "-bootstrap-" + specName,
			BootstrapKubeconfigPath: input.BootstrapClusterProxy.GetKubeconfigPath(),
		})

		testenv.TryCollectArtifacts(ctx, testenv.CollectArtifactsInput{
			KubeconfigPath: originalKubeconfig.TempFilePath,
			Path:           input.ClusterName + "-workload-" + specName,
		})

		// If SKIP_RESOURCE_CLEANUP=true & if the SkipDeletionTest is true, all the resources should stay as they are,
		// nothing should be deleted. If SkipDeletionTest is true, deleting the git repo will delete the clusters too.
		// If SKIP_RESOURCE_CLEANUP=false, everything must be cleaned up.
		if input.SkipCleanup && input.SkipDeletionTest {
			log.FromContext(ctx).Info("Skipping Cluster deletion from Rancher")
		} else {
			By("Deleting CAPI Cluster")
			capiClusterObj := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
				Getter: input.BootstrapClusterProxy.GetClient(), Name: input.ClusterName, Namespace: namespace.Name,
			})
			framework.DeleteCluster(ctx, framework.DeleteClusterInput{
				Deleter: input.BootstrapClusterProxy.GetClient(),
				Cluster: capiClusterObj,
			})
			framework.WaitForClusterDeleted(ctx, framework.WaitForClusterDeletedInput{
				ClusterProxy:         input.BootstrapClusterProxy,
				Cluster:              capiClusterObj,
				ClusterctlConfigPath: input.ClusterctlBinaryPath,
			}, deleteClusterWait...)

			By("Waiting for the rancher cluster record to be removed")
			Eventually(komega.Get(rancherCluster), deleteClusterWait...).Should(MatchError(ContainSubstring("not found")), "Rancher cluster should be deleted")
		}
		e2e.DumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, namespace, cancelWatches, capiCluster, input.E2EConfig.GetIntervals, input.SkipCleanup)
	})
}

// CreateUsingGitOpsV1Beta1Spec implements a spec that will create a `v1beta1` cluster via Fleet and test that it
// automatically imports into Rancher Manager.
// NOTE: this can be removed when base Turtles version used during upgrade test already uses `v1beta2`.
func CreateUsingGitOpsV1Beta1Spec(ctx context.Context, inputGetter func() CreateUsingGitOpsSpecInput) {
	var (
		specName              = "creategitops"
		input                 CreateUsingGitOpsSpecInput
		namespace             *corev1.Namespace
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

		By("Rancher cluster should have the 'provisioning.cattle.io/externally-managed' annotation")
		Eventually(func() bool {
			Eventually(komega.Get(rancherCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
			_, found := rancherCluster.Annotations[turtlesannotations.ExternalFleetAnnotation]
			return found
		}, input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(BeTrue())

		if !input.SkipLatestFeatureChecks { // Can be removed after Turtles v0.26 is used a starter for the chart upgrade test.
			By("Rancher cluster should have the 'rancher.io/imported-cluster-version-management' annotation")
			Eventually(func() bool {
				Eventually(komega.Get(rancherCluster), input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
				value, found := rancherCluster.Annotations[turtlesannotations.ImportedClusterVersionManagementAnnotation]
				return found && value == "false"
			}, input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher")...).Should(BeTrue())
		}

		By("Waiting for the CAPI cluster to be connectable using Rancher kubeconfig")
		turtlesframework.RancherGetClusterKubeconfig(ctx, turtlesframework.RancherGetClusterKubeconfigInput{
			ClusterProxy:     input.BootstrapClusterProxy,
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

		input.ClusterName = fmt.Sprintf("%s-%s", input.ClusterName, util.RandomString(6))
		turtlesframework.Byf("Testing with cluster %s", input.ClusterName)

		Expect(input.E2EConfig).ToNot(BeNil(), "Invalid argument. input.E2EConfig can't be nil when calling %s spec", specName)
		Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "Invalid argument. input.BootstrapClusterProxy can't be nil when calling %s spec", specName)
		Expect(os.MkdirAll(input.ArtifactFolder, 0o750)).To(Succeed(), "Invalid argument. input.ArtifactFolder can't be created for %s spec", specName)

		Expect(input.E2EConfig.Variables).To(HaveKey(e2e.KubernetesManagementVersionVar))
		namespace, cancelWatches = e2e.SetupSpecNamespace(ctx, specName, input.BootstrapClusterProxy, input.ArtifactFolder)

		capiClusterCreateWait = input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), input.CAPIClusterCreateWaitName)
		Expect(capiClusterCreateWait).ToNot(BeNil(), "Failed to get wait intervals %s", input.CAPIClusterCreateWaitName)

		deleteClusterWait = input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), input.DeleteClusterWaitName)
		Expect(deleteClusterWait).ToNot(BeNil(), "Failed to get wait intervals %s", input.DeleteClusterWaitName)

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

		additionalVars := map[string]string{
			"NAMESPACE":                   namespace.Name,
			"TOPOLOGY_NAMESPACE":          cmp.Or(input.TopologyNamespace, namespace.Name),
			"CLUSTER_NAME":                input.ClusterName,
			"CLUSTER_CLASS_NAME":          fmt.Sprintf("%s-class", input.ClusterName),
			"WORKER_MACHINE_COUNT":        strconv.Itoa(workerMachineCount),
			"CONTROL_PLANE_MACHINE_COUNT": strconv.Itoa(controlPlaneMachineCount),
		}

		for k, v := range input.AdditionalTemplateVariables {
			additionalVars[k] = v
		}

		Eventually(func() error {
			return turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
				Template:                      input.ClusterTemplate,
				AddtionalEnvironmentVariables: additionalVars,
				Proxy:                         input.BootstrapClusterProxy,
			})
		}).WithTimeout(2*time.Minute).WithPolling(2*time.Second).To(Succeed(), "Cluster must be created")

		for _, template := range input.AdditionalTemplates {
			Expect(turtlesframework.ApplyFromTemplate(ctx, turtlesframework.ApplyFromTemplateInput{
				Template:                      template,
				AddtionalEnvironmentVariables: additionalVars,
				Proxy:                         input.BootstrapClusterProxy,
			})).To(Succeed())
		}

		By("Waiting for the CAPI cluster to appear")
		Eventually(func() error {
			cluster := &clusterv1beta1.Cluster{}
			key := client.ObjectKey{Name: input.ClusterName, Namespace: namespace.Name}

			return input.BootstrapClusterProxy.GetClient().Get(ctx, key, cluster)
		})

		By("Waiting for the CAPI cluster to be connectable")
		Eventually(func() error {
			secret := &corev1.Secret{}
			key := client.ObjectKey{
				Name:      fmt.Sprintf("%s-kubeconfig", capiCluster.Name),
				Namespace: capiCluster.Namespace,
			}

			if err := input.BootstrapClusterProxy.GetClient().Get(ctx, key, secret); err != nil {
				return err
			}

			remoteClient := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, capiCluster.Namespace, capiCluster.Name).GetClient()
			namespaces := &corev1.NamespaceList{}

			return remoteClient.List(ctx, namespaces)
		}, capiClusterCreateWait...).Should(Succeed(), "Failed to connect to workload cluster using CAPI kubeconfig")

		By("Storing the original CAPI cluster kubeconfig")
		turtlesframework.RancherGetOriginalKubeconfig(ctx, turtlesframework.RancherGetClusterKubeconfigInput{
			ClusterProxy:    input.BootstrapClusterProxy,
			SecretName:      fmt.Sprintf("%s-kubeconfig", capiCluster.Name),
			ClusterName:     capiCluster.Name,
			Namespace:       capiCluster.Namespace,
			WriteToTempFile: true,
		}, originalKubeconfig)

		By("Waiting for cluster control plane to be Ready")
		Eventually(func() bool {
			cluster := &clusterv1beta1.Cluster{}
			key := client.ObjectKey{Name: input.ClusterName, Namespace: namespace.Name}

			if err := input.BootstrapClusterProxy.GetClient().Get(ctx, key, cluster); err != nil {
				return false
			}

			return cluster.Status.ControlPlaneReady
		}, capiClusterCreateWait...).Should(BeTrue(), "Control plane did not become ready")

		By("Running checks on Rancher cluster")
		validateRancherCluster()

		By("Waiting for the CAPI Cluster to be Available")
		turtlesframework.WaitForV1Beta1ClusterReady(ctx, turtlesframework.WaitForV1Beta1ClusterReadyInput{
			Getter:    input.BootstrapClusterProxy.GetClient(),
			Name:      input.ClusterName,
			Namespace: namespace.Name,
		}, capiClusterCreateWait...)
	})

	AfterEach(func() {
		By(fmt.Sprintf("Collecting artifacts for %s/%s", input.ClusterName, specName))

		testenv.TryCollectArtifacts(ctx, testenv.CollectArtifactsInput{
			Path:                    input.ClusterName + "-bootstrap-" + specName,
			BootstrapKubeconfigPath: input.BootstrapClusterProxy.GetKubeconfigPath(),
		})

		testenv.TryCollectArtifacts(ctx, testenv.CollectArtifactsInput{
			KubeconfigPath: originalKubeconfig.TempFilePath,
			Path:           input.ClusterName + "-workload-" + specName,
		})

		// If SKIP_RESOURCE_CLEANUP=true & if the SkipDeletionTest is true, all the resources should stay as they are,
		// nothing should be deleted. If SkipDeletionTest is true, deleting the git repo will delete the clusters too.
		// If SKIP_RESOURCE_CLEANUP=false, everything must be cleaned up.
		if input.SkipCleanup && input.SkipDeletionTest {
			log.FromContext(ctx).Info("Skipping Cluster deletion from Rancher")
		} else {
			By("Deleting CAPI Cluster")
			capiClusterObj := framework.GetClusterByName(ctx, framework.GetClusterByNameInput{
				Getter: input.BootstrapClusterProxy.GetClient(), Name: input.ClusterName, Namespace: namespace.Name,
			})
			framework.DeleteCluster(ctx, framework.DeleteClusterInput{
				Deleter: input.BootstrapClusterProxy.GetClient(),
				Cluster: capiClusterObj,
			})
			framework.WaitForClusterDeleted(ctx, framework.WaitForClusterDeletedInput{
				ClusterProxy:         input.BootstrapClusterProxy,
				Cluster:              capiClusterObj,
				ClusterctlConfigPath: input.ClusterctlBinaryPath,
			}, deleteClusterWait...)

			By("Waiting for the rancher cluster record to be removed")
			Eventually(komega.Get(rancherCluster), deleteClusterWait...).Should(MatchError(ContainSubstring("not found")), "Rancher cluster should be deleted")
		}
		e2e.DumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, namespace, cancelWatches, capiCluster, input.E2EConfig.GetIntervals, input.SkipCleanup)
	})
}
