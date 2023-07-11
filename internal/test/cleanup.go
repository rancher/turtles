package test

import (
	"context"
	"time"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	cacheSyncBackoff = wait.Backoff{
		Duration: 100 * time.Millisecond,
		Factor:   1.5,
		Steps:    8,
		Jitter:   0.4,
	}
)

// CleanupAndWait deletes all the given objects and waits for the cache to be updated accordingly.
func CleanupAndWait(ctx context.Context, cl client.Client, objs ...client.Object) error {
	if err := cleanup(ctx, cl, objs...); err != nil {
		return err
	}

	// Makes sure the cache is updated with the deleted object
	errs := []error{}
	for _, o := range objs {
		// Ignoring namespaces because in testenv the namespace cleaner is not running.
		if o.GetObjectKind().GroupVersionKind().GroupKind() == corev1.SchemeGroupVersion.WithKind("Namespace").GroupKind() {
			continue
		}

		oCopy := o.DeepCopyObject().(client.Object)
		key := client.ObjectKeyFromObject(o)
		err := wait.ExponentialBackoff(
			cacheSyncBackoff,
			func() (done bool, err error) {
				if err := cl.Get(ctx, key, oCopy); err != nil {
					if apierrors.IsNotFound(err) {
						return true, nil
					}
					if o.GetName() == "" { // resource is being deleted
						return true, nil
					}
					return false, err
				}
				return false, nil
			})
		errs = append(errs, errors.Wrapf(err, "key %s, %s is not being deleted from the testenv client cache", o.GetObjectKind().GroupVersionKind().String(), key))
	}
	return kerrors.NewAggregate(errs)
}

// cleanup deletes all the given objects.
func cleanup(ctx context.Context, cl client.Client, objs ...client.Object) error {
	errs := []error{}
	for _, o := range objs {
		copyObj := o.DeepCopyObject().(client.Object)

		if err := cl.Get(ctx, client.ObjectKeyFromObject(o), copyObj); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			if o.GetName() == "" { // resource is being deleted
				continue
			}
			errs = append(errs, err)
			continue
		}

		// Remove finalizers from the object
		if copyObj.GetFinalizers() != nil {
			copyObj.SetFinalizers(nil)
		}

		err := cl.Update(ctx, copyObj)
		if apierrors.IsNotFound(err) {
			continue
		}
		errs = append(errs, err)

		err = cl.Delete(ctx, copyObj)
		if apierrors.IsNotFound(err) {
			continue
		}
		errs = append(errs, err)
	}
	return kerrors.NewAggregate(errs)
}
