# opensearch-k8s-operator
Opster &amp; SberBank OpenSearch k8s Operator

# insatlling the operator
  - clone the repo
  - make manifests
  - make insatll

# create opensearch-cluster
  - use os-cluster.yaml to difine your cluster - when "ClusterName" is also the Namespace that the cluster will create in.
    kubectl create os-cluster.yml
    
# delete opensearch-cluster
    to delete cluster please delete your Os cluster resource ,that will delete the clsuter namespace and all resources.
    
 # issues for the future 
  - error handling.
  - implement all nodes types.
  - implement versions.
  - exposing the cluster with seleted vendor (ingress/haproxy/etc...).
  - supports in local-storage.
     
   
