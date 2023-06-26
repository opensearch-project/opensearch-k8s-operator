# OpenSearch-k8s-operator

The Kubernetes [OpenSearch Operator](https://github.com/Opster/opensearch-k8s-operator) is used for automating the deployment, provisioning, management, and orchestration of OpenSearch clusters and OpenSearch dashboards.

## Install OpenSearchCluster Using Helm
The Operator can be easily installed using helm on any CNCF-certified Kubernetes cluster. Please refer to the [User Guide](https://github.com/Opster/opensearch-k8s-operator/blob/main/docs/userguide/main.md) for more information.
Once the operator is installed, OpenSearch cluster can be installed using helm in the same CNCF-certified Kubernetes cluster. 

### OpenSearchCluster Installation Using Helm

#### Install Chart
```
helm install [RELEASE_NAME] opensearch-operator/opensearch-cluster
```
#### Uninstall Chart
```
helm uninstall [RELEASE_NAME]
```
#### Upgrade Chart
```
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


