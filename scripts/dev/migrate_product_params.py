#!/usr/bin/env python3
"""Migrate one product's parameter overlays from hints into ParamDecl declarations.

Usage: python3 scripts/dev/migrate_product_params.py <product>

For each tool with parameter overlays in schema_hints/metadata/<product>.json:
1. Finds the DeclareLeafMetadata block containing the tool's RPCName.
2. Inserts Parameters: []corecmd.ParamDecl{...} into the Schema block.
3. Moves any overlaid flag registrations before the DeclareLeafMetadata call.
4. Clears the tool's parameters from the hints file.

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


def migrate_product(product):
    hint_path = os.path.join(HINTS_DIR, f"{product}.json")
    hints = json.load(open(hint_path, encoding="utf-8"))
    tools = hints.get("tools") or {}
    overlays = {t: (v or {}).get("parameters") or {} for t, v in tools.items()}
    overlays = {t: p for t, p in overlays.items() if p}
    if not overlays:
        print(f"{product}: no overlays, skipping")
        return True

    # Load all helper files into memory
    file_contents = {}
    for fp in all_helper_files():
        file_contents[fp] = open(fp, encoding="utf-8").read()

    modified_files = set()
    flags_to_move = []
    migrated_tools = set()  # only clear hints for tools we actually migrated

    for tool_name, params in sorted(overlays.items()):
        rpc_name = tool_name.split(".")[-1]

        # Strategy 1: RPCName field in Schema.Interface
        rpc_pattern = re.compile(r'RPCName:\s*"' + re.escape(rpc_name) + '"')
        # Strategy 2: callMCPTool / CallMCP call sites
        call_patterns = [
            re.compile(r'callMCPTool\("' + re.escape(rpc_name) + '"'),
            re.compile(r'callMCPToolOnServer\([^,]+,\s*"' + re.escape(rpc_name) + '"'),
            re.compile(r'rt\.CallMCP\w*\("' + re.escape(rpc_name) + '"'),
        ]

        found_file = None
        found_line = -1
        for fp, src in file_contents.items():
            lines_src = src.split("\n")
            # Try strategy 1 first
            for i, line in enumerate(lines_src):
                if rpc_pattern.search(line):
                    found_file = fp
                    found_line = i
                    break
            if found_file:
                break
            # Try strategy 2
            for i, line in enumerate(lines_src):
                for cp in call_patterns:
                    if cp.search(line):
                        found_file = fp
                        found_line = i
                        break
                if found_file:
                    break
            if found_file:
                break

        if not found_file:
            print(f"  WARNING: {tool_name} — RPCName '{rpc_name}' not found in any helper file")
            continue

        lines = file_contents[found_file].split("\n")

        # Find DeclareLeafMetadata: search backwards first, then forwards
        decl_start = -1
        search_range = list(range(found_line, max(found_line - 60, -1), -1)) + \
                       list(range(found_line + 1, min(found_line + 60, len(lines))))
        for i in search_range:
            if "DeclareLeafMetadata(" in lines[i]:
                # Verify this DeclareLeafMetadata's Schema contains the target
                # RPCName (or the tool name) to avoid matching the wrong command.
                block_end = find_block_end(lines, i)
                block_text = "\n".join(lines[i:block_end + 1])
                if rpc_name in block_text or tool_name in block_text:
                    decl_start = i
                    break
                # Also check for callMCPTool("rpc_name") inside the block
                if f'callMCPTool("{rpc_name}"' in block_text or \
                   f'callMCPToolOnServer(' in block_text and rpc_name in block_text:
                    decl_start = i
                    break
                # Skip this DeclareLeafMetadata — it doesn't belong to our tool
        if decl_start < 0:
            print(f"  WARNING: {tool_name} — no matching DeclareLeafMetadata near line {found_line + 1} in {os.path.basename(found_file)}")
            continue

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
        if "Parameters:" in schema_text:
            print(f"  SKIP: {tool_name} — Parameters already declared")
            migrated_tools.add(tool_name)
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
        print(f"  OK: {tool_name} — inserted {len(params)} param decls in {os.path.basename(found_file)} at line {insert_at + 1}")

        # Check flag registration order
        for pname in params:
            flag_pattern = re.compile(r'\.Flags\(\)\.\w+\("' + re.escape(pname) + '"')
            for i, line in enumerate(lines):
                if flag_pattern.search(line):
                    if i > decl_start:
                        flags_to_move.append((pname, i, decl_start, os.path.basename(found_file)))
                    break

    # Clear hints for migrated/skipped tools (even if no source files changed)
    if migrated_tools:
        for tool_name in migrated_tools:
            if tool_name in hints.get("tools", {}):
                hints["tools"][tool_name].pop("parameters", None)
                if not hints["tools"][tool_name]:
                    del hints["tools"][tool_name]
        if not hints.get("tools"):
            hints["tools"] = {}
        json.dump(hints, open(hint_path, "w", encoding="utf-8"), ensure_ascii=False, indent=2)
        print(f"{product}: cleared hints for {len(migrated_tools)} tools ({len(overlays) - len(migrated_tools)} remaining)")

    if not modified_files:
        if not migrated_tools:
            print(f"{product}: no changes needed")
        return True

    # Write modified files and ensure corecmd import
    for fp in modified_files:
        src = file_contents[fp]
        if "internal/corecmd" not in src:
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
    if len(sys.argv) != 2:
        print("Usage: migrate_product_params.py <product>")
        sys.exit(1)
    ok = migrate_product(sys.argv[1])
    sys.exit(0 if ok else 1)
