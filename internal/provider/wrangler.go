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

package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	admissionv1 "k8s.io/api/admissionregistration/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"sigs.k8s.io/cluster-api-operator/controller"
	"sigs.k8s.io/cluster-api/util/conditions"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
)

const (
	// CertificateAnnotationKey is the annotation that triggers wrangler managed certificates.
	CertificateAnnotationKey = "need-a-cert.cattle.io/secret-name"
	// CertManagerInjectAnnotationKey is the annotation that triggers cert-manager managed certificates.
	CertManagerInjectAnnotationKey = "cert-manager.io/inject-ca-from"

	// MutatingWebhookConfigurationKind is the MutatingWebhookConfiguration Kind.
	MutatingWebhookConfigurationKind = "MutatingWebhookConfiguration"
	// ValidatingWebhookConfigurationKind is the ValidatingWebhookConfiguration Kind.
	ValidatingWebhookConfigurationKind = "ValidatingWebhookConfiguration"

	// CAPIProviderLabel is the label identifying all resources applied for a provider.
	CAPIProviderLabel = "cluster.x-k8s.io/provider"
)

// ErrNoCertificateSecret is returned when the Certificate has no secretName defined.
var ErrNoCertificateSecret = errors.New("certificate has no secretName defined")

type service struct {
	Name                  string
	Namespace             string
	CertificateSecretName string
}

// WranglerPatcher is a function that converts a given manifest from cert-manager usage to wrangler.
func WranglerPatcher(objs []unstructured.Unstructured) ([]unstructured.Unstructured, error) {
	if len(objs) == 0 {
		return nil, nil
	}

	// Step 1: Fetch all Certificates
	certificates, err := getCertificates(objs)
	if err != nil {
		return nil, fmt.Errorf("parsing Certificates: %w", err)
	}

	// Step 2: Fetch all Services linked to Validating or Mutating webhooks from the manifest
	services, err := getServices(objs, certificates)
	if err != nil {
		return nil, fmt.Errorf("parsing Services: %w", err)
	}

	// Step 3: Patch all found Services with the need-a-cert annotation
	for i := range objs {
		o := &objs[i]

		for _, service := range services {
			if o.GetKind() == "Service" &&
				o.GetName() == service.Name &&
				o.GetNamespace() == service.Namespace {
				annotations := o.GetAnnotations()
				if annotations == nil {
					annotations = map[string]string{}
				}

				annotations[CertificateAnnotationKey] = service.CertificateSecretName

				o.SetAnnotations(annotations)
			}
		}
	}

	filteredObjs := []unstructured.Unstructured{}
	// Step 4: Cleanup
	for _, o := range objs {
		// Delete cert-manager inject annotation from any object
		annotations := o.GetAnnotations()
		if annotations != nil {
			delete(annotations, CertManagerInjectAnnotationKey)
			o.SetAnnotations(annotations)
		}

		// Filter Certificates and Issuers
		if o.GetKind() != "Certificate" && o.GetKind() != "Issuer" {
			filteredObjs = append(filteredObjs, o)
		}
	}

	return filteredObjs, nil
}

// getCertificates returns a map containing all parsed Certificates.
// The map key is the Certificate "namespace/name".
// The map value is the Certificate.spec.secretName.
func getCertificates(objs []unstructured.Unstructured) (map[string]string, error) {
	certificates := map[string]string{}

	for _, o := range objs {
		if o.GetKind() == "Certificate" {
			key := fmt.Sprintf("%s/%s", o.GetNamespace(), o.GetName())

			secretName, found, err := unstructured.NestedString(o.Object, "spec", "secretName")
			if err != nil {
				return nil, fmt.Errorf("parsing Certificate.spec.secretName: %w", err)
			}

			if !found {
				return nil, ErrNoCertificateSecret
			}

			certificates[key] = secretName
		}
	}

	return certificates, nil
}

// getServices returns a map containing all parsed Services linked to Validating or Mutating webhooks.
// The cert-manager.io/inject-ca-from annotation value is also matched against parsed Certificates, to extract
// the Certificate's secretName.
//
// The map key is the Service "namespace/name".
func getServices(objs []unstructured.Unstructured, certificates map[string]string) (map[string]service, error) {
	services := map[string]service{}

	for _, o := range objs {
		// Skip the object if the cert-manager inject annotation is not found.
		if o.GetAnnotations() == nil {
			continue
		}

		injectCAFromValue, found := o.GetAnnotations()[CertManagerInjectAnnotationKey]

		if !found {
			continue
		}

		certificateSecretName, found := certificates[injectCAFromValue]

		if !found {
			return nil, fmt.Errorf("could not find secret for certificate %s", injectCAFromValue)
		}

		switch o.GetKind() {
		case MutatingWebhookConfigurationKind:
			if err := addMutatingWebhookServices(o, services, certificateSecretName); err != nil {
				return nil, fmt.Errorf("evaluating MutatingWebhookConfiguration %s/%s: %w", o.GetNamespace(), o.GetName(), err)
			}
		case ValidatingWebhookConfigurationKind:
			if err := addValidatingWebhookServices(o, services, certificateSecretName); err != nil {
				return nil, fmt.Errorf("evaluating ValidatingWebhookConfiguration %s/%s: %w", o.GetNamespace(), o.GetName(), err)
			}
		}
	}

	return services, nil
}

func addMutatingWebhookServices(object unstructured.Unstructured, services map[string]service, certificateSecretName string) error {
	webhook := &admissionv1.MutatingWebhookConfiguration{}

	webhookBytes, err := object.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}

	if err := json.Unmarshal(webhookBytes, webhook); err != nil {
		return fmt.Errorf("unmarshalling: %w", err)
	}

	for _, w := range webhook.Webhooks {
		if w.ClientConfig.Service != nil {
			key := fmt.Sprintf("%s/%s", w.ClientConfig.Service.Namespace, w.ClientConfig.Service.Name)
			services[key] = service{
				Name:                  w.ClientConfig.Service.Name,
				Namespace:             w.ClientConfig.Service.Namespace,
				CertificateSecretName: certificateSecretName,
			}
		}
	}

	return nil
}

func addValidatingWebhookServices(object unstructured.Unstructured, services map[string]service, certificateSecretName string) error {
	webhook := &admissionv1.ValidatingWebhookConfiguration{}

	webhookBytes, err := object.MarshalJSON()
	if err != nil {
		return fmt.Errorf("marshalling: %w", err)
	}

	if err := json.Unmarshal(webhookBytes, webhook); err != nil {
		return fmt.Errorf("unmarshalling: %w", err)
	}

	for _, w := range webhook.Webhooks {
		if w.ClientConfig.Service != nil {
			key := fmt.Sprintf("%s/%s", w.ClientConfig.Service.Namespace, w.ClientConfig.Service.Name)
			services[key] = service{
				Name:                  w.ClientConfig.Service.Name,
				Namespace:             w.ClientConfig.Service.Namespace,
				CertificateSecretName: certificateSecretName,
			}
		}
	}

	return nil
}

// CleanupCertManagerResources will delete all Certificate and Issuer resources associated with a CAPI provider.
// Additionally, it will remove all `cert-manager.io/inject-ca-from` annotations from provider resources.
// Finally, provider pods are restarted to ensure loading of new certificates.
func CleanupCertManagerResources(ctx context.Context, cl client.Client, provider *turtlesv1.CAPIProvider) (*controller.Result, error) {
	log := log.FromContext(ctx)

	if conditions.IsTrue(provider, string(turtlesv1.CAPIProviderWranglerManagedCertificatesCondition)) {
		// Provider already converted to Wrangler. Nothing to do.
		return &controller.Result{}, nil
	}

	listOpts, err := getSelector(provider)
	if err != nil {
		return &controller.Result{}, fmt.Errorf("getting selector: %w", err)
	}

	log.Info("Cleaning Certificates deployed for provider")

	certs := schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Certificate",
	}

	certList := &unstructured.UnstructuredList{}
	certList.SetGroupVersionKind(certs)

	if err := cl.List(ctx, certList, listOpts...); err != nil && !apimeta.IsNoMatchError(err) {
		return &controller.Result{}, fmt.Errorf("listing Certificates: %w", err)
	}

	for _, cert := range certList.Items {
		log.Info("Deleting Certificate", "certificateName", cert.GetName())

		if err := cl.Delete(ctx, &cert); err != nil {
			return &controller.Result{}, fmt.Errorf("deleting Certificate %s: %w", cert.GetName(), err)
		}
	}

	log.Info("Cleaning Issuers deployed for provider")

	issuers := schema.GroupVersionKind{
		Group:   "cert-manager.io",
		Version: "v1",
		Kind:    "Issuer",
	}

	issuerList := &unstructured.UnstructuredList{}
	issuerList.SetGroupVersionKind(issuers)

	if err := cl.List(ctx, issuerList, listOpts...); err != nil && !apimeta.IsNoMatchError(err) {
		return &controller.Result{}, fmt.Errorf("listing Issuers: %w", err)
	}

	for _, issuer := range issuerList.Items {
		log.Info("Deleting Issuer", "issuerName", issuer.GetName())

		if err := cl.Delete(ctx, &issuer); err != nil {
			return &controller.Result{}, fmt.Errorf("deleting Issuer %s: %w", issuer.GetName(), err)
		}
	}

	log.Info("Cleaning cert-manager.io/inject-ca-from annotation from provider resources")

	// Listing/getting objects requires a GroupVersionKind to be set explicitly.
	// This can be removed if it's possible to list *any* object in some way.
	cleanupKinds := []schema.GroupVersionKind{
		{
			Group:   "apiextensions.k8s.io",
			Version: "v1",
			Kind:    "CustomResourceDefinition",
		},
		{
			Group:   "admissionregistration.k8s.io",
			Version: "v1",
			Kind:    "MutatingWebhookConfiguration",
		},
		{
			Group:   "admissionregistration.k8s.io",
			Version: "v1",
			Kind:    "ValidatingWebhookConfiguration",
		},
	}

	for _, cleanupKind := range cleanupKinds {
		resourcesList := &unstructured.UnstructuredList{}
		resourcesList.SetGroupVersionKind(cleanupKind)

		if err := cl.List(ctx, resourcesList, listOpts...); err != nil {
			return &controller.Result{}, fmt.Errorf("listing resources: %w", err)
		}

		for i := range resourcesList.Items {
			resource := &resourcesList.Items[i]
			annotations := resource.GetAnnotations()

			if annotations == nil {
				continue
			}

			_, found := annotations[CertManagerInjectAnnotationKey]

			if !found {
				continue
			}

			delete(annotations, CertManagerInjectAnnotationKey)

			resource.SetAnnotations(annotations)

			if err := cl.Update(ctx, resource); err != nil {
				// In case of conflicts, wait a few seconds for other controllers to update resources. (ex. ValidatingWebhookConfigurations)
				return &controller.Result{RequeueAfter: 10 * time.Second},
					fmt.Errorf("updating %s %s: %w", resource.GetObjectKind().GroupVersionKind(), resource.GetName(), err)
			}

			log.Info("Removed cert-manager annotation from resource", "resourceKind",
				resource.GetObjectKind().GroupVersionKind(), "resourceName", resource.GetName())
		}
	}

	if err := providerDeploymentRestart(ctx, cl, provider); err != nil {
		return &controller.Result{}, fmt.Errorf("restarting Deployment: %w", err)
	}

	conditions.Set(provider, metav1.Condition{
		Type:               string(turtlesv1.CAPIProviderWranglerManagedCertificatesCondition),
		Status:             metav1.ConditionTrue,
		Reason:             "CertificatesManaged",
		Message:            "Certificates are now managed by wrangler",
		LastTransitionTime: metav1.Now(),
	})

	return &controller.Result{}, nil
}

// CleanupWranglerResources removes the `need-a-cert.cattle.io/secret-name` from all provider Services.
// Finally, provider pods are restarted to ensure loading of new certificates.
func CleanupWranglerResources(ctx context.Context, cl client.Client, provider *turtlesv1.CAPIProvider) (*controller.Result, error) {
	if conditions.IsFalse(provider, string(turtlesv1.CAPIProviderWranglerManagedCertificatesCondition)) {
		// Provider not converted to Wrangler. Nothing to do.
		return &controller.Result{}, nil
	}

	log := log.FromContext(ctx)
	log.Info("Cleaning wrangler annotation from Services")

	listOpts, err := getSelector(provider)
	if err != nil {
		return &controller.Result{}, fmt.Errorf("getting selector: %w", err)
	}

	servicesList := &corev1.ServiceList{}
	if err := cl.List(ctx, servicesList, listOpts...); err != nil {
		return &controller.Result{}, fmt.Errorf("listing Services: %w", err)
	}

	for _, service := range servicesList.Items {
		annotations := service.GetAnnotations()
		if annotations == nil {
			continue
		}

		_, found := annotations[CertificateAnnotationKey]
		if !found {
			continue
		}

		delete(annotations, CertificateAnnotationKey)
		service.SetAnnotations(annotations)

		if err := cl.Update(ctx, &service); err != nil {
			return &controller.Result{}, fmt.Errorf("updating Service %s/%s: %w", service.GetNamespace(), service.GetName(), err)
		}

		log.Info("Removed wrangler annotation from Service", "serviceName", service.GetName())
	}

	if err := providerDeploymentRestart(ctx, cl, provider); err != nil {
		return &controller.Result{}, fmt.Errorf("restarting Deployment: %w", err)
	}

	conditions.Delete(provider, string(turtlesv1.CAPIProviderWranglerManagedCertificatesCondition))

	return &controller.Result{}, nil
}

// providerDeploymentRestart will force a provider re-rollout by adding an annotation
// to the provider deployment spec template. This will trigger the creation of a new provider controller pod.
// It is used to restart a provider after removing cert-manager dependencies and resources from the manifest.
func providerDeploymentRestart(ctx context.Context, cl client.Client, provider *turtlesv1.CAPIProvider) error {
	log := log.FromContext(ctx)

	log.Info("Rolling out provider Deployments to load new certificates")

	deploymentList := &appsv1.DeploymentList{}

	listOpts, err := getSelector(provider)
	if err != nil {
		return fmt.Errorf("getting selector: %w", err)
	}

	if err := cl.List(ctx, deploymentList, listOpts...); err != nil {
		return fmt.Errorf("listing Deployments: %w", err)
	}

	for _, deployment := range deploymentList.Items {
		log.Info("Restarting Deployment", "deploymentName", deployment.Name)

		if deployment.Spec.Template.Annotations == nil {
			deployment.Spec.Template.Annotations = map[string]string{}
		}

		deployment.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] = time.Now().Format(time.RFC3339)

		if err := cl.Update(ctx, &deployment); err != nil {
			return fmt.Errorf("updating Deployment: %w", err)
		}
	}

	return nil
}

func getSelector(provider *turtlesv1.CAPIProvider) ([]client.ListOption, error) {
	var matchingLabels []string
	if provider.Spec.Name != "" {
		matchingLabels = []string{
			provider.Spec.Type.ToName() + provider.Spec.Name,
			provider.Spec.Name, // ex. "fleet"
		}
	} else { // support for CAPIProvider's name used as CAPIProvider.spec.name
		matchingLabels = []string{
			provider.Spec.Type.ToName() + provider.GetName(),
			provider.GetName(),
		}
	}

	requirement, err := labels.NewRequirement(CAPIProviderLabel, selection.In, matchingLabels)
	if err != nil {
		return nil, fmt.Errorf("creating labels requirement: %w", err)
	}

	selector := client.MatchingLabelsSelector{
		Selector: labels.NewSelector().
			Add(*requirement),
	}

	return []client.ListOption{
		client.InNamespace(provider.GetNamespace()),
		selector,
	}, nil
}
