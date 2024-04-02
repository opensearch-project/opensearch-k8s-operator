# API Reference

## Packages
- [opensearch.opster.io/v1](#opensearchopsteriov1)


## opensearch.opster.io/v1

Package v1 contains API Schema definitions for the opster v1 API group

### Resource Types
- [OpenSearchCluster](#opensearchcluster)
- [OpenSearchClusterList](#opensearchclusterlist)
- [OpenSearchISMPolicy](#opensearchismpolicy)
- [OpenSearchISMPolicyList](#opensearchismpolicylist)
- [OpensearchActionGroup](#opensearchactiongroup)
- [OpensearchActionGroupList](#opensearchactiongrouplist)
- [OpensearchComponentTemplate](#opensearchcomponenttemplate)
- [OpensearchComponentTemplateList](#opensearchcomponenttemplatelist)
- [OpensearchIndexTemplate](#opensearchindextemplate)
- [OpensearchIndexTemplateList](#opensearchindextemplatelist)
- [OpensearchRole](#opensearchrole)
- [OpensearchRoleList](#opensearchrolelist)
- [OpensearchTenant](#opensearchtenant)
- [OpensearchTenantList](#opensearchtenantlist)
- [OpensearchUser](#opensearchuser)
- [OpensearchUserList](#opensearchuserlist)
- [OpensearchUserRoleBinding](#opensearchuserrolebinding)
- [OpensearchUserRoleBindingList](#opensearchuserrolebindinglist)



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
| `readOnly` _string_ | Sets a managed index to be read only. |  |  |
| `readWrite` _string_ | Sets a managed index to be writeable. |  |  |
| `replicaCount` _[ReplicaCount](#replicacount)_ | Sets the number of replicas to assign to an index. |  |  |
| `retry` _[Retry](#retry)_ | The retry configuration for the action. |  |  |
| `rollover` _[Rollover](#rollover)_ | Rolls an alias over to a new index when the managed index meets one of the rollover conditions. |  |  |
| `rollup` _[Rollup](#rollup)_ | Periodically reduce data granularity by rolling up old data into summarized indexes. |  |  |
| `shrink` _[Shrink](#shrink)_ | Allows you to reduce the number of primary shards in your indexes |  |  |
| `snapshot` _[Snapshot](#snapshot)_ | Back up your cluster’s indexes and state |  |  |
| `timeout` _string_ | The timeout period for the action. |  |  |


#### AdditionalVolume







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name to use for the volume. Required. |  |  |
| `path` _string_ | Path in the container to mount the volume at. Required. |  |  |
| `subPath` _string_ | SubPath of the referenced volume to mount. |  |  |
| `secret` _[SecretVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#secretvolumesource-v1-core)_ | Secret to use populate the volume |  |  |
| `configMap` _[ConfigMapVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#configmapvolumesource-v1-core)_ | ConfigMap to use to populate the volume |  |  |
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#emptydirvolumesource-v1-core)_ | EmptyDir to use to populate the volume |  |  |
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
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#toleration-v1-core) array_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#affinity-v1-core)_ |  |  |  |
| `jvm` _string_ |  |  |  |
| `additionalConfig` _object (keys:string, values:string)_ | Extra items to add to the opensearch.yml, defaults to General.AdditionalConfig |  |  |


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


#### ClusterStatus



ClusterStatus defines the observed state of Es



_Appears in:_
- [OpenSearchCluster](#opensearchcluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _string_ | INSERT ADDITIONAL STATUS FIELD - define observed state of cluster<br />Important: Run "make" to regenerate code after modifying this file |  |  |
| `componentsStatus` _[ComponentStatus](#componentstatus) array_ |  |  |  |
| `version` _string_ |  |  |  |
| `initialized` _boolean_ |  |  |  |
| `availableNodes` _integer_ | AvailableNodes is the number of available instances. |  |  |
| `health` _[OpenSearchHealth](#opensearchhealth)_ |  |  |  |


#### ComponentStatus







_Appears in:_
- [ClusterStatus](#clusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `component` _string_ |  |  |  |
| `status` _string_ |  |  |  |
| `description` _string_ |  |  |  |
| `conditions` _string array_ |  |  |  |


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
| `smartScaler` _boolean_ |  |  |  |


#### Cron







_Appears in:_
- [Condition](#condition)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `expression` _string_ | The cron expression that triggers the transition. |  |  |
| `timezone` _string_ | The timezone that triggers the transition. |  |  |


#### DashboardsConfig







_Appears in:_
- [ClusterSpec](#clusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ |  |  |  |
| `replicas` _integer_ |  |  |  |
| `tls` _[DashboardsTlsConfig](#dashboardstlsconfig)_ |  |  |  |
| `version` _string_ |  |  |  |
| `basePath` _string_ | Base Path for Opensearch Clusters running behind a reverse proxy |  |  |
| `additionalConfig` _object (keys:string, values:string)_ | Additional properties for opensearch_dashboards.yaml |  |  |
| `opensearchCredentialsSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ | Secret that contains fields username and password for dashboards to use to login to opensearch, must only be supplied if a custom securityconfig is provided |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#envvar-v1-core) array_ |  |  |  |
| `additionalVolumes` _[AdditionalVolume](#additionalvolume) array_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#toleration-v1-core) array_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#affinity-v1-core)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `service` _[DashboardsServiceSpec](#dashboardsservicespec)_ |  |  |  |
| `pluginsList` _string array_ |  |  |  |
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#podsecuritycontext-v1-core)_ | Set security context for the dashboards pods |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#securitycontext-v1-core)_ | Set security context for the dashboards pods' container |  |  |


#### DashboardsServiceSpec







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ServiceType](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#servicetype-v1-core)_ |  | ClusterIP | Enum: [ClusterIP NodePort LoadBalancer] <br /> |
| `loadBalancerSourceRanges` _string array_ |  |  |  |


#### DashboardsTlsConfig







_Appears in:_
- [DashboardsConfig](#dashboardsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ | Enable HTTPS for Dashboards |  |  |
| `generate` _boolean_ | Generate certificate, if false secret must be provided |  |  |
| `TlsCertificateConfig` _[TlsCertificateConfig](#tlscertificateconfig)_ | foobar |  |  |


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
| `setVMMaxMapCount` _boolean_ |  |  |  |
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
| `podSecurityContext` _[PodSecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#podsecuritycontext-v1-core)_ | Set security context for the cluster pods |  |  |
| `securityContext` _[SecurityContext](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#securitycontext-v1-core)_ | Set security context for the cluster pods' container |  |  |


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
| `imagePullPolicy` _[PullPolicy](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#pullpolicy-v1-core)_ |  |  |  |
| `imagePullSecrets` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core) array_ |  |  |  |


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
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ |  |  |  |
| `version` _string_ |  |  |  |


#### KeystoreValue







_Appears in:_
- [GeneralConfig](#generalconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ | Secret containing key value pairs |  |  |
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
| `diskSize` _string_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#resourcerequirements-v1-core)_ |  |  |  |
| `jvm` _string_ |  |  |  |
| `roles` _string array_ |  |  |  |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#toleration-v1-core) array_ |  |  |  |
| `nodeSelector` _object (keys:string, values:string)_ |  |  |  |
| `affinity` _[Affinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#affinity-v1-core)_ |  |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#topologyspreadconstraint-v1-core) array_ |  |  |  |
| `persistence` _[PersistenceConfig](#persistenceconfig)_ |  |  |  |
| `additionalConfig` _object (keys:string, values:string)_ |  |  |  |
| `labels` _object (keys:string, values:string)_ |  |  |  |
| `annotations` _object (keys:string, values:string)_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#envvar-v1-core) array_ |  |  |  |
| `priorityClassName` _string_ |  |  |  |
| `pdb` _[PdbConfig](#pdbconfig)_ |  |  |  |
| `probes` _[ProbesConfig](#probesconfig)_ |  |  |  |


#### Notification







_Appears in:_
- [Action](#action)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `destination` _string_ |  |  |  |
| `messageTemplate` _[MessageTemplate](#messagetemplate)_ |  |  |  |


#### Open







_Appears in:_
- [Action](#action)



#### OpenSearchCluster



Es is the Schema for the es API



_Appears in:_
- [OpenSearchClusterList](#opensearchclusterlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpenSearchCluster` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[ClusterSpec](#clusterspec)_ |  |  |  |
| `status` _[ClusterStatus](#clusterstatus)_ |  |  |  |


#### OpenSearchClusterList



EsList contains a list of Es





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpenSearchClusterList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[OpenSearchCluster](#opensearchcluster) array_ |  |  |  |


#### OpenSearchHealth

_Underlying type:_ _string_

OpenSearchHealth is the health of the cluster as returned by the health API.



_Appears in:_
- [ClusterStatus](#clusterstatus)



#### OpenSearchISMPolicy







_Appears in:_
- [OpenSearchISMPolicyList](#opensearchismpolicylist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpenSearchISMPolicy` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpenSearchISMPolicySpec](#opensearchismpolicyspec)_ |  |  |  |
| `status` _[OpensearchISMPolicyStatus](#opensearchismpolicystatus)_ |  |  |  |


#### OpenSearchISMPolicyList



ISMPolicyList contains a list of ISMPolicy





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpenSearchISMPolicyList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[OpenSearchISMPolicy](#opensearchismpolicy) array_ |  |  |  |


#### OpenSearchISMPolicySpec



ISMPolicySpec is the specification for the ISM policy for OS.



_Appears in:_
- [OpenSearchISMPolicy](#opensearchismpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ |  |  |  |
| `defaultState` _string_ | The default starting state for each index that uses this policy. |  |  |
| `description` _string_ | A human-readable description of the policy. |  |  |
| `errorNotification` _[ErrorNotification](#errornotification)_ |  |  |  |
| `ismTemplate` _[ISMTemplate](#ismtemplate)_ | Specify an ISM template pattern that matches the index to apply the policy. |  |  |
| `policyId` _string_ |  |  |  |
| `states` _[State](#state) array_ | The states that you define in the policy. |  |  |


#### OpensearchActionGroup



OpensearchActionGroup is the Schema for the opensearchactiongroups API



_Appears in:_
- [OpensearchActionGroupList](#opensearchactiongrouplist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchActionGroup` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchActionGroupSpec](#opensearchactiongroupspec)_ |  |  |  |
| `status` _[OpensearchActionGroupStatus](#opensearchactiongroupstatus)_ |  |  |  |


#### OpensearchActionGroupList



OpensearchActionGroupList contains a list of OpensearchActionGroup





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchActionGroupList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[OpensearchActionGroup](#opensearchactiongroup) array_ |  |  |  |


#### OpensearchActionGroupSpec



OpensearchActionGroupSpec defines the desired state of OpensearchActionGroup



_Appears in:_
- [OpensearchActionGroup](#opensearchactiongroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ |  |  |  |
| `allowedActions` _string array_ |  |  |  |
| `type` _string_ |  |  |  |
| `description` _string_ |  |  |  |


#### OpensearchActionGroupState

_Underlying type:_ _string_





_Appears in:_
- [OpensearchActionGroupStatus](#opensearchactiongroupstatus)



#### OpensearchActionGroupStatus



OpensearchActionGroupStatus defines the observed state of OpensearchActionGroup



_Appears in:_
- [OpensearchActionGroup](#opensearchactiongroup)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[OpensearchActionGroupState](#opensearchactiongroupstate)_ |  |  |  |
| `reason` _string_ |  |  |  |
| `existingActionGroup` _boolean_ |  |  |  |
| `managedCluster` _[UID](#uid)_ |  |  |  |




#### OpensearchComponentTemplate



OpensearchComponentTemplate is the schema for the OpenSearch component templates API



_Appears in:_
- [OpensearchComponentTemplateList](#opensearchcomponenttemplatelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchComponentTemplate` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchComponentTemplateSpec](#opensearchcomponenttemplatespec)_ |  |  |  |
| `status` _[OpensearchComponentTemplateStatus](#opensearchcomponenttemplatestatus)_ |  |  |  |


#### OpensearchComponentTemplateList



OpensearchComponentTemplateList contains a list of OpensearchComponentTemplate





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchComponentTemplateList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[OpensearchComponentTemplate](#opensearchcomponenttemplate) array_ |  |  |  |


#### OpensearchComponentTemplateSpec







_Appears in:_
- [OpensearchComponentTemplate](#opensearchcomponenttemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ |  |  |  |
| `name` _string_ | The name of the component template. Defaults to metadata.name |  |  |
| `template` _[OpensearchIndexSpec](#opensearchindexspec)_ | The template that should be applied |  |  |
| `version` _integer_ | Version number used to manage the component template externally |  |  |
| `allowAutoCreate` _boolean_ | If true, then indices can be automatically created using this template |  |  |
| `_meta` _[JSON](#json)_ | Optional user metadata about the component template |  |  |


#### OpensearchComponentTemplateState

_Underlying type:_ _string_





_Appears in:_
- [OpensearchComponentTemplateStatus](#opensearchcomponenttemplatestatus)



#### OpensearchComponentTemplateStatus







_Appears in:_
- [OpensearchComponentTemplate](#opensearchcomponenttemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[OpensearchComponentTemplateState](#opensearchcomponenttemplatestate)_ |  |  |  |
| `reason` _string_ |  |  |  |
| `existingComponentTemplate` _boolean_ |  |  |  |
| `managedCluster` _[UID](#uid)_ |  |  |  |
| `componentTemplateName` _string_ | Name of the currently managed component template |  |  |


#### OpensearchISMPolicyState

_Underlying type:_ _string_





_Appears in:_
- [OpensearchISMPolicyStatus](#opensearchismpolicystatus)



#### OpensearchISMPolicyStatus



OpensearchISMPolicyStatus defines the observed state of OpensearchISMPolicy



_Appears in:_
- [OpenSearchISMPolicy](#opensearchismpolicy)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[OpensearchISMPolicyState](#opensearchismpolicystate)_ |  |  |  |
| `reason` _string_ |  |  |  |
| `existingISMPolicy` _boolean_ |  |  |  |
| `managedCluster` _[UID](#uid)_ |  |  |  |
| `policyId` _string_ |  |  |  |


#### OpensearchIndexAliasSpec



Describes the specs of an index alias



_Appears in:_
- [OpensearchIndexSpec](#opensearchindexspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `index` _string_ | The name of the index that the alias points to. |  |  |
| `alias` _string_ | The name of the alias. |  |  |
| `filter` _[JSON](#json)_ | Query used to limit documents the alias can access. |  |  |
| `routing` _string_ | Value used to route indexing and search operations to a specific shard. |  |  |
| `isWriteIndex` _boolean_ | If true, the index is the write index for the alias |  |  |


#### OpensearchIndexSpec



Describes the specs of an index



_Appears in:_
- [OpensearchComponentTemplateSpec](#opensearchcomponenttemplatespec)
- [OpensearchIndexTemplateSpec](#opensearchindextemplatespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `settings` _[JSON](#json)_ | Configuration options for the index |  |  |
| `mappings` _[JSON](#json)_ | Mapping for fields in the index |  |  |
| `aliases` _object (keys:string, values:[OpensearchIndexAliasSpec](#opensearchindexaliasspec))_ | Aliases to add |  |  |


#### OpensearchIndexTemplate



OpensearchIndexTemplate is the schema for the OpenSearch index templates API



_Appears in:_
- [OpensearchIndexTemplateList](#opensearchindextemplatelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchIndexTemplate` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchIndexTemplateSpec](#opensearchindextemplatespec)_ |  |  |  |
| `status` _[OpensearchIndexTemplateStatus](#opensearchindextemplatestatus)_ |  |  |  |


#### OpensearchIndexTemplateList



OpensearchIndexTemplateList contains a list of OpensearchIndexTemplate





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchIndexTemplateList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[OpensearchIndexTemplate](#opensearchindextemplate) array_ |  |  |  |


#### OpensearchIndexTemplateSpec







_Appears in:_
- [OpensearchIndexTemplate](#opensearchindextemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ |  |  |  |
| `name` _string_ | The name of the index template. Defaults to metadata.name |  |  |
| `indexPatterns` _string array_ | Array of wildcard expressions used to match the names of indices during creation |  |  |
| `template` _[OpensearchIndexSpec](#opensearchindexspec)_ | The template that should be applied |  |  |
| `composedOf` _string array_ | An ordered list of component template names. Component templates are merged in the order specified,<br />meaning that the last component template specified has the highest precedence |  |  |
| `priority` _integer_ | Priority to determine index template precedence when a new data stream or index is created.<br />The index template with the highest priority is chosen |  |  |
| `version` _integer_ | Version number used to manage the component template externally |  |  |
| `_meta` _[JSON](#json)_ | Optional user metadata about the index template |  |  |


#### OpensearchIndexTemplateState

_Underlying type:_ _string_





_Appears in:_
- [OpensearchIndexTemplateStatus](#opensearchindextemplatestatus)



#### OpensearchIndexTemplateStatus







_Appears in:_
- [OpensearchIndexTemplate](#opensearchindextemplate)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[OpensearchIndexTemplateState](#opensearchindextemplatestate)_ |  |  |  |
| `reason` _string_ |  |  |  |
| `existingIndexTemplate` _boolean_ |  |  |  |
| `managedCluster` _[UID](#uid)_ |  |  |  |
| `indexTemplateName` _string_ | Name of the currently managed index template |  |  |


#### OpensearchRole



OpensearchRole is the Schema for the opensearchroles API



_Appears in:_
- [OpensearchRoleList](#opensearchrolelist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchRole` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchRoleSpec](#opensearchrolespec)_ |  |  |  |
| `status` _[OpensearchRoleStatus](#opensearchrolestatus)_ |  |  |  |


#### OpensearchRoleList



OpensearchRoleList contains a list of OpensearchRole





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchRoleList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[OpensearchRole](#opensearchrole) array_ |  |  |  |


#### OpensearchRoleSpec



OpensearchRoleSpec defines the desired state of OpensearchRole



_Appears in:_
- [OpensearchRole](#opensearchrole)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ |  |  |  |
| `clusterPermissions` _string array_ |  |  |  |
| `indexPermissions` _[IndexPermissionSpec](#indexpermissionspec) array_ |  |  |  |
| `tenantPermissions` _[TenantPermissionsSpec](#tenantpermissionsspec) array_ |  |  |  |


#### OpensearchRoleState

_Underlying type:_ _string_





_Appears in:_
- [OpensearchRoleStatus](#opensearchrolestatus)



#### OpensearchRoleStatus



OpensearchRoleStatus defines the observed state of OpensearchRole



_Appears in:_
- [OpensearchRole](#opensearchrole)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[OpensearchRoleState](#opensearchrolestate)_ |  |  |  |
| `reason` _string_ |  |  |  |
| `existingRole` _boolean_ |  |  |  |
| `managedCluster` _[UID](#uid)_ |  |  |  |


#### OpensearchTenant



OpensearchTenant is the Schema for the opensearchtenants API



_Appears in:_
- [OpensearchTenantList](#opensearchtenantlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchTenant` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchTenantSpec](#opensearchtenantspec)_ |  |  |  |
| `status` _[OpensearchTenantStatus](#opensearchtenantstatus)_ |  |  |  |


#### OpensearchTenantList



OpensearchTenantList contains a list of OpensearchTenant





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchTenantList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[OpensearchTenant](#opensearchtenant) array_ |  |  |  |


#### OpensearchTenantSpec



OpensearchTenantSpec defines the desired state of OpensearchTenant



_Appears in:_
- [OpensearchTenant](#opensearchtenant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ |  |  |  |
| `description` _string_ |  |  |  |


#### OpensearchTenantState

_Underlying type:_ _string_





_Appears in:_
- [OpensearchTenantStatus](#opensearchtenantstatus)



#### OpensearchTenantStatus



OpensearchTenantStatus defines the observed state of OpensearchTenant



_Appears in:_
- [OpensearchTenant](#opensearchtenant)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[OpensearchTenantState](#opensearchtenantstate)_ |  |  |  |
| `reason` _string_ |  |  |  |
| `existingTenant` _boolean_ |  |  |  |
| `managedCluster` _[UID](#uid)_ |  |  |  |


#### OpensearchUser



OpensearchUser is the Schema for the opensearchusers API



_Appears in:_
- [OpensearchUserList](#opensearchuserlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchUser` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchUserSpec](#opensearchuserspec)_ |  |  |  |
| `status` _[OpensearchUserStatus](#opensearchuserstatus)_ |  |  |  |


#### OpensearchUserList



OpensearchUserList contains a list of OpensearchUser





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchUserList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[OpensearchUser](#opensearchuser) array_ |  |  |  |


#### OpensearchUserRoleBinding



OpensearchUserRoleBinding is the Schema for the opensearchuserrolebindings API



_Appears in:_
- [OpensearchUserRoleBindingList](#opensearchuserrolebindinglist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchUserRoleBinding` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[OpensearchUserRoleBindingSpec](#opensearchuserrolebindingspec)_ |  |  |  |
| `status` _[OpensearchUserRoleBindingStatus](#opensearchuserrolebindingstatus)_ |  |  |  |


#### OpensearchUserRoleBindingList



OpensearchUserRoleBindingList contains a list of OpensearchUserRoleBinding





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `opensearch.opster.io/v1` | | |
| `kind` _string_ | `OpensearchUserRoleBindingList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[OpensearchUserRoleBinding](#opensearchuserrolebinding) array_ |  |  |  |


#### OpensearchUserRoleBindingSpec



OpensearchUserRoleBindingSpec defines the desired state of OpensearchUserRoleBinding



_Appears in:_
- [OpensearchUserRoleBinding](#opensearchuserrolebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ |  |  |  |
| `roles` _string array_ |  |  |  |
| `users` _string array_ |  |  |  |
| `backendRoles` _string array_ |  |  |  |


#### OpensearchUserRoleBindingState

_Underlying type:_ _string_





_Appears in:_
- [OpensearchUserRoleBindingStatus](#opensearchuserrolebindingstatus)



#### OpensearchUserRoleBindingStatus



OpensearchUserRoleBindingStatus defines the observed state of OpensearchUserRoleBinding



_Appears in:_
- [OpensearchUserRoleBinding](#opensearchuserrolebinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[OpensearchUserRoleBindingState](#opensearchuserrolebindingstate)_ |  |  |  |
| `reason` _string_ |  |  |  |
| `managedCluster` _[UID](#uid)_ |  |  |  |
| `provisionedRoles` _string array_ |  |  |  |
| `provisionedUsers` _string array_ |  |  |  |
| `provisionedBackendRoles` _string array_ |  |  |  |


#### OpensearchUserSpec



OpensearchUserSpec defines the desired state of OpensearchUser



_Appears in:_
- [OpensearchUser](#opensearchuser)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `opensearchCluster` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ |  |  |  |
| `passwordFrom` _[SecretKeySelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#secretkeyselector-v1-core)_ |  |  |  |
| `opendistroSecurityRoles` _string array_ |  |  |  |
| `backendRoles` _string array_ |  |  |  |
| `attributes` _object (keys:string, values:string)_ |  |  |  |


#### OpensearchUserState

_Underlying type:_ _string_





_Appears in:_
- [OpensearchUserStatus](#opensearchuserstatus)



#### OpensearchUserStatus



OpensearchUserStatus defines the observed state of OpensearchUser



_Appears in:_
- [OpensearchUser](#opensearchuser)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[OpensearchUserState](#opensearchuserstate)_ |  |  |  |
| `reason` _string_ |  |  |  |
| `managedCluster` _[UID](#uid)_ |  |  |  |


#### PVCSource







_Appears in:_
- [PersistenceSource](#persistencesource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `storageClass` _string_ |  |  |  |
| `accessModes` _[PersistentVolumeAccessMode](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#persistentvolumeaccessmode-v1-core) array_ |  |  |  |


#### PdbConfig







_Appears in:_
- [NodePool](#nodepool)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enable` _boolean_ |  |  |  |
| `minAvailable` _[IntOrString](#intorstring)_ |  |  |  |
| `maxUnavailable` _[IntOrString](#intorstring)_ |  |  |  |


#### PersistenceConfig



PersistencConfig defines options for data persistence



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
| `emptyDir` _[EmptyDirVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#emptydirvolumesource-v1-core)_ |  |  |  |
| `hostPath` _[HostPathVolumeSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#hostpathvolumesource-v1-core)_ |  |  |  |


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
| `readiness` _[ReadinessProbeConfig](#readinessprobeconfig)_ |  |  |  |
| `startup` _[ProbeConfig](#probeconfig)_ |  |  |  |


#### ReadinessProbeConfig







_Appears in:_
- [ProbesConfig](#probesconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `initialDelaySeconds` _integer_ |  |  |  |
| `periodSeconds` _integer_ |  |  |  |
| `timeoutSeconds` _integer_ |  |  |  |
| `failureThreshold` _integer_ |  |  |  |


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
| `securityConfigSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ | Secret that contains the differnt yml files of the opensearch-security config (config.yml, internal_users.yml, ...) |  |  |
| `adminSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ | TLS Secret that contains a client certificate (tls.key, tls.crt, ca.crt) with admin rights in the opensearch cluster. Must be set if transport certificates are provided by user and not generated |  |  |
| `adminCredentialsSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ | Secret that contains fields username and password to be used by the operator to access the opensearch cluster for node draining. Must be set if custom securityconfig is provided. |  |  |


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
| `secret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ | Optional, name of a TLS secret that contains ca.crt, tls.key and tls.crt data. If ca.crt is in a different secret provide it via the caSecret field |  |  |
| `caSecret` _[LocalObjectReference](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.22/#localobjectreference-v1-core)_ | Optional, secret that contains the ca certificate as ca.crt. If this and generate=true is set the existing CA cert from that secret is used to generate the node certs. In this case must contain ca.crt and ca.key fields |  |  |


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
| `generate` _boolean_ | If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied |  |  |
| `TlsCertificateConfig` _[TlsCertificateConfig](#tlscertificateconfig)_ |  |  |  |


#### TlsConfigTransport







_Appears in:_
- [TlsConfig](#tlsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `generate` _boolean_ | If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied |  |  |
| `perNode` _boolean_ | Configure transport node certificate |  |  |
| `TlsCertificateConfig` _[TlsCertificateConfig](#tlscertificateconfig)_ |  |  |  |
| `nodesDn` _string array_ | Allowed Certificate DNs for nodes, only used when existing certificates are provided |  |  |
| `adminDn` _string array_ | DNs of certificates that should have admin access, mainly used for securityconfig updates via securityadmin.sh, only used when existing certificates are provided |  |  |




#### Transition







_Appears in:_
- [State](#state)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `conditions` _[Condition](#condition)_ | conditions for the transition. |  |  |
| `stateName` _string_ | The name of the state to transition to if the conditions are met. |  |  |


