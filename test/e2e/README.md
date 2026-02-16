# E2E tests

This package contains e2e tests for rancher-turtles project. Implementation is based on e2e suite from [cluster-api](https://github.com/kubernetes-sigs/cluster-api/tree/main/test).

## Quickstart

### Prerequisites

Required IAM permissions: <https://eksctl.io/usage/minimum-iam-policies/>

In order to correctly populate provided URL rancher requires to be configured with resolvable server-url setting. In local e2e setup this is handled with [helm based](https://github.com/ngrok/kubernetes-ingress-controller#helm) installation for [ngrok-ingress](https://github.com/ngrok/kubernetes-ingress-controller), using free account.

To setup a free account, the simplest way is to follow their [quickstart](https://ngrok.com/docs/using-ngrok-with/k8s/) guide, specifically steps required to populate these two environment variables:

```bash
export NGROK_AUTHTOKEN=[AUTHTOKEN]
export NGROK_API_KEY=[API_KEY]
```

Then you need to configure the domain name by going into `Dashboard -> Cloud Edge -> Domains` and clicking the `New Domain` button. This will generate a random domain name for the free account. Copy the value and then store it in your environment with:

```bash
export RANCHER_HOSTNAME=<YOUR_DOMAIN_NAME>
```

Now you are ready to start your e2e test run.

**Note:** If you want to run vSphere e2e tests locally, please refer to the instructions provided in the [Running vSphere e2e tests locally](#running-vsphere-e2e-tests-locally) section.

### Workspace Setup

Setup the multi-module workspace, by running the following commands in the turtles root folder.  
This will generate a `turtles/go.work` file to allow linting in tests. Be aware that this may break vendoring process used for `make test` task due to incompatibility between the two.  

```bash
go work init
go work use ./test ./
```

Additionally, you will need to add `e2e` to build tags in your IDE of choice for the imports in the test suite to become resolvable.  
For example in VSCode `settings.json`:  

```json
 "go.buildTags": "e2e",
```

### Running the tests

From the project root directory:

```bash
make test-e2e
```

This will consequently:

1. Install all prerequisite dependencies, like `helm`, `kustomize`, `controller-gen`.
1. Build a docker image with the current repository code using `docker-build-prime` Makefile target.
1. Generate a test release chart containing docker image tag built in the previous step
1. Create the test cluster, run the test suite, cleanup all test resourses.
1. Collect the [artifacts](#artifacts)

### Running the short e2e tests

Tests tagged with the `short` label are verified for any submitted Pull Requests as a minimum requirement.
These tests can be ran locally:

```bash
MANAGEMENT_CLUSTER_ENVIRONMENT=isolated-kind TAG=v0.0.1 GINKGO_LABEL_FILTER=short SOURCE_REPO=https://github.com/your-organization/turtles GITHUB_HEAD_REF=your_feature_branch make test-e2e
```

Note that `SOURCE_REPO` needs to point to your forked repository, and `GITHUB_HEAD_REF` to your feature branch where commits have been pushed already.  

## Providers chart usage in E2E

The E2E suite installs the `rancher-turtles-providers` Helm chart via `test/testenv/providers.go`.

Inputs:

- HELM_BINARY_PATH: Path to the Helm binary used by the installer.
- TURTLES_PROVIDERS_URL: Helm repo URL for the providers chart.
- TURTLES_PROVIDERS_PATH: Local path to the `rancher-turtles-providers` chart.
- TURTLES_PROVIDERS_REPO_NAME: Helm repo name to register.
- TURTLES_PROVIDERS: Comma separated providers to enable on first install (note: this is only specific to e2e and not a helm chart option). Default "all".

Enable providers:

```bash
# Enable everything
export TURTLES_PROVIDERS=all
# OR
# Enable only Azure and AWS
export TURTLES_PROVIDERS=azure,aws
# OR
# Enable CAPD and vSphere
export TURTLES_PROVIDERS=capd,capv
```

If you are testing providers such as azure, aws, or gcp you will need to export the necessary environment variables.
For vsphere, refer to [Running vSphere e2e tests locally](#running-vsphere-e2e-tests-locally) section.

| Provider | Environment Variables                    |
| -------- | ---------------------------------------- |
| GCP      | CAPG_ENCODED_CREDS                       |
| Azure    | AZURE_CLIENT_SECRET                      |
| AWS      | AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY |

### Running vSphere e2e tests locally

#### Prerequisites

1. Initiate a VPN connection using a client such as OpenVPN or Cisco AnyConnect Secure Mobility Client.
1. Set up all the necessary environment variables. You can start with the [`.envrc.example`](../../.envrc.example) file and
  use [direnv](https://direnv.net/) to load these variables into your shell. Rename the file and modify the values as required.
  You can obtain these values from a team common credentials shared location. Also, they can be fetched by accessing the vcenter
  URL and following the steps described on the table below:

| Name                       | Details                                                                                       | How to get
| -------------------------- | ----------------------------------------------------------------------------------------------| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------
| VSPHERE_TLS_THUMBPRINT     | sha1 thumbprint of the vcenter certificate: openssl x509 -sha1 -fingerprint -in ca.crt -noout | browse to vcenter URL and check sha-1 fingerprint in the SSL certificate
| VSPHERE_SERVER             | The vCenter server IP or FQDN | once logged in vSphere Client, click on second icon from left to right on top left and select vcenter name (top level name in the folders tree)
| VSPHERE_DATACENTER         | The vSphere datacenter to deploy the management cluster on | `vcenter` name => select `Datacenters`
| VSPHERE_DATASTORE          | The vSphere datastore to deploy the management cluster on  | `vcenter` name => `Datacenter` name => select `Datastores`
| VSPHERE_FOLDER             | The VM folder for your VMs. Set to "" to use the root vSphere folder | Inventory => Folder =>`vcenter` name => `Datacenter` name =>  select folder
| VSPHERE_TEMPLATE           | The VM template to use for your management cluster | `vcenter` name => `Datacenter` name => `VMs` => select `VM Templates`
| VSPHERE_NETWORK            | The VM network to deploy the management cluster on | Inventory => Network => `vcenter` name => `Datacenter` name => `Switch` name => select distributed port group
| VSPHERE_RESOURCE_POOL      | The resource pool to assign to any VMs created | Inventory => Resource pool => `vcenter` name => `Datacenter` name => `Cluster` name => select resource pool
| VSPHERE_PASSWORD           | The password used to access the remote vSphere endpoint | reach out to team lead
| VSPHERE_USERNAME           | The username used to access the remote vSphere endpoint | reach out to team lead
| VSPHERE_SSH_AUTHORIZED_KEY | The public ssh authorized key on all machines in this cluster | can be left empty
| VSPHERE_KUBE_VIP_IP_KUBEADM  | The IP that kube-vip is going to use as a control plane endpoint | `vcenter` name => `Datacenter` name => `Hosts & Clusters` => `Hosts` => select `host IP` and right click => `Settings` => `Networking` => `Virtual Switches` => click on three dots of `host IP` => `View Settings` => `IPv4 settings` => `Subnet mask`. i.e host IP is 10.10.10.20 and subnet mask is 255.255.255.0, the last IP address of the subnet which is 10.10.10.255 can be used
| VSPHERE_KUBE_VIP_IP_RKE2  | The IP that kube-vip is going to use as a control plane endpoint | `vcenter` name => `Datacenter` name => `Hosts & Clusters` => `Hosts` => select `host IP` and right click => `Settings` => `Networking` => `Virtual Switches` => click on three dots of `host IP` => `View Settings` => `IPv4 settings` => `Subnet mask`. i.e host IP is 10.10.10.20 and subnet mask is 255.255.255.0, the last IP address of the subnet which is 10.10.10.255 can be used
| EXP_CLUSTER_RESOURCE_SET   | This enables the ClusterResourceSet feature that we are using to deploy CSI | default: true
| GOVC_URL                   | The URL of ESXi or vCenter instance to connect to | in the form of `https://<VSPHERE_SERVER>`
| GOVC_USERNAME              | The username to use if not specified in `GOVC_URL` |  equivalent of `VSPHERE_USERNAME`
| GOVC_PASSWORD              | The password to use if not specified in `GOVC_URL` |  equivalent of `VSPHERE_PASSWORD`
| GOVC_INSECURE              | Disable certificate verification | default: true

#### Running the tests

From the project root directory:

```bash
TAG=v0.0.1 GINKGO_LABEL_FILTER=vsphere make test-e2e
```

**Important note:** The vSphere e2e tests require a VPN connection, which makes their integration into the daily e2e CI job challenging. Therefore,
team members should run these tests locally every two weeks. By this, it can ensured that the tests remain functional and up-to-date over the
time.

## Architecture

### E2E config

The config is located in `test/e2e/config/operator.yaml`.

`E2E_CONFIG` env variable is required to point to an existing path, where the config is located.

`variables` section provides the default values for all missing environment variables used by test suite with the same name.

Most notable ones:

```yaml
variables:
  RANCHER_VERSION: "v2.14.0-alpha3" # Default rancher version to install
  RANCHER_HOSTNAME: "localhost" # Your ngrok domain
  NGROK_API_KEY: "" # Key and token values for establishing ingress
  NGROK_AUTHTOKEN: ""
  MANAGEMENT_CLUSTER_ENVIRONMENT: "isolated-kind" # Environment to run the tests in: eks, isolated-kind, kind.
  TURTLES_VERSION: "v0.0.1" # Version of the turtles image to use
  TURTLES_IMAGE: "ghcr.io/rancher/turtles-e2e" # Rancher turtles image to use. It is pre-loaded from local docker registry in kind environment, but expected to be pulled and available in `eks` cluster environment
  ARTIFACTS_FOLDER: "_artifacts" # Folder for the e2e run artifacts collection with crust-gather.
  SECRET_KEYS: "NGROK_AUTHTOKEN,NGROK_API_KEY,RANCHER_HOSTNAME,RANCHER_PASSWORD,CAPG_ENCODED_CREDS,AWS_ACCESS_KEY_ID,AWS_SECRET_ACCESS_KEY,AZURE_SUBSCRIPTION_ID,AZURE_CLIENT_ID,AZURE_CLIENT_SECRET,AZURE_TENANT_ID,GCP_PROJECT,GCP_NETWORK_NAME,VSPHERE_TLS_THUMBPRINT,VSPHERE_SERVER,VSPHERE_DATACENTER,VSPHERE_DATASTORE,VSPHERE_FOLDER,VSPHERE_TEMPLATE,VSPHERE_NETWORK,VSPHERE_RESOURCE_POOL,VSPHERE_USERNAME,VSPHERE_PASSWORD,VSPHERE_KUBE_VIP_IP_KUBEADM,VSPHERE_KUBE_VIP_IP_RKE2,DOCKER_REGISTRY_TOKEN,DOCKER_REGISTRY_USERNAME,DOCKER_REGISTRY_CONFIG" # Is a list of environment variable keys, values of which would be excluded from collected artifacts data.
```

## Artifacts collection

To collect information of e2e failures we use `crust-gather` kubectl plugin installed via `krew`.

To install it outside of the e2e environment you need to [install](https://krew.sigs.k8s.io/docs/user-guide/setup/install/) `krew` and run `kubectl krew install crust-gather`:

```bash
(
  set -x; cd "$(mktemp -d)" &&
  OS="$(uname | tr '[:upper:]' '[:lower:]')" &&
  ARCH="$(uname -m | sed -e 's/x86_64/amd64/' -e 's/\(arm\)\(64\)\?.*/\1\2/' -e 's/aarch64$/arm64/')" &&
  KREW="krew-${OS}_${ARCH}" &&
  curl -fsSLO "https://github.com/kubernetes-sigs/krew/releases/latest/download/${KREW}.tar.gz" &&
  tar zxvf "${KREW}.tar.gz" &&
  ./"${KREW}" install krew
)
kubectl krew install crust-gather
```

To exlude any sensitive information from the collected artifacts, set the `SECRET_KEYS` environment variable accordingly.

## Testdata

For simplicity each test case testdata is stored under `test/e2e/data`. This allows to use it with golang [embed](https://pkg.go.dev/embed) to avoid error checks and mistakes in the path resolution while loading the resources.

Import of the resources can be found in `test/e2e/helpers.go`.

## Testing

While all the tests are based on the combination of [ginkgo](https://github.com/onsi/ginkgo) and [gomega](https://github.com/onsi/gomega), to simplify the process of writing tests the [komega](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest/komega) could be used. However this requires conformity to the `client.Object` inteface for all custom resources `rancher-turtles` is using.

## Cluster configuration

### Kind

[Kind](https://kind.sigs.k8s.io/) is used to set up a cluster for e2e tests. All required components like rancher and rancher-turtles are installed using [helm](https://helm.sh/docs) charts. This option can be enabled by setting `MANAGEMENT_CLUSTER_ENVIRONMENT` to `kind`. It's also required to set `NGROK_API_KEY`, `NGROK_AUTHTOKEN` and `RANCHER_HOSTNAME` environment variables.

### Isolated Kind

This is similar to Kind but instead of public endpoint for Rancher, it uses the internal IP of CP node. This setup can be used to test providers are running in the same network as Rancher. This option can be enabled by setting `MANAGEMENT_CLUSTER_ENVIRONMENT` to `isolated-kind`.

### EKS

EKS is used to set up a cluster for e2e tests. In this setup nginx ingress will be deployed to provide a public endpoint for Rancher. This option can be enabled by setting `MANAGEMENT_CLUSTER_ENVIRONMENT` to `eks`.

### Customizing the cluster

To configure individual components, a series of `server-side-apply` patches are being issued. All required patch manifests are located under `test/e2e/resources/config`. Under circumstances each manifest could have a limited environment based configuration with `envsubst` (for example: setting `RANCHER_HOSTNAME` value in ingress configuration).

Import of the resources could be found in `test/e2e/helpers.go`.

## Artifacts

Artifacts are located under `./_artifacts` directory and is the default location for the stored logs from both workload and child cluster pods collected after each run.

## Cluster and resource cleanup

There are 2 environment variables used to handle cluster cleanup:

1. SKIP_RESOURCE_CLEANUP - Used to decide if management cluster, and supporting charts such as Rancher Turtles should be deleted or retained
2. SKIP_DELETION_TEST - Used to decide if the cluster created during test should be deleted or retained

The following table can help decide how the variables should be used:

| input.SkipDeletionTest | input.SkipCleanup | Rancher Turtles | Git repo & cluster | Management Cluster
|--------|--------|-------------------------------|--------|--------
| true | true | no delete                     | no delete | no delete
| true | false | delete                        | delete | delete
| false | true | no delete                     | delete | no delete
| false | false | delete                        | delete | delete
