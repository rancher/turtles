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

## Architecture

### E2E config

The config is located in `test/e2e/config/operator.yaml`.

`variables` section provides the default values for all missing environment variables used by test suite with the same name.

Most notable ones:
```yaml
variables:
  RANCHER_VERSION: "v2.7.5" # Default rancher version to install
  RANCHER_HOSTNAME: "localhost" # Your ngrok domain
  CAPI_INFRASTRUCTURE: "docker" # Default list of capi providers installed in the cluster. Using docker:latest by default. Could be expanded with `docker,azure` to include lates azure provider for example.
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
