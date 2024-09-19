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
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util/conditions"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/api"
	"github.com/rancher/turtles/internal/controllers/clusterctl"
)

// AppliedSpecHashAnnotation is a spec hash annotation set by CAPI Operator,
// to prevent infrastructure rollout without spec changes.
const AppliedSpecHashAnnotation = "operator.cluster.x-k8s.io/applied-spec-hash"

// ProviderSync is a structure mirroring state of the CAPI Operator Provider object.
type ProviderSync struct {
	*DefaultSynchronizer
	Destination api.Provider
}

// NewProviderSync creates a new mirror object.
func NewProviderSync(cl client.Client, capiProvider *turtlesv1.CAPIProvider) Sync {
	template := ProviderSync{}.Template(capiProvider)

	destination, ok := template.(api.Provider)
	if !ok || destination == nil {
		return nil
	}

	return &ProviderSync{
		DefaultSynchronizer: NewDefaultSynchronizer(cl, capiProvider, template),
		Destination:         destination,
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

	var template api.Provider

	for _, provider := range turtlesv1.Providers {
		if provider.GetType() == strings.ToLower(string(capiProvider.Spec.Type)) {
			// We always know the template type, so we can safely typecast.
			//nolint: forcetypeassert
			template = provider.DeepCopyObject().(api.Provider)

			template.SetName(meta.Name)
			template.SetNamespace(meta.Namespace)

			return template
		}
	}

	return template
}

// Sync updates the mirror object state from the upstream source object
// Direction of updates:
// Spec -> down
// up <- Status.
func (s *ProviderSync) Sync(ctx context.Context) error {
	if err := s.updateLatestVersion(ctx); err != nil {
		return err
	}

	s.SyncObjects()

	return nil
}

// SyncObjects updates the Source CAPIProvider object and the destination provider object states.
// Direction of updates:
// Spec -> <Common>Provider
// CAPIProvider <- Status.
func (s *ProviderSync) SyncObjects() {
	s.Destination.SetSpec(s.Source.GetSpec())

	oldConditions := s.Source.Status.Conditions.DeepCopy()
	newConditions := s.Destination.GetConditions().DeepCopy()
	s.Source.SetStatus(s.Destination.GetStatus())
	s.Source.Status.Conditions = oldConditions

	for _, condition := range newConditions {
		condition := condition
		conditions.Set(s.Source, &condition)
	}

	s.syncStatus()
}

func (s *ProviderSync) syncStatus() {
	switch {
	case conditions.IsTrue(s.Source, operatorv1.ProviderInstalledCondition):
		s.Source.SetPhase(turtlesv1.Ready)
	case conditions.IsFalse(s.Source, operatorv1.PreflightCheckCondition):
		s.Source.SetPhase(turtlesv1.Failed)
	default:
		s.Source.SetPhase(turtlesv1.Provisioning)
	}

	s.rolloutInfrastructure()
}

func (s *ProviderSync) rolloutInfrastructure() {
	now := time.Now().UTC()
	lastApplied := conditions.Get(s.Source, turtlesv1.LastAppliedConfigurationTime)

	if lastApplied != nil && lastApplied.LastTransitionTime.Add(time.Minute).After(now) {
		return
	}

	conditions.MarkUnknown(s.Source, turtlesv1.LastAppliedConfigurationTime, "Requesting infrastructure rollout", "")

	// Unsetting operator.cluster.x-k8s.io/applied-spec-hash to sync infrastructure if needed
	annotations := s.Destination.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}

	annotations[AppliedSpecHashAnnotation] = ""
	s.Destination.SetAnnotations(annotations)

	conditions.MarkTrue(s.Source, turtlesv1.LastAppliedConfigurationTime)
}

func (s *ProviderSync) updateLatestVersion(ctx context.Context) error {
	// Skip for user specified versions
	// TODO: We may potentially need to verify if version specified is built in the override,
	// and notify user with condition otherwise
	if s.Source.Spec.Version != "" {
		return nil
	}

	log := log.FromContext(ctx)

	config, err := clusterctl.ClusterConfig(ctx, s.client)
	if err != nil {
		return err
	}

	providerVersion := config.GetProviderVersion(ctx, cmp.Or(s.Source.Spec.Name, s.Source.Name), s.Source.Spec.Type.ToKind())
	expected := cmp.Or(s.Source.Spec.Version, "latest")

	if valid, err := config.VerifyProviderVersion(providerVersion, expected); err != nil {
		return err
	} else if !valid {
		lastCheck := conditions.Get(s.Source, turtlesv1.CheckLatestVersionTime)
		updatedMessage := fmt.Sprintf("Updated to latest %s version", providerVersion)

		if lastCheck == nil || lastCheck.Message != updatedMessage {
			log.Info(fmt.Sprintf("Version %s is beyound current latest, setting to %s", expected, providerVersion))

			lastCheck = conditions.TrueCondition(turtlesv1.CheckLatestVersionTime)
			lastCheck.Message = updatedMessage

			conditions.Set(s.Source, lastCheck)
		}

		s.Source.Spec.Version = providerVersion
	}

	return nil
}
