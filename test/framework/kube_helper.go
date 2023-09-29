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

package framework

import (
	"context"
	"time"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	retryableOperationInterval = 3 * time.Second
	retryableOperationTimeout  = 3 * time.Minute
)

// GetNodeAddressInput is th einput to GetNodeAddress.
type GetNodeAddressInput struct {
	Lister       framework.Lister
	NodeIndex    int
	AddressIndex int
}

// GetNodeAddress gets the address for a node based on index.
func GetNodeAddress(ctx context.Context, input GetNodeAddressInput) string {
	Expect(ctx).NotTo(BeNil(), "ctx is required for GetNodeAddress")
	Expect(input.Lister).ToNot(BeNil(), "Invalid argument. input.Lister can't be nil when calling GetNodeAddress")

	listOptions := []client.ListOption{}

	nodeList := &corev1.NodeList{}
	Eventually(func() error {
		return input.Lister.List(ctx, nodeList, listOptions...)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to list nodes")

	Expect(nodeList.Items).NotTo(HaveLen(0), "Expected there to be at least 1 node")
	Expect(len(nodeList.Items) >= input.NodeIndex).To(BeTrue(), "Node index is greater than number of nodes")
	node := nodeList.Items[input.NodeIndex]

	Expect(len(node.Status.Addresses) >= input.AddressIndex).To(BeTrue(), "Address index is greater than number of node addresses")
	return node.Status.Addresses[input.AddressIndex].Address
}

// GetServicePortByNameInput is the input to GetServicePortByName.
type GetServicePortByNameInput struct {
	GetLister        framework.GetLister
	ServiceName      string
	ServiceNamespace string
	PortName         string
}

type GetServicePortByNameOutput struct {
	Port     int32
	NodePort int32
}

// GetServicePortByName will get the ports for a service by port name.
func GetServicePortByName(ctx context.Context, input GetServicePortByNameInput, intervals ...interface{}) GetServicePortByNameOutput {
	Expect(ctx).NotTo(BeNil(), "ctx is required for GetServicePortByName")
	Expect(input.GetLister).ToNot(BeNil(), "Invalid argument. input.GetLister can't be nil when calling GetServicePortByName")
	Expect(input.ServiceNamespace).ToNot(BeEmpty(), "Invalid argument. input.ServiceNamespace can't be empty when calling GetServicePortByName")
	Expect(input.ServiceName).ToNot(BeEmpty(), "Invalid argument. input.ServiceName can't be empty when calling GetServicePortByName")
	Expect(input.PortName).ToNot(BeEmpty(), "Invalid argument. input.PortName can't be empty when calling GetServicePortByName")

	svc := &corev1.Service{}
	Eventually(func() error {
		return input.GetLister.Get(ctx, client.ObjectKey{Namespace: input.ServiceNamespace, Name: input.ServiceName}, svc)
	}, intervals...).Should(Succeed(), "Failed to get service")

	var svcPort corev1.ServicePort
	for _, port := range svc.Spec.Ports {
		if port.Name == input.PortName {
			svcPort = *port.DeepCopy()
			break
		}
	}
	Expect(svcPort).ToNot(BeNil(), "Failed to find the port")

	return GetServicePortByNameOutput{
		Port:     svcPort.Port,
		NodePort: svcPort.NodePort,
	}
}

// CreateSecretInput is the input to CreateSecret.
type CreateSecretInput struct {
	Creator     framework.Creator
	Name        string
	Namespace   string
	Type        corev1.SecretType
	Data        map[string]string
	Labels      map[string]string
	Annotations map[string]string
}

// CreateSecret will create a new Kubernetes secret.
func CreateSecret(ctx context.Context, input CreateSecretInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for CreateSecret")
	Expect(input.Creator).ToNot(BeNil(), "Invalid argument. input.Creator can't be nil when calling CreateSecret")
	Expect(input.Name).ToNot(BeEmpty(), "Invalid argument. input.Name can't be empty when calling CreateSecret")

	if input.Namespace == "" {
		input.Namespace = "default"
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.Namespace,
		},
		StringData: input.Data,
		Type:       input.Type,
	}

	if len(input.Annotations) > 0 {
		secret.Annotations = input.Annotations
	}
	if len(input.Labels) > 0 {
		secret.Labels = input.Labels
	}

	Eventually(func() error {
		return input.Creator.Create(ctx, secret)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to create secret %s", klog.KObj(secret))
}

// AddLabelsToNamespaceInput is th einput to AddLabelsToNamespace.
type AddLabelsToNamespaceInput struct {
	ClusterProxy framework.ClusterProxy
	Name         string
	Labels       map[string]string
}

// AddLabelsToNamespace will add labels to a namespace.
func AddLabelsToNamespace(ctx context.Context, input AddLabelsToNamespaceInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for AddLabelsToNamespace")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.ClusterProxy can't be nil when calling AddLabelsToNamespace")
	Expect(input.Name).ToNot(BeEmpty(), "Invalid argument. input.Name can't be empty when calling AddLabelsToNamespace")

	if len(input.Labels) == 0 {
		return
	}
	Eventually(func() error {
		ns := &corev1.Namespace{}

		err := input.ClusterProxy.GetClient().Get(ctx, types.NamespacedName{Name: input.Name}, ns)
		if err != nil {
			return err
		}

		namespaceCopy := ns.DeepCopy()
		if namespaceCopy.Labels == nil {
			namespaceCopy.Labels = map[string]string{}
		}

		for name, val := range input.Labels {
			namespaceCopy.Labels[name] = val
		}

		return input.ClusterProxy.GetClient().Update(ctx, namespaceCopy)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to update namespace %s with new labels", input.Name)
}
