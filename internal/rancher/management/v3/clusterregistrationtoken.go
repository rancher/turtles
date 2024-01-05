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

package v3

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterRegistrationToken is the struct representing a Rancher ClusterRegistrationToken.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type ClusterRegistrationToken struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterRegistrationTokenSpec `json:"spec"`

	Status ClusterRegistrationTokenStatus `json:"status,omitempty"`
}

// ClusterRegistrationTokenSpec is the struct representing the spec of a Rancher ClusterRegistrationToken.
type ClusterRegistrationTokenSpec struct {
	ClusterName string `json:"clusterName"`
}

// ClusterRegistrationTokenStatus is the struct representing the status of a Rancher ClusterRegistrationToken.
type ClusterRegistrationTokenStatus struct {
	ManifestURL string `json:"manifestUrl"`
}

// ClusterRegistrationTokenList contains a list of ClusterRegistrationTokens.
// +kubebuilder:object:root=true
type ClusterRegistrationTokenList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []ClusterRegistrationToken `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterRegistrationToken{}, &ClusterRegistrationTokenList{})
}
