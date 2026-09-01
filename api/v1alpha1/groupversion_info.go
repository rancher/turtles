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

// +kubebuilder:object:generate=true
// +groupName=turtles-capi.cattle.io

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
)

var (
	// SchemeGroupVersion is group version used to register these objects.
	SchemeGroupVersion = schema.GroupVersion{Group: "turtles-capi.cattle.io", Version: "v1alpha1"}
	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&CAPIProvider{},
		&CAPIProviderList{},
		&ClusterctlConfig{},
		&ClusterctlConfigList{},
		&operatorv1.CoreProvider{},
		&operatorv1.BootstrapProvider{},
		&operatorv1.ControlPlaneProvider{},
		&operatorv1.InfrastructureProvider{},
		&operatorv1.AddonProvider{},
		&operatorv1.IPAMProvider{},
		&operatorv1.CoreProviderList{},
		&operatorv1.BootstrapProviderList{},
		&operatorv1.ControlPlaneProviderList{},
		&operatorv1.InfrastructureProviderList{},
		&operatorv1.AddonProviderList{},
		&operatorv1.IPAMProviderList{},
	)

	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	metav1.AddToGroupVersion(scheme, operatorv1.GroupVersion)

	return nil
}
