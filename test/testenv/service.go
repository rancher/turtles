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

package testenv

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	turtlesframework "github.com/rancher/turtles/test/framework"
)

type WaitForServiceIngressHostnameInput struct {
	BootstrapClusterProxy framework.ClusterProxy
	ServiceName           string
	ServiceNamespace      string
	IngressWaitInterval   []interface{}
}

type WaitForServiceIngressHostnameResult struct {
	Hostname string
}

func WaitForServiceIngressHostname(ctx context.Context, input WaitForServiceIngressHostnameInput, result *WaitForServiceIngressHostnameResult) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for WaitForServiceIngressHostname")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for WaitForServiceIngressHostname")
	Expect(input.ServiceName).ToNot(BeEmpty(), "ServiceName is required for WaitForServiceIngressHostname")
	Expect(input.ServiceNamespace).ToNot(BeEmpty(), "ServiceNamespace is required for WaitForServiceIngressHostname")
	Expect(input.IngressWaitInterval).ToNot(BeNil(), "IngressWaitInterval is required for WaitForServiceIngressHostname")
	Expect(result).ToNot(BeNil(), "result is required for WaitForServiceIngressHostname")

	komega.SetClient(input.BootstrapClusterProxy.GetClient())
	komega.SetContext(ctx)

	turtlesframework.Byf("Getting service %s\\%s", input.ServiceNamespace, input.ServiceNamespace)
	svc := &corev1.Service{ObjectMeta: v1.ObjectMeta{Name: input.ServiceName, Namespace: input.ServiceNamespace}}
	Eventually(
		komega.Get(svc),
		input.IngressWaitInterval...,
	).Should(Succeed(), "Failed to get service")

	By("Waiting for service to have an external ip")
	Eventually(func() error {
		if err := komega.Get(svc)(); err != nil {
			return fmt.Errorf("failedf to get service: %w", err)
		}

		if len(svc.Status.LoadBalancer.Ingress) == 0 {
			return errors.New("No load balancer ingress")
		}

		if svc.Status.LoadBalancer.Ingress[0].Hostname == "" {
			return errors.New("No service host name")
		}

		return nil
	}, input.IngressWaitInterval...).Should(Succeed(), "Failed to get service host name")

	result.Hostname = svc.Status.LoadBalancer.Ingress[0].Hostname
}
