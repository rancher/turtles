//go:build e2e
// +build e2e

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

package e2e

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"path/filepath"
	"text/template"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/test/framework/clusterctl"
	"sigs.k8s.io/cluster-api/util"

	turtlesframework "github.com/rancher-sandbox/rancher-turtles/test/framework"
)

var (
	ctx = context.Background()

	//go:embed data/capi-operator/capi-providers-secret.yaml
	capiProvidersSecret []byte

	//go:embed data/capi-operator/capi-providers.yaml
	capiProviders []byte

	//go:embed data/capi-operator/full-variables.yaml
	fullProvidersSecret []byte

	//go:embed data/capi-operator/full-providers.yaml
	fullProviders []byte

	//go:embed data/rancher/ingress.yaml
	ingressConfig []byte

	//go:embed data/rancher/rancher-service-patch.yaml
	rancherServicePatch []byte

	//go:embed data/rancher/ingress-class-patch.yaml
	ingressClassPatch []byte

	//go:embed data/rancher/rancher-setting-patch.yaml
	rancherSettingPatch []byte

	//go:embed data/rancher/nginx-ingress.yaml
	nginxIngress []byte
)

const (
	operatorNamespace       = "capi-operator-system"
	rancherTurtlesNamespace = "rancher-turtles-system"
	rancherNamespace        = "cattle-system"
	capiClusterName         = "cluster1"
	capiClusterNamespace    = "default"
	nginxIngressNamespace   = "ingress-nginx"
)

func setupSpecNamespace(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string) (*corev1.Namespace, context.CancelFunc) {
	turtlesframework.Byf("Creating a namespace for hosting the %q test spec", specName)
	namespace, cancelWatches := framework.CreateNamespaceAndWatchEvents(ctx, framework.CreateNamespaceAndWatchEventsInput{
		Creator:   clusterProxy.GetClient(),
		ClientSet: clusterProxy.GetClientSet(),
		Name:      fmt.Sprintf("%s-%s", specName, util.RandomString(6)),
		LogFolder: filepath.Join(artifactFolder, "clusters", clusterProxy.GetName()),
	})

	return namespace, cancelWatches
}

func createRepoName(specName string) string {
	return fmt.Sprintf("repo-%s-%s", specName, util.RandomString(6))
}

func dumpSpecResourcesAndCleanup(ctx context.Context, specName string, clusterProxy framework.ClusterProxy, artifactFolder string, namespace *corev1.Namespace, cancelWatches context.CancelFunc, capiCluster *types.NamespacedName, intervalsGetter func(spec, key string) []interface{}, skipCleanup bool) {
	turtlesframework.Byf("Dumping logs from the %q workload cluster", capiCluster.Name)

	// Dump all the logs from the workload cluster before deleting them.
	clusterProxy.CollectWorkloadClusterLogs(ctx, capiCluster.Namespace, capiCluster.Name, filepath.Join(artifactFolder, "clusters", capiCluster.Name))

	turtlesframework.Byf("Dumping all the Cluster API resources in the %q namespace", namespace.Name)

	// Dump all Cluster API related resources to artifacts before deleting them.
	framework.DumpAllResources(ctx, framework.DumpAllResourcesInput{
		Lister:    clusterProxy.GetClient(),
		Namespace: namespace.Name,
		LogPath:   filepath.Join(artifactFolder, "clusters", clusterProxy.GetName(), "resources"),
	})

	if !skipCleanup {
		turtlesframework.Byf("Deleting cluster %s", capiCluster)
		// While https://github.com/kubernetes-sigs/cluster-api/issues/2955 is addressed in future iterations, there is a chance
		// that cluster variable is not set even if the cluster exists, so we are calling DeleteAllClustersAndWait
		// instead of DeleteClusterAndWait
		framework.DeleteAllClustersAndWait(ctx, framework.DeleteAllClustersAndWaitInput{
			Client:    clusterProxy.GetClient(),
			Namespace: namespace.Name,
		}, intervalsGetter(specName, "wait-delete-cluster")...)

		turtlesframework.Byf("Deleting namespace used for hosting the %q test spec", specName)
		framework.DeleteNamespace(ctx, framework.DeleteNamespaceInput{
			Deleter: clusterProxy.GetClient(),
			Name:    namespace.Name,
		})
	}
	cancelWatches()
}

func getFullProviderVariables(config *clusterctl.E2EConfig, varsTemplate string) []byte {
	capaCreds := config.GetVariable(capaEncodedCredentials)
	Expect(capaCreds).ToNot(BeEmpty(), "Invalid input. You must supply encoded CAPA credentials")

	providerVars := struct {
		AWSEncodedCredentials string
	}{
		AWSEncodedCredentials: capaCreds,
	}

	t := template.New("providers-variables")
	t, err := t.Parse(varsTemplate)
	Expect(err).ShouldNot(HaveOccurred(), "Failed to pass full infra variables")

	var renderedTemplate bytes.Buffer
	err = t.Execute(&renderedTemplate, providerVars)
	Expect(err).NotTo(HaveOccurred(), "Failed to execute template")

	return renderedTemplate.Bytes()
}
