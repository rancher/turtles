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

package framework

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"strings"
	"text/template"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	fleetv1 "github.com/rancher/fleet/pkg/apis/fleet.cattle.io/v1alpha1"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"

	"sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/test/framework"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FleetCreateGitRepoInput represents the input parameters for creating a Git repository in Fleet.
type FleetCreateGitRepoInput struct {
	// Name is the name of the Git repository.
	Name string

	// Namespace is the namespace in which the Git repository will be created.
	Namespace string `envDefault:"fleet-local"`

	// TargetNamespace is the namespace in which the Git repository will apply its content.
	TargetNamespace string

	// TargetClusterNamespace is defining git repo to use cluster namespace as a target namespace by default.
	TargetClusterNamespace bool

	// SourceRepo is the default source URL of the Git repository to use in a CI setting.
	SourceRepo string `env:"SOURCE_REPO"`

	// Repo is the URL of the Git repository.
	Repo string

	// SourceBranch is the default source branch of the Git repository to use in a CI setting.
	SourceBranch string `env:"GITHUB_HEAD_REF"`

	// Branch is the branch of the Git repository to use.
	Branch string `envDefault:"main"`

	// Revision is the specific commit in the Git repository to use.
	Revision string

	// Paths are the paths within the Git repository to sync.
	Paths []string

	// FleetGeneration is the generation of the Fleet instance.
	FleetGeneration int `envDefault:"1"`

	// ClientSecretName is the name of the client secret to use for authentication.
	ClientSecretName string `envDefault:"basic-auth-secret"`

	// ClusterSelectors is a list of optional target selectors. These will override the default target.
	ClusterSelectors []*metav1.LabelSelector

	// ClusterProxy is the ClusterProxy instance for interacting with the cluster.
	ClusterProxy framework.ClusterProxy
}

// FleetCreateGitRepo will create and apply a GitRepo resource to the cluster. See the Fleet docs
// for further information: https://fleet.rancher.io/gitrepo-add
func FleetCreateGitRepo(ctx context.Context, input FleetCreateGitRepoInput) types.NamespacedName {
	Expect(Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	defaultToCurrentGitRepo(&input)

	Expect(ctx).NotTo(BeNil(), "ctx is required for FleetCreateGitRepo")
	Expect(input.Name).ToNot(BeEmpty(), "Invalid argument. input.Name can't be empty when calling FleetCreateGitRepo")
	Expect(input.Repo).ToNot(BeEmpty(), "Invalid argument. input.Repo can't be empty when calling FleetCreateGitRepo")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.Clusterproxy can't be nil when calling FleetCreateGitRepo")
	Expect(input.Paths).ToNot(HaveLen(0), "Invalid argument. input.Paths can't be empty when calling FleetCreateGitRepo")

	Byf("Creating GitRepo from template %s with path %s", input.Name, input.Paths[0])

	input.Name = fmt.Sprintf("%s-%s", input.Name, util.RandomString(6))

	Byf("Ensuring uniqueness of GitRepo by naming it %s with path %s", input.Name, input.Paths[0])

	t := template.New("fleet-repo-template").Funcs(template.FuncMap{
		"toYaml": func(v any) (string, error) {
			data, err := yaml.Marshal(v)
			return string(data), err
		},
		"nindent": func(spaces int, v string) string {
			pad := strings.Repeat(" ", spaces)
			return "\n" + pad + strings.Replace(v, "\n", "\n"+pad, -1)
		},
	})
	t, err := t.Parse(gitRepoTemplate)
	Expect(err).ShouldNot(HaveOccurred(), "Failed to pass GitRepo template")

	var renderedTemplate bytes.Buffer
	err = t.Execute(&renderedTemplate, input)
	Expect(err).NotTo(HaveOccurred(), "Failed to execute template")

	Eventually(func() error {
		Byf("Applying GitRepo: %s", renderedTemplate.String())
		return Apply(ctx, input.ClusterProxy, renderedTemplate.Bytes())
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to apply GitRepo")

	return types.NamespacedName{
		Namespace: input.Namespace,
		Name:      input.Name,
	}
}

// FleetDeleteGitRepoInput represents the input parameters for deleting a Git repository in the fleet.
type FleetDeleteGitRepoInput struct {
	// Name is the name of the Git repository to be deleted.
	Name string

	// Namespace is the namespace of the Git repository to be deleted.
	Namespace string `envDefault:"fleet-local"`

	// ClusterProxy is the cluster proxy used for interacting with the cluster.
	ClusterProxy framework.ClusterProxy
}

// FleetDeleteGitRepo will delete a GitRepo resource from a cluster.
func FleetDeleteGitRepo(ctx context.Context, input FleetDeleteGitRepoInput) {
	Expect(Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for FleetDeleteGitRepoInput")
	Expect(input.Name).ToNot(BeEmpty(), "Invalid argument. input.Name can't be empty when calling FleetDeleteGitRepoInput")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.Clusterproxy can't be nil when calling FleetDeleteGitRepoInput")

	if input.Namespace == "" {
		input.Namespace = DefaultNamespace
	}

	By("Getting GitRepo from cluster")

	gvkGitRepo := schema.GroupVersionKind{Group: "fleet.cattle.io", Version: "v1alpha1", Kind: "GitRepo"}

	repo := &unstructured.Unstructured{}
	repo.SetGroupVersionKind(gvkGitRepo)
	err := input.ClusterProxy.GetClient().Get(ctx, client.ObjectKey{Namespace: input.Namespace, Name: input.Name}, repo)
	if err != nil {
		if apierrors.IsNotFound(err) {
			By("Skipping deletion as GitRepo not found")

			return
		}
		Expect(err).ShouldNot(HaveOccurred(), "Failed getting GitRepo")
	}

	By("Deleting GitRepo from cluster")

	Eventually(func() error {
		return input.ClusterProxy.GetClient().Delete(ctx, repo)
	}, retryableOperationTimeout, retryableOperationInterval).Should(Succeed(), "Failed to delete GitRepo")
}

// FleetWaitForGitRepoInput represents the input parameters for FleetWaitForGitRepo.
type FleetWaitForGitRepoInput struct {
	// Name is the name of the Git repository.
	Name string

	// Namespace is the namespace of the Git repository.
	Namespace string `envDefault:"fleet-local"`

	// ClusterProxy is the cluster proxy used for interacting with the cluster.
	ClusterProxy framework.ClusterProxy

	// WaitInterval is the time interval used to wait for the GitRepo to be Ready.
	WaitInterval []any
}

// FleetWaitForGitRepo waits for the Fleet GitRepo to be Ready.
func FleetWaitForGitRepo(ctx context.Context, input FleetWaitForGitRepoInput) {
	Expect(Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for FleetWaitForGitRepoInput")
	Expect(input.Name).ToNot(BeEmpty(), "Invalid argument. input.Name can't be empty when calling FleetWaitForGitRepoInput")
	Expect(input.ClusterProxy).ToNot(BeNil(), "Invalid argument. input.Clusterproxy can't be nil when calling FleetWaitForGitRepoInput")

	By(fmt.Sprintf("Waiting for GitRepo to be Ready %s/%s", input.Namespace, input.Name))
	Eventually(func() error {
		gitRepo := &fleetv1.GitRepo{}
		if err := input.ClusterProxy.GetClient().Get(ctx, types.NamespacedName{
			Namespace: input.Namespace,
			Name:      input.Name,
		}, gitRepo); err != nil {
			return fmt.Errorf("getting GitRepo: %w", err)
		}

		for _, condition := range gitRepo.Status.Conditions {
			if condition.Type == "Ready" {
				if condition.Status == v1.ConditionTrue {
					return nil
				} else {
					return fmt.Errorf("Ready condition is not True")
				}
			}
		}

		return fmt.Errorf("No Ready Condition found")
	}, input.WaitInterval...).Should(Succeed())
}

// FleetCreateFleetFileInput represents the input parameters for creating a fleet file.
type FleetCreateFleetFileInput struct {
	// Namespace is the namespace in which the fleet file will be created.
	Namespace string

	// FilePath is the file path of the fleet file.
	FilePath string
}

// FleetCreateFleetFile will create a fleet.yaml file in the given location.
// See the Fleet docs for further information: https://fleet.rancher.io/ref-fleet-yaml
func FleetCreateFleetFile(ctx context.Context, input FleetCreateFleetFileInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for FleetCreateFleetFile")
	Expect(input.FilePath).ToNot(BeEmpty(), "Invalid argument. input.Filepath can't be empty when calling FleetCreateFleetFile")

	if input.Namespace == "" {
		input.Namespace = DefaultNamespace
	}

	t := template.New("fleet-file-template")
	t, err := t.Parse(fleetTemplate)
	Expect(err).ShouldNot(HaveOccurred(), "Failed to pass fleet file template")

	f, err := os.OpenFile(input.FilePath, os.O_WRONLY|os.O_CREATE, os.ModePerm)
	Expect(err).ShouldNot(HaveOccurred(), "Failed to create writer for file")

	err = t.Execute(f, input)
	Expect(err).NotTo(HaveOccurred(), "Failed to execute template")
}

const gitRepoTemplate = `
kind: GitRepo
apiVersion: fleet.cattle.io/v1alpha1
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
spec:
  repo: {{ .Repo }}
  {{- if .Branch }}
  branch: {{ .Branch }}
  {{ end -}}
  {{- if .Revision }}
  revision: {{ .Revision }}
  {{ end -}}

  forceSyncGeneration: {{ .FleetGeneration }}

  paths:
  {{ range .Paths }}
  - {{.}}
  {{ end }}

  {{- if .ClusterSelectors }}
  targets:
  {{ range .ClusterSelectors }}
  - clusterSelector: {{ . | toYaml | nindent 6 }}
  {{- end }}
  {{- end }}

  {{- if .ClientSecretName }}
  clientSecretName: {{ .ClientSecretName }}
  {{ end -}}

  {{- if .TargetNamespace }}
  targetNamespace: {{ .TargetNamespace }}
  {{ end -}}
  `

const fleetTemplate = `
namespace: {{ .Namespace }}
defaultNamespace: {{ .Namespace }}
`

// VerifyFleetClusterInput is the input for VerifyFleetCluster.
type VerifyFleetClusterInput struct {
	// Getter is a client.
	Getter client.Client

	// CAPIClusterKey is the key of the CAPI Cluster to import into Fleet.
	CAPIClusterKey types.NamespacedName

	// WaitInterval is the time interval used to wait for the Fleet Cluster to be Ready.
	WaitInterval []any
}

// VerifyFleetCluster runs CAAPF validation.
// This can be used to determine whether a test is failing due to issues with Fleet
// or the fleet-agent installation.
func VerifyFleetCluster(ctx context.Context, input VerifyFleetClusterInput) {
	// Fetch CAPI Cluster
	cluster := &v1beta2.Cluster{}
	Expect(input.Getter.Get(ctx, input.CAPIClusterKey, cluster)).Should(Succeed())

	// Test ClusterClass mapping if this Cluster uses one
	if len(cluster.Spec.Topology.ClassRef.Name) > 0 {
		className := cluster.Spec.Topology.ClassRef.Name
		classNamespace := cmp.Or(cluster.Spec.Topology.ClassRef.Namespace, cluster.Namespace)

		Byf("Verifying Fleet ClusterGroup %s/%s in ClusterClass namespace", classNamespace, className)
		Eventually(func() error {
			clusterGroup := &fleetv1.ClusterGroup{}
			if err := input.Getter.Get(ctx, types.NamespacedName{
				Name: className, Namespace: classNamespace,
			}, clusterGroup); err != nil {
				return fmt.Errorf("getting ClusterGroup: %w", err)
			}
			for _, condition := range clusterGroup.Status.Conditions {
				if condition.Type == "Ready" {
					if condition.Status == v1.ConditionTrue {
						return nil
					} else if clusterGroup.Status.Display.State == "Modified" {
						// Tolerate Modified state as controllers may modify resources after deployment.
						return nil
					}
					return fmt.Errorf("Ready condition is not True")
				}
			}
			return fmt.Errorf("No Ready Condition found")
		}, input.WaitInterval...).Should(Succeed())

		Byf("Verifying Fleet BundleNamespaceMapping %s/%s in ClusterClass namespace", classNamespace, cluster.Namespace)
		Eventually(func() error {
			bundleNamespaceMapping := &fleetv1.BundleNamespaceMapping{}
			return input.Getter.Get(ctx, types.NamespacedName{
				Name: cluster.Namespace, Namespace: classNamespace,
			}, bundleNamespaceMapping)
		}, input.WaitInterval...).Should(Succeed())

		Byf("Verifying Fleet ClusterGroup %s/%s for ClusterClass in Cluster namespace", cluster.Namespace, className+"."+classNamespace)
		Eventually(func() error {
			name := className + "." + classNamespace
			namespace := cluster.Namespace
			clusterGroup := &fleetv1.ClusterGroup{}
			if err := input.Getter.Get(ctx, types.NamespacedName{
				Name: name, Namespace: namespace,
			}, clusterGroup); err != nil {
				return fmt.Errorf("getting ClusterGroup: %w", err)
			}
			ready := false
			for _, condition := range clusterGroup.Status.Conditions {
				if condition.Type == "Ready" {
					if condition.Status == v1.ConditionTrue {
						return nil
					} else if clusterGroup.Status.Display.State == "Modified" {
						// Tolerate Modified state as controllers may modify resources after deployment.
						return nil
					}
					return fmt.Errorf("Ready condition is not True")
				}
			}
			if !ready {
				return fmt.Errorf("No Ready Condition found")
			}

			if clusterGroup.Status.ClusterCount == 1 {
				return nil
			}
			return fmt.Errorf("ClusterGroup clusterCount %d is not 1", clusterGroup.Status.ClusterCount)
		}, input.WaitInterval...).Should(Succeed())
	}

	// Test Fleet Cluster
	Byf("Verifying Fleet Cluster %s/%s", cluster.Namespace, cluster.Name)
	Eventually(func() error {
		cluster := &fleetv1.Cluster{}
		if err := input.Getter.Get(ctx, input.CAPIClusterKey, cluster); err != nil {
			return fmt.Errorf("getting ClusterGroup: %w", err)
		}

		for _, condition := range cluster.Status.Conditions {
			if condition.Type == "Ready" {
				if condition.Status == v1.ConditionTrue {
					return nil
				} else if cluster.Status.Display.State == "Modified" {
					// Tolerate Modified state as controllers may modify resources after deployment.
					return nil
				}
				return fmt.Errorf("Ready condition is not True")
			}
		}

		return fmt.Errorf("No Ready Condition found")
	}, input.WaitInterval...).Should(Succeed())

}
