# Migrating away from the deprecated auxiliary CRDs

The following CRDs are deprecated and will be removed in v4 of the operator:
`OpensearchUser`, `OpensearchRole`, `OpensearchUserRoleBinding`, `OpensearchActionGroup`, `OpensearchTenant`,
`OpenSearchISMPolicy`, `OpensearchIndexTemplate`, `OpensearchComponentTemplate` and `OpensearchSnapshotPolicy`.

The operator is narrowing its scope to OpenSearch cluster lifecycle management (`OpenSearchCluster`). Configuration that lives
inside OpenSearch (security objects, ISM/snapshot policies, templates) is better managed by tools built for that job.

Until v4 these CRDs keep working unchanged. `kubectl` prints a deprecation warning when you create or update them. Each reconcile also records a Warning event with reason `Deprecated` on the object, visible with `kubectl describe`.

## Replacements

| Deprecated CRD | Terraform / OpenTofu ([`opensearch-project/opensearch`](https://registry.terraform.io/providers/opensearch-project/opensearch/latest/docs)) | OpenSearch REST API | Operator securityconfig secret |
|---|---|---|---|
| `OpensearchUser` | `opensearch_user` | `PUT _plugins/_security/api/internalusers/<name>` | `internal_users.yml` |
| `OpensearchRole` | `opensearch_role` | `PUT _plugins/_security/api/roles/<name>` | `roles.yml` |
| `OpensearchUserRoleBinding` | `opensearch_roles_mapping` | `PUT _plugins/_security/api/rolesmapping/<role>` | `roles_mapping.yml` |
| `OpensearchActionGroup` | none, use the REST API | `PUT _plugins/_security/api/actiongroups/<name>` | `action_groups.yml` |
| `OpensearchTenant` | `opensearch_dashboard_tenant` | `PUT _plugins/_security/api/tenants/<name>` | `tenants.yml` |
| `OpenSearchISMPolicy` | `opensearch_ism_policy` (set `ism_template` to cover existing indices; `opensearch_ism_policy_mapping` is deprecated in the provider) | `PUT _plugins/_ism/policies/<id>` | - |
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

`OpensearchUser` and `OpensearchUserRoleBinding` have no such flag. Deleting an `OpensearchUser` removes the OpenSearch user only when the UID stored on that user matches the CR UID. Re-apply the user with the new tool right away, and supply the password from `spec.passwordFrom` yourself. Deleting an `OpensearchUserRoleBinding` removes that binding's users and backend roles from each roles mapping. If nothing remains on a mapping (users, backend roles, or hosts), the operator deletes the mapping. Re-apply those mappings right away (e.g. `terraform apply`).

If you deploy clusters with the `opensearch-cluster` Helm chart, these CRs are rendered from the `users`, `roles`,
`usersRoleBinding`, `actionGroups`, `tenants`, `ismPolicies`, `indexTemplates` and `componentTemplates` values. Removing
those values deletes the CRs, so apply the steps above before running `helm upgrade` without them.
