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
	"cmp"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/go-git/go-git/v5"
)

// defaultToCurrentGitRepo retrieves the repository URL and the current branch
func defaultToCurrentGitRepo(input *FleetCreateGitRepoInput) {
	if input.Repo != "" {
		return
	}

	if input.SourceRepo != "" && input.SourceBranch != "" {
		input.Repo = input.SourceRepo
		input.Branch = input.SourceBranch

		// Unset secret name to use public repository
		input.ClientSecretName = ""

		return
	}

	// Open the current repository
	repo, err := git.PlainOpen(cmp.Or(os.Getenv("ROOT_DIR"), "."))
	Expect(err).ShouldNot(HaveOccurred(), fmt.Errorf("failed to open git repo: %w", err).Error())

	// Get remote repository URL
	remotes, err := repo.Remotes()
	Expect(err).ShouldNot(HaveOccurred(), fmt.Errorf("failed to get remotes: %w", err).Error())

	// Find origin remote
	for _, remote := range remotes {
		if remote.Config().Name == "origin" {
			sshURL := remote.Config().URLs[0]

			httpURL := sshURL
			if strings.HasPrefix(sshURL, "git@") {
				parts := strings.SplitN(strings.TrimPrefix(sshURL, "git@"), ":", 2)
				if len(parts) == 2 {
					httpURL = fmt.Sprintf("https://%s/%s", parts[0], parts[1])
				}
			}
			input.Repo = httpURL

			// Get the current branch
			head, err := repo.Head()
			Expect(err).ShouldNot(HaveOccurred(), fmt.Errorf("failed to get HEAD: %w", err).Error())

			input.Branch = head.Name().Short()

			// Unset secret name to use public repository
			input.ClientSecretName = ""
			return
		}
	}
}
