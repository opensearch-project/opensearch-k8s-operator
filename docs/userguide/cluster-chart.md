# Install OpenSearchCluster Using Helm

After installing the operator (please refer to the [User Guide](./main.md) for details) you can deploy OpenSearch clusters using a separate helm chart, `opensearch-cluster`. It is published in the same Helm repository as the operator chart.

## Install Chart

```bash
helm repo add opensearch-operator https://opensearch-project.github.io/opensearch-k8s-operator/
helm repo update
helm install [RELEASE_NAME] opensearch-operator/opensearch-cluster
```

The `OpenSearchCluster` is named after the release unless you set `cluster.name`.

## Uninstall Chart

```bash
helm uninstall [RELEASE_NAME]
```

## Upgrade Chart

```bash
helm repo update
helm upgrade [RELEASE_NAME] opensearch-operator/opensearch-cluster
```

### Upgrading from chart version 2.x

Chart version 3.0.0 was a full refactor of the chart. This is the chart's own version and is unrelated to the operator version. Before upgrading from a 2.x chart, compare your configuration with the [default chart values](../../charts/opensearch-cluster/values.yaml).

The `opensearchCluster` value was replaced by `cluster`. The configuration structure of each custom resource (OpenSearchCluster, OpensearchIndexTemplate, etc.) follows the corresponding CRD documentation.

**Make sure to test the upgrade process on a non-production environment first.**

## Configuring OpenSearch Cluster

By default, the chart deploys one node pool named `masters` with 3 replicas and the `master` and `data` roles (30Gi disk, 2Gi memory), plus one Dashboards replica. The OpenSearch and Dashboards version defaults to the chart's `appVersion`; set `cluster.general.version` and `cluster.dashboards.version` to pin it. For all available values, see the [chart README](../../charts/opensearch-cluster/README.md) and [values.yaml](../../charts/opensearch-cluster/values.yaml).

The values under `cluster` use the same field names as the `OpenSearchCluster` spec described in the [User Guide](./main.md), with one exception for images. `cluster.general.image` and `cluster.dashboards.image` are repository names without a tag (the chart default is `docker.io/opensearchproject/opensearch` and `docker.io/opensearchproject/opensearch-dashboards`). The chart renders each as `<image>:<version>`, using `cluster.general.version` or `cluster.dashboards.version` when set and the chart `appVersion` otherwise. A full reference such as `myregistry.example.com/opensearch:3.2.0` is rendered with a second tag appended. In the `OpenSearchCluster` spec, `general.image` is a full image reference; a per-node-pool `image` in this chart is passed through unchanged.

Other values worth knowing:

- `apiGroup`: the API group of the rendered resources. Defaults to `opensearch.org`; `opensearch.opster.io` is deprecated, and the operator's legacy webhooks reject creating resources in it (see the [Migration Guide](./migration-guide.md)).
- `cluster.ingress.opensearch` and `cluster.ingress.dashboards`: create an Ingress for the cluster Service and the Dashboards Service. Each node pool can also get its own Ingress through `cluster.nodePools[].ingress`.
- `users`, `roles`, `usersRoleBinding`, `tenants`, `actionGroups`, `componentTemplates`, `indexTemplates` and `ismPolicies`: create the corresponding security, template and ISM resources. They reference the chart's cluster automatically.

The chart has no template for snapshot policies. Create `OpensearchSnapshotPolicy` resources separately, as described in the [User Guide](./main.md).
