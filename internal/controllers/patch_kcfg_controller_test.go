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

package controllers

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	provisioningv1 "github.com/rancher/turtles/api/rancher/provisioning/v1"
	"github.com/rancher/turtles/internal/test"
)

var _ = Describe("Patch Rancher v2Prov Kubeconfig secrets", func() {
	var (
		r                *RancherKubeconfigSecretReconciler
		rancherCluster   *provisioningv1.Cluster
		kubeconfigSecret *corev1.Secret
		clusterName      string
		ns               *corev1.Namespace
	)

	BeforeEach(func() {
		var err error

		ns, err = testEnv.CreateNamespace(ctx, "v2prov")
		Expect(err).ToNot(HaveOccurred())

		r = &RancherKubeconfigSecretReconciler{
			Client: cl,
		}
		clusterName = "test1"

		rancherCluster = &provisioningv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: ns.Name,
			},
			TypeMeta: metav1.TypeMeta{
				Kind:       "Cluster",
				APIVersion: "provisioning.cattle.io/v1",
			},
			Spec: provisioningv1.ClusterSpec{
				RKEConfig: &provisioningv1.RKEConfig{},
			},
		}
		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())

		kubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-kubeconfig", clusterName),
				Namespace: ns.Name,
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "provisioning.cattle.io/v1",
						Kind:       "Cluster",
						Name:       clusterName,
						UID:        rancherCluster.UID,
					},
				},
			},
			Data: map[string][]byte{
				secret.KubeconfigDataName: kubeConfigBytes,
			},
		}
	})

	AfterEach(func() {
		clientObjs := []client.Object{
			rancherCluster,
			kubeconfigSecret,
		}
		Expect(test.CleanupAndWait(ctx, cl, clientObjs...)).To(Succeed())
		Expect(testEnv.Cleanup(ctx, ns)).To(Succeed())
	})

	It("should add label to a v2prov secret", func() {
		Expect(cl.Create(ctx, kubeconfigSecret)).To(Succeed())

		_, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: kubeconfigSecret.Namespace,
				Name:      kubeconfigSecret.Name,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		updatedSecret := &corev1.Secret{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(kubeconfigSecret), updatedSecret)).ToNot(HaveOccurred())
			g.Expect(updatedSecret.GetLabels()).To(HaveLen(1))
		}).Should(Succeed())

		labvelVal, labelFound := updatedSecret.Labels["cluster.x-k8s.io/cluster-name"]
		Expect(labelFound).To(BeTrue(), "Failed to find expected CAPI label")
		Expect(labvelVal).To(Equal(clusterName))
	})

	It("should not change anything if label already exists on v2prov secret", func() {
		kubeconfigSecret.Labels = map[string]string{
			"cluster.x-k8s.io/cluster-name": clusterName,
		}
		Expect(cl.Create(ctx, kubeconfigSecret)).To(Succeed())

		_, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: kubeconfigSecret.Namespace,
				Name:      kubeconfigSecret.Name,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		updatedSecret := &corev1.Secret{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(kubeconfigSecret), updatedSecret)).ToNot(HaveOccurred())
			g.Expect(updatedSecret.Labels).To(HaveLen(1))
			labvelVal, labelFound := updatedSecret.Labels["cluster.x-k8s.io/cluster-name"]
			g.Expect(labelFound).To(BeTrue(), "Failed to find expected CAPI label")
			g.Expect(labvelVal).To(Equal(clusterName))
			g.Expect(kubeconfigSecret.ResourceVersion).To(Equal(updatedSecret.ResourceVersion), "Secret shouldn't have been updated")
		})
	})

	It("should not change already existing labels but add label to v2prov secret", func() {
		// Add an label
		kubeconfigSecret.Labels = map[string]string{
			"existing": "myvalue",
		}
		Expect(cl.Create(ctx, kubeconfigSecret)).To(Succeed())

		Eventually(func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: kubeconfigSecret.Namespace,
					Name:      kubeconfigSecret.Name,
				},
			})
			g.Expect(err).NotTo(HaveOccurred())

			updatedSecret := &corev1.Secret{}

			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(kubeconfigSecret), updatedSecret)).ToNot(HaveOccurred())
			g.Expect(updatedSecret.GetLabels()).To(HaveLen(2))

			labvelVal, labelFound := updatedSecret.Labels["cluster.x-k8s.io/cluster-name"]
			g.Expect(labelFound).To(BeTrue(), "Failed to find expected CAPI label")
			g.Expect(labvelVal).To(Equal(clusterName))

			labvelVal, labelFound = updatedSecret.Labels["existing"]
			g.Expect(labelFound).To(BeTrue(), "Failed to find existing label")
			g.Expect(labvelVal).To(Equal("myvalue"))
		}).Should(Succeed())

	})

	It("should not add a label to a non-v2prov secret", func() {
		// Remove the owner ref on the secret
		kubeconfigSecret.OwnerReferences = []metav1.OwnerReference{}
		Expect(cl.Create(ctx, kubeconfigSecret)).To(Succeed())

		_, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: kubeconfigSecret.Namespace,
				Name:      kubeconfigSecret.Name,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Eventually(testEnv.GetAs(kubeconfigSecret, &corev1.Secret{})).Should(HaveField("Labels", HaveLen(0)))
	})

	It("should not add a label to a non-v2prov Rancher cluster secret", func() {
		rancherClusterCopy := rancherCluster.DeepCopy()
		rancherClusterCopy.Spec.RKEConfig = nil
		err := cl.Update(ctx, rancherClusterCopy)
		Expect(err).ShouldNot(HaveOccurred())

		Expect(cl.Create(ctx, kubeconfigSecret)).To(Succeed())

		_, err = r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: kubeconfigSecret.Namespace,
				Name:      kubeconfigSecret.Name,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		updatedSecret := &corev1.Secret{}
		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(kubeconfigSecret), updatedSecret)).ToNot(HaveOccurred())
			g.Expect(updatedSecret.Labels).To(HaveLen(0))
		})
	})
})
