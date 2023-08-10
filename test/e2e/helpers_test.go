//go:build e2e
// +build e2e

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

package e2e

import (
	"context"
	_ "embed"
)

var (
	ctx = context.Background()

	//go:embed resources/testdata/fleet-capi-test.yaml
	fleetCAPITestdata []byte

	//go:embed resources/config/docker-infra-secret.yaml
	dockerVariablesSecret []byte

	//go:embed resources/config/ingress.yaml
	ingressConfig []byte

	//go:embed resources/config/rancher-service-patch.yaml
	rancherServicePatch []byte

	//go:embed resources/config/ingress-class-patch.yaml
	ingressClassPatch []byte

	//go:embed resources/config/rancher-setting-patch.yaml
	rancherSettingPatch []byte
)

const (
	operatorNamespace       = "capi-operator-system"
	rancherTurtlesNamespace = "rancher-turtles-system"
	rancherNamespace        = "cattle-system"
	capiClusterName         = "test2"
	capiClusterNamespace    = "fleet-default"
)
