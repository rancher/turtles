/*
Copyright 2024.

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
	fleetv1 "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ClusterUpgradeGroupSpec defines the desired state of ClusterUpgradeGroup
type ClusterUpgradeGroupSpec struct {
	// +required
	ClassName string `json:"className"`

	// RolloutStrategy controls the rollout of bundles, by defining
	// partitions, canaries and percentages for cluster availability.
	// +optional
	RolloutStrategy *fleetv1.RolloutStrategy `json:"rolloutStrategy,omitempty"`

	// Targets refer to the clusters that should be upgraded.
	Targets []ClusterTargets `json:"targets,omitempty"`
}

type ClusterTargets struct {
	// Name of target. This value is largely for display and logging. If
	// not specified a default name of the format "target000" will be used
	Name string `json:"name,omitempty"`
	// ClusterName to match a specific cluster by name that will be
	// selected
	// +nullable
	ClusterName string `json:"clusterName,omitempty"`
	// ClusterSelector is a selector to match clusters. The structure is
	// the standard metav1.LabelSelector format. If clusterGroupSelector or
	// clusterGroup is specified, clusterSelector will be used only to
	// further refine the selection after clusterGroupSelector and
	// clusterGroup is evaluated.
	// +nullable
	ClusterSelector *metav1.LabelSelector `json:"clusterSelector,omitempty"`
	// ClusterGroup to match a specific cluster group by name.
	// +nullable
	ClusterGroup string `json:"clusterGroup,omitempty"`
	// ClusterGroupSelector is a selector to match cluster groups.
	// +nullable
	ClusterGroupSelector *metav1.LabelSelector `json:"clusterGroupSelector,omitempty"`
	// DoNotDeploy if set to true, will not deploy to this target.
	DoNotDeploy bool `json:"doNotDeploy,omitempty"`
}

// ClusterUpgradeGroupStatus defines the observed state of ClusterUpgradeGroup
type ClusterUpgradeGroupStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// ClusterUpgradeGroup is the Schema for the clusterupgrades API
type ClusterUpgradeGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterUpgradeGroupSpec   `json:"spec,omitempty"`
	Status ClusterUpgradeGroupStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterUpgradeGroupList contains a list of ClusterUpgradeGroup
type ClusterUpgradeGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterUpgradeGroup `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterUpgradeGroup{}, &ClusterUpgradeGroupList{})
}
