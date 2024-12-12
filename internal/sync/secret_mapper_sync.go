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

package sync

import (
	"bytes"
	"cmp"
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"text/template"

	_ "embed"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
)

const (
	// RancherCredentialsNamespace is the default namespace for
	// cloud provider credentials.
	RancherCredentialsNamespace = "cattle-global-data"

	// NameAnnotation is the annotation key for the cloud credential secret name.
	NameAnnotation = "field.cattle.io/name"

	// DriverNameAnnotation is the annotation key for the cloud provider driver name.
	DriverNameAnnotation = "provisioning.cattle.io/driver"
)

var (
	//go:embed templates/aws.ini
	awsDataTemplate string

	knownProviderRequirements = map[string][]Mapping{
		"aws": {
			{to: "AWS_ACCESS_KEY_ID", from: Raw{source: "amazonec2credentialConfig-accessKey"}},
			{to: "AWS_SECRET_ACCESS_KEY", from: Raw{source: "amazonec2credentialConfig-secretKey"}},
			{to: "AWS_REGION", from: Raw{source: "amazonec2credentialConfig-defaultRegion"}},
			{to: "AWS_B64ENCODED_CREDENTIALS", from: Template{
				template: awsDataTemplate,
				sources: []string{
					"amazonec2credentialConfig-accessKey",
					"amazonec2credentialConfig-secretKey",
					"amazonec2credentialConfig-defaultRegion",
				},
			}},
		},
		"azure": {
			{to: "AZURE_SUBSCRIPTION_ID", from: Raw{source: "azurecredentialConfig-subscriptionId"}},
			{to: "AZURE_CLIENT_ID", from: Raw{source: "azurecredentialConfig-clientId"}},
			{to: "AZURE_CLIENT_SECRET", from: Raw{source: "azurecredentialConfig-clientSecret"}},
			{to: "AZURE_TENANT_ID", from: Raw{source: "azurecredentialConfig-tenantId"}},
			{to: "AZURE_SUBSCRIPTION_ID_B64", from: B64{source: "azurecredentialConfig-subscriptionId"}},
			{to: "AZURE_CLIENT_ID_B64", from: B64{source: "azurecredentialConfig-clientId"}},
			{to: "AZURE_CLIENT_SECRET_B64", from: B64{source: "azurecredentialConfig-clientSecret"}},
			{to: "AZURE_TENANT_ID_B64", from: B64{source: "azurecredentialConfig-tenantId"}},
		},
		"vsphere": {
			{to: "VSPHERE_PASSWORD", from: Raw{source: "vmwarevsphere-password"}},
			{to: "VSPHERE_USERNAME", from: Raw{source: "vmwarevsphere-username"}},
		},
		"gcp": {
			{to: "GCP_B64ENCODED_CREDENTIALS", from: B64{source: "googlecredentialConfig-authEncodedJson"}},
		},
		"digitalocean": {
			{to: "DIGITALOCEAN_ACCESS_TOKEN", from: Raw{source: "digitaloceancredentialConfig-accessToken"}},
			{to: "DO_B64ENCODED_CREDENTIALS", from: B64{source: "digitaloceancredentialConfig-accessToken"}},
		},
	}

	driverMapping = map[string]string{
		"vsphere": "vmwarevsphere",
	}
)

var (
	missingKey    = "Credential keys missing: %s"
	missingSource = "Rancher Credentials secret named %s was not located"
)

type convert interface {
	validate(data map[string][]byte) error
	convert(data map[string][]byte) string
}

// Mapping defines a mapping between a source and destination secret keys.
type Mapping struct {
	from convert
	to   string
}

// Template is a structure for rendering a template as a secret value.
type Template struct {
	template string
	sources  []string
}

func (t Template) validate(data map[string][]byte) error {
	for _, key := range t.sources {
		if _, found := data[key]; !found {
			return fmt.Errorf("key not found: %s", key)
		}
	}

	return nil
}

func (t Template) convert(data map[string][]byte) string {
	var renderedTemplate bytes.Buffer

	if err := t.validate(data); err != nil {
		return ""
	}

	stringData := map[string]string{}
	for k, v := range data {
		stringData[k] = string(v)
	}

	if t, err := template.New("").Parse(t.template); err != nil {
		return err.Error()
	} else if err = t.Execute(&renderedTemplate, stringData); err != nil {
		return err.Error()
	}

	return base64.StdEncoding.EncodeToString(renderedTemplate.Bytes())
}

// Raw is a structure for storing a secret key without encoding.
type Raw struct {
	source string
}

func (r Raw) validate(data map[string][]byte) error {
	if _, found := data[r.source]; !found {
		return fmt.Errorf("key not found: %s", r.source)
	}

	return nil
}

func (r Raw) convert(data map[string][]byte) string {
	return string(data[r.source])
}

// B64 is a structure for encoding a secret key as base64.
type B64 struct {
	source string
}

func (r B64) validate(data map[string][]byte) error {
	if _, found := data[r.source]; !found {
		return fmt.Errorf("key not found: %s", r.source)
	}

	return nil
}

func (r B64) convert(data map[string][]byte) string {
	return base64.StdEncoding.EncodeToString(data[r.source])
}

// SecretMapperSync is a structure mirroring variable secret state of the Rancher secret data.
type SecretMapperSync struct {
	*SecretSync
	RancherSecret *corev1.Secret
}

// NewSecretMapperSync creates a new secret mapper object sync.
func NewSecretMapperSync(ctx context.Context, cl client.Client, capiProvider *turtlesv1.CAPIProvider) Sync {
	log := log.FromContext(ctx)

	if capiProvider.Spec.Credentials == nil ||
		cmp.Or(capiProvider.Spec.Credentials.RancherCloudCredential,
			capiProvider.Spec.Credentials.RancherCloudCredentialNamespaceName) == "" ||
		knownProviderRequirements[capiProvider.ProviderName()] == nil {
		log.V(6).Info("No rancher credentials source provided, skipping.")
		return nil
	}

	secretSync, ok := NewSecretSync(cl, capiProvider).(*SecretSync)
	if !ok {
		return nil
	}

	return &SecretMapperSync{
		SecretSync:    secretSync,
		RancherSecret: SecretMapperSync{}.GetSecret(capiProvider),
	}
}

// GetSecret returning the source secret resource template.
func (SecretMapperSync) GetSecret(capiProvider *turtlesv1.CAPIProvider) *corev1.Secret {
	splitName := cmp.Or(capiProvider.Spec.Credentials.RancherCloudCredentialNamespaceName, ":")
	namespaceName := strings.SplitN(splitName, ":", 2)
	namespace, name := namespaceName[0], namespaceName[1]
	meta := metav1.ObjectMeta{
		Name:      cmp.Or(name, capiProvider.Spec.Credentials.RancherCloudCredential),
		Namespace: cmp.Or(namespace, RancherCredentialsNamespace),
	}

	return &corev1.Secret{ObjectMeta: meta}
}

// Template returning the mirrored secret resource template.
func (SecretMapperSync) Template(capiProvider *turtlesv1.CAPIProvider) client.Object {
	return SecretSync{}.GetSecret(capiProvider)
}

// Get retrieves the source Rancher secret and destenation secret.
func (s *SecretMapperSync) Get(ctx context.Context) error {
	log := log.FromContext(ctx)
	secretList := &corev1.SecretList{}

	if s.Source.Spec.Credentials.RancherCloudCredentialNamespaceName != "" {
		err := s.client.Get(ctx, client.ObjectKeyFromObject(s.RancherSecret), s.RancherSecret)
		if err == nil {
			return s.SecretSync.Get(ctx)
		}

		log.Error(err, "Unable to get source rancher secret by reference, looking for: "+
			client.ObjectKeyFromObject(s.RancherSecret).String())
	} else if err := s.client.List(ctx, secretList, client.InNamespace(RancherCredentialsNamespace)); err != nil {
		log.Error(err, "Unable to list source rancher secrets, looking for: "+client.ObjectKeyFromObject(s.RancherSecret).String())

		return err
	}

	for _, secret := range secretList.Items {
		if secret.GetAnnotations() == nil {
			continue
		}

		if name, found := secret.GetAnnotations()[NameAnnotation]; !found || name != s.RancherSecret.GetName() {
			continue
		}

		driverName := s.Source.ProviderName()
		if name, found := driverMapping[driverName]; found {
			driverName = name
		}

		if driver, found := secret.GetAnnotations()[DriverNameAnnotation]; !found || driver != driverName {
			continue
		}

		secret := secret
		s.RancherSecret = &secret

		return s.SecretSync.Get(ctx)
	}

	conditions.Set(s.Source, conditions.FalseCondition(
		turtlesv1.RancherCredentialsSecretCondition,
		turtlesv1.RancherCredentialSourceMissing,
		clusterv1.ConditionSeverityError,
		fmt.Sprintf(missingSource, cmp.Or(
			s.Source.Spec.Credentials.RancherCloudCredential,
			s.Source.Spec.Credentials.RancherCloudCredentialNamespaceName)),
	))

	return fmt.Errorf("unable to locate rancher secret with name %s for provider %s", s.RancherSecret.GetName(), s.Source.ProviderName())
}

// Sync updates the credentials secret with required values from rancher manager secret.
func (s *SecretMapperSync) Sync(ctx context.Context) error {
	log := log.FromContext(ctx)
	s.SecretSync.Secret.StringData = map[string]string{}

	if err := Into(s.Source.ProviderName(), s.RancherSecret.Data, s.SecretSync.Secret.StringData); err != nil {
		log.Error(err, "failed to map credential keys")

		conditions.Set(s.Source, conditions.FalseCondition(
			turtlesv1.RancherCredentialsSecretCondition,
			turtlesv1.RancherCredentialKeyMissing,
			clusterv1.ConditionSeverityError,
			fmt.Sprintf(missingKey, err.Error()),
		))

		return nil
	}

	log.Info(fmt.Sprintf("Credential keys from %s (%s) are successfully mapped to secret %s",
		client.ObjectKeyFromObject(s.RancherSecret).String(),
		cmp.Or(s.Source.Spec.Credentials.RancherCloudCredential, s.Source.Spec.Credentials.RancherCloudCredentialNamespaceName),
		client.ObjectKeyFromObject(s.SecretSync.Secret).String()))

	conditions.Set(s.Source, conditions.TrueCondition(
		turtlesv1.RancherCredentialsSecretCondition,
	))

	return nil
}

// Apply performs SSA patch of the secret mapper resources, using different FieldOwner from default
// to avoid collisions with patches performed by variable syncer on the same secret resource.
func (s *SecretMapperSync) Apply(ctx context.Context, reterr *error, options ...client.PatchOption) {
	s.DefaultSynchronizer.Apply(ctx, reterr, append(options, client.FieldOwner("secret-mapper-sync"))...)
}

// Into maps the secret keys from source secret data according to credentials map.
func Into(provider string, from map[string][]byte, to map[string]string) error {
	providerValues := knownProviderRequirements[provider]
	if providerValues == nil {
		return nil
	}

	errors := []error{}
	for _, value := range providerValues {
		errors = append(errors, value.from.validate(from))
		to[value.to] = value.from.convert(from)
	}

	return kerrors.NewAggregate(errors)
}
