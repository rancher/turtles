package rancher

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/rancher-turtles/internal/test"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("get rancher cluster", func() {
	var (
		rancherClusterHandler *ClusterHandler
		ranchercluster        *Cluster
	)

	BeforeEach(func() {
		rancherClusterHandler = NewClusterHandler(ctx, cl)
		ranchercluster = &Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Status: ClusterStatus{
				ClusterName:   "test-cluster",
				AgentDeployed: true,
			},
		}

	})

	AfterEach(func() {
		rancherClusterUnstructured, err := ranchercluster.ToUnstructured()
		Expect(err).NotTo(HaveOccurred())
		Expect(test.CleanupAndWait(ctx, cl, rancherClusterUnstructured)).To(Succeed())
	})

	It("should get rancher cluster when it exists", func() {
		Expect(rancherClusterHandler.Create(ranchercluster)).To(Succeed())
		cluster, err := rancherClusterHandler.Get(types.NamespacedName{Namespace: ranchercluster.Namespace, Name: ranchercluster.Name})
		Expect(err).NotTo(HaveOccurred())
		cluster.Status = ClusterStatus{
			ClusterName:   "test-cluster",
			AgentDeployed: true,
		}
		Expect(rancherClusterHandler.UpdateStatus(cluster))

		cluster, err = rancherClusterHandler.Get(types.NamespacedName{Namespace: ranchercluster.Namespace, Name: ranchercluster.Name})
		Expect(err).NotTo(HaveOccurred())
		Expect(cluster).NotTo(BeNil())
		Expect(cluster.Name).To(Equal("test"))
		Expect(cluster.Namespace).To(Equal("test"))
		Expect(cluster.Status.ClusterName).To(Equal("test-cluster"))
		Expect(cluster.Status.AgentDeployed).To(BeTrue())
	})

	It("fail to get rancher cluster when it doesn't exist", func() {
		cluster, err := rancherClusterHandler.Get(types.NamespacedName{Namespace: ranchercluster.Namespace, Name: ranchercluster.Name})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(cluster).To(BeNil())
	})
})

var _ = Describe("create rancher cluster", func() {
	var (
		rancherClusterHandler *ClusterHandler
		ranchercluster        *Cluster
	)

	BeforeEach(func() {
		rancherClusterHandler = NewClusterHandler(ctx, cl)
		ranchercluster = &Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		}
	})

	AfterEach(func() {
		rancherClusterUnstructured, err := ranchercluster.ToUnstructured()
		Expect(err).NotTo(HaveOccurred())
		Expect(test.CleanupAndWait(ctx, cl, rancherClusterUnstructured)).To(Succeed())
	})

	It("should create rancher cluster", func() {
		Expect(rancherClusterHandler.Create(ranchercluster)).To(Succeed())
	})

	It("should fail to create rancher cluster when it already exists", func() {
		Expect(rancherClusterHandler.Create(ranchercluster)).To(Succeed())
		err := rancherClusterHandler.Create(ranchercluster)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
	})
})

var _ = Describe("delete rancher cluster", func() {
	var (
		rancherClusterHandler *ClusterHandler
		ranchercluster        *Cluster
	)

	BeforeEach(func() {
		rancherClusterHandler = NewClusterHandler(ctx, cl)
		ranchercluster = &Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		}
		Expect(rancherClusterHandler.Create(ranchercluster)).To(Succeed())
	})

	It("should delete rancher cluster", func() {
		Expect(rancherClusterHandler.Delete(ranchercluster)).To(Succeed())
	})
})
