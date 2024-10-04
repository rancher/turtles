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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	bootstrapv1 "github.com/rancher/cluster-api-provider-rke2/bootstrap/api/v1beta1"
	k3sv1 "github.com/rancher/turtles/api/rancher/k3s/v1"
	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	provisioningv1 "github.com/rancher/turtles/api/rancher/provisioning/v1"
	snapshotrestorev1 "github.com/rancher/turtles/exp/etcdrestore/api/v1alpha1"
	expcontrollers "github.com/rancher/turtles/exp/etcdrestore/controllers"
	expwebhooks "github.com/rancher/turtles/exp/etcdrestore/webhooks"
	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/controllers/remote"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	ctrlwebhook "sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")

	// flags.
	metricsBindAddr             string
	enableLeaderElection        bool
	leaderElectionLeaseDuration time.Duration
	leaderElectionRenewDeadline time.Duration
	leaderElectionRetryPeriod   time.Duration
	watchFilterValue            string
	profilerAddress             string
	syncPeriod                  time.Duration
	healthAddr                  string
	concurrencyNumber           int
	rancherKubeconfig           string
	insecureSkipVerify          bool
	webhookPort                 int
	webhookCertDir              string
)

func init() {
	klog.InitFlags(nil)

	//+kubebuilder:scaffold:scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(snapshotrestorev1.AddToScheme(scheme))
	utilruntime.Must(bootstrapv1.AddToScheme(scheme))
	utilruntime.Must(k3sv1.AddToScheme(scheme))
	utilruntime.Must(provisioningv1.AddToScheme(scheme))
	utilruntime.Must(managementv3.AddToScheme(scheme))
}

// initFlags initializes the flags.
func initFlags(fs *pflag.FlagSet) {
	fs.StringVar(&metricsBindAddr, "metrics-bind-addr", ":8080",
		"The address the metric endpoint binds to.")

	fs.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")

	fs.DurationVar(&leaderElectionLeaseDuration, "leader-elect-lease-duration", 15*time.Second,
		"Interval at which non-leader candidates will wait to force acquire leadership (duration string)")

	fs.DurationVar(&leaderElectionRenewDeadline, "leader-elect-renew-deadline", 10*time.Second,
		"Duration that the leading controller manager will retry refreshing leadership before giving up (duration string)")

	fs.DurationVar(&leaderElectionRetryPeriod, "leader-elect-retry-period", 2*time.Second,
		"Duration the LeaderElector clients should wait between tries of actions (duration string)")

	fs.StringVar(&watchFilterValue, "watch-filter", "", fmt.Sprintf("Label value that the controller watches to reconcile cluster-api objects. Label key is always %s. If unspecified, the controller watches for all cluster-api objects.", clusterv1.WatchLabel)) //nolint:lll

	fs.StringVar(&profilerAddress, "profiler-address", "",
		"Bind address to expose the pprof profiler (e.g. localhost:6060)")

	fs.DurationVar(&syncPeriod, "sync-period", 10*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")

	fs.StringVar(&healthAddr, "health-addr", ":9440",
		"The address the health endpoint binds to.")

	fs.IntVar(&concurrencyNumber, "concurrency", 1,
		"Number of resources to process simultaneously")

	fs.StringVar(&rancherKubeconfig, "rancher-kubeconfig", "",
		"Path to the Rancher kubeconfig file. Only required if running out-of-cluster.")

	fs.BoolVar(&insecureSkipVerify, "insecure-skip-verify", false,
		"Skip TLS certificate verification when connecting to Rancher. Only used for development and testing purposes. Use at your own risk.")

	fs.IntVar(&webhookPort, "webhook-port", 9443, "Webhook Server port")

	fs.StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs/",
		"Webhook cert dir, only used when webhook-port is specified.")
}

func main() {
	initFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	ctrl.SetLogger(klogr.New())

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsBindAddr,
		},
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "controller-leader-election-etcd-restore",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaseDuration:              &leaderElectionLeaseDuration,
		RenewDeadline:              &leaderElectionRenewDeadline,
		RetryPeriod:                &leaderElectionRetryPeriod,
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.ConfigMap{},
					&corev1.Secret{},
				},
			},
		},
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		HealthProbeBindAddress: healthAddr,
		WebhookServer: ctrlwebhook.NewServer(
			ctrlwebhook.Options{
				Port:    webhookPort,
				CertDir: webhookCertDir,
			},
		),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup the context that's going to be used in controllers and for the manager.
	ctx := ctrl.SetupSignalHandler()

	setupChecks(mgr)
	setupReconcilers(ctx, mgr)
	setupWebhooks(mgr)

	// +kubebuilder:scaffold:builder
	setupLog.Info("starting manager", "version", version.Get().String())

	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupChecks(mgr ctrl.Manager) {
	if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}
}

func setupReconcilers(ctx context.Context, mgr ctrl.Manager) {
	secretCachingClient, err := client.New(mgr.GetConfig(), client.Options{
		HTTPClient: mgr.GetHTTPClient(),
		Cache: &client.CacheOptions{
			Reader: mgr.GetCache(),
		},
	})
	if err != nil {
		setupLog.Error(err, "unable to create secret caching client")
		os.Exit(1)
	}

	// Set up a ClusterCacheTracker and ClusterCacheReconciler to provide to controllers
	// requiring a connection to a remote cluster
	tracker, err := remote.NewClusterCacheTracker(
		mgr,
		remote.ClusterCacheTrackerOptions{
			SecretCachingClient: secretCachingClient,
			ControllerName:      "etcd-restore-controller",
			Log:                 &ctrl.Log,
			Indexes:             []remote.Index{remote.NodeProviderIDIndex},
		},
	)
	if err != nil {
		setupLog.Error(err, "unable to create cluster cache tracker")
		os.Exit(1)
	}

	setupLog.Info("enabling ETCDMachineSnapshot controller")

	if err := (&expcontrollers.ETCDMachineSnapshotReconciler{
		Client:           mgr.GetClient(),
		Tracker:          tracker,
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: concurrencyNumber}); err != nil {
		setupLog.Error(err, "unable to create EtcdMachineSnapshot controller")
		os.Exit(1)
	}

	setupLog.Info("enabling EtcdSnapshotRestore controller")

	if err := (&expcontrollers.ETCDSnapshotRestoreReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: concurrencyNumber}); err != nil {
		setupLog.Error(err, "unable to create EtcdSnapshotRestore controller")
		os.Exit(1)
	}

	setupLog.Info("enabling EtcdSnapshotSync controller")

	if err := (&expcontrollers.EtcdSnapshotSyncReconciler{
		Client:           mgr.GetClient(),
		Tracker:          tracker,
		WatchFilterValue: watchFilterValue,
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: concurrencyNumber}); err != nil {
		setupLog.Error(err, "unable to create EtcdSnapshotSync controller")
		os.Exit(1)
	}

	setupLog.Info("enabling ETCDSnapshotSecret controller")

	if err := (&expcontrollers.ETCDSnapshotSecretReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(ctx, mgr, controller.Options{MaxConcurrentReconciles: concurrencyNumber}); err != nil {
		setupLog.Error(err, "unable to create ETCDSnapshotSecret controller")
		os.Exit(1)
	}
}

func setupWebhooks(mgr ctrl.Manager) {
	if err := (&expwebhooks.RKE2ConfigWebhook{
		Client:                mgr.GetClient(),
		InsecureSkipTLSVerify: insecureSkipVerify,
	}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "RKE2Config")
		os.Exit(1)
	}

	if err := (&expwebhooks.EtcdMachineSnapshotWebhook{
		Client: mgr.GetClient(),
	}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "EtcdMachineSnapshot")
		os.Exit(1)
	}

	if err := (&expwebhooks.EtcdSnapshotRestoreWebhook{
		Client: mgr.GetClient(),
	}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "EtcdSnapshotRestore")
		os.Exit(1)
	}
}
