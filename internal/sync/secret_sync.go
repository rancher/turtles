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

package sync

import (
	"cmp"
	"context"
	"maps"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
)

// SecretSync is a structure mirroring variable secret state of the CAPI Operator Provider object.
type SecretSync struct { //nolint: recvcheck
	*DefaultSynchronizer

	Secret *corev1.Secret
}

// NewSecretSync creates a new secret object sync.
func NewSecretSync(cl client.Client, capiProvider *turtlesv1.CAPIProvider) Sync {
	secret := SecretSync{}.GetSecret(capiProvider)

	return &SecretSync{
		DefaultSynchronizer: NewDefaultSynchronizer(cl, capiProvider, secret),
		Secret:              secret,
	}
}

// GetSecret returning the mirrored secret resource template.
func (SecretSync) GetSecret(capiProvider *turtlesv1.CAPIProvider) *corev1.Secret {
	meta := metav1.ObjectMeta{
		Name:      capiProvider.Name,
		Namespace: capiProvider.Namespace,
	}

	if capiProvider.Spec.ConfigSecret != nil {
		meta.Name = capiProvider.Spec.ConfigSecret.Name
	}

	return &corev1.Secret{ObjectMeta: meta}
}

// Template returning the mirrored secret resource template.
func (SecretSync) Template(capiProvider *turtlesv1.CAPIProvider) client.Object {
	return SecretSync{}.GetSecret(capiProvider)
}

// Sync updates the mirror object state from the upstream source object
// Direction of updates:
// Spec -> down
// up <- Status.
func (s *SecretSync) Sync(_ context.Context) error {
	s.SyncObjects()

	s.Source.Spec.ProviderSpec.ConfigSecret = cmp.Or(s.Source.Spec.ProviderSpec.ConfigSecret, &operatorv1.SecretReference{
		Name: s.Source.Name,
	})

	return nil
}

// SyncObjects updates the Source CAPIProvider object and the environment secret state.
// Direction of updates:
// Spec.Features + Spec.Variables -> Status.Variables -> Secret.
func (s *SecretSync) SyncObjects() {
	setVariables(s.DefaultSynchronizer.Source)
	setFeatures(s.DefaultSynchronizer.Source)

	s.Secret.StringData = s.DefaultSynchronizer.Source.Status.Variables
}

func setVariables(capiProvider *turtlesv1.CAPIProvider) {
	if capiProvider.Spec.Variables != nil {
		maps.Copy(capiProvider.Status.Variables, capiProvider.Spec.Variables)
	}
}

func setFeatures(capiProvider *turtlesv1.CAPIProvider) {
	features := capiProvider.Spec.Features
	variables := capiProvider.Status.Variables

	if features != nil {
		variables["EXP_CLUSTER_RESOURCE_SET"] = strconv.FormatBool(features.ClusterResourceSet)
		variables["CLUSTER_TOPOLOGY"] = strconv.FormatBool(features.ClusterTopology)
		variables["EXP_MACHINE_POOL"] = strconv.FormatBool(features.MachinePool)
	}
}
