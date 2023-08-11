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

package predicates

import (
	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/rancher-turtles/util/annotations"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("ClusterWithoutImportedAnnotation", func() {
	var (
		logger      logr.Logger
		capiCluster *clusterv1.Cluster
	)

	BeforeEach(func() {
		// Initialize the logger
		logger = logr.Discard()

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-ns",
			},
		}
	})

	Context("when CAPI cluster has clusterImportedAnnotation", func() {
		It("should return false", func() {
			capiCluster.Annotations = map[string]string{
				annotations.ClusterImportedAnnotation: "true",
			}
			result := ClusterWithoutImportedAnnotation(logger).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
			Expect(result).To(BeFalse())
		})
	})
	Context("when CAPI cluster has no annotation", func() {
		It("should return true", func() {
			result := ClusterWithoutImportedAnnotation(logger).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
			Expect(result).To(BeTrue())
		})
	})
	Context("when CAPI cluster has a random annotation", func() {
		It("should return true", func() {
			capiCluster.Annotations = map[string]string{
				"some-random-annotation": "true",
			}
			result := ClusterWithoutImportedAnnotation(logger).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
			Expect(result).To(BeTrue())
		})
	})
})

var _ = Describe("ClusterWithReadyControlPlane", func() {
	var (
		logger      logr.Logger
		capiCluster *clusterv1.Cluster
	)

	BeforeEach(func() {
		// Initialize the logger
		logger = logr.Discard()

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: "test-ns",
			},
		}
	})

	It("should return true when cluster has ready control plane", func() {
		capiCluster.Status.ControlPlaneReady = true
		result := ClusterWithReadyControlPlane(logger).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
		Expect(result).To(BeTrue())
	})

	It("should return false when cluster does not have ready control plane", func() {
		result := ClusterWithReadyControlPlane(logger).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
		Expect(result).To(BeFalse())
	})
})

var _ = Describe("ClusterOrNamespaceWithImportLabel", func() {
	var (
		logger      logr.Logger
		capiCluster *clusterv1.Cluster
		namespace   *corev1.Namespace
	)

	BeforeEach(func() {
		// Initialize the logger
		logger = logr.Discard()

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-cluster",
			},
		}

		namespace = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					importLabel: "true",
				},
			},
		}
	})

	It("should return true when cluster has import label", func() {
		capiCluster.Labels = map[string]string{
			importLabel: "true",
		}
		result := ClusterOrNamespaceWithImportLabel(ctx, logger, cl, importLabel).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
		Expect(result).To(BeTrue())
	})

	It("should return true when namespace has import label", func() {
		namespace.Name = "test-ns-1"
		Expect(cl.Create(ctx, namespace)).To(Succeed())

		capiCluster.Namespace = namespace.Name
		result := ClusterOrNamespaceWithImportLabel(ctx, logger, cl, importLabel).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
		Expect(result).To(BeTrue())
	})

	It("should return true if client fails to get namespace", func() {
		namespace.Name = "non-existent-ns"
		capiCluster.Namespace = namespace.Name
		result := ClusterOrNamespaceWithImportLabel(ctx, logger, cl, importLabel).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
		Expect(result).To(BeTrue())
	})

	It("should return false when cluster and namespace have no import label", func() {
		namespace.Name = "test-ns-2"
		namespace.Labels = nil
		Expect(cl.Create(ctx, namespace)).To(Succeed())

		capiCluster.Namespace = namespace.Name

		result := ClusterOrNamespaceWithImportLabel(ctx, logger, cl, importLabel).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
		Expect(result).To(BeFalse())
	})
})
