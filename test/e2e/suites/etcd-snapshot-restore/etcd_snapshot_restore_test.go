//go:build e2e
// +build e2e

/*
Copyright © 2023 - 2024 SUSE LLC

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

package etcd_snapshot_restore

import (
	_ "embed"

	. "github.com/onsi/ginkgo/v2"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	"github.com/rancher/turtles/test/e2e"
)

var _ = Describe("[Docker] [RKE2] Perform an ETCD backup and restore of the cluster", Label(e2e.ShortTestLabel), func() {
	BeforeEach(func() {
		SetClient(setupClusterResult.BootstrapClusterProxy.GetClient())
		SetContext(ctx)
	})

	It("should perform an ETCD backup and restore of the cluster", func() {
	})
})
