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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	bootstrapv1 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
)

var (
	rke2Config         *bootstrapv1.RKE2Config
	r                  *RKE2ConfigWebhook
	serviceAccountName string
	planSecretName     string
	serverUrl          string
	pem                string
	systemAgentVersion string
	token              []byte
	ns                 *corev1.Namespace
)

var _ = Describe("RKE2ConfigWebhook tests", func() {
	BeforeEach(func() {
		ctx = context.Background()
		serviceAccountName = "service-account"
		planSecretName = "rke2-system-agent"
		serverUrl = "https://example.com"
		pem = "test-pem"
		systemAgentVersion = "v1.0.0"
		token = []byte("test-token")

		var err error

		ns, err = testEnv.CreateNamespace(ctx, "capiprovider")
		Expect(err).ToNot(HaveOccurred())

		rke2Config = &bootstrapv1.RKE2Config{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				UID:       "test-uid",
				Namespace: ns.Name,
				Labels: map[string]string{
					clusterv1.ClusterNameLabel:         "rke2",
					clusterv1.MachineControlPlaneLabel: "",
				},
			},
			Spec: bootstrapv1.RKE2ConfigSpec{
				Files: []bootstrapv1.File{},
			},
		}

		r = &RKE2ConfigWebhook{
			Client:  cl,
			Tracker: new(remote.ClusterCacheTracker),
		}
	})

	AfterEach(func() {
		Expect(testEnv.Cleanup(ctx, ns)).To(Succeed())
	})

	It("Should return error when non-RKE2Config object is passed", func() {
		err := r.Default(ctx, &corev1.Pod{})
		Expect(err).To(HaveOccurred())
		Expect(apierrors.IsBadRequest(err)).To(BeTrue())
	})

	It("Should skip defaulting for non CP machines", func() {
		delete(rke2Config.Labels, clusterv1.MachineControlPlaneLabel)
		err := r.Default(ctx, rke2Config)
		Expect(err).ToNot(HaveOccurred())
	})

	It("Should create secret plan resources without error", func() {
		err := r.createSecretPlanResources(ctx, rke2Config)
		Expect(err).NotTo(HaveOccurred())

		// Check if the resources are created
		serviceAccount := &corev1.ServiceAccount{}
		err = cl.Get(ctx, types.NamespacedName{Name: planSecretName, Namespace: rke2Config.Namespace}, serviceAccount)
		Expect(err).NotTo(HaveOccurred())
	})

	It("Should create a service account with the correct properties", func() {
		serviceAccount := r.createServiceAccount(rke2Config)

		Expect(serviceAccount.ObjectMeta.Name).To(Equal(planSecretName))
		Expect(serviceAccount.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
	})

	It("Should create a service account with the correct properties", func() {
		secret := connectInfoTemplate(rke2Config)

		Expect(secret.ObjectMeta.Name).To(Equal("test-system-agent-connect-info-config"))
		Expect(secret.ObjectMeta.Labels[RKE2ConfigNameLabel]).To(Equal(rke2Config.Name))
		Expect(secret.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
	})

	It("Should return service account token when secret is present and populated", func() {
		err := r.createSecretPlanResources(ctx, rke2Config)
		Expect(err).NotTo(HaveOccurred())

		secret := connectInfoTemplate(rke2Config)
		Expect(cl.Create(ctx, secret)).ToNot(HaveOccurred())

		token, err := r.issueBootstrapToken(ctx, rke2Config)
		Expect(err).ToNot(HaveOccurred())
		Expect(token).ToNot(BeNil())
	})

	It("Should add connect-info-config.json when it's not present", func() {
		err := r.createSecretPlanResources(ctx, rke2Config)
		Expect(err).NotTo(HaveOccurred())

		err = r.createConnectInfoJson(ctx, rke2Config, "plan-secret", serverUrl, pem)
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

		err := r.createConnectInfoJson(ctx, rke2Config, "plan-secret", serverUrl, pem)
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

		Expect(rke2Config.Spec.PostRKE2Commands).To(ContainElement("sh /opt/system-agent-install.sh"))
	})

	It("Should not add post-install command when it's already present", func() {
		rke2Config.Spec.PostRKE2Commands = append(rke2Config.Spec.PostRKE2Commands, "sh /opt/system-agent-install.sh")

		r.AddPostInstallCommands(rke2Config)

		Expect(rke2Config.Spec.PostRKE2Commands).To(HaveLen(1))
	})
})
