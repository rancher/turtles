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

package controllers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
)

func objectFromKey(key client.ObjectKey, obj client.Object) client.Object {
	obj.SetName(key.Name)
	obj.SetNamespace(key.Namespace)
	return obj
}

var _ = Describe("Reconcile CAPIProvider", func() {
	var (
		ns *corev1.Namespace
	)

	BeforeEach(func() {
		var err error

		ns, err = testEnv.CreateNamespace(ctx, "capiprovider")
		Expect(err).ToNot(HaveOccurred())
		_ = ns

		r := &CAPIProviderReconciler{
			Client: testEnv.GetClient(),
			Scheme: testEnv.GetScheme(),
		}

		Expect(r.SetupWithManager(ctx, testEnv.Manager, controller.Options{})).ToNot(HaveOccurred())
	})

	It("Should create infrastructure docker provider and secret", func() {
		provider := &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "docker",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Type: turtlesv1.Infrastructure,
		}}
		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())

		dockerProvider := objectFromKey(client.ObjectKeyFromObject(provider), &operatorv1.InfrastructureProvider{})
		dockerSecret := objectFromKey(client.ObjectKeyFromObject(provider), &corev1.Secret{})
		Eventually(Object(dockerProvider)).ShouldNot(BeNil())
		Eventually(Object(dockerSecret)).Should(HaveField("Data", Equal(map[string][]byte{
			"CLUSTER_TOPOLOGY":         []byte("true"),
			"EXP_CLUSTER_RESOURCE_SET": []byte("true"),
			"EXP_MACHINE_POOL":         []byte("true"),
		})))
	})

	It("Should inherit docker provider name", func() {
		provider := &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "docker",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Type: turtlesv1.Infrastructure,
		}}
		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())

		Eventually(Object(provider)).Should(
			HaveField("Status.Name", Equal(provider.Name)))
	})

	It("Should inherit docker provider name from the spec", func() {
		provider := &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Name: "docker",
			Type: turtlesv1.Infrastructure,
		}}
		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())

		Eventually(Object(provider)).Should(
			HaveField("Status.Name", Equal(provider.Spec.Name)))
	})

	It("Should update infrastructure docker provider version and secret content from CAPI Provider change", func() {
		provider := &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "docker",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Type: turtlesv1.Infrastructure,
		}}
		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())

		dockerProvider := objectFromKey(client.ObjectKeyFromObject(provider), &operatorv1.InfrastructureProvider{})
		dockerSecret := objectFromKey(client.ObjectKeyFromObject(provider), &corev1.Secret{})
		Eventually(Object(dockerProvider)).ShouldNot(BeNil())
		Eventually(Object(dockerSecret)).ShouldNot(BeNil())

		Eventually(Update(provider, func() {
			provider.Spec.Version = "v1.2.3"
			provider.Spec.Variables = map[string]string{
				"other": "var",
			}
		})).Should(Succeed())

		Eventually(Object(dockerProvider)).Should(HaveField("Spec.Version", Equal("v1.2.3")))
		Eventually(Object(dockerSecret)).Should(HaveField("Data", Equal(map[string][]byte{
			"other":                    []byte("var"),
			"CLUSTER_TOPOLOGY":         []byte("true"),
			"EXP_CLUSTER_RESOURCE_SET": []byte("true"),
			"EXP_MACHINE_POOL":         []byte("true"),
		})))
	})

	It("Should update infrastructure digitalocean provider features and convert rancher credentials secret on CAPI Provider change", func() {
		provider := &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "digitalocean",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Type: turtlesv1.Infrastructure,
		}}
		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())

		doSecret := objectFromKey(client.ObjectKeyFromObject(provider), &corev1.Secret{})
		Eventually(testEnv.GetAs(provider, &operatorv1.InfrastructureProvider{})).ShouldNot(BeNil())
		Eventually(testEnv.GetAs(provider, doSecret)).ShouldNot(BeNil())

		Expect(cl.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: sync.RancherCredentialsNamespace,
			},
		})).To(Succeed())

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:         "cc",
				GenerateName: "cc-",
				Namespace:    sync.RancherCredentialsNamespace,
				Annotations: map[string]string{
					sync.NameAnnotation:       "test-rancher-secret",
					sync.DriverNameAnnotation: "digitalocean",
				},
			},
			StringData: map[string]string{
				"digitaloceancredentialConfig-accessToken": "token",
			},
		}
		Expect(cl.Create(ctx, secret)).To(Succeed())

		Eventually(Update(provider, func() {
			provider.Spec.Features = &turtlesv1.Features{MachinePool: true}
			provider.Spec.Credentials = &turtlesv1.Credentials{
				RancherCloudCredential: "test-rancher-secret",
			}
		})).Should(Succeed())

		Eventually(Object(doSecret)).Should(HaveField("Data", Equal(map[string][]byte{
			"EXP_MACHINE_POOL":          []byte("true"),
			"CLUSTER_TOPOLOGY":          []byte("false"),
			"EXP_CLUSTER_RESOURCE_SET":  []byte("false"),
			"DIGITALOCEAN_ACCESS_TOKEN": []byte("token"),
			"DO_B64ENCODED_CREDENTIALS": []byte("dG9rZW4="),
		})))

		Eventually(func(g Gomega) {
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(provider), provider)).ToNot(HaveOccurred())
			g.Expect(conditions.IsTrue(provider, turtlesv1.RancherCredentialsSecretCondition))
		}).Should(Succeed())
	})

	It("Should reflect missing infrastructure digitalocean provider credential secret in the status", func() {
		provider := &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "digitalocean",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Type: turtlesv1.Infrastructure,
		}}
		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())

		doSecret := objectFromKey(client.ObjectKeyFromObject(provider), &corev1.Secret{})
		Eventually(testEnv.GetAs(provider, &operatorv1.InfrastructureProvider{})).ShouldNot(BeNil())
		Eventually(testEnv.GetAs(provider, doSecret)).ShouldNot(BeNil())

		Eventually(Update(provider, func() {
			provider.Spec.Features = &turtlesv1.Features{MachinePool: true}
			provider.Spec.Credentials = &turtlesv1.Credentials{
				RancherCloudCredential: "some-missing",
			}
		})).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(provider), provider)).ToNot(HaveOccurred())
			g.Expect(conditions.IsFalse(provider, turtlesv1.RancherCredentialsSecretCondition))
			g.Expect(conditions.GetMessage(provider, turtlesv1.RancherCredentialsSecretCondition)).To(Equal("Credential keys missing: key not found: digitaloceancredentialConfig-accessToken"))
		}).Should(Succeed())
	})
})
