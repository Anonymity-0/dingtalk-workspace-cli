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

import minutes_recent_summary
from minutes_recent_summary import uuid_title_pairs_from_payload


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

    def test_dws_failure_is_not_treated_as_empty_result(self):
        failed = subprocess.CompletedProcess(
            args=['dws'], returncode=2, stdout='', stderr='boom'
        )
        with mock.patch.object(
            minutes_recent_summary.subprocess, 'run', return_value=failed
        ):
            with self.assertRaises(minutes_recent_summary.DWSCommandError):
                minutes_recent_summary.run_dws(['minutes', 'list', 'mine'])

    def test_summary_dry_run_never_starts_subprocess(self):
        argv = ['minutes_recent_summary.py', '--max', '2', '--dry-run']
        with mock.patch.object(sys, 'argv', argv):
            with mock.patch.object(
                minutes_recent_summary.subprocess,
                'run',
                side_effect=AssertionError('dry-run called subprocess'),
            ):
                output = io.StringIO()
                with redirect_stdout(output):
                    minutes_recent_summary.main()
        rendered = output.getvalue()
        self.assertIn('dws minutes list mine --max 2', rendered)
        self.assertIn(
            'dws minutes get summary --id <TASK_UUID>', rendered
        )

    def test_phase2_execution_capsules_and_capability_boundaries(self):
        skill = SKILL_FILE.read_text(encoding='utf-8')
        intent_guide = (REFERENCES_DIR / 'intent-guide.md').read_text(
            encoding='utf-8'
        )
        minutes_reference = (REFERENCES_DIR / 'minutes.md').read_text(
            encoding='utf-8'
        )
        workflow_reference = (REFERENCES_DIR / '07-minutes.md').read_text(
            encoding='utf-8'
        )

        for command in (
            'dws minutes +search --query "<关键词>" --scope all --page-all',
            'dws minutes +list-mine --page-all --format json',
            'dws minutes +list-shared --page-all --format json',
            'dws minutes +list-all --page-all --format json',
            'dws minutes +export-pack --id <taskUuid>',
            'dws minutes record start --dry-run --format json',
        ):
            self.assertIn(command, skill)
        self.assertNotIn('+record-start --dry-run', skill)

        self.assertIn('--pair "旧词1=>新词1"', intent_guide)
        self.assertIn('total` 是替换规则数', intent_guide)
        self.assertIn('minutes tag query --tag-id <tagId>', intent_guide)
        self.assertNotIn('+replace-batch ...', intent_guide)
        self.assertNotIn('minutes tag ...', intent_guide)

        self.assertIn('无公开 `permission list/get/inspect` 命令', minutes_reference)
        self.assertIn('`0` | 管理员', minutes_reference)
        self.assertIn('`4` | 仅查看', minutes_reference)
        self.assertIn('verification.mode=write_ack_only', minutes_reference)
        self.assertIn('dws minutes +share --id <taskUuid>', workflow_reference)
        self.assertIn('dws minutes +unshare --id <taskUuid>', workflow_reference)


if __name__ == '__main__':
    unittest.main()
