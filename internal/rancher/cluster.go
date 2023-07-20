package rancher

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	gvkRancherCluster = schema.GroupVersionKind{Group: "provisioning.cattle.io", Version: "v1", Kind: "Cluster"}

	// stringToTimeHook is a hook for the mapstructure library to proprely decode into metav1.Time
	stringToTimeHook = func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if f.Kind() == reflect.String && t == reflect.TypeOf(metav1.Time{}) {
			time, err := time.Parse(time.RFC3339, data.(string))
			return metav1.Time{Time: time}, err
		}
		return data, nil
	}
)

type Cluster struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            ClusterStatus `json:"status,omitempty"`
}

type ClusterStatus struct {
	ClusterName   string `json:"clusterName,omitempty"`
	AgentDeployed bool   `json:"agentDeployed,omitempty"`
}

func (r *Cluster) ToUnstructured() (*unstructured.Unstructured, error) {
	rancherClusterUnstructured := &unstructured.Unstructured{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:    "json",
		Result:     &rancherClusterUnstructured.Object,
		DecodeHook: stringToTimeHook,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(r); err != nil {
		return nil, fmt.Errorf("failed to decode rancher cluster: %w", err)
	}

	rancherClusterUnstructured.SetGroupVersionKind(gvkRancherCluster)
	rancherClusterUnstructured.SetCreationTimestamp(metav1.Now())

	return rancherClusterUnstructured, nil
}

func (r *Cluster) FromUnstructured(rancherClusterUnstructured *unstructured.Unstructured) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:    "json",
		Result:     &r,
		DecodeHook: stringToTimeHook,
	})
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(rancherClusterUnstructured.Object); err != nil {
		return fmt.Errorf("failed to decode rancher cluster: %w", err)
	}

	return nil
}

type ClusterHandler struct {
	cl  client.Client
	ctx context.Context
}

func NewClusterHandler(ctx context.Context, cl client.Client) *ClusterHandler {
	return &ClusterHandler{
		cl:  cl,
		ctx: ctx,
	}
}

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
