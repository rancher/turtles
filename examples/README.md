# ClusterClass examples

This directory contains example `ClusterClass` definitions and the supporting applications they rely on. They are plain Kubernetes manifests and can be applied with `kubectl`.

## Layout

```text
examples/
├── clusterclasses/<provider>/<flavor>/   # one ClusterClass per directory, e.g. azure/rke2, docker/kubeadm
├── applications/<type>/<name>/           # CNI, CCM, CSI and load balancer add-ons
└── clusters/                             # example Cluster definitions using the ClusterClasses
```

## Applying a ClusterClass

From a local checkout of the repository:

```bash
kubectl apply -f examples/clusterclasses/azure/rke2/
```

Without cloning the repository, point `kubectl` at the file for a specific release tag:

```bash
kubectl apply -f https://raw.githubusercontent.com/rancher/turtles/<tag>/examples/clusterclasses/azure/rke2/clusterclass-rke2-example.yaml
```

To apply into a namespace other than the current one, add `-n <namespace>`.

## Applying the supporting applications

Some ClusterClasses need additional applications, such as a CNI or a cloud provider. Apply only the ones that match the ClusterClass you are using, in the namespace where the `Cluster` will be created.

For example, to apply the Docker RKE2 ClusterClass with its load balancer configuration from a local checkout:

```bash
kubectl apply -n <namespace> \
  -f examples/clusterclasses/docker/rke2/ \
  -f examples/applications/lb/docker/
```

Or from a release tag:

```bash
BASE=https://raw.githubusercontent.com/rancher/turtles/<tag>/examples
kubectl apply -n <namespace> \
  -f ${BASE}/clusterclasses/docker/rke2/clusterclass-docker-rke2.yaml \
  -f ${BASE}/applications/lb/docker/configmap.yaml
```

## Creating a cluster

`clusters/` contains example `Cluster` definitions that use these ClusterClasses, e.g. `clusters/docker/rke2/cluster.yaml`. Review and adjust them (namespace, labels, versions) before applying.
