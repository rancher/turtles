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

package framework

import (
	"context"
	"time"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	retryableOperationInterval = 3 * time.Second
	retryableOperationTimeout  = 99 * time.Minute
)

// GetNodeAddressInput represents the input parameters for retrieving a specific node's address.
type GetNodeAddressInput struct {
	// Lister is an interface used for listing resources.
	Lister framework.Lister

	// NodeIndex is the index of the node to retrieve the address from.
	NodeIndex int

	// AddressIndex is the index of the address to retrieve from the node.
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

// GetServicePortByNameInput represents the input parameters for retrieving a service port by name.
type GetServicePortByNameInput struct {
	// GetLister is the function used to retrieve a lister.
	GetLister framework.GetLister

	// ServiceName is the name of the service.
	ServiceName string

	// ServiceNamespace is the namespace of the service.
	ServiceNamespace string

	// PortName is the name of the port.
	PortName string
}

// GetServicePortByNameOutput represents the output of the GetServicePortByName function.
type GetServicePortByNameOutput struct {
	// Port is the port number of the service.
	Port int32

	// NodePort is the node port number of the service.
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

// CreateSecretInput represents the input parameters for creating a secret.
type CreateSecretInput struct {
	// Creator is the framework.Creator responsible for creating the secret.
	Creator framework.Creator

	// Name is the name of the secret.
	Name string

	// Namespace is the namespace in which the secret will be created.
	Namespace string

	// Type is the type of the secret.
	Type corev1.SecretType

	// Data is a map of key-value pairs representing the secret data.
	Data map[string]string

	// Labels is a map of key-value pairs representing the labels associated with the secret.
	Labels map[string]string

	// Annotations is a map of key-value pairs representing the annotations associated with the secret.
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

// AddLabelsToNamespaceInput represents the input parameters for adding labels to a namespace.
type AddLabelsToNamespaceInput struct {
	// ClusterProxy is the cluster proxy object used for interacting with the Kubernetes cluster.
	ClusterProxy framework.ClusterProxy

	// Name is the name of the namespace to which labels will be added.
	Name string

	// Labels is a map of key-value pairs representing the labels to be added to the namespace.
	Labels map[string]string
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

// CreateDockerRegistrySecretInput represents the input parameters for creating a Docker registry secret.
type CreateDockerRegistrySecretInput struct {
	// BootstrapClusterProxy is the bootstrap cluster proxy.
	BootstrapClusterProxy framework.ClusterProxy

	// Name is the name of the secret.
	Name string `envDefault:"regcred"`

	// Namespace is the namespace where the secret will be created.
	Namespace string `envDefault:"cattle-turtles-system"`

	// DockerServer is the Docker server URL.
	DockerServer string `envDefault:"https://ghcr.io/"`

	// DockerUsername is the username for authenticating with the Docker registry.
	DockerUsername string `env:"GITHUB_USERNAME"`

	// DockerPassword is the password for authenticating with the Docker registry.
	DockerPassword string `env:"GITHUB_TOKEN"`
}

func CreateDockerRegistrySecret(ctx context.Context, input CreateDockerRegistrySecretInput) {
	Expect(Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for CreateDockerRegistrySecret")
	Expect(input.BootstrapClusterProxy).ToNot(BeNil(), "BootstrapClusterProxy is required for CreateDockerRegistrySecret")
	Expect(input.Name).ToNot(BeEmpty(), "Name is required for CreateDockerRegistrySecret")
	Expect(input.Namespace).ToNot(BeEmpty(), "Namespace is required for CreateDockerRegistrySecret")
	Expect(input.DockerUsername).ToNot(BeEmpty(), "DockerUsername is required for CreateDockerRegistrySecret")
	Expect(input.DockerPassword).ToNot(BeEmpty(), "DockerPassword is required for CreateDockerRegistrySecret")
	Expect(input.DockerServer).ToNot(BeEmpty(), "DockerServer is required for CreateDockerRegistrySecret")

	Byf("Creating namespace %s", input.Namespace)

	framework.CreateNamespace(ctx, framework.CreateNamespaceInput{
		Name:    input.Namespace,
		Creator: input.BootstrapClusterProxy.GetClient(),
	})

	Byf("Creating docker registry k8s secret (%s\\%s)", input.Namespace, input.Name)

	cmdCreateSecret := &RunCommandResult{}
	RunCommand(ctx, RunCommandInput{
		Command: "kubectl",
		Args: []string{
			"--kubeconfig",
			input.BootstrapClusterProxy.GetKubeconfigPath(),
			"--namespace",
			input.Namespace,
			"create",
			"secret",
			"docker-registry",
			input.Name,
			"--docker-server",
			input.DockerServer,
			"--docker-username",
			input.DockerUsername,
			"--docker-password",
			input.DockerPassword,
		},
	}, cmdCreateSecret)

	Expect(cmdCreateSecret.Error).NotTo(HaveOccurred(), "Failed creating docker registry k8s secret")
	Expect(cmdCreateSecret.ExitCode).To(Equal(0), "Creating secret return non-zero exit code")
}

// GetIngressHostInput represents the input parameters for retrieving the host of an Ingress.
type GetIngressHostInput struct {
	// GetLister is a function that returns a lister for accessing Kubernetes resources.
	GetLister framework.GetLister

	// IngressName is the name of the Ingress.
	IngressName string

	// IngressNamespace is the namespace of the Ingress.
	IngressNamespace string

	// IngressRuleIndex is the index of the Ingress rule.
	IngressRuleIndex int
}

// GetIngressHost gets the host from an ingress object.
func GetIngressHost(ctx context.Context, input GetIngressHostInput) string {
	Expect(ctx).NotTo(BeNil(), "ctx is required for GetNodeAddress")
	Expect(input.GetLister).ToNot(BeNil(), "Invalid argument. input.GetLister can't be nil when calling GetIngressHost")

	ingress := &networkingv1.Ingress{}
	Eventually(func() error {
		return input.GetLister.Get(ctx, client.ObjectKey{Namespace: input.IngressNamespace, Name: input.IngressName}, ingress)
	}).Should(Succeed(), "Failed to get ingress")

	Expect(ingress.Spec.Rules).NotTo(HaveLen(0), "Expected ingress to have at least 1 rule")
	Expect(len(ingress.Spec.Rules) >= input.IngressRuleIndex).To(BeTrue(), "Ingress rule index is greater than number of rules")

	rule := ingress.Spec.Rules[input.IngressRuleIndex]
	return rule.Host
}

// GetClusterctlInput represents the input parameters for retrieving the clusterctl config.
type GetClusterctlInput struct {
	// GetLister is a function that returns a lister for accessing Kubernetes resources.
	GetLister framework.GetLister

	// ConfigMapName is the name of the Ingress.
	ConfigMapName string

	// IngressNamespace is the namespace of the Ingress.
	ConfigMapNamespace string
}

// GetClusterctlConfig gets clusterctl config
func GetClusterctl(ctx context.Context, input GetClusterctlInput) string {
	Expect(ctx).NotTo(BeNil(), "ctx is required for GetClusterctl")
	Expect(input.GetLister).ToNot(BeNil(), "Invalid argument. input.GetLister can't be nil when calling GetClusterctl")

	config := &corev1.ConfigMap{}
	Eventually(func() error {
		return input.GetLister.Get(ctx, client.ObjectKey{Namespace: input.ConfigMapNamespace, Name: input.ConfigMapName}, config)
	}).Should(Succeed(), "Failed to get ConfigMap")
	Byf("Found ConfigMap %s/%s", input.ConfigMapNamespace, input.ConfigMapName)
	Byf("ConfigMap data: %v", config.Data)

	Expect(config.Data).NotTo(BeNil(), "Expected ConfigMap to have data")
	Expect(config.Data["clusterctl.yaml"]).NotTo(BeEmpty(), "Expected ConfigMap to have clusterctl.yaml data")

	return config.Data["clusterctl.yaml"]
}
