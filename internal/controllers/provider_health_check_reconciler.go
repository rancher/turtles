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
	ctrl "sigs.k8s.io/controller-runtime"
	ctr "sigs.k8s.io/controller-runtime/pkg/controller"

	"sigs.k8s.io/cluster-api-operator/controller"

	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
)

// ProviderHealthCheckReconciler is a health check wrapper for operator provider resources.
type ProviderHealthCheckReconciler struct{}

// SetupWithManager is setup manager wrapper for operator healthcheck.
func (r *ProviderHealthCheckReconciler) SetupWithManager(mgr ctrl.Manager, options ctr.Options) error {
	return kerrors.NewAggregate([]error{
		(&controller.GenericProviderHealthCheckReconciler{
			Client:   mgr.GetClient(),
			Provider: &turtlesv1.CAPIProvider{},
		}).SetupWithManager(mgr, options),
	})
}
