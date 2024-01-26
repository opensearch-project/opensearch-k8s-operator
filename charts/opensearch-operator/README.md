# OpenSearch-k8s-operator

The Kubernetes [OpenSearch Operator](https://github.com/Opster/opensearch-k8s-operator) is used for automating the deployment, provisioning, management, and orchestration of OpenSearch clusters and OpenSearch dashboards.

## Getting started

The Operator can be easily installed using helm on any CNCF-certified Kubernetes cluster. Please refer to the [User Guide](https://github.com/Opster/opensearch-k8s-operator/blob/main/docs/userguide/main.md) for more information.

### Installation Using Helm

#### Get Repo Info
```
helm repo add opensearch-operator https://opster.github.io/opensearch-k8s-operator/
helm repo update
```
#### Install Chart
```
helm install [RELEASE_NAME] opensearch-operator/opensearch-operator
```
#### Uninstall Chart
```
helm uninstall [RELEASE_NAME]
```
#### Upgrade Chart
```
helm repo update
helm upgrade [RELEASE_NAME] opensearch-operator/opensearch-operator
```

## Installation Tutorial and Demo

[![Watch the video](https://opster.com/wp-content/uploads/2022/05/Operator-Installation-Tutorial.png)](https://player.vimeo.com/video/708641527)

