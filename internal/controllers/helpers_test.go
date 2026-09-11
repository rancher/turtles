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
	"bytes"
	"context"
	"log"
	"time"

	"github.com/go-logr/logr"
	"github.com/go-logr/stdr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	"github.com/rancher/turtles/feature"
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
				Name: string(feature.AgentTLSMode),
			},
			Value: agentTLSModeStrict,
		}
	})

	It("should return error when agent-tls-mode setting is not found", func() {
		result, err := getTrustedCAcert(ctx, fakeClient, true)
		Expect(err).To(HaveOccurred())
		Expect(result).To(BeNil())
	})

	It("should return nil when agent-tls-mode is set to system-store", func() {
		agentTLSModeSetting.Value = agentTLSModeSystemStore
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
		agentTLSModeSetting.Default = agentTLSModeStrict
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

var _ = Describe("resolveMultipleRancherManagementClusters", func() {
	var (
		clusterList *managementv3.ClusterList
		logBuffer   bytes.Buffer
		fakeLogger  logr.Logger
	)

	BeforeEach(func() {
		stdLogger := log.New(&logBuffer, "", 0)
		fakeLogger = stdr.New(stdLogger)
	})

	It("Should return nil if list is empty", func() {
		Expect(resolveMultipleRancherManagementClusters(fakeLogger, clusterList)).Should(BeNil())
		Expect(logBuffer.String()).Should(BeEmpty())
	})

	It("Should return one Cluster if list contains one element", func() {
		expectedCluster := managementv3.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "test-cluster"}}
		clusterList = &managementv3.ClusterList{
			Items: []managementv3.Cluster{expectedCluster},
		}

		resolvedCluster := resolveMultipleRancherManagementClusters(fakeLogger, clusterList)
		Expect(logBuffer.String()).Should(BeEmpty())
		Expect(resolvedCluster).ShouldNot(BeNil())
		Expect(resolvedCluster.Name).Should(Equal(expectedCluster.Name))
	})

	It("Should return oldest Cluster if list contains multiple items", func() {
		now := time.Now()
		expectedCluster := managementv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-cluster",
				CreationTimestamp: metav1.Time{Time: now},
			},
		}

		otherCluster := managementv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "other-cluster",
				CreationTimestamp: metav1.Time{Time: now.Add(1 * time.Second)},
			},
		}

		clusterList = &managementv3.ClusterList{
			Items: []managementv3.Cluster{otherCluster, expectedCluster},
		}

		resolvedCluster := resolveMultipleRancherManagementClusters(fakeLogger, clusterList)
		Expect(logBuffer.String()).ShouldNot(BeEmpty())
		Expect(resolvedCluster).ShouldNot(BeNil())
		Expect(resolvedCluster.Name).Should(Equal(expectedCluster.Name))
	})

	It("Should print logs if list contains multiple items", func() {
		now := time.Now()
		expectedCluster := managementv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-cluster",
				CreationTimestamp: metav1.Time{Time: now},
			},
		}

		otherCluster := managementv3.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "other-cluster",
				CreationTimestamp: metav1.Time{Time: now.Add(1 * time.Second)},
			},
		}

		clusterList = &managementv3.ClusterList{
			Items: []managementv3.Cluster{otherCluster, expectedCluster},
		}

		resolveMultipleRancherManagementClusters(fakeLogger, clusterList)
		logs := logBuffer.String()
		Expect(logs).ShouldNot(BeEmpty())
		Expect(logs).Should(ContainSubstring("Defaulting to: test-cluster"))
		Expect(logs).Should(ContainSubstring("other-cluster"), "Logs should contain name of other found clusters.")
	})

})
