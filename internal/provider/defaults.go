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
	"cmp"
	"context"
	"fmt"
	"maps"
	"strconv"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/controllers/clusterctl"
)

const (
	// AzureProvider is the default capz provider name.
	AzureProvider = "azure"
	// GCPProvider is the default capg provider name.
	GCPProvider = "gcp"
	trueValue   = "true"
)

// SetProviderSpec sets the default values for the provider spec and updates to latest available version.
func SetProviderSpec(ctx context.Context, cl client.Client, provider *turtlesv1.CAPIProvider) error {
	SetDefaultProviderSpec(provider)

	if err := setLatestVersion(ctx, cl, provider); err != nil {
		return err
	}

	switch provider.ProviderName() {
	case AzureProvider:
		if provider.Status.Variables == nil {
			provider.Status.Variables = map[string]string{}
		}

		provider.Status.Variables["EXP_AKS_RESOURCE_HEALTH"] = trueValue
	case GCPProvider:
		if provider.Status.Variables == nil {
			provider.Status.Variables = map[string]string{}
		}

		provider.Status.Variables["EXP_CAPG_GKE"] = trueValue
	}

	return nil
}

// SetDefaultProviderSpec sets the default values for the provider spec.
func SetDefaultProviderSpec(o operatorv1.GenericProvider) {
	providerSpec := o.GetSpec()
	providerNamespace := o.GetNamespace()

	if providerSpec.ConfigSecret != nil && providerSpec.ConfigSecret.Namespace == "" {
		providerSpec.ConfigSecret.Namespace = providerNamespace
	}

	if providerSpec.AdditionalManifestsRef != nil && providerSpec.AdditionalManifestsRef.Namespace == "" {
		providerSpec.AdditionalManifestsRef.Namespace = providerNamespace
	}

	if provider, ok := o.(*turtlesv1.CAPIProvider); ok {
		setVariables(provider)
		setFeatures(provider)
	}

	providerSpec.ConfigSecret = cmp.Or(providerSpec.ConfigSecret, &operatorv1.SecretReference{
		Name: o.GetName(),
	})

	providerSpec.ConfigSecret.Namespace = cmp.Or(providerSpec.ConfigSecret.Namespace, providerNamespace)

	o.SetSpec(providerSpec)
}

func setLatestVersion(ctx context.Context, cl client.Client, provider *turtlesv1.CAPIProvider) error {
	log := log.FromContext(ctx)

	config, err := clusterctl.ClusterConfig(ctx, cl)
	if err != nil {
		return err
	}

	providerVersion, knownProvider := config.GetProviderVersion(ctx, provider.ProviderName(), provider.Spec.Type.ToKind())

	latest, err := config.IsLatestVersion(providerVersion, provider.Spec.Version)
	if err != nil {
		return err
	}

	switch {
	case !knownProvider:
		conditions.MarkUnknown(provider, turtlesv1.CheckLatestVersionTime, turtlesv1.CheckLatestProviderUnknownReason, "Provider is unknown")
	case latest:
		conditions.MarkTrue(provider, turtlesv1.CheckLatestVersionTime)
		provider.Spec.Version = providerVersion
	case !latest && !provider.Spec.EnableAutomaticUpdate:
		conditions.MarkFalse(
			provider,
			turtlesv1.CheckLatestVersionTime,
			turtlesv1.CheckLatestUpdateAvailableReason,
			clusterv1.ConditionSeverityInfo,
			"Provider version update available. Current latest is %s", providerVersion,
		)
	case !latest && provider.Spec.EnableAutomaticUpdate:
		lastCheck := conditions.Get(provider, turtlesv1.CheckLatestVersionTime)
		updatedMessage := fmt.Sprintf("Updated to latest %s version", providerVersion)

		if lastCheck == nil || lastCheck.Message != updatedMessage {
			log.Info(fmt.Sprintf("Version %s is beyond current latest, updated to %s", cmp.Or(provider.Spec.Version, "latest"), providerVersion))

			lastCheck = conditions.TrueCondition(turtlesv1.CheckLatestVersionTime)
			lastCheck.Message = updatedMessage

			conditions.Set(provider, lastCheck)
		}

		provider.Spec.Version = providerVersion
	}

	return nil
}

func setVariables(capiProvider *turtlesv1.CAPIProvider) {
	if capiProvider.Spec.Variables != nil {
		maps.Copy(capiProvider.Status.Variables, capiProvider.Spec.Variables)
	}
}

func setFeatures(capiProvider *turtlesv1.CAPIProvider) {
	features := capiProvider.Spec.Features
	variables := capiProvider.Status.Variables

	if features != nil {
		variables["EXP_CLUSTER_RESOURCE_SET"] = strconv.FormatBool(features.ClusterResourceSet)
		variables["CLUSTER_TOPOLOGY"] = strconv.FormatBool(features.ClusterTopology)
		variables["EXP_MACHINE_POOL"] = strconv.FormatBool(features.MachinePool)
	}
}
