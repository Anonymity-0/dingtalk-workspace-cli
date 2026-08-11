"""test_eval_poll_validate.py — 验证消费端拒绝伪造的 eval-dispatch 评论。"""

import json
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "scripts", "ci"))
from eval_poll_validate import validate_comment_author, extract_payload


def test_rejects_regular_user_comment():
    """普通用户手写 eval-dispatch 标记的评论必须被拒绝。"""
    comment = {
        "id": 1234,
        "user": {"login": "malicious-user", "type": "User"},
        "body": '<!-- eval-dispatch: {"pr_number":"899","pr_head_sha":"a"*40,"products":"drive","cases_ref":"","run_id":"99999"} -->',
        "performed_via_github_app": None,
    }
    assert validate_comment_author(comment) is False


def test_rejects_wrong_bot_login():
    """非 github-actions[bot] 的 Bot 账号必须被拒绝。"""
    comment = {
        "id": 1235,
        "user": {"login": "some-other-bot[bot]", "type": "Bot"},
        "body": '<!-- eval-dispatch: {"pr_number":"899","pr_head_sha":"a"*40,"products":"drive","cases_ref":"","run_id":"99999"} -->',
        "performed_via_github_app": {"slug": "some-other-app"},
    }
    assert validate_comment_author(comment) is False


def test_rejects_bot_without_app_signature():
    """github-actions[bot] 但缺少 performed_via_github_app 签名。"""
    comment = {
        "id": 1236,
        "user": {"login": "github-actions[bot]", "type": "Bot"},
        "body": '<!-- eval-dispatch: {"pr_number":"899"} -->',
        "performed_via_github_app": None,
    }
    assert validate_comment_author(comment) is False


def test_rejects_bot_with_wrong_app_slug():
    """github-actions[bot] 但 App slug 不匹配。"""
    comment = {
        "id": 1237,
        "user": {"login": "github-actions[bot]", "type": "Bot"},
        "body": '<!-- eval-dispatch: {"pr_number":"899"} -->',
        "performed_via_github_app": {"slug": "dependabot"},
    }
    assert validate_comment_author(comment) is False


def test_accepts_legitimate_github_actions_comment():
    """正确的 github-actions[bot] + github-actions App 签名通过身份校验。"""
    comment = {
        "id": 1238,
        "user": {"login": "github-actions[bot]", "type": "Bot"},
        "body": '<!-- eval-dispatch: {"pr_number":"899","pr_head_sha":"abcd1234","products":"drive","cases_ref":"","run_id":"12345"} -->',
        "performed_via_github_app": {"slug": "github-actions"},
    }
    assert validate_comment_author(comment) is True


def test_extract_payload_valid():
    body = '<!-- eval-dispatch: {"pr_number":"899","products":"drive","run_id":"123"} -->\nsome text'
    payload = extract_payload(body)
    assert payload is not None
    assert payload["pr_number"] == "899"
    assert payload["run_id"] == "123"


def test_extract_payload_no_marker():
    body = "just a regular comment"
    assert extract_payload(body) is None


def test_extract_payload_malformed_json():
    body = "<!-- eval-dispatch: {invalid json} -->"
    assert extract_payload(body) is None


if __name__ == "__main__":
    test_rejects_regular_user_comment()
    test_rejects_wrong_bot_login()
    test_rejects_bot_without_app_signature()
    test_rejects_bot_with_wrong_app_slug()
    test_accepts_legitimate_github_actions_comment()
    test_extract_payload_valid()
    test_extract_payload_no_marker()
    test_extract_payload_malformed_json()
    print("All eval_poll_validate tests passed.")
