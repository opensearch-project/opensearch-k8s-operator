# Image URL to use all building/pushing image targets
IMG ?= operator-sidecar:latest
PROJECT_PATH=$(CURDIR)

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

all: build

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: lint
lint:
	docker run --rm --volume="${PROJECT_PATH}:/go/src/opensearch-operator" -w /go/src/opensearch-operator golangci/golangci-lint:v1.57-alpine golangci-lint run -E gofmt --timeout=3m

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./

##@ Build

build: fmt vet ## Build manager binary.
	go build -o bin/sidecar main.go

docker-build: fmt vet ## Build docker image for the sidecar
	DOCKER_BUILDKIT=1 docker build -t ${IMG} .

docker-build-multiarch: ## Build docker image for all supported architectures
	DOCKER_BUILDKIT=1 docker buildx build --platform="linux/amd64,linux/arm,linux/arm64" -t ${IMG} .

docker-push: ## Push docker image to registry
	docker push ${IMG}
