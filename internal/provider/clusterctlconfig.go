/*
Copyright © 2023 - 2025 SUSE LLC

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

package provider

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"

	"sigs.k8s.io/cluster-api-operator/controller"

	"github.com/rancher/turtles/internal/controllers/clusterctl"
)

const (
	waitForClusterctlConfigDuration = 10 * time.Second
)

// WaitForClusterctlConfigUpdate is a phase that waits for the clusterctl-config Configmap
// mounted in `/config/clusterctl.yaml` to be updated with the intended content.
// This should contain the base embedded in-memory ConfigMap, with overrides
// from the user defined ClusterctlConfig, if any.
// It may take a few minutes for the changes to take effect.
// We need to wait since the cluster-api-operator library is going to use the mounted file
// to deploy providers, therefore we need it to be synced with embedded and user overrides.
func WaitForClusterctlConfigUpdate(ctx context.Context, client client.Client) (*controller.Result, error) {
	logger := log.FromContext(ctx)

	// Load the mounted config from filesystem
	configBytes, err := os.ReadFile(clusterctl.ConfigPath)
	if os.IsNotExist(err) {
		logger.Info("ClusterctlConfig is not initialized yet, waiting for mounted ConfigMap to be updated.")
		return &controller.Result{RequeueAfter: waitForClusterctlConfigDuration}, nil
	} else if err != nil {
		return &controller.Result{}, fmt.Errorf("reading %s file: %w", clusterctl.ConfigPath, err)
	}

	// Get the expected config with user overrides
	config, err := clusterctl.ClusterConfig(ctx, client)
	if err != nil {
		return &controller.Result{}, fmt.Errorf("getting updated ClusterctlConfig: %w", err)
	}

	// Compare the filesystem config with the expected one
	clusterctlYaml, err := yaml.Marshal(config)
	if err != nil {
		return &controller.Result{}, fmt.Errorf("serializing updated ClusterctlConfig: %w", err)
	}

	synced := bytes.Equal(clusterctlYaml, configBytes)

	if !synced {
		logger.Info("ClusterctlConfig is not synced yet, waiting for mounted ConfigMap to be updated.")
		return &controller.Result{RequeueAfter: waitForClusterctlConfigDuration}, nil
	}

	return &controller.Result{}, nil
}
