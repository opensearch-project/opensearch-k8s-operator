#!/bin/bash

# Remove ism policies from k8s cluster
echo "Removing ISM policy "opensearch-ismpolicy-apply.yaml" from K8S cluster..."
kubectl delete -f examples/2.x/opensearch-ismpolicy-apply.yaml

# Clean up the OpenSearch cluster
echo $'\nCleaning Opensearch Cluster...'
OPENSEARCH_HOST="https://localhost:9200"

# Delete all ISM policies
echo $'\nFetching ISM policies...'
POLICIES=$(curl -k -u admin:admin -X GET "$OPENSEARCH_HOST/_plugins/_ism/policies/" | jq -r '.policies[].policy.policy_id')

echo "Deleting ISM policies..."
for policy_id in $POLICIES; do
    echo "Deleting policy: $policy_id"
    curl -k -u admin:admin -X DELETE "$OPENSEARCH_HOST/_plugins/_ism/policies/$policy_id"
done

# Delete all test-* indices
echo $'\nDeleting indices matching 'test-*'...'
curl -k -u admin:admin -X DELETE "$OPENSEARCH_HOST/test-*" || echo "No matching indices found or error occurred"
echo "Cleanup complete."

# Build the Docker image
echo $'\nCreating new docker image...'
make build
make docker-build

# Check if we're using k3d or kind
# k3d image import controller:latest
kind load docker-image controller:latest

# Restart the OpenSearch operator
echo $'\nRestarting OpenSearch operator...'
kubectl delete pod -l app.kubernetes.io/instance=opensearch-operator -n opensearch-operator

# Wait for the operator to be ready
echo $'\nWaiting for OpenSearch operator to be ready...'
while ! kubectl get pods -n opensearch-operator | grep "Running"; do
    echo '.'
    sleep 5
done
echo $'\nOpenSearch operator is ready.'