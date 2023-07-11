package ranchercluster

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

type RancherCluster struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func (r *RancherCluster) ToUnstructured() (*unstructured.Unstructured, error) {
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

func (r *RancherCluster) FromUnstructured(rancherClusterUnstructured *unstructured.Unstructured) error {
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

type rancherClusterHandler struct {
	cl  client.Client
	ctx context.Context
}

func NewRancherClusterHandler(ctx context.Context, cl client.Client) *rancherClusterHandler {
	return &rancherClusterHandler{
		cl:  cl,
		ctx: ctx,
	}
}

func (h *rancherClusterHandler) Get(objKey client.ObjectKey) (*RancherCluster, error) {
	rancherClusterUnstructured := &unstructured.Unstructured{}
	rancherClusterUnstructured.SetGroupVersionKind(gvkRancherCluster)
	if err := h.cl.Get(h.ctx, objKey, rancherClusterUnstructured); err != nil {
		return nil, fmt.Errorf("failed to get rancher cluster: %w", err)
	}

	rancherCluster := &RancherCluster{}
	if err := rancherCluster.FromUnstructured(rancherClusterUnstructured); err != nil {
		return nil, fmt.Errorf("failed to convert rancher cluster: %w", err)
	}

	return rancherCluster, nil
}

func (h *rancherClusterHandler) Create(rancherCluster *RancherCluster) error {
	rancherClusterUnstructured, err := rancherCluster.ToUnstructured()
	if err != nil {
		return fmt.Errorf("failed to convert rancher cluster: %w", err)
	}

	if err := h.cl.Create(h.ctx, rancherClusterUnstructured); err != nil {
		return fmt.Errorf("failed to create rancher cluster: %w", err)
	}

	return nil
}

func (h *rancherClusterHandler) Delete(rancherCluster *RancherCluster) error {
	rancherClusterUnstructured, err := rancherCluster.ToUnstructured()
	if err != nil {
		return fmt.Errorf("failed to convert rancher cluster: %w", err)
	}

	if err := h.cl.Delete(h.ctx, rancherClusterUnstructured); err != nil {
		return fmt.Errorf("failed to delete rancher cluster: %w", err)
	}

	return nil
}
