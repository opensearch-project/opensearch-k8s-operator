![build](https://github.com/opster/opensearch-k8s-operator/actions/workflows/docker-build.yaml/badge.svg) ![test](https://github.com/opster/opensearch-k8s-operator/actions/workflows/testing.yaml/badge.svg) ![release](https://img.shields.io/github/v/release/opster/opensearch-k8s-operator)

# OpenSearch-k8s-operator

The Kubernetes OpenSearch Operator is used for automating the deployment, provisioning, management, and orchestration of OpenSearch clusters and OpenSearch dashboards.

## Getting started

The Operator can be easily installed using helm on any CNCF-certified Kubernetes cluster. Please refer to the [User Guide](./docs/userguide/main.md) for installation instructions.

## Roadmap

The full roadmap is available in the [Development plan](./docs/designs/dev-plan.md).

Currently planned features:

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
- [x] Scaling nodes' disks - increase disk size.
- [ ] Cluster configurations and nodes' settings updates.
- [ ] Auto scaler based on usage, load, and resources.
- [ ] Operator Monitoring, with Prometheus and Grafana.
- [ ] Control shard balancing and allocation: AZ/Rack awareness, Hot/Warm.

## Development

### Running the Operator locally

- Clone the repo and go to the `opensearch-operator` folder.
- Run `make build manifests` to build the controller binary and the manifests
- Start a Kubernetes cluster (e.g. with k3d or minikube) and make sure your `~/.kube/config` points to it
- Run `make install` to create the CRD in the kubernetes cluster
- Start the Operator by running `make run`

**Note: use GO 1.17 version** 

Now you can deploy an Opensearch cluster.

Go to `opensearch-operator` and use `opensearch-cluster.yaml` as a starting point to define your cluster. Then run:

```bash
kubectl apply -f opensearch-cluster.yaml
```

In order to delete the cluster, you just delete your OpenSearch cluster resource. This will delete the cluster and all of its resources.

```bash
kubectl delete -f opensearch-cluster.yaml
```
## Installation Using Helm

# Get Repo Info
```
helm repo add opensearch-operator https://opster.github.io/opensearch-k8s-operator/
helm repo update
```

# Install Chart
```
helm install [RELEASE_NAME] opensearch-operator/opensearch-operator
```

# Uninstall Chart
```
helm uninstall [RELEASE_NAME]
```

# Upgrade Chart
```
helm upgrade [RELEASE_NAME] opensearch-operator/opensearch-operator
```

## Installation Tutorial and Demo

[![Watch the video](https://opster.com/wp-content/uploads/2022/05/Operator-Installation-Tutorial.png)](https://player.vimeo.com/video/708641527)

## Contributions

We welcome contributions! See how you can get involved by reading [CONTRIBUTING.md](./CONTRIBUTING.md).
