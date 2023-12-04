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
)

// ToKind converts ProviderType to CAPI Operator provider object kind.
func (t Type) ToKind() string {
	return cases.Title(language.English).String(string(t)) + "Provider"
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
