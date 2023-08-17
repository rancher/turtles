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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/rancher-turtles/internal/controllers/testdata"
	managementv3 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/management/v3"
	provisioningv1 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/provisioning/v1"
	"github.com/rancher-sandbox/rancher-turtles/internal/test"
	turtelesnaming "github.com/rancher-sandbox/rancher-turtles/util/naming"
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
		capiCluster              *clusterv1.Cluster
		rancherCluster           *provisioningv1.Cluster
		clusterRegistrationToken *managementv3.ClusterRegistrationToken
		capiKubeconfigSecret     *corev1.Secret
	)

	BeforeEach(func() {
		r = &CAPIImportReconciler{
			Client:             cl,
			RancherClient:      cl, // rancher and rancher-turtles deployed in the same cluster
			remoteClientGetter: remote.NewClusterClient,
		}

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: testNamespace,
			},
		}

		rancherCluster = &provisioningv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      turtelesnaming.Name(capiCluster.Name).ToRancherName(),
				Namespace: testNamespace,
			},
		}

		clusterRegistrationToken = &managementv3.ClusterRegistrationToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterRegistrationTokenName,
				Namespace: testNamespace,
			},
		}

		capiKubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-kubeconfig", capiCluster.Name),
				Namespace: testNamespace,
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

		res, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: capiCluster.Namespace,
				Name:      capiCluster.Name,
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeTrue())

		cluster := &provisioningv1.Cluster{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(rancherCluster), cluster)).ToNot(HaveOccurred())
	})

	It("should reconcile a CAPI cluster when rancher cluster doesn't exist and annotation is set on the namespace", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		res, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: capiCluster.Namespace,
				Name:      capiCluster.Name,
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeTrue())

		cluster := &provisioningv1.Cluster{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(rancherCluster), cluster)).ToNot(HaveOccurred())
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
		cluster := &provisioningv1.Cluster{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(rancherCluster), cluster)).To(Succeed())
		cluster.Status.ClusterName = testNamespace
		Expect(cl.Status().Update(ctx, cluster)).To(Succeed())

		Expect(cl.Create(ctx, clusterRegistrationToken)).To(Succeed())
		token := &managementv3.ClusterRegistrationToken{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(clusterRegistrationToken), token)).To(Succeed())
		token.Status.ManifestURL = server.URL
		Expect(cl.Status().Update(ctx, token)).To(Succeed())

		_, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: capiCluster.Namespace,
				Name:      capiCluster.Name,
			},
		})
		Expect(err).ToNot(HaveOccurred())

		objs, err := manifestToObjects(strings.NewReader(testdata.ImportManifest))
		Expect(err).ToNot(HaveOccurred())

		for _, obj := range objs {
			u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
			Expect(err).ToNot(HaveOccurred())

			unstructuredObj := &unstructured.Unstructured{}
			unstructuredObj.SetUnstructuredContent(u)
			unstructuredObj.SetGroupVersionKind(obj.GetObjectKind().GroupVersionKind())

			Expect(cl.Get(ctx, client.ObjectKey{
				Namespace: unstructuredObj.GetNamespace(),
				Name:      unstructuredObj.GetName(),
			}, unstructuredObj)).To(Succeed())
		}
	})

	It("should reconcile a CAPI cluster when rancher cluster exists but cluster name not set", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())
		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())

		res, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: capiCluster.Namespace,
				Name:      capiCluster.Name,
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeTrue())
	})

	It("should reconcile a CAPI cluster when rancher cluster exists and agent is deployed", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())
		cluster := &provisioningv1.Cluster{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(rancherCluster), cluster)).ToNot(HaveOccurred())
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
		cluster := &provisioningv1.Cluster{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(rancherCluster), cluster)).ToNot(HaveOccurred())
		cluster.Status.ClusterName = testNamespace
		Expect(cl.Status().Update(ctx, cluster)).To(Succeed())

		Expect(cl.Create(ctx, clusterRegistrationToken)).To(Succeed())
		token := &managementv3.ClusterRegistrationToken{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(clusterRegistrationToken), token)).To(Succeed())
		token.Status.ManifestURL = server.URL
		Expect(cl.Status().Update(ctx, token)).To(Succeed())

		res, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: capiCluster.Namespace,
				Name:      capiCluster.Name,
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeTrue())
	})

	It("should reconcile a CAPI cluster when rancher cluster exists and registration manifests url is empty", func() {
		Expect(cl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(cl.Status().Update(ctx, capiCluster)).To(Succeed())

		Expect(cl.Create(ctx, capiKubeconfigSecret)).To(Succeed())

		Expect(cl.Create(ctx, rancherCluster)).To(Succeed())
		cluster := &provisioningv1.Cluster{}
		Expect(cl.Get(ctx, client.ObjectKeyFromObject(rancherCluster), cluster)).ToNot(HaveOccurred())
		cluster.Status.ClusterName = testNamespace
		Expect(cl.Status().Update(ctx, cluster)).To(Succeed())

		Expect(cl.Create(ctx, clusterRegistrationToken)).To(Succeed())

		res, err := r.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: capiCluster.Namespace,
				Name:      capiCluster.Name,
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(res.Requeue).To(BeTrue())
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
