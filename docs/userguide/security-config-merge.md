# Security Config Merge Feature

## Overview

The OpenSearch Kubernetes Operator provides a feature to preserve existing internal users when applying custom security configurations. By default, when you apply a custom security config secret, the operator will merge it with the existing configuration in the cluster, preventing the deletion of users that exist in OpenSearch but not in your custom config.

## Default Behavior (Merge Mode)

By default, the operator operates in **merge mode**:

1. Before applying your custom security configuration, it backs up the current security configuration from OpenSearch using the `-backup` option of `securityadmin.sh`
2. It merges the backed-up `internal_users.yml` with your custom `internal_users.yml`, with your custom config taking precedence
3. It applies the merged configuration back to OpenSearch

This ensures that:
- Users created through the operator's CRDs (OpenSearchUser) are preserved
- Users created manually in OpenSearch are preserved
- Your custom user definitions override any existing users with the same name

## Overwrite Mode

If you want the **legacy behavior** where all security config is replaced (potentially deleting existing users), you can enable overwrite mode by adding an annotation to your OpenSearchCluster:

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
metadata:
  name: my-cluster
  annotations:
    opensearch.org/securityconfig-overwrite: "true"
spec:
  # ... rest of spec
```

When this annotation is set to `"true"`, the operator will:
- Skip the backup and merge steps
- Directly apply your custom security configuration
- **Warning:** This will delete any internal users not present in your custom config

## Example: Using Custom Security Config with Merge

Here's a complete example showing how to apply custom security configurations while preserving existing users:

### Step 1: Create your custom security config secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-securityconfig
  namespace: opensearch
type: Opaque
stringData:
  internal_users.yml: |
    ---
    _meta:
      type: "internalusers"
      config_version: 2

    # Define your custom users
    myapp_user:
      hash: "$2y$12$..."  # bcrypt hash of password
      reserved: false
      backend_roles:
        - "myapp_role"
      description: "Custom application user"

    readonly_user:
      hash: "$2y$12$..."  # bcrypt hash of password
      reserved: false
      backend_roles:
        - "readall"
      description: "Read-only access user"

  roles.yml: |
    ---
    _meta:
      type: "roles"
      config_version: 2

    myapp_role:
      reserved: false
      cluster_permissions:
        - "cluster_composite_ops"
      index_permissions:
        - index_patterns:
            - "myapp-*"
          allowed_actions:
            - "crud"
            - "search"
```

### Step 2: Reference the secret in your OpenSearchCluster

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
metadata:
  name: my-cluster
  namespace: opensearch
  # No annotation = merge mode (default)
spec:
  general:
    serviceName: my-cluster
    version: "2.11.0"

  security:
    config:
      securityConfigSecret:
        name: my-securityconfig
      adminCredentialsSecret:
        name: admin-credentials  # Your admin credentials

  nodePools:
    - component: masters
      replicas: 3
      roles:
        - cluster_manager
        - data
```

### Step 3: Apply and verify

```bash
# Apply the configuration
kubectl apply -f securityconfig-secret.yaml
kubectl apply -f opensearch-cluster.yaml

# Watch the security config update job
kubectl get jobs -n opensearch -w

# Check the job logs to see the merge process
kubectl logs -n opensearch job/my-cluster-securityconfig-update

# You should see logs like:
# "Security config merge mode enabled - will preserve existing internal users"
# "Backing up current security configuration..."
# "Backup completed successfully"
# "Merging internal_users.yml (custom config takes precedence)..."
# "Merge completed successfully"
```

## How the Merge Works

The merge process uses `yq` (YAML processor) to combine the files:

1. **Backup**: Current `internal_users.yml` is backed up from OpenSearch
2. **Merge**: Your custom `internal_users.yml` is merged with the backup
   - Custom users override existing users with the same name
   - Existing users not in your custom config are preserved
3. **Apply**: The merged configuration is applied back to OpenSearch

### Merge Priority

When merging `internal_users.yml`:
- **Your custom config takes precedence**: If you define a user that already exists, your definition replaces the existing one
- **Existing users are preserved**: Users not in your custom config remain unchanged
- **Admin user** (`admin`) and **kibanaserver** user are always managed by the operator

## Technical Details

### Init Container

The security config update job uses an init container to download and install `yq`:

```yaml
initContainers:
  - name: install-yq
    image: busybox  # or your configured init helper image
    command: ["/bin/sh", "-c"]
    args:
      - |
        YQ_VERSION=v4.35.1
        wget -qO /tools/yq https://github.com/mikefarah/yq/releases/download/${YQ_VERSION}/yq_linux_amd64
        chmod +x /tools/yq
```

### Command Flow

In merge mode, the security update job executes:

1. Wait for cluster to be ready
2. Run `securityadmin.sh -backup` to save current config
3. Merge `internal_users.yml` using `yq`
4. Apply each security config file individually

## Troubleshooting

### Check if merge mode is active

Look at the security config update job logs:

```bash
kubectl logs -n opensearch job/my-cluster-securityconfig-update
```

You should see either:
- `"Security config merge mode enabled"` (default)
- `"Security config overwrite mode enabled"` (if annotation is set)

### Verify the annotation

```bash
kubectl get opensearchcluster my-cluster -o jsonpath='{.metadata.annotations.opensearch\.org/securityconfig-overwrite}'
```

- No output or `false`: Merge mode (default)
- `true`: Overwrite mode

### Common Issues

1. **yq download fails**: Ensure the job pods have internet access or pre-install yq in your init helper image
2. **Backup fails**: Check that the admin certificate is valid and the cluster is reachable
3. **Merge fails**: Verify your custom `internal_users.yml` is valid YAML

### Manual Override

If you need to force a full overwrite temporarily:

```bash
kubectl annotate opensearchcluster my-cluster \
  opensearch.org/securityconfig-overwrite=true \
  --overwrite
```

To return to merge mode:

```bash
kubectl annotate opensearchcluster my-cluster \
  opensearch.org/securityconfig-overwrite-
# or
kubectl annotate opensearchcluster my-cluster \
  opensearch.org/securityconfig-overwrite=false \
  --overwrite
```

## Best Practices

1. **Start with merge mode**: Use the default merge behavior unless you have a specific reason to overwrite
2. **Test in development**: Always test security config changes in a dev environment first
3. **Backup manually**: Keep your own backups of security configurations
4. **Version control**: Store your security config secrets in version control
5. **Monitor job logs**: Check the security update job logs to ensure merges succeed
6. **Document users**: Keep track of which users are managed via CRDs vs. custom configs

## Migration from Overwrite Mode

If you're currently using the overwrite behavior and want to switch to merge mode:

1. Remove the `opensearch.org/securityconfig-overwrite: "true"` annotation
2. Ensure your custom security config includes all users you want to keep
3. Apply the changes
4. Verify in the logs that merge mode is active
5. Check that all expected users are present in OpenSearch

## Related Documentation

- [OpenSearch Security Configuration](https://opensearch.org/docs/latest/security/configuration/security-admin/)
- [Custom Security Config Example](../../opensearch-operator/examples/securityconfig-secret.yaml)
- [OpenSearchUser CRD](./main.md#opensearchuser)