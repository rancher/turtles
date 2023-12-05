package sync

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesv1 "github.com/rancher-sandbox/rancher-turtles/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Core provider", func() {
	var (
		err          error
		ns           *corev1.Namespace
		capiProvider *turtlesv1.CAPIProvider
	)

	BeforeEach(func() {
		SetClient(testEnv)
		SetContext(ctx)

		ns, err = testEnv.CreateNamespace(ctx, "ns")
		Expect(err).ToNot(HaveOccurred())

		capiProvider = &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Name: turtlesv1.Docker,
			Type: turtlesv1.Core,
		}}

		Expect(testEnv.Client.Create(ctx, capiProvider)).To(Succeed())
	})

	AfterEach(func() {
		testEnv.Cleanup(ctx, ns)
	})

	It("Should repetively sync any object to the cluster", func() {
		secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
		}}
		s := NewDefaultSyncer(testEnv, capiProvider, secret)
		Expect(s.Get(ctx)).To(Succeed())

		secret.StringData = map[string]string{
			"something": "added",
		}

		var err error
		s.Apply(ctx, &err)
		Expect(err).To(Succeed())

		Eventually(Object(secret)).Should(
			HaveField("Data", HaveKey("something")))
		Eventually(Object(secret)).Should(
			HaveField("OwnerReferences", HaveLen(1)))

		Expect(s.Get(ctx)).To(Succeed())
		secret.StringData = map[string]string{
			"removed": "something",
		}
		s.Apply(ctx, &err)
		Expect(err).To(Succeed())

		Eventually(Object(secret)).Should(
			HaveField("Data", HaveKey("removed")))
		Eventually(Object(secret)).Should(
			HaveField("OwnerReferences", HaveLen(1)))
	})
})
