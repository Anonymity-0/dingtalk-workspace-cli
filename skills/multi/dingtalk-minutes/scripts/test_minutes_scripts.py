"""Minutes 示例脚本的离线 Contract 回归测试。"""

import io
import subprocess
import sys
import unittest
from contextlib import redirect_stdout
from pathlib import Path
from unittest import mock


SCRIPTS_DIR = Path(__file__).resolve().parent
if str(SCRIPTS_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPTS_DIR))

import minutes_extract_todos
import minutes_recent_summary
from minutes_list_parse import uuid_title_pairs_from_payload


class MinutesScriptContractTest(unittest.TestCase):
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
