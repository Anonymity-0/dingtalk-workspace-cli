"""test_eval_poll_validate.py — 验证消费端拒绝伪造的 eval-dispatch 评论。"""

import json
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "scripts", "ci"))
from eval_poll_validate import validate_comment_author, extract_payload, validate_run_id


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
    body = '<!-- eval-dispatch: {"pr_number":"899","pr_head_sha":"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2","products":"drive","cases_ref":"","run_id":"123"} -->\nsome text'
    payload = extract_payload(body)
    assert payload is not None
    assert payload["pr_number"] == "899"
    assert payload["run_id"] == "123"
    assert payload["pr_head_sha"] == "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"


def test_extract_payload_no_marker():
    body = "just a regular comment"
    assert extract_payload(body) is None


def test_extract_payload_malformed_json():
    body = "<!-- eval-dispatch: {invalid json} -->"
    assert extract_payload(body) is None


def test_extract_payload_rejects_non_dict_integer():
    """JSON 整数不是合法 payload。"""
    body = "<!-- eval-dispatch: 1 -->"
    assert extract_payload(body) is None


def test_extract_payload_rejects_non_dict_array():
    """JSON 数组不是合法 payload。"""
    body = '<!-- eval-dispatch: [1, 2, 3] -->'
    assert extract_payload(body) is None


def test_extract_payload_rejects_non_dict_string():
    """JSON 字符串不是合法 payload。"""
    body = '<!-- eval-dispatch: "hello" -->'
    assert extract_payload(body) is None


def test_extract_payload_rejects_non_dict_null():
    """JSON null 不是合法 payload。"""
    body = "<!-- eval-dispatch: null -->"
    assert extract_payload(body) is None


def test_extract_payload_rejects_numeric_run_id():
    """run_id 为数值类型时拒绝。"""
    body = '<!-- eval-dispatch: {"pr_number":"899","pr_head_sha":"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2","products":"drive","cases_ref":"","run_id":12345} -->'
    assert extract_payload(body) is None


def test_extract_payload_rejects_numeric_pr_number():
    """pr_number 为数值类型时拒绝。"""
    body = '<!-- eval-dispatch: {"pr_number":899,"pr_head_sha":"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2","products":"drive","cases_ref":"","run_id":"123"} -->'
    assert extract_payload(body) is None


def test_extract_payload_rejects_invalid_sha_format():
    """SHA 非 40 位十六进制时拒绝。"""
    body = '<!-- eval-dispatch: {"pr_number":"899","pr_head_sha":"short","products":"drive","cases_ref":"","run_id":"123"} -->'
    assert extract_payload(body) is None


def test_extract_payload_rejects_invalid_products():
    """products 含非法字符时拒绝。"""
    body = '<!-- eval-dispatch: {"pr_number":"899","pr_head_sha":"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2","products":"drive; rm -rf /","cases_ref":"","run_id":"123"} -->'
    assert extract_payload(body) is None


def test_extract_payload_rejects_missing_required_field():
    """缺少必要字段时拒绝。"""
    body = '<!-- eval-dispatch: {"pr_number":"899","products":"drive","run_id":"123"} -->'
    assert extract_payload(body) is None


def test_validate_run_id_rejects_non_string():
    """run_id 为非字符串类型时拒绝。"""
    assert validate_run_id(12345) is False
    assert validate_run_id(None) is False
    assert validate_run_id(["123"]) is False


def test_validate_run_id_rejects_non_digit_string():
    """run_id 含非数字字符时拒绝。"""
    assert validate_run_id("abc") is False
    assert validate_run_id("123abc") is False
    assert validate_run_id("") is False


if __name__ == "__main__":
    test_rejects_regular_user_comment()
    test_rejects_wrong_bot_login()
    test_rejects_bot_without_app_signature()
    test_rejects_bot_with_wrong_app_slug()
    test_accepts_legitimate_github_actions_comment()
    test_extract_payload_valid()
    test_extract_payload_no_marker()
    test_extract_payload_malformed_json()
    test_extract_payload_rejects_non_dict_integer()
    test_extract_payload_rejects_non_dict_array()
    test_extract_payload_rejects_non_dict_string()
    test_extract_payload_rejects_non_dict_null()
    test_extract_payload_rejects_numeric_run_id()
    test_extract_payload_rejects_numeric_pr_number()
    test_extract_payload_rejects_invalid_sha_format()
    test_extract_payload_rejects_invalid_products()
    test_extract_payload_rejects_missing_required_field()
    test_validate_run_id_rejects_non_string()
    test_validate_run_id_rejects_non_digit_string()
    print("All eval_poll_validate tests passed.")
