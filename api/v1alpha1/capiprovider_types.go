/*
Copyright SUSE 2023.

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

const (
	ProviderFinalizer = "capiprovider.turtles.cattle.io"
)

// CAPIProviderSpec defines the desired state of CAPIProvider
type CAPIProviderSpec struct {
	// Name is the name of the provider to enable.
	Name string `json:"name"`

	// Version indicates the provider version.
	// +optional
	Version string `json:"version,omitempty"`
}

// CAPIProviderStatus defines the observed state of CAPIProvider
type CAPIProviderStatus struct {
	// Ready indicates if the provider is ready to be used.
	Ready bool `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CAPIProvider is the Schema for the capiproviders API
type CAPIProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CAPIProviderSpec   `json:"spec,omitempty"`
	Status CAPIProviderStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CAPIProviderList contains a list of CAPIProvider
type CAPIProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CAPIProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CAPIProvider{}, &CAPIProviderList{})
}
