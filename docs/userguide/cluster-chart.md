# Install OpenSearchCluster Using Helm

After installing the operator (please refer to the [User Guide](./main.md) for details) you can deploy OpenSearch clusters using a separate helm chart.

## Install Chart

```bash
helm install [RELEASE_NAME] opensearch-operator/opensearch-cluster
```

## Uninstall Chart

```bash
helm uninstall [RELEASE_NAME]
```

## Upgrade Chart

### Upgrading to version 3

Version 3.0.0 of opensearch-cluster helm chart is a fully refactored chart. Before upgrading to v3 check that [default chart values](../../charts/opensearch-cluster/values.yaml)
matches with your configuration.

In v3 `opensearchCluster` variable was replaced by `cluster`. The configuration structure of each custom resource (OpenSearchCluster, OpensearchIndexTemplate, etc) follows the corresponding CRD documentation.

**Make sure to test the upgrade process on none-production environment first.**

If the cluster was installed by using the default `values.yaml`, then the upgrade could be done by running:

```bash
helm repo update
helm upgrade [RELEASE_NAME] opensearch-operator/opensearch-cluster
```

## Configuring OpenSearch Cluster

By default, the installation will deploy a node pool consisting of three master nodes with the dashboard enabled. For the entire configuration, check [helm chart values](../../charts/opensearch-cluster/values.yaml).

To further customize your OpenSearchCluster installation, you can utilize configuration overrides and modify your `values.yaml`, this allows you to tailor various aspects of the installation to meet your specific requirements.
Version 3 of the helm chart is designed to have configuration options with the same format and naming as it is defined in the operator doc.
