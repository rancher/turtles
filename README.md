# CAPI + Rancher = :cupid:

![image](./cupid.png)

A project looking at various aspects of making `Rancher` :heart: `Cluster API`.

## Documentation

Configuration steps, the quickstart guide and the architecture for this project can be found in our [documentation](https://docs.rancher-turtles.com/).

---

## What is covered in this project?

Currently, this project has the following functionality:

- Automatically import CAPI-created cluster into `Rancher`.
- Install and configure the `Cluster API Operator` project.

## How to use this?

### Quick start

```
Note: The following will only work after we release the first version of the extension.
```

#### Prerequisites:

- Running [Rancher Manager cluster](https://ranchermanager.docs.rancher.com/) with cert-manager
- [Helm](https://helm.sh/)

Additional details on the required `Rancher Manager` cluster setup can be found in [rancher qetting-started](https://docs.rancher-turtles.com/docs/getting-started/rancher) documentation section.

These commands will install:
- `Rancher Turtles` extension
- `CAPI Operator`
- `CAPI` itself with the kubeadm bootstrap and control plane providers.

```bash
helm repo add rancher-turtles https://rancher-sandbox.github.io/rancher-turtles
helm repo update
helm install rancher-turtles rancher-turtles/rancher-turtles --create-namespace -n rancher-turtles-system
```

For additional quickstart details please refer to the `Rancher Turtles` [quickstart](https://docs.rancher-turtles.com/docs/getting-started/install_turtles_operator#install-rancher-turtles-operator-with-cluster-api-operator-as-a-helm-dependency) documentation.

Customizing the deployment:

The `Rancher Turtles` Helm chart supports the following values:

```yaml
rancherTurtles:
  image: controller # image to use for the extension
  tag: v0.0.0 # tag to use for the extension
  imagePullPolicy: Never # image pull policy to use for the extension
  namespace: rancher-turtles-system # namespace to deploy to (default: rancher-turtles-system)
cluster-api-operator: # contains all values passed to the Cluster API Operator helm chart. Full list of values could be found in https://github.com/kubernetes-sigs/cluster-api-operator/blob/main/hack/charts/cluster-api-operator/values.yaml
  enabled: true # indicates if CAPI operator should be installed (default: true)
  cert-manager:
    enabled: true # indicates if cert-manager should be installed (default: true)
  cluster-api:
    enabled: true # indicates if core CAPI controllers should be installed (default: true)
    version: v1.4.6 # version of CAPI to install (default: v1.4.6)
    configSecret: # set the name/namespace of configuration secret. Leave empty unless you want to use your own secret.
      name: "" # name of the config secret to use for core CAPI controllers, used by the CAPI operator. See CAPI operator: https://github.com/kubernetes-sigs/cluster-api-operator/tree/main/docs#installing-azure-infrastructure-provider docs for more details.
      namespace: "" # namespace of the config secret to use for core CAPI controllers, used by the CAPI operator.
      defaultName: "capi-env-variables" # default name for the secret.
    core:
      namespace: capi-system
      fetchConfig: # (only required for airgapped environments)
        url: ""  # url to fetch config from, used by the CAPI operator. See CAPI operator: https://github.com/kubernetes-sigs/cluster-api-operator/tree/main/docs#provider-spec docs for more details.
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

```
#### Installing CAPI providers

The `Rancher Turtles` extension does not install any CAPI providers, you will need to install them yourself using [CAPI operator](https://github.com/kubernetes-sigs/cluster-api-operator/tree/main/docs).

To quickly deploy docker infrastructure, kubeadm bootstrap and control plane providers, apply the following:

```
kubectl apply -f https://raw.githubusercontent.com/rancher-sandbox/rancher-turtles/main/test/e2e/resources/config/capi-providers-secret.yaml
kubectl apply -f https://raw.githubusercontent.com/rancher-sandbox/rancher-turtles/main/test/e2e/resources/config/capi-providers.yaml
```

---

## How to contribute?
See our [contributor guide](CONTRIBUTING.md) for more details on how to get involved.

## Development setup

Details on setting up the development environment can be found [here](./development.md)

## Testing

We are using a combination of unit tests and e2e tests both using ginkgo and gomega frameworks.

To run unit tests, execute:
```sh
make test
```

Detailed documentation on e2e tests architecture and usage can be found [here](./test/e2e/README.md#e2e-tests).

## Code of Conduct

Participation in the project is governed by [Code of Conduct](code-of-conduct.md).
