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
	"k8s.io/apimachinery/pkg/types"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	capiframework "sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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

		Byf("Verifying cluster %s is available", key.String())
		if err := input.BootstrapClusterProxy.GetClient().Get(ctx, key, cluster); err != nil {
			return fmt.Errorf("failed to get cluster %s: %w", key.String(), err)
		}

		framework.VerifyClusterAvailable(ctx,
			framework.VerifyClusterAvailableInput{
				Getter:    input.BootstrapClusterProxy.GetClient(),
				Name:      cluster.Name,
				Namespace: cluster.Namespace,
			},
		)

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
					if condition.Status != metav1.ConditionTrue {
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

		By("Waiting for CAPI cluster to be deleted")
		Eventually(func(g Gomega) {
			err := input.BootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, "5m", "10s").Should(Succeed(), "Failed to verify cluster deletion")
	}
}
