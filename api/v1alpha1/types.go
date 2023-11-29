package v1alpha1

// ProviderName defines the designated provider infrastructure provider name.
type ProviderName string

const (
	// AWSProvider is the name for the aws provider.
	AWSProvider ProviderName = "aws"
	// AzureProvider is the name for Azure provider.
	AzureProvider ProviderName = "azure"
	// GCPProvider is the name for the GCP provider.
	GCPProvider ProviderName = "gcp"
	// DockerProvider is the name for the docker provider.
	DockerProvider ProviderName = "docker"
	// RKE2Provider is the name for the RKE2 provider.
	RKE2Provider ProviderName = "rke2"
)

// ProviderType defines the type of the CAPI Provider.
type ProviderType string

const (
	// InfrastructureProvider is the name for the infrastructure CAPI Provider.
	InfrastructureProvider ProviderType = "infrastructure"
	// CoreProvider is the name for core CAPI Provider.
	CoreProvider ProviderType = "core"
	// ControlPlaneProvider is the name for the controlPlane CAPI Provider.
	ControlPlaneProvider ProviderType = "controlPlane"
	// BootstrapProvider is the name for the bootstrap CAPI Provider.
	BootstrapProvider ProviderType = "bootstrap"
	// AddonProvider is the name for the addon CAPI Provider.
	AddonProvider ProviderType = "addon"
)

// ProviderState defines the current state of the CAPI Provider resource.
type ProviderState string

const (
	// Pending status identifies a provder which has not yet started provisioning.
	Pending ProviderState = "Pending"
	// Provisioning status defines provider in a provisioning state.
	Provisioning ProviderState = "Provisioning"
	// Ready status identifies that the provider is ready to be used.
	Ready ProviderState = "Ready"
	// Failed status defines a failed state of provider provisioning.
	Failed ProviderState = "Failed"
)
