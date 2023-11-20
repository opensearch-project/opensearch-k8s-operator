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

```bash
helm repo update
helm upgrade [RELEASE_NAME] opensearch-operator/opensearch-cluster
```

## Configuring OpenSearch Cluster

By default, the installation will deploy a node pool consisting of three master nodes with the dashboard enabled. For the entire configuration, check [helm chart values](../../charts/opensearch-cluster/values.yaml).

To further customize your OpenSearchCluster installation, you can utilize configuration overrides and modify your `values.yaml`, this allows you to tailor various aspects of the installation to meet your specific requirements.
For instance, if you need to change the httpPort to 9300, this can be achieved by setting `OpenSearchClusterSpec.general.httpPort` to `9300` in the [helm chart values](../../charts/opensearch-cluster/values.yaml).

```yaml
OpenSearchClusterSpec:
  general:
    httpPort: "9300"
    version: 2.3.0
    serviceName: "my-cluster"
```
