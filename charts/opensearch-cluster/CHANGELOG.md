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
## [3.3.5] - 2026-09-22
Chart versions 3.3.1 to 3.3.4 were tagged but never published; their changes are listed here.
### Added
- `tls.http.enabled` and `tls.transport.enabled` to turn TLS off explicitly, e.g. with the security plugin disabled (#1302).
- Commented `initHelper.securityContext` example for the init helper containers (#1461).
### Changed
- **Breaking:** `appVersion` is now the OpenSearch version (`3.4.0`, was the operator version), and `cluster.general.version` and `cluster.dashboards.version` default to it (were `2.3.0`) (#1427, #1594). With default values, upgrading the chart upgrades OpenSearch and Dashboards to 3.4.0.
- `tls.*.enableHotReload` defaults to unset (`~`), which defers to the operator: on for OpenSearch 3.x+, off below (#1460).
### Fixed
- `tls.transport.adminDn` is used again as a deprecated fallback for `adminDn` (#1489).

---
## [3.3.0] - 2026-04-21
### Added
- Ingress per node pool (`cluster.nodePools[].ingress`) (#1317).

---
## [3.2.2] - 2026-03-06
### Fixed
- Set `metadata.namespace` on the resources the chart renders (#1338).

---
## [3.2.1] - 2026-02-25
### Added
- Commented `opensearchHome` and `opensearchDashboardsHome` examples for custom install paths.

---
## [3.2.0] - 2026-02-12
### Added
- `apiGroup` to pick the API group of the rendered resources (#1254).
- `cluster.general.hostNetwork`.
- `tls.*.duration`, `tls.*.rotateDaysBeforeExpiry` and `tls.*.enableHotReload` (#1086) for generated certificates.
- `cluster.general.operatorClusterURL` and `tls.http.customFQDN` (#1147).
- `cluster.dashboards.topologySpreadConstraints` (#1195).
- `cluster.general.monitoring.labels` (#1005).
- `cluster.nodePools[].sidecarContainers` (#1110), and pod annotations for the bootstrap pod and node pools (#1099).
- `cluster.bootstrap.env` and `cluster.nodePools[].additionalConfig` (#1214).
### Changed
- **Breaking:** resources are rendered with `apiVersion: opensearch.org/v1` by default, which needs operator 3.x. Set `apiGroup: opensearch.opster.io` to keep the old group (#1254).
- **Breaking:** `adminDn` is read from `tls.http.adminDn`; `tls.transport.adminDn` is no longer rendered.
- `cluster.confMgmt.smartScaler` defaults to `true` (#1240).
### Deprecated
- `tls.transport.adminDn` (#1207).
### Removed
- **Breaking:** `cluster.bootstrap.additionalConfig`; use `cluster.bootstrap.env` (#1214).
### Fixed
- `cluster.initHelper.image` and `version` are combined into one image reference (#1040).
- Ingress backends use `cluster.general.serviceName` when it is set (#1043).

---
## [3.1.0] - 2025-06-25
### Added
- `dataStream` for index templates (#1007).
### Changed
- Bumped `appVersion` to `2.8.0` (#1037).
### Fixed
- **Breaking:** `cluster.name` is honored; before, the cluster was always named after the release. Releases that set a different `cluster.name` rename their `OpenSearchCluster` on upgrade (#1025).
- Duplicate `image` keys in the rendered `general` and `dashboards` specs (#940).

---
## [3.0.0]
### Added
- Now it is possible to define any configuration that is supported by corresponding CRD by using exactly the same format
as it is defined in the CRD
- Support for all existing CRDs
- Ingress configuration for Opensearch and Dashboards
- Auto-generated README.md file with description for all possible configuration values
### Changed
- `opensearchCluster` variable was replaced by `cluster`. The configuration structure of each custom resource (OpenSearchCluster, OpensearchIndexTemplate, etc) follows the corresponding CRD documentation
### Deprecated
- opensearch-cluster helm chart is a fully refactored chart. Before upgrading to v3 check that [default chart values](../../charts/opensearch-cluster/values.yaml)
  matches with your configuration.
### Removed
### Fixed
### Security

---
## [2.7.0] - 2024-11-05
### Changed
- Bumped `version` and `appVersion` to `2.7.0` (#892).

---
## [2.6.1]
### Added
### Changed
- Updated `version` and `appVersion` to `2.6.1` for the initial release after the helm release decouple.
### Deprecated
### Removed
### Fixed
### Security

[Unreleased]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-cluster-3.3.5...HEAD
[3.3.5]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-cluster-3.3.0...opensearch-cluster-3.3.5
[3.3.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-cluster-3.2.2...opensearch-cluster-3.3.0
[3.2.2]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-cluster-3.2.1...opensearch-cluster-3.2.2
[3.2.1]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-cluster-3.2.0...opensearch-cluster-3.2.1
[3.2.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-cluster-3.1.0...opensearch-cluster-3.2.0
[3.1.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-cluster-3.0.0...opensearch-cluster-3.1.0
[3.0.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-cluster-2.7.0...opensearch-cluster-3.0.0
[2.7.0]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-cluster-2.6.1...opensearch-cluster-2.7.0
[2.6.1]: https://github.com/opensearch-project/opensearch-k8s-operator/releases/tag/opensearch-cluster-2.6.1

