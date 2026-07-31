#!/usr/bin/env python3
# Regenerates internal/cli/schema_hint_decls_generated.go from schema_hints.
# After the one-shot migration, selection/*.json tools are empty and this
# script refuses to overwrite the compiled table. Restore selection+metadata
# from git history if you need to regenerate.
"""Generate compiled Schema declarations from reviewed hints."""

from __future__ import annotations

import json
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SEL = ROOT / "internal/cli/schema_hints/selection"
META = ROOT / "internal/cli/schema_hints/metadata"
OUT = ROOT / "internal/cli/schema_hint_decls_generated.go"
DRY_SRC = ROOT / "internal/cli/schema_dry_run_capabilities.go"
CATALOG = ROOT / "internal/cli/schema_catalog/tools"
IREF = ROOT / "internal/cli/schema_hints/interface-refs.json"


def go_string(s: str | None) -> str:
    return json.dumps(s or "", ensure_ascii=False)


def go_strings(arr: list | None) -> str:
    if not arr:
        return "nil"
    parts = ",\n\t\t\t".join(go_string(x) for x in arr)
    return f"[]string{{\n\t\t\t{parts},\n\t\t}}"


def complete_safety(meta: dict, catalog_tool: dict) -> dict:
    effect = (meta.get("effect") or "").strip()
    risk = (meta.get("risk") or "").strip()
    conf = (meta.get("confirmation") or "").strip()
    idem = (meta.get("idempotency") or "").strip()
    if not any([effect, risk, conf, idem]) and catalog_tool:
        effect = (catalog_tool.get("effect") or "").strip()
        risk = (catalog_tool.get("risk") or "").strip()
        conf = (catalog_tool.get("confirmation") or "").strip()
        idem = (catalog_tool.get("idempotency") or "").strip()
    if not any([effect, risk, conf, idem]):
        return {
            "Effect": "read",
            "Risk": "low",
            "Confirmation": "not_required",
            "Idempotency": "idempotent",
        }
    if not effect:
        if catalog_tool and catalog_tool.get("effect"):
            effect = str(catalog_tool.get("effect")).strip()
        elif risk == "high":
            effect = "destructive"
        elif conf == "user_required" or risk == "medium":
            effect = "write"
        else:
            effect = "read"
    if not risk:
        if catalog_tool and catalog_tool.get("risk"):
            risk = str(catalog_tool.get("risk")).strip()
        else:
            risk = (
                "high"
                if effect == "destructive"
                else ("medium" if effect == "write" else "low")
            )
    if not conf:
        if catalog_tool and catalog_tool.get("confirmation"):
            conf = str(catalog_tool.get("confirmation")).strip()
        else:
            conf = (
                "user_required"
                if effect in ("write", "destructive")
                else "not_required"
            )
    if not idem:
        if catalog_tool and catalog_tool.get("idempotency"):
            idem = str(catalog_tool.get("idempotency")).strip()
        else:
            idem = "unknown" if effect in ("write", "destructive") else "idempotent"
    return {
        "Effect": effect,
        "Risk": risk,
        "Confirmation": conf,
        "Idempotency": idem,
    }


def load_dry_run() -> dict[str, str]:
    dry: dict[str, str] = {}
    text = DRY_SRC.read_text()
    mapping = {
        "DryRunPreviewRequest": "request",
        "DryRunPreviewPlan": "plan",
        "DryRunPreviewInvocation": "invocation",
        "DryRunPreviewDiff": "diff",
    }
    for m in re.finditer(
        r"\{PreviewKind:\s*(\w+),\s*CanonicalPaths:\s*\[\]string\{([^}]*)\}",
        text,
        re.S,
    ):
        kind = mapping.get(m.group(1), m.group(1))
        for path in re.findall(r'"([^"]+)"', m.group(2)):
            dry[path] = kind
    return dry


def load_catalog() -> dict:
    out: dict = {}
    for p in CATALOG.glob("*.json"):
        d = json.loads(p.read_text())
        tools = d.get("tools") or {}
        if isinstance(tools, dict):
            tools = list(tools.values())
        for t in tools:
            cp = t.get("canonical_path")
            if cp:
                out[cp] = t
    return out


def main() -> int:
    selection: dict = {}
    for p in sorted(SEL.glob("*.json")):
        selection.update(json.loads(p.read_text()).get("tools") or {})
    if not selection:
        print(
            "selection hints are empty; refusing to overwrite compiled decls.\n"
            "Restore internal/cli/schema_hints/selection from git to regenerate.",
            file=sys.stderr,
        )
        return 1

    metadata: dict = {}
    for p in sorted(META.glob("*.json")):
        metadata.update(json.loads(p.read_text()).get("tools") or {})
    catalog = load_catalog()
    iref = {
        k: v["interface_ref"]
        for k, v in (json.loads(IREF.read_text()).get("tools") or {}).items()
        if v.get("interface_ref")
    }
    dry = load_dry_run()

    lines: list[str] = []
    a = lines.append
    a("// Copyright 2026 Alibaba Group")
    a("// Code generated by scripts/gen_schema_decls.py from schema_hints; DO NOT EDIT.")
    a("")
    a("package cli")
    a("")
    a("var schemaHintDeclsByCanonical = map[string]schemaHintDecl{")

    for canonical in sorted(selection):
        sel = selection[canonical]
        meta = metadata.get(canonical) or {}
        ct = catalog.get(canonical) or {}
        safety = complete_safety(meta, ct)
        mode = (meta.get("interface_mode") or ct.get("interface_mode") or "").strip()
        avail = (meta.get("availability") or ct.get("availability") or "").strip()
        reason = (meta.get("interface_reason") or ct.get("interface_reason") or "").strip()
        ref = meta.get("interface_ref") or {}
        if mode == "mcp":
            if not (ref.get("product_id") and ref.get("rpc_name")):
                ref = iref.get(canonical) or ct.get("interface_ref") or {}
        else:
            ref = {}
        pid = (ref.get("product_id") or "").strip()
        rpc = (ref.get("rpc_name") or "").strip()
        if not mode or not avail:
            print(f"missing interface for {canonical}", file=sys.stderr)
            return 1
        if mode == "mcp" and (not pid or not rpc):
            print(f"mcp without ref {canonical}", file=sys.stderr)
            return 1
        if mode == "composite" and not reason:
            print(f"composite without reason {canonical}", file=sys.stderr)
            return 1

        a(f"\t{go_string(canonical)}: {{")
        a("\t\tSafety: SafetySpec{")
        a(
            f"\t\t\tEffect: {go_string(safety['Effect'])}, Risk: {go_string(safety['Risk'])},"
        )
        a(
            f"\t\t\tConfirmation: {go_string(safety['Confirmation'])}, Idempotency: {go_string(safety['Idempotency'])},"
        )
        a("\t\t},")
        a(f"\t\tDescription: {go_string(sel.get('agent_summary') or '')},")
        if dry.get(canonical):
            a(f"\t\tDryRun: &DryRunSpec{{PreviewKind: {go_string(dry[canonical])}}},")
        a("\t\tInterface: &InterfaceSpec{")
        a(f"\t\t\tMode: {go_string(mode)}, Availability: {go_string(avail)},")
        if reason:
            a(f"\t\t\tReason: {go_string(reason)},")
        if pid:
            a(
                f"\t\t\tRef: &InterfaceRefSpec{{ProductID: {go_string(pid)}, RPCName: {go_string(rpc)}}},"
            )
        a("\t\t},")
        a("\t\tSelection: SelectionSpec{")
        a(f"\t\t\tAgentSummary: {go_string(sel.get('agent_summary') or '')},")
        a(f"\t\t\tUseWhen: {go_strings(sel.get('use_when') or [])},")
        a(f"\t\t\tAvoidWhen: {go_strings(sel.get('avoid_when') or [])},")
        if sel.get("prerequisites"):
            a(f"\t\t\tPrerequisites: {go_strings(sel.get('prerequisites'))},")
        if sel.get("tips"):
            a(f"\t\t\tTips: {go_strings(sel.get('tips'))},")
        a(f"\t\t\tExamples: {go_strings(sel.get('examples') or [])},")
        a("\t\t},")
        a("\t},")

    a("}")
    a("")
    OUT.write_text("\n".join(lines))
    print(f"wrote {OUT} entries={len(selection)}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
