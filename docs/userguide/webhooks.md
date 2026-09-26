# Admission Controller Webhooks

The OpenSearch Operator uses Kubernetes [Validating Admission Webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) to validate OpenSearch Custom Resource Definitions (CRDs) before they are persisted to the cluster. This ensures that only valid configurations are accepted, preventing misconfigurations and potential runtime errors.

## Overview

When webhooks are enabled, the operator intercepts CREATE and UPDATE operations on OpenSearch CRDs and validates them before Kubernetes persists the changes. This provides immediate feedback on configuration errors and helps maintain cluster integrity.

## Validation Rules

The operator registers a validating webhook for every `opensearch.org/v1` resource kind. Deletes are never intercepted. The rules below are the complete set enforced at admission time; everything else is left to the CRD schema and the reconcilers.

### OpenSearchCluster

| Check | Create | Update |
|-------|:------:|:------:|
| Every node pool has a non-empty `component`, and no two pools share one (the component names the pool's StatefulSet, Services and ConfigMaps) | ✓ | ✓ |
| Every entry in `roles` is a known role: `master`, `cluster_manager`, `data`, `data_content`, `data_hot`, `data_warm`, `data_cold`, `data_frozen`, `ingest`, `ml`, `remote_cluster_client`, `transform`, `search`, `warm` | ✓ | ✓ |
| At least one node pool has the cluster-manager role and `replicas >= 1`. `master` and `cluster_manager` are both accepted on any version and mapped to the name that `general.version` uses, so `master` still works on 2.x and 3.x. This blocks scaling the last manager pool to 0 or removing it | ✓ | ✓ |
| A change to `general.version` must be valid semver, must not be a downgrade and must not jump more than one major version, compared with the version in `status.version`. Only checked once the cluster has been initialized; changing back to the running version is always allowed | | ✓ |
| `general.version` cannot change while `general.image` pins a custom image that stays the same (the version would be ignored). Change or remove the image in the same update | | ✓ |
| `persistence.pvc.storageClass` of an existing node pool cannot change (StatefulSet volume claims are immutable). New node pools are not checked | | ✓ |
| When transport or HTTP TLS is enabled (the block is present and `enabled` is not `false`), it needs `generate: true` or a `secret.name` | ✓ | ✓ |
| When `security.config.adminSecret` is empty and the operator can run securityadmin (HTTP TLS enabled on 2.x+, transport TLS enabled on 1.x), the admin certificate is generated, so `tls.http.generate` (2.x+) or `tls.transport.generate` (1.x) must be `true` | ✓ | ✓ |

Updates to a cluster that is being deleted are not validated.

### Other resource kinds

Every other kind references its cluster through `spec.opensearchCluster.name`. On create, that `OpenSearchCluster` (in either API group) must already exist in the same namespace; on update, the reference cannot change. With GitOps tools, make sure the cluster is applied before the resources that reference it (for example with sync waves), otherwise the first apply of those resources is rejected.

| Kind | Additional checks (create and update) | Immutable after creation |
|------|----------------------------------------|--------------------------|
| OpensearchUser | `passwordFrom.name` and `passwordFrom.key` are set | |
| OpensearchRole | At least one of `clusterPermissions`, `indexPermissions`, `tenantPermissions` | |
| OpensearchUserRoleBinding | `roles` is not empty; at least one of `users`, `backendRoles` | |
| OpensearchActionGroup | `allowedActions` is not empty | |
| OpensearchTenant | | |
| OpenSearchISMPolicy | `states` is not empty; `defaultState` is set and names one of the `states` | Policy ID (`policyId`, or `metadata.name` if unset) |
| OpensearchSnapshotPolicy | `policyName`, `snapshotConfig.repository` and `creation.schedule.cron.expression` are set | `policyName` |
| OpensearchIndexTemplate | | Template name (`name`, or `metadata.name` if unset) |
| OpensearchComponentTemplate | | Template name (`name`, or `metadata.name` if unset) |

The immutability checks apply once the resource has been created in OpenSearch, that is, once its status records the name or ID.

### Legacy `opensearch.opster.io` resources

While legacy API support is enabled (Helm value `legacyAPI.enabled`, default `true`, which sets the operator flag `--enable-legacy-api`), the operator also registers a webhook for each deprecated `opensearch.opster.io/v1` kind (`v<kind>.opensearch.opster.io`). These webhooks:

- deny every create; use `opensearch.org/v1` instead,
- deny any update that changes `spec`,
- allow status-only and metadata-only updates, which the migration controller relies on, and updates to objects that are being deleted.

Deletes are not intercepted. With `legacyAPI.enabled=false` the legacy webhooks, CRDs and migration controllers are not installed. See the [Migration Guide](migration-guide.md) for the full migration flow.

## Webhook Naming Convention

The webhook names follow a specific naming convention:

- **Prefix `v`**: Stands for "validating" (indicating these are ValidatingWebhookConfigurations)
- **Suffix `.opensearch.org`**: Uses the same domain as the operator's API group (`opensearch.org`), ensuring consistency across the operator's resources

For example:
- `vopensearchcluster.opensearch.org` - Validating webhook for OpenSearchCluster resources
- `vopensearchuser.opensearch.org` - Validating webhook for OpenSearchUser resources

This naming convention aligns with the operator's domain and makes it clear that these webhooks belong to the OpenSearch operator. The legacy webhooks use the `.opensearch.opster.io` suffix instead.

The Helm chart names the related Kubernetes objects after the release's full name, `<fullname>`. This is the release name, or `<release-name>-opensearch-operator` when the release name does not contain `opensearch-operator` (`fullnameOverride` replaces it). With the usual `helm install opensearch-operator ...`, `<fullname>` is `opensearch-operator`.

| Object | Name |
|--------|------|
| Operator Deployment | `<fullname>` |
| Webhook Service | `<fullname>-webhook-service` |
| Webhook TLS Secret | `<fullname>-webhook-server-cert` (or `webhook.secretName`) |
| ValidatingWebhookConfiguration | `<fullname>-validating-webhook-configuration` |
| cert-manager Issuer / Certificate | `<fullname>-selfsigned-issuer` / `<fullname>-serving-cert` |

## Configuration

### Enabling/Disabling Webhooks

Webhooks are enabled by default when installing the operator via Helm. You can control this behavior using the `webhook.enabled` value, which also drives the operator's `--enable-webhooks` flag:

```yaml
webhook:
  enabled: true  # Set to false to disable webhooks
  port: 9443     # Customize the webhook server port
```

With `webhook.enabled: false` the chart renders no webhook Service, certificate or ValidatingWebhookConfiguration, so cert-manager is not required, but none of the validations above are applied.

When running the manager outside of Helm (for example, during local development), use the CLI flag directly:

```bash
./manager --enable-webhooks=false
# optionally override the webhook port
./manager --webhook-port=9443
```

### Failure Policy

The failure policy determines what happens when the webhook cannot be reached or returns an error:

- **Fail** (default): The API request is rejected if the webhook fails
- **Ignore**: The API request is allowed to proceed even if the webhook fails

You can configure this in `values.yaml`:

```yaml
webhook:
  failurePolicy: Fail  # Options: Fail, Ignore
```

## Certificate Management

Webhooks require TLS certificates to secure communication between the Kubernetes API server and the operator. The operator supports two methods for certificate management:

### Using Cert-Manager (Recommended)

Cert-manager automatically generates and manages TLS certificates for the webhooks. This is the default and recommended approach.

**Prerequisites:**
- Cert-manager must be installed in your cluster
- Cert-manager version 1.0 or later

**Configuration:**
```yaml
webhook:
  enabled: true
  certManager:
    enabled: true  # Enable cert-manager integration
```

When enabled, the chart creates a `selfSigned` cert-manager `Issuer` and a `Certificate` for the webhook Service, and cert-manager:
1. Issues a self-signed serving certificate into the webhook TLS Secret
2. Injects it as the CA bundle into the ValidatingWebhookConfiguration (through the `cert-manager.io/inject-ca-from` annotation)
3. Renews the certificate before it expires

### Manual Certificate Management

If cert-manager is not available or you prefer to manage certificates manually, you can provide your own TLS secret.

**Prerequisites:**
- A TLS secret containing the webhook certificates
- The secret must be created in the same namespace as the operator

**Configuration:**
```yaml
webhook:
  enabled: true
  certManager:
    enabled: false  # Disable cert-manager
  secretName: "my-webhook-cert-secret"  # Optional: defaults to <fullname>-webhook-server-cert
```

**Creating the Secret:**

The secret must contain the following keys:
- `tls.crt`: The TLS certificate
- `tls.key`: The TLS private key
- `ca.crt`: The CA certificate (optional, for client verification)

The certificate must be valid for the following DNS names (as subject alternative names; the API server ignores the CN):
- `<fullname>-webhook-service.<namespace>.svc`
- `<fullname>-webhook-service.<namespace>.svc.cluster.local`

**Example: Creating a self-signed certificate manually** (release `opensearch-operator` in namespace `default`; requires OpenSSL 1.1.1 or later for `-addext`):

```bash
SVC=opensearch-operator-webhook-service
NS=default

# Generate a key and a self-signed certificate with the required SANs (valid for 365 days)
openssl req -x509 -newkey rsa:2048 -nodes -days 365 \
  -keyout webhook.key -out webhook.crt \
  -subj "/CN=${SVC}.${NS}.svc" \
  -addext "subjectAltName=DNS:${SVC}.${NS}.svc,DNS:${SVC}.${NS}.svc.cluster.local"

# Create the Kubernetes secret
kubectl create secret tls opensearch-operator-webhook-server-cert \
  --cert=webhook.crt \
  --key=webhook.key \
  --namespace=${NS}
```

**Note:** When using manual certificates, you must also manually inject the CA bundle into the ValidatingWebhookConfiguration. The CA bundle should be base64-encoded and added to the `caBundle` field in each webhook's `clientConfig`. For a self-signed certificate like the one above, the CA bundle is `webhook.crt` itself.

## Troubleshooting

### Webhook Not Responding

If webhook validation is failing, check the following:

1. **Verify the webhook service is running:**
   ```bash
   kubectl get svc -n <operator-namespace> | grep webhook
   ```

2. **Check the operator logs:**
   ```bash
   kubectl logs -n <operator-namespace> deployment/<fullname>
   ```

3. **Verify certificates are valid:**
   ```bash
   kubectl get secret -n <operator-namespace> <webhook-secret-name> -o yaml
   ```

4. **Check the ValidatingWebhookConfiguration:**
   ```bash
   kubectl get validatingwebhookconfiguration <fullname>-validating-webhook-configuration -o yaml
   ```

### Certificate Issues

If you're experiencing certificate-related issues:

1. **Verify cert-manager is installed:**
   ```bash
   kubectl get pods -n cert-manager
   ```

2. **Check certificate status:**
   ```bash
   kubectl get certificate -n <operator-namespace>
   kubectl describe certificate -n <operator-namespace> <fullname>-serving-cert
   ```

3. **Verify the certificate secret exists:**
   ```bash
   kubectl get secret -n <operator-namespace> <webhook-secret-name>
   ```

### Temporarily Disabling Webhooks

If you need to temporarily disable webhooks for troubleshooting:

```yaml
webhook:
  enabled: false
  port: 9443
```

Or, if you are running the binary directly:

```bash
./manager --enable-webhooks=false
# If you also need to change the webhook port:
./manager --webhook-port=9443
```

**Warning:** Disabling webhooks will bypass validation, which may allow invalid configurations to be created. Only disable webhooks for troubleshooting purposes.
