import pytest

from eval_comment_parse import parse

SHA = "0123456789abcdef0123456789abcdef01234567"


def test_single_product():
    assert parse(f"/eval drive sha={SHA}") == ("drive", "", SHA)


def test_multiple_products():
    assert parse(f"/eval drive,doc,edu-app sha={SHA}") == ("drive,doc,edu-app", "", SHA)


def test_cases_ref_escape_hatch():
    assert parse(f"/eval drive sha={SHA} cases=feat/drive-latest") == (
        "drive",
        "feat/drive-latest",
        SHA,
    )


def test_extra_lines_after_command_are_ignored():
    body = f"/eval drive sha={SHA}\n\n顺便说明：这个 PR 只动了 drive 的 --latest。"
    assert parse(body) == ("drive", "", SHA)


def test_missing_products_rejected():
    with pytest.raises(ValueError, match="产品集显式必填"):
        parse("/eval")


def test_missing_reviewed_sha_rejected():
    with pytest.raises(ValueError, match="审核 SHA 显式必填"):
        parse("/eval drive")


def test_similar_command_prefix_rejected():
    with pytest.raises(ValueError, match="/eval 命令开头"):
        parse(f"/evaluate drive sha={SHA}")


def test_illegal_product_name_rejected():
    with pytest.raises(ValueError, match="产品名非法"):
        parse(f"/eval drive;rm sha={SHA}")


def test_unknown_extra_token_rejected():
    with pytest.raises(ValueError, match="未知参数"):
        parse(f"/eval drive sha={SHA} --force")


def test_illegal_cases_ref_rejected():
    with pytest.raises(ValueError, match="cases 引用非法"):
        parse(f"/eval drive sha={SHA} cases=$(whoami)")


def test_short_reviewed_sha_rejected():
    with pytest.raises(ValueError, match="审核 SHA 非法"):
        parse("/eval drive sha=0123456")


def test_empty_comment_rejected():
    with pytest.raises(ValueError, match="评论为空"):
        parse("   \n  ")
