import pytest

from eval_comment_parse import parse


def test_single_product():
    assert parse("/eval drive") == ("drive", "")


def test_multiple_products():
    assert parse("/eval drive,doc,edu-app") == ("drive,doc,edu-app", "")


def test_cases_ref_escape_hatch():
    assert parse("/eval drive cases=feat/drive-latest") == ("drive", "feat/drive-latest")


def test_extra_lines_after_command_are_ignored():
    body = "/eval drive\n\n顺便说明：这个 PR 只动了 drive 的 --latest。"
    assert parse(body) == ("drive", "")


def test_missing_products_rejected():
    with pytest.raises(ValueError, match="产品集显式必填"):
        parse("/eval")


def test_similar_command_prefix_rejected():
    with pytest.raises(ValueError, match="/eval 命令开头"):
        parse("/evaluate drive")


def test_illegal_product_name_rejected():
    with pytest.raises(ValueError, match="产品名非法"):
        parse("/eval drive;rm")


def test_unknown_extra_token_rejected():
    with pytest.raises(ValueError, match="未知参数"):
        parse("/eval drive --force")


def test_illegal_cases_ref_rejected():
    with pytest.raises(ValueError, match="cases 引用非法"):
        parse("/eval drive cases=$(whoami)")


def test_empty_comment_rejected():
    with pytest.raises(ValueError, match="评论为空"):
        parse("   \n  ")
