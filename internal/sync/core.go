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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
)

// DefaultSynchronizer is a structure mirroring state of the CAPI Operator Provider object.
type DefaultSynchronizer struct {
	client      client.Client
	Source      *turtlesv1.CAPIProvider
	Destination client.Object
}

// NewDefaultSynchronizer returns a new instance of DefaultSynchronizer.
func NewDefaultSynchronizer(cl client.Client, capiProvider *turtlesv1.CAPIProvider, destination client.Object) *DefaultSynchronizer {
	return &DefaultSynchronizer{
		client:      cl,
		Source:      capiProvider,
		Destination: destination,
	}
}

// Get updates the destination object from the cluster.
func (s *DefaultSynchronizer) Get(ctx context.Context) error {
	log := log.FromContext(ctx)

	if err := s.client.Get(ctx, client.ObjectKeyFromObject(s.Destination), s.Destination); client.IgnoreNotFound(err) != nil {
		log.Error(err, "Unable to get mirrored manifest: "+client.ObjectKeyFromObject(s.Destination).String())

		return err
	}

	return nil
}

// Apply applies the destination object to the cluster.
func (s *DefaultSynchronizer) Apply(ctx context.Context, reterr *error) {
	log := log.FromContext(ctx)
	uid := s.Destination.GetUID()

	setOwnerReference(s.Source, s.Destination)

	if err := Patch(ctx, s.client, s.Destination); err != nil {
		*reterr = kerrors.NewAggregate([]error{*reterr, err})
		log.Error(*reterr, fmt.Sprintf("Unable to patch object: %s", *reterr))
	}

	if s.Destination.GetUID() != uid {
		log.Info(fmt.Sprintf("Created %s: %s", s.Destination.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(s.Destination)))
	}
}

func setOwnerReference(owner, obj client.Object) {
	obj.SetFinalizers([]string{metav1.FinalizerDeleteDependents})
	obj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion:         turtlesv1.GroupVersion.String(),
		Kind:               turtlesv1.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}})
}
