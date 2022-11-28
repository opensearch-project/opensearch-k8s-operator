![build](https://github.com/opster/opensearch-k8s-operator/actions/workflows/docker-build.yaml/badge.svg) ![test](https://github.com/opster/opensearch-k8s-operator/actions/workflows/testing.yaml/badge.svg) ![release](https://img.shields.io/github/v/release/opster/opensearch-k8s-operator) [![Golang Lint](https://github.com/Opster/opensearch-k8s-operator/actions/workflows/linting.yaml/badge.svg)](https://github.com/Opster/opensearch-k8s-operator/actions/workflows/linting.yaml) [![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/opensearch-operator)](https://artifacthub.io/packages/search?repo=opensearch-operator)

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
- [x] Cluster configurations and nodes' settings updates.
- [ ] Operator Monitoring, with Prometheus and Grafana.
- [ ] Auto scaler based on usage, load, and resources.
- [ ] Control shard balancing and allocation: AZ/Rack awareness, Hot/Warm.

## Installation

The Operator can be easily installed using Helm:

1. Add the helm repo: `helm repo add opensearch-operator https://opster.github.io/opensearch-k8s-operator/`
2. Install the Operator: `helm install opensearch-operator opensearch-operator/opensearch-operator`

## Compatibility

The opensearch k8s operator aims to be compatible to all supported opensearch versions. Please check the table below for details:

| Operator Version | Min Supported Opensearch Version | Max supported Opensearch version | Comment |
|------------------|----------------------------------|----------------------------------|---------|
| 2.1              | 1.0                              | 2.3                              |         |
| 2.0              | 1.0                              | 2.3                              |         |
| 1.x              | 1.0                              | 1.x                              |         |
| 0.x              | 1.0                              | 1.x                              | Beta    |

## Development

If you want to develop the operator, please see the separate [developer docs](./docs/developing.md).

## Installation Tutorial and Demo

[![Watch the video](https://opster.com/wp-content/uploads/2022/05/Operator-Installation-Tutorial.png)](https://player.vimeo.com/video/708641527)

## Contributions

We welcome contributions! See how you can get involved by reading [CONTRIBUTING.md](./CONTRIBUTING.md).
