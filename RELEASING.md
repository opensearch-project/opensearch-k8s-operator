# Release process

Releases are driven by three git tags, each versioned independently:

* `v<operator-version>` (e.g. `v3.0.0`): operator source release. Does not trigger any workflow.
* `opensearch-operator-<operator-chart-version>` (e.g. `opensearch-operator-3.0.12`): operator Helm chart release.
* `opensearch-cluster-<cluster-chart-version>` (e.g. `opensearch-cluster-3.3.5`): cluster Helm chart release.

Each chart tag's version must match the `version` in that chart's `Chart.yaml`. The `Publish Release from tag` workflow fails if they differ.

To release:

1. Update each chart's `Chart.yaml` `version` (and `appVersion` if applicable) and merge it. The PR title must start with `release:` (e.g. `release: bump versions for 3.0.1`); the Helm Lint workflow fails any other PR that changes a chart version. _(Maintainer)_
2. Cut and push the three tags on the release commit ([example request](https://github.com/opensearch-project/.github/issues/657)). _(Admin)_
3. Package each chart, create a pre-release with the `.tgz` attached, open a `gh-pages` PR updating `index.yaml`, and push the chart to GHCR as an OCI artifact (`oci://ghcr.io/opensearch-project/charts/<chart>`). _(Workflow)_
4. Merge both `gh-pages` PRs. The HTTP helm repo is not updated until merged. _(Maintainer)_
5. After the first OCI publish (or if packages are still private), set the GHCR packages `charts/opensearch-operator` and `charts/opensearch-cluster` to public so anonymous `helm pull`/`helm install` work. _(Maintainer)_
6. Edit each pre-release, add notes, and mark it as a normal release. _(Maintainer)_
7. Update the compatibility matrix in the [README](README.md). _(Maintainer)_

Tags and releases are immutable. To fix a mistake, cut a new version.
