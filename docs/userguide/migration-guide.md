# Migration Guide: opensearch.opster.io to opensearch.org

This guide explains how to migrate your OpenSearch Kubernetes resources from the deprecated `opensearch.opster.io` API group to the new `opensearch.org` API group.

## Overview

The OpenSearch Kubernetes Operator is transitioning from `opensearch.opster.io/v1` to `opensearch.org/v1` API group. This change reflects the project's evolution and alignment with the OpenSearch project branding.

### Timeline

- **Current Release**: Both API groups are supported
- **Deprecation Period**: 2-3 releases (old API group logs warnings)
- **Future Release**: `opensearch.opster.io` will be removed

## Automatic Migration

The operator includes a migration controller that automatically handles the transition from old to new API groups. The migration process is designed to be seamless and safe.

### How Migration Works

1. **Automatic Resource Creation**: When you have resources using `opensearch.opster.io/v1`, the migration controller automatically creates corresponding `opensearch.org/v1` resources
2. **Readiness Check**: Migration only occurs when the old resource is in a ready status:
   - **Clusters**: Must be in `RUNNING` phase
   - **Other Resources**: Must be in `CREATED` state (not `PENDING`, `ERROR`, or `IGNORED`)
3. **Spec Change**: Spec changes to old API resources are not allowed by webhooks
4. **Status Sync**: Status is synchronized from new resources back to old resources (new → old direction)
5. **Deletion Behavior**:
   - **Deleting new resource** → Automatically deletes the corresponding old resource
   - **Deleting old resource** → Only allowed if the corresponding new resource exists (ensures migration completed)

### Migration Annotations

Migrated resources include these annotations to track the migration:

```yaml
metadata:
  annotations:
    opensearch.org/migrated-from: "opensearch.opster.io/v1"
    opensearch.org/migration-timestamp: "2024-01-15T10:30:00Z"
    opensearch.org/source-uid: "original-resource-uid"
    opensearch.org/migration-sync: "2024-01-15T10:35:00Z"  # Updated on each sync
```

### Migration Controller Behavior

The migration controller watches both old and new API groups and handles:

- **Old Resource Events**: Creates/updates new resources, syncs status back
- **New Resource Events**: Handles deletion of old resources when new ones are deleted
- **Status Synchronization**: Periodically syncs status from new to old (every 30 seconds)
- **Finalizer Management**: Adds migration finalizers to old resources to ensure proper cleanup

### Resource Readiness Requirements

Migration will be **skipped** (and requeued) if the old resource is not ready:

| Resource Type | Ready Status | Not Ready Statuses |
|--------------|--------------|-------------------|
| OpenSearchCluster | `Phase: RUNNING` | `PENDING`, `UPGRADING`, or any other phase |
| OpensearchUser | `State: CREATED` | `PENDING`, `ERROR` |
| OpensearchRole | `State: CREATED` | `PENDING`, `ERROR`, `IGNORED` |
| OpensearchUserRoleBinding | `State: CREATED` | `PENDING`, `ERROR` |
| OpensearchTenant | `State: CREATED` | `PENDING`, `ERROR`, `IGNORED` |
| OpensearchActionGroup | `State: CREATED` | `PENDING`, `ERROR`, `IGNORED` |
| OpenSearchISMPolicy | `State: CREATED` | `PENDING`, `ERROR`, `IGNORED` |
| OpensearchSnapshotPolicy | `State: CREATED` | `PENDING`, `ERROR`, `IGNORED` |
| OpensearchIndexTemplate | `State: CREATED` | `PENDING`, `ERROR`, `IGNORED` |
| OpensearchComponentTemplate | `State: CREATED` | `PENDING`, `ERROR`, `IGNORED` |

### Legacy Webhook Behavior

Resources using the old API group (`opensearch.opster.io/v1`) have restricted webhook validation:

- ✅ **Allowed**: Status-only updates
- ✅ **Allowed**: Deletion
- ❌ **Denied**: Creation (use new API group instead)
- ❌ **Denied**: Spec changes (use new API group instead)

This ensures that once migration starts, users are guided to use the new API group for any modifications.

## Manual Migration Steps

**Important Note**: You cannot change the API version of existing Kubernetes resources. Once a resource is created with `opensearch.opster.io/v1`, you cannot edit it to use `opensearch.org/v1`. The migration controller automatically handles this by creating new resources with the new API version.

However, if you want to create new resources directly using the new API group (for example, in new deployments or when creating resources from scratch), follow these steps:

### Step 1: Verify Existing Resource Readiness (if migrating existing resources)

If you have existing resources using the old API group, the migration controller will automatically create new resources. Before this happens, ensure all existing resources are in ready status:

```bash
# Check cluster status
kubectl get opensearchclusters.opensearch.opster.io -o jsonpath='{.items[*].status.phase}'

# Check other resources
kubectl get opensearchusers.opensearch.opster.io -o jsonpath='{.items[*].status.state}'
```

The migration controller will wait until resources are ready before creating the new API group resources.

### Step 2: Use New API Version in New Manifests

When creating new resources (not modifying existing ones), use the new API version in your manifest files:

```yaml
apiVersion: opensearch.org/v1
kind: OpenSearchCluster
metadata:
  name: my-cluster
spec:
  # ... your spec
```

**Note**: For existing resources, the migration controller automatically creates corresponding `opensearch.org/v1` resources. You do not need to manually create them.

### Step 3: Update Labels and Annotations (for new resources)

If you're creating new resources and using label selectors, use the new label domain:

| Old Label | New Label |
|-----------|-----------|
| `opster.io/opensearch-cluster` | `opensearch.org/opensearch-cluster` |
| `opster.io/opensearch-nodepool` | `opensearch.org/opensearch-nodepool` |
| `opster.io/opensearch-job` | `opensearch.org/opensearch-job` |

### Step 4: Update Helm Values (for new deployments)

If deploying new resources via Helm, the `opensearch-cluster` chart now defaults to the new API group:

```yaml
# values.yaml
apiGroup: opensearch.org  # Default (recommended)
# apiGroup: opensearch.opster.io  # Legacy (deprecated, only for existing resources)
```

### Step 5: Apply New Resources

For new resources (not existing ones):

```bash
kubectl apply -f your-cluster.yaml
```

**For existing resources**: The migration controller automatically creates the new API group resources. You don't need to manually apply anything.

### Step 6: Verify Migration

Check that both old and new resources exist and are synced:

```bash
# Old API group (existing resources)
kubectl get opensearchclusters.opensearch.opster.io

# New API group (created automatically by migration controller or manually)
kubectl get opensearchclusters.opensearch.org

# Check status sync
kubectl get opensearchclusters.opensearch.opster.io my-cluster -o jsonpath='{.status.phase}'
kubectl get opensearchclusters.opensearch.org my-cluster -o jsonpath='{.status.phase}'
# Both should show the same phase
```

### Step 7: Remove Old Resources (Optional, after migration)

Once you've verified the new resources are working correctly and status is synced, you can optionally remove the old resources:

```bash
# Remove old API resources
# Note: This will only succeed if the new resource exists
kubectl delete opensearchclusters.opensearch.opster.io <cluster-name>
```

**Important**: 
- The old resource can only be deleted if the corresponding new resource exists. This ensures migration has completed successfully.
- Deleting the old resource will not affect the new resource - they operate independently after migration.
- The migration controller automatically handles the deletion of old resources when new resources are deleted.

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

## Deletion Behavior

Understanding how deletion works during migration:

### Scenario 1: Delete New Resource

```bash
kubectl delete opensearchclusters.opensearch.org my-cluster
```

**Result**: The corresponding old resource is automatically deleted as well.

### Scenario 2: Delete Old Resource (Before Migration)

```bash
kubectl delete opensearchclusters.opensearch.opster.io my-cluster
```

**Result**: Deletion is **blocked** if the new resource doesn't exist. The migration controller will prevent deletion until the new resource is created.

### Scenario 3: Delete Old Resource (After Migration)

```bash
kubectl delete opensearchclusters.opensearch.opster.io my-cluster
```

**Result**: Deletion is **allowed** because the new resource exists. The new resource continues to function independently.

## Troubleshooting

### Resources Not Migrating

If automatic migration isn't working:

1. **Check resource readiness**:
   ```bash
   # For clusters
   kubectl get opensearchclusters.opensearch.opster.io <name> -o jsonpath='{.status.phase}'
   # Should be "RUNNING"
   
   # For other resources
   kubectl get opensearchusers.opensearch.opster.io <name> -o jsonpath='{.status.state}'
   # Should be "CREATED"
   ```

2. **Check the operator logs for migration controller**:
   ```bash
   kubectl logs -n opensearch-operator-system deployment/opensearch-operator | grep -i migration
   ```

3. **Look for readiness messages**:
   ```
   "Old resource is not ready, skipping migration"
   ```

4. **Verify both CRDs are installed**:
   ```bash
   kubectl get crd | grep opensearch
   ```

### Status Not Syncing

If status isn't syncing between old and new resources:

1. Check that the migration controller is running:
   ```bash
   kubectl get pods -n opensearch-operator-system | grep opensearch-operator
   ```

2. Verify RBAC permissions for both API groups:
   ```bash
   kubectl get clusterrole opensearch-operator-manager-role -o yaml | grep -A 5 opensearch
   ```

3. Look for errors in the controller logs:
   ```bash
   kubectl logs -n opensearch-operator-system deployment/opensearch-operator | grep -i "status\|sync"
   ```

### Webhook Errors

If you encounter webhook validation errors:

1. **For old API group resources**: Remember that creation and spec changes are denied. Use the new API group instead.

2. **Check webhook configuration**:
   ```bash
   kubectl get validatingwebhookconfigurations | grep opensearch
   ```

3. **Ensure webhook certificates are valid**:
   ```bash
   kubectl get certificates -n opensearch-operator-system
   ```

### Migration Stuck in Pending

If migration is stuck because resources are not ready:

1. **For clusters**: Wait until the cluster reaches `RUNNING` phase
   ```bash
   kubectl get opensearchclusters.opensearch.opster.io <name> -w
   ```

2. **For other resources**: Fix any errors and wait for `CREATED` state
   ```bash
   kubectl describe opensearchusers.opensearch.opster.io <name>
   ```

3. The migration controller will automatically retry every 30 seconds

### Cannot Delete Old Resource

If deletion of old resource is blocked:

1. **Check if new resource exists**:
   ```bash
   kubectl get opensearchclusters.opensearch.org <name>
   ```

2. **If new resource doesn't exist**: Wait for migration to complete, or manually create the new resource

3. **Check migration controller logs** for details:
   ```bash
   kubectl logs -n opensearch-operator-system deployment/opensearch-operator | grep -i "cannot delete"
   ```

## Rollback

If you need to rollback to the old API group:

1. Uninstall operator 3.x and install the operator 2.x

2. Set Helm value to use legacy API:
   ```yaml
   apiGroup: opensearch.opster.io
   ```

3. Or manually change manifests back to `opensearch.opster.io/v1`

**Note**: Once you've migrated to the new API group, we recommend staying on it. The old API group will be removed in a future release.

## Best Practices

1. **Migrate during maintenance windows**: While migration is automatic, plan migrations during low-traffic periods

2. **Verify readiness before migration**: Ensure all resources are in ready status before starting migration

3. **Monitor migration progress**: Watch the operator logs and resource status during migration

4. **Test in non-production first**: Test the migration process in a development or staging environment

5. **Update CI/CD pipelines**: Update any automation to use the new API group

6. **Document your resources**: Keep track of which resources have been migrated

## FAQ

### Q: Will my existing clusters continue to work?

**A**: Yes. The migration controller ensures that existing `opensearch.opster.io` resources continue to function. Changes are automatically synced to `opensearch.org` resources, and status is synced back.

### Q: Do I need to recreate my OpenSearch clusters?

**A**: No. The migration is handled at the Kubernetes resource level. Your actual OpenSearch clusters (pods, data, etc.) are not affected. Only the Kubernetes CustomResource objects are migrated.

### Q: When will the old API group be removed?

**A**: The `opensearch.opster.io` API group will be removed approximately 2-3 releases after the deprecation announcement. Watch release notes for specific dates.

### Q: Can I use both API groups simultaneously?

**A**: Yes, during the deprecation period. However, we recommend migrating to `opensearch.org` to avoid future disruption. Note that:
- Old API group resources can only be updated for status changes
- New API group resources should be used for all spec changes
- Deletion of old resources requires the new resource to exist

### Q: What happens if I delete a new resource?

**A**: The corresponding old resource is automatically deleted as well. This ensures consistency between the two API groups.

### Q: What happens if I try to delete an old resource before migration?

**A**: The deletion will be blocked by the migration controller until the corresponding new resource exists. This prevents accidental data loss.

### Q: How do I update my CI/CD pipelines?

**A**: Update any manifests or Helm values to use `opensearch.org` for **new resources**. The Helm chart defaults to the new API group. For existing resources, the migration controller handles the migration automatically. Update any scripts or automation that create new resources to reference the new API group.

### Q: Can I manually change the API version of an existing resource?

**A**: No. Kubernetes does not allow changing the API version of an existing resource. You cannot edit a resource's API version. The migration controller automatically creates new resources with the new API version based on your existing resources. Once the new resources are created, you can optionally delete the old ones.

### Q: Why is my resource not migrating?

**A**: Check that the resource is in a ready status:
- Clusters must be in `RUNNING` phase
- Other resources must be in `CREATED` state
- Resources in `PENDING`, `ERROR`, or `IGNORED` states will not migrate

### Q: Can I manually create resources in the new API group?

**A**: Yes! You can create resources directly in the new API group. The migration controller only handles syncing from old to new, not the reverse.

## Additional Resources

- [Operator User Guide](main.md)
- [Cluster Chart Documentation](cluster-chart.md)
- [Webhook Configuration](webhooks.md)
- [Operator Development Guide](../../developing.md)
