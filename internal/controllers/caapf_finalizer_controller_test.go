/*
Copyright © 2026 SUSE LLC

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"

	fleetv1 "github.com/rancher/turtles/api/fleet/v1alpha1"
	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
)

var _ = Describe("CAAPFFinalizerReconciler", func() {
	const (
		namespace   = "capi-clusters"
		clusterName = "cluster1"
	)

	var (
		ctx         context.Context
		scheme      *runtime.Scheme
		capiCluster *clusterv1.Cluster
		request     ctrl.Request
	)

	capiOwnerRef := func(name string) metav1.OwnerReference {
		return metav1.OwnerReference{
			APIVersion: clusterv1.GroupVersion.String(),
			Kind:       "Cluster",
			Name:       name,
			UID:        "capi-uid",
		}
	}

	fleetClusterOwnedBy := func(ns string, owner metav1.OwnerReference, labels map[string]string) *fleetv1.Cluster {
		return &fleetv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "caapf-" + owner.Name,
				Namespace:       ns,
				Labels:          labels,
				OwnerReferences: []metav1.OwnerReference{owner},
			},
		}
	}

	newReconciler := func(objs ...client.Object) (*CAAPFFinalizerReconciler, client.Client) {
		c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

		return &CAAPFFinalizerReconciler{Client: c, Scheme: scheme}, c
	}

	getCAPICluster := func(c client.Client) *clusterv1.Cluster {
		cluster := &clusterv1.Cluster{}
		Expect(c.Get(ctx, request.NamespacedName, cluster)).To(Succeed())

		return cluster
	}

	BeforeEach(func() {
		ctx = context.TODO()
		scheme = runtime.NewScheme()
		Expect(clusterv1.AddToScheme(scheme)).To(Succeed())
		Expect(fleetv1.AddToScheme(scheme)).To(Succeed())

		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:       clusterName,
				Namespace:  namespace,
				UID:        "capi-uid",
				Finalizers: []string{FleetAddonFinalizer},
			},
		}
		request = ctrl.Request{NamespacedName: types.NamespacedName{Namespace: namespace, Name: clusterName}}
	})

	It("ignores clusters outside the watch filter", func() {
		r, c := newReconciler(capiCluster)
		r.WatchFilterValue = "mine"

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())
		Expect(getCAPICluster(c).Finalizers).To(ContainElement(FleetAddonFinalizer))
	})

	It("ignores Fleet clusters owned by a CAPI cluster with a stale UID", func() {
		stale := capiOwnerRef(clusterName)
		stale.UID = "old-uid"
		r, c := newReconciler(capiCluster, fleetClusterOwnedBy(namespace, stale, nil))

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())

		got := &clusterv1.Cluster{}
		Expect(c.Get(ctx, request.NamespacedName, got)).To(Succeed())
		Expect(got.Finalizers).ToNot(ContainElement(FleetAddonFinalizer))
	})

	It("should remove the finalizer when no CAAPF-owned Fleet cluster exists", func() {
		r, c := newReconciler(capiCluster)

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())
		Expect(getCAPICluster(c).Finalizers).ToNot(ContainElement(FleetAddonFinalizer))
	})

	It("should remove the finalizer from a cluster with the imported=true annotation", func() {
		capiCluster.Annotations = map[string]string{turtlesannotations.ClusterImportedAnnotation: "true"}
		r, c := newReconciler(capiCluster)

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())
		Expect(getCAPICluster(c).Finalizers).ToNot(ContainElement(FleetAddonFinalizer))
	})

	It("should keep other finalizers", func() {
		capiCluster.Finalizers = []string{managementv3.CapiClusterFinalizer, FleetAddonFinalizer}
		r, c := newReconciler(capiCluster)

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())
		Expect(getCAPICluster(c).Finalizers).To(HaveExactElements(managementv3.CapiClusterFinalizer))
	})

	It("should keep the finalizer while a CAAPF-owned Fleet cluster exists (not migrated)", func() {
		r, c := newReconciler(capiCluster, fleetClusterOwnedBy(namespace, capiOwnerRef(clusterName), nil))

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())
		Expect(getCAPICluster(c).Finalizers).To(ContainElement(FleetAddonFinalizer))
	})

	It("should keep the finalizer while a CAAPF-owned Fleet cluster is labelled by the pre phase", func() {
		labels := map[string]string{"migration.fleet.cattle.io/caapf-managed": "true"}
		r, c := newReconciler(capiCluster, fleetClusterOwnedBy(namespace, capiOwnerRef(clusterName), labels))

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())
		Expect(getCAPICluster(c).Finalizers).To(ContainElement(FleetAddonFinalizer))
	})

	It("should ignore Fleet clusters not owned by this CAPI cluster", func() {
		rancherOwned := fleetClusterOwnedBy(namespace, metav1.OwnerReference{
			APIVersion: managementv3.SchemeGroupVersion.String(),
			Kind:       "Cluster",
			Name:       clusterName,
			UID:        "rancher-uid",
		}, nil)
		otherCAPICluster := fleetClusterOwnedBy(namespace, capiOwnerRef("cluster2"), nil)
		otherNamespace := fleetClusterOwnedBy("other", capiOwnerRef(clusterName), nil)

		r, c := newReconciler(capiCluster, rancherOwned, otherCAPICluster, otherNamespace)

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())
		Expect(getCAPICluster(c).Finalizers).ToNot(ContainElement(FleetAddonFinalizer))
	})

	It("should not block deletion of a cluster that has not been migrated", func() {
		r, c := newReconciler(capiCluster, fleetClusterOwnedBy(namespace, capiOwnerRef(clusterName), nil))
		Expect(c.Delete(ctx, capiCluster)).To(Succeed())

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())

		err = c.Get(ctx, request.NamespacedName, &clusterv1.Cluster{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "CAPI cluster should be deleted")
	})

	It("should keep other finalizers of a cluster being deleted", func() {
		capiCluster.Finalizers = []string{managementv3.CapiClusterFinalizer, FleetAddonFinalizer}
		r, c := newReconciler(capiCluster, fleetClusterOwnedBy(namespace, capiOwnerRef(clusterName), nil))
		Expect(c.Delete(ctx, capiCluster)).To(Succeed())

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())

		cluster := getCAPICluster(c)
		Expect(cluster.DeletionTimestamp.IsZero()).To(BeFalse())
		Expect(cluster.Finalizers).To(HaveExactElements(managementv3.CapiClusterFinalizer))
	})

	It("should unblock deletion of a migrated cluster", func() {
		r, c := newReconciler(capiCluster)
		Expect(c.Delete(ctx, capiCluster)).To(Succeed())

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())

		err = c.Get(ctx, request.NamespacedName, &clusterv1.Cluster{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "CAPI cluster should be deleted")
	})

	It("should remove the finalizer once the CAAPF-owned Fleet cluster is deleted", func() {
		fleetCluster := fleetClusterOwnedBy(namespace, capiOwnerRef(clusterName), nil)
		r, c := newReconciler(capiCluster, fleetCluster)

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())
		Expect(getCAPICluster(c).Finalizers).To(ContainElement(FleetAddonFinalizer))

		Expect(c.Delete(ctx, fleetCluster)).To(Succeed())

		_, err = r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())
		Expect(getCAPICluster(c).Finalizers).ToNot(ContainElement(FleetAddonFinalizer))
	})

	Context("CAAPF BundleNamespaceMapping cleanup", func() {
		const classNamespace = "capi-classes"

		withClassNamespace := func(cluster *clusterv1.Cluster, ns string) *clusterv1.Cluster {
			cluster.Spec.Topology.ClassRef = clusterv1.ClusterClassRef{Name: "quick-start", Namespace: ns}

			return cluster
		}

		sibling := func(name string, finalizers ...string) *clusterv1.Cluster {
			return withClassNamespace(&clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Finalizers: finalizers},
			}, classNamespace)
		}

		mapping := func(ns string) *fleetv1.BundleNamespaceMapping {
			return &fleetv1.BundleNamespaceMapping{Name: namespace, Namespace: ns}
		}

		mappingExists := func(c client.Client, ns string) bool {
			err := c.Get(ctx, client.ObjectKeyFromObject(mapping(ns)), &fleetv1.BundleNamespaceMapping{})
			if apierrors.IsNotFound(err) {
				return false
			}
			Expect(err).ToNot(HaveOccurred())

			return true
		}

		deleteAndReconcile := func(objs ...client.Object) client.Client {
			r, c := newReconciler(objs...)
			Expect(c.Delete(ctx, capiCluster)).To(Succeed())

			_, err := r.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())

			return c
		}

		BeforeEach(func() {
			withClassNamespace(capiCluster, classNamespace)
		})

		It("should delete the mapping of the last CAAPF-managed cluster being deleted", func() {
			c := deleteAndReconcile(capiCluster, mapping(classNamespace), fleetClusterOwnedBy(namespace, capiOwnerRef(clusterName), nil))
			Expect(mappingExists(c, classNamespace)).To(BeFalse())
		})

		It("should keep the mapping while another CAAPF-managed cluster uses it", func() {
			c := deleteAndReconcile(capiCluster, mapping(classNamespace), sibling("cluster2", FleetAddonFinalizer))
			Expect(mappingExists(c, classNamespace)).To(BeTrue())
		})

		It("should delete the mapping when the other clusters do not use it", func() {
			beingDeleted := sibling("deleting", FleetAddonFinalizer)
			beingDeleted.DeletionTimestamp = new(metav1.Now())
			otherClassNamespace := withClassNamespace(sibling("other-class-ns", FleetAddonFinalizer), "other")

			c := deleteAndReconcile(capiCluster, mapping(classNamespace), beingDeleted, otherClassNamespace,
				sibling("not-caapf-managed"))
			Expect(mappingExists(c, classNamespace)).To(BeFalse())
		})

		It("should not delete a mapping for a ClusterClass in the cluster namespace", func() {
			withClassNamespace(capiCluster, namespace)
			c := deleteAndReconcile(capiCluster, mapping(namespace))
			Expect(mappingExists(c, namespace)).To(BeTrue())
		})

		It("should unblock deletion when the mapping is already gone", func() {
			c := deleteAndReconcile(capiCluster)

			err := c.Get(ctx, request.NamespacedName, &clusterv1.Cluster{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(), "CAPI cluster should be deleted")
		})

		It("should not delete the mapping of a cluster that is not being deleted", func() {
			r, c := newReconciler(capiCluster, mapping(classNamespace))

			_, err := r.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(mappingExists(c, classNamespace)).To(BeTrue())
		})
	})

	It("should ignore a CAPI cluster that no longer exists", func() {
		r, _ := newReconciler()

		_, err := r.Reconcile(ctx, request)
		Expect(err).ToNot(HaveOccurred())
	})

	It("should only pass Fleet cluster updates that change the CAPI cluster owner", func() {
		p := fleetClusterOwnerChanged()
		owned := fleetClusterOwnedBy(namespace, capiOwnerRef(clusterName), nil)

		// Turtles' Fleet Cluster type has no status, so a status update only shows as a new resource version.
		statusUpdate := owned.DeepCopy()
		statusUpdate.ResourceVersion = "2"
		Expect(p.Update(event.UpdateEvent{ObjectOld: owned, ObjectNew: statusUpdate})).To(BeFalse())

		labelled := owned.DeepCopy()
		labelled.Labels = map[string]string{"migration.fleet.cattle.io/caapf-managed": "true"}
		Expect(p.Update(event.UpdateEvent{ObjectOld: owned, ObjectNew: labelled})).To(BeFalse())

		ownerRemoved := owned.DeepCopy()
		ownerRemoved.OwnerReferences = nil
		Expect(p.Update(event.UpdateEvent{ObjectOld: owned, ObjectNew: ownerRemoved})).To(BeTrue())

		ownerReplaced := owned.DeepCopy()
		ownerReplaced.OwnerReferences[0].UID = "new-uid"
		Expect(p.Update(event.UpdateEvent{ObjectOld: owned, ObjectNew: ownerReplaced})).To(BeTrue())

		Expect(p.Delete(event.DeleteEvent{Object: owned})).To(BeTrue())
		Expect(p.Create(event.CreateEvent{Object: owned})).To(BeTrue())
	})

	It("should map a Fleet cluster to its owning CAPI cluster", func() {
		Expect(fleetClusterToCAPIOwner(ctx, fleetClusterOwnedBy(namespace, capiOwnerRef(clusterName), nil))).
			To(HaveExactElements(request))

		Expect(fleetClusterToCAPIOwner(ctx, &fleetv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "fleet", Namespace: namespace},
		})).To(BeEmpty())
	})
})
