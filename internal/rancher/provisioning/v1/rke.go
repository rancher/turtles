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

package v1

import (
	corev1 "k8s.io/api/core/v1"
)

// RKEConfig represents the specification for an RKE2 based cluster in Rancher.
type RKEConfig struct {
	// InfrastructureRef is a reference to the CAPI infrastructure cluster.
	InfrastructureRef *corev1.ObjectReference `json:"infrastructureRef,omitempty"`
}
