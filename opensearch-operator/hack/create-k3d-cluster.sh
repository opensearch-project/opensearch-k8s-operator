#!/usr/bin/env bash

# This script is based on https://github.com/tilt-dev/k3d-local-registry/blob/master/k3d-with-registry.sh

# Starts a k3s cluster (via k3d) with local image registry enabled,
# and with nodes annotated such that Tilt (https://tilt.dev/) can
# auto-detect the registry.

set -o errexit

# desired cluster name (default is "k3s-default")
CLUSTER_NAME="${CLUSTER_NAME:-k3s-tilt-opensearch}"

KUBECONFIG= k3d cluster create \
  --config $(dirname $0)/k3d-local.yaml \
  --timeout 30s \
  ${CLUSTER_NAME} "$@"
