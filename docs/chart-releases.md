# Helm chart release preparation

Release Please prepares a combined release PR for two independently versioned charts:

| Chart path | Release tag |
| --- | --- |
| `charts/opensearch-operator` | `opensearch-operator-<version>` |
| `charts/opensearch-cluster` | `opensearch-cluster-<version>` |

The workflow is initially disabled. Until maintainers set the repository variable `ENABLE_CHART_RELEASE_AUTOMATION` to `true`, chart PRs must continue to increment `Chart.yaml.version` and regenerate their README files. The existing chart-testing version check remains enabled during this period. Automation applies to `main`; PRs targeting other branches retain manual version increments.

## Contributing after automation is enabled

Submit chart changes without incrementing `Chart.yaml.version`. Use a Conventional Commit PR title so maintainers can preserve the intended release classification when merging:

| Example title | Version change |
| --- | --- |
| `fix(helm): correct a service selector` | Patch |
| `feat(helm): add a configuration option` | Minor |
| `feat(helm)!: remove a configuration option` | Major |
| `docs(helm): clarify a chart value` | Patch |

The accepted types are `feat`, `fix`, `perf`, `refactor`, `docs`, `build`, `ci`, `test`, `chore`, and `style`. The scope is optional. A `!` before the colon denotes a breaking change; a Conventional Commit `BREAKING CHANGE:` footer also affects version selection. This initial configuration releases documentation and maintenance changes within a chart as patch versions.

Helm linting, schema validation, generated-documentation checks, and other existing tests still run. Chart PRs also need a valid Conventional Commit title. A title edit reruns Helm lint so classification can be corrected without another commit. Only the trusted release PR requires a version increment once automation is enabled.

Continue regenerating chart documentation when changing chart values or templates:

```bash
make -C opensearch-operator helm-docs
```

The release workflow regenerates documentation again after choosing the release versions. Chart `appVersion`, image tags, operator application versions, and OLM bundles remain explicit changes; Release Please only updates chart versions and changelogs.

## Maintainer rollout

Before enabling the repository variable:

1. Agree on release-time versioning and the classification policy above. Squash-merge chart PRs using their classified titles, or ensure the commits retained by another merge method carry the intended Conventional Commit messages. A valid PR title alone does not guarantee that a manually edited merge commit preserves the classification.
2. Check `.release-please-manifest.json` against the latest published chart tags and `Chart.yaml` versions. The initial entries are operator `3.0.14` and cluster `3.3.5`; refresh these if another release has happened before rollout. Review unreleased chart changes since those tags, including older commits without Conventional Commit messages, so the first release does not omit them or choose an incorrect version.
3. Confirm `opensearch-ci-bot` owns its repository fork and `OPENSEARCH_CI_BOT_TOKEN` can push to that fork, open and update upstream PRs, and manage release labels. Use the minimum required repository permissions and the project's credential policy. This credential must trigger normal PR checks; the default `GITHUB_TOKEN` does not trigger those follow-up workflow runs. No cluster credentials are needed.
4. Confirm the bot identity and DCO signoff are approved. Both Release Please and documentation commits use `opensearch-ci-bot <opensearch-infra@amazon.com>` with a `Signed-off-by` trailer. Create or confirm the `autorelease: pending` and `autorelease: tagged` labels.
5. Set `ENABLE_CHART_RELEASE_AUTOMATION=true` and run **Prepare Helm chart release** on `main`, or allow the next push to trigger it. Review the first generated PR before merging it.

The action versions are pinned to commit SHAs. The documentation tool is installed from trusted `main`; the workflow checks the bot author, fork, head branch, and base repository before checking out the release PR at an immutable commit SHA. Documentation commits stage only the two generated chart READMEs.

## Reviewing and publishing a release

After qualifying changes merge to `main`, Release Please creates or updates one PR from `opensearch-ci-bot:release-please--branches--main`. It includes each affected chart's version and `CHANGELOG.md`, the release manifest, and regenerated READMEs. Changes outside the chart directories do not independently trigger a chart release.

1. Review the proposed versions, changelogs, and documentation, and wait for CI. In particular, check breaking changes and any changes whose commit messages may have been misclassified. Release Please supports [release version overrides](https://github.com/googleapis/release-please#how-do-i-change-the-version-number); review overrides before merging.
2. Merge the release PR, preserving its DCO signoffs and generated release body.
3. Create and push each affected chart's existing-format tag at the release PR's merge commit. The **Publish Release from tag** workflow continues to package the chart, create its GitHub prerelease, and propose the `gh-pages` index update. Review and merge the index updates using the existing publication process.
4. Once **all charts in that release PR** have been published successfully, remove `autorelease: pending` and add `autorelease: tagged` on the merged release PR. For example:

   ```bash
   gh pr edit <release-pr-number> \
     --repo opensearch-project/opensearch-k8s-operator \
     --remove-label 'autorelease: pending' \
     --add-label 'autorelease: tagged'
   ```

5. Run **Prepare Helm chart release** again if more changes are waiting, or let the next push trigger it.

This initial workflow uses `skip-github-release: true`; it does not create tags or publish releases. Release Please deliberately stops preparing another release while a merged PR has `autorelease: pending`. Do not remove that label before publication is complete. If publishing fails, fix or retry publishing before marking the PR as tagged.

If documentation generation fails, rerun **Prepare Helm chart release**. It looks up the open bot PR even when Release Please reports no new changes, and skips the documentation commit when the generated content already matches.

To roll back, unset the repository variable or set it to `false`. New chart PR checks will require manual version increments again; rerun checks on open chart PRs. Inspect any open or merged release PR and restore the manifest to the last published versions before a later re-enable. Disabling automation does not undo a merged version change or a published tag.

## Testing changes to the automation

Run the policy tests from the repository root:

```bash
python3 -m unittest discover -s scripts/tests -p 'test_chart_release.py' -v
```

The **Chart release automation tests** workflow also checks out the pinned Release Please action, installs its locked dependencies with lifecycle scripts disabled, and runs `scripts/tests/chart_release_strategy.cjs` with `NODE_PATH` pointing at that checkout's `node_modules`. These tests exercise the actual Helm strategy for one or both charts, breaking changes, other supported commit types, unchanged application versions, and repeated preparation. They also check that the merged release PR uses the branch expected by the identity checks.

Further automation of tags, publication, release-label completion, prerelease version schemes, maintenance branches, application versions, and OLM bundles is outside this initial rollout.
