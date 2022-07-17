# Opensearch Operator User Guide

This guide is intended for users of the Opensearch Operator. If you want to contribute to the development of the Operator, please see the [Design documents](../designs/high-level.md) instead.

## Installation

The Operator can be easily installed using Helm:

1. Add the helm repo: `helm repo add opensearch-operator https://opster.github.io/opensearch-k8s-operator/`
2. Install the Operator: `helm install opensearch-operator opensearch-operator/opensearch-operator`

Follow the instructions in this video to install the Operator:

[![Watch the video](https://opster.com/wp-content/uploads/2022/05/Operator-Installation-Tutorial.png)](https://player.vimeo.com/video/708641527)

## Quickstart

After you have successfully installed the Operator, you can deploy your first OpenSearch cluster. This is done by creating an `OpenSearchCluster` custom object in Kubernetes.

Create a file `cluster.yaml` with the following content:

```yaml
apiVersion: opensearch.opster.io/v1
kind: OpenSearchCluster
metadata:
  name: my-first-cluster
  namespace: default
spec:
  general:
    serviceName: my-first-cluster
    version: 1.3.1
  dashboards:
    enable: true
    version: 1.3.1
    replicas: 1
    resources:
      requests:
         memory: "512Mi"
         cpu: "200m"
      limits:
         memory: "512Mi"
         cpu: "200m"
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "5Gi"
      NodeSelector:
      resources:
         requests:
            memory: "2Gi"
            cpu: "500m"
         limits:
            memory: "2Gi"
            cpu: "500m"
      roles:
        - "data"
        - "master"

```

Then run `kubectl apply -f cluster.yaml`. If you watch the cluster (e.g. `watch -n 2 kubectl get pods`), you will see that after a few seconds the Operator will create several pods. First, a bootstrap pod will be created (`my-first-cluster-bootstrap-0`) that helps with initial master discovery. Then three pods for the OpenSearch cluster will be created (`my-first-cluster-masters-0/1/2`), and one pod for the dashboards instance. After the pods are appearing as ready, which normally takes about 1-2 minutes, you can connect to your cluster using port-forwarding.

Run `kubectl port-forward svc/my-first-cluster-dashboards 5601`, then open [http://localhost:5601](http://localhost:5601) in your browser and log in with the default demo credentials `admin / admin`.
Alternatively, if you want to access the OpenSearch REST API, run: `kubectl port-forward svc/my-first-cluster 9200`. Then open a second terminal and run: `curl -k -u admin:admin https://localhost:9200/_cat/nodes?v`. You should see the three deployed pods listed.

If you'd like to delete your cluster, run: `kubectl delete -f cluster.yaml`. The Operator will then clean up and delete any Kubernetes resources created for the cluster. Note that this will not delete the persistent volumes for the cluster, in most cases. For a complete cleanup, run: `kubectl delete pvc -l opster.io/opensearch-cluster=my-first-cluster` to also delete the PVCs.

The minimal cluster you deployed in this section is only intended for demo purposes. Please see the next sections on how to configure the different aspects of your cluster.

## Data Persistence

By default, the Operator will create OpenSearch node pools with persistent storage from the default [Storage Class](https://kubernetes.io/docs/concepts/storage/storage-classes/).  This behaviour can be changed per node pool. You may supply an alternative storage class and access mode, or configure hostPath or emptyDir storage. Please note that hostPath is strongly discouraged, and if you do choose this option, then you must also configure affinity for the node pool to ensure that multiple pods do not schedule to the same Kubernetes host:

### PVC
Default option is persistent storage, to explicitly add pvc to custom `storageClass`.
```yaml
nodePools:
- component: masters
  replicas: 3
  diskSize: 30
  roles:
    - "data"
    - "master"
  persistence:
    pvc:
      storageClass: mystorageclass
      accessModes:
      - ReadWriteOnce
```
### EmptyDir
Persistent source as `emptyDir`.

```yaml
nodePools:
- component: masters
  replicas: 3
  diskSize: 30
  roles:
    - "data"
    - "master"
  persistence:
    emptyDir: {}
```
If you are using emptyDir, it is recommended that you set `spec.general.drainDataNodes` to be `true`. This will ensure that shards are drained from the pods before rolling upgrades or restart operations are performed.

### HostPath
Persistent source as `hostPath`.

```yaml
nodePools:
- component: masters
  replicas: 3
  diskSize: 30
  roles:
    - "data"
    - "master"
  persistence:
    hostPath:
      path: "/var/opensearch"
```

## Configuring opensearch.yml

The Operator automatically generates the main OpenSearch configuration file `opensearch.yml` based on the parameters you provide in the different sections (e.g. TLS configuration). If you need to add your own settings, you can do that using the `additionalConfig` field in the custom resource:

```yaml
spec:
  general:
    # ...
    additionalConfig:
      some.config.option: somevalue
  # ...
nodePools:
- component: masters
  # ...
  additionalConfig:
    some.other.config: foobar
```

Using `spec.general.additionalConfig` you can add settings to all nodes, using `nodePools[].additionalConfig` you can add settings to only a pool of nodes. The settings must be provided as a map of strings, so use the flat form of any setting. The Operator merges its own generated settings with whatever extra settings you provide. Note that basic settings like `node.name`, `node.roles`, `cluster.name` and settings related to network and discovery are set by the Operator and cannot be overwritten using `additionalConfig`.

As of right now, the settings cannot be changed after the initial installation of the cluster (that feature is planned for the next version). If you need to change any settings please use the [Cluster Settings API](https://opensearch.org/docs/latest/opensearch/configuration/#update-cluster-settings-using-the-api) to change them at runtime.

## Configuring opensearch_dashboards.yml

You can customize the OpenSearch dashboard configuration file [`opensearch_dashboards.yml`](https://github.com/opensearch-project/OpenSearch-Dashboards/blob/main/config/opensearch_dashboards.yml) using the `additionalConfig` field in the dashboards section of the `OpenSearchCluster` custom resource:

```yaml
apiVersion: opensearch.opster.io/v1
kind: OpenSearchCluster
...
spec:
  dashboards:
    additionalConfig:
      opensearch_security.auth.type: "proxy"
      opensearch.requestHeadersWhitelist: |
        ["securitytenant","Authorization","x-forwarded-for","x-auth-request-access-token", "x-auth-request-email", "x-auth-request-groups"]
      opensearch_security.multitenancy.enabled: "true"
```

This allows one to set up any of the [backend](https://opensearch.org/docs/latest/security-plugin/configuration/configuration/) authentication types for the dashboard.

*The configuration must be valid or the dashboard will fail to start.*

## TLS

For security reasons, encryption is required for communication with the OpenSearch cluster and between cluster nodes. If you do not configure any encryption, OpenSearch will use the included demo TLS certificates, which are not ideal for most active deployments.

Depending on your requirements, the Operator offers two ways of managing TLS certificates. You can either supply your own certificates, or the Operator will generate its own CA and sign certificates for all nodes using that CA. The second option is recommended, unless you want to directly expose your OpenSearch cluster outside your Kubernetes cluster, or your organization has rules about using self-signed certificates for internal communication.

TLS certificates are used in three places, and each can be configured independently.

### Node Transport

OpenSearch cluster nodes communicate with each other using the OpenSearch transport protocol (port 9300 by default). This is not exposed externally, so in almost all cases, generated certificates should be adequate.

To configure node transport security you can use the following fields in the `OpenSearchCluster` custom resource:

```yaml
# ...
spec:
  security:
    tls:  # Everything related to TLS configuration
      transport:  # Configuration of the transport endpoint
        generate: true  # Have the operator generate and sign certificates
        perNode: true  # Separate certificate per node
        secret:
          name:  # Name of the secret that contains the provided certificate
        caSecret:
          name:  # Name of the secret that contains a CA the operator should use
        nodesDn: []  # List of certificate DNs allowed to connect
        adminDn: []  # List of certificate DNs that should get admin access
# ...
```

To have the Operator generate the certificates, you only need to set the `generate` and `perNode` fields to `true` (all other fields can be omitted). The Operator will then generate a CA certificate and one certificate per node, and then use the CA to sign the node certificates. These certificates are valid for one year. Note that the Operator does not currently have certificate renewal implemented. 

Alternatively, you can provide the certificates yourself (e.g. if your organization has an internal CA). You can either provide one certificate to be used by all nodes or provide a certificate for each node (recommended). In this mode, set `generate: false` and `perNode` to `true` or `false` depending on whether you're providing per-node certificates. 

If you provide just one certificate, it must be placed in a Kubernetes TLS secret (with the fields `ca.crt`, `tls.key` and `tls.crt`, must all be PEM-encoded), and you must provide the name of the secret as `secret.name`. If you want to keep the CA certificate separate, you can place it in a separate secret and supply that as `caSecret.name`. If you provide one certificate per node, you must place all certificates into one secret (including the `ca.crt`) with a `<hostname>.key` and `<hostname>.crt` for each node. The hostname is defined as `<cluster-name>-<nodepool-component>-<index>` (e.g. `my-first-cluster-masters-0`).

If you provide the certificates yourself, you must also provide the list of certificate DNs in `nodesDn`, wildcards can be used (e.g. `"CN=my-first-cluster-*,OU=my-org"`). The `adminDn` field is only needed if you also supply your own securityconfig (see below). The Operator will then use your CA certificate to sign the node certificates.

### Node HTTP/REST API

Each OpenSearch cluster node exposes the REST API using HTTPS (by default port 9200).

To configure HTTP API security, the following fields in the `OpenSearchCluster` custom resource are available:

```yaml
# ...
spec:
  security:
    tls:  # Everything related to TLS configuration
      http:  # Configuration of the HTTP endpoint
        generate: true  # Have the Operator generate and sign certificates
        secret:
          name:  # Name of the secret that contains the provided certificate
        caSecret:
          name:  # Name of the secret that contains a CA the Operator should use
# ...
```

Again, you have the option of either letting the Operator generate and sign the certificates or providing your own. The only difference between node transport certificates and node HTTP/REST APIs is that per-node certificate are not possible here. In all other respects the two work the same way. 

If you provide your own certificates, please make sure the following names are added as SubjectAltNames (SAN): `<cluster-name>`, `<cluster-name>.<namespace>`, `<cluster-name>.<namespace>.svc`,`<cluster-name>.<namespace>.svc.cluster.local`.

Directly exposing the node HTTP port outside the Kubernetes cluster is not recommended. Rather than doing so, you should configure an ingress. The ingress can then also present a certificate from an accredited CA (for example LetsEncrypt) and hide self-signed certificates that are being used internally. In this way, the nodes should be supplied internally with properly signed certificates.

### Dashboards HTTP

OpenSearch Dashboards can expose its API/UI via HTTP or HTTPS. It is unencrypted by default. As mentioned above, to secure the connection you can either let the Operator generate and sign a certificate, or provide your own. The following fields in the `OpenSearchCluster` custom resource are available to configure it:

```yaml
# ...
spec:
  dashboards:
    enable: true  # Deploy Dashboards component
    tls:
      enable: true  # Configure TLS
      generate: true  # Have the Operator generate and sign a certificate
      secret:
        name:  # Name of the secret that contains the provided certificate
      caSecret:
       name:  # Name of the secret that contains a CA the Operator should use
# ...
```

To let the Operator generate the certificate, just set `tls.enable: true` and `tls.generate: true` (the other fields under `tls` can be ommitted). Again, as with the node certificates, you can supply your own CA via `caSecret.name` for the Operator to use.
If you want to use your own certificate, you need to provide it as a Kubernetes TLS secret (with fields `tls.key` and `tls.crt`) and provide the name as `secret.name`.

If you want to expose Dashboards outside of the cluster, it is recommended to use Operator-generated certificates internally and let an Ingress present a valid certificate from an accredited CA.

## Securityconfig

By default, Opensearch clusters use the opensearch-security plugin to handle authentication and authorization. If nothing is specifically configured, clusters deployed using the Operator use the demo securityconfig provided by the OpenSearch project (see [internal_users.yml](https://github.com/opensearch-project/security/blob/main/securityconfig/internal_users.yml) for a list of users).

You can provide your own securityconfig (see the entire [demo securityconfig](https://github.com/opensearch-project/security/blob/main/securityconfig) as an example and the [Access control documentation](https://opensearch.org/docs/latest/security-plugin/access-control/index/) of the OpenSearch project) with your own users and roles. To do that, you must provide a secret with all the required securityconfig yaml files.

The Operator can be controlled using the following fields in the `OpenSearchCluster` custom resource:

```yaml
# ...
spec:
  security:
    config:  # Everything related to the securityconfig
      securityConfigSecret:
        name:  # Name of the secret that contains the securityconfig files
      adminSecret:
        name:  # Name of a secret that contains the admin client certificate
      adminCredentialsSecret:
        name:  # Name of a secret that contains username/password for admin access
# ...
```

Provide the name of the secret that contains your securityconfig yaml files as `securityconfigSecret.name`. Note that all files must be provided, you cannot provide only some of them, as the demo files and your provided ones cannot be merged. In addition, you must provide the name of a secret as `adminCredentialsSecret.name` that has fields `username` and `password` for a user that the Operator can use for communicating with OpenSearch (currently used for getting the cluster status, doing health checks and coordinating node draining during cluster scaling operations).

If you provided your own certificate for node transport communication, then you must also provide an admin client certificate (as a Kubernetes TLS secret with fields `ca.crt`, `tls.key` and `tls.crt`) as `adminSecret.name`. The DN of the certificate must be listed under `security.tls.transport.adminDn`. Be advised that the `adminDn` and `nodesDn` must be defined in a way that the admin certficate cannot be used or recognized as a node certficiate, otherwise OpenSearch will reject any authentication request using the admin certificate.

To apply the securityconfig to the OpenSearch cluster, the Operator uses a separate Kubernetes job (called `<cluster-name>-securityconfig-update`). This job is run during the initial provisioning of the cluster. The Operator also monitors the secret with the securityconfig for any changes and then reruns the update job to apply the new config. Note that the Operator only checks for changes in certain intervals, so it might take a minute or two for the changes to be applied. If the changes are not applied after a few minutes, please use 'kubectl' to check the logs of the pod of the `<cluster-name>-securityconfig-update` job. If you have an error in your configuration it will be reported there.


## Add plugins 
In order to use some OpenSearch features (snapshot,monitoring,etc...) you will have to install OpenSearch plugins.
To install those plugins, all you have to do is to declaer them under PluginsList in general section:
For example you can install official OpenSearch plugins :
* ֻopensearch-alerting                  
* opensearch-anomaly-detection         
* opensearch-asynchronous-search       
* opensearch-cross-cluster-replication
* opensearch-index-management          
* opensearch-job-scheduler             
* opensearch-knn                      
* opensearch-ml                        
* opensearch-notifications             
* opensearch-observability             
* opensearch-performance-analyzer     
* opensearch-reports-scheduler         
* opensearch-security                  
* opensearch-sql

Or custom ones, for example that Aiven plugin for prometheus-exporter:
* https://github.com/aiven/prometheus-exporter-plugin-for-opensearch/releases/download/1.3.0.0/prometheus-exporter-1.3.0.0.zip

```yaml
  general:
    version: 1.3.0
    httpPort: 9200
    vendor: opensearch
    serviceName: my-cluster
    pluginsList: ["repository-s3","https://github.com/aiven/prometheus-exporter-plugin-for-opensearch/releases/download/1.3.0.0/prometheus-exporter-1.3.0.0.zip"]
```

## Nodepools and Scaling
OpenSearch clusters can be composed of one or more node pools, with each representing a logical group or unified roles. Each node pool can have its own resources, and will have autonomic StatefulSets and services.

```yaml
spec:
    nodePools:
      - component: masters
        replicas: 3
        diskSize: "30Gi"
        NodeSelector:
        resources:
          requests:
            memory: "2Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "500m"
        roles:
          - "master"
          - "data"
      - component: nodes
        replicas: 3
        diskSize: "10Gi"
        NodeSelector:
        resources:
          requests:
            memory: "2Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "500m"
        roles:
          - "data"
```

## Volume Expansion

To increase the disk volume size set  the`diskSize` to desired value and re-apply the cluster yaml. This operation is expected to have no downtime and the cluster should be operational.

The following considerations should be taken into account in order to increase the PVC size.

* Before considering the expansion of the the cluster disk, make sure the volumes/data is backed up in desired format, so that any failure can be tolerated by restoring from the backup.

* Make sure the cluster storage class has `allowVolumeExpansion: true` before applying the new `diskSize`. For more details checkout the [kubernetes storage classes](https://kubernetes.io/docs/concepts/storage/storage-classes/) document.

* Once the above step is done, the cluster yaml can be applied with new `diskSize` value, to all decalared nodepool components or to single component.

* It is best recommended not to apply any new changes to the cluster along with volume expansion.

* Make sure the declared size definitions are proper and consistent, example if the `diskSize` is in `G` or `Gi`, make sure the same size definitions are followed for expansion.

Note: To change the `diskSize` from `G` to `Gi` or vice-versa, first make sure data is backed up and make sure the right conversion number is identified, so that the underlying volume has the same value and then re-apply the cluster yaml. This will make sure the statefulset is re-created with right value in VolueClaimTemplates, this operation is expected to have no downtime.

## Rolling Upgrades

OpenSearch upgrades are controlled by the `spec.general.version` field:

```yaml
spec:
  general:
    version: 1.2.3
    drainDataNodes: false
```

To perform a rolling upgrade on the cluster, simply change this version and the Operator will perform a rolling upgrade. Downgrades and upgrades that span more than one major version are not supported, as this will put the OpenSearch cluster in an unsupported state. If you are using emptyDir storage for data nodes, it is recommended to set `general.drainDataNodes` to `true`, otherwise you might lose data.

## Set Java heap size

To configure the amount of memory allocated to the OpenSearch nodes, configure the heap size using the JVM args. This operation is expected to have no downtime and the cluster should be operational.

Recommendation: Set to half of memory request

```yaml
spec:
    nodePools:
      - component: nodes
        replicas: 3
        diskSize: "10Gi"
        jvm: -Xmx1024M -Xms1024M
        NodeSelector:
        resources:
          requests:
            memory: "2Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "500m"
        roles:
          - "data"
```

## Additional Volumes

Sometimes it is neccessary to mount ConfigMaps or Secrets into the Opensearch pods as volumes to provide additional configuration (e.g. plugin config files).  This can be achieved by providing an array of additional volumes to mount to the custom resource.  This option is located in either `spec.general.additionalVolumes` or `spec.dashboards.additionalVolumes.  The format is as follows:

```yaml
spec:
  general:
    additionalVolumes:
    - name: example-configmap
      path: /path/to/mount/volume
      configMap:
        name: config-map-name
      restartPods: true #set this to true to restart the pods when the content of the configMap changes
  dashboards:
    additionalVolumes:
    - name: example-secret
      path: /path/to/mount/volume
      secret:
        secretName: secret-name
```