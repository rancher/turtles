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

// EtcdMachineSnapshotSpec defines the desired state of EtcdMachineSnapshot.
type EtcdMachineSnapshotSpec struct {
	Foo string `json:"foo,omitempty""`
}

// EtcdMachineSnapshotStatus defines observed state of EtcdMachineSnapshot.
type EtcdMachineSnapshotStatus struct {
	Bar string `json:"bar,omitempty""`
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
