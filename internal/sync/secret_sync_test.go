package sync_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesv1 "github.com/rancher-sandbox/rancher-turtles/api/v1alpha1"
	"github.com/rancher-sandbox/rancher-turtles/internal/sync"
	corev1 "k8s.io/api/core/v1"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
)

var _ = Describe("Provider sync", func() {
	var (
		err          error
		ns           *corev1.Namespace
		capiProvider *turtlesv1.CAPIProvider
		secret       *corev1.Secret
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
			Name: "docker",
			Type: turtlesv1.Infrastructure,
			ProviderSpec: operatorv1.ProviderSpec{
				ConfigSecret: &operatorv1.SecretReference{
					Name: "test",
				},
			},
			Variables: map[string]string{
				"variable": "one",
			},
			Features: &turtlesv1.Features{
				ClusterTopology: true,
			},
		}}

		secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      string(capiProvider.Spec.ProviderSpec.ConfigSecret.Name),
			Namespace: capiProvider.Namespace,
		}}

		Expect(testEnv.Client.Create(ctx, capiProvider)).To(Succeed())
	})

	AfterEach(func() {
		testEnv.Cleanup(ctx, ns)
	})

	It("Should sync spec down", func() {
		s := sync.NewSecretSync(testEnv, capiProvider.DeepCopy())
		Eventually(s.Get(ctx)).Should(Succeed())

		Expect(s.Sync(ctx)).To(Succeed())

		var err error
		s.Apply(ctx, &err)
		Expect(err).To(Succeed())

		Eventually(Object(secret)).Should(
			HaveField("Data", HaveKey("variable")))
		Eventually(Object(secret)).Should(
			HaveField("Data", HaveKey("CLUSTER_TOPOLOGY")))
	})
})
