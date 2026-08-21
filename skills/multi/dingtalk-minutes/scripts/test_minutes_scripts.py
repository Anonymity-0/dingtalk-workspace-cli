"""Minutes 示例脚本的离线 Contract 回归测试。"""

import io
import re
import subprocess
import sys
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock


SCRIPTS_DIR = Path(__file__).resolve().parent
SKILL_DIR = SCRIPTS_DIR.parent
SKILL_FILE = SKILL_DIR / 'SKILL.md'
REFERENCES_DIR = SKILL_DIR / 'references'
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import minutes_extract_todos
import minutes_recent_summary
from minutes_list_parse import uuid_title_pairs_from_payload


class MinutesScriptContractTest(unittest.TestCase):
    def test_reference_graph_has_no_dead_or_orphaned_files(self):
        markdown_files = [SKILL_FILE, *sorted(REFERENCES_DIR.rglob('*.md'))]
        markdown_link = re.compile(r'\[[^\]]+\]\(([^)]+)\)')
        graph = {path.resolve(): set() for path in markdown_files}
        linked_scripts = set()

        for source in markdown_files:
            for raw_target in markdown_link.findall(
                source.read_text(encoding='utf-8')
            ):
                target = raw_target.split('#', 1)[0].strip()
                if not target or '://' in target or target.startswith('mailto:'):
                    continue
                resolved = (source.parent / target).resolve()
                self.assertTrue(
                    resolved.exists(),
                    f'dead link: {source.relative_to(SKILL_DIR)} -> {target}',
                )
                if resolved in graph:
                    graph[source.resolve()].add(resolved)
                if resolved.parent == SCRIPTS_DIR.resolve():
                    linked_scripts.add(resolved)

        reachable = set()
        pending = [SKILL_FILE.resolve()]
        while pending:
            current = pending.pop()
            if current in reachable:
                continue
            reachable.add(current)
            pending.extend(graph.get(current, ()))

        orphaned = sorted(
            str(path.relative_to(SKILL_DIR))
            for path in graph
            if path != SKILL_FILE.resolve() and path not in reachable
        )
        self.assertEqual(orphaned, [], f'orphan references: {orphaned}')

        production_scripts = {
            path.resolve()
            for path in SCRIPTS_DIR.glob('*.py')
            if not path.name.startswith('test_')
        }
        missing_scripts = sorted(
            path.name for path in production_scripts - linked_scripts
        )
        self.assertEqual(
            missing_scripts, [],
            f'production scripts missing from references: {missing_scripts}',
        )

    def test_list_parser_accepts_runtime_item_list(self):
        payload = {
            'result': {
                'itemList': [{'taskUuid': 'u1', 'title': '周会'}],
            },
        }
        self.assertEqual(
            uuid_title_pairs_from_payload(payload), [('u1', '周会')]
        )

    def test_summary_accepts_runtime_full_summary(self):
        payload = {'result': {'fullSummary': '完整摘要'}}
        self.assertEqual(
            minutes_recent_summary.summary_text_from_payload(payload),
            '完整摘要',
        )

    def test_todos_accept_current_runtime_keys(self):
        self.assertEqual(
            minutes_extract_todos.todo_items_from_payload(
                {'result': {'actions': [{'content': 'A'}]}}
            ),
            [{'content': 'A'}],
        )
        self.assertEqual(
            minutes_extract_todos.todo_items_from_payload(
                {'result': {'dingtalkTodoList': [{'content': 'B'}]}}
            ),
            [{'content': 'B'}],
        )

    def test_dws_failure_is_not_treated_as_empty_result(self):
        failed = subprocess.CompletedProcess(
            args=['dws'], returncode=2, stdout='', stderr='boom'
        )
        with mock.patch.object(
            minutes_recent_summary.subprocess, 'run', return_value=failed
        ):
            with self.assertRaises(minutes_recent_summary.DWSCommandError):
                minutes_recent_summary.run_dws(['minutes', 'list', 'mine'])

    def test_explicit_id_dry_run_never_starts_subprocess(self):
        argv = ['minutes_extract_todos.py', '--id', 'u1', '--dry-run']
        with mock.patch.object(sys, 'argv', argv):
            with mock.patch.object(
                minutes_extract_todos.subprocess,
                'run',
                side_effect=AssertionError('dry-run called subprocess'),
            ):
                output = io.StringIO()
                with redirect_stdout(output):
                    minutes_extract_todos.main()
        self.assertIn('dws minutes get todos --id u1', output.getvalue())


if __name__ == '__main__':
    unittest.main()
