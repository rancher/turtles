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

	"github.com/rancher-sandbox/rancher-turtles/util/annotations"
)

// ClusterWithoutImportedAnnotation returns a predicate that returns true only if the provided resource does not contain
// "clusterImportedAnnotation" annotation. When annotation is present on the resource, controller will skip reconciliation.
func ClusterWithoutImportedAnnotation(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return processIfClusterNotImported(logger.WithValues("predicate", "ClusterWithoutImportedAnnotation", "eventType", "update"), e.ObjectNew)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return processIfClusterNotImported(logger.WithValues("predicate", "ClusterWithoutImportedAnnotation", "eventType", "create"), e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return processIfClusterNotImported(logger.WithValues("predicate", "ClusterWithoutImportedAnnotation", "eventType", "delete"), e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return processIfClusterNotImported(logger.WithValues("predicate", "ClusterWithoutImportedAnnotation", "eventType", "generic"), e.Object)
		},
	}
}

func processIfClusterNotImported(logger logr.Logger, obj client.Object) bool {
	kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	log := logger.WithValues("namespace", obj.GetNamespace(), kind, obj.GetName())

	if annotations.HasAnnotation(obj, annotations.ClusterImportedAnnotation) {
		log.V(4).Info("Cluster has an import annotation, will not attempt to map resource")
		return false
	}

	log.V(6).Info("Cluster does not have an import annotation, will attempt to map resource")

	return true
}
