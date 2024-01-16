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

package sync_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	turtlesv1 "github.com/rancher-sandbox/rancher-turtles/api/v1alpha1"
	"github.com/rancher-sandbox/rancher-turtles/internal/sync"

	corev1 "k8s.io/api/core/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
)

var _ = Describe("SecretMapperSync get", func() {
	var (
		err           error
		ns            *corev1.Namespace
		globalDataNs  *corev1.Namespace
		capiProvider  *turtlesv1.CAPIProvider
		secret        *corev1.Secret
		rancherSecret *corev1.Secret
	)

	BeforeEach(func() {
		SetClient(testEnv)
		SetContext(ctx)

		ns, err = testEnv.CreateNamespace(ctx, "ns")
		Expect(err).ToNot(HaveOccurred())
		globalDataNs = &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: sync.RancherCredentialsNamespace,
			},
		}
		if apierrors.IsNotFound(testEnv.Get(ctx, client.ObjectKeyFromObject(globalDataNs), &corev1.Namespace{})) {
			Expect(testEnv.Create(ctx, globalDataNs)).ToNot(HaveOccurred())
		}

		rancherSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:         "cc",
				GenerateName: "cc-",
				Namespace:    sync.RancherCredentialsNamespace,
				Annotations: map[string]string{
					sync.NameAnnotation:       "test-rancher-secret",
					sync.DriverNameAnnotation: "docker",
				},
			},
		}

		capiProvider = &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
		}, Spec: turtlesv1.CAPIProviderSpec{
			Name: "docker",
			Type: turtlesv1.Infrastructure,
			ProviderSpec: operatorv1.ProviderSpec{
				ConfigSecret: &operatorv1.SecretReference{
					Name: "test",
				},
			},
			Credentials: &turtlesv1.Credentials{
				RancherCloudCredential: "test-rancher-secret",
			},
		}}

		Expect(testEnv.Client.Create(ctx, capiProvider)).To(Succeed())
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, ns, rancherSecret)).ToNot(HaveOccurred())
	})

	It("should get the source Rancher secret", func() {
		secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      string(capiProvider.Spec.ProviderSpec.ConfigSecret.Name),
			Namespace: capiProvider.Namespace,
		}}
		Expect(testEnv.Client.Create(ctx, secret)).To(Succeed())
		Expect(testEnv.Client.Create(ctx, rancherSecret)).ToNot(HaveOccurred())

		syncer := sync.SecretMapperSync{
			SecretSync:    sync.NewSecretSync(testEnv.Client, capiProvider).(*sync.SecretSync),
			RancherSecret: sync.SecretMapperSync{}.GetSecret(capiProvider),
		}

		err := syncer.Get(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(syncer.RancherSecret.Annotations).To(Equal(rancherSecret.Annotations))
		Expect(syncer.RancherSecret.Name).To(Equal(rancherSecret.Name))
	})

	It("should ignore similarly named secret for a different driver", func() {
		secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      string(capiProvider.Spec.ProviderSpec.ConfigSecret.Name),
			Namespace: capiProvider.Namespace,
		}}
		Expect(testEnv.Client.Create(ctx, secret)).To(Succeed())

		rancherSecret.Annotations[sync.DriverNameAnnotation] = "aws"
		Expect(testEnv.Client.Create(ctx, rancherSecret)).ToNot(HaveOccurred())

		syncer := sync.SecretMapperSync{
			SecretSync:    sync.NewSecretSync(testEnv.Client, capiProvider).(*sync.SecretSync),
			RancherSecret: sync.SecretMapperSync{}.GetSecret(capiProvider),
		}

		err := syncer.Get(context.Background())
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unable to locate rancher secret with name"))
		Expect(conditions.Get(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).ToNot(BeNil())
		Expect(conditions.IsFalse(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(BeTrue())
		Expect(conditions.GetMessage(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(Equal("Rancher Credentials secret named test-rancher-secret was not located"))
	})

	It("should handle when the source Rancher secret is not found", func() {
		secret = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
			Name:      string(capiProvider.Spec.ProviderSpec.ConfigSecret.Name),
			Namespace: capiProvider.Namespace,
		}}
		Expect(testEnv.Client.Create(ctx, secret)).To(Succeed())

		syncer := sync.SecretMapperSync{
			SecretSync:    sync.NewSecretSync(testEnv.Client, capiProvider).(*sync.SecretSync),
			RancherSecret: sync.SecretMapperSync{}.GetSecret(capiProvider),
		}

		err := syncer.Get(context.Background())

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unable to locate rancher secret with name"))
		Expect(conditions.Get(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).ToNot(BeNil())
		Expect(conditions.IsFalse(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(BeTrue())
		Expect(conditions.GetMessage(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(Equal("Rancher Credentials secret named test-rancher-secret was not located"))
	})

	It("should not error when the destination secret is not created yet", func() {
		Expect(testEnv.Client.Create(ctx, rancherSecret)).ToNot(HaveOccurred())

		syncer := sync.SecretMapperSync{
			SecretSync:    sync.NewSecretSync(testEnv.Client, capiProvider).(*sync.SecretSync),
			RancherSecret: sync.SecretMapperSync{}.GetSecret(capiProvider),
		}

		err := syncer.Get(context.Background())
		Expect(err).NotTo(HaveOccurred())
		Expect(syncer.RancherSecret.Annotations).To(Equal(rancherSecret.Annotations))
		Expect(syncer.RancherSecret.Name).To(Equal(rancherSecret.Name))
	})

	It("should point to the right initial secret", func() {
		Expect(sync.SecretMapperSync{}.GetSecret(capiProvider).ObjectMeta).To(Equal(metav1.ObjectMeta{
			Name:      capiProvider.Spec.Credentials.RancherCloudCredential,
			Namespace: sync.RancherCredentialsNamespace}))
		_, isSecret := sync.SecretMapperSync{}.Template(capiProvider).(*corev1.Secret)
		Expect(isSecret).To(BeTrue())
	})

	It("provider requirements not found", func() {
		syncer := sync.SecretMapperSync{
			SecretSync:    sync.NewSecretSync(testEnv, capiProvider).(*sync.SecretSync),
			RancherSecret: sync.SecretMapperSync{}.GetSecret(capiProvider),
		}
		err := syncer.Sync(context.Background())
		Expect(err).To(BeNil())
		Expect(syncer.SecretSync.Secret.StringData).To(HaveLen(0))
	})

	It("provider requirements azure", func() {
		capiProvider.Spec.Name = "azure"
		rancherSecret.Annotations[sync.DriverNameAnnotation] = "azure"
		Expect(testEnv.Client.Create(ctx, rancherSecret)).ToNot(HaveOccurred())
		syncer := sync.NewSecretMapperSync(testEnv, capiProvider).(*sync.SecretMapperSync)

		err := syncer.Sync(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(conditions.Get(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).ToNot(BeNil())
		Expect(conditions.IsFalse(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(BeTrue())
		Expect(conditions.GetMessage(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(
			ContainSubstring("key not found: azurecredentialConfig-subscriptionId, key not found: azurecredentialConfig-clientId, key not found: azurecredentialConfig-clientSecret, key not found: azurecredentialConfig-tenantId"))

		Expect(syncer.Secret.StringData).To(Equal(map[string]string{
			"AZURE_CLIENT_ID_B64":       "",
			"AZURE_CLIENT_SECRET_B64":   "",
			"AZURE_TENANT_ID_B64":       "",
			"AZURE_SUBSCRIPTION_ID":     "",
			"AZURE_CLIENT_ID":           "",
			"AZURE_CLIENT_SECRET":       "",
			"AZURE_TENANT_ID":           "",
			"AZURE_SUBSCRIPTION_ID_B64": "",
		}))
	})

	It("provider requirements aws", func() {
		capiProvider.Spec.Name = "aws"
		rancherSecret.Annotations[sync.DriverNameAnnotation] = "aws"
		syncer := sync.NewSecretMapperSync(testEnv, capiProvider).(*sync.SecretMapperSync)
		syncer.RancherSecret = rancherSecret

		err := syncer.Sync(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(conditions.Get(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).ToNot(BeNil())
		Expect(conditions.IsFalse(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(BeTrue())
		Expect(conditions.GetMessage(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(
			ContainSubstring("key not found: amazonec2credentialConfig-accessKey, key not found: amazonec2credentialConfig-secretKey, key not found: amazonec2credentialConfig-defaultRegion"))

		Expect(syncer.Secret.StringData).To(Equal(map[string]string{
			"AWS_REGION":                 "",
			"AWS_B64ENCODED_CREDENTIALS": "",
			"AWS_ACCESS_KEY_ID":          "",
			"AWS_SECRET_ACCESS_KEY":      "",
		}))
	})

	It("provider requirements gcp", func() {
		capiProvider.Spec.Name = "gcp"
		rancherSecret.Annotations[sync.DriverNameAnnotation] = "gcp"
		Expect(testEnv.Client.Create(ctx, rancherSecret)).ToNot(HaveOccurred())
		syncer := sync.NewSecretMapperSync(testEnv, capiProvider).(*sync.SecretMapperSync)

		err := syncer.Sync(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(conditions.Get(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).ToNot(BeNil())
		Expect(conditions.IsFalse(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(BeTrue())
		Expect(conditions.GetMessage(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(
			ContainSubstring("googlecredentialConfig-authEncodedJson"))

		Expect(syncer.Secret.StringData).To(Equal(map[string]string{
			"GCP_B64ENCODED_CREDENTIALS": "",
		}))
	})
	It("provider requirements digitalocean", func() {
		capiProvider.Spec.Name = "digitalocean"
		rancherSecret.Annotations[sync.DriverNameAnnotation] = "digitalocean"
		Expect(testEnv.Client.Create(ctx, rancherSecret)).ToNot(HaveOccurred())
		syncer := sync.NewSecretMapperSync(testEnv, capiProvider).(*sync.SecretMapperSync)

		err := syncer.Sync(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(conditions.Get(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).ToNot(BeNil())
		Expect(conditions.IsFalse(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(BeTrue())
		Expect(conditions.GetMessage(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(
			ContainSubstring("key not found: digitaloceancredentialConfig-accessToken"))

		Expect(syncer.Secret.StringData).To(Equal(map[string]string{
			"DO_B64ENCODED_CREDENTIALS": "",
			"DIGITALOCEAN_ACCESS_TOKEN": "",
		}))
	})
	It("provider requirements vsphere", func() {
		capiProvider.Spec.Name = "vsphere"
		rancherSecret.Annotations[sync.DriverNameAnnotation] = "vmwarevsphere"
		Expect(testEnv.Client.Create(ctx, rancherSecret)).ToNot(HaveOccurred())
		syncer := sync.NewSecretMapperSync(testEnv, capiProvider).(*sync.SecretMapperSync)

		err := syncer.Sync(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(conditions.Get(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).ToNot(BeNil())
		Expect(conditions.IsFalse(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(BeTrue())
		Expect(conditions.GetMessage(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(
			ContainSubstring("key not found: vmwarevsphere-password, key not found: vmwarevsphere-username"))
		Expect(syncer.Secret.StringData).To(Equal(map[string]string{
			"VSPHERE_PASSWORD": "",
			"VSPHERE_USERNAME": "",
		}))
	})

	It("prepare aws secret", func() {
		capiProvider.Spec.Name = "aws"
		rancherSecret.Annotations[sync.DriverNameAnnotation] = "aws"
		rancherSecret.StringData = map[string]string{
			"amazonec2credentialConfig-accessKey":     "test",
			"amazonec2credentialConfig-secretKey":     "test",
			"amazonec2credentialConfig-defaultRegion": "us-west-1",
		}
		Expect(testEnv.Client.Create(ctx, rancherSecret)).ToNot(HaveOccurred())

		syncer := sync.NewSecretMapperSync(testEnv, capiProvider).(*sync.SecretMapperSync)

		Expect(syncer.Get(context.Background())).ToNot(HaveOccurred())
		Expect(syncer.RancherSecret.Data["amazonec2credentialConfig-defaultRegion"]).ToNot(BeEmpty())
		Expect(syncer.Sync(context.Background())).ToNot(HaveOccurred())
		Expect(syncer.Secret.StringData).To(Equal(map[string]string{
			"AWS_REGION":                 "us-west-1",
			"AWS_B64ENCODED_CREDENTIALS": "W2RlZmF1bHRdCmF3c19hY2Nlc3Nfa2V5X2lkID0gdGVzdAphd3Nfc2VjcmV0X2FjY2Vzc19rZXkgPSB0ZXN0CnJlZ2lvbiA9IHVzLXdlc3QtMQ==",
			"AWS_ACCESS_KEY_ID":          "test",
			"AWS_SECRET_ACCESS_KEY":      "test",
		}))

		Expect(conditions.Get(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).ToNot(BeNil())
		Expect(conditions.IsTrue(syncer.Source, turtlesv1.RancherCredentialsSecretCondition)).To(BeTrue())
	})
})
