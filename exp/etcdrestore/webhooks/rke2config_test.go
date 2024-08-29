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

package webhooks

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	bootstrapv1 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	rke2Config              *bootstrapv1.RKE2Config
	r                       *RKE2ConfigWebhook
	fakeClient              client.Client
	fakeTracker             *remote.ClusterCacheTracker
	serviceAccountName      string
	serviceAccountNamespace string
	planSecretName          string
	serverUrl               string
	pem                     string
	systemAgentVersion      string
	token                   []byte
)

type mockClient struct {
	client.Client
	listFunc func(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

var _ = Describe("RKE2ConfigWebhook tests", func() {
	BeforeEach(func() {
		ctx = context.Background()
		serviceAccountName = "service-account"
		serviceAccountNamespace = "service-account-namespace"
		planSecretName = "plan-secret"
		serverUrl = "https://example.com"
		pem = "test-pem"
		systemAgentVersion = "v1.0.0"
		token = []byte("test-token")

		rke2Config = &bootstrapv1.RKE2Config{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				UID:       "test-uid",
				Namespace: "test-namespace",
			},
			Spec: bootstrapv1.RKE2ConfigSpec{
				Files: []bootstrapv1.File{},
			},
		}
		fakeClient = fake.NewClientBuilder().WithObjects(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					serviceAccountSecretLabel: planSecretName,
				},
			},
			Data: map[string][]byte{
				corev1.ServiceAccountTokenKey: []byte("test-token"),
			},
		}).Build()
		fakeTracker = new(remote.ClusterCacheTracker)

		r = &RKE2ConfigWebhook{
			Client:  fakeClient,
			Tracker: fakeTracker,
		}
	})

	It("Should return error when non-RKE2Config object is passed", func() {
		err := r.Default(ctx, &corev1.Pod{})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("Should create secret plan resources without error", func() {
		err := r.createSecretPlanResources(ctx, planSecretName, rke2Config)
		Expect(err).NotTo(HaveOccurred())

		// Check if the resources are created
		serviceAccount := &corev1.ServiceAccount{}
		err = fakeClient.Get(ctx, types.NamespacedName{Name: planSecretName, Namespace: rke2Config.Namespace}, serviceAccount)
		Expect(err).NotTo(HaveOccurred())

		secret := &corev1.Secret{}
		err = fakeClient.Get(ctx, types.NamespacedName{Name: planSecretName, Namespace: rke2Config.Namespace}, secret)
		Expect(err).NotTo(HaveOccurred())

		role := &rbacv1.Role{}
		err = fakeClient.Get(ctx, types.NamespacedName{Name: planSecretName, Namespace: rke2Config.Namespace}, role)
		Expect(err).NotTo(HaveOccurred())

		roleBinding := &rbacv1.RoleBinding{}
		err = fakeClient.Get(ctx, types.NamespacedName{Name: planSecretName, Namespace: rke2Config.Namespace}, roleBinding)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should create a service account with the correct properties", func() {
		serviceAccount := r.createServiceAccount(planSecretName, rke2Config)

		Expect(serviceAccount.ObjectMeta.Name).To(Equal(planSecretName))
		Expect(serviceAccount.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
		Expect(serviceAccount.ObjectMeta.Labels[rke2ConfigNameLabel]).To(Equal(rke2Config.Name))
		Expect(serviceAccount.ObjectMeta.Labels[planSecretNameLabel]).To(Equal(planSecretName))
	})

	It("Should create a secret with the correct properties", func() {
		secret := r.createSecret(planSecretName, rke2Config)

		Expect(secret.ObjectMeta.Name).To(Equal(planSecretName))
		Expect(secret.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
		Expect(secret.ObjectMeta.Labels[rke2ConfigNameLabel]).To(Equal(rke2Config.Name))
		Expect(string(secret.Type)).To(Equal(secretTypeMachinePlan))
	})

	It("Should create a role with the correct properties", func() {
		role := r.createRole(planSecretName, rke2Config)

		Expect(role.ObjectMeta.Name).To(Equal(planSecretName))
		Expect(role.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
		Expect(role.Rules[0].Verbs).To(Equal([]string{"watch", "get", "update", "list"}))
		Expect(role.Rules[0].APIGroups).To(Equal([]string{""}))
		Expect(role.Rules[0].Resources).To(Equal([]string{"secrets"}))
		Expect(role.Rules[0].ResourceNames).To(Equal([]string{planSecretName}))
	})

	It("Should create a role binding with the correct properties", func() {
		roleBinding := r.createRoleBinding(planSecretName, rke2Config)

		Expect(roleBinding.ObjectMeta.Name).To(Equal(planSecretName))
		Expect(roleBinding.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
		Expect(roleBinding.Subjects[0].Kind).To(Equal("ServiceAccount"))
		Expect(roleBinding.Subjects[0].Name).To(Equal(planSecretName))
		Expect(roleBinding.Subjects[0].Namespace).To(Equal(rke2Config.Namespace))
		Expect(roleBinding.RoleRef.APIGroup).To(Equal(rbacv1.GroupName))
		Expect(roleBinding.RoleRef.Kind).To(Equal("Role"))
		Expect(roleBinding.RoleRef.Name).To(Equal(planSecretName))
	})

	It("Should create a service account secret with the correct properties", func() {
		secret := r.createServiceAccountSecret(planSecretName, rke2Config)

		Expect(secret.ObjectMeta.Name).To(Equal(fmt.Sprintf("%s-token", planSecretName)))
		Expect(secret.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
		Expect(secret.ObjectMeta.Annotations["kubernetes.io/service-account.name"]).To(Equal(planSecretName))
		Expect(secret.ObjectMeta.Labels[serviceAccountSecretLabel]).To(Equal(planSecretName))
		Expect(secret.Type).To(Equal(corev1.SecretTypeServiceAccountToken))
	})

	It("Should return service account token when secret is present and populated", func() {
		token, err := r.ensureServiceAccountSecretPopulated(ctx, planSecretName)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).To(Equal([]byte("test-token")))
	})

	It("Should add connect-info-config.json when it's not present", func() {
		err := r.createConnectInfoJson(ctx, rke2Config, "plan-secret", serverUrl, pem, token)
		Expect(err).ToNot(HaveOccurred())

		Expect(rke2Config.Spec.Files).To(ContainElement(bootstrapv1.File{
			Path:        "/etc/rancher/agent/connect-info-config.json",
			Owner:       defaultFileOwner,
			Permissions: "0600",
			ContentFrom: &bootstrapv1.FileSource{
				Secret: bootstrapv1.SecretFileSource{
					Name: "test-system-agent-connect-info-config",
					Key:  "connect-info-config.json",
				},
			},
		}))
	})

	It("Should not add connect-info-config.json when it's already present", func() {
		rke2Config.Spec.Files = append(rke2Config.Spec.Files, bootstrapv1.File{
			Path: "/etc/rancher/agent/connect-info-config.json",
		})

		err := r.createConnectInfoJson(ctx, rke2Config, "plan-secret", serverUrl, pem, token)
		Expect(err).ToNot(HaveOccurred())

		Expect(rke2Config.Spec.Files).To(HaveLen(1))
	})

	It("Should add system-agent-install.sh when it's not present", func() {
		err := r.createSystemAgentInstallScript(ctx, serverUrl, systemAgentVersion, rke2Config)
		Expect(err).ToNot(HaveOccurred())

		Expect(rke2Config.Spec.Files).To(ContainElement(bootstrapv1.File{
			Path:        "/opt/system-agent-install.sh",
			Owner:       defaultFileOwner,
			Permissions: "0600",
			ContentFrom: &bootstrapv1.FileSource{
				Secret: bootstrapv1.SecretFileSource{
					Name: "test-system-agent-install-script",
					Key:  "install.sh",
				},
			},
		}))
	})

	It("Should not add system-agent-install.sh when it's already present", func() {
		rke2Config.Spec.Files = append(rke2Config.Spec.Files, bootstrapv1.File{
			Path: "/opt/system-agent-install.sh",
		})

		err := r.createSystemAgentInstallScript(ctx, serverUrl, systemAgentVersion, rke2Config)
		Expect(err).ToNot(HaveOccurred())

		Expect(rke2Config.Spec.Files).To(HaveLen(1))
	})

	It("Should add config.yaml when it's not present", func() {
		err := r.createConfigYAML(rke2Config)
		Expect(err).ToNot(HaveOccurred())

		Expect(rke2Config.Spec.Files).To(ContainElement(bootstrapv1.File{
			Path:        "/etc/rancher/agent/config.yaml",
			Owner:       defaultFileOwner,
			Permissions: "0600",
			Content: `workDirectory: /var/lib/rancher/agent/work
localPlanDirectory: /var/lib/rancher/agent/plans
interlockDirectory: /var/lib/rancher/agent/interlock
remoteEnabled: true
connectionInfoFile: /etc/rancher/agent/connect-info-config.json
preserveWorkDirectory: true`,
		}))
	})

	It("Should not add config.yaml when it's already present", func() {
		rke2Config.Spec.Files = append(rke2Config.Spec.Files, bootstrapv1.File{
			Path: "/etc/rancher/agent/config.yaml",
		})

		err := r.createConfigYAML(rke2Config)
		Expect(err).ToNot(HaveOccurred())

		Expect(rke2Config.Spec.Files).To(HaveLen(1))
	})

	It("Should add post-install command when it's not present", func() {
		r.AddPostInstallCommands(rke2Config)

		Expect(rke2Config.Spec.PostRKE2Commands).To(ContainElement("sudo sh /opt/system-agent-install.sh"))
	})

	It("Should not add post-install command when it's already present", func() {
		rke2Config.Spec.PostRKE2Commands = append(rke2Config.Spec.PostRKE2Commands, "sudo sh /opt/system-agent-install.sh")

		r.AddPostInstallCommands(rke2Config)

		Expect(rke2Config.Spec.PostRKE2Commands).To(HaveLen(1))
	})
})
