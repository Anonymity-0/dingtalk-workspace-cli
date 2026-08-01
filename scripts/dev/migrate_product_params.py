#!/usr/bin/env python3
"""Migrate one product's parameter overlays from hints into ParamDecl declarations.

Usage: python3 scripts/dev/migrate_product_params.py <product>

For each tool with parameter overlays in schema_hints/metadata/<product>.json:
1. Finds the DeclareLeafMetadata block for the tool's reviewed cli_path.
2. Inserts Parameters: []corecmd.ParamDecl{...} into the Schema block.
3. Moves any overlaid flag registrations before the DeclareLeafMetadata call.
4. Clears the tool's parameters from the hints file.

Shared-RPC siblings (e.g. drive rename / doc rename both use rename_document,
mail message list / search both call search_emails) MUST be disambiguated by
cli_path. If cli_path cannot uniquely select a leaf, the script refuses to
insert or clear hints — never pick the first RPC hit.

Run for one product at a time; verify compilation and schema output between runs.
"""
import json
import os
import re
import sys

HELPER_DIR = "internal/helpers"
HINTS_DIR = "internal/cli/schema_hints/metadata"

# Primary file per product (used for hints clearing only; RPCName search covers all files)
PRODUCT_FILES = {
    "aisearch": "aisearch.go", "aitable": "aitable.go", "attendance": "attendance.go",
    "chat": "chat.go", "contact": "contact.go", "devdoc": "devdoc.go",
    "doc": "doc.go", "drive": "drive.go", "hrbrain": "hrbrain.go",
    "mail": "mail.go", "markdown": "markdown.go", "minutes": "minutes.go",
    "report": "report.go", "sheet": "sheet.go", "todo": "todo.go",
}


def all_helper_files():
    """Return all non-test .go files in internal/helpers/."""
    files = []
    for f in sorted(os.listdir(HELPER_DIR)):
        if f.endswith(".go") and not f.endswith("_test.go"):
            files.append(os.path.join(HELPER_DIR, f))
    return files


def helper_files_for_product(product):
    """Prefer the product primary file, then the rest of helpers."""
    files = all_helper_files()
    primary = PRODUCT_FILES.get(product)
    if not primary:
        return files
    preferred = os.path.join(HELPER_DIR, primary)
    ordered = []
    if preferred in files:
        ordered.append(preferred)
    for fp in files:
        if fp != preferred:
            ordered.append(fp)
    return ordered


def bool_ptr(v):
    return "boolPtr(true)" if v else "boolPtr(false)"


def render_param(name, ov):
    parts = [f'Name: "{name}"']
    if "property" in ov:
        parts.append(f'Property: "{ov["property"]}"')
    if "required" in ov:
        parts.append(f"Required: {bool_ptr(ov['required'])}")
    if "interface_type" in ov:
        parts.append(f'InterfaceType: "{ov["interface_type"]}"')
    if "description" in ov:
        parts.append(f'Description: "{ov["description"]}"')
    if "required_when" in ov:
        parts.append(f'RequiredWhen: "{ov["required_when"]}"')
    if "enum" in ov and ov["enum"]:
        vals = ", ".join(f'"{v}"' for v in ov["enum"])
        parts.append(f"Enum: []string{{{vals}}}")
    return "\t\t\t\t{" + ", ".join(parts) + "},"


def find_block_end(lines, start_idx):
    """Find the line index of the closing '})' or '},' for a block starting at start_idx."""
    depth = 0
    for i in range(start_idx, len(lines)):
        depth += lines[i].count("{") - lines[i].count("}")
        if depth <= 0 and i > start_idx:
            return i
    return -1


def find_cmd_var_for_call(lines, call_line):
    """Find the nearest cmd := &cobra.Command{ whose block contains call_line."""
    candidates = []
    for i in range(call_line, -1, -1):
        m = re.search(r'(\w+)\s*:?=\s*&cobra\.Command\s*{', lines[i])
        if not m:
            continue
        end = find_block_end(lines, i)
        if end >= call_line:
            return m.group(1), i
        # Keep searching further back; an outer command may still enclose us.
        candidates.append((m.group(1), i))
    return "", -1


def find_declare_for_var(lines, var_name, after_line=0):
    """Find DeclareLeafMetadata(var_name, ...) at or after after_line."""
    if not var_name:
        return -1
    pat = re.compile(r'DeclareLeafMetadata\(\s*' + re.escape(var_name) + r'\s*,')
    for i in range(max(after_line, 0), len(lines)):
        if pat.search(lines[i]):
            return i
    return -1


# Minimum score to accept a multi-leaf shared-RPC candidate.
# Example/Examples primary invoke = 100; Use+product file = 40.
SCORE_MULTI_LEAF_MIN = 40


def cli_path_tokens(cli_path):
    """Normalize a reviewed cli_path into path tokens (no flags)."""
    if not cli_path:
        return []
    # "mail message list" or "minutes +replace-batch"
    parts = []
    for tok in str(cli_path).split():
        if tok.startswith("-"):
            break
        parts.append(tok)
    return parts


def example_blobs(block_text):
    """Extract Example / Examples string bodies (ignore UseWhen/AvoidWhen prose)."""
    blobs = []
    for m in re.finditer(r"Example:\s*`([^`]*)`", block_text, re.S):
        blobs.append(m.group(1))
    for m in re.finditer(r"Examples:\s*\[\]string\{([^{}]*)\}", block_text, re.S):
        blobs.append(m.group(1))
    return "\n".join(blobs)


def file_matches_cli_product(fp, product):
    """True when helper/shortcut file clearly belongs to the cli_path product."""
    if not product or not fp:
        return False
    base = os.path.basename(fp)
    norm = fp.replace("\\", "/")
    if base == f"{product}.go" or base.startswith(f"{product}_"):
        return True
    if f"/shortcut/{product}/" in norm or norm.endswith(f"/shortcut/{product}.go"):
        return True
    return False


def score_block_for_cli_path(block_text, cli_path, fp=""):
    """Score how strongly a command/Declare block owns reviewed cli_path.

    Sibling AvoidWhen/UseWhen often mention the other leaf's path (e.g. doc rename
    says "prefer dws drive rename"). Bare substring matching is therefore unsafe.
    Only Example/Examples primary invokes and Use+product-file evidence count.
    """
    tokens = cli_path_tokens(cli_path)
    if not tokens:
        return 0
    path = " ".join(tokens)
    product = tokens[0]
    leaf = tokens[-1]
    score = 0

    examples = example_blobs(block_text)
    # Primary: this leaf's Example documents `dws <cli_path> ...`
    if re.search(
        r'(?:^|[\s"\'`])dws\s+' + re.escape(path) + r'(?:\s|--|["\'`]|$)',
        examples,
        re.M,
    ):
        score += 100

    if re.search(r'Use:\s*"' + re.escape(leaf) + r'"', block_text):
        if file_matches_cli_product(fp, product):
            score += 40
        elif len(tokens) >= 2 and tokens[-2] in block_text:
            # Same-file siblings (mail list/search): Use leaf differs; parent token
            # is weak confirmation only and must not beat Example evidence.
            score += 10
        else:
            score += 5

    return score


def block_matches_cli_path(block_text, cli_path, fp=""):
    """True when score shows clear ownership of cli_path (not bare substring)."""
    return score_block_for_cli_path(block_text, cli_path, fp) >= SCORE_MULTI_LEAF_MIN


def collect_rpc_sites(file_contents, rpc_name):
    """Return [(file, line_idx)] for every RPCName / callMCPTool site."""
    rpc_pattern = re.compile(r'RPCName:\s*"' + re.escape(rpc_name) + '"')
    call_patterns = [
        re.compile(r'callMCPTool\("' + re.escape(rpc_name) + '"'),
        re.compile(r'callMCPToolOnServer\([^,]+,\s*"' + re.escape(rpc_name) + '"'),
        re.compile(r'callAitableTool\("' + re.escape(rpc_name) + '"'),
        re.compile(r'rt\.CallMCP\w*\("' + re.escape(rpc_name) + '"'),
    ]
    sites = []
    for fp, src in file_contents.items():
        lines_src = src.split("\n")
        for i, line in enumerate(lines_src):
            if rpc_pattern.search(line) or any(cp.search(line) for cp in call_patterns):
                sites.append((fp, i))
    return sites


def _dedupe_candidates(candidates):
    """Keep highest-scoring candidate per DeclareLeafMetadata site."""
    best = {}
    for fp, decl_start, cmd_var, score, call_line in candidates:
        key = (fp, decl_start)
        prev = best.get(key)
        if prev is None or score > prev[3]:
            best[key] = (fp, decl_start, cmd_var, score, call_line)
    return list(best.values())


def _pick_unique_candidate(tool_name, rpc_name, cli_path, candidates, *, multi_leaf):
    """Return (fp, decl_start, cmd_var) or (None, -1, "") when unsafe."""
    ranked = sorted(candidates, key=lambda c: (-c[3], c[0], c[1]))
    if not ranked:
        return None, -1, ""

    if multi_leaf:
        if not cli_path:
            print(f"  WARNING: {tool_name} — RPC '{rpc_name}' has multiple DeclareLeafMetadata "
                  f"candidates; reviewed cli_path is required for disambiguation")
            return None, -1, ""
        if ranked[0][3] < SCORE_MULTI_LEAF_MIN:
            print(f"  WARNING: {tool_name} — RPC '{rpc_name}' has {len(ranked)} leaf candidates; "
                  f"cli_path={cli_path!r} did not uniquely select a leaf; refusing to insert")
            return None, -1, ""
        top = [c for c in ranked if c[3] == ranked[0][3]]
        if len(top) > 1:
            print(f"  WARNING: {tool_name} — ambiguous cli_path match for RPC '{rpc_name}' "
                  f"({len(top)} DeclareLeafMetadata candidates at score {ranked[0][3]}); "
                  f"refusing to insert")
            return None, -1, ""
        fp, decl_start, cmd_var, _, _ = top[0]
        return fp, decl_start, cmd_var

    # Single leaf: allow RPC-only resolution (no cli_path required).
    fp, decl_start, cmd_var, _, _ = ranked[0]
    return fp, decl_start, cmd_var


def resolve_declare_target(file_contents, tool_name, rpc_name, cli_path):
    """Resolve (file, decl_start, cmd_var) for a tool, or (None, -1, "").

    Shared RPCs (e.g. mail list/search both call search_emails) must be
    disambiguated by reviewed cli_path. Never return the first RPC hit blindly.
    """
    sites = collect_rpc_sites(file_contents, rpc_name)
    candidates = []  # (fp, decl_start, cmd_var, score, call_line)

    for fp, call_line in sites:
        lines = file_contents[fp].split("\n")
        cmd_var, cmd_line = find_cmd_var_for_call(lines, call_line)
        decl_start = find_declare_for_var(lines, cmd_var, after_line=cmd_line) if cmd_var else -1
        if decl_start < 0:
            continue
        block_end = find_block_end(lines, decl_start)
        # Include the command literal block for Use/Example matching.
        window_start = cmd_line if cmd_line >= 0 else decl_start
        block_text = "\n".join(lines[window_start:block_end + 1])
        score = score_block_for_cli_path(block_text, cli_path, fp)
        candidates.append((fp, decl_start, cmd_var, score, call_line))

    candidates = _dedupe_candidates(candidates)
    multi_leaf = len(candidates) > 1
    if candidates:
        picked = _pick_unique_candidate(
            tool_name, rpc_name, cli_path, candidates, multi_leaf=multi_leaf)
        if picked[0] is not None:
            return picked
        if multi_leaf:
            return None, -1, ""

    # Strategy: composite / shortcut helpers keyed by cli_path or Use leaf.
    tokens = cli_path_tokens(cli_path)
    use_name = tokens[-1] if tokens else tool_name.split(".", 1)[-1].replace("_", "-")
    for prefix in ("shortcut-", "+"):
        if use_name.startswith(prefix):
            use_name = use_name[len(prefix):]
    use_pat = re.compile(r'Use:\s*"' + re.escape(use_name) + '"')
    use_hits = []
    for fp, src in file_contents.items():
        lines_src = src.split("\n")
        for i, line in enumerate(lines_src):
            if not use_pat.search(line):
                continue
            cmd_var, cmd_line = find_cmd_var_for_call(lines_src, i)
            decl_start = find_declare_for_var(lines_src, cmd_var, after_line=cmd_line) if cmd_var else -1
            if decl_start < 0:
                # shortcut.FromShortcut path: Schema may be on Shortcut literal.
                continue
            block_end = find_block_end(lines_src, decl_start)
            window_start = cmd_line if cmd_line >= 0 else decl_start
            block_text = "\n".join(lines_src[window_start:block_end + 1])
            score = score_block_for_cli_path(block_text, cli_path, fp)
            use_hits.append((fp, decl_start, cmd_var, score, i))
    use_hits = _dedupe_candidates(use_hits)
    if use_hits:
        picked = _pick_unique_candidate(
            tool_name, rpc_name, cli_path, use_hits, multi_leaf=len(use_hits) > 1)
        if picked[0] is not None:
            return picked
        return None, -1, ""

    print(f"  WARNING: {tool_name} — RPCName '{rpc_name}' not found in any helper file")
    return None, -1, ""


def schema_has_param_decls(schema_text, params):
    """True when Schema already declares every overlay flag Name."""
    if "Parameters:" not in schema_text:
        return False
    for pname in params:
        if not re.search(r'Name:\s*"' + re.escape(pname) + r'"', schema_text):
            return False
    return True


def _self_test_fixtures():
    """Synthetic shared-RPC helpers mirroring mail list/search and doc/drive rename."""
    mail_src = r'''
	messageSearchCmd := &cobra.Command{
		Use:   "search",
		Example: `  dws mail message search --email user@company.com --query "subject:周报"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return callMCPTool("search_emails", toolArgs)
		},
	}
	DeclareLeafMetadata(messageSearchCmd, LeafSpec{
		Schema: LeafSchema{
			Interface: &LeafInterfaceDecl{
				ProductID: "mail", RPCName: "search_emails",
			},
			Selection: LeafSelectionDecl{
				AvoidWhen: []string{
					"只列某文件夹邮件且无需复杂条件时优先 mail message list",
				},
				Examples: []string{"dws mail message search --email user@company.com --query \"subject:周报\""},
			},
		},
	})

	messageListCmd := &cobra.Command{
		Use:   "list",
		Example: `  dws mail message list --email user@company.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return callMCPTool("search_emails", toolArgs)
		},
	}
	DeclareLeafMetadata(messageListCmd, LeafSpec{
		Schema: LeafSchema{
			Interface: &LeafInterfaceDecl{
				Mode: "composite",
				Reason: "wrapper around search_emails",
			},
			Selection: LeafSelectionDecl{
				AvoidWhen: []string{"需要组合条件时用 mail message search"},
				Examples: []string{
					"dws mail message list --email user@company.com",
				},
			},
		},
	})
'''
    doc_src = r'''
	renameCmd := &cobra.Command{
		Use:   "rename",
		Example: `  dws doc rename --node DOC_ID --name "新名称"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return callMCPTool("rename_document", map[string]any{})
		},
	}
	DeclareLeafMetadata(renameCmd, LeafSpec{
		Schema: LeafSchema{
			Interface: &LeafInterfaceDecl{
				ProductID: "doc", RPCName: "rename_document",
			},
			Selection: LeafSelectionDecl{
				AvoidWhen: []string{
					"文件或文件夹重命名优先用 dws drive rename，由该命令读取真实节点类型",
				},
				Examples: []string{"dws doc rename --node <DOC_ID> --name \"新名称\" --format json"},
			},
		},
	})
'''
    drive_src = r'''
	driveRenameCmd := &cobra.Command{
		Use:   "rename",
		Example: `  dws drive rename --node DOC_ID --name "新名称"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return callMCPToolOnServer("doc", "rename_document", map[string]any{})
		},
	}
	DeclareLeafMetadata(driveRenameCmd, LeafSpec{
		Schema: LeafSchema{
			Interface: &LeafInterfaceDecl{
				ProductID: "doc", RPCName: "rename_document",
			},
			Selection: LeafSelectionDecl{
				AvoidWhen: []string{
					"要改正文里的标题改用 dws doc block update，不要用 rename",
				},
				Examples: []string{"dws drive rename --node <ID> --name \"新名称\" --format json"},
			},
			Parameters: []corecmd.ParamDecl{
				{Name: "name", Description: "drive-only extension stripping"},
			},
		},
	})
'''
    return {
        "internal/helpers/mail.go": mail_src,
        "internal/helpers/doc.go": doc_src,
        "internal/helpers/drive.go": drive_src,
    }


def run_self_test():
    """Verify shared-RPC cli_path disambiguation; exit 0/1."""
    fc = _self_test_fixtures()
    cases = [
        ("mail.list_emails", "search_emails", "mail message list", "mail.go", "messageListCmd"),
        ("mail.search_emails", "search_emails", "mail message search", "mail.go", "messageSearchCmd"),
        ("drive.rename_document", "rename_document", "drive rename", "drive.go", "driveRenameCmd"),
        ("doc.rename_document", "rename_document", "doc rename", "doc.go", "renameCmd"),
    ]
    failed = 0
    for tool, rpc, cli, want_file, want_var in cases:
        fp, decl, var = resolve_declare_target(fc, tool, rpc, cli)
        base = os.path.basename(fp) if fp else None
        ok = base == want_file and var == want_var and decl >= 0
        status = "OK" if ok else "FAIL"
        print(f"  {status}: {tool} cli_path={cli!r} -> {base}:{var} (want {want_file}:{want_var})")
        if not ok:
            failed += 1

    # Bare AvoidWhen cross-links must not make block_matches true alone.
    doc_block = fc["internal/helpers/doc.go"]
    if block_matches_cli_path(doc_block, "drive rename", "internal/helpers/doc.go"):
        print("  FAIL: doc rename AvoidWhen must not own cli_path 'drive rename'")
        failed += 1
    else:
        print("  OK: doc AvoidWhen cross-link does not claim drive rename")

    # Multi-leaf without cli_path must refuse.
    fp, decl, var = resolve_declare_target(fc, "mail.list_emails", "search_emails", "")
    if fp is not None:
        print(f"  FAIL: multi-leaf without cli_path should refuse, got {fp}:{var}")
        failed += 1
    else:
        print("  OK: multi-leaf without cli_path refused")

    if failed:
        print(f"self-test: {failed} failure(s)")
        return False
    print("self-test: all passed")
    return True


def migrate_product(product):
    hint_path = os.path.join(HINTS_DIR, f"{product}.json")
    hints = json.load(open(hint_path, encoding="utf-8"))
    tools = hints.get("tools") or {}
    overlays = {t: (v or {}).get("parameters") or {} for t, v in tools.items()}
    overlays = {t: p for t, p in overlays.items() if p}
    if not overlays:
        print(f"{product}: no overlays, skipping")
        return True

    # Optional canonical-path → actual MCP tool name when they diverge.
    RPC_ALIASES = {
        "attendance.adjustment_search": "get_adjustment_rule",
        "attendance.group_search": "get_simple_groups",
        "attendance.overtime_search": "get_overtime_rule",
        "mail.list_emails": "search_emails",
        "devdoc.search_open_platform_docs_rag": "search_open_platform_docs",
    }

    # Also search shortcut packages for composite / no-single-RPC commands.
    file_contents = {}
    for fp in helper_files_for_product(product):
        file_contents[fp] = open(fp, encoding="utf-8").read()
    shortcut_root = "internal/shortcut"
    if os.path.isdir(shortcut_root):
        for root, _, fns in os.walk(shortcut_root):
            for fn in fns:
                if fn.endswith(".go") and not fn.endswith("_test.go"):
                    fp = os.path.join(root, fn)
                    file_contents[fp] = open(fp, encoding="utf-8").read()

    modified_files = set()
    flags_to_move = []
    migrated_tools = set()  # only clear hints for tools we actually migrated

    for tool_name, params in sorted(overlays.items()):
        meta = tools.get(tool_name) or {}
        cli_path = (meta.get("cli_path") or "").strip()
        rpc_name = RPC_ALIASES.get(tool_name, tool_name.split(".")[-1])
        if not cli_path:
            # Shared RPCs cannot be safely resolved without reviewed cli_path.
            print(f"  WARNING: {tool_name} — parameters overlay missing cli_path; "
                  f"shared-RPC siblings will refuse multi-leaf resolution")

        found_file, decl_start, cmd_var = resolve_declare_target(
            file_contents, tool_name, rpc_name, cli_path)
        if not found_file or decl_start < 0:
            continue

        lines = file_contents[found_file].split("\n")

        # Find Schema block
        schema_start = -1
        for i in range(decl_start, min(decl_start + 80, len(lines))):
            if "Schema: LeafSchema{" in lines[i] or "Schema: corecmd.SchemaDecl{" in lines[i]:
                schema_start = i
                break
        if schema_start < 0:
            print(f"  WARNING: {tool_name} — no Schema block in {os.path.basename(found_file)}")
            continue

        schema_end = find_block_end(lines, schema_start)
        schema_text = "\n".join(lines[schema_start:schema_end + 1])
        if schema_has_param_decls(schema_text, params):
            # Only treat as migrated when THIS leaf already has the overlay names.
            print(f"  SKIP: {tool_name} — Parameters already declared on matched leaf "
                  f"(var={cmd_var}, cli_path={cli_path!r})")
            migrated_tools.add(tool_name)
            continue
        if "Parameters:" in schema_text:
            print(f"  WARNING: {tool_name} — matched leaf already has Parameters but is missing "
                  f"overlay flags {sorted(params)}; refusing to clear hints (var={cmd_var})")
            continue

        # Build and insert Parameters
        param_lines = ["\t\t\tParameters: []corecmd.ParamDecl{"]
        for pname in sorted(params):
            param_lines.append(render_param(pname, params[pname]))
        param_lines.append("\t\t\t},")

        insert_at = schema_end
        for i in range(schema_end - 1, schema_start, -1):
            stripped = lines[i].strip()
            if stripped and not stripped.startswith("//"):
                insert_at = i + 1
                break

        for j, pl in enumerate(param_lines):
            lines.insert(insert_at + j, pl)
        modified_files.add(found_file)
        file_contents[found_file] = "\n".join(lines)
        migrated_tools.add(tool_name)
        print(f"  OK: {tool_name} — inserted {len(params)} param decls in {os.path.basename(found_file)} "
              f"at line {insert_at + 1} (var={cmd_var}, cli_path={cli_path!r})")

        # Check flag registration order
        for pname in params:
            flag_pattern = re.compile(r'\.Flags\(\)\.\w+\("' + re.escape(pname) + '"')
            for i, line in enumerate(lines):
                if flag_pattern.search(line):
                    if i > decl_start:
                        flags_to_move.append((pname, i, decl_start, os.path.basename(found_file)))
                    break

    # Clear hints ONLY for tools we migrated onto the matched leaf.
    if migrated_tools:
        for tool_name in migrated_tools:
            if tool_name in hints.get("tools", {}):
                hints["tools"][tool_name].pop("parameters", None)
                if not hints["tools"][tool_name]:
                    del hints["tools"][tool_name]
        if not hints.get("tools"):
            hints["tools"] = {}
        json.dump(hints, open(hint_path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
        print(f"{product}: cleared hints for {len(migrated_tools)} tools "
              f"({len(overlays) - len(migrated_tools)} remaining)")

    if not modified_files:
        if not migrated_tools:
            print(f"{product}: no changes needed")
        return True

    # Write modified files and ensure corecmd import
    for fp in modified_files:
        src = file_contents[fp]
        if fp.startswith(HELPER_DIR) and "internal/corecmd" not in src:
            m = re.search(r'(import \(\n(?:.*\n)*?\))', src)
            if m:
                block = m.group(1)
                new_block = block.rstrip(")\n") + '\n\n\t"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"\n)'
                src = src[:m.start()] + new_block + src[m.end():]
                print(f"  added corecmd import to {os.path.basename(fp)}")
        open(fp, "w", encoding="utf-8").write(src)

    if flags_to_move:
        print(f"  ⚠ {len(flags_to_move)} flags registered AFTER DeclareLeafMetadata — need manual reorder:")
        for fname, reg_line, decl_line, fname_file in flags_to_move:
            print(f"    --{fname} in {fname_file}: reg line {reg_line + 1} > decl line {decl_line + 1}")

    return True


if __name__ == "__main__":
    if len(sys.argv) == 2 and sys.argv[1] in ("--self-test", "self-test"):
        sys.exit(0 if run_self_test() else 1)
    if len(sys.argv) != 2:
        print("Usage: migrate_product_params.py <product>")
        print("       migrate_product_params.py --self-test")
        sys.exit(1)
    ok = migrate_product(sys.argv[1])
    sys.exit(0 if ok else 1)
