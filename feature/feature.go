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

package feature

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// ExternalFleet allows to disable in-tree management of the Fleet clusters
	// in the imported rancher clusters, by setting "provisioning.cattle.io/externally-managed"
	// annotation.
	ExternalFleet featuregate.Feature = "addon-provider-fleet"

	// AgentTLSMode if enabled Turtles will use the agent-tls-mode setting to determine
	// CA cert trust mode for importing clusters.
	AgentTLSMode featuregate.Feature = "agent-tls-mode"

	// UIPlugin if enabled Turtles will install and manage UIPlugin resource for CAPI UI.
	UIPlugin featuregate.Feature = "ui-plugin"
)

func init() {
	utilruntime.Must(MutableGates.Add(defaultGates))
}

var defaultGates = map[featuregate.Feature]featuregate.FeatureSpec{
	ExternalFleet: {Default: true, PreRelease: featuregate.Beta},
	AgentTLSMode:  {Default: false, PreRelease: featuregate.Beta},
	UIPlugin:      {Default: false, PreRelease: featuregate.Alpha},
}
