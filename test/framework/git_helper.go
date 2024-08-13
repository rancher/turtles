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
	"os"
	"time"

	. "github.com/onsi/gomega"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// GitCloneRepoInput is the input to GitCloneRepo.
type GitCloneRepoInput struct {
	// Address is the URL of the repository to clone.
	Address string

	// CloneLocation is the directory where the repository will be cloned.
	CloneLocation string

	// Username is the username for authentication (optional).
	Username string

	// Password is the password for authentication (optional).
	Password string
}

// GitCloneRepo will clone a repo to a given location.
func GitCloneRepo(ctx context.Context, input GitCloneRepoInput) string {
	Expect(ctx).NotTo(BeNil(), "ctx is required for GitCloneRepo")
	Expect(input.Address).ToNot(BeEmpty(), "Invalid argument. input.Address can't be empty when calling GitCloneRepo")

	cloneDir := input.CloneLocation

	if input.CloneLocation == "" {
		dir, err := os.MkdirTemp("", "turtles-clone")
		Expect(err).ShouldNot(HaveOccurred(), "Failed creating temporary clone directory")
		cloneDir = dir
	}

	opts := &git.CloneOptions{
		URL:      input.Address,
		Progress: os.Stdout,
	}
	if input.Username != "" {
		opts.Auth = &http.BasicAuth{
			Username: input.Username,
			Password: input.Password,
		}
	}

	_, err := git.PlainClone(cloneDir, false, opts)
	Expect(err).ShouldNot(HaveOccurred(), "Failed cloning repo")

	return cloneDir
}

// GitCommitAndPushInput is the input to GitCommitAndPush.
type GitCommitAndPushInput struct {
	// CloneLocation is the directory where the repository is cloned.
	CloneLocation string

	// Username is the username for authentication (optional).
	Username string

	// Password is the password for authentication (optional).
	Password string

	// CommitMessage is the message for the commit.
	CommitMessage string

	// GitPushWait is the wait time for the git push operation.
	GitPushWait []interface{}
}

// GitCommitAndPush will commit the files for a repo and push the changes to the origin.
func GitCommitAndPush(ctx context.Context, input GitCommitAndPushInput) {
	Expect(ctx).NotTo(BeNil(), "ctx is required for GitCommitAndPush")
	Expect(input.CloneLocation).ToNot(BeEmpty(), "Invalid argument. input.CloneLoaction can't be empty when calling GitCommitAndPush")
	Expect(input.CommitMessage).ToNot(BeEmpty(), "Invalid argument. input.CommitMessage can't be empty when calling GitCommitAndPush")

	repo, err := git.PlainOpen(input.CloneLocation)
	Expect(err).ShouldNot(HaveOccurred(), "Failed opening the repo")

	tree, err := repo.Worktree()
	Expect(err).ShouldNot(HaveOccurred(), "Failed getting work tree for repo")

	err = tree.AddWithOptions(&git.AddOptions{
		All: true,
	})
	Expect(err).ShouldNot(HaveOccurred(), "Failed adding all files")

	commitOptions := &git.CommitOptions{
		Author: &object.Signature{
			Name:  "Rancher Turtles Tests",
			Email: "ci@rancher-turtles.com",
			When:  time.Now(),
		},
	}

	_, err = tree.Commit(input.CommitMessage, commitOptions)
	Expect(err).ShouldNot(HaveOccurred(), "Failed to commit files")

	pushOptions := &git.PushOptions{}
	if input.Username != "" {
		pushOptions.Auth = &http.BasicAuth{
			Username: input.Username,
			Password: input.Password,
		}
	}
	err = repo.Push(pushOptions)
	Expect(err).ShouldNot(HaveOccurred(), "Failed pushing changes")

	Eventually(func() error {
		err := repo.Push(pushOptions)
		if err.Error() == "already up-to-date" {
			return nil
		}
		return err
	}, input.GitPushWait...).Should(Succeed(), "Failed to connect to workload cluster using CAPI kubeconfig")
}
