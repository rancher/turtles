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
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	batchv1 "k8s.io/api/batch/v1"
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

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	"github.com/rancher/turtles/util"
)

const (
	importLabelName           = "cluster-api.cattle.io/rancher-auto-import"
	ownedLabelName            = "cluster-api.cattle.io/owned"
	capiClusterOwner          = "cluster-api.cattle.io/capi-cluster-owner"
	capiClusterOwnerNamespace = "cluster-api.cattle.io/capi-cluster-owner-ns"
	v1ClusterMigrated         = "cluster-api.cattle.io/migrated"
	fleetNamespaceMigrated    = "cluster-api.cattle.io/fleet-namespace-migrated"
	fleetDisabledLabel        = "cluster-api.cattle.io/disable-fleet-auto-import"

	defaultRequeueDuration = 1 * time.Minute
	trueValue              = "true"
)

func getClusterRegistrationManifest(ctx context.Context, clusterName, namespace string, cl client.Client,
	caCert []byte, insecureSkipVerify bool,
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

	manifestData, err := downloadManifest(token.Status.ManifestURL, caCert, insecureSkipVerify)
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

func downloadManifest(url string, caCert []byte, insecureSkipVerify bool) (string, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec
	}

	// Only trust the CA certificate if it is provided
	if caCert != nil { //
		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return "", errors.New("failed to append CA certificate")
		}

		tlsConfig.RootCAs = caCertPool
	}

	client := &http.Client{Transport: &http.Transport{
		TLSClientConfig: tlsConfig,
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

// removeFleetNamespace cleans up previous namespace of the deployed agent on the downstream cluster.
func removeFleetNamespace(ctx context.Context, cl client.Client, cluster *managementv3.Cluster) (bool, error) {
	log := log.FromContext(ctx)
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: "fleet-addon-agent",
	}}

	if err := cl.Get(ctx, client.ObjectKeyFromObject(ns), ns); client.IgnoreNotFound(err) != nil {
		return true, fmt.Errorf("unable to check fleet agent namespace on downstream cluster: %w", err)
	} else if err == nil {
		if err := cl.Delete(ctx, ns); err != nil {
			return true, fmt.Errorf("unable to remove old fleet agent namespace on downstream cluster: %w", err)
		}
	}

	if err := cl.Get(ctx, client.ObjectKeyFromObject(ns), ns); apierrors.IsNotFound(err) {
		ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: "cattle-fleet-system",
		}}

		if err := cl.Get(ctx, client.ObjectKeyFromObject(ns), ns); err != nil {
			return true, fmt.Errorf("cattle-fleet-system namespace is not present yet: %w", err)
		}

		log.Info("fleet agent namespace is migrated")

		annotations := cluster.GetAnnotations()
		annotations[fleetNamespaceMigrated] = ns.Name
		cluster.SetAnnotations(annotations)

		return false, nil
	} else if err == nil {
		log.Info("fleet agent namespace is not migrated yet")
	}

	return true, nil
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

func validateImportReadiness(ctx context.Context, remoteClient client.Client, in io.Reader) (bool, error) {
	log := log.FromContext(ctx)

	jobs := &batchv1.JobList{}
	if err := remoteClient.List(ctx, jobs, client.MatchingLabels(map[string]string{"cattle.io/creator": "norman"})); err != nil {
		return false, fmt.Errorf("error looking for cleanup job: %w", err)
	}

	for _, job := range jobs.Items {
		if job.GenerateName == "cattle-cleanup-" {
			log.Info("cleanup job is being performed, waiting...", "gvk", job.GroupVersionKind(), "name", job.GetName(), "namespace", job.GetNamespace())
			return true, nil
		}
	}

	reader := yamlDecoder.NewYAMLReader(bufio.NewReaderSize(in, 4096))

	for {
		raw, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return false, err
		}

		if requeue, err := verifyRawManifest(ctx, remoteClient, raw); err != nil || requeue {
			return requeue, err
		}
	}

	return false, nil
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

func verifyRawManifest(ctx context.Context, remoteClient client.Client, bytes []byte) (bool, error) {
	items, err := utilyaml.ToUnstructured(bytes)
	if err != nil {
		return false, fmt.Errorf("error unmarshalling bytes or empty object passed: %w", err)
	}

	for _, obj := range items {
		if requeue, err := checkDeletion(ctx, remoteClient, obj.DeepCopy()); err != nil || requeue {
			return requeue, err
		}
	}

	return false, nil
}

func checkDeletion(ctx context.Context, c client.Client, obj client.Object) (bool, error) {
	log := log.FromContext(ctx)
	gvk := obj.GetObjectKind().GroupVersionKind()

	err := c.Get(ctx, client.ObjectKeyFromObject(obj), obj)
	if apierrors.IsNotFound(err) {
		log.V(4).Info("object is missing, ready to be created", "gvk", gvk, "name", obj.GetName(), "namespace", obj.GetNamespace())
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("checking object in remote cluster: %w", err)
	}

	if obj.GetDeletionTimestamp() != nil {
		log.Info("object is being deleted, waiting", "gvk", gvk, "name", obj.GetName(), "namespace", obj.GetNamespace())
		return true, nil
	}

	return false, nil
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

func getTrustedCAcert(ctx context.Context, cl client.Client, agentTLSModeFeatureEnabled bool) ([]byte, error) {
	log := log.FromContext(ctx)

	if !agentTLSModeFeatureEnabled {
		log.Info("agent-tls-mode feature is disabled, using system store")
		return nil, nil
	}

	agentTLSModeSetting := &managementv3.Setting{}

	if err := cl.Get(ctx, client.ObjectKey{
		Name: "agent-tls-mode",
	}, agentTLSModeSetting); err != nil {
		return nil, fmt.Errorf("error getting agent-tls-mode setting: %w", err)
	}

	agentTLSModeValue := agentTLSModeSetting.Value
	if len(agentTLSModeValue) == 0 {
		agentTLSModeValue = agentTLSModeSetting.Default
	}

	switch agentTLSModeValue {
	case "system-store":
		log.Info("using system store for CA certificates")
		return nil, nil
	case "strict":
		log.Info("using strict mode for CA certificates")

		caCertsSetting := &managementv3.Setting{}

		if err := cl.Get(ctx, client.ObjectKey{
			Name: "cacerts",
		}, caCertsSetting); err != nil {
			return nil, fmt.Errorf("error getting ca-certs setting: %w", err)
		}

		if caCertsSetting.Value == "" {
			return nil, errors.New("ca-certs setting value is empty")
		}

		return []byte(caCertsSetting.Value), nil
	default:
		return nil, fmt.Errorf("invalid agent-tls-mode setting value: %s", agentTLSModeValue)
	}
}
