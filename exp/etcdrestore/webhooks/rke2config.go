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
	"encoding/json"
	"fmt"
	"strings"

	bootstrapv1 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta1"
	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"sigs.k8s.io/cluster-api/controllers/remote"

	_ "embed"
)

const (
	RKE2ConfigNameLabel       = "cluster-api.cattle.io/rke2config-name"
	planSecretNameLabel       = "cluster-api.cattle.io/plan-secret-name"
	serviceAccountSecretLabel = "cluster-api.cattle.io/service-account.name"
	secretTypeMachinePlan     = "cluster-api.cattle.io/machine-plan"
	defaultFileOwner          = "root:root"
)

const (
	SystemAgentAnnotation = "cluster-api.cattle.io/turtles-system-agent"
)

var (
	//go:embed install.sh
	installSh []byte
)

// +kubebuilder:webhook:path=/mutate-bootstrap-cluster-x-k8s-io-v1beta1-rke2config,mutating=true,failurePolicy=fail,sideEffects=None,groups=bootstrap.cluster.x-k8s.io,resources=rke2configs,verbs=create;update,versions=v1beta1,name=systemagentrke2config.kb.io,admissionReviewVersions=v1

// RKE2ConfigWebhook defines a webhook for RKE2Config.
type RKE2ConfigWebhook struct {
	client.Client
	Tracker               *remote.ClusterCacheTracker
	InsecureSkipTLSVerify bool
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
func (r *RKE2ConfigWebhook) Default(ctx context.Context, obj runtime.Object) error {
	logger := log.FromContext(ctx)

	logger.Info("Configuring system agent on for RKE2Config")

	rke2Config, ok := obj.(*bootstrapv1.RKE2Config)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a RKE2Config but got a %T", obj))
	}

	if rke2Config.Annotations == nil {
		rke2Config.Annotations = map[string]string{}
	}

	rke2Config.Annotations[SystemAgentAnnotation] = ""

	planSecretName := strings.Join([]string{rke2Config.Name, "rke2config", "plan"}, "-")

	if err := r.createSecretPlanResources(ctx, planSecretName, rke2Config); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to create secret plan resources: %s", err))
	}

	serviceAccountToken, err := r.ensureServiceAccountSecretPopulated(ctx, planSecretName)
	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to ensure service account secret is populated: %s", err))
	}

	logger.Info("Service account secret is populated")

	serverUrlSetting := &managementv3.Setting{}

	if err := r.Get(context.Background(), client.ObjectKey{
		Name: "server-url",
	}, serverUrlSetting); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to get server url setting: %s", err))
	}

	serverUrl := serverUrlSetting.Value

	if serverUrl == "" {
		return apierrors.NewBadRequest("server url setting is empty")
	}

	caSetting := &managementv3.Setting{}
	if err := r.Get(context.Background(), client.ObjectKey{
		Name: "cacerts",
	}, caSetting); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to get ca setting: %s", err))
	}

	pem := caSetting.Value

	systemAgentVersionSetting := &managementv3.Setting{}
	if err := r.Get(context.Background(), client.ObjectKey{
		Name: "system-agent-version",
	}, systemAgentVersionSetting); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to get system agent version setting: %s", err))
	}

	systemAgentVersion := systemAgentVersionSetting.Value

	if systemAgentVersion == "" {
		return apierrors.NewBadRequest("system agent version setting is empty")
	}

	if err := r.createConnectInfoJson(ctx, rke2Config, planSecretName, serverUrl, pem, serviceAccountToken); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to create connect info json: %s", err))
	}

	if err := r.createSystemAgentInstallScript(ctx, serverUrl, systemAgentVersion, rke2Config); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to create system agent install script: %s", err))
	}

	if err := r.createConfigYAML(rke2Config); err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to create config.yaml: %s", err))
	}

	r.AddPostInstallCommands(rke2Config)

	return nil
}

// createSecretPlanResources creates the secret, role, rolebinding, and service account for the plan.
func (r *RKE2ConfigWebhook) createSecretPlanResources(ctx context.Context, planSecretName string, rke2Config *bootstrapv1.RKE2Config) error {
	logger := log.FromContext(ctx)

	logger.Info("Creating secret plan resources")

	var errs []error

	resources := []client.Object{
		r.createServiceAccount(planSecretName, rke2Config),
		r.createSecret(planSecretName, rke2Config),
		r.createRole(planSecretName, rke2Config),
		r.createRoleBinding(planSecretName, rke2Config),
		r.createServiceAccountSecret(planSecretName, rke2Config),
	}

	for _, resource := range resources {
		if err := r.Create(ctx, resource); err != nil {
			if !apierrors.IsAlreadyExists(err) {
				errs = append(errs, fmt.Errorf("failed to create %s %s: %w", resource.GetObjectKind().GroupVersionKind().String(), resource.GetName(), err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors occurred during resource creation: %v", errs)
	}

	return nil
}

// createServiceAccount creates a ServiceAccount for the plan.
func (r *RKE2ConfigWebhook) createServiceAccount(planSecretName string, rke2Config *bootstrapv1.RKE2Config) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
			Namespace: rke2Config.Namespace,
			Labels: map[string]string{
				RKE2ConfigNameLabel: rke2Config.Name,
				planSecretNameLabel: planSecretName,
			},
		},
	}
}

// createSecret creates a Secret for the plan.
func (r *RKE2ConfigWebhook) createSecret(planSecretName string, rke2Config *bootstrapv1.RKE2Config) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
			Namespace: rke2Config.Namespace,
			Labels: map[string]string{
				RKE2ConfigNameLabel: rke2Config.Name,
			},
		},
		Type: secretTypeMachinePlan,
	}
}

// createRole creates a Role for the plan.
func (r *RKE2ConfigWebhook) createRole(planSecretName string, rke2Config *bootstrapv1.RKE2Config) *rbacv1.Role {
	return &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
			Namespace: rke2Config.Namespace,
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:         []string{"watch", "get", "update", "list"},
				APIGroups:     []string{""},
				Resources:     []string{"secrets"},
				ResourceNames: []string{planSecretName},
			},
		},
	}
}

// createRoleBinding creates a RoleBinding for the plan.
func (r *RKE2ConfigWebhook) createRoleBinding(planSecretName string, rke2Config *bootstrapv1.RKE2Config) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      planSecretName,
			Namespace: rke2Config.Namespace,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      planSecretName,
				Namespace: rke2Config.Namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     planSecretName,
		},
	}
}

// createServiceAccountSecret creates a Secret for the ServiceAccount token.
func (r *RKE2ConfigWebhook) createServiceAccountSecret(planSecretName string, rke2Config *bootstrapv1.RKE2Config) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-token", planSecretName),
			Namespace: rke2Config.Namespace,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": planSecretName,
			},
			Labels: map[string]string{
				serviceAccountSecretLabel: planSecretName,
			},
		},
		Type: corev1.SecretTypeServiceAccountToken,
	}
}

// ensureServiceAccountSecretPopulated ensures the ServiceAccount secret is populated.
func (r *RKE2ConfigWebhook) ensureServiceAccountSecretPopulated(ctx context.Context, planSecretName string) ([]byte, error) {
	logger := log.FromContext(ctx)

	logger.Info("Ensuring service account secret is populated")

	serviceAccountToken := []byte{}

	if err := retry.OnError(retry.DefaultRetry, func(err error) bool {
		return true
	}, func() error {
		secretList := &corev1.SecretList{}

		if err := r.List(ctx, secretList, client.MatchingLabels{serviceAccountSecretLabel: planSecretName}); err != nil {
			err = fmt.Errorf("failed to list secrets: %w", err)
			logger.Error(err, "failed to list secrets")
			return err
		}

		if len(secretList.Items) == 0 || len(secretList.Items) > 1 {
			err := fmt.Errorf("secret for %s doesn't exist, or more than one secret exists", planSecretName)
			logger.Error(err, "secret for %s doesn't exist, or more than one secret exists", "secret", planSecretName)
			return err
		}

		saSecret := secretList.Items[0]

		if len(saSecret.Data[corev1.ServiceAccountTokenKey]) == 0 {
			err := fmt.Errorf("secret %s not yet populated", planSecretName)
			logger.Error(err, "Secret not yet populated", "secret", planSecretName)
			return err
		}

		serviceAccountToken = saSecret.Data[corev1.ServiceAccountTokenKey]

		return nil
	}); err != nil {
		return nil, err
	}
	return serviceAccountToken, nil
}

// createConnectInfoJson creates the connect-info-config.json file.
func (r *RKE2ConfigWebhook) createConnectInfoJson(ctx context.Context, rke2Config *bootstrapv1.RKE2Config, planSecretName, serverUrl, pem string, serviceAccountToken []byte) error {
	logger := log.FromContext(ctx)

	connectInfoJsonPath := "/etc/rancher/agent/connect-info-config.json"

	filePaths := make(map[string]struct{})
	for _, file := range rke2Config.Spec.Files {
		filePaths[file.Path] = struct{}{}
	}

	if _, exists := filePaths[connectInfoJsonPath]; exists {
		return nil
	}

	kubeConfig := clientcmdapi.Config{
		Clusters: map[string]*clientcmdapi.Cluster{
			"agent": {
				Server:                   serverUrl,
				CertificateAuthorityData: []byte(pem),
			},
		},
		AuthInfos: map[string]*clientcmdapi.AuthInfo{
			"agent": {
				Token: string(serviceAccountToken),
			},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"agent": {
				Cluster:  "agent",
				AuthInfo: "agent",
			},
		},
		CurrentContext: "agent",
	}

	if r.InsecureSkipTLSVerify {
		logger.Info("InsecureSkipTLSVerify is set to true, skipping skip tls verification in kubeconfig")
		kubeConfig.Clusters["agent"].InsecureSkipTLSVerify = true
		kubeConfig.Clusters["agent"].CertificateAuthorityData = nil
	}

	kubeConfigYAML, err := clientcmd.Write(kubeConfig)

	if err != nil {
		return apierrors.NewBadRequest(fmt.Sprintf("failed to write kubeconfig: %s", err))
	}

	connectInfoConfig := struct {
		Namespace  string `json:"namespace"`
		SecretName string `json:"secretName"`
		KubeConfig string `json:"kubeConfig"`
	}{
		Namespace:  rke2Config.Namespace,
		SecretName: planSecretName,
		KubeConfig: string(kubeConfigYAML),
	}

	connectInfoConfigJson, err := json.MarshalIndent(connectInfoConfig, "", " ")
	if err != nil {
		return err
	}

	connectInfoConfigSecretName := fmt.Sprintf("%s-system-agent-connect-info-config", rke2Config.Name)
	connectInfoConfigKey := "connect-info-config.json"

	if err := r.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      connectInfoConfigSecretName,
			Namespace: rke2Config.Namespace,
			Labels: map[string]string{
				RKE2ConfigNameLabel: rke2Config.Name,
			},
		},
		Data: map[string][]byte{
			connectInfoConfigKey: connectInfoConfigJson,
		},
	},
	); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	rke2Config.Spec.Files = append(rke2Config.Spec.Files, bootstrapv1.File{
		Path:        connectInfoJsonPath,
		Owner:       defaultFileOwner,
		Permissions: "0600",
		ContentFrom: &bootstrapv1.FileSource{
			Secret: bootstrapv1.SecretFileSource{
				Name: connectInfoConfigSecretName,
				Key:  connectInfoConfigKey,
			},
		},
	})

	return nil
}

// createSystemAgentInstallScript creates the system-agent-install.sh script.
func (r *RKE2ConfigWebhook) createSystemAgentInstallScript(ctx context.Context, serverUrl, systemAgentVersion string, rke2Config *bootstrapv1.RKE2Config) error {
	systemAgentInstallScriptPath := "/opt/system-agent-install.sh"

	filePaths := make(map[string]struct{})
	for _, file := range rke2Config.Spec.Files {
		filePaths[file.Path] = struct{}{}
	}

	if _, exists := filePaths[systemAgentInstallScriptPath]; exists {
		return nil
	}

	installScriptSecretName := fmt.Sprintf("%s-system-agent-install-script", rke2Config.Name)
	installScriptKey := "install.sh"

	serverUrlBash := fmt.Sprintf("CATTLE_SERVER=%s\n", serverUrl)
	binaryURL := fmt.Sprintf("CATTLE_AGENT_BINARY_BASE_URL=\"%s/assets\"\n", serverUrl)

	if err := r.Create(ctx, &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      installScriptSecretName,
			Namespace: rke2Config.Namespace,
			Labels: map[string]string{
				RKE2ConfigNameLabel: rke2Config.Name,
			},
		},
		Data: map[string][]byte{
			installScriptKey: []byte(fmt.Sprintf("%s%s%s", serverUrlBash, binaryURL, installSh)),
		},
	},
	); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
	}

	rke2Config.Spec.Files = append(rke2Config.Spec.Files, bootstrapv1.File{
		Path:        systemAgentInstallScriptPath,
		Owner:       defaultFileOwner,
		Permissions: "0600",
		ContentFrom: &bootstrapv1.FileSource{
			Secret: bootstrapv1.SecretFileSource{
				Name: installScriptSecretName,
				Key:  installScriptKey,
			},
		},
	})

	return nil
}

// createConfigYAML creates the config.yaml file.
func (r *RKE2ConfigWebhook) createConfigYAML(rke2Config *bootstrapv1.RKE2Config) error {
	configYAMLPath := "/etc/rancher/agent/config.yaml"

	filePaths := make(map[string]struct{})
	for _, file := range rke2Config.Spec.Files {
		filePaths[file.Path] = struct{}{}
	}

	if _, exists := filePaths[configYAMLPath]; exists {
		return nil
	}

	rke2Config.Spec.Files = append(rke2Config.Spec.Files, bootstrapv1.File{
		Path:        configYAMLPath,
		Owner:       defaultFileOwner,
		Permissions: "0600",
		Content: `workDirectory: /var/lib/rancher/agent/work
localPlanDirectory: /var/lib/rancher/agent/plans
interlockDirectory: /var/lib/rancher/agent/interlock
remoteEnabled: true
connectionInfoFile: /etc/rancher/agent/connect-info-config.json
preserveWorkDirectory: true`,
	})

	return nil
}

// addPostInstallCommands adds the post install command to the RKE2Config.
func (r *RKE2ConfigWebhook) AddPostInstallCommands(rke2Config *bootstrapv1.RKE2Config) {
	postInstallCommand := "sh /opt/system-agent-install.sh"

	for _, cmd := range rke2Config.Spec.PostRKE2Commands {
		if cmd == postInstallCommand {
			return
		}
	}
	rke2Config.Spec.PostRKE2Commands = append(rke2Config.Spec.PostRKE2Commands, postInstallCommand)
}
