package annotations

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ = Describe("ClusterWithoutImportedAnnotation", func() {
	BeforeEach(func() {
		//
	})

	Context("when object has specifed annotation", func() {
		It("should return true", func() {
			obj := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"test-annotation": "value",
					},
				},
			}
			result := HasAnnotation(obj, "test-annotation")
			Expect(result).To(BeTrue())
		})
	})

	Context("when object does not have specifed annotationn", func() {
		It("should return false", func() {
			obj := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-obj",
					Namespace: "test-ns",
					Annotations: map[string]string{
						"some-other-annotation": "value",
					},
				},
			}
			result := HasAnnotation(obj, "test-annotation")
			Expect(result).To(BeFalse())
		})
	})

	Context("when object has no annotations", func() {
		It("should return false", func() {
			obj := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{},
			}
			result := HasAnnotation(obj, "test-annotation")
			Expect(result).To(BeFalse())
		})
	})
})

func TestAnnotationHelpers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AnnotationHelpers Suite")
}
