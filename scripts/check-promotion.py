#!/usr/bin/env python3
"""Require same-repository dev promotions and new versions for main PRs."""
import json
import os
from pathlib import Path
import re
import subprocess

REPOSITORY = "alcxyz/grove"


def semver(value):
    if not re.fullmatch(r"(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)", value):
        raise ValueError("VERSION must contain plain X.Y.Z semver")
    return tuple(map(int, value.split(".")))


def check(event, current, base_version, tag_exists):
    pr = event.get("pull_request")
    if pr is None or pr["base"]["ref"] != "main":
        return
    if pr["head"]["ref"] != "dev" or pr["head"]["repo"]["full_name"] != REPOSITORY:
        raise ValueError("main accepts release promotions from this repository's dev branch only")
    if semver(current) <= semver(base_version):
        raise ValueError("promotion to main requires a VERSION bump")
    if tag_exists:
        raise ValueError("promotion version already has a tag; choose a new version")


def main():
    if os.environ.get("GITHUB_EVENT_NAME") != "pull_request":
        return
    event = json.loads(Path(os.environ["GITHUB_EVENT_PATH"]).read_text())
    pr = event["pull_request"]
    if pr["base"]["ref"] != "main":
        return
    base = pr["base"]["sha"]
    if not re.fullmatch(r"[a-f0-9]{40}", base):
        raise ValueError("invalid base commit")
    current = Path("VERSION").read_text().strip()
    semver(current)
    base_version = subprocess.check_output(
        ["git", "show", f"{base}:VERSION"], text=True).strip()
    status = subprocess.run(["git", "show-ref", "--verify", "--quiet",
                             f"refs/tags/v{current}"]).returncode
    if status not in (0, 1):
        raise ValueError("could not inspect version tag")
    check(event, current, base_version, status == 0)


if __name__ == "__main__":
    try:
        main()
    except (ValueError, OSError, subprocess.CalledProcessError) as error:
        raise SystemExit(str(error))
