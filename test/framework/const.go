/*
Copyright 2023 SUSE.

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

const (
	// DefaultNamespace is the name of the default Kubernetes namespace.
	DefaultNamespace = "default"
	// DefaultBranchName is the name of the default git branch.
	DefaultBranchName = "main"
	// FleetLocalNamespace is the name of the namespace used for local cluster by Fleet.
	FleetLocalNamespace = "fleet-local"
	// MagicDNS is the dns name to use in isolated mode
	MagicDNS = "sslip.io"
	// DefaulRancherTurtlesNamespace is the name of the default namespace for Rancher Turtles.
	DefaultRancherTurtlesNamespace = "rancher-turtles-system"
)
