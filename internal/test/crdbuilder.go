/*
Copyright 2023 SUSE.

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

package test

import (
	"fmt"
	"strings"

	"github.com/gobuffalo/flect"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
)

var (
	managementGroupVersion      = schema.GroupVersion{Group: "provisioning.cattle.io", Version: "v1"}
	clusterGroupVersion         = schema.GroupVersion{Group: "cluster.x-k8s.io", Version: "v1beta1"}
	clusterRegTokenGroupVersion = schema.GroupVersion{Group: "management.cattle.io", Version: "v3"}

	// fakeRancherClusterKind is the Kind for the RancherCluster object.
	fakeRancherClusterKind = "Cluster"
	// fakeRancherClusterCRD is a fake RancherCluster CRD.
	fakeRancherClusterCRD = generateCRD(managementGroupVersion.WithKind(fakeRancherClusterKind), apiextensionsv1.NamespaceScoped)

	// fakeRegistrationTokenKind is the Kind for the RegistrationToken object.
	fakeRegistrationTokenKind = "ClusterRegistrationToken"
	// fakeRegistrationTokenCRD is a fake RegistrationToken CRD.
	fakeRegistrationTokenCRD = generateCRD(clusterRegTokenGroupVersion.WithKind(fakeRegistrationTokenKind), apiextensionsv1.NamespaceScoped)

	// fakeCAPIClusterKind is the Kind for the CAPI Cluster object.
	fakeCAPIClusterKind = "Cluster"
	// fakeCAPIClusterCRD is a fake CAPI Cluster CRD.
	fakeCAPIClusterCRD = generateCRD(clusterGroupVersion.WithKind(fakeCAPIClusterKind), apiextensionsv1.NamespaceScoped)
)

func generateCRD(gvk schema.GroupVersionKind, scope apiextensionsv1.ResourceScope) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		TypeMeta: metav1.TypeMeta{
			APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", flect.Pluralize(strings.ToLower(gvk.Kind)), gvk.Group),
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: gvk.Group,
			Scope: scope,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:   gvk.Kind,
				Plural: flect.Pluralize(strings.ToLower(gvk.Kind)),
			},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    gvk.Version,
					Served:  true,
					Storage: true,
					Subresources: &apiextensionsv1.CustomResourceSubresources{
						Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
					},
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"spec": {
									Type:                   "object",
									XPreserveUnknownFields: pointer.Bool(true),
								},
								"value": {
									Type:                   "string",
									XPreserveUnknownFields: pointer.Bool(true),
								},
								"status": {
									Type:                   "object",
									XPreserveUnknownFields: pointer.Bool(true),
								},
							},
						},
					},
				},
			},
		},
	}
}
