# opensearch-k8s-operator
OpenSearch k8s Operator
The Operator is reconciling the Os CRD, a crd that defining OpensearchCluster (data,masters and opensearch-dashboard).
After the reconciliation the opeartor will create a full working OpenSearch cluster.

# insatlling the operator
  - clone the repo
  - make manifests
  - make insatll

# create opensearch-cluster
use os-cluster.yaml to difine your cluster - when "ClusterName" is also the Namespace that the cluster will create in.
kubectl create os-cluster.yaml

    kubectl create os-cluster.yaml
    
# delete opensearch-cluster
 to delete cluster please delete your Os cluster resource ,that will delete the clsuter namespace and all resources.
 
    kubectl get os --all-namespaces
    kubectl delete os os-from-operator -n <namespace>
    
 # issues for the future 
  - error handling.
  - implement all nodes types.
  - implement versions.
  - exposing the cluster with seleted vendor (ingress/haproxy/etc...).
  - supports in local-storage.
  - proper logging.
  - enable OS/ES auto-upgrade.
