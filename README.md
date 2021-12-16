# OpenSearch-k8s-operator
The Operator is used for creating an OpenSearch cluster.
it uses a CRD in order to define the OpenSearch cluster (defaining data, masters nodes and opensearch-dashboard).
When the reconciliation is done, the opeartor will create a full working OpenSearch cluster.

# Getting Started
  # installing
  - clone the repo
  - make manifests
  - make insatll

# Create openSearch-cluster
Use os-cluster.yaml to define your cluster - when the "ClusterName" is also the namespace that the new cluster will resides in.
kubectl create os-cluster.yaml

    kubectl create os-cluster.yaml
    
# Delete openSearch-cluster
 In order to delete the cluster please delete your OpenSearch cluster resource; this will delete the cluster namespace and all its resources.
 
    kubectl get os --all-namespaces
    kubectl delete os os-from-operator -n <namespace>
    
 # Next items for development 
  - error handling.
  - implement all nodes types.
  - implement versions.
  - exposing the cluster with seleted vendor (ingress/haproxy/etc...).
  - supports in local-storage.
  - proper logging.
  - enable OS/ES auto-upgrade.

# Contributions welcome
If you want to conribute to this project please reach out to us at: k8s.operatorOpenSearch@opster.com.
Thank you!

# References
  https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
