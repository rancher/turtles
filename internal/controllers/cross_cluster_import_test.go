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

package controllers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	provisioningv1 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/provisioning/v1"
	"github.com/rancher-sandbox/rancher-turtles/internal/rancher/setup"
	"github.com/rancher-sandbox/rancher-turtles/internal/test"
	turtlesnaming "github.com/rancher-sandbox/rancher-turtles/util/naming"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/kubeconfig"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	rancherEnv     *envtest.Environment
	otherEnv       *envtest.Environment
	rancherCfg     *rest.Config
	otherCfg       *rest.Config
	rancherCl      client.Client
	otherCl        client.Client
	clusterCtx     context.Context
	cancel         context.CancelFunc
	capiCluster    *clusterv1.Cluster
	rancherCluster *provisioningv1.Cluster
	r              *CAPIImportReconciler
)

var _ = Describe("In separate clusters", func() {

	BeforeEach(func() {
		By("bootstrapping rancher environment")
		var err error
		rancherEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "..", "hack", "crd", "bases"),
			},
			ErrorIfCRDPathMissing: true,
			Scheme:                test.RancherScheme,
		}
		rancherCfg, rancherCl, err = test.StartEnvTest(rancherEnv)
		Expect(err).NotTo(HaveOccurred())
		Expect(rancherCfg).NotTo(BeNil())
		Expect(rancherCl).NotTo(BeNil())

		By("Bootstrapping other cluster environment")
		otherEnv = &envtest.Environment{
			CRDDirectoryPaths: []string{
				filepath.Join("..", "..", "hack", "crd", "bases"),
			},
			ErrorIfCRDPathMissing: true,
			Scheme:                test.PartialScheme,
		}
		otherCfg, otherCl, err = test.StartEnvTest(otherEnv)
		Expect(err).NotTo(HaveOccurred())
		Expect(otherCfg).NotTo(BeNil())
		Expect(otherCl).NotTo(BeNil())

		Expect(otherCl.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
				Labels: map[string]string{
					importLabelName: "true",
				},
			},
		})).To(Succeed())

		Expect(rancherCl.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: testNamespace,
			},
		})).To(Succeed())

		clusterCtx, cancel = context.WithCancel(ctx)
		capiCluster = &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-cluster",
				Namespace: testNamespace,
			},
		}

		rancherCluster = &provisioningv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      turtlesnaming.Name(capiCluster.Name).ToRancherName(),
				Namespace: testNamespace,
			},
		}
	})

	AfterEach(func() {
		cancel()
		By("tearing down the 2 clsuter environment")
		Expect(test.StopEnvTest(rancherEnv)).To(Succeed())
		Expect(test.StopEnvTest(otherEnv)).To(Succeed())
	})

	It("minimal controller setup should create a Rancher cluster object from a CAPI cluster object located in a different cluster", func() {
		mgr, err := ctrl.NewManager(otherCfg, ctrl.Options{
			Scheme:                 otherEnv.Scheme,
			MetricsBindAddress:     "0",
			HealthProbeBindAddress: "0",
		})
		Expect(err).ToNot(HaveOccurred())

		config := kubeconfig.FromEnvTestConfig(rancherCfg, &clusterv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "test"},
		})
		kubeconfigFile, err := os.CreateTemp("", "kubeconfig")
		Expect(err).ToNot(HaveOccurred())
		defer os.Remove(kubeconfigFile.Name())
		Expect(os.WriteFile(kubeconfigFile.Name(), config, 0600)).To(Succeed())

		rancher, err := setup.RancherCluster(mgr, kubeconfigFile.Name())
		Expect(err).ToNot(HaveOccurred())

		reconciler := &CAPIImportReconciler{
			Client:         mgr.GetClient(),
			RancherCluster: rancher,
		}
		Expect(reconciler.SetupWithManager(ctx, mgr, controller.Options{})).To(Succeed())

		go func() {
			Expect(mgr.Start(clusterCtx)).To(Succeed())
		}()

		Expect(otherCl.Create(ctx, capiCluster)).To(Succeed())
		capiCluster.Status.ControlPlaneReady = true
		Expect(otherCl.Status().Update(ctx, capiCluster)).To(Succeed())

		args := []string{"get", fmt.Sprintf("clusters.%s", provisioningv1.GroupVersion.Group), rancherCluster.Name, "-n", rancherCluster.Namespace, "--kubeconfig", kubeconfigFile.Name()}
		Eventually(exec.Command("kubectl", args...).Run()).Should(Succeed())
	})

})
