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
	"path/filepath"
	"strconv"

	"github.com/drone/envsubst/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	provisioningv1 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/provisioning/v1"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	turtlesnaming "github.com/rancher-sandbox/rancher-turtles/util/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("Create and delete CAPI cluster functionality should work", func() {
	var (
		rancherCluster *provisioningv1.Cluster
		capiCluster    *clusterv1.Cluster
		// Manually bumping fleet generation is required to force resources rollout
		// after deleting them in the cluster by other means then fleet.
		//
		// TODO: Could be removed after https://github.com/rancher/fleet/issues/1551 is closed
		fleetGeneration int
	)

	BeforeEach(func() {
		SetClient(bootstrapClusterProxy.GetClient())
		SetContext(ctx)
		capiCluster = &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Namespace: capiClusterNamespace,
			Name:      capiClusterName,
		}}
		rancherCluster = &provisioningv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Namespace: capiClusterNamespace,
			Name:      turtlesnaming.Name(capiCluster.Name).ToRancherName(),
		}}

		By("Creating a CAPI cluster with calico CNI")
		Eventually(func() error {
			fleetCAPI, err := envsubst.Eval(string(fleetCAPITestdata), func(_ string) string {
				fleetGeneration += 1
				return strconv.Itoa(fleetGeneration)
			})
			Expect(err).ToNot(HaveOccurred())
			return bootstrapClusterProxy.Apply(ctx, []byte(fleetCAPI))
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).Should(Succeed())

		By("Waiting for the CAPI cluster to appear")
		Eventually(Get(capiCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())
	})

	AfterEach(func() {
		By("Dumping child cluster logs")
		bootstrapClusterProxy.CollectWorkloadClusterLogs(ctx, capiCluster.Namespace, capiCluster.Name, filepath.Join(artifactFolder, "clusters", capiCluster.Name, strconv.Itoa(fleetGeneration)))

		By("Removing CAPI cluster record")
		Eventually(func() error {
			return bootstrapClusterProxy.GetClient().Delete(ctx, capiCluster)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).Should(Succeed())

		By("Waiting for the rancher cluster record to be removed")
		Eventually(Get(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).Should(MatchError(ContainSubstring("not found")))
	})

	It("should successfully create a rancher cluster from a CAPI cluster", func() {
		By("Waiting for the CAPI cluster to be connectable")
		Eventually(func() error {
			namespaces := &corev1.NamespaceList{}
			remoteClient := bootstrapClusterProxy.GetWorkloadCluster(ctx, capiCluster.Namespace, capiCluster.Name).GetClient()
			return remoteClient.List(ctx, namespaces)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())

		By("Waiting for the rancher cluster record to appear")
		Eventually(Get(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())

		By("Waiting for the rancher cluster to have a deployed agent")
		Eventually(Object(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(HaveField("Status.AgentDeployed", BeTrue()))

		By("Waiting for the rancher cluster to be ready")
		Eventually(Object(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(HaveField("Status.Ready", BeTrue()))
	})

	It("should successfully remove a rancher cluster and check it if is no longer re-imported", func() {
		By("Waiting for the rancher cluster record to appear")
		Eventually(Get(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())

		By("Removing rancher cluster record, and waiting for it to be removed")
		Eventually(func() error {
			return bootstrapClusterProxy.GetClient().Delete(ctx, rancherCluster)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).Should(Succeed())
		Eventually(Get(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).Should(MatchError(ContainSubstring("not found")))

		By("Checking if rancher cluster record will not be re-created again")
		Consistently(Get(rancherCluster), e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-consistently")...).Should(MatchError(ContainSubstring("not found")))
	})
})
