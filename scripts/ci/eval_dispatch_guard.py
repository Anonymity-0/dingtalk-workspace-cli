#!/usr/bin/env python3
"""校验 `/eval` 派发的仓库权限与已审核 PR head。"""

import json
import os
import re
import sys


TRUSTED_PERMISSIONS = frozenset({"write", "maintain", "admin"})
FULL_SHA_RE = re.compile(r"^[0-9a-fA-F]{40}$")


def _read_api_response():
    try:
        value = json.load(sys.stdin)
    except (json.JSONDecodeError, OSError) as exc:
        raise ValueError(f"GitHub API response is not valid JSON: {exc}") from exc
    if not isinstance(value, dict):
        raise ValueError("GitHub API response must be a JSON object")
    return value


def require_trusted_permission(response, commenter: str):
    permission = response.get("permission")
    if permission not in TRUSTED_PERMISSIONS:
        raise PermissionError(
            f"{commenter or 'commenter'} does not have write, maintain, or admin permission"
        )
    return permission


def require_reviewed_current_head(response, expected_pr_number: str, reviewed_sha: str):
    try:
        expected_number = int(expected_pr_number)
    except (TypeError, ValueError) as exc:
        raise ValueError("expected PR number is invalid") from exc
    if response.get("number") != expected_number:
        raise ValueError("GitHub API response does not match the requested PR")
    if response.get("state") != "open":
        raise ValueError("PR is not open")
    if not FULL_SHA_RE.fullmatch(reviewed_sha or ""):
        raise ValueError("reviewed SHA must be a full 40-character commit SHA")

    head = response.get("head")
    current_sha = head.get("sha") if isinstance(head, dict) else ""
    if not FULL_SHA_RE.fullmatch(current_sha or ""):
        raise ValueError("GitHub API response does not contain a valid PR head SHA")
    if current_sha.lower() != reviewed_sha.lower():
        raise ValueError(
            f"PR head changed after review: reviewed {reviewed_sha.lower()}, "
            f"current {current_sha.lower()}"
        )
    return reviewed_sha.lower()


def main() -> int:
    mode = sys.argv[1] if len(sys.argv) == 2 else ""
    try:
        response = _read_api_response()
        if mode == "permission":
            permission = require_trusted_permission(response, os.environ.get("COMMENTER", ""))
            print(f"permission={permission}")
            return 0
        if mode == "head":
            head_sha = require_reviewed_current_head(
                response,
                os.environ.get("EXPECTED_PR_NUMBER", ""),
                os.environ.get("REVIEWED_SHA", ""),
            )
            print(f"head_sha={head_sha}")
            return 0
        raise ValueError(f"unknown guard mode: {mode}")
    except (PermissionError, ValueError) as exc:
        print(str(exc), file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
