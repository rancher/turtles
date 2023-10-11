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

package predicates

import (
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestV2ProvClusteredOwnedPredicate(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name      string
		ownerRefs []metav1.OwnerReference
		expected  bool
	}{
		{
			name:      "no owner refs, should be false",
			ownerRefs: nil,
			expected:  false,
		},
		{
			name: "non-v2prov owner refer, should be false",
			ownerRefs: []metav1.OwnerReference{
				{
					APIVersion: "someapi/v1",
					Kind:       "Queue",
				},
			},
			expected: false,
		},
		{
			name: "with v2prov cluster owner, should be true",
			ownerRefs: []metav1.OwnerReference{
				{
					APIVersion: "provisioning.cattle.io/v1",
					Kind:       "Cluster",
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			predicate := V2ProvClusterOwned(logr.New(log.NullLogSink{}))

			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "cluster-kubeconfig",
					Namespace:       "default",
					OwnerReferences: tc.ownerRefs,
				},
			}

			updateEv := event.UpdateEvent{
				ObjectOld: secret,
				ObjectNew: secret,
			}
			g.Expect(predicate.Update(updateEv)).To(Equal(tc.expected), "V2ProvClusterOwned update predicate did have expected result")

			createEv := event.CreateEvent{
				Object: secret,
			}
			g.Expect(predicate.Create(createEv)).To(Equal(tc.expected), "V2ProvClusterOwned create predicate did have expected result")

		})
	}
}
