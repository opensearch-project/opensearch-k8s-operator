# opensearch-cluster

A Helm chart for OpenSearch Cluster

## Values

The following table lists the configurable parameters of the Helm chart.

| Parameter | Type | Default | Description |
| --- | ---- | ------- | ----------- |
| `nameOverride` | string | `""` |  |
| `fullnameOverride` | string | `""` |  |
| `serviceAccount.create` | bool | `false` | Create Service Account |
| `serviceAccount.name` | string | `""` | Service Account name. Set `general.serviceAccount` to use this Service Account for the Opensearch cluster |
| `serviceAccount.annotations` | object | `{}` | Service Account annotations |
| `cluster.name` | string | `""` | cluster name, by default release name is used |
| `cluster.annotations` | object | `{}` | cluster annotations |
| `cluster.labels` | object | `{}` | cluster labels |
| `cluster.general.additionalConfig` | object | `{}` | Extra items to add to the opensearch.yml |
| `cluster.general.additionalVolumes` | list | `[]` | Additional volumes to mount to all pods in the cluster. Supported volume types configMap, emptyDir, secret (with default Kubernetes configuration schema) |
| `cluster.general.drainDataNodes` | bool | `true` | Controls whether to drain data notes on rolling restart operations |
| `cluster.general.httpPort` | int | `9200` | Opensearch service http port |
| `cluster.general.image` | string | `"docker.io/opensearchproject/opensearch"` | Opensearch image |
| `cluster.general.imagePullPolicy` | string | `"IfNotPresent"` | Default image pull policy |
| `cluster.general.operatorClusterURL` | string | `""` | Optional custom URL for operator to communicate with OpenSearch This is useful when using external certificates (e.g., from cert-manager) that are valid for a specific FQDN. The operator will use this URL instead of the default internal Kubernetes service DNS name. |
| `cluster.general.keystore` | list | `[]` | Populate opensearch keystore before startup |
| `cluster.general.monitoring.enable` | bool | `false` | Enable cluster monitoring |
| `cluster.general.monitoring.labels` | object | `{}` | Additional labels to apply to the generated ServiceMonitor |
| `cluster.general.monitoring.monitoringUserSecret` | string | `""` | Secret with 'username' and 'password' keys for monitoring user. You could also use OpenSearchUser CRD instead of setting it. |
| `cluster.general.monitoring.pluginUrl` | string | `""` | Custom URL for the monitoring plugin |
| `cluster.general.monitoring.scrapeInterval` | string | `"30s"` | How often to scrape metrics |
| `cluster.general.monitoring.tlsConfig` | object | `{}` | Override the tlsConfig of the generated ServiceMonitor |
| `cluster.general.pluginsList` | list | `[]` | List of Opensearch plugins to install |
| `cluster.general.podSecurityContext` | object | `{}` | Opensearch pod security context configuration |
| `cluster.general.securityContext` | object | `{}` | Opensearch securityContext |
| `cluster.general.serviceAccount` | string | `""` | Opensearch serviceAccount name. If Service Account doesn't exist it could be created by setting `serviceAccount.create` and `serviceAccount.name` |
| `cluster.general.serviceName` | string | `""` | Opensearch service name |
| `cluster.general.setVMMaxMapCount` | bool | `true` | Enable setVMMaxMapCount. OpenSearch requires the Linux kernel vm.max_map_count option to be set to at least 262144 |
| `cluster.general.snapshotRepositories` | list | `[]` | Opensearch snapshot repositories configuration |
| `cluster.general.vendor` | string | `"Opensearch"` |  |
| `cluster.general.version` | string | `"2.3.0"` | Opensearch version |
| `cluster.bootstrap.additionalConfig` | object | `{}` | bootstrap additional configuration, key-value pairs that will be added to the opensearch.yml configuration |
| `cluster.bootstrap.affinity` | object | `{}` | bootstrap pod affinity rules |
| `cluster.bootstrap.annotations` | object | `{}` | bootstrap pod annotations |
| `cluster.bootstrap.jvm` | string | `""` | bootstrap pod jvm options. If jvm is not provided then the java heap size will be set to half of resources.requests.memory which is the recommend value for data nodes. If jvm is not provided and resources.requests.memory does not exist then value will be -Xmx512M -Xms512M |
| `cluster.bootstrap.nodeSelector` | object | `{}` | bootstrap pod node selectors |
| `cluster.bootstrap.resources` | object | `{}` | bootstrap pod cpu and memory resources |
| `cluster.bootstrap.tolerations` | list | `[]` | bootstrap pod tolerations |
| `cluster.confMgmt.smartScaler` | bool | `false` | Enable nodes to be safely removed from the cluster |
| `cluster.dashboards.additionalConfig` | object | `{}` | Additional properties for opensearch_dashboards.yaml |
| `cluster.dashboards.affinity` | object | `{}` | dashboards pod affinity rules |
| `cluster.dashboards.annotations` | object | `{}` | dashboards annotations |
| `cluster.dashboards.basePath` | string | `""` | dashboards Base Path for Opensearch Clusters running behind a reverse proxy |
| `cluster.dashboards.enable` | bool | `true` | Enable dashboards deployment |
| `cluster.dashboards.env` | list | `[]` | dashboards pod env variables |
| `cluster.dashboards.image` | string | `"docker.io/opensearchproject/opensearch-dashboards"` | dashboards image |
| `cluster.dashboards.imagePullPolicy` | string | `"IfNotPresent"` | dashboards image pull policy |
| `cluster.dashboards.imagePullSecrets` | list | `[]` | dashboards image pull secrets |
| `cluster.dashboards.labels` | object | `{}` | dashboards labels |
| `cluster.dashboards.nodeSelector` | object | `{}` | dashboards pod node selectors |
| `cluster.dashboards.opensearchCredentialsSecret` | object | `{}` | Secret that contains fields username and password for dashboards to use to login to opensearch, must only be supplied if a custom securityconfig is provided |
| `cluster.dashboards.pluginsList` | list | `[]` | List of dashboards plugins to install |
| `cluster.dashboards.podSecurityContext` | object | `{}` | dasboards pod security context configuration |
| `cluster.dashboards.replicas` | int | `1` | number of dashboards replicas |
| `cluster.dashboards.resources` | object | `{}` | dashboards pod cpu and memory resources |
| `cluster.dashboards.securityContext` | object | `{}` | dashboards security context configuration |
| `cluster.dashboards.service.loadBalancerSourceRanges` | list | `[]` | source ranges for a loadbalancer |
| `cluster.dashboards.service.type` | string | `"ClusterIP"` | dashboards service type |
| `cluster.dashboards.tls.caSecret` | object | `{}` | Secret that contains the ca certificate as ca.crt. If this and generate=true is set the existing CA cert from that secret is used to generate the node certs. In this case must contain ca.crt and ca.key fields |
| `cluster.dashboards.tls.enable` | bool | `false` | Enable HTTPS for dashboards |
| `cluster.dashboards.tls.generate` | bool | `true` | generate certificate, if false secret must be provided |
| `cluster.dashboards.tls.secret` | string | `nil` | Optional, name of a TLS secret that contains ca.crt, tls.key and tls.crt data. If ca.crt is in a different secret provide it via the caSecret field |
| `cluster.dashboards.tls.duration` | string | `"8760h"` | Duration controls the validity period of generated certificates (e.g. "8760h", "720h"). This is used only when generate is true. |
| `cluster.dashboards.tolerations` | list | `[]` | dashboards pod tolerations |
| `cluster.dashboards.version` | string | `"2.3.0"` | dashboards version |
| `cluster.initHelper.image` | string | `"docker.io/busybox"` | initHelper image |
| `cluster.initHelper.imagePullPolicy` | string | `"IfNotPresent"` | initHelper image pull policy |
| `cluster.initHelper.imagePullSecrets` | list | `[]` | initHelper image pull secret |
| `cluster.initHelper.resources` | object | `{}` | initHelper pod cpu and memory resources |
| `cluster.initHelper.version` | string | `"1.36"` | initHelper version |
| `cluster.nodePools` | list | `[{"annotations":{},"component":"masters","diskSize":"30Gi","replicas":3,"resources":{"limits":{"cpu":"500m","memory":"2Gi"},"requests":{"cpu":"500m","memory":"2Gi"}},"roles":["master","data"],"sidecarContainers":[]}]` | Opensearch nodes configuration |
| `cluster.nodePools[0].annotations` | object | `{}` | node pool pod annotations |
| `cluster.nodePools[0].sidecarContainers` | list | `[]` | These containers will be deployed as sidecars in the same pod as the OpenSearch container |
| `cluster.security.config.adminCredentialsSecret` | object | `{}` | Secret that contains fields username and password to be used by the operator to access the opensearch cluster for node draining. Must be set if custom securityconfig is provided. |
| `cluster.security.config.adminSecret` | object | `{}` | TLS Secret that contains a client certificate (tls.key, tls.crt, ca.crt) with admin rights in the opensearch cluster. Must be set if http certificates are provided by user and not generated |
| `cluster.security.config.securityConfigSecret` | object | `{}` | Secret that contains the differnt yml files of the opensearch-security config (config.yml, internal_users.yml, etc) |
| `cluster.security.tls.http.adminDn` | list | `[]` | DNs of certificates that should have admin access, mainly used for securityconfig updates via securityadmin.sh, only used when existing certificates are provided |
| `cluster.security.tls.http.caSecret` | object | `{}` | Optional, secret that contains the ca certificate as ca.crt. If this and generate=true is set the existing CA cert from that secret is used to generate the node certs. In this case must contain ca.crt and ca.key fields |
| `cluster.security.tls.http.generate` | bool | `true` | If set to true the operator will generate a CA and certificates for the cluster to use, if false - secrets with existing certificates must be supplied |
| `cluster.security.tls.http.customFQDN` | string | `""` | Optional custom FQDN to use for the HTTP certificate. If provided, this FQDN will be used as the primary DNS name in the certificate's Subject Alternative Names (SAN), along with the default cluster DNS names. This allows you to use a single certificate that works with both your custom domain and the internal Kubernetes DNS names. |
| `cluster.security.tls.http.secret` | object | `{}` | Optional, name of a TLS secret that contains ca.crt, tls.key and tls.crt data. If ca.crt is in a different secret provide it via the caSecret field |
| `cluster.security.tls.http.duration` | string | `"8760h"` | Duration controls the validity period of generated certificates (e.g. "8760h", "720h"). This is used only when generate is true. |
| `cluster.security.tls.transport.caSecret` | object | `{}` | Optional, secret that contains the ca certificate as ca.crt. If this and generate=true is set the existing CA cert from that secret is used to generate the node certs. In this case must contain ca.crt and ca.key fields |
| `cluster.security.tls.transport.generate` | bool | `true` | If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied |
| `cluster.security.tls.transport.nodesDn` | list | `[]` | Allowed Certificate DNs for nodes, only used when existing certificates are provided |
| `cluster.security.tls.transport.perNode` | bool | `true` | Separate certificate per node |
| `cluster.security.tls.transport.secret` | object | `{}` | Optional, name of a TLS secret that contains ca.crt, tls.key and tls.crt data. If ca.crt is in a different secret provide it via the caSecret field |
| `cluster.security.tls.transport.duration` | string | `"8760h"` | Duration controls the validity period of generated certificates (e.g. "8760h", "720h"). This is used only when generate is true. |
| `cluster.ingress.opensearch.enabled` | bool | `false` | Enable ingress for Opensearch service |
| `cluster.ingress.opensearch.annotations` | object | `{}` | Opensearch ingress annotations |
| `cluster.ingress.opensearch.className` | string | `""` | Opensearch Ingress class name |
| `cluster.ingress.opensearch.hosts` | list | `[]` | Opensearch Ingress hostnames |
| `cluster.ingress.opensearch.tls` | list | `[]` | Opensearch tls configuration |
| `cluster.ingress.dashboards.enabled` | bool | `false` | Enable ingress for dashboards service |
| `cluster.ingress.dashboards.annotations` | object | `{}` | dashboards ingress annotations |
| `cluster.ingress.dashboards.className` | string | `""` | Ingress class name |
| `cluster.ingress.dashboards.hosts` | list | `[]` | Ingress hostnames |
| `cluster.ingress.dashboards.tls` | list | `[]` | Ingress tls configuration |
| `roles` | list | `[]` | List of OpensearchRole. Check values.yaml file for examples. |
| `users` | list | `[]` | List of OpensearchUser. Check values.yaml file for examples. |
| `usersRoleBinding` | list | `[]` | Allows to link any number of users, backend roles and roles with a OpensearchUserRoleBinding. Each user in the binding will be granted each role Check values.yaml file for examples. |
| `tenants` | list | `[]` | List of additional tenants. Check values.yaml file for examples. |
| `actionGroups` | list | `[]` | List of OpensearchActionGroup. Check values.yaml file for examples. |
| `componentTemplates` | list | `[]` | List of OpensearchComponentTemplate. Check values.yaml file for examples. |
| `indexTemplates` | list | `[]` | List of OpensearchIndexTemplate. Check values.yaml file for examples. |
| `ismPolicies` | list | `[]` | List of OpenSearchISMPolicy. Check values.yaml file for examples. |

Specify each parameter using the `--set key=value[,key=value]` argument to `helm install`.

Opensearch-cluster Helm Chart version: `3.0.0`
