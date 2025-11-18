# Admission Controller Webhooks

The OpenSearch Operator uses Kubernetes [Validating Admission Webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) to validate OpenSearch Custom Resource Definitions (CRDs) before they are persisted to the cluster. This ensures that only valid configurations are accepted, preventing misconfigurations and potential runtime errors.

## Overview

When webhooks are enabled, the operator intercepts CREATE and UPDATE operations on OpenSearch CRDs and validates them before Kubernetes persists the changes. This provides immediate feedback on configuration errors and helps maintain cluster integrity.

## Supported Resources

The operator provides validation webhooks for the following OpenSearch CRDs:

- **OpenSearchActionGroup** - Validates action group configurations
- **OpenSearchCluster** - Validates cluster specifications
- **OpenSearchComponentTemplate** - Validates component template definitions
- **OpenSearchIndexTemplate** - Validates index template configurations
- **OpenSearchISMPolicy** - Validates Index State Management policies
- **OpenSearchRole** - Validates role definitions
- **OpenSearchSnapshotPolicy** - Validates snapshot policy configurations
- **OpenSearchTenant** - Validates tenant configurations
- **OpenSearchUser** - Validates user specifications
- **OpenSearchUserRoleBinding** - Validates user-role binding configurations

## Webhook Naming Convention

The webhook names follow a specific naming convention:

- **Prefix `v`**: Stands for "validating" (indicating these are ValidatingWebhookConfigurations)
- **Suffix `.opensearch.opster.io`**: Uses the same domain as the operator's API group (`opensearch.opster.io`), ensuring consistency across the operator's resources

For example:
- `vopensearchcluster.opensearch.opster.io` - Validating webhook for OpenSearchCluster resources
- `vopensearchuser.opensearch.opster.io` - Validating webhook for OpenSearchUser resources

This naming convention aligns with the operator's domain and makes it clear that these webhooks belong to the OpenSearch operator.

## Configuration

### Enabling/Disabling Webhooks

Webhooks are enabled by default when installing the operator via Helm. You can control this behavior using the `webhook.enabled` value:

```yaml
webhook:
  enabled: true  # Set to false to disable webhooks
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

When enabled, cert-manager will:
1. Create a self-signed Certificate Authority (CA)
2. Generate TLS certificates for the webhook service
3. Automatically inject the CA bundle into the ValidatingWebhookConfiguration
4. Rotate certificates automatically before expiration

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
  secretName: "my-webhook-cert-secret"  # Optional: defaults to <release-name>-opensearch-operator-webhook-server-cert
```

**Creating the Secret:**

The secret must contain the following keys:
- `tls.crt`: The TLS certificate
- `tls.key`: The TLS private key
- `ca.crt`: The CA certificate (optional, for client verification)

The certificate must be valid for the following DNS names:
- `<release-name>-opensearch-operator-webhook-service.<namespace>.svc`
- `<release-name>-opensearch-operator-webhook-service.<namespace>.svc.cluster.local`

**Example: Creating a self-signed certificate manually:**

```bash
# Generate a private key
openssl genrsa -out webhook.key 2048

# Create a certificate signing request
openssl req -new -key webhook.key -out webhook.csr -subj "/CN=opensearch-operator-webhook-service.default.svc"

# Generate the certificate (valid for 365 days)
openssl x509 -req -in webhook.csr -signkey webhook.key -out webhook.crt -days 365

# Create the Kubernetes secret
kubectl create secret tls opensearch-operator-webhook-server-cert \
  --cert=webhook.crt \
  --key=webhook.key \
  --namespace=<operator-namespace>
```

**Note:** When using manual certificates, you must also manually inject the CA bundle into the ValidatingWebhookConfiguration. The CA bundle should be base64-encoded and added to the `caBundle` field in each webhook's `clientConfig`.

## Troubleshooting

### Webhook Not Responding

If webhook validation is failing, check the following:

1. **Verify the webhook service is running:**
   ```bash
   kubectl get svc -n <operator-namespace> | grep webhook
   ```

2. **Check the operator logs:**
   ```bash
   kubectl logs -n <operator-namespace> deployment/<operator-name>-controller-manager
   ```

3. **Verify certificates are valid:**
   ```bash
   kubectl get secret -n <operator-namespace> <webhook-secret-name> -o yaml
   ```

4. **Check the ValidatingWebhookConfiguration:**
   ```bash
   kubectl get validatingwebhookconfiguration <webhook-config-name> -o yaml
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
   kubectl describe certificate -n <operator-namespace> <certificate-name>
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
```

**Warning:** Disabling webhooks will bypass validation, which may allow invalid configurations to be created. Only disable webhooks for troubleshooting purposes.
