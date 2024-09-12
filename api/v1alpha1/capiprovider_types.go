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

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
)

const (
	// ProviderFinalizer is the finalizer apply on the CAPI Provider resource.
	ProviderFinalizer = "capiprovider.turtles.cattle.io"
)

// CAPIProviderSpec defines the desired state of CAPIProvider.
// +kubebuilder:validation:XValidation:message="CAPI Provider version should be in the semver format prefixed with 'v'. Example: v1.9.3",rule="!has(self.version) || self.version.matches(r\"\"\"^v([0-9]+)\\.([0-9]+)\\.([0-9]+)(?:-([0-9A-Za-z-]+(?:\\.[0-9A-Za-z-]+)*))?(?:\\+[0-9A-Za-z-]+)?$\"\"\")"
// +kubebuilder:validation:XValidation:message="Config secret namespace is always equal to the resource namespace and should not be set.",rule="!has(self.configSecret) || !has(self.configSecret.__namespace__)"
// +kubebuilder:validation:XValidation:message="One of fetchConfig url or selector should be set.",rule="!has(self.fetchConfig) || [has(self.fetchConfig.url), has(self.fetchConfig.selector)].exists_one(e, e)"
//
//nolint:lll
type CAPIProviderSpec struct {
	// Name is the name of the provider to enable
	// +optional
	// +kubebuilder:example=aws
	Name string `json:"name"`

	// Type is the type of the provider to enable
	// +required
	// +kubebuilder:example=InfrastructureProvider
	Type Type `json:"type"`

	// Credentials is the structure holding the credentials to use for the provider. Only one credential type could be set at a time.
	// +kubebuilder:example={rancherCloudCredential: user-credential}
	// +optional
	Credentials *Credentials `json:"credentials,omitempty"`

	// Features is a collection of features to enable.
	// +optional
	// +kubebuilder:example={machinePool: true, clusterResourceSet: true, clusterTopology: true}
	Features *Features `json:"features,omitempty"`

	// Variables is a map of environment variables to add to the content of the ConfigSecret
	// +optional
	// +kubebuilder:example={CLUSTER_TOPOLOGY:"true",EXP_CLUSTER_RESOURCE_SET:"true",EXP_MACHINE_POOL: "true"}
	Variables map[string]string `json:"variables,omitempty"`

	// ProviderSpec is the spec of the underlying CAPI Provider resource.
	operatorv1.ProviderSpec `json:",inline"`
}

// Features defines a collection of features for the CAPI Provider to apply.
type Features struct {
	// MachinePool if set to true will enable the machine pool feature.
	MachinePool bool `json:"machinePool,omitempty"`

	// ClusterResourceSet if set to true will enable the cluster resource set feature.
	ClusterResourceSet bool `json:"clusterResourceSet,omitempty"`

	// ClusterTopology if set to true will enable the clusterclass feature.
	ClusterTopology bool `json:"clusterTopology,omitempty"`
}

// Credentials defines the external credentials information for the provider.
// +kubebuilder:validation:MaxProperties=1
// +kubebuilder:validation:MinProperties=1
// +kubebuilder:validation:XValidation:message="rancherCloudCredentialNamespaceName should be in the namespace:name format.",rule="!has(self.rancherCloudCredentialNamespaceName) || self.rancherCloudCredentialNamespaceName.matches('^.+:.+$')"
// +structType=atomic
//
//nolint:godot
//nolint:lll
type Credentials struct {
	// RancherCloudCredential is the Rancher Cloud Credential name
	RancherCloudCredential string `json:"rancherCloudCredential,omitempty"`

	// RancherCloudCredentialNamespaceName is the Rancher Cloud Credential namespace:name reference
	RancherCloudCredentialNamespaceName string `json:"rancherCloudCredentialNamespaceName,omitempty"`

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
	// Indicates the provider status
	// +kubebuilder:default=Pending
	Phase Phase `json:"phase,omitempty"`

	// Variables is a map of environment variables added to the content of the ConfigSecret
	// +kubebuilder:default={CLUSTER_TOPOLOGY:"true",EXP_CLUSTER_RESOURCE_SET:"true",EXP_MACHINE_POOL: "true"}
	Variables map[string]string `json:"variables,omitempty"`

	operatorv1.ProviderStatus `json:",inline"`
}

// CAPIProvider is the Schema for the CAPI Providers API.
//
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Type",type="string",JSONPath=".spec.type"
// +kubebuilder:printcolumn:name="ProviderName",type="string",JSONPath=".spec.name"
// +kubebuilder:printcolumn:name="InstalledVersion",type="string",JSONPath=".status.installedVersion"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:validation:XValidation:message="CAPI Provider type should always be set.",rule="has(self.spec.type)"
type CAPIProvider struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:example={name: aws, version: "v2.3.0", type: infrastructure, credentials: {rancherCloudCredential: user-credential}}
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
