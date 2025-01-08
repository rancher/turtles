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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// Setting is the struct representing a Rancher Setting.
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
type Setting struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Value      string `json:"value"`
	Default    string `json:"default,omitempty"`
	Customized bool   `json:"customized,omitempty"`
	Source     string `json:"source"`
}

// SettingList contains a list of Settings.
// +kubebuilder:object:root=true
type SettingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Setting `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Setting{}, &SettingList{})
}
