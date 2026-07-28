#!/usr/bin/env python3
"""Prepare, contract-check, and score the DWS IM GSB evaluation set."""

from __future__ import annotations

import argparse
import concurrent.futures
import datetime as dt
import hashlib
import json
import re
import shlex
import subprocess
import sys
from collections import defaultdict
from pathlib import Path
from typing import Any, Iterable


CASE_ID_RE = re.compile(r"^(?:DWS-[SCP]|LARK-X)\d{3}$")
TAG_RE = re.compile(r"`(S|C|R|P|H|L|GAP):([^`]+)`")
ALLOWED_STATUSES = {"pass", "fail", "blocked_fixture", "skipped", "not_run"}
CHECK_NAMES = ("selection", "parameters", "safety", "execution")


def utc_now() -> str:
    return dt.datetime.now(dt.timezone.utc).replace(microsecond=0).isoformat()


def pct(numerator: int, denominator: int) -> float:
    return round(100.0 * numerator / denominator, 2) if denominator else 100.0


def rate_text(numerator: int, denominator: int) -> str:
    if denominator == 0:
        return f"{numerator} / {denominator} = N/A"
    return f"{numerator} / {denominator} = {pct(numerator, denominator):.2f}%"


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")


def write_jsonl(path: Path, values: Iterable[dict[str, Any]]) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as handle:
        for value in values:
            handle.write(json.dumps(value, ensure_ascii=False) + "\n")


def read_jsonl(path: Path) -> list[dict[str, Any]]:
    values: list[dict[str, Any]] = []
    for line_no, line in enumerate(path.read_text().splitlines(), start=1):
        if not line.strip():
            continue
        try:
            value = json.loads(line)
        except json.JSONDecodeError as exc:
            raise ValueError(f"{path}:{line_no}: invalid JSON: {exc}") from exc
        if not isinstance(value, dict):
            raise ValueError(f"{path}:{line_no}: each JSONL record must be an object")
        values.append(value)
    return values


def find_repo_root(explicit: str | None) -> Path:
    if explicit:
        root = Path(explicit).expanduser().resolve()
        if not (root / "docs/dws-im-gsb-core-query-set.md").is_file():
            raise ValueError(f"{root} does not contain docs/dws-im-gsb-core-query-set.md")
        return root
    candidates = [Path.cwd().resolve(), Path(__file__).resolve()]
    for candidate in candidates:
        for parent in (candidate, *candidate.parents):
            if (parent / "docs/dws-im-gsb-core-query-set.md").is_file():
                return parent
    raise ValueError("cannot locate the DWS repository; pass --repo-root")


def split_markdown_row(line: str) -> list[str]:
    cells: list[str] = []
    current: list[str] = []
    in_code = False
    escaped = False
    for char in line.strip().strip("|"):
        if escaped:
            current.append(char)
            escaped = False
        elif char == "\\":
            current.append(char)
            escaped = True
        elif char == "`":
            current.append(char)
            in_code = not in_code
        elif char == "|" and not in_code:
            cells.append("".join(current).strip())
            current = []
        else:
            current.append(char)
    cells.append("".join(current).strip())
    return cells


def strip_single_code_span(value: str) -> str:
    value = value.strip()
    if value.startswith("`") and value.endswith("`") and value.count("`") == 2:
        return value[1:-1]
    return value


def suite_for(case_id: str) -> str:
    if case_id.startswith("DWS-S"):
        return "dws_schema"
    if case_id.startswith("DWS-C"):
        return "dws_compat"
    if case_id.startswith("DWS-P"):
        return "dws_smart"
    return "lark_delta"


def parse_query_set(path: Path) -> tuple[list[dict[str, Any]], dict[str, set[str]]]:
    text = path.read_text()
    cases: list[dict[str, Any]] = []
    seen: set[str] = set()
    lines = text.splitlines()
    for line in lines:
        if not line.startswith("|"):
            continue
        cells = split_markdown_row(line)
        if len(cells) != 6 or not CASE_ID_RE.fullmatch(cells[0]):
            continue
        case_id = cells[0]
        if case_id in seen:
            continue
        seen.add(case_id)
        tags = [
            {"namespace": namespace, "value": value.strip()}
            for namespace, value in TAG_RE.findall(cells[5])
        ]
        cases.append(
            {
                "id": case_id,
                "suite": suite_for(case_id),
                "query": cells[1],
                "scenario": cells[2],
                "expected": {
                    "dws": strip_single_code_span(cells[3]),
                    "lark_cli": strip_single_code_span(cells[4]),
                },
                "coverage": tags,
            }
        )
    case_by_id = {case["id"]: case for case in cases}
    for line in lines:
        if not line.startswith("|"):
            continue
        cells = split_markdown_row(line)
        for index in range(0, len(cells) - 1, 2):
            tags = TAG_RE.findall(cells[index])
            target_ids = re.findall(r"(?:DWS-[SCP]|LARK-X)\d{3}", cells[index + 1])
            if len(tags) != 1 or len(target_ids) != 1 or target_ids[0] not in case_by_id:
                continue
            namespace, value = tags[0]
            tag = {"namespace": namespace, "value": value.strip()}
            if tag not in case_by_id[target_ids[0]]["coverage"]:
                case_by_id[target_ids[0]]["coverage"].append(tag)
    cases.sort(key=lambda item: item["id"])
    coverage: dict[str, set[str]] = defaultdict(set)
    for namespace, value in TAG_RE.findall(text):
        coverage[namespace].add(value.strip())
    return cases, coverage


def result_template(case: dict[str, Any]) -> dict[str, Any]:
    platform = "lark" if case["id"].startswith("LARK-") else "dws"
    return {
        "id": case["id"],
        "platform": platform,
        "mode": "contract",
        "status": "not_run",
        "checks": {
            "selection": None,
            "parameters": None,
            "safety": None,
            "execution": None,
        },
        "actual_instruction": "",
        "evidence": "",
        "notes": "",
    }


def prepare(repo_root: Path, out_dir: Path) -> dict[str, Any]:
    query_path = repo_root / "docs/dws-im-gsb-core-query-set.md"
    cases, coverage = parse_query_set(query_path)
    manifest_path = out_dir / "manifest.jsonl"
    golden_path = out_dir / "golden.jsonl"
    template_path = out_dir / "results.template.jsonl"
    write_jsonl(
        manifest_path,
        (
            {"id": case["id"], "suite": case["suite"], "query": case["query"]}
            for case in cases
        ),
    )
    write_jsonl(golden_path, cases)
    write_jsonl(template_path, (result_template(case) for case in cases))
    metadata = {
        "generated_at": utc_now(),
        "repo_root": str(repo_root),
        "query_set": str(query_path),
        "query_count": len(cases),
        "coverage_denominators": {key: len(value) for key, value in sorted(coverage.items())},
        "manifest": str(manifest_path),
        "golden": str(golden_path),
        "results_template": str(template_path),
    }
    write_json(out_dir / "run.json", metadata)
    return metadata


def run(command: list[str], cwd: Path, timeout: int = 30) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        command,
        cwd=cwd,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        timeout=timeout,
        check=False,
    )


def run_json(command: list[str], cwd: Path, timeout: int = 30) -> Any:
    result = run(command, cwd, timeout)
    if result.returncode != 0:
        detail = (result.stderr or result.stdout).strip()
        raise RuntimeError(f"{shlex.join(command)} failed ({result.returncode}): {detail[:800]}")
    try:
        return json.loads(result.stdout)
    except json.JSONDecodeError as exc:
        raise RuntimeError(f"{shlex.join(command)} did not return JSON") from exc


def check_help(binary: str, path: str, cwd: Path) -> tuple[bool, str]:
    command = [binary, *shlex.split(path), "--help"]
    try:
        result = run(command, cwd, timeout=15)
    except (OSError, subprocess.TimeoutExpired) as exc:
        return False, str(exc)
    output = result.stdout + result.stderr
    expected_usage = " ".join([Path(binary).name, path])
    ok = result.returncode == 0 and "Usage:" in output and expected_usage in output
    return ok, "" if ok else output.strip()[:500]


def source_chat_shortcuts(repo_root: Path) -> set[str]:
    commands: set[str] = set()
    roots = [repo_root / "internal/shortcut/chat"]
    for root in roots:
        for path in root.rglob("*.go"):
            commands.update(re.findall(r'Command:\s*"(\+[^"]+)"', path.read_text(errors="ignore")))
    smart = repo_root / "internal/shortcut/smart"
    for path in smart.rglob("*.go"):
        text = path.read_text(errors="ignore")
        if re.search(r'Service:\s*"chat"', text):
            commands.update(re.findall(r'Command:\s*"(\+[^"]+)"', text))
    return {f"chat {command}" for command in commands}


def lark_skill_capabilities(skill_root: Path, expected: set[str]) -> set[str]:
    corpus = "\n".join(
        path.read_text(errors="ignore") for path in skill_root.rglob("*.md")
    )
    found: set[str] = set()
    for capability in expected:
        needle = capability[3:] if capability.startswith("im.") else capability
        if needle in corpus:
            found.add(capability)
    return found


def lark_cli_capabilities(binary: str, expected: set[str], cwd: Path) -> tuple[set[str], dict[str, str]]:
    found: set[str] = set()
    errors: dict[str, str] = {}

    def inspect(capability: str) -> tuple[str, bool, str]:
        if capability.startswith("im +"):
            command = [binary, "im", capability[3:], "--help"]
        else:
            command = [binary, "schema", capability]
        try:
            result = run(command, cwd, timeout=20)
        except (OSError, subprocess.TimeoutExpired) as exc:
            return capability, False, str(exc)
        output = result.stdout + result.stderr
        ok = result.returncode == 0 and (
            (
                "Usage:" in output
                and f"{Path(binary).name} {capability}" in output
                and capability.startswith("im +")
            )
            or ('"inputSchema"' in output and not capability.startswith("im +"))
        )
        return capability, ok, "" if ok else output.strip()[:500]

    # lark-cli refreshes and locks shared discovery/auth caches during startup.
    # Parallel schema/help probes can serialize behind that lock and exceed the
    # per-command timeout even though every path succeeds immediately alone.
    # Keep these read-only probes sequential so a local lock-contention artifact
    # is never reported as a missing Lark capability.
    for capability in sorted(expected):
        capability, ok, detail = inspect(capability)
        if ok:
            found.add(capability)
        else:
            errors[capability] = detail
    return found, errors


def parse_fixture_ids(path: Path) -> set[str]:
    text = path.read_text()
    section = text.split("## 7. Query → Fixture 覆盖映射", 1)[-1]
    section = section.split("## 8. 清理顺序", 1)[0]
    ids: set[str] = set()
    pattern = re.compile(
        r"(?:(DWS-)?([SCP])|(LARK-)?(X))(\d{3})"
        r"(?:[–-](?:(?:DWS-)?([SCP])|(?:LARK-)?(X))?(\d{3}))?"
    )
    for match in pattern.finditer(section):
        family = match.group(2) or match.group(4)
        start = int(match.group(5))
        end_family = match.group(6) or match.group(7) or family
        end = int(match.group(8)) if match.group(8) else start
        if end_family != family or end < start:
            continue
        prefix = "LARK-X" if family == "X" else f"DWS-{family}"
        ids.update(f"{prefix}{value:03d}" for value in range(start, end + 1))
    return ids


def checksum_status(repo_root: Path, fixture_root: Path) -> tuple[int, int, list[str]]:
    checksum_file = fixture_root / "checksums.sha256"
    if not checksum_file.is_file():
        return 0, 0, ["checksums.sha256 missing"]
    total = 0
    passed = 0
    failures: list[str] = []
    for line in checksum_file.read_text().splitlines():
        if not line.strip():
            continue
        total += 1
        digest, name = line.split(maxsplit=1)
        relative_name = name.lstrip("*")
        target = (
            repo_root / relative_name
            if "/" in relative_name
            else fixture_root / relative_name
        )
        if not target.is_file():
            failures.append(f"{name}: missing")
            continue
        actual = hashlib.sha256(target.read_bytes()).hexdigest()
        if actual == digest:
            passed += 1
        else:
            failures.append(f"{name}: checksum mismatch")
    return passed, total, failures


def metric(expected: set[str], actual: set[str]) -> dict[str, Any]:
    covered = expected & actual
    return {
        "covered": len(covered),
        "denominator": len(actual),
        "rate": pct(len(covered), len(actual)),
        "missing_from_eval": sorted(actual - expected),
        "stale_expectations": sorted(expected - actual),
    }


def expected_metric(expected: set[str], found: set[str]) -> dict[str, Any]:
    covered = expected & found
    return {
        "covered": len(covered),
        "denominator": len(expected),
        "rate": pct(len(covered), len(expected)),
        "missing_from_eval": [],
        "stale_expectations": sorted(expected - found),
    }


def contract(repo_root: Path, out_dir: Path, dws: str, lark_cli: str, lark_skill: Path) -> dict[str, Any]:
    query_path = repo_root / "docs/dws-im-gsb-core-query-set.md"
    fixture_plan = repo_root / "docs/dws-im-gsb-fixture-plan.md"
    cases, expected = parse_query_set(query_path)

    schema = run_json([dws, "schema", "--all", "-f", "json"], repo_root, timeout=60)
    chat_product = next(product for product in schema["products"] if product["id"] == "chat")
    actual_schema = {tool["canonical_path"] for tool in chat_product["tools"]}

    shortcuts = run_json(
        [dws, "shortcut", "list", "--service", "chat", "--format", "json"],
        repo_root,
        timeout=30,
    )
    actual_public_shortcuts = {item["cli_path"] for item in shortcuts["shortcuts"]}

    exclusions = json.loads((repo_root / "internal/cli/schema_command_exclusions.json").read_text())
    compat_group = next(
        group for group in exclusions["groups"] if group["id"] == "compatibility-helpers-pending-review"
    )
    actual_compat = {path for path in compat_group["commands"] if path.startswith("chat ")}

    expected_runnable = expected.get("R", set())
    help_paths = expected.get("C", set()) | expected_runnable
    with concurrent.futures.ThreadPoolExecutor(max_workers=8) as executor:
        help_results = dict(
            zip(
                sorted(help_paths),
                executor.map(
                    lambda path: check_help(dws, path, repo_root),
                    sorted(help_paths),
                ),
            )
        )
    help_valid = {path for path, (ok, _) in help_results.items() if ok}

    actual_source_shortcuts = source_chat_shortcuts(repo_root)
    expected_shortcut_semantics = expected.get("P", set()) | expected.get("H", set())

    lark_skill_found = lark_skill_capabilities(lark_skill, expected.get("L", set()))
    lark_cli_found, lark_cli_errors = lark_cli_capabilities(
        lark_cli, expected.get("L", set()), repo_root
    )

    fixture_ids = parse_fixture_ids(fixture_plan)
    case_ids = {case["id"] for case in cases}
    checksums_passed, checksums_total, checksum_failures = checksum_status(
        repo_root,
        repo_root / "docs/fixtures/im-gsb"
    )

    schema_metric = metric(expected.get("S", set()), actual_schema)
    compat_metric = metric(expected.get("C", set()), actual_compat)
    public_metric = metric(expected.get("P", set()), actual_public_shortcuts)
    runnable_metric = {
        "covered": len(expected_runnable & help_valid),
        "denominator": len(expected_runnable),
        "rate": pct(len(expected_runnable & help_valid), len(expected_runnable)),
        "missing_from_eval": [],
        "stale_expectations": sorted(expected_runnable - help_valid),
    }
    dws_covered = (
        schema_metric["covered"]
        + compat_metric["covered"]
        + public_metric["covered"]
        + runnable_metric["covered"]
    )
    dws_denominator = (
        schema_metric["denominator"]
        + compat_metric["denominator"]
        + public_metric["denominator"]
        + runnable_metric["denominator"]
    )

    report: dict[str, Any] = {
        "generated_at": utc_now(),
        "query_set": str(query_path),
        "query_count": len(cases),
        "query_ids": sorted(case_ids),
        "expected_counts": {key: len(value) for key, value in sorted(expected.items())},
        "dws": {
            "delivered_surface": {
                "covered": dws_covered,
                "denominator": dws_denominator,
                "rate": pct(dws_covered, dws_denominator),
            },
            "schema": schema_metric,
            "compatibility": compat_metric,
            "runnable_parent": runnable_metric,
            "published_shortcuts": public_metric,
            "help": {
                "passed": len(help_valid),
                "denominator": len(help_paths),
                "rate": pct(len(help_valid), len(help_paths)),
                "failures": {
                    path: detail for path, (ok, detail) in help_results.items() if not ok
                },
            },
            "source_shortcut_semantics": metric(
                expected_shortcut_semantics, actual_source_shortcuts
            ),
        },
        "lark": {
            "skill_semantics": expected_metric(expected.get("L", set()), lark_skill_found),
            "cli_executable": expected_metric(expected.get("L", set()), lark_cli_found),
            "cli_errors": lark_cli_errors,
        },
        "fixtures": {
            "query_mapping": {
                "covered": len(case_ids & fixture_ids),
                "denominator": len(case_ids),
                "rate": pct(len(case_ids & fixture_ids), len(case_ids)),
                "missing": sorted(case_ids - fixture_ids),
                "extra": sorted(fixture_ids - case_ids),
            },
            "checksums": {
                "passed": checksums_passed,
                "denominator": checksums_total,
                "rate": pct(checksums_passed, checksums_total),
                "failures": checksum_failures,
            },
        },
    }
    write_json(out_dir / "contract-coverage.json", report)
    (out_dir / "contract-coverage.md").write_text(render_contract_markdown(report))
    return report


def render_contract_markdown(report: dict[str, Any]) -> str:
    dws = report["dws"]
    lark = report["lark"]
    fixtures = report["fixtures"]
    lines = [
        "# DWS IM GSB 契约覆盖率报告",
        "",
        f"- 生成时间：`{report['generated_at']}`",
        f"- Query 集：`{report['query_set']}`",
        f"- 唯一 Query：{report['query_count']}",
        "",
        "## 汇总",
        "",
        "| 指标 | 覆盖率 |",
        "|---|---:|",
        f"| DWS 当前交付面 | {rate_text(dws['delivered_surface']['covered'], dws['delivered_surface']['denominator'])} |",
        f"| DWS Schema | {rate_text(dws['schema']['covered'], dws['schema']['denominator'])} |",
        f"| DWS 兼容/辅助路径 | {rate_text(dws['compatibility']['covered'], dws['compatibility']['denominator'])} |",
        f"| DWS Runnable Parent | {rate_text(dws['runnable_parent']['covered'], dws['runnable_parent']['denominator'])} |",
        f"| DWS 已发布 Shortcut | {rate_text(dws['published_shortcuts']['covered'], dws['published_shortcuts']['denominator'])} |",
        f"| DWS Shortcut 源码语义 | {rate_text(dws['source_shortcut_semantics']['covered'], dws['source_shortcut_semantics']['denominator'])} |",
        f"| Lark Skill 语义 | {rate_text(lark['skill_semantics']['covered'], lark['skill_semantics']['denominator'])} |",
        f"| Lark CLI 当前可执行 | {rate_text(lark['cli_executable']['covered'], lark['cli_executable']['denominator'])} |",
        f"| Query → Fixture 映射 | {rate_text(fixtures['query_mapping']['covered'], fixtures['query_mapping']['denominator'])} |",
        f"| Fixture SHA-256 | {rate_text(fixtures['checksums']['passed'], fixtures['checksums']['denominator'])} |",
        "",
        "## 漂移与缺口",
        "",
    ]
    issues: list[str] = []
    for label, value in (
        ("DWS Schema", dws["schema"]),
        ("DWS 兼容路径", dws["compatibility"]),
        ("DWS 已发布 Shortcut", dws["published_shortcuts"]),
        ("DWS Shortcut 源码", dws["source_shortcut_semantics"]),
        ("Lark Skill", lark["skill_semantics"]),
    ):
        if value["missing_from_eval"]:
            issues.append(f"- {label} 新增但评测未覆盖：`{', '.join(value['missing_from_eval'])}`")
        if value["stale_expectations"]:
            issues.append(f"- {label} 已从当前契约消失：`{', '.join(value['stale_expectations'])}`")
    if dws["help"]["failures"]:
        issues.append(f"- DWS Help 失败路径：{len(dws['help']['failures'])} 条。")
    if fixtures["query_mapping"]["missing"]:
        issues.append(
            f"- 缺少 Fixture 映射：`{', '.join(fixtures['query_mapping']['missing'])}`"
        )
    if fixtures["checksums"]["failures"]:
        issues.append(f"- Fixture 校验失败：{len(fixtures['checksums']['failures'])} 个文件。")
    if lark["cli_executable"]["covered"] != lark["cli_executable"]["denominator"]:
        missing = sorted(
            set(lark["cli_executable"]["missing_from_eval"])
            | set(lark["cli_executable"]["stale_expectations"])
        )
        issues.append(
            f"- Lark CLI 当前版本缺少 {len(missing)} 个评测能力：`{', '.join(missing)}`"
        )
    lines.extend(issues or ["- 未发现契约漂移。"])
    lines.extend(
        [
            "",
            "> 契约覆盖率只证明 Query 集覆盖当前可发现能力，不代表真实业务调用成功；真实执行仍受账号、权限、Fixture 和上游服务影响。",
            "",
        ]
    )
    return "\n".join(lines)


def validate_result(result: dict[str, Any]) -> list[str]:
    errors: list[str] = []
    status = result.get("status")
    mode = result.get("mode")
    checks = result.get("checks")
    if status not in ALLOWED_STATUSES:
        errors.append(f"invalid status {status!r}")
    if mode not in {"contract", "live"}:
        errors.append(f"invalid mode {mode!r}")
    if not isinstance(checks, dict):
        errors.append("checks must be an object")
        return errors
    for name in CHECK_NAMES:
        if checks.get(name) not in {True, False, None}:
            errors.append(f"checks.{name} must be true, false, or null")
    if status == "pass":
        required = CHECK_NAMES if mode == "live" else CHECK_NAMES[:3]
        for name in required:
            if checks.get(name) is not True:
                errors.append(f"pass requires checks.{name}=true in {mode} mode")
        if mode == "contract" and checks.get("execution") is not None:
            errors.append("contract mode requires checks.execution=null")
    return errors


def score(
    manifest_path: Path,
    golden_path: Path,
    results_path: Path,
    output: Path,
    json_output: Path,
) -> dict[str, Any]:
    manifest = read_jsonl(manifest_path)
    golden = read_jsonl(golden_path)
    results = read_jsonl(results_path)
    cases = {case["id"]: case for case in golden}
    manifest_ids = {case["id"] for case in manifest}
    if manifest_ids != set(cases):
        missing = sorted(set(cases) - manifest_ids)
        extra = sorted(manifest_ids - set(cases))
        raise ValueError(f"manifest/golden id drift; missing={missing}, extra={extra}")
    result_map: dict[str, dict[str, Any]] = {}
    validation_errors: list[str] = []
    for result in results:
        case_id = result.get("id")
        if case_id not in cases:
            validation_errors.append(f"unknown result id {case_id!r}")
            continue
        if case_id in result_map:
            validation_errors.append(f"duplicate result id {case_id}")
            continue
        result_map[case_id] = result
        validation_errors.extend(
            f"{case_id}: {error}" for error in validate_result(result)
        )
    if validation_errors:
        raise ValueError("; ".join(validation_errors))

    statuses: dict[str, int] = defaultdict(int)
    for case_id in cases:
        statuses[result_map.get(case_id, {}).get("status", "not_run")] += 1
    evaluated = statuses["pass"] + statuses["fail"]
    total = len(cases)

    checks: dict[str, dict[str, Any]] = {}
    for name in CHECK_NAMES:
        attempted = [
            result["checks"][name]
            for result in result_map.values()
            if isinstance(result.get("checks"), dict)
            and result["checks"].get(name) is not None
        ]
        passed = sum(value is True for value in attempted)
        checks[name] = {
            "passed": passed,
            "denominator": len(attempted),
            "rate": pct(passed, len(attempted)) if attempted else None,
        }

    capability_cases: dict[str, set[str]] = defaultdict(set)
    for case in golden:
        for tag in case.get("coverage", []):
            namespace = tag["namespace"]
            if namespace == "GAP":
                continue
            capability_cases[f"{namespace}:{tag['value']}"].add(case["id"])

    capability_by_namespace: dict[str, dict[str, Any]] = {}
    for namespace in ("S", "C", "R", "P", "H", "L"):
        capabilities = {
            key: case_ids
            for key, case_ids in capability_cases.items()
            if key.startswith(f"{namespace}:")
        }
        evaluated_capabilities = 0
        passed_capabilities = 0
        for case_ids in capabilities.values():
            outcomes = {result_map.get(case_id, {}).get("status", "not_run") for case_id in case_ids}
            if outcomes & {"pass", "fail"}:
                evaluated_capabilities += 1
            if "pass" in outcomes:
                passed_capabilities += 1
        capability_by_namespace[namespace] = {
            "evaluated": evaluated_capabilities,
            "passed": passed_capabilities,
            "denominator": len(capabilities),
            "evaluation_rate": pct(evaluated_capabilities, len(capabilities)),
            "pass_coverage": pct(passed_capabilities, len(capabilities)),
        }

    dws_namespaces = ("S", "C", "R", "P")
    dws_passed = sum(capability_by_namespace[name]["passed"] for name in dws_namespaces)
    dws_denominator = sum(
        capability_by_namespace[name]["denominator"] for name in dws_namespaces
    )
    shortcut_passed = (
        capability_by_namespace["P"]["passed"] + capability_by_namespace["H"]["passed"]
    )
    shortcut_denominator = (
        capability_by_namespace["P"]["denominator"]
        + capability_by_namespace["H"]["denominator"]
    )

    report = {
        "generated_at": utc_now(),
        "manifest": str(manifest_path),
        "golden": str(golden_path),
        "results": str(results_path),
        "queries": {
            "evaluated": evaluated,
            "passed": statuses["pass"],
            "failed": statuses["fail"],
            "blocked_fixture": statuses["blocked_fixture"],
            "skipped": statuses["skipped"],
            "not_run": statuses["not_run"],
            "denominator": total,
            "evaluation_rate": pct(evaluated, total),
            "pass_rate": pct(statuses["pass"], evaluated) if evaluated else None,
        },
        "checks": checks,
        "capabilities": capability_by_namespace,
        "summary": {
            "dws_delivered_pass_coverage": {
                "passed": dws_passed,
                "denominator": dws_denominator,
                "rate": pct(dws_passed, dws_denominator),
            },
            "dws_shortcut_semantic_pass_coverage": {
                "passed": shortcut_passed,
                "denominator": shortcut_denominator,
                "rate": pct(shortcut_passed, shortcut_denominator),
            },
            "lark_skill_pass_coverage": {
                "passed": capability_by_namespace["L"]["passed"],
                "denominator": capability_by_namespace["L"]["denominator"],
                "rate": capability_by_namespace["L"]["pass_coverage"],
            },
        },
    }
    write_json(json_output, report)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(render_score_markdown(report))
    return report


def render_score_markdown(report: dict[str, Any]) -> str:
    queries = report["queries"]
    summary = report["summary"]
    lines = [
        "# DWS IM GSB Eval 覆盖率报告",
        "",
        f"- 生成时间：`{report['generated_at']}`",
        f"- Manifest：`{report['manifest']}`",
        f"- Golden：`{report['golden']}`",
        f"- Results：`{report['results']}`",
        "",
        "## Query 结果",
        "",
        "| 指标 | 结果 |",
        "|---|---:|",
        f"| Query 评测覆盖率 | {rate_text(queries['evaluated'], queries['denominator'])} |",
        f"| 已评测 Query 通过率 | {rate_text(queries['passed'], queries['evaluated'])} |",
        f"| 失败 | {queries['failed']} |",
        f"| Fixture 阻塞 | {queries['blocked_fixture']} |",
        f"| 跳过 | {queries['skipped']} |",
        f"| 未运行 | {queries['not_run']} |",
        "",
        "## 能力通过覆盖率",
        "",
        "| 能力面 | 已评测 | 通过覆盖 |",
        "|---|---:|---:|",
    ]
    for namespace in ("S", "C", "R", "P", "H", "L"):
        value = report["capabilities"][namespace]
        lines.append(
            f"| `{namespace}:` | {rate_text(value['evaluated'], value['denominator'])} | "
            f"{rate_text(value['passed'], value['denominator'])} |"
        )
    lines.extend(
        [
            f"| DWS 当前交付面 | — | {rate_text(summary['dws_delivered_pass_coverage']['passed'], summary['dws_delivered_pass_coverage']['denominator'])} |",
            f"| DWS Shortcut 语义 | — | {rate_text(summary['dws_shortcut_semantic_pass_coverage']['passed'], summary['dws_shortcut_semantic_pass_coverage']['denominator'])} |",
            f"| Lark Skill 语义 | — | {rate_text(summary['lark_skill_pass_coverage']['passed'], summary['lark_skill_pass_coverage']['denominator'])} |",
            "",
            "## 检查项",
            "",
            "| 检查 | 通过率 |",
            "|---|---:|",
        ]
    )
    for name in CHECK_NAMES:
        value = report["checks"][name]
        lines.append(f"| `{name}` | {rate_text(value['passed'], value['denominator'])} |")
    lines.extend(
        [
            "",
            "> `blocked_fixture` 不计为产品失败，也不计入已完成 Query；应补齐账号、权限或状态后重跑。",
            "",
        ]
    )
    return "\n".join(lines)


def add_common_repo_argument(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--repo-root", help="DWS repository root")


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)

    prepare_parser = subparsers.add_parser("prepare", help="export manifest and result template")
    add_common_repo_argument(prepare_parser)
    prepare_parser.add_argument("--out-dir", required=True)

    contract_parser = subparsers.add_parser("contract", help="run read-only contract coverage")
    add_common_repo_argument(contract_parser)
    contract_parser.add_argument("--out-dir", required=True)
    contract_parser.add_argument("--dws", default="./dws")
    contract_parser.add_argument("--lark-cli", default="lark-cli")
    contract_parser.add_argument(
        "--lark-skill",
        default=str(Path.home() / ".agents/skills/lark-im"),
    )

    quick_parser = subparsers.add_parser("quick", help="prepare and run contract coverage")
    add_common_repo_argument(quick_parser)
    quick_parser.add_argument("--out-dir", required=True)
    quick_parser.add_argument("--dws", default="./dws")
    quick_parser.add_argument("--lark-cli", default="lark-cli")
    quick_parser.add_argument(
        "--lark-skill",
        default=str(Path.home() / ".agents/skills/lark-im"),
    )

    score_parser = subparsers.add_parser("score", help="score harness result JSONL")
    score_parser.add_argument("--manifest", required=True)
    score_parser.add_argument("--golden", required=True)
    score_parser.add_argument("--results", required=True)
    score_parser.add_argument("--output", required=True)
    score_parser.add_argument("--json-output", required=True)

    args = parser.parse_args()
    try:
        if args.command == "prepare":
            repo_root = find_repo_root(args.repo_root)
            summary = prepare(repo_root, Path(args.out_dir).resolve())
        elif args.command == "contract":
            repo_root = find_repo_root(args.repo_root)
            summary = contract(
                repo_root,
                Path(args.out_dir).resolve(),
                args.dws,
                args.lark_cli,
                Path(args.lark_skill).expanduser().resolve(),
            )
        elif args.command == "quick":
            repo_root = find_repo_root(args.repo_root)
            out_dir = Path(args.out_dir).resolve()
            prepared = prepare(repo_root, out_dir)
            covered = contract(
                repo_root,
                out_dir,
                args.dws,
                args.lark_cli,
                Path(args.lark_skill).expanduser().resolve(),
            )
            summary = {
                "query_count": prepared["query_count"],
                "dws_delivered_surface": covered["dws"]["delivered_surface"],
                "lark_cli_executable": covered["lark"]["cli_executable"],
                "out_dir": str(out_dir),
            }
        else:
            summary = score(
                Path(args.manifest).resolve(),
                Path(args.golden).resolve(),
                Path(args.results).resolve(),
                Path(args.output).resolve(),
                Path(args.json_output).resolve(),
            )
    except (ValueError, RuntimeError, OSError, subprocess.TimeoutExpired) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
