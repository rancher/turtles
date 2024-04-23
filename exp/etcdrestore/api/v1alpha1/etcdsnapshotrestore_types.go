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

// EtcdSnapshotRestoreSpec defines the desired state of EtcdSnapshotRestore.
type EtcdSnapshotRestoreSpec struct {
	Foo string `json:"foo"`
}

// EtcdSnapshotRestoreStatus defines observed state of EtcdSnapshotRestore.
type EtcdSnapshotRestoreStatus struct {
	Bar string `json:"bar"`
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
