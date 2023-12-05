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

	It("Status patch only updates the status of the resource", func() {
		capiProvider.Spec.Name = turtlesv1.RKE2
		capiProvider.Status.Phase = turtlesv1.Ready

		Expect(PatchStatus(ctx, testEnv, capiProvider)).To(Succeed())
		Eventually(Object(capiProvider)).Should(HaveField("Status.Phase", Equal(turtlesv1.Ready)))
		Eventually(Object(capiProvider)).Should(HaveField("Spec.Name", Equal(turtlesv1.Docker)))
	})

	It("Regular patch only updates the spec of the resource", func() {
		capiProvider.Spec.Name = turtlesv1.RKE2
		capiProvider.Status.Phase = turtlesv1.Ready

		Expect(Patch(ctx, testEnv, capiProvider)).To(Succeed())
		Eventually(Object(capiProvider)).Should(HaveField("Spec.Name", Equal(turtlesv1.RKE2)))
		Eventually(Object(capiProvider)).Should(HaveField("Status.Phase", BeEmpty()))
	})
})
