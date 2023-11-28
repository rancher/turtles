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

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
)

const (
	// ProviderFinalizer is the finalizer apply on the CAPI Provider resource.
	ProviderFinalizer = "capiprovider.turtles.cattle.io"
)

// CAPIProviderSpec defines the desired state of CAPIProvider.
// +kubebuilder:validation:XValidation:message="CAPI Provider version should be in the semver format",rule="!has(self.version) || self.version.matches(r\"\"\"^([0-9]+)\\.([0-9]+)\\.([0-9]+)(?:-([0-9A-Za-z-]+(?:\\.[0-9A-Za-z-]+)*))?(?:\\+[0-9A-Za-z-]+)?$\"\"\")"
//
//nolint:lll
type CAPIProviderSpec struct {
	// Name is the name of the provider to enable
	// +required
	// +kubebuilder:validation:Enum=aws;azure;gcp;docker;rke2
	Name string `json:"name"`

	// Type is the type of the provider to enable
	// +required
	// +kubebuilder:validation:Enum=infrastructure;core;controlPlane;bootstrap;addon
	Type string `json:"type"`

	// Credentials is the structure holding the credentials to use for the provider.
	// +optional
	Credentials *ProviderCredentials `json:"credentials,omitempty"`

	// Features is a collection of features to enable.
	Features *Features `json:"features,omitempty"`

	// Variables is a map of environment variables to add to the content of the ConfigSecret
	// +optional
	Variables map[string]string `json:"variables"`

	// ProviderSpec is the spec
	ProviderSpec *operatorv1.ProviderSpec `json:",inline"`
}

// Features defines a collection of features for the CAPI Provider to apply.
type Features struct {
	// MachinePool if set to true will enable the machine pool. feature
	MachinePool bool `json:"machinePool,omitempty"`

	// ClusterResourceSet if set to true will enable the cluster resource set feature
	ClusterResourceSet bool `json:"clusterResourceSet,omitempty"`

	// ClusterTopology if set to true will enable the clusterclass feature.
	ClusterTopology bool `json:"clusterTopology,omitempty"`
}

// ProviderCredentials defines the external credentials information for the provider.
type ProviderCredentials struct {
	// RancherCloudCredential is the Rancher Cloud Credential name
	// +required
	RancherCloudCredential string `json:"rancherCloudCredential"`

	// +optional
	// TODO: decide how to handle workload identity
	// WorkloadIdentityRef *WorkloadIdentityRef `json:"workloadIdentityRef,omitempty"`
}

// WorkloadIdentityRef is a reference to an identity to be used when reconciling the cluster.
type WorkloadIdentityRef struct {
	// Name of the identity
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Kind of the identity
	Kind string `json:"kind"`
}

// CAPIProviderStatus defines the observed state of CAPIProvider.
type CAPIProviderStatus struct {
	// Ready indicates if the provider is ready to be used
	// +kubebuilder:default=false
	Ready bool `json:"ready,omitempty"`

	// Version indicates the version of the provider installed.
	// +kubebuilder:default="latest"
	Version string `json:"version,omitempty"`

	// Variables is a map of environment variables added to the content of the ConfigSecret
	// +kubebuilder:default={CLUSTER_TOPOLOGY:"true",EXP_CLUSTER_RESOURCE_SET:"true",EXP_MACHINE_POOL: "true"}
	Variables map[string]string `json:"variables,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CAPIProvider is the Schema for the CAPI Providers API.
type CAPIProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec CAPIProviderSpec `json:"spec,omitempty"`

	// +kubebuilder:default={}
	Status CAPIProviderStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CAPIProviderList contains a list of CAPIProviders.
type CAPIProviderList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CAPIProvider `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CAPIProvider{}, &CAPIProviderList{})
}
