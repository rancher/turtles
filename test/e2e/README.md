# E2E tests

This package contains e2e tests for rancher-turtles project. Implementation is based on e2e suite from [cluster-api](https://github.com/kubernetes-sigs/cluster-api/tree/main/test).

## Quickstart

### Prerequisites

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

Additionally, you could reuse [`go.work.example`](../../go.work.example) and copy it into `rancher-turtles/../go.work` to allow linting in tests. Be aware that this may break vendoring process used for `make test` task due to incompatibility between the two. You will need to add `e2e` to build tags in your IDE of choice for the imports in the test suite to become resolvable.

### Running the tests

From the project root directory:
```bash
make test-e2e
```

This will consequently:
1. Build a docker image with the current repository code using `docker-build` Makefile target.
2. Generate a test release chart containing docker image tag built in the previous step
3. Install all prerequisite dependencies, like `helm`, `kubectl>=v1.27.0`, download `cluster-api-operator` helm release file from pre-specified URL.
4. Create the test cluster, run the test suite, cleanup all test resourses.
5. Collect the [artifacts](#artifacts)

### Running vSphere e2e tests locally

#### Prerequisites

1. Initiate a VPN connection using a client such as OpenVPN or Cisco AnyConnect Secure Mobility Client.
1. Set up all the necessary environment variables. You can start with the [`.envrc.example`](../../.envrc.example) file and
  use [direnv](https://direnv.net/) to load these variables into your shell. Rename the file and modify the values as required.
  You can obtain these values from a team common credentials shared location. Also, they can be fetched by accessing the vcenter
  URL and following the steps described on the table below:

| Name                       | Details                                                                                       | How to get                                                                     
| -------------------------- | ----------------------------------------------------------------------------------------------| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| VSPHERE_TLS_THUMBPRINT     | sha1 thumbprint of the vcenter certificate: openssl x509 -sha1 -fingerprint -in ca.crt -noout | browse to vcenter URL and check sha-1 fingerprint in the SSL certificate |
| VSPHERE_SERVER             | The vCenter server IP or FQDN | once logged in vSphere Client, click on second icon from left to right on top left and select vcenter name (top level name in the folders tree) |
| VSPHERE_DATACENTER         | The vSphere datacenter to deploy the management cluster on | `vcenter` name => select `Datacenters` |
| VSPHERE_DATASTORE          | The vSphere datastore to deploy the management cluster on  | `vcenter` name => `Datacenter` name => select `Datastores` |
| VSPHERE_FOLDER             | The VM folder for your VMs. Set to "" to use the root vSphere folder | `vcenter` name => `Datacenter` name => `VMs` => select `VM folders` |
| VSPHERE_TEMPLATE           | The VM template to use for your management cluster | `vcenter` name => `Datacenter` name => `VMs` => select `VM Templates` |
| VSPHERE_NETWORK            | The VM network to deploy the management cluster on | `vcenter` name => `Datacenter` name => `Networks` => select `Networks` |
| VSPHERE_PASSWORD           | The password used to access the remote vSphere endpoint | reach out to team lead |
| VSPHERE_USERNAME           | The username used to access the remote vSphere endpoint | reach out to team lead |
| VSPHERE_SSH_AUTHORIZED_KEY | The public ssh authorized key on all machines in this cluster | can be left empty |
| CONTROL_PLANE_ENDPOINT_IP  | The IP that kube-vip is going to use as a control plane endpoint | `vcenter` name => `Datacenter` name => `Hosts & Clusters` => `Hosts` => select `host IP` and right click => `Settings` => `Networking` => `Virtual Switches` => click on three dots of `host IP` => `View Settings` => `IPv4 settings` => `Subnet mask`. i.e host IP is 10.10.10.20 and subnet mask is 255.255.255.0, the last IP address of the subnet which is 10.10.10.255 can be used |
| EXP_CLUSTER_RESOURCE_SET   | This enables the ClusterResourceSet feature that we are using to deploy CSI | default: true |
| GOVC_URL                   | The URL of ESXi or vCenter instance to connect to | in the form of `https://<VSPHERE_SERVER>` |
| GOVC_USERNAME              | The username to use if not specified in `GOVC_URL` |  equivalent of `VSPHERE_USERNAME` |
| GOVC_PASSWORD              | The password to use if not specified in `GOVC_URL` |  equivalent of `VSPHERE_PASSWORD` |
| GOVC_INSECURE              | Disable certificate verification | default: true |

#### Running the tests

From the project root directory:

```bash
GINKGO_LABEL_FILTER=local make test-e2e
```

**Important note:** The vSphere e2e tests require a VPN connection, which makes their integration into the daily e2e CI job challenging. Therefore,
team members should run these tests locally every two weeks. By this, it can ensured that the tests remain functional and up-to-date over the
time.

## Architecture

### E2E config

The config is located in `test/e2e/config/operator.yaml`.

`variables` section provides the default values for all missing environment variables used by test suite with the same name.

Most notable ones:
```yaml
variables:
  RANCHER_VERSION: "v2.7.5" # Default rancher version to install
  RANCHER_HOSTNAME: "localhost" # Your ngrok domain
  NGROK_API_KEY: "" # Key and token values for establishing ingress
  NGROK_AUTHTOKEN: ""
```

## Testdata

For simplicity each test case testdata is stored under `test/e2e/resources/testdata`. This allows to use it with golang [embed](https://pkg.go.dev/embed) to avoid error checks and mistakes in the path resolution while loading the resources.

Import of the resources can be found in `test/e2e/helpers_test.go`.

## Testing

While all the tests are based on the combination of [ginkgo](https://github.com/onsi/ginkgo) and [gomega](https://github.com/onsi/gomega), to simplify the process of writing tests the [komega](https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/envtest/komega) could be used. However this requires conformity to the `client.Object` inteface for all custom resources `rancher-turtles` is using.

## Cluster configuration

[Kind](https://kind.sigs.k8s.io/) is used to set up a cluster for e2e tests. All required components like rancher, rancher-turtles and [cluster-api-operator](https://github.com/kubernetes-sigs/cluster-api-operator) (which provisions cluster-api with required providers) are installed using [helm](https://kind.sigs.k8s.io/) charts.

To configure individual components, a series of `server-side-apply` patches are being issued. All required patch manifests are located under `test/e2e/resources/config`. Under circumstances each manifest could have a limited environment based configuration with `envsubst` (for example: setting `RANCHER_HOSTNAME` value in ingress configuration).

Import of the resources could be found in `test/e2e/helpers_test.go`.

## Artifacts

Artifacts are located under `./_artifacts` directory and is the default location for the stored logs from both workload and child cluster pods collected after each run.
