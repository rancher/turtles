package v1alpha1

import (
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// GetConditions return CAPI Provider conditions.
func (p *CAPIProvider) GetConditions() clusterv1.Conditions {
	return p.Status.Conditions
}
