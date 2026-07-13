/*
Copyright © 2026 SUSE LLC

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
	"context"
	"fmt"
	"strings"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/feature"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clustercltconfig "sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	// clusterctlConfigName is the unique name of the user defined ClusterctlConfig.
	clusterctlConfigName = "clusterctl-config"
	// clusterctlConfigNamespace is the namespace used to lookup the ClusterctlConfig.
	clusterctlConfigNamespace = "cattle-turtles-system"
	// rancherSystemDefaultRegistrySettingKey is the system default registry Rancher Setting key.
	rancherSystemDefaultRegistrySettingKey = "system-default-registry"
	// providersVariableKey is the clusterctl config key for providers override.
	providersVariableKey = "providers"
	// imagesVariableKey is the clusterctl config key for images override.
	imagesVariableKey = "images"
)

var _ clustercltconfig.Reader = &ClusterctlConfigReader{}

// ClusterctlConfigReader is a clusterctl config Reader implementation
// backed by the user defined Turtles ClusterctlConfig resource.
type ClusterctlConfigReader struct {
	// client is a k8s client used to fetch resources.
	client client.Client
	// variables contains the list of marshalled clusterctl config keys (ex. "providers", "images")
	variables map[string]string

	// Providers contains a list of providers overrides.
	Providers []ConfigProvider `json:"providers"`
	// Images contains a list of images overrides.
	Images map[string]ConfigImage `json:"images"`
}

// NewClusterctlConfigReader return a new ClusterctlConfigReader.
func NewClusterctlConfigReader(client client.Client) *ClusterctlConfigReader {
	return &ClusterctlConfigReader{
		client: client,

		Providers: []ConfigProvider{},
		Images:    map[string]ConfigImage{},
	}
}

// Init initialize the reader.
func (c *ClusterctlConfigReader) Init(ctx context.Context, _ string) error {
	// 0. Reset providers and images.
	c.Providers = []ConfigProvider{}
	c.Images = map[string]ConfigImage{}

	// 1. Initialize the Providers list using the upstream embedded clusterctl config.
	upstreamConfig, err := clustercltconfig.New(ctx, "")
	if err != nil {
		return fmt.Errorf("initializing upstream clusterctlconfig Client: %w", err)
	}

	upstreamProviders, err := upstreamConfig.Providers().List()
	if err != nil {
		return fmt.Errorf("listing upstream clusterctl config Providers: %w", err)
	}

	for _, upstreamProvider := range upstreamProviders {
		c.Providers = append(c.Providers, ConfigProvider{
			Name: upstreamProvider.Name(),
			Type: string(upstreamProvider.Type()),
			URL:  upstreamProvider.URL(),
		})
	}

	// 2. Fetch the user defined ClusterctlConfig (if any).
	clusterctlConfig := &turtlesv1.ClusterctlConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterctlConfigName,
			Namespace: clusterctlConfigNamespace,
		},
	}
	if err := c.client.Get(ctx, client.ObjectKeyFromObject(clusterctlConfig), clusterctlConfig); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("fetching ClusterctlConfig %s/%s: %w", clusterctlConfigNamespace, clusterctlConfigName, err)
	}

	if apierrors.IsNotFound(err) {
		// If the ClusterctlConfig is not found, there's nothing more to do.
		return nil
	}

	// 3. Merge the initial upstream providers list with the user defined one.
	for _, userDefinedProvider := range clusterctlConfig.Spec.Providers {
		// If this provider is found in the initial list, override it.
		found := false
		for i, provider := range c.Providers {
			if userDefinedProvider.Name == provider.Name {
				c.Providers[i] = ConfigProvider{
					Name: userDefinedProvider.Name,
					Type: userDefinedProvider.Type,
					URL:  userDefinedProvider.URL,
				}
				found = true
				break
			}
		}
		if found {
			break
		}
		// Otherwise, append it to the list of providers.
		c.Providers = append(c.Providers, ConfigProvider{
			Name: userDefinedProvider.Name,
			Type: userDefinedProvider.Type,
			URL:  userDefinedProvider.URL,
		})
	}

	// 4. Add the user defined images list to the config.
	for _, userDefinedImage := range clusterctlConfig.Spec.Images {
		c.Images[userDefinedImage.Name] = ConfigImage{
			Repository: userDefinedImage.Repository,
			Tag:        userDefinedImage.Tag,
		}
	}

	// 5. Override images repositories using the Rancher system-default-registry Setting.
	if feature.Gates.Enabled(feature.UseRancherDefaultRegistry) {
		setting := &managementv3.Setting{}
		if err := c.client.Get(ctx, client.ObjectKey{Name: rancherSystemDefaultRegistrySettingKey}, setting); err != nil {
			return fmt.Errorf("fetching %s Setting: %w", rancherSystemDefaultRegistrySettingKey, err)
		}

		registry := setting.Value
		if registry != "" {
			// Iterate through all images for the supported providers and override
			// the repository to use Rancher's system default registry
			for imageKey, configImage := range c.Images {
				if configImage.Repository != "" {
					configImage.Repository = overrideRegistry(configImage.Repository, registry)
					c.Images[imageKey] = configImage
				}
			}
		}
	}

	// 6. Populate the variables
	c.variables = map[string]string{}

	providersData, err := yaml.Marshal(c.Providers)
	if err != nil {
		return fmt.Errorf("marshalling providers data: %w", err)
	}
	c.variables[providersVariableKey] = string(providersData)

	imagesData, err := yaml.Marshal(c.Images)
	if err != nil {
		return fmt.Errorf("marshalling images data: %w", err)
	}
	c.variables[imagesVariableKey] = string(imagesData)

	return nil
}

// Get gets a value for the given key.
func (c *ClusterctlConfigReader) Get(key string) (string, error) {
	if val, ok := c.variables[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf("value for variable %q is not set", key)
}

// Set sets a value for the given key.
func (c *ClusterctlConfigReader) Set(_, _ string) {
	// Only allow variables to be set via the ClusterctlConfig.
}

// UnmarshalKey gets a value for the given key, then unmarshal it.
func (c *ClusterctlConfigReader) UnmarshalKey(key string, rawval interface{}) error {
	data, err := c.Get(key)
	if err != nil {
		return nil //nolint:nilerr // We expect to not error if the key is not present
	}
	return yaml.Unmarshal([]byte(data), rawval)
}

// overrideRegistry is a utility function that overrides the registry given a repository uri.
// For example from "myorg.io/my-namespace/my-repo" to "myoverride.io/my-namespace/my-repo"
func overrideRegistry(repository string, registryOverride string) string {
	slices := strings.Split(repository, "/")
	slices[0] = registryOverride
	return strings.Join(slices, "/")
}
