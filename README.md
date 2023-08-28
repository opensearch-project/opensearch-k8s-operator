![build](https://github.com/opster/opensearch-k8s-operator/actions/workflows/docker-build.yaml/badge.svg) ![test](https://github.com/opster/opensearch-k8s-operator/actions/workflows/testing.yaml/badge.svg) ![release](https://img.shields.io/github/v/release/opster/opensearch-k8s-operator) [![Golang Lint](https://github.com/Opster/opensearch-k8s-operator/actions/workflows/linting.yaml/badge.svg)](https://github.com/Opster/opensearch-k8s-operator/actions/workflows/linting.yaml) [![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/opensearch-operator)](https://artifacthub.io/packages/search?repo=opensearch-operator)

# OpenSearch Kubernetes Operator

The Kubernetes OpenSearch Operator is used for automating the deployment, provisioning, management, and orchestration of OpenSearch clusters and OpenSearch dashboards.

To get all the capabilities of the Operator with a UI, you can use the free [Opster Management Console](https://opster.com/opensearch-opster-management-console/). Beyond being able to easily carry out all of the actions provided by the Operator, it also include additional features like monitoring and more.

## Getting started

The Operator can be easily installed using helm on any CNCF-certified Kubernetes cluster. Please refer to the [User Guide](./docs/userguide/main.md) for installation instructions.

## Current feature list

Features:

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
- [x] Operator Monitoring, with Prometheus and Grafana.
- [ ] Auto scaler based on usage, load, and resources.
- [ ] Ability to deploy data prepper solutions (logstash).

A full roadmap is available in the [Development plan](./docs/designs/dev-plan.md).

## Installation

The Operator can be easily installed using Helm:

1. Add the helm repo: `helm repo add opensearch-operator https://opster.github.io/opensearch-k8s-operator/`
2. Install the Operator: `helm install opensearch-operator opensearch-operator/opensearch-operator`

## OpenSearch Kubernetes Operator installation & demo video

[![Watch the video](https://opster.com/wp-content/uploads/2022/05/Operator-Installation-Tutorial.png)](https://player.vimeo.com/video/708641527)

## Manage OpenSearch on K8s through a single interface (UI)

To get all the capabilities of the Operator with a UI, you can use the free [Opster Management Console](https://opster.com/opensearch-opster-management-console/). Beyond being able to easily carry out all of the actions provided by the Operator, it also include additional features like monitoring and more.

[![Watch the video](https://opster.com/wp-content/uploads/2023/04/Screen-cap-for-omc-video.png)](https://player.vimeo.com/video/767761262)

## Compatibility

The opensearch k8s operator aims to be compatible to all supported opensearch versions. Please check the table below for details:

| Operator Version | Min Supported Opensearch Version | Max supported Opensearch version | Comment |
|------------------|----------------------------------|----------------------------------|---------|
| 2.3              | 1.0                              | 2.8                              |         |
| 2.2              | 1.0                              | 2.5                              |         |
| 2.1              | 1.0                              | 2.3                              |         |
| 2.0              | 1.0                              | 2.3                              |         |
| 1.x              | 1.0                              | 1.x                              |         |
| 0.x              | 1.0                              | 1.x                              | Beta    |

This table only lists versions that have been explicitly tested with the operator, the operator will not prevent you from using other versions. Newer minor versions (2.x) not listed here generally also work but you should proceed with caution and test it our in a non-production environment first.

## Development

If you want to develop the operator, please see the separate [developer docs](./docs/developing.md).

## Contributions

We welcome contributions! See how you can get involved by reading [CONTRIBUTING.md](./CONTRIBUTING.md).
