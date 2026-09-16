/*
Copyright © 2023 - 2026 SUSE LLC

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

package v1alpha1

import (
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Cluster is the struct representing a Fleet Cluster.
// It is a partial representation, only containing the fields Turtles interacts with.
// +kubebuilder:object:root=true
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterSpec `json:"spec"`
}

// ClusterSpec is the struct representing the specification of a Fleet Cluster.
type ClusterSpec struct {
	// TemplateValues is a generic map of keys to values that are arbitrary JSON objects.
	// Note that this matches the source type in CAPI but not the destination type in Fleet
	// which is a custom `GenericMap` implementation.
	TemplateValues map[string]apiextensionsv1.JSON `json:"templateValues,omitempty"`
}

// ClusterList contains a list of Cluster.
// +kubebuilder:object:root=true
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Cluster `json:"items"`
}
