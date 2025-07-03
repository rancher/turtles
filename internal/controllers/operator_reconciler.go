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
	"context"

	"sigs.k8s.io/cluster-api-operator/controller"
	ctrl "sigs.k8s.io/controller-runtime"
	ctr "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
)

// OperatorReconciler is a mapping wrapper for CAPIProvider -> operator provider resources
type OperatorReconciler struct{}

// SetupWithManager is a mapping wrapper for CAPIProvider -> operator provider resources
func (r *OperatorReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options ctr.Options) error {
	log := log.FromContext(ctx)

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.CoreProvider{},
			ProviderList:             &operatorv1.CoreProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
		}}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "CoreProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.InfrastructureProvider{},
			ProviderList:             &operatorv1.InfrastructureProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		}}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "InfrastructureProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.BootstrapProvider{},
			ProviderList:             &operatorv1.BootstrapProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		}}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "BootstrapProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.ControlPlaneProvider{},
			ProviderList:             &operatorv1.ControlPlaneProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		}}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "ControlPlaneProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.AddonProvider{},
			ProviderList:             &operatorv1.AddonProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		}}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "AddonProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.IPAMProvider{},
			ProviderList:             &operatorv1.IPAMProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		}}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "IPAMProvider")
		return err
	}

	if err := (&CAPIProviderReconcilerWrapper{
		GenericProviderReconciler: controller.GenericProviderReconciler{
			Provider:                 &operatorv1.RuntimeExtensionProvider{},
			ProviderList:             &operatorv1.RuntimeExtensionProviderList{},
			Client:                   mgr.GetClient(),
			Config:                   mgr.GetConfig(),
			WatchConfigSecretChanges: true,
			WatchCoreProviderChanges: true,
		}}).SetupWithManager(ctx, mgr, options); err != nil {
		log.Error(err, "unable to create controller", "controller", "RuntimeExtensionProvider")
		return err
	}

	return nil
}

//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=turtles-capi.cattle.io,resources=capiproviders/finalizers,verbs=update

// CAPIProviderReconcilerWrapper wraps the upstream CAPIProviderReconciler
type CAPIProviderReconcilerWrapper struct {
	controller.GenericProviderReconciler
}

// BuildWithManager builds the CAPIProviderReconciler
func (r *CAPIProviderReconcilerWrapper) BuildWithManager(ctx context.Context, mgr ctrl.Manager) (*ctrl.Builder, error) {
	builder, err := r.GenericProviderReconciler.BuildWithManager(ctx, mgr)
	if err != nil {
		return nil, err
	}

	reconciler := controller.NewPhaseReconciler(r.GenericProviderReconciler, r.Provider, r.ProviderList)

	r.GenericProviderReconciler.ReconcilePhases = []controller.PhaseFn{
		r.setDefaultProviderSpec,
		reconciler.ApplyFromCache,
		reconciler.PreflightChecks,
		reconciler.InitializePhaseReconciler,
		reconciler.DownloadManifests,
		reconciler.Load,
		reconciler.Fetch,
		reconciler.Store,
		reconciler.Upgrade,
		reconciler.Install,
		reconciler.ReportStatus,
		reconciler.Finalize,
	}

	return builder, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *CAPIProviderReconcilerWrapper) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options ctr.Options) error {
	builder, err := r.BuildWithManager(ctx, mgr)
	if err != nil {
		return err
	}

	return builder.WithOptions(options).Complete(r)
}

func (r *CAPIProviderReconcilerWrapper) Reconcile(ctx context.Context, req reconcile.Request) (_ reconcile.Result, reterr error) {
	return r.GenericProviderReconciler.Reconcile(ctx, req)
}

func (r *CAPIProviderReconcilerWrapper) setDefaultProviderSpec(ctx context.Context) (*controller.Result, error) {
	setDefaultProviderSpec(r.Provider)

	return &controller.Result{}, nil
}

// setDefaultProviderSpec sets the default values for the provider spec.
func setDefaultProviderSpec(o operatorv1.GenericProvider) {
	providerSpec := o.GetSpec()
	providerNamespace := o.GetNamespace()

	if providerSpec.ConfigSecret != nil && providerSpec.ConfigSecret.Namespace == "" {
		providerSpec.ConfigSecret.Namespace = providerNamespace
	}

	if providerSpec.AdditionalManifestsRef != nil && providerSpec.AdditionalManifestsRef.Namespace == "" {
		providerSpec.AdditionalManifestsRef.Namespace = providerNamespace
	}

	o.SetSpec(providerSpec)
}
