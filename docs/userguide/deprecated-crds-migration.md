# Migrating away from the deprecated auxiliary CRDs

The following CRDs are deprecated and will be removed in v4 of the operator:
`OpensearchUser`, `OpensearchRole`, `OpensearchUserRoleBinding`, `OpensearchActionGroup`, `OpensearchTenant`,
`OpenSearchISMPolicy`, `OpensearchIndexTemplate`, `OpensearchComponentTemplate` and `OpensearchSnapshotPolicy`.

The operator is narrowing its scope to OpenSearch cluster lifecycle management (`OpenSearchCluster`). Configuration that lives
inside OpenSearch (security objects, ISM/snapshot policies, templates) is better managed by tools built for that job.

Until v4 these CRDs keep working unchanged. The API server returns a deprecation warning on every request for them (including `get` and `list`), so `kubectl` and GitOps tools such as Argo CD or Flux show it on each apply or sync. The operator also records a Warning event with reason `Deprecated` on each object, once per spec change, visible with `kubectl describe`.

## Replacements

| Deprecated CRD | Terraform / OpenTofu ([`opensearch-project/opensearch`](https://registry.terraform.io/providers/opensearch-project/opensearch/latest/docs)) | OpenSearch REST API | Operator securityconfig secret |
|---|---|---|---|
| `OpensearchUser` | `opensearch_user` | `PUT _plugins/_security/api/internalusers/<name>` | `internal_users.yml` |
| `OpensearchRole` | `opensearch_role` | `PUT _plugins/_security/api/roles/<name>` | `roles.yml` |
| `OpensearchUserRoleBinding` | `opensearch_roles_mapping` | `PUT _plugins/_security/api/rolesmapping/<role>` | `roles_mapping.yml` |
| `OpensearchActionGroup` | none, use the REST API | `PUT _plugins/_security/api/actiongroups/<name>` | `action_groups.yml` |
| `OpensearchTenant` | `opensearch_dashboard_tenant` | `PUT _plugins/_security/api/tenants/<name>` | `tenants.yml` |
| `OpenSearchISMPolicy` | `opensearch_ism_policy` (`ism_template` only covers indices created later; see below for existing indices) | `PUT _plugins/_ism/policies/<id>` (+ `POST _plugins/_ism/add/<index>` for existing indices) | - |
| `OpensearchIndexTemplate` | `opensearch_composable_index_template` | `PUT _index_template/<name>` | - |
| `OpensearchComponentTemplate` | `opensearch_component_template` | `PUT _component_template/<name>` | - |
| `OpensearchSnapshotPolicy` | `opensearch_sm_policy` | `POST _plugins/_sm/policies/<name>` | - |

- **Terraform / OpenTofu** is the recommended option for GitOps-style management. Each Terraform type in the table supports
  `terraform import`, so objects the operator already created can be adopted without recreating them.
  `OpensearchActionGroup` has no Terraform resource; use the REST API or `action_groups.yml`.
  The import id is the object name in OpenSearch, which is not always `metadata.name`:
  - `OpensearchRole`, `OpensearchUser`, and `OpensearchTenant`: `metadata.name`
  - `OpensearchIndexTemplate` and `OpensearchComponentTemplate`: `spec.name` when set, otherwise `metadata.name`
  - `OpenSearchISMPolicy`: `spec.policyId` when set, otherwise `metadata.name`
  - `OpensearchSnapshotPolicy`: `spec.policyName` when set, otherwise `metadata.name`
  - `OpensearchUserRoleBinding`: one `opensearch_roles_mapping` per role in `spec.roles`, imported by that role name

  Example: `terraform import opensearch_role.sample sample-role`.

  Importing `opensearch_user` does not copy the password. Set `password` or `password_hash` from the secret referenced by `spec.passwordFrom`.

  An `opensearch_roles_mapping` owns the whole mapping of a role. If several `OpensearchUserRoleBinding` CRs grant the same
  role, or the mapping also has entries from elsewhere (for example the securityconfig secret), put all of their users,
  backend roles and hosts into that single resource, or `terraform apply` removes the missing ones.

  `applyToExistingIndices: true` on an `OpenSearchISMPolicy` makes the operator call `POST _plugins/_ism/add/<index>` for the
  indices matching `ismTemplate.indexPatterns`. `ism_template` in `opensearch_ism_policy` does not do this; it only attaches
  the policy to indices created afterwards. Indices the operator already attached keep the policy after the CR is removed.
  To attach the policy to other indices that already exist, call `POST _plugins/_ism/add/<index>` yourself. Do not use
  `opensearch_ism_policy_mapping`, which is deprecated in the provider.
- **REST API** calls can be scripted (e.g. from a Kubernetes `Job` or your CI pipeline). Note that the CRD specs use
  camelCase field names while the API uses the native snake_case bodies.
- **Securityconfig secret**: security objects can be defined in the securityconfig secret referenced by
  `spec.security.config.securityConfigSecret` (see [Securityconfig](main.md#securityconfig)). The secret is applied as a whole,
  so objects created through the API or Dashboards that are not in the secret are overwritten.

If you still have resources in the legacy `opensearch.opster.io` API group, finish the [API group migration](migration-guide.md) first.

## Handing an object over without deleting it from OpenSearch

**Deleting one of these CRs makes the operator delete the corresponding object from OpenSearch** (unless it already existed
before the CR was created). Once the replacement tool manages the object, detach the CR as follows.

For `OpensearchRole`, `OpensearchActionGroup`, `OpensearchTenant`, `OpenSearchISMPolicy`, `OpensearchIndexTemplate`,
`OpensearchComponentTemplate` and `OpensearchSnapshotPolicy`, mark the object as pre-existing (requires kubectl 1.24+). The
operator then stops updating it and leaves it in place when the CR is deleted. For `OpenSearchISMPolicy` and
`OpensearchSnapshotPolicy`, delete the CR immediately after the patch: until it is gone the reconciler logs an error and keeps requeueing.

```bash
# status field per kind: existingRole, existingActionGroup, existingTenant, existingISMPolicy,
# existingIndexTemplate, existingComponentTemplate, existingSnapshotPolicy
kubectl patch opensearchroles.opensearch.org sample-role -n <namespace> \
  --subresource=status --type=merge -p '{"status":{"existingRole":true}}'
kubectl delete opensearchroles.opensearch.org sample-role -n <namespace>
```

`OpensearchUser` and `OpensearchUserRoleBinding` have no such flag.

The operator stores the CR UID in the `k8s-uid` attribute of each user it manages. It only updates a user whose `k8s-uid`
matches the CR, and deleting the CR removes the OpenSearch user only on a match. To hand a user over without downtime,
apply it with the new tool first, with the password from `spec.passwordFrom` and an `attributes` map that does not contain
`k8s-uid` (or `secret-version`). The operator then stops updating the user, and deleting the CR leaves it in place:

```bash
terraform apply   # opensearch_user without the k8s-uid attribute
kubectl delete opensearchusers.opensearch.org sample-user -n <namespace>
```

Deleting an `OpensearchUserRoleBinding` removes that binding's users and backend roles from each roles mapping. If nothing remains on a mapping (users, backend roles, or hosts), the operator deletes the mapping. Re-apply those mappings right away (e.g. `terraform apply`).

If you deploy clusters with the `opensearch-cluster` Helm chart, these CRs are rendered from the `users`, `roles`,
`usersRoleBinding`, `actionGroups`, `tenants`, `ismPolicies`, `indexTemplates` and `componentTemplates` values. Removing
those values deletes the CRs, so apply the steps above before running `helm upgrade` without them.
