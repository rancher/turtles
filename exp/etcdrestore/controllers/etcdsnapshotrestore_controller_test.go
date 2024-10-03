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
	// . "github.com/onsi/gomega"
	bootstrapv1 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta1"
	// corev1 "k8s.io/api/core/v1"
)

var (
	rke2Config              *bootstrapv1.RKE2Config
	serviceAccountName      string
	serviceAccountNamespace string
	planSecretName          string
	serverUrl               string
	pem                     string
	systemAgentVersion      string
	token                   []byte
)

var _ = Describe("RKE2ConfigWebhook tests", func() {
	// It("Should create a role with the correct properties", func() {
	// 	role := r.createRole(planSecretName, rke2Config)

	// 	Expect(role.ObjectMeta.Name).To(Equal(planSecretName))
	// 	Expect(role.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
	// 	Expect(role.Rules[0].Verbs).To(Equal([]string{"watch", "get", "update", "list"}))
	// 	Expect(role.Rules[0].APIGroups).To(Equal([]string{""}))
	// 	Expect(role.Rules[0].Resources).To(Equal([]string{"secrets"}))
	// 	Expect(role.Rules[0].ResourceNames).To(Equal([]string{planSecretName}))
	// })

	// It("Should create a role binding with the correct properties", func() {
	// 	roleBinding := r.createRoleBinding(planSecretName, rke2Config)

	// 	Expect(roleBinding.ObjectMeta.Name).To(Equal(planSecretName))
	// 	Expect(roleBinding.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
	// 	Expect(roleBinding.Subjects[0].Kind).To(Equal("ServiceAccount"))
	// 	Expect(roleBinding.Subjects[0].Name).To(Equal(planSecretName))
	// 	Expect(roleBinding.Subjects[0].Namespace).To(Equal(rke2Config.Namespace))
	// 	Expect(roleBinding.RoleRef.APIGroup).To(Equal(rbacv1.GroupName))
	// 	Expect(roleBinding.RoleRef.Kind).To(Equal("Role"))
	// 	Expect(roleBinding.RoleRef.Name).To(Equal(planSecretName))
	// })

	// It("Should create a role with the correct properties", func() {
	// 	role := r.createRole(planSecretName, rke2Config)

	// 	Expect(role.ObjectMeta.Name).To(Equal(planSecretName))
	// 	Expect(role.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
	// 	Expect(role.Rules[0].Verbs).To(Equal([]string{"watch", "get", "update", "list"}))
	// 	Expect(role.Rules[0].APIGroups).To(Equal([]string{""}))
	// 	Expect(role.Rules[0].Resources).To(Equal([]string{"secrets"}))
	// 	Expect(role.Rules[0].ResourceNames).To(Equal([]string{planSecretName}))
	// })

	// It("Should create a role binding with the correct properties", func() {
	// 	roleBinding := r.createRoleBinding(planSecretName, rke2Config)

	// 	Expect(roleBinding.ObjectMeta.Name).To(Equal(planSecretName))
	// 	Expect(roleBinding.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
	// 	Expect(roleBinding.Subjects[0].Kind).To(Equal("ServiceAccount"))
	// 	Expect(roleBinding.Subjects[0].Name).To(Equal(planSecretName))
	// 	Expect(roleBinding.Subjects[0].Namespace).To(Equal(rke2Config.Namespace))
	// 	Expect(roleBinding.RoleRef.APIGroup).To(Equal(rbacv1.GroupName))
	// 	Expect(roleBinding.RoleRef.Kind).To(Equal("Role"))
	// 	Expect(roleBinding.RoleRef.Name).To(Equal(planSecretName))
	// })

	// It("Should create a service account secret with the correct properties", func() {
	// 	secret := r.createServiceAccountSecret(planSecretName, rke2Config)

	// 	Expect(secret.ObjectMeta.Name).To(Equal(fmt.Sprintf("%s-token", planSecretName)))
	// 	Expect(secret.ObjectMeta.Namespace).To(Equal(rke2Config.Namespace))
	// 	Expect(secret.ObjectMeta.Annotations["kubernetes.io/service-account.name"]).To(Equal(planSecretName))
	// 	Expect(secret.ObjectMeta.Labels[serviceAccountSecretLabel]).To(Equal(planSecretName))
	// 	Expect(secret.Type).To(Equal(corev1.SecretTypeServiceAccountToken))
	// })

	// It("Should return service account token when secret is present and populated", func() {
	// 	token, err := r.issueBootstrapToken(ctx, planSecretName)
	// 	Expect(err).ToNot(HaveOccurred())
	// 	Expect(token).To(Equal([]byte("test-token")))
	// })
})
