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
        <td><b>version</b></td>
        <td>string</td>
        <td> Opensearch applicative version  </td>
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
        <td>k8s service name</td>
        <td>false</td>
        <td>cluster name</td>
      </tr><tr>
        <td><b>SetVMMaxMapCount</b></td>
        <td>bool</td>
        <td>will add VMmaxMapCount</td>
        <td>false</td>
        <td></td>
      </tr><tr>
        <td><b>ExtraConfig</b></td>
        <td>string</td>
        <td>Added extra items to opensearch.yml</td>
        <td>string</td>
        <td></td>
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
      </tr><tr>
        <td><b>Tls</b></td>
        <td>DashboardsTlsConfig</td>
        <td>defining Dashbaord TLS configuration</td>
        <td>false</td>
        <td>false</td>
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
        <td>Default Opensearch resources</td>
      </tr><tr>
      </tr><tr>
        <td><b>roles</b></td>
        <td>[]string </td>
        <td> define Opensearch roles to the nodeGroup </td>
        <td>true</td>
        <td> - </td>
      </tr><tr>
      </tr><tr>
        <td><b>JVM</b></td>
        <td>string</td>
        <td> declare JVM</td>
        <td>false</td>
        <td> half of the STS requests  </td>
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
</table>