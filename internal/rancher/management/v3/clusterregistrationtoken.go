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

	Status ClusterRegistrationTokenStatus `json:"status,omitempty"`
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
