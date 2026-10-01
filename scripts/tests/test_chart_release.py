import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[2]
sys.path.insert(0, str(ROOT / "scripts"))
import chart_release  # noqa: E402


def release_pr():
    return {
        "title": "chore: release main",
        "user": {"login": chart_release.BOT},
        "head": {
            "ref": chart_release.RELEASE_BRANCH,
            "sha": "a" * 40,
            "repo": {"full_name": chart_release.FORK},
        },
        "base": {"ref": "main", "repo": {"full_name": chart_release.REPOSITORY}},
    }


class ChartReleasePolicyTests(unittest.TestCase):
    def test_disabled_automation_keeps_version_check_and_accepts_existing_titles(self):
        self.assertTrue(chart_release.check_version_increment(
            {"title": "An existing contribution"}, ["charts/opensearch-cluster"], False
        ))

    def test_ordinary_chart_prs_use_release_classification_instead_of_version_bumps(self):
        for charts in (["charts/opensearch-cluster"], ["charts/opensearch-operator"],
                       ["charts/opensearch-cluster", "charts/opensearch-operator"]):
            with self.subTest(charts=charts):
                self.assertFalse(chart_release.check_version_increment(
                    {"title": "fix(helm): correct service labels"}, charts, True
                ))

    def test_all_configured_commit_types_are_accepted(self):
        config = json.loads((ROOT / "release-please-config.json").read_text())
        for section in config["changelog-sections"]:
            with self.subTest(commit_type=section["type"]):
                self.assertFalse(chart_release.check_version_increment(
                    {"title": f'{section["type"]}: update the chart'}, ["charts/example"], True
                ))

    def test_breaking_change_titles_are_accepted(self):
        for title in ("feat!: remove a value", "fix(helm)!: change a default"):
            with self.subTest(title=title):
                self.assertFalse(chart_release.check_version_increment(
                    {"title": title}, ["charts/example"], True
                ))

    def test_invalid_titles_fail_only_for_chart_changes(self):
        for title in ("Update chart", "fix:", "fix: ", "unknown: update", "fix: update\nfeat: another"):
            with self.subTest(title=title):
                with self.assertRaisesRegex(ValueError, "Conventional Commit"):
                    chart_release.check_version_increment({"title": title}, ["charts/example"], True)
                self.assertFalse(chart_release.check_version_increment({"title": title}, [], True))

    def test_release_pr_still_requires_version_increments(self):
        self.assertTrue(chart_release.check_version_increment(
            release_pr(), ["charts/opensearch-cluster"], True
        ))

    def test_bot_name_branch_and_labels_do_not_individually_establish_trust(self):
        changes = (
            ("user", "login", "contributor"),
            ("head", "repo", {"full_name": "contributor/opensearch-k8s-operator"}),
            ("head", "ref", "a-different-branch"),
            ("base", "ref", "release-2.x"),
            ("base", "repo", {"full_name": "another/project"}),
            ("head", "repo", None),
        )
        for section, field, value in changes:
            with self.subTest(section=section, field=field, value=value):
                pr = copy.deepcopy(release_pr())
                pr[section][field] = value
                pr["labels"] = [{"name": "autorelease: pending"}]
                self.assertFalse(chart_release.is_release_pr(pr))
                with self.assertRaisesRegex(ValueError, "unrecognized release PR"):
                    chart_release.release_head(pr)

    def test_docs_checkout_uses_verified_repository_and_immutable_sha(self):
        self.assertEqual(chart_release.release_head(release_pr()), {
            "repository": chart_release.FORK,
            "branch": chart_release.RELEASE_BRANCH,
            "sha": "a" * 40,
        })
        for sha in ("main", "abc123", "a" * 40 + "\nbranch=malicious"):
            pr = release_pr()
            pr["head"]["sha"] = sha
            with self.subTest(sha=sha), self.assertRaisesRegex(ValueError, "full commit SHA"):
                chart_release.release_head(pr)

    def test_cli_writes_boolean_github_output(self):
        with tempfile.TemporaryDirectory() as directory:
            event = Path(directory) / "event.json"
            output = Path(directory) / "output"
            event.write_text(json.dumps({"pull_request": {"title": "fix: change chart"}}))
            result = subprocess.run([
                sys.executable, str(ROOT / "scripts/chart_release.py"), "check-pr",
                "--event", str(event), "--charts", "charts/opensearch-cluster",
                "--enabled", "true",
            ], env={**os.environ, "GITHUB_OUTPUT": str(output)}, capture_output=True, text=True)
            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertEqual(output.read_text(), "check-version-increment=false\n")

    def test_config_tracks_two_independent_charts_with_existing_tag_names(self):
        config = json.loads((ROOT / "release-please-config.json").read_text())
        manifest = json.loads((ROOT / ".release-please-manifest.json").read_text())
        self.assertEqual(set(config["packages"]), set(manifest))
        self.assertEqual(len(manifest), 2)
        self.assertEqual(config["release-type"], "helm")
        self.assertTrue(config["include-component-in-tag"])
        self.assertFalse(config["include-v-in-tag"])
        self.assertEqual(config["tag-separator"], "-")
        self.assertEqual(config["signoff"], "opensearch-ci-bot <opensearch-infra@amazon.com>")
        for path, package in config["packages"].items():
            self.assertEqual(package["component"], Path(path).name)
            self.assertNotIn("extra-files", package)


if __name__ == "__main__":
    unittest.main()
