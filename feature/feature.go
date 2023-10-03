package feature

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/component-base/featuregate"
)

const (
	// RancherKubeSecretPatch is used to enable patching of the Rancher v2prov created kubeconfig
	// secrets so that they can be used with CAPI 1.5.x.
	RancherKubeSecretPatch featuregate.Feature = "rancher-kube-secret-patch" //nolint:gosec
)

func init() {
	utilruntime.Must(MutableGates.Add(defaultGates))
}

var defaultGates = map[featuregate.Feature]featuregate.FeatureSpec{
	RancherKubeSecretPatch: {Default: false, PreRelease: featuregate.Beta},
}
