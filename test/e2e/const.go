//go:build e2e
// +build e2e

/*
Copyright 2023 SUSE.

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

package e2e

import (
	_ "embed"
)

var (
	//go:embed data/capi-operator/capi-providers-secret.yaml
	CapiProvidersSecret []byte

	//go:embed data/capi-operator/capi-providers.yaml
	CapiProviders []byte

	//go:embed data/capi-operator/full-variables.yaml
	FullProvidersSecret []byte

	//go:embed data/capi-operator/full-providers.yaml
	FullProviders []byte

	//go:embed data/rancher/ingress.yaml
	IngressConfig []byte

	//go:embed data/rancher/rancher-service-patch.yaml
	RancherServicePatch []byte

	//go:embed data/rancher/ingress-class-patch.yaml
	IngressClassPatch []byte

	//go:embed data/rancher/rancher-setting-patch.yaml
	RancherSettingPatch []byte

	//go:embed data/rancher/nginx-ingress.yaml
	NginxIngress []byte
)

const (
	RancherTurtlesNamespace = "rancher-turtles-system"
	RancherNamespace        = "cattle-system"
	NginxIngressNamespace   = "ingress-nginx"
)

const (
	KubernetesVersionVar  = "KUBERNETES_VERSION"
	RancherFeaturesVar    = "RANCHER_FEATURES"
	RancherHostnameVar    = "RANCHER_HOSTNAME"
	RancherVersionVar     = "RANCHER_VERSION"
	RancherPathVar        = "RANCHER_PATH"
	RancherUrlVar         = "RANCHER_URL"
	RancherRepoNameVar    = "RANCHER_REPO_NAME"
	RancherPasswordVar    = "RANCHER_PASSWORD"
	CapiInfrastructureVar = "CAPI_INFRASTRUCTURE"
	CapiCoreVar           = "CAPI_CORE"

	NgrokRepoNameVar  = "NGROK_REPO_NAME"
	NgrokUrlVar       = "NGROK_URL"
	NgrokPathVar      = "NGROK_PATH"
	NgrokApiKeyVar    = "NGROK_API_KEY"
	NgrokAuthTokenVar = "NGROK_AUTHTOKEN"

	GiteaRepoNameVar     = "GITEA_REPO_NAME"
	GiteaRepoURLVar      = "GITEA_REPO_URL"
	GiteaChartNameVar    = "GITEA_CHART_NAME"
	GiteaChartVersionVar = "GITEA_CHART_VERSION"
	GiteaUserNameVar     = "GITEA_USER_NAME"
	GiteaUserPasswordVar = "GITEA_USER_PWD"

	CapaEncodedCredentialsVar = "CAPA_ENCODED_CREDS"

	AuthSecretName = "basic-auth-secret"

	ShortTestLabel = "short"
	FullTestLabel  = "full"
)
