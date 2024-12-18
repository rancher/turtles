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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ClusterUpgradeGroupSpec defines the desired state of ClusterUpgradeGroup
type ClusterUpgradeGroupSpec struct {
	// +required
	ClassName string `json:"className"`

	// RolloutStrategy controls the rollout of bundles, by defining
	// partitions, canaries and percentages for cluster availability.
	// +optional
	RolloutStrategy *RolloutStrategy `json:"rolloutStrategy,omitempty"`

	// Targets refer to the clusters that should be upgraded.
	Targets []ClusterTargets `json:"targets,omitempty"`
}

type RolloutStrategyType string

const (
	// RollingUpdateStrategyType updates clusters by using rolling update
	RollingUpdateStrategyType RolloutStrategyType = "RollingUpdate"
)

// RolloutStrategy describes how to replace existing machines
// with new ones.
type RolloutStrategy struct {
	// Type of rollout.
	// Default is RollingUpdate.
	// +optional
	Type RolloutStrategyType `json:"type,omitempty"`

	// Rolling update config params. Present only if
	// RolloutStrategyType = RollingUpdate.
	// +optional
	RollingUpdate *RollingUpdate `json:"rollingUpdate,omitempty"`
}

// RollingUpdate is used to control the desired behavior of rolling update.
type RollingUpdate struct {
	// The maximum number of clusters that can be in update state (non-active) during a
	// rolling update.
	// +optional
	MaxRollouts *intstr.IntOrString `json:"maxRollouts,omitempty"`
	// The delay between subsequent cluster rollouts.
	// +optional
	RolloutDelay *intstr.IntOrString `json:"rolloutDelay,omitempty"`
	// The maximum number of failed attempts before skipping the update for a given
	// cluster.
	// +optional
	MaxFailures *intstr.IntOrString `json:"maxFailures,omitempty"`
	// NOTE: in future iterations we add a `FailureAction` field here to control the behavior
	// The action to perform when a cluster rollout is considered a failure.
	// Defaults is Skip.
	// +optional
	// FailureAction FailureActionType `json:"failureAction,omitempty"`
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
