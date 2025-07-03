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

package controllers

import (
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/cluster-api-operator/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	ctr "sigs.k8s.io/controller-runtime/pkg/controller"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
)

// ProviderHealthCheckReconciler is a health check wrapper for operator provider resources
type ProviderHealthCheckReconciler struct{}

// SetupWithManager is setup manager wrapper for operator healthcheck
func (r *ProviderHealthCheckReconciler) SetupWithManager(mgr ctrl.Manager, options ctr.Options) error {
	return kerrors.NewAggregate([]error{
		(&controller.GenericProviderHealthCheckReconciler{
			Client:   mgr.GetClient(),
			Provider: &operatorv1.CoreProvider{},
		}).SetupWithManager(mgr, options),
		(&controller.GenericProviderHealthCheckReconciler{
			Client:   mgr.GetClient(),
			Provider: &operatorv1.InfrastructureProvider{},
		}).SetupWithManager(mgr, options),
		(&controller.GenericProviderHealthCheckReconciler{
			Client:   mgr.GetClient(),
			Provider: &operatorv1.BootstrapProvider{},
		}).SetupWithManager(mgr, options),
		(&controller.GenericProviderHealthCheckReconciler{
			Client:   mgr.GetClient(),
			Provider: &operatorv1.ControlPlaneProvider{},
		}).SetupWithManager(mgr, options),
		(&controller.GenericProviderHealthCheckReconciler{
			Client:   mgr.GetClient(),
			Provider: &operatorv1.AddonProvider{},
		}).SetupWithManager(mgr, options),
		(&controller.GenericProviderHealthCheckReconciler{
			Client:   mgr.GetClient(),
			Provider: &operatorv1.RuntimeExtensionProvider{},
		}).SetupWithManager(mgr, options),
		(&controller.GenericProviderHealthCheckReconciler{
			Client:   mgr.GetClient(),
			Provider: &operatorv1.IPAMProvider{},
		}).SetupWithManager(mgr, options),
	})
}
