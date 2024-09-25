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

package framework

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	capiframework "sigs.k8s.io/cluster-api/test/framework"

	. "github.com/onsi/gomega"
)

type WaitForCAPIProviderRolloutInput struct {
	capiframework.Getter
	Name, Namespace, Version string
}

func WaitForCAPIProviderRollout(ctx context.Context, input WaitForCAPIProviderRolloutInput, intervals ...interface{}) {
	capiProvider := &turtlesv1.CAPIProvider{}
	key := types.NamespacedName{
		Name:      input.Name,
		Namespace: input.Namespace,
	}

	Byf("Waiting for CAPIProvider %s to be at version %s", key.String(), input.Version)

	Eventually(func(g Gomega) {
		g.Expect(input.Getter.Get(ctx, key, capiProvider)).To(Succeed())
		g.Expect(capiProvider.Status.InstalledVersion).ToNot(BeNil())
		g.Expect(*capiProvider.Status.InstalledVersion).To(Equal(input.Version))
	}, intervals...).Should(Succeed(),
		"Failed to get CAPIProvider %s with version %s. Last observed: %s",
		key.String(), input.Version, klog.KObj(capiProvider))
}
