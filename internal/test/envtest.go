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

package test

import (
	"errors"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	managementv3 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/management/v3"
	provisioningv1 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/provisioning/v1"
)

var (
	// FullScheme is a runtime scheme containing full set of used API GVKs.
	FullScheme = runtime.NewScheme()

	// PartialScheme is a runtime scheme containing only set of CAPI and external from API GVKs form Rancher.
	PartialScheme = runtime.NewScheme()

	// RancherScheme is a runtime scheme containing only set of used Rancher API GVKs.
	RancherScheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(FullScheme))
	utilruntime.Must(clusterv1.AddToScheme(FullScheme))
	utilruntime.Must(provisioningv1.AddToScheme(FullScheme))
	utilruntime.Must(managementv3.AddToScheme(FullScheme))

	utilruntime.Must(clientgoscheme.AddToScheme(PartialScheme))
	utilruntime.Must(clusterv1.AddToScheme(PartialScheme))

	utilruntime.Must(clientgoscheme.AddToScheme(RancherScheme))
	utilruntime.Must(provisioningv1.AddToScheme(RancherScheme))
	utilruntime.Must(managementv3.AddToScheme(RancherScheme))
}

// StartEnvTest starts a new test environment.
func StartEnvTest(testEnv *envtest.Environment) (*rest.Config, client.Client, error) {
	cfg, err := testEnv.Start()
	if err != nil {
		return nil, nil, err
	}

	if cfg == nil {
		return nil, nil, errors.New("envtest.Environment.Start() returned nil config")
	}

	cl, err := client.New(cfg, client.Options{Scheme: testEnv.Scheme})
	if err != nil {
		return nil, nil, err
	}

	return cfg, cl, nil
}

// StopEnvTest stops the test environment.
func StopEnvTest(testEnv *envtest.Environment) error {
	return testEnv.Stop()
}
