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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestNameHasSuffixPredicate(t *testing.T) {
	g := NewWithT(t)

	secretWithSuffix := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1-kubeconfig",
			Namespace: "default",
		},
	}

	secretWithoutSuffix := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "default",
		},
	}

	testCases := []struct {
		name     string
		object   client.Object
		suffix   string
		expected bool
	}{
		{
			name:     "with suffix and matching name, should be true",
			object:   secretWithSuffix,
			suffix:   "-kubeconfig",
			expected: true,
		},
		{
			name:     "without suffix, should be false",
			object:   secretWithoutSuffix,
			suffix:   "-kubeconfig",
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			predicate := NameHasSuffix(logr.New(log.NullLogSink{}), tc.suffix)

			updateEv := event.UpdateEvent{
				ObjectOld: tc.object,
				ObjectNew: tc.object,
			}
			g.Expect(predicate.Update(updateEv)).To(Equal(tc.expected), "NameHasSuffix update predicate did have expected result")

			createEv := event.CreateEvent{
				Object: tc.object,
			}
			g.Expect(predicate.Create(createEv)).To(Equal(tc.expected), "NameHasSuffix create predicate did have expected result")

			deleteEv := event.DeleteEvent{
				Object: tc.object,
			}
			g.Expect(predicate.Delete(deleteEv)).To(Equal(tc.expected), "NameHasSuffix delete predicate did have expected result")

			genericEv := event.GenericEvent{
				Object: tc.object,
			}
			g.Expect(predicate.Generic(genericEv)).To(Equal(tc.expected), "NameHasSuffix generic predicate did have expected result")
		})
	}
}
