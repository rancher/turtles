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

package annotations

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ClusterImportedAnnotation represents cluster imported annotation.
	ClusterImportedAnnotation = "imported"
	// NoCreatorRBACAnnotation represents no creator rbac annotation.
	// When it is set the webhook does not set the `field.cattle.io/creatorId` annotation
	// to clusters because the controllers will no longer be creating RBAC for the creator of the cluster.
	// This is required so that logs don't get flooded with errors when project handler and cluster handler
	// attempt to create `ClusterOwner` and `ProjectOwner` roles.
	NoCreatorRBACAnnotation = "field.cattle.io/no-creator-rbac"
	// EtcdAutomaticSnapshot represents automatically generated etcd snapshot.
	EtcdAutomaticSnapshot = "etcd.turtles.cattle.io/automatic-snapshot"
)

// HasClusterImportAnnotation returns true if the object has the `imported` annotation.
func HasClusterImportAnnotation(o metav1.Object) bool {
	return HasAnnotation(o, ClusterImportedAnnotation)
}

// HasAnnotation returns true if the object has the specified annotation.
func HasAnnotation(o metav1.Object, annotation string) bool {
	annotations := o.GetAnnotations()
	if annotations == nil {
		return false
	}

	_, ok := annotations[annotation]

	return ok
}
