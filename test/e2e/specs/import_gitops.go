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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	clusterv1beta1 "sigs.k8s.io/cluster-api/api/core/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/log"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	"github.com/rancher/turtles/test/e2e"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"github.com/rancher/turtles/test/testenv"
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

	// TopologyNamespace is the namespace to use for topology-related resources (e.g., cluster classes).
	TopologyNamespace string

	// AdditionalFleetGitRepos specifies additional FleetGitRepos to be created before the main GitRepo.
	// This is useful for setting up resources like cluster classes/cni/cpi that some tests require.
	AdditionalFleetGitRepos []turtlesframework.FleetCreateGitRepoInput

	// SkipLatestFeatureChecks can be used to skip tests that have not been released yet and can not be tested
	// with stable versions of Turtles, for example during the chart upgrade test.
	SkipLatestFeatureChecks bool

	// SkipClusterAvailableWait can be used to skip the VerifyClusterAvailable check.
	// CAPI's VerifyClusterAvailable asserts that the Available condition has an empty Message,
	// but managed clusters like GKE may have a non-empty message during automatic upgrades
	// (e.g. "TopologyReconciled: Cluster is upgrading to v1.x.x"). Since CAPG's releaseChannel
	// enum (rapid|regular|stable|extended) does not support disabling auto-upgrades, this skip
	// is necessary for GKE clusters.
	SkipClusterAvailableWait bool

	// VerifyETCDSize can be used to verify ETCD database size and for the supported environments,
	// collect debug data.
	VerifyETCDSize bool

	// RancherManagedFleet is used to determine whether the `provisioning.cattle.io/externally-managed`
	// annotation should be present or not in an imported test cluster.
	RancherManagedFleet bool

	// ValidateFleetAgentWasInstalled is used to indicate whether the test should also check that
	// the fleet-agent has been installed on the downstream cluster.
	ValidateFleetAgentWasInstalled bool
}

// CreateUsingGitOpsSpec implements a spec that will create a cluster via Fleet and test that it
// automatically imports into Rancher Manager.
func CreateUsingGitOpsSpec(ctx context.Context, inputGetter func() CreateUsingGitOpsSpecInput) {
	var (
		specName              = "creategitops"
		input                 CreateUsingGitOpsSpecInput
		namespace             *corev1.Namespace
		cancelWatches         context.CancelFunc
		capiClusterKey        types.NamespacedName
		rancherCluster        *managementv3.Cluster
		originalKubeconfig    *turtlesframework.RancherGetClusterKubeconfigResult
		capiClusterCreateWait []interface{}
		deleteClusterWait     []interface{}
	)

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

		capiClusterKey = types.NamespacedName{
			Namespace: namespace.Name,
			Name:      input.ClusterName,
		}

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
		originalKubeconfig = turtlesframework.RancherGetOriginalKubeconfig(ctx, turtlesframework.RancherGetClusterKubeconfigInput{
			ClusterProxy:    input.BootstrapClusterProxy,
			SecretName:      fmt.Sprintf("%s-kubeconfig", capiCluster.Name),
			ClusterName:     capiClusterKey.Name,
			Namespace:       capiClusterKey.Namespace,
			WriteToTempFile: true,
		})

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

		// Validate Rancher importing
		rancherCluster = turtlesframework.ValidateRancherCluster(ctx, turtlesframework.ValidateRancherClusterInput{
			BootstrapClusterProxy:   input.BootstrapClusterProxy,
			CAPIClusterKey:          capiClusterKey,
			CAPIClusterAnnotations:  capiCluster.Annotations,
			RancherServerURL:        input.RancherServerURL,
			WaitRancherIntervals:    input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher"),
			WaitKubeconfigIntervals: input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-kubeconfig"),
			SkipLatestFeatureChecks: input.SkipLatestFeatureChecks,
			RancherManagedFleet:     input.RancherManagedFleet,
		})

		if !input.SkipClusterAvailableWait {
			By("Waiting for the CAPI Cluster to be Available")
			framework.VerifyClusterAvailable(
				ctx,
				framework.VerifyClusterAvailableInput{
					Getter:    input.BootstrapClusterProxy.GetClient(),
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				})
		}

		if input.ValidateFleetAgentWasInstalled {
			By("Waiting for Fleet agent to be installed on downstream cluster")
			framework.WaitForDeploymentsAvailable(ctx, framework.WaitForDeploymentsAvailableInput{
				Getter: input.BootstrapClusterProxy.GetWorkloadCluster(ctx, capiCluster.Namespace, capiCluster.Name).GetClient(),
				Deployment: &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
					Name:      "fleet-agent",
					Namespace: "cattle-fleet-system",
				}},
			}, input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-controllers")...)
		}

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
			rancherCluster = turtlesframework.ValidateRancherCluster(ctx, turtlesframework.ValidateRancherClusterInput{
				BootstrapClusterProxy:   input.BootstrapClusterProxy,
				CAPIClusterKey:          capiClusterKey,
				CAPIClusterAnnotations:  capiCluster.Annotations,
				RancherServerURL:        input.RancherServerURL,
				WaitRancherIntervals:    input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher"),
				SkipLatestFeatureChecks: input.SkipLatestFeatureChecks,
			})
		}
	})

	AfterEach(func() {
		By(fmt.Sprintf("Collecting artifacts for %s/%s", input.ClusterName, specName))
		testenv.TryCollectArtifacts(ctx, testenv.CollectArtifactsInput{
			Path:                    input.ClusterName + "-bootstrap-" + specName,
			BootstrapKubeconfigPath: input.BootstrapClusterProxy.GetKubeconfigPath(),
		})

		if originalKubeconfig != nil {
			By("Collecting workload Cluster artifacts")
			testenv.TryCollectArtifacts(ctx, testenv.CollectArtifactsInput{
				KubeconfigPath: originalKubeconfig.TempFilePath,
				Path:           input.ClusterName + "-workload-" + specName,
			})
		} else {
			By("Skipping workload Cluster artifacts collection. No workload Cluster found.")
		}

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

			if rancherCluster != nil {
				By("Waiting for the Rancher Cluster record to be removed")
				Eventually(komega.Get(rancherCluster), deleteClusterWait...).Should(MatchError(ContainSubstring("not found")), "Rancher cluster should be deleted")
			} else {
				By("Skipping Rancher Cluster deletion check. No Rancher Cluster found.")
			}
		}
		e2e.DumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, namespace, cancelWatches, capiClusterKey, input.E2EConfig.GetIntervals, input.SkipCleanup)
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
		capiClusterKey        types.NamespacedName
		rancherCluster        *managementv3.Cluster
		originalKubeconfig    *turtlesframework.RancherGetClusterKubeconfigResult
		capiClusterCreateWait []interface{}
		deleteClusterWait     []interface{}
	)

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

		capiClusterKey = types.NamespacedName{
			Namespace: namespace.Name,
			Name:      input.ClusterName,
		}

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
				Name:      fmt.Sprintf("%s-kubeconfig", capiClusterKey.Name),
				Namespace: capiClusterKey.Namespace,
			}

			if err := input.BootstrapClusterProxy.GetClient().Get(ctx, key, secret); err != nil {
				return err
			}

			remoteClient := input.BootstrapClusterProxy.GetWorkloadCluster(ctx, capiClusterKey.Namespace, capiClusterKey.Name).GetClient()
			namespaces := &corev1.NamespaceList{}

			return remoteClient.List(ctx, namespaces)
		}, capiClusterCreateWait...).Should(Succeed(), "Failed to connect to workload cluster using CAPI kubeconfig")

		By("Storing the original CAPI cluster kubeconfig")
		originalKubeconfig = turtlesframework.RancherGetOriginalKubeconfig(ctx, turtlesframework.RancherGetClusterKubeconfigInput{
			ClusterProxy:    input.BootstrapClusterProxy,
			SecretName:      fmt.Sprintf("%s-kubeconfig", capiClusterKey.Name),
			ClusterName:     capiClusterKey.Name,
			Namespace:       capiClusterKey.Namespace,
			WriteToTempFile: true,
		})

		By("Waiting for cluster control plane to be Ready")
		cluster := &clusterv1beta1.Cluster{}
		Eventually(func() bool {
			if err := input.BootstrapClusterProxy.GetClient().Get(ctx, capiClusterKey, cluster); err != nil {
				return false
			}

			return cluster.Status.ControlPlaneReady
		}, capiClusterCreateWait...).Should(BeTrue(), "Control plane did not become ready")

		turtlesframework.ValidateRancherCluster(ctx, turtlesframework.ValidateRancherClusterInput{
			BootstrapClusterProxy:   input.BootstrapClusterProxy,
			CAPIClusterKey:          capiClusterKey,
			CAPIClusterAnnotations:  cluster.Annotations,
			RancherServerURL:        input.RancherServerURL,
			WaitRancherIntervals:    input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-rancher"),
			WaitKubeconfigIntervals: input.E2EConfig.GetIntervals(input.BootstrapClusterProxy.GetName(), "wait-kubeconfig"),
			SkipLatestFeatureChecks: input.SkipLatestFeatureChecks,
		})

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

		if originalKubeconfig != nil {
			By("Collecting workload Cluster artifacts")
			testenv.TryCollectArtifacts(ctx, testenv.CollectArtifactsInput{
				KubeconfigPath: originalKubeconfig.TempFilePath,
				Path:           input.ClusterName + "-workload-" + specName,
			})
		} else {
			By("Skipping workload Cluster artifacts collection. No workload Cluster found.")
		}

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

			if rancherCluster != nil {
				By("Waiting for the Rancher Cluster record to be removed")
				Eventually(komega.Get(rancherCluster), deleteClusterWait...).Should(MatchError(ContainSubstring("not found")), "Rancher cluster should be deleted")
			} else {
				By("Skipping Rancher Cluster deletion check. No Rancher Cluster found.")
			}
		}
		e2e.DumpSpecResourcesAndCleanup(ctx, specName, input.BootstrapClusterProxy, namespace, cancelWatches, capiClusterKey, input.E2EConfig.GetIntervals, input.SkipCleanup)
	})
}
