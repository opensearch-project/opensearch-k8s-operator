# OpenSearch-k8s-operator
The Kubernetes OpenSearch Operator is used for automating the deployment, provisioning, management, and orchestration of OpenSearch clusters and OpenSearch Dashboards.

# Roadmap
The project is currently a work in progress and is not (yet) recommended to be used for non test environments. The plan is to reach General Availability and fully functioning operator by end of 22-Q1.

## The Operator features:
- [x] Deploy a new OS cluster.
- [x] Ability to deploy multiple clusters.
- [x] Spin up OS dashboards.
- [ ] Configuration of all node roles (master, data, coordinating,..).
- [ ] Scale (manually) the cluster resources, per nodes role group. 
- [ ] Drain strategy for scale down.
- [ ] Cluster configurations and nodes settings updates.
- [ ] Single/Rolling restarts.
- [ ] Version update.
- [ ] Scaling nodes disks - increase/replace disks.
- [ ] Change nodes memory allocation and limits.
- [ ] Control Shards balancing and allocation.
- [ ] Advanced allocation strategies: AZ/Rack awareness, Hot/Warm.
- [ ] Secured installation features.
- [ ] Auto scaler based on usage, load, and resources.

# Getting Started
## Installing the Operator
- clone the repo
- make manifests
- make insatll

## Deploying a new OpenSearch cluster
Use os-cluster.yaml to define your cluster - note that the `ClusterName` is also the namespace that the new cluster will resides in.

    kubectl create os-cluster.yaml
    
## Delete an OpenSearch cluster
In order to delete the cluster please delete your OpenSearch cluster resource; this will delete the cluster namespace and all its resources.
 
    kubectl get os --all-namespaces
    kubectl delete os os-from-operator -n <namespace>
    
 
# Contributions
We welcome contributions! If you want to conribute to this project please reach out to us at: <operator@opster.com>. 

