#!/usr/bin/env python3
"""
从听记中提取所有待办事项并汇总

用法:
    python minutes_extract_todos.py              # 最近 5 条听记
    python minutes_extract_todos.py --max 10     # 最近 10 条
    python minutes_extract_todos.py --id <uuid>  # 指定听记
    python minutes_extract_todos.py --dry-run
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


def todo_items_from_payload(payload: Any) -> List[Any]:
    """兼容当前 Runtime 的 actions/dingtalkTodoList 与历史 todos。"""
    if isinstance(payload, list):
        return payload
    if not isinstance(payload, dict):
        return []
    inner = payload.get('result', payload)
    if isinstance(inner, list):
        return inner
    if not isinstance(inner, dict):
        return []
    for key in ('actions', 'dingtalkTodoList', 'todos'):
        items = inner.get(key)
        if isinstance(items, list):
            return items
    return []


def main():
    parser = argparse.ArgumentParser(
        description='从听记中提取待办事项'
    )
    parser.add_argument('--max', type=int, default=5)
    parser.add_argument('--id', default='', help='指定听记 UUID')
    parser.add_argument('--dry-run', action='store_true')
    args = parser.parse_args()

    if args.dry_run:
        if args.id:
            run_dws([
                'minutes', 'get', 'todos',
                '--id', args.id, '--format', 'json',
            ], dry_run=True)
        else:
            run_dws([
                'minutes', 'list', 'mine',
                '--max', str(args.max), '--format', 'json',
            ], dry_run=True)
            run_dws([
                'minutes', 'get', 'todos',
                '--id', '<TASK_UUID>', '--format', 'json',
            ], dry_run=True)
        return

    uuids_with_titles = []
    if args.id:
        uuids_with_titles = [(args.id, args.id)]
    else:
        print('🎙️ 获取听记列表...')
        data = run_dws([
            'minutes', 'list', 'mine',
            '--max', str(args.max),
            '--format', 'json',
        ])
        if not data:
            return
        uuids_with_titles = uuid_title_pairs_from_payload(data)

    all_todos = []
    for uuid, title in uuids_with_titles:
        print(f"  提取待办: {title}")
        todos_data = run_dws([
            'minutes', 'get', 'todos',
            '--id', uuid, '--format', 'json',
        ])
        if not todos_data:
            continue
        items = todo_items_from_payload(todos_data)
        for t in items:
            if isinstance(t, dict):
                item = dict(t)
                item['_source'] = title
                all_todos.append(item)
            else:
                all_todos.append(t)

    print(f"\n📋 听记待办汇总")
    print('=' * 50)

    if not all_todos:
        print('  ✅ 暂无待办事项')
        return

    for t in all_todos:
        if not isinstance(t, dict):
            print(f"  • {t!r}")
            continue
        content = (t.get('content') or t.get('text')
                   or t.get('title', ''))
        source = t.get('_source', '')
        print(f"  • {content}")
        if source:
            print(f"    来自: {source}")

    print(f"\n合计: {len(all_todos)} 条待办")


if __name__ == '__main__':
    try:
        main()
    except DWSCommandError as exc:
        print(f"错误：{exc}", file=sys.stderr)
        sys.exit(1)
