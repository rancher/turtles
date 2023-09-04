/*
Copyright 2023 SUSE.

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

var cacheSyncBackoff = wait.Backoff{
	Duration: 100 * time.Millisecond,
	Factor:   1.5,
	Steps:    8,
	Jitter:   0.4,
}

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

		oCopy, ok := o.DeepCopyObject().(client.Object)
		if !ok {
			return errors.Errorf("unable to convert object %s to client.Object", o.GetObjectKind().GroupVersionKind().String())
		}

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
		errs = append(errs,
			errors.Wrapf(err, "key %s, %s is not being deleted from the testenv client cache", o.GetObjectKind().GroupVersionKind().String(), key))
	}

	return kerrors.NewAggregate(errs)
}

// cleanup deletes all the given objects.
func cleanup(ctx context.Context, cl client.Client, objs ...client.Object) error {
	errs := []error{}

	for _, o := range objs {
		copyObj, ok := o.DeepCopyObject().(client.Object)
		if !ok {
			return errors.Errorf("unable to convert object %s to client.Object", o.GetObjectKind().GroupVersionKind().String())
		}

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
