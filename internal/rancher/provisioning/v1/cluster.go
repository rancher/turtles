package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Cluster is the struct representing a Rancher Cluster.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status ClusterStatus `json:"status,omitempty"`
}

// ClusterStatus is the struct representing the status of a Rancher Cluster.
type ClusterStatus struct {
	ClusterName   string `json:"clusterName,omitempty"`
	AgentDeployed bool   `json:"agentDeployed,omitempty"`
	Ready         bool   `json:"ready,omitempty"`
}

// ClusterList contains a list of ClusterList.
// +kubebuilder:object:root=true
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
