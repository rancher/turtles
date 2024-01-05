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
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util/conditions"

	turtlesv1 "github.com/rancher-sandbox/rancher-turtles/api/v1alpha1"
)

// ProviderSync is a structure mirroring state of the CAPI Operator Provider object.
type ProviderSync struct {
	*DefaultSynchronizer
}

// NewProviderSync creates a new mirror object.
func NewProviderSync(cl client.Client, capiProvider *turtlesv1.CAPIProvider) Sync {
	return &ProviderSync{
		DefaultSynchronizer: NewDefaultSynchronizer(cl, capiProvider, ProviderSync{}.Template(capiProvider)),
	}
}

// Template returning the mirrored CAPI Operator manifest template.
func (ProviderSync) Template(capiProvider *turtlesv1.CAPIProvider) client.Object {
	meta := metav1.ObjectMeta{
		Name:      capiProvider.Spec.Name,
		Namespace: capiProvider.GetNamespace(),
	}

	if meta.Name == "" {
		meta.Name = capiProvider.Name
	}

	switch capiProvider.Spec.Type {
	case turtlesv1.Infrastructure:
		return &operatorv1.InfrastructureProvider{ObjectMeta: meta}
	case turtlesv1.Core:
		return &operatorv1.CoreProvider{ObjectMeta: meta}
	case turtlesv1.ControlPlane:
		return &operatorv1.ControlPlaneProvider{ObjectMeta: meta}
	case turtlesv1.Bootstrap:
		return &operatorv1.BootstrapProvider{ObjectMeta: meta}
	case turtlesv1.Addon:
		return &operatorv1.AddonProvider{ObjectMeta: meta}
	default:
	}

	return nil
}

// Sync updates the mirror object state from the upstream source object
// Direction of updates:
// Spec -> down
// up <- Status.
func (s *ProviderSync) Sync(_ context.Context) error {
	s.SyncObjects()

	return nil
}

// SyncObjects updates the Source CAPIProvider object and the destination provider object states.
// Direction of updates:
// Spec -> <Common>Provider
// CAPIProvider <- Status.
func (s *ProviderSync) SyncObjects() {
	switch mirror := s.Destination.(type) {
	case *operatorv1.InfrastructureProvider:
		s.Source.Spec.ProviderSpec.DeepCopyInto(&mirror.Spec.ProviderSpec)
		mirror.Status.ProviderStatus.DeepCopyInto(&s.Source.Status.ProviderStatus)
	case *operatorv1.CoreProvider:
		s.Source.Spec.ProviderSpec.DeepCopyInto(&mirror.Spec.ProviderSpec)
		mirror.Status.ProviderStatus.DeepCopyInto(&s.Source.Status.ProviderStatus)
	case *operatorv1.ControlPlaneProvider:
		s.Source.Spec.ProviderSpec.DeepCopyInto(&mirror.Spec.ProviderSpec)
		mirror.Status.ProviderStatus.DeepCopyInto(&s.Source.Status.ProviderStatus)
	case *operatorv1.BootstrapProvider:
		s.Source.Spec.ProviderSpec.DeepCopyInto(&mirror.Spec.ProviderSpec)
		mirror.Status.ProviderStatus.DeepCopyInto(&s.Source.Status.ProviderStatus)
	case *operatorv1.AddonProvider:
		s.Source.Spec.ProviderSpec.DeepCopyInto(&mirror.Spec.ProviderSpec)
		mirror.Status.ProviderStatus.DeepCopyInto(&s.Source.Status.ProviderStatus)
	default:
	}

	s.syncStatus()
}

func (s *ProviderSync) syncStatus() {
	switch {
	case conditions.IsTrue(s.Source, operatorv1.ProviderInstalledCondition):
		s.Source.Status.Phase = turtlesv1.Ready
	case conditions.IsFalse(s.Source, operatorv1.PreflightCheckCondition):
		s.Source.Status.Phase = turtlesv1.Failed
	default:
		s.Source.Status.Phase = turtlesv1.Provisioning
	}
}
