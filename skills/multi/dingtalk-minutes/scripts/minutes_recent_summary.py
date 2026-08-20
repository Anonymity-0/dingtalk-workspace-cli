#!/usr/bin/env python3
"""
获取最近 N 条听记的 AI 摘要并合并输出

用法:
    python minutes_recent_summary.py          # 最近 5 条
    python minutes_recent_summary.py --max 10 # 最近 10 条
    python minutes_recent_summary.py --output summary.md
    python minutes_recent_summary.py --dry-run
"""

import sys
import json
import subprocess
import argparse
from pathlib import Path
from typing import List, Any, Optional

_scripts_dir = Path(__file__).resolve().parent
if str(_scripts_dir) not in sys.path:
    sys.path.insert(0, str(_scripts_dir))

from minutes_list_parse import uuid_title_pairs_from_payload


class DWSCommandError(RuntimeError):
    """DWS 没有返回可用 JSON；调用方不得把它解释成空业务结果。"""


def run_dws(
    args: List[str], dry_run: bool = False,
) -> Optional[Any]:
    cmd = ['dws'] + args
    if dry_run:
        print(f"[dry-run] {' '.join(cmd)}")
        return None
    try:
        result = subprocess.run(
            cmd, capture_output=True, text=True, timeout=60
        )
    except (subprocess.TimeoutExpired, FileNotFoundError) as exc:
        raise DWSCommandError(str(exc)) from exc
    if result.returncode != 0:
        detail = result.stderr.strip() or f"退出码 {result.returncode}"
        raise DWSCommandError(detail)
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        raise DWSCommandError(f"DWS 返回的不是合法 JSON：{exc}") from exc


def summary_text_from_payload(payload: Any) -> str:
    """兼容当前 Runtime 的 result.fullSummary 与历史直接字段。"""
    if isinstance(payload, str):
        return payload
    if not isinstance(payload, dict):
        return ''
    inner = payload.get('result', payload)
    if isinstance(inner, str):
        return inner
    if not isinstance(inner, dict):
        return ''
    value = (inner.get('fullSummary') or inner.get('summary')
             or inner.get('content'))
    if isinstance(value, str):
        return value
    if value is not None:
        return json.dumps(value, ensure_ascii=False)
    return json.dumps(inner, ensure_ascii=False)


def main():
    parser = argparse.ArgumentParser(
        description='获取最近听记的 AI 摘要'
    )
    parser.add_argument(
        '--max', type=int, default=5, help='获取条数 (默认 5)'
    )
    parser.add_argument(
        '--output', default='', help='输出到 Markdown 文件'
    )
    parser.add_argument('--dry-run', action='store_true')
    args = parser.parse_args()

    print('🎙️ 获取听记列表...')
    list_data = run_dws([
        'minutes', 'list', 'mine',
        '--max', str(args.max),
        '--format', 'json',
    ], dry_run=args.dry_run)

    if args.dry_run:
        run_dws([
            'minutes', 'get', 'summary',
            '--id', '<TASK_UUID>', '--format', 'json',
        ], dry_run=True)
        return

    if not list_data:
        print('未找到听记')
        return

    pairs = uuid_title_pairs_from_payload(list_data)
    if not pairs:
        print('暂无听记')
        return

    output_lines = [f"# 最近 {len(pairs)} 条听记摘要\n"]
    for i, (uuid, title) in enumerate(pairs, 1):
        print(f"  [{i}/{len(pairs)}] 获取摘要: {title}")

        summary_data = run_dws([
            'minutes', 'get', 'summary',
            '--id', uuid, '--format', 'json',
        ])
        summary_text = summary_text_from_payload(summary_data)

        output_lines.append(f"## {i}. {title}\n")
        if summary_text:
            output_lines.append(f"{summary_text}\n")
        else:
            output_lines.append("(暂无摘要)\n")

    full_output = '\n'.join(output_lines)

    if args.output:
        with open(args.output, 'w', encoding='utf-8') as f:
            f.write(full_output)
        print(f"\n✓ 已输出到 {args.output}")
    else:
        print('\n' + full_output)


if __name__ == '__main__':
    try:
        main()
    except DWSCommandError as exc:
        print(f"错误：{exc}", file=sys.stderr)
        sys.exit(1)
