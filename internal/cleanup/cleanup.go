/*
Copyright © 2023 - 2026 SUSE LLC

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

package cleanup

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
)

// Run dispatches a `manager cleanup <subcommand> ...` invocation.
func Run(ctx context.Context, args []string, scheme *runtime.Scheme) error {
	var (
		capiProviderName      string
		capiProviderNamespace string
		allClusterctlConfigs  bool
		configMapName         string
		configMapNamespace    string
		timeout               time.Duration
	)

	fs := pflag.NewFlagSet("cleanup", pflag.ExitOnError)

	fs.StringVar(&capiProviderName, "capiprovider-name", "", "Name of the CAPIProvider to delete, waiting for its removal. Requires --capiprovider-namespace.") //nolint:lll
	fs.StringVar(&capiProviderNamespace, "capiprovider-namespace", "", "Namespace of the CAPIProvider to delete.")
	fs.BoolVar(&allClusterctlConfigs, "all-clusterctlconfigs", false, "Delete all ClusterctlConfig resources in all namespaces.")
	fs.StringVar(&configMapName, "configmap-name", "", "Name of the ConfigMap to delete. Requires --configmap-namespace.")
	fs.StringVar(&configMapNamespace, "configmap-namespace", "", "Namespace of the ConfigMap to delete.")
	fs.DurationVar(&timeout, "timeout", 5*time.Minute, "Time to wait for the cleanup to complete.")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if capiProviderName == "" && !allClusterctlConfigs && configMapName == "" {
		return errors.New("no resources to clean up, see --help for the available flags")
	}

	if (capiProviderName == "" && capiProviderNamespace != "") || (capiProviderName != "" && capiProviderNamespace == "") {
		return errors.New("--capiprovider-name and --capiprovider-namespace must be set together")
	}

	if (configMapName == "" && configMapNamespace != "") || (configMapName != "" && configMapNamespace == "") {
		return errors.New("--configmap-name and --configmap-namespace must be set together")
	}

	log := ctrl.Log.WithName("cleanup")

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cl, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	if capiProviderName != "" {
		log.Info("Deleting CAPIProvider", "name", capiProviderName, "namespace", capiProviderNamespace)

		provider := &turtlesv1.CAPIProvider{ObjectMeta: metav1.ObjectMeta{
			Name:      capiProviderName,
			Namespace: capiProviderNamespace,
		}}

		if err := deleteAndWait(ctx, cl, provider); err != nil {
			return fmt.Errorf("failed to delete CAPIProvider %s/%s: %w", capiProviderNamespace, capiProviderName, err)
		}
	}

	if allClusterctlConfigs {
		log.Info("Deleting all ClusterctlConfigs")

		configs := &turtlesv1.ClusterctlConfigList{}
		if err := cl.List(ctx, configs); err != nil {
			return fmt.Errorf("failed to list ClusterctlConfigs: %w", err)
		}

		for i := range configs.Items {
			config := &configs.Items[i]
			if err := cl.Delete(ctx, config); client.IgnoreNotFound(err) != nil {
				return fmt.Errorf("failed to delete ClusterctlConfig %s/%s: %w", config.Namespace, config.Name, err)
			}
		}
	}

	if configMapName != "" {
		log.Info("Deleting ConfigMap", "name", configMapName, "namespace", configMapNamespace)

		configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: configMapNamespace,
		}}

		if err := cl.Delete(ctx, configMap); client.IgnoreNotFound(err) != nil {
			return fmt.Errorf("failed to delete ConfigMap %s/%s: %w", configMapNamespace, configMapName, err)
		}
	}

	log.Info("Cleanup completed")

	return nil
}

func deleteAndWait(ctx context.Context, cl client.Client, obj client.Object) error {
	if err := cl.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationForeground)); err != nil {
		return client.IgnoreNotFound(err)
	}

	return wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
		err := cl.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		return false, err
	})
}
