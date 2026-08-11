#!/usr/bin/env python3
"""解析 PR 评论中的 `/eval <products> sha=<full-head-sha> [cases=<ref>]` 触发命令。

供 .github/workflows/eval-dispatch.yml 调用：评论体经环境变量 COMMENT_BODY
传入（避免 argv 注入），解析结果以 GitHub Actions output
（products / cases_ref / reviewed_sha）写出；格式非法时把单行错误写入
output `error` 并以非零退出。

产品集和完整审核 SHA 显式必填；`cases=<ref>` 为用例仓库版本逃生舱
（破坏性变更配对验证）。
"""

import os
import re
import sys

PRODUCT_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")
REF_RE = re.compile(r"^[A-Za-z0-9._/-]+$")
SHA_RE = re.compile(r"^[0-9a-fA-F]{40}$")
USAGE = (
    "用法: /eval <product>[,<product>...] sha=<40位PR-head-SHA> [cases=<ref>]；"
    "示例: /eval drive,doc sha=0123456789abcdef0123456789abcdef01234567 cases=feat/x"
)


def parse(body: str):
    lines = [line.strip() for line in (body or "").splitlines() if line.strip()]
    if not lines:
        raise ValueError(f"评论为空。{USAGE}")
    tokens = lines[0].split()
    if tokens[0] != "/eval":
        raise ValueError(f"首行必须以 /eval 命令开头。{USAGE}")
    if len(tokens) < 2:
        raise ValueError(f"产品集显式必填。{USAGE}")

    products = [p for p in tokens[1].split(",") if p]
    if not products:
        raise ValueError(f"产品集显式必填。{USAGE}")
    for product in products:
        if not PRODUCT_RE.match(product):
            raise ValueError(f"产品名非法: {product}。{USAGE}")

    cases_ref = ""
    reviewed_sha = ""
    for extra in tokens[2:]:
        if extra.startswith("cases="):
            if cases_ref:
                raise ValueError(f"cases 引用只能指定一次。{USAGE}")
            cases_ref = extra[len("cases="):]
            if not cases_ref or not REF_RE.match(cases_ref):
                raise ValueError(f"cases 引用非法: {extra}。{USAGE}")
        elif extra.startswith("sha="):
            if reviewed_sha:
                raise ValueError(f"审核 SHA 只能指定一次。{USAGE}")
            reviewed_sha = extra[len("sha="):]
            if not SHA_RE.match(reviewed_sha):
                raise ValueError(f"审核 SHA 非法: {extra}。{USAGE}")
        else:
            raise ValueError(f"未知参数: {extra}。{USAGE}")

    if not reviewed_sha:
        raise ValueError(f"审核 SHA 显式必填。{USAGE}")

    return ",".join(products), cases_ref, reviewed_sha.lower()


def _write_outputs(pairs):
    lines = [f"{key}={value}" for key, value in pairs]
    output_path = os.environ.get("GITHUB_OUTPUT", "")
    if output_path:
        with open(output_path, "a", encoding="utf-8") as f:
            f.write("\n".join(lines) + "\n")
    print("\n".join(lines))


def main() -> int:
    try:
        products, cases_ref, reviewed_sha = parse(os.environ.get("COMMENT_BODY", ""))
    except ValueError as exc:
        _write_outputs([("error", str(exc))])
        print(str(exc), file=sys.stderr)
        return 1
    _write_outputs(
        [
            ("products", products),
            ("cases_ref", cases_ref),
            ("reviewed_sha", reviewed_sha),
        ]
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
