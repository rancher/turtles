# CAPI + Rancher = :cupid:

A project looking at various aspects of making Rancher :heart: Cluster API.

## What is covered in this project?

Currently this project has the following functionality:

- Automatically import CAPI-created cluster into Rancher

## How to use this?

Instructions coming soon :)

## How to contribute?

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
