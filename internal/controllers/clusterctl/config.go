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

package clusterctl

import (
	"cmp"
	"os"

	_ "embed"

	corev1 "k8s.io/api/core/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/yaml"
)

var (
	//go:embed config.yaml
	configDefault []byte

	config *corev1.ConfigMap
)

func init() {
	utilruntime.Must(yaml.UnmarshalStrict(configDefault, &config))
}

// Config returns current set of turtles clusterctl overrides.
func Config() *corev1.ConfigMap {
	configMap := config.DeepCopy()
	configMap.Namespace = cmp.Or(os.Getenv("POD_NAMESPACE"), "rancher-turtles-system")

	return configMap
}
