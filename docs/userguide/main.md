# Opensearch Operator User Guide

This guide is intended for users of the Opensearch Operator. If you want to contribute to the development of the Operator, please see the [Design documents](../designs/high-level.md) and the [Developer guide](../developing.md) instead.

## Installation

The Operator can be easily installed using Helm:

1. Add the helm repo: `helm repo add opensearch-operator https://opster.github.io/opensearch-k8s-operator/`
2. Install the Operator: `helm install opensearch-operator opensearch-operator/opensearch-operator`

Follow the instructions in this video to install the Operator:

[![Watch the video](https://opster.com/wp-content/uploads/2022/05/Operator-Installation-Tutorial.png)](https://player.vimeo.com/video/708641527)

A few notes on operator releases:

* Please see the project README for a compatibility matrix which operator release is compatible with which OpenSearch release.
* The userguide in the repository corresponds to the current development state of the code. To view the documentation for a specific released version switch to that tag in the Github menu.
* We track feature requests as Github Issues. If you are missing a feature and find an issue for it, please be aware that an issue ticket closed as completed only means that feature has been implemented in the development version. After that it might still take some for the feature to be contained in a release. If you are unsure, please check the list of releases in our Github project if your feature is mentioned in the release notes.

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
    version: 2.3.0
  dashboards:
    enable: true
    version: 2.3.0
    replicas: 1
    resources:
      requests:
         memory: "512Mi"
         cpu: "200m"
      limits:
         memory: "512Mi"
         cpu: "200m"
  nodePools:
    - component: nodes
      replicas: 3
      diskSize: "5Gi"
      nodeSelector:
      resources:
         requests:
            memory: "2Gi"
            cpu: "500m"
         limits:
            memory: "2Gi"
            cpu: "500m"
      roles:
        - "cluster_manager"
        - "data"
```

Then run `kubectl apply -f cluster.yaml`. If you watch the cluster (e.g. `watch -n 2 kubectl get pods`), you will see that after a few seconds the Operator will create several pods. First, a bootstrap pod will be created (`my-first-cluster-bootstrap-0`) that helps with initial master discovery. Then three pods for the OpenSearch cluster will be created (`my-first-cluster-masters-0/1/2`), and one pod for the dashboards instance. After the pods are appearing as ready, which normally takes about 1-2 minutes, you can connect to your cluster using port-forwarding.

Run `kubectl port-forward svc/my-first-cluster-dashboards 5601`, then open [http://localhost:5601](http://localhost:5601) in your browser and log in with the default demo credentials `admin / admin`.
Alternatively, if you want to access the OpenSearch REST API, run: `kubectl port-forward svc/my-first-cluster 9200`. Then open a second terminal and run: `curl -k -u admin:admin https://localhost:9200/_cat/nodes?v`. You should see the three deployed pods listed.

If you'd like to delete your cluster, run: `kubectl delete -f cluster.yaml`. The Operator will then clean up and delete any Kubernetes resources created for the cluster. Note that this will not delete the persistent volumes for the cluster, in most cases. For a complete cleanup, run: `kubectl delete pvc -l opster.io/opensearch-cluster=my-first-cluster` to also delete the PVCs.

The minimal cluster you deployed in this section is only intended for demo purposes. Please see the next sections on how to configure and manage the different aspects of your cluster.

**Single-Node clusters are currently not supported**. Your cluster must have at least 3 nodes with the `master/cluster_manager` role configured.

## Configuring OpenSearch

The main job of the operator is to deploy and manage OpenSearch clusters. As such it offers a wide range of options to configure clusters.

### Nodepools and Scaling

OpenSearch clusters are composed of one or more node pools, with each representing a logical group of nodes that have the same [role](https://opensearch.org/docs/latest/opensearch/cluster/). Each node pool can have its own resources. For each configured nodepool the operator will create a Kubernetes StatefulSet. It also creates a Kubernetes service object for each nodepool so you can communicate with a specfic nodepool if you want.

```yaml
spec:
    nodePools:
      - component: masters
        replicas: 3  # The number of replicas
        diskSize: "30Gi" # The disk size to use 
        resources: # The resource requests and limits for that nodepool
          requests:
            memory: "2Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "500m"
        roles: # The roles the nodes should have
          - "cluster_manager"
          - "data"
      - component: nodes
        replicas: 3
        diskSize: "10Gi"
        nodeSelector:
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

Additional configuration options are available for node pools and are documented in this guide in later sections.

### Configuring opensearch.yml

The Operator automatically generates the main OpenSearch configuration file `opensearch.yml` based on the parameters you provide in the different sections (e.g. TLS configuration). If you need to add your own settings, you can do that using the `additionalConfig` field in the cluster spec:

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

Using `spec.general.additionalConfig` you can add settings to all nodes, using `nodePools[].additionalConfig` you can add settings to only a pool of nodes. The settings must be provided as a map of strings, so use the flat form of any setting. If the value you want to provide is not a string, put it in quotes (for example `"true"` or `"1234"`). The Operator merges its own generated settings with whatever extra settings you provide. Note that basic settings like `node.name`, `node.roles`, `cluster.name` and settings related to network and discovery are set by the Operator and cannot be overwritten using `additionalConfig`. The value of `spec.general.additionalConfig` is also used for configuring the bootstrap pod. To overwrite the values of the bootstrap pod, set the field `spec.bootstrap.additionalConfig`.

Note that changing any of the `additionalConfig` will trigger a rolling restart of the cluster. If want to avoid that please use the [Cluster Settings API](https://opensearch.org/docs/latest/opensearch/configuration/#update-cluster-settings-using-the-api) to change them at runtime.

### TLS

For security reasons, encryption is required for communication with the OpenSearch cluster and between cluster nodes. If you do not configure any encryption, OpenSearch will use the included demo TLS certificates, which are not ideal for most active deployments.

Depending on your requirements, the Operator offers two ways of managing TLS certificates. You can either supply your own certificates, or the Operator will generate its own CA and sign certificates for all nodes using that CA. The second option is recommended, unless you want to directly expose your OpenSearch cluster outside your Kubernetes cluster, or your organization has rules about using self-signed certificates for internal communication.

TLS certificates are used in three places, and each can be configured independently.

#### Node Transport

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

If you provide the certificates yourself, you must also provide the list of certificate DNs in `nodesDn`, wildcards can be used (e.g. `"CN=my-first-cluster-*,OU=my-org"`).

If you provide your own node certificates you must also provide an admin cert that the operator can use for managing the cluster:

```yaml
spec:
  security:
    config:
      adminSecret: 
        name: my-first-cluster-admin-cert # The secret must have keys tls.crt and tls.key
```

Make sure the DN of the certificate is set in the `adminDn` field.

#### Node HTTP/REST API

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

### Adding plugins

You can extend the functionality of OpenSearch via [plugins](https://opensearch.org/docs/latest/install-and-configure/install-opensearch/plugins/#available-plugins). Commonly used ones are snapshot repository plugins for external backups (e.g. to AWS S3 or Azure Blog Storage). The operator has support to automatically install such plugins during setup.

To install a plugin for opensearch add it to the list under `general.pluginsList`:

```yaml
  general:
    version: 2.3.0
    httpPort: 9200
    vendor: opensearch
    serviceName: my-cluster
    pluginsList: ["repository-s3","https://github.com/aiven/prometheus-exporter-plugin-for-opensearch/releases/download/1.3.0.0/prometheus-exporter-1.3.0.0.zip"]
```

To install a plugin for opensearch dashboards add it to the list under `dashboards.pluginsList`:

```yaml
  dashboards:
    enable: true
    version: 2.4.1
    pluginsList:
      - sample-plugin-name
```

Please note:

* [Bundled plugins](https://opensearch.org/docs/latest/install-and-configure/install-opensearch/plugins/#bundled-plugins) do not have to be added to the list, they are installed automatically
* You can provide either a plugin name or a complete to the plugin zip. The items you provide are passed to the `bin/opensearch-plugin install <plugin-name>` command.
* Updating the list for an already installed cluster will lead to a rolling restart of all opensearch nodes to install the new plugin.
* If your plugin requires additional configuration you must provide that either through `additionalConfig` (see section [Configuring opensearch.yml](#configuring-opensearchyml)) or as secrets in the opensearch keystore (see section [Add secrets to keystore](#add-secrets-to-keystore)).

## Add secrets to keystore

Some OpenSearch features (e.g. snapshot repository plugins) require sensitive configuration. This is handled via the opensearch keystore. The operator allows you to populate this keystore using Kubernetes secrets.
To do so add the secrets under the `general.keystore` section:

```yaml
  general:
    # ...
    keystore:
    - secret:
        name: credentials
    - secret:
        name: some-other-secret
```

With this configuration all keys of the secrets will become keys in the keystore.

If you only want to load some keys from a secret or rename the existing keys, you can add key mappings as a map:

```yaml
  general:
    # ...
    keystore:
    - secret:
        name: many-secret-values
      keyMappings:
        # Only read "sensitive-value" from the secret, keep its name.
        sensitive-value: sensitive-value
    - secret:
        name: credentials
      keyMappings:
        # Renames key accessKey in secret to s3.client.default.access_key in keystore
        accessKey: s3.client.default.access_key
        password: s3.client.default.secret_key
```

Note that only provided keys will be loaded from the secret! Any keys not specified will be ignored.

### Set Java heap size

To configure the amount of memory allocated to the OpenSearch nodes, configure the heap size using the JVM args. This operation is expected to have no downtime and the cluster should be operational.

Recommendation: Set to half of memory request

```yaml
spec:
    nodePools:
      - component: nodes
        replicas: 3
        diskSize: "10Gi"
        jvm: -Xmx1024M -Xms1024M
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

## Configuring Dashboards

The operator can automatically deploy and manage a OpenSearch Dashboards instance. To do so add the following section to your cluster spec:

```yaml
# ...
spec:
  dashboards:
    enable: true  # Set this to true to enable the Dashboards deployment
    version: 2.3.0  # The Dashboards version to deploy. This should match the configured opensearch version
    replicas: 1  # The number of replicas to deploy
```

### Configuring opensearch_dashboards.yml

You can customize the OpenSearch Dashboards configuration ([`opensearch_dashboards.yml`](https://github.com/opensearch-project/OpenSearch-Dashboards/blob/main/config/opensearch_dashboards.yml)) using the `additionalConfig` field in the dashboards section of the `OpenSearchCluster` custom resource:

```yaml
apiVersion: opensearch.opster.io/v1
kind: OpenSearchCluster
#...
spec:
  dashboards:
    additionalConfig:
      opensearch_security.auth.type: "proxy"
      opensearch.requestHeadersWhitelist: |
        ["securitytenant","Authorization","x-forwarded-for","x-auth-request-access-token", "x-auth-request-email", "x-auth-request-groups"]
      opensearch_security.multitenancy.enabled: "true"
```

You can for example use this to set up any of the [backend](https://opensearch.org/docs/latest/security-plugin/configuration/configuration/) authentication types for Dashboards.

Note that the configuration must be valid or the Dashboards instance will fail to start.

### Storing sensitive information in the dashboards configuration

There are situations where you need to store sensitive information inside the dashboards configuration file (for example a client secret for OpenIDConnect). To do this safely you can utilize the fact that OpenSearch Dashboards does variable substitution in its configuration file.

For this to work you need to create a secret with the sensitive information (for example `dashboards-oidc-config`) and then mount that secret as an environment variable into the Dashboards pod (see the section on [Adding environment variables to pods](#adding-environment-variables-to-pods) on how to do that). You can then reference any keys from that secret in your dashboards configuration.

As an example this is a part of a cluster spec:

```yaml
spec:
  dashboards:
    env:
      - name: OPENID_CLIENT_SECRET
        valueFrom:
          secretKeyRef:
            name: dashboards-oidc-config
            key: client_secret
    additionalConfig:
      opensearch_security.openid.client_secret: "${OPENID_CLIENT_SECRET}"
```

Note that changing the value in the secret has no direct influence on the dashboards config. For this to take effect you need to restart the dashboards pods.

### Configuring a basePath

When using OpenSearch behind a reverse proxy on a subpath (e.g. `/logs`) you have to configure a base path. This can be achieved by setting the base path field in the configuraiton of OpenSearch Dashboards. Behind the scenes the correct configuration options are automatically added to the dashboards configuration.

```yaml
apiVersion: opensearch.opster.io/v1
kind: OpenSearchCluster
...
spec:
  dashboards:
    enable: true
    basePath: "/logs"
```

This also sets the `server.rewriteBasePath` option to `true`. So if you expose Dashboards via an ingress controller you must configure it appropriately.

### Dashboards HTTP

OpenSearch Dashboards can expose its API/UI via HTTP or HTTPS. It is unencrypted by default. Similar to how the operator handles TLS for the opensearch nodes, to secure the connection you can either let the Operator generate and sign a certificate, or provide your own. The following fields in the `OpenSearchCluster` custom resource are available to configure it:

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

If you want to expose Dashboards outside of the cluster, it is recommended to use Operator-generated certificates internally and let an Ingress present a valid certificate from an accredited CA (e.g. LetsEncrypt).

## Customizing the kubernetes deployment

Besides configuring OpenSearch itself, the operator also allows you to customize how the operator deploys the opensearch and dashboards pods.

### Data Persistence

By default, the Operator will create OpenSearch node pools with persistent storage from the default [Storage Class](https://kubernetes.io/docs/concepts/storage/storage-classes/). This behaviour can be changed per node pool. You may supply an alternative storage class and access mode, or configure hostPath or emptyDir storage.

The available storage options are:

#### PVC

The default option is persistent storage via PVCs. You can explicity define the `storageClass` if needed:

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
      storageClass: mystorageclass  # Set the name of the storage class to be used
      accessModes: # You can change the accessMode
      - ReadWriteOnce
```

#### EmptyDir

If you do not want to use persistent storage you can use the `emptyDir` option. Beware that this can lead to data loss, so you should only use this option for testing, or for data that is otherwise persisted.

```yaml
nodePools:
- component: masters
  replicas: 3
  diskSize: 30
  roles:
    - "data"
    - "master"
  persistence:
    emptyDir: {}  # This configures emptyDir
```

If you are using emptyDir, it is recommended that you set `spec.general.drainDataNodes` to be `true`. This will ensure that shards are drained from the pods before rolling upgrades or restart operations are performed.

#### HostPath

As a last option you can hose a `hostPath`. Please note that hostPath is strongly discouraged, and if you do choose this option, then you must also configure affinity for the node pool to ensure that multiple pods do not schedule to the same Kubernetes host.

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
      path: "/var/opensearch"  # Define the path on the host here
```

### Labels or Annotations on OpenSearch nodes

You can add additional labels or annotations on the nodepool configuration. This is useful for integration with other applications such as a service mesh, or configuring a prometheus scrape endpoint:

```yaml
spec:
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "5Gi"
      labels:  # Add any extra labels as key-value pairs here
        someLabelKey: someLabelValue
      annotations:  # Add any extra annotations as key-value pairs here
        someAnnotationKey: someAnnotationValue
      nodeSelector:
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

Any annotations and labels defined will be added directly to the pods of the nodepools.

### Add Labels or Annotations to the Dashboard Deployment

You can add labels or annotations to the dashboard pod specification. This is helpful if you want the dashboard to be part of a service mesh or integrate with other applications that rely on labels or annotations.

```yaml
spec:
  dashboards:
    enable: true
    version: 1.3.1
    replicas: 1
    labels:  # Add any extra labels as key-value pairs here
      someLabelKey: someLabelValue
    annotations:  # Add any extra annotations as key-value pairs here
      someAnnotationKey: someAnnotationValue
```

Any annotations and labels defined will be added directly to the dashboards pods.

### Priority class on OpenSearch nodes

You can configure OpenSearch nodes to use a `PriorityClass` using the name of the priority class. This is useful to prevent unwanted evictions of your OpenSearch nodes.

```yaml
spec:
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "5Gi"
      priorityClassName: somePriorityClassName
      resources:
         requests:
            memory: "2Gi"
            cpu: "500m"
         limits:
            memory: "2Gi"
            cpu: "500m"
      roles:
        - "master"
```

### Additional Volumes

Sometimes it is neccessary to mount ConfigMaps or Secrets into the Opensearch pods as volumes to provide additional configuration (e.g. plugin config files).  This can be achieved by providing an array of additional volumes to mount to the custom resource. This option is located in either `spec.general.additionalVolumes` or `spec.dashboards.additionalVolumes`.  The format is as follows:

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

The defined volumes are added to all pods of the opensearch cluster. It is currently not possible to define them per nodepool.

### Adding environment variables to pods

The operator allows you to add your own environment variables to the opensearch pods and the Dashboards pods. You can provide the value as a string literal or mount it from a secret or configmap.

The structure is the same for both opensearch and dashboards:

```yaml
spec:
  dashboards:
    env:
      - name: MY_ENV_VAR
        value: "myvalue"
      - name: MY_SECRET_VAR
        valueFrom:
          secretKeyRef:
            name: my-secret
            key: some_key
      - name: MY_CONFIGMAP_VAR
        valueFrom:
          configMapKeyRef:
            name: my-configmap
            key: some_key
  nodePools:
    - component: nodes
      env:
        - name: MY_ENV_VAR
          value: "myvalue"
        # the other options are supported here as well 
```

### Custom cluster domain name

If your Kubernetes cluster is configured with a custom domain name (default is `cluster.local`) you need to configure the operator accordingly in order for internal routing to work properly. This can be achieved by setting `manager.dnsBase` in the **helm chart values**.

```yaml
manager:
  # ...
  dnsBase: custom.domain
```

### Custom init helper

During cluster initialization the operator uses init containers as helpers. For these containers a busybox image is used ( specifically `public.ecr.aws/opsterio/busybox:1.27.2-buildx`). In case you are working in an offline environment and the cluster cannot access the registry or you want to customize the image, you can override the image used by specifying the `initHelper` image in your cluster spec:

```yaml
  spec:     
    initHelper:
      # You can either only specify the version
      version: "1.27.2-buildcustom"
      # or specify a totally different image
      image: "mycustomrepo.cr/mycustombusybox:myversion"
      # Additionally you can define the imagePullPolicy
      imagePullPolicy: IfNotPresent
      # and imagePullSecrets if needed
      imagePullSecrets:
        - name: docker-pull-secret
```

### Expsing OpenSearch Dashboards

If you want to expose the Dashboards instance of your cluster for users/services outside of your Kubernetes cluster, the recommended way is to do this via ingress.

A simple example:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: opensearch-dashboards
  namespace: default
spec:
  tls:
  - hosts:
    - dashboards.my.company
  rules:
  - host: dashboards.my.company
    http:
      paths:
      - backend:
          service:
            name: my-cluster-dashboards
            port:
              number: 5601
        path: "/(.*)"
        pathType: ImplementationSpecific
```

Note: If you have enabled HTTPS for dashboards you need to instruct your ingress-controller to use a HTTPS connection internally. This is specific for the controller you are using (e.g. nginx-ingress, traefik, ...).

### Configuring the Dashboards K8s Service

You can customize the Kubernetes Service object that the operator generates for the Dashboards deployment.

Supported Service Types

* ClusterIP (default)
* NodePort
* LoadBalancer

When using type LoadBalancer you can optionally set the load balancer source ranges.

```yaml
apiVersion: opensearch.opster.io/v1
kind: OpenSearchCluster
...
spec:
  dashboards:
    service:
      type: LoadBalancer  # Set one of the supported types
      loadBalancerSourceRanges: "10.0.0.0/24, 192.168.0.0/24"  # Optional, add source ranges for a loadbalancer
```

### Exposing the OpenSearch cluster REST API

If you want to expose the REST API of OpenSearch outside your Kubernetes cluster, the recommended way is to do this via ingress.
Internally you should use self-signed certificates (you can let the operator generate them), and then let the ingress use a certificate from an accepted CA (for example LetsEncrypt or a company-internal CA). That way you do not have the hassle of supplying custom certificates to the opensearch cluster but your users still see valid certificates.

## Cluster operations

The operator contains several features that automate management tasks that might be needed during the cluster lifecycle. The different available options are documented here.

### Rolling Upgrades

The operator supports automatic rolling version upgrades. To do so simply change the `general.version` in your cluster spec and reapply it:

```yaml
spec:
  general:
    version: 1.2.3
```

The Operator will then perform a rolling upgrade and restart the nodes one-by-one, waiting after each node for the cluster to stabilize and have a green cluster status. Depending on the number of nodes and the size of the data stored this can take some time.
Downgrades and upgrades that span more than one major version are not supported, as this will put the OpenSearch cluster in an unsupported state. If you are using emptyDir storage for data nodes, it is recommended to set `general.drainDataNodes` to `true`, otherwise you might lose data.

### Configuration changes

As explained in the section [Configuring opensearch.yml](#configuring-opensearchyml) you can add extra opensearch configuration to your cluster. Changing this configuration on an already installed cluster will be detected by the operator and it will do a rolling restart of all cluster nodes to apply that new configuration. The same goes for nodepool-specific configuration like `resources`, `annotation` or `labels`.

### Volume Expansion

If your underlying storage supports online volume expansion the operator can orchestrate that action for you.

To increase the disk volume size set the `diskSize` of a nodepool to the desired value and re-apply the cluster spec yaml. This operation is expected to have no downtime and the cluster should be operational.

The following considerations should be taken into account in order to increase the PVC size.

* This only works for PVC-based persistence
* Before considering the expansion of the the cluster disk, make sure the volumes/data is backed up in desired format, so that any failure can be tolerated by restoring from the backup.
* Make sure the cluster storage class has `allowVolumeExpansion: true` before applying the new `diskSize`. For more details checkout the [kubernetes storage classes](https://kubernetes.io/docs/concepts/storage/storage-classes/) document.
* Once the above step is done, the cluster yaml can be applied with new `diskSize` value, to all decalared nodepool components or to single component.
* It is best recommended not to apply any new changes to the cluster along with volume expansion.
* Make sure the declared size definitions are proper and consistent, example if the `diskSize` is in `G` or `Gi`, make sure the same size definitions are followed for expansion.

Note: To change the `diskSize` from `G` to `Gi` or vice-versa, first make sure data is backed up and make sure the right conversion number is identified, so that the underlying volume has the same value and then re-apply the cluster yaml. This will make sure the statefulset is re-created with right value in VolueClaimTemplates, this operation is expected to have no downtime.

## User and role management

An important part of any OpenSearch cluster is the user and role management to give users access to the cluster (via the opensearch-security plugin). By default the operator will use the included demo securityconfig with default users (see [internal_users.yml](https://github.com/opensearch-project/security/blob/main/securityconfig/internal_users.yml) for a list of users). For any production installation you should swap that out with your own configuration.
There are two ways to do that with the operator:

* Defining your own securityconfig
* Managing users and roles via kubernetes resources

Note that currently a combination of both approaches is not possible. Once you use the CRDs you cannot provide your own securityconfig as those would overwrite each other. We are working on a feature to merge these options.

### Securityconfig

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

Provide the name of the secret that contains your securityconfig yaml files as `securityconfigSecret.name`. Note that all files must be provided, you cannot provide only some of them, as the demo files and your provided ones cannot be merged. In addition, you must provide the name of a secret as `adminCredentialsSecret.name` that has fields `username` and `password` for a user that the Operator can use for communicating with OpenSearch (currently used for getting the cluster status, doing health checks and coordinating node draining during cluster scaling operations). This user must be defined in your securityconfig and must have appropriate permissions (currently admin).

In addition you must also configure TLS transport (see [Node Transport](#node-transport)). You can either let the operator generate all needed certificates or supply them yourself. If you use your own certificates you must also provide an admin certificate that the operator can use to apply the securityconfig.

If you provided your own certificate for node transport communication, then you must also provide an admin client certificate (as a Kubernetes TLS secret with fields `ca.crt`, `tls.key` and `tls.crt`) as `adminSecret.name`. The DN of the certificate must be listed under `security.tls.transport.adminDn`. Be advised that the `adminDn` and `nodesDn` must be defined in a way that the admin certficate cannot be used or recognized as a node certficiate, otherwise OpenSearch will reject any authentication request using the admin certificate.

To apply the securityconfig to the OpenSearch cluster, the Operator uses a separate Kubernetes job (named `<cluster-name>-securityconfig-update`). This job is run during the initial provisioning of the cluster. The Operator also monitors the secret with the securityconfig for any changes and then reruns the update job to apply the new config. Note that the Operator only checks for changes in certain intervals, so it might take a minute or two for the changes to be applied. If the changes are not applied after a few minutes, please use 'kubectl' to check the logs of the pod of the `<cluster-name>-securityconfig-update` job. If you have an error in your configuration it will be reported there.

### Managing users and roles with kubernetes resources

The operator provides custom kubernetes resources that allow you to manage users and roles as kubernetes objects.

#### Opensearch Users

It is possible to manage Opensearch users in Kubernetes with the operator. The operator will not modify users that already exist. You can create an example user as follows:

```yaml
apiVersion: opensearch.opster.io/v1
kind: OpensearchUser
metadata:
  name: sample-user
  namespace: default
spec:
  opensearchCluster:
    name: my-first-cluster
  passwordFrom:
    name: sample-user-password
    key: password
  backendRoles:
  - kibanauser
```

The namespace of the `OpenSearchUser` must be the namespace the OpenSearch cluster itself is deployed in.

Note that a secret called `sample-user-password` will need to exist in the `default` namespace with the base64 encoded password in the `password` key.

#### Opensearch Roles

It is possible to manage Opensearch roles in Kubernetes with the operator. The operator will not modify roles that already exist. You can create an example role as follows:

```yaml
apiVersion: opensearch.opster.io/v1
kind: OpensearchRole
metadata:
  name: sample-role
  namespace: default
spec:
  opensearchCluster:
    name: my-first-cluster
  clusterPermissions:
  - cluster_composite_ops
  - cluster_monitor
  indexPermissions:
  - indexPatterns:
    - logs*
    allowedActions:
    - index
    - read
```

#### Linking Opensearch Users and Roles

The operator allows you link any number of users, backend roles and roles with a OpensearchUserRoleBinding. Each user in the binding will be granted each role. E.g:

```yaml
apiVersion: opensearch.opster.io/v1
kind: OpensearchUserRoleBinding
metadata:
  name: sample-urb
  namespace: default
spec:
  opensearchCluster:
    name: my-first-cluster
  users:
  - sample-user
  backendRoles:
  - sample-backend-role
  roles:
  - sample-role
```

### Custom Admin User

In order to create your cluster with an adminuser different from the default `admin:admin` you will have to walk through the following steps:
First you will have to create a secret with your admin user configuration (in this example `admin-credentials-secret`):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: admin-credentials-secret
type: Opaque
data:
  # admin
  username: YWRtaW4=
  # admin123
  password: YWRtaW4xMjM=
```

Then you have to create your own securityconfig and store it in a secret (`securityconfig-secret` in this example). You can take a look at [securityconfig-secret.yaml](../../opensearch-operator/examples/securityconfig-secret.yaml) for how such a secret should look like.
Make sure that the password hash of the admin user corresponds to the password you stored in the `admin-credentials-secret`.

Notice that inside `securityconfig-secret` You must edit the `hash` of the admin user before creating the secret. if you have python 3.x installed on your machine you can use the following command to hash your password: `python -c 'import bcrypt; print(bcrypt.hashpw("admin123".encode("utf-8"), bcrypt.gensalt(12, prefix=b"2a")).decode("utf-8"))'`

```yaml
  internal_users.yml: |-
    _meta:
      type: "internalusers"
      config_version: 2
    admin:
      hash: "$2y$12$lJsHWchewGVcGlYgE3js/O4bkTZynETyXChAITarCHLz8cuaueIyq"   <------- change that hash to your new password hash
      reserved: true
      backend_roles:
      - "admin"
      description: "Demo admin user"
  ```

The last thing that you have to do is to add that security configuration to your cluster spec:

```yaml
  security:
    config:
      adminCredentialsSecret:
        name: admin-credentials-secret  # The secret with the admin credentials for the operator to use
      securityConfigSecret:
       name: securityconfig-secret  # The secret containing your customized securityconfig
    tls:
      transport:
        generate: true
      http:
        generate: true
```

Changing the admin password after the cluster has been created is possible via the same way. You must update your securityconfig (in the `securityconfig-secret`) and the content of the `admin-credentials-secret` to both reflect the new password. Note that currently the operator cannot make changes in the securityconfig itself. As such you must always update the securityconfig in the secret with the new password and in addition provide it via the credentials secret so that the operator can still access the cluster.

### Custom Dashboards user

Dashboards requires an opensearch user to connect to the cluster. By default Dashboards is configured to use the demo admin user. If you supply your own securityconfig and want to change the credentials Dashboards should use, you must create a secret with keys `username` and `password` that contains the new credentials and then supply that secret to the operator via the cluster spec:

```yaml
spec:
  dashboards:
    opensearchCredentialsSecret:
      name: dashboards-credentials  # This is the name of your secret that contains the credentials for Dashboards to use
```


## Adding Opensearch Monitoring to your cluster

The operator allows you to install and enable the Aiven monitoring plugin for OpenSearch on your cluster as a built-in feature (https://github.com/aiven/prometheus-exporter-plugin-for-opensearch)
That feature needs internet connectivity to download the plugin. if you are working in a restricted environment, please download the plugin zip for your cluster version (example for 2.3.0: https://github.com/aiven/prometheus-exporter-plugin-for-opensearch/releases/download/2.3.0.0/prometheus-exporter-2.3.0.0.zip) and provide it at a location the operator can reach. Configure that URL as `pluginURL` in the monitoring config.
By default the Opensearch admin user will be used to access the monitoring API. If you want to use a separate user with limited permissions you need to create that user using either of the following options:
1) Create new applicative User using OpenSearch API/UI, create new secret with 'username':'password' keys and provide that secret name under monitoringUserSecret.
2) Use Our OpenSearchUser CRD and provide the secret under monitoringUserSecret.
```yaml
apiVersion: opensearch.opster.io/v1
kind: OpenSearchCluster
metadata:
  name: my-first-cluster
  namespace: default
spec:
  general:
    version: <YOUR_CLUSTER_VERSION>
    monitoring:
      enable: true
      interval: 30s
      monitoringUserSecret: appUserSecret
      pluginUrl: https://github.com/aiven/prometheus-exporter-plugin-for-opensearch/releases/download/<YOUR_CLUSTER_VERSION>/prometheus-exporter-<YOUR_CLUSTER_VERSION>.zip
```
