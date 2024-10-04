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

// RKE2EtcdMachineSnapshotConfigSpec defines the desired state of RKE2EtcdMachineSnapshotConfig
type RKE2EtcdMachineSnapshotConfigSpec struct {
	S3    S3Config    `json:"s3"`
	Local LocalConfig `json:"local"`
}

type LocalConfig struct {
	DataDir string `json:"dataDir"`
}

type S3Config struct {
	Endpoint           string `json:"endpoint,omitempty"`
	EndpointCASecret   string `json:"endpointCAsecret,omitempty"`
	SkipSSLVerify      bool   `json:"skipSSLVerify,omitempty"`
	S3CredentialSecret string `json:"s3CredentialSecret,omitempty"`
	Bucket             string `json:"bucket,omitempty"`
	Region             string `json:"region,omitempty"`
	Folder             string `json:"folder,omitempty"`
	Insecure           bool   `json:"insecure,omitempty"`
	Location           string `json:"location,omitempty"`
}

// RKE2EtcdMachineSnapshotConfig is the schema for the snapshot config.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// RKE2EtcdMachineSnapshotConfig is the config for the RKE2EtcdMachineSnapshotConfig API
type RKE2EtcdMachineSnapshotConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec RKE2EtcdMachineSnapshotConfigSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// RKE2EtcdMachineSnapshotConfigList contains a list of RKE2EtcdMachineSnapshotConfigs.
type RKE2EtcdMachineSnapshotConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ETCDSnapshotRestore `json:"items"`
}

func init() {
	objectTypes = append(objectTypes, &RKE2EtcdMachineSnapshotConfig{}, &RKE2EtcdMachineSnapshotConfigList{})
}
