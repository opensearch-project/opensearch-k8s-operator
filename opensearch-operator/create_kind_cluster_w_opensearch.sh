#!/bin/bash
set -e  # Exit on any error

# Function to check if a command exists
check_command() {
    if ! command -v "$1" &> /dev/null; then
        echo "Error: $1 is required but not installed."
        exit 1
    fi
}

# Check required tools

for cmd in kind kubectl helm make docker; do
    check_command "$cmd"
done

echo 'Creating kind cluster...'
if ! kind create cluster; then
    echo "Error: Failed to create kind cluster"
    exit 1
fi

echo $'\nCreating OpenSearch operator namespace...'
kubectl create namespace opensearch-operator || true  # Continue if namespace exists

echo $'\nBuilding operator...'
make build manifests
make docker-build

echo $'\nLoading Docker image into kind cluster...'
kind load docker-image controller:latest

# Opensearch operator installation

echo $'\nInstalling OpenSearch operator with Helm...'

helm install opensearch-operator ../charts/opensearch-operator \
    --namespace opensearch-operator \
    --create-namespace \
    --set manager.image.repository=controller \
    --set manager.image.tag=latest \
    --set manager.image.pullPolicy=IfNotPresent

echo "Waiting for OpenSearch operator to be ready..."

timeout 300 bash -c 'while true; do
    if kubectl get pods -n opensearch-operator --no-headers 2>/dev/null | grep -q "Running"; then
        break
    fi
    echo -n "."
    sleep 5
done'

echo -e "OpenSearch operator is ready."

# Install CRDs
echo $'\nInstalling CRDs...'
make install

# Opensearch Cluster installation

echo $'\nInstalling OpenSearch Cluster with Helm...\n'

helm install opensearch-cluster ../charts/opensearch-cluster \
    --set general.serviceName=opensearch-cluster \
    --set general.serviceType=ClusterIP

echo $'\n Wait for opensearch cluster to spin up in k9s...\n'