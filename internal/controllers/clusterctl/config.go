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
	"slices"
	"strings"

	"github.com/blang/semver/v4"
	"sigs.k8s.io/controller-runtime/pkg/log"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
)

const (
	latestVersionKey = "latest"
)

// ConfigRepository is a direct clusterctl config repository representation.
type ConfigRepository struct {
	Providers []turtlesv1.Provider   `json:"providers"`
	Images    map[string]ConfigImage `json:"images"`
}

// ConfigImage is a direct clusterctl representation of image config value.
type ConfigImage struct {
	// Repository sets the container registry override to pull images from.
	Repository string `json:"repository,omitempty"`

	// Tag allows to specify a tag for the images.
	Tag string `json:"tag,omitempty"`
}

// ConfigProvider mirrors turtlesv1.Provider and allows serialization of the corresponding info.
type ConfigProvider struct {
	// Name of the provider
	Name string `json:"name"`

	// URL of the provider components. Will be used unless and override is specified
	URL string `json:"url"`

	// Type is the type of the provider (ex. InfrastructureProvider)
	Type string `json:"type"`
}

// GetProviderVersion collects version of the collected provider overrides state.
// Returns latest if the version is not found.
func (r *ConfigRepository) GetProviderVersion(ctx context.Context, name, providerType string) (version string, providerKnown bool) {
	for _, provider := range r.Providers {
		if provider.Name == name && strings.EqualFold(provider.Type, providerType) {
			return collectVersion(ctx, provider.URL), true
		}
	}

	return latestVersionKey, false
}

func collectVersion(ctx context.Context, url string) string {
	version := strings.Split(url, "/")
	slices.Reverse(version)

	if len(version) < 2 {
		log.FromContext(ctx).Info("Provider url is invalid for version resolve, defaulting to latest", "url", url)

		return latestVersionKey
	}

	return version[1]
}

// IsLatestVersion checks version against the expected max version, and returns false
// if the version given is newer then the latest in the clusterctlconfig override.
func (r *ConfigRepository) IsLatestVersion(providerVersion, expected string) (bool, error) {
	// Return true for providers without version boundary or unknown providers
	if providerVersion == latestVersionKey {
		return true, nil
	}

	version, _ := strings.CutPrefix(providerVersion, "v")

	maxVersion, err := semver.Parse(version)
	if err != nil {
		return false, fmt.Errorf("unable to parse default provider version %s: %w", providerVersion, err)
	}

	expected = cmp.Or(expected, latestVersionKey)
	if expected == latestVersionKey {
		// Latest should be reduced to the actual version set on the clusterctlprovider resource
		return false, nil
	}

	version, _ = strings.CutPrefix(expected, "v")

	desiredVersion, err := semver.Parse(version)
	if err != nil {
		return false, fmt.Errorf("unable to parse desired version %s: %w", expected, err)
	}

	// Disallow versions beyond current clusterctl.yaml override default
	return maxVersion.LTE(desiredVersion), nil
}
