# Copyright © 2023 - 2024 SUSE LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

#
# Go.
#
ifeq ($(shell go env GOARCH),amd64)
GO_ARCH = x86_64
else
GO_ARCH = arm64
endif

ifeq ($(shell go env GOOS),linux)
UPDATECLI_OS = Linux
else
UPDATECLI_OS = Darwin
endif

GO_VERSION ?= $(shell grep "go " go.mod | head -1 |awk '{print $$NF}')
GO_CONTAINER_IMAGE ?= docker.io/library/golang:$(GO_VERSION)
REPO ?= rancher/turtles

CAPI_VERSION ?= $(shell grep "sigs.k8s.io/cluster-api" go.mod | head -1 |awk '{print $$NF}')
CAPI_UPSTREAM_REPO ?= https://github.com/kubernetes-sigs/cluster-api
CAPI_UPSTREAM_RELEASES ?= $(CAPI_UPSTREAM_REPO)/releases

# Use GOPROXY environment variable if set
GOPROXY := $(shell go env GOPROXY)
ifeq ($(GOPROXY),)
GOPROXY := https://proxy.golang.org
endif
export GOPROXY

# Active module mode, as we use go modules to manage dependencies
export GO111MODULE=on

# This option is for running docker manifest command
export DOCKER_CLI_EXPERIMENTAL := enabled

CURL_RETRIES=3

# Directories
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))
BIN_DIR := bin
TEST_DIR := test
TOOLS_DIR := hack/tools
TOOLS_BIN_DIR := $(abspath $(TOOLS_DIR)/$(BIN_DIR))

$(TOOLS_BIN_DIR):
	mkdir -p $@

export PATH := $(abspath $(TOOLS_BIN_DIR)):$(PATH)
export KREW_ROOT := $(abspath $(TOOLS_BIN_DIR))
export PATH := $(KREW_ROOT)/bin:$(PATH)

# Set --output-base for conversion-gen if we are not within GOPATH
ifneq ($(abspath $(ROOT_DIR)),$(shell go env GOPATH)/src/github.com/rancher/turtles)
	CONVERSION_GEN_OUTPUT_BASE := --output-base=$(ROOT_DIR)
else
	export GOPATH := $(shell go env GOPATH)
endif


#
# Ginkgo configuration.
#
GINKGO_FOCUS ?=
GINKGO_SKIP ?=
GINKGO_NODES ?= 1
GINKGO_TIMEOUT ?= 2h
GINKGO_POLL_PROGRESS_AFTER ?= 60m
GINKGO_POLL_PROGRESS_INTERVAL ?= 5m
E2E_CONFIG ?= $(ROOT_DIR)/$(TEST_DIR)/e2e/config/operator.yaml
GINKGO_ARGS ?=
SKIP_RESOURCE_CLEANUP ?= false
USE_EXISTING_CLUSTER ?= false
GINKGO_NOCOLOR ?= false
GINKGO_LABEL_FILTER ?= short
TURTLES_PROVIDERS ?= ALL
GINKGO_TESTS ?= $(ROOT_DIR)/$(TEST_DIR)/e2e/suites/...

MANAGEMENT_CLUSTER_ENVIRONMENT ?= eks

export ARTIFACTS_FOLDER ?= ${ROOT_DIR}/_artifacts
export ARTIFACTS ?= ${ARTIFACTS_FOLDER}

# to set multiple ginkgo skip flags, if any
ifneq ($(strip $(GINKGO_SKIP)),)
_SKIP_ARGS := $(foreach arg,$(strip $(GINKGO_SKIP)),-skip="$(arg)")
endif

# Helper function to get dependency version from go.mod
get_go_version = $(shell go list -m $1 | awk '{print $$2}')

GO_INSTALL := ./scripts/go-install.sh

# Binaries
KUSTOMIZE_VER := v5.7.0
KUSTOMIZE_BIN := kustomize
KUSTOMIZE := $(abspath $(TOOLS_BIN_DIR)/$(KUSTOMIZE_BIN)-$(KUSTOMIZE_VER))
KUSTOMIZE_PKG := sigs.k8s.io/kustomize/kustomize/v5

SETUP_ENVTEST_VER := release-0.21
SETUP_ENVTEST_BIN := setup-envtest
SETUP_ENVTEST := $(abspath $(TOOLS_BIN_DIR)/$(SETUP_ENVTEST_BIN)-$(SETUP_ENVTEST_VER))
SETUP_ENVTEST_PKG := sigs.k8s.io/controller-runtime/tools/setup-envtest

CONTROLLER_GEN_VER := v0.18.0
CONTROLLER_GEN_BIN := controller-gen
CONTROLLER_GEN := $(abspath $(TOOLS_BIN_DIR)/$(CONTROLLER_GEN_BIN)-$(CONTROLLER_GEN_VER))
CONTROLLER_GEN_PKG := sigs.k8s.io/controller-tools/cmd/controller-gen

CONVERSION_GEN_VER := v0.33.0
CONVERSION_GEN_BIN := conversion-gen
# We are intentionally using the binary without version suffix, to avoid the version
# in generated files.
CONVERSION_GEN := $(abspath $(TOOLS_BIN_DIR)/$(CONVERSION_GEN_BIN))
CONVERSION_GEN_PKG := k8s.io/code-generator/cmd/conversion-gen

ENVSUBST_VER := v2.0.0-20210730161058-179042472c46
ENVSUBST_BIN := envsubst
ENVSUBST := $(abspath $(TOOLS_BIN_DIR)/$(ENVSUBST_BIN)-$(ENVSUBST_VER))
ENVSUBST_PKG := github.com/drone/envsubst/v2/cmd/envsubst

GO_APIDIFF_VER := v0.8.3
GO_APIDIFF_BIN := go-apidiff
GO_APIDIFF := $(abspath $(TOOLS_BIN_DIR)/$(GO_APIDIFF_BIN)-$(GO_APIDIFF_VER))
GO_APIDIFF_PKG := github.com/joelanford/go-apidiff

GINKGO_BIN := ginkgo
GINGKO_VER := $(call get_go_version,github.com/onsi/ginkgo/v2)
GINKGO := $(abspath $(TOOLS_BIN_DIR)/$(GINKGO_BIN)-$(GINGKO_VER))
GINKGO_PKG := github.com/onsi/ginkgo/v2/ginkgo

UPDATECLI_BIN := updatecli
UPDATECLI_VER := v0.94.0
UPDATECLI := $(abspath $(TOOLS_BIN_DIR)/$(UPDATECLI_BIN)-$(UPDATECLI_VER))

HELM_VER := v3.18.4
HELM_BIN := helm
HELM := $(TOOLS_BIN_DIR)/$(HELM_BIN)-$(HELM_VER)

CLUSTERCTL_VER := v1.12.2
CLUSTERCTL_BIN := clusterctl
CLUSTERCTL := $(TOOLS_BIN_DIR)/$(CLUSTERCTL_BIN)-$(CLUSTERCTL_VER)

GOLANGCI_LINT_BIN := golangci-lint
GOLANGCI_LINT_VER := $(shell cat .github/workflows/golangci-lint.yaml | grep [[:space:]]version: | sed 's/.*version: //')
GOLANGCI_LINT := $(abspath $(TOOLS_BIN_DIR)/$(GOLANGCI_LINT_BIN)-$(GOLANGCI_LINT_VER))
GOLANGCI_LINT_PKG := github.com/golangci/golangci-lint/v2/cmd/golangci-lint

NOTES_BIN := notes
NOTES := $(abspath $(TOOLS_BIN_DIR)/$(NOTES_BIN))

CRUST_GATHER_BIN := crust-gather
CRUST_GATHER := $(abspath $(TOOLS_BIN_DIR)/$(CRUST_GATHER_BIN))

CHART_TESTING_VER := v3.14.0

# Registry / images
TAG ?= dev
ARCH ?= linux/$(shell go env GOARCH)
TARGET_BUILD ?= prime
TARGET_PLATFORMS := linux/amd64,linux/arm64
MACHINE := rancher-turtles
REGISTRY ?= ghcr.io
ORG ?= rancher
CONTROLLER_IMAGE_NAME ?= turtles
CONTROLLER_IMG ?= $(REGISTRY)/$(ORG)/$(CONTROLLER_IMAGE_NAME)
IID_FILE ?= $(shell mktemp)

# Release
# Exclude tags with the prefix 'test/'
RELEASE_TAG ?= $(shell git describe --abbrev=0 --exclude 'examples/*' --exclude 'test/*' 2>/dev/null)
# Exclude the current RELEASE_TAG and any tags with the prefix 'test/'
PREVIOUS_TAG ?= $(shell git describe --abbrev=0 --exclude $(RELEASE_TAG) --exclude 'examples/*' --exclude 'test/*' 2>/dev/null)
HELM_CHART_TAG ?= $(shell echo $(RELEASE_TAG) | cut -c 2-)
CHART_DIR := charts/rancher-turtles
PROVIDERS_CHART_DIR := charts/rancher-turtles-providers
RELEASE_DIR ?= out
CHART_PACKAGE_DIR ?= $(RELEASE_DIR)/package
CHART_RELEASE_DIR ?= $(RELEASE_DIR)/$(CHART_DIR)
PROVIDERS_CHART_RELEASE_DIR ?= $(RELEASE_DIR)/$(PROVIDERS_CHART_DIR)

# Rancher charts testing
export RANCHER_CHARTS_REPO_DIR ?=  $(abspath $(RELEASE_DIR)/rancher-charts)
export RANCHER_CHART_DEV_VERSION ?= 108.0.0+up99.99.99
export RANCHER_CHARTS_BASE_BRANCH ?= dev-v2.14

# Allow overriding the imagePullPolicy
PULL_POLICY ?= IfNotPresent

# Development config
RANCHER_HOSTNAME ?= my.hostname.dev
CLUSTER_NAME ?= capi-test

CACHE_DIR ?= .buildx-cache/
CACHE_COMMANDS = "--cache-from type=local,src=$(CACHE_DIR) --cache-to type=local,dest=$(CACHE_DIR),mode=max"

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development
.PHONY: generate
generate: vendor ## Run all generators
	$(MAKE) vendor
	$(MAKE) generate-modules
	$(MAKE) generate-manifests-api
	$(MAKE) generate-manifests-external
	$(MAKE) generate-go-deepcopy
	$(MAKE) vendor-clean

.PHONY: manifests
manifests: generate

.PHONY: generate-manifests-external
generate-manifests-external: vendor controller-gen ## Generate ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) crd paths="./api/rancher/..." output:crd:artifacts:config=hack/crd/bases
	$(CONTROLLER_GEN) crd paths="./vendor/sigs.k8s.io/cluster-api/..." output:crd:artifacts:config=hack/crd/bases
	# Vendor is only required for pulling latest CRDs from the dependencies
	$(MAKE) vendor-clean

.PHONY: generate-manifests-api
generate-manifests-api: controller-gen ## Generate ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd paths="./api/v1alpha1/..." paths="./internal/controllers/..." \
			output:crd:artifacts:config=./config/crd/bases \
			output:rbac:dir=./config/rbac \

.PHONY: generate-modules
generate-modules: ## Run go mod tidy to ensure modules are up to date
	go mod tidy
	cd $(TEST_DIR); go mod tidy

.PHONY: generate-go-deepcopy
generate-go-deepcopy:  ## Run deepcopy generation
	$(CONTROLLER_GEN) \
		object:headerFile=./hack/boilerplate.go.txt \
		paths=./api/... 

# Run go mod
.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor
	go mod verify

.PHONY: vendor-clean
vendor-clean:
	rm -rf vendor

.PHOHY: dev-env
dev-env: build-local-rancher-charts ## Create a local development environment
	./scripts/turtles-dev.sh ${RANCHER_HOSTNAME}

## --------------------------------------
## Lint / Verify
## --------------------------------------

##@ lint and verify:

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet -tags $(TARGET_BUILD) ./...

.PHONY: lint
lint: $(GOLANGCI_LINT) ## Lint the codebase
	$(GOLANGCI_LINT) run -v --timeout 5m $(GOLANGCI_LINT_EXTRA_ARGS)

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT) ## Lint the codebase and run auto-fixers if supported by the linter
	GOLANGCI_LINT_EXTRA_ARGS=--fix $(MAKE) lint

.PHONY: updatecli
updatecli-apply: $(UPDATECLI)
	$(UPDATECLI) apply --config ./updatecli/updatecli.d

## --------------------------------------
## Testing
## --------------------------------------

##@ test:

KUBEBUILDER_ASSETS ?= $(shell $(SETUP_ENVTEST) use --use-env -p path $(KUBEBUILDER_ENVTEST_KUBERNETES_VERSION))

.PHONY: test
test: $(SETUP_ENVTEST) manifests ## Run all generators and tests.
	go clean -testcache
	KUBEBUILDER_ASSETS="$(KUBEBUILDER_ASSETS)" go test -tags $(TARGET_BUILD) ./... $(TEST_ARGS)

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -tags $(TARGET_BUILD) -o bin/manager main.go

.PHONY: build-prime
build-prime: ## Build with prime tag
	$(MAKE) build TARGET_BUILD=prime

.PHONY: build-community
build-community: ## Build with community tag
	$(MAKE) build TARGET_BUILD=community

.PHONY: run
run: generate fmt vet ## Run a controller from your host.
	go run ./main.go

buildx-machine:
	@docker buildx inspect $(MACHINE) || \
		docker buildx create --name=$(MACHINE) --platform=$(TARGET_PLATFORMS)

## --------------------------------------
## Docker
## --------------------------------------

.PHONY: docker-pull-prerequisites
docker-pull-prerequisites:
	docker pull docker.io/docker/dockerfile:1.4
	docker pull $(GO_CONTAINER_IMAGE)
	docker pull gcr.io/distroless/static:latest

## --------------------------------------
## Docker - turtles
## --------------------------------------

.PHONY: docker-build
docker-build: buildx-machine docker-pull-prerequisites ## Build docker image for a specific architecture
	# buildx does not support using local registry for multi-architecture images
	DOCKER_BUILDKIT=1 BUILDX_BUILDER=$(MACHINE) docker buildx build $(ADDITIONAL_COMMANDS) \
			--platform $(ARCH) \
			--load \
			--build-arg builder_image=$(GO_CONTAINER_IMAGE) \
			--build-arg goproxy=$(GOPROXY) \
			--build-arg package=. \
			--build-arg go_build_tags=$(TARGET_BUILD) \
			--build-arg ldflags="$(LDFLAGS)" . -t $(CONTROLLER_IMG):$(TAG)

.PHONY: docker-build-prime
docker-build-prime: ## Build docker image with prime tag
	$(MAKE) docker-build TARGET_BUILD=prime

.PHONY: docker-build-community
docker-build-community: ## Build docker image with community tag
	$(MAKE) docker-build TARGET_BUILD=community

.PHONY: docker-build-and-push
docker-build-and-push: buildx-machine docker-pull-prerequisites ## Run docker-build-and-push targets for all architectures
	DOCKER_BUILDKIT=1 BUILDX_BUILDER=$(MACHINE) docker buildx build $(ADDITIONAL_COMMANDS) \
			--platform $(TARGET_PLATFORMS) \
			--push \
			--sbom=true \
			--attest type=provenance,mode=max \
			--iidfile=$(IID_FILE) \
			--build-arg builder_image=$(GO_CONTAINER_IMAGE) \
			--build-arg goproxy=$(GOPROXY) \
			--build-arg package=. \
			--build-arg go_build_tags=$(TARGET_BUILD) \
			--build-arg ldflags="$(LDFLAGS)" . -t $(CONTROLLER_IMG):$(TAG)

.PHONY: docker-build-and-push-prime
docker-build-and-push-prime:
	$(MAKE) docker-build-and-push TARGET_BUILD=prime

.PHONY: docker-build-and-push-community
docker-build-and-push-community:
	$(MAKE) docker-build-and-push TARGET_BUILD=community

docker-list-all:
	@echo $(CONTROLLER_IMG):${TAG}

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(CONTROLLER_IMG)
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

## --------------------------------------
## Lint / Verify
## --------------------------------------

ALL_VERIFY_CHECKS = modules gen

.PHONY: verify
verify: $(addprefix verify-,$(ALL_VERIFY_CHECKS)) ## Run all verify-* targets

.PHONY: verify-modules
verify-modules: generate-modules  ## Verify go modules are up to date
	@if !(git diff --quiet HEAD -- go.sum go.mod); then \
		git diff; \
		echo "go module files are out of date"; exit 1; \
	fi
	@if (find . -name 'go.mod' | xargs -n1 grep -q -i 'k8s.io/client-go.*+incompatible'); then \
		find . -name "go.mod" -exec grep -i 'k8s.io/client-go.*+incompatible' {} \; -print; \
		echo "go module contains an incompatible client-go version"; exit 1; \
	fi

.PHONY: verify-gen
verify-gen: generate  ## Verify go generated files are up to date
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi

## --------------------------------------
## Hack / Tools
## --------------------------------------

##@ hack/tools:

.PHONY: $(CONTROLLER_GEN_BIN)
$(CONTROLLER_GEN_BIN): $(CONTROLLER_GEN) ## Build a local copy of controller-gen.

.PHONY: $(CONVERSION_GEN_BIN)
$(CONVERSION_GEN_BIN): $(CONVERSION_GEN) ## Build a local copy of conversion-gen.

.PHONY: $(GO_APIDIFF_BIN)
$(GO_APIDIFF_BIN): $(GO_APIDIFF) ## Build a local copy of go-apidiff

.PHONY: $(ENVSUBST_BIN)
$(ENVSUBST_BIN): $(ENVSUBST) ## Build a local copy of envsubst.

.PHONY: $(KUSTOMIZE_BIN)
$(KUSTOMIZE_BIN): $(KUSTOMIZE) ## Build a local copy of kustomize.

.PHONY: $(NOTES_BIN)
$(NOTES_BIN): $(NOTES) ## Build a local copy of kustomize.

.PHONY: $(SETUP_ENVTEST_BIN)
$(SETUP_ENVTEST_BIN): $(SETUP_ENVTEST) ## Build a local copy of setup-envtest.

.PHONY: $(GOLANGCI_LINT_BIN)
$(GOLANGCI_LINT_BIN): $(GOLANGCI_LINT) ## Build a local copy of golangci-lint

$(CONTROLLER_GEN): # Build controller-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(CONTROLLER_GEN_PKG) $(CONTROLLER_GEN_BIN) $(CONTROLLER_GEN_VER)

.PHONY: $(CONVERSION_GEN)
$(CONVERSION_GEN): # Build conversion-gen from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(CONVERSION_GEN_PKG) $(CONVERSION_GEN_BIN) $(CONVERSION_GEN_VER)

.PHONY: $(GINKGO_BIN)
$(GINKGO_BIN): $(GINKGO) ## Build a local copy of ginkgo.

.PHONY: $(CRUST_GATHER_BIN)
$(CRUST_GATHER_BIN): $(CRUST_GATHER) ## Download crust-gather.

$(GO_APIDIFF): # Build go-apidiff from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(GO_APIDIFF_PKG) $(GO_APIDIFF_BIN) $(GO_APIDIFF_VER)

$(ENVSUBST): # Build gotestsum from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(ENVSUBST_PKG) $(ENVSUBST_BIN) $(ENVSUBST_VER)

$(KUSTOMIZE): # Build kustomize from tools folder.
	CGO_ENABLED=0 GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(KUSTOMIZE_PKG) $(KUSTOMIZE_BIN) $(KUSTOMIZE_VER)

$(SETUP_ENVTEST): # Build setup-envtest from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(SETUP_ENVTEST_PKG) $(SETUP_ENVTEST_BIN) $(SETUP_ENVTEST_VER)

$(GINKGO): # Build ginkgo from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(GINKGO_PKG) $(GINKGO_BIN) $(GINGKO_VER)

$(UPDATECLI): # Install updatecli
	curl -sSL -o ${TOOLS_BIN_DIR}/updatecli_${GO_ARCH}.tar.gz https://github.com/updatecli/updatecli/releases/download/${UPDATECLI_VER}/updatecli_${UPDATECLI_OS}_${GO_ARCH}.tar.gz
	cd ${TOOLS_BIN_DIR} && tar -xzf updatecli_${GO_ARCH}.tar.gz
	cd ${TOOLS_BIN_DIR} && chmod +x updatecli
	cd ${TOOLS_BIN_DIR} && mv updatecli $(UPDATECLI_BIN)-$(UPDATECLI_VER)

$(GOLANGCI_LINT): # Build golangci-lint from tools folder.
	GOBIN=$(TOOLS_BIN_DIR) $(GO_INSTALL) $(GOLANGCI_LINT_PKG) $(GOLANGCI_LINT_BIN) $(GOLANGCI_LINT_VER)

$(NOTES): # Download and install note generator from cluster-api commit
	hack/make-release-notes.sh $(TOOLS_BIN_DIR) $(CAPI_VERSION)

$(GH): # Download GitHub cli into the tools bin folder
	hack/ensure-gh.sh \
		-b $(TOOLS_BIN_DIR) \
		$(GH_VERSION)

$(CRUST_GATHER): # Downloads and install crust-gather
	curl -sSfL https://github.com/crust-gather/crust-gather/raw/main/install.sh | sh -s - -f -b $(TOOLS_BIN_DIR)

kubectl: # Download kubectl cli into tools bin folder
	hack/ensure-kubectl.sh \
		-b $(TOOLS_BIN_DIR) \
		$(KUBECTL_VERSION)

$(HELM): ## Put helm into tools folder.
	mkdir -p $(TOOLS_BIN_DIR)
	rm -f "$(TOOLS_BIN_DIR)/$(HELM_BIN)*"
	curl --retry $(CURL_RETRIES) -fsSL -o $(TOOLS_BIN_DIR)/get_helm.sh https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3
	chmod 700 $(TOOLS_BIN_DIR)/get_helm.sh
	USE_SUDO=false HELM_INSTALL_DIR=$(TOOLS_BIN_DIR) DESIRED_VERSION=$(HELM_VER) BINARY_NAME=$(HELM_BIN)-$(HELM_VER) $(TOOLS_BIN_DIR)/get_helm.sh
	ln -sf $(HELM) $(TOOLS_BIN_DIR)/$(HELM_BIN)
	rm -f $(TOOLS_BIN_DIR)/get_helm.sh

$(CLUSTERCTL): $(TOOLS_BIN_DIR) ## Download and install clusterctl
	curl --retry $(CURL_RETRIES) -fsSL -o $(CLUSTERCTL) $(CAPI_UPSTREAM_RELEASES)/download/$(CLUSTERCTL_VER)/clusterctl-linux-amd64
	chmod +x $(CLUSTERCTL) 

## --------------------------------------
## Release
## --------------------------------------

$(RELEASE_DIR):
	mkdir -p $(RELEASE_DIR)/

$(CHART_RELEASE_DIR):
	mkdir -p $(CHART_RELEASE_DIR)/templates

$(PROVIDERS_CHART_RELEASE_DIR):
	mkdir -p $(PROVIDERS_CHART_RELEASE_DIR)/templates

$(CHART_PACKAGE_DIR):
	mkdir -p $(CHART_PACKAGE_DIR)

.PHONY: release
release: clean-release $(RELEASE_DIR)  ## Builds and push container images using the latest git tag for the commit.
	$(MAKE) release-chart

.PHONY: build-chart
build-chart: $(HELM) $(KUSTOMIZE) $(RELEASE_DIR) $(CHART_RELEASE_DIR) $(CHART_PACKAGE_DIR) ## Builds the chart to publish with a release
	$(KUSTOMIZE) build ./config/chart > $(CHART_DIR)/templates/rancher-turtles-components.yaml
	$(KUSTOMIZE) build ./config/operatorchart > $(CHART_DIR)/templates/operator-crds.yaml

	cp -rf $(CHART_DIR)/* $(CHART_RELEASE_DIR)

	yq -i '.image.tag="${RELEASE_TAG}"' $(CHART_RELEASE_DIR)/values.yaml
	yq -i '.image.imagePullPolicy="${PULL_POLICY}"' $(CHART_RELEASE_DIR)/values.yaml
	yq -i '.image.repository="${CONTROLLER_IMG}"' $(CHART_RELEASE_DIR)/values.yaml

	cd $(CHART_RELEASE_DIR) && $(HELM) dependency update
	$(HELM) package $(CHART_RELEASE_DIR) --app-version=$(HELM_CHART_TAG) --version=$(HELM_CHART_TAG) --destination=$(CHART_PACKAGE_DIR)

.PHONY: build-providers-chart
build-providers-chart: $(HELM) $(RELEASE_DIR) $(PROVIDERS_CHART_RELEASE_DIR) $(CHART_PACKAGE_DIR)
	cp -rf $(PROVIDERS_CHART_DIR)/* $(PROVIDERS_CHART_RELEASE_DIR)
	cd $(PROVIDERS_CHART_RELEASE_DIR) && $(HELM) dependency update
	$(HELM) package $(PROVIDERS_CHART_RELEASE_DIR) --app-version=$(HELM_CHART_TAG) --version=$(HELM_CHART_TAG) --destination=$(CHART_PACKAGE_DIR)

.PHONY: release-chart
release-chart: $(HELM) $(NOTES) build-chart verify-gen
	$(NOTES) --repository $(REPO) -add-kubernetes-version-support=false -from=tags/$(PREVIOUS_TAG) -release=$(RELEASE_TAG) -branch=main > $(CHART_RELEASE_DIR)/RELEASE_NOTES.md
	$(HELM) package $(CHART_RELEASE_DIR) --app-version=$(HELM_CHART_TAG) --version=$(HELM_CHART_TAG) --destination=$(CHART_PACKAGE_DIR)

.PHONY: test-chart
test-chart: build-chart
	docker run --rm -v $(shell pwd):/charts --workdir /charts quay.io/helmpack/chart-testing:$(CHART_TESTING_VER) ct lint --validate-maintainers=false --charts $(CHART_RELEASE_DIR)

.PHONY: test-providers-chart
test-providers-chart: build-providers-chart
	docker run --rm -v $(shell pwd):/charts --workdir /charts quay.io/helmpack/chart-testing:$(CHART_TESTING_VER) ct lint --validate-maintainers=false --charts $(PROVIDERS_CHART_RELEASE_DIR)


## --------------------------------------
## Rancher charts testing
## --------------------------------------
.PHONY: build-local-rancher-charts
build-local-rancher-charts:
	$(MAKE) clean-rancher-charts
	mkdir -p $(RANCHER_CHARTS_REPO_DIR)
	# First build the Turtles chart
	RELEASE_TAG=$(TAG) HELM_CHART_TAG=$(RANCHER_CHART_DEV_VERSION) $(MAKE) build-chart
	CHART_RELEASE_DIR=$(CHART_RELEASE_DIR) HELM=$(HELM) ./scripts/build-local-rancher-charts.sh
	# Finally build the providers chart
	RELEASE_TAG=$(TAG) HELM_CHART_TAG=$(RANCHER_CHART_DEV_VERSION) $(MAKE) build-providers-chart

## --------------------------------------
## E2E Tests
## --------------------------------------

$(CACHE_DIR):
	mkdir -p $(CACHE_DIR)/

E2ECONFIG_VARS ?= MANAGEMENT_CLUSTER_ENVIRONMENT=$(MANAGEMENT_CLUSTER_ENVIRONMENT) \
ROOT_DIR=$(ROOT_DIR) \
TURTLES_VERSION=$(TAG) \
E2E_CONFIG=$(E2E_CONFIG) \
TURTLES_IMAGE=$(REGISTRY)/$(ORG)/turtles-e2e \
ARTIFACTS=$(ARTIFACTS) \
ARTIFACTS_FOLDER=$(ARTIFACTS_FOLDER) \
HELM_BINARY_PATH=$(HELM) \
CLUSTERCTL_BINARY_PATH=$(CLUSTERCTL) \
SKIP_RESOURCE_CLEANUP=$(SKIP_RESOURCE_CLEANUP) \
USE_EXISTING_CLUSTER=$(USE_EXISTING_CLUSTER) \
TURTLES_PROVIDERS=$(TURTLES_PROVIDERS) \
TURTLES_PROVIDERS_PATH=$(ROOT_DIR)/$(CHART_PACKAGE_DIR)/rancher-turtles-providers-$(RANCHER_CHART_DEV_VERSION).tgz

E2E_RUN_COMMAND=$(E2ECONFIG_VARS) $(GINKGO) -v --trace -p -procs=10 -poll-progress-after=$(GINKGO_POLL_PROGRESS_AFTER) \
		-poll-progress-interval=$(GINKGO_POLL_PROGRESS_INTERVAL) --tags=e2e --focus="$(GINKGO_FOCUS)" --label-filter="$(GINKGO_LABEL_FILTER)" \
		$(_SKIP_ARGS) --nodes=$(GINKGO_NODES) --timeout=$(GINKGO_TIMEOUT) --no-color=$(GINKGO_NOCOLOR) \
		--output-dir="$(ARTIFACTS)" --junit-report="junit.e2e_suite.1.xml" $(GINKGO_ARGS) $(GINKGO_TESTS)

.PHONY: test-e2e
test-e2e: ## If MANAGEMENT_CLUSTER_ENVIRONMENT is 'eks', run remote e2e tests, otherwise run local e2e tests
	# if 'eks' -> you should call `e2e-image-build-and-push` first
	# any case that's not 'eks' means a local management cluster -> build e2e-image
	if [ "$(MANAGEMENT_CLUSTER_ENVIRONMENT)" = "eks" ]; then \
		echo "Running remote E2E tests using an EKS management cluster. You should have already built and pushed the E2E image using 'make e2e-image-build-and-push'."; \
		$(MAKE) test-e2e-remote; \
	else \
		$(MAKE) test-e2e-local; \
	fi

.PHONY: test-e2e-local
test-e2e-local: $(GINKGO) $(HELM) $(CLUSTERCTL) $(ENVSUBST) kubectl e2e-image build-local-rancher-charts ## Run the end-to-end tests
	$(E2E_RUN_COMMAND)

.PHONY: test-e2e-remote
test-e2e-remote: $(GINKGO) $(HELM) $(CLUSTERCTL) kubectl build-local-rancher-charts ## Run the end-to-end using a remote management cluster
	# You need to first build and push the e2e image by calling `make e2e-image-build-and-push`
	$(E2E_RUN_COMMAND)

.PHONY: e2e-image
e2e-image: ## Build the image for e2e tests
	# First build the regular Turtles image
	CONTROLLER_IMG=$(REGISTRY)/$(ORG)/turtles-e2e $(MAKE) e2e-image-build

.PHONY: e2e-image-build-and-push
e2e-image-build-and-push: e2e-image
	# First push the regular Turtles image
	CONTROLLER_IMG=$(REGISTRY)/$(ORG)/turtles-e2e $(MAKE) e2e-image-push

.PHONY: e2e-image-build
e2e-image-build: ## Build the image for e2e tests
	docker build \
		--build-arg builder_image=$(GO_CONTAINER_IMAGE) \
		--build-arg goproxy=$(GOPROXY) \
		--build-arg package=. \
		--build-arg go_build_tags=$(TARGET_BUILD) \
		--build-arg ldflags="$(LDFLAGS)" . -t $(CONTROLLER_IMG):$(TAG)

.PHONY: e2e-image-push
e2e-image-push: ## Push the image for e2e tests
	docker push $(CONTROLLER_IMG):$(TAG)

.PHONY: compile-e2e
e2e-compile: ## Test e2e compilation
	go test -c -o /dev/null -tags=e2e ./test/e2e/suites/***

## --------------------------------------
## Documentation
## --------------------------------------

.PHONY: generate-doctoc
generate-doctoc:
	TRACE=$(TRACE) ./hack/generate-doctoc.sh

.PHONY: verify-doctoc
verify-doctoc: generate-doctoc
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "doctoc is out of date, run make generate-doctoc"; exit 1; \
	fi

## --------------------------------------
## Cleanup / Verification
## --------------------------------------

.PHONY: clean-release
clean-release: ## Remove the release folder
	rm -rf $(RELEASE_DIR)

.PHOHY: clean-dev-env
clean-dev-env: ## Remove the dev env
	kind delete cluster --name=$(CLUSTER_NAME)

.PHOHY: clean-rancher-charts
clean-rancher-charts: ## Remove the local rancher charts folder
	rm -rf $(RANCHER_CHARTS_REPO_DIR)

## --------------------------------------
## Collect artifacts
## --------------------------------------

.PHONY: collect-artifacts
collect-artifacts: $(CRUST_GATHER_BIN)
	$(CRUST_GATHER) collect -f $(ARTIFACTS_FOLDER)/gather
