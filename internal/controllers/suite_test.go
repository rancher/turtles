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
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/rancher-turtles/internal/test"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	testEnv         *envtest.Environment
	cfg             *rest.Config
	cl              client.Client
	ctx             = context.Background()
	testNamespace   = "test-namespace"
	kubeConfigBytes []byte
)

func TestController(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rancher Turtles Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	var err error
	testEnv = &envtest.Environment{}
	cfg, cl, err = test.StartEnvTest(testEnv)
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())
	Expect(cl).NotTo(BeNil())

	kubeConfig := &api.Config{
		Kind:       "Config",
		APIVersion: "v1",
		Clusters: map[string]*api.Cluster{
			"envtest": {
				Server:                   cfg.Host,
				CertificateAuthorityData: cfg.CAData,
			},
		},
		Contexts: map[string]*api.Context{
			"envtest": {
				Cluster:  "envtest",
				AuthInfo: "envtest",
			},
		},
		AuthInfos: map[string]*api.AuthInfo{
			"envtest": {
				ClientKeyData:         cfg.KeyData,
				ClientCertificateData: cfg.CertData,
			},
		},
		CurrentContext: "envtest",
	}

	kubeConfigBytes, err = clientcmd.Write(*kubeConfig)
	Expect(err).NotTo(HaveOccurred())

	Expect(cl.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: testNamespace,
			Labels: map[string]string{
				importLabelName: "true",
			},
		},
	})).To(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Expect(test.StopEnvTest(testEnv)).To(Succeed())
})
