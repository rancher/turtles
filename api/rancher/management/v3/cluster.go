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

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterConditionAgentDeployed is the condition type for the agent deployed condition.
	ClusterConditionAgentDeployed clusterv1.ConditionType = "AgentDeployed"
	// ClusterConditionReady is the condition type for the ready condition.
	ClusterConditionReady clusterv1.ConditionType = "Ready"
	// CapiClusterFinalizer is the finalizer applied to capi clusters.
	CapiClusterFinalizer = "capicluster.turtles.cattle.io"
)

// Cluster is the struct representing a Rancher Cluster.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterSpec   `json:"spec"`
	Status ClusterStatus `json:"status,omitempty"`
}

// ClusterSpec is the struct representing the specification of a Rancher Cluster.
type ClusterSpec struct {
	DisplayName        string `json:"displayName,omitempty"`
	Description        string `json:"description,omitempty"`
	FleetWorkspaceName string `json:"fleetWorkspaceName,omitempty"`
}

// ClusterStatus is the struct representing the status of a Rancher Cluster.
type ClusterStatus struct {
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// ClusterList contains a list of ClusterList.
// +kubebuilder:object:root=true
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Cluster `json:"items"`
}

// GetConditions method to implement capi conditions getter interface.
func (c *Cluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions method to implement capi conditions setter interface.
func (c *Cluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

func init() {
	SchemeBuilder.Register(&Cluster{}, &ClusterList{})
}
