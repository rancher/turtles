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

package predicates

import (
	"context"
	"strings"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	"github.com/rancher/turtles/util"
	"github.com/rancher/turtles/util/annotations"
)

// ClusterWithoutImportedAnnotation returns a predicate that returns true only if the provided resource does not contain
// "clusterImportedAnnotation" annotation. When annotation is present on the resource, controller will skip reconciliation.
func ClusterWithoutImportedAnnotation(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return processIfClusterNotImported(logger.WithValues("predicate", "ClusterWithoutImportedAnnotation", "eventType", "update"), e.ObjectNew)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return processIfClusterNotImported(logger.WithValues("predicate", "ClusterWithoutImportedAnnotation", "eventType", "create"), e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return processIfClusterNotImported(logger.WithValues("predicate", "ClusterWithoutImportedAnnotation", "eventType", "delete"), e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return processIfClusterNotImported(logger.WithValues("predicate", "ClusterWithoutImportedAnnotation", "eventType", "generic"), e.Object)
		},
	}
}

// processIfClusterNotImported returns true if the provided object is a cluster and does not have the imported annotation.
func processIfClusterNotImported(logger logr.Logger, obj client.Object) bool {
	kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	log := logger.WithValues("namespace", obj.GetNamespace(), kind, obj.GetName())

	if annotations.HasAnnotation(obj, annotations.ClusterImportedAnnotation) {
		log.V(4).Info("Cluster has an import annotation, will not attempt to map resource")
		return false
	}

	log.V(6).Info("Cluster does not have an import annotation, will attempt to map resource")

	return true
}

// ClusterWithReadyControlPlane returns a predicate that returns true only if the provided resource is a cluster with a
// ready control plane.
func ClusterWithReadyControlPlane(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return processIfClusterReadyControlPlane(logger.WithValues("predicate", "ClusterWithReadyControlPlane", "eventType", "update"), e.ObjectNew)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return processIfClusterReadyControlPlane(logger.WithValues("predicate", "ClusterWithReadyControlPlane", "eventType", "create"), e.Object)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return processIfClusterReadyControlPlane(logger.WithValues("predicate", "ClusterWithReadyControlPlane", "eventType", "delete"), e.Object)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return processIfClusterReadyControlPlane(logger.WithValues("predicate", "ClusterWithReadyControlPlane", "eventType", "generic"), e.Object)
		},
	}
}

// processIfClusterReadyControlPlane returns true if the provided object is a cluster and has a ready control plane.
func processIfClusterReadyControlPlane(logger logr.Logger, obj client.Object) bool {
	kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	log := logger.WithValues("namespace", obj.GetNamespace(), kind, obj.GetName())

	cluster, ok := obj.(*clusterv1.Cluster)
	if !ok {
		log.V(4).Info("Expected a Cluster but got a different object, will not attempt to map resource", "object", obj)
		return false
	}

	if cluster.Status.ControlPlaneReady {
		log.V(6).Info("Cluster has a ready control plane, will attempt to map resource")
		return true
	}

	if conditions.IsTrue(cluster, clusterv1.ControlPlaneReadyCondition) {
		log.V(6).Info("Cluster has a ready control plane condition, will attempt to map resource")
		return true
	}

	log.V(4).Info("Cluster does not have a ready control plane, will not attempt to map resource")

	return false
}

// ClusterOrNamespaceWithImportLabel returns a predicate that returns true only if the provided resource is a cluster and
// has an import label set on it or on its namespace.
func ClusterOrNamespaceWithImportLabel(ctx context.Context, logger logr.Logger, cl client.Client, label string) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return processIfClusterOrNamespaceWithImportLabel(ctx,
				logger.WithValues("predicate", "ClusterOrNamespaceWithImportLabel", "eventType", "update"), cl, e.ObjectNew, label)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return processIfClusterOrNamespaceWithImportLabel(ctx,
				logger.WithValues("predicate", "ClusterOrNamespaceWithImportLabel", "eventType", "create"), cl, e.Object, label)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return processIfClusterOrNamespaceWithImportLabel(ctx,
				logger.WithValues("predicate", "ClusterOrNamespaceWithImportLabel", "eventType", "delete"), cl, e.Object, label)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return processIfClusterOrNamespaceWithImportLabel(ctx,
				logger.WithValues("predicate", "ClusterOrNamespaceWithImportLabel", "eventType", "generic"), cl, e.Object, label)
		},
	}
}

// processIfClusterOrNamespaceWithImportLabel returns true if the provided object is a cluster and has an import label. If the
// label is not set on the cluster, it will check if it is set on the cluster's namespace.
func processIfClusterOrNamespaceWithImportLabel(ctx context.Context, logger logr.Logger, cl client.Client, obj client.Object, label string) bool {
	kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	log := logger.WithValues("namespace", obj.GetNamespace(), kind, obj.GetName())

	cluster, ok := obj.(*clusterv1.Cluster)
	if !ok {
		log.V(4).Info("Expected a Cluster but got a different object, will not attempt to map resource", "object", obj)
		return false
	}

	shouldImport, err := util.ShouldAutoImport(ctx, log, cl, cluster, label)
	if err != nil {
		log.Error(err, "namespace or cluster has already import annotation set, ignoring it")
		return false
	}

	return shouldImport
}
