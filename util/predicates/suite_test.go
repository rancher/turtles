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
	"fmt"
	"path"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/rancher-turtles/internal/test/helpers"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"

	turtlesv1 "github.com/rancher-sandbox/rancher-turtles/api/v1alpha1"
	managementv3 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/management/v3"
	provisioningv1 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/provisioning/v1"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	testEnv     *helpers.TestEnvironment
	cfg         *rest.Config
	cl          client.Client
	ctx         = context.Background()
	importLabel = "cluster-api.cattle.io/rancher-auto-import" // hardcode this value to avoid circular dependency
)

func TestClusterPredicates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ClusterPredicates Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	var err error

	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(operatorv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(turtlesv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(provisioningv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(managementv3.AddToScheme(scheme.Scheme))

	testEnvConfig := helpers.NewTestEnvironmentConfiguration(
		path.Join("hack", "crd", "bases"),
	)

	testEnv, err = testEnvConfig.Build()
	if err != nil {
		panic(err)
	}

	cfg = testEnv.Config
	cl = testEnv

	go func() {
		fmt.Println("Starting the manager")
		if err := testEnv.StartManager(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the envtest manager: %v", err))
		}
	}()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	Expect(cl).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	teardown()
})

func teardown() {
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop envtest: %v", err))
	}
}
