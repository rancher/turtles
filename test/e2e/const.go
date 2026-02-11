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

	//go:embed data/capi-operator/capi-providers-oci.yaml
	CapiProvidersOci []byte

	//go:embed data/capi-operator/capv-provider.yaml
	CapvProvider []byte

	//go:embed data/capi-operator/aws-provider.yaml
	AWSProvider []byte

	//go:embed data/capi-operator/gcp-provider.yaml
	GCPProvider []byte

	//go:embed data/capi-operator/azure-provider.yaml
	AzureProvider []byte

	//go:embed data/capi-operator/capa-identity-secret.yaml
	AWSIdentitySecret []byte

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

	//go:embed data/rancher/azure-rke-config.yaml
	V2ProvAzureRkeConfig []byte

	//go:embed data/rancher/azure-cluster.yaml
	V2ProvAzureCluster []byte

	//go:embed data/cluster-templates/docker-kubeadm-topology.yaml
	CAPIDockerKubeadmTopology []byte

	//go:embed data/cluster-templates/docker-rke2-topology.yaml
	CAPIDockerRKE2Topology []byte

	//go:embed data/cluster-templates/docker-rke2-v1beta1-topology.yaml
	CAPIDockerRKE2V1Beta1Topology []byte

	//go:embed data/cluster-templates/aws-eks-topology.yaml
	CAPIAwsEKSTopology []byte

	//go:embed data/cluster-templates/aws-ec2-rke2-topology.yaml
	CAPIAwsEC2RKE2Topology []byte

	//go:embed data/cluster-templates/aws-kubeadm-topology.yaml
	CAPIAwsKubeadmTopology []byte

	//go:embed data/cluster-templates/gcp-gke.yaml
	CAPIGCPGKE []byte

	//go:embed data/cluster-templates/gcp-kubeadm-topology.yaml
	CAPIGCPKubeadmTopology []byte

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

	//go:embed data/capi-operator/clusterctlconfig.yaml
	ClusterctlConfig []byte

	// CAPIProvider test data

	//go:embed data/test-providers/capv-provider-no-ver.yaml
	CAPVProviderNoVersion []byte

	//go:embed data/test-providers/unknown-provider.yaml
	UnknownProvider []byte

	//go:embed data/test-providers/clusterctlconfig.yaml
	ClusterctlConfigInitial []byte

	//go:embed data/test-providers/clusterctlconfig-updated.yaml
	ClusterctlConfigUpdated []byte

	//go:embed data/test-providers/dummy-vsphere-template.yaml
	CAPVDummyMachineTemplate []byte

	// Extra Environment

	//go:embed data/gitea/ingress.yaml
	GiteaIngress []byte

	//go:embed data/gitea/values.yaml
	GiteaValues []byte

	//go:embed data/test-switch/turtles-embedded-cluster-api-feature.yaml
	TurtlesEmbeddedCAPIFeature []byte
)

const (
	RancherTurtlesNamespace    = "rancher-turtles-system"
	NewRancherTurtlesNamespace = "cattle-turtles-system"
	RancherNamespace           = "cattle-system"
	NginxIngressNamespace      = "ingress-nginx"
	NginxIngressDeployment     = "ingress-nginx-controller"
)

type ManagementClusterEnvironmentType string

const (
	ManagementClusterEnvironmentEKS          ManagementClusterEnvironmentType = "eks"
	ManagementClusterEnvironmentIsolatedKind ManagementClusterEnvironmentType = "isolated-kind"
	ManagementClusterEnvironmentKind         ManagementClusterEnvironmentType = "kind"
	ManagementClusterEnvironmentInternalKind ManagementClusterEnvironmentType = "internal-kind"
)

const (
	KubernetesManagementVersionVar = "KUBERNETES_MANAGEMENT_VERSION"

	BootstrapClusterNameVar = "BOOTSTRAP_CLUSTER_NAME"

	RancherHostnameVar = "RANCHER_HOSTNAME"

	ArtifactsFolderVar     = "ARTIFACTS_FOLDER"
	UseExistingClusterVar  = "USE_EXISTING_CLUSTER"
	HelmBinaryPathVar      = "HELM_BINARY_PATH"
	SkipResourceCleanupVar = "SKIP_RESOURCE_CLEANUP"
	SkipDeletionTestVar    = "SKIP_DELETION_TEST"

	KubernetesVersionChartUpgradeVar = "KUBERNETES_MANAGEMENT_VERSION_CHART_UPGRADE"

	RKE2VersionVar = "RKE2_KUBERNETES_VERSION"

	AzureSubIDVar        = "AZURE_SUBSCRIPTION_ID"
	AzureClientIDVar     = "AZURE_CLIENT_ID"
	AzureClientSecretVar = "AZURE_CLIENT_SECRET"

	ShortTestLabel   = "short"
	FullTestLabel    = "full"
	DontRunLabel     = "dontrun"
	VsphereTestLabel = "vsphere"
	KubeadmTestLabel = "kubeadm"
	Rke2TestLabel    = "rke2"

	CapiClusterOwnerLabel          = "cluster-api.cattle.io/capi-cluster-owner"
	CapiClusterOwnerNamespaceLabel = "cluster-api.cattle.io/capi-cluster-owner-ns"
	OwnedLabelName                 = "cluster-api.cattle.io/owned"

	GCPImageIDVar          = "GCP_IMAGE_ID"
	GCPImageIDFormattedVar = "GCP_IMAGE_ID_FORMATTED"
	GCPProjectIDVar        = "GCP_PROJECT"
)

const (
	CAPIVersion = "v1.12.2"
)

const (
	ProvidersChartName = "rancher-turtles-providers"
)
