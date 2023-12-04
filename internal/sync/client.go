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
func Patch(ctx context.Context, cl client.Client, obj client.Object) error {
	log := log.FromContext(ctx)

	obj.SetManagedFields(nil)

	if err := setKind(cl, obj); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Updating %s: %s", obj.GetObjectKind().GroupVersionKind().Kind, client.ObjectKeyFromObject(obj)))

	return cl.Patch(ctx, obj, client.Apply, []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner(fieldOwner),
	}...)
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
