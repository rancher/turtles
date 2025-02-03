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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ETCDSnapshotPhase is a string representation of the phase of the etcd snapshot
type ETCDSnapshotRestorePhase string

const (
	// ETCDSnapshotRestorePhasePending is the phase when the snapshot was submitted but was not registered
	ETCDSnapshotRestorePhasePending ETCDSnapshotRestorePhase = "Pending"
	// ETCDSnapshotRestorePhaseStarted is the phase when the snapshot creation has started
	ETCDSnapshotRestorePhaseStarted ETCDSnapshotRestorePhase = "Started"
	// ETCDSnapshotRestorePhaseShutdown is the phase when the etcd cluster is being shutdown
	ETCDSnapshotRestorePhaseShutdown ETCDSnapshotRestorePhase = "Shutdown"
	// ETCDSnapshotRestorePhaseRunning is the phase when the snapshot is being restored
	ETCDSnapshotRestorePhaseRunning ETCDSnapshotRestorePhase = "Running"
	// ETCDSnapshotRestorePhaseAgentRestart is the phase when the cluster is being restarted
	ETCDSnapshotRestorePhaseAgentRestart ETCDSnapshotRestorePhase = "Restart"
	// ETCDSnapshotRestoreUnpauseCluster is the phase when the cluster can be unpaused
	ETCDSnapshotRestoreUnpauseCluster ETCDSnapshotRestorePhase = "Unpause"
	// ETCDSnapshotRestorePhaseJoinAgents is the phase when the snapshot creation has finished
	ETCDSnapshotRestorePhaseJoinAgents ETCDSnapshotRestorePhase = "Joining"
	// ETCDSnapshotRestorePhaseFailed is the phase when the snapshot creation has failed
	ETCDSnapshotRestorePhaseFailed ETCDSnapshotRestorePhase = "Failed"
	// ETCDSnapshotRestorePhaseFinished is the phase when the snapshot creation has finished
	ETCDSnapshotRestorePhaseFinished ETCDSnapshotRestorePhase = "Done"
)

// +kubebuilder:validation:XValidation:message="Cluster Name can't be empty.",rule="size(self.clusterName)>0"
// +kubebuilder:validation:XValidation:message="ETCD machine snapshot name can't be empty.",rule="size(self.etcdMachineSnapshotName)>0"
//
// ETCDSnapshotRestoreSpec defines the desired state of EtcdSnapshotRestore.
type ETCDSnapshotRestoreSpec struct {
	// +required
	ClusterName string `json:"clusterName"`

	// +required
	ETCDMachineSnapshotName string `json:"etcdMachineSnapshotName"`

	// TTLSecondsAfterFinished int `json:"ttlSecondsAfterFinished"`

	// // +required
	// ConfigRef corev1.LocalObjectReference `json:"configRef"`
}

// ETCDSnapshotRestoreStatus defines observed state of EtcdSnapshotRestore.
type ETCDSnapshotRestoreStatus struct {
	// +kubebuilder:default=Pending
	Phase      ETCDSnapshotRestorePhase `json:"phase,omitempty"`
	Conditions clusterv1.Conditions     `json:"conditions,omitempty"`
}

// ETCDSnapshotRestore is the schema for the ETCDSnapshotRestore API.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type ETCDSnapshotRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ETCDSnapshotRestoreSpec `json:"spec,omitempty"`

	// +kubebuilder:default={}
	Status ETCDSnapshotRestoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ETCDSnapshotRestoreList contains a list of EtcdSnapshotRestores.
type ETCDSnapshotRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ETCDSnapshotRestore `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &ETCDSnapshotRestore{}, &ETCDSnapshotRestoreList{})
}
