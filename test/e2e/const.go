//go:build e2e
// +build e2e

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

package e2e

import (
	_ "embed"
)

var (
	//go:embed data/capi-operator/capi-providers.yaml
	CapiProviders []byte

	//go:embed data/capi-operator/capi-providers-legacy.yaml
	CapiProvidersLegacy []byte

	//go:embed data/capi-operator/capv-provider.yaml
	CapvProvider []byte

	//go:embed data/capi-operator/aws-provider.yaml
	AWSProvider []byte

	//go:embed data/capi-operator/gcp-provider.yaml
	GCPProvider []byte

	//go:embed data/capi-operator/azure-provider.yaml
	AzureProvider []byte

	//go:embed data/capi-operator/capa-variables.yaml
	AWSProviderSecret []byte

	//go:embed data/capi-operator/capz-identity-secret.yaml
	AzureIdentitySecret []byte

	//go:embed data/capi-operator/capg-variables.yaml
	GCPProviderSecret []byte

	//go:embed data/capi-operator/capv-identity-secret.yaml
	VSphereProviderSecret []byte

	//go:embed data/rancher/ingress.yaml
	IngressConfig []byte

	//go:embed data/rancher/system-store-setting-patch.yaml
	SystemStoreSettingPatch []byte

	//go:embed data/rancher/rancher-service-patch.yaml
	RancherServicePatch []byte

	//go:embed data/rancher/ingress-class-patch.yaml
	IngressClassPatch []byte

	//go:embed data/rancher/rancher-setting-patch.yaml
	RancherSettingPatch []byte

	//go:embed data/rancher/nginx-ingress.yaml
	NginxIngress []byte

	//go:embed data/rancher/ingress-nginx-lb.yaml
	NginxIngressLoadBalancer []byte

	//go:embed data/chartmuseum/deployment.yaml
	ChartMuseum []byte

	//go:embed data/chartmuseum/ingress.yaml
	ChartMuseumIngress []byte

	//go:embed data/rancher/azure-rke-config.yaml
	V2ProvAzureRkeConfig []byte

	//go:embed data/rancher/azure-cluster.yaml
	V2ProvAzureCluster []byte

	//go:embed data/cluster-templates/docker-kubeadm-topology.yaml
	CAPIDockerKubeadmTopology []byte

	//go:embed data/cluster-templates/docker-rke2-topology.yaml
	CAPIDockerRKE2Topology []byte

	//go:embed data/cluster-templates/aws-eks-mmp.yaml
	CAPIAwsEKSMMP []byte

	//go:embed data/cluster-templates/aws-ec2-rke2-topology.yaml
	CAPIAwsEC2RKE2Topology []byte

	//go:embed data/cluster-templates/aws-kubeadm-topology.yaml
	CAPIAwsKubeadmTopology []byte

	//go:embed data/cluster-templates/gcp-gke.yaml
	CAPIGCPGKE []byte

	//go:embed data/cluster-templates/azure-aks-topology.yaml
	CAPIAzureAKSTopology []byte

	//go:embed data/cluster-templates/azure-rke2-topology.yaml
	CAPIAzureRKE2Topology []byte

	//go:embed data/cluster-templates/azure-kubeadm-topology.yaml
	CAPIAzureKubeadmTopology []byte

	//go:embed data/cluster-templates/vsphere-kubeadm-topology.yaml
	CAPIvSphereKubeadmTopology []byte

	//go:embed data/cluster-templates/vsphere-rke2-topology.yaml
	CAPIvSphereRKE2Topology []byte

	//go:embed data/gitea/ingress.yaml
	GiteaIngress []byte

	//go:embed data/gitea/values.yaml
	GiteaValues []byte
)

const (
	RancherTurtlesNamespace = "rancher-turtles-system"
	RancherNamespace        = "cattle-system"
	NginxIngressNamespace   = "ingress-nginx"
	NginxIngressDeployment  = "ingress-nginx-controller"
)

type ManagementClusterEnvironmentType string

const (
	ManagementClusterEnvironmentEKS          ManagementClusterEnvironmentType = "eks"
	ManagementClusterEnvironmentIsolatedKind ManagementClusterEnvironmentType = "isolated-kind"
	ManagementClusterEnvironmentKind         ManagementClusterEnvironmentType = "kind"
	ManagementClusterEnvironmentInternalKind ManagementClusterEnvironmentType = "internal-kind"
)

const (
	ManagementClusterEnvironmentVar = "MANAGEMENT_CLUSTER_ENVIRONMENT"

	KubernetesManagementVersionVar = "KUBERNETES_MANAGEMENT_VERSION"

	BootstrapClusterNameVar       = "BOOTSTRAP_CLUSTER_NAME"
	BootstrapClusterKubeconfigVar = "BOOTSTRAP_CLUSTER_KUBECONFIG_PATH"

	ClusterctlRepositoryFolderVar = "CLUSTERCTL_REPOSITORY_FOLDER"

	KubernetesVersionVar      = "KUBERNETES_VERSION"
	AWSKubernetesVersionVar   = "AWS_KUBERNETES_VERSION"
	AzureKubernetesVersionVar = "AZURE_KUBERNETES_VERSION"
	RancherFeaturesVar        = "RANCHER_FEATURES"
	RancherHostnameVar        = "RANCHER_HOSTNAME"
	RancherVersionVar         = "RANCHER_VERSION"
	RancherAlphaVersionVar    = "RANCHER_ALPHA_VERSION"
	RancherPathVar            = "RANCHER_PATH"
	RancherAlphaPathVar       = "RANCHER_ALPHA_PATH"
	RancherUrlVar             = "RANCHER_URL"
	RancherAlphaUrlVar        = "RANCHER_ALPHA_URL"
	RancherRepoNameVar        = "RANCHER_REPO_NAME"
	RancherAlphaRepoNameVar   = "RANCHER_ALPHA_REPO_NAME"
	RancherPasswordVar        = "RANCHER_PASSWORD"
	CertManagerUrlVar         = "CERT_MANAGER_URL"
	CertManagerRepoNameVar    = "CERT_MANAGER_REPO_NAME"
	CertManagerPathVar        = "CERT_MANAGER_PATH"
	CapiInfrastructureVar     = "CAPI_INFRASTRUCTURE"

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

	ArtifactsFolderVar       = "ARTIFACTS_FOLDER"
	UseExistingClusterVar    = "USE_EXISTING_CLUSTER"
	HelmBinaryPathVar        = "HELM_BINARY_PATH"
	HelmExtraValuesFolderVar = "HELM_EXTRA_VALUES_FOLDER"
	TurtlesVersionVar        = "TURTLES_VERSION"
	TurtlesPathVar           = "TURTLES_PATH"
	TurtlesUrlVar            = "TURTLES_URL"
	TurtlesRepoNameVar       = "TURTLES_REPO_NAME"
	SkipResourceCleanupVar   = "SKIP_RESOURCE_CLEANUP"
	SkipDeletionTestVar      = "SKIP_DELETION_TEST"
	ClusterctlBinaryPathVar  = "CLUSTERCTL_BINARY_PATH"

	RKE2VersionVar = "RKE2_VERSION"

	AzureSubIDVar        = "AZURE_SUBSCRIPTION_ID"
	AzureClientIDVar     = "AZURE_CLIENT_ID"
	AzureClientSecretVar = "AZURE_CLIENT_SECRET"

	AuthSecretName = "basic-auth-secret"

	ShortTestLabel   = "short"
	FullTestLabel    = "full"
	DontRunLabel     = "dontrun"
	VsphereTestLabel = "vsphere"
	KubeadmTestLabel = "kubeadm"
	Rke2TestLabel    = "rke2"

	CapiClusterOwnerLabel          = "cluster-api.cattle.io/capi-cluster-owner"
	CapiClusterOwnerNamespaceLabel = "cluster-api.cattle.io/capi-cluster-owner-ns"
	OwnedLabelName                 = "cluster-api.cattle.io/owned"
)

const (
	CAPIVersion = "v1.10.2"
)
