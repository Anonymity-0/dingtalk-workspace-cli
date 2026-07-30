# RFC: Converging the DWS command frameworks onto one typed base

**Status**: draft, seeking review
**Scope**: `internal/cmdcore`, `internal/helpers` (LeafSpec), `internal/shortcut`
**Related PR**: #830 (landed the shared base; supersedes #827 and #829)

This document is self-contained: a reviewer without repository access should be
able to judge the design from it.

---

## 1. Context

`dws` (DingTalk Workspace CLI) is a Go/Cobra CLI whose commands are thin wrappers
that dispatch a JSON-RPC `tools/call` to DingTalk MCP servers. It targets both
humans and AI agents, so every command also publishes an Agent-visible "Runtime
Schema" (parameters, constraints, safety/confirmation semantics).

### 1.1 Three command-definition systems (measured)

| System | Count | Shape | Notes |
|---|---:|---|---|
| Handwritten `cobra.Command` | **1195** | `dws <product> <cmd>` | 96% of commands; `internal/helpers/*.go`, 7.3万 lines |
| Shortcut | **376** | `dws <product> +<cmd>` | Declarative + **multi-step orchestration** |
| LeafSpec | **27** | `dws <product> <cmd>` | Declarative, single-step; only `devapp` |

Duplication measured inside the handwritten commands: `mustGetFlag` ×500,
`callMCPTool*` ×469, `flagOrFallback` ×306, required validation ×262,
`AttachRuntimeSchema`/`AnnotateRuntimeConstraints` ×165 each.

A static AST census classified the 1195 handwritten commands by how they could
migrate:

| Class | Count | Criterion | Disposition |
|---|---:|---|---|
| GROUP | 594 | no MCP call, no loop/IO (pure parent) | not migrated |
| **A** pure CRUD | **141** | exactly 1 MCP call, no loop/IO/confirm | 1:1 migratable |
| **B** confirm/fallback | **92** | 1 MCP call + confirm/dry-run/alias fallback | migratable |
| **C** multi-step / special IO | **368** | 0 or ≥2 MCP calls, loops, upload/poll | needs multi-step support |

So 233/601 (38.8%) of commands with a body are migratable **today**; the 368
C-class ones require the base to model multi-step orchestration.

### 1.2 What already landed (PR #830)

1. `LeafSpec` gained a declarative contract: `at_least_one` / `exactly_one` /
   `mutually_exclusive` constraints, `bool` + `string-slice` flag kinds, and
   `Risk`-driven write confirmation honoring `--dry-run` / `--yes` (including
   root-injected globals, via a 3-level flag lookup).
2. `internal/cmdcore` — a dispatch-agnostic shared base extracted **verbatim**
   from `leaf.go`: flag registration, the `explicit → alias → env → default`
   value chain, required validation, constraint declaration checks + runtime
   enforcement, Risk confirmation, `toolArgs` assembly, Runtime Schema
   projection, help rendering.
3. `cmdcore.CommandSpec` + `cmdcore.NewCommand` — one typed definition and one
   orchestration path; dispatch became a spec field.
4. `helpers.FromLeafSpec`; `NewLeafCommand` is now a one-line mapping. All
   LeafSpec commands (incl. all 27 devapp leaves) flow through the unified spec.
5. `shortcut.FromShortcut` — a typed seam projecting a Shortcut's shared base
   into a `CommandSpec`, deliberately **not** wired into the live `mount()`.

### 1.3 Verification discipline (already enforced, non-negotiable)

- **`check-generated-drift`**: regenerate the Agent metadata + command catalog
  and byte-compare against the committed artifacts. For a refactor, a
  byte-identical catalog (840 tools, identical hashes) is the equivalence proof
  for the **build-time projection** — identity, flags, help, annotations.
- **Runtime semantics are NOT covered by drift.** Required validation, args
  assembly, confirmation and dispatch order are evidenced only by unit tests.
  (An earlier version of this work overstated drift as proof of runtime
  behavior; that claim was corrected.)
- **100% changed-code coverage** gate. Important subtlety: CI runs
  `go test -coverprofile` **without `-coverpkg`**, so each package is measured
  only by its *own* tests. A new package whose logic is exercised only from a
  consumer package will fail the gate (this actually happened: `cmdcore`
  self-coverage was 31.5%, CI changed-code coverage 40.5%, plus an overall
  regression — fixed by giving `cmdcore` its own exhaustive tests).
- **Interface compatibility**: the public Cobra command surface is diffed
  against both the PR merge-base and the latest stable GA tag.

---

## 2. Design review findings

An adversarial review found **no BLOCKERs**: the Phase-1 extraction was
mechanically verified as a verbatim move, and the unified `BoolFlag` was proven
equivalent to the two readers it replaced. The following are the structural
issues that motivate this plan. (Items marked ✅ were already fixed in #830.)

### Structural

**F1. `Dispatch` is single-step biased; the type permits an unrunnable value.**
```go
Dispatch func(cmd *cobra.Command, args []string, toolArgs map[string]any) error
```
This presumes "assemble `toolArgs` → make one call". A Shortcut's
`Execute(rt *RuntimeContext)` is arbitrary multi-step orchestration and **cannot
be expressed**. Consequently `FromShortcut` returned a spec that could not run.
✅ Partially mitigated in #830 by making `NewCommand` reject such a spec — but
that is a patch, not a model fix. **A type that can be constructed into a
permanently unusable state means the model is missing a dimension.**

**F2. The base leaks the MCP payload shape.**
`BuildArgs → map[string]any` is the MCP `tools/call` argument shape, yet it lives
in a package documented as dispatch-agnostic. `devapp` dispatches through
`executor.Runner` and is still handed a `toolArgs` map.

**F3. The same field name carries two semantics.**
- `Required`: cmdcore accepts *any effective value* (including registration
  default and env fallback); Shortcut requires the flag to be `Changed`.
- Decline behavior: LeafSpec/cmdcore return a typed validation error (**exit ≠
  0**); Shortcut's `mount()` returns `nil` (**exit 0**).

These are semantic splits inside a "shared" base — documentation cannot fix them.

**F4. The constraint model is incomplete and asymmetric with the Schema.**

| Capability | Runtime Schema | `cmdcore.Constraint` |
|---|---|---|
| `mutually_exclusive` | ✅ | ✅ |
| `require_one_of` | ✅ | ✅ (`at_least_one` / `exactly_one`) |
| **`require_together`** | ✅ | ❌ |
| `enum` | ✅ (`AnnotateRuntimeFlagEnum`) | ❌ |

Anything unrepresentable falls back to the imperative `Validate` hook, which
**loses the Schema projection and help rendering** — destroying the main value of
the design ("declare once, take effect in three places").

**F5. Too many hooks; `PostMount` is a back door.**
Extension points: `Validate` + `PostMount` + `RunE` + dispatch. `PostMount(cmd)`
can rewrite anything on the built Cobra command (args, annotations, flags), so
any declarative guarantee can be bypassed — which is precisely why drift became
the only safety net. Measured real usage is narrow: `devAppLeafMeta` ×8 (attach
schema identity) and `registerDevAppCursorFlags` ×4 (pagination flags).

### Secondary

**F6. `LeafSpec` is now vestigial** — 179 lines that are mostly comments and type
aliases; the only substantive own fields are the three dispatch fields
(`Server`/`Tool`/`Call`). It is a pure forwarding layer kept for 27 call sites.

**F7. `FlagSpec` has 14 fields with three "two ways to do one thing" pairs**:
`Default` vs `ArgDefault`, `OmitEmpty` vs `Transform`-returning-nil, `Required`
vs `MarkRequired`. This accreted from per-command special cases rather than a
clean model.

**F8. "Single registry" currently covers ~2%** of commands. The 368 C-class
handwritten commands are exactly the ones the current `CommandSpec` cannot
express, so F1 is a prerequisite for coverage, not an optional polish.

**F9. The Schema still has two sources.** `AnnotateConstraints` projects only two
constraint kinds; `required` / `enum` / `require_together` still come from the
separately reviewed registry + hint files. The declarative spec is not yet the
single source of parameter truth.

### What is worth preserving

- **Declare once → runtime validation + Schema projection + `--help`** (the core
  value).
- **Drift-as-equivalence-proof** — makes a 400+ command migration verifiable.
- **Type aliases for zero-churn migration** (`type LeafFlag = cmdcore.FlagSpec`).
- **Dispatch as a spec property** rather than a framework identity.

---

## 3. Plan

### Dependency graph
```
S1 dispatch layering + Ctx ──┬─→ S3 constraints/enum ──┐
                             │                         ├─→ S8 Shortcut live-wire → S9 Schema single source
S2 semantics unification ────┘                         │
   (needs a product decision)                          │
S4 MCP shape push-down → S5 FlagSpec orthogonalization → S6 narrow PostMount → S7 delete LeafSpec
```
S1/S2/S3 are **hard prerequisites** for S8. S4→S7 is an independent cleanup
chain that can proceed in parallel.

---

### S1 — Dispatch layering + unified `Ctx` (biggest lever)

Fixes **F1**; unblocks the 368 C-class commands and S8.

```go
// cmdcore: framework-neutral execution context (knows nothing about MCP)
type Ctx struct { /* cmd, args, declared flags */ }
func (c *Ctx) Command() *cobra.Command
func (c *Ctx) Args() []string
func (c *Ctx) Str(name string) string      // reuses alias → env → default chain
func (c *Ctx) Int(name string) int
func (c *Ctx) Bool(name string) bool
func (c *Ctx) StrSlice(name string) []string
func (c *Ctx) Changed(name string) bool
func (c *Ctx) DryRun() bool
func (c *Ctx) Yes() bool

// CommandSpec: exactly one of the three, validated at CONSTRUCTION time
RunE        func(cmd *cobra.Command, args []string) error // escape hatch
Invoke      func(c *Ctx, toolArgs map[string]any) error   // single-step
Orchestrate func(c *Ctx) error                            // multi-step
```

Notes:
- `Ctx` accessors honor the declared fallback chain, so a body reads exactly the
  value the required/constraint checks saw. This is **stronger** than Shortcut's
  `RuntimeContext`, which reads flags bare.
- **Layering rule**: MCP capability stays out of `cmdcore`.
  `shortcut.RuntimeContext` keeps `CallMCPData`/`CallMCPWriteData`/`Output`; the
  `FromShortcut` adapter builds it from `Ctx.Command()` inside `Orchestrate`.
  (Embedding `*cmdcore.Ctx` into `RuntimeContext` is deferred to S8 to avoid
  changing 376 shortcuts' flag-reading behavior in this step.)
- Construction-time `panic` replaces the run-time error added in #830,
  consistent with the existing `ValidateConstraintDecls` philosophy: a spec with
  no runnable body (or two competing ones) is a programming error that every
  test and startup path trips immediately.

**Gates**: drift zero; leaf suites green; three construction-panic cases;
`Orchestrate` confirmation/decline cases; `Ctx` accessor cases.
**Risk**: low (existing dispatch shifts to `Invoke`).
**Effort**: 1 PR (medium). *Status: in progress on the branch.*

---

### S2 — Semantics unification (contains a **decision that needs sign-off**)

Fixes **F3**.

**S2a. `Required`** — make the divergence explicit instead of implicit:
```go
type RequiredMode int
const (
    RequiredAnyEffectiveValue RequiredMode = iota // = today's LeafSpec behavior (default)
    RequiredMustBePassed                          // = today's Shortcut behavior (Changed)
)
```
During migration both modes coexist; at S8 each shortcut flag maps to
`MustBePassed`, so behavior is unchanged.

**S2b. Decline behavior — ⚠️ user-visible contract change.**
LeafSpec returns an error (exit ≠ 0); Shortcut returns `nil` (exit 0). Options:
1. Converge on **error** (declining a destructive operation should exit non-zero).
   Changes the exit code of 376 shortcuts → needs product sign-off + CHANGELOG.
2. Carry a `DeclineBehavior` field through migration and converge later.
3. Converge on `nil` (weakens the atomic-command contract; not recommended).

**S8 must not start before this decision is made.**

**Gates**: cases for both modes; Shortcut's existing suites unchanged.
**Effort**: 1 PR (small) + 1 decision.

---

### S3 — Complete the constraint model and add `enum`

Fixes **F4**; restores "declare once → three effects".

| Addition | Runtime validation | Schema projection | `--help` |
|---|---|---|---|
| `RequireTogether` constraint | ✅ | `RuntimeSchemaConstraints.RequireTogether` (exists) | ✅ |
| `FlagSpec.Enum` | ✅ | `cli.AnnotateRuntimeFlagEnum` (exists) | `（可选值: a, b）` |
| `Required` projection | — | `cli.AnnotateRuntimeRequiredFlags` (exists) | `（必填）` |

All three annotation APIs already exist in `internal/cli`, so this is wiring, not
new Schema surface.

**By-product**: the base can derive the `必填 / 可选值` help decoration itself, so
`FromShortcut` no longer needs to borrow Shortcut's `flagHelp` (removing a
coupling that caused a real bug — see §4).

**Gates**: drift zero (**new annotations must appear only on commands that
declare them; all existing commands unchanged**) + runtime/help/schema cases for
each addition.
**Risk**: medium (touches Schema annotations; drift is the guard).
**Effort**: 1 PR (medium).

---

### S4 — Push the MCP shape out of the base

Fixes **F2**. Do it while only 27 call sites exist.

```go
// base: CLI-facing contract
FlagSpec{ Name, Usage, Kind, Default, Aliases, EnvVar, Required, RequiredMode, Enum, Trim }
// MCP binding layer: payload shaping
ArgBinding{ Flag, Bind string; Transform func(string)(any,error); OmitEmpty bool; ArgDefault string }
```
`BuildArgs` moves to the MCP binding layer (`helpers` or `cmdcore/mcpbind`).

**Gates**: drift zero + per-key comparison of devapp's 27 commands' `toolArgs`.
**Effort**: 1 PR (medium-large; rewrites 27 call sites).

---

### S5 — Orthogonalize `FlagSpec`

Fixes **F7**.

| Redundancy | Resolution |
|---|---|
| `Default` vs `ArgDefault` | S4 moves `ArgDefault` into `ArgBinding` — resolved naturally |
| `OmitEmpty` vs `Transform`-nil | keep `OmitEmpty` (declarative wins); document `Transform`-nil as the general escape |
| `Required` vs `MarkRequired` | merge into `Required` + `EnforceBy{Framework,Cobra}`; `Cobra` preserves the catalog `required` hard floor |

**Gates**: drift zero (the `MarkRequired` → catalog `required` projection must be
byte-identical).
**Effort**: 1 PR (small-medium).

---

### S6 — Narrow `PostMount`

Fixes **F5**. Measured usage is only two kinds:
- `devAppLeafMeta(cmd, tool)` ×8 → named field `SchemaIdentity{Product, Tool, Source}`
- `registerDevAppCursorFlags(cmd)` ×4 → named `ExtraFlags []FlagSpec` / `Pagination`

`PostMount` survives as an explicitly labelled escape hatch with a call-site
inventory (migration-era asset).

**Gates**: drift zero. **Effort**: 1 PR (small).

---

### S7 — Delete `LeafSpec`

Fixes **F6**. Convert devapp's 27 sites to `CommandSpec` + an MCP dispatch
helper; delete `LeafSpec`, the type aliases, `FromLeafSpec`, `NewLeafCommand`.

**Gates**: drift zero + interface-compatibility dual baseline (the command
surface must not change).
**Effort**: 1 PR (medium).

---

### S8 — Live-wire Shortcut (prerequisites: S1 + S2 + S3)

Migrate per service package (20 packages, 1 PR each), ordered
`usage` (smallest) → `todo`/`contact` → … → `chat`/`aitable` (largest).

**Per-PR gates**
1. **Generalize the existing mount-equivalence test into a table over
   `allShortcuts`**: for every registered shortcut, assert
   `cmdcore.NewCommand(FromShortcut(s))` and `mount(s)` agree byte-for-byte on
   flag set / types / usage / `Long` / annotations. ← the most important guard.
2. `shortcut list` output byte-identical (265 entries).
3. `dws schema --all` byte-identical.
4. drift zero + 100% changed-code coverage.

**Still unmodeled before S8** (must be represented or explicitly rejected):
`Flag.Hidden` (no `CommandSpec` field), `Shortcut.Tips → Example`,
`ConstraintCustom`.

**Effort**: ~20 PRs.

---

### S9 — Schema single source (endgame)

Fixes **F9**. Make the declarative spec the only source of parameter facts
(`required` / `enum` / `require_together` / constraints); `schema_hints` retains
only Agent selection prose (`agent_summary` / `use_when` / `avoid_when`).

**Risk**: high (touches the Schema generation pipeline's division of
responsibility with the reviewed registry). **Effort**: separate project.

---

## 4. Bugs this design review already caught (evidence the gates matter)

Fixed in #830 after review:
- A spec with neither `RunE` nor `Dispatch` ran the whole pipeline — **including
  the write-confirmation prompt** — then silently exited 0 having done nothing.
- `FromShortcut` **double-rendered** the `参数约束` help section (`shortcutLongHelp`
  already appends it; `NewCommand` appended `ConstraintHelp` again) and **dropped**
  the `必填 / 可选值` usage decoration. Both would have silently changed help and
  catalog output at S8.
- A new **mount-equivalence test** (projected flag set/types/usage + rendered
  `Long` vs live `mount(s)`) now catches this whole class; it is the seed of the
  S8 gate.

---

## 5. Handwritten commands (1195)

- After **S1**, the 368 C-class commands become expressible for the first time
  (via `Orchestrate`).
- The 233 A/B-class commands are migratable today, but should be scheduled
  **after S4/S5**, otherwise they get migrated onto the old `FlagSpec` shape and
  then rewritten again.
- The 594 GROUP nodes need no migration.

---

## 6. Global discipline (per PR, non-negotiable)

1. `check-generated-drift` zero drift = build-time projection equivalence proof.
2. Runtime semantics can only be proven by unit tests — drift cannot see them.
3. 100% changed-code coverage, and **a new package must have its own direct
   tests** (never rely on cross-package `coverpkg`; this was a real CI failure).
4. Anything touching a user-visible contract (exit codes, flag names, help text)
   requires a CHANGELOG entry and an explicit human decision.

---

## 7. Questions for the reviewer

1. **S2b**: should declining a risky operation exit non-zero (unify on error), or
   should Shortcut's exit-0 behavior be preserved? This is the only user-visible
   contract change in the plan.
2. **S1**: is `Invoke` / `Orchestrate` / `RunE` with a construction-time
   "exactly one" rule the right factoring, or should dispatch be an interface
   (e.g. `Dispatcher` with two implementations) instead of three nullable
   function fields?
3. **S4**: is splitting `FlagSpec` (CLI-facing) from `ArgBinding` (payload-facing)
   worth the churn on 27 call sites now, versus keeping one struct and accepting
   the leak?
4. **S8 ordering**: is per-service migration with a byte-identity gate the right
   risk posture for 376 shipped shortcuts, or is a parallel "shadow mode"
   (build both, compare at run time) warranted?
5. Is **S9** worth attempting at all, given the reviewed-registry design
   intentionally separates identity/safety review from code?
