package setup

import (
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/cluster"

	managementv3 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/management/v3"
	provisioningv1 "github.com/rancher-sandbox/rancher-turtles/internal/rancher/provisioning/v1"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	setupLog      = ctrl.Log.WithName("setup")
	rancherScheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(setupScheme(rancherScheme))
}

func setupScheme(scheme *runtime.Scheme) error {
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		return err
	}
	if err := provisioningv1.AddToScheme(scheme); err != nil {
		return err
	}
	if err := managementv3.AddToScheme(scheme); err != nil {
		return err
	}
	return nil
}

// RancherCluster creates a controller runtime cluster instance
// always connected to the rancher manager cluster.
func RancherCluster(mgr ctrl.Manager, rancherKubeconfig string) (cluster.Cluster, error) {
	if rancherKubeconfig == "" {
		return mgr, setupScheme(mgr.GetScheme())
	}

	config, err := GetConfig(rancherKubeconfig)
	if err != nil {
		return nil, fmt.Errorf("unable to get rest config: %w", err)
	}

	rancherCluster, err := cluster.New(
		config,
		func(clusterOptions *cluster.Options) {
			clusterOptions.Scheme = rancherScheme
			clusterOptions.Cache = cache.Options{
				Scheme: rancherScheme,
			}
		})
	if err != nil {
		return nil, fmt.Errorf("unable to setup rancher cluster: %w", err)
	}

	// Add rancher cluster as a runnable to the manager instance
	// to allow it to be started/stopped with the manager under it's leader election
	if err := mgr.Add(rancherCluster); err != nil {
		return nil, fmt.Errorf("unable to add rancher cluster as runnable: %w", err)
	}

	return rancherCluster, nil
}

// RancherClusterOrDie creates a controller runtime cluster instance
// always connected to the rancher manager cluster.
func RancherClusterOrDie(mgr ctrl.Manager, rancherKubeconfig string) cluster.Cluster {
	cluster, err := RancherCluster(mgr, rancherKubeconfig)
	if err != nil {
		setupLog.Error(err, "unable to add rancher cluster to manager")
		os.Exit(1)
	}

	return cluster
}

// GetConfig loads a REST Config from a path using default kubeconfig context.
func GetConfig(kubeconfigPath string) (*rest.Config, error) {
	kubeconfig, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read rancher kubeconfig from file: %w", err)
	}

	cfg, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("unable to initiate rancher client: %w", err)
	}

	if cfg.QPS == 0.0 {
		cfg.QPS = 20.0
	}

	if cfg.Burst == 0 {
		cfg.Burst = 30
	}

	return cfg, nil
}
