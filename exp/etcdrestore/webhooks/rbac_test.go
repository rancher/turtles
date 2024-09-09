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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("RBAC tests", func() {

	var (
		namespace   = "default"
		role        *rbacv1.Role
		roleBinding *rbacv1.RoleBinding
	)

	BeforeEach(func() {
		role = &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-role",
				Namespace: namespace,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{clusterv1.GroupVersion.Group},
					Resources: []string{"clusters"},
					Verbs:     []string{"*"},
				},
			},
		}

		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-role-binding",
				Namespace: namespace,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "User",
					Name: "clusteradmin",
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     role.Name,
			},
		}
	})

	AfterEach(func() {
		testEnv.Cleanup(ctx, role, roleBinding)
	})

	It("should pass if user is allowed to access the cluster", func() {
		Expect(cl.Create(ctx, role)).To(Succeed())
		Expect(cl.Create(ctx, roleBinding)).To(Succeed())
		Expect(validateRBAC(admission.NewContextWithRequest(ctx, admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UserInfo: authenticationv1.UserInfo{
					Username: "clusteradmin",
				},
			},
		}), cl, "test-cluster", namespace)).To(Succeed())
	})

	It("should fail if user is not allowed to access the cluster", func() {
		role.Rules = []rbacv1.PolicyRule{
			{
				APIGroups: []string{clusterv1.GroupVersion.Group},
				Resources: []string{"clusters"},
				Verbs:     []string{"get"},
			},
		}
		Expect(cl.Create(ctx, role)).To(Succeed())
		Expect(cl.Create(ctx, roleBinding)).To(Succeed())
		Expect(validateRBAC(admission.NewContextWithRequest(ctx, admission.Request{
			AdmissionRequest: admissionv1.AdmissionRequest{
				UserInfo: authenticationv1.UserInfo{
					Username: "clusteradmin",
				},
			},
		}), cl, "test-cluster", namespace)).ToNot(Succeed())
	})
})
