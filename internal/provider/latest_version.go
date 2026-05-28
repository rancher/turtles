package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/blang/semver/v4"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/controllers/clusterctl"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	configclient "sigs.k8s.io/cluster-api/cmd/clusterctl/client/config"
)

// lookupLatestVersion tries to find the latest available version of a given provider.
// This is used to notify the user there's a new version available, or automatically update to it
// if automatic updates are enabled.
//
// The lookup strategy follows this order:
//  1. Lookup against Turtles clusterctl config. This is built from the embedded clusterctl config
//     and includes user defined ClusterctlConfig overrides if any. We always expect a pinned version here, not "latest".
//  2. If the provider repository is not found in Turtles clusterctl config, the upstream list of providers is used.
//     This is equivalent of running `clusterctl config repositories`.
//  3. If a provider is found in this upstream list, leave the version empty and let cluster-api-operator do the remote version lookup.
//
// The function returns the version if the provider was found and a version was pinned, empty otherwise.
func lookupLatestVersion(ctx context.Context, cl client.Client, provider *turtlesv1.CAPIProvider) (latestVersion string, providerKnown bool, lookupError error) {
	log := log.FromContext(ctx)

	if provider.Spec.Version == "" {
		log.V(5).Info("Provider has empty version. Can not compare to determine latest.")
		return "", false, fmt.Errorf("Can not compare empty provider version to determine if there are updates.")
	}

	// Load the Turtles clusterctl config.
	config, err := clusterctl.ClusterConfig(ctx, cl)
	if err != nil {
		return "", false, fmt.Errorf("getting Turtles clusterctl config: %w", err)
	}

	// 1. Check if the provider is known by Turtles. This includes the ClusterctlConfig user overrides if any.
	foundProviderVersion, err := config.GetProviderVersion(ctx, provider.ProviderName(), provider.Spec.Type.ToKind())
	if err != nil {
		return "", false, fmt.Errorf("getting provider version from Turtles clusterctl config")
	}

	if foundProviderVersion != "" {
		log.V(5).Info(fmt.Sprintf("Provider found in Turtles config. Found version: %s", foundProviderVersion))

		if latestAvailableVersion, err := getLatestAvailableVersion(ctx, provider.Spec.Version, foundProviderVersion); err != nil {
			return latestAvailableVersion, true, nil
		} else {
			return "", true, fmt.Errorf("determining latest available version: %w", err)
		}
	}

	// 2. If the provider is not known by Turtles, lookup the upstream provider list.
	if foundProviderVersion == "" {
		configClient, err := configclient.New(ctx, "/config/clusterctlconfig.yaml")
		if err != nil {
			return "", false, fmt.Errorf("initializing config client: %w", err)
		}

		knownProviders, err := configClient.Providers().List()
		if err != nil {
			return "", false, fmt.Errorf("initializing config client: %w", err)
		}
	}

	return "", false, nil
}

// getLatestAvailableVersion compares two semantic versions and returns the highest available.
func getLatestAvailableVersion(ctx context.Context, currentVersion, comparedVersion string) (string, error) {
	log := log.FromContext(ctx)

	providerNeedsUpdate, err := needsUpdate(currentVersion, comparedVersion)
	if err != nil {
		return "", fmt.Errorf("determining if provider needs update from Turtles clusterctl config: %w", err)
	}

	if providerNeedsUpdate {
		log.V(5).Info(fmt.Sprintf("Provider can be updated to: %s", comparedVersion))
		return comparedVersion, nil
	} else {
		log.V(5).Info(fmt.Sprintf("Provider version %s is already up to date.", currentVersion))
		return currentVersion, nil
	}
}

// needsUpdate checks the current version against the compared version.
func needsUpdate(currentVersion, comparedVersion string) (bool, error) {
	if currentVersion == "" {
		return false, fmt.Errorf("Can not compare empty provider version")
	}
	if comparedVersion == "" {
		return false, fmt.Errorf("Can not compare empty expected version")
	}

	current, _ := strings.CutPrefix(currentVersion, "v")

	currentSemVer, err := semver.Parse(current)
	if err != nil {
		return false, fmt.Errorf("parsing the current provider version %s: %w", currentVersion, err)
	}

	compared, _ := strings.CutPrefix(comparedVersion, "v")

	comparedSemVer, err := semver.Parse(compared)
	if err != nil {
		return false, fmt.Errorf("parsing the compared version %s: %w", comparedVersion, err)
	}

	return currentSemVer.LT(comparedSemVer), nil
}
