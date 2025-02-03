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
)

// ETCDSnapshotPhase is a string representation of the phase of the etcd snapshot
type ETCDSnapshotPhase string

const (
	// ETCDSnapshotPhasePending is the phase when the snapshot was submitted but was not registered
	ETCDSnapshotPhasePending ETCDSnapshotPhase = "Pending"
	// ETCDSnapshotPhasePlanning is the phase when the snapshot was submitted to the planner, but was not registered
	ETCDSnapshotPhasePlanning ETCDSnapshotPhase = "Planning"
	// ETCDSnapshotPhaseRunning is the phase when the snapshot creation has started
	ETCDSnapshotPhaseRunning ETCDSnapshotPhase = "Running"
	// ETCDSnapshotPhaseFailed is the phase when the snapshot creation has failed
	ETCDSnapshotPhaseFailed ETCDSnapshotPhase = "Failed"
	// ETCDSnapshotPhaseDone is the phase when the snapshot creation has finished
	ETCDSnapshotPhaseDone ETCDSnapshotPhase = "Done"

	// ETCDMachineSnapshotFinalizer allows the controller to clean up resources associated with EtcdMachineSnapshot
	ETCDMachineSnapshotFinalizer = "etcdmachinesnapshot.turtles.cattle.io"
)

// +kubebuilder:validation:XValidation:message="ETCD snapshot location can't be empty.",rule="size(self.clusterName)>0"
//
// ETCDMachineSnapshotSpec defines the desired state of EtcdMachineSnapshot
type ETCDMachineSnapshotSpec struct {
	ClusterName string `json:"clusterName"`
	MachineName string `json:"machineName,omitempty"`
	Location    string `json:"location,omitempty"`
}

// EtcdSnapshotRestoreStatus defines observed state of EtcdSnapshotRestore
type ETCDMachineSnapshotStatus struct {
	Phase ETCDSnapshotPhase `json:"phase,omitempty"`

	// +optional
	Error *string `json:"error,omitempty"`

	// +optional
	SnapshotFileName *string `json:"snapshotFileName,omitempty"`

	// +optional
	Snapshots []ETCDMachineSnapshotFile `json:"snapshots,omitempty"`

	// +optional
	S3Snapshots []S3SnapshotFile `json:"s3Snapshots,omitempty"`
}

type ETCDMachineSnapshotFile struct {
	Name        string `json:"name"`
	MachineName string `json:"machineName"`
	Location    string `json:"location"`
	// CreationTime is the timestamp when the snapshot was taken by etcd.
	CreationTime *metav1.Time `json:"creationTime,omitempty"`
}

type S3SnapshotFile struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	// CreationTime is the timestamp when the snapshot was taken by etcd.
	CreationTime *metav1.Time `json:"creationTime,omitempty"`
}

// ETCDMachineSnapshot is the Schema for the ETCDMachineSnapshot API.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type ETCDMachineSnapshot struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ETCDMachineSnapshotSpec   `json:"spec,omitempty"`
	Status ETCDMachineSnapshotStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ETCDMachineSnapshotList contains a list of EtcdMachineSnapshots.
type ETCDMachineSnapshotList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ETCDMachineSnapshot `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &ETCDMachineSnapshot{}, &ETCDMachineSnapshotList{})
}
