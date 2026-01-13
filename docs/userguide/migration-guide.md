# Migration Guide: opensearch.opster.io to opensearch.org

This guide explains how to migrate your OpenSearch Kubernetes resources from the deprecated `opensearch.opster.io` API group to the new `opensearch.org` API group.

## Overview

The OpenSearch Kubernetes Operator is transitioning from `opensearch.opster.io/v1` to `opensearch.org/v1` API group. This change reflects the project's evolution and alignment with the OpenSearch project branding.

### Timeline

- **Current Release**: Both API groups are supported
- **Deprecation Period**: 2-3 releases (old API group logs warnings)
- **Future Release**: `opensearch.opster.io` will be removed

## Automatic Migration

The operator includes a migration controller that automatically handles the transition:

1. **Auto-sync**: When you create or update resources using `opensearch.opster.io/v1`, the migration controller automatically creates/updates corresponding `opensearch.org/v1` resources
2. **Status Sync**: Status is synchronized bidirectionally between old and new resources
3. **Deletion Handling**: Deleting an old API resource will delete its corresponding new API resource

### Migration Annotations

Migrated resources include these annotations:

```yaml
metadata:
  annotations:
    opensearch.org/migrated-from: "opensearch.opster.io/v1"
    opensearch.org/migration-timestamp: "2024-01-15T10:30:00Z"
    opensearch.org/source-uid: "original-resource-uid"
```

## Manual Migration Steps

For a clean migration without relying on automatic sync:

### Step 1: Update API Version in Manifests

Change your manifest files from:

```yaml
apiVersion: opensearch.opster.io/v1
kind: OpenSearchCluster
```

To:

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
```

### Step 2: Update Labels and Annotations

If you're using label selectors, update the domain:

| Old Label | New Label |
|-----------|-----------|
| `opster.io/opensearch-cluster` | `opensearch.org/opensearch-cluster` |
| `opster.io/opensearch-nodepool` | `opensearch.org/opensearch-nodepool` |
| `opster.io/opensearch-job` | `opensearch.org/opensearch-job` |

### Step 3: Update Helm Values (if using Helm)

If deploying via Helm, the `opensearch-cluster` chart now supports configurable API group:

```yaml
# values.yaml
apiGroup: opensearch.org  # Default (recommended)
# apiGroup: opensearch.opster.io  # Legacy (deprecated)
```

### Step 4: Apply New Resources

```bash
kubectl apply -f your-cluster.yaml
```

### Step 5: Verify Migration

Check that both old and new resources exist:

```bash
# Old API group
kubectl get opensearchclusters.opensearch.opster.io

# New API group
kubectl get opensearchclusters.opensearch.org
```

### Step 6: Remove Old Resources (Optional)

Once you've verified the new resources are working correctly:

```bash
# Remove old API resources (new ones will continue to function)
kubectl delete opensearchclusters.opensearch.opster.io <cluster-name>
```

## Resource Mapping

All CRDs have equivalent types in the new API group:

| Old Resource (opensearch.opster.io/v1) | New Resource (opensearch.org/v1) |
|----------------------------------------|----------------------------------|
| OpenSearchCluster | OpenSearchCluster |
| OpensearchUser | OpensearchUser |
| OpensearchRole | OpensearchRole |
| OpensearchUserRoleBinding | OpensearchUserRoleBinding |
| OpensearchTenant | OpensearchTenant |
| OpensearchActionGroup | OpensearchActionGroup |
| OpenSearchISMPolicy | OpenSearchISMPolicy |
| OpensearchSnapshotPolicy | OpensearchSnapshotPolicy |
| OpensearchIndexTemplate | OpensearchIndexTemplate |
| OpensearchComponentTemplate | OpensearchComponentTemplate |

## Troubleshooting

### Resources Not Migrating

If automatic migration isn't working:

1. Check the operator logs for migration controller errors:
   ```bash
   kubectl logs -n opensearch-operator-system deployment/opensearch-operator-controller-manager | grep -i migration
   ```

2. Verify both CRDs are installed:
   ```bash
   kubectl get crd | grep opensearch
   ```

### Status Not Syncing

If status isn't syncing between old and new resources:

1. Check that the migration controller is running
2. Verify RBAC permissions for both API groups
3. Look for errors in the controller logs

### Webhook Errors

If you encounter webhook validation errors:

1. Ensure webhook certificates are valid for both API groups
2. Check webhook configuration:
   ```bash
   kubectl get validatingwebhookconfigurations | grep opensearch
   ```

## Rollback

If you need to rollback to the old API group:

1. Set Helm value to use legacy API:
   ```yaml
   apiGroup: opensearch.opster.io
   ```

2. Or manually change manifests back to `opensearch.opster.io/v1`

The operator supports both API groups during the deprecation period.

## FAQ

### Q: Will my existing clusters continue to work?

Yes. The migration controller ensures that existing `opensearch.opster.io` resources continue to function. Changes are automatically synced to `opensearch.org` resources.

### Q: Do I need to recreate my OpenSearch clusters?

No. The migration is handled at the Kubernetes resource level. Your actual OpenSearch clusters (pods, data, etc.) are not affected.

### Q: When will the old API group be removed?

The `opensearch.opster.io` API group will be removed approximately 2-3 releases after the deprecation announcement. Watch release notes for specific dates.

### Q: Can I use both API groups simultaneously?

Yes, during the deprecation period. However, we recommend migrating to `opensearch.org` to avoid future disruption.

### Q: How do I update my CI/CD pipelines?

Update any manifests or Helm values to use `opensearch.org`. The Helm chart defaults to the new API group.
