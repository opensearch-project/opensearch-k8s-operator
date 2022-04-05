![build](https://github.com/opster/opensearch-k8s-operator/actions/workflows/docker-build.yaml/badge.svg) ![test](https://github.com/opster/opensearch-k8s-operator/actions/workflows/testing.yaml/badge.svg)

# OpenSearch-k8s-operator
The Kubernetes OpenSearch Operator is used for automating the deployment, provisioning, management, and orchestration of OpenSearch clusters and OpenSearch dashboards.

# Roadmap
The full roadmap is available here: [Development plan](https://github.com/Opster/opensearch-k8s-operator/blob/main/docs/designs/dev-plan.md)

## The Operator features:
- [x] Deploy a new OS cluster.
- [x] Ability to deploy multiple clusters.
- [x] Spin up OS dashboards.
- [x] Configuration of all node roles (master, data, coordinating..).
- [x] Scale the cluster resources (manually), per nodes' role group. 
- [x] Drain strategy for scale down.
- [x] Version updates.
- [x] Change nodes' memory allocation and limits.
- [x] Secured installation features.
- [x] Certificate management.
- [x] Rolling restarts - through API.
- [ ] Scaling nodes' disks - increase/replace disks.
- [ ] Cluster configurations and nodes' settings updates.
- [ ] Auto scaler based on usage, load, and resources.
- [ ] Operator Monitoring, with Prometheus and Grafana.
- [ ] Control shard balancing and allocation: AZ/Rack awareness, Hot/Warm.

# Getting Started
## Installing the Operator

- Clone the repo
- Run `make build manifests` to build the controller binary and the manifests
- Start a kubernetes cluster (e.g. with k3d or minikube) and make sure your `~/.kube/config` points to it
- Run `make install` to create the CRD in the kubernetes cluster

## Deploying a new OpenSearch cluster

Go to `opensearch-operator` and use `opensearch-cluster.yaml` as a starting point to define your cluster - note that the `clusterName` is also the namespace that the new cluster will reside in. Then run:

```bash
kubectl apply -f opensearch-cluster.yaml
```

Note: the current installation deploys with the default demo certificate provided by OpenSearch.

## Deleting an OpenSearch cluster

In order to delete the cluster, please delete your OpenSearch cluster resource; this will delete the cluster namespace and all its resources.

```bash
kubectl get opensearchclusters --all-namespaces
kubectl delete opensearchclusters my-cluster -n <namespace>
```

# Contributions

We welcome contributions! See how you can get involved [here](https://github.com/opster/opensearch-k8s-operator/blob/main/CONTRIBUTING.md).
