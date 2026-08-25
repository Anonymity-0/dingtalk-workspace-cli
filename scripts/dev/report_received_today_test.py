#!/usr/bin/env python3
"""Focused regression tests for the packaged Report inbox helper."""

from __future__ import annotations

import importlib.util
import subprocess
import sys
import unittest
from pathlib import Path
from unittest import mock


ROOT = Path(__file__).resolve().parents[2]
sys.dont_write_bytecode = True
SCRIPT = (
    ROOT
    / "skills"
    / "multi"
    / "dingtalk-misc"
    / "scripts"
    / "report_received_today.py"
)
SPEC = importlib.util.spec_from_file_location("report_received_today", SCRIPT)
assert SPEC and SPEC.loader
REPORT = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(REPORT)


def inbox_page(reports: list[dict], *, complete: bool, next_token: int = 0) -> dict:
    return {
        "ok": True,
        "outcome": "success",
        "data": {
            "reports": reports,
            "count": len(reports),
            "complete": complete,
        },
        "meta": {
            "pagination": {
                "endpoint_exhausted": complete,
                "next_token": next_token,
            }
        },
    }


class ReportReceivedTodayTest(unittest.TestCase):
    def test_nonzero_error_keeps_structured_error_and_stderr_bounded(self) -> None:
        result = subprocess.CompletedProcess(
            ["dws"],
            2,
            stdout='{"error":{"code":"permission_denied","message":"denied"}}',
            stderr="diagnostic " + "x" * 10000,
        )
        detail = REPORT.process_error_detail(result)
        self.assertIn("permission_denied", detail)
        self.assertIn("stderr=", detail)
        self.assertLessEqual(len(detail), REPORT.MAX_ERROR_DETAIL_CHARS)

    def test_scan_rejects_invalid_display_limit_before_calling_dws(self) -> None:
        with mock.patch.object(REPORT, "run_dws") as run_dws:
            with self.assertRaisesRegex(REPORT.ReportCommandError, "展示上限"):
                REPORT.scan_inbox(
                    REPORT.datetime.now(REPORT.SHANGHAI),
                    REPORT.datetime.now(REPORT.SHANGHAI),
                    1,
                    display_limit=0,
                )
        run_dws.assert_not_called()

    def test_scan_deduplicates_identical_page_overlap(self) -> None:
        pages = [
            inbox_page(
                [
                    {"reportId": "older", "createTime": 1},
                    {"reportId": "same", "createTime": 2},
                ],
                complete=False,
                next_token=20,
            ),
            inbox_page(
                [{"reportId": "same", "createTime": 2}],
                complete=True,
            ),
        ]
        calls: list[dict] = []

        def fake_run_dws(_args: list[str], **kwargs: object) -> dict:
            calls.append(kwargs)
            return pages.pop(0)

        with mock.patch.object(REPORT, "run_dws", side_effect=fake_run_dws):
            scan = REPORT.scan_inbox(
                REPORT.datetime.now(REPORT.SHANGHAI),
                REPORT.datetime.now(REPORT.SHANGHAI),
                2,
                display_limit=1,
            )
        self.assertEqual(scan.total_count, 2)
        self.assertEqual(
            [item["reportId"] for item in scan.visible_items], ["older"]
        )
        self.assertEqual(len(calls), 2)
        self.assertTrue(all(call["timeout_seconds"] <= 60 for call in calls))

    def test_scan_rejects_conflicting_duplicate(self) -> None:
        pages = [
            inbox_page(
                [{"reportId": "same", "createTime": 1}],
                complete=False,
                next_token=20,
            ),
            inbox_page(
                [{"reportId": "same", "createTime": 2}],
                complete=True,
            ),
        ]
        with mock.patch.object(REPORT, "run_dws", side_effect=pages):
            with self.assertRaisesRegex(REPORT.ReportCommandError, "createTime 冲突"):
                REPORT.scan_inbox(
                    REPORT.datetime.now(REPORT.SHANGHAI),
                    REPORT.datetime.now(REPORT.SHANGHAI),
                    2,
                )


if __name__ == "__main__":
    unittest.main(verbosity=2)
