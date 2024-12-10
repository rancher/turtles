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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const fieldOwner = "capi-provider-operator"

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
func Patch(ctx context.Context, cl client.Client, obj client.Object, options ...client.PatchOption) error {
	log := log.FromContext(ctx)

	obj.SetManagedFields(nil)

	if err := setKind(cl, obj); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Updating %s: %s", obj.GetObjectKind().GroupVersionKind().Kind, client.ObjectKeyFromObject(obj)))

	patchOptions := []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(fieldOwner),
	}
	patchOptions = append(patchOptions, options...)

	return cl.Patch(ctx, obj, client.Apply, patchOptions...)
}

// PatchStatus will only patch the status subresource of the provided object.
func PatchStatus(ctx context.Context, cl client.Client, obj client.Object) error {
	log := log.FromContext(ctx)

	obj.SetManagedFields(nil)
	obj.SetFinalizers([]string{metav1.FinalizerDeleteDependents})

	if err := setKind(cl, obj); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Patching status %s: %s", obj.GetObjectKind().GroupVersionKind().Kind, client.ObjectKeyFromObject(obj)))

	return cl.Status().Patch(ctx, obj, client.Apply, []client.SubResourcePatchOption{
		client.ForceOwnership,
		client.FieldOwner(fieldOwner),
	}...)
}
