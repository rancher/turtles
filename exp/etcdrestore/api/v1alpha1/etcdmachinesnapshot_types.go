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

	corev1 "k8s.io/api/core/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ETCDSnapshotPhase is a string representation of the phase of the etcd snapshot
type ETCDSnapshotPhase string

const (
	// ETCDSnapshotPhasePending is the phase when the snapshot was submitted but was not registered
	ETCDSnapshotPhasePending ETCDSnapshotPhase = "Pending"
	// ETCDSnapshotPhaseRunning is the phase when the snapshot creation has started
	ETCDSnapshotPhaseRunning ETCDSnapshotPhase = "Running"
	// ETCDSnapshotPhaseFailed is the phase when the snapshot creation has failed
	ETCDSnapshotPhaseFailed ETCDSnapshotPhase = "Failed"
	// ETCDSnapshotPhaseDone is the phase when the snapshot creation has finished
	ETCDSnapshotPhaseDone ETCDSnapshotPhase = "Done"

	// ETCDMachineSnapshotFinalizer allows the controller to clean up resources associated with EtcdMachineSnapshot
	ETCDMachineSnapshotFinalizer = "etcdmachinesnapshot.turtles.cattle.io"
)

// EtcdMachineSnapshotSpec defines the desired state of EtcdMachineSnapshot
type EtcdMachineSnapshotSpec struct {
	ClusterName string                 `json:"clusterName"`
	MachineName string                 `json:"machineName"`
	ConfigRef   corev1.ObjectReference `json:"configRef"`
}

// EtcdSnapshotRestoreStatus defines observed state of EtcdSnapshotRestore
type EtcdMachineSnapshotStatus struct {
	Phase      ETCDSnapshotPhase    `json:"phase,omitempty"`
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// EtcdMachineSnapshot is the Schema for the EtcdMachineSnapshot API.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type EtcdMachineSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdMachineSnapshotSpec   `json:"spec,omitempty"`
	Status EtcdMachineSnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// EtcdMachineSnapshotList contains a list of EtcdMachineSnapshots.
type EtcdMachineSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdMachineSnapshot `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &EtcdMachineSnapshot{}, &EtcdMachineSnapshotList{})
}
