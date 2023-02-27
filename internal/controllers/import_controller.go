package controllers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/rancher-sandbox/rancher-turtles/internal/patcher"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/controllers/remote"
	"sigs.k8s.io/cluster-api/util/predicates"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/util"

	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/record"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	importLabelName = "rancher-auto-import"
)

var (
	gvkRancherCluster        = schema.GroupVersionKind{Group: "provisioning.cattle.io", Version: "v1", Kind: "Cluster"}
	gvkRancherClusterList    = schema.GroupVersionKind{Group: "provisioning.cattle.io", Version: "v1", Kind: "ClusterList"}
	gvkRancherClusterRegToke = schema.GroupVersionKind{Group: "management.cattle.io", Version: "v3", Kind: "ClusterRegistrationToken"}
)

type CAPIImportReconciler struct {
	client.Client
	recorder         record.EventRecorder
	WatchFilterValue string
	Scheme           *runtime.Scheme

	controller      controller.Controller
	externalTracker external.ObjectTracker
}

// +kubebuilder:rbac:groups=core,resources=events,verbs=get;list;watch;create;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=provisioning.cattle.io,resources=clusters;clusters/status,verbs=get;list;watch;create;update;delete;patch
func (r *CAPIImportReconciler) Reconcile(ctx context.Context, req ctrl.Request) (res ctrl.Result, reterr error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPI cluster")

	capiCluster := &clusterv1.Cluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, capiCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// fetch the rancher clusters
	rancherCluster, err := getRancherClusterByName(ctx, r.Client, capiCluster.Namespace, capiCluster.Name)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{Requeue: true}, err
		}
	}

	if !capiCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, capiCluster, rancherCluster)
	}

	return r.reconcileNormal(ctx, capiCluster, rancherCluster)
}

func (r *CAPIImportReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := log.FromContext(ctx)

	//TODO: we want the control plane initialized but removing for the time being
	//capiPredicates := predicates.All(log, predicates.ClusterControlPlaneInitialized(log), predicates.ResourceHasFilterLabel(log, r.WatchFilterValue))
	capiPredicates := predicates.All(log, predicates.ResourceHasFilterLabel(log, r.WatchFilterValue))

	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&clusterv1.Cluster{}).
		WithOptions(options).
		WithEventFilter(capiPredicates).
		Build(r)
	if err != nil {
		return fmt.Errorf("creating new controller: %w", err)
	}

	// Watch Rancher provisioningv2 clusters
	// NOTE: we will import the types from rancher in the future
	gvk := schema.GroupVersionKind{Group: "provisioning.cattle.io", Version: "v1", Kind: "Cluster"}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)

	err = c.Watch(
		&source.Kind{Type: u},
		handler.EnqueueRequestsFromMapFunc(r.rancherClusterToCapiCluster(ctx)),
		//&handler.EnqueueRequestForOwner{OwnerType: &clusterv1.Cluster{}},
	)
	if err != nil {
		return fmt.Errorf("adding watch for Rancher cluster: %w", err)
	}

	ns := &corev1.Namespace{}
	err = c.Watch(
		&source.Kind{Type: ns},
		handler.EnqueueRequestsFromMapFunc(r.namespaceToCapiClusters(ctx)),
	)
	if err != nil {
		return fmt.Errorf("adding watch for namespaces: %w", err)
	}

	r.recorder = mgr.GetEventRecorderFor("rancher-turtles")
	r.controller = c
	r.externalTracker = external.ObjectTracker{
		Controller: c,
	}

	return nil
}

func (r *CAPIImportReconciler) reconcileNormal(ctx context.Context, capiCluster *clusterv1.Cluster, rancherCluster *unstructured.Unstructured) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPI Cluster")

	if rancherCluster == nil {
		shouldImport, err := r.shouldAutoImport(ctx, capiCluster)
		if err != nil {
			return ctrl.Result{}, err
		}
		if !shouldImport {
			log.Info("not auto importing cluster as namespace isn't marked auto import", "clustername", capiCluster.Name, "namespace", capiCluster.Namespace)
			return ctrl.Result{}, nil
		}

		if err := r.createRancherCluster(ctx, capiCluster); err != nil {
			log.Error(err, "failed creating rancher cluster")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Get the cluster name
	clusterStatus, ok := rancherCluster.Object["status"]
	if !ok {
		log.Info("cluster status not set, requeue")
		return ctrl.Result{Requeue: true}, nil
	}
	clusterName, ok := clusterStatus.(map[string]interface{})["clusterName"]
	if !ok {
		log.Info("clusterName not set, requeue")
		return ctrl.Result{Requeue: true}, nil
	}
	log.Info("found cluster name", "name", clusterName)

	agentDeployed, ok := rancherCluster.Object["status"].(map[string]interface{})["agentDeployed"]
	if ok {
		if agentDeployed.(bool) {
			log.Info("agent already deployed, no action needed")
			return ctrl.Result{}, nil
		}
	}

	// get the registration token
	manifest, err := r.getClusterRegistrationManifest(ctx, clusterName.(string))
	if err != nil {
		return ctrl.Result{}, err
	}
	if manifest == "" {
		log.Info("Import manifest URL not set yet, requeue")
		return ctrl.Result{Requeue: true}, nil
	}

	if applyErr := r.applyImportManifest(ctx, capiCluster, manifest); applyErr != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

func (r *CAPIImportReconciler) shouldAutoImport(ctx context.Context, capiCluster *clusterv1.Cluster) (bool, error) {
	log := log.FromContext(ctx)
	log.V(2).Info("should we auto import the capi cluster", "name", capiCluster.Name, "namespace", capiCluster.Namespace)

	ns := &corev1.Namespace{}
	key := client.ObjectKey{Name: capiCluster.Namespace}
	if err := r.Client.Get(ctx, key, ns); err != nil {
		log.Error(err, "getting namespace")
		return false, err
	}

	labelVal, ok := ns.Labels[importLabelName]
	if !ok {
		log.V(2).Info("Namespace does not have label")
		return false, nil
	}
	autoImport, err := strconv.ParseBool(labelVal)
	if err != nil {
		log.Error(err, fmt.Sprintf("Parsing label value: %s", labelVal))
		return false, err
	}

	return autoImport, nil
}

func (r *CAPIImportReconciler) reconcileDelete(ctx context.Context, capiCluster *clusterv1.Cluster, rancherCluster *unstructured.Unstructured) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling CAPI Cluster Delete")

	//TODO: add implementation

	return ctrl.Result{}, nil
}

func (r *CAPIImportReconciler) getClusterRegistrationManifest(ctx context.Context, clusterName string) (string, error) {
	log := log.FromContext(ctx)

	token, err := r.getClusterRegistrationToken(ctx, clusterName)
	if err != nil {
		return "", err
	}

	manifestUrl, ok := token.Object["status"].(map[string]interface{})["manifestUrl"]
	if !ok {
		return "", nil
	}

	manifestData, err := downloadManifest(manifestUrl.(string))
	if err != nil {
		log.Error(err, "downloading import manifest")
		return "", err
	}

	return manifestData, nil
}

func (r *CAPIImportReconciler) applyImportManifest(ctx context.Context, capiCluster *clusterv1.Cluster, manifest string) error {
	log := log.FromContext(ctx)
	log.Info("Applying import manifest")

	clusterKey := client.ObjectKey{
		Name:      capiCluster.Name,
		Namespace: capiCluster.Namespace,
	}

	restConfig, err := remote.RESTConfig(context.Background(), capiCluster.Name, r.Client, clusterKey)
	if err != nil {
		return fmt.Errorf("getting remote rest config for %s/%s: %w", capiCluster.Namespace, capiCluster.Name, err)
	}
	restConfig.Timeout = 1 * time.Minute

	objs, err := manifestToObjects(strings.NewReader(manifest))
	if err != nil {
		return fmt.Errorf("getting objects from manifest: %w", err)
	}

	for _, obj := range objs {
		u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
		if err != nil {
			return err
		}
		unstructuredObj := &unstructured.Unstructured{}
		unstructuredObj.SetUnstructuredContent(u)

		if createErr := createObject(restConfig, unstructuredObj, &log); createErr != nil {
			return err
		}
	}

	return nil
}

func createObject(cfg *rest.Config, obj *unstructured.Unstructured, log *logr.Logger) error {
	gvk := obj.GroupVersionKind()

	discovery, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return err
	}

	groupResources, err := restmapper.GetAPIGroupResources(discovery)
	if err != nil {
		return err
	}
	rm := restmapper.NewDiscoveryRESTMapper(groupResources)

	restMapping, err := rm.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	//gvr := restMapping.Resource
	gv := gvk.GroupVersion()

	log.Info("Creating REST client")
	restClient, err := newRestClient(*cfg, gv)
	if err != nil {
		return err
	}

	helper := resource.NewHelper(restClient, restMapping)
	setDefaultNamespaceIfScopedAndNoneSet(obj, helper)

	info := &resource.Info{
		Client:          restClient,
		Mapping:         restMapping,
		Namespace:       obj.GetNamespace(),
		Name:            obj.GetName(),
		Source:          "",
		Object:          obj,
		ResourceVersion: restMapping.Resource.Version,
	}

	patcher, err := patcher.New(info, helper)
	if err != nil {
		return err
	}

	modified, err := util.GetModifiedConfiguration(info.Object, true, unstructured.UnstructuredJSONScheme)
	if err != nil {
		return err
	}

	if err := info.Get(); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		// Create the resource if it doesn't exist
		// First, update the annotation used by kubectl apply
		if err := util.CreateApplyAnnotation(info.Object, unstructured.UnstructuredJSONScheme); err != nil {
			return err
		}

		// Then create the resource and skip the three-way merge
		log.Info("creating object", "name", info.Name, "namespace", info.Namespace, "gvk", gvk)
		obj, err := helper.Create(info.Namespace, true, info.Object)
		if err != nil {
			return err
		}
		info.Refresh(obj, true)
	}

	_, patchedObject, err := patcher.Patch(info.Object, modified, info.Namespace, info.Name)
	if err != nil {
		return err
	}

	info.Refresh(patchedObject, true)

	return nil
}

func newRestClient(restConfig rest.Config, gv schema.GroupVersion) (rest.Interface, error) {
	restConfig.ContentConfig = resource.UnstructuredPlusDefaultContentConfig()
	restConfig.GroupVersion = &gv
	if len(gv.Group) == 0 {
		restConfig.APIPath = "/api"
	} else {
		restConfig.APIPath = "/apis"
	}

	return rest.RESTClientFor(&restConfig)
}

func setDefaultNamespaceIfScopedAndNoneSet(u *unstructured.Unstructured, helper *resource.Helper) {
	namespace := u.GetNamespace()
	if helper.NamespaceScoped && namespace == "" {
		namespace = "default"
		u.SetNamespace(namespace)
	}
}

func (r *CAPIImportReconciler) getClusterRegistrationToken(ctx context.Context, clusterName string) (*unstructured.Unstructured, error) {
	token := &unstructured.Unstructured{}
	token.SetGroupVersionKind(gvkRancherClusterRegToke)

	key := client.ObjectKey{Name: "default-token", Namespace: clusterName}

	if err := r.Client.Get(ctx, key, token); err != nil {
		return nil, fmt.Errorf("getting registration token for cluster %s: %w", clusterName, err)
	}

	return token, nil
}

func (r *CAPIImportReconciler) createRancherCluster(ctx context.Context, capiCluster *clusterv1.Cluster) error {
	//TODO: we would not use unstructured in the future, instead import the API definitions from Rancher
	rancherCluster := &unstructured.Unstructured{}
	rancherCluster.SetUnstructuredContent(map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      capiCluster.Name,
			"namespace": capiCluster.Namespace,
		},
		"spec": map[string]interface{}{},
	})
	rancherCluster.SetGroupVersionKind(gvkRancherCluster)

	// ownerRefs := []metav1.OwnerReference{
	// 	{
	// 		Kind:       "Cluster",
	// 		APIVersion: clusterv1.GroupVersion.Identifier(),
	// 		Name:       capiCluster.Name,
	// 		UID:        capiCluster.UID,
	// 	},
	// }
	// rancherCluster.SetOwnerReferences(ownerRefs)

	if err := r.Client.Create(ctx, rancherCluster); err != nil {
		return fmt.Errorf("creating rancher cluster: %w", err)
	}

	return nil
}

func (r *CAPIImportReconciler) rancherClusterToCapiCluster(ctx context.Context) handler.MapFunc {
	log := log.FromContext(ctx)

	return func(o client.Object) []ctrl.Request {
		key := client.ObjectKey{Name: o.GetName(), Namespace: o.GetNamespace()}

		capiCluster := &clusterv1.Cluster{}
		if err := r.Client.Get(ctx, key, capiCluster); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "getting capi cluster")
			}
			return nil
		}

		return []ctrl.Request{{NamespacedName: client.ObjectKey{Namespace: capiCluster.Namespace, Name: capiCluster.Name}}}
	}
}

func (r *CAPIImportReconciler) namespaceToCapiClusters(ctx context.Context) handler.MapFunc {
	log := log.FromContext(ctx)

	return func(o client.Object) []ctrl.Request {
		ns, ok := o.(*corev1.Namespace)
		if !ok {
			log.Error(nil, fmt.Sprintf("Expected a Namespace but got a %T", o))
			return nil
		}

		labelVal, ok := ns.Labels[importLabelName]
		if !ok {
			log.V(2).Info("Namespace does not have label, skipping")
			return nil
		}
		autoImport, err := strconv.ParseBool(labelVal)
		if err != nil {
			log.Error(err, fmt.Sprintf("Parsing label value: %s", labelVal))
			return nil
		}

		if !autoImport {
			log.V(2).Info("Namespace has auto import label set to false, skipping")
			return nil
		}

		capiClusters := &clusterv1.ClusterList{}
		if err := r.Client.List(ctx, capiClusters, client.InNamespace(o.GetNamespace())); err != nil {
			log.Error(err, "getting capi cluster")
			return nil
		}

		if len(capiClusters.Items) == 0 {
			log.V(2).Info("No CAPI clusters in namespace, no action")
			return nil
		}

		reqs := []ctrl.Request{}
		for _, cluster := range capiClusters.Items {
			reqs = append(reqs, ctrl.Request{
				NamespacedName: client.ObjectKey{
					Namespace: cluster.Namespace,
					Name:      cluster.Name,
				},
			})
		}

		return reqs
	}
}

func getRancherClusterByName(ctx context.Context, c client.Client, namespace, name string) (*unstructured.Unstructured, error) {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvkRancherCluster)
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, u); err != nil {
		return nil, err
	}
	return u, nil
}

func downloadManifest(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("downloading manifest: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading manifest: %w", err)
	}

	return string(data), err
}

func manifestToObjects(in io.Reader) ([]runtime.Object, error) {
	var result []runtime.Object
	reader := yamlDecoder.NewYAMLReader(bufio.NewReaderSize(in, 4096))
	for {
		raw, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		obj, err := toObjects(raw)
		if err != nil {
			return nil, err
		}

		result = append(result, obj...)
	}

	return result, nil
}

func toObjects(bytes []byte) ([]runtime.Object, error) {
	bytes, err := yamlDecoder.ToJSON(bytes)
	if err != nil {
		return nil, err
	}

	check := map[string]interface{}{}
	if err := json.Unmarshal(bytes, &check); err != nil || len(check) == 0 {
		return nil, err
	}

	obj, _, err := unstructured.UnstructuredJSONScheme.Decode(bytes, nil, nil)
	if err != nil {
		return nil, err
	}

	if l, ok := obj.(*unstructured.UnstructuredList); ok {
		var result []runtime.Object
		for _, obj := range l.Items {
			copy := obj
			result = append(result, &copy)
		}
		return result, nil
	}

	return []runtime.Object{obj}, nil
}
