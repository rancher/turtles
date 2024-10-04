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

package snapshotters

import (
	"context"
	"fmt"

	k3sv1 "github.com/rancher/turtles/api/rancher/k3s/v1"
	snapshotrestorev1 "github.com/rancher/turtles/exp/etcdrestore/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type RKE2Snapshotter struct {
	client.Client
	remoteClient client.Client
	cluster      *clusterv1.Cluster
}

func NewRKE2Snapshotter(client client.Client, remoteClient client.Client, cluster *clusterv1.Cluster) *RKE2Snapshotter {
	return &RKE2Snapshotter{
		Client:       client,
		remoteClient: remoteClient,
		cluster:      cluster,
	}
}

func (s *RKE2Snapshotter) Sync(ctx context.Context) error {
	log := log.FromContext(ctx)

	etcdnapshotFileList := &k3sv1.ETCDSnapshotFileList{}

	if err := s.remoteClient.List(ctx, etcdnapshotFileList); err != nil {
		return fmt.Errorf("failed to list etcd snapshot files: %w", err)
	}

	for _, snapshotFile := range etcdnapshotFileList.Items {
		log.V(5).Info("Found etcd snapshot file", "name", snapshotFile.GetName())

		readyToUse := *snapshotFile.Status.ReadyToUse
		if !readyToUse {
			log.V(5).Info("Snapshot is not ready to use, skipping")
			continue
		}

		machineName, err := s.findMachineForSnapshot(ctx, snapshotFile.Spec.NodeName)
		if err != nil {
			return fmt.Errorf("failed to find machine to take a snapshot: %w", err)
		}

		if machineName == "" {
			log.V(5).Info("Machine not found to take a snapshot, skipping. Will try again later.")
			continue
		}

		rke2EtcdMachineSnapshotConfig := &snapshotrestorev1.RKE2EtcdMachineSnapshotConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapshotFile.Name,
				Namespace: s.cluster.Namespace,
			},
		}

		if snapshotFile.Spec.S3 != nil {
			s3EndpointCASecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      snapshotFile.Name + "-s3-endpoint-ca",
					Namespace: s.cluster.Namespace,
				},
				StringData: map[string]string{
					"ca.crt": snapshotFile.Spec.S3.EndpointCA,
				},
			}

			if err := s.Create(ctx, s3EndpointCASecret); err != nil {
				if apierrors.IsAlreadyExists(err) {
					log.V(5).Info("S3 endpoint CA secret already exists")
				} else {
					return fmt.Errorf("failed to create S3 endpoint CA secret: %w", err)
				}
			}

			rke2EtcdMachineSnapshotConfig.Spec.S3 = snapshotrestorev1.S3Config{
				Endpoint:         snapshotFile.Spec.S3.Endpoint,
				EndpointCASecret: s3EndpointCASecret.Name,
				SkipSSLVerify:    snapshotFile.Spec.S3.SkipSSLVerify,
				Bucket:           snapshotFile.Spec.S3.Bucket,
				Region:           snapshotFile.Spec.S3.Region,
				Insecure:         snapshotFile.Spec.S3.Insecure,
				Location:         snapshotFile.Spec.Location,
			}
		} else {
			rke2EtcdMachineSnapshotConfig.Spec.Local = snapshotrestorev1.LocalConfig{
				DataDir: snapshotFile.Spec.Location,
			}
		}

		if err := s.Create(ctx, rke2EtcdMachineSnapshotConfig); err != nil {
			if apierrors.IsAlreadyExists(err) {
				log.V(5).Info("RKE2EtcdMachineSnapshotConfig already exists")
			} else {
				return fmt.Errorf("failed to create RKE2EtcdMachineSnapshotConfig: %w", err)
			}
		}

		etcdMachineSnapshot := &snapshotrestorev1.ETCDMachineSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      snapshotFile.Name,
				Namespace: s.cluster.Namespace,
			},
			Spec: snapshotrestorev1.ETCDMachineSnapshotSpec{
				ClusterName: s.cluster.Name,
				MachineName: machineName,
				ConfigRef:   snapshotFile.Name,
				Location:    snapshotFile.Spec.Location,
			},
		}

		if err := s.Create(ctx, etcdMachineSnapshot); err != nil {
			if apierrors.IsAlreadyExists(err) {
				log.V(5).Info("EtcdMachineSnapshot already exists")
			} else {
				return fmt.Errorf("failed to create EtcdMachineSnapshot: %w", err)
			}
		}
	}

	return nil
}

func (s *RKE2Snapshotter) findMachineForSnapshot(ctx context.Context, nodeName string) (string, error) {
	machineList := &clusterv1.MachineList{}
	if err := s.List(ctx, machineList, client.InNamespace(s.cluster.Namespace)); err != nil {
		return "", fmt.Errorf("failed to list machines: %w", err)
	}

	for _, machine := range machineList.Items {
		if machine.Spec.ClusterName == s.cluster.Name {
			if machine.Status.NodeRef != nil && machine.Status.NodeRef.Name == nodeName {
				return machine.Name, nil
			}
		}
	}

	return "", nil
}
