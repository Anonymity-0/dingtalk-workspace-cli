"""eval_poll_validate.py — 消费端校验 eval-dispatch 结构化评论的合法性。

三重防伪造校验：
  1. comment.user.login == 'github-actions[bot]' — 平台级身份，普通用户不可冒充
  2. comment.performed_via_github_app.slug == 'github-actions' — GitHub App 签名
  3. payload.run_id 对应本仓库真实成功完成的 workflow run

任何一层不通过即返回 False，拒绝触发内网流水线。
"""

import json
import re
import subprocess
import sys
from typing import Optional


REPO = "DingTalk-Real-AI/dingtalk-workspace-cli"
TRUSTED_BOT_LOGIN = "github-actions[bot]"
TRUSTED_APP_SLUG = "github-actions"


def validate_comment_author(comment: dict) -> bool:
    """校验评论由 github-actions[bot] 通过 GitHub Actions App 发出。"""
    user = comment.get("user", {})
    if user.get("login") != TRUSTED_BOT_LOGIN:
        return False
    if user.get("type") != "Bot":
        return False
    app = comment.get("performed_via_github_app") or {}
    if app.get("slug") != TRUSTED_APP_SLUG:
        return False
    return True


def validate_run_id(run_id: str, repo: str = REPO) -> bool:
    """校验 run_id 对应真实存在且成功完成的 workflow run。"""
    if not run_id or not run_id.isdigit():
        return False
    result = subprocess.run(
        ["gh", "api", f"repos/{repo}/actions/runs/{run_id}",
         "--jq", ".status + \"|\" + .conclusion + \"|\" + .name"],
        capture_output=True, text=True
    )
    if result.returncode != 0:
        return False
    parts = result.stdout.strip().split("|")
    if len(parts) < 3:
        return False
    status, conclusion, name = parts[0], parts[1], parts[2]
    if status != "completed" or conclusion != "success":
        return False
    if "Eval Dispatch" not in name:
        return False
    return True


def validate_pr_head(pr_number: str, expected_sha: str, repo: str = REPO) -> bool:
    """二次校验 PR 仍然 open 且 head SHA 一致（防 TOCTOU）。"""
    result = subprocess.run(
        ["gh", "api", f"repos/{repo}/pulls/{pr_number}",
         "--jq", ".state + \"|\" + .head.sha"],
        capture_output=True, text=True
    )
    if result.returncode != 0:
        return False
    parts = result.stdout.strip().split("|")
    if len(parts) < 2:
        return False
    state, current_sha = parts[0], parts[1]
    if state != "open":
        return False
    if current_sha != expected_sha:
        return False
    return True


def extract_payload(body: str) -> Optional[dict]:
    """从评论 body 提取 eval-dispatch JSON payload。"""
    match = re.search(r"<!-- eval-dispatch: ({.*?}) -->", body)
    if not match:
        return None
    try:
        return json.loads(match.group(1))
    except json.JSONDecodeError:
        return None


def validate_comment(comment: dict, verify_run: bool = True, verify_pr: bool = True) -> Optional[dict]:
    """完整校验流程，通过返回 payload dict，不通过返回 None。"""
    if not validate_comment_author(comment):
        print(f"  REJECT: 评论作者不是可信 bot (id={comment.get('id')})")
        return None

    payload = extract_payload(comment.get("body", ""))
    if not payload:
        return None

    if verify_run:
        run_id = payload.get("run_id", "")
        if not validate_run_id(run_id):
            print(f"  REJECT: run_id={run_id} 不是有效的成功 workflow run")
            return None

    if verify_pr:
        pr_number = payload.get("pr_number", "")
        pr_head_sha = payload.get("pr_head_sha", "")
        if not validate_pr_head(pr_number, pr_head_sha):
            print(f"  REJECT: PR #{pr_number} 非 open 或 head 已变更")
            return None

    return payload


if __name__ == "__main__":
    comment_json = json.load(sys.stdin)
    result = validate_comment(comment_json)
    if result:
        print(json.dumps(result))
        sys.exit(0)
    else:
        sys.exit(1)
