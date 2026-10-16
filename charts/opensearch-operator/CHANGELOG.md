# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---
## [Unreleased]
### Added
### Changed
### Deprecated
### Removed
### Fixed
### Security

---
## [3.0.14] - 2026-09-25
Chart version 3.0.13 was never published; its changes are listed here.
### Changed
- Bumped `appVersion` to `3.0.0` (#1603).
### Fixed
- Removing a master-eligible node now excludes it from the voting configuration first, so a master scale-down, deleting a master pool, or tearing down the bootstrap pod cannot lose quorum. A reverted scale-down clears that exclusion without waiting and re-applies exclusions for nodes that are still being removed. (#1483)

---
## [3.0.12] - 2026-09-22
Chart versions 3.0.3 to 3.0.11 were never published; their changes are listed here.
### Added
- `legacyAPI.enabled` (default `true`). Set it to `false` to skip the deprecated `opensearch.opster.io` CRDs, webhooks, RBAC rules and manager watches (#1438).
- `manager.maxConcurrentReconciles` and `manager.maxConcurrentReconcilesPerController` (#1510).
- CRD fields for: operator authentication to OpenSearch with an mTLS client certificate (#1414), per-node-pool image (#1442), init helper security context (#1461), node-label based shard allocation awareness (#1463), PVC retention policy (#1437).
### Changed
- Bumped `appVersion` to `3.0.0-beta` (#1594).
- **Breaking:** `enableHotReload` is now a tri-state pointer. Omitting it enables TLS certificate hot reload on OpenSearch 3.x+ (and leaves it off on older versions). Existing 3.x clusters that never set the field take one rolling restart on operator upgrade because `plugins.security.ssl.certificates_hot_reload.enabled` is added to `opensearch.yml`. (#1460)
- **Breaking:** StatefulSets now use `Parallel` `podManagementPolicy` (was `OrderedReady`). On operator upgrade, existing STS objects are recreated once with orphan propagation; pods are retained and re-adopted. Scaling and rolling operations remain sequenced by the operator. (#1329)
### Removed
- Experimental parallel recovery mode and related Helm/env config (`manager.parallelRecoveryEnabled` / `PARALLEL_RECOVERY_ENABLED`). (#1329)
### Fixed
- Generated TLS certificates are now rotated 30 days before expiry by default, and expired or unparseable certificates are always regenerated. TLS certificate hot reload is enabled by default on OpenSearch 3.x and above so nodes load renewed certificates without a restart; when hot reload is off (`enableHotReload: false` or OpenSearch < 2.19.1) renewals trigger a rolling restart instead. (#1460)
  Existing CRs keep a stored `rotateDaysBeforeExpiry: -1` until the spec is re-applied — set `30` (or re-apply) to rotate before expiry rather than recovering after it. Replacing the generated CA secret in place is not a supported rotation procedure; leaf reissue cannot keep dual-CA trust during the swap.
- `smartScaler` now defaults to `true` when `confMgmt` is omitted (#1536).

---
## [3.0.2] - 2026-03-23
### Fixed
- `statefulsets/status` RBAC rule for strict RBAC installs (#1358).

---
## [3.0.1] - 2026-03-14
### Fixed
- CRD validation rejected `additionalVolumes` entries that use `persistentVolumeClaim`.

---
## [3.0.0] - 2026-03-11
### Changed
- Bumped `version` to `3.0.0`. Same templates and `appVersion` (`3.0.0-alpha`) as 2.8.4.

---
## [2.8.4] - 2026-03-06
### Fixed
- Set `metadata.namespace` on the namespaced resources the chart renders (#1338).

---
## [2.8.3] - 2026-03-02
### Fixed
- `appVersion` set to `3.0.0-alpha`. 2.8.1 and 2.8.2 deployed the 2.8.0 operator image with the 3.x templates and CRDs.

---
## [2.8.2] - 2026-02-25
### Added
- CRD fields for custom OpenSearch and Dashboards home paths.

---
## [2.8.1] - 2026-02-17
Built from the 3.0.0-alpha chart, so it carries all of its changes, but `appVersion` stayed `2.8.0`. Use 2.8.3 or later.
### Added
- CRD fields for `hostPath` additional volumes (#1312) and the securityconfig update pod (#1270).

---
## [3.0.0-alpha] - 2026-01-26
First chart for operator 3.x. Read the [migration guide](../../docs/userguide/migration-guide.md) before upgrading from 2.x.
### Added
- `opensearch.org/v1` CRDs, installed next to the deprecated `opensearch.opster.io/v1` ones. The operator migrates existing resources to the new group (#1254).
- Validating webhook, enabled by default (`webhook.*`) (#1126).
- `manager.watchNamespace` accepts several namespaces, comma-separated or as a list.
- `manager.metricsBindAddress` (#1112).
- CRD fields for: TLS certificate hot reload (#1086), generated certificate duration and rotation (#1112), disabling SSL, coordinator nodes, NFS (#1091) and PVC (#852) additional volumes, `hostAliases` (#1085), sidecar containers (#1110), `initContainers` (#1143), PVC labels and annotations (#1128), bootstrap pod annotations (#1099) and disk size (#1216), configurable readiness and startup probe commands (#981).
### Changed
- **Breaking:** the webhook gets its certificate from cert-manager by default (`webhook.certManager.enabled: true`). Install cert-manager, or set `webhook.certManager.enabled: false` and provide `webhook.secretName`.
- **Breaking:** resource names and labels changed (#1072). The Deployment is now `<fullname>` with `app.kubernetes.io/*` selector labels (was `<fullname>-controller-manager` with `control-plane: controller-manager`), the default ServiceAccount name is `<fullname>`, and the RBAC templates were merged.
- `useRoleBindings: true` now creates namespaced Roles instead of ClusterRoles (#1183).
- Raised the operator CPU limit to `1000m` and request to `200m`.
- `smartScaler` (#1240) and `setVMMaxMapCount` (#1084) default to `true` in the CRD.
- `additionalConfig` is written to `opensearch.yml` instead of being passed as environment variables; the bootstrap pod takes `env` instead of `additionalConfig` (#1214).
### Removed
- **Breaking:** kube-rbac-proxy sidecar and all `kubeRbacProxy.*` values. The metrics endpoint is protected by controller-runtime's authentication and authorization instead (#1095).
- `namespaces` from the manager ClusterRole.

---
## [2.8.0] - 2025-06-25
### Added
- `OpenSearchSnapshotPolicy` CRD and RBAC (#1018).
### Changed
- Bumped `appVersion` to `2.8.0` (#1037). CRDs regenerated (#1033).

---
## [2.7.0] - 2024-11-05
### Added
- `useRoleBindings` to bind the operator roles with RoleBindings in `manager.watchNamespace` instead of ClusterRoleBindings (#841).
- `podLabels`, `podAnnotations` and `priorityClassName` for the operator pod (#881).
- CRD fields for bootstrap `pluginsList` and `keystore` (#862), projected volumes (#808) and ServiceMonitor labels (#770).
### Changed
- Bumped `appVersion` to `2.7.0` (#892).
- **Breaking:** ISM policy `transitions[].conditions.cron` now nests `expression` and `timezone` under another `cron` key, matching the OpenSearch API (#838).
- kube-rbac-proxy runs with a read-only root filesystem, no privilege escalation and all capabilities dropped (#848).
### Fixed
- ISM policy CRD types for actions (#788).

---
## [2.6.1] - 2024-06-19
### Added
- `manager.pprofEndpointsEnabled` (#813).
### Changed
- Bumped `appVersion` to `2.6.1` (#847).

---
## [2.6.0] - 2024-04-22
### Added
- CRD fields for data streams (#735), CSI additional volumes (#784), role `maskedFields` (#762), probe settings (#728) and init container resources (#628).
### Changed
- Bumped `appVersion` to `2.6.0` (#777).

---
## [2.5.1] - 2024-02-02
### Changed
- Operator image moved to `opensearchproject/opensearch-operator` (#716).
- Bumped `appVersion` to `2.5.1`.

---
## [2.5.0] - 2024-02-02
### Added
- `OpenSearchISMPolicy` (#575), `OpenSearchIndexTemplate` and `OpenSearchComponentTemplate` (#590) CRDs.
- `manager.imagePullSecrets` (#619).
- CRD fields for `subPath` on ConfigMap and Secret volumes.
### Changed
- Bumped `appVersion` to `2.5.0` (#714).
- Operator image moved to `public.ecr.aws/opensearch-project/opensearch-operator` (#710).
- kube-rbac-proxy bumped to `v0.15.0`, with HTTPS health probes on port 10443.

---
## [2.4.0] - 2023-09-04
### Added
- `OpenSearchActionGroup` and `OpenSearchTenant` (#507) CRDs.
- `installCRDs`, `serviceAccount.create`, `serviceAccount.name`, `nameOverride` and `fullnameOverride`, so the chart can be installed in several namespaces (#516).
- `kubeRbacProxy.enable` to run without the kube-rbac-proxy sidecar (#515).
- `manager.loglevel`; development logging is no longer the default.
- `securityContext` and `manager.securityContext` for the operator pod.
- CRD fields for PodDisruptionBudgets per node pool (#547), `emptyDir` additional volumes and extra printer columns (#541).
### Changed
- Bumped `appVersion` to `2.4.0`.
- CRDs moved to `files/` and are rendered by a template.

---
## [2.3.2] - 2023-08-21
### Changed
- Bumped `appVersion` to `2.3.2`.

---
## [2.3.1] - 2023-06-01
### Changed
- Bumped `appVersion` to `2.3.1`.

---
## [2.3.0] - 2023-04-14
### Added
- `manager.extraEnv`.
- CRD fields for pod and container security contexts, snapshot repositories (#370) and Dashboards `pluginsList` (#403).
- RBAC for `servicemonitors` (#475).
### Changed
- Bumped `appVersion` to `2.3.0`.
- kube-rbac-proxy bumped to `v0.12.0`.

---
## [2.2.1] - 2023-02-08
### Added
- `manager.parallelRecoveryEnabled` (default `true`) to turn off the experimental parallel recovery.
### Changed
- Bumped `appVersion` to `2.2.1`.

---
## [2.2.0] - 2023-01-09
### Added
- Added support for custom image used by `kubeRbacProxy`.
- `manager.dnsBase` (default `cluster.local`).
- Liveness and readiness probes, and lower default resource requests and limits, for the operator and kube-rbac-proxy (#319).
- CRD fields for Dashboards `basePath`, bootstrap `additionalConfig` and `backendRoles` in user role bindings (#313).
### Changed
- Bumped `appVersion` to `2.2.0`.
- Synced the manager ClusterRole with the operator (#357).

---
## [2.1.1] - 2022-10-31
### Added
- CRD fields for `initHelper` and keystore `keyMappings`.
### Changed
- Bumped `appVersion` to `2.1.1`.

---
## [2.1.0] - 2022-10-17
### Added
- CRD fields for `additionalVolumes`, affinity, node selectors, tolerations and `topologySpreadConstraints` (#264).
### Changed
- Bumped `appVersion` to `2.1.0`.

---
## [2.0.4] - 2022-08-18
### Added
- CRD field `pluginsList` (#250).
- Manager role can update the status of users, roles and role bindings.

---
## [2.0.3] - 2022-08-16
### Added
- RBAC for the user, role and role binding CRDs.

---
## [2.0.2] - 2022-08-16
### Added
- `OpensearchRole`, `OpensearchUser` and `OpensearchUserRoleBinding` CRDs.

---
## [2.0.1] - 2022-08-15
### Changed
- Bumped `appVersion` to `v2.0.1`.

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

[Unreleased]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-3.0.14...HEAD
[3.0.14]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-3.0.12...opensearch-operator-3.0.14
[3.0.12]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-3.0.2...opensearch-operator-3.0.12
[3.0.2]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-3.0.1...opensearch-operator-3.0.2
[3.0.1]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-3.0.0...opensearch-operator-3.0.1
[3.0.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.8.4...opensearch-operator-3.0.0
[2.8.4]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.8.3...opensearch-operator-2.8.4
[2.8.3]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.8.2...opensearch-operator-2.8.3
[2.8.2]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.8.1...opensearch-operator-2.8.2
[2.8.1]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-3.0.0-alpha...opensearch-operator-2.8.1
[3.0.0-alpha]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.8.0...opensearch-operator-3.0.0-alpha
[2.8.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.7.0...opensearch-operator-2.8.0
[2.7.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.6.1...opensearch-operator-2.7.0
[2.6.1]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.6.0...v2.6.1
[2.6.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.5.1...v2.6.0
[2.5.1]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.5.0...v2.5.1
[2.5.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.4.0...v2.5.0
[2.4.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.3.2...v2.4.0
[2.3.2]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.3.1...v2.3.2
[2.3.1]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.3.0...v2.3.1
[2.3.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.2.1...v2.3.0
[2.2.1]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.2.0...v2.2.1
[2.2.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.1.1...v2.2.0
[2.1.1]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/v2.1.0...v2.1.1
[2.1.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.0.4...v2.1.0
[2.0.4]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.0.3...opensearch-operator-2.0.4
[2.0.3]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.0.2...opensearch-operator-2.0.3
[2.0.2]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.0.1...opensearch-operator-2.0.2
[2.0.1]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.0.0...opensearch-operator-2.0.1
[2.0.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-1.0.3...opensearch-operator-2.0.0
[1.0.3]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-1.0.2...opensearch-operator-1.0.3
[1.0.2]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-1.0.1...opensearch-operator-1.0.2
