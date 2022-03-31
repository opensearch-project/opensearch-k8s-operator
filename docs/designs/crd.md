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
        <td>Responsible about enabling additional OpensaerchOperator components (reconcilers)  </td>
        <td>false</td>
      </tr><tr>
        <td><b>security</b></td>
        <td>object</td>
        <td>Defined security reconciler configuration</td>
        <td>false</td>
      </tr><tr>
        <td><b>nodePools</b></td>
        <td>[]object</td>
        <td>The nodePools object is build from nodePool objects and define different nodePools  in OpensearchCluster. Thats the main resource that responsible for deploying OpensearchCluster Statefulsets  </td>
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
        <td>Vendor declaration (sts docker image is built for it)</td>
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
        <td></td>
        <td>false</td>
        <td></td>
      </tr><tr>
        <td><b>ExtraConfig</b></td>
        <td>string</td>
        <td>Added extra items to opensearch.yml</td>
        <td>string</td>
        <td></td>
</table>



