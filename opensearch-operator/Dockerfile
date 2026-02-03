# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.24.11 AS builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/   pkg/
COPY webhook/ webhook/
COPY opensearch-gateway/   opensearch-gateway/

# Build
ARG TARGETOS
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -a -o manager main.go

# Create licenses directory and copy license file
RUN mkdir -p /workspace/licenses
COPY licenses/LICENSE /workspace/licenses/

# Use Red Hat UBI micro as minimal base image for Red Hat certification compatibility
FROM registry.access.redhat.com/ubi9/ubi-micro:latest

# Red Hat certification labels
ARG VERSION=3.0.0

LABEL name="opensearch-operator" \
      vendor="OpenSearch Project" \
      version="${VERSION}" \
      release="1" \
      summary="OpenSearch Kubernetes Operator" \
      description="A Kubernetes operator for deploying and managing OpenSearch clusters, including node pools, security configuration, dashboards, index management, and snapshots." \
      maintainer="OpenSearch Project <opensearch-infra@amazon.com>" \
      io.k8s.display-name="OpenSearch Operator" \
      io.k8s.description="OpenSearch Kubernetes Operator for managing OpenSearch clusters" \
      io.openshift.tags="opensearch,elasticsearch,search,analytics,logging,database,operator"

WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=builder /workspace/licenses/ /licenses/
USER 65532:65532

ENTRYPOINT ["/manager"]
