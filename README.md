# CAPI + Rancher = :cupid:

A project looking at various aspects of making Rancher :heart: Cluster API.

## What is covered in this project?

Currently this project has the following functionality:

- Automatically import CAPI created cluster into Rancher

## How to use this?

### Installation

```
Note: The following will only work after we release the first version of the extension.
```

Prerequisites:

- Running [Rancher Manager cluster](https://ranchermanager.docs.rancher.com/) with cert-manager
- [Helm](https://helm.sh/)

Quick start:

These commands will install: Rancher turtles extension, CAPI Operator, CAPI itself with kubeadmin bootstrap and control plane providers.

```bash
helm repo add rancher-turtles https://rancher-sandbox.github.io/rancher-turtles
helm repo update
helm install rancher-turtles rancher-turtles/rancher-turtles --create-namespace -n rancher-turtles-system
```

Customizing the deployment:

The Rancher turtles Helm chart supports the following values:

```yaml
rancherTurtles:
  image: controller # image to use for the extension
  tag: v0.0.0 # tag to use for the extension
  imagePullPolicy: Never # image pull policy to use for the extension
  namespace: rancher-turtles-system # namespace to deploy to (default: rancher-turtles-system)
clusterAPI:
  enabled: true # indicates if core CAPI controllers should be installed (default: true)
  version: v1.4.6 # version of CAPI to install (default: v1.4.6)
  configSecret:
    name: "" # name of the config secret to use for core CAPI controllers, used by the CAPI operator. See [CAPI operator](https://github.com/kubernetes-sigs/cluster-api-operator/tree/main/docs#installing-azure-infrastructure-provider) docs for more details.
    namespace: "" # namespace of the config secret to use for core CAPI controllers, used by the CAPI operator.
  core:
    namespace: capi-system
    fetchConfig: # (only required for airgapped environments)
      url: ""  # url to fetch config from, used by the CAPI operator. See [CAPI operator](https://github.com/kubernetes-sigs/cluster-api-operator/tree/main/docs#provider-spec) docs for more details.
      selector: ""  # selector to use for fetching config, used by the CAPI operator.
  kubeadmBootstrap:
    namespace: capi-kubeadm-bootstrap-system
    fetchConfig:
      url: ""
      selector: ""
  kubeadmControlPlane:
    namespace: capi-kubeadm-control-plane-system
    fetchConfig:
      url: ""
      selector: ""
cluster-api-operator: 
  enabled: true # indicates if CAPI operator should be installed (default: true)

```
### Installing CAPI providers

The Rancher turtles extension does not install any CAPI providers, you will need to install them yourself using [CAPI operator](https://github.com/kubernetes-sigs/cluster-api-operator/tree/main/docs).
 
To quickly deploy docker infrastructure, kubeadm bootstrap and control plane providers, apply the following:

```
kubectl apply -f https://raw.githubusercontent.com/rancher-sandbox/rancher-turtles/main/test/e2e/resources/config/capi-providers-secret.yaml
kubectl apply -f https://raw.githubusercontent.com/rancher-sandbox/rancher-turtles/main/test/e2e/resources/config/capi-providers.yaml
```

## How to contribute?
See our [contributor guide](CONTRIBUTING.md) for more details on how to get involved.

### Development setup

Prerequisites:

- [kind](https://kind.sigs.k8s.io/)
- [helm](https://helm.sh/)
- [clusterctl](https://cluster-api.sigs.k8s.io/user/quick-start.html#install-clusterctl)
- [tilt](https://tilt.dev/)

To create a local development environment:

1. Create **tilt-settings.yaml** like this:

```yaml
{
    "k8s_context": "k3d-rancher-test",
    "default_registry": "ghcr.io/richardcase",
    "debug": {
        "turtles": {
            "continue": true,
            "port": 40000
        }
    }
}
```

2. Open a terminal in the root of the repo
3. Run the following

```bash
make dev-env

# Or if you want to use a custom hostname for Rancher
RANCHER_HOSTNAME=my.customhost.dev make dev-env
```

4. When tilt has started then start ngrok or inlets

```bash
kubectl port-forward --namespace cattle-system svc/rancher 10000:443
ngrok http https://localhost:10000
```

What happens when you run `make dev-env`?

1. A [kind](https://kind.sigs.k8s.io/) cluster is created with the following [configuration](./scripts/kind-cluster-with-extramounts.yaml)
2. [Cert manager](https://cert-manager.io/) is installed on the cluster, we require it for running `Rancher turtes` extension.
3. `clusterctl` is used to bootstrap CAPI components onto the cluster, we use a default configuraion that includes: core Cluster API controller, Kubeadm bootstrap and control plane providers, Docker infrastructure provider.
4. `Rancher manager` is installed using helm.
5. Run `tilt up` to start the development environment.

## Code of Conduct

Participation in the project is governed by [Code of Conduct](code-of-conduct.md).
