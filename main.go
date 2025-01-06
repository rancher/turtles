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
	"cmp"
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/component-base/version"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/textlogger"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"

	operatorv1 "sigs.k8s.io/cluster-api-operator/api/v1alpha2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"

	managementv3 "github.com/rancher/turtles/api/rancher/management/v3"
	provisioningv1 "github.com/rancher/turtles/api/rancher/provisioning/v1"
	turtlesv1 "github.com/rancher/turtles/api/v1alpha1"
	"github.com/rancher/turtles/feature"
	"github.com/rancher/turtles/internal/controllers"
)

const maxDuration time.Duration = 1<<63 - 1

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
)

func init() {
	klog.InitFlags(nil)

	//+kubebuilder:scaffold:scheme
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme))
	utilruntime.Must(provisioningv1.AddToScheme(scheme))
	utilruntime.Must(managementv3.AddToScheme(scheme))
	utilruntime.Must(operatorv1.AddToScheme(scheme))
	utilruntime.Must(turtlesv1.AddToScheme(scheme))
	turtlesv1.AddKnownTypes(scheme)
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

	fs.DurationVar(&syncPeriod, "sync-period", 2*time.Minute,
		"The minimum interval at which watched resources are reconciled (e.g. 15m)")

	fs.StringVar(&healthAddr, "health-addr", ":9440",
		"The address the health endpoint binds to.")

	fs.IntVar(&concurrencyNumber, "concurrency", 1,
		"Number of resources to process simultaneously")

	fs.StringVar(&rancherKubeconfig, "rancher-kubeconfig", "",
		"Path to the Rancher kubeconfig file. Only required if running out-of-cluster.")

	fs.BoolVar(&insecureSkipVerify, "insecure-skip-verify", false,
		"Skip TLS certificate verification when connecting to Rancher. Only used for development and testing purposes. Use at your own risk.")

	feature.MutableGates.AddFlag(fs)
}

func main() {
	initFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	ctrl.SetLogger(textlogger.NewLogger(textlogger.NewConfig()))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: server.Options{
			BindAddress: metricsBindAddr,
		},
		LeaderElection:             enableLeaderElection,
		LeaderElectionID:           "controller-leader-election-rancher-turtles",
		LeaderElectionResourceLock: resourcelock.LeasesResourceLock,
		LeaseDuration:              &leaderElectionLeaseDuration,
		RenewDeadline:              &leaderElectionRenewDeadline,
		RetryPeriod:                &leaderElectionRetryPeriod,
		Client: client.Options{
			Cache: &client.CacheOptions{
				DisableFor: []client.Object{
					&corev1.ConfigMap{},
					&corev1.Secret{},
					&turtlesv1.ClusterctlConfig{},
				},
			},
		},
		Cache: cache.Options{
			SyncPeriod: &syncPeriod,
		},
		HealthProbeBindAddress: healthAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Setup the context that's going to be used in controllers and for the manager.
	ctx := ctrl.SetupSignalHandler()

	setupChecks(mgr)
	setupReconcilers(ctx, mgr)

	// +kubebuilder:scaffold:builder
	setupLog.Info("starting manager", "version", version.Get().String())

	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupChecks(mgr ctrl.Manager) {
	if err := mgr.AddReadyzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("ping", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}
}

func setupReconcilers(ctx context.Context, mgr ctrl.Manager) {
	rancherClient, err := setupRancherClient(client.Options{Scheme: mgr.GetClient().Scheme()})
	if err != nil {
		setupLog.Error(err, "failed to create client")
		os.Exit(1)
	}

	rancherClient = cmp.Or(rancherClient, mgr.GetClient())

	options := client.Options{
		Scheme: mgr.GetClient().Scheme(),
		Cache: &client.CacheOptions{
			DisableFor: []client.Object{
				&clusterv1.Cluster{},
			},
		},
	}

	uncachedClient, err := setupRancherClient(options)
	if err != nil {
		setupLog.Error(err, "failed to create uncached rancher client")
		os.Exit(1)
	}

	if uncachedClient == nil {
		cl, err := client.New(mgr.GetConfig(), options)
		if err != nil {
			setupLog.Error(err, "failed to create uncached rancher client (same cluster)")
			os.Exit(1)
		}

		uncachedClient = cl
	}

	if err := (&controllers.CAPIImportManagementV3Reconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		UncachedClient:     uncachedClient,
		RancherClient:      rancherClient,
		WatchFilterValue:   watchFilterValue,
		InsecureSkipVerify: insecureSkipVerify,
	}).SetupWithManager(ctx, mgr, controller.Options{
		MaxConcurrentReconciles: concurrencyNumber,
		CacheSyncTimeout:        maxDuration,
	}); err != nil {
		setupLog.Error(err, "unable to create capi controller")
		os.Exit(1)
	}

	if err := (&controllers.CAPICleanupReconciler{
		RancherClient: rancherClient,
	}).SetupWithManager(ctx, mgr, controller.Options{
		MaxConcurrentReconciles: concurrencyNumber,
		CacheSyncTimeout:        maxDuration,
	}); err != nil {
		setupLog.Error(err, "unable to create rancher management v3 cleanup controller")
		os.Exit(1)
	}

	if feature.Gates.Enabled(feature.RancherKubeSecretPatch) {
		setupLog.Info("enabling Rancher kubeconfig secret patching")

		if err := (&controllers.RancherKubeconfigSecretReconciler{
			Client:           mgr.GetClient(),
			Scheme:           mgr.GetScheme(),
			WatchFilterValue: watchFilterValue,
		}).SetupWithManager(ctx, mgr, controller.Options{
			MaxConcurrentReconciles: concurrencyNumber,
			CacheSyncTimeout:        maxDuration,
		}); err != nil {
			setupLog.Error(err, "unable to create Rancher kubeconfig secret controller")
			os.Exit(1)
		}
	}

	setupLog.Info("enabling Clusterctl Config synchronization controller")

	if err := (&controllers.ClusterctlConfigReconciler{
		Client: mgr.GetClient(),
	}).SetupWithManager(ctx, mgr, controller.Options{
		MaxConcurrentReconciles: concurrencyNumber,
		CacheSyncTimeout:        maxDuration,
	}); err != nil {
		setupLog.Error(err, "unable to create ClusterctlConfig controller")
		os.Exit(1)
	}

	setupLog.Info("enabling CAPI Operator synchronization controller")

	if err := (&controllers.CAPIProviderReconciler{
		Client: mgr.GetClient(),
		Scheme: scheme,
	}).SetupWithManager(ctx, mgr, controller.Options{
		MaxConcurrentReconciles: concurrencyNumber,
		CacheSyncTimeout:        maxDuration,
	}); err != nil {
		setupLog.Error(err, "unable to create CAPI Provider controller")
		os.Exit(1)
	}
}

// setupRancherClient can either create a client for an in-cluster installation (rancher and rancher-turtles in the same cluster)
// or create a client for an out-of-cluster installation (rancher and rancher-turtles in different clusters) based on the
// existence of Rancher kubeconfig file.
func setupRancherClient(options client.Options) (client.Client, error) {
	if len(rancherKubeconfig) > 0 {
		setupLog.Info("out-of-cluster installation of rancher-turtles", "using kubeconfig from path", rancherKubeconfig)

		restConfig, err := loadConfigWithContext("", &clientcmd.ClientConfigLoadingRules{ExplicitPath: rancherKubeconfig}, "")
		if err != nil {
			return nil, fmt.Errorf("unable to load kubeconfig from file: %w", err)
		}

		rancherClient, err := client.New(restConfig, options)
		if err != nil {
			return nil, err
		}

		return rancherClient, nil
	}

	setupLog.Info("in-cluster installation of rancher-turtles")

	return nil, nil
}

// loadConfigWithContext loads a REST Config from a path using a logic similar to the one used in controller-runtime.
func loadConfigWithContext(apiServerURL string, loader clientcmd.ClientConfigLoader, context string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loader,
		&clientcmd.ConfigOverrides{
			ClusterInfo: clientcmdapi.Cluster{
				Server: apiServerURL,
			},
			CurrentContext: context,
		}).ClientConfig()
}
