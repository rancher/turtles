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

package rancher

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var gvkRancherClusterRegToken = schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "ClusterRegistrationToken"}

// ClusterRegistrationToken is the struct representing a Rancher ClusterRegistrationToken.
type ClusterRegistrationToken struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            ClusterRegistrationTokenStatus `json:"status,omitempty"`
}

// ClusterRegistrationTokenStatus is the struct representing the status of a Rancher ClusterRegistrationToken.
type ClusterRegistrationTokenStatus struct {
	ManifestURL string `json:"manifestUrl"`
}

// ToUnstructured converts a ClusterRegistrationToken to an unstructured object.
func (r *ClusterRegistrationToken) ToUnstructured() (*unstructured.Unstructured, error) {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
	if err != nil {
		return nil, fmt.Errorf("failed to convert token: %w", err)
	}

	clusterRegistrationToken := &unstructured.Unstructured{}
	clusterRegistrationToken.SetUnstructuredContent(obj)
	clusterRegistrationToken.SetGroupVersionKind(gvkRancherClusterRegToken)

	return clusterRegistrationToken, nil
}

// FromUnstructured converts an unstructured object to a ClusterRegistrationToken.
func (r *ClusterRegistrationToken) FromUnstructured(clusterRegistrationToken *unstructured.Unstructured) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(clusterRegistrationToken.Object, r)
}

// ClusterRegistrationTokenHandler is the struct allowing to interact with Rancher ClusterRegistrationToken.
type ClusterRegistrationTokenHandler struct {
	cl  client.Client
	ctx context.Context
}

// NewClusterRegistrationTokenHandler returns a new ClusterRegistrationTokenHandler.
func NewClusterRegistrationTokenHandler(ctx context.Context, cl client.Client) *ClusterRegistrationTokenHandler {
	return &ClusterRegistrationTokenHandler{
		cl:  cl,
		ctx: ctx,
	}
}

// Get gets Rancher ClusterRegistrationToken and converts it to wrapper structure.
func (h *ClusterRegistrationTokenHandler) Get(objKey client.ObjectKey) (*ClusterRegistrationToken, error) {
	clusterRegistrationTokenUnstructured := &unstructured.Unstructured{}
	clusterRegistrationTokenUnstructured.SetGroupVersionKind(gvkRancherClusterRegToken)

	if err := h.cl.Get(h.ctx, objKey, clusterRegistrationTokenUnstructured); err != nil {
		return nil, fmt.Errorf("failed to get cluster registration token: %w", err)
	}

	clusterRegistrationToken := &ClusterRegistrationToken{}
	if err := clusterRegistrationToken.FromUnstructured(clusterRegistrationTokenUnstructured); err != nil {
		return nil, fmt.Errorf("failed to convert cluster registration token: %w", err)
	}

	return clusterRegistrationToken, nil
}

// Create creates Rancher ClusterRegistrationToken.
func (h *ClusterRegistrationTokenHandler) Create(clusterRegistrationToken *ClusterRegistrationToken) error {
	clusterRegistrationTokenUnstructured, err := clusterRegistrationToken.ToUnstructured()
	if err != nil {
		return fmt.Errorf("failed to create cluster registration token: %w", err)
	}

	if err := h.cl.Create(h.ctx, clusterRegistrationTokenUnstructured); err != nil {
		return fmt.Errorf("failed to create cluster registration token: %w", err)
	}

	return nil
}

// UpdateStatus updates Rancher ClusterRegistrationToken status.
func (h *ClusterRegistrationTokenHandler) UpdateStatus(clusterRegistrationToken *ClusterRegistrationToken) error {
	clusterRegistrationTokenUnstructured, err := clusterRegistrationToken.ToUnstructured()
	if err != nil {
		return fmt.Errorf("failed to convert cluster registration token: %w", err)
	}

	if err := h.cl.Status().Update(h.ctx, clusterRegistrationTokenUnstructured); err != nil {
		return fmt.Errorf("failed to update cluster registration token status: %w", err)
	}

	return nil
}
