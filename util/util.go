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

package util

import (
	"context"
	"strconv"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	turtlesannotations "github.com/rancher/turtles/util/annotations"
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
	if hasLabel && autoImport && !turtlesannotations.HasClusterImportAnnotation(capiCluster) {
		logger.V(2).Info("Cluster contains import label and has no `imported` annotation")

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
