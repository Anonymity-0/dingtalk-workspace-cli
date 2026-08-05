#!/usr/bin/env python3
"""Deterministic regression checks for the live Chat audit summarizer."""

from __future__ import annotations

from run_chat_shortcut_live_audit import (
    primary_projection_for_message_metrics,
    reaction_message_count,
    thread_message_count,
)


def check(name: str, condition: bool) -> None:
    if not condition:
        raise AssertionError(f"FAIL: {name}")
    print(f"  ok: {name}")


def test_compatibility_aliases_are_counted_once() -> None:
    rows = [{"threadId": "thread-1", "reactions": [{"type": "LIKE"}]}]
    payload = {"messages": rows, "items": rows, "count": 1}
    projection = primary_projection_for_message_metrics(payload)

    check("messages is the canonical projection", projection is rows)
    check("reaction-bearing message counted once", reaction_message_count(projection, upper=True) == 1)
    check("thread-bearing message counted once", thread_message_count(projection, upper=True) == 1)


def test_projection_fallback_order() -> None:
    replies = [{"threadId": "thread-2"}]
    items = [{"threadId": "thread-3"}]
    check(
        "replies precedes items",
        primary_projection_for_message_metrics({"replies": replies, "items": items}) is replies,
    )
    check("items remains a supported fallback", primary_projection_for_message_metrics({"items": items}) is items)


def test_non_envelope_passthrough() -> None:
    rows = [{"threadId": "thread-4"}]
    check("bare list passes through", primary_projection_for_message_metrics(rows) is rows)


def main() -> None:
    tests = [
        test_compatibility_aliases_are_counted_once,
        test_projection_fallback_order,
        test_non_envelope_passthrough,
    ]
    for test in tests:
        print(f"{test.__name__}:")
        test()
    print(f"\nAll {len(tests)} test groups passed.")


if __name__ == "__main__":
    main()
