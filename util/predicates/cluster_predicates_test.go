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
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/rancher-turtles/util/annotations"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("ClusterWithoutImportedAnnotation", func() {
	var logger logr.Logger
	BeforeEach(func() {
		// Initialize the logger
		logger = logr.Discard()
	})

	Context("when CAPI cluster has clusterImportedAnnotation", func() {
		It("should return false", func() {
			capiCluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-ns",
					Annotations: map[string]string{
						annotations.ClusterImportedAnnotation: "true",
					},
				},
			}
			result := ClusterWithoutImportedAnnotation(logger).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
			Expect(result).To(BeFalse())
		})
	})
	Context("when CAPI cluster has no annotation", func() {
		It("should return true", func() {
			capiCluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"some-other-annoations": "true",
					},
				},
			}
			capiCluster = &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "test-ns",
				},
			}
			result := ClusterWithoutImportedAnnotation(logger).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
			Expect(result).To(BeTrue())
		})
	})
	Context("when CAPI cluster has a random annotation", func() {
		It("should return true", func() {
			capiCluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{"some-random-annoation": "true"},
				},
			}
			capiCluster = &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-cluster",
					Namespace:   "test-ns",
					Annotations: map[string]string{},
				},
			}
			result := ClusterWithoutImportedAnnotation(logger).UpdateFunc(event.UpdateEvent{ObjectNew: capiCluster})
			Expect(result).To(BeTrue())
		})
	})
})

func TestClusterPredicates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ClusterPredicates Suite")
}
