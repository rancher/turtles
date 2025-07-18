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
	"cmp"
	"strings"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

var _ operatorv1.GenericProvider = &CAPIProvider{}

// GetConditions returns the Conditions field from the CAPIProvider status.
func (b *CAPIProvider) GetConditions() clusterv1.Conditions {
	return b.Status.Conditions
}

// SetConditions updates the Conditions field in the CAPIProvider status.
func (b *CAPIProvider) SetConditions(conditions clusterv1.Conditions) {
	b.Status.Conditions = conditions
}

// GetSpec returns the Spec field in the CAPIProvider status.
func (b *CAPIProvider) GetSpec() operatorv1.ProviderSpec {
	return b.Spec.ProviderSpec
}

// SetSpec updates the Spec field in the CAPIProvider status.
func (b *CAPIProvider) SetSpec(in operatorv1.ProviderSpec) {
	b.Spec.ProviderSpec = in
}

// GetStatus returns the Status.ProviderStatus field in the CAPIProvider status.
func (b *CAPIProvider) GetStatus() operatorv1.ProviderStatus {
	return b.Status.ProviderStatus
}

// SetStatus updates the Status.ProviderStatus field in the CAPIProvider status.
func (b *CAPIProvider) SetStatus(in operatorv1.ProviderStatus) {
	b.Status.ProviderStatus = in
}

// GetType returns the type of the CAPIProvider.
func (b *CAPIProvider) GetType() string {
	return strings.ToLower(string(b.Spec.Type))
}

// GetItems returns the list of GenericProviders for CAPIProviderList.
func (b *CAPIProviderList) GetItems() []operatorv1.GenericProvider {
	providers := []operatorv1.GenericProvider{}

	for index := range b.Items {
		providers = append(providers, &b.Items[index])
	}

	return providers
}

// SetVariables updates the Variables field in the CAPIProvider status.
func (b *CAPIProvider) SetVariables(v map[string]string) {
	b.Status.Variables = v
}

// SetProviderName updates provider name based on spec field or metadata.name.
func (b *CAPIProvider) SetProviderName() {
	b.Status.Name = cmp.Or(b.Spec.Name, b.Name)
}

// SetPhase updates the Phase field in the CAPIProvider status.
func (b *CAPIProvider) SetPhase(p Phase) {
	b.Status.Phase = p
}

// ProviderName is a name for the managed CAPI provider resource.
func (b *CAPIProvider) ProviderName() string {
	if b.Spec.Name != "" {
		return b.Spec.Name
	}

	return b.Name
}
