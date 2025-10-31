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
| `manager.resources.limits.cpu` | string | `"200m"` |  |
| `manager.resources.limits.memory` | string | `"500Mi"` |  |
| `manager.resources.requests.cpu` | string | `"100m"` |  |
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
| `kubeRbacProxy.enable` | bool | `true` |  |
| `kubeRbacProxy.securityContext.allowPrivilegeEscalation` | bool | `false` |  |
| `kubeRbacProxy.securityContext.readOnlyRootFilesystem` | bool | `true` |  |
| `kubeRbacProxy.securityContext.capabilities.drop[0]` | string | `"ALL"` |  |
| `kubeRbacProxy.resources.limits.cpu` | string | `"50m"` |  |
| `kubeRbacProxy.resources.limits.memory` | string | `"50Mi"` |  |
| `kubeRbacProxy.resources.requests.cpu` | string | `"25m"` |  |
| `kubeRbacProxy.resources.requests.memory` | string | `"25Mi"` |  |
| `kubeRbacProxy.livenessProbe.failureThreshold` | int | `3` |  |
| `kubeRbacProxy.livenessProbe.httpGet.path` | string | `"/healthz"` |  |
| `kubeRbacProxy.livenessProbe.httpGet.port` | int | `10443` |  |
| `kubeRbacProxy.livenessProbe.httpGet.scheme` | string | `"HTTPS"` |  |
| `kubeRbacProxy.livenessProbe.periodSeconds` | int | `15` |  |
| `kubeRbacProxy.livenessProbe.successThreshold` | int | `1` |  |
| `kubeRbacProxy.livenessProbe.timeoutSeconds` | int | `3` |  |
| `kubeRbacProxy.livenessProbe.initialDelaySeconds` | int | `10` |  |
| `kubeRbacProxy.readinessProbe.failureThreshold` | int | `3` |  |
| `kubeRbacProxy.readinessProbe.httpGet.path` | string | `"/healthz"` |  |
| `kubeRbacProxy.readinessProbe.httpGet.port` | int | `10443` |  |
| `kubeRbacProxy.readinessProbe.httpGet.scheme` | string | `"HTTPS"` |  |
| `kubeRbacProxy.readinessProbe.periodSeconds` | int | `15` |  |
| `kubeRbacProxy.readinessProbe.successThreshold` | int | `1` |  |
| `kubeRbacProxy.readinessProbe.timeoutSeconds` | int | `3` |  |
| `kubeRbacProxy.readinessProbe.initialDelaySeconds` | int | `10` |  |
| `kubeRbacProxy.image.repository` | string | `"gcr.io/kubebuilder/kube-rbac-proxy"` |  |
| `kubeRbacProxy.image.tag` | string | `"v0.15.0"` |  |
| `useRoleBindings` | bool | `false` |  |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`.

Opensearch-operator Helm Chart version: `2.8.0`
