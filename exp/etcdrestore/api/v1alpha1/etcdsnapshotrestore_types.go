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
	corev1 "k8s.io/api/core/v1"
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
	// ETCDSnapshotRestorePhaseJoinAgents is the phase when the snapshot creation has finished
	ETCDSnapshotRestorePhaseJoinAgents ETCDSnapshotRestorePhase = "Joining"
	// ETCDSnapshotRestorePhaseFailed is the phase when the snapshot creation has failed
	ETCDSnapshotRestorePhaseFailed ETCDSnapshotRestorePhase = "Failed"
	// ETCDSnapshotRestorePhaseFinished is the phase when the snapshot creation has finished
	ETCDSnapshotRestorePhaseFinished ETCDSnapshotRestorePhase = "Done"
)

// EtcdSnapshotRestoreSpec defines the desired state of EtcdSnapshotRestore.
type EtcdSnapshotRestoreSpec struct {
	ClusterName             string                 `json:"clusterName"`
	EtcdMachineSnapshotName string                 `json:"etcdMachineSnapshotName"`
	TTLSecondsAfterFinished int                    `json:"ttlSecondsAfterFinished"`
	ConfigRef               corev1.ObjectReference `json:"configRef"`
}

// EtcdSnapshotRestoreStatus defines observed state of EtcdSnapshotRestore.
type EtcdSnapshotRestoreStatus struct {
	Phase      ETCDSnapshotPhase    `json:"phase,omitempty"`
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// EtcdSnapshotRestore is the schema for the EtcdSnapshotRestore API.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type EtcdSnapshotRestore struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdSnapshotRestoreSpec   `json:"spec,omitempty"`
	Status EtcdSnapshotRestoreStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EtcdSnapshotRestoreList contains a list of EtcdSnapshotRestores.
type EtcdSnapshotRestoreList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdSnapshotRestore `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &EtcdSnapshotRestore{}, &EtcdSnapshotRestoreList{})
}
