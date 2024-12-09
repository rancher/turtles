package testenv

import (
	"context"
	"fmt"
	"os"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rancher/turtles/test/framework"
	turtlesframework "github.com/rancher/turtles/test/framework"
	"k8s.io/apimachinery/pkg/util/version"

	"sigs.k8s.io/cluster-api/test/framework/bootstrap"
)

// NewEKSClusterProvider creates a new instance of EKSClusterProvider.
// It expects the required input parameters to be non-nil.
func NewEKSClusterProvider(Name, Version, region string, NumWorkers int) bootstrap.ClusterProvider {
	Expect(Name).ToNot(BeEmpty(), "Name is required for NewEKSClusterProvider")
	Expect(Version).ToNot(BeEmpty(), "Version is required for NewEKSClusterProvider")
	Expect(NumWorkers).To(BeNumerically(">", 0), "NumWorkers must be greater than 0 for NewEKSClusterProvider")
	Expect(region).ToNot(BeEmpty(), "region is required for NewEKSClusterProvider")

	return &EKSClusterProvider{
		Name:       Name,
		Version:    Version,
		NumWorkers: NumWorkers,
		Region:     region,
	}
}

// EKSClusterProvider represents a provider for managing EKS clusters.
// EKSClusterProvider represents a provider for managing EKS clusters.
type EKSClusterProvider struct {
	// Name of the EKS cluster.
	Name string
	// Version of the EKS cluster.
	Version string
	// region where the EKS cluster is located.
	Region string
	// number of worker nodes in the EKS cluster.
	NumWorkers int
	// path to the kubeconfig file for the EKS cluster.
	KubeconfigPath string
}

// Create creates an EKS cluster using eksctl.
// It creates a temporary file for kubeconfig and writes the EKS kubeconfig to it.
// The cluster is created with the specified Name, Version, number of worker nodes, region, and tags.
// The kubeconfig path is set to the path of the temporary file.
func (k *EKSClusterProvider) Create(ctx context.Context) {
	tempFile, err := os.CreateTemp("", "kubeconfig")
	Expect(err).NotTo(HaveOccurred(), "Failed to create temp file for kubeconfig")
	turtlesframework.Byf("EKS kubeconfig will be written to temp file %s", tempFile.Name())

	eksVersion := VersionToEKS(parseEKSVersion(k.Version))

	turtlesframework.Byf("Creating cluster using eksctl (Version %s)", eksVersion)

	createClusterRes := &turtlesframework.RunCommandResult{}
	numWorkerNodes := strconv.Itoa(k.NumWorkers)
	turtlesframework.RunCommand(ctx, turtlesframework.RunCommandInput{
		Command: "eksctl",
		Args: []string{
			"create",
			"cluster",
			"--name",
			k.Name,
			"--version",
			eksVersion,
			"--nodegroup-name",
			"ng1",
			"--nodes",
			numWorkerNodes,
			"--nodes-min",
			numWorkerNodes,
			"--nodes-max",
			numWorkerNodes,
			"--managed",
			"--region",
			k.Region,
			"--kubeconfig",
			tempFile.Name(),
			"--tags",
			"team=highlander,purpose=e2e",
		},
	}, createClusterRes)
	Expect(createClusterRes.Error).NotTo(HaveOccurred(), "Failed to create cluster using eksctl: %s", createClusterRes.Stderr)
	Expect(createClusterRes.ExitCode).To(Equal(0), "Creating cluster returned non-zero exit code")

	k.KubeconfigPath = tempFile.Name()
}

// GetKubeconfigPath returns the path to the kubeconfig file for the cluster.
func (k *EKSClusterProvider) GetKubeconfigPath() string {
	return k.KubeconfigPath
}

// Dispose the EKS cluster and its kubeconfig file.
func (k *EKSClusterProvider) Dispose(ctx context.Context) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for Dispose")

	By("Deleting cluster using eksctl")

	deleteClusterRes := &turtlesframework.RunCommandResult{}
	turtlesframework.RunCommand(ctx, turtlesframework.RunCommandInput{
		Command: "eksctl",
		Args: []string{
			"delete",
			"cluster",
			"--name",
			k.Name,
			"--wait",
		},
	}, deleteClusterRes)
	Expect(deleteClusterRes.Error).NotTo(HaveOccurred(), "Failed to delete cluster using eksctl")
	Expect(deleteClusterRes.ExitCode).To(Equal(0), "Deleting cluster returned non-zero exit code")

	if err := os.Remove(k.KubeconfigPath); err != nil {
		framework.Byf("Error deleting the kubeconfig file %q file. You may need to remove this by hand.", k.KubeconfigPath)
	}
}

func parseEKSVersion(raw string) *version.Version {
	v := version.MustParseGeneric(raw)
	return version.MustParseGeneric(fmt.Sprintf("%d.%d", v.Major(), v.Minor()))
}

func VersionToEKS(v *version.Version) string {
	return fmt.Sprintf("%d.%d", v.Major(), v.Minor())
}
