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

// ClusterctlConfigName is a name of the clusterctl config in any namespace.
const (
	ClusterctlConfigName string = "clusterctl-config"
)

// ClusterctlConfigSpec defines the user overrides for images and known providers with sources
//
//nolint:lll
type ClusterctlConfigSpec struct {
	// Images is a list of image overrided for specified providers
	Images []Image `json:"images"`

	// Provider overrides
	Providers ProviderList `json:"providers"`
}

// Provider allows to define providers with known URLs to pull the components.
type Provider struct {
	// Name of the provider
	// +required
	Name string `json:"name"`

	// URL of the provider components. Will be used unless and override is specified
	// +required
	URL string `json:"url"`

	// Type is the type of the provider
	// +required
	// +kubebuilder:validation:Enum=infrastructure;core;controlPlane;bootstrap;addon;runtimeextension;ipam
	// +kubebuilder:example=infrastructure
	ProviderType Type `json:"type"`
}

// ProviderList is a list of providers.
type ProviderList []Provider

// Image allows to define transformations to apply to the image contained in the YAML manifests.
type Image struct {
	// Repository sets the container registry override to pull images from.
	// +kubebuilder:example=my-registry/my-org
	Repository string `json:"repository,omitempty"`

	// Tag allows to specify a tag for the images.
	Tag string `json:"tag,omitempty"`

	// Name of the provider image override
	// +required
	// +kubebuilder:example=all
	Name string `json:"name"`
}

// ClusterctlConfig is the Schema for the CAPI Clusterctl config API.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:validation:XValidation:message="Clusterctl Config should be named clusterctl-config.",rule="self.metadata.name == 'clusterctl-config'"
type ClusterctlConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec ClusterctlConfigSpec `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterctlConfigList contains a list of ClusterctlConfigs.
type ClusterctlConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CAPIProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterctlConfig{}, &ClusterctlConfigList{})
}
