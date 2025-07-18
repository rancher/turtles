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
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Kind is a CAPIProvider kind string.
const Kind = "CAPIProvider"

// Type defines the type of the CAPI Provider.
type Type string

const (
	// Infrastructure is the name for the infrastructure CAPI Provider.
	Infrastructure Type = "infrastructure"
	// Core is the name for core CAPI Provider.
	Core Type = "core"
	// ControlPlane is the name for the controlPlane CAPI Provider.
	ControlPlane Type = "controlPlane"
	// Bootstrap is the name for the bootstrap CAPI Provider.
	Bootstrap Type = "bootstrap"
	// Addon is the name for the addon CAPI Provider.
	Addon Type = "addon"
	// IPAM is the name for the addon for IPAM CAPI Provider.
	IPAM Type = "ipam"
	// RuntimeExtension is the name for the RuntimeExtension Provider.
	RuntimeExtension Type = "runtimeextension"
)

// ToKind converts ProviderType to CAPI Operator provider object kind.
func (t Type) ToKind() string {
	return cases.Title(language.English).String(string(t)) + "Provider"
}

// ToName converts ProviderType to CAPI Operator provider object name prefix.
func (t Type) ToName() string {
	switch t {
	case Infrastructure:
		return "infrastructure-"
	case Core:
		return "core-"
	case ControlPlane:
		return "control-plane-"
	case Bootstrap:
		return "bootstrap-"
	case Addon:
		return "addon-"
	case IPAM:
		return "ipam-"
	case RuntimeExtension:
		return "runtime-extension-"
	default:
		return ""
	}
}

// Phase defines the current state of the CAPI Provider resource.
type Phase string

const (
	// Pending status identifies a provder which has not yet started provisioning.
	Pending Phase = "Pending"
	// Provisioning status defines provider in a provisioning state.
	Provisioning Phase = "Provisioning"
	// Ready status identifies that the provider is ready to be used.
	Ready Phase = "Ready"
	// Failed status defines a failed state of provider provisioning.
	Failed Phase = "Failed"
)
