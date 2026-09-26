# OpenSearch API resources

Besides `OpenSearchCluster`, the operator provides custom resources that are reconciled against the OpenSearch REST API of a managed cluster. Each has its own controller in `controllers/` and reconciler in `pkg/reconcilers/`:

| Kind | OpenSearch object | Name in OpenSearch |
|---|---|---|
| `OpensearchUser` | Internal user (security plugin) | `metadata.name` |
| `OpensearchRole` | Role | `metadata.name` |
| `OpensearchUserRoleBinding` | Role mappings | the roles listed in the spec |
| `OpensearchTenant` | Tenant | `metadata.name` |
| `OpensearchActionGroup` | Action group | `metadata.name` |
| `OpenSearchISMPolicy` | ISM policy | `spec.policyId`, default `metadata.name` |
| `OpensearchIndexTemplate` | Composable index template | `spec.name`, default `metadata.name` |
| `OpensearchComponentTemplate` | Component template | `spec.name`, default `metadata.name` |
| `OpensearchSnapshotPolicy` | Snapshot management policy | `spec.policyName` |

## Cluster reference

Every resource has a `spec.opensearchCluster` reference to an `OpenSearchCluster` in the same namespace. The reference is immutable. Changing it could orphan objects in the old cluster or overwrite objects in the new one. This is enforced twice:

* The validating webhook of each kind rejects updates that change `spec.opensearchCluster.name`.
* On first reconciliation the reconciler stores the cluster's UID in `status.managedCluster`. If a later reconciliation finds a different cluster UID (for example because the cluster was deleted and recreated under the same name), it stops with a "cannot change the cluster a ... refers to" error and a Warning event.

## Not overwriting unmanaged objects

The reconcilers must not overwrite or delete OpenSearch objects that were not created through Kubernetes:

* **Roles, tenants, action groups, ISM policies, index templates, component templates and snapshot policies**: on first reconciliation the reconciler checks whether the object already exists in OpenSearch. If it does, this is recorded in the resource status (`existingRole`, `existingTenant`, ...), and the object is left alone: it is neither updated nor deleted when the custom resource is deleted.
* **Users**: when the operator creates a user it stores the UID of the `OpensearchUser` object as a user attribute. Updates and deletes are only performed if that attribute is present and matches. The password comes from a Kubernetes secret referenced in the spec, not from the custom resource itself.
* **Role mappings**: an `OpensearchUserRoleBinding` links users and backend roles to roles in a many-to-many relationship. For each listed role the operator makes sure the role mapping contains the users and backend roles from the resource. The users, backend roles and roles it provisioned are tracked in the resource status; when they are removed from the spec (or the resource is deleted) only those entries are removed from the mapping, and a mapping left empty is deleted. Entries added outside Kubernetes are kept, but role mappings have no ownership marker like users do.
