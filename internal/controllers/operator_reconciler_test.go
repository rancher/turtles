/*
Copyright © 2023 - 2025 SUSE LLC

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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/provider"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"

	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	. "sigs.k8s.io/controller-runtime/pkg/envtest/komega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	"sigs.k8s.io/cluster-api-operator/controller"
)

const (
	CAPIVersion = "v1.11.5"
)

var _ = Describe("Provider sync", func() {
	var (
		err                 error
		ns                  *corev1.Namespace
		otherNs             *corev1.Namespace
		capiProvider        *turtlesv1.CAPIProvider
		customCAPIProvider  *turtlesv1.CAPIProvider
		unknownCAPIProvider *turtlesv1.CAPIProvider
		capiProviderAzure   *turtlesv1.CAPIProvider
		capiProviderGCP     *turtlesv1.CAPIProvider
		clusterctlconfig    *turtlesv1.ClusterctlConfig
		setting             *managementv3.Setting
	)

	BeforeEach(func() {
		SetClient(testEnv)
		SetContext(ctx)

		ns, err = testEnv.CreateNamespace(ctx, "ns")
		Expect(err).ToNot(HaveOccurred())

		otherNs, err = testEnv.CreateNamespace(ctx, "other")
		Expect(err).ToNot(HaveOccurred())

		capiProvider = &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: ns.Name,
			Finalizers: []string{
				operatorv1.ProviderFinalizer,
			},
		}, Spec: turtlesv1.CAPIProviderSpec{
			Name: "docker",
			Type: turtlesv1.Infrastructure,
		}}

		capiProviderAzure = capiProvider.DeepCopy()
		capiProviderAzure.Spec.Name = provider.AzureProvider
		capiProviderAzure.Name = provider.AzureProvider

		capiProviderGCP = capiProvider.DeepCopy()
		capiProviderGCP.Spec.Name = provider.GCPProvider
		capiProviderGCP.Name = provider.GCPProvider

		customCAPIProvider = capiProvider.DeepCopy()
		customCAPIProvider.Name = "custom-provider"
		customCAPIProvider.Spec.Name = "custom-provider"

		unknownCAPIProvider = capiProvider.DeepCopy()
		unknownCAPIProvider.Name = "unknown-provider"
		unknownCAPIProvider.Spec.Name = "unknown-provider"

		clusterctlconfig = &turtlesv1.ClusterctlConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      turtlesv1.ClusterctlConfigName,
				Namespace: ns.Name,
			},
			Spec: turtlesv1.ClusterctlConfigSpec{
				Providers: turtlesv1.ProviderList{{
					Name: "custom-provider",
					URL:  "https://github.com/org/repo/releases/v1.2.3/components.yaml",
					Type: "InfrastructureProvider",
				}},
			},
		}

		setting = &managementv3.Setting{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system-default-registry",
			},
			Value: "",
		}

		os.Setenv("POD_NAMESPACE", ns.Name)
	})

	AfterEach(func() {
		testEnv.Cleanup(ctx, ns, otherNs)
	})

	It("Should sync spec down and leave version to latest", func() {
		origin := capiProvider.DeepCopy()
		origin.Spec.EnableAutomaticUpdate = true
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(origin, setting).Build()
		r := &CAPIProviderReconciler{
			Client: fakeClient,
			GenericProviderReconciler: controller.GenericProviderReconciler{
				Provider:     origin,
				ProviderList: &turtlesv1.CAPIProviderList{},
				Client:       fakeClient,
				Config:       testEnv.GetConfig(),
			},
		}

		Eventually(func(g Gomega) {
			res, err := r.setProviderSpec(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			g.Expect(origin.Spec.Version).To(Equal(CAPIVersion))
		}).Should(Succeed())
	})

	It("Should use unknown provider to clusterctl override with unchanged 'latest' version", func() {
		origin := unknownCAPIProvider.DeepCopy()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(origin, setting).Build()
		r := &CAPIProviderReconciler{
			Client: fakeClient,
			GenericProviderReconciler: controller.GenericProviderReconciler{
				Provider:     origin,
				ProviderList: &turtlesv1.CAPIProviderList{},
				Client:       fakeClient,
			},
		}

		Eventually(func(g Gomega) {
			res, err := r.setProviderSpec(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			res, err = r.setConditions(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			g.Expect(conditions.IsUnknown(origin, string(turtlesv1.CheckLatestVersionTime))).To(BeTrue())
			g.Expect(origin.Spec.Version).To(BeEmpty())
		}).Should(Succeed())
	})

	It("Should reconcile unknown provider to clusterctl override with a specified version unchanged", func() {
		origin := unknownCAPIProvider.DeepCopy()
		origin.Spec.Version = "v1.0.0"
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(origin, setting).Build()
		r := &CAPIProviderReconciler{
			Client: fakeClient,
			GenericProviderReconciler: controller.GenericProviderReconciler{
				Provider:     origin,
				ProviderList: &turtlesv1.CAPIProviderList{},
				Client:       fakeClient,
				Config:       testEnv.GetConfig(),
			},
		}

		Eventually(func(g Gomega) {
			res, err := r.setProviderSpec(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			res, err = r.setConditions(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			g.Expect(conditions.IsUnknown(origin, string(turtlesv1.CheckLatestVersionTime))).To(BeTrue())
			g.Expect(origin.Spec.Version).To(Equal("v1.0.0"))
		}).Should(Succeed())
	})

	It("Should set custom provider version to latest according to clusterctlconfig override", func() {
		origin := customCAPIProvider.DeepCopy()
		origin.Spec.EnableAutomaticUpdate = true
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(origin, clusterctlconfig, setting).Build()
		r := &CAPIProviderReconciler{
			Client: fakeClient,
			GenericProviderReconciler: controller.GenericProviderReconciler{
				Provider:     origin,
				ProviderList: &turtlesv1.CAPIProviderList{},
				Client:       fakeClient,
				Config:       testEnv.GetConfig(),
			},
		}

		Eventually(func(g Gomega) {
			res, err := r.setProviderSpec(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			res, err = r.setConditions(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			g.Expect(conditions.IsTrue(origin, string(turtlesv1.CheckLatestVersionTime))).To(BeTrue())
			g.Expect(origin.Spec.Version).To(Equal("v1.2.3"))
		}).Should(Succeed())
	})

	It("Should not change custom provider version even if it is in the clusterctlconfig override", func() {
		origin := customCAPIProvider.DeepCopy()
		origin.Spec.Version = "v1.0.0"
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(origin, clusterctlconfig, setting).Build()
		r := &CAPIProviderReconciler{
			Client: fakeClient,
			GenericProviderReconciler: controller.GenericProviderReconciler{
				Provider: origin,
				Client:   fakeClient,
			},
		}

		Eventually(func(g Gomega) {
			res, err := r.setProviderSpec(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			res, err = r.setConditions(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			g.Expect(conditions.IsFalse(origin, string(turtlesv1.CheckLatestVersionTime))).To(BeTrue())
			g.Expect(origin.Spec.Version).To(Equal("v1.0.0"))
		}).Should(Succeed())
	})

	It("Should sync azure spec", func() {
		origin := capiProviderAzure.DeepCopy()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(origin, setting).Build()
		r := &CAPIProviderReconciler{
			Client: fakeClient,
			GenericProviderReconciler: controller.GenericProviderReconciler{
				Provider: origin,
				Client:   fakeClient,
			},
		}

		res, err := r.setProviderSpec(ctx)
		Expect(err).To(Succeed())
		Expect(res.IsZero()).To(BeTrue())

		Expect(origin.Status.Variables["EXP_AKS_RESOURCE_HEALTH"]).To(Equal("true"))
	})

	It("Should sync gcp spec", func() {
		origin := capiProviderGCP.DeepCopy()
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(origin, setting).Build()
		r := &CAPIProviderReconciler{
			Client: fakeClient,
			GenericProviderReconciler: controller.GenericProviderReconciler{
				Provider: origin,
				Client:   fakeClient,
			},
		}

		res, err := r.setProviderSpec(ctx)
		Expect(err).To(Succeed())
		Expect(res.IsZero()).To(BeTrue())

		Expect(origin.Status.Variables["EXP_CAPG_GKE"]).To(Equal("true"))
	})

	It("Should sync status up and set provisioning state", func() {
		origin := capiProvider.DeepCopy()
		origin.Spec.EnableAutomaticUpdate = true
		fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(origin, setting).Build()
		r := &CAPIProviderReconciler{
			Client: fakeClient,
			GenericProviderReconciler: controller.GenericProviderReconciler{
				Provider: origin,
				Client:   fakeClient,
			},
		}

		Eventually(func(g Gomega) {
			res, err := r.setProviderSpec(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			res, err = r.setConditions(ctx)
			g.Expect(err).To(Succeed())
			g.Expect(res.IsZero()).To(BeTrue())

			g.Expect(conditions.IsTrue(origin, string(turtlesv1.CheckLatestVersionTime))).To(BeTrue())
			g.Expect(origin.Status.Conditions).To(HaveLen(1))
			g.Expect(origin).To(HaveField("Status.Phase", Equal(turtlesv1.Provisioning)))
		}).Should(Succeed())
	})
})
