# Operator Custom Resource Reference Guide

Custom resources are extensions of the Kubernetes API.

A resource is an endpoint in the Kubernetes API that stores a collection of API objects of a certain kind; for example, the built-in pods resource contains a collection of Pod objects.
A [Custom Resource](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/) is an extension of the Kubernetes API, many core Kubernetes functions are now built using custom resources, making Kubernetes more modular.
Cluster admins can update custom resources independently of the cluster itself. Once a custom resource is installed, users can create and access its objects using kubectl, just as they do for built-in resources like Pods.

The CustomResourceDefinition API resource allows you to define custom resources. Defining a CRD object creates a new custom resource with a name and schema that you specify. The Kubernetes API serves and handles the storage of your custom resource. Every resource is build from `KGV` that stands for Group Version Resource and this is what drives the Kubernetes API Server structure. 
The `OpensearchCLuster` CRD is representing an Opensearch cluster.


Our CRD is Defined by kind: `OpenSearchCluster`,group: `opensearch.opster.io` and version `v1`.
<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
      <td><b>apiVersion</b></td>
      <td>string</td>
      <td>opensearch.opster.io/v1</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>kind</b></td>
      <td>string</td>
      <td>OpenSearchCluster</td>
      <td>true</td>
      </tr>
      <tr>
      <td><b>metadata</b></td>
      <td>object</td>
      <td>Refer to the Kubernetes API documentation for the fields of the `metadata` field.</td>
      <td>true</td>
      </tr><tr>
        <td><b>spec</b></td>
        <td>object</td>
        <td>ClusterSpec defines the desired state of OpenSearchSpec</td>
        <td>true</td>
      </tr><tr>
        <td><b>status</b></td>
        <td>object</td>
        <td>OpensearchClusterStatus defines the observed state of ClusterStatus. include ComponentsStatus that saves and share necessary state of the operator components.  </td>
        <td>true</td>
      </tr></tbody>
</table>
<h3 id="OpensearchClusterSPec">
  OpensearchCluster.spec
</h3>



ClusterSpec defines the desired state of OpensearchCluster

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>general</b></td>
        <td>object</td>
        <td>Opensearch general configuration</td>
        <td>true</td>
      </tr><tr>
        <td><b>Bootstrap</b></td>
        <td>object</td>
        <td>Bootstrap pod configuration</td>
        <td>false</td>
      </tr><tr>
        <td><b>Dashboards</b></td>
        <td>object</td>
        <td>Opensearch-dashboards configuration</td>
        <td>false</td>
      </tr><tr>
        <td><b>confMgmt</b></td>
        <td>object</td>
        <td>Config object to enable additional OpensearchOperator features/components</td>
        <td>false</td>
      </tr><tr>
        <td><b>security</b></td>
        <td>object</td>
        <td>Defined security reconciler configuration</td>
        <td>false</td>
      </tr><tr>
        <td><b>nodePools</b></td>
        <td>[]object</td>
        <td>List of objects that define the different nodePools in an OpensearchCluster. Each nodePool represents a group of nodes with the same opensearch roles and resources. Each nodePool is deployed as a Kubernetes StatefulSet. Together they form the opensearch cluster.</td>
        <td>true</td>
      </tr><tr>
        <td><b>monitoring</b></td>
        <td>object</td>
        <td>monitoring configuration in an OpensearchCluster</td>
        <td>false</td>
      </tr><tr>
        <td><b>initHelper</b></td>
        <td>object</td>
        <td>InitHelper image configuration</td>
        <td>false</td>
      </tr>
</table>



<h3 id="GeneralConfig">
  GeneralConfig
</h3>

GeneralConfig defines global Opensearch cluster configuration 

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
            <th>default</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>httpPort</b></td>
        <td>int32</td>
        <td>http exposure port</td>
        <td>false</td>
        <td>9200</td>
      </tr><tr>
        <td><b>vendor</b></td>
        <td>string</td>
        <td>Vendor distribution to use for the cluster, currently only opensearch is supported</td>
        <td>false</td>
        <td>opensearch</td>
      </tr><tr>
        <td><b>command</b></td>
        <td>string</td>
        <td>Specify command in case you want to override the default command, useful if you have a custom image.</td>
        <td>false</td>
        <td>./opensearch-docker-entrypoint.sh</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>Version of opensearch to deploy</td>
        <td>false</td>
        <td>latest</td>
      </tr><tr>
        <td><b>ServiceAccount</b></td>
        <td>string</td>
        <td>k8s service account name</td>
        <td>false</td>
        <td>cluster name</td>
      </tr><tr>
        <td><b>ServiceName</b></td>
        <td>string</td>
        <td>Name to use for the k8s service to expose the cluster internally</td>
        <td>false</td>
        <td>cluster name</td>
      </tr><tr>
        <td><b>SetVMMaxMapCount</b></td>
        <td>bool</td>
        <td>will add VMmaxMapCount</td>
        <td>false</td>
        <td></td>
      </tr><tr>
        <td><b>additionalConfig</b></td>
        <td>string</td>
        <td>Added extra items to opensearch.yml</td>
        <td>string</td>
        <td></td>
      </tr><tr>
        <td><b>labels</b></td>
        <td>map[string]string</td>
        <td>add user defined labels to nodePool</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
        <td><b>env</b></td>
        <td>[]corev1.Env</td>
        <td>add user defined environment variables to nodePool</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
        <td><b>DefaultRepo</b></td>
        <td>string</td>
        <td>Default image repository to use</td>
      </tr><tr>
        <td><b>keystore</b></td>
        <td>[]opsterv1.KeystoreValue</td>
        <td>List of objects that define secret values that will populate the opensearch keystore.</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
        <td><b>pluginsList</b></td>
        <td>[]string</td>
        <td>List of plugins that should be installed for OpenSearch at startup.</td>
        <td>false</td>
        <td> [] </td>
      </tr><tr>
        <td><b>podSecurityContext</b></td>
        <td>*corev1.PodSecurityContext</td>
        <td>Set the security context for the cluster pods.</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
        <td><b>securityContext</b></td>
        <td>*corev1.SecurityContext</td>
        <td>Set the security context for the cluster pods' containers.</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
        <td><b>snapshotRepositories</b></td>
        <td>[]SnapshotRepoConfig</td>
        <td>Snapshot Repo settings</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
        <td><b>additionalVolumes</b></td>
        <td>[]object</td>
        <td>List of additional volume mounts</td>
        <td>false</td>
        <td>-</td>
      </tr>
</table>

<h3 id="GeneralConfig">
  Bootstrap
</h3>

Bootstrap defines Opensearch bootstrap pod configuration

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
            <th>default</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>resources</b></td>
        <td>corev1.ResourceRequirements</td>
        <td>Define Opensearch bootstrap pod resources</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
        <td><b>tolerations</b></td>
        <td>[]corev1.Toleration</td>
        <td>add toleration to bootstrap pod</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
        <td><b>nodeSelector</b></td>
        <td>map[string]string</td>
        <td>Add NodeSelector to bootstrap pod</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
        <td><b>affinity</b></td>
        <td>corev1.Affinity</td>
        <td>add affinity to bootstrap pod</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
        <td><b>jvm</b></td>
        <td>string</td>
        <td>JVM args. Use this to define heap size</td>
        <td>false</td>
        <td>-Xmx512M -Xms512M<td>
      </tr><tr>
        <td><b>additionalConfig</b></td>
        <td>string</td>
        <td>Added extra items to opensearch.yml in the bootstrap pod</td>
        <td>map[string]string</td>
        <td>general.additionalConfig</td>
      </tr>
</table>

<h3 id="GeneralConfig">
  Dashboards
</h3>

Dashboards defines Opensearch-Dashboard configuration and deployment

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
            <th>default</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>enable</b></td>
        <td>bool</td>
        <td>if true, will deploy Opensearch-dashboards with the cluster</td>
        <td>false</td>
        <td>false</td>
      </tr><tr>
        <td><b>replicas</b></td>
        <td>int</td>
        <td>defines Opensearch-Dashboards deployment's replicas</td>
        <td>true</td>
        <td>1</td>
      </tr><tr>
        <td><b>basePath</b></td>
        <td>string</td>
        <td>Defines the base path of opensearch dashboards (e.g. when using a reverse proxy)</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
        <td><b>resources</b></td>
        <td>corev1.ResourceRequirements</td>
        <td> Define Opensearch-Dashboard resources </td>
        <td>false</td>
        <td>Default Opensearch-dashboard resources</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>Opensearch-dashboards version</td>
        <td>false</td>
        <td>latest</td>
      </tr><tr>
        <td><b>Tls</b></td>
        <td>DashboardsTlsConfig</td>
        <td>defining Dashbaord TLS configuration</td>
        <td>false</td>
        <td>false</td>
      </tr><tr>
      </tr><tr>
        <td><b>env</b></td>
        <td>[]corev1.Env</td>
        <td>add user defined environment variables to dashboard app</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
      </tr><tr>
        <td><b>image</b></td>
        <td>string</td>
        <td>Define Opensearch-dashboards image</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
      </tr><tr>
        <td><b>imagePullPolicy</b></td>
        <td>corev1.PullPolicy</td>
        <td>Define Opensearch-dashboards image pull policy</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
      </tr><tr>
        <td><b>imagePullSecrets</b></td>
        <td>corev1.LocalObjectReference</td>
        <td>Define Opensearch-dashboards image pull secrets</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
      <td><b>tolerations</b></td>
        <td>[]corev1.Toleration</td>
        <td>Adds toleration to dashboard pods</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
        <td><b>nodeSelector</b></td>
        <td>map[string]string</td>
        <td>Adds NodeSelector to dashboard pods</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
        <td><b>affinity</b></td>
        <td>corev1.Affinity</td>
        <td>Adds affinity to dashboard pods</td>
        <td>false</td>
        <td>-</td>
      </tr>
      </tr><tr>
        <td><b>labels</b></td>
        <td>map[string]string</td>
        <td>Adds labels to dashboard pods</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
      </tr><tr>
        <td><b>annotations</b></td>
        <td>map[string]string</td>
        <td>Adds annotations to dashboard pods</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
        <td><b>service</b></td>
        <td>opsterv1.DashboardsService</td>
        <td>Customize dashboard service</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
        <td><b>pluginsList</b></td>
        <td>[]string</td>
        <td>List of plugins that should be installed for OpenSearch Dashboards at startup.</td>
        <td>false</td>
        <td> [] </td>
      </tr><tr>
        <td><b>podSecurityContext</b></td>
        <td>*corev1.PodSecurityContext</td>
        <td>Set the security context for the dashboards pods.</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
        <td><b>securityContext</b></td>
        <td>*corev1.SecurityContext</td>
        <td>Set the security context for the dashboards pods' containers.</td>
        <td>false</td>
        <td> - </td>
      </tr>
    </tr><tr>
</table>


<h3 id="GeneralConfig">
  NodePools
</h3>

Every NodePool is defining different Opensearch Nodes StatefulSet 

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
            <th>default</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>component</b></td>
        <td>string</td>
        <td>statefulset name - will create $cluster-name-$component STS </td>
        <td>true</td>
        <td>-</td>
      </tr><tr>
        <td><b>replicas</b></td>
        <td>int</td>
        <td>defines NodePool deployment's replicas</td>
        <td>true</td>
        <td>1</td>
      </tr><tr>
        <td><b>diskSize</b></td>
        <td>string</td>
        <td> nodePool data disk size </td>
        <td>true</td>
        <td> - </td>
      </tr><tr>
        <td><b>NodeSelector</b></td>
        <td>map[string]string</td>
        <td>add NodeSelector to nodePool</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
      </tr><tr>
        <td><b>Tls</b></td>
        <td>DashboardsTlsConfig</td>
        <td>defining Dashbaord TLS configuration</td>
        <td>false</td>
        <td>false</td>
      </tr><tr>
      </tr><tr>
        <td><b>resources</b></td>
        <td>corev1.ResourceRequirements</td>
        <td> Define NodePool resources </td>
        <td>false</td>
        <td></td>
      </tr><tr>
      </tr><tr>
        <td><b>roles</b></td>
        <td>[]string </td>
        <td>List of OpenSearch roles to assign to the nodePool</td>
        <td>true</td>
        <td> - </td>
      </tr><tr>
      </tr><tr>
        <td><b>JVM</b></td>
        <td>string</td>
        <td>JVM args. Use this to define heap size (recommendation: Set to half of memory request)</td>
        <td>false</td>
        <td>Half of `resources.requests.memory` if jvm is not set. Fallback value is `-Xmx512M -Xms512M` if neither `resources.requests.memory` nor jvm are set.</td>
      </tr><tr>
      </tr><tr>
        <td><b>Affinity</b></td>
        <td>corev1.Affinity</td>
        <td>add affinity to nodePool</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
      </tr><tr>
        <td><b>Tolerations</b></td>
        <td>[]corev1.Toleration</td>
        <td>add toleration to nodePool</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
      </tr><tr>
        <td><b>topologySpreadConstraints</b></td>
        <td>[]corev1.TopologySpreadConstraint</td>
        <td>add topology spread contraints to nodePool</td>
        <td>false</td>
        <td> - </td>
      </tr>
      </tr><tr>
        <td><b>annotations</b></td>
        <td>map[string]string</td>
        <td>Adds annotations to node pods</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
      </tr><tr>
        <td><b>priorityClassName</b></td>
        <td>string</td>
        <td>Adds a priority class to nodes</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
</table>

<h3 id="InitHelperConfig">
  InitHelperConfig
</h3>

InitHelperConfig defines global Opensearch InitHelper image configuration

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
            <th>default</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>image</b></td>
        <td>string</td>
        <td>Define InitHelper image</td>
        <td>false</td>
        <td>public.ecr.aws/opsterio/busybox</td>
      </tr><tr>
      </tr><tr>
        <td><b>imagePullPolicy</b></td>
        <td>corev1.PullPolicy</td>
        <td>Define InitHelper image pull policy</td>
        <td>false</td>
        <td> - </td>
      </tr><tr>
        <td><b>resources</b></td>
        <td>corev1.ResourceRequirements</td>
        <td>Define initcontainer resorces</td>
        <td>false</td>
        <td>-</td>
      </tr><tr>
        <td><b>version</b></td>
        <td>string</td>
        <td>Version of InitHelper (busybox) image to deploy</td>
        <td>false</td>
        <td>1.27.2-buildx</td>       
      </tr> 
</table>

<h3 id="GeneralConfig">
  Monitoring
</h3>

Monitoring defines Opensearch monitoring configuration

<table>
    <thead>
        <tr>
            <th>Name</th>
            <th>Type</th>
            <th>Description</th>
            <th>Required</th>
            <th>default</th>
        </tr>
    </thead>
    <tbody><tr>
        <td><b>enable</b></td>
        <td>bool</td>
        <td>Define if to enable monitoring for that cluster</td>
        <td>true</td>
        <td>-</td>
      </tr><tr>
        <td><b>monitoringUserSecret</b></td>
        <td>[]string</td>
        <td>Define from which user the monitor will run (Getting Secret name, the secret should contain 'username':'password' fileds).</td>
        <td>false</td>
        <td>admin</td>
      </tr><tr>
        <td><b>scrapeInterval</b></td>
        <td>string</td>
        <td>Define interval for scraping</td>
        <td>false</td>
        <td>30s</td>
      </tr><tr>
      </tr><tr>
        <td><b>pluginURL</b></td>
        <td>string</td>
        <td>Define offline link to Aiven Plugin</td>
        <td>false</td>
        <td>https://github.com/aiven/prometheus-exporter-plugin-for-opensearch/releases/download/<YOUR_CLUSTER_VERSION>/prometheus-exporter-<YOUR_CLUSTER_VERSION>.zip/</td>
      </tr><tr>
      </tr><tr>
        <td><b>tlsConfig</b></td>
        <td>map[]</td>
        <td>Tls Configuration <b>See <i>tlsConfig</i> below</b></td>
        <td>false</td>
        <td> - </td>
     </tr><tr>
</table>

<h3 id="GeneralConfig">
  Monitoring.tlsConfig
</h3>

Monitoring TLS configuration options

<table>
  <thead>
      <tr>
          <th>Name</th>
          <th>Type</th>
          <th>Description</th>
          <th>Required</th>
          <th>default</th>
      </tr>
  </thead>
  <tbody><tr>
    <td><b>serverName</b></td>
    <td>string</td>
    <td>Used to verify the hostname for the targets</td>
    <td>false</td>
    <td></td>
    </tr><tr>
    <td><b>insecureSkipVerify</b></td>
    <td>bool</td>
    <td>Disable target certificate validation</td>
    <td>false</td>
    <td>false</td>
    </tr><tr>
  </tbody>
</table>

<h3 id="GeneralConfig">
  Keystore
</h3>

Every Keystore Value defines a secret to pull secrets from. 
<table>
    <tbody>
      <tr>
        <td><b>secret</b></td>
        <td>corev1.LocalObjectReference</td>
        <td>Define secret that contains key value pairs</td>
        <td>true</td>
        <td>-</td>
      </tr>
      <tr>
        <td><b>keyMappings</b></td>
        <td>map</td>
        <td>Define key mappings from secret to keystore entry. Example: "old: new" creates a keystore entry "new" with the value from the secret entry "old". When a map is provided, only the specified keys are loaded from the secret, so use "key: key" to load a key that should not be renamed.</td>
        <td>false</td>
        <td>-</td>
      </tr>
    </tbody>
</table>

<h3 id="GeneralConfig">
  AdditionalVolume
</h3>

AdditionalVolume object define additional volume and volumeMount
<table>
  <tbody>
    <tr>
      <td><b>name</b></td>
      <td>string</td>
      <td>Defines name for additional volume</td>
      <td>true</td>
      <td>-</td>
    </tr><tr>
      <td><b>path</b></td>
      <td>string</td>
      <td>Defines mount path for additional volume</td>
      <td>true</td>
      <td>-</td>
    </tr><tr>
      <td><b>restartPods</b></td>
      <td>bool</td>
      <td>Defines if pod should restar or not in case of change in VolumeSource object</td>
      <td>false</td>
      <td>false</td>
    </tr><tr>
      <td><b>emptyDir</b></td>
      <td>corev1.EmptyDirVolumeSource</td>
      <td>Defines emptyDir object to be mouted</td>
      <td>false</td>
      <td>-</td>
    </tr><tr>
      <td><b>configMap</b></td>
      <td>corev1.ConfigMapVolumeSource</td>
      <td>Defines ConfgMap object to be mounted</td>
      <td>false</td>
      <td>-</td>
    </tr><tr>
      <td><b>secret</b></td>
      <td>corev1.SecretVolumeSource</td>
      <td>Defines Secret object to be mounted</td>
      <td>false</td>
      <td>-</td>
    </tr>
  </tbody>
</table>

