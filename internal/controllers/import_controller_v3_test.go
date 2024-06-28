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
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/turtles/internal/controllers/testdata"
	managementv3 "github.com/rancher/turtles/internal/rancher/management/v3"
	provisioningv1 "github.com/rancher/turtles/internal/rancher/provisioning/v1"
	"github.com/rancher/turtles/internal/test"
	turtlesnaming "github.com/rancher/turtles/util/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("reconcile CAPI Cluster", func() {
	var (
		r                        *CAPIImportManagementV3Reconciler
		ns                       *corev1.Namespace
		capiCluster              *clusterv1.Cluster
		rancherClusters          *managementv3.ClusterList
		rancherCluster           *managementv3.Cluster
		clusterRegistrationToken *managementv3.ClusterRegistrationToken
		v1rancherCluster         *provisioningv1.Cluster
		capiKubeconfigSecret     *corev1.Secret
		selectors                []client.ListOption
		capiClusterName          = "generated-rancher-cluster"
		sampleTemplate           string
	)
	BeforeEach(func() {
		var err error

		ns, err = testEnv.CreateNamespace(ctx, "commonns")
		Expect(err).ToNot(HaveOccurred())
		ns.Labels = map[string]string{
			importLabelName: "true",
		}
		Expect(cl.Update(ctx, ns)).To(Succeed())

		sampleTemplate = setTemplateParams(
			testdata.ImportManifest,
			map[string]string{"${TEST_CASE_NAME}": "mgmtv3"},
		)

		r = &CAPIImportManagementV3Reconciler{
			Client:             cl,
			RancherClient:      cl, // rancher and rancher-turtles deployed in the same cluster
			remoteClientGetter: remote.NewClusterClient,
			Scheme:             testEnv.GetScheme(),
		}

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      capiClusterName,
				Namespace: ns.Name,
			},
		}

		rancherCluster = &managementv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:    capiCluster.Namespace,
				GenerateName: "c-",
				Labels: map[string]string{
					capiClusterOwner:          capiCluster.Name,
					capiClusterOwnerNamespace: capiCluster.Namespace,
					ownedLabelName:            "",
				},
			},
		}

		rancherClusters = &managementv3.ClusterList{}

		selectors = []client.ListOption{
			client.MatchingLabels{
				capiClusterOwner:          capiCluster.Name,
				capiClusterOwnerNamespace: capiCluster.Namespace,
			},
		}

		clusterRegistrationToken = &managementv3.ClusterRegistrationToken{
			ObjectMeta: metav1.ObjectMeta{},
		}

		capiKubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-kubeconfig", capiCluster.Name),
				Namespace: ns.Name,
			},
			Data: map[string][]byte{
				secret.KubeconfigDataName: kubeConfigBytes,
			},
		}

		v1rancherCluster = &provisioningv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: capiCluster.Namespace,
				Name:      turtlesnaming.Name(capiCluster.Name).ToRancherName(),
			},
		}
	})

	AfterEach(func() {
		objs, err := manifestToObjects(strings.NewReader(sampleTemplate))
		clientObjs := []client.Object{
			capiCluster,
			rancherCluster,
			v1rancherCluster,
			clusterRegistrationToken,
			capiKubeconfigSecret,
		}
		for _, obj := range objs {
			clientObj, ok := obj.(client.Object)
			Expect(ok).To(BeTrue())
			clientObjs = append(clientObjs, clientObj)
		}
		Expect(err).ToNot(HaveOccurred())
		Expect(test.CleanupAndWait(ctx, cl, clientObjs...)).To(Succeed())
		Expect(testEnv.Cleanup(ctx, ns)).To(Succeed())
		for _, cluster := range rancherClusters.Items {
			testEnv.Cleanup(ctx, &cluster)
			testEnv.Cleanup(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: cluster.Name},
			},
			)
		}
	})

	It("should reconcile a CAPI cluster when control plane not ready", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.RequeueAfter).To(Equal(defaultRequeueDuration))
		}).Should(Succeed())
	})

	It("should reconcile a CAPI cluster when rancher cluster doesn't exist", func() {
		ns.Labels = map[string]string{}
		Expect(cl.Update(ctx, ns)).To(Succeed())
		capiCluster.Labels = map[string]string{
			importLabelName: "true",
			testLabelName:   testLabelVal,
		}
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Eventually(func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.Requeue).To(BeTrue())
		}).Should(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
			g.Expect(rancherClusters.Items).To(HaveLen(1))
		}).Should(Succeed())
		Expect(rancherClusters.Items[0].Name).To(ContainSubstring("c-"))
		Expect(rancherClusters.Items[0].Labels).To(HaveKeyWithValue(testLabelName, testLabelVal))
	})

	It("should reconcile a CAPI cluster when rancher cluster doesn't exist and annotation is set on the namespace", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Eventually(func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.Requeue).To(BeTrue())
		}).Should(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
			g.Expect(rancherClusters.Items).To(HaveLen(1))
		}).Should(Succeed())
		Expect(rancherClusters.Items[0].Name).To(ContainSubstring("c-"))
	})

	It("should reconcile a CAPI cluster when rancher cluster exists", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(sampleTemplate))
		}))
		defer server.Close()

		capiCluster.Labels = map[string]string{
			testLabelName: testLabelVal,
		}
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Expect(cl.Create(ctx, capiKubeconfigSecret)).To(Succeed())

		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
			g.Expect(rancherClusters.Items).To(HaveLen(1))
		}).Should(Succeed())
		cluster := rancherClusters.Items[0]
		Expect(cluster.Name).To(ContainSubstring("c-"))

		clusterRegistrationToken.Name = cluster.Name
		clusterRegistrationToken.Namespace = cluster.Name
		_, err := testEnv.CreateNamespaceWithName(ctx, cluster.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Create(ctx, clusterRegistrationToken)).To(Succeed())
		token := clusterRegistrationToken.DeepCopy()
		token.Status.ManifestURL = server.URL
		Expect(cl.Status().Update(ctx, token)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())

			objs, err := manifestToObjects(strings.NewReader(sampleTemplate))
			g.Expect(err).ToNot(HaveOccurred())

			for _, obj := range objs {
				u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
				g.Expect(err).ToNot(HaveOccurred())

				unstructuredObj := &unstructured.Unstructured{}
				unstructuredObj.SetUnstructuredContent(u)
				unstructuredObj.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

				g.Expect(cl.Get(ctx, client.ObjectKey{
					Namespace: unstructuredObj.GetNamespace(),
					Name:      unstructuredObj.GetName(),
				}, unstructuredObj)).To(Succeed())

				g.Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
				g.Expect(rancherClusters.Items).To(HaveLen(1))
				g.Expect(rancherClusters.Items[0].Name).To(ContainSubstring("c-"))
				g.Expect(rancherClusters.Items[0].Labels).To(HaveKeyWithValue(testLabelName, testLabelVal))
			}
		}, 10*time.Second).Should(Succeed())
	})

	It("should reconcile a CAPI cluster when rancher cluster exists but cluster name not set", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())
		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
			g.Expect(rancherClusters.Items).To(HaveLen(1))
		}).Should(Succeed())
		cluster := rancherClusters.Items[0]
		Expect(cluster.Name).To(ContainSubstring("c-"))

		_, err := testEnv.CreateNamespaceWithName(ctx, cluster.Name)
		Expect(err).ToNot(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.Requeue).To(BeTrue())
		}).Should(Succeed())
	})

	It("should reconcile a CAPI cluster when rancher cluster exists and agent is deployed", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
			g.Expect(rancherClusters.Items).To(HaveLen(1))
		}).Should(Succeed())
		cluster := rancherClusters.Items[0]
		Expect(cluster.Name).To(ContainSubstring("c-"))

		conditions.Set(&cluster, conditions.TrueCondition(managementv3.ClusterConditionAgentDeployed))
		Expect(conditions.IsTrue(&cluster, managementv3.ClusterConditionAgentDeployed)).To(BeTrue())
		Expect(cl.Status().Update(ctx, &cluster)).To(Succeed())

		_, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: capiCluster.Namespace,
				Name:      capiCluster.Name,
			},
		})
		Expect(err).ToNot(HaveOccurred())
	})

	It("should reconcile a CAPI cluster when rancher cluster exists and registration manifests not exist", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(""))
		}))
		defer server.Close()

		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Expect(cl.Create(ctx, capiKubeconfigSecret)).To(Succeed())

		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
			g.Expect(rancherClusters.Items).To(HaveLen(1))
		}).Should(Succeed())
		cluster := rancherClusters.Items[0]
		Expect(cluster.Name).To(ContainSubstring("c-"))

		clusterRegistrationToken.Name = cluster.Name
		clusterRegistrationToken.Namespace = cluster.Name
		_, err := testEnv.CreateNamespaceWithName(ctx, cluster.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Create(ctx, clusterRegistrationToken)).To(Succeed())
		token := clusterRegistrationToken.DeepCopy()
		token.Status.ManifestURL = server.URL
		Expect(cl.Status().Update(ctx, token)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.Requeue).To(BeTrue())
		}).Should(Succeed())
	})

	It("should reconcile a CAPI cluster when rancher cluster exists and a cluster registration token does not exist", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(""))
		}))
		defer server.Close()

		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Expect(cl.Create(ctx, capiKubeconfigSecret)).To(Succeed())

		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
			g.Expect(rancherClusters.Items).To(HaveLen(1))
		}).Should(Succeed())
		cluster := rancherClusters.Items[0]
		Expect(cluster.Name).To(ContainSubstring("c-"))

		clusterRegistrationToken.Name = cluster.Name
		clusterRegistrationToken.Namespace = cluster.Name
		_, err := testEnv.CreateNamespaceWithName(ctx, cluster.Name)
		Expect(err).ToNot(HaveOccurred())

		Eventually(ctx, func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.Requeue).To(BeTrue())
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(clusterRegistrationToken), clusterRegistrationToken)).ToNot(HaveOccurred())
		}).Should(Succeed())
	})

	It("should reconcile a CAPI cluster when rancher cluster exists and registration manifests url is empty", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Expect(cl.Create(ctx, capiKubeconfigSecret)).To(Succeed())

		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
			g.Expect(rancherClusters.Items).To(HaveLen(1))
		}).Should(Succeed())
		cluster := rancherClusters.Items[0]
		Expect(cluster.Name).To(ContainSubstring("c-"))

		clusterRegistrationToken.Name = cluster.Name
		clusterRegistrationToken.Namespace = cluster.Name
		_, err := testEnv.CreateNamespaceWithName(ctx, cluster.Name)
		Expect(err).ToNot(HaveOccurred())
		Expect(cl.Create(ctx, clusterRegistrationToken)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.Requeue).To(BeTrue())
		}).Should(Succeed())
	})

	It("should reconcile a CAPI Cluster when V1 cluster exists and is migrated", func() {
		ns.Labels = map[string]string{}
		Expect(cl.Update(ctx, ns)).To(Succeed())
		capiCluster.Labels = map[string]string{
			importLabelName: "true",
			testLabelName:   testLabelVal,
		}
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.Get(ctx, client.ObjectKeyFromObject(rancherCluster), rancherCluster)).To(Succeed())
			conditions.Set(rancherCluster, conditions.TrueCondition(managementv3.ClusterConditionAgentDeployed))
			g.Expect(conditions.IsTrue(rancherCluster, managementv3.ClusterConditionAgentDeployed)).To(BeTrue())
			g.Expect(cl.Status().Update(ctx, rancherCluster)).To(Succeed())
		}).Should(Succeed())

		v1rancherCluster.Annotations = map[string]string{
			v1ClusterMigrated: "true",
		}
		Expect(cl.Create(ctx, v1rancherCluster)).To(Succeed())

		Eventually(func(g Gomega) {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
		}).Should(Succeed())

		Eventually(ctx, func(g Gomega) {
			g.Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
			g.Expect(rancherClusters.Items).To(HaveLen(1))
		}).Should(Succeed())
		Expect(rancherClusters.Items[0].Name).To(ContainSubstring("c-"))
		Expect(rancherClusters.Items[0].Labels).To(HaveKeyWithValue(testLabelName, testLabelVal))
	})

	It("should reconcile a CAPI Cluster when V1 cluster exists and not migrated", func() {
		ns.Labels = map[string]string{}
		Expect(cl.Update(ctx, ns)).To(Succeed())
		capiCluster.Labels = map[string]string{
			importLabelName: "true",
			testLabelName:   testLabelVal,
		}
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Expect(cl.Create(ctx, v1rancherCluster)).To(Succeed())

		Eventually(func(g Gomega) {
			res, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: capiCluster.Namespace,
					Name:      capiCluster.Name,
				},
			})
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(res.Requeue).To(BeTrue())
		}).Should(Succeed())

		Eventually(ctx, func(g Gomega) {
			Expect(cl.List(ctx, rancherClusters, selectors...)).ToNot(HaveOccurred())
			Expect(rancherClusters.Items).To(HaveLen(0))
		}).Should(Succeed())
	})
})
