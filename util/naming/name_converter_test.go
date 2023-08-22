package naming

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Cluster name mapping", func() {

	It("Should suffix rancher cluster name with -capi", func() {
		name := Name("some-cluster").ToRancherName()
		Expect(name).To(Equal("some-cluster-capi"))
	})

	It("Should only add suffix once", func() {
		name := Name("some-cluster").ToRancherName()
		name = Name(name).ToRancherName()
		Expect(string(name)).To(Equal("some-cluster-capi"))
	})

	It("should remove suffix from rancher cluster", func() {
		name := Name("some-cluster").ToRancherName()
		name = Name(name).ToCapiName()
		Expect(string(name)).To(Equal("some-cluster"))
	})

	It("should remove suffix from rancher cluster only if it is present", func() {
		name := Name("some-cluster").ToCapiName()
		Expect(string(name)).To(Equal("some-cluster"))
	})
})

func TestNameConverter(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Test naming convention")
}
