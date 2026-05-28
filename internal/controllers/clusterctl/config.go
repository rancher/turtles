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

package clusterctl

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	accorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/feature"
)

var config *corev1.ConfigMap

const (
	// ConfigPath is the path of the mounted clusterctl config.
	ConfigPath = "/config/clusterctl.yaml"
)

func init() {
	utilruntime.Must(yaml.UnmarshalStrict(configDefault, &config))
}

// ConfigRepository is a direct clusterctl config repository representation.
type ConfigRepository struct {
	Providers turtlesv1.ProviderList `json:"providers"`
	Images    map[string]ConfigImage `json:"images"`
}

// ConfigImage is a direct clusterctl representation of image config value.
type ConfigImage struct {
	// Repository sets the container registry override to pull images from.
	Repository string `json:"repository,omitempty"`

	// Tag allows to specify a tag for the images.
	Tag string `json:"tag,omitempty"`
}

// Config returns current set of embedded turtles clusterctl overrides.
func Config() *corev1.ConfigMap {
	configMap := config.DeepCopy()

	namespace := cmp.Or(os.Getenv("POD_NAMESPACE"), "cattle-turtles-system")
	configMap.Namespace = namespace
	configMap.Annotations["meta.helm.sh/release-namespace"] = namespace

	return configMap
}

// SyncConfigMap updates the Clusterctl ConfigMap with the user-specified
// overrides from ClusterctlConfig.
func SyncConfigMap(ctx context.Context, c client.Client, owner string) error {
	configMap := Config()

	clusterctlConfig, err := ClusterConfig(ctx, c)
	if err != nil {
		return fmt.Errorf("getting updated ClusterctlConfig: %w", err)
	}

	clusterctlYaml, err := yaml.Marshal(clusterctlConfig)
	if err != nil {
		return fmt.Errorf("serializing updated ClusterctlConfig: %w", err)
	}

	acConfigMap := accorev1.ConfigMap(configMap.GetName(), configMap.GetNamespace())

	acConfigMap.Data = map[string]string{
		"clusterctl.yaml": string(clusterctlYaml),
	}

	if err := c.Apply(ctx, acConfigMap, []client.ApplyOption{
		client.ForceOwnership,
		client.FieldOwner(owner),
	}...); err != nil {
		return fmt.Errorf("patching clusterctl ConfigMap: %w", err)
	}

	return nil
}

// ClusterConfig collects overrides config from the local in-memory state
// and the user-specified ClusterctlConfig overrides layer.
func ClusterConfig(ctx context.Context, c client.Client) (*ConfigRepository, error) {
	log := log.FromContext(ctx)

	configMap := Config()

	config := &turtlesv1.ClusterctlConfig{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(configMap), config); client.IgnoreNotFound(err) != nil {
		log.Error(err, "Unable to collect ClusterctlConfig resource")

		return nil, err
	}

	clusterctlConfig := &ConfigRepository{}
	if err := yaml.UnmarshalStrict([]byte(configMap.Data["clusterctl.yaml"]), &clusterctlConfig); err != nil {
		log.Error(err, "Unable to deserialize initial clusterctl config")

		return nil, err
	}

	if clusterctlConfig.Images == nil {
		clusterctlConfig.Images = map[string]ConfigImage{}
	}

	// Deduplicate and update providers
	existingProviders := make(map[string]int)

	for i, provider := range clusterctlConfig.Providers {
		key := provider.Name + "-" + provider.Type
		existingProviders[key] = i
	}

	for _, newProvider := range config.Spec.Providers {
		key := newProvider.Name + "-" + newProvider.Type
		if idx, exists := existingProviders[key]; exists {
			// Update existing provider
			oldProvider := clusterctlConfig.Providers[idx]
			clusterctlConfig.Providers[idx] = newProvider
			log.Info("Updated existing provider", "name", newProvider.Name, "type", newProvider.Type, "oldURL", oldProvider.URL, "newURL", newProvider.URL)
		} else {
			// Add new provider
			clusterctlConfig.Providers = append(clusterctlConfig.Providers, newProvider)
			log.Info("Added new provider", "name", newProvider.Name, "type", newProvider.Type, "url", newProvider.URL)
		}
	}

	if feature.Gates.Enabled(feature.UseRancherDefaultRegistry) {
		log.Info("Turtles configured to use Rancher default registry for images")

		setting := &managementv3.Setting{}
		if err := c.Get(ctx, client.ObjectKey{Name: "system-default-registry"}, setting); err != nil {
			log.Error(err, "Unable to get system-default-registry setting")
			return nil, err
		}

		registry := setting.Value
		if registry != "" {
			log.Info("Rancher default registry has been set", "registry", registry)

			if !strings.HasSuffix(registry, "/") {
				registry += "/"
			}

			// Iterate through all images for the supported providers and override
			// the repository to use Rancher's system default registry
			for image, url := range clusterctlConfig.Images {
				namespace := extractNamespace(url.Repository)

				clusterctlConfig.Images[image] = ConfigImage{
					Tag:        url.Tag,
					Repository: registry + namespace,
				}
				log.Info("Overridden provider image to use Rancher default registry", "image", image,
					"repository", clusterctlConfig.Images[image].Repository, "tag", url.Tag)
			}
		}
	}

	// Override images from ClusterctlConfig
	for _, image := range config.Spec.Images {
		clusterctlConfig.Images[image.Name] = ConfigImage{
			Tag:        image.Tag,
			Repository: image.Repository,
		}

		log.Info("Overridden provider image from ClusterctlConfig", "image", image.Name, "repository", image.Repository, "tag", image.Tag)
	}

	return clusterctlConfig, nil
}

func extractNamespace(imageURI string) string {
	parts := strings.Split(imageURI, "/")
	if len(parts) > 1 {
		return strings.Join(parts[1:], "/")
	}

	return imageURI
}

// GetProviderVersion gets the version of the provider parsing the provider URL in the clusterctl config.
// It will return the found version if any, empty otherwise.
func (r *ConfigRepository) GetProviderVersion(ctx context.Context, name, providerType string) (string, error) {
	log := log.FromContext(ctx)

	for _, provider := range r.Providers {
		if provider.Name == name && strings.EqualFold(provider.Type, providerType) {
			log.V(5).Info("Provider found in Turtles clusterctl config")
			return parseVersionFromURL(provider.URL)
		}
	}

	return "", nil
}

// parseVersionFromURL parses the version from the provider URL.
// For example: "https://github.com/rancher/cluster-api-provider-aws/releases/v2.11.1/infrastructure-components.yaml"
// Beware that "latest" is also a valid version, this does not check for a semantic version format.
func parseVersionFromURL(url string) (string, error) {
	version := strings.Split(url, "/")
	slices.Reverse(version)

	if len(version) < 2 {
		return "", fmt.Errorf("Provider url is invalid for version resolve: %s", url)
	}

	return version[1], nil
}
