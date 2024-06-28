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

package controllers

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	utilyaml "sigs.k8s.io/cluster-api/util/yaml"

	managementv3 "github.com/rancher/turtles/internal/rancher/management/v3"
	"github.com/rancher/turtles/util"
)

const (
	importLabelName           = "cluster-api.cattle.io/rancher-auto-import"
	ownedLabelName            = "cluster-api.cattle.io/owned"
	capiClusterOwner          = "cluster-api.cattle.io/capi-cluster-owner"
	capiClusterOwnerNamespace = "cluster-api.cattle.io/capi-cluster-owner-ns"
	v1ClusterMigrated         = "cluster-api.cattle.io/migrated"

	defaultRequeueDuration = 1 * time.Minute
)

func getClusterRegistrationManifest(ctx context.Context, clusterName, namespace string, cl client.Client,
	insecureSkipVerify bool,
) (string, error) {
	log := log.FromContext(ctx)

	token := &managementv3.ClusterRegistrationToken{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: namespace,
		},
		Spec: managementv3.ClusterRegistrationTokenSpec{
			ClusterName: clusterName,
		},
	}
	err := cl.Get(ctx, client.ObjectKeyFromObject(token), token)

	if client.IgnoreNotFound(err) != nil {
		return "", fmt.Errorf("error getting registration token for cluster %s: %w", clusterName, err)
	} else if err != nil {
		if err := cl.Create(ctx, token); err != nil {
			return "", fmt.Errorf("failed to create cluster registration token for cluster %s: %w", clusterName, err)
		}
	}

	if token.Status.ManifestURL == "" {
		return "", nil
	}

	manifestData, err := downloadManifest(token.Status.ManifestURL, insecureSkipVerify)
	if err != nil {
		log.Error(err, "failed downloading import manifest")
		return "", err
	}

	return manifestData, nil
}

func namespaceToCapiClusters(ctx context.Context, clusterPredicate predicate.Funcs, cl client.Client) handler.MapFunc {
	log := log.FromContext(ctx)

	return func(_ context.Context, o client.Object) []ctrl.Request {
		ns, ok := o.(*corev1.Namespace)
		if !ok {
			log.Error(nil, fmt.Sprintf("Expected a Namespace but got a %T", o))
			return nil
		}

		if _, autoImport := util.ShouldImport(ns, importLabelName); !autoImport {
			log.V(2).Info("Namespace doesn't have import annotation label with a true value, skipping")
			return nil
		}

		capiClusters := &clusterv1.ClusterList{}
		if err := cl.List(ctx, capiClusters, client.InNamespace(o.GetNamespace())); err != nil {
			log.Error(err, "getting capi cluster")
			return nil
		}

		if len(capiClusters.Items) == 0 {
			log.V(2).Info("No CAPI clusters in namespace, no action")
			return nil
		}

		reqs := []ctrl.Request{}

		for _, cluster := range capiClusters.Items {
			cluster := cluster
			if !clusterPredicate.Generic(event.GenericEvent{Object: &cluster}) {
				continue
			}

			reqs = append(reqs, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
				},
			})
		}

		return reqs
	}
}

func downloadManifest(url string, insecureSkipVerify bool) (string, error) {
	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify, //nolint:gosec
		},
	}}

	resp, err := client.Get(url) //nolint:gosec,noctx
	if err != nil {
		return "", fmt.Errorf("downloading manifest: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading manifest: %w", err)
	}

	return string(data), err
}

func createImportManifest(ctx context.Context, remoteClient client.Client, in io.Reader) error {
	reader := yamlDecoder.NewYAMLReader(bufio.NewReaderSize(in, 4096))

	for {
		raw, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return err
		}

		if err := createRawManifest(ctx, remoteClient, raw); err != nil {
			return err
		}
	}

	return nil
}

func createRawManifest(ctx context.Context, remoteClient client.Client, bytes []byte) error {
	items, err := utilyaml.ToUnstructured(bytes)
	if err != nil {
		return fmt.Errorf("error unmarshalling bytes or empty object passed: %w", err)
	}

	for _, obj := range items {
		if err := createObject(ctx, remoteClient, obj.DeepCopy()); err != nil {
			return err
		}
	}

	return nil
}

func createObject(ctx context.Context, c client.Client, obj client.Object) error {
	log := log.FromContext(ctx)
	gvk := obj.GetObjectKind().GroupVersionKind()

	err := c.Create(ctx, obj)
	if apierrors.IsAlreadyExists(err) {
		log.V(4).Info("object already exists in remote cluster", "gvk", gvk, "name", obj.GetName(), "namespace", obj.GetNamespace())
		return nil
	}

	if err != nil {
		return fmt.Errorf("creating object in remote cluster: %w", err)
	}

	log.V(4).Info("object was created", "gvk", gvk, "name", obj.GetName(), "namespace", obj.GetNamespace())

	return nil
}
