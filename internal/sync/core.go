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
type DefaultSynchronizer[T client.Object] struct {
	client      client.Client
	Source      *turtlesv1.CAPIProvider
	Destination T
}

// NewDefaultSynchronizer returns a new instance of DefaultSynchronizer.
func NewDefaultSynchronizer[T client.Object](cl client.Client, source *turtlesv1.CAPIProvider, destination T) *DefaultSynchronizer[T] {
	return &DefaultSynchronizer[T]{
		client:      cl,
		Source:      source,
		Destination: destination,
	}
}

// Get updates the destination object from the cluster.
func (s *DefaultSynchronizer[T]) Get(ctx context.Context) error {
	log := log.FromContext(ctx)

	objKey := client.ObjectKeyFromObject(s.Destination)
	if err := s.client.Get(ctx, objKey, s.Destination); client.IgnoreNotFound(err) != nil {
		log.Error(err, "Unable to get mirrored manifest: "+objKey.String())
	}

	return nil
}

// Apply applies the destination object to the cluster.
func (s *DefaultSynchronizer[T]) Apply(ctx context.Context, reterr *error, options ...client.PatchOption) {
	log := log.FromContext(ctx)
	uid := s.Destination.GetUID()

	setFinalizers(s.Destination)
	setOwnerReference(s.Source, s.Destination)

	if err := Patch(ctx, s.client, s.Destination, options...); err != nil {
		*reterr = kerrors.NewAggregate([]error{*reterr, err})
		log.Error(*reterr, fmt.Sprintf("Unable to patch object: %s", *reterr))
	}

	if s.Destination.GetUID() != uid {
		log.Info(fmt.Sprintf("Created %s: %s", s.Destination.GetObjectKind().GroupVersionKind().String(), client.ObjectKeyFromObject(s.Destination)))
	}
}

func setFinalizers(obj client.Object) {
	finalizers := obj.GetFinalizers()
	if finalizers == nil {
		finalizers = []string{}
	}

	// Only append the desired finalizer if it doesn't exist
	for _, finalizer := range finalizers {
		if finalizer == metav1.FinalizerDeleteDependents {
			return
		}
	}

	finalizers = append(finalizers, metav1.FinalizerDeleteDependents)
	obj.SetFinalizers(finalizers)
}

func setOwnerReference(owner, obj client.Object) {
	obj.SetOwnerReferences([]metav1.OwnerReference{{
		APIVersion:         turtlesv1.GroupVersion.String(),
		Kind:               turtlesv1.Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}})
}
