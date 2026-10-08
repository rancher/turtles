/*
Copyright © 2026 SUSE LLC

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// systemUpgradeControllerNamespace and systemUpgradeControllerName identify the deployment Rancher installs in
	// workload clusters, which the RKE2 runtime extension relies on to update machines in-place.
	systemUpgradeControllerNamespace = "cattle-system"
	systemUpgradeControllerName      = "system-upgrade-controller"
)

// ValidateInPlaceUpdateInput represents the input parameters for validating an in-place update.
type ValidateInPlaceUpdateInput struct {
	// ClusterProxy is the cluster proxy for the management cluster.
	ClusterProxy framework.ClusterProxy

	// ClusterKey is the namespaced name of the CAPI Cluster to update.
	ClusterKey types.NamespacedName

	// KubernetesVersion is the version the Cluster is updated to.
	KubernetesVersion string

	// WaitIntervals are the intervals used when waiting for the update to complete.
	WaitIntervals []interface{}

	// SystemUpgradeControllerWaitIntervals are the intervals used when waiting for the system-upgrade-controller
	// to be available in the workload cluster, before triggering the update.
	SystemUpgradeControllerWaitIntervals []interface{}
}

// ValidateInPlaceUpdate bumps the Kubernetes version of a ClusterClass based Cluster and validates that all Machines
// are updated in-place: no Machine is created or deleted, and all Machines and Nodes end up on the new version.
// The Cluster's class must provide the inPlaceUpdates variable, which sets the control plane maxSurge to 0.
func ValidateInPlaceUpdate(ctx context.Context, input ValidateInPlaceUpdateInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for ValidateInPlaceUpdate")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.ClusterProxy can't be nil when calling ValidateInPlaceUpdate")
	Expect(input.KubernetesVersion).ToNot(BeEmpty(), "Invalid argument. input.KubernetesVersion can't be empty when calling ValidateInPlaceUpdate")

	cl := input.ClusterProxy.GetClient()
	workloadClient := input.ClusterProxy.GetWorkloadCluster(ctx, input.ClusterKey.Namespace, input.ClusterKey.Name).GetClient()

	// The runtime extension updates machines through machine-plan secrets, which are only available once Rancher
	// has deployed the system-upgrade-controller to the workload cluster.
	Byf("Waiting for deployment %s/%s to be available in the workload cluster", systemUpgradeControllerNamespace, systemUpgradeControllerName)
	Eventually(func() error {
		deployment := &appsv1.Deployment{}
		key := client.ObjectKey{Namespace: systemUpgradeControllerNamespace, Name: systemUpgradeControllerName}
		if err := workloadClient.Get(ctx, key, deployment); err != nil {
			return err
		}

		for _, c := range deployment.Status.Conditions {
			if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionTrue {
				return nil
			}
		}

		return fmt.Errorf("deployment %s is not available", key)
	}, input.SystemUpgradeControllerWaitIntervals...).Should(Succeed(),
		"Deployment %s/%s is not available in the workload cluster: in-place updates cannot run",
		systemUpgradeControllerNamespace, systemUpgradeControllerName)

	By("Recording the Machines before the in-place update")
	originalMachines, err := listClusterMachines(ctx, cl, input.ClusterKey)
	Expect(err).ToNot(HaveOccurred())
	Expect(originalMachines).ToNot(BeEmpty())

	originalNames := sets.New[string]()
	for _, m := range originalMachines {
		originalNames.Insert(m.Name)
	}

	Byf("Updating Cluster %s to %s in-place", input.ClusterKey, input.KubernetesVersion)
	Eventually(func() error {
		cluster := &clusterv1.Cluster{}
		if err := cl.Get(ctx, input.ClusterKey, cluster); err != nil {
			return err
		}

		patchBase := client.MergeFrom(cluster.DeepCopy())
		cluster.Spec.Topology.Version = input.KubernetesVersion
		setClusterVariable(cluster, "inPlaceUpdates", "true")

		// Without allowing one unavailable Machine, CAPI creates an additional Machine during in-place updates.
		for i := range cluster.Spec.Topology.Workers.MachineDeployments {
			cluster.Spec.Topology.Workers.MachineDeployments[i].Rollout.Strategy = clusterv1.MachineDeploymentTopologyRolloutStrategy{
				Type: clusterv1.RollingUpdateMachineDeploymentStrategyType,
				RollingUpdate: clusterv1.MachineDeploymentTopologyRolloutStrategyRollingUpdate{
					MaxSurge:       new(intstr.FromInt32(0)),
					MaxUnavailable: new(intstr.FromInt32(1)),
				},
			}
		}

		return cl.Patch(ctx, cluster, patchBase)
	}).Should(Succeed(), "Failed to update Cluster %s", input.ClusterKey)

	By("Waiting for all Machines to be updated in-place")
	Eventually(func() error {
		machines, err := listClusterMachines(ctx, cl, input.ClusterKey)
		if err != nil {
			return err
		}

		currentNames := sets.New[string]()
		for _, m := range machines {
			currentNames.Insert(m.Name)
		}

		if !currentNames.Equal(originalNames) {
			return StopTrying(fmt.Sprintf("Machines were replaced instead of updated in-place: before %v, now %v",
				sets.List(originalNames), sets.List(currentNames)))
		}

		for _, m := range machines {
			if m.Spec.Version != input.KubernetesVersion {
				return fmt.Errorf("machine %s is on version %s, expected %s", m.Name, m.Spec.Version, input.KubernetesVersion)
			}

			if _, ok := m.Annotations[clusterv1.UpdateInProgressAnnotation]; ok {
				return fmt.Errorf("machine %s is still being updated in-place", m.Name)
			}
		}

		return nil
	}, input.WaitIntervals...).Should(Succeed())

	By("Validating all Nodes run the new version")
	Eventually(func() error {
		nodes := &corev1.NodeList{}
		if err := workloadClient.List(ctx, nodes); err != nil {
			return err
		}

		if len(nodes.Items) != originalNames.Len() {
			return fmt.Errorf("expected %d Nodes, found %d", originalNames.Len(), len(nodes.Items))
		}

		for _, n := range nodes.Items {
			if n.Status.NodeInfo.KubeletVersion != input.KubernetesVersion {
				return fmt.Errorf("node %s runs kubelet %s, expected %s", n.Name, n.Status.NodeInfo.KubeletVersion, input.KubernetesVersion)
			}
		}

		return nil
	}, input.WaitIntervals...).Should(Succeed())
}

func listClusterMachines(ctx context.Context, cl client.Client, clusterKey types.NamespacedName) ([]clusterv1.Machine, error) {
	machines := &clusterv1.MachineList{}
	if err := cl.List(ctx, machines,
		client.InNamespace(clusterKey.Namespace),
		client.MatchingLabels{clusterv1.ClusterNameLabel: clusterKey.Name},
	); err != nil {
		return nil, err
	}

	return machines.Items, nil
}

func setClusterVariable(cluster *clusterv1.Cluster, name, rawValue string) {
	for i := range cluster.Spec.Topology.Variables {
		if cluster.Spec.Topology.Variables[i].Name == name {
			cluster.Spec.Topology.Variables[i].Value = apiextensionsv1.JSON{Raw: []byte(rawValue)}

			return
		}
	}

	cluster.Spec.Topology.Variables = append(cluster.Spec.Topology.Variables, clusterv1.ClusterVariable{
		Name:  name,
		Value: apiextensionsv1.JSON{Raw: []byte(rawValue)},
	})
}
