#!/bin/bash

# Remove ism policies from k8s cluster
echo "Removing ISM policy "opensearch-ismpolicy-apply.yaml" from K8S cluster..."
kubectl delete -f examples/2.x/opensearch-ismpolicy-apply.yaml

echo "CLEANING OPENSEARCH CLUSTER..."

# Set your OpenSearch endpoint (adjust if using port-forward or NodePort)
OPENSEARCH_HOST="https://localhost:9200"

# Delete all ISM policies
echo "Fetching ISM policies..."
POLICIES=$(curl -k -u admin:admin -X GET "$OPENSEARCH_HOST/_plugins/_ism/policies/" | jq -r '.policies[].policy.policy_id')

echo "Deleting ISM policies..."
for policy_id in $POLICIES; do
    echo "Deleting policy: $policy_id"
    curl -k -u admin:admin -X DELETE "$OPENSEARCH_HOST/_plugins/_ism/policies/$policy_id"
done

# Delete all test-* indices
echo "Deleting indices matching 'test-*'..."
curl -k -u admin:admin -X DELETE "$OPENSEARCH_HOST/test-*" || echo "No matching indices found or error occurred"

echo "Cleanup complete."

echo "Creating new docker image..."

# Build the Docker image
make build
make docker-build

# Load the image into the OpenSearch cluster
k3d image import controller:latest

echo "Ready - Delete opensearch operator pod"