
# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Namespace to deploy the operator to
NAMESPACE ?= opensearch-operator-system
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.27
PROJECT_PATH=$(CURDIR)

# OLM Bundle Configuration
VERSION ?= 3.0.0
BUNDLE_IMG ?= opensearchproject/opensearch-operator-bundle:v$(VERSION)
CATALOG_IMG ?= opensearchproject/opensearch-operator-catalog:v$(VERSION)
BUNDLE_CHANNELS ?= stable,alpha
BUNDLE_DEFAULT_CHANNEL ?= stable

# Local registry for development/CI testing
LOCAL_REGISTRY ?= localhost:5000
LOCAL_IMG ?= $(LOCAL_REGISTRY)/opensearch-operator:$(VERSION)
LOCAL_BUNDLE_IMG ?= $(LOCAL_REGISTRY)/opensearch-operator-bundle:v$(VERSION)

# Preflight operator check requirements
KUBECONFIG ?= $(HOME)/.kube/config
PFLT_INDEXIMAGE ?= registry.redhat.io/redhat/redhat-operator-index:v4.14

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

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
.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen helm-docs ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations and helm-docs.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: docs
docs: crd-ref-docs manifests ## Generate CRD API reference documentation.
	@echo "Generating CRD documentation..."
	@mkdir -p ../docs/designs
	@if [ -f ../docs/designs/crd-ref-docs-config.yaml ]; then \
		$(CRD_REF_DOCS) \
			--source-path=$(CURDIR)/api \
			--config=../docs/designs/crd-ref-docs-config.yaml \
			--renderer=markdown \
			--output-path=../docs/designs/crd.md; \
	else \
		$(CRD_REF_DOCS) \
			--source-path=$(CURDIR)/api \
			--renderer=markdown \
			--output-path=../docs/designs/crd.md; \
	fi
	@echo "CRD documentation generated at ../docs/designs/crd.md"

.PHONY: helm-docs
helm-docs: ${HELM_DOCS}
	${HELM_DOCS} -s file -c ../charts/opensearch-operator
	${HELM_DOCS} -s file -c ../charts/opensearch-cluster

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: lint
lint:
	docker run --rm --volume="${PROJECT_PATH}:/go/src/opensearch-operator" -w /go/src/opensearch-operator golangci/golangci-lint:v2.7-alpine golangci-lint run

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: genmocks
genmocks: mockery
	$(MOCKERY) --all

.PHONY: test
test: manifests generate fmt vet envtest genmocks ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go test ./... -coverprofile cover.out -test.v

##@ Build

build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go

run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go --loglevel debug

docker-build: generate fmt vet ## Build docker image with the manager.
	DOCKER_BUILDKIT=1 docker build -t ${IMG} .

docker-build-multiarch: ## Build docker image with the manager for all supported architectures
	DOCKER_BUILDKIT=1 docker buildx build --platform="linux/amd64,linux/arm,linux/arm64" -t ${IMG} .

docker-push: ## Push docker image with the manager.
	docker push ${IMG}

##@ Deployment
.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	cd config/default && $(KUSTOMIZE) edit set namespace ${NAMESPACE}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/default | kubectl delete -f -

##@ OLM Bundle

.PHONY: bundle
bundle: manifests operator-sdk ## Sync CRDs to bundle and validate.
	@echo "Syncing CRDs to bundle/manifests..."
	cp config/crd/bases/opensearch.org_*.yaml bundle/manifests/
	@echo "Validating bundle..."
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-update-version
bundle-update-version: ## Update version in CSV (usage: make bundle-update-version VERSION=x.y.z)
	@echo "Updating CSV version to $(VERSION)..."
	sed -i 's/opensearch-operator\.v[0-9]*\.[0-9]*\.[0-9]*/opensearch-operator.v$(VERSION)/g' bundle/manifests/opensearch-operator.clusterserviceversion.yaml
	sed -i 's/version: [0-9]*\.[0-9]*\.[0-9]*/version: $(VERSION)/g' bundle/manifests/opensearch-operator.clusterserviceversion.yaml
	sed -i 's/opensearch-operator:[0-9]*\.[0-9]*\.[0-9]*/opensearch-operator:$(VERSION)/g' bundle/manifests/opensearch-operator.clusterserviceversion.yaml

.PHONY: bundle-build
bundle-build: ## Build the bundle image.
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	docker push $(BUNDLE_IMG)

.PHONY: bundle-validate
bundle-validate: operator-sdk ## Validate the bundle.
	$(OPERATOR_SDK) bundle validate ./bundle --select-optional suite=operatorframework

.PHONY: bundle-scorecard
bundle-scorecard: operator-sdk ## Run scorecard tests on the bundle.
	$(OPERATOR_SDK) scorecard ./bundle --wait-time 60s

# Build a catalog image by adding bundle images to an empty catalog using the operator-sdk tool
# https://olm.operatorframework.io/docs/tasks/creating-a-catalog/
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool docker --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMG)

.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	docker push $(CATALOG_IMG)

##@ Red Hat Certification

# Red Hat certification project ID (set this for your project)
CERTIFICATION_PROJECT_ID ?= ""

.PHONY: local-registry-start
local-registry-start: ## Start a local Docker registry for testing.
	@if [ -z "$$(docker ps -q -f name=preflight-registry)" ]; then \
		echo "Starting local registry at $(LOCAL_REGISTRY)..."; \
		docker run -d -p 5000:5000 --restart=always --name preflight-registry registry:2; \
	else \
		echo "Local registry already running."; \
	fi

.PHONY: local-registry-stop
local-registry-stop: ## Stop the local Docker registry.
	@docker stop preflight-registry 2>/dev/null || true
	@docker rm preflight-registry 2>/dev/null || true

.PHONY: docker-push-local
docker-push-local: docker-build ## Build and push operator image to local registry.
	docker tag $(IMG) $(LOCAL_IMG)
	docker push $(LOCAL_IMG)

.PHONY: bundle-push-local
bundle-push-local: bundle-build ## Build and push bundle image to local registry.
	docker tag $(BUNDLE_IMG) $(LOCAL_BUNDLE_IMG)
	docker push $(LOCAL_BUNDLE_IMG)

.PHONY: preflight-container-local
preflight-container-local: preflight local-registry-start docker-push-local ## Run preflight checks on local operator image.
	$(PREFLIGHT) check container $(LOCAL_IMG) --insecure

.PHONY: preflight-operator-local
preflight-operator-local: preflight local-registry-start bundle-push-local ## Run preflight checks on local bundle (requires cluster).
	@echo "NOTE: This requires a running cluster with OLM installed."
	@echo "Using KUBECONFIG=$(KUBECONFIG) and PFLT_INDEXIMAGE=$(PFLT_INDEXIMAGE)"
	@echo "For bundle validation without a cluster, use: make bundle-validate"
	KUBECONFIG=$(KUBECONFIG) PFLT_INDEXIMAGE=$(PFLT_INDEXIMAGE) $(PREFLIGHT) check operator $(LOCAL_BUNDLE_IMG)

.PHONY: preflight-container
preflight-container: preflight ## Run preflight checks on remote container image.
	$(PREFLIGHT) check container $(IMG) --docker-config=$(HOME)/.docker/config.json

.PHONY: preflight-operator
preflight-operator: preflight ## Run preflight checks on remote operator bundle (requires cluster).
	@echo "NOTE: This requires a running cluster with OLM installed."
	@echo "Using KUBECONFIG=$(KUBECONFIG) and PFLT_INDEXIMAGE=$(PFLT_INDEXIMAGE)"
	@echo "For bundle validation without a cluster, use: make bundle-validate"
	KUBECONFIG=$(KUBECONFIG) $(PREFLIGHT) check operator $(BUNDLE_IMG) --docker-config=$(HOME)/.docker/config.json --indeximage=$(PFLT_INDEXIMAGE)

.PHONY: preflight-submit
preflight-submit: preflight ## Submit certification results to Red Hat (requires CERTIFICATION_PROJECT_ID and PYXIS_API_TOKEN).
	@if [ -z "$(CERTIFICATION_PROJECT_ID)" ]; then echo "Error: CERTIFICATION_PROJECT_ID is not set"; exit 1; fi
	@if [ -z "$(PYXIS_API_TOKEN)" ]; then echo "Error: PYXIS_API_TOKEN is not set"; exit 1; fi
	$(PREFLIGHT) check container $(IMG) \
		--submit \
		--certification-project-id=$(CERTIFICATION_PROJECT_ID) \
		--pyxis-api-token=$(PYXIS_API_TOKEN) \
		--docker-config=$(HOME)/.docker/config.json

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
MOCKERY ?= $(LOCALBIN)/mockery
HELM_DOCS := $(LOCALBIN)/helm-docs
CRD_REF_DOCS ?= $(LOCALBIN)/crd-ref-docs
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
OPM ?= $(LOCALBIN)/opm
PREFLIGHT ?= $(LOCALBIN)/preflight

## Tool Versions
KUSTOMIZE_VERSION ?= v5.3.0
CONTROLLER_TOOLS_VERSION ?= v0.19.0
MOCKERY_VERSION ?= v2.52.2
HELM_DOCS_VERSION ?= 1.14.2
CRD_REF_DOCS_VERSION ?= v0.2.0
OPERATOR_SDK_VERSION ?= v1.42.0
OPM_VERSION ?= v1.47.0
PREFLIGHT_VERSION ?= 1.16.0

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary.
$(KUSTOMIZE): $(LOCALBIN)
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: mockery
mockery: $(MOCKERY) ## Download mockery locally if necessary.
$(MOCKERY): $(LOCALBIN)
	test -s $(LOCALBIN)/mockery || GOBIN=$(LOCALBIN) go install github.com/vektra/mockery/v2@$(MOCKERY_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: helm-docs
helm-docs: $(HELM_DOCS) ## Download helm-docs locally if necessary.
$(HELM_DOCS): $(LOCALBIN)
	@ARCH=$$(uname -m); \
	if [ "$$ARCH" = "aarch64" ] || [ "$$ARCH" = "arm64" ]; then ARCH="arm64"; else ARCH="x86_64"; fi; \
	curl -sL "https://github.com/norwoodj/helm-docs/releases/download/v$(HELM_DOCS_VERSION)/helm-docs_$(HELM_DOCS_VERSION)_$$(uname -s)_$$ARCH.tar.gz" | tar -xz -C $(LOCALBIN) helm-docs
	@chmod +x $(HELM_DOCS)

.PHONY: crd-ref-docs
crd-ref-docs: $(CRD_REF_DOCS) ## Download crd-ref-docs locally if necessary.
$(CRD_REF_DOCS): $(LOCALBIN)
	test -s $(LOCALBIN)/crd-ref-docs || GOBIN=$(LOCALBIN) go install github.com/elastic/crd-ref-docs@$(CRD_REF_DOCS_VERSION)

.PHONY: operator-sdk
operator-sdk: $(OPERATOR_SDK) ## Download operator-sdk locally if necessary.
$(OPERATOR_SDK): $(LOCALBIN)
	@if [ ! -s $(LOCALBIN)/operator-sdk ]; then \
		OS=$$(go env GOOS) && ARCH=$$(go env GOARCH) && \
		curl -sSLo $(LOCALBIN)/operator-sdk https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} && \
		chmod +x $(LOCALBIN)/operator-sdk; \
	fi

.PHONY: opm
opm: $(OPM) ## Download opm locally if necessary.
$(OPM): $(LOCALBIN)
	@if [ ! -s $(LOCALBIN)/opm ]; then \
		OS=$$(go env GOOS) && ARCH=$$(go env GOARCH) && \
		curl -sSLo $(LOCALBIN)/opm https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/$${OS}-$${ARCH}-opm && \
		chmod +x $(LOCALBIN)/opm; \
	fi

.PHONY: preflight
preflight: $(PREFLIGHT) ## Download preflight locally if necessary.
$(PREFLIGHT): $(LOCALBIN)
	@if [ ! -s $(LOCALBIN)/preflight ]; then \
		OS=$$(go env GOOS) && ARCH=$$(go env GOARCH) && \
		curl -sSLo $(LOCALBIN)/preflight https://github.com/redhat-openshift-ecosystem/openshift-preflight/releases/download/$(PREFLIGHT_VERSION)/preflight-$${OS}-$${ARCH} && \
		chmod +x $(LOCALBIN)/preflight; \
	fi
