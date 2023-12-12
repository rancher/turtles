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

// Package v1alpha1 contains API Schema definitions for the turtles-capi.cattle.io v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=turtles-capi.cattle.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
)

var (
	// GroupVersion is group version used to register these objects.
	GroupVersion = schema.GroupVersion{Group: "turtles-capi.cattle.io", Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

// AddKnownTypes adds the list of known types to api.Scheme.
func AddKnownTypes(scheme *runtime.Scheme) {
	scheme.AddKnownTypes(GroupVersion, &CAPIProvider{}, &CAPIProviderList{})
	scheme.AddKnownTypes(operatorv1.GroupVersion,
		&operatorv1.CoreProvider{}, &operatorv1.CoreProviderList{},
		&operatorv1.BootstrapProvider{}, &operatorv1.BootstrapProviderList{},
		&operatorv1.ControlPlaneProvider{}, &operatorv1.ControlPlaneProviderList{},
		&operatorv1.InfrastructureProvider{}, &operatorv1.InfrastructureProviderList{},
		&operatorv1.AddonProvider{}, &operatorv1.AddonProviderList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	metav1.AddToGroupVersion(scheme, operatorv1.GroupVersion)
}
