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
	"context"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/sync"
	turtlesannotations "github.com/rancher/turtles/util/annotations"
)

const (
	credTestName = "cc-test123"
)

var awsStaticIdentityGVK = schema.GroupVersionKind{
	Group:   "infrastructure.cluster.x-k8s.io",
	Version: "v1beta2",
	Kind:    "AWSClusterStaticIdentity",
}

// newTestScheme builds a runtime.Scheme with all the types required by the
// credential translation tests: core Kubernetes types, turtlesv1 (CAPIProvider),
// and the unstructured AWSClusterStaticIdentity GVK.
func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = turtlesv1.AddToScheme(s)

	// Register AWSClusterStaticIdentity as an unstructured kind so the fake client can handle it.
	s.AddKnownTypeWithName(awsStaticIdentityGVK, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Version: "v1beta2",
		Kind:    "AWSClusterStaticIdentityList",
	}, &unstructured.UnstructuredList{})

	return s
}

// newTestRESTMapper returns a REST mapper that includes mappings for
// AWSClusterStaticIdentity and CAPIProvider so that RESTMapping() calls in the
// controller do not return a NoMatchError.
func newTestRESTMapper() apimeta.RESTMapper {
	mapper := apimeta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "", Version: "v1"},
		{Group: "infrastructure.cluster.x-k8s.io", Version: "v1beta2"},
		{Group: turtlesv1.GroupVersion.Group, Version: turtlesv1.GroupVersion.Version},
	})

	mapper.Add(awsStaticIdentityGVK, apimeta.RESTScopeRoot)
	mapper.Add(schema.GroupVersionKind{
		Group:   "infrastructure.cluster.x-k8s.io",
		Version: "v1beta2",
		Kind:    "AWSClusterStaticIdentityList",
	}, apimeta.RESTScopeRoot)

	mapper.Add(schema.GroupVersionKind{
		Group:   turtlesv1.GroupVersion.Group,
		Version: turtlesv1.GroupVersion.Version,
		Kind:    "CAPIProvider",
	}, apimeta.RESTScopeNamespace)

	mapper.Add(schema.GroupVersionKind{
		Group:   turtlesv1.GroupVersion.Group,
		Version: turtlesv1.GroupVersion.Version,
		Kind:    "CAPIProviderList",
	}, apimeta.RESTScopeNamespace)

	return mapper
}

// newCAPAProvider creates a valid CAPA CAPIProvider object that satisfies the
// controller's early-exit guard (name "aws" in namespace "capa-system").
func newCAPAProvider() *turtlesv1.CAPIProvider {
	return &turtlesv1.CAPIProvider{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "aws",
			Namespace: "capa-system",
		},
		Spec: turtlesv1.CAPIProviderSpec{
			Name: "aws",
			Type: turtlesv1.Infrastructure,
		},
	}
}

func newAWSCredentialSecret(name string, accessKey, secretKey string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: sync.RancherCredentialsNamespace,
			Annotations: map[string]string{
				sync.DriverNameAnnotation: awsDriverAnnotationValue,
			},
		},
		Data: map[string][]byte{
			rancherAWSAccessKeyField: []byte(accessKey),
			rancherAWSSecretKeyField: []byte(secretKey),
		},
	}
}

func newReconciler(cl client.Client) *RancherCredentialReconciler {
	return &RancherCredentialReconciler{
		Client:              cl,
		CAPASystemNamespace: "capa-system",
	}
}

func reconcileCredential(g *WithT, r *RancherCredentialReconciler, credential *corev1.Secret) ctrl.Result {
	result, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      credential.Name,
			Namespace: credential.Namespace,
		},
	})
	g.Expect(err).ToNot(HaveOccurred())

	return result
}

func TestRancherCredentialReconciler_CreatesAWSIdentity(t *testing.T) {
	g := NewWithT(t)
	scheme := newTestScheme()
	mapper := newTestRESTMapper()
	provider := newCAPAProvider()

	// Use consistent access key / secret key values throughout the test.
	credential := newAWSCredentialSecret(credTestName, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjects(provider, credential).
		Build()

	r := newReconciler(cl)
	reconcileCredential(g, r, credential)

	// Verify the credentials Secret was created in capa-system.
	credSecret := &corev1.Secret{}
	g.Expect(cl.Get(context.Background(), types.NamespacedName{
		Name:      credTestName,
		Namespace: "capa-system",
	}, credSecret)).To(Succeed())

	g.Expect(credSecret.Data).To(HaveKeyWithValue("AccessKeyID", []byte("AKIAIOSFODNN7EXAMPLE")))
	g.Expect(credSecret.Data).To(HaveKeyWithValue("SecretAccessKey", []byte("wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")))

	// Verify the AWSClusterStaticIdentity was created.
	awsIdentity := &unstructured.Unstructured{}
	awsIdentity.SetGroupVersionKind(awsStaticIdentityGVK)
	g.Expect(cl.Get(context.Background(), types.NamespacedName{Name: credTestName}, awsIdentity)).To(Succeed())

	spec := awsIdentity.Object["spec"].(map[string]interface{})
	g.Expect(spec["secretRef"]).To(Equal(credTestName))
	g.Expect(spec).To(HaveKey("allowedNamespaces"))

	// Verify the Rancher credential was annotated.
	updated := &corev1.Secret{}
	g.Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(credential), updated)).To(Succeed())
	g.Expect(updated.GetAnnotations()).To(HaveKeyWithValue(
		turtlesannotations.AWSClusterStaticIdentityRefAnnotation, credTestName))

	// Verify finalizer was added.
	g.Expect(updated.Finalizers).To(ContainElement("cloudcredential.cattle.io/aws-identity-finalizer"))
}

func TestRancherCredentialReconciler_SkipsNonAWSCredentials(t *testing.T) {
	g := NewWithT(t)
	scheme := newTestScheme()
	mapper := newTestRESTMapper()

	// A secret with no driver annotation (or non-AWS driver) – no provider needed because
	// the controller exits early after the provider/CRD check, before the driver check.
	// We include the provider so the reconciler reaches the driver annotation check.
	provider := newCAPAProvider()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "non-aws-cred",
			Namespace: sync.RancherCredentialsNamespace,
			Annotations: map[string]string{
				sync.DriverNameAnnotation: "azure",
			},
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjects(provider, secret).
		Build()

	r := newReconciler(cl)

	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      secret.Name,
			Namespace: secret.Namespace,
		},
	})
	g.Expect(err).ToNot(HaveOccurred())

	// No identity should have been created.
	awsIdentity := &unstructured.Unstructured{}
	awsIdentity.SetGroupVersionKind(awsStaticIdentityGVK)
	err = cl.Get(context.Background(), types.NamespacedName{Name: "non-aws-cred"}, awsIdentity)
	g.Expect(err).To(HaveOccurred())
}

func TestRancherCredentialReconciler_SkipsMissingCredentialKeys(t *testing.T) {
	g := NewWithT(t)
	scheme := newTestScheme()
	mapper := newTestRESTMapper()
	provider := newCAPAProvider()

	// A secret with the amazonec2 driver annotation but missing credential keys.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "incomplete-aws-cred",
			Namespace: sync.RancherCredentialsNamespace,
			Annotations: map[string]string{
				sync.DriverNameAnnotation: awsDriverAnnotationValue,
			},
		},
		Data: map[string][]byte{
			// Missing amazonec2credentialConfig-secretKey.
			"amazonec2credentialConfig-accessKey": []byte("AKIAIOSFODNN7EXAMPLE"),
		},
	}

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjects(provider, secret).
		Build()

	r := newReconciler(cl)
	reconcileCredential(g, r, secret)

	// No identity should have been created.
	awsIdentity := &unstructured.Unstructured{}
	awsIdentity.SetGroupVersionKind(awsStaticIdentityGVK)
	err := cl.Get(context.Background(), types.NamespacedName{Name: "incomplete-aws-cred"}, awsIdentity)
	g.Expect(err).To(HaveOccurred())
}

func TestRancherCredentialReconciler_DeleteCleansUpResources(t *testing.T) {
	g := NewWithT(t)
	scheme := newTestScheme()
	mapper := newTestRESTMapper()
	provider := newCAPAProvider()

	now := metav1.Now()

	// A credential with a deletion timestamp and the finalizer already set.
	credential := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cc-delete-me",
			Namespace:         sync.RancherCredentialsNamespace,
			DeletionTimestamp: &now,
			Finalizers:        []string{"cloudcredential.cattle.io/aws-identity-finalizer"},
			Annotations: map[string]string{
				sync.DriverNameAnnotation: awsDriverAnnotationValue,
			},
		},
	}

	// Pre-existing credentials secret and AWSClusterStaticIdentity.
	credSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cc-delete-me",
			Namespace: "capa-system",
		},
	}

	awsIdentity := &unstructured.Unstructured{}
	awsIdentity.SetGroupVersionKind(awsStaticIdentityGVK)
	awsIdentity.SetName("cc-delete-me")

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjects(provider, credential, credSecret, awsIdentity).
		Build()

	r := newReconciler(cl)
	reconcileCredential(g, r, credential)

	// Credentials secret should be deleted.
	deletedSecret := &corev1.Secret{}
	err := cl.Get(context.Background(), types.NamespacedName{
		Name:      "cc-delete-me",
		Namespace: "capa-system",
	}, deletedSecret)
	g.Expect(err).To(HaveOccurred())

	// AWSClusterStaticIdentity should be deleted.
	deletedIdentity := &unstructured.Unstructured{}
	deletedIdentity.SetGroupVersionKind(awsStaticIdentityGVK)
	err = cl.Get(context.Background(), types.NamespacedName{Name: "cc-delete-me"}, deletedIdentity)
	g.Expect(err).To(HaveOccurred())

	// The credential itself should also be gone (fake client removes objects once
	// the deletion timestamp is set and all finalizers are removed).
	updatedCredential := &corev1.Secret{}
	err = cl.Get(context.Background(), client.ObjectKeyFromObject(credential), updatedCredential)
	g.Expect(err).To(HaveOccurred())
}

func TestRancherCredentialReconciler_UpdatesCredentials(t *testing.T) {
	g := NewWithT(t)
	scheme := newTestScheme()
	mapper := newTestRESTMapper()
	provider := newCAPAProvider()

	credential := newAWSCredentialSecret("cc-update-test", "OLD_ACCESS_KEY", "OLD_SECRET_KEY")

	existingCredSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cc-update-test",
			Namespace: "capa-system",
		},
		Data: map[string][]byte{
			"AccessKeyID":     []byte("OLD_ACCESS_KEY"),
			"SecretAccessKey": []byte("OLD_SECRET_KEY"),
		},
	}

	existingIdentity := &unstructured.Unstructured{}
	existingIdentity.SetGroupVersionKind(awsStaticIdentityGVK)
	existingIdentity.SetName("cc-update-test")

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjects(provider, credential, existingCredSecret, existingIdentity).
		Build()

	// Simulate an update to the credential keys.
	updatedCredential := credential.DeepCopy()
	updatedCredential.Data["amazonec2credentialConfig-accessKey"] = []byte("NEW_ACCESS_KEY")
	updatedCredential.Data["amazonec2credentialConfig-secretKey"] = []byte("NEW_SECRET_KEY")
	g.Expect(cl.Update(context.Background(), updatedCredential)).To(Succeed())

	r := newReconciler(cl)
	reconcileCredential(g, r, updatedCredential)

	// Verify credentials secret was updated.
	credSecret := &corev1.Secret{}
	g.Expect(cl.Get(context.Background(), types.NamespacedName{
		Name:      "cc-update-test",
		Namespace: "capa-system",
	}, credSecret)).To(Succeed())

	g.Expect(credSecret.Data).To(HaveKeyWithValue("AccessKeyID", []byte("NEW_ACCESS_KEY")))
	g.Expect(credSecret.Data).To(HaveKeyWithValue("SecretAccessKey", []byte("NEW_SECRET_KEY")))
}

// TestRancherCredentialReconciler_NoOpWhenProviderMissing verifies that the reconciler
// exits cleanly without creating any resources when the CAPA CAPIProvider object is absent.
// This covers the new early-return path introduced in PR #2465.
func TestRancherCredentialReconciler_NoOpWhenProviderMissing(t *testing.T) {
	g := NewWithT(t)
	scheme := newTestScheme()
	mapper := newTestRESTMapper()

	// Seed only the Rancher credential – no CAPIProvider object.
	credential := newAWSCredentialSecret(credTestName, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(mapper).
		WithObjects(credential).
		Build()

	r := newReconciler(cl)
	result := reconcileCredential(g, r, credential)

	// Reconcile should return an empty result (no requeue).
	g.Expect(result).To(Equal(ctrl.Result{}))

	// No credentials secret should have been created in capa-system.
	credSecret := &corev1.Secret{}
	err := cl.Get(context.Background(), types.NamespacedName{
		Name:      credTestName,
		Namespace: "capa-system",
	}, credSecret)
	g.Expect(err).To(HaveOccurred())

	// No AWSClusterStaticIdentity should have been created.
	awsIdentity := &unstructured.Unstructured{}
	awsIdentity.SetGroupVersionKind(awsStaticIdentityGVK)
	err = cl.Get(context.Background(), types.NamespacedName{Name: credTestName}, awsIdentity)
	g.Expect(err).To(HaveOccurred())

	// The credential should not have had a finalizer added.
	updated := &corev1.Secret{}
	g.Expect(cl.Get(context.Background(), client.ObjectKeyFromObject(credential), updated)).To(Succeed())
	g.Expect(updated.Finalizers).NotTo(ContainElement(awsCredentialFinalizer))
}

// TestRancherCredentialReconciler_NoOpWhenCRDMissing verifies that the reconciler exits
// cleanly when the AWSClusterStaticIdentity CRD is not registered in the REST mapper,
// even when the CAPA CAPIProvider object is present.
func TestRancherCredentialReconciler_NoOpWhenCRDMissing(t *testing.T) {
	g := NewWithT(t)
	scheme := newTestScheme()
	provider := newCAPAProvider()

	credential := newAWSCredentialSecret(credTestName, "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")

	// Build a REST mapper that does NOT include AWSClusterStaticIdentity.
	minimalMapper := apimeta.NewDefaultRESTMapper([]schema.GroupVersion{
		{Group: "", Version: "v1"},
		{Group: turtlesv1.GroupVersion.Group, Version: turtlesv1.GroupVersion.Version},
	})
	minimalMapper.Add(schema.GroupVersionKind{
		Group:   turtlesv1.GroupVersion.Group,
		Version: turtlesv1.GroupVersion.Version,
		Kind:    "CAPIProvider",
	}, apimeta.RESTScopeNamespace)
	minimalMapper.Add(schema.GroupVersionKind{
		Group:   turtlesv1.GroupVersion.Group,
		Version: turtlesv1.GroupVersion.Version,
		Kind:    "CAPIProviderList",
	}, apimeta.RESTScopeNamespace)

	cl := fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(minimalMapper).
		WithObjects(provider, credential).
		Build()

	r := newReconciler(cl)
	result := reconcileCredential(g, r, credential)

	// Reconcile should return an empty result (no requeue).
	g.Expect(result).To(Equal(ctrl.Result{}))

	// No credentials secret should have been created in capa-system.
	credSecret := &corev1.Secret{}
	err := cl.Get(context.Background(), types.NamespacedName{
		Name:      credTestName,
		Namespace: "capa-system",
	}, credSecret)
	g.Expect(err).To(HaveOccurred())

	// No AWSClusterStaticIdentity should have been created.
	awsIdentity := &unstructured.Unstructured{}
	awsIdentity.SetGroupVersionKind(awsStaticIdentityGVK)
	err = cl.Get(context.Background(), types.NamespacedName{Name: credTestName}, awsIdentity)
	g.Expect(err).To(HaveOccurred())
}
