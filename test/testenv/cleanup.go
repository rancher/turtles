/*
Copyright 2023 SUSE.

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

package testenv

import (
	"context"
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/test/framework"
)

type CleanupTestClusterInput struct {
	SetupTestClusterResult
	SkipCleanup    bool
	ArtifactFolder string
}

func CleanupTestCluster(ctx context.Context, input CleanupTestClusterInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for CleanupTestCluster")
	Expect(input.SetupTestClusterResult).ToNot(BeNil(), "SetupTestClusterResult is required for CleanupTestCluster")
	Expect(input.ArtifactFolder).ToNot(BeEmpty(), "ArtifactFolder is required for CleanupTestCluster")

	By("Dumping logs from the bootstrap cluster")
	dumpBootstrapClusterLogs(ctx, input.BootstrapClusterProxy, input.ArtifactFolder)

	if input.SkipCleanup {
		return
	}

	By("Tearing down the management cluster")
	if input.BootstrapClusterProxy != nil {
		input.BootstrapClusterProxy.Dispose(ctx)
	}
	if input.BootstrapClusterProvider != nil {
		input.BootstrapClusterProvider.Dispose(ctx)
	}
}

func dumpBootstrapClusterLogs(ctx context.Context, bootstrapClusterProxy framework.ClusterProxy, artifactFolder string) {
	if bootstrapClusterProxy == nil {
		return
	}

	clusterLogCollector := bootstrapClusterProxy.GetLogCollector()
	if clusterLogCollector == nil {
		return
	}

	nodes, err := bootstrapClusterProxy.GetClientSet().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Printf("Failed to get nodes for the bootstrap cluster: %v\n", err)
		return
	}

	for i := range nodes.Items {
		nodeName := nodes.Items[i].GetName()
		err = clusterLogCollector.CollectMachineLog(
			ctx,
			bootstrapClusterProxy.GetClient(),
			&clusterv1.Machine{
				Spec:       clusterv1.MachineSpec{ClusterName: nodeName},
				ObjectMeta: metav1.ObjectMeta{Name: nodeName},
			},
			filepath.Join(artifactFolder, "clusters", bootstrapClusterProxy.GetName(), "machines", nodeName),
		)
		if err != nil {
			fmt.Printf("Failed to get logs for the bootstrap cluster node %s: %v\n", nodeName, err)
		}
	}
}
