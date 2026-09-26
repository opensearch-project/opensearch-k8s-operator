# Migration Guide: opensearch.opster.io to opensearch.org

This guide explains how to migrate your OpenSearch Kubernetes resources from the deprecated `opensearch.opster.io` API group to the new `opensearch.org` API group.

## Overview

The OpenSearch Kubernetes Operator is transitioning from `opensearch.opster.io/v1` to `opensearch.org/v1` API group. This change reflects the project's evolution and alignment with the OpenSearch project branding.

### Timeline

- **Current Release**: Both API groups are supported
- **Deprecation Period**: 2-3 releases (the operator logs a `DEPRECATION WARNING` line when it reconciles legacy resources other than `OpenSearchCluster`; no Kubernetes event or `kubectl` warning is emitted)
- **Future Release**: `opensearch.opster.io` will be removed

## Automatic Migration

The operator includes a migration controller that automatically handles the transition from old to new API groups. The migration process is designed to be seamless and safe.

### How Migration Works

1. **Automatic Resource Creation**: When you have resources using `opensearch.opster.io/v1`, the migration controller automatically creates corresponding `opensearch.org/v1` resources with the same name and namespace. It copies the spec, labels, annotations and status once, at creation time
2. **Readiness Check**: Migration only occurs when the old resource is in a ready status:
   - **Clusters**: Must be in `RUNNING` phase
   - **Other Resources**: Must be in `CREATED` state (not `PENDING`, `ERROR`, or `IGNORED`)
3. **No Spec Sync**: After the new resource exists, the spec is never synced from old to new. The `opensearch.org` resource is the source of truth; make all changes there. The legacy webhooks deny spec changes to old resources
4. **Status Sync**: For `OpenSearchCluster` only, status is synced from the new resource back to the old one (new → old) every 30 seconds. For the other kinds, the old resource's status is copied to the new resource once and is not updated afterwards
5. **Deletion Behavior**:
   - **Deleting new resource** → Automatically deletes the corresponding old resource, once the new resource's own cleanup has finished
   - **Deleting old resource** → Completes only if the corresponding new resource exists. Otherwise the old resource stays `Terminating` (see [Deletion Behavior](#deletion-behavior))

### Migration Annotations and Finalizer

Migrated resources carry every annotation of the old resource plus these annotations to track the migration:

```yaml
metadata:
  annotations:
    opensearch.org/migrated-from: "opensearch.opster.io/v1"
    opensearch.org/migration-timestamp: "2024-01-15T10:30:00Z"
    opensearch.org/source-uid: "original-resource-uid"
```

The migration controller also uses these annotations:

| Annotation | Set on | Meaning |
|------------|--------|---------|
| `opensearch.org/migration-status-pending: "true"` | New resource (all kinds except `OpenSearchCluster`) | The legacy status has not been copied yet. It is removed once the status is written; while it is present the controller retries the copy on every reconcile |
| `opensearch.org/cert-ownership-transferred: "true"` | New `OpenSearchCluster` | The owner references of the generated certificate Secrets (`<cluster>-ca`, `-transport-cert`, `-http-cert`, `-admin-cert`) have been moved to the new cluster, so deleting the old cluster does not garbage-collect them |
| `opensearch.org/deleted-by-new-resource: "true"` | Old resource | The old resource is being deleted because its new twin was deleted; the controller does not recreate the new resource from it |

Both the old resource and its new twin get the `opensearch.org/migration` finalizer, which lets the controller coordinate deletion between the two. A new resource gets it only if a legacy twin exists, so resources created directly in `opensearch.org` are not affected.

### Migration Controller Behavior

The migration controller watches both old and new API groups and handles:

- **Old Resource Events**: Creates the new resource once the old one is ready. It never updates the new resource's spec afterwards
- **New Resource Events**: Deletes the old resource when the new one is deleted
- **Status Synchronization**: Copies `OpenSearchCluster` status from new to old every 30 seconds
- **Finalizer Management**: Adds the `opensearch.org/migration` finalizer to old and new resources to coordinate deletion
- **PVC Label Backfill**: Once the new `OpenSearchCluster` is initialized, adds the `opensearch.org/opensearch-cluster` and `opensearch.org/opensearch-nodepool` labels to PVCs that only carry the 2.x `opster.io/...` labels
- **Certificate Secret Ownership**: Moves the owner references of the generated certificate Secrets from the old cluster to the new one

### One-Time Rolling Restart of Existing Clusters

Upgrading the operator from 2.x to 3.x performs a **one-time rolling restart of every node pool** of each existing cluster, as soon as the migrated `opensearch.org` cluster is first reconciled. No change to your OpenSearch spec is needed to trigger it; the operator upgrade alone does.

Why this happens:

- The StatefulSet selector is immutable, and 3.x selects pods by `opensearch.org/opensearch-cluster` / `opensearch.org/opensearch-nodepool` instead of the 2.x `opster.io/...` labels (`podManagementPolicy`, also immutable, changed as well). The operator relabels the running pods, deletes each node pool's StatefulSet while orphaning its pods, and recreates it with the 3.x spec.
- The 3.x pod template also differs from the 2.x one (annotations, environment, init containers). The adopted pods still carry the 2.x revision hash, so the operator restarts them one at a time to pick up the new template.

The restart follows the regular rolling restart path: one pod at a time, continuing once shard recovery has settled. The cluster is often still `yellow` at that point. Plan it like any other rolling restart:

- Expect the cluster health to go `yellow` while each node restarts.
- Make sure every index has at least one replica, otherwise its shards are unavailable while their node restarts.
- Persistent volume data is kept. A node using emptyDir loses the data on that node when its pod restarts.
- Allow time in proportion to the cluster size; a pool with many nodes or a lot of data can take hours.

For each recreated StatefulSet the operator emits a `Warning` event with reason `StatefulSetRecreated` on the cluster, followed by the usual `RollingRestart` events. If an OpenSearch version upgrade is also in progress, the upgrade reconciler restarts the pods instead, and those show up as upgrade events rather than `RollingRestart` events. Watch them with `kubectl get events -n <namespace> --field-selector involvedObject.name=<cluster-name>`.

### Other Changes When Upgrading from 2.x

Check these settings on existing clusters after upgrading the operator and CRDs. CRD defaults only apply to fields that are missing; a value already stored in the object is kept.

- **Certificate rotation**: `security.tls.transport.rotateDaysBeforeExpiry` and `security.tls.http.rotateDaysBeforeExpiry` now default to `30`, but existing clusters keep their stored value (typically `-1`, rotation disabled). Set it to `30` so generated certificates are rotated before they expire. Do not delete and regenerate the CA Secret in place; that is not a supported rotation procedure.
- **TLS hot reload**: OpenSearch 3.x clusters that omit `enableHotReload` take one extra rolling restart, because the hot-reload setting is added to `opensearch.yml`.
- **SmartScaler**: clusters may have `spec.confMgmt.smartScaler: false` stored even though the default is now `true`. Set it to `true` if you want data nodes drained safely before scale-down.
- **`setVMMaxMapCount: false`**: before 3.0.0 an explicit `false` could be dropped from the stored object. If the field is now missing from a cluster that should not set `vm.max_map_count`, set it again:

  ```bash
  kubectl patch opensearchcluster <name> -n <namespace> --type merge -p '{"spec":{"general":{"setVMMaxMapCount":false}}}'
  ```

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

This ensures that once migration starts, users are guided to use the new API group for any modifications. The legacy webhooks are only registered while legacy API support is enabled; see [Webhooks](webhooks.md#legacy-opensearchopsterio-resources).

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
# Note: The deletion only completes if the new resource exists
kubectl delete opensearchclusters.opensearch.opster.io <cluster-name>
```

**Important**: 
- The old resource is only removed if the corresponding new resource exists. This ensures migration has completed successfully.
- Deleting the old resource does not affect the new resource. The operator removes the old resource's finalizers without any cleanup in OpenSearch.
- The migration controller automatically handles the deletion of old resources when new resources are deleted.

### Step 8: Disable Legacy API Support

Complete this step only after every legacy resource has migrated, its
`opensearch.org` replacement has been verified, and the legacy resource has
been deleted.

> **Warning:** Helm manages the legacy CRDs as regular Helm resources.
> Setting `legacyAPI.enabled=false` during an upgrade removes the
> `opensearch.opster.io` CRDs. Kubernetes then deletes every remaining custom
> resource stored under those CRDs across the cluster.

Check every legacy resource type and namespace before disabling support:

```bash
kubectl api-resources --api-group=opensearch.opster.io -o name |
  while read -r resource; do
    kubectl get "$resource" --all-namespaces
  done
```

If the command reports any resources, finish migrating and deleting them as
described in the preceding steps. When no legacy resources remain, disable
legacy API support:

```bash
helm upgrade <release-name> opensearch-operator/opensearch-operator \
  --namespace <operator-namespace> \
  --reuse-values \
  --set legacyAPI.enabled=false
```

This also disables the legacy migration controllers, validation webhooks, and
RBAC rules. Continue using only `opensearch.org/v1` resources after the
upgrade.

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

**Result**: The operator first finishes the new resource's cleanup (for example removing the user from OpenSearch), then deletes the corresponding old resource as well.

### Scenario 2: Delete Old Resource (Before Migration)

```bash
kubectl delete opensearchclusters.opensearch.opster.io my-cluster
```

**Result**: The delete request is accepted, but the `opensearch.org/migration` finalizer keeps the old resource in `Terminating` because the new resource doesn't exist. The migration controller does not create the new resource from an old resource that is being deleted, so the old resource stays `Terminating` until you either:

- create the `opensearch.org/v1` resource yourself (same name and namespace); the controller then removes the old resource's finalizers, or
- remove the old resource's finalizers manually (`kubectl patch ... --type merge -p '{"metadata":{"finalizers":null}}'`). Only do this if you really want the old resource gone: Kubernetes then garbage-collects the objects it owns, which for an unmigrated `OpenSearchCluster` includes its StatefulSets.

### Scenario 3: Delete Old Resource (After Migration)

```bash
kubectl delete opensearchclusters.opensearch.opster.io my-cluster
```

**Result**: The deletion completes because the new resource exists. The new resource continues to function independently.

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
   kubectl logs -n <operator-namespace> deployment/<fullname> | grep -i migration
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

Only `OpenSearchCluster` status is synced (new → old). If it isn't syncing:

1. Check that the operator is running:
   ```bash
   kubectl get pods -n <operator-namespace> | grep <fullname>
   ```

2. Verify RBAC permissions for both API groups (with `useRoleBindings: true`, check the Role in the operator namespace instead):
   ```bash
   kubectl get clusterrole <fullname> -o yaml | grep -A 5 opensearch
   ```

3. Look for errors in the controller logs:
   ```bash
   kubectl logs -n <operator-namespace> deployment/<fullname> | grep -i "status\|sync"
   ```

Here `<operator-namespace>` is the namespace of the operator's Helm release, and `<fullname>` is the release's full name (`opensearch-operator` for `helm install opensearch-operator ...`; see [Webhooks](webhooks.md#webhook-naming-convention)).

### Webhook Errors

If you encounter webhook validation errors:

1. **For old API group resources**: Remember that creation and spec changes are denied. Use the new API group instead.

2. **Check webhook configuration**:
   ```bash
   kubectl get validatingwebhookconfigurations | grep opensearch
   ```

3. **Ensure webhook certificates are valid**:
   ```bash
   kubectl get certificates -n <operator-namespace>
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

2. **If new resource doesn't exist**: Migration does not start for a resource that is already being deleted. Create the new resource manually, or remove the old resource's finalizers (see [Scenario 2](#scenario-2-delete-old-resource-before-migration))

3. **Check migration controller logs** for details:
   ```bash
   kubectl logs -n <operator-namespace> deployment/<fullname> | grep -i "cannot delete"
   ```

## Rollback

Rolling back from operator 3.x to 2.x is **not supported and has not been tested**. There is no documented rollback procedure.

> **Warning:** With `installCRDs: true` (the default), the operator chart manages the CRDs as regular Helm resources. `helm uninstall` of the 3.x operator therefore deletes the `opensearch.org` and `opensearch.opster.io` CRDs, and Kubernetes then deletes **every** custom resource of those kinds in the cluster, including your `OpenSearchCluster` objects and, through garbage collection, the StatefulSets and other objects they own. Do not uninstall the 3.x operator chart while it manages clusters you want to keep.

Keep in mind as well that after migration the `opensearch.org` resources are the source of truth: changes made there are never written back to the `opensearch.opster.io` resources, and operator 2.x does not know the `opensearch.org` API group. If you must return to 2.x, take snapshots of your data first and rehearse the procedure in a non-production environment.

## Best Practices

1. **Migrate during maintenance windows**: While migration is automatic, the operator upgrade rolling-restarts every OpenSearch node once (see [One-Time Rolling Restart of Existing Clusters](#one-time-rolling-restart-of-existing-clusters)), so plan it during low-traffic periods

2. **Verify readiness before migration**: Ensure all resources are in ready status before starting migration

3. **Monitor migration progress**: Watch the operator logs and resource status during migration

4. **Test in non-production first**: Test the migration process in a development or staging environment

5. **Update CI/CD pipelines**: Update any automation to use the new API group

6. **Document your resources**: Keep track of which resources have been migrated

## FAQ

### Q: Will my existing clusters continue to work?

**A**: Yes. The migration controller creates an `opensearch.org` resource for each ready `opensearch.opster.io` resource, and the operator manages the cluster through it from then on. The spec is copied once; make later changes to the `opensearch.org` resource (the old one rejects spec changes). `OpenSearchCluster` status is synced back to the old resource.

### Q: Do I need to recreate my OpenSearch clusters?

**A**: No. The migration is handled at the Kubernetes resource level, and persistent volume data is kept. A node using emptyDir loses the data on that node when its pod restarts. Upgrading the operator from 2.x to 3.x rolling-restarts every OpenSearch pod once, because each node pool's StatefulSet is recreated with the new `opensearch.org` selector and pod template. See [One-Time Rolling Restart of Existing Clusters](#one-time-rolling-restart-of-existing-clusters).

### Q: When will the old API group be removed?

**A**: The `opensearch.opster.io` API group will be removed approximately 2-3 releases after the deprecation announcement. Watch release notes for specific dates.

### Q: Can I use both API groups simultaneously?

**A**: Yes, during the deprecation period. However, we recommend migrating to `opensearch.org` to avoid future disruption. Note that:
- Old API group resources can only be updated for status changes
- New API group resources should be used for all spec changes
- Deletion of old resources only completes once the new resource exists

### Q: What happens if I delete a new resource?

**A**: The corresponding old resource is automatically deleted as well. This ensures consistency between the two API groups.

### Q: What happens if I try to delete an old resource before migration?

**A**: The old resource stays `Terminating` until the corresponding new resource exists or you remove its finalizers. The migration controller does not migrate a resource that is being deleted, so create the new resource yourself. See [Scenario 2](#scenario-2-delete-old-resource-before-migration).

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

**A**: Yes! You can create resources directly in the new API group. The migration controller only creates new resources from old ones; it never creates or changes old resources from new ones.

## Additional Resources

- [Operator User Guide](main.md)
- [Cluster Chart Documentation](cluster-chart.md)
- [Webhook Configuration](webhooks.md)
- [Operator Development Guide](../developing.md)
