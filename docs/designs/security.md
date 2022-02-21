# Security Controller

The security controller deals with everything related to the opensearch-security plugin. This entails two areas:

* Configuring TLS for nodes: Provides default encryption of communication between OpenSearch cluster nodes by [generating self-signed certificates](https://opensearch.org/docs/latest/security-plugin/configuration/generate-certificates/) and configuring them for all nodes. A user may also supply externally managed (e.g. from a company CA) certificates to be used.
* Managing the securityconfig: Allows the user to provide a custom securityconfig and takes care of applying that to the OpenSearch cluster when it changes.

The controller is configured via the `security` object in the cluster spec. If no config is provided the controller will fall back to the demo certificates and configuration that is included with the opensearch docker image.

All generated keys and certificates by the operator are stored in Kubernetes secrets to be securely used by the cluster pods. The operator has two modes for generating/using the certificates: By default it will generate one certificate that is used by all nodes, if the operator is switched to a per-node mode it will generate a certificate for each node.

## CRD

```yaml
spec:
  # ...
  security:
    tls:
      transport:
        perNode: false # If set to true the operator will generate a key+certificate per node instead of just one for all nodes
        generate: false # If true the operator will generate self-signed certificates, if false the secrets below must be specified
        secret: opensearch-certs # Name of a secret. Only used if generate=false. If perNode=true secret must have keys ${HOSTNAME}.key and ${HOSTNAME}.crt for each pod and a ca.crt, if perNode=false secret must have keys ca.crt, tls.key and tls.crt. Either this field or caSecret,keySecret and certSecret must be set if generate=false
        caSecret: # Only used if generate=false
          secretName: opensearch-ca # Name of the secret that contains the PEM-encoded CA certificate
          key: ca.crt # Optional, key in the secret that contains the ca cert, if not set ca.crt is used
        keySecret: # Only used if generate=false
          secretName: opensearch-node # Name of the secret that contains the PEM-encoded node private key
          key: tls.key # Optional, key in the secret that contains the node private key, if not set tls.key is used
        certSecret: # Only used if generate=false
          secretName: opensearch-node # Name of the secret that contains the PEM-encoded node certificate
          key: tls.crt # Optional, key in the secret that contains the node certificate, if not set tls.crt is used
        nodes_dn: # Only used if generate=false, must list the DNs of the provided certificate
          - "CN=foobar"
        admin_dn: # Only used if generate=false, must list the DN of the admin certificate, note: admin cert must be signed by the same CA as the node certs
          - "CN=fobar-admin"
      http:
        generate: true # If true the operator will generate self-signed certificates, if false the secrets below must be specified
        secret: null # Only set if generate=false, details see tansport above
        caSecret: null # Only set if generate=false, details see transport above
        keySecret: null # Only set if generate=false, details see transport above
        certSecret: null # Only set if generate=false, details see transport above
    auth:
      securityConfigSecret: opensearch-securityconfig # optional, if set will be used as securityconfig, must have keys conforming to the different files (config.yml, roles.yml, ...)
```

## Relevant opensearch config

The following lines are potentially added to the `opensearch.yml` by the security controller:

```yaml
plugins.security.ssl.transport.pemcert_filepath: tls-transport/tls.crt # If per-node certificates are activated: tls-transport/${HOSTNAME}.crt
plugins.security.ssl.transport.pemkey_filepath: tls-transport/tls.key # If per-node certificates are activated: tls-transport/${HOSTNAME}.key
plugins.security.ssl.transport.pemtrustedcas_filepath: tls-transport/ca.crt
plugins.security.ssl.transport.enforce_hostname_verification: false  # Set to true for per-node certificates
plugins.security.ssl.http.enabled: true
plugins.security.ssl.http.pemcert_filepath: tls-http/tls.crt
plugins.security.ssl.http.pemkey_filepath: tls-http/tls.key
plugins.security.ssl.http.pemtrustedcas_filepath: tls-http/ca.crt
plugins.security.allow_unsafe_democertificates: false
plugins.security.nodes_dn: ["CN=my-cluster"]
plugins.security.admin_dn: ["CN=my-cluster-admin"]
```

## Features for Phase 1

* Generate self-signed certificates for the cluster to use
* Use existing certificates provided via secrets
* Allow user to supply custom securityconfig via Secret
* Execute [securityadmin.sh](https://opensearch.org/docs/latest/security-plugin/configuration/security-admin/) to update securityconfig when Secret changes

## Features for Phase 2

* Renewal of node certificates when they are close to expiring or when the user requests a manual renewal (e.g. because a certificate was compromised)
* Manage securityconfig via extra CRDs (e.g. OpenSearchUser, OpenSearchRole, etc.)
