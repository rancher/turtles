/*
Copyright © 2026 SUSE LLC

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

package testenv

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/yaml"

	turtlesframework "github.com/rancher/turtles/test/framework"
)

const (
	// rancherVersionAnnotation is the annotation in the Turtles Chart.yaml that defines the supported Rancher versions.
	rancherVersionAnnotation = "catalog.cattle.io/rancher-version"

	// rancherChartName is the name of the Rancher chart in the Helm repository index.
	rancherChartName = "rancher"

	// rancherHeadSuffix is the suffix of the Rancher head chart versions, e.g. 2.16.0-ffe08fd-head.
	rancherHeadSuffix = "-head"
)

// resolvedRancherVersions caches resolved versions per resolution kind and repository URL,
// so all installations in the same test process use the same Rancher version.
var resolvedRancherVersions sync.Map

type helmRepoIndex struct {
	Entries map[string][]helmRepoIndexEntry `json:"entries"`
}

type helmRepoIndexEntry struct {
	Version string    `json:"version"`
	Created time.Time `json:"created"`
}

type turtlesChartMetadata struct {
	Annotations map[string]string `json:"annotations"`
}

// ResolveRancherHeadVersionInput represents the input parameters for resolving the Rancher head version.
type ResolveRancherHeadVersionInput struct {
	// RancherChartURL is the URL of the Helm repository serving Rancher head charts.
	RancherChartURL string `env:"RANCHER_URL"`

	// TurtlesChartMetadataPath is the path to the Turtles Chart.yaml, which defines the supported Rancher versions.
	TurtlesChartMetadataPath string `env:"TURTLES_CHART_METADATA_PATH,expand" envDefault:"${ROOT_DIR}/charts/rancher-turtles/Chart.yaml"`
}

// ResolveRancherHeadVersion returns the newest Rancher head chart version available in the Helm repository
// that satisfies the Rancher version constraint annotated in the Turtles Chart.yaml.
// Candidates are ordered by their release version (major.minor.patch) and, for the same release, by creation
// date, since head chart pre-release identifiers are commit hashes and do not sort chronologically.
// The returned version is prefixed with "v", e.g. v2.16.0-ffe08fd-head.
func ResolveRancherHeadVersion(ctx context.Context, input ResolveRancherHeadVersionInput) string {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for ResolveRancherHeadVersion")
	Expect(input.RancherChartURL).NotTo(BeEmpty(), "RancherChartURL is required for ResolveRancherHeadVersion")
	Expect(input.TurtlesChartMetadataPath).To(BeAnExistingFile(), "TurtlesChartMetadataPath must point to the Turtles Chart.yaml")

	cacheKey := "head/" + input.RancherChartURL
	if version, ok := resolvedRancherVersions.Load(cacheKey); ok {
		return version.(string)
	}

	constraint := turtlesSupportedRancherVersions(input.TurtlesChartMetadataPath)

	By(fmt.Sprintf("Resolving the latest Rancher head version matching %q from %s", constraint, input.RancherChartURL))
	index := fetchHelmRepoIndex(ctx, input.RancherChartURL)

	var (
		latest        *semver.Version
		latestCreated time.Time
	)
	for _, entry := range index.Entries[rancherChartName] {
		if !strings.HasSuffix(entry.Version, rancherHeadSuffix) {
			continue
		}
		version, err := semver.NewVersion(entry.Version)
		if err != nil || !constraint.Check(version) {
			continue
		}
		if latest == nil || cmp.Or(compareRelease(version, latest), entry.Created.Compare(latestCreated)) > 0 {
			latest = version
			latestCreated = entry.Created
		}
	}
	Expect(latest).NotTo(BeNil(), "No Rancher head chart matching %q found in %s", constraint, input.RancherChartURL)

	resolved := "v" + latest.Original()
	GinkgoWriter.Printf("Resolved Rancher head version: %s (created %s)\n", resolved, latestCreated.Format(time.RFC3339))
	resolvedRancherVersions.Store(cacheKey, resolved)

	return resolved
}

// ResolveRancherPreviousStableVersionInput represents the input parameters for resolving the previous stable Rancher version.
type ResolveRancherPreviousStableVersionInput struct {
	// RancherChartURL is the URL of the Helm repository serving stable Rancher charts.
	RancherChartURL string `env:"RANCHER_STABLE_URL"`

	// RancherVersion is the Rancher version the previous minor is computed from.
	// If empty, it is resolved by ResolveRancherHeadVersion.
	RancherVersion string
}

// ResolveRancherPreviousStableVersion returns the newest stable (non pre-release) Rancher chart version available
// in the Helm repository for the minor release preceding RancherVersion. For example, if RancherVersion is
// v2.16.0-ffe08fd-head, it returns the latest v2.15.x release.
// The returned version is prefixed with "v", e.g. v2.15.2.
func ResolveRancherPreviousStableVersion(ctx context.Context, input ResolveRancherPreviousStableVersionInput) string {
	Expect(turtlesframework.Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for ResolveRancherPreviousStableVersion")
	Expect(input.RancherChartURL).NotTo(BeEmpty(), "RancherChartURL is required for ResolveRancherPreviousStableVersion")

	if input.RancherVersion == "" {
		input.RancherVersion = ResolveRancherHeadVersion(ctx, ResolveRancherHeadVersionInput{})
	}

	current, err := semver.NewVersion(input.RancherVersion)
	Expect(err).NotTo(HaveOccurred(), "Failed to parse Rancher version %q", input.RancherVersion)
	Expect(current.Minor()).To(BeNumerically(">", 0), "Can not compute the previous minor of Rancher version %q", input.RancherVersion)
	previousMinor := current.Minor() - 1

	cacheKey := fmt.Sprintf("stable/%d.%d/%s", current.Major(), previousMinor, input.RancherChartURL)
	if version, ok := resolvedRancherVersions.Load(cacheKey); ok {
		return version.(string)
	}

	By(fmt.Sprintf("Resolving the latest stable Rancher v%d.%d.x version from %s", current.Major(), previousMinor, input.RancherChartURL))
	index := fetchHelmRepoIndex(ctx, input.RancherChartURL)

	var latest *semver.Version
	for _, entry := range index.Entries[rancherChartName] {
		version, err := semver.NewVersion(entry.Version)
		if err != nil || version.Prerelease() != "" || version.Major() != current.Major() || version.Minor() != previousMinor {
			continue
		}
		if latest == nil || version.GreaterThan(latest) {
			latest = version
		}
	}
	Expect(latest).NotTo(BeNil(), "No stable Rancher v%d.%d.x chart found in %s", current.Major(), previousMinor, input.RancherChartURL)

	resolved := "v" + latest.Original()
	GinkgoWriter.Printf("Resolved previous stable Rancher version: %s\n", resolved)
	resolvedRancherVersions.Store(cacheKey, resolved)

	return resolved
}

// turtlesSupportedRancherVersions reads the supported Rancher versions constraint from the Turtles Chart.yaml.
func turtlesSupportedRancherVersions(chartMetadataPath string) *semver.Constraints {
	data, err := os.ReadFile(chartMetadataPath)
	Expect(err).NotTo(HaveOccurred(), "Failed to read Turtles chart metadata %s", chartMetadataPath)

	metadata := turtlesChartMetadata{}
	Expect(yaml.Unmarshal(data, &metadata)).To(Succeed(), "Failed to parse Turtles chart metadata %s", chartMetadataPath)

	annotation := metadata.Annotations[rancherVersionAnnotation]
	Expect(annotation).NotTo(BeEmpty(), "Turtles chart metadata %s has no %s annotation", chartMetadataPath, rancherVersionAnnotation)

	constraint, err := semver.NewConstraint(annotation)
	Expect(err).NotTo(HaveOccurred(), "Failed to parse %s annotation %q", rancherVersionAnnotation, annotation)

	return constraint
}

// fetchHelmRepoIndex downloads and parses the index.yaml of a Helm repository.
func fetchHelmRepoIndex(ctx context.Context, repoURL string) *helmRepoIndex {
	indexURL, err := url.JoinPath(repoURL, "index.yaml")
	Expect(err).NotTo(HaveOccurred(), "Failed to build Helm repository index URL from %s", repoURL)

	index := &helmRepoIndex{}
	Eventually(func() error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, indexURL, nil)
		if err != nil {
			return err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code %d fetching %s", resp.StatusCode, indexURL)
		}

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		index = &helmRepoIndex{}
		return yaml.Unmarshal(data, index)
	}, 2*time.Minute, 10*time.Second).Should(Succeed(), "Failed to fetch Helm repository index %s", indexURL)

	return index
}

// compareRelease compares the major, minor and patch numbers of two versions, ignoring pre-release and metadata.
func compareRelease(a, b *semver.Version) int {
	return cmp.Or(
		cmp.Compare(a.Major(), b.Major()),
		cmp.Compare(a.Minor(), b.Minor()),
		cmp.Compare(a.Patch(), b.Patch()),
	)
}
