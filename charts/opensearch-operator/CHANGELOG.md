# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---
## [Unreleased]
### Added
- Added support for custom image used by `kubeRbacProxy`.
- Added `spec.dashboards.tls.rotateDaysBeforeExpiry` to rotate the generated Dashboards certificate before it expires (default `30`, `-1` disables).
### Changed
- `enableHotReload` is now a tri-state pointer. Omitting it enables TLS certificate hot reload on OpenSearch 3.x+ (and leaves it off on older versions). Existing 3.x clusters that never set the field take one rolling restart on operator upgrade because `plugins.security.ssl.certificates_hot_reload.enabled` is added to `opensearch.yml`.
- StatefulSets now use `Parallel` `podManagementPolicy` (was `OrderedReady`). On operator upgrade, existing STS objects are recreated once with orphan propagation; pods are retained and re-adopted. Scaling and rolling operations remain sequenced by the operator.
- Clusters no longer use a dedicated bootstrap pod. Cluster-manager StatefulSet pods form the cluster via `cluster.initial_master_nodes` (pod-0 of each master-eligible pool that has replicas). Once a cluster is initialized, existing StatefulSets keep the value they already have and pools added later omit it. Existing StatefulSets also keep a startup probe that is still the previous authenticated curl command, so the new TCP default applies to new pools. This upgrade does not by itself roll existing nodes. A leftover bare `{cluster}-bootstrap-0` pod is removed only after the cluster is initialized and a non-bootstrap node holds the committed voting configuration. A pod of that name owned by a StatefulSet is left in place.
### Deprecated
- `OpenSearchISMPolicy`, `OpensearchIndexTemplate`, `OpensearchComponentTemplate`, `OpensearchActionGroup`, `OpensearchSnapshotPolicy`, `OpensearchUserRoleBinding`, `OpensearchTenant`, `OpensearchRole`, and `OpensearchUser` CRDs are deprecated and will be removed in v4. The operator is narrowing its scope to core OpenSearch cluster lifecycle management; manage these auxiliary resources with a tool built for declarative OpenSearch config management (e.g. Terraform) instead.
- `spec.bootstrap` is ignored. The field stays in the CRD so existing manifests still apply under strict validation, and will be removed in v4. Configure plugins, keystore, hostAliases, affinity, and initContainers on `spec.general` / `spec.nodePools`.
### Removed
- Experimental parallel recovery mode and related Helm/env config (`manager.parallelRecoveryEnabled` / `PARALLEL_RECOVERY_ENABLED`).
### Fixed
- CRDs are now annotated with `helm.sh/resource-policy: keep`, so `helm uninstall` (or an upgrade with `installCRDs=false` or `legacyAPI.enabled=false`) no longer deletes them along with every custom resource in the cluster. Set the new `crds.keep` value to `false` to restore the old behavior. Helm checks the annotation on the live CRD, so existing releases are protected only after one upgrade to this chart version.
- The generated Dashboards TLS certificate is now renewed when it is expired, unreadable or not signed by the current CA, and the Dashboards pods restart on renewal. It was never renewed before. Clusters with `spec.dashboards.tls.generate: true` take one Dashboards rollout on operator upgrade, because a certificate checksum is added to the pod template.
- Rotation windows are capped at half a generated certificate's lifetime, so a certificate valid for less than `rotateDaysBeforeExpiry` days is no longer reissued on every reconcile.
- Removing a master-eligible node now excludes it from the voting configuration first, so a master scale-down or deleting a master pool cannot lose quorum. A reverted scale-down clears that exclusion without waiting and re-applies exclusions for nodes that are still being removed.
- Generated TLS certificates are now rotated 30 days before expiry by default, and expired or unparseable certificates are always regenerated. TLS certificate hot reload is enabled by default on OpenSearch 3.x and above so nodes load renewed certificates without a restart; when hot reload is off (`enableHotReload: false` or OpenSearch < 2.19.1) renewals trigger a rolling restart instead.
  Existing CRs keep a stored `rotateDaysBeforeExpiry: -1` until the spec is re-applied — set `30` (or re-apply) to rotate before expiry rather than recovering after it. Replacing the generated CA secret in place is not a supported rotation procedure; leaf reissue cannot keep dual-CA trust during the swap.
### Security

---
## [3.0.15]
### Added
- Added `webhook.createConfiguration` (default `true`). Set it to `false`, together with `installCRDs=false` and `useRoleBindings=true`, to install the chart without cluster-wide permissions while a cluster admin applies the CRDs and the `ValidatingWebhookConfiguration` separately.

---
## [2.0.0]
### Added
### Changed
- Modified `version` to `2.0.0` and `appVersion` to `v2.0`.
- Allow chart image tag to pick from `appVersion`, unless explicitly passed `tag` values in `values.yaml` file.
### Deprecated
### Removed
### Fixed
### Security

---
## [1.0.3]
### Added
### Changed
- Added missing spec `dashboards.additionalConfig`
### Deprecated
### Removed
### Fixed
### Security

---
## [1.0.2]
### Added
### Changed
- Added README.md file to charts/ folder.
### Deprecated
### Removed
### Fixed
### Security

---
## [1.0.1]
### Added
### Changed
- Updated version to 1.0.1
### Deprecated
### Removed
### Fixed
### Security

[Unreleased]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.0.0...HEAD
[2.0.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-1.0.3...opensearch-operator-2.0.0
[1.0.3]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-1.0.2...opensearch-operator-1.0.3
[1.0.2]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-1.0.1...opensearch-operator-1.0.2
