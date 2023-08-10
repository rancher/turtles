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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/rancher-turtles/internal/rancher"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("Create and delete CAPI cluster functionality should work", func() {
	var (
		rancherClusterHandler *rancher.ClusterHandler
		rancherCluster        *rancher.Cluster
		capiCluster           *clusterv1.Cluster
		rancherClusterKey     client.ObjectKey
	)

	BeforeEach(func() {
		rancherClusterHandler = rancher.NewClusterHandler(ctx, bootstrapClusterProxy.GetClient())
		capiCluster = &clusterv1.Cluster{ObjectMeta: metav1.ObjectMeta{
			Namespace: capiClusterNamespace,
			Name:      capiClusterName,
		}}
		rancherCluster = &rancher.Cluster{ObjectMeta: metav1.ObjectMeta{
			Namespace: capiClusterNamespace,
			Name:      fmt.Sprintf("%s-capi", capiClusterName),
		}}
		rancherClusterKey = client.ObjectKey{
			Namespace: rancherCluster.Namespace,
			Name:      rancherCluster.Name,
		}

		By("Creating a CAPI cluster with calico CNI")
		Eventually(func() error {
			return bootstrapClusterProxy.Apply(ctx, fleetCAPITestdata)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).Should(Succeed())
	})

	AfterEach(func() {
		By("Removing CAPI cluster record")
		Eventually(func() bool {
			return apierrors.IsNotFound(bootstrapClusterProxy.GetClient().Delete(ctx, capiCluster))
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).Should(BeTrue())

		By("Waiting for the rancher cluster record to be removed")
		Eventually(func() bool {
			_, err := rancherClusterHandler.Get(rancherClusterKey)
			return apierrors.IsNotFound(err)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).Should(BeTrue())
	})

	It("should successfully create a rancher cluster from CAPI cluster", func() {
		By("Waiting for the CAPI cluster to appear")
		Eventually(func() error {
			return bootstrapClusterProxy.GetClient().Get(ctx, client.ObjectKeyFromObject(capiCluster), capiCluster)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())

		By("Waiting for the rancher cluster record to appear")
		Eventually(func() error {
			_, err := rancherClusterHandler.Get(rancherClusterKey)
			return err
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Succeed())

		By("Waiting for the rancher cluster to have deployed agent")
		Eventually(func() bool {
			cluster, err := rancherClusterHandler.Get(rancherClusterKey)
			return err == nil && cluster.Status.AgentDeployed == true
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Equal(true))

		By("Waiting for the rancher cluster to be ready")
		Eventually(func() bool {
			cluster, err := rancherClusterHandler.Get(rancherClusterKey)
			return err == nil && cluster.Status.Ready == true
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-rancher")...).Should(Equal(true))
	})

	It("should successfully remove a rancher cluster after CAPI cluster removal and no longer re-import it", func() {
		By("Removing rancher cluster record, and waiting to be removed")
		Eventually(func() bool {
			err := rancherClusterHandler.Delete(rancherCluster)
			return apierrors.IsNotFound(err)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-controllers")...).Should(BeTrue())

		By("Checking if rancher cluster record will not be created again")
		Consistently(func() bool {
			_, err := rancherClusterHandler.Get(rancherClusterKey)
			return apierrors.IsNotFound(err)
		}, e2eConfig.GetIntervals(bootstrapClusterProxy.GetName(), "wait-consistently")...).Should(BeTrue())
	})
})
