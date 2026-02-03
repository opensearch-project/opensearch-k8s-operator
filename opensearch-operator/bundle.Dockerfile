# Bundle dockerfile for OpenSearch Operator
# This Dockerfile builds an operator bundle image for OLM

FROM scratch

# Core bundle labels
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=opensearch-operator
LABEL operators.operatorframework.io.bundle.channels.v1=stable,alpha
LABEL operators.operatorframework.io.bundle.channel.default.v1=stable

# Labels for Red Hat OpenShift certification
LABEL com.redhat.openshift.versions="v4.12-v4.16"
LABEL com.redhat.delivery.operator.bundle=true
LABEL com.redhat.delivery.backport=false

# Copy files to locations specified by labels
COPY bundle/manifests /manifests/
COPY bundle/metadata /metadata/

# Copy scorecard tests if present
COPY bundle/tests/scorecard /tests/scorecard/
