# DWS Agent Schema Hints

This directory contains versioned, structured Agent inputs for Schema
generation. Hints belong to the CLI Schema subsystem rather than either
installable Skill layout. They are excluded from embedded binaries and release
Skill bundles. Files generate `internal/cli/schema_agent_metadata/`; generated
runtime metadata must not be edited directly.

## Layout

Human-authored inputs:

- `index.json` (`format: dws-agent-hint-index`) — maps product IDs to required
  selection files and optional metadata map, plus reference review.
  Strategy: `metadata` is optional (omitted or `{}`); the on-disk
  `metadata/` directory is retired and may be absent.
- `selection/<product>.json` — Agent selection prose (required)
- `imported/` — sanitized baseline from a fixed external revision

When `index.json` is present, the generator loads `imported/` plus any
metadata files listed in `index.metadata` (optional) and the selection files
listed in `index.selection` (required). Sibling review JSON files in this
directory remain CI/audit inputs and are not applied as Agent metadata
sources.

For the end-to-end Agent curation workflow, see `AGENTS.md` § “Agent curation
workflow (Schema hints)”.

## Source kinds

- `contract_final`: leaf-declared Safety / Schema / ParamDecl
  (`DeclareLeafMetadata`, `Shortcut.Schema`). This is the production authority
  for safety, interface disposition, and parameter facts.
- `reviewed_explicit`: reviewed selection HintFiles under `selection/`
  (`reviewed: true`) for Agent selection prose. Residual metadata tool rows are
  not used in production shells.
- `explicit`: explicit but not per-tool-reviewed DWS hints.
- `imported`: sanitized metadata from a fixed external revision. It fills missing Agent semantics but cannot redefine command paths or parameter contracts.

Skill Markdown and audit JSON in this directory remain authoring evidence.
Normal generation does not semantically combine Markdown into the final Agent
prose and never rewrites metadata/selection files.

The Agent metadata generator also reads the committed `internal/cli/schema_mcp_metadata.json` after Skill and Hint parsing. A sanitized MCP description can fill an otherwise empty `agent_summary`; it is marked `reviewed: false`, retains revision provenance, and cannot infer or override risk/effect fields.

Tool keys should use stable `canonical_path` values from `internal/cli/schema_command_registry.json`. CLI paths and aliases are also accepted and are reconciled to the canonical public tool during generation.

`selection-review.json` fixes the reviewed command-selection contract for every
public tool: `use_when`, `avoid_when`, safe examples, and the ordinary direct
interface disposition. Exceptional or formerly unmatched wrappers are owned by
the interface-only reviewed source `zz-interface-disposition-review.json`.
`runtime-surface-completeness.json` remains unreviewed selection evidence and
must not promote its other fields merely to classify an interface. These values
are build inputs; the generator must not derive them from the previous Catalog.

`reference-review.json` classifies every Skill command reference that is not a
current public leaf. `alias` entries bind an old or cross-product path to an
explicit current target. `group`, `stale`, and `out_of_surface` entries remain
visible in the audit but are never fuzzy-matched to a leaf.

`interface_ref` is a separate interface binding. Use it when a public helper/canonical tool calls a differently named MCP RPC or another source product. Prefer declaring it on the leaf Contract (`Schema.Interface`) rather than adding a metadata tool row:

```json
{
  "version": 1,
  "source": {"kind": "explicit", "name": "reviewed-interface-map"},
  "tools": {
    "chat.bot_search": {
      "interface_ref": {
        "product_id": "bot",
        "rpc_name": "search_my_robots"
      }
    }
  }
}
```

An entry containing only `interface_ref` participates in interface projection but does not count as Agent semantic coverage. It cannot add a command, change a flag, or expose a Wukong-only tool.

`interface_mode` and `availability` are orthogonal reviewed fields.
`interface_mode` has exactly three values:

- `mcp`: exactly one pinned `interface_ref` implements the command, with an auditable parameter mapping and semantically equivalent execution contract.
- `composite`: multiple RPCs, conditional routing, local projection, or a reviewed remote adapter absent from pinned metadata implements the command; a singular ref would be misleading.
- `local`: the command is fully implemented by the local process, static data, or local policy. An unpinned remote RPC is never `local`.

`availability` is `available` or `unavailable`. An unavailable command keeps
its real implementation mode, must not carry `interface_ref`, and must include
an explicit `interface_reason`; `unavailable` is never an interface mode.

The missing `notify` MCP service is separately dispositioned in
`internal/cli/schema_mcp_service_review.json`; it is outside the public command
surface and must not trigger runtime discovery.

`internal/cli/schema_mcp_metadata.json.coverage.surface_tools` describes only
the immutable MCP import at its declared `source_revision`; it is not the
current CLI/Catalog tool count. Its `coverage.surface_scope` must remain
`source_revision`, and policy verifies the snapshot's internal matched and
unmatched arithmetic. Current Catalog interface coverage is instead proved for
every generated tool: each tool must have one valid `interface_mode` /
availability disposition and retain reviewed provenance to a leaf Contract
declaration (`contract_final`) or another reviewed interface source. This makes
newly added CLI tools explicit without rewriting historical MCP evidence or
promoting an unreviewed selection hint.

Interface metadata contributes lower-priority typed candidates, including
`required`; source precedence is value-neutral for most fields, so a candidate
may raise or lower the Agent-facing value when no higher-priority source wins.
It cannot override leaf Contract declarations, versioned bindings, typed/native
metadata, or current Cobra/constraint facts for those fields. `required` is
special: Cobra `MarkFlagRequired` is a hard floor that cannot be projected away
as optional. `cli_required` remains the executable Cobra marker with its own
provenance.

Production `RegisterSchemaHints` maps are empty after ParamDecl migration.
Temporary `tool_schema_hint` injection is allowed only inside unit-test
fixtures that exercise precedence edges.

```json
{
  "version": 1,
  "source": {
    "kind": "explicit",
    "name": "calendar-schema-review"
  },
  "products": {
    "calendar": {
      "agent_summary": "管理日程、参与人、会议室和闲忙信息"
    }
  },
  "tools": {}
}
```

Run `make generate-schema` after changing selection Hint or Skill sources, or
after changing leaf Contract declarations that feed Schema generation. External
Wukong metadata must be refreshed by the controlled offline import pipeline
with an immutable revision, then committed together with its audit before
regenerating the Catalog; runtime refresh is forbidden.

## Selection prose

Leaf facts live on Contract declarations; `index.metadata` may be `{}` or
omitted. The on-disk `metadata/` directory is retired and must not be
reintroduced as a leaf-fact source.

| Block | Owns |
|---|---|
| leaf Contract (`Safety` / `Schema` / ParamDecl) | safety, interface, parameter facts (`contract_final`) |
| `selection/<product>.json` | `agent_summary`, `use_when`, `avoid_when`, `examples` |

Selection prose is committed as a reviewed source. `go generate` only reads,
validates, and projects this data. It must never call a model, copy a previous
Catalog, or overwrite selection files. Selection cannot change command
identity, flags, parameters, safety, or interface facts.

Commands intentionally kept outside Schema remain in the separate exact
reviewed exclusion file `internal/cli/schema_command_exclusions.json`. An
included command cannot also remain excluded: completeness validation treats
that exclusion as stale.

### Agent editing workflow

1. Locate the real Cobra leaf and verify its exact path and current flags.
2. Declare safety/interface/parameters on the owning leaf; edit
   `selection/<product>.json` for selection prose only.
3. Add only fields that need review. Do not copy generated Catalog fields into
   the input.
4. Run:

   ```bash
   make generate-schema
   ./scripts/policy/check-generated-drift.sh
   ./scripts/policy/check-schema-catalog.sh
   ./scripts/policy/check-runtime-confirmation-truth.sh
   go test ./internal/cli ./internal/app
   ```

   `check-runtime-confirmation-truth.sh` accepts optional/empty metadata
   shells (`tools: {}` or absent directory); gated confirmation truth comes
   from leaf Contract SafetySpec, not from metadata tool rows.

6. Review the generated Catalog diff. A leaf declaration change should affect
   only the intended contract. A selection edit updates prose provenance to
   `reviewed_explicit` from `selection/`; safety/interface provenance stays
   `contract_final`.
