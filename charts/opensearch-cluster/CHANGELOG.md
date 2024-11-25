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

## [2.6.1]
### Added
### Changed
- Updated `version` and `appVersion` to `2.6.1` for the initial release after the helm release decouple.
### Deprecated
### Removed
### Fixed
### Security

[Unreleased]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.6.1...HEAD
[2.6.1]: https://github.com/opensearch-project/opensearch-k8s-operator/compare/opensearch-operator-2.6.0...opensearch-operator-2.6.1

