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

package api

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
)

var (
	_ Provider     = &turtlesv1.CAPIProvider{}
	_ ProviderList = &turtlesv1.CAPIProviderList{}
)

// Provider is a interface on top of GenericProvider,
// providing CAPIProvider compatible functionality with client.Object.
type Provider interface {
	client.Object
	operatorv1.GenericProvider
}

// ProviderList is a interface on top of GenericProviderList,
// providing CAPIProviderList compatible functionality with client.Object.
type ProviderList interface {
	client.ObjectList
	operatorv1.GenericProviderList
}
