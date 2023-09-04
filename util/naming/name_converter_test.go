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
