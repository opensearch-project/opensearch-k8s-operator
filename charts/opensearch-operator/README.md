# opensearch-operator

The OpenSearch Operator Helm chart for Kubernetes

## Getting started

The Operator can be easily installed using helm on any CNCF-certified Kubernetes cluster. Please refer to the [User Guide](https://github.com/opensearch-project/opensearch-k8s-operator/blob/main/docs/userguide/main.md) for more information.

### Installation Using Helm

#### Get Repo Info

```shell
helm repo add opensearch-operator https://opensearch-project.github.io/opensearch-k8s-operator/
helm repo update
```

#### Install Chart

```shell
helm install [RELEASE_NAME] opensearch-operator/opensearch-operator
```

#### Uninstall Chart

```shell
helm uninstall [RELEASE_NAME]
```

#### Upgrade Chart

```shell
helm repo update
helm upgrade [RELEASE_NAME] opensearch-operator/opensearch-operator
```

## Values

The following table lists the configurable parameters of the Helm chart.

| Parameter | Type | Default | Description |
| --- | ---- | ------- | ----------- |
| `nameOverride` | string | `""` |  |
| `fullnameOverride` | string | `""` |  |
| `podAnnotations` | object | `{}` |  |
| `podLabels` | object | `{}` |  |
| `nodeSelector` | object | `{}` |  |
| `tolerations` | list | `[]` |  |
| `securityContext.runAsNonRoot` | bool | `true` |  |
| `priorityClassName` | string | `""` |  |
| `manager.securityContext.allowPrivilegeEscalation` | bool | `false` |  |
| `manager.extraEnv` | list | `[]` |  |
| `manager.resources.limits.cpu` | string | `"1000m"` |  |
| `manager.resources.limits.memory` | string | `"500Mi"` |  |
| `manager.resources.requests.cpu` | string | `"200m"` |  |
| `manager.resources.requests.memory` | string | `"350Mi"` |  |
| `manager.livenessProbe.failureThreshold` | int | `3` |  |
| `manager.livenessProbe.httpGet.path` | string | `"/healthz"` |  |
| `manager.livenessProbe.httpGet.port` | int | `8081` |  |
| `manager.livenessProbe.periodSeconds` | int | `15` |  |
| `manager.livenessProbe.successThreshold` | int | `1` |  |
| `manager.livenessProbe.timeoutSeconds` | int | `3` |  |
| `manager.livenessProbe.initialDelaySeconds` | int | `10` |  |
| `manager.readinessProbe.failureThreshold` | int | `3` |  |
| `manager.readinessProbe.httpGet.path` | string | `"/readyz"` |  |
| `manager.readinessProbe.httpGet.port` | int | `8081` |  |
| `manager.readinessProbe.periodSeconds` | int | `15` |  |
| `manager.readinessProbe.successThreshold` | int | `1` |  |
| `manager.readinessProbe.timeoutSeconds` | int | `3` |  |
| `manager.readinessProbe.initialDelaySeconds` | int | `10` |  |
| `manager.parallelRecoveryEnabled` | bool | `true` |  |
| `manager.pprofEndpointsEnabled` | bool | `false` |  |
| `manager.image.repository` | string | `"opensearchproject/opensearch-operator"` |  |
| `manager.image.tag` | string | `""` |  |
| `manager.image.pullPolicy` | string | `"Always"` |  |
| `manager.imagePullSecrets` | list | `[]` |  |
| `manager.dnsBase` | string | `"cluster.local"` |  |
| `manager.loglevel` | string | `"info"` |  |
| `manager.watchNamespace` | string | `nil` |  |
| `manager.metricsBindAddress` | string | `"127.0.0.1:8080"` |  |
| `installCRDs` | bool | `true` |  |
| `serviceAccount.create` | bool | `true` |  |
| `serviceAccount.name` | string | `""` |  |
| `useRoleBindings` | bool | `false` |  |
| `webhook.enabled` | bool | `true` |  |
| `webhook.port` | int | `9443` |  |
| `webhook.failurePolicy` | string | `"Fail"` |  |
| `webhook.secretName` | string | `""` |  |
| `webhook.certManager.enabled` | bool | `true` |  |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`.

## Referencing an External OpenSearch Cluster

The operator can manage resources on an OpenSearch cluster running **outside Kubernetes** (bare metal, VMs, managed cloud service) without deploying any infrastructure itself. When `externalClusterURL` is set on an `OpenSearchCluster` resource, the operator skips all infrastructure reconcilers (TLS, StatefulSets, Services, etc.) and connects directly to the provided hostname.

### How it works

- The operator marks the cluster as initialized immediately — no nodes need to be ready
- Only reconcilers that operate via the OpenSearch API are executed (currently: snapshot repositories)
- `nodePools` and `serviceName` are not required when `externalClusterURL` is set
- Deleting the `OpenSearchCluster` object does not affect the external cluster

### Example

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-external-credentials
  namespace: my-namespace
type: Opaque
stringData:
  username: admin
  password: my-secure-password
---
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
metadata:
  name: my-external-cluster
  namespace: my-namespace
spec:
  general:
    httpPort: 9200
    externalClusterURL: "my-opensearch.example.com"
    # externalClusterScheme defaults to "https". Set to "http" for unencrypted connections.
    # externalClusterScheme: http
  security:
    config:
      adminCredentialsSecret:
        name: my-external-credentials
```

### Fields

| Field | Type | Default | Description |
|---|---|---|---|
| `spec.general.externalClusterURL` | string | — | Hostname of the external cluster, without scheme or port (e.g. `my-opensearch.example.com`). When set, all infrastructure reconcilers are skipped. |
| `spec.general.externalClusterScheme` | `https` \| `http` | `https` | Scheme used to connect to the external cluster. |
| `spec.general.httpPort` | int | `9200` | Port used to connect to the external cluster. |
| `spec.security.config.adminCredentialsSecret` | LocalObjectReference | — | Secret containing `username` and `password` fields used by the operator to authenticate against the cluster. |

### Difference between `externalClusterURL` and `operatorClusterURL`

| | `operatorClusterURL` | `externalClusterURL` |
|---|---|---|
| **Purpose** | Override the URL the operator uses to reach an in-cluster OpenSearch node | Point the operator to a cluster running entirely outside Kubernetes |
| **Infrastructure management** | Normal (StatefulSets, TLS, Services are created) | None (all infrastructure reconcilers are skipped) |
| **Use case** | Custom FQDN for TLS certificates (e.g. cert-manager) | Bare metal, VM, or managed cloud OpenSearch |

### What is and is not managed for external clusters

| Managed via OpenSearch API | Not managed |
|---|---|
| Snapshot repositories (`spec.general.snapshotRepositories`) | TLS certificates |
| | StatefulSets, Services, ConfigMaps |
| | Rolling restarts and version upgrades |
| | OpenSearch Dashboards |

> **Note**: If you also define `nodePools` alongside `externalClusterURL`, the operator emits a Kubernetes `Warning` event and ignores the node pools entirely.

> **Deletion behaviour**: deleting the `OpenSearchCluster` object only removes the Kubernetes resource. The external OpenSearch cluster is not affected.

## Namespace-scoped RBAC

By default, the operator uses cluster-scoped RBAC resources (ClusterRole and ClusterRoleBinding). If you want to restrict the operator's permissions to a specific namespace, you can enable namespace-scoped RBAC by setting `useRoleBindings: true`.

### Installation with namespace-scoped RBAC

```shell
helm install opensearch-operator opensearch-operator/opensearch-operator \
  --set useRoleBindings=true \
  --set manager.watchNamespace=<your-namespace>
```

### Important considerations

When `useRoleBindings` is enabled:

- **Manager and proxy roles** will be created as namespace-scoped `Role` resources instead of `ClusterRole`
- **Metrics ClusterRole** will NOT be created, as Kubernetes does not allow namespace-scoped Roles to grant permissions to non-resource URLs (like `/metrics`)
- **Metrics endpoint access**: The operator's `/metrics` endpoint is exposed via the kube-apiserver and requires authentication (via `TokenReviews`) and authorization (via `SubjectAccessReviews`). If you need to access metrics with monitoring tools (e.g., Prometheus), you must manually create the appropriate `ClusterRole` and `ClusterRoleBinding`:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: opensearch-operator-metrics-reader
rules:
- nonResourceURLs:
  - /metrics
  verbs:
  - get
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: opensearch-operator-metrics-reader
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: opensearch-operator-metrics-reader
subjects:
- kind: ServiceAccount
  name: <your-monitoring-service-account>
  namespace: <monitoring-namespace>
```

Opensearch-operator Helm Chart version: `3.1.0`
