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

package predicates

import (
	"strings"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// NameHasSuffix returns a predicate that checks the name of the object has a specific suffix.
func NameHasSuffix(logger logr.Logger, suffix string) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return processIfNameHasSuffix(logger.WithValues("predicate", "NameHasSuffix", "eventType", "update"), e.ObjectNew, suffix)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return processIfNameHasSuffix(logger.WithValues("predicate", "NameHasSuffix", "eventType", "create"), e.Object, suffix)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return processIfNameHasSuffix(logger.WithValues("predicate", "NameHasSuffix", "eventType", "delete"), e.Object, suffix)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return processIfNameHasSuffix(logger.WithValues("predicate", "NameHasSuffix", "eventType", "generic"), e.Object, suffix)
		},
	}
}

func processIfNameHasSuffix(logger logr.Logger, obj client.Object, suffix string) bool {
	kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	log := logger.WithValues("namespace", obj.GetNamespace(), kind, obj.GetName())

	if strings.HasSuffix(obj.GetName(), suffix) {
		log.V(4).Info("Object name has suffix, will attempt to map", "object", obj)

		return true
	}

	log.V(4).Info("Object name doesn't have suffix, will not map resource", "object", obj)

	return false
}
