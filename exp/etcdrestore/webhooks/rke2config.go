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

package webhooks

import (
	"context"

	bootstrapv1 "github.com/rancher-sandbox/cluster-api-provider-rke2/bootstrap/api/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/cluster-api/controllers/remote"
)

// RKE2ConfigWebhook defines a webhook for RKE2Config.
type RKE2ConfigWebhook struct {
	client.Client
	Tracker *remote.ClusterCacheTracker
}

var _ webhook.CustomDefaulter = &RKE2ConfigWebhook{}

// SetupWebhookWithManager sets up and registers the webhook with the manager.
func (r *RKE2ConfigWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&bootstrapv1.RKE2Config{}).
		WithDefaulter(r).
		Complete()
}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *RKE2ConfigWebhook) Default(_ context.Context, _ runtime.Object) error {
	return nil
}
