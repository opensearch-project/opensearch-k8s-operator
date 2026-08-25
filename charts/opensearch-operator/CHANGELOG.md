# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---
## [Unreleased]
### Added
- Added support for custom image used by `kubeRbacProxy`.
### Changed
- `enableHotReload` is now a tri-state pointer. Omitting it enables TLS certificate hot reload on OpenSearch 3.x+ (and leaves it off on older versions). Existing 3.x clusters that never set the field take one rolling restart on operator upgrade because `plugins.security.ssl.certificates_hot_reload.enabled` is added to `opensearch.yml`.
### Deprecated
### Removed
### Fixed
- Generated TLS certificates are now rotated 30 days before expiry by default, and expired or unparseable certificates are always regenerated. TLS certificate hot reload is enabled by default on OpenSearch 3.x and above so nodes load renewed certificates without a restart; when hot reload is off (`enableHotReload: false` or OpenSearch < 2.19.1) renewals trigger a rolling restart instead.
  Existing CRs keep a stored `rotateDaysBeforeExpiry: -1` until the spec is re-applied — set `30` (or re-apply) to rotate before expiry rather than recovering after it. Replacing the generated CA secret in place is not a supported rotation procedure; leaf reissue cannot keep dual-CA trust during the swap.
### Security

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
