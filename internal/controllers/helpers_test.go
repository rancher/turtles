/*
Copyright © 2025 SUSE LLC

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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("getTrustedCAcert", func() {
	var (
		ctx                 context.Context
		fakeClient          client.Client
		cacertsSetting      *managementv3.Setting
		agentTLSModeSetting *managementv3.Setting
	)

	BeforeEach(func() {
		ctx = context.TODO()
		fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		cacertsSetting = &managementv3.Setting{
			ObjectMeta: metav1.ObjectMeta{
				Name: "cacerts",
			},
			Value: "cert-data",
		}

		agentTLSModeSetting = &managementv3.Setting{
			ObjectMeta: metav1.ObjectMeta{
				Name: "agent-tls-mode",
			},
			Value: "strict",
		}
	})

	It("should return error when agent-tls-mode setting is not found", func() {
		result, err := getTrustedCAcert(ctx, fakeClient, true)
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeNil())
	})

	It("should return nil when agent-tls-mode is set to system-store", func() {
		agentTLSModeSetting.Value = "system-store"
		Expect(fakeClient.Create(ctx, agentTLSModeSetting)).To(Succeed())

		result, err := getTrustedCAcert(ctx, fakeClient, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(BeNil())
	})

	It("should return CA certs when agent-tls-mode is strict and cacerts is set", func() {
		Expect(fakeClient.Create(ctx, agentTLSModeSetting)).To(Succeed())
		Expect(fakeClient.Create(ctx, cacertsSetting)).To(Succeed())

		result, err := getTrustedCAcert(ctx, fakeClient, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("cert-data")))
	})

	It("should use default agent-tls-mode when value is empty", func() {
		agentTLSModeSetting.Value = ""
		agentTLSModeSetting.Default = "strict"
		Expect(fakeClient.Create(ctx, agentTLSModeSetting)).To(Succeed())
		Expect(fakeClient.Create(ctx, cacertsSetting)).To(Succeed())

		result, err := getTrustedCAcert(ctx, fakeClient, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal([]byte("cert-data")))
	})

	It("should return error when agent-tls-mode is strict and cacerts is empty", func() {
		cacertsSetting.Value = ""
		Expect(fakeClient.Create(ctx, agentTLSModeSetting)).To(Succeed())
		Expect(fakeClient.Create(ctx, cacertsSetting)).To(Succeed())

		result, err := getTrustedCAcert(ctx, fakeClient, true)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ca-certs setting value is empty"))
		Expect(result).To(BeNil())
	})

	It("should return error for invalid agent-tls-mode value", func() {
		agentTLSModeSetting.Value = "invalid"
		Expect(fakeClient.Create(ctx, agentTLSModeSetting)).To(Succeed())

		result, err := getTrustedCAcert(ctx, fakeClient, true)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid agent-tls-mode setting value"))
		Expect(result).To(BeNil())
	})

	It("should return error for missing agent-tls-mode value and default", func() {
		agentTLSModeSetting.Value = ""
		agentTLSModeSetting.Default = ""
		Expect(fakeClient.Create(ctx, agentTLSModeSetting)).To(Succeed())

		result, err := getTrustedCAcert(ctx, fakeClient, true)
		Expect(err).To(HaveOccurred(), "Should not make assumptions on default agent-tls-mode value")
		Expect(err.Error()).To(ContainSubstring("invalid agent-tls-mode setting value"))
		Expect(result).To(BeNil())
	})
})
