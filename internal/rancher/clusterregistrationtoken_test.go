package rancher

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/rancher-turtles/internal/test"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("get cluster registration token", func() {
	var (
		clusterRegistrationTokenHandler *ClusterRegistrationTokenHandler
		clusterRegistrationToken        *ClusterRegistrationToken
	)

	BeforeEach(func() {
		clusterRegistrationTokenHandler = NewClusterRegistrationTokenHandler(ctx, cl)
		clusterRegistrationToken = &ClusterRegistrationToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
		}
	})

	AfterEach(func() {
		clusterRegistrationTokenUnstructured, err := clusterRegistrationToken.ToUnstructured()
		Expect(err).NotTo(HaveOccurred())
		Expect(test.CleanupAndWait(ctx, cl, clusterRegistrationTokenUnstructured)).To(Succeed())
	})

	It("should get cluster registration token when it exists", func() {
		Expect(clusterRegistrationTokenHandler.Create(clusterRegistrationToken)).To(Succeed())
		token, err := clusterRegistrationTokenHandler.Get(types.NamespacedName{Namespace: clusterRegistrationToken.Namespace, Name: clusterRegistrationToken.Name})
		Expect(err).NotTo(HaveOccurred())
		token.Status = ClusterRegistrationTokenStatus{
			ManifestURL: "https://test.com",
		}
		Expect(clusterRegistrationTokenHandler.UpdateStatus(token)).To(Succeed())

		token, err = clusterRegistrationTokenHandler.Get(types.NamespacedName{Namespace: clusterRegistrationToken.Namespace, Name: clusterRegistrationToken.Name})
		Expect(err).NotTo(HaveOccurred())
		Expect(token).NotTo(BeNil())
		Expect(token.Name).To(Equal("test"))
		Expect(token.Namespace).To(Equal("test"))
		Expect(token.Status.ManifestURL).To(Equal("https://test.com"))
	})

	It("fail to get cluster registration token when it doesn't exist", func() {
		token, err := clusterRegistrationTokenHandler.Get(types.NamespacedName{Namespace: clusterRegistrationToken.Namespace, Name: clusterRegistrationToken.Name})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsNotFound(err)).To(BeTrue())
		Expect(token).To(BeNil())
	})
})

var _ = Describe("create cluster registration", func() {
	var (
		clusterRegistrationTokenHandler *ClusterRegistrationTokenHandler
		clusterRegistrationToken        *ClusterRegistrationToken
	)

	BeforeEach(func() {
		clusterRegistrationTokenHandler = NewClusterRegistrationTokenHandler(ctx, cl)
		clusterRegistrationToken = &ClusterRegistrationToken{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test",
			},
			Status: ClusterRegistrationTokenStatus{
				ManifestURL: "https://test.com",
			},
		}
	})

	AfterEach(func() {
		clusterRegistrationTokenUnstructured, err := clusterRegistrationToken.ToUnstructured()
		Expect(err).NotTo(HaveOccurred())
		Expect(test.CleanupAndWait(ctx, cl, clusterRegistrationTokenUnstructured)).To(Succeed())
	})

	It("should create cluster registration", func() {
		Expect(clusterRegistrationTokenHandler.Create(clusterRegistrationToken)).To(Succeed())
	})

	It("should fail to create cluster registration when it already exists", func() {
		Expect(clusterRegistrationTokenHandler.Create(clusterRegistrationToken)).To(Succeed())
		err := clusterRegistrationTokenHandler.Create(clusterRegistrationToken)
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
	})
})
