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

import clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

const (
	// RancherCredentialsSecretCondition provides information on Rancher credentials secret mapping result.
	RancherCredentialsSecretCondition clusterv1.ConditionType = "RancherCredentialsSecretMapped"

	// RancherCredentialKeyMissing notifies about missing credential secret key required for provider during credentials mapping.
	RancherCredentialKeyMissing = "RancherCredentialKeyMissing"

	// RancherCredentialSourceMissing occures when a source credential secret is missing.
	RancherCredentialSourceMissing = "RancherCredentialSourceMissing"

	// LastAppliedConfigurationTime is set as a timestamp infor of the last configuration update byt the CAPI Operator resource.
	LastAppliedConfigurationTime = "LastAppliedConfigurationTime"
)
