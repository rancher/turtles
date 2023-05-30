package controllers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/rancher-sandbox/rancher-turtles/internal/controllers/testdata"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	fakeremote "sigs.k8s.io/cluster-api/controllers/remote/fake"
	"sigs.k8s.io/cluster-api/util/secret"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestImportController_NoExistingRancherCluster(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	capiCluster := createCAPICluster("cluster1", "ns1", true)
	markForImport(capiCluster)

	objects := []client.Object{
		createNamespace("ns1"),
		capiCluster,
	}

	c := fake.NewClientBuilder().WithObjects(objects...).Build()

	r := CAPIImportReconciler{
		Client: c,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "cluster1"},
	}
	res, err := r.Reconcile(ctx, req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())

	rancherCluster := &unstructured.Unstructured{}
	rancherCluster.SetGroupVersionKind(gvkRancherCluster)
	err = c.Get(ctx, types.NamespacedName{Namespace: "ns1", Name: "cluster1"}, rancherCluster)
	g.Expect(err).NotTo(HaveOccurred())
}

func TestImportController_ExistingRancherClusterNoStatus(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	capiCluster := createCAPICluster("cluster1", "ns1", true)
	markForImport(capiCluster)
	rancherCluster := createRancherCluster("cluster1", "ns1")

	objects := []client.Object{
		createNamespace("ns1"),
		capiCluster,
		rancherCluster,
	}

	c := fake.NewClientBuilder().WithObjects(objects...).Build()

	r := CAPIImportReconciler{
		Client: c,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "cluster1"},
	}
	res, err := r.Reconcile(ctx, req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())

	var min time.Duration
	g.Expect(res.Requeue).To(BeTrue())
	g.Expect(res.RequeueAfter).To(BeEquivalentTo(min))
}

func TestImportController_ExistingRancherClusterWithStatusNoAgentRegistsred(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	capiCluster := createCAPICluster("cluster1", "ns1", true)
	markForImport(capiCluster)
	rancherCluster := createRancherClusterWithStatus("cluster1", "ns1", false)

	objects := []client.Object{
		createNamespace("ns1"),
		capiCluster,
		rancherCluster,
	}

	c := fake.NewClientBuilder().WithObjects(objects...).Build()

	r := CAPIImportReconciler{
		Client: c,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "cluster1"},
	}
	res, err := r.Reconcile(ctx, req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())

	var min time.Duration
	g.Expect(res.Requeue).To(BeTrue())
	g.Expect(res.RequeueAfter).To(BeEquivalentTo(min))
}

func TestImportController_ExistingRancherClusterWithStatusWithRegTokenNoUrl(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	capiCluster := createCAPICluster("cluster1", "ns1", true)
	markForImport(capiCluster)
	rancherCluster := createRancherClusterWithStatus("cluster1", "ns1", false)
	token := createRegistrationToken("cluster1", "")

	objects := []client.Object{
		createNamespace("ns1"),
		capiCluster,
		rancherCluster,
		token,
	}

	c := fake.NewClientBuilder().WithObjects(objects...).Build()

	r := CAPIImportReconciler{
		Client: c,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "cluster1"},
	}
	res, err := r.Reconcile(ctx, req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())

	var min time.Duration
	g.Expect(res.Requeue).To(BeTrue())
	g.Expect(res.RequeueAfter).To(BeEquivalentTo(min))
}

func TestImportController_ExistingRancherClusterWithStatusWithRegTokenWithUrl(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	capiClusterName := "cluster1"
	rancherClusterName := fmt.Sprintf("%s-capi", capiClusterName)
	namespace := "ns1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testdata.ImportManifest))
	}))
	defer server.Close()

	capiCluster := createCAPICluster(capiClusterName, namespace, true)
	markForImport(capiCluster)
	rancherCluster := createRancherClusterWithStatus(rancherClusterName, namespace, false)
	token := createRegistrationToken(rancherClusterName, server.URL)
	kubeCfgSecret := createKubeconfigSecret(capiClusterName, namespace)

	objects := []client.Object{
		createNamespace(namespace),
		capiCluster,
		rancherCluster,
		token,
		kubeCfgSecret,
	}
	controllerClient := fake.NewClientBuilder().WithObjects(objects...).Build()

	r := CAPIImportReconciler{
		Client:             controllerClient,
		remoteClientGetter: fakeremote.NewClusterClient,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "cluster1"},
	}
	res, err := r.Reconcile(ctx, req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())

	var min time.Duration
	g.Expect(res.Requeue).To(BeFalse())
	g.Expect(res.RequeueAfter).To(BeEquivalentTo(min))

	deploymentKey := types.NamespacedName{
		Name:      "cattle-cluster-agent",
		Namespace: "cattle-system",
	}
	deployment := &appsv1.Deployment{}
	err = controllerClient.Get(ctx, deploymentKey, deployment)
	g.Expect(err).NotTo(HaveOccurred())
	fmt.Println(deployment.Status.ObservedGeneration)

	res, err = r.Reconcile(ctx, req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())
}

func TestImportController_ExistingRancherClusterWithStatusWithAgentDeployedNotStatus(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	capiClusterName := "cluster1"
	rancherClusterName := fmt.Sprintf("%s-capi", capiClusterName)
	namespace := "ns1"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testdata.ImportManifest))
	}))
	defer server.Close()

	capiCluster := createCAPICluster(capiClusterName, namespace, true)
	markForImport(capiCluster)
	rancherCluster := createRancherClusterWithStatus(rancherClusterName, namespace, false)
	token := createRegistrationToken(rancherClusterName, server.URL)
	kubeCfgSecret := createKubeconfigSecret(capiClusterName, namespace)
	agentNamespace := createNamespace("cattle-system")
	agentDeployment := createDeployment("cattle-cluster-agent", "cattle-system")

	objects := []client.Object{
		createNamespace(namespace),
		capiCluster,
		rancherCluster,
		token,
		kubeCfgSecret,
		agentNamespace,
		agentDeployment,
	}
	controllerClient := fake.NewClientBuilder().WithObjects(objects...).Build()

	r := CAPIImportReconciler{
		Client:             controllerClient,
		remoteClientGetter: fakeremote.NewClusterClient,
	}

	req := reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: "ns1", Name: "cluster1"},
	}
	res, err := r.Reconcile(ctx, req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())

	var min time.Duration
	g.Expect(res.Requeue).To(BeFalse())
	g.Expect(res.RequeueAfter).To(BeEquivalentTo(min))

	deploymentKey := types.NamespacedName{
		Name:      "cattle-cluster-agent",
		Namespace: "cattle-system",
	}
	deployment := &appsv1.Deployment{}
	err = controllerClient.Get(ctx, deploymentKey, deployment)
	g.Expect(err).NotTo(HaveOccurred())
	fmt.Println(deployment.Status.ObservedGeneration)

	res, err = r.Reconcile(ctx, req)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(res).NotTo(BeNil())
}

func createCAPICluster(name, namespace string, controlPlaneReady bool) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: v1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: clusterv1.ClusterSpec{},
		Status: clusterv1.ClusterStatus{
			ControlPlaneReady: controlPlaneReady,
		},
	}
}

func createRancherCluster(name, namespace string) *unstructured.Unstructured {
	cluster := &unstructured.Unstructured{}
	cluster.SetGroupVersionKind(gvkRancherCluster)
	cluster.SetName(name)
	cluster.SetNamespace(namespace)
	return cluster
}

func createRancherClusterWithStatus(name, namespace string, agentDeployed bool) *unstructured.Unstructured {
	cluster := createRancherCluster(name, namespace)

	status := map[string]interface{}{}
	status["clusterName"] = name

	if agentDeployed {
		status["agentDeployed"] = true
	}

	cluster.Object["status"] = status

	return cluster
}

func createRegistrationToken(clusterName string, manifestUrl string) *unstructured.Unstructured {
	regToken := &unstructured.Unstructured{}
	regToken.SetGroupVersionKind(gvkRancherClusterRegToke)
	regToken.SetName("default-token")
	regToken.SetNamespace(clusterName)

	status := map[string]interface{}{}
	if manifestUrl != "" {
		status["manifestUrl"] = manifestUrl
	}
	regToken.Object["status"] = status

	return regToken
}

func createNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: v1.ObjectMeta{
			Name: name,
		},
	}
}

func markForImport(obj metav1.Object) {
	a := obj.GetAnnotations()
	if a == nil {
		a = map[string]string{}
	}
	a[importAnnotationName] = "true"
	obj.SetAnnotations(a)
}

func createKubeconfigSecret(clusterName, clusterNamespace string) *corev1.Secret {
	validKubeConfig := `
clusters:
- cluster:
    server: https://testcluster.com:6443
  name: test-cluster-api
contexts:
- context:
    cluster: test-cluster-api
    user: kubernetes-admin
  name: kubernetes-admin@test-cluster-api
current-context: kubernetes-admin@test-cluster-api
kind: Config
preferences: {}
users:
-
`
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-kubeconfig", clusterName),
			Namespace: clusterNamespace,
		},
		Data: map[string][]byte{
			secret.KubeconfigDataName: []byte(validKubeConfig),
		},
	}
}

func createDeployment(name, namespace string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec:   appsv1.DeploymentSpec{},
		Status: appsv1.DeploymentStatus{},
	}
}
