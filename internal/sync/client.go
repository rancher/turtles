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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func setKind(cl client.Client, obj client.Object) error {
	kinds, _, err := cl.Scheme().ObjectKinds(obj)
	if err != nil {
		return err
	} else if len(kinds) > 0 {
		obj.GetObjectKind().SetGroupVersionKind(kinds[0])
	}

	return nil
}

// Patch will only patch mirror object in the cluster.
func Patch(ctx context.Context, cl client.Client, obj client.Object) error {
	log := log.FromContext(ctx)

	obj.SetManagedFields(nil)

	if err := setKind(cl, obj); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Updating %s: %s", obj.GetObjectKind().GroupVersionKind().Kind, client.ObjectKeyFromObject(obj)))

	// Avoiding client.Apply (SSA) Patch due to resourceVersion always being bumped on empty patches.
	// See: https://github.com/kubernetes/kubernetes/issues/131175
	//
	// Also avoiding client.Merge in order to correctly propagate keys that have been removed.
	err := cl.Update(ctx, obj)
	if apierrors.IsNotFound(err) {
		return cl.Create(ctx, obj)
	}

	return err
}
