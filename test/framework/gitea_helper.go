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
	"context"
	"fmt"

	. "github.com/onsi/gomega"

	"code.gitea.io/sdk/gitea"
)

// GiteaCreateRepoInput represents the input parameters for creating a repository in Gitea.
type GiteaCreateRepoInput struct {
	// ServerAddr is the address of the Gitea server.
	ServerAddr string

	// RepoName is the name of the repository to be created.
	RepoName string

	// Username is the username of the user creating the repository.
	Username string `env:"GITEA_USER_NAME"`

	// Password is the password of the user creating the repository.
	Password string `env:"GITEA_USER_PWD"`
}

// GiteaCreateRepo will create a new repo in the Gitea server.
func GiteaCreateRepo(ctx context.Context, input GiteaCreateRepoInput) string {
	Expect(Parse(&input)).To(Succeed(), "Failed to parse environment variables")

	Expect(ctx).NotTo(BeNil(), "ctx is required for GiteaCreateRepo")
	Expect(input.ServerAddr).ToNot(BeEmpty(), "Invalid argument. input.ServerAddr can't be empty when calling GiteaCreateRepo")
	Expect(input.RepoName).ToNot(BeEmpty(), "Invalid argument. input.RepoName can't be empty when calling GiteaCreateRepo")
	Expect(input.Username).ToNot(BeEmpty(), "Invalid argument. input.Username can't be empty when calling GiteaCreateRepo")
	Expect(input.Password).ToNot(BeEmpty(), "Invalid argument. input.Password can't be empty when calling GiteaCreateRepo")

	opts := []gitea.ClientOption{
		gitea.SetBasicAuth(input.Username, input.Password),
		gitea.SetContext(ctx),
	}

	client, err := gitea.NewClient(input.ServerAddr, opts...)
	Expect(err).ShouldNot(HaveOccurred())

	repo, _, err := client.CreateRepo(gitea.CreateRepoOption{
		Name:     input.RepoName,
		AutoInit: true,
	})
	Expect(err).ShouldNot(HaveOccurred())

	return fmt.Sprintf("%s/%s.git", input.ServerAddr, repo.FullName)
}
