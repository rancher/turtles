package util

import (
	"context"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// ShouldImport checks if the object has the label set to true.
func ShouldImport(obj metav1.Object, label string) (hasLabel bool, labelValue bool) {
	labelVal, ok := obj.GetLabels()[label]
	if !ok {
		return false, false
	}

	autoImport, err := strconv.ParseBool(labelVal)
	if err != nil {
		return true, false
	}

	return true, autoImport
}

// ShouldAutoImport checks if the namespace or cluster has the label set to true.
func ShouldAutoImport(ctx context.Context, logger logr.Logger, cl client.Client, capiCluster *clusterv1.Cluster, label string) (bool, error) {
	logger.V(2).Info("should we auto import the capi cluster", "name", capiCluster.Name, "namespace", capiCluster.Namespace)

	// Check CAPI cluster for label first
	hasLabel, autoImport := ShouldImport(capiCluster, label)
	if hasLabel && autoImport {
		logger.V(2).Info("Cluster contains import annotation")

		return true, nil
	}

	if hasLabel && !autoImport {
		logger.V(2).Info("Cluster contains annotation to not import")

		return false, nil
	}

	// Check namespace wide
	ns := &corev1.Namespace{}
	key := client.ObjectKey{Name: capiCluster.Namespace}

	if err := cl.Get(ctx, key, ns); err != nil {
		logger.Error(err, "getting namespace")
		return false, err
	}

	_, autoImport = ShouldImport(ns, label)

	return autoImport, nil
}
