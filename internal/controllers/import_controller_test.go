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
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/rancher-turtles/internal/controllers/testdata"
	managementv3 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/management/v3"
	provisioningv1 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/provisioning/v1"
	"github.com/rancher-sandbox/rancher-turtles/internal/test"
	turtlesnaming "github.com/rancher-sandbox/rancher-turtles/util/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("reconcile CAPI Cluster", func() {
	var (
		r                        *CAPIImportReconciler
		ns                       *corev1.Namespace
		capiCluster              *clusterv1.Cluster
		rancherCluster           *provisioningv1.Cluster
		clusterRegistrationToken *managementv3.ClusterRegistrationToken
		capiKubeconfigSecret     *corev1.Secret
		clusterName              = "generated-rancher-cluster"
	)

	BeforeEach(func() {
		var err error

		ns, err = testEnv.CreateNamespace(ctx, "commonns")
		Expect(err).ToNot(HaveOccurred())
		ns.Labels = map[string]string{
			importLabelName: "true",
		}
		Expect(cl.Update(ctx, ns)).To(Succeed())

		r = &CAPIImportReconciler{
			Client:             testEnv,
			RancherClient:      testEnv, // rancher and rancher-turtles deployed in the same cluster
			remoteClientGetter: remote.NewClusterClient,
			Scheme:             testEnv.GetScheme(),
		}

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: ns.Name,
			},
		}

		rancherCluster = &provisioningv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      turtlesnaming.Name(capiCluster.Name).ToRancherName(),
				Namespace: ns.Name,
			},
		}

		clusterRegistrationToken = &managementv3.ClusterRegistrationToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: ns.Name,
			},
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
	})

	AfterEach(func() {
		objs, err := manifestToObjects(strings.NewReader(testdata.ImportManifest))
		clientObjs := []client.Object{
			capiCluster,
			rancherCluster,
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
	})

	It("should reconcile a CAPI cluster when control plane not ready", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())

		res, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: capiCluster.Namespace,
				Name:      capiCluster.Name,
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.RequeueAfter).To(Equal(defaultRequeueDuration))
	})

	It("should reconcile a CAPI cluster when rancher cluster doesn't exist", func() {
		capiCluster.Labels = map[string]string{
			importLabelName: "true",
		}
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

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

		Eventually(testEnv.GetAs(rancherCluster, &provisioningv1.Cluster{})).ShouldNot(BeNil())
	})

	It("should reconcile a CAPI cluster when rancher cluster doesn't exist and annotation is set on the namespace", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

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

		Eventually(testEnv.GetAs(rancherCluster, &provisioningv1.Cluster{})).ShouldNot(BeNil())
	})

	It("should reconcile a CAPI cluster when rancher cluster exists", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testdata.ImportManifest))
		}))
		defer server.Close()

		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Expect(cl.Create(ctx, capiKubeconfigSecret)).To(Succeed())

		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())
		cluster := rancherCluster.DeepCopy()
		cluster.Status.ClusterName = clusterName
		Expect(cl.Status().Update(ctx, cluster)).To(Succeed())

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

			objs, err := manifestToObjects(strings.NewReader(testdata.ImportManifest))
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
			}
		}, 30*time.Second).Should(Succeed())
	})

	It("should reconcile a CAPI cluster when rancher cluster exists but cluster name not set", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())
		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())

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
		cluster := rancherCluster.DeepCopy()
		cluster.Status.AgentDeployed = true
		Expect(cl.Status().Update(ctx, cluster)).To(Succeed())

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
		cluster := rancherCluster.DeepCopy()
		cluster.Status.ClusterName = clusterName
		Expect(cl.Status().Update(ctx, cluster)).To(Succeed())

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
		cluster := rancherCluster.DeepCopy()
		cluster.Status.ClusterName = clusterName
		Expect(cl.Status().Update(ctx, cluster)).To(Succeed())

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
		cluster := rancherCluster.DeepCopy()
		cluster.Status.ClusterName = clusterName
		Expect(cl.Status().Update(ctx, cluster)).To(Succeed())

		Expect(testEnv.Create(ctx, clusterRegistrationToken)).To(Succeed())

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
})

func manifestToObjects(in io.Reader) ([]runtime.Object, error) {
	var result []runtime.Object

	reader := yamlDecoder.NewYAMLReader(bufio.NewReaderSize(in, 4096))

	for {
		raw, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, err
		}

		bytes, err := yamlDecoder.ToJSON(raw)
		if err != nil {
			return nil, err
		}

		check := map[string]interface{}{}
		if err := json.Unmarshal(bytes, &check); err != nil {
			return nil, err
		}

		if len(check) == 0 {
			continue
		}

		obj, _, err := unstructured.UnstructuredJSONScheme.Decode(bytes, nil, nil)
		if err != nil {
			return nil, err
		}

		result = append(result, obj)
	}

	return result, nil
}
