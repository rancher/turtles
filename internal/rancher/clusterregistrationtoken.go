package rancher

import (
	"context"
	"fmt"

	"github.com/mitchellh/mapstructure"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	gvkRancherClusterRegToken = schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "ClusterRegistrationToken"}
)

type ClusterRegistrationToken struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Status            ClusterRegistrationTokenStatus `json:"status,omitempty"`
}

type ClusterRegistrationTokenStatus struct {
	ManifestURL string `json:"manifestUrl"`
}

func (r *ClusterRegistrationToken) ToUnstructured() (*unstructured.Unstructured, error) {
	clusterRegistrationToken := &unstructured.Unstructured{}
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:    "json",
		Result:     &clusterRegistrationToken.Object,
		DecodeHook: stringToTimeHook,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(r); err != nil {
		return nil, fmt.Errorf("failed to decode cluster registration token: %w", err)
	}

	clusterRegistrationToken.SetGroupVersionKind(gvkRancherClusterRegToken)
	clusterRegistrationToken.SetCreationTimestamp(metav1.Now())

	return clusterRegistrationToken, nil
}

func (r *ClusterRegistrationToken) FromUnstructured(clusterRegistrationToken *unstructured.Unstructured) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		TagName:    "json",
		Result:     &r,
		DecodeHook: stringToTimeHook,
	})
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := decoder.Decode(clusterRegistrationToken.Object); err != nil {
		return fmt.Errorf("failed to decode cluster registration token: %w", err)
	}

	return nil
}

type ClusterRegistrationTokenHandler struct {
	cl  client.Client
	ctx context.Context
}

func NewClusterRegistrationTokenHandler(ctx context.Context, cl client.Client) *ClusterRegistrationTokenHandler {
	return &ClusterRegistrationTokenHandler{
		cl:  cl,
		ctx: ctx,
	}
}

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
