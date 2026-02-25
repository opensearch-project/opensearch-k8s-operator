# API Reference

## Packages
- [opensearch.opster.io/v1](#opensearchopsteriov1)
- [opensearch.org/v1](#opensearchorgv1)


## opensearch.opster.io/v1

Package v1 contains API Schema definitions for the opster v1 API group

DEPRECATED: The opensearch.opster.io API group is deprecated and will be removed
in a future release. Please migrate to opensearch.org/v1 API group.
See docs/userguide/migration-guide.md for migration instructions.


### Resource Types
- [OpenSearchCluster](#opensearchcluster)
- [OpenSearchISMPolicy](#opensearchismpolicy)
- [OpensearchActionGroup](#opensearchactiongroup)
- [OpensearchComponentTemplate](#opensearchcomponenttemplate)
- [OpensearchIndexTemplate](#opensearchindextemplate)
- [OpensearchRole](#opensearchrole)
- [OpensearchSnapshotPolicy](#opensearchsnapshotpolicy)
- [OpensearchTenant](#opensearchtenant)
- [OpensearchUser](#opensearchuser)
- [OpensearchUserRoleBinding](#opensearchuserrolebinding)



#### Action



Actions are the steps that the policy sequentially executes on entering a specific state.



_Appears in:_
- [State](#state)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `alias` _[Alias](#alias)_ |  |  |  |
| `allocation` _[Allocation](#allocation)_ | Allocate the index to a node with a specific attribute set |  |  |
| `close` _[Close](#close)_ | Closes the managed index. |  |  |
| `delete` _[Delete](#delete)_ | Deletes a managed index. |  |  |
| `forceMerge` _[ForceMerge](#forcemerge)_ | Reduces the number of Lucene segments by merging the segments of individual shards. |  |  |
| `indexPriority` _[IndexPriority](#indexpriority)_ | Set the priority for the index in a specific state. |  |  |
| `notification` _[Notification](#notification)_ | Name          string        `json:"name,omitempty"` |  |  |
| `open` _[Open](#open)_ | Opens a managed index. |  |  |
| `readOnly` _[ReadOnly](#readonly)_ | Sets a managed index to be read only. |  |  |
| `readWrite` _[ReadWrite](#readwrite)_ | Sets a managed index to be writeable. |  |  |
| `replicaCount` _[ReplicaCount](#replicacount)_ | Sets the number of replicas to assign to an index. |  |  |
| `retry` _[Retry](#retry)_ | The retry configuration for the action. |  |  |
| `rollover` _[Rollover](#rollover)_ | Rolls an alias over to a new index when the managed index meets one of the rollover conditions. |  |  |
| `rollup` _[Rollup](#rollup)_ | Periodically reduce data granularity by rolling up old data into summarized indexes. |  |  |
| `shrink` _[Shrink](#shrink)_ | Allows you to reduce the number of primary shards in your indexes |  |  |
| `snapshot` _[Snapshot](#snapshot)_ | Back up your cluster’s indexes and state |  |  |
| `timeout` _string_ | The timeout period for the action. Accepts time units for minutes, hours, and days. |  |  |


#### AdditionalVolume







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name to use for the volume. Required. |  |  |
| `path` _string_ | Path in the container to mount the volume at. Required. |  |  |
| `subPath` _string_ | SubPath of the referenced volume to mount. |  |  |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretvolumesource-v1-core)_ | Secret to use populate the volume |  |  |
| `configMap` _[ConfigMapVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapvolumesource-v1-core)_ | ConfigMap to use to populate the volume |  |  |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ | EmptyDir to use to populate the volume |  |  |
| `csi` _[CSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#csivolumesource-v1-core)_ | CSI object to use to populate the volume |  |  |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimvolumesource-v1-core)_ | PersistentVolumeClaim object to use to populate the volume |  |  |
| `projected` _[ProjectedVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#projectedvolumesource-v1-core)_ | Projected object to use to populate the volume |  |  |
| `nfs` _[NFSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#nfsvolumesource-v1-core)_ | NFS object to use to populate the volume |  |  |
| `restartPods` _boolean_ | Whether to restart the pods on content change |  |  |


#### Alias







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `actions` _[AliasAction](#aliasaction) array_ | Allocate the index to a node with a specified attribute. |  |  |


#### AliasAction







_Appears in:_
- [Alias](#alias)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `add` _[AliasDetails](#aliasdetails)_ |  |  |  |
| `remove` _[AliasDetails](#aliasdetails)_ |  |  |  |




#### Allocation







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `exclude` _string_ | Allocate the index to a node with a specified attribute. |  |  |
| `include` _string_ | Allocate the index to a node with any of the specified attributes. |  |  |
| `require` _string_ | Don’t allocate the index to a node with any of the specified attributes. |  |  |
| `waitFor` _string_ | Wait for the policy to execute before allocating the index to a node with a specified attribute. |  |  |


#### BootstrapConfig







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `jvm` _string_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `pluginsList` _string array_ |  |  |  |
| `keystore` _[KeystoreValue](#keystorevalue) array_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ |  |  |  |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#container-v1-core) array_ |  |  | Schemaless: \{\} <br /> |
| `hostAliases` _[HostAlias](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostalias-v1-core) array_ |  |  |  |
| `diskSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quantity-resource-api)_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |


#### Close







_Appears in:_
- [Action](#action)



#### ClusterSpec



ClusterSpec defines the desired state of OpenSearchCluster



_Appears in:_
- [OpenSearchCluster](#opensearchcluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `general` _[GeneralConfig](#generalconfig)_ | INSERT ADDITIONAL SPEC FIELDS - desired state of cluster<br />Important: Run "make" to regenerate code after modifying this file |  |  |
| `confMgmt` _[ConfMgmt](#confmgmt)_ |  |  |  |
| `bootstrap` _[BootstrapConfig](#bootstrapconfig)_ |  |  |  |
| `dashboards` _[DashboardsConfig](#dashboardsconfig)_ |  |  |  |
| `security` _[Security](#security)_ |  |  |  |
| `nodePools` _[NodePool](#nodepool) array_ |  |  |  |
| `initHelper` _[InitHelperConfig](#inithelperconfig)_ |  |  |  |


#### CommandProbeConfig







_Appears in:_
- [ProbesConfig](#probesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `initialDelaySeconds` _integer_ |  |  |  |
| `periodSeconds` _integer_ |  |  |  |
| `timeoutSeconds` _integer_ |  |  |  |
| `successThreshold` _integer_ |  |  |  |
| `failureThreshold` _integer_ |  |  |  |
| `command` _string array_ |  |  |  |


#### Condition







_Appears in:_
- [Transition](#transition)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cron` _[Cron](#cron)_ | The cron job that triggers the transition if no other transition happens first. |  |  |
| `minDocCount` _integer_ | The minimum document count of the index required to transition. |  |  |
| `minIndexAge` _string_ | The minimum age of the index required to transition. |  |  |
| `minRolloverAge` _string_ | The minimum age required after a rollover has occurred to transition to the next state. |  |  |
| `minSize` _string_ | The minimum size of the total primary shard storage (not counting replicas) required to transition. |  |  |


#### ConfMgmt



ConfMgmt defines which additional services will be deployed



_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `autoScaler` _boolean_ |  |  |  |
| `VerUpdate` _boolean_ |  |  |  |
| `smartScaler` _boolean_ |  | true | Required: \{\} <br /> |


#### Cron







_Appears in:_
- [Condition](#condition)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cron` _[CronDetails](#crondetails)_ | A wrapper for the cron job that triggers the transition if no other transition happens first. This wrapper is here to adhere to the OpenSearch API. |  |  |


#### CronDetails

_Underlying type:_ _[struct{Expression string "json:\"expression\""; Timezone string "json:\"timezone\""}](#struct{expression-string-"json:\"expression\"";-timezone-string-"json:\"timezone\""})_





_Appears in:_
- [Cron](#cron)



#### CronExpression







_Appears in:_
- [CronSchedule](#cronschedule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `expression` _string_ |  |  |  |
| `timezone` _string_ |  |  |  |


#### CronSchedule







_Appears in:_
- [SnapshotCreation](#snapshotcreation)
- [SnapshotDeletion](#snapshotdeletion)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cron` _[CronExpression](#cronexpression)_ |  |  |  |


#### DashboardsConfig







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `replicas` _integer_ |  | 1 |  |
| `tls` _[DashboardsTlsConfig](#dashboardstlsconfig)_ |  |  |  |
| `version` _string_ |  |  |  |
| `basePath` _string_ | Base Path for Opensearch Clusters running behind a reverse proxy |  |  |
| `additionalConfig` _object (keys:string, values:string)_ | Additional properties for opensearch_dashboards.yaml |  |  |
| `opensearchCredentialsSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Secret that contains fields username and password for dashboards to use to login to opensearch, must only be supplied if a custom securityconfig is provided |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ |  |  |  |
| `additionalVolumes` _[AdditionalVolume](#additionalvolume) array_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `service` _[DashboardsServiceSpec](#dashboardsservicespec)_ |  |  |  |
| `pluginsList` _string array_ |  |  |  |
| `hostAliases` _[HostAlias](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostalias-v1-core) array_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ |  |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | Set security context for the dashboards pods |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | Set security context for the dashboards pods' container |  |  |
| `priorityClassName` _string_ |  |  |  |


#### DashboardsServiceSpec







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  | ClusterIP | Enum: [ClusterIP NodePort LoadBalancer] <br /> |
| `loadBalancerSourceRanges` _string array_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |


#### DashboardsTlsConfig







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | Enable HTTPS for Dashboards |  |  |
| `generate` _boolean_ | Generate certificate, if false secret must be provided |  |  |
| `TlsCertificateConfig` _[TlsCertificateConfig](#tlscertificateconfig)_ | TLS certificate configuration |  |  |


#### Delete







_Appears in:_
- [Action](#action)



#### Destination







_Appears in:_
- [ErrorNotification](#errornotification)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `slack` _[DestinationURL](#destinationurl)_ |  |  |  |
| `amazon` _[DestinationURL](#destinationurl)_ |  |  |  |
| `chime` _[DestinationURL](#destinationurl)_ |  |  |  |
| `customWebhook` _[DestinationURL](#destinationurl)_ |  |  |  |


#### DestinationURL







_Appears in:_
- [Destination](#destination)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `url` _string_ |  |  |  |


#### ErrorNotification







_Appears in:_
- [OpenSearchISMPolicySpec](#opensearchismpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `destination` _[Destination](#destination)_ | The destination URL. |  |  |
| `channel` _string_ |  |  |  |
| `messageTemplate` _[MessageTemplate](#messagetemplate)_ | The text of the message |  |  |


#### ForceMerge







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxNumSegments` _integer_ | The number of segments to reduce the shard to. |  |  |


#### GeneralConfig







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `httpPort` _integer_ |  | 9200 |  |
| `vendor` _string_ |  |  | Enum: [Opensearch Op OP os opensearch] <br /> |
| `version` _string_ |  |  |  |
| `serviceAccount` _string_ |  |  |  |
| `serviceName` _string_ |  |  |  |
| `setVMMaxMapCount` _boolean_ |  | true |  |
| `defaultRepo` _string_ |  |  |  |
| `hostNetwork` _boolean_ | HostNetwork enables host networking for all pods in the cluster. |  |  |
| `additionalConfig` _object (keys:string, values:string)_ | Extra items to add to the opensearch.yml |  |  |
| `annotations` _object (keys:string, values:string)_ | Adds support for annotations in services |  |  |
| `drainDataNodes` _boolean_ | Drain data nodes controls whether to drain data notes on rolling restart operations |  |  |
| `pluginsList` _string array_ |  |  |  |
| `command` _string_ |  |  |  |
| `additionalVolumes` _[AdditionalVolume](#additionalvolume) array_ | Additional volumes to mount to all pods in the cluster |  |  |
| `monitoring` _[MonitoringConfig](#monitoringconfig)_ |  |  |  |
| `keystore` _[KeystoreValue](#keystorevalue) array_ | Populate opensearch keystore before startup |  |  |
| `snapshotRepositories` _[SnapshotRepoConfig](#snapshotrepoconfig) array_ |  |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | Set security context for the cluster pods |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | Set security context for the cluster pods' container |  |  |
| `hostAliases` _[HostAlias](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostalias-v1-core) array_ |  |  |  |
| `operatorClusterURL` _string_ | Operator cluster URL. If set, the operator will use this URL to communicate with OpenSearch<br />instead of the default internal Kubernetes service DNS name. |  |  |


#### ISMTemplate







_Appears in:_
- [OpenSearchISMPolicySpec](#opensearchismpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `indexPatterns` _string array_ | Index patterns on which this policy has to be applied |  |  |
| `priority` _integer_ | Priority of the template, defaults to 0 |  |  |


#### ImageSpec







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)
- [GeneralConfig](#generalconfig)
- [InitHelperConfig](#inithelperconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ |  |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |


#### IndexPermissionSpec







_Appears in:_
- [OpensearchRoleSpec](#opensearchrolespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `indexPatterns` _string array_ |  |  |  |
| `dls` _string_ |  |  |  |
| `fls` _string array_ |  |  |  |
| `allowedActions` _string array_ |  |  |  |
| `maskedFields` _string array_ |  |  |  |


#### IndexPriority







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `priority` _integer_ | The priority for the index as soon as it enters a state. |  |  |


#### InitHelperConfig







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `version` _string_ |  |  |  |


#### KeystoreValue







_Appears in:_
- [BootstrapConfig](#bootstrapconfig)
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Secret containing key value pairs |  |  |
| `keyMappings` _object (keys:string, values:string)_ | Key mappings from secret to keystore keys |  |  |


#### MessageTemplate







_Appears in:_
- [ErrorNotification](#errornotification)
- [Notification](#notification)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `source` _string_ |  |  |  |


#### MonitoringConfig







_Appears in:_
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ |  |  |  |
| `monitoringUserSecret` _string_ |  |  |  |
| `scrapeInterval` _string_ |  |  |  |
| `pluginUrl` _string_ |  |  |  |
| `tlsConfig` _[MonitoringConfigTLS](#monitoringconfigtls)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |


#### MonitoringConfigTLS







_Appears in:_
- [MonitoringConfig](#monitoringconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serverName` _string_ |  |  |  |
| `insecureSkipVerify` _boolean_ |  |  |  |


#### NodePool







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `component` _string_ |  |  |  |
| `replicas` _integer_ |  |  |  |
| `diskSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quantity-resource-api)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `jvm` _string_ |  |  |  |
| `roles` _string array_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ |  |  |  |
| `persistence` _[PersistenceConfig](#persistenceconfig)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |
| `pdb` _[PdbConfig](#pdbconfig)_ |  |  |  |
| `probes` _[ProbesConfig](#probesconfig)_ |  |  |  |
| `additionalConfig` _object (keys:string, values:string)_ | Extra items to add to the opensearch.yml for this nodepool (merged with general.additionalConfig) |  |  |
| `sidecarContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#container-v1-core) array_ |  |  | Schemaless: \{\} <br /> |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#container-v1-core) array_ |  |  | Schemaless: \{\} <br /> |


#### Notification







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `destination` _string_ |  |  |  |
| `messageTemplate` _[MessageTemplate](#messagetemplate)_ |  |  |  |


#### NotificationChannel







_Appears in:_
- [SnapshotNotification](#snapshotnotification)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `id` _string_ |  |  |  |


#### NotificationConditions







_Appears in:_
- [SnapshotNotification](#snapshotnotification)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `creation` _boolean_ |  |  |  |
| `deletion` _boolean_ |  |  |  |
| `failure` _boolean_ |  |  |  |


#### Open







_Appears in:_
- [Action](#action)



#### OpenSearchCluster



Es is the Schema for the es API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpenSearchCluster` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ClusterSpec](#clusterspec)_ |  |  |  |




#### OpenSearchISMPolicy









| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpenSearchISMPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpenSearchISMPolicySpec](#opensearchismpolicyspec)_ |  |  |  |


#### OpenSearchISMPolicySpec



ISMPolicySpec is the specification for the ISM policy for OS.



_Appears in:_
- [OpenSearchISMPolicy](#opensearchismpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `defaultState` _string_ | The default starting state for each index that uses this policy. |  |  |
| `description` _string_ | A human-readable description of the policy. |  |  |
| `applyToExistingIndices` _boolean_ | If true, apply the policy to existing indices that match the index patterns in the ISM template. |  |  |
| `errorNotification` _[ErrorNotification](#errornotification)_ |  |  |  |
| `ismTemplate` _[ISMTemplate](#ismtemplate)_ | Specify an ISM template pattern that matches the index to apply the policy. |  |  |
| `policyId` _string_ |  |  |  |
| `states` _[State](#state) array_ | The states that you define in the policy. |  |  |


#### OpensearchActionGroup



OpensearchActionGroup is the Schema for the opensearchactiongroups API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchActionGroup` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchActionGroupSpec](#opensearchactiongroupspec)_ |  |  |  |


#### OpensearchActionGroupSpec



OpensearchActionGroupSpec defines the desired state of OpensearchActionGroup



_Appears in:_
- [OpensearchActionGroup](#opensearchactiongroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `allowedActions` _string array_ |  |  |  |
| `type` _string_ |  |  |  |
| `description` _string_ |  |  |  |






#### OpensearchComponentTemplate



OpensearchComponentTemplate is the schema for the OpenSearch component templates API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchComponentTemplate` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchComponentTemplateSpec](#opensearchcomponenttemplatespec)_ |  |  |  |


#### OpensearchComponentTemplateSpec







_Appears in:_
- [OpensearchComponentTemplate](#opensearchcomponenttemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `name` _string_ | The name of the component template. Defaults to metadata.name |  |  |
| `template` _[OpensearchIndexSpec](#opensearchindexspec)_ | The template that should be applied |  |  |
| `version` _integer_ | Version number used to manage the component template externally |  |  |
| `allowAutoCreate` _boolean_ | If true, then indices can be automatically created using this template |  |  |
| `_meta` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#json-v1-apiextensions-k8s-io)_ | Optional user metadata about the component template |  |  |




#### OpensearchDatastreamSpec







_Appears in:_
- [OpensearchIndexTemplateSpec](#opensearchindextemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `timestamp_field` _[OpensearchDatastreamTimestampFieldSpec](#opensearchdatastreamtimestampfieldspec)_ | TimestampField for dataStream |  |  |


#### OpensearchDatastreamTimestampFieldSpec







_Appears in:_
- [OpensearchDatastreamSpec](#opensearchdatastreamspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the field that are used for the DataStream |  |  |




#### OpensearchIndexAliasSpec



Describes the specs of an index alias



_Appears in:_
- [OpensearchIndexSpec](#opensearchindexspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `index` _string_ | The name of the index that the alias points to. |  |  |
| `alias` _string_ | The name of the alias. |  |  |
| `filter` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#json-v1-apiextensions-k8s-io)_ | Query used to limit documents the alias can access. |  |  |
| `routing` _string_ | Value used to route indexing and search operations to a specific shard. |  |  |
| `isWriteIndex` _boolean_ | If true, the index is the write index for the alias |  |  |


#### OpensearchIndexSpec



Describes the specs of an index



_Appears in:_
- [OpensearchComponentTemplateSpec](#opensearchcomponenttemplatespec)
- [OpensearchIndexTemplateSpec](#opensearchindextemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `settings` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#json-v1-apiextensions-k8s-io)_ | Configuration options for the index |  |  |
| `mappings` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#json-v1-apiextensions-k8s-io)_ | Mapping for fields in the index |  |  |
| `aliases` _object (keys:string, values:[OpensearchIndexAliasSpec](#opensearchindexaliasspec))_ | Aliases to add |  |  |


#### OpensearchIndexTemplate



OpensearchIndexTemplate is the schema for the OpenSearch index templates API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchIndexTemplate` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchIndexTemplateSpec](#opensearchindextemplatespec)_ |  |  |  |


#### OpensearchIndexTemplateSpec







_Appears in:_
- [OpensearchIndexTemplate](#opensearchindextemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `name` _string_ | The name of the index template. Defaults to metadata.name |  |  |
| `indexPatterns` _string array_ | Array of wildcard expressions used to match the names of indices during creation |  |  |
| `dataStream` _[OpensearchDatastreamSpec](#opensearchdatastreamspec)_ | The dataStream config that should be applied |  |  |
| `template` _[OpensearchIndexSpec](#opensearchindexspec)_ | The template that should be applied |  |  |
| `composedOf` _string array_ | An ordered list of component template names. Component templates are merged in the order specified,<br />meaning that the last component template specified has the highest precedence |  |  |
| `priority` _integer_ | Priority to determine index template precedence when a new data stream or index is created.<br />The index template with the highest priority is chosen |  |  |
| `version` _integer_ | Version number used to manage the component template externally |  |  |
| `_meta` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#json-v1-apiextensions-k8s-io)_ | Optional user metadata about the index template |  |  |




#### OpensearchRole



OpensearchRole is the Schema for the opensearchroles API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchRole` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchRoleSpec](#opensearchrolespec)_ |  |  |  |


#### OpensearchRoleSpec



OpensearchRoleSpec defines the desired state of OpensearchRole



_Appears in:_
- [OpensearchRole](#opensearchrole)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `clusterPermissions` _string array_ |  |  |  |
| `indexPermissions` _[IndexPermissionSpec](#indexpermissionspec) array_ |  |  |  |
| `tenantPermissions` _[TenantPermissionsSpec](#tenantpermissionsspec) array_ |  |  |  |




#### OpensearchSnapshotPolicy



OpensearchSnapshotPolicy is the Schema for the opensearchsnapshotpolicies API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchSnapshotPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchSnapshotPolicySpec](#opensearchsnapshotpolicyspec)_ |  |  |  |


#### OpensearchSnapshotPolicySpec







_Appears in:_
- [OpensearchSnapshotPolicy](#opensearchsnapshotpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `policyName` _string_ |  |  |  |
| `description` _string_ |  |  |  |
| `enabled` _boolean_ |  |  |  |
| `snapshotConfig` _[SnapshotConfig](#snapshotconfig)_ |  |  |  |
| `creation` _[SnapshotCreation](#snapshotcreation)_ |  |  |  |
| `deletion` _[SnapshotDeletion](#snapshotdeletion)_ |  |  |  |
| `notification` _[SnapshotNotification](#snapshotnotification)_ |  |  |  |




#### OpensearchTenant



OpensearchTenant is the Schema for the opensearchtenants API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchTenant` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchTenantSpec](#opensearchtenantspec)_ |  |  |  |


#### OpensearchTenantSpec



OpensearchTenantSpec defines the desired state of OpensearchTenant



_Appears in:_
- [OpensearchTenant](#opensearchtenant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `description` _string_ |  |  |  |




#### OpensearchUser



OpensearchUser is the Schema for the opensearchusers API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchUser` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchUserSpec](#opensearchuserspec)_ |  |  |  |


#### OpensearchUserRoleBinding



OpensearchUserRoleBinding is the Schema for the opensearchuserrolebindings API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchUserRoleBinding` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchUserRoleBindingSpec](#opensearchuserrolebindingspec)_ |  |  |  |


#### OpensearchUserRoleBindingSpec



OpensearchUserRoleBindingSpec defines the desired state of OpensearchUserRoleBinding



_Appears in:_
- [OpensearchUserRoleBinding](#opensearchuserrolebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `roles` _string array_ |  |  |  |
| `users` _string array_ |  |  |  |
| `backendRoles` _string array_ |  |  |  |




#### OpensearchUserSpec



OpensearchUserSpec defines the desired state of OpensearchUser



_Appears in:_
- [OpensearchUser](#opensearchuser)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `passwordFrom` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ |  |  |  |
| `opendistroSecurityRoles` _string array_ |  |  |  |
| `backendRoles` _string array_ |  |  |  |
| `attributes` _object (keys:string, values:string)_ |  |  |  |




#### PVCSource







_Appears in:_
- [PersistenceSource](#persistencesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `storageClass` _string_ |  |  |  |
| `accessModes` _[PersistentVolumeAccessMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeaccessmode-v1-core) array_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |


#### PdbConfig







_Appears in:_
- [NodePool](#nodepool)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ |  |  |  |
| `minAvailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#intorstring-intstr-util)_ |  |  |  |
| `maxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#intorstring-intstr-util)_ |  |  |  |


#### PersistenceConfig



PersistenceConfig defines options for data persistence



_Appears in:_
- [NodePool](#nodepool)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `PersistenceSource` _[PersistenceSource](#persistencesource)_ |  |  |  |


#### PersistenceSource







_Appears in:_
- [PersistenceConfig](#persistenceconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pvc` _[PVCSource](#pvcsource)_ |  |  |  |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ |  |  |  |
| `hostPath` _[HostPathVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostpathvolumesource-v1-core)_ |  |  |  |


#### ProbeConfig







_Appears in:_
- [ProbesConfig](#probesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `initialDelaySeconds` _integer_ |  |  |  |
| `periodSeconds` _integer_ |  |  |  |
| `timeoutSeconds` _integer_ |  |  |  |
| `successThreshold` _integer_ |  |  |  |
| `failureThreshold` _integer_ |  |  |  |


#### ProbesConfig







_Appears in:_
- [NodePool](#nodepool)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `liveness` _[ProbeConfig](#probeconfig)_ |  |  |  |
| `readiness` _[CommandProbeConfig](#commandprobeconfig)_ |  |  |  |
| `startup` _[CommandProbeConfig](#commandprobeconfig)_ |  |  |  |


#### ReadOnly







_Appears in:_
- [Action](#action)



#### ReadWrite







_Appears in:_
- [Action](#action)



#### ReplicaCount







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `numberOfReplicas` _integer_ |  |  |  |


#### Retry







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backoff` _string_ | The backoff policy type to use when retrying. |  |  |
| `count` _integer_ | The number of retry counts. |  |  |
| `delay` _string_ | The time to wait between retries. |  |  |


#### Rollover







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minDocCount` _integer_ | The minimum number of documents required to roll over the index. |  |  |
| `minIndexAge` _string_ | The minimum age required to roll over the index. |  |  |
| `minPrimaryShardSize` _string_ | The minimum storage size of a single primary shard required to roll over the index. |  |  |
| `minSize` _string_ | The minimum size of the total primary shard storage (not counting replicas) required to roll over the index. |  |  |


#### Rollup







_Appears in:_
- [Action](#action)



#### Security



Security defines options for managing the opensearch-security plugin



_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tls` _[TlsConfig](#tlsconfig)_ |  |  |  |
| `config` _[SecurityConfig](#securityconfig)_ |  |  |  |


#### SecurityConfig







_Appears in:_
- [Security](#security)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `securityConfigSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Optional secret that contains the different yml files of the opensearch-security config (config.yml, internal_users.yml, ...).<br />When omitted the operator seeds the cluster with its bundled defaults. |  |  |
| `adminSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | TLS Secret that contains a client certificate (tls.key, tls.crt, ca.crt) with admin rights in the opensearch cluster. Must be set if http certificates are provided by user and not generated |  |  |
| `adminCredentialsSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Secret that contains fields username and password to be used by the operator to access the opensearch cluster for node draining. Must be set if custom securityconfig is provided. |  |  |
| `updateJob` _[SecurityUpdateJobConfig](#securityupdatejobconfig)_ |  |  |  |


#### SecurityUpdateJobConfig



Specific configs for the SecurityConfig update job



_Appears in:_
- [SecurityConfig](#securityconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |


#### Shrink







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `forceUnsafe` _boolean_ | If true, executes the shrink action even if there are no replicas. |  |  |
| `maxShardSize` _string_ | The maximum size in bytes of a shard for the target index. |  |  |
| `numNewShards` _integer_ | The maximum number of primary shards in the shrunken index. |  |  |
| `percentageOfSourceShards` _integer_ | Percentage of the number of original primary shards to shrink. |  |  |
| `targetIndexNameTemplate` _string_ | The name of the shrunken index. |  |  |


#### Snapshot







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ | The repository name that you register through the native snapshot API operations. |  |  |
| `snapshot` _string_ | The name of the snapshot. |  |  |


#### SnapshotConfig







_Appears in:_
- [OpensearchSnapshotPolicySpec](#opensearchsnapshotpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `dateFormat` _string_ |  |  |  |
| `dateFormatTimezone` _string_ |  |  |  |
| `indices` _string_ |  |  |  |
| `repository` _string_ |  |  |  |
| `ignoreUnavailable` _boolean_ |  |  |  |
| `includeGlobalState` _boolean_ |  |  |  |
| `partial` _boolean_ |  |  |  |
| `metadata` _object (keys:string, values:string)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |


#### SnapshotCreation







_Appears in:_
- [OpensearchSnapshotPolicySpec](#opensearchsnapshotpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `schedule` _[CronSchedule](#cronschedule)_ |  |  |  |
| `timeLimit` _string_ |  |  |  |


#### SnapshotDeleteCondition







_Appears in:_
- [SnapshotDeletion](#snapshotdeletion)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxCount` _integer_ |  |  |  |
| `maxAge` _string_ |  |  |  |
| `minCount` _integer_ |  |  |  |


#### SnapshotDeletion







_Appears in:_
- [OpensearchSnapshotPolicySpec](#opensearchsnapshotpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `schedule` _[CronSchedule](#cronschedule)_ |  |  |  |
| `timeLimit` _string_ |  |  |  |
| `deleteCondition` _[SnapshotDeleteCondition](#snapshotdeletecondition)_ |  |  |  |


#### SnapshotNotification







_Appears in:_
- [OpensearchSnapshotPolicySpec](#opensearchsnapshotpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `channel` _[NotificationChannel](#notificationchannel)_ |  |  |  |
| `conditions` _[NotificationConditions](#notificationconditions)_ |  |  |  |


#### SnapshotRepoConfig







_Appears in:_
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `type` _string_ |  |  |  |
| `settings` _object (keys:string, values:string)_ |  |  |  |


#### State







_Appears in:_
- [OpenSearchISMPolicySpec](#opensearchismpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `actions` _[Action](#action) array_ | The actions to execute after entering a state. |  |  |
| `name` _string_ | The name of the state. |  |  |
| `transitions` _[Transition](#transition) array_ | The next states and the conditions required to transition to those states. If no transitions exist, the policy assumes that it’s complete and can now stop managing the index |  |  |


#### TenantPermissionsSpec







_Appears in:_
- [OpensearchRoleSpec](#opensearchrolespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tenantPatterns` _string array_ |  |  |  |
| `allowedActions` _string array_ |  |  |  |


#### TlsCertificateConfig







_Appears in:_
- [DashboardsTlsConfig](#dashboardstlsconfig)
- [TlsConfigHttp](#tlsconfighttp)
- [TlsConfigTransport](#tlsconfigtransport)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Optional, name of a TLS secret that contains ca.crt, tls.key and tls.crt data. If ca.crt is in a different secret provide it via the caSecret field |  |  |
| `caSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Optional, secret that contains the ca certificate as ca.crt. If this and generate=true is set the existing CA cert from that secret is used to generate the node certs. In this case must contain ca.crt and ca.key fields |  |  |
| `duration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Duration controls the validity period of generated certificates (e.g. "8760h", "720h"). | 8760h |  |
| `enableHotReload` _boolean_ | Enable hot reloading of TLS certificates. When enabled, certificates are mounted as directories instead of using subPath, allowing Kubernetes to update certificate files when secrets are updated. |  |  |


#### TlsConfig



Configure tls usage for transport and http interface



_Appears in:_
- [Security](#security)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `transport` _[TlsConfigTransport](#tlsconfigtransport)_ |  |  |  |
| `http` _[TlsConfigHttp](#tlsconfighttp)_ |  |  |  |


#### TlsConfigHttp







_Appears in:_
- [TlsConfig](#tlsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled controls if TLS should be enabled for the HTTP layer.<br />If false: TLS is explicitly disabled for HTTP.<br />If not set (default): TLS is enabled if HTTP configuration is provided, or if security.tls is set.<br />If true: TLS is explicitly enabled. HTTP configuration must be provided. |  |  |
| `generate` _boolean_ | If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied |  |  |
| `customFQDN` _string_ | Custom FQDN to use for the HTTP certificate. If not set, the operator will use the default cluster DNS names. |  |  |
| `rotateDaysBeforeExpiry` _integer_ | Automatically rotate certificates before they expire, set to -1 to disable | -1 |  |
| `TlsCertificateConfig` _[TlsCertificateConfig](#tlscertificateconfig)_ |  |  |  |
| `adminDn` _string array_ | DNs of certificates that should have admin access, mainly used for securityconfig updates via securityadmin.sh, only used when existing certificates are provided |  |  |


#### TlsConfigTransport







_Appears in:_
- [TlsConfig](#tlsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled controls if TLS should be enabled for the transport layer.<br />If false: TLS is explicitly disabled for transport.<br />If not set (default): TLS is enabled if transport configuration is provided, or if security.tls is set.<br />If true: TLS is explicitly enabled. Transport configuration must be provided. |  |  |
| `generate` _boolean_ | If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied |  |  |
| `perNode` _boolean_ | Configure transport node certificate |  |  |
| `rotateDaysBeforeExpiry` _integer_ | Automatically rotate certificates before they expire, set to -1 to disable | -1 |  |
| `TlsCertificateConfig` _[TlsCertificateConfig](#tlscertificateconfig)_ |  |  |  |
| `nodesDn` _string array_ | Allowed Certificate DNs for nodes, only used when existing certificates are provided |  |  |
| `adminDn` _string array_ | Deprecated: DNs of certificates that should have admin access. This field is deprecated and will be removed in a future version.<br />For OpenSearch 2.0.0+, use security.tls.http.adminDn instead. |  |  |




#### Transition







_Appears in:_
- [State](#state)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](#condition)_ | conditions for the transition. |  |  |
| `stateName` _string_ | The name of the state to transition to if the conditions are met. |  |  |



## opensearch.org/v1

Package v1 contains API Schema definitions for the opensearch.org v1 API group

### Resource Types
- [OpenSearchCluster](#opensearchcluster)
- [OpenSearchISMPolicy](#opensearchismpolicy)
- [OpensearchActionGroup](#opensearchactiongroup)
- [OpensearchComponentTemplate](#opensearchcomponenttemplate)
- [OpensearchIndexTemplate](#opensearchindextemplate)
- [OpensearchRole](#opensearchrole)
- [OpensearchSnapshotPolicy](#opensearchsnapshotpolicy)
- [OpensearchTenant](#opensearchtenant)
- [OpensearchUser](#opensearchuser)
- [OpensearchUserRoleBinding](#opensearchuserrolebinding)



#### Action



Actions are the steps that the policy sequentially executes on entering a specific state.



_Appears in:_
- [State](#state)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `alias` _[Alias](#alias)_ |  |  |  |
| `allocation` _[Allocation](#allocation)_ | Allocate the index to a node with a specific attribute set |  |  |
| `close` _[Close](#close)_ | Closes the managed index. |  |  |
| `delete` _[Delete](#delete)_ | Deletes a managed index. |  |  |
| `forceMerge` _[ForceMerge](#forcemerge)_ | Reduces the number of Lucene segments by merging the segments of individual shards. |  |  |
| `indexPriority` _[IndexPriority](#indexpriority)_ | Set the priority for the index in a specific state. |  |  |
| `notification` _[Notification](#notification)_ | Name          string        `json:"name,omitempty"` |  |  |
| `open` _[Open](#open)_ | Opens a managed index. |  |  |
| `readOnly` _[ReadOnly](#readonly)_ | Sets a managed index to be read only. |  |  |
| `readWrite` _[ReadWrite](#readwrite)_ | Sets a managed index to be writeable. |  |  |
| `replicaCount` _[ReplicaCount](#replicacount)_ | Sets the number of replicas to assign to an index. |  |  |
| `retry` _[Retry](#retry)_ | The retry configuration for the action. |  |  |
| `rollover` _[Rollover](#rollover)_ | Rolls an alias over to a new index when the managed index meets one of the rollover conditions. |  |  |
| `rollup` _[Rollup](#rollup)_ | Periodically reduce data granularity by rolling up old data into summarized indexes. |  |  |
| `shrink` _[Shrink](#shrink)_ | Allows you to reduce the number of primary shards in your indexes |  |  |
| `snapshot` _[Snapshot](#snapshot)_ | Back up your cluster's indexes and state |  |  |
| `timeout` _string_ | The timeout period for the action. Accepts time units for minutes, hours, and days. |  |  |


#### AdditionalVolume







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name to use for the volume. Required. |  |  |
| `path` _string_ | Path in the container to mount the volume at. Required. |  |  |
| `subPath` _string_ | SubPath of the referenced volume to mount. |  |  |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretvolumesource-v1-core)_ | Secret to use populate the volume |  |  |
| `configMap` _[ConfigMapVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#configmapvolumesource-v1-core)_ | ConfigMap to use to populate the volume |  |  |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ | EmptyDir to use to populate the volume |  |  |
| `csi` _[CSIVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#csivolumesource-v1-core)_ | CSI object to use to populate the volume |  |  |
| `persistentVolumeClaim` _[PersistentVolumeClaimVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeclaimvolumesource-v1-core)_ | PersistentVolumeClaim object to use to populate the volume |  |  |
| `projected` _[ProjectedVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#projectedvolumesource-v1-core)_ | Projected object to use to populate the volume |  |  |
| `nfs` _[NFSVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#nfsvolumesource-v1-core)_ | NFS object to use to populate the volume |  |  |
| `hostPath` _[HostPathVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostpathvolumesource-v1-core)_ | HostPath object to use to populate the volume |  |  |
| `restartPods` _boolean_ | Whether to restart the pods on content change |  |  |


#### Alias







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `actions` _[AliasAction](#aliasaction) array_ | Allocate the index to a node with a specified attribute. |  |  |


#### AliasAction







_Appears in:_
- [Alias](#alias)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `add` _[AliasDetails](#aliasdetails)_ |  |  |  |
| `remove` _[AliasDetails](#aliasdetails)_ |  |  |  |




#### Allocation







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `exclude` _string_ | Allocate the index to a node with a specified attribute. |  |  |
| `include` _string_ | Allocate the index to a node with any of the specified attributes. |  |  |
| `require` _string_ | Don't allocate the index to a node with any of the specified attributes. |  |  |
| `waitFor` _string_ | Wait for the policy to execute before allocating the index to a node with a specified attribute. |  |  |


#### BootstrapConfig







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `jvm` _string_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `pluginsList` _string array_ |  |  |  |
| `keystore` _[KeystoreValue](#keystorevalue) array_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ |  |  |  |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#container-v1-core) array_ |  |  | Schemaless: \{\} <br /> |
| `hostAliases` _[HostAlias](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostalias-v1-core) array_ |  |  |  |
| `diskSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quantity-resource-api)_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |
| `storageClass` _string_ |  |  |  |


#### Close







_Appears in:_
- [Action](#action)



#### ClusterSpec



ClusterSpec defines the desired state of OpenSearchCluster



_Appears in:_
- [OpenSearchCluster](#opensearchcluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `general` _[GeneralConfig](#generalconfig)_ | INSERT ADDITIONAL SPEC FIELDS - desired state of cluster<br />Important: Run "make" to regenerate code after modifying this file |  |  |
| `confMgmt` _[ConfMgmt](#confmgmt)_ |  |  |  |
| `bootstrap` _[BootstrapConfig](#bootstrapconfig)_ |  |  |  |
| `dashboards` _[DashboardsConfig](#dashboardsconfig)_ |  |  |  |
| `security` _[Security](#security)_ |  |  |  |
| `nodePools` _[NodePool](#nodepool) array_ |  |  |  |
| `initHelper` _[InitHelperConfig](#inithelperconfig)_ |  |  |  |


#### CommandProbeConfig







_Appears in:_
- [ProbesConfig](#probesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `initialDelaySeconds` _integer_ |  |  |  |
| `periodSeconds` _integer_ |  |  |  |
| `timeoutSeconds` _integer_ |  |  |  |
| `successThreshold` _integer_ |  |  |  |
| `failureThreshold` _integer_ |  |  |  |
| `command` _string array_ |  |  |  |


#### Condition







_Appears in:_
- [Transition](#transition)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cron` _[Cron](#cron)_ | The cron job that triggers the transition if no other transition happens first. |  |  |
| `minDocCount` _integer_ | The minimum document count of the index required to transition. |  |  |
| `minIndexAge` _string_ | The minimum age of the index required to transition. |  |  |
| `minRolloverAge` _string_ | The minimum age required after a rollover has occurred to transition to the next state. |  |  |
| `minSize` _string_ | The minimum size of the total primary shard storage (not counting replicas) required to transition. |  |  |


#### ConfMgmt



ConfMgmt defines which additional services will be deployed



_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `autoScaler` _boolean_ |  |  |  |
| `VerUpdate` _boolean_ |  |  |  |
| `smartScaler` _boolean_ |  | true | Required: \{\} <br /> |


#### Cron







_Appears in:_
- [Condition](#condition)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cron` _[CronDetails](#crondetails)_ | A wrapper for the cron job that triggers the transition if no other transition happens first. This wrapper is here to adhere to the OpenSearch API. |  |  |


#### CronDetails

_Underlying type:_ _[struct{Expression string "json:\"expression\""; Timezone string "json:\"timezone\""}](#struct{expression-string-"json:\"expression\"";-timezone-string-"json:\"timezone\""})_





_Appears in:_
- [Cron](#cron)



#### CronExpression







_Appears in:_
- [CronSchedule](#cronschedule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `expression` _string_ |  |  |  |
| `timezone` _string_ |  |  |  |


#### CronSchedule







_Appears in:_
- [SnapshotCreation](#snapshotcreation)
- [SnapshotDeletion](#snapshotdeletion)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cron` _[CronExpression](#cronexpression)_ |  |  |  |


#### DashboardsConfig







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `replicas` _integer_ |  | 1 |  |
| `tls` _[DashboardsTlsConfig](#dashboardstlsconfig)_ |  |  |  |
| `version` _string_ |  |  |  |
| `basePath` _string_ | Base Path for Opensearch Clusters running behind a reverse proxy |  |  |
| `additionalConfig` _object (keys:string, values:string)_ | Additional properties for opensearch_dashboards.yaml |  |  |
| `opensearchCredentialsSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Secret that contains fields username and password for dashboards to use to login to opensearch, must only be supplied if a custom securityconfig is provided |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ |  |  |  |
| `additionalVolumes` _[AdditionalVolume](#additionalvolume) array_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `service` _[DashboardsServiceSpec](#dashboardsservicespec)_ |  |  |  |
| `pluginsList` _string array_ |  |  |  |
| `hostAliases` _[HostAlias](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostalias-v1-core) array_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ |  |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | Set security context for the dashboards pods |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | Set security context for the dashboards pods' container |  |  |
| `priorityClassName` _string_ |  |  |  |


#### DashboardsServiceSpec







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#servicetype-v1-core)_ |  | ClusterIP | Enum: [ClusterIP NodePort LoadBalancer] <br /> |
| `loadBalancerSourceRanges` _string array_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |


#### DashboardsTlsConfig







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | Enable HTTPS for Dashboards |  |  |
| `generate` _boolean_ | Generate certificate, if false secret must be provided |  |  |
| `TlsCertificateConfig` _[TlsCertificateConfig](#tlscertificateconfig)_ | TLS certificate configuration |  |  |


#### Delete







_Appears in:_
- [Action](#action)



#### Destination







_Appears in:_
- [ErrorNotification](#errornotification)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `slack` _[DestinationURL](#destinationurl)_ |  |  |  |
| `amazon` _[DestinationURL](#destinationurl)_ |  |  |  |
| `chime` _[DestinationURL](#destinationurl)_ |  |  |  |
| `customWebhook` _[DestinationURL](#destinationurl)_ |  |  |  |


#### DestinationURL







_Appears in:_
- [Destination](#destination)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `url` _string_ |  |  |  |


#### ErrorNotification







_Appears in:_
- [OpenSearchISMPolicySpec](#opensearchismpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `destination` _[Destination](#destination)_ | The destination URL. |  |  |
| `channel` _string_ |  |  |  |
| `messageTemplate` _[MessageTemplate](#messagetemplate)_ | The text of the message |  |  |


#### ForceMerge







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxNumSegments` _integer_ | The number of segments to reduce the shard to. |  |  |


#### GeneralConfig







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `httpPort` _integer_ |  | 9200 |  |
| `vendor` _string_ |  |  | Enum: [Opensearch Op OP os opensearch] <br /> |
| `version` _string_ |  |  |  |
| `serviceAccount` _string_ |  |  |  |
| `serviceName` _string_ |  |  |  |
| `setVMMaxMapCount` _boolean_ |  | true |  |
| `defaultRepo` _string_ |  |  |  |
| `additionalConfig` _object (keys:string, values:string)_ | Extra items to add to the opensearch.yml |  |  |
| `annotations` _object (keys:string, values:string)_ | Adds support for annotations in services |  |  |
| `drainDataNodes` _boolean_ | Drain data nodes controls whether to drain data notes on rolling restart operations |  |  |
| `pluginsList` _string array_ |  |  |  |
| `command` _string_ |  |  |  |
| `additionalVolumes` _[AdditionalVolume](#additionalvolume) array_ | Additional volumes to mount to all pods in the cluster |  |  |
| `monitoring` _[MonitoringConfig](#monitoringconfig)_ |  |  |  |
| `keystore` _[KeystoreValue](#keystorevalue) array_ | Populate opensearch keystore before startup |  |  |
| `snapshotRepositories` _[SnapshotRepoConfig](#snapshotrepoconfig) array_ |  |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#podsecuritycontext-v1-core)_ | Set security context for the cluster pods |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#securitycontext-v1-core)_ | Set security context for the cluster pods' container |  |  |
| `hostAliases` _[HostAlias](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostalias-v1-core) array_ |  |  |  |
| `operatorClusterURL` _string_ | Operator cluster URL. If set, the operator will use this URL to communicate with OpenSearch<br />instead of the default internal Kubernetes service DNS name. |  |  |
| `grpc` _[GrpcConfig](#grpcconfig)_ | gRPC API configuration for OpenSearch |  |  |
| `hostNetwork` _boolean_ | HostNetwork enables host networking for all pods in the cluster. |  |  |
| `persistentVolumeClaimRetentionPolicy` _[StatefulSetPersistentVolumeClaimRetentionPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#statefulsetpersistentvolumeclaimretentionpolicy-v1-apps)_ | Set the retention policy for the cluster PVCs |  |  |


#### GrpcConfig



GrpcConfig defines gRPC API configuration for OpenSearch



_Appears in:_
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | Enable gRPC transport. When enabled, gRPC APIs will be available. |  |  |
| `port` _string_ | Port range for gRPC transport (e.g., "9400-9500"). If not specified, defaults to "9400-9500". |  |  |
| `host` _string array_ | Host addresses the gRPC server will bind to. If not specified, defaults to ["0.0.0.0"]. |  |  |
| `bindHost` _string array_ | Bind host addresses for the gRPC server. Can be distinct from publish hosts. |  |  |
| `publishHost` _string array_ | Publish hostnames or IPs for client connections. |  |  |
| `publishPort` _integer_ | Publish port number that this node uses to publish itself to peers for gRPC transport. |  |  |
| `nettyWorkerCount` _integer_ | Number of Netty worker threads for the gRPC server. Controls concurrency and parallelism. |  |  |
| `nettyExecutorCount` _integer_ | Number of threads in the fork-join pool for processing gRPC service calls. |  |  |
| `maxConcurrentConnectionCalls` _integer_ | Maximum number of simultaneous in-flight requests allowed per client connection. |  |  |
| `maxConnectionAge` _string_ | Maximum age a connection can reach before being gracefully closed (e.g., "500ms", "2m"). |  |  |
| `maxConnectionIdle` _string_ | Maximum duration for which a connection can be idle before being closed (e.g., "2m"). |  |  |
| `keepaliveTimeout` _string_ | Amount of time to wait for keepalive ping acknowledgment before closing the connection (e.g., "1s"). |  |  |
| `maxMsgSize` _string_ | Maximum inbound message size for gRPC requests (e.g., "10mb", "10485760"). |  |  |


#### ISMTemplate







_Appears in:_
- [OpenSearchISMPolicySpec](#opensearchismpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `indexPatterns` _string array_ | Index patterns on which this policy has to be applied |  |  |
| `priority` _integer_ | Priority of the template, defaults to 0 |  |  |


#### ImageSpec







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)
- [GeneralConfig](#generalconfig)
- [InitHelperConfig](#inithelperconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ |  |  |  |
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#pullpolicy-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core) array_ |  |  |  |


#### IndexPermissionSpec







_Appears in:_
- [OpensearchRoleSpec](#opensearchrolespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `indexPatterns` _string array_ |  |  |  |
| `dls` _string_ |  |  |  |
| `fls` _string array_ |  |  |  |
| `allowedActions` _string array_ |  |  |  |
| `maskedFields` _string array_ |  |  |  |


#### IndexPriority







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `priority` _integer_ | The priority for the index as soon as it enters a state. |  |  |


#### InitHelperConfig







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `version` _string_ |  |  |  |


#### KeystoreValue







_Appears in:_
- [BootstrapConfig](#bootstrapconfig)
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Secret containing key value pairs |  |  |
| `keyMappings` _object (keys:string, values:string)_ | Key mappings from secret to keystore keys |  |  |


#### MessageTemplate







_Appears in:_
- [ErrorNotification](#errornotification)
- [Notification](#notification)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `source` _string_ |  |  |  |


#### MonitoringConfig







_Appears in:_
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ |  |  |  |
| `monitoringUserSecret` _string_ |  |  |  |
| `scrapeInterval` _string_ |  |  |  |
| `pluginUrl` _string_ |  |  |  |
| `tlsConfig` _[MonitoringConfigTLS](#monitoringconfigtls)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |


#### MonitoringConfigTLS







_Appears in:_
- [MonitoringConfig](#monitoringconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serverName` _string_ |  |  |  |
| `insecureSkipVerify` _boolean_ |  |  |  |


#### NodePool







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `component` _string_ |  |  |  |
| `replicas` _integer_ |  |  |  |
| `diskSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#quantity-resource-api)_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `jvm` _string_ |  |  |  |
| `roles` _string array_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#topologyspreadconstraint-v1-core) array_ |  |  |  |
| `persistence` _[PersistenceConfig](#persistenceconfig)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#envvar-v1-core) array_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |
| `pdb` _[PdbConfig](#pdbconfig)_ |  |  |  |
| `probes` _[ProbesConfig](#probesconfig)_ |  |  |  |
| `additionalConfig` _object (keys:string, values:string)_ | Extra items to add to the opensearch.yml for this nodepool (merged with general.additionalConfig) |  |  |
| `sidecarContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#container-v1-core) array_ |  |  | Schemaless: \{\} <br /> |
| `initContainers` _[Container](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#container-v1-core) array_ |  |  | Schemaless: \{\} <br /> |


#### Notification







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `destination` _string_ |  |  |  |
| `messageTemplate` _[MessageTemplate](#messagetemplate)_ |  |  |  |


#### NotificationChannel







_Appears in:_
- [SnapshotNotification](#snapshotnotification)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `id` _string_ |  |  |  |


#### NotificationConditions







_Appears in:_
- [SnapshotNotification](#snapshotnotification)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `creation` _boolean_ |  |  |  |
| `deletion` _boolean_ |  |  |  |
| `failure` _boolean_ |  |  |  |


#### Open







_Appears in:_
- [Action](#action)



#### OpenSearchCluster



Es is the Schema for the es API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.org/v1` | | |
| `kind` _string_ | `OpenSearchCluster` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ClusterSpec](#clusterspec)_ |  |  |  |




#### OpenSearchISMPolicy









| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.org/v1` | | |
| `kind` _string_ | `OpenSearchISMPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpenSearchISMPolicySpec](#opensearchismpolicyspec)_ |  |  |  |


#### OpenSearchISMPolicySpec



ISMPolicySpec is the specification for the ISM policy for OS.



_Appears in:_
- [OpenSearchISMPolicy](#opensearchismpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `defaultState` _string_ | The default starting state for each index that uses this policy. |  |  |
| `description` _string_ | A human-readable description of the policy. |  |  |
| `applyToExistingIndices` _boolean_ | If true, apply the policy to existing indices that match the index patterns in the ISM template. |  |  |
| `errorNotification` _[ErrorNotification](#errornotification)_ |  |  |  |
| `ismTemplate` _[ISMTemplate](#ismtemplate)_ | Specify an ISM template pattern that matches the index to apply the policy. |  |  |
| `policyId` _string_ |  |  |  |
| `states` _[State](#state) array_ | The states that you define in the policy. |  |  |


#### OpensearchActionGroup



OpensearchActionGroup is the Schema for the opensearchactiongroups API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.org/v1` | | |
| `kind` _string_ | `OpensearchActionGroup` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchActionGroupSpec](#opensearchactiongroupspec)_ |  |  |  |


#### OpensearchActionGroupSpec



OpensearchActionGroupSpec defines the desired state of OpensearchActionGroup



_Appears in:_
- [OpensearchActionGroup](#opensearchactiongroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `allowedActions` _string array_ |  |  |  |
| `type` _string_ |  |  |  |
| `description` _string_ |  |  |  |






#### OpensearchComponentTemplate



OpensearchComponentTemplate is the schema for the OpenSearch component templates API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.org/v1` | | |
| `kind` _string_ | `OpensearchComponentTemplate` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchComponentTemplateSpec](#opensearchcomponenttemplatespec)_ |  |  |  |


#### OpensearchComponentTemplateSpec







_Appears in:_
- [OpensearchComponentTemplate](#opensearchcomponenttemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `name` _string_ | The name of the component template. Defaults to metadata.name |  |  |
| `template` _[OpensearchIndexSpec](#opensearchindexspec)_ | The template that should be applied |  |  |
| `version` _integer_ | Version number used to manage the component template externally |  |  |
| `allowAutoCreate` _boolean_ | If true, then indices can be automatically created using this template |  |  |
| `_meta` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#json-v1-apiextensions-k8s-io)_ | Optional user metadata about the component template |  |  |




#### OpensearchDatastreamSpec







_Appears in:_
- [OpensearchIndexTemplateSpec](#opensearchindextemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `timestamp_field` _[OpensearchDatastreamTimestampFieldSpec](#opensearchdatastreamtimestampfieldspec)_ | TimestampField for dataStream |  |  |


#### OpensearchDatastreamTimestampFieldSpec







_Appears in:_
- [OpensearchDatastreamSpec](#opensearchdatastreamspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the field that are used for the DataStream |  |  |




#### OpensearchIndexAliasSpec



Describes the specs of an index alias



_Appears in:_
- [OpensearchIndexSpec](#opensearchindexspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `index` _string_ | The name of the index that the alias points to. |  |  |
| `alias` _string_ | The name of the alias. |  |  |
| `filter` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#json-v1-apiextensions-k8s-io)_ | Query used to limit documents the alias can access. |  |  |
| `routing` _string_ | Value used to route indexing and search operations to a specific shard. |  |  |
| `isWriteIndex` _boolean_ | If true, the index is the write index for the alias |  |  |


#### OpensearchIndexSpec



Describes the specs of an index



_Appears in:_
- [OpensearchComponentTemplateSpec](#opensearchcomponenttemplatespec)
- [OpensearchIndexTemplateSpec](#opensearchindextemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `settings` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#json-v1-apiextensions-k8s-io)_ | Configuration options for the index |  |  |
| `mappings` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#json-v1-apiextensions-k8s-io)_ | Mapping for fields in the index |  |  |
| `aliases` _object (keys:string, values:[OpensearchIndexAliasSpec](#opensearchindexaliasspec))_ | Aliases to add |  |  |


#### OpensearchIndexTemplate



OpensearchIndexTemplate is the schema for the OpenSearch index templates API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.org/v1` | | |
| `kind` _string_ | `OpensearchIndexTemplate` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchIndexTemplateSpec](#opensearchindextemplatespec)_ |  |  |  |


#### OpensearchIndexTemplateSpec







_Appears in:_
- [OpensearchIndexTemplate](#opensearchindextemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `name` _string_ | The name of the index template. Defaults to metadata.name |  |  |
| `indexPatterns` _string array_ | Array of wildcard expressions used to match the names of indices during creation |  |  |
| `dataStream` _[OpensearchDatastreamSpec](#opensearchdatastreamspec)_ | The dataStream config that should be applied |  |  |
| `template` _[OpensearchIndexSpec](#opensearchindexspec)_ | The template that should be applied |  |  |
| `composedOf` _string array_ | An ordered list of component template names. Component templates are merged in the order specified,<br />meaning that the last component template specified has the highest precedence |  |  |
| `priority` _integer_ | Priority to determine index template precedence when a new data stream or index is created.<br />The index template with the highest priority is chosen |  |  |
| `version` _integer_ | Version number used to manage the component template externally |  |  |
| `_meta` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#json-v1-apiextensions-k8s-io)_ | Optional user metadata about the index template |  |  |




#### OpensearchRole



OpensearchRole is the Schema for the opensearchroles API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.org/v1` | | |
| `kind` _string_ | `OpensearchRole` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchRoleSpec](#opensearchrolespec)_ |  |  |  |


#### OpensearchRoleSpec



OpensearchRoleSpec defines the desired state of OpensearchRole



_Appears in:_
- [OpensearchRole](#opensearchrole)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `clusterPermissions` _string array_ |  |  |  |
| `indexPermissions` _[IndexPermissionSpec](#indexpermissionspec) array_ |  |  |  |
| `tenantPermissions` _[TenantPermissionsSpec](#tenantpermissionsspec) array_ |  |  |  |




#### OpensearchSnapshotPolicy



OpensearchSnapshotPolicy is the Schema for the opensearchsnapshotpolicies API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.org/v1` | | |
| `kind` _string_ | `OpensearchSnapshotPolicy` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchSnapshotPolicySpec](#opensearchsnapshotpolicyspec)_ |  |  |  |


#### OpensearchSnapshotPolicySpec







_Appears in:_
- [OpensearchSnapshotPolicy](#opensearchsnapshotpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `policyName` _string_ |  |  |  |
| `description` _string_ |  |  |  |
| `enabled` _boolean_ |  |  |  |
| `snapshotConfig` _[SnapshotConfig](#snapshotconfig)_ |  |  |  |
| `creation` _[SnapshotCreation](#snapshotcreation)_ |  |  |  |
| `deletion` _[SnapshotDeletion](#snapshotdeletion)_ |  |  |  |
| `notification` _[SnapshotNotification](#snapshotnotification)_ |  |  |  |




#### OpensearchTenant



OpensearchTenant is the Schema for the opensearchtenants API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.org/v1` | | |
| `kind` _string_ | `OpensearchTenant` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchTenantSpec](#opensearchtenantspec)_ |  |  |  |


#### OpensearchTenantSpec



OpensearchTenantSpec defines the desired state of OpensearchTenant



_Appears in:_
- [OpensearchTenant](#opensearchtenant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `description` _string_ |  |  |  |




#### OpensearchUser



OpensearchUser is the Schema for the opensearchusers API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.org/v1` | | |
| `kind` _string_ | `OpensearchUser` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchUserSpec](#opensearchuserspec)_ |  |  |  |


#### OpensearchUserRoleBinding



OpensearchUserRoleBinding is the Schema for the opensearchuserrolebindings API





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.org/v1` | | |
| `kind` _string_ | `OpensearchUserRoleBinding` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchUserRoleBindingSpec](#opensearchuserrolebindingspec)_ |  |  |  |


#### OpensearchUserRoleBindingSpec



OpensearchUserRoleBindingSpec defines the desired state of OpensearchUserRoleBinding



_Appears in:_
- [OpensearchUserRoleBinding](#opensearchuserrolebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `roles` _string array_ |  |  |  |
| `users` _string array_ |  |  |  |
| `backendRoles` _string array_ |  |  |  |




#### OpensearchUserSpec



OpensearchUserSpec defines the desired state of OpensearchUser



_Appears in:_
- [OpensearchUser](#opensearchuser)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ |  |  |  |
| `passwordFrom` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#secretkeyselector-v1-core)_ |  |  |  |
| `opendistroSecurityRoles` _string array_ |  |  |  |
| `backendRoles` _string array_ |  |  |  |
| `attributes` _object (keys:string, values:string)_ |  |  |  |




#### PVCSource







_Appears in:_
- [PersistenceSource](#persistencesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `storageClass` _string_ |  |  |  |
| `accessModes` _[PersistentVolumeAccessMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#persistentvolumeaccessmode-v1-core) array_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |


#### PdbConfig







_Appears in:_
- [NodePool](#nodepool)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ |  |  |  |
| `minAvailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#intorstring-intstr-util)_ |  |  |  |
| `maxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#intorstring-intstr-util)_ |  |  |  |


#### PersistenceConfig



PersistenceConfig defines options for data persistence



_Appears in:_
- [NodePool](#nodepool)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `PersistenceSource` _[PersistenceSource](#persistencesource)_ |  |  |  |


#### PersistenceSource







_Appears in:_
- [PersistenceConfig](#persistenceconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `pvc` _[PVCSource](#pvcsource)_ |  |  |  |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#emptydirvolumesource-v1-core)_ |  |  |  |
| `hostPath` _[HostPathVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#hostpathvolumesource-v1-core)_ |  |  |  |


#### ProbeConfig







_Appears in:_
- [ProbesConfig](#probesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `initialDelaySeconds` _integer_ |  |  |  |
| `periodSeconds` _integer_ |  |  |  |
| `timeoutSeconds` _integer_ |  |  |  |
| `successThreshold` _integer_ |  |  |  |
| `failureThreshold` _integer_ |  |  |  |


#### ProbesConfig







_Appears in:_
- [NodePool](#nodepool)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `liveness` _[ProbeConfig](#probeconfig)_ |  |  |  |
| `readiness` _[CommandProbeConfig](#commandprobeconfig)_ |  |  |  |
| `startup` _[CommandProbeConfig](#commandprobeconfig)_ |  |  |  |


#### ReadOnly







_Appears in:_
- [Action](#action)



#### ReadWrite







_Appears in:_
- [Action](#action)



#### ReplicaCount







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `numberOfReplicas` _integer_ |  |  |  |


#### Retry







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backoff` _string_ | The backoff policy type to use when retrying. |  |  |
| `count` _integer_ | The number of retry counts. |  |  |
| `delay` _string_ | The time to wait between retries. |  |  |


#### Rollover







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minDocCount` _integer_ | The minimum number of documents required to roll over the index. |  |  |
| `minIndexAge` _string_ | The minimum age required to roll over the index. |  |  |
| `minPrimaryShardSize` _string_ | The minimum storage size of a single primary shard required to roll over the index. |  |  |
| `minSize` _string_ | The minimum size of the total primary shard storage (not counting replicas) required to roll over the index. |  |  |


#### Rollup







_Appears in:_
- [Action](#action)



#### Security



Security defines options for managing the opensearch-security plugin



_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tls` _[TlsConfig](#tlsconfig)_ |  |  |  |
| `config` _[SecurityConfig](#securityconfig)_ |  |  |  |


#### SecurityConfig







_Appears in:_
- [Security](#security)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `securityConfigSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Optional secret that contains the different yml files of the opensearch-security config (config.yml, internal_users.yml, ...).<br />When omitted the operator seeds the cluster with its bundled defaults. |  |  |
| `adminSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | TLS Secret that contains a client certificate (tls.key, tls.crt, ca.crt) with admin rights in the opensearch cluster. Must be set if http certificates are provided by user and not generated |  |  |
| `adminCredentialsSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Secret that contains fields username and password to be used by the operator to access the opensearch cluster for node draining. Must be set if custom securityconfig is provided. |  |  |
| `updateJob` _[SecurityUpdateJobConfig](#securityupdatejobconfig)_ |  |  |  |


#### SecurityUpdateJobConfig



Specific configs for the SecurityConfig update job



_Appears in:_
- [SecurityConfig](#securityconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#resourcerequirements-v1-core)_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#toleration-v1-core) array_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#affinity-v1-core)_ |  |  |  |


#### Shrink







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `forceUnsafe` _boolean_ | If true, executes the shrink action even if there are no replicas. |  |  |
| `maxShardSize` _string_ | The maximum size in bytes of a shard for the target index. |  |  |
| `numNewShards` _integer_ | The maximum number of primary shards in the shrunken index. |  |  |
| `percentageOfSourceShards` _integer_ | Percentage of the number of original primary shards to shrink. |  |  |
| `targetIndexNameTemplate` _string_ | The name of the shrunken index. |  |  |


#### Snapshot







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `repository` _string_ | The repository name that you register through the native snapshot API operations. |  |  |
| `snapshot` _string_ | The name of the snapshot. |  |  |


#### SnapshotConfig







_Appears in:_
- [OpensearchSnapshotPolicySpec](#opensearchsnapshotpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `dateFormat` _string_ |  |  |  |
| `dateFormatTimezone` _string_ |  |  |  |
| `indices` _string_ |  |  |  |
| `repository` _string_ |  |  |  |
| `ignoreUnavailable` _boolean_ |  |  |  |
| `includeGlobalState` _boolean_ |  |  |  |
| `partial` _boolean_ |  |  |  |
| `metadata` _object (keys:string, values:string)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |


#### SnapshotCreation







_Appears in:_
- [OpensearchSnapshotPolicySpec](#opensearchsnapshotpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `schedule` _[CronSchedule](#cronschedule)_ |  |  |  |
| `timeLimit` _string_ |  |  |  |


#### SnapshotDeleteCondition







_Appears in:_
- [SnapshotDeletion](#snapshotdeletion)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `maxCount` _integer_ |  |  |  |
| `maxAge` _string_ |  |  |  |
| `minCount` _integer_ |  |  |  |


#### SnapshotDeletion







_Appears in:_
- [OpensearchSnapshotPolicySpec](#opensearchsnapshotpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `schedule` _[CronSchedule](#cronschedule)_ |  |  |  |
| `timeLimit` _string_ |  |  |  |
| `deleteCondition` _[SnapshotDeleteCondition](#snapshotdeletecondition)_ |  |  |  |


#### SnapshotNotification







_Appears in:_
- [OpensearchSnapshotPolicySpec](#opensearchsnapshotpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `channel` _[NotificationChannel](#notificationchannel)_ |  |  |  |
| `conditions` _[NotificationConditions](#notificationconditions)_ |  |  |  |


#### SnapshotRepoConfig







_Appears in:_
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  |  |  |
| `type` _string_ |  |  |  |
| `settings` _object (keys:string, values:string)_ |  |  |  |


#### State







_Appears in:_
- [OpenSearchISMPolicySpec](#opensearchismpolicyspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `actions` _[Action](#action) array_ | The actions to execute after entering a state. |  |  |
| `name` _string_ | The name of the state. |  |  |
| `transitions` _[Transition](#transition) array_ | The next states and the conditions required to transition to those states. If no transitions exist, the policy assumes that it's complete and can now stop managing the index |  |  |


#### TenantPermissionsSpec







_Appears in:_
- [OpensearchRoleSpec](#opensearchrolespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `tenantPatterns` _string array_ |  |  |  |
| `allowedActions` _string array_ |  |  |  |


#### TlsCertificateConfig







_Appears in:_
- [DashboardsTlsConfig](#dashboardstlsconfig)
- [TlsConfigHttp](#tlsconfighttp)
- [TlsConfigTransport](#tlsconfigtransport)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Optional, name of a TLS secret that contains ca.crt, tls.key and tls.crt data. If ca.crt is in a different secret provide it via the caSecret field |  |  |
| `caSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#localobjectreference-v1-core)_ | Optional, secret that contains the ca certificate as ca.crt. If this and generate=true is set the existing CA cert from that secret is used to generate the node certs. In this case must contain ca.crt and ca.key fields |  |  |
| `duration` _[Duration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.28/#duration-v1-meta)_ | Duration controls the validity period of generated certificates (e.g. "8760h", "720h"). | 8760h |  |
| `enableHotReload` _boolean_ | Enable hot reloading of TLS certificates. When enabled, certificates are mounted as directories instead of using subPath, allowing Kubernetes to update certificate files when secrets are updated. |  |  |


#### TlsConfig



Configure tls usage for transport and http interface



_Appears in:_
- [Security](#security)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `transport` _[TlsConfigTransport](#tlsconfigtransport)_ |  |  |  |
| `http` _[TlsConfigHttp](#tlsconfighttp)_ |  |  |  |


#### TlsConfigHttp







_Appears in:_
- [TlsConfig](#tlsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled controls if TLS should be enabled for the HTTP layer.<br />If false: TLS is explicitly disabled for HTTP.<br />If not set (default): TLS is enabled if HTTP configuration is provided, or if security.tls is set.<br />If true: TLS is explicitly enabled. HTTP configuration must be provided. |  |  |
| `generate` _boolean_ | If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied |  |  |
| `customFQDN` _string_ | Custom FQDN to use for the HTTP certificate. If not set, the operator will use the default cluster DNS names. |  |  |
| `rotateDaysBeforeExpiry` _integer_ | Automatically rotate certificates before they expire, set to -1 to disable | -1 |  |
| `TlsCertificateConfig` _[TlsCertificateConfig](#tlscertificateconfig)_ |  |  |  |
| `adminDn` _string array_ | DNs of certificates that should have admin access, mainly used for securityconfig updates via securityadmin.sh, only used when existing certificates are provided |  |  |


#### TlsConfigTransport







_Appears in:_
- [TlsConfig](#tlsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | enabled controls if TLS should be enabled for the transport layer.<br />If false: TLS is explicitly disabled for transport.<br />If not set (default): TLS is enabled if transport configuration is provided, or if security.tls is set.<br />If true: TLS is explicitly enabled. Transport configuration must be provided. |  |  |
| `generate` _boolean_ | If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied |  |  |
| `perNode` _boolean_ | Configure transport node certificate |  |  |
| `rotateDaysBeforeExpiry` _integer_ | Automatically rotate certificates before they expire, set to -1 to disable | -1 |  |
| `TlsCertificateConfig` _[TlsCertificateConfig](#tlscertificateconfig)_ |  |  |  |
| `nodesDn` _string array_ | Allowed Certificate DNs for nodes, only used when existing certificates are provided |  |  |
| `adminDn` _string array_ | Deprecated: DNs of certificates that should have admin access. This field is deprecated and will be removed in a future version.<br />For OpenSearch 2.0.0+, use security.tls.http.adminDn instead. |  |  |




#### Transition







_Appears in:_
- [State](#state)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](#condition)_ | conditions for the transition. |  |  |
| `stateName` _string_ | The name of the state to transition to if the conditions are met. |  |  |


