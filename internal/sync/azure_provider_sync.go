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

package sync

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/internal/api"
)

// NewAzureProviderSync creates a new mirror object.
func NewAzureProviderSync(cl client.Client, capiProvider *turtlesv1.CAPIProvider) Sync {
	template := ProviderSync{}.Template(capiProvider)

	destination, ok := template.(api.Provider)
	if !ok || destination == nil {
		return nil
	}

	spec := capiProvider.GetSpec()
	if spec.Deployment == nil {
		spec.Deployment = &operatorv1.DeploymentSpec{}
	}

	var container *operatorv1.ContainerSpec

	for i := range spec.Deployment.Containers {
		if spec.Deployment.Containers[i].Name == "manager" {
			container = &spec.Deployment.Containers[i]
			break
		}
	}

	if container != nil {
		if len(container.Args) == 0 {
			container.Args = map[string]string{
				"--bootstrap-config-gvk": "RKE2Config.v1beta1.bootstrap.cluster.x-k8s.io",
			}
		} else if _, found := container.Args["--bootstrap-config-gvk"]; !found {
			container.Args["--bootstrap-config-gvk"] = "RKE2Config.v1beta1.bootstrap.cluster.x-k8s.io"
		}
	} else {
		spec.Deployment.Containers = append(spec.Deployment.Containers, operatorv1.ContainerSpec{
			Name: "manager",
			Args: map[string]string{
				"--bootstrap-config-gvk": "RKE2Config.v1beta1.bootstrap.cluster.x-k8s.io",
			},
		})
	}

	capiProvider.SetSpec(spec)

	return &ProviderSync{
		DefaultSynchronizer: NewDefaultSynchronizer(cl, capiProvider, template),
		Destination:         destination,
	}
}
