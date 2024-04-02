<p>Packages:</p>
<ul>
<li>
<a href="#opensearch.opster.io%2fv1">opensearch.opster.io/v1</a>
</li>
</ul>
<h2 id="opensearch.opster.io/v1">opensearch.opster.io/v1</h2>
<div>
<p>Package v1 contains API Schema definitions for the opster v1 API group</p>
</div>
Resource Types:
<ul></ul>
<h3 id="opensearch.opster.io/v1.Action">Action
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.State">State</a>)
</p>
<div>
<p>Actions are the steps that the policy sequentially executes on entering a specific state.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>alias</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Alias">
Alias
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>allocation</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Allocation">
Allocation
</a>
</em>
</td>
<td>
<p>Allocate the index to a node with a specific attribute set</p>
</td>
</tr>
<tr>
<td>
<code>close</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Close">
Close
</a>
</em>
</td>
<td>
<p>Closes the managed index.</p>
</td>
</tr>
<tr>
<td>
<code>delete</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Delete">
Delete
</a>
</em>
</td>
<td>
<p>Deletes a managed index.</p>
</td>
</tr>
<tr>
<td>
<code>forceMerge</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ForceMerge">
ForceMerge
</a>
</em>
</td>
<td>
<p>Reduces the number of Lucene segments by merging the segments of individual shards.</p>
</td>
</tr>
<tr>
<td>
<code>indexPriority</code><br/>
<em>
<a href="#opensearch.opster.io/v1.IndexPriority">
IndexPriority
</a>
</em>
</td>
<td>
<p>Set the priority for the index in a specific state.</p>
</td>
</tr>
<tr>
<td>
<code>notification</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Notification">
Notification
</a>
</em>
</td>
<td>
<p>Name          string        <code>json:&quot;name,omitempty&quot;</code></p>
</td>
</tr>
<tr>
<td>
<code>open</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Open">
Open
</a>
</em>
</td>
<td>
<p>Opens a managed index.</p>
</td>
</tr>
<tr>
<td>
<code>readOnly</code><br/>
<em>
string
</em>
</td>
<td>
<p>Sets a managed index to be read only.</p>
</td>
</tr>
<tr>
<td>
<code>readWrite</code><br/>
<em>
string
</em>
</td>
<td>
<p>Sets a managed index to be writeable.</p>
</td>
</tr>
<tr>
<td>
<code>replicaCount</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ReplicaCount">
ReplicaCount
</a>
</em>
</td>
<td>
<p>Sets the number of replicas to assign to an index.</p>
</td>
</tr>
<tr>
<td>
<code>retry</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Retry">
Retry
</a>
</em>
</td>
<td>
<p>The retry configuration for the action.</p>
</td>
</tr>
<tr>
<td>
<code>rollover</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Rollover">
Rollover
</a>
</em>
</td>
<td>
<p>Rolls an alias over to a new index when the managed index meets one of the rollover conditions.</p>
</td>
</tr>
<tr>
<td>
<code>rollup</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Rollup">
Rollup
</a>
</em>
</td>
<td>
<p>Periodically reduce data granularity by rolling up old data into summarized indexes.</p>
</td>
</tr>
<tr>
<td>
<code>shrink</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Shrink">
Shrink
</a>
</em>
</td>
<td>
<p>Allows you to reduce the number of primary shards in your indexes</p>
</td>
</tr>
<tr>
<td>
<code>snapshot</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Snapshot">
Snapshot
</a>
</em>
</td>
<td>
<p>Back up your cluster’s indexes and state</p>
</td>
</tr>
<tr>
<td>
<code>timeout</code><br/>
<em>
string
</em>
</td>
<td>
<p>The timeout period for the action.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.AdditionalVolume">AdditionalVolume
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.DashboardsConfig">DashboardsConfig</a>, <a href="#opensearch.opster.io/v1.GeneralConfig">GeneralConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name to use for the volume. Required.</p>
</td>
</tr>
<tr>
<td>
<code>path</code><br/>
<em>
string
</em>
</td>
<td>
<p>Path in the container to mount the volume at. Required.</p>
</td>
</tr>
<tr>
<td>
<code>subPath</code><br/>
<em>
string
</em>
</td>
<td>
<p>SubPath of the referenced volume to mount.</p>
</td>
</tr>
<tr>
<td>
<code>secret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretvolumesource-v1-core">
Kubernetes core/v1.SecretVolumeSource
</a>
</em>
</td>
<td>
<p>Secret to use populate the volume</p>
</td>
</tr>
<tr>
<td>
<code>configMap</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#configmapvolumesource-v1-core">
Kubernetes core/v1.ConfigMapVolumeSource
</a>
</em>
</td>
<td>
<p>ConfigMap to use to populate the volume</p>
</td>
</tr>
<tr>
<td>
<code>emptyDir</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#emptydirvolumesource-v1-core">
Kubernetes core/v1.EmptyDirVolumeSource
</a>
</em>
</td>
<td>
<p>EmptyDir to use to populate the volume</p>
</td>
</tr>
<tr>
<td>
<code>restartPods</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Whether to restart the pods on content change</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Alias">Alias
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>actions</code><br/>
<em>
<a href="#opensearch.opster.io/v1.AliasAction">
[]AliasAction
</a>
</em>
</td>
<td>
<p>Allocate the index to a node with a specified attribute.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.AliasAction">AliasAction
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Alias">Alias</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>add</code><br/>
<em>
<a href="#opensearch.opster.io/v1.AliasDetails">
AliasDetails
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>remove</code><br/>
<em>
<a href="#opensearch.opster.io/v1.AliasDetails">
AliasDetails
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.AliasDetails">AliasDetails
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.AliasAction">AliasAction</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>index</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the index that the alias points to.</p>
</td>
</tr>
<tr>
<td>
<code>aliases</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>The name of the alias.</p>
</td>
</tr>
<tr>
<td>
<code>routing</code><br/>
<em>
string
</em>
</td>
<td>
<p>Limit search to an associated shard value</p>
</td>
</tr>
<tr>
<td>
<code>isWriteIndex</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Specify the index that accepts any write operations to the alias.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Allocation">Allocation
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>exclude</code><br/>
<em>
string
</em>
</td>
<td>
<p>Allocate the index to a node with a specified attribute.</p>
</td>
</tr>
<tr>
<td>
<code>include</code><br/>
<em>
string
</em>
</td>
<td>
<p>Allocate the index to a node with any of the specified attributes.</p>
</td>
</tr>
<tr>
<td>
<code>require</code><br/>
<em>
string
</em>
</td>
<td>
<p>Don’t allocate the index to a node with any of the specified attributes.</p>
</td>
</tr>
<tr>
<td>
<code>waitFor</code><br/>
<em>
string
</em>
</td>
<td>
<p>Wait for the policy to execute before allocating the index to a node with a specified attribute.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.BootstrapConfig">BootstrapConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tolerations</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>nodeSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>affinity</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>jvm</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>additionalConfig</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Extra items to add to the opensearch.yml, defaults to General.AdditionalConfig</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Close">Close
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<h3 id="opensearch.opster.io/v1.ClusterSpec">ClusterSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpenSearchCluster">OpenSearchCluster</a>)
</p>
<div>
<p>ClusterSpec defines the desired state of OpenSearchCluster</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>general</code><br/>
<em>
<a href="#opensearch.opster.io/v1.GeneralConfig">
GeneralConfig
</a>
</em>
</td>
<td>
<p>INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
Important: Run &ldquo;make&rdquo; to regenerate code after modifying this file</p>
</td>
</tr>
<tr>
<td>
<code>confMgmt</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ConfMgmt">
ConfMgmt
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>bootstrap</code><br/>
<em>
<a href="#opensearch.opster.io/v1.BootstrapConfig">
BootstrapConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>dashboards</code><br/>
<em>
<a href="#opensearch.opster.io/v1.DashboardsConfig">
DashboardsConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>security</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Security">
Security
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>nodePools</code><br/>
<em>
<a href="#opensearch.opster.io/v1.NodePool">
[]NodePool
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>initHelper</code><br/>
<em>
<a href="#opensearch.opster.io/v1.InitHelperConfig">
InitHelperConfig
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ClusterStatus">ClusterStatus
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpenSearchCluster">OpenSearchCluster</a>)
</p>
<div>
<p>ClusterStatus defines the observed state of Es</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>phase</code><br/>
<em>
string
</em>
</td>
<td>
<p>INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
Important: Run &ldquo;make&rdquo; to regenerate code after modifying this file</p>
</td>
</tr>
<tr>
<td>
<code>componentsStatus</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ComponentStatus">
[]ComponentStatus
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>initialized</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>availableNodes</code><br/>
<em>
int32
</em>
</td>
<td>
<p>AvailableNodes is the number of available instances.</p>
</td>
</tr>
<tr>
<td>
<code>health</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpenSearchHealth">
OpenSearchHealth
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ComponentStatus">ComponentStatus
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ClusterStatus">ClusterStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>component</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>conditions</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Condition">Condition
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Transition">Transition</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>cron</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Cron">
Cron
</a>
</em>
</td>
<td>
<p>The cron job that triggers the transition if no other transition happens first.</p>
</td>
</tr>
<tr>
<td>
<code>minDocCount</code><br/>
<em>
int64
</em>
</td>
<td>
<p>The minimum document count of the index required to transition.</p>
</td>
</tr>
<tr>
<td>
<code>minIndexAge</code><br/>
<em>
string
</em>
</td>
<td>
<p>The minimum age of the index required to transition.</p>
</td>
</tr>
<tr>
<td>
<code>minRolloverAge</code><br/>
<em>
string
</em>
</td>
<td>
<p>The minimum age required after a rollover has occurred to transition to the next state.</p>
</td>
</tr>
<tr>
<td>
<code>minSize</code><br/>
<em>
string
</em>
</td>
<td>
<p>The minimum size of the total primary shard storage (not counting replicas) required to transition.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ConfMgmt">ConfMgmt
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
<p>ConfMgmt defines which additional services will be deployed</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>autoScaler</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>VerUpdate</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>smartScaler</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Cron">Cron
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Condition">Condition</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>expression</code><br/>
<em>
string
</em>
</td>
<td>
<p>The cron expression that triggers the transition.</p>
</td>
</tr>
<tr>
<td>
<code>timezone</code><br/>
<em>
string
</em>
</td>
<td>
<p>The timezone that triggers the transition.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.DashboardsConfig">DashboardsConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ImageSpec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ImageSpec">
ImageSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>ImageSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>enable</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>replicas</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tls</code><br/>
<em>
<a href="#opensearch.opster.io/v1.DashboardsTlsConfig">
DashboardsTlsConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>basePath</code><br/>
<em>
string
</em>
</td>
<td>
<p>Base Path for Opensearch Clusters running behind a reverse proxy</p>
</td>
</tr>
<tr>
<td>
<code>additionalConfig</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Additional properties for opensearch_dashboards.yaml</p>
</td>
</tr>
<tr>
<td>
<code>opensearchCredentialsSecret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Secret that contains fields username and password for dashboards to use to login to opensearch, must only be supplied if a custom securityconfig is provided</p>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>additionalVolumes</code><br/>
<em>
<a href="#opensearch.opster.io/v1.AdditionalVolume">
[]AdditionalVolume
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tolerations</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>nodeSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>affinity</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>service</code><br/>
<em>
<a href="#opensearch.opster.io/v1.DashboardsServiceSpec">
DashboardsServiceSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>pluginsList</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>podSecurityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#podsecuritycontext-v1-core">
Kubernetes core/v1.PodSecurityContext
</a>
</em>
</td>
<td>
<p>Set security context for the dashboards pods</p>
</td>
</tr>
<tr>
<td>
<code>securityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#securitycontext-v1-core">
Kubernetes core/v1.SecurityContext
</a>
</em>
</td>
<td>
<p>Set security context for the dashboards pods&rsquo; container</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.DashboardsServiceSpec">DashboardsServiceSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.DashboardsConfig">DashboardsConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#servicetype-v1-core">
Kubernetes core/v1.ServiceType
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>loadBalancerSourceRanges</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.DashboardsTlsConfig">DashboardsTlsConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.DashboardsConfig">DashboardsConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enable</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Enable HTTPS for Dashboards</p>
</td>
</tr>
<tr>
<td>
<code>generate</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Generate certificate, if false secret must be provided</p>
</td>
</tr>
<tr>
<td>
<code>TlsCertificateConfig</code><br/>
<em>
<a href="#opensearch.opster.io/v1.TlsCertificateConfig">
TlsCertificateConfig
</a>
</em>
</td>
<td>
<p>foobar</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Delete">Delete
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<h3 id="opensearch.opster.io/v1.Destination">Destination
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ErrorNotification">ErrorNotification</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>slack</code><br/>
<em>
<a href="#opensearch.opster.io/v1.DestinationURL">
DestinationURL
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>amazon</code><br/>
<em>
<a href="#opensearch.opster.io/v1.DestinationURL">
DestinationURL
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>chime</code><br/>
<em>
<a href="#opensearch.opster.io/v1.DestinationURL">
DestinationURL
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>customWebhook</code><br/>
<em>
<a href="#opensearch.opster.io/v1.DestinationURL">
DestinationURL
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.DestinationURL">DestinationURL
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Destination">Destination</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>url</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ErrorNotification">ErrorNotification
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpenSearchISMPolicySpec">OpenSearchISMPolicySpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>destination</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Destination">
Destination
</a>
</em>
</td>
<td>
<p>The destination URL.</p>
</td>
</tr>
<tr>
<td>
<code>channel</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>messageTemplate</code><br/>
<em>
<a href="#opensearch.opster.io/v1.MessageTemplate">
MessageTemplate
</a>
</em>
</td>
<td>
<p>The text of the message</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ForceMerge">ForceMerge
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>maxNumSegments</code><br/>
<em>
int64
</em>
</td>
<td>
<p>The number of segments to reduce the shard to.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.GeneralConfig">GeneralConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ImageSpec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ImageSpec">
ImageSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>ImageSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>httpPort</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>vendor</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>serviceAccount</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>serviceName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>setVMMaxMapCount</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>defaultRepo</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>additionalConfig</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Extra items to add to the opensearch.yml</p>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Adds support for annotations in services</p>
</td>
</tr>
<tr>
<td>
<code>drainDataNodes</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Drain data nodes controls whether to drain data notes on rolling restart operations</p>
</td>
</tr>
<tr>
<td>
<code>pluginsList</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>command</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>additionalVolumes</code><br/>
<em>
<a href="#opensearch.opster.io/v1.AdditionalVolume">
[]AdditionalVolume
</a>
</em>
</td>
<td>
<p>Additional volumes to mount to all pods in the cluster</p>
</td>
</tr>
<tr>
<td>
<code>monitoring</code><br/>
<em>
<a href="#opensearch.opster.io/v1.MonitoringConfig">
MonitoringConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>keystore</code><br/>
<em>
<a href="#opensearch.opster.io/v1.KeystoreValue">
[]KeystoreValue
</a>
</em>
</td>
<td>
<p>Populate opensearch keystore before startup</p>
</td>
</tr>
<tr>
<td>
<code>snapshotRepositories</code><br/>
<em>
<a href="#opensearch.opster.io/v1.SnapshotRepoConfig">
[]SnapshotRepoConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>podSecurityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#podsecuritycontext-v1-core">
Kubernetes core/v1.PodSecurityContext
</a>
</em>
</td>
<td>
<p>Set security context for the cluster pods</p>
</td>
</tr>
<tr>
<td>
<code>securityContext</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#securitycontext-v1-core">
Kubernetes core/v1.SecurityContext
</a>
</em>
</td>
<td>
<p>Set security context for the cluster pods&rsquo; container</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ISMTemplate">ISMTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpenSearchISMPolicySpec">OpenSearchISMPolicySpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>indexPatterns</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Index patterns on which this policy has to be applied</p>
</td>
</tr>
<tr>
<td>
<code>priority</code><br/>
<em>
int
</em>
</td>
<td>
<p>Priority of the template, defaults to 0</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ImageSpec">ImageSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.DashboardsConfig">DashboardsConfig</a>, <a href="#opensearch.opster.io/v1.GeneralConfig">GeneralConfig</a>, <a href="#opensearch.opster.io/v1.InitHelperConfig">InitHelperConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>image</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>imagePullPolicy</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#pullpolicy-v1-core">
Kubernetes core/v1.PullPolicy
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>imagePullSecrets</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
[]Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.IndexPermissionSpec">IndexPermissionSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchRoleSpec">OpensearchRoleSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>indexPatterns</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>dls</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>fls</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>allowedActions</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>maskedFields</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.IndexPriority">IndexPriority
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>priority</code><br/>
<em>
int64
</em>
</td>
<td>
<p>The priority for the index as soon as it enters a state.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.InitHelperConfig">InitHelperConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ImageSpec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ImageSpec">
ImageSpec
</a>
</em>
</td>
<td>
<p>
(Members of <code>ImageSpec</code> are embedded into this type.)
</p>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.KeystoreValue">KeystoreValue
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.GeneralConfig">GeneralConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Secret containing key value pairs</p>
</td>
</tr>
<tr>
<td>
<code>keyMappings</code><br/>
<em>
map[string]string
</em>
</td>
<td>
<p>Key mappings from secret to keystore keys</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.MessageTemplate">MessageTemplate
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ErrorNotification">ErrorNotification</a>, <a href="#opensearch.opster.io/v1.Notification">Notification</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>source</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.MonitoringConfig">MonitoringConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.GeneralConfig">GeneralConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enable</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>monitoringUserSecret</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>scrapeInterval</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>pluginUrl</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tlsConfig</code><br/>
<em>
<a href="#opensearch.opster.io/v1.MonitoringConfigTLS">
MonitoringConfigTLS
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.MonitoringConfigTLS">MonitoringConfigTLS
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.MonitoringConfig">MonitoringConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>serverName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>insecureSkipVerify</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.NodePool">NodePool
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>component</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>replicas</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>diskSize</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>resources</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#resourcerequirements-v1-core">
Kubernetes core/v1.ResourceRequirements
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>jvm</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>roles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tolerations</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#toleration-v1-core">
[]Kubernetes core/v1.Toleration
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>nodeSelector</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>affinity</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#affinity-v1-core">
Kubernetes core/v1.Affinity
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>topologySpreadConstraints</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#topologyspreadconstraint-v1-core">
[]Kubernetes core/v1.TopologySpreadConstraint
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>persistence</code><br/>
<em>
<a href="#opensearch.opster.io/v1.PersistenceConfig">
PersistenceConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>additionalConfig</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>labels</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>annotations</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>env</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#envvar-v1-core">
[]Kubernetes core/v1.EnvVar
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>priorityClassName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>pdb</code><br/>
<em>
<a href="#opensearch.opster.io/v1.PdbConfig">
PdbConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>probes</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ProbesConfig">
ProbesConfig
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Notification">Notification
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>destination</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>messageTemplate</code><br/>
<em>
<a href="#opensearch.opster.io/v1.MessageTemplate">
MessageTemplate
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Open">Open
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<h3 id="opensearch.opster.io/v1.OpenSearchCluster">OpenSearchCluster
</h3>
<div>
<p>Es is the Schema for the es API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ClusterSpec">
ClusterSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>general</code><br/>
<em>
<a href="#opensearch.opster.io/v1.GeneralConfig">
GeneralConfig
</a>
</em>
</td>
<td>
<p>INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
Important: Run &ldquo;make&rdquo; to regenerate code after modifying this file</p>
</td>
</tr>
<tr>
<td>
<code>confMgmt</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ConfMgmt">
ConfMgmt
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>bootstrap</code><br/>
<em>
<a href="#opensearch.opster.io/v1.BootstrapConfig">
BootstrapConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>dashboards</code><br/>
<em>
<a href="#opensearch.opster.io/v1.DashboardsConfig">
DashboardsConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>security</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Security">
Security
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>nodePools</code><br/>
<em>
<a href="#opensearch.opster.io/v1.NodePool">
[]NodePool
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>initHelper</code><br/>
<em>
<a href="#opensearch.opster.io/v1.InitHelperConfig">
InitHelperConfig
</a>
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ClusterStatus">
ClusterStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpenSearchHealth">OpenSearchHealth
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ClusterStatus">ClusterStatus</a>)
</p>
<div>
<p>OpenSearchHealth is the health of the cluster as returned by the health API.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;green&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;red&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;unknown&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;yellow&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpenSearchISMPolicy">OpenSearchISMPolicy
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpenSearchISMPolicySpec">
OpenSearchISMPolicySpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>defaultState</code><br/>
<em>
string
</em>
</td>
<td>
<p>The default starting state for each index that uses this policy.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<p>A human-readable description of the policy.</p>
</td>
</tr>
<tr>
<td>
<code>errorNotification</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ErrorNotification">
ErrorNotification
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ismTemplate</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ISMTemplate">
ISMTemplate
</a>
</em>
</td>
<td>
<p>Specify an ISM template pattern that matches the index to apply the policy.</p>
</td>
</tr>
<tr>
<td>
<code>policyId</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>states</code><br/>
<em>
<a href="#opensearch.opster.io/v1.State">
[]State
</a>
</em>
</td>
<td>
<p>The states that you define in the policy.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchISMPolicyStatus">
OpensearchISMPolicyStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpenSearchISMPolicySpec">OpenSearchISMPolicySpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpenSearchISMPolicy">OpenSearchISMPolicy</a>)
</p>
<div>
<p>ISMPolicySpec is the specification for the ISM policy for OS.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>defaultState</code><br/>
<em>
string
</em>
</td>
<td>
<p>The default starting state for each index that uses this policy.</p>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
<p>A human-readable description of the policy.</p>
</td>
</tr>
<tr>
<td>
<code>errorNotification</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ErrorNotification">
ErrorNotification
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ismTemplate</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ISMTemplate">
ISMTemplate
</a>
</em>
</td>
<td>
<p>Specify an ISM template pattern that matches the index to apply the policy.</p>
</td>
</tr>
<tr>
<td>
<code>policyId</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>states</code><br/>
<em>
<a href="#opensearch.opster.io/v1.State">
[]State
</a>
</em>
</td>
<td>
<p>The states that you define in the policy.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchActionGroup">OpensearchActionGroup
</h3>
<div>
<p>OpensearchActionGroup is the Schema for the opensearchactiongroups API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchActionGroupSpec">
OpensearchActionGroupSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>allowedActions</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchActionGroupStatus">
OpensearchActionGroupStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchActionGroupSpec">OpensearchActionGroupSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchActionGroup">OpensearchActionGroup</a>)
</p>
<div>
<p>OpensearchActionGroupSpec defines the desired state of OpensearchActionGroup</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>allowedActions</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchActionGroupState">OpensearchActionGroupState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchActionGroupStatus">OpensearchActionGroupStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;CREATED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ERROR&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;IGNORED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;PENDING&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchActionGroupStatus">OpensearchActionGroupStatus
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchActionGroup">OpensearchActionGroup</a>)
</p>
<div>
<p>OpensearchActionGroupStatus defines the observed state of OpensearchActionGroup</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>state</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchActionGroupState">
OpensearchActionGroupState
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>existingActionGroup</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>managedCluster</code><br/>
<em>
k8s.io/apimachinery/pkg/types.UID
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchClusterSelector">OpensearchClusterSelector
</h3>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>namespace</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchComponentTemplate">OpensearchComponentTemplate
</h3>
<div>
<p>OpensearchComponentTemplate is the schema for the OpenSearch component templates API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchComponentTemplateSpec">
OpensearchComponentTemplateSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the component template. Defaults to metadata.name</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchIndexSpec">
OpensearchIndexSpec
</a>
</em>
</td>
<td>
<p>The template that should be applied</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
int
</em>
</td>
<td>
<p>Version number used to manage the component template externally</p>
</td>
</tr>
<tr>
<td>
<code>allowAutoCreate</code><br/>
<em>
bool
</em>
</td>
<td>
<p>If true, then indices can be automatically created using this template</p>
</td>
</tr>
<tr>
<td>
<code>_meta</code><br/>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<p>Optional user metadata about the component template</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchComponentTemplateStatus">
OpensearchComponentTemplateStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchComponentTemplateSpec">OpensearchComponentTemplateSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchComponentTemplate">OpensearchComponentTemplate</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the component template. Defaults to metadata.name</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchIndexSpec">
OpensearchIndexSpec
</a>
</em>
</td>
<td>
<p>The template that should be applied</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
int
</em>
</td>
<td>
<p>Version number used to manage the component template externally</p>
</td>
</tr>
<tr>
<td>
<code>allowAutoCreate</code><br/>
<em>
bool
</em>
</td>
<td>
<p>If true, then indices can be automatically created using this template</p>
</td>
</tr>
<tr>
<td>
<code>_meta</code><br/>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<p>Optional user metadata about the component template</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchComponentTemplateState">OpensearchComponentTemplateState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchComponentTemplateStatus">OpensearchComponentTemplateStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;CREATED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ERROR&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;IGNORED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;PENDING&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchComponentTemplateStatus">OpensearchComponentTemplateStatus
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchComponentTemplate">OpensearchComponentTemplate</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>state</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchComponentTemplateState">
OpensearchComponentTemplateState
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>existingComponentTemplate</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>managedCluster</code><br/>
<em>
k8s.io/apimachinery/pkg/types.UID
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>componentTemplateName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the currently managed component template</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchISMPolicyState">OpensearchISMPolicyState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchISMPolicyStatus">OpensearchISMPolicyStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;CREATED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ERROR&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;IGNORED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;PENDING&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchISMPolicyStatus">OpensearchISMPolicyStatus
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpenSearchISMPolicy">OpenSearchISMPolicy</a>)
</p>
<div>
<p>OpensearchISMPolicyStatus defines the observed state of OpensearchISMPolicy</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>state</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchISMPolicyState">
OpensearchISMPolicyState
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>existingISMPolicy</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>managedCluster</code><br/>
<em>
k8s.io/apimachinery/pkg/types.UID
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>policyId</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchIndexAliasSpec">OpensearchIndexAliasSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchIndexSpec">OpensearchIndexSpec</a>)
</p>
<div>
<p>Describes the specs of an index alias</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>index</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the index that the alias points to.</p>
</td>
</tr>
<tr>
<td>
<code>alias</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the alias.</p>
</td>
</tr>
<tr>
<td>
<code>filter</code><br/>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<p>Query used to limit documents the alias can access.</p>
</td>
</tr>
<tr>
<td>
<code>routing</code><br/>
<em>
string
</em>
</td>
<td>
<p>Value used to route indexing and search operations to a specific shard.</p>
</td>
</tr>
<tr>
<td>
<code>isWriteIndex</code><br/>
<em>
bool
</em>
</td>
<td>
<p>If true, the index is the write index for the alias</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchIndexSpec">OpensearchIndexSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchComponentTemplateSpec">OpensearchComponentTemplateSpec</a>, <a href="#opensearch.opster.io/v1.OpensearchIndexTemplateSpec">OpensearchIndexTemplateSpec</a>)
</p>
<div>
<p>Describes the specs of an index</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>settings</code><br/>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<p>Configuration options for the index</p>
</td>
</tr>
<tr>
<td>
<code>mappings</code><br/>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<p>Mapping for fields in the index</p>
</td>
</tr>
<tr>
<td>
<code>aliases</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchIndexAliasSpec">
map[string]github.com/Opster/opensearch-k8s-operator/opensearch-operator/api/v1.OpensearchIndexAliasSpec
</a>
</em>
</td>
<td>
<p>Aliases to add</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchIndexTemplate">OpensearchIndexTemplate
</h3>
<div>
<p>OpensearchIndexTemplate is the schema for the OpenSearch index templates API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchIndexTemplateSpec">
OpensearchIndexTemplateSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the index template. Defaults to metadata.name</p>
</td>
</tr>
<tr>
<td>
<code>indexPatterns</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Array of wildcard expressions used to match the names of indices during creation</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchIndexSpec">
OpensearchIndexSpec
</a>
</em>
</td>
<td>
<p>The template that should be applied</p>
</td>
</tr>
<tr>
<td>
<code>composedOf</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>An ordered list of component template names. Component templates are merged in the order specified,
meaning that the last component template specified has the highest precedence</p>
</td>
</tr>
<tr>
<td>
<code>priority</code><br/>
<em>
int
</em>
</td>
<td>
<p>Priority to determine index template precedence when a new data stream or index is created.
The index template with the highest priority is chosen</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
int
</em>
</td>
<td>
<p>Version number used to manage the component template externally</p>
</td>
</tr>
<tr>
<td>
<code>_meta</code><br/>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<p>Optional user metadata about the index template</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchIndexTemplateStatus">
OpensearchIndexTemplateStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchIndexTemplateSpec">OpensearchIndexTemplateSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchIndexTemplate">OpensearchIndexTemplate</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the index template. Defaults to metadata.name</p>
</td>
</tr>
<tr>
<td>
<code>indexPatterns</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Array of wildcard expressions used to match the names of indices during creation</p>
</td>
</tr>
<tr>
<td>
<code>template</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchIndexSpec">
OpensearchIndexSpec
</a>
</em>
</td>
<td>
<p>The template that should be applied</p>
</td>
</tr>
<tr>
<td>
<code>composedOf</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>An ordered list of component template names. Component templates are merged in the order specified,
meaning that the last component template specified has the highest precedence</p>
</td>
</tr>
<tr>
<td>
<code>priority</code><br/>
<em>
int
</em>
</td>
<td>
<p>Priority to determine index template precedence when a new data stream or index is created.
The index template with the highest priority is chosen</p>
</td>
</tr>
<tr>
<td>
<code>version</code><br/>
<em>
int
</em>
</td>
<td>
<p>Version number used to manage the component template externally</p>
</td>
</tr>
<tr>
<td>
<code>_meta</code><br/>
<em>
k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1.JSON
</em>
</td>
<td>
<p>Optional user metadata about the index template</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchIndexTemplateState">OpensearchIndexTemplateState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchIndexTemplateStatus">OpensearchIndexTemplateStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;CREATED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ERROR&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;IGNORED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;PENDING&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchIndexTemplateStatus">OpensearchIndexTemplateStatus
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchIndexTemplate">OpensearchIndexTemplate</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>state</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchIndexTemplateState">
OpensearchIndexTemplateState
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>existingIndexTemplate</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>managedCluster</code><br/>
<em>
k8s.io/apimachinery/pkg/types.UID
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>indexTemplateName</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name of the currently managed index template</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchRole">OpensearchRole
</h3>
<div>
<p>OpensearchRole is the Schema for the opensearchroles API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchRoleSpec">
OpensearchRoleSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>clusterPermissions</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>indexPermissions</code><br/>
<em>
<a href="#opensearch.opster.io/v1.IndexPermissionSpec">
[]IndexPermissionSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tenantPermissions</code><br/>
<em>
<a href="#opensearch.opster.io/v1.TenantPermissionsSpec">
[]TenantPermissionsSpec
</a>
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchRoleStatus">
OpensearchRoleStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchRoleSpec">OpensearchRoleSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchRole">OpensearchRole</a>)
</p>
<div>
<p>OpensearchRoleSpec defines the desired state of OpensearchRole</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>clusterPermissions</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>indexPermissions</code><br/>
<em>
<a href="#opensearch.opster.io/v1.IndexPermissionSpec">
[]IndexPermissionSpec
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>tenantPermissions</code><br/>
<em>
<a href="#opensearch.opster.io/v1.TenantPermissionsSpec">
[]TenantPermissionsSpec
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchRoleState">OpensearchRoleState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchRoleStatus">OpensearchRoleStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;IGNORED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;CREATED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ERROR&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;PENDING&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchRoleStatus">OpensearchRoleStatus
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchRole">OpensearchRole</a>)
</p>
<div>
<p>OpensearchRoleStatus defines the observed state of OpensearchRole</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>state</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchRoleState">
OpensearchRoleState
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>existingRole</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>managedCluster</code><br/>
<em>
k8s.io/apimachinery/pkg/types.UID
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchTenant">OpensearchTenant
</h3>
<div>
<p>OpensearchTenant is the Schema for the opensearchtenants API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchTenantSpec">
OpensearchTenantSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchTenantStatus">
OpensearchTenantStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchTenantSpec">OpensearchTenantSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchTenant">OpensearchTenant</a>)
</p>
<div>
<p>OpensearchTenantSpec defines the desired state of OpensearchTenant</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>description</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchTenantState">OpensearchTenantState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchTenantStatus">OpensearchTenantStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;CREATED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ERROR&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;IGNORED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;PENDING&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchTenantStatus">OpensearchTenantStatus
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchTenant">OpensearchTenant</a>)
</p>
<div>
<p>OpensearchTenantStatus defines the observed state of OpensearchTenant</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>state</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchTenantState">
OpensearchTenantState
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>existingTenant</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>managedCluster</code><br/>
<em>
k8s.io/apimachinery/pkg/types.UID
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchUser">OpensearchUser
</h3>
<div>
<p>OpensearchUser is the Schema for the opensearchusers API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchUserSpec">
OpensearchUserSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>passwordFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretkeyselector-v1-core">
Kubernetes core/v1.SecretKeySelector
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>opendistroSecurityRoles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>backendRoles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>attributes</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchUserStatus">
OpensearchUserStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchUserRoleBinding">OpensearchUserRoleBinding
</h3>
<div>
<p>OpensearchUserRoleBinding is the Schema for the opensearchuserrolebindings API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchUserRoleBindingSpec">
OpensearchUserRoleBindingSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>roles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>users</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>backendRoles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchUserRoleBindingStatus">
OpensearchUserRoleBindingStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchUserRoleBindingSpec">OpensearchUserRoleBindingSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchUserRoleBinding">OpensearchUserRoleBinding</a>)
</p>
<div>
<p>OpensearchUserRoleBindingSpec defines the desired state of OpensearchUserRoleBinding</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>roles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>users</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>backendRoles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchUserRoleBindingState">OpensearchUserRoleBindingState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchUserRoleBindingStatus">OpensearchUserRoleBindingStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;PENDING&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;CREATED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ERROR&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchUserRoleBindingStatus">OpensearchUserRoleBindingStatus
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchUserRoleBinding">OpensearchUserRoleBinding</a>)
</p>
<div>
<p>OpensearchUserRoleBindingStatus defines the observed state of OpensearchUserRoleBinding</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>state</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchUserRoleBindingState">
OpensearchUserRoleBindingState
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>managedCluster</code><br/>
<em>
k8s.io/apimachinery/pkg/types.UID
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>provisionedRoles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>provisionedUsers</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>provisionedBackendRoles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchUserSpec">OpensearchUserSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchUser">OpensearchUser</a>)
</p>
<div>
<p>OpensearchUserSpec defines the desired state of OpensearchUser</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>opensearchCluster</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>passwordFrom</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#secretkeyselector-v1-core">
Kubernetes core/v1.SecretKeySelector
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>opendistroSecurityRoles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>backendRoles</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>attributes</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchUserState">OpensearchUserState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchUserStatus">OpensearchUserStatus</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;CREATED&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;ERROR&#34;</p></td>
<td></td>
</tr><tr><td><p>&#34;PENDING&#34;</p></td>
<td></td>
</tr></tbody>
</table>
<h3 id="opensearch.opster.io/v1.OpensearchUserStatus">OpensearchUserStatus
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchUser">OpensearchUser</a>)
</p>
<div>
<p>OpensearchUserStatus defines the observed state of OpensearchUser</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>state</code><br/>
<em>
<a href="#opensearch.opster.io/v1.OpensearchUserState">
OpensearchUserState
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>reason</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>managedCluster</code><br/>
<em>
k8s.io/apimachinery/pkg/types.UID
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.PVCSource">PVCSource
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.PersistenceSource">PersistenceSource</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>storageClass</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>accessModes</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#persistentvolumeaccessmode-v1-core">
[]Kubernetes core/v1.PersistentVolumeAccessMode
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.PdbConfig">PdbConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.NodePool">NodePool</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>enable</code><br/>
<em>
bool
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>minAvailable</code><br/>
<em>
k8s.io/apimachinery/pkg/util/intstr.IntOrString
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>maxUnavailable</code><br/>
<em>
k8s.io/apimachinery/pkg/util/intstr.IntOrString
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.PersistenceConfig">PersistenceConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.NodePool">NodePool</a>)
</p>
<div>
<p>PersistencConfig defines options for data persistence</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>,</code><br/>
<em>
<a href="#opensearch.opster.io/v1.PersistenceSource">
PersistenceSource
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.PersistenceSource">PersistenceSource
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.PersistenceConfig">PersistenceConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pvc</code><br/>
<em>
<a href="#opensearch.opster.io/v1.PVCSource">
PVCSource
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>emptyDir</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#emptydirvolumesource-v1-core">
Kubernetes core/v1.EmptyDirVolumeSource
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>hostPath</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#hostpathvolumesource-v1-core">
Kubernetes core/v1.HostPathVolumeSource
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ProbeConfig">ProbeConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ProbesConfig">ProbesConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>initialDelaySeconds</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>periodSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>timeoutSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>successThreshold</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>failureThreshold</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ProbesConfig">ProbesConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.NodePool">NodePool</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>liveness</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ProbeConfig">
ProbeConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>readiness</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ReadinessProbeConfig">
ReadinessProbeConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>startup</code><br/>
<em>
<a href="#opensearch.opster.io/v1.ProbeConfig">
ProbeConfig
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ReadinessProbeConfig">ReadinessProbeConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ProbesConfig">ProbesConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>initialDelaySeconds</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>periodSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>timeoutSeconds</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>failureThreshold</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.ReplicaCount">ReplicaCount
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>numberOfReplicas</code><br/>
<em>
int64
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Retry">Retry
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>backoff</code><br/>
<em>
string
</em>
</td>
<td>
<p>The backoff policy type to use when retrying.</p>
</td>
</tr>
<tr>
<td>
<code>count</code><br/>
<em>
int64
</em>
</td>
<td>
<p>The number of retry counts.</p>
</td>
</tr>
<tr>
<td>
<code>delay</code><br/>
<em>
string
</em>
</td>
<td>
<p>The time to wait between retries.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Rollover">Rollover
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>minDocCount</code><br/>
<em>
int64
</em>
</td>
<td>
<p>The minimum number of documents required to roll over the index.</p>
</td>
</tr>
<tr>
<td>
<code>minIndexAge</code><br/>
<em>
string
</em>
</td>
<td>
<p>The minimum age required to roll over the index.</p>
</td>
</tr>
<tr>
<td>
<code>minPrimaryShardSize</code><br/>
<em>
string
</em>
</td>
<td>
<p>The minimum storage size of a single primary shard required to roll over the index.</p>
</td>
</tr>
<tr>
<td>
<code>minSize</code><br/>
<em>
string
</em>
</td>
<td>
<p>The minimum size of the total primary shard storage (not counting replicas) required to roll over the index.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Rollup">Rollup
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<h3 id="opensearch.opster.io/v1.Security">Security
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.ClusterSpec">ClusterSpec</a>)
</p>
<div>
<p>Security defines options for managing the opensearch-security plugin</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>tls</code><br/>
<em>
<a href="#opensearch.opster.io/v1.TlsConfig">
TlsConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>config</code><br/>
<em>
<a href="#opensearch.opster.io/v1.SecurityConfig">
SecurityConfig
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.SecurityConfig">SecurityConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Security">Security</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>securityConfigSecret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Secret that contains the differnt yml files of the opensearch-security config (config.yml, internal_users.yml, &hellip;)</p>
</td>
</tr>
<tr>
<td>
<code>adminSecret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>TLS Secret that contains a client certificate (tls.key, tls.crt, ca.crt) with admin rights in the opensearch cluster. Must be set if transport certificates are provided by user and not generated</p>
</td>
</tr>
<tr>
<td>
<code>adminCredentialsSecret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Secret that contains fields username and password to be used by the operator to access the opensearch cluster for node draining. Must be set if custom securityconfig is provided.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Shrink">Shrink
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>forceUnsafe</code><br/>
<em>
bool
</em>
</td>
<td>
<p>If true, executes the shrink action even if there are no replicas.</p>
</td>
</tr>
<tr>
<td>
<code>maxShardSize</code><br/>
<em>
string
</em>
</td>
<td>
<p>The maximum size in bytes of a shard for the target index.</p>
</td>
</tr>
<tr>
<td>
<code>numNewShards</code><br/>
<em>
int
</em>
</td>
<td>
<p>The maximum number of primary shards in the shrunken index.</p>
</td>
</tr>
<tr>
<td>
<code>percentageOfSourceShards</code><br/>
<em>
int64
</em>
</td>
<td>
<p>Percentage of the number of original primary shards to shrink.</p>
</td>
</tr>
<tr>
<td>
<code>targetIndexNameTemplate</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the shrunken index.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Snapshot">Snapshot
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Action">Action</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>repository</code><br/>
<em>
string
</em>
</td>
<td>
<p>The repository name that you register through the native snapshot API operations.</p>
</td>
</tr>
<tr>
<td>
<code>snapshot</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the snapshot.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.SnapshotRepoConfig">SnapshotRepoConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.GeneralConfig">GeneralConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>settings</code><br/>
<em>
map[string]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.State">State
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpenSearchISMPolicySpec">OpenSearchISMPolicySpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>actions</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Action">
[]Action
</a>
</em>
</td>
<td>
<p>The actions to execute after entering a state.</p>
</td>
</tr>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the state.</p>
</td>
</tr>
<tr>
<td>
<code>transitions</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Transition">
[]Transition
</a>
</em>
</td>
<td>
<p>The next states and the conditions required to transition to those states. If no transitions exist, the policy assumes that it’s complete and can now stop managing the index</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.TenantPermissionsSpec">TenantPermissionsSpec
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.OpensearchRoleSpec">OpensearchRoleSpec</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>tenantPatterns</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>allowedActions</code><br/>
<em>
[]string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.TlsCertificateConfig">TlsCertificateConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.DashboardsTlsConfig">DashboardsTlsConfig</a>, <a href="#opensearch.opster.io/v1.TlsConfigHttp">TlsConfigHttp</a>, <a href="#opensearch.opster.io/v1.TlsConfigTransport">TlsConfigTransport</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Optional, name of a TLS secret that contains ca.crt, tls.key and tls.crt data. If ca.crt is in a different secret provide it via the caSecret field</p>
</td>
</tr>
<tr>
<td>
<code>caSecret</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.24/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>Optional, secret that contains the ca certificate as ca.crt. If this and generate=true is set the existing CA cert from that secret is used to generate the node certs. In this case must contain ca.crt and ca.key fields</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.TlsConfig">TlsConfig
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.Security">Security</a>)
</p>
<div>
<p>Configure tls usage for transport and http interface</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>transport</code><br/>
<em>
<a href="#opensearch.opster.io/v1.TlsConfigTransport">
TlsConfigTransport
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>http</code><br/>
<em>
<a href="#opensearch.opster.io/v1.TlsConfigHttp">
TlsConfigHttp
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.TlsConfigHttp">TlsConfigHttp
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.TlsConfig">TlsConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>generate</code><br/>
<em>
bool
</em>
</td>
<td>
<p>If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied</p>
</td>
</tr>
<tr>
<td>
<code>TlsCertificateConfig</code><br/>
<em>
<a href="#opensearch.opster.io/v1.TlsCertificateConfig">
TlsCertificateConfig
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.TlsConfigTransport">TlsConfigTransport
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.TlsConfig">TlsConfig</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>generate</code><br/>
<em>
bool
</em>
</td>
<td>
<p>If set to true the operator will generate a CA and certificates for the cluster to use, if false secrets with existing certificates must be supplied</p>
</td>
</tr>
<tr>
<td>
<code>perNode</code><br/>
<em>
bool
</em>
</td>
<td>
<p>Configure transport node certificate</p>
</td>
</tr>
<tr>
<td>
<code>TlsCertificateConfig</code><br/>
<em>
<a href="#opensearch.opster.io/v1.TlsCertificateConfig">
TlsCertificateConfig
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>nodesDn</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>Allowed Certificate DNs for nodes, only used when existing certificates are provided</p>
</td>
</tr>
<tr>
<td>
<code>adminDn</code><br/>
<em>
[]string
</em>
</td>
<td>
<p>DNs of certificates that should have admin access, mainly used for securityconfig updates via securityadmin.sh, only used when existing certificates are provided</p>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.TlsSecret">TlsSecret
</h3>
<div>
<p>Reference to a secret</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>secretName</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>key</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="opensearch.opster.io/v1.Transition">Transition
</h3>
<p>
(<em>Appears on:</em><a href="#opensearch.opster.io/v1.State">State</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>conditions</code><br/>
<em>
<a href="#opensearch.opster.io/v1.Condition">
Condition
</a>
</em>
</td>
<td>
<p>conditions for the transition.</p>
</td>
</tr>
<tr>
<td>
<code>stateName</code><br/>
<em>
string
</em>
</td>
<td>
<p>The name of the state to transition to if the conditions are met.</p>
</td>
</tr>
</tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
on git commit <code>c559878</code>.
</em></p>
