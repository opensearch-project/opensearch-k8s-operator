# Opensearch Operator User Guide

This guide is intended for users of the Opensearch Operator. If you want to contribute to the development of the Operator, please see the [Design documents](../designs/high-level.md) and the [Developer guide](../developing.md) instead.

> **API Group Migration Notice**: The operator is migrating from `opensearch.opster.io` to `opensearch.org` API group. Both are currently supported, but `opensearch.opster.io` is deprecated. Upgrading the operator from 2.x to 3.x rolling-restarts every node of existing clusters once. Please see the [Migration Guide](./migration-guide.md) for details.

## Installation

### Prerequisites

The chart installs a validation webhook by default (`webhook.enabled=true`) and issues its serving certificate with [cert-manager](https://cert-manager.io/) (`webhook.certManager.enabled=true`), so cert-manager 1.0 or later has to be present in the cluster before the Operator is installed. See the [cert-manager installation docs](https://cert-manager.io/docs/installation/helm/) for the current options:

```bash
helm repo add jetstack https://charts.jetstack.io
helm install cert-manager jetstack/cert-manager --namespace cert-manager --create-namespace --set crds.enabled=true
```

Without it, `helm install` fails while rendering the `Issuer` and `Certificate` resources of the webhook:

```text
Error: INSTALLATION FAILED: unable to build kubernetes objects from release manifest: ... no matches for kind "Certificate" in version "cert-manager.io/v1" ... ensure CRDs are installed first
```

If you would rather not run cert-manager, the [Webhooks guide](./webhooks.md) describes the two alternatives: set `webhook.certManager.enabled=false` and provide the serving certificate yourself through `webhook.secretName`, or turn the webhook off entirely with `webhook.enabled=false`.

The Operator can be easily installed using Helm:

1. Add the helm repo: `helm repo add opensearch-operator https://opensearch-project.github.io/opensearch-k8s-operator/`
2. Install the Operator: `helm install opensearch-operator opensearch-operator/opensearch-operator`

Follow the instructions in this video to install the Operator:

[![Watch the video](https://github.com/user-attachments/assets/3e8881b4-4b93-4322-86e2-f46baa01cad0)](https://pulse.support/kb/running-opensearch-on-kubernetes-video-tutorial-series)

A few notes on operator releases:

- Please see the project README for a compatibility matrix which operator release is compatible with which OpenSearch release.
- The userguide in the repository corresponds to the current development state of the code. To view the documentation for a specific released version switch to that tag in the Github menu.
- A feature issue closed as completed means the feature is in the development version. Check the release notes to see whether it has been released.

## Quickstart

After you have successfully installed the Operator, you can deploy your first OpenSearch cluster. This is done by creating an `OpenSearchCluster` custom object in Kubernetes or using Helm.

### Using Helm

An OpenSearch cluster can be easily deployed using Helm. Follow the instructions in [Cluster Chart Guide](./cluster-chart.md) to install a cluster.

### Using Custom Object

Create a file `cluster.yaml` with the following content:

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
metadata:
  name: my-first-cluster
  namespace: default
spec:
  general:
    serviceName: my-first-cluster
    version: "3"
  dashboards:
    enable: true
    version: "3"
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
  security:
    tls:
      transport:
        generate: true
      http:
        generate: true
```

Then run `kubectl apply -f cluster.yaml`. If you watch the cluster (e.g. `watch -n 2 kubectl get pods`), you will see that after a few seconds the Operator will create several pods. First, a bootstrap pod will be created (`my-first-cluster-bootstrap-0`) that helps with initial master discovery. Then three pods for the OpenSearch cluster will be created (`my-first-cluster-nodes-0/1/2`), and one pod for the dashboards instance. After the pods are appearing as ready, which normally takes about 1-2 minutes, you can connect to your cluster using port-forwarding.

The operator generates a random password for the `admin` user and stores it in the `my-first-cluster-admin-password` Secret. Read it with:

```bash
kubectl get secret my-first-cluster-admin-password -o jsonpath='{.data.password}' | base64 -d
```

Run `kubectl port-forward svc/my-first-cluster-dashboards 5601`, then open [http://localhost:5601](http://localhost:5601) in your browser and log in as `admin` with that password. Dashboards itself connects to OpenSearch with its own generated credentials (the `my-first-cluster-dashboards-password` Secret), so it needs no extra configuration.
Alternatively, if you want to access the OpenSearch REST API, run: `kubectl port-forward svc/my-first-cluster 9200`. Then open a second terminal and run: `curl -k -u "admin:$(kubectl get secret my-first-cluster-admin-password -o jsonpath='{.data.password}' | base64 -d)" https://localhost:9200/_cat/nodes?v`. You should see the three deployed pods listed.

If you'd like to delete your cluster, run: `kubectl delete -f cluster.yaml`. The Operator will then clean up and delete any Kubernetes resources created for the cluster. The PVCs of the node pools are kept unless `general.persistentVolumeClaimRetentionPolicy` says `Delete` (see [Data Persistence](#data-persistence)). For a complete cleanup, run: `kubectl delete pvc -l opensearch.org/opensearch-cluster=my-first-cluster` to also delete the PVCs.

The minimal cluster you deployed in this section is only intended for demo purposes. Please see the next sections on how to configure and manage the different aspects of your cluster.

At least one node pool must have the `cluster_manager` (or `master`) role and `replicas` of at least 1; the webhook rejects clusters without one. Use 3 cluster-manager-eligible nodes for production so the cluster keeps a quorum when one node is down.

## Configuring the operator

The majority of this guide deals with configuring and managing OpenSearch clusters. But there are some general options that can be configured for the operator itself. All of this is done using helm values you provide during installation: `helm install opensearch-operator opensearch-operator/opensearch-operator -f values.yaml`.

For a list of all possible values see the [chart README](../../charts/opensearch-operator/README.md) and the [chart default values.yaml](../../charts/opensearch-operator/values.yaml). Some important ones:

> **Note:** The operator includes admission controller webhooks for validating OpenSearch CRDs. See the [Webhooks Documentation](./webhooks.md) for detailed information about webhook configuration, certificate management, and troubleshooting.

```yaml
manager:
  # Log level of the operator. Possible values: debug, info, warn, error
  loglevel: info

  # If specified, the operator will be restricted to watch objects only in the desired namespace. Defaults is to watch all namespaces.
  # To watch multiple namespaces, either separate their name via commas or define it as a list.
  # Examples:
  # watchNamespace: 'ns1,ns2'
  # watchNamespace: [ns1, ns2]
  watchNamespace:

  # Global default max concurrent reconciles for all controllers.
  maxConcurrentReconciles: 1

  # Per-controller overrides (controller name -> max concurrent reconciles).
  # Example:
  # maxConcurrentReconcilesPerController:
  #   opensearchcluster: 4
  maxConcurrentReconcilesPerController: {}

  # Configure extra environment variables for the operator. You can also pull them from secrets or configmaps
  extraEnv: []
  #  - name: MY_ENV
  #    value: somevalue
```

Other commonly used values:

| Value | Default | Description |
| --- | --- | --- |
| `installCRDs` | `true` | Install the CRDs with the chart. |
| `useRoleBindings` | `false` | Use namespace-scoped Roles/RoleBindings instead of ClusterRoles. Set `manager.watchNamespace` too. |
| `serviceAccount.create` / `serviceAccount.name` | `true` / `""` | Create the operator's ServiceAccount, or use an existing one. |
| `webhook.enabled` / `webhook.failurePolicy` | `true` / `Fail` | Validation webhooks and their failure policy (`Fail` or `Ignore`). |
| `manager.metricsBindAddress` | `127.0.0.1:8080` | Bind address of the operator's metrics endpoint. |
| `manager.dnsBase` | `cluster.local` | Kubernetes cluster domain (see [Custom cluster domain name](#custom-cluster-domain-name)). |
| `nodeSelector`, `tolerations`, `priorityClassName`, `podAnnotations`, `podLabels` | empty | Scheduling and metadata of the operator pod. |
| `securityContext` / `manager.securityContext` | `runAsNonRoot: true` / `allowPrivilegeEscalation: false` | Pod and container security context of the operator. |

Valid controller names for `maxConcurrentReconcilesPerController` are `opensearchcluster`, `opensearchuser`, `opensearchrole`, `opensearchtenant`, `opensearchuserrolebinding`, `opensearchactiongroup`, `opensearchismpolicy`, `opensearchindextemplate`, `opensearchcomponenttemplate` and `opensearchsnapshotpolicy`.

Logging can be tuned with environment variables set through `manager.extraEnv`: `OPERATOR_DEV_LOGGING=true` switches to human-readable development logging, and `OPERATOR_LOGGING_MESSAGE_KEY` renames the JSON key of the log message (default `msg`).

### Legacy API support

Support for the deprecated `opensearch.opster.io/v1` API group is enabled by
default. After migrating to `opensearch.org/v1`, it can be disabled in the
operator Helm values:

```yaml
legacyAPI:
  enabled: false
```

> **Warning:** Do not disable legacy API support while
> `opensearch.opster.io` resources still exist. The legacy CRDs are managed as
> regular Helm release resources, so disabling the legacy API during an upgrade
> removes those CRDs and deletes all remaining custom resources stored under
> them. Follow the [migration guide](./migration-guide.md) to migrate, verify,
> and delete every legacy resource before changing this setting.

### Pprof endpoints

To help diagnose memory or CPU problems in the operator, the standard go [pprof](https://pkg.go.dev/net/http/pprof) endpoints can be enabled by adding the following to your `values.yaml`:

```yaml
manager:
  pprofEndpointsEnabled: true
```

To access the endpoints you will need to use a port-forward as for security reasons the endpoints are only exposed on localhost inside the pod: `kubectl port-forward deployment/opensearch-operator 6060` (the Deployment is named after the Helm release fullname, `opensearch-operator` for a release of that name). Then from another terminal you can use the [go pprof tool](https://pkg.go.dev/net/http/pprof#hdr-Usage_examples), e.g.: `go tool pprof http://localhost:6060/debug/pprof/heap`.

### Custom Operator Communication URL

You can configure the operator to use a custom URL when communicating with OpenSearch by setting `operatorClusterURL`:

```yaml
spec:
  general:
    serviceName: my-cluster
    version: "3.2.0"
    httpPort: 9200
    vendor: "opensearch"
    operatorClusterURL: "opensearch.example.com"  # Optional: custom FQDN for operator communication
```

This is useful when using external certificates (e.g., from cert-manager) that are valid for a specific FQDN. The operator will use this URL instead of the default internal Kubernetes DNS name, allowing you to use a single certificate for both external access and operator communication.

For example, with a cert-manager `Certificate` for `opensearch.example.com` whose secret is set as `security.tls.http.secret.name`, set `operatorClusterURL: opensearch.example.com` so the operator's hostname matches the certificate. With provided HTTP certificates you also need an admin certificate in `security.config.adminSecret` (see [Node HTTP/REST API](#node-httprest-api)).

## Configuring OpenSearch

The main job of the operator is to deploy and manage OpenSearch clusters. As such it offers a wide range of options to configure clusters.

### Nodepools and Scaling

OpenSearch clusters are composed of one or more node pools, with each representing a logical group of nodes that have the same [role](https://opensearch.org/docs/latest/opensearch/cluster/). Each node pool can have its own resources. For each configured nodepool the operator will create a Kubernetes StatefulSet. It also creates a Kubernetes service object for each nodepool so you can communicate with a specific nodepool if you want.

```yaml
spec:
  nodePools:
    - component: masters
      replicas: 3 # The number of replicas
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
      resources:
        requests:
          memory: "2Gi"
          cpu: "500m"
        limits:
          memory: "2Gi"
          cpu: "500m"
      roles:
        - "data"
      nodeSelector: # Optional, schedule the pool's pods on matching Kubernetes nodes
        disktype: ssd
      tolerations: # Optional
        - key: "dedicated"
          operator: "Equal"
          value: "opensearch"
          effect: "NoSchedule"
```

- `diskSize` defaults to `30Gi` when omitted.
- Valid roles are `cluster_manager`, `master`, `data`, `data_content`, `data_hot`, `data_warm`, `data_cold`, `data_frozen`, `ingest`, `ml`, `remote_cluster_client`, `transform`, `search` and `warm`. The webhook rejects any other value.
- `master` is rendered as `cluster_manager` on OpenSearch 2.x and later (and `cluster_manager` as `master` on 1.x).
- `roles: []` creates coordinating-only nodes.

Additional configuration options are available for node pools and are documented in this guide in later sections. See the [CRD reference](../designs/crd/opensearch.org.md) for the full list of fields.

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

Using `spec.general.additionalConfig` you can add settings that will be applied to all nodes in the cluster. The settings are added to a shared configmap that is mounted to all node pools. If you need nodepool-specific configuration, you can use `nodePools[].additionalConfig` which will be merged with `spec.general.additionalConfig` for that specific nodepool (nodepool settings override general settings). When a nodepool has `additionalConfig` specified, it will get its own configmap with the merged configuration.

The settings must be provided as a map of strings, so use the flat form of any setting. If the value you want to provide is not a string, put it in quotes (for example `"true"` or `"1234"`). The Operator merges its own generated settings with whatever extra settings you provide. Note that basic settings like `node.name`, `node.roles`, `cluster.name` and settings related to network and discovery are set by the Operator and cannot be overwritten using `additionalConfig`.

Note that changing any of the `additionalConfig` will trigger a rolling restart of the cluster. If you want to avoid that, use the [Cluster Settings API](https://opensearch.org/docs/latest/opensearch/configuration/#update-cluster-settings-using-the-api) to change them at runtime.

### Images and other general options

By default the operator runs `docker.io/opensearchproject/opensearch:<general.version>`. The following `spec.general` fields change that and a few other pod-level settings:

```yaml
spec:
  general:
    version: "3.2.0"
    image: "myregistry.example.com/opensearch:3.2.0" # Optional, full image reference; version is then not used to pick the image
    defaultRepo: "myregistry.example.com/mirror" # Optional, registry/prefix for the opensearch, opensearch-dashboards and busybox images
    imagePullPolicy: IfNotPresent
    imagePullSecrets:
      - name: docker-pull-secret
    hostNetwork: false # Optional, run all cluster pods with host networking
    command: "./opensearch-docker-entrypoint.sh" # Optional, replaces the container's start command (plugins are still installed first)
```

With `defaultRepo` set and no `image`, the images become `<defaultRepo>/opensearch:<version>`, `<defaultRepo>/opensearch-dashboards:<version>` and `<defaultRepo>/busybox:latest` (the [init helper](#custom-init-helper) image).

`general.grpc` enables the gRPC API of OpenSearch 3.x (`grpc.enable: true`, plus port and tuning options). See the [gRPC example](../../opensearch-operator/examples/3.x/opensearch-cluster-grpc-example.yaml).

### Per-node pool image override

By default, all node pools use the image configured in `spec.general` (or the default `opensearchproject/opensearch` image for the cluster version). You can override the OpenSearch container image for a specific node pool by setting `image` on that node pool. This is useful when some node pools require a specialized image, such as GPU-enabled nodes that need a CUDA build of OpenSearch.

Node pool image settings override `spec.general` for that pool only. You can also set `imagePullPolicy` and `imagePullSecrets` per node pool.

```yaml
spec:
  general:
    version: "3.2.0"
  nodePools:
    - component: masters
      replicas: 3
      roles:
        - cluster_manager
    - component: ml
      replicas: 2
      roles:
        - ml
      image: "myregistry.example.com/opensearch-cuda:3.2.0"
      imagePullPolicy: IfNotPresent
      imagePullSecrets:
        - name: docker-pull-secret
```

### TLS

The OpenSearch security plugin requires TLS. If you configure neither `security.tls.transport` nor `security.tls.http`, the operator disables the security plugin (`plugins.security.disabled: true`) and the cluster runs without TLS and without authentication (see [Security Plugin Disabled](#security-plugin-disabled)).

Depending on your requirements, the Operator offers two ways of managing TLS certificates. You can either supply your own certificates, or the Operator will generate its own CA and sign certificates for all nodes using that CA. The second option is recommended, unless you want to directly expose your OpenSearch cluster outside your Kubernetes cluster, or your organization has rules about using self-signed certificates for internal communication.

When the operator generates certificates, you can control certificate validity using the `duration` field (e.g. `"720h"`, `"17520h"`). If omitted, it defaults to one year (`"8760h"`).

TLS certificates are used in three places, and each can be configured independently.

#### Node Transport

OpenSearch cluster nodes communicate with each other using the OpenSearch transport protocol (port 9300 by default). This is not exposed externally, so in almost all cases, generated certificates should be adequate.

To configure node transport security you can use the following fields in the `OpenSearchCluster` custom resource:

```yaml
# ...
spec:
  security:
    tls: # Everything related to TLS configuration
      transport: # Configuration of the transport endpoint
        enabled: true # Enable TLS for transport (default: true if transport config exists)
        generate: true # Have the operator generate and sign certificates
        perNode: true # Separate certificate per node
        # How long generated certificates are valid (default: 8760h = 1 year)
        duration: "8760h"
        secret:
          name: # Name of the secret that contains the provided certificate
        caSecret:
          name: # Name of the secret that contains a CA the operator should use
        nodesDn: [] # List of certificate DNs allowed to connect
# ...
```

To have the Operator generate the certificates, set `generate` and `perNode` to `true` (other fields can be omitted). The Operator will generate a CA certificate, issue one certificate per node, and sign them. Certificates default to one year validity, configurable via `duration`. The Operator reissues certificates `rotateDaysBeforeExpiry` days before they expire (default 30, set to -1 to disable). On OpenSearch 3.x and above TLS certificate hot reload is enabled by default (configurable via `enableHotReload`), so nodes load renewed certificates without a restart; when hot reload is disabled or unsupported (OpenSearch < 2.19.1) the Operator performs a rolling restart of the cluster instead. Expired or unreadable generated certificates are always reissued, regardless of `rotateDaysBeforeExpiry`.

Do not delete and regenerate the CA secret in place: that is not a supported rotation procedure and can leave the cluster in split trust while nodes pick up the new CA. Clusters created with operator 2.x may still have `rotateDaysBeforeExpiry: -1` stored; see [Other Changes When Upgrading from 2.x](./migration-guide.md#other-changes-when-upgrading-from-2x).

Alternatively, you can provide the certificates yourself (e.g. if your organization has an internal CA). You can either provide one certificate to be used by all nodes or provide a certificate for each node (recommended). In this mode, set `generate: false` and `perNode` to `true` or `false` depending on whether you're providing per-node certificates.

If you provide just one certificate, it must be placed in a Kubernetes TLS secret (with the fields `ca.crt`, `tls.key` and `tls.crt`, must all be PEM-encoded), and you must provide the name of the secret as `secret.name`. If you want to keep the CA certificate separate, you can place it in a separate secret and supply that as `caSecret.name`. If you provide one certificate per node, you must place all certificates into one secret (including the `ca.crt`) with a `<hostname>.key` and `<hostname>.crt` for each node. The hostname is defined as `<cluster-name>-<nodepool-component>-<index>` (e.g. `my-first-cluster-masters-0`).

If you provide the certificates yourself, you must also provide the list of certificate DNs in `nodesDn`, wildcards can be used (e.g. `"CN=my-first-cluster-*,OU=my-org"`).

#### Node HTTP/REST API

Each OpenSearch cluster node exposes the REST API using HTTPS (by default port 9200).

To configure HTTP API security, the following fields in the `OpenSearchCluster` custom resource are available:

```yaml
# ...
spec:
  security:
    tls: # Everything related to TLS configuration
      http: # Configuration of the HTTP endpoint
        enabled: true # Enable TLS for HTTP (default: true if http config exists, false to disable)
        generate: true # Have the Operator generate and sign certificates
        customFQDN: "opensearch.example.com" # Optional: Custom FQDN for the certificate
        # How long generated certificates are valid (default: 8760h = 1 year)
        duration: "8760h"
        secret:
          name: # Name of the secret that contains the provided certificate
        caSecret:
          name: # Name of the secret that contains a CA the Operator should use
# ...
```

Again, you have the option of either letting the Operator generate and sign the certificates or providing your own. The only difference between node transport certificates and node HTTP/REST APIs is that per-node certificate are not possible here. In all other respects the two work the same way.

**Note:** The `enabled` field controls whether TLS is enabled for the HTTP endpoint. If `enabled` is set to `false`, the cluster will use HTTP instead of HTTPS. If `enabled` is `nil` (not set), TLS is enabled by default when the HTTP config exists. To explicitly disable TLS, set `enabled: false`.

When using generated certificates, you can optionally specify a `customFQDN` field to include a custom domain in the certificate's Subject Alternative Names (SAN) alongside the default cluster DNS names.

If you provide your own certificates, please make sure the following names are added as SubjectAltNames (SAN): `<cluster-name>`, `<cluster-name>.<namespace>`, `<cluster-name>.<namespace>.svc`, `<cluster-name>.<namespace>.svc.<dnsBase>` (`dnsBase` is the operator's `manager.dnsBase` Helm value, `cluster.local` by default).

Directly exposing the node HTTP port outside the Kubernetes cluster is not recommended. Rather than doing so, you should configure an ingress. The ingress can then also present a certificate from an accredited CA (for example LetsEncrypt) and hide self-signed certificates that are being used internally. In this way, the nodes should be supplied internally with properly signed certificates.

If you provide your own node certificates you must also provide an admin cert that the operator can use for managing the cluster:

```yaml
spec:
  security:
    config:
      adminSecret:
        name: my-first-cluster-admin-cert # The secret must have keys tls.crt and tls.key
```

Make sure the DN of the certificate is listed under `security.tls.http.adminDn` (OpenSearch 2.0.0+). Clusters created with operator 2.x may still have the value under the deprecated `security.tls.transport.adminDn`; the operator honors that as a fallback.

### Adding plugins

You can extend the functionality of OpenSearch via [plugins](https://opensearch.org/docs/latest/install-and-configure/install-opensearch/plugins/#available-plugins). Commonly used ones are snapshot repository plugins for external backups (e.g. to AWS S3 or Azure Blob Storage). The operator has support to automatically install such plugins during setup.

To install a plugin for opensearch add it to the list under `general.pluginsList`:

```yaml
spec:
  general:
    version: "3.2.0"
    httpPort: 9200
    vendor: opensearch
    serviceName: my-cluster
    pluginsList:
      - "repository-s3"
      # A plugin zip URL must match the OpenSearch version exactly
      - "https://github.com/opensearch-project/opensearch-prometheus-exporter/releases/download/3.2.0.0/prometheus-exporter-3.2.0.0.zip"
```

To install a plugin for opensearch dashboards add it to the list under `dashboards.pluginsList`:

```yaml
spec:
  dashboards:
    enable: true
    pluginsList:
      - sample-plugin-name
```

To install a plugin for the bootstrap pod add it to the list under `bootstrap.pluginsList`:

```yaml
spec:
  bootstrap:
    pluginsList: ["repository-s3"]
```

Please note:

- [Bundled plugins](https://opensearch.org/docs/latest/install-and-configure/install-opensearch/plugins/#bundled-plugins) do not have to be added to the list, they are installed automatically
- You can provide either a plugin name or a complete URL to the plugin zip. The items you provide are passed to the `bin/opensearch-plugin install <plugin-name>` command.
- Updating the list for an already installed cluster will lead to a rolling restart of all opensearch nodes to install the new plugin.
- If your plugin requires additional configuration you must provide that either through `additionalConfig` (see section [Configuring opensearch.yml](#configuring-opensearchyml)) or as secrets in the opensearch keystore (see section [Add secrets to keystore](#add-secrets-to-keystore)).

### Add secrets to keystore

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

To populate the keystore of the bootstrap pod add the secrets under the `bootstrap.keystore` section:

```yaml
bootstrap:
  # ...
  keystore:
    - secret:
        name: credentials
    - secret:
        name: some-other-secret
```

### SmartScaler

SmartScaler lets the operator remove data nodes safely when you lower `replicas` of a node pool or remove a pool. Before a node is removed, the operator adds it to `cluster.routing.allocation.exclude._name`, so OpenSearch moves its shards to the remaining nodes. Only after the node holds no more shards does the operator delete the pod, and it then removes the node from the exclusion list again.

SmartScaler is enabled by default (`spec.confMgmt.smartScaler: true`) for newly created clusters, whether or not the `confMgmt` block is present in the manifest. Setting it to `false` removes nodes without draining, and the operator emits a `Warning` event each time it does so. Clusters created with operator 2.x may have `smartScaler: false` stored; see [Other Changes When Upgrading from 2.x](./migration-guide.md#other-changes-when-upgrading-from-2x).

The other `confMgmt` fields, `autoScaler` and `VerUpdate`, are not used by the operator.

#### Removing master-eligible nodes

Master-eligible nodes (`master` / `cluster_manager` role) are removed one at a time, regardless of the SmartScaler setting. The operator follows the procedure OpenSearch documents for shrinking the voting configuration (`/_cluster/voting_config_exclusions`): it adds the node to the voting configuration exclusions, removes the pod once the node has left the voting configuration, and clears the exclusions when the node has left the cluster. The same applies when a whole master-eligible node pool is removed from `spec.nodePools`, and when the bootstrap pod is removed after initialization. With SmartScaler off only this voting step is performed; shards are not drained.

The operator also clears exclusions that are left over (for example after an operator restart during a removal), and emits a `Warning` event if it finds a live node that was left excluded. The webhook rejects clusters without at least one master-eligible replica (see [webhooks](webhooks.md)).

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

If `jvm` does not contain `-Xms` or `-Xmx`, the operator appends a heap setting of half of
`resources.requests.memory` (the recommended value for data nodes) to whatever `jvm` contains, or
`-Xms512M -Xmx512M` when no memory request is set. For example `jvm: "-XX:+UseG1GC"` still gets the automatic heap size.

### Deal with `max virtual memory areas vm.max_map_count` errors

OpenSearch requires the Linux kernel `vm.max_map_count` option [to be set to at least 262144](https://opensearch.org/docs/latest/install-and-configure/install-opensearch/index/#important-settings). The operator sets this option as 262144 in default using an init container for each opensearch pod. If you already set this option yourself on the Kubernetes hosts using sysctl and don't want to change it by the operator again, you can disable by adding the following option to your cluster spec:

```yaml
spec:
  general:
    setVMMaxMapCount: false
```

By default the init container uses a busybox image. If you want to change that (for example to use an image from a private registry), see [Custom init helper](#custom-init-helper).

Clusters created with operator 2.x may have lost an explicit `setVMMaxMapCount: false`; see [Other Changes When Upgrading from 2.x](./migration-guide.md#other-changes-when-upgrading-from-2x).

### Configuring Snapshot Repositories

You can configure the snapshot repositories for the OpenSearch cluster through the operator. Using `general.snapshotRepositories` settings you can configure multiple snapshot repositories. The operator registers them once the cluster is `RUNNING`, and updates them when their settings change. Removing a repository from the spec does not unregister it from OpenSearch. Once a repository exists you can take snapshots with an [OpensearchSnapshotPolicy](#managing-snapshot-policies-with-kubernetes-resources).

```yaml
spec:
  general:
    snapshotRepositories:
      - name: my_s3_repository_1
        type: s3
        settings:
          bucket: opensearch-s3-snapshot
          region: us-east-1
          base_path: os-snapshot
      - name: my_s3_repository_3
        type: s3
        settings:
          bucket: opensearch-s3-snapshot
          region: us-east-1
          base_path: os-snapshot_1
```

#### Prerequisites for Configuring Snapshot Repo

Before configuring `snapshotRepositories` for a cluster, please ensure the following prerequisites are met:

1. The right cloud provider native plugins are installed. For example:

   ```yaml
   spec:
     general:
       pluginsList: ["repository-s3"]
   ```

2. The required roles/permissions for the backend cloud are pre-created. An example AWS IAM role added for kubernetes nodes so that snapshots can be published to the `opensearch-s3-snapshot` s3 bucket:

   ```json
   {
     "Statement": [
       {
         "Action": [
           "s3:ListBucket",
           "s3:GetBucketLocation",
           "s3:ListBucketMultipartUploads",
           "s3:ListBucketVersions"
         ],
         "Effect": "Allow",
         "Resource": ["arn:aws:s3:::opensearch-s3-snapshot"]
       },
       {
         "Action": [
           "s3:GetObject",
           "s3:PutObject",
           "s3:DeleteObject",
           "s3:AbortMultipartUpload",
           "s3:ListMultipartUploadParts"
         ],
         "Effect": "Allow",
         "Resource": ["arn:aws:s3:::opensearch-s3-snapshot/*"]
       }
     ],
     "Version": "2012-10-17"
   }
   ```

## Configuring Dashboards

The operator can automatically deploy and manage a OpenSearch Dashboards instance. To do so add the following section to your cluster spec:

```yaml
# ...
spec:
  dashboards:
    enable: true # Set this to true to enable the Dashboards deployment
    version: "3.2.0" # Optional, defaults to general.version. Should match the OpenSearch version
    replicas: 1 # The number of replicas to deploy
    # Optional scheduling settings, same format as for node pools
    nodeSelector: {}
    tolerations: []
    topologySpreadConstraints: []
    priorityClassName: ""
    service:
      labels: # Optional, extra labels for the Dashboards Service
        team: search
```

See the [CRD reference](../designs/crd/opensearch.org.md) for all Dashboards fields.

### Configuring opensearch_dashboards.yml

You can customize the OpenSearch Dashboards configuration ([`opensearch_dashboards.yml`](https://github.com/opensearch-project/OpenSearch-Dashboards/blob/main/config/opensearch_dashboards.yml)) using the `additionalConfig` field in the dashboards section of the `OpenSearchCluster` custom resource:

```yaml
apiVersion: opensearch.org/v1
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

When using OpenSearch behind a reverse proxy on a subpath (e.g. `/logs`) you have to configure a base path. This can be achieved by setting the base path field in the configuration of OpenSearch Dashboards. Behind the scenes the correct configuration options are automatically added to the dashboards configuration.

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
# ...
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
    enable: true # Deploy Dashboards component
    tls:
      enable: true # Configure TLS
      generate: true # Have the Operator generate and sign a certificate
      # How long generated certificates are valid (default: 8760h = 1 year)
      duration: "8760h"
      secret:
        name: # Name of the secret that contains the provided certificate
      caSecret:
        name: # Name of the secret that contains a CA the Operator should use
# ...
```

To let the Operator generate the certificate, just set `tls.enable: true` and `tls.generate: true` (the other fields under `tls` can be omitted). Again, as with the node certificates, you can supply your own CA via `caSecret.name` for the Operator to use.
If you want to use your own certificate, you need to provide it as a Kubernetes TLS secret (with fields `tls.key` and `tls.crt`) and provide the name as `secret.name`.

If you want to expose Dashboards outside of the cluster, it is recommended to use Operator-generated certificates internally and let an Ingress present a valid certificate from an accredited CA (e.g. LetsEncrypt).

## Customizing the kubernetes deployment

Besides configuring OpenSearch itself, the operator also allows you to customize how the operator deploys the opensearch and dashboards pods.

### Data Persistence

By default, the Operator will create OpenSearch node pools with persistent storage from the default [Storage Class](https://kubernetes.io/docs/concepts/storage/storage-classes/). This behaviour can be changed per node pool. You may supply an alternative storage class and access mode, or configure hostPath or emptyDir storage.

PVCs of the node pools are kept when a pool is scaled down or the cluster is deleted, unless you set a [StatefulSet PVC retention policy](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/#persistentvolumeclaim-retention) for all pools:

```yaml
spec:
  general:
    persistentVolumeClaimRetentionPolicy:
      whenDeleted: Delete # Delete the PVCs when the cluster is deleted (default: Retain)
      whenScaled: Retain # Keep the PVCs of removed replicas (default: Retain)
```

- If you re-create a cluster with the same name over retained PVCs, the operator skips the bootstrap: the nodes re-form the old cluster from disk. Delete the PVCs first if you want a fresh cluster.
- The webhook rejects changes to a pool's `storageClass`, because StatefulSet volume claim templates are immutable.
- `diskSize` can be increased (see [Volume Expansion](#volume-expansion)) but not decreased.

The available storage options are:

#### PVC

The default option is persistent storage via PVCs. You can explicitly define the `storageClass`, the `annotations` and the `labels` if needed. When you set `persistence.pvc`, `accessModes` is required:

```yaml
spec:
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "30Gi"
      roles:
        - "data"
        - "master"
      persistence:
        pvc:
          storageClass: mystorageclass # Set the name of the storage class to be used
          accessModes: # Required when persistence.pvc is set
            - ReadWriteOnce
          annotations: # You can add annotations
            test.io/crypt-key-id: "your-kms-key-id"
          labels: # You can add labels
            team: "backend-data"
```

#### EmptyDir

If you do not want to use persistent storage you can use the `emptyDir` option. Beware that this can lead to data loss, so you should only use this option for testing, or for data that is otherwise persisted.

```yaml
spec:
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "30Gi"
      roles:
        - "data"
        - "master"
      persistence:
        emptyDir: {} # This configures emptyDir
```

If you are using emptyDir, it is recommended that you set `spec.general.drainDataNodes` to be `true`. This will ensure that shards are drained from the pods before rolling upgrades or restart operations are performed.

#### HostPath

As a last option you can use a `hostPath`. Please note that hostPath is strongly discouraged. By default, the operator applies pod anti-affinity to prevent multiple pods from scheduling on the same node, which helps when using hostPath. However, if you need stricter control, you can configure explicit affinity rules for the node pool to ensure that multiple pods do not schedule to the same Kubernetes host.

```yaml
spec:
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "30Gi"
      roles:
        - "data"
        - "master"
      persistence:
        hostPath:
          path: "/var/opensearch" # Define the path on the host here
```

### Security Context for pods and containers

You can set the security context for the Opensearch pods and the Dashboard pod. This is useful when you want to define privilege and access control settings for a Pod or Container. To specify security settings for Pods, include the `podSecurityContext` field and for containers, include the `securityContext` field.

The structure is the same for both Opensearch pods (in `spec.general`) and the Dashboard pod (in `spec.dashboards`):

```yaml
spec:
  general:
    podSecurityContext:
      runAsUser: 1000
      runAsGroup: 1000
      runAsNonRoot: true
    securityContext:
      allowPrivilegeEscalation: false
      privileged: false
  dashboards:
    podSecurityContext:
      fsGroup: 1000
      runAsNonRoot: true
    securityContext:
      capabilities:
        drop:
          - ALL
      privileged: false
```

The Opensearch pods by default launch an init container to configure the volume. This container needs to run with root permissions (`runAsUser: 0`) and does not inherit `general.securityContext`; configure it via `initHelper.securityContext` instead (see [Custom init container security context](#custom-init-container-security-context)). If your k8s environment does not allow containers with the root user you need to [disable this init helper](#disabling-the-init-helper). In this situation also make sure to set `general.setVMMaxMapCount` to `false` as this feature also launches a privileged init container.

Note that the bootstrap pod started during initial cluster setup uses the same (pod)securityContext as the Opensearch pods, and the same `initHelper.securityContext` for its init containers.

### Bootstrap pod

During initial cluster formation the operator runs a single bootstrap pod (`<cluster-name>-bootstrap-0`) and removes it once the cluster is initialized. It uses a PVC to keep cluster state across restarts during initialization; the PVC is created and deleted along with the pod. No bootstrap pod is started for an `OpenSearchCluster` that is re-created over retained data PVCs of a previous cluster with the same name (see [Data Persistence](#data-persistence)).

The bootstrap pod is configured in `spec.bootstrap`:

```yaml
spec:
  bootstrap:
    resources: # Also used to compute the heap size when jvm has no -Xms/-Xmx
      requests:
        memory: "2Gi"
        cpu: "500m"
    jvm: "-Xms1g -Xmx1g"
    diskSize: "1Gi" # Default 1Gi
    storageClass: mystorageclass # Optional, defaults to the cluster default storage class
    nodeSelector:
      disktype: ssd
```

`spec.bootstrap` also accepts `tolerations`, `affinity` (see [Pod Affinity](#pod-affinity)), `priorityClassName`, `labels`, `annotations`, `env`, `hostAliases`, `pluginsList`, `keystore` and `initContainers`. It uses the same image and (pod) security context as the node pools.

### Host Aliases for pods and containers

You can add entries to Opensearch, Bootstrap and Dashboard pods /etc/hosts files using [HostAliases](https://kubernetes.io/docs/concepts/services-networking/add-entries-to-pod-etc-hosts-with-host-aliases/).

The structure is the same for both Opensearch pods (in `spec.general`) and the Dashboard pod (in `spec.dashboards`):

```yaml
spec:
  general:
    hostAliases:
    - hostnames:
      - example.com
      ip: 127.0.0.1
  dashboards:
    hostAliases:
    - hostnames:
      - example.com
      ip: 127.0.0.1
  bootstrap:
    hostAliases:
    - hostnames:
      - example.com
      ip: 127.0.0.1
```

By default, the bootstrap pods will have the same hostAliases set as the Opensearch pods. To overwrite this, set the hostAliases in the bootstrap section.

### Labels or Annotations on OpenSearch nodes

You can add additional labels or annotations on the nodepool configuration. This is useful for integration with other applications such as a service mesh, or configuring a prometheus scrape endpoint:

Annotations set in `spec.general.annotations` are added to the cluster's Services only (the main cluster Service and each node pool's headless Service), not to pods.

```yaml
spec:
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "5Gi"
      labels: # Add any extra labels as key-value pairs here
        someLabelKey: someLabelValue
      annotations: # Add any extra annotations as key-value pairs here
        someAnnotationKey: someAnnotationValue
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

Node pool labels and annotations are added to the pool's StatefulSet and pods. Node pool annotations are also added to the pool's headless Service.

### Add Labels or Annotations to the Dashboard Deployment

You can add labels or annotations to the dashboard pod specification. This is helpful if you want the dashboard to be part of a service mesh or integrate with other applications that rely on labels or annotations.

```yaml
spec:
  dashboards:
    enable: true
    replicas: 1
    labels: # Add any extra labels as key-value pairs here
      someLabelKey: someLabelValue
    annotations: # Add any extra annotations as key-value pairs here
      someAnnotationKey: someAnnotationValue
```

Labels and annotations are added to the Dashboards Deployment and pods. The annotations are also added to the Dashboards Service; use `dashboards.service.labels` for Service labels.

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

### Pod Affinity

By default, the operator applies pod anti-affinity rules to prevent multiple pods from the same OpenSearch cluster from being scheduled on the same node. This improves high availability by reducing the risk of multiple pods being affected by a single node failure.

The default anti-affinity uses `PreferredDuringSchedulingIgnoredDuringExecution`, which is a soft preference that won't prevent scheduling if no other nodes are available, but will prefer to spread pods across nodes.

You can override this default behavior by explicitly setting the `affinity` field in your node pool, bootstrap, or dashboards configuration:

```yaml
spec:
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "30Gi"
      roles:
        - "master"
        - "data"
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchLabels:
                  opensearch.org/opensearch-cluster: my-cluster
              topologyKey: kubernetes.io/hostname
  bootstrap:
    affinity:
      podAntiAffinity:
        preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchLabels:
                  opensearch.org/opensearch-cluster: my-cluster
              topologyKey: kubernetes.io/hostname
  dashboards:
    enable: true
    affinity:
      podAffinity:
        preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchLabels:
                  app: opensearch-dashboards
              topologyKey: kubernetes.io/zone
```

If you set an explicit `affinity`, it will completely replace the default anti-affinity behavior. To disable anti-affinity entirely, you can set `affinity: {}`.

### Shard allocation awareness from node labels

OpenSearch can spread shard copies across failure domains such as zones or racks using [shard allocation awareness](https://opensearch.org/docs/latest/tuning-your-cluster/availability-and-recovery/configuring-allocation-awareness/). This requires every node to advertise its own location through a `node.attr.<attribute>` setting. Kubernetes does not expose node labels to pods through the Downward API, so the operator can populate these attributes from node labels at runtime using `spec.general.nodeAttributes`:

```yaml
spec:
  general:
    nodeAttributes:
      - name: zone
        nodeLabel: topology.kubernetes.io/zone
    additionalConfig:
      cluster.routing.allocation.awareness.attributes: zone
      cluster.routing.allocation.awareness.force.zone.values: dc1,dc2
```

For each entry the operator injects an init container that reads the given label off the node hosting the pod (via the Kubernetes API) and writes its value into a small file on a shared volume. The OpenSearch container sources that file on startup, so `node.attr.zone` resolves to the zone of the node the pod actually landed on. The example above yields `node.attr.zone: dc1` (or `dc2`) per pod, which the awareness settings then use to balance and force shard copies across both data centers.

You can map several attributes at once (for example a `zone` and a `rack`), and combine this with `topologySpreadConstraints` to also control pod placement:

```yaml
spec:
  nodePools:
    - component: data
      replicas: 6
      roles:
        - "data"
      topologySpreadConstraints:
        - maxSkew: 1
          topologyKey: topology.kubernetes.io/zone
          whenUnsatisfiable: ScheduleAnyway
          labelSelector:
            matchLabels:
              opensearch.org/opensearch-nodepool: data
```

> **Note:** The init container queries the Kubernetes API for the node object, so the pods'
`ServiceAccount` must be allowed to `get` `nodes`. By default (no `spec.general.serviceAccount`
set) the operator handles this for you: it creates a dedicated `ServiceAccount` for the
cluster and binds it to the shared `opensearch-node-attributes` `ClusterRole` shipped by
the operator's Helm chart — no manual RBAC required. If you set a custom `spec.general.serviceAccount`,
the operator leaves RBAC to you and you must grant that account `get` on `nodes` yourself
(see the [example manifest](../../opensearch-operator/examples/opensearch-zone-awareness.yaml)).
Automatic RBAC management requires a cluster-scoped installation (the Helm chart must be
installed with `useRoleBindings=false`). Node label values must not contain `,`, `{` or `}`.

### Sidecar Containers

You can deploy additional sidecar containers alongside OpenSearch in the same pod. This is useful for log shipping, monitoring agents, or other auxiliary services that need to run alongside OpenSearch nodes.

```yaml
spec:
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "30Gi"
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
      sidecarContainers:
        - name: log-shipper
          image: fluent/fluent-bit:latest
          resources:
            requests:
              memory: "64Mi"
              cpu: "100m"
            limits:
              memory: "128Mi"
              cpu: "200m"
          volumeMounts:
            - name: varlog
              mountPath: /var/log
        - name: monitoring-agent
          image: prometheus/node-exporter:latest
          ports:
            - containerPort: 9100
              name: metrics
```

Sidecar containers share the same network namespace and storage volumes as the OpenSearch container as they are on the same pod.

### Additional Volumes

Sometimes it is necessary to mount ConfigMaps, Secrets, emptyDir, projected volumes, CSI volumes, NFS volumes, or hostPath volumes into the Opensearch pods as volumes to provide additional configuration (e.g. plugin config files). This can be achieved by providing an array of additional volumes to mount to the custom resource. This option is located in either `spec.general.additionalVolumes` or `spec.dashboards.additionalVolumes`. The format is as follows:

```yaml
spec:
  general:
    additionalVolumes:
      - name: example-configmap
        path: /path/to/mount/volume
        #subPath: mykey # Add this to mount only a specific key of the configmap/secret
        configMap:
          name: config-map-name
        restartPods: true # Restart the OpenSearch pods when the content of the configMap changes
      - name: temp
        path: /tmp
        emptyDir: {}
      - name: example-csi-volume
        path: /path/to/mount/volume
        #subPath: "subpath" # Add this to mount the CSI volume at a specific subpath
        csi:
          driver: csi-driver-name
          readOnly: true
          volumeAttributes:
            secretProviderClass: example-secret-provider-class
      - name: example-projected-volume
        path: /path/to/mount/volume
        projected:
          sources:
            - serviceAccountToken:
                path: "token"
      - name: example-persistentvolumeclaim-volume
        path: /path/to/mount/volume
        persistentVolumeClaim:
          claimName: claim-name
  dashboards:
    additionalVolumes:
      - name: example-secret
        path: /path/to/mount/volume
        secret:
          secretName: secret-name
```

- Exactly one volume source must be set per entry.
- `subPath` is supported for configMap, secret, CSI and projected volumes.
- ConfigMap, secret and projected volumes are mounted read-only. CSI, PVC and NFS volumes follow their `readOnly` field (CSI defaults to read-only); emptyDir and hostPath volumes are writable.
- `restartPods` only works for configMap and secret volumes on the OpenSearch pods: the operator restarts the pods when the content changes. It has no effect in `spec.dashboards.additionalVolumes`.

#### NFS Volume Support

NFS volumes can be mounted directly into OpenSearch pods without requiring external provisioners or CSI drivers. This is particularly useful for snapshot repositories stored on NFS shares. To configure an NFS volume, specify the `nfs` field with the required `server` and `path` parameters:

```yaml
spec:
  general:
    additionalVolumes:
      - name: nfs-backups
        path: /mnt/backups/opensearch
        nfs:
          server: 192.168.1.233
          path: /export/backups/opensearch
          readOnly: false # false gives a writable mount
```

Operator 3.0.0 and earlier mount NFS volumes read-only regardless of this field.

This can be combined with snapshot repository configuration:

```yaml
spec:
  general:
    additionalVolumes:
      - name: nfs-backups
        path: /mnt/backups/opensearch
        nfs:
          server: 192.168.1.233
          path: /export/backups/opensearch
          readOnly: false
    snapshotRepositories:
      - name: nfs-repository
        type: fs
        settings:
          location: /mnt/backups/opensearch
```

#### HostPath Volume Support

HostPath volumes allow you to mount a file or directory from the host node's filesystem into your OpenSearch pods. This is useful for accessing host-specific data, but should be used with caution as it can create security and portability issues.

> **Warning:** HostPath volumes are strongly discouraged in production environments as they:
> - Create security risks by allowing pods to access the host filesystem
> - Reduce portability across different nodes
> - Can cause issues if pods are scheduled on different nodes
>
> Consider using PersistentVolumeClaims, NFS, or other network storage solutions instead.

To configure a hostPath volume, specify the `hostPath` field with the required `path` parameter:

```yaml
spec:
  general:
    additionalVolumes:
      - name: hostpath-data
        path: /host/data
        hostPath:
          path: /var/lib/opensearch
          type: DirectoryOrCreate # Optional, defaults to empty string
```

The `type` field is optional and can be one of:
- `Directory` - Directory must exist on the host
- `DirectoryOrCreate` - Directory will be created if it doesn't exist
- `File` - File must exist on the host
- `FileOrCreate` - File will be created if it doesn't exist
- `Socket` - Unix socket must exist on the host
- `CharDevice` - Character device must exist on the host
- `BlockDevice` - Block device must exist on the host

If `type` is not specified, the path must exist and be of the correct type.

> **Note:** When using hostPath volumes, ensure proper pod anti-affinity rules are configured to prevent multiple pods from scheduling on the same node, which could cause data conflicts.

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

The operator adds init containers to the OpenSearch and bootstrap pods (the `init` container that `chown`s the data directory and, with `setVMMaxMapCount`, the `init-sysctl` container). They run on every pod start and use a busybox image, `docker.io/busybox:latest` by default, or `<general.defaultRepo>/busybox:latest` when `defaultRepo` is set. In case you are working in an offline environment and the cluster cannot access the registry or you want to customize the image, you can override the image used by specifying the `initHelper` image in your cluster spec:

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

### Edit init container resources

The init helper containers run without resource requests or limits by default. Set `initHelper.resources` to give them requests and limits (for example when a namespace `ResourceQuota` requires them):

```yaml
spec:
  initHelper:
    resources:
      requests:
        memory: "128Mi"
        cpu: "250m"
      limits:
        memory: "512Mi"
        cpu: "500m"
```

### Custom init container security context

By default the chown init container runs with `runAsUser: 0` and the sysctl init container (enabled via `general.setVMMaxMapCount`) runs with `privileged: true`. If your cluster enforces admission policies that require additional security context fields (for example a seccomp profile or dropped capabilities), you can replace the security context of these init containers:

```yaml
spec:
  initHelper:
    securityContext:
      runAsUser: 0
      privileged: true  # required when general.setVMMaxMapCount is enabled
      seccompProfile:
        type: RuntimeDefault
      capabilities:
        drop:
          - ALL
        add:
          - CHOWN
```

Note that this replaces the defaults entirely for all init helper containers (chown and sysctl share one context): the chown init container still needs to run as root, and the sysctl init container still needs privileged access, so make sure your custom security context grants the required permissions for both.

### Disabling the init helper

In some cases, you may want to avoid the `chown` init container (e.g. on OpenShift or if your cluster blocks containers running as `root`).
It can be disabled by adding the following to the operator's Helm `values.yaml`. This skips only the `chown` init container; `init-sysctl` is controlled by `general.setVMMaxMapCount`:

```yaml
manager:
  extraEnv:
    - name: SKIP_INIT_CONTAINER
      value: "true"
```

### Custom OpenSearch Path

By default, the operator assumes OpenSearch is installed at `/usr/share/opensearch` inside the container (and `/usr/share/opensearch-dashboards` for Dashboards). If you use a custom OpenSearch image with a different installation directory, you can override these paths:

```yaml
spec:
  general:
    opensearchHome: "/opt/opensearch"
  dashboards:
    opensearchDashboardsHome: "/opt/opensearch-dashboards"
```

The operator uses these paths for all volume mounts (data, config, TLS certificates, keystore, security plugin) and init container commands. When not set, the defaults are used. Any trailing slashes in the provided path are automatically removed.

### PodDisruptionBudget

The PDB (Pod Disruption Budget) is a Kubernetes resource that helps ensure the high availability of applications by defining the acceptable disruption level during maintenance or unexpected events.
It specifies the minimum number of pods that must remain available to maintain the desired level of service.
The PDB definition is unique for every nodePool.
You must provide either `minAvailable` or `maxUnavailable` to configure PDB, but not both.

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
# ...
spec:
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "30Gi"
      pdb:
        enable: true
        minAvailable: 3
    - component: datas
      replicas: 7
      diskSize: "100Gi"
      pdb:
        enable: true
        maxUnavailable: 2
```

### Exposing OpenSearch Dashboards

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
                name: my-cluster-dashboards # <general.serviceName>-dashboards
                port:
                  number: 5601
            path: "/(.*)"
            pathType: ImplementationSpecific
```

The Dashboards Service is named `<general.serviceName>-dashboards`. If you have enabled HTTPS for dashboards you need to instruct your ingress-controller to use a HTTPS connection internally. This is specific for the controller you are using (e.g. nginx-ingress, traefik, ...).

### Configuring the Dashboards K8s Service

You can customize the Kubernetes Service object that the operator generates for the Dashboards deployment.

Supported Service Types

- ClusterIP (default)
- NodePort
- LoadBalancer

When using type LoadBalancer you can optionally set the load balancer source ranges.

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
# ...
spec:
  dashboards:
    service:
      type: LoadBalancer # Set one of the supported types
      loadBalancerSourceRanges: # Optional, add source ranges for a loadbalancer
        - "10.0.0.0/24"
        - "192.168.0.0/24"
```

### Exposing the OpenSearch cluster REST API

If you want to expose the REST API of OpenSearch outside your Kubernetes cluster, the recommended way is to do this via ingress.
Internally you should use self-signed certificates (you can let the operator generate them), and then let the ingress use a certificate from an accepted CA (for example LetsEncrypt or a company-internal CA). That way you do not have the hassle of supplying custom certificates to the opensearch cluster but your users still see valid certificates.

### Customizing probe timeouts and thresholds

If the nodes take longer to start than the probes allow and get restarted, you can configure the probe timeouts and thresholds per node pool:

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
# ...
spec:
  nodePools:
    - component: masters
      replicas: 3
      diskSize: "30Gi"
      probes:
        liveness:
          initialDelaySeconds: 10
          periodSeconds: 20
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 10
        startup:
          initialDelaySeconds: 10
          periodSeconds: 20
          timeoutSeconds: 5
          successThreshold: 1
          failureThreshold: 10
        readiness:
          initialDelaySeconds: 60
          periodSeconds: 30
          timeoutSeconds: 30
          successThreshold: 1
          failureThreshold: 5
```

### Customize startup and readiness probe command

While liveness probe is a TCP check the startup and readiness probes use the OpenSearch API with curl.

If you need to customize the startup or readiness probe commands you can override it as shown below:

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
# ...
spec:
  nodePools:
    - component: masters
      # ...
      probes:
        startup:
          command:
            - echo
            - "Hello, World!"
        readiness:
          command:
            - echo
            - "Hello, World!"
```

### Configuring Resource Limits/Requests

In addition to the information provided in the previous sections on how to specify resource requirements for the node pools, it is also possible to specify resources for all entities created by the operator for more advanced use cases.

Resources for the init helper containers are set with `initHelper.resources` (see [Edit init container resources](#edit-init-container-resources)).

You can also configure the resources and scheduling options for the security update job as shown below.

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
# ...
spec:
  security:
    config:
      updateJob:
        resources:
          limits:
            cpu: "100m"
            memory: "100Mi"
          requests:
            cpu: "100m"
            memory: "100Mi"
```

The `updateJob` also supports standard Kubernetes scheduling options: `nodeSelector`, `tolerations`, `affinity`, `labels`, and `priorityClassName`.

Please note that the examples provided here do not reflect actual resource requirements. You may need to conduct further testing to properly adjust the resources based on your specific needs.

### ReadOnlyRootFilesystem: Enhancing Container Security

The `readOnlyRootFilesystem` security context setting prevents runtime modifications to the container's filesystem, significantly improving security by reducing the attack surface. This section explains how to configure OpenSearch clusters with this security feature.

#### Configuration Overview

To enable `readOnlyRootFilesystem`, you need to:

1. Set `readOnlyRootFilesystem: true` in `general.securityContext`
2. Configure writable volumes using `emptyDir` in the `general` section
3. Add initialization containers to copy necessary files before the main container starts

#### Step 1: Configure Writable Volumes

Add the following `emptyDir` volumes to provide writable paths for OpenSearch:

```yaml
spec:
  general:
    securityContext:
      readOnlyRootFilesystem: true
    additionalVolumes:
      - emptyDir: {}
        name: rw-tmp
        path: /tmp
      - emptyDir: {}
        name: rw-config
        path: /usr/share/opensearch/config
      - emptyDir: {}
        name: rw-plugins
        path: /usr/share/opensearch/plugins
      - emptyDir: {}
        name: rw-logs
        path: /usr/share/opensearch/logs
```

#### Step 2: Add Initialization Containers

The operator mounts the volumes specified in `additionalVolumes` before any other volumes. To prevent issues with empty directories in `config` and `plugins`, add initialization containers to copy the necessary files.

> **Note:** The operator ensures these initialization containers run first in the initialization sequence, before any other init containers you may have defined.

##### For the bootstrap section:

```yaml
spec:
  bootstrap:
    initContainers:
      - name: init-copier
        image: opensearchproject/opensearch:3.2.0 # Same image as the cluster
        volumeMounts:
          - name: rw-config
            mountPath: /config-tmp
          - name: rw-plugins
            mountPath: /plugins-tmp
        command:
          - "bash"
          - "-c"
          - "cp -r /usr/share/opensearch/plugins/* /plugins-tmp && cp -r /usr/share/opensearch/config/* /config-tmp"
```

##### For the nodePool section:

When using `readOnlyRootFilesystem`, it's recommended to install plugins in the nodePool's initialization container:

```yaml
spec:
  nodePools:
    - component: nodes
      # ...
      initContainers:
        - name: init-copier
          image: opensearchproject/opensearch:3.2.0 # Same image as the cluster
          volumeMounts:
            - name: rw-config
              mountPath: /config-tmp
            - name: rw-plugins
              mountPath: /plugins-tmp
          command:
            - "bash"
            - "-c"
            - "bin/opensearch-plugin -v install --batch repository-s3 && cp -r /usr/share/opensearch/plugins/* /plugins-tmp && cp -r /usr/share/opensearch/config/* /config-tmp"
```

## Cluster operations

The operator contains several features that automate management tasks that might be needed during the cluster lifecycle. The different available options are documented here.

### Cluster recovery

The operator uses `Parallel` pod management for StatefulSets by default, so pods are started in parallel. This allows the cluster to form quorum and recover when multiple pods are missing or crashed, without a separate recovery mode.

Scaling, rolling restarts, and version upgrades remain sequenced by the operator (one replica / one node at a time). `Parallel` only removes the StatefulSet controller's readiness gate on pod creation and recreation.

`podManagementPolicy` is immutable on StatefulSets. After upgrading to an operator version that sets `Parallel`, existing node-pool StatefulSets are recreated once via orphan delete (pods are retained and re-adopted). This is a one-time, fleet-wide STS recreate that normally converges within one or two reconciles.

If the cluster is using emptyDir i.e. every node pool is using emptyDir, the operator starts recovery in case of these failure scenarios:

1. More than half the master nodes have lost their original emptyDir data and thus, the quorum is broken.
2. All data nodes have lost their original emptyDir data and thus, no data node is available.

"Lost emptyDir data" means the pod that held the volume is gone — not merely NotReady. An emptyDir lives exactly as long as its pod UID: container restarts and readiness blips keep the same UID and the volume, while force-delete, eviction, or node loss causes the StatefulSet to recreate the pod with the same name, a new UID, and a fresh empty directory. The operator records the UID of each Ready pod and treats a NotReady pod with a different UID as missing for this check.

Recovery waits for a 5 minute grace period after the condition is first observed, then deletes and recreates the StatefulSets (and related resources), sets `status.initialized` to `false`, and re-bootstraps the cluster. Because the cluster is using emptyDir, the previous data is not recoverable.

Until each pod has been observed Ready at least once after this tracking is enabled (for example right after an operator upgrade), a NotReady pod with no recorded UID is treated as missing only when its `creationTimestamp` is newer than the 5 minute grace period (a freshly recreated pod). A longer-lived NotReady pod with no record is still counted as existing, so upgrading the operator while nodes are crash-looping does not wipe an intact emptyDir cluster.

### Rolling Upgrades

The operator supports automatic rolling version upgrades. To do so simply change the `general.version` in your cluster spec and reapply it:

```yaml
spec:
  general:
    version: "3.2.0"
```

The Operator will then perform a rolling upgrade and restart the nodes one-by-one, waiting after each node for the cluster to stabilize and have a green cluster status. Depending on the number of nodes and the size of the data stored this can take some time.
If the cluster stays yellow because some replicas can never be assigned (e.g. `number_of_replicas` is higher than the number of other data nodes), rolling restarts and upgrades with `drainDataNodes: false` still continue once all data nodes have joined and no shards are initializing, relocating or waiting for delayed allocation. The operator never restarts a node that holds the only active copy of a shard that should have replicas, except in a cluster with a single data node, where there is nowhere else to keep a copy. With `drainDataNodes: true` a node is only restarted once its shards have moved elsewhere, so replicas that have nowhere to go still block the restart.
With `drainDataNodes: true` and exactly two data nodes, the operator only waits for primaries of system indices (such as `.opendistro_security`) to move off a node before restarting it, since the other shards have nowhere to go.

Node pools are upgraded one at a time: data-only pools first, then pools that are both data and master-eligible, then all other pools (for example dedicated cluster managers and coordinating nodes).

The webhook rejects version changes that the operator cannot perform:

- downgrades,
- upgrades that span more than one major version,
- changing `general.version` while `general.image` pins a custom image, unless the image changes in the same update (the version does not select the image then).

If you are using emptyDir storage for data nodes, it is recommended to set `general.drainDataNodes` to `true`, otherwise you might lose data.

### Configuration changes

As explained in the section [Configuring opensearch.yml](#configuring-opensearchyml) you can add extra opensearch configuration to your cluster. Changing this configuration on an already installed cluster will be detected by the operator and it will do a rolling restart of all cluster nodes to apply that new configuration. The same goes for nodepool-specific configuration like `resources`, `annotation` or `labels`.

### Volume Expansion

If your underlying storage supports online volume expansion the operator can orchestrate that action for you.

To increase the disk volume size set the `diskSize` of a nodepool to the desired value and re-apply the cluster spec yaml. This operation is expected to have no downtime and the cluster should be operational.

The following considerations should be taken into account in order to increase the PVC size.

- This only works for PVC-based persistence
- Before considering the expansion of the cluster disk, make sure the volumes/data is backed up in desired format, so that any failure can be tolerated by restoring from the backup.
- Make sure the cluster storage class has `allowVolumeExpansion: true` before applying the new `diskSize`. For more details checkout the [kubernetes storage classes](https://kubernetes.io/docs/concepts/storage/storage-classes/) document.
- Once the above step is done, the cluster yaml can be applied with new `diskSize` value, to all declared nodepool components or to single component.
- It is best recommended not to apply any new changes to the cluster along with volume expansion.
- Make sure the declared size definitions are proper and consistent, example if the `diskSize` is in `G` or `Gi`, make sure the same size definitions are followed for expansion.

Note: To change the `diskSize` from `G` to `Gi` or vice-versa, first make sure data is backed up and make sure the right conversion number is identified, so that the underlying volume has the same value and then re-apply the cluster yaml. This will make sure the statefulset is re-created with right value in VolumeClaimTemplates, this operation is expected to have no downtime.

### Status and events

`kubectl get opensearchcluster <name> -o yaml` shows the cluster status:

- `status.phase`: `PENDING` until the operator first reconciles the cluster, `RUNNING` afterwards, and `UPGRADING` during a version upgrade. `RUNNING` does not mean the cluster is healthy.
- `status.health` (`green`, `yellow`, `red` or `unknown`) and `status.availableNodes`, as reported by OpenSearch.
- `status.componentsStatus`: progress of ongoing operations, such as `Scaler`, `Upgrader`, `RollingRestart` and `Securityconfig` entries.

The operator records Kubernetes events on the `OpenSearchCluster` (`kubectl describe opensearchcluster <name>`). The event reasons are `RollingRestart`, `Upgrade`, `Scaler`, `PVC`, `PDB`, `StatefulSetRecreated`, `EmptyDirRecovery`, `Monitoring`, `Security`, `DashboardsUserUnmapped` and `ConfigDuplicateKey`.

A few behaviours worth knowing:

- **Stuck pods:** during a rolling restart or upgrade, a pod on an old StatefulSet revision that is stuck (`CrashLoopBackOff`, `ImagePullBackOff`, `ErrImagePull`, `InvalidImageName`, or `RepeatedlyFailing` when its container keeps failing the startup probe) is deleted so it is recreated from the new template. Stuck pods on the current revision are only reported with a `Warning` event.
- **Yellow clusters:** rolling restarts and upgrades continue on a cluster that stays yellow because some replicas can never be assigned (see [Rolling Upgrades](#rolling-upgrades)).
- **Securityconfig job:** if the `<cluster-name>-securityconfig-update` job fails, the operator retries it with an exponential backoff from 30 seconds up to 15 minutes.

## User and role management

An important part of any OpenSearch cluster is the user and role management to give users access to the cluster (via the opensearch-security plugin). The operator ships a minimal `internal_users.yml` with only the `admin` and `kibanaserver` users, both with operator-generated random passwords. All other security files (roles, role mappings, `config.yml`, ...) come from the defaults of the security plugin in the OpenSearch image. For any production installation you should replace them with your own configuration.
There are two ways to do that with the operator:

- Defining your own securityconfig
- Managing users, roles and other objects via kubernetes resources

The two can be combined, but see the caveat about `internal_users.yml` below.

### Securityconfig

You can provide your own securityconfig (see the [example securityconfig secret](../../opensearch-operator/examples/securityconfig-secret.yaml) and the [Access control documentation](https://opensearch.org/docs/latest/security-plugin/access-control/index/) of the OpenSearch project) with your own users and roles. To do that, you provide a secret with the securityconfig yaml files you want to replace.

The Operator can be controlled using the following fields in the `OpenSearchCluster` custom resource:

```yaml
# ...
spec:
  security:
    config: # Everything related to the securityconfig
      securityConfigSecret:
        name: # Name of the secret that contains the securityconfig files
      adminSecret:
        name: # Name of a secret that contains the admin client certificate
      adminCredentialsSecret:
        name: # Name of a secret that contains username/password for admin access
# ...
```

Provide the name of the secret that contains your securityconfig yaml files as `securityConfigSecret.name`. This secret is the source of truth that you manage. The operator builds its own runtime secret named `<cluster-name>-security-config-generated`: it starts from the bundled `internal_users.yml`, replaces it with any file of the same name from your secret, adds your other files, and then sets the password hashes of the `admin` and Dashboards users before the configuration is applied.

- **Password hashes:** you do not need to provide hashes for the `admin` or `kibanaserver` users. The operator generates them from the credentials secrets and overrides any hash you provide for these users, so passwords are managed in one place (the credentials secrets). If you provide your own `internal_users.yml`, it must contain an `admin` user.
- **Role mappings:** password hashes are the only thing the operator manages for you. If your secret includes its own `roles_mapping.yml`, it replaces the image's default file entirely, and you are responsible for keeping the Dashboards user mapped to a role that has cluster monitoring permissions (`kibana_server` by default). See [Custom Dashboards user](#custom-dashboards-user) below.
- **Missing files:** when the cluster is first created, files you do not provide come from the security plugin defaults in the image (see [opensearch-security](https://github.com/opensearch-project/security/tree/main/config)). To start with an empty file instead, provide a minimal one, for example:

  ```yaml
  tenants.yml: |-
    _meta:
      type: "tenants"
      config_version: 2
  ```

  Once the cluster exists, you can remove such minimal files from the secret so that later updates do not overwrite objects created via the CRDs or the REST API.
- **`internal_users.yml` is always re-applied:** because the generated secret always contains `internal_users.yml`, every run of the update job replaces all internal users, including users created via `OpensearchUser` resources or the REST API. Leaving the file out of your secret does not prevent this. The operator emits a `Warning` event whenever it re-applies the securityconfig on an initialized cluster; users managed by `OpensearchUser` resources are re-created on their next check (every 30 seconds).

In addition, you can provide the name of a secret as `adminCredentialsSecret.name` that has fields `username` and `password` for a user that the Operator can use for communicating with OpenSearch (for health checks, cluster status and node draining). When you omit this field the operator creates `<cluster-name>-admin-password` with the `admin` username and a **random password**. Either way, the operator hashes the password into the generated securityconfig without modifying your source secret, and copies the credentials into `<cluster-name>-admin-password`. See [Custom admin password](#custom-admin-password).

Similarly, for OpenSearch Dashboards, if you don't provide `dashboards.opensearchCredentialsSecret`, the operator automatically creates `<cluster-name>-dashboards-password` with a **random password** for the `kibanaserver` user and adds its hash to the generated securityconfig.

You must also configure SSL/TLS HTTP. You can either let the operator generate all needed certificates or supply them yourself.

If you provided your own certificate for SSL/TLS HTTP, then you must also provide an admin client certificate (as a Kubernetes TLS secret with fields `ca.crt`, `tls.key` and `tls.crt`) as `adminSecret.name`. The DN of the certificate must be listed under `security.tls.http.adminDn`. For clusters migrated from operator 2.x, the deprecated `security.tls.transport.adminDn` is still honored when `http.adminDn` is empty. Be advised that the `adminDn` must be defined in a way that the admin certificate cannot be used or recognized as a node certificate, otherwise OpenSearch will reject any authentication request using the admin certificate.

To apply the securityconfig to the OpenSearch cluster, the Operator uses a separate Kubernetes job (named `<cluster-name>-securityconfig-update`). This job is run during the initial provisioning of the cluster. The Operator also monitors the securityconfig and credentials secrets for changes and then reruns the update job to apply the new config. Note that the Operator only checks for changes in certain intervals, so it might take a minute or two for the changes to be applied. A failed job is retried with a backoff from 30 seconds up to 15 minutes. If the changes are not applied after a few minutes, please use 'kubectl' to check the logs of the pod of the `<cluster-name>-securityconfig-update` job. If you have an error in your configuration it will be reported there.

#### Applying the securityconfig without HTTP TLS

Clusters that enable transport TLS but disable HTTP TLS (`security.tls.http.enabled: false`) are supported, for example when TLS is terminated by a service mesh:

```yaml
# ...
spec:
  general:
    version: "3.2.0"
  security:
    tls:
      transport:
        generate: true
        perNode: true
      http:
        enabled: false
# ...
```

On OpenSearch 2.0 and later `securityadmin.sh` needs TLS on the HTTP port, so in this mode the Operator cannot run the `<cluster-name>-securityconfig-update` job. Instead it sets `plugins.security.allow_default_init_securityindex: true` and mounts the generated securityconfig into the nodes, and the security plugin loads it into the security index itself when the cluster first forms. The admin and `kibanaserver` credentials are generated and hashed into the securityconfig as usual, no admin certificate is required, and the Operator talks to the cluster over plain HTTP.

Limitations of this mode:

- **Changes to the securityconfig secret after the cluster is created are not applied automatically.** The securityconfig is only read once, when the security index is first initialized. To manage users, roles, tenants and action groups on a running cluster use the [Kubernetes resources](#managing-security-configurations-with-kubernetes-resources) or the security plugin REST API. `config.yml` (authentication backends) has no CRD equivalent, so plan for a cluster recreation or keep HTTP TLS enabled if you expect to change it later.
- There is no `<cluster-name>-securityconfig-update` job to inspect. The `Securityconfig` entry in the cluster's `status.componentsStatus` reports `securityconfig applied via default init`.

### Authenticating the operator to OpenSearch with mTLS (client certificate)

By default the operator uses HTTP basic auth (`adminCredentialsSecret`) when calling the OpenSearch REST API for tasks like health checks, ISM policy / role / user reconciliation, snapshot management, and node-draining during scale operations. You can instead authenticate the operator's runtime client using a TLS client certificate (mTLS) by setting `security.config.operatorClientCert`.

The referenced secret must be of type `kubernetes.io/tls` and contain `tls.crt` and `tls.key`. If `ca.crt` is also present it is used to verify the OpenSearch HTTP server certificate; otherwise the operator falls back to skipping verification (preserving the previous default).

#### Option A: reuse the operator-managed admin client cert (easiest)

When `security.config.adminSecret` is empty, the operator creates an admin client certificate for its own use (running `securityadmin.sh` to apply the securityconfig), signed by the operator CA. This requires HTTP TLS with generated certificates (`generate` defaults to `false`, so set it explicitly). The certificate is stored in the secret `<cluster-name>-admin-cert`, its DN is `CN=admin,OU=<cluster-name>`, and that DN is added to `plugins.security.authcz.admin_dn`, so OpenSearch accepts it as a full-privilege admin client. `adminDn` is ignored for this generated certificate. You can simply point `operatorClientCert` at it, no new certificates or mappings required:

```yaml
# ...
spec:
  security:
    config:
      operatorClientCert:
        name: dev-cluster-admin-cert   # <cluster-name>-admin-cert, auto-created by the TLS reconciler
    tls:
      http:
        generate: true
```

#### Option B: bring your own client certificate

If you manage your own PKI (e.g. cert-manager, an external CA, or `security.tls.http.generate: false`), create a `kubernetes.io/tls` secret yourself and point `operatorClientCert` at it. In this case **you** are responsible for ensuring the certificate's DN is recognized by OpenSearch — typically by listing it under `plugins.security.authcz.admin_dn`, or by configuring a `clientcert` auth domain in `config.yml` and mapping the DN to a role in `roles_mapping.yml`.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: opensearch-operator-client-cert
  namespace: opensearch
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded PEM>
  tls.key: <base64-encoded PEM>
  ca.crt:  <base64-encoded PEM>   # optional, enables TLS verification
```

```yaml
# ...
spec:
  security:
    config:
      operatorClientCert:
        name: opensearch-operator-client-cert
```

#### Optional: override the TLS ServerName

```yaml
spec:
  security:
    config:
      operatorClientServerName: dev-cluster.opensearch.svc.cluster.local
```

`operatorClientServerName` overrides the TLS SNI / `ServerName` used when verifying the OpenSearch HTTP server certificate. It defaults to the host portion of the cluster URL, so you only need to set it when `operatorClusterURL` points at a hostname that isn't in the server certificate's SANs (for example `localhost` during local debugging via `kubectl port-forward`). It only affects the operator's runtime HTTP client; it does not change how OpenSearch itself is configured.

#### Notes

- When `operatorClientCert` is set, the operator does **not** send basic-auth credentials and `adminCredentialsSecret` is no longer required for runtime API calls. (`adminCredentialsSecret` may still be useful for other purposes such as seeding the admin user password during initial securityconfig generation.)
- The operator only re-reads the secret on its next reconcile, so a cert rotation triggers a normal reconcile loop.

### Managing security configurations with kubernetes resources

The operator provides custom kubernetes resources that allow you to create/update/manage security configuration resources such as users, roles, action groups etc. as kubernetes objects.

These resources, and the ISM policy, template and snapshot policy resources further below, share some behaviour:

- They must be in the namespace of the `OpenSearchCluster`. `spec.opensearchCluster.name` is immutable, and the webhook rejects a resource whose cluster does not exist yet (create the cluster first when using GitOps).
- The operator waits until the cluster's `status.phase` is `RUNNING` before creating the object in OpenSearch.
- `metadata.name` is the name of the object in OpenSearch. ISM policies (`policyId`) and index and component templates (`name`) can set a different name in the spec, and snapshot policies always use their required `policyName`. These name fields are immutable.
- If an object with the same name already exists in OpenSearch and was not created by the operator, the resource's `status.state` is `IGNORED`, `status.existing*` is `true`, and the object is neither modified nor deleted. Users are the exception: the resource goes into the `ERROR` state instead.
- Deleting the resource deletes the object from OpenSearch (through a finalizer), unless it was pre-existing.

Fields not shown in the examples below include `attributes` and `opendistroSecurityRoles` for users; `tenantPermissions`, and `dls`, `fls` and `maskedFields` in `indexPermissions`, for roles. See the [CRD reference](../designs/crd/opensearch.org.md) for all fields.

#### Opensearch Users

It is possible to manage Opensearch users in Kubernetes with the operator. The operator will not modify users that already exist. You can create an example user as follows:

```yaml
apiVersion: opensearch.org/v1
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

The namespace of the `OpensearchUser` must be the namespace the OpenSearch cluster itself is deployed in.

Note that a secret called `sample-user-password` will need to exist in the `default` namespace with the base64 encoded password in the `password` key.

The operator re-checks each user every 30 seconds and updates the password in OpenSearch when the secret's `resourceVersion` changes.

Also, it is possible to store multiple Users password in the same Secret. To do this, you **must** create a secret where
each key will be equal to a **User name** and value is a user password. **Otherwise, changes in the secret will not trigger User
reconcile!**

#### Opensearch Roles

It is possible to manage Opensearch roles in Kubernetes with the operator. The operator will not modify roles that already exist. You can create an example role as follows:

```yaml
apiVersion: opensearch.org/v1
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

The operator allows you to link any number of users, backend roles and roles with a OpensearchUserRoleBinding. Each user and backend role in the binding will be granted each role. If a role mapping already exists in OpenSearch, the binding adds its users and backend roles to it, and removes only those again when the binding changes or is deleted. E.g:

```yaml
apiVersion: opensearch.org/v1
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

#### Opensearch Action Groups

It is possible to manage Opensearch action groups in Kubernetes with the operator. The operator will not modify action groups that already exist. You can create an example action group as follows:

```yaml
apiVersion: opensearch.org/v1
kind: OpensearchActionGroup
metadata:
  name: sample-action-group
  namespace: default
spec:
  opensearchCluster:
    name: my-first-cluster
  allowedActions:
    - indices:admin/aliases/get
    - indices:admin/aliases/exists
  type: index
  description: Sample action group
```

#### Opensearch Tenants

It is possible to manage Opensearch tenants in Kubernetes with the operator. The operator will not modify tenants that already exist. You can create an example tenant as follows:

```yaml
apiVersion: opensearch.org/v1
kind: OpensearchTenant
metadata:
  name: sample-tenant
  namespace: default
spec:
  opensearchCluster:
    name: my-first-cluster
  description: Sample tenant
```

### Custom admin password

The operator always manages the password of the internal user named `admin`. To choose that password yourself instead of using a generated one, create a secret with `username` and `password` keys and reference it as `adminCredentialsSecret`. The username should be `admin`: the operator uses the secret's credentials for its own API calls, but always writes the password hash to the `admin` user.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: admin-credentials-secret
type: Opaque
data:
  # admin
  username: YWRtaW4=
  # Adm1n-Secure-Passw0rd!
  password: QWRtMW4tU2VjdXJlLVBhc3N3MHJkIQ==
```

Use a strong password. The example meets the security plugin's default password policy (at least 8 characters with upper- and lowercase letters, a digit and a special character), as do the passwords the operator generates.

```yaml
spec:
  security:
    config:
      adminCredentialsSecret:
        name: admin-credentials-secret # The secret with the admin credentials for the operator to use
      securityConfigSecret:
        name: securityconfig-secret # Optional: The secret containing your customized securityconfig
    tls:
      transport:
        generate: true
      http:
        generate: true
```

The operator reads the password, generates the bcrypt hash and writes it to the `admin` user in `<cluster-name>-security-config-generated`, overriding any hash in your securityconfig secret. If you provide your own `internal_users.yml`, keep an `admin` entry (the hash can be omitted):

```yaml
internal_users.yml: |-
  _meta:
    type: "internalusers"
    config_version: 2
  admin:
    reserved: true
    backend_roles:
    - "admin"
    description: "Admin user"
```

**Changing the admin password:** update the password in your `admin-credentials-secret`, or, if you do not set `adminCredentialsSecret`, in `<cluster-name>-admin-password`. The operator detects the change, updates the hash in the generated securityconfig and reruns the securityconfig update job.

### Custom Dashboards user

Dashboards connects to OpenSearch with its own user. By default the operator creates `<cluster-name>-dashboards-password` with a random password for the `kibanaserver` user and configures Dashboards to use it. To use other credentials, create a secret with keys `username` and `password` and reference it:

```yaml
spec:
  dashboards:
    opensearchCredentialsSecret:
      name: dashboards-credentials # This is the name of your secret that contains the credentials for Dashboards to use
```

As for the admin user, the operator writes the password hash for this user (the `username` from the secret, or `kibanaserver`) into the generated securityconfig; you do not need to include it in your securityconfig secret.

**Important:** the operator only manages the password hash. If you supply your own `roles_mapping.yml`, it replaces the image's default file, and the default `kibana_server -> kibanaserver` mapping is gone with it. You must keep the Dashboards user mapped to a role with cluster monitoring permissions yourself, for example:

```yaml
roles_mapping.yml: |-
  kibana_server:
    users:
      - "kibanaserver" # or the username from opensearchCredentialsSecret
```

`kibana_server` is a static role built into the security plugin, so you do not need to define it in `roles.yml` (a definition there is ignored in favor of the built-in one). If you use a custom Dashboards username via `opensearchCredentialsSecret`, you must map that username even when you do not supply `roles_mapping.yml` — the image default only maps `kibanaserver`. Without this mapping, the Dashboards user authenticates successfully but has no permissions, and the Dashboards deployment crash-loops with authorization errors in its logs (e.g. `no permissions for [cluster:monitor/nodes/info]`). The operator emits a `DashboardsUserUnmapped` warning event on the `OpenSearchCluster` when it detects this.

### Security Plugin Disabled

There is no field to disable the security plugin directly. When neither `security.tls.transport` nor `security.tls.http` TLS is enabled, the operator sets `plugins.security.disabled: true`, and the cluster runs over plain HTTP without authentication. In that case:

**Admin User:**
- The operator sets the `OPENSEARCH_INITIAL_ADMIN_PASSWORD` environment variable in the bootstrap pod and all OpenSearch pods from `<cluster-name>-admin-password`, which holds the password of `adminCredentialsSecret` if you provide one, or a generated one otherwise.

**Dashboards User:**
- Custom passwords for the Dashboards user are **not supported**; the generated `<cluster-name>-dashboards-password` secret uses the default `kibanaserver` password.

## Adding Opensearch Monitoring to your cluster

The operator allows you to install and enable the [Prometheus exporter plugin for OpenSearch](https://github.com/opensearch-project/opensearch-prometheus-exporter) on your cluster as a built-in feature. If enabled the operator will install the plugin into the opensearch pods and generate a Prometheus ServiceMonitor object to configure the plugin for scraping. If the ServiceMonitor CRD (from the Prometheus Operator) is not installed, the operator skips the ServiceMonitor and emits a `Warning` event; the rest of the cluster is reconciled normally.
This feature needs internet connectivity to download the plugin. If you are working in a restricted environment, please download the plugin zip for your cluster version (example for 3.2.0: `https://github.com/opensearch-project/opensearch-prometheus-exporter/releases/download/3.2.0.0/prometheus-exporter-3.2.0.0.zip`) and provide it at a location the pods can reach. Configure that URL as `pluginUrl` in the monitoring config. By default the convention shown below in the example will be used if no `pluginUrl` is specified.

By default the Opensearch admin user will be used to access the monitoring API. If you want to use a separate user with limited permissions you need to create that user using either of the following options:

a. Create new applicative User using OpenSearch API/UI, create new secret with 'username' and 'password' keys and provide that secret name under `monitoringUserSecret`.
b. Use the `OpensearchUser` CRD and provide the secret under `monitoringUserSecret`.

### Configuration

To configure monitoring you can add the following fields to your cluster spec:

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
metadata:
  name: my-first-cluster
  namespace: default
spec:
  general:
    version: <YOUR_CLUSTER_VERSION>
    monitoring:
      enable: true # Enable or disable the monitoring plugin
      labels: # The labels add for ServiceMonitor
        someLabelKey: someLabelValue
      scrapeInterval: 30s # The scrape interval for Prometheus, default 30s
      monitoringUserSecret: monitoring-user-secret # Optional, name of a secret with username/password for prometheus to access the plugin metrics endpoint with, defaults to the admin user
      pluginUrl: https://github.com/opensearch-project/opensearch-prometheus-exporter/releases/download/<YOUR_CLUSTER_VERSION>.0/prometheus-exporter-<YOUR_CLUSTER_VERSION>.0.zip # Optional, custom URL for the monitoring plugin
      tlsConfig: # Optional, use this to override the tlsConfig of the generated ServiceMonitor, only the following provided options can be set currently
        serverName: "testserver.test.local"
        insecureSkipVerify: true # The operator currently does not allow configuring the ServiceMonitor with certificates, so this needs to be set
      # Optional: customize relabeling behavior
      relabelings:
        - sourceLabels: [pod]
          regex: "service-east-opensearch-cluster-(masters|nodes)-[1-9][0-9]*"
          action: drop

      metricRelabelings:
        - sourceLabels: [__name__]
          regex: "opensearch_indices_.*"
          action: keep
  # ...
```

#### Relabeling and metric relabeling

You can customize the generated Prometheus `ServiceMonitor` endpoint with relabeling rules.

- `relabelings` are applied **before scraping** and allow modifying target labels.
- `metricRelabelings` are applied **after scraping** and allow filtering or transforming metrics.

## Managing ISM policies with Kubernetes resources

The operator provides a custom Kubernetes resource that allows you to create/update/manage ISM policies using Kubernetes objects.

It is possible to manage OpenSearch ISM policies in Kubernetes with the operator. Fields in the CRD directly map to the OpenSearch ISM Policy structure. The operator will not modify policies that already exist (see the [common behaviour](#managing-security-configurations-with-kubernetes-resources) of these resources). You can create an example policy as follows:

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchISMPolicy
metadata:
  name: sample-policy
spec:
  opensearchCluster:
    name: my-first-cluster
  description: Sample policy
  policyId: sample-policy
  defaultState: hot
  states:
    - name: hot
      actions:
        - replicaCount:
            numberOfReplicas: 4
      transitions:
        - stateName: warm
          conditions:
            minIndexAge: "10d"
    - name: warm
      actions:
        - replicaCount:
            numberOfReplicas: 2
      transitions:
        - stateName: delete
          conditions:
            minIndexAge: "30d"
    - name: delete
      actions:
        - delete: {}
```

The namespace of the `OpenSearchISMPolicy` must be the namespace the OpenSearch cluster itself is deployed in. `policyId` is an optional field, and if not provided `metadata.name` is used as the default. It cannot be changed later. `errorNotification` (and `notification` actions) configure ISM notifications; see the [CRD reference](../designs/crd/opensearch.org.md).

### Apply ism policies to existing indices

The operator provides a flag to apply ism policies to already existing indices in the opensearch cluster.
This is done by setting the `applyToExistingIndices` flag to true in the `OpenSearchISMPolicy` CRD. An example of this can be seen below:

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchISMPolicy
metadata:
  name: test-policy-apply
spec:
  opensearchCluster:
    name: my-first-cluster
  applyToExistingIndices: true
  description: "Test ISM policy - Apply to existing indices is true"
  defaultState: "hot"
  ismTemplate:
    indexPatterns:
      - "test-*"
  states:
    - name: hot
      actions:
        - replicaCount:
            numberOfReplicas: 2
      transitions:
        - stateName: warm
          conditions:
            minIndexAge: "1d"
    - name: warm
      actions:
        - replicaCount:
            numberOfReplicas: 1
```

Note that the default setting of `applyToExistingIndices` is false and it will be false if the flag is omitted in the manifest.

When the flag is true, existing indices matching `ismTemplate.indexPatterns` get this ism policy applied when the policy is first created. Without `ismTemplate.indexPatterns` the flag does nothing, and setting it on an existing policy has no effect. If multiple ism policies use this with the same index pattern, the priority has to be different between the ism policies.

## Managing index and component templates

The operator provides the OpensearchIndexTemplate and OpensearchComponentTemplate CRDs, which are used for managing index and component templates respectively.

The two CRD specifications attempt to be as close as possible to what the OpenSearch API expects, with some changes from snake_case to camelCase.
The fields that have been changed are `index_patterns` to `indexPatterns` (OpensearchIndexTemplate only), `composed_of` to `composedOf` (OpensearchIndexTemplate only) and `template.aliases.<alias>.is_write_index` to `template.aliases.<alias>.isWriteIndex`.

The following example creates a component template for setting the number of shards and replicas, together with specifying a specific time format for documents:

```yaml
apiVersion: opensearch.org/v1
kind: OpensearchComponentTemplate
metadata:
  name: sample-component-template
spec:
  opensearchCluster:
    name: my-first-cluster

  template: # required
    aliases: # optional
      my_alias: {}
    settings: # optional
      number_of_shards: 2
      number_of_replicas: 1
    mappings: # optional
      properties:
        timestamp:
          type: date
          format: yyyy-MM-dd HH:mm:ss||yyyy-MM-dd||epoch_millis
        value:
          type: double
  version: 1 # optional
  _meta: # optional
    description: example description
```

The following index template makes use of the above component template (see `composedOf`) for all indices which follows the `logs-2020-01-*` index pattern:

```yaml
apiVersion: opensearch.org/v1
kind: OpensearchIndexTemplate
metadata:
  name: sample-index-template
spec:
  opensearchCluster:
    name: my-first-cluster

  name: logs_template # name of the index template - defaults to metadata.name. Can't be updated in-place

  indexPatterns: # required index patterns
    - "logs-2020-01-*"
  composedOf: # optional
    - sample-component-template
  priority: 100 # optional

  template: {} # optional
  version: 1 # optional
  _meta: {} # optional
```

Note: `.spec.name` of both resources is immutable, meaning that it cannot be changed after the resources have been deployed to a Kubernetes cluster.

Index templates also accept `dataStream` (with `timestamp_field.name`) to create data streams; component templates accept `allowAutoCreate`. See the [CRD reference](../designs/crd/opensearch.org.md) for all fields.

## Managing Snapshot Policies with Kubernetes Resources

The OpenSearch Operator provides a custom Kubernetes resource to create, update, and manage Snapshot Lifecycle Management (SLM) policies using Kubernetes manifests. This makes it possible to declaratively define and control snapshot policies alongside your cluster resources.

Fields in the CRD map directly to the OpenSearch snapshot policy structure, allowing seamless integration. Policies are not modified if they already exist in OpenSearch. You can define a new policy using the following example:

```yaml
apiVersion: opensearch.org/v1
kind: OpensearchSnapshotPolicy
metadata:
  name: sample-policy
  namespace: default
spec:
  policyName: sample-policy
  enabled: true
  description: Sample policy
  opensearchCluster:
    name: my-first-cluster
  creation:
    schedule:
      cron:
        expression: "0 0 * * *"
        timezone: "UTC"
    timeLimit: "1h"
  deletion:
    schedule:
      cron:
        expression: "0 1 * * *"
        timezone: "UTC"
    timeLimit: "30m"
    deleteCondition:
      maxAge: "7d"
      maxCount: 10
      minCount: 3
  snapshotConfig:
    repository: sample-repository
    indices: "*"
    includeGlobalState: true
    ignoreUnavailable: false
    partial: false
    dateFormat: "yyyy-MM-dd-HH-mm"
    dateFormatTimezone: "UTC"
    metadata:
      createdBy: "sample-operator"
  notification: # Optional
    channel:
      id: my-notification-channel-id
    conditions:
      creation: false
      deletion: false
      failure: true
```

Note:

- The OpensearchSnapshotPolicy must be created in the same namespace as the OpenSearch cluster it targets.

- `policyName` is required and cannot be changed later.

- The repository field must reference an existing snapshot repository in the OpenSearch cluster. For creating a snapshot repository, see [Configuring Snapshot Repositories](#configuring-snapshot-repositories).
