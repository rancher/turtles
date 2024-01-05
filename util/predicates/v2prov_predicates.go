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

package predicates

import (
	"strings"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	provisioningv1 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/provisioning/v1"
)

// V2ProvClusterOwned returns a predicate that checks for a v2prov cluster owner reference.
func V2ProvClusterOwned(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return processIfV2ProvOwned(logger.WithValues("predicate", "V2ProvClusterOwned", "eventType", "update"), e.ObjectNew)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return processIfV2ProvOwned(logger.WithValues("predicate", "V2ProvClusterOwned", "eventType", "create"), e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

func processIfV2ProvOwned(logger logr.Logger, obj client.Object) bool {
	kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	log := logger.WithValues("namespace", obj.GetNamespace(), kind, obj.GetName())

	ownerRefs := obj.GetOwnerReferences()
	for _, ref := range ownerRefs {
		if ref.APIVersion == provisioningv1.GroupVersion.Identifier() {
			if ref.Kind == "Cluster" {
				log.V(4).Info("Object is owned by v2prov cluster, will attempt to map", "object", obj)
				return true
			}
		}
	}

	log.V(4).Info("No owner reference for v2prov cluster, will not map resource", "object", obj)

	return false
}
