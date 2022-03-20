# Opensearch Operator User Guide

This guide is intended for users of the Opensearch Operator. If you want to contribute to the development of the operator please see the [Design documents](../designs/high-level.md) instead.

## Installation

TBD

## Quickstart

After you have successfully installed the operator you can deploy your first opensearch cluster. This is done by creating an `OpenSearchCluster` custom object in Kubernetes.

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
  dashboards:
    enable: true
    replicas: 2
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
      diskSize: 30
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

Then run `kubectl apply -f cluster.yaml`. If you watch the cluster (e.g. `watch -n 2 kubectl get pods`) you will see that after a few seconds the operator will create several pods: Three pods for the opensearch cluster (`my-first-cluster-masters-0/1/2`) and one pod for the dashboards instance. After the pods are showing ready (normally takes about 1-2 minutes) you can connect to your cluster using port-forwarding:

Run `kubectl port-forward svc/my-first-cluster-dashboards 5601`, then open [http://localhost:5601](http://localhost:5601) in your browser and log in with the default demo credentials `admin / admin`.
Or if you want to access the opensearch REST API run `kubectl port-forward svc/my-first-cluster 9200`, then open a second terminal and run `curl -k -u admin:admin https://localhost:9200/_cat/nodes?v`. You should see the three deployed pods listed.

To delete your cluster run `kubectl delete -f cluster.yaml`. The operator will then cleanup and delete any kubernetes resources created for the cluster. Note that this will also delete the persistent volumes for the cluster and therefore all data stored in opensearch.

The minimal cluster you deployed in this section is only intended for demo purposes. Please see the next sections on how to configure the different aspects of your cluster.

## TLS

For security reasons communication with the opensearch cluster and between cluster nodes is only done encrypted. If you do not configure anything opensearch will use included demo TLS certificates that are not suited for real deployments.

Depending on your requirements the operator offers two ways of managing TLS certificates: Either you can supply your own certificates or the operator will generate its own CA and sign certificates for all nodes using that CA. The second way is the recommended one unless you want to directly expose your opensearch cluster outside your kubernetes cluster or your organization has rules about using self-signed certificates for internal communication.

TLS certificates are used in three places, and each can be configured independently.

### Node transport

Opensearch cluster nodes communicate between each other using the opensearch transport protocol (by default port 9300). This is not exposed externally, so in almost all cases generated certificates should be adequate.

To configure node transport security the following fields in the `OpenSearchCluster` custom resource are available:

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

To have the operator generate the certificates you only need the `generate` and `perNode` fields set to `true` (all other fields can be omited). The operator will then generate a CA certificate and one certificate per node and then use the CA to sign the node certificates. These certificates are valid for one year. Note that currently the operator does not have certificate renewal implemented. Optionally you can also supply your own CA certificate by putting it into a secret (it must be a PEM-encoded X509 certificate, the certificate must be in the data field `ca.crt`, the private key in `ca.key`) and providing the name as `caSecret.name`, the operator will then use your CA certificate to sign the node certificates.

Alternatively you can provide the certificates yourself (e.g. if your organization has an internal CA). You can either provide one certificate to be used by all nodes or provide a certificate for each node (recommended). In this mode set `generate: false` and `perNode` to `true` or `false` depending on if you provide per-node certificates. if you provide just one certificate it must be placed in a Kubernetes TLS secret (with the fields `ca.crt`, `tls.key` and `tls.crt`, must all be PEM-encoded) and you must provide the name of the secret as `secret.name`. If you want to keep the CA certificate separate you can place it in a separate secret and supply that as `caSecret.name`.
If you provide one certificate per node you must place all certificates into one secret (including the `ca.crt`) with a `<hostname>.key` and `<hostname>.crt` for each node. The hostname is defined as `<cluster-name>-<nodepool-component>-<index>` (e.g. `my-first-cluster-masters-0`).
If you provide the certificates yourself you must also provide the list of certificate DNs in `nodesDn`, wildcards can be used (e.g. `"CN=my-first-cluster-*,OU=my-org"`). The `adminDn` field is only needed if you also supply your own securityconfig (see below).

### Node HTTP/REST API

Each opensearch cluster node exposes the REST API using HTTPS (by default port 9200).

To configure http api security the following fields in the `OpenSearchCluster` custom resource are available:

```yaml
# ...
spec:
  security:
    tls:  # Everything related to TLS configuration
      http:  # Configuration of the http endpoint
        generate: true  # Have the operator generate and sign certificates
        secret:
          name:  # Name of the secret that contains the provided certificate
        caSecret:
          name:  # Name of the secret that contains a CA the operator should use
# ...
```

Again you have the option of either letting the operator generate and sign the certificates or provide your own. The only difference is that per-node certificate are not possible here. For everything else it works the same as the node transport certificates.

If you provide your own certificates please make sure the following names are added as SubjectAltNames (SAN): `<cluster-name>`, `<cluster-name>.<namespace>`, `<cluster-name>.<namespace>.svc`,`<cluster-name>.<namespace>.svc.cluster.local`.

Directly exposing the node http port outside the kubernetes cluster is not recommended. Instead you should configure an ingress. The ingress can then also present a certificate from an accredited CA (for example LetsEncrypt) and hide internally used self-signed certificates. That way the nodes must not be supplied externally with properly signed certificates.

### Dashboards HTTP

Opensearch Dashboards itself can expose its API/UI via HTTP or HTTPS. By default it is unencrypted. To secure the connection you have the option of either letting the operator generate and sign a certificate or providing your own. The following fields in the `OpenSearchCluster` custom resource are available to configure it:

```yaml
# ...
spec:
  dashboards:
    enable: true  # Deploy Dashboards component
    tls:
      enable: true  # Configure TLS
      generate: true  # Have the operator generate and sign a certificate
      secret:
        name:  # Name of the secret that contains the provided certificate
      caSecret:
       name:  # Name of the secret that contains a CA the operator should use
# ...
```

To let the operator generate the certificate just set `tls.enable: true` and `tls.generate: true` (the other fields under `tls` can be omited). Again as with the node certificates you can supply your own CA via `caSecret.name` for the operator to use.
If instead you want to use your own certificate you need to provide it as a Kubernetes TLS secret (with fields `tls.key` and `tls.crt`) and provide the name as `secret.name`.

If you want to expose Dashboards outside of the cluster it is recommended to use operator-generated certificates internally and let an Ingress present a valid certificate from an accredited CA.

## Securityconfig

By default Opensearch clusters use the opensearch-security plugin to handle authentication and authorization. If nothing is specifically configured clusters deployed using the operator use the demo securityconfig provided by the opensearch project (see [internal_users.yml](https://github.com/opensearch-project/security/blob/main/securityconfig/internal_users.yml) for a list of users).

You can provide your own securityconfig (see the entire [demo securityconfig](https://github.com/opensearch-project/security/blob/main/securityconfig) as an example and the [Access control documentation](https://opensearch.org/docs/latest/security-plugin/access-control/index/) of the opensearch project) with your own users and roles. To do that you must provide a secret with all the required securityconfig yaml files.

The operator can be controlled using the following fields in the `OpenSearchCluster` custom resource:

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

Provide the name of the secret that contains your securityconfig yaml files as `securityconfigSecret.name`. Note that it is not possible to only provide some of the files, all must be provided, there is no merge between the demo files and your provided ones. In addition you must provide the name of a secret as `adminCredentialsSecret.name` that has fields `username` and `password` for a user that the operator can use for communicating with opensearch (currently used for getting the cluster status, doing health checks and coordinating node draining during cluster scaling operations).

If you provided your own certificate for node transport communication then you must also provide an admin client certificate (as a Kubernetes TLS secret with fields `ca.crt`, `tls.key` and `tls.crt`) as `adminSecret.name`. The DN of the certificate must be listed under `security.tls.transport.adminDn`. Be advised that the `adminDn` and `nodesDn` must be defined in a way that the admin certficate cannot be used or recognized as a node certficiate, otherwise opensearch will reject any authentication request using the admin certificate.

To apply the securityconfig to the opensearch cluster the operator uses a separate kubernetes job (called `<cluster-name>-securityconfig-update`). This job is run during the initial provisioning of the cluster. The operator also monitors the secret with the securityconfig for any changes and then reruns the update job to apply the new config. Note that the operator only checks for changes in a certain interval so it might take a minute or two for the changes to be applied. If the changes are not applied after a few minutes please use kubectl to check the logs of the pod of the `<cluster-name>-securityconfig-update` job. If you have an error in your configuration it will be reported there.

## Nodepools and scaling

TBD
