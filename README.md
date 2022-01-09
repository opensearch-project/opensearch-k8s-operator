# OpenSearch-k8s-operator
The Kubernetes OpenSearch Operator is used for automating the deployment, provisioning, management, and orchestration of OpenSearch clusters and OpenSearch dashboards.

# Roadmap
The project is currently a work in progress and is not (yet) recommended for use in non-test environments. The plan is to reach General Availability and a fully functioning Operator by end of 22-Q1.

## The Operator features:
- [x] Deploy a new OS cluster.
- [x] Ability to deploy multiple clusters.
- [x] Spin up OS dashboards.
- [ ] Operator Monitoring, with Prometheus and Grafana.
- [ ] Configuration of all node roles (master, data, coordinating..).
- [ ] Scale the cluster resources (manually), per nodes' role group. 
- [ ] Drain strategy for scale down.
- [ ] Cluster configurations and nodes' settings updates.
- [ ] Single/Rolling restarts.
- [ ] Version updates.
- [ ] Scaling nodes' disks - increase/replace disks.
- [ ] Change nodes' memory allocation and limits.
- [ ] Control shard balancing and allocation.
- [ ] Advanced allocation strategies: AZ/Rack awareness, Hot/Warm.
- [ ] Secured installation features.
- [ ] Auto scaler based on usage, load, and resources.

# Getting Started
## Installing the Operator
- Clone the repo.
- Make manifests.
- Make install.

## Deploying a new OpenSearch cluster
Use os-cluster.yaml to define your cluster - note that the `ClusterName` is also the namespace that the new cluster will reside in.

    kubectl create os-cluster.yaml
    
- Note: the current installation deploys with the default demo certificate provided by OpenSearch. 

## Deleting an OpenSearch cluster
In order to delete the cluster, please delete your OpenSearch cluster resource; this will delete the cluster namespace and all its resources.
 
    kubectl get os --all-namespaces
    kubectl delete os os-from-operator -n <namespace>
    
 
# Contributions
We welcome contributions! If you want to contribute to this project please reach out to us at: <operator@opster.com>. 

