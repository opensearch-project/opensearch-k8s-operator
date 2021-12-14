# Opensearch-k8s-operator
OpenSearch k8s Operator
The Operator is reconciling the OpenSearch CRD, a CRD for defining an OpensearchCluster (data,masters and opensearch-dashboard).
When the reconciliation is done, the opeartor will create a full working OpenSearch cluster.

# Insatlling the operator
  - clone the repo
  - make manifests
  - make insatll

# Create opensearch-cluster
Use os-cluster.yaml to define your cluster - when the "ClusterName" is also the Namespace that the new cluster will resides in.
kubectl create os-cluster.yaml

    kubectl create os-cluster.yaml
    
# Delete opensearch-cluster
 In order to delete the cluster please delete your OpenSearch cluster resource ,this will delete the clsuter namespace and all its resources.
 
    kubectl get os --all-namespaces
    kubectl delete os os-from-operator -n <namespace>
    
 # Issues for the future 
  - error handling.
  - implement all nodes types.
  - implement versions.
  - exposing the cluster with seleted vendor (ingress/haproxy/etc...).
  - supports in local-storage.
  - proper logging.
  - enable OS/ES auto-upgrade.
