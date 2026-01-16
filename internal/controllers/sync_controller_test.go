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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/sync"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/cluster-api-operator/controller"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func objectFromKey(key client.ObjectKey, obj client.Object) client.Object {
	obj.SetName(key.Name)
	obj.SetNamespace(key.Namespace)
	return obj
}

var _ = Describe("Reconcile CAPIProvider", Ordered, func() {
	var (
		ns *corev1.Namespace
	)

	BeforeAll(func() {
		mgr := testEnv.Manager
		r := &CAPIProviderReconciler{
			Client: mgr.GetClient(),
			GenericProviderReconciler: controller.GenericProviderReconciler{
				Provider:     &turtlesv1.CAPIProvider{},
				ProviderList: &turtlesv1.CAPIProviderList{},
				Client:       mgr.GetClient(),
				Config:       mgr.GetConfig(),
			},
		}

		builder, err := r.BuildWithManager(ctx, mgr)
		Expect(err).ToNot(HaveOccurred())

		r.GenericProviderReconciler.ReconcilePhases = []controller.PhaseFn{
			r.setProviderSpec,
			r.syncSecrets,
		}

		Expect(builder.Complete(reconcile.AsReconciler(r.Client, r))).ToNot(HaveOccurred())
	})

	BeforeEach(func() {
		var err error

		ns, err = testEnv.CreateNamespace(ctx, "capiprovider")
		Expect(err).ToNot(HaveOccurred())
		testEnv.Create(ctx, &managementv3.Setting{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system-default-registry",
			},
			Value: "",
		})
	})

	AfterEach(func() {
		Expect(testEnv.Delete(ctx, ns)).Should(Succeed())
		Expect(testEnv.Delete(ctx, &managementv3.Setting{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system-default-registry",
			},
		})).Should(Succeed())
	})

	It("Should create CAPIProvider secret", func() {
		provider := &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "docker",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Type: turtlesv1.Infrastructure,
		}}
		Expect(cl.Create(ctx, provider)).ToNot(HaveOccurred())

		dockerSecret := objectFromKey(client.ObjectKeyFromObject(provider), &corev1.Secret{})
		Eventually(Object(dockerSecret)).WithTimeout(5 * time.Second).Should(HaveField("Data", Equal(map[string][]byte{
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

		setConditions(provider)
		Expect(provider).To(HaveField("Status.Name", Equal(provider.Name)))
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

		copy := provider.DeepCopy()
		Eventually(Update(provider, func() {
			copy.Spec.Credentials = &turtlesv1.Credentials{
				RancherCloudCredential: "test-rancher-secret",
			}
			provider.Spec = copy.Spec
		})).Should(Succeed())

		Eventually(Object(doSecret)).WithTimeout(5 * time.Second).Should(HaveField("Data", Equal(map[string][]byte{
			"EXP_MACHINE_POOL":          []byte("true"),
			"CLUSTER_TOPOLOGY":          []byte("true"),
			"EXP_CLUSTER_RESOURCE_SET":  []byte("true"),
			"DIGITALOCEAN_ACCESS_TOKEN": []byte("token"),
			"DO_B64ENCODED_CREDENTIALS": []byte("dG9rZW4="),
		})))

		Eventually(func(g Gomega) {
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(provider), provider)).ToNot(HaveOccurred())
			g.Expect(conditions.IsTrue(provider, string(turtlesv1.RancherCredentialsSecretCondition)))
		}).Should(Succeed())

		resourceVersion := ""
		Eventually(func(g Gomega) {
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(doSecret), doSecret)).ToNot(HaveOccurred())
			previousVersion := resourceVersion
			resourceVersion = doSecret.GetResourceVersion()
			g.Expect(previousVersion).To(Equal(resourceVersion))
		}, time.Minute, 10*time.Second).Should(Succeed())
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
		Eventually(testEnv.GetAs(provider, doSecret)).ShouldNot(BeNil())

		copy := provider.DeepCopy()
		Eventually(Update(provider, func() {
			copy.Spec.Features = &turtlesv1.Features{MachinePool: true}
			copy.Spec.Credentials = &turtlesv1.Credentials{
				RancherCloudCredential: "some-missing",
			}
			provider.Spec = copy.Spec
		})).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(testEnv.Get(ctx, client.ObjectKeyFromObject(provider), provider)).ToNot(HaveOccurred())
			g.Expect(conditions.IsFalse(provider, string(turtlesv1.RancherCredentialsSecretCondition)))
			g.Expect(conditions.GetMessage(provider, string(turtlesv1.RancherCredentialsSecretCondition))).To(Equal("Credential keys missing: key not found: digitaloceancredentialConfig-accessToken"))
		}).Should(Succeed())
	})
})
