"""Validate chart contribution metadata and the release bot's PR identity."""

import argparse
import json
import os
from pathlib import Path
import re
import sys


REPOSITORY = "opensearch-project/opensearch-k8s-operator"
BOT = "opensearch-ci-bot"
FORK = f"{BOT}/opensearch-k8s-operator"
RELEASE_BRANCH = "release-please--branches--main"
CONVENTIONAL_TITLE = re.compile(
    r"^(feat|fix|perf|refactor|docs|build|ci|test|chore|style)"
    r"(?:\([^()\r\n]+\))?!?: \S[^\r\n]*$"
)


def is_release_pr(pr):
    """Use author, repository, and branch identity, never a contributor-set label."""
    head = pr.get("head") or {}
    base = pr.get("base") or {}
    return (
        (pr.get("user") or {}).get("login") == BOT
        and (head.get("repo") or {}).get("full_name") == FORK
        and head.get("ref") == RELEASE_BRANCH
        and (base.get("repo") or {}).get("full_name") == REPOSITORY
        and base.get("ref") == "main"
    )


def check_version_increment(pr, changed_charts, enabled):
    """Keep the existing policy until release automation is explicitly enabled."""
    if not enabled or is_release_pr(pr):
        return True
    if changed_charts and not CONVENTIONAL_TITLE.fullmatch(pr.get("title", "")):
        raise ValueError(
            "Chart changes need a Conventional Commit PR title, for example "
            "'fix(helm): correct the service selector', 'feat(helm): add an option', "
            "or 'feat(helm)!: remove an option' for a breaking change."
        )
    return False


def release_head(pr):
    """Only allow the privileged docs step to check out the expected bot fork."""
    if not is_release_pr(pr):
        raise ValueError("Refusing to update documentation on an unrecognized release PR")
    sha = pr.get("head", {}).get("sha", "")
    if not re.fullmatch(r"[0-9a-f]{40}", sha):
        raise ValueError("Release PR head must be a full commit SHA")
    return {"repository": FORK, "branch": RELEASE_BRANCH, "sha": sha}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest="command", required=True)
    policy = commands.add_parser("check-pr")
    policy.add_argument("--event", type=Path, required=True)
    policy.add_argument("--charts", default="")
    policy.add_argument("--enabled", choices=("true", "false"), required=True)
    head = commands.add_parser("release-head")
    head.add_argument("--pr", type=Path, required=True)
    args = parser.parse_args()
    try:
        if args.command == "check-pr":
            event = json.loads(args.event.read_text())
            required = check_version_increment(
                event["pull_request"], args.charts.splitlines(), args.enabled == "true"
            )
            outputs = {"check-version-increment": str(required).lower()}
        else:
            outputs = release_head(json.loads(args.pr.read_text()))
        with open(os.environ["GITHUB_OUTPUT"], "a", encoding="utf-8") as output:
            for key, value in outputs.items():
                output.write(f"{key}={value}\n")
    except (ValueError, KeyError, OSError) as error:
        print(str(error), file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
