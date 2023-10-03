package v1

import (
	corev1 "k8s.io/api/core/v1"
)

// RKEConfig represents the specification for an RKE2 based cluster in Rancher.
type RKEConfig struct {
	// InfrastructureRef is a reference to the CAPI infrastructure cluster.
	InfrastructureRef *corev1.ObjectReference `json:"infrastructureRef,omitempty"`
}
