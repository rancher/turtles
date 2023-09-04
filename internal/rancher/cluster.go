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

var gvkRancherCluster = schema.GroupVersionKind{Group: "provisioning.cattle.io", Version: "v1", Kind: "Cluster"}

// Cluster is the struct representing a Rancher Cluster.
type Cluster struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            ClusterStatus `json:"status,omitempty"`
}

// ClusterStatus is the struct representing the status of a Rancher Cluster.
type ClusterStatus struct {
	ClusterName   string `json:"clusterName,omitempty"`
	AgentDeployed bool   `json:"agentDeployed,omitempty"`
	Ready         bool   `json:"ready,omitempty"`
}

// ToUnstructured converts a Cluster to an unstructured object.
func (r *Cluster) ToUnstructured() (*unstructured.Unstructured, error) {
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cluster: %w", err)
	}

	rancherClusterUnstructured := &unstructured.Unstructured{}
	rancherClusterUnstructured.SetUnstructuredContent(obj)
	rancherClusterUnstructured.SetGroupVersionKind(gvkRancherCluster)

	return rancherClusterUnstructured, nil
}

// FromUnstructured converts an unstructured object to a Cluster.
func (r *Cluster) FromUnstructured(rancherClusterUnstructured *unstructured.Unstructured) error {
	return runtime.DefaultUnstructuredConverter.FromUnstructured(rancherClusterUnstructured.Object, r)
}

// ClusterHandler is the struct handling the Rancher Cluster.
type ClusterHandler struct {
	cl  client.Client
	ctx context.Context
}

// NewClusterHandler creates a new ClusterHandler.
func NewClusterHandler(ctx context.Context, cl client.Client) *ClusterHandler {
	return &ClusterHandler{
		cl:  cl,
		ctx: ctx,
	}
}

// Get gets Rancher cluster and converts it to wrapper structure.
func (h *ClusterHandler) Get(objKey client.ObjectKey) (*Cluster, error) {
	rancherClusterUnstructured := &unstructured.Unstructured{}
	rancherClusterUnstructured.SetGroupVersionKind(gvkRancherCluster)

	if err := h.cl.Get(h.ctx, objKey, rancherClusterUnstructured); err != nil {
		return nil, fmt.Errorf("failed to get rancher cluster: %w", err)
	}

	rancherCluster := &Cluster{}
	if err := rancherCluster.FromUnstructured(rancherClusterUnstructured); err != nil {
		return nil, fmt.Errorf("failed to convert rancher cluster: %w", err)
	}

	return rancherCluster, nil
}

// Create creates Rancher cluster.
func (h *ClusterHandler) Create(rancherCluster *Cluster) error {
	rancherClusterUnstructured, err := rancherCluster.ToUnstructured()
	if err != nil {
		return fmt.Errorf("failed to convert rancher cluster: %w", err)
	}

	if err := h.cl.Create(h.ctx, rancherClusterUnstructured); err != nil {
		return fmt.Errorf("failed to create rancher cluster: %w", err)
	}

	return nil
}

// Delete deletes Rancher cluster.
func (h *ClusterHandler) Delete(rancherCluster *Cluster) error {
	rancherClusterUnstructured, err := rancherCluster.ToUnstructured()
	if err != nil {
		return fmt.Errorf("failed to convert rancher cluster: %w", err)
	}

	if err := h.cl.Delete(h.ctx, rancherClusterUnstructured); err != nil {
		return fmt.Errorf("failed to delete rancher cluster: %w", err)
	}

	return nil
}

// UpdateStatus updates Rancher cluster status.
func (h *ClusterHandler) UpdateStatus(rancherCluster *Cluster) error {
	rancherClusterUnstructured, err := rancherCluster.ToUnstructured()
	if err != nil {
		return fmt.Errorf("failed to convert rancher cluster: %w", err)
	}

	if err := h.cl.Status().Update(h.ctx, rancherClusterUnstructured); err != nil {
		return fmt.Errorf("failed to update rancher cluster status: %w", err)
	}

	return nil
}
