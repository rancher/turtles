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
	turtlesannotations "github.com/rancher/turtles/util/annotations"
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

	snapshots := []snapshotrestorev1.ETCDMachineSnapshotFile{}
	s3Snapshots := []snapshotrestorev1.S3SnapshotFile{}

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

		if snapshotFile.Spec.S3 != nil {
			s3Snapshots = append(s3Snapshots, snapshotrestorev1.S3SnapshotFile{
				Name:         snapshotFile.Name,
				Location:     snapshotFile.Spec.Location,
				CreationTime: snapshotFile.Status.CreationTime,
			})
		} else {
			snapshots = append(snapshots, snapshotrestorev1.ETCDMachineSnapshotFile{
				Name:         snapshotFile.Name,
				Location:     snapshotFile.Spec.Location,
				MachineName:  machineName,
				CreationTime: snapshotFile.Status.CreationTime,
			})
		}
	}

	etcdMachineSnapshot := &snapshotrestorev1.ETCDMachineSnapshot{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ETCDMachineSnapshot",
			APIVersion: snapshotrestorev1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.cluster.Name,
			Namespace: s.cluster.Namespace,
			Annotations: map[string]string{
				turtlesannotations.EtcdAutomaticSnapshot: "true",
			},
		},
		Spec: snapshotrestorev1.ETCDMachineSnapshotSpec{
			ClusterName: s.cluster.Name,
		},
		Status: snapshotrestorev1.ETCDMachineSnapshotStatus{
			Snapshots:   snapshots,
			S3Snapshots: s3Snapshots,
		},
	}

	if err := s.Patch(ctx, etcdMachineSnapshot.DeepCopy(), client.Apply, []client.PatchOption{
		client.ForceOwnership,
		client.FieldOwner("turtles-snapshot-controller"),
	}...); err != nil {
		return fmt.Errorf("failed to create/modify EtcdMachineSnapshot: %w", err)
	}

	if err := s.Status().Patch(ctx, etcdMachineSnapshot, client.Apply, []client.SubResourcePatchOption{
		client.ForceOwnership,
		client.FieldOwner("turtles-snapshot-controller"),
	}...); err != nil {
		return fmt.Errorf("failed to create/modify EtcdMachineSnapshot status: %w", err)
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
