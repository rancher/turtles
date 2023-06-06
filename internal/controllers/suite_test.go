package controllers

import (
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

func init() {
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))
	//utilruntime.Must(infrav1.AddToScheme(scheme.Scheme))
}
