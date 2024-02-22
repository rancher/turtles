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

func NewEKSClusterProvider(name, version, region string, numWorkers int) bootstrap.ClusterProvider {
	Expect(name).ToNot(BeEmpty(), "name is required for NewEKSClusterProvider")
	Expect(version).ToNot(BeEmpty(), "version is required for NewEKSClusterProvider")
	Expect(numWorkers).To(BeNumerically(">", 0), "numWorkers must be greater than 0 for NewEKSClusterProvider")
	Expect(region).ToNot(BeEmpty(), "region is required for NewEKSClusterProvider")

	return &EKSClusterProvider{
		name:       name,
		version:    version,
		numWorkers: numWorkers,
		region:     region,
	}
}

type EKSClusterProvider struct {
	name           string
	version        string
	region         string
	numWorkers     int
	kubeconfigPath string
}

// Create a EKS cluster.
func (k *EKSClusterProvider) Create(ctx context.Context) {
	tempFile, err := os.CreateTemp("", "kubeconfig")
	Expect(err).NotTo(HaveOccurred(), "Failed to create temp file for kubeconfig")
	turtlesframework.Byf("EKS kubeconfig will be written to temp file %s", tempFile.Name())

	eksVersion := versionToEKS(parseEKSVersion(k.version))

	turtlesframework.Byf("Creating cluster using eksctl (version %s)", eksVersion)

	createClusterRes := &turtlesframework.RunCommandResult{}
	numWorkerNodes := strconv.Itoa(k.numWorkers)
	turtlesframework.RunCommand(ctx, turtlesframework.RunCommandInput{
		Command: "eksctl",
		Args: []string{
			"create",
			"cluster",
			"--name",
			k.name,
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
			k.region,
			"--kubeconfig",
			tempFile.Name(),
		},
	}, createClusterRes)
	Expect(createClusterRes.Error).NotTo(HaveOccurred(), "Failed to create cluster using eksctl: %s", createClusterRes.Stderr)
	Expect(createClusterRes.ExitCode).To(Equal(0), "Creating cluster returned non-zero exit code")

	k.kubeconfigPath = tempFile.Name()
}

// GetKubeconfigPath returns the path to the kubeconfig file for the cluster.
func (k *EKSClusterProvider) GetKubeconfigPath() string {
	return k.kubeconfigPath
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
			k.name,
		},
	}, deleteClusterRes)
	Expect(deleteClusterRes.Error).NotTo(HaveOccurred(), "Failed to delete cluster using eksctl")
	Expect(deleteClusterRes.ExitCode).To(Equal(0), "Deleting cluster returned non-zero exit code")

	if err := os.Remove(k.kubeconfigPath); err != nil {
		framework.Byf("Error deleting the kubeconfig file %q file. You may need to remove this by hand.", k.kubeconfigPath)
	}
}

func parseEKSVersion(raw string) *version.Version {
	v := version.MustParseGeneric(raw)
	return version.MustParseGeneric(fmt.Sprintf("%d.%d", v.Major(), v.Minor()))
}

func versionToEKS(v *version.Version) string {
	return fmt.Sprintf("%d.%d", v.Major(), v.Minor())
}
