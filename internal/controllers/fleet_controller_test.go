/*
Copyright © 2023 - 2026 SUSE LLC

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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	fleetv1 "github.com/rancher/turtles/api/fleet/v1alpha1"
	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/util"
)

var _ = Describe("FleetReconciler", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
		scheme     *runtime.Scheme

		fleetReconciler FleetReconciler
		capiCluster     *clusterv1.Cluster
		fleetCluster    *fleetv1.Cluster
		rancherCluster  *managementv3.Cluster
	)

	BeforeEach(func() {
		ctx = context.TODO()
		scheme = runtime.NewScheme()

		Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
		Expect(fleetv1.AddToScheme(scheme)).To(Succeed())
		Expect(managementv3.AddToScheme(scheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		fleetReconciler = FleetReconciler{
			Client: fakeClient,
			Scheme: scheme,
		}

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "capi-cluster",
				Namespace: "capi-namespace",
				Labels: map[string]string{
					turtlesv1.LabelRancherAutoImport: "true",
				},
			},
		}
		Expect(fakeClient.Create(ctx, capiCluster)).Should(Succeed())

		rancherCluster = &managementv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-cluster",
				Labels: util.RancherClusterManagedLabels(*capiCluster),
			},
			Spec: managementv3.ClusterSpec{
				FleetWorkspaceName: "fleet-namespace",
			},
		}
		Expect(fakeClient.Create(ctx, rancherCluster)).Should(Succeed())

		fleetCluster = &fleetv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      rancherCluster.Name,
				Namespace: rancherCluster.Spec.FleetWorkspaceName,
				Labels: map[string]string{
					managementv3.LabelRancherOwnerName: rancherCluster.Name,
				},
			},
		}
		Expect(fakeClient.Create(ctx, fleetCluster)).Should(Succeed())
	})

	It("Should enqueue Fleet Clusters from CAPI Clusters", func() {
		enqueuedRequest := fleetReconciler.CAPIClusterToFleetCluster(ctx, capiCluster)
		expectedResult := []ctrl.Request{{NamespacedName: client.ObjectKeyFromObject(fleetCluster)}}

		Expect(enqueuedRequest).Should(HaveExactElements(expectedResult))
	})

	It("Should set and remove finalizer on Fleet Cluster", func() {
		request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(fleetCluster)}
		expectedResult := reconcile.Result{}

		result, err := fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		// Test finalizer is applied
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(fleetCluster), fleetCluster)).Should(Succeed())
		Expect(fleetCluster.Finalizers).Should(HaveExactElements([]string{fleetv1.FleetClusterFinalizer}))

		// Trigger deletion
		Expect(fakeClient.Delete(ctx, fleetCluster)).Should(Succeed())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(fleetCluster), fleetCluster)).Should(Succeed())
		Expect(fleetCluster.DeletionTimestamp.IsZero()).Should(BeFalse())

		result, err = fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		// Test finalizer is removed
		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(fleetCluster), fleetCluster)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
	})

	It("Should manage ClusterClass name and namespace labels on Fleet Cluster", func() {
		request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(fleetCluster)}
		expectedResult := reconcile.Result{}

		result, err := fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		// CAPI Cluster not using topology. No labels should be applied.
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(fleetCluster), fleetCluster)).Should(Succeed())
		for label := range fleetCluster.Labels {
			Expect(label).ShouldNot(Equal(turtlesv1.LabelCAPIClusterClassName))
			Expect(label).ShouldNot(Equal(turtlesv1.LabelCAPIClusterClassNamespace))
		}

		// Use ClusterClass in same Cluster namespace.
		expectedClusterClassName := "test-class"
		capiCluster.Spec = clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{
					Name: expectedClusterClassName,
				},
			},
		}
		Expect(fakeClient.Update(ctx, capiCluster)).Should(Succeed())

		result, err = fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(fleetCluster), fleetCluster)).Should(Succeed())
		Expect(fleetCluster.Labels).ShouldNot(BeEmpty())

		clusterClassName, found := fleetCluster.Labels[turtlesv1.LabelCAPIClusterClassName]
		Expect(found).Should(BeTrue(), "ClusterClass name label must be found")
		Expect(clusterClassName).Should(Equal(expectedClusterClassName))

		clusterClassNamespace, found := fleetCluster.Labels[turtlesv1.LabelCAPIClusterClassNamespace]
		Expect(found).Should(BeTrue(), "ClusterClass namespace label must be found")
		Expect(clusterClassNamespace).Should(Equal(capiCluster.Namespace))

		// Use ClusterClass cross-namespaces
		expectedClusterClassNamespace := "test-class-namespace"
		capiCluster.Spec.Topology.ClassRef.Namespace = expectedClusterClassNamespace
		Expect(fakeClient.Update(ctx, capiCluster)).Should(Succeed())

		result, err = fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(fleetCluster), fleetCluster)).Should(Succeed())
		Expect(fleetCluster.Labels).ShouldNot(BeEmpty())

		clusterClassName, found = fleetCluster.Labels[turtlesv1.LabelCAPIClusterClassName]
		Expect(found).Should(BeTrue(), "ClusterClass name label must be found")
		Expect(clusterClassName).Should(Equal(expectedClusterClassName))

		clusterClassNamespace, found = fleetCluster.Labels[turtlesv1.LabelCAPIClusterClassNamespace]
		Expect(found).Should(BeTrue(), "ClusterClass namespace label must be found")
		Expect(clusterClassNamespace).Should(Equal(expectedClusterClassNamespace))
	})

	It("Should manage Fleet BundleNamespaceMapping when cross-namespace ClusterClass", func() {
		request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(fleetCluster)}
		expectedResult := reconcile.Result{}

		result, err := fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		// CAPI Cluster not using topology. No BundleNamespaceMapping should have been created.
		bundleNamespaceMappingList := &fleetv1.BundleNamespaceMappingList{}
		Expect(fakeClient.List(ctx, bundleNamespaceMappingList)).Should(Succeed())
		Expect(bundleNamespaceMappingList.Items).Should(BeEmpty())

		// Use ClusterClass in same Fleet Cluster namespace. No BundleNamespaceMapping should have been created.
		capiCluster.Spec = clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{
					Name:      "test-class",
					Namespace: fleetCluster.Namespace,
				},
			},
		}
		Expect(fakeClient.Update(ctx, capiCluster)).Should(Succeed())

		result, err = fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		Expect(fakeClient.List(ctx, bundleNamespaceMappingList)).Should(Succeed())
		Expect(bundleNamespaceMappingList.Items).Should(BeEmpty())

		// Use ClusterClass cross-namespaces
		expectedClusterClassNamespace := "test-class-namespace"
		capiCluster.Spec.Topology.ClassRef.Namespace = expectedClusterClassNamespace
		Expect(fakeClient.Update(ctx, capiCluster)).Should(Succeed())

		result, err = fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		// Test BundleNamespaceMapping has been created
		bundleNamespaceMapping := &fleetv1.BundleNamespaceMapping{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fleetCluster.Namespace,
				Namespace: expectedClusterClassNamespace,
			},
		}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(bundleNamespaceMapping), bundleNamespaceMapping)).
			Should((Succeed()))
		Expect(bundleNamespaceMapping.NamespaceSelector).ShouldNot(BeNil())
		Expect(bundleNamespaceMapping.NamespaceSelector.MatchLabels).ShouldNot(BeEmpty())
		targetNamespace, found := bundleNamespaceMapping.NamespaceSelector.MatchLabels[corev1.LabelMetadataName]
		Expect(found).Should(BeTrue(), "matchLabels selector must contain kubernetes.io/metadata.name label")
		Expect(targetNamespace).Should(Equal(fleetCluster.Namespace))

		// Test BundleNamespaceMapping has been deleted
		Expect(fakeClient.Delete(ctx, fleetCluster))

		result, err = fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(bundleNamespaceMapping), bundleNamespaceMapping)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
	})

	It("Should not delete BundleNamespaceMapping when in use by other Fleet Clusters", func() {
		request := ctrl.Request{NamespacedName: client.ObjectKeyFromObject(fleetCluster)}
		expectedResult := reconcile.Result{}

		// Use ClusterClass cross-namespaces
		expectedClusterClassNamespace := "test-class-namespace"
		capiCluster.Spec = clusterv1.ClusterSpec{
			Topology: clusterv1.Topology{
				ClassRef: clusterv1.ClusterClassRef{
					Name:      "test-class",
					Namespace: expectedClusterClassNamespace,
				},
			},
		}
		Expect(fakeClient.Update(ctx, capiCluster)).Should(Succeed())

		// Create a second CAPI Cluster that uses the same ClusterClass namespace
		secondCapiCluster := &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "capi-cluster-2",
				Namespace: "capi-namespace",
			},
		}
		Expect(fakeClient.Create(ctx, secondCapiCluster)).Should(Succeed())

		secondRancherCluster := &managementv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-cluster-2",
				Labels: util.RancherClusterManagedLabels(*secondCapiCluster),
			},
			Spec: managementv3.ClusterSpec{
				FleetWorkspaceName: "fleet-namespace",
			},
		}
		Expect(fakeClient.Create(ctx, secondRancherCluster)).Should(Succeed())

		secondFleetCluster := &fleetv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:       secondRancherCluster.Name,
				Namespace:  secondRancherCluster.Spec.FleetWorkspaceName,
				Finalizers: []string{fleetv1.FleetClusterFinalizer},
				Labels: map[string]string{
					managementv3.LabelRancherOwnerName:       secondRancherCluster.Name,
					turtlesv1.LabelCAPIClusterClassNamespace: expectedClusterClassNamespace,
				},
			},
		}
		Expect(fakeClient.Create(ctx, secondFleetCluster)).Should(Succeed())

		result, err := fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		// Test BundleNamespaceMapping has been created
		bundleNamespaceMapping := &fleetv1.BundleNamespaceMapping{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fleetCluster.Namespace,
				Namespace: expectedClusterClassNamespace,
			},
		}
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(bundleNamespaceMapping), bundleNamespaceMapping)).
			Should((Succeed()))

		// Trigger first Fleet Cluster deletion. BundleNamespaceMapping should persist.
		Expect(fakeClient.Delete(ctx, fleetCluster)).Should(Succeed())

		result, err = fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(bundleNamespaceMapping), bundleNamespaceMapping)).
			Should((Succeed()))

		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(fleetCluster), fleetCluster)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())

		// Trigger second Fleet Cluster deletion. Orphan BundleNamespaceMapping should be deleted.
		Expect(fakeClient.Delete(ctx, secondFleetCluster)).Should(Succeed())
		request = ctrl.Request{NamespacedName: client.ObjectKeyFromObject(secondFleetCluster)}

		result, err = fleetReconciler.Reconcile(ctx, request)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(result).Should(Equal(expectedResult))

		err = fakeClient.Get(ctx, client.ObjectKeyFromObject(bundleNamespaceMapping), bundleNamespaceMapping)
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
	})
})
