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

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	turtlesv1 "github.com/rancher-sandbox/rancher-turtles/api/v1alpha1"
)

// Sync is an inteface for mirroring state of the CAPI Operator Provider object on child objects.
type Sync interface {
	Template(*turtlesv1.CAPIProvider) client.Object
	Get(context.Context) error
	Sync(context.Context) error
	Apply(context.Context, *error)
}

// List contains a list of syncers to apply the syncing logic.
type List []Sync

// Sync applies synchronization logic on all syncers in the list.
func (s List) Sync(ctx context.Context) error {
	errors := []error{}

	for _, syncer := range s {
		if syncer == nil {
			continue
		}

		errors = append(errors, syncer.Get(ctx), syncer.Sync(ctx))
	}

	return kerrors.NewAggregate(errors)
}

// Apply updates all syncer objects in the cluster.
func (s List) Apply(ctx context.Context, reterr *error) {
	errors := []error{*reterr}

	for _, syncer := range s {
		if syncer == nil {
			continue
		}

		var err error

		syncer.Apply(ctx, &err)
		errors = append(errors, err)
	}

	*reterr = kerrors.NewAggregate(errors)
}
