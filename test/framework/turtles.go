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

package framework

import (
	"context"
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type WaitForCAPIProviderRolloutInput struct {
	capiframework.Getter
	Deployment                      *appsv1.Deployment
	Name, Namespace, Version, Image string
}

func WaitForCAPIProviderRollout(ctx context.Context, input WaitForCAPIProviderRolloutInput, intervals ...interface{}) {
	capiProvider := &turtlesv1.CAPIProvider{}
	key := types.NamespacedName{
		Name:      input.Name,
		Namespace: input.Namespace,
	}

	if input.Version != "" {
		Byf("Waiting for CAPIProvider %s to be at version %s", key.String(), input.Version)
		Eventually(func(g Gomega) {
			g.Expect(input.Getter.Get(ctx, key, capiProvider)).To(Succeed())
			g.Expect(capiProvider.Status.InstalledVersion).ToNot(BeNil())
			g.Expect(*capiProvider.Status.InstalledVersion).To(Equal(input.Version))
		}, intervals...).Should(Succeed(),
			"Failed to get CAPIProvider %s with version %s. Last observed: %s",
			key.String(), input.Version, klog.KObj(capiProvider))
	}

	if input.Deployment != nil && input.Image != "" {
		Byf("Waiting for Deployment %s to contain image %s", client.ObjectKeyFromObject(input.Deployment).String(), input.Image)
		Eventually(func(g Gomega) {
			g.Expect(input.Getter.Get(ctx, client.ObjectKeyFromObject(input.Deployment), input.Deployment)).To(Succeed())
			found := false
			for _, container := range input.Deployment.Spec.Template.Spec.Containers {
				if strings.HasPrefix(container.Image, input.Image) {
					found = true
				}
			}
			g.Expect(found).To(BeTrue())
		}, intervals...).Should(Succeed(),
			"Failed to get Deployment %s with image %s. Last observed: %s",
			client.ObjectKeyFromObject(input.Deployment).String(), input.Image, klog.KObj(input.Deployment))
	}
}

// WaitForDeploymentsRemovedInput is the input for WaitForDeploymentsRemoved.
type WaitForDeploymentsRemovedInput = capiframework.WaitForDeploymentsAvailableInput

// WaitForDeploymentsAvRemoved waits until the Deployment is removed.
func WaitForDeploymentsRemoved(ctx context.Context, input WaitForDeploymentsRemovedInput, intervals ...interface{}) {
	Byf("Waiting for deployment %s to be removed", klog.KObj(input.Deployment))
	deployment := &appsv1.Deployment{}
	Eventually(func() bool {
		key := client.ObjectKey{
			Namespace: input.Deployment.GetNamespace(),
			Name:      input.Deployment.GetName(),
		}
		if err := input.Getter.Get(ctx, key, deployment); err == nil {
			return false
		} else if apierrors.IsNotFound(err) {
			return true
		}

		return false
	}, intervals...).Should(BeTrue(), func() string { return capiframework.DescribeFailedDeployment(input, deployment) })
}

type VerifyCustomResourceHasBeenRemovedInput struct {
	capiframework.Lister
	GroupVersionKind schema.GroupVersionKind
}

func VerifyCustomResourceHasBeenRemoved(ctx context.Context, input VerifyCustomResourceHasBeenRemovedInput) {
	Byf("Verifying that custom resource %q has been removed", input.GroupVersionKind.String())
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(input.GroupVersionKind)
	err := input.Lister.List(ctx, list, &client.ListOptions{})
	Expect(err).NotTo(HaveOccurred(), "Failed to list custom resource %q: %v", input.GroupVersionKind.String(), err)
	Expect(list.Items).To(BeEmpty(), "Custom resource %q should have been removed, but found %d items", input.GroupVersionKind.String(), len(list.Items))
}

type VerifyClusterInput struct {
	BootstrapClusterProxy   capiframework.ClusterProxy
	Name                    string
	DeleteAfterVerification bool
}

func VerifyCluster(ctx context.Context, input VerifyClusterInput) {
	var cluster *clusterv1.Cluster

	Consistently(func() error {
		clusterList := &clusterv1.ClusterList{}
		Expect(input.BootstrapClusterProxy.GetClient().List(ctx, clusterList)).Should(Succeed())
		Expect(clusterList.Items).ShouldNot(BeEmpty(), "At least 1 Cluster must be found")

		for i, c := range clusterList.Items {
			if strings.HasPrefix(c.Name, input.Name) {
				cluster = &clusterList.Items[i]
				break
			}
		}

		Expect(cluster).ShouldNot(BeNil(), fmt.Sprintf("Cluster %s must be found", input.Name))

		key := types.NamespacedName{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		}

		Byf("Verifying cluster %s is ready", key.String())
		if err := input.BootstrapClusterProxy.GetClient().Get(ctx, key, cluster); err != nil {
			return fmt.Errorf("failed to get cluster %s: %w", key.String(), err)
		}
		if cluster.Status.ControlPlaneReady == false {
			return fmt.Errorf("cluster %s does not have a ControlPlaneReady status", key.String())
		}
		if cluster.Status.InfrastructureReady == false {
			return fmt.Errorf("cluster %s does not have an InfrastructureReady status", key.String())
		}

		readyCondition := conditions.Get(cluster, clusterv1.ReadyCondition)
		if readyCondition == nil {
			return fmt.Errorf("cluster %s does not have a Ready condition", key.String())
		}
		if readyCondition.Status != corev1.ConditionTrue {
			return fmt.Errorf("cluster %s Ready condition is not true: %s", key.String(), readyCondition.Message)
		}

		machineList := &clusterv1.MachineList{}
		if err := input.BootstrapClusterProxy.GetClient().List(ctx, machineList, client.InNamespace(cluster.Namespace),
			client.MatchingLabels{
				clusterv1.ClusterNameLabel: cluster.Name,
			}); err != nil {
			return fmt.Errorf("failed to list machines for cluster %s: %w", key.String(), err)
		}
		if len(machineList.Items) == 0 {
			return fmt.Errorf("no machines found for cluster %s", key.String())
		}

		for _, machine := range machineList.Items {
			readyConditionFound := false
			for _, condition := range machine.Status.Conditions {
				if condition.Type == clusterv1.ReadyCondition {
					readyConditionFound = true
					if condition.Status != corev1.ConditionTrue {
						return fmt.Errorf("machine %s Ready condition is not true: %s", machine.Name, condition.Message)
					}
					if condition.Message != "" {
						return fmt.Errorf("machine %s Ready condition has a non-empty message: %s", machine.Name, condition.Message)
					}
					break
				}
			}
			if !readyConditionFound {
				return fmt.Errorf("machine %s does not have a Ready condition", machine.Name)
			}
		}

		return nil
	}, "5m", "10s").Should(Succeed(), "Failed to verify cluster")

	if input.DeleteAfterVerification {
		By("Deleting Cluster")
		Expect(input.BootstrapClusterProxy.GetClient().Delete(ctx, cluster)).To(Succeed())
	}
}
