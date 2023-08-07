package annotations

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ClusterImportedAnnotation represents cluster imported annotation.
	ClusterImportedAnnotation = "imported"
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
