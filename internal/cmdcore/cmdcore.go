// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package cmdcore is the shared, dispatch-agnostic base for building leaf
// commands. It concentrates flag registration, the alias/env/default effective
// value fallback chain, required validation, cross-flag constraint declaration
// checks + runtime enforcement, SafetySpec-driven confirmation, toolArgs
// assembly, and Agent Runtime Schema projection.
//
// Declaration vs execution (framework rule):
//
//   - Declare = CommandSpec data fields (Flags, Constraints, Safety,
//     ConstParams, Use/Short/Long/Example). NewCommand registers, validates,
//     confirms, and embeds those facts into dws.schema.*.
//   - Execute = Validate / Invoke / Orchestrate / RunE / PostMount. Hooks
//     consume assembled args; they must not invent the CLI surface.
//   - Annotate = explicit cobra annotations when a fact is not (yet) a Contract
//     field (e.g. write-guard runtime_gate). Inference-only Schema/help is
//     forbidden.
//   - Reviewed non-Contract Schema (identity, selection, interface_*, dry-run,
//     idempotency) has its own sources and must not create CLI flags.
//
// Full ToolSpec field authority: RFC §5.0.4 / homology §1.4.
//
// It is deliberately dispatch-agnostic: it never calls an MCP tool. The
// LeafSpec framework (internal/helpers) and, later, the Shortcut framework wrap
// these primitives and supply their own dispatch (single-step MCP / multi-step
// orchestration / escape hatch). Extracting the primitives here lets both
// frameworks share one flag + constraint + safety + schema base, differing only
// in how they dispatch — the first step toward a single typed command registry.
//
// Behavioral contract: this package is a pure extraction of the logic that
// previously lived in internal/helpers/leaf.go, so flag registration, value
// fallback, required/constraint semantics, confirmation behavior, and schema projection
// stay semantically identical. The evidence is split: check-generated-drift
// (catalog unchanged) proves only the build-time projection — identity, flags,
// help and annotations — while the runtime pipeline (required validation,
// toolArgs assembly, write confirmation, dispatch order) is evidenced solely by
// this package's own tests plus the leaf/risk/constraint unit tests.
package cmdcore

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/cmdutil"
)

// FlagKind is the value type of a flag.
type FlagKind int

const (
	// KindString is a string flag (default).
	KindString FlagKind = iota
	// KindInt is an integer flag (registered as cobra Int); it enters toolArgs
	// only when the value is non-zero, matching the handwritten "putInt only
	// when non-zero" semantics (e.g. devapp app-group-id).
	KindInt
	// KindBool is a boolean flag (registered as cobra Bool); it enters toolArgs
	// only when the user explicitly provided it (Changed), matching the
	// handwritten "transmit on Changed, explicit false is still sent" semantics.
	// Booleans do not participate in the alias/env fallback chain.
	KindBool
	// KindStringSlice is a string-list flag (registered as cobra StringSlice);
	// it enters toolArgs only when a non-empty element exists, elements are
	// always TrimSpace'd and empties dropped.
	KindStringSlice
)

// FlagValidationMode selects runtime validation semantics for a flag.
//
// The zero value keeps LeafSpec's fallback-aware effective-value semantics.
// ValidationShortcut preserves Shortcut's declaration-order checks: Required
// means the user must explicitly provide the flag token (even with a default),
// followed immediately by that flag's Enum validation.
type FlagValidationMode string

const (
	ValidationEffective FlagValidationMode = ""
	ValidationShortcut  FlagValidationMode = "shortcut"
)

// FlagSpec declares how a flag is registered and bound into MCP toolArgs. Its
// fields intentionally mirror the former helpers.LeafFlag one-for-one so that
// helpers can alias to it without touching any call site.
type FlagSpec struct {
	Name    string   // flag name (kebab-case)
	Usage   string   // registration usage text
	Kind    FlagKind // value type, defaults to KindString
	Default string   // registration default for every Kind; also the fallback-chain tail when aliases/env are empty
	Hidden  bool     // hide the real flag from help/Schema while keeping it invocable

	// Required, when true, validates a non-empty effective value in RunE. Plain
	// Required flags aggregate into a cmdutil.ValidateRequiredFlags-compatible
	// error; when EnvVar is configured the env var is a fallback and, still
	// empty, RequiredHint (or a default hint) is reported.
	Required       bool
	ValidationMode FlagValidationMode
	RequiredError  string // exact missing-token error for ValidationShortcut
	RequiredHint   string
	// MarkRequired, when true, calls cobra MarkFlagRequired (the hard floor for
	// the catalog required projection); cobra errors before RunE.
	MarkRequired bool

	Aliases []string // hidden aliases, registered with the main flag's Kind; used in order when the main flag is not explicitly provided
	EnvVar  string   // environment variable consulted when the effective value is empty (an integer flag's env value must be parseable)
	// ArgDefault covers the case where the registration default is empty but
	// toolArgs still needs a fallback. For KindString it is used when the
	// effective value is empty. For KindInt it is also the floor: when the
	// resolved integer is < 1, ArgDefault is emitted instead (cursor page-size
	// semantics).
	ArgDefault string
	// Bind is the toolArgs key; empty uses Name.
	Bind string
	// Transform converts a string effective value into the arg value; nil sends
	// it as-is. Returning (nil, nil) skips the key (for "nullable numeric: skip
	// on empty or parse failure" semantics).
	Transform func(raw string) (any, error)
	// OmitEmpty, when true, drops an empty effective value from toolArgs (KindInt
	// is always "non-zero only" and ignores this field).
	OmitEmpty bool
	// Trim, when true, TrimSpace's the effective value (main flag/alias/env
	// alike) and makes a whitespace-only value count as empty in required checks.
	Trim bool

	// Schema parameter final facts (embedded to dws.schema.*; assembly pass-through).
	Enum              []string // accepted values
	Format            string   // machine-readable format (e.g. uri)
	Example           string   // representative CLI value
	RequiredWhen      string   // conditional required expression (descriptive)
	SchemaDescription string   // Schema description; empty uses Usage
}

// ConstraintKind is the type of a cross-flag relationship constraint. Values
// match the shortcut framework's ConstraintKind verbatim.
type ConstraintKind string

const (
	// AtLeastOne requires at least one of Flags to be provided.
	AtLeastOne ConstraintKind = "at_least_one"
	// ExactlyOne requires exactly one of Flags to be provided.
	ExactlyOne ConstraintKind = "exactly_one"
	// MutuallyExclusive allows at most one of Flags.
	MutuallyExclusive ConstraintKind = "mutually_exclusive"
	// Custom documents validation implemented by CommandSpec.Validate. cmdcore
	// validates the declaration and renders its help, but does not infer the
	// command-specific runtime rule.
	Custom ConstraintKind = "custom"
)

// Constraint declares a relationship over a group of flags. It is enforced
// after required validation and before the framework's Validate hook;
// "provided" reuses the effective-value fallback chain (explicit main flag →
// alias → env), so passing a compatible alias counts as provided — a capability
// the shortcut framework's bare Changed check lacks. The constraint is also
// projected into the Agent Runtime Schema (mutually_exclusive / require_one_of)
// and rendered into the --help "参数约束" section.
type Constraint struct {
	Kind  ConstraintKind
	Flags []string
	// Description, when non-empty, replaces the constraint's default help text.
	Description string
}

// ParameterProjectionMode selects how declared flags are embedded into Runtime
// Schema annotations.
type ParameterProjectionMode string

const (
	// ProjectDeclaredParameters makes the declaration the final parameter
	// authority. This is the LeafSpec/cmdcore default.
	ProjectDeclaredParameters ParameterProjectionMode = ""
	// ProjectCobraParameters preserves Cobra usage/type/default provenance and
	// annotates only facts Cobra cannot express: Required and Enum. Shortcut
	// uses this mode to converge its runtime without rewriting Catalog facts.
	ProjectCobraParameters ParameterProjectionMode = "cobra"
)

// CommandSpec is the single typed definition of a leaf command, shared by the
// LeafSpec and (via FromShortcut) Shortcut frameworks.
//
// Declaration surface is the final Schema data source for managed leaves:
//
//	Flags (+ parameter Schema fields), Constraints, Safety, ConstParams,
//	Use/Short/Long/Example, Schema (ToolSpec groups)
//
// Schema assembly pass-throughs embedded dws.schema.* — no reviewed/hints
// parallel authority for declared fields. Safety uses cli.SafetySpec directly:
// confirmation drives the runtime gate, while effect/risk/idempotency are
// published unchanged. No safety field is inferred from another.
//
// Execution surface (hooks — not declaration):
//
//   - RunE — full escape hatch: the framework only registers flags/constraints/
//     help and hands control over.
//   - Invoke — single-step: runs after required/constraint/Validate checks, args
//     assembly and the Safety confirmation gate, receiving the assembled toolArgs.
//   - Orchestrate — multi-step: runs after the same checks and confirmation but
//     receives only the Ctx, so it can chain several backend calls itself.
//   - Validate / PostMount — orchestration only; must not register business flags
//     or assemble business params that belong in Flags/ConstParams.
//
// Exactly one of RunE / Invoke / Orchestrate must be set; NewCommand validates
// this at construction time. cmdcore itself stays dispatch-agnostic and never
// calls a backend: the adapters (FromLeafSpec / FromShortcut) supply the body.
type CommandSpec struct {
	Use     string
	Short   string
	Long    string
	Example string
	Hidden  bool

	Flags       []FlagSpec
	Constraints []Constraint
	// ParameterProjection controls whether parameter facts are final
	// declaration annotations or Cobra-backed compatibility facts.
	ParameterProjection ParameterProjectionMode
	// Safety is the command's single safety source. The same cli.SafetySpec is
	// used for runtime confirmation and the published Schema. A completely
	// empty value keeps the historical read-only default; a non-empty value
	// must declare effect/risk/confirmation/idempotency together.
	Safety cli.SafetySpec
	// ConfirmFirst runs the Safety confirmation before required/constraint/
	// Validate checks instead of after them. Use it where the legacy semantics
	// were guard-first (a write without --yes fails fast with
	// confirmation_required regardless of parameter completeness). The default
	// preserves the shortcut order (checks first, confirmation just before the
	// backend call).
	ConfirmFirst bool
	// ConstParams are fixed toolArgs merged after flag assembly (e.g. precheckOnly).
	// They are payload declaration, not user flags, and never satisfy Required.
	ConstParams map[string]any
	// Schema is the final ToolSpec payload (identity/selection/safety/…).
	// When non-empty, embed marks the leaf for Schema pass-through.
	Schema SchemaDecl

	// Validate is the cross-flag validation hook, run after required/constraint
	// checks and before args assembly; nil skips it. Not a declaration surface.
	Validate func(cmd *cobra.Command, args []string) error
	// PostMount adjusts the built command after flag registration and before
	// RunE is set (Args/DisableAutoGenTag/annotate/…); always runs. Business
	// flags belong in Flags, not here.
	PostMount func(cmd *cobra.Command)
	// RunE fully replaces the generated body (escape hatch).
	RunE func(cmd *cobra.Command, args []string) error
	// Invoke executes a single-step command with the assembled toolArgs.
	Invoke func(c *Ctx, toolArgs map[string]any) error
	// Orchestrate executes a multi-step command; it assembles whatever payloads
	// it needs from the Ctx.
	Orchestrate func(c *Ctx) error
}

// Ctx is the framework-neutral execution context handed to Invoke/Orchestrate.
// It deliberately knows nothing about MCP or any other backend: it exposes the
// command, its positional args, and typed flag access that reuses the declared
// alias → env → default fallback chain, so a consumer reading a flag through Ctx
// gets exactly the value the required/constraint checks saw.
type Ctx struct {
	cmd   *cobra.Command
	args  []string
	flags map[string]FlagSpec
}

// newCtx builds the execution context for one command invocation.
func newCtx(cmd *cobra.Command, args []string, flags []FlagSpec) *Ctx {
	byName := make(map[string]FlagSpec, len(flags))
	for _, flag := range flags {
		byName[flag.Name] = flag
	}
	return &Ctx{cmd: cmd, args: args, flags: byName}
}

// Command returns the running cobra command.
func (c *Ctx) Command() *cobra.Command { return c.cmd }

// Args returns the positional arguments.
func (c *Ctx) Args() []string { return c.args }

// Str returns a flag's effective string value (explicit → alias → env →
// default). An undeclared name yields "".
func (c *Ctx) Str(name string) string {
	flag, ok := c.flags[name]
	if !ok {
		return ""
	}
	return EffectiveValue(c.cmd, flag)
}

// Int returns a flag's effective integer value; an unparseable or undeclared
// value yields 0 (BuildArgs reports the precise parse error for Invoke specs).
func (c *Ctx) Int(name string) int {
	flag, ok := c.flags[name]
	if !ok {
		return 0
	}
	v, err := integerValue(c.cmd, flag)
	if err != nil {
		return 0
	}
	return int(v)
}

// Bool returns a boolean flag's value.
func (c *Ctx) Bool(name string) bool {
	v, _ := c.cmd.Flags().GetBool(name)
	return v
}

// StrSlice returns a list flag's effective elements (trimmed, empties dropped).
func (c *Ctx) StrSlice(name string) []string {
	flag, ok := c.flags[name]
	if !ok {
		return nil
	}
	return sliceValue(c.cmd, flag)
}

// Changed reports whether the user explicitly passed the flag.
func (c *Ctx) Changed(name string) bool { return c.cmd.Flags().Changed(name) }

// DryRun reports the effective global --dry-run.
func (c *Ctx) DryRun() bool { return BoolFlag(c.cmd, "dry-run") }

// Yes reports the effective global --yes.
func (c *Ctx) Yes() bool { return BoolFlag(c.cmd, "yes") }

// NewCommand builds a cobra command from a CommandSpec. It is the single
// orchestration path: dispatch declaration check → flag registration →
// constraint declaration checks → Runtime Schema projection → constraint help →
// PostMount → generated RunE{ [ConfirmFirst: ConfirmSafety →]
// required → constraints → Validate → BuildArgs → ConfirmSafety →
// Invoke/Orchestrate }.
//
// Behavior matches the former helpers.NewLeafCommand, which always dispatched
// (Call → Server → callMCPTool) and therefore could not express a dispatcher-less
// spec. Here that is a programming error caught at construction time, so a
// malformed spec can never run the pipeline — write-confirmation prompt
// included — and then silently exit 0 having done nothing.
func NewCommand(spec CommandSpec) *cobra.Command {
	validateDispatchDecl(spec)
	validateSafetySpec(spec)
	validateSchemaDecl(spec)
	// Help prose inherits the declaration when not authored separately:
	// Selection.Examples (already contract-validated against the real flags)
	// double as the --help Example block, keeping one authored source.
	example := spec.Example
	if strings.TrimSpace(example) == "" && len(spec.Schema.Selection.Examples) > 0 {
		example = "  " + strings.Join(spec.Schema.Selection.Examples, "\n  ")
	}
	cmd := &cobra.Command{
		Use:     spec.Use,
		Short:   spec.Short,
		Long:    spec.Long,
		Example: example,
		Hidden:  spec.Hidden,
	}
	RegisterFlags(cmd, spec.Flags)
	ValidateConstraintDecls(spec.Use, spec.Flags, spec.Constraints)
	embedContractIntoSchema(cmd, spec)
	AnnotateConstraints(cmd, spec.Constraints)
	if help := ConstraintHelp(spec.Constraints); help != "" {
		cmd.Long = strings.TrimRight(cmd.Long, "\n") + help
	}
	if spec.PostMount != nil {
		spec.PostMount(cmd)
	}
	if spec.RunE != nil {
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if err := ConfirmSafety(cmd, spec.Safety); err != nil {
				return err
			}
			return spec.RunE(cmd, args)
		}
		return cmd
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if spec.ConfirmFirst {
			if err := ConfirmSafety(cmd, spec.Safety); err != nil {
				return err
			}
		}
		if err := ValidateRequired(cmd, spec.Flags); err != nil {
			return err
		}
		if err := ValidateEnums(cmd, spec.Flags); err != nil {
			return err
		}
		if err := ValidateConstraints(cmd, spec.Flags, spec.Constraints); err != nil {
			return err
		}
		if spec.Validate != nil {
			if err := spec.Validate(cmd, args); err != nil {
				return err
			}
		}
		ctx := newCtx(cmd, args, spec.Flags)
		if spec.Orchestrate != nil {
			if !spec.ConfirmFirst {
				if err := ConfirmSafety(cmd, spec.Safety); err != nil {
					return err
				}
			}
			return spec.Orchestrate(ctx)
		}
		toolArgs, err := BuildArgs(cmd, spec.Flags)
		if err != nil {
			return err
		}
		for key, value := range spec.ConstParams {
			toolArgs[key] = value
		}
		if !spec.ConfirmFirst {
			if err := ConfirmSafety(cmd, spec.Safety); err != nil {
				return err
			}
		}
		return spec.Invoke(ctx, toolArgs)
	}
	return cmd
}

// validateDispatchDecl enforces "exactly one dispatcher" at build time. Like
// ValidateConstraintDecls this panics: a spec with no runnable body (or with two
// competing ones) is a programming error that every test and startup path should
// trip immediately, not a condition to surface at user run time.
func validateDispatchDecl(spec CommandSpec) {
	declared := 0
	if spec.RunE != nil {
		declared++
	}
	if spec.Invoke != nil {
		declared++
	}
	if spec.Orchestrate != nil {
		declared++
	}
	if declared != 1 {
		panic(fmt.Sprintf(
			"command %q must declare exactly one of RunE/Invoke/Orchestrate, got %d",
			spec.Use, declared))
	}
	// ConfirmFirst only changes the ordering of a declared confirmation gate.
	if spec.ConfirmFirst && strings.TrimSpace(spec.Safety.Confirmation) != "user_required" {
		panic(fmt.Sprintf(
			"command %q sets ConfirmFirst but Safety.Confirmation is not user_required",
			spec.Use))
	}
}

// validateSafetySpec rejects partial safety declarations. A zero value remains
// the historical read-only default, but once any field is authored all four
// independent Schema dimensions must be explicit.
func validateSafetySpec(spec CommandSpec) {
	safety := spec.Safety
	fields := []struct {
		name  string
		value string
	}{
		{"Safety.Effect", safety.Effect},
		{"Safety.Risk", safety.Risk},
		{"Safety.Confirmation", safety.Confirmation},
		{"Safety.Idempotency", safety.Idempotency},
	}
	declared := false
	for _, field := range fields {
		if strings.TrimSpace(field.value) != "" {
			declared = true
			break
		}
	}
	if !declared {
		return
	}
	missing := make([]string, 0, len(fields))
	for _, field := range fields {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, field.name)
		}
	}
	if len(missing) > 0 {
		panic(fmt.Sprintf(
			"command %q declares partial SafetySpec; missing %s: safety fields are independent and are never inferred from one another",
			spec.Use, strings.Join(missing, ", ")))
	}
}

// RegisterFlags registers every flag (plus hidden aliases and MarkFlagRequired)
// declared by the spec set onto cmd.
func RegisterFlags(cmd *cobra.Command, flags []FlagSpec) {
	for _, flag := range flags {
		RegisterFlag(cmd, flag.Kind, flag.Name, flag.Default, flag.Usage)
		// Aliases are registered with the main flag's Kind, otherwise an integer
		// alias's value would never be readable (silently dropped).
		for _, alias := range flag.Aliases {
			RegisterFlag(cmd, flag.Kind, alias, "", flag.Usage+" (alias)")
			_ = cmd.Flags().MarkHidden(alias)
		}
		if flag.MarkRequired {
			_ = cmd.MarkFlagRequired(flag.Name)
		}
		if flag.Hidden {
			_ = cmd.Flags().MarkHidden(flag.Name)
		}
	}
}

// RegisterFlag registers one flag by Kind. Default is applied at registration
// for every kind so --help DefValue matches the declared fallback.
func RegisterFlag(cmd *cobra.Command, kind FlagKind, name, def, usage string) {
	switch kind {
	case KindInt:
		defInt := 0
		if def != "" {
			if v, err := strconv.Atoi(def); err == nil {
				defInt = v
			}
		}
		cmd.Flags().Int(name, defInt, usage)
	case KindBool:
		cmd.Flags().Bool(name, def == "true", usage)
	case KindStringSlice:
		var defaults []string
		if value := strings.TrimSpace(def); value != "" {
			defaults = strings.Split(value, ",")
		}
		cmd.Flags().StringSlice(name, defaults, usage)
	default:
		cmd.Flags().String(name, def, usage)
	}
}

// ValidateRequired reproduces the handwritten required semantics: plain Required
// flags report a unified "missing required flag(s)" error; Required flags with
// EnvVar/RequiredHint report their hint separately. The plain group is checked
// before the env group to preserve the handwritten order. Both groups use the
// declared "main flag → alias → env" fallback: a compatible alias counts as
// provided.
func ValidateRequired(cmd *cobra.Command, flags []FlagSpec) error {
	for _, flag := range flags {
		if flag.ValidationMode != ValidationShortcut {
			continue
		}
		if flag.Required {
			registered := cmd.Flags().Lookup(flag.Name)
			if registered == nil || !registered.Changed {
				message := strings.TrimSpace(flag.RequiredError)
				if message == "" {
					message = fmt.Sprintf("缺少必填参数 --%s", flag.Name)
				}
				return apperrors.NewValidation(message)
			}
			switch flag.Kind {
			case KindStringSlice:
				values, _ := cmd.Flags().GetStringSlice(flag.Name)
				if !sliceHasValue(values) {
					return apperrors.NewValidation(fmt.Sprintf("必填参数 --%s 不能为空", flag.Name))
				}
			case KindString:
				value, _ := cmd.Flags().GetString(flag.Name)
				if strings.TrimSpace(value) == "" {
					return apperrors.NewValidation(fmt.Sprintf("必填参数 --%s 不能为空", flag.Name))
				}
			}
		}
		if err := validateEnum(cmd, flag); err != nil {
			return err
		}
	}

	var plain []string
	for _, flag := range flags {
		if flag.Required && flag.ValidationMode != ValidationShortcut &&
			flag.EnvVar == "" && flag.RequiredHint == "" && !hasEffectiveValue(cmd, flag) {
			plain = append(plain, flag.Name)
		}
	}
	if err := cmdutil.MissingRequiredFlagsError(cmd, plain...); err != nil {
		return err
	}
	for _, flag := range flags {
		if !flag.Required || flag.ValidationMode == ValidationShortcut ||
			(flag.EnvVar == "" && flag.RequiredHint == "") {
			continue
		}
		if !hasEffectiveValue(cmd, flag) {
			hint := flag.RequiredHint
			if hint == "" {
				hint = fmt.Sprintf("flag --%s is required", flag.Name)
			}
			return fmt.Errorf("%s", hint)
		}
	}
	return nil
}

// ValidateEnums enforces the accepted values declared on changed flags. A
// registration default does not trigger validation, matching Shortcut's
// historical behavior.
func ValidateEnums(cmd *cobra.Command, flags []FlagSpec) error {
	for _, flag := range flags {
		if flag.ValidationMode == ValidationShortcut {
			continue
		}
		if err := validateEnum(cmd, flag); err != nil {
			return err
		}
	}
	return nil
}

func validateEnum(cmd *cobra.Command, flag FlagSpec) error {
	if len(flag.Enum) == 0 || !cmd.Flags().Changed(flag.Name) {
		return nil
	}
	values := []string{flagString(cmd, flag.Kind, flag.Name)}
	if flag.Kind == KindStringSlice {
		values, _ = cmd.Flags().GetStringSlice(flag.Name)
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		valid := false
		for _, allowed := range flag.Enum {
			if value == allowed {
				valid = true
				break
			}
		}
		if !valid {
			return apperrors.NewValidation(fmt.Sprintf(
				"参数 --%s 取值 %q 不合法，允许值：%s",
				flag.Name, value, strings.Join(flag.Enum, ", ")))
		}
	}
	return nil
}

// hasEffectiveValue decides whether a Required flag is satisfied, matching the
// BuildArgs entry predicate (KindInt non-zero, string non-empty, KindBool
// explicitly provided, KindStringSlice has a non-empty element). Integer parse
// failure counts as provided so BuildArgs reports the precise invalid-integer
// error.
func hasEffectiveValue(cmd *cobra.Command, flag FlagSpec) bool {
	switch flag.Kind {
	case KindInt:
		v, err := integerValue(cmd, flag)
		if err != nil {
			return true
		}
		return v != 0
	case KindBool:
		return cmd.Flags().Changed(flag.Name)
	case KindStringSlice:
		return sliceValue(cmd, flag) != nil
	}
	return EffectiveValue(cmd, flag) != ""
}

// sliceValue reads a list flag's effective value by "explicit main flag →
// explicit alias" order: elements are TrimSpace'd and empties dropped, and an
// all-empty result counts as not provided (returns nil). Lists do not
// participate in the env/Default fallback chain.
func sliceValue(cmd *cobra.Command, flag FlagSpec) []string {
	names := append([]string{flag.Name}, flag.Aliases...)
	for _, name := range names {
		if !cmd.Flags().Changed(name) {
			continue
		}
		raw, _ := cmd.Flags().GetStringSlice(name)
		var out []string
		for _, value := range raw {
			if value = strings.TrimSpace(value); value != "" {
				out = append(out, value)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

// BuildArgs assembles toolArgs from the flag set by binding relationship.
func BuildArgs(cmd *cobra.Command, flags []FlagSpec) (map[string]any, error) {
	toolArgs := map[string]any{}
	for _, flag := range flags {
		bind := flag.Bind
		if bind == "" {
			bind = flag.Name
		}
		if flag.Kind == KindInt {
			v, err := integerValue(cmd, flag)
			if err != nil {
				return nil, err
			}
			// ArgDefault floors values < 1 (cursor page-size: 0/-1 → default).
			if v < 1 && flag.ArgDefault != "" {
				parsed, err := strconv.ParseInt(flag.ArgDefault, 10, 64)
				if err != nil {
					return nil, fmt.Errorf("flag --%s: invalid ArgDefault %q", flag.Name, flag.ArgDefault)
				}
				v = parsed
			}
			// Keep "non-zero only" (putInt semantics).
			if v != 0 {
				toolArgs[bind] = int(v)
			}
			continue
		}
		if flag.Kind == KindBool {
			// Enter on Changed only: explicit false is still sent (matching the
			// handwritten "transmit on Changed" semantics).
			if cmd.Flags().Changed(flag.Name) {
				v, _ := cmd.Flags().GetBool(flag.Name)
				toolArgs[bind] = v
			}
			continue
		}
		if flag.Kind == KindStringSlice {
			if v := sliceValue(cmd, flag); v != nil {
				toolArgs[bind] = v
			}
			continue
		}
		effective := EffectiveValue(cmd, flag)
		if effective == "" && flag.ArgDefault != "" {
			effective = flag.ArgDefault
		}
		if effective == "" && flag.OmitEmpty {
			continue
		}
		if flag.Transform != nil {
			value, err := flag.Transform(effective)
			if err != nil {
				return nil, err
			}
			if value == nil {
				continue
			}
			toolArgs[bind] = value
			continue
		}
		toolArgs[bind] = effective
	}
	return toolArgs, nil
}

// EffectiveValue reads the value by "explicit main flag → alias → env →
// registration default" order (string form, integers uniformly formatted);
// Trim TrimSpace's the result.
func EffectiveValue(cmd *cobra.Command, flag FlagSpec) string {
	v := rawValue(cmd, flag)
	if flag.Trim {
		v = strings.TrimSpace(v)
	}
	return v
}

// rawValue reads the un-trimmed effective value. The main flag wins only when
// explicitly provided (Changed) and non-empty; the registration default is
// demoted to a chain tail and no longer shadows aliases/env. When Trim is set,
// candidates are judged empty after trimming, so whitespace-only and empty fall
// through to the next fallback level.
func rawValue(cmd *cobra.Command, flag FlagSpec) string {
	usable := func(v string) bool {
		if flag.Trim {
			v = strings.TrimSpace(v)
		}
		return v != ""
	}
	if cmd.Flags().Changed(flag.Name) {
		if v := flagString(cmd, flag.Kind, flag.Name); usable(v) {
			return v
		}
	}
	for _, alias := range flag.Aliases {
		if !cmd.Flags().Changed(alias) {
			continue
		}
		if v := flagString(cmd, flag.Kind, alias); usable(v) {
			return v
		}
	}
	if flag.EnvVar != "" {
		if v := os.Getenv(flag.EnvVar); usable(v) {
			return v
		}
	}
	return flag.Default
}

// flagString reads a flag by registered type and normalizes to string form so
// integer flags can reuse the same fallback chain (required checks, aliases, env).
func flagString(cmd *cobra.Command, kind FlagKind, name string) string {
	switch kind {
	case KindInt:
		v, _ := cmd.Flags().GetInt(name)
		return strconv.Itoa(v)
	default:
		return cmdutil.MustGetFlag(cmd, name)
	}
}

// integerValue reads an integer flag's effective value by the fallback chain; an
// env-provided string must be parseable, otherwise it errors rather than
// silently dropping.
func integerValue(cmd *cobra.Command, flag FlagSpec) (int64, error) {
	raw := EffectiveValue(cmd, flag)
	if raw == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("flag --%s: invalid integer value %q", flag.Name, raw)
	}
	return v, nil
}

// ValidateConstraintDecls validates constraint declarations at build time: an
// unknown kind, an under-sized flag group, or a reference to an undeclared flag
// is a programming error and panics so any test/startup path fails immediately
// rather than at user runtime. use is only used for the panic message.
func ValidateConstraintDecls(use string, flags []FlagSpec, constraints []Constraint) {
	declared := map[string]bool{}
	for _, flag := range flags {
		declared[flag.Name] = true
	}
	for _, constraint := range constraints {
		switch constraint.Kind {
		case AtLeastOne, ExactlyOne, MutuallyExclusive:
			if len(constraint.Flags) < 2 {
				panic(fmt.Sprintf("command %q: constraint %s needs at least two flags", use, constraint.Kind))
			}
		case Custom:
			if len(constraint.Flags) < 1 {
				panic(fmt.Sprintf("command %q: constraint %s needs at least one flag", use, constraint.Kind))
			}
			if strings.TrimSpace(constraint.Description) == "" {
				panic(fmt.Sprintf("command %q: custom constraint requires a description", use))
			}
		default:
			panic(fmt.Sprintf("command %q: unknown constraint kind %q", use, constraint.Kind))
		}
		for _, name := range constraint.Flags {
			if !declared[name] {
				panic(fmt.Sprintf("command %q: constraint %s references undeclared flag %q", use, constraint.Kind, name))
			}
		}
	}
}

// constraintProvided decides whether a flag is "provided" for constraint
// purposes: an explicit main flag, explicit alias, or env var counts; the
// registration default/ArgDefault does not — otherwise a defaulted flag would
// always satisfy at_least_one and always trip mutually_exclusive. KindBool only
// counts Changed (booleans have no alias/env fallback semantics).
func constraintProvided(cmd *cobra.Command, flag FlagSpec) bool {
	switch flag.Kind {
	case KindBool:
		return cmd.Flags().Changed(flag.Name)
	case KindStringSlice:
		if cmd.Flags().Changed(flag.Name) {
			if v, _ := cmd.Flags().GetStringSlice(flag.Name); sliceHasValue(v) {
				return true
			}
		}
		for _, alias := range flag.Aliases {
			if !cmd.Flags().Changed(alias) {
				continue
			}
			if v, _ := cmd.Flags().GetStringSlice(alias); sliceHasValue(v) {
				return true
			}
		}
		return false
	}
	usable := func(v string) bool { return strings.TrimSpace(v) != "" }
	if cmd.Flags().Changed(flag.Name) && usable(flagString(cmd, flag.Kind, flag.Name)) {
		return true
	}
	for _, alias := range flag.Aliases {
		if cmd.Flags().Changed(alias) && usable(flagString(cmd, flag.Kind, alias)) {
			return true
		}
	}
	return flag.EnvVar != "" && usable(os.Getenv(flag.EnvVar))
}

// ValidateConstraints enforces the relationship constraints. Error wording
// matches the shortcut framework's RuntimeContext.AtLeastOne/ExactlyOne/
// MutuallyExclusive verbatim, so atomic commands and smart shortcuts fail
// identically for users and agents.
func ValidateConstraints(cmd *cobra.Command, flags []FlagSpec, constraints []Constraint) error {
	flagsByName := map[string]FlagSpec{}
	for _, flag := range flags {
		flagsByName[flag.Name] = flag
	}
	for _, constraint := range constraints {
		var set []string
		for _, name := range constraint.Flags {
			if constraintProvided(cmd, flagsByName[name]) {
				set = append(set, name)
			}
		}
		switch constraint.Kind {
		case AtLeastOne:
			if len(set) == 0 {
				return apperrors.NewValidation(fmt.Sprintf(
					"请至少指定 %s 之一", dashed(constraint.Flags)))
			}
		case ExactlyOne:
			switch len(set) {
			case 1:
			case 0:
				return apperrors.NewValidation(fmt.Sprintf("请指定 %s 之一", dashed(constraint.Flags)))
			default:
				return apperrors.NewValidation(fmt.Sprintf(
					"参数 %s 只能指定其一（当前指定了 %s）", dashed(constraint.Flags), dashed(set)))
			}
		case MutuallyExclusive:
			if len(set) > 1 {
				return apperrors.NewValidation(fmt.Sprintf(
					"参数 %s 互斥，只能指定其一（当前指定了 %s）", dashed(constraint.Flags), dashed(set)))
			}
		case Custom:
			// The declaration is published and rendered in help. Its actual
			// command-specific rule remains owned by CommandSpec.Validate.
		}
	}
	return nil
}

// ConfirmSafety enforces the command's declared confirmation requirement.
// Effect, risk and idempotency are metadata only and never imply confirmation.
// Semantics:
//
//   - read-only, --dry-run, or --yes → nil (proceed);
//   - interactive yes/y → nil;
//   - interactive decline → validation "用户取消了操作" (existing cmdcore path);
//   - no interactive answer (EOF / closed stdin) → confirmation_required.
//
// EOF must not be treated as decline: that silently drops writes in agent/CI.
func ConfirmSafety(cmd *cobra.Command, safety cli.SafetySpec) error {
	if strings.TrimSpace(safety.Confirmation) != "user_required" ||
		BoolFlag(cmd, "dry-run") || BoolFlag(cmd, "yes") {
		return nil
	}
	// Only print the interactive prompt on a real terminal. In non-interactive
	// environments (agent/CI: pipe, closed stdin, /dev/null) the prompt is
	// noise on stderr ahead of the structured confirmation_required error —
	// callers there must pass --yes/--dry-run instead of answering. A piped
	// answer (printf 'yes\n' | cmd) is still honored: the read happens either
	// way, only the prompt print is terminal-gated.
	if stdinIsTerminal(cmd.InOrStdin()) {
		fmt.Fprintf(
			cmd.ErrOrStderr(),
			"即将执行 %s（effect=%s, risk=%s），确认继续？(yes/no): ",
			cmd.CommandPath(), strings.TrimSpace(safety.Effect), strings.TrimSpace(safety.Risk),
		)
	}
	reader := bufio.NewReader(cmd.InOrStdin())
	answer, err := reader.ReadString('\n')
	if errors.Is(err, io.EOF) && strings.TrimSpace(answer) == "" {
		return confirmationRequiredError(cmd.CommandPath())
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return confirmationRequiredError(cmd.CommandPath())
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	if answer == "yes" || answer == "y" {
		return nil
	}
	return apperrors.NewValidation("用户取消了操作")
}

// stdinIsTerminal reports whether the given input is a real terminal. Only
// *os.File inputs can be terminals; cobra SetIn buffers and other readers are
// treated as non-interactive. An ioctl-level TTY check is required — a plain
// character-device stat would misclassify redirects like `< /dev/null`.
func stdinIsTerminal(in io.Reader) bool {
	file, ok := in.(*os.File)
	if !ok {
		return false
	}
	fd := file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func confirmationRequiredError(operation string) error {
	return apperrors.NewValidation(
		fmt.Sprintf("%s 需要用户确认，当前环境无法交互；加 --dry-run 预览，或确认后加 --yes 执行", operation),
		apperrors.WithReason("confirmation_required"),
		apperrors.WithHint("非交互环境（agent/CI）必须显式传入 --yes，不能依赖 stdin 提示"),
		apperrors.WithActions("确认目标与变更影响", "以相同参数追加 --yes 执行"),
	)
}

// BoolFlag robustly reads a bool flag that may live on the command, its
// inherited flags, or the root's persistent flags (e.g. root-injected global
// --yes / --dry-run). Returns the first flagset that resolves the name.
func BoolFlag(cmd *cobra.Command, name string) bool {
	if cmd == nil {
		return false
	}
	getters := []func(string) (bool, error){
		cmd.Flags().GetBool,
		cmd.InheritedFlags().GetBool,
	}
	if root := cmd.Root(); root != nil {
		getters = append(getters, root.PersistentFlags().GetBool)
	}
	for _, get := range getters {
		if v, err := get(name); err == nil {
			return v
		}
	}
	return false
}

// embedContractIntoSchema projects CommandSpec declaration onto the live Cobra
// leaf as the final Schema payload (dws.schema.*). Assembly pass-throughs these
// annotations; declared fields do not compete with reviewed hints.
func embedContractIntoSchema(cmd *cobra.Command, spec CommandSpec) {
	if spec.ParameterProjection == ProjectCobraParameters {
		for _, flag := range spec.Flags {
			name := strings.TrimSpace(flag.Name)
			if name == "" || flag.Hidden {
				continue
			}
			if flag.Required || flag.MarkRequired {
				cli.AnnotateRuntimeRequiredFlags(cmd, name)
			}
			if len(flag.Enum) > 0 {
				cli.AnnotateRuntimeFlagEnum(cmd, name, flag.Enum...)
			}
		}
		embedSchemaDecl(cmd, spec)
		return
	}

	cli.AnnotateRuntimeContract(cmd)
	required := make([]string, 0, len(spec.Flags))
	for _, flag := range spec.Flags {
		name := strings.TrimSpace(flag.Name)
		if name == "" || flag.Hidden {
			continue
		}
		requiredFlag := flag.Required || flag.MarkRequired
		cli.AnnotateRuntimeFlag(cmd, name, strings.TrimSpace(flag.Bind), flagKindSchemaType(flag.Kind), requiredFlag, "")
		desc := strings.TrimSpace(flag.SchemaDescription)
		if desc == "" {
			desc = strings.TrimSpace(flag.Usage)
		}
		if desc != "" {
			cli.AnnotateRuntimeFlagDescription(cmd, name, desc)
		}
		if flag.RequiredWhen != "" {
			cli.AnnotateRuntimeFlagRequiredWhen(cmd, name, flag.RequiredWhen)
		}
		if flag.Format != "" {
			cli.AnnotateRuntimeFlagFormat(cmd, name, flag.Format)
		}
		if flag.Example != "" {
			cli.AnnotateRuntimeFlagExample(cmd, name, flag.Example)
		}
		if len(flag.Enum) > 0 {
			cli.AnnotateRuntimeFlagEnum(cmd, name, flag.Enum...)
		}
		if requiredFlag {
			required = append(required, name)
		}
	}
	cli.AnnotateRuntimeRequiredFlags(cmd, required...)
	embedSchemaDecl(cmd, spec)
}

// embedSchemaDecl does a light runtime write: only when SchemaDecl is authored,
// convert once and RegisterRuntimeContractFinal.
func embedSchemaDecl(cmd *cobra.Command, spec CommandSpec) {
	if spec.Schema.empty() {
		return
	}
	AttachSchema(cmd, spec.Safety, spec.Schema, spec.Short, spec.Long)
}

// AttachSchema registers a ContractFinal Schema overlay on an existing leaf
// without replacing its RunE/Execute body. Used to migrate reviewed hint facts
// onto helpers while keeping execution substance frozen. Overwrites any prior
// ContractFinal on cmd (catalog/agent metadata source); does not alter an
// already-installed ConfirmSafety closure.
func AttachSchema(cmd *cobra.Command, safety cli.SafetySpec, schema SchemaDecl, short, long string) {
	if cmd == nil || schema.empty() {
		return
	}
	// Reuse NewCommand's completeness rules so bind-time attaches cannot ship
	// a partial declaration that would only fail in generated artifacts.
	validateSchemaDecl(CommandSpec{Use: cmd.Name(), Safety: safety, Schema: schema})
	validateSafetySpec(CommandSpec{Use: cmd.Name(), Safety: safety})

	payload := cli.ContractFinalPayload{
		Title:       firstNonEmpty(schema.Title, short),
		Description: firstNonEmpty(schema.Description, long),
		Safety:      schemaSafetyFromDecl(safety),
	}
	if n := len(schema.Positionals); n > 0 {
		payload.Positionals = make([]cli.RuntimeSchemaPositional, n)
		for i, p := range schema.Positionals {
			payload.Positionals[i] = cli.RuntimeSchemaPositional{
				Name: p.Name, Type: p.Type, Description: p.Description,
				Required: p.Required, Variadic: p.Variadic, Index: p.Index,
			}
		}
	}
	if schema.DryRun != nil && strings.TrimSpace(schema.DryRun.PreviewKind) != "" {
		payload.DryRun = &cli.DryRunSpec{
			PreviewKind: strings.TrimSpace(schema.DryRun.PreviewKind),
			RemoteReads: schema.DryRun.RemoteReads,
		}
	}
	if schema.Interface != nil {
		iface := &cli.InterfaceSpec{
			Mode:         strings.TrimSpace(schema.Interface.Mode),
			Availability: strings.TrimSpace(schema.Interface.Availability),
			Reason:       strings.TrimSpace(schema.Interface.Reason),
		}
		if pid := strings.TrimSpace(schema.Interface.ProductID); pid != "" {
			iface.Ref = &cli.InterfaceRefSpec{ProductID: pid, RPCName: strings.TrimSpace(schema.Interface.RPCName)}
		}
		if iface.Mode != "" || iface.Ref != nil || iface.Availability != "" || iface.Reason != "" {
			payload.Interface = iface
		}
	}
	if sel := schema.Selection; strings.TrimSpace(sel.AgentSummary) != "" || len(sel.UseWhen) > 0 ||
		len(sel.AvoidWhen) > 0 || len(sel.Examples) > 0 || len(sel.Prerequisites) > 0 ||
		len(sel.Tips) > 0 || len(sel.WorkflowRefs) > 0 {
		payload.Selection = &cli.SelectionSpec{
			AgentSummary: strings.TrimSpace(sel.AgentSummary),
			UseWhen:      sel.UseWhen, AvoidWhen: sel.AvoidWhen,
			Prerequisites: sel.Prerequisites, Tips: sel.Tips,
			WorkflowRefs: sel.WorkflowRefs, Examples: sel.Examples,
		}
	}
	if id := schema.Identity; strings.TrimSpace(id.ProductID) != "" || strings.TrimSpace(id.Name) != "" {
		payload.Identity = &cli.ToolIdentitySpec{
			ProductID: strings.TrimSpace(id.ProductID), SourceProductID: strings.TrimSpace(id.SourceProductID),
			Name: strings.TrimSpace(id.Name), CLIName: strings.TrimSpace(id.CLIName),
			CanonicalPath: strings.TrimSpace(id.CanonicalPath), CLIPath: strings.TrimSpace(id.CLIPath),
			PrimaryCLIPath: strings.TrimSpace(id.PrimaryCLIPath), Group: strings.TrimSpace(id.Group),
			Aliases: id.Aliases, Source: strings.TrimSpace(id.Source),
		}
	}
	cli.RegisterRuntimeContractFinal(cmd, payload)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// schemaSafetyFromDecl copies the single command SafetySpec into the final
// Schema payload. The zero value keeps the historical read-only default.
func schemaSafetyFromDecl(safety cli.SafetySpec) *cli.SafetySpec {
	out := effectiveSafetySpec(safety)
	out.EffectSource = "cmdcore.contract"
	return &out
}

func effectiveSafetySpec(safety cli.SafetySpec) cli.SafetySpec {
	out := cli.SafetySpec{
		Effect:       strings.TrimSpace(safety.Effect),
		Risk:         strings.TrimSpace(safety.Risk),
		Confirmation: strings.TrimSpace(safety.Confirmation),
		Idempotency:  strings.TrimSpace(safety.Idempotency),
	}
	if out.Effect == "" && out.Risk == "" && out.Confirmation == "" && out.Idempotency == "" {
		out.Effect = "read"
		out.Risk = "low"
		out.Confirmation = "not_required"
		out.Idempotency = "idempotent"
	}
	return out
}

func flagKindSchemaType(kind FlagKind) string {
	switch kind {
	case KindInt:
		return "integer"
	case KindBool:
		return "boolean"
	case KindStringSlice:
		return "array"
	default:
		return "string"
	}
}

// AnnotateConstraints projects the relationship constraints into the Agent
// Runtime Schema: exactly_one decomposes into require_one_of + mutually_exclusive
// (matching the handwritten commands' use of AnnotateRuntimeConstraints).
func AnnotateConstraints(cmd *cobra.Command, constraints []Constraint) {
	var projected cli.RuntimeSchemaConstraints
	var required []string
	for _, constraint := range constraints {
		flags := make([]string, 0, len(constraint.Flags))
		for _, name := range constraint.Flags {
			flag := cmd.Flags().Lookup(name)
			if flag != nil && !flag.Hidden {
				flags = append(flags, name)
			}
		}
		switch constraint.Kind {
		case AtLeastOne:
			if len(flags) == 1 {
				required = append(required, flags[0])
			} else if len(flags) > 1 {
				projected.RequireOneOf = append(projected.RequireOneOf, flags)
			}
		case ExactlyOne:
			if len(flags) == 1 {
				required = append(required, flags[0])
			} else if len(flags) > 1 {
				projected.RequireOneOf = append(projected.RequireOneOf, flags)
				projected.MutuallyExclusive = append(projected.MutuallyExclusive, flags)
			}
		case MutuallyExclusive:
			if len(flags) > 1 {
				projected.MutuallyExclusive = append(projected.MutuallyExclusive, flags)
			}
		}
	}
	cli.AnnotateRuntimeRequiredFlags(cmd, required...)
	cli.AnnotateRuntimeConstraints(cmd, projected)
}

// ConstraintHelp renders the --help "参数约束" section, matching the shortcut
// leaf help shape; returns "" when there are no constraints.
func ConstraintHelp(constraints []Constraint) string {
	if len(constraints) == 0 {
		return ""
	}
	lines := make([]string, 0, len(constraints))
	for _, constraint := range constraints {
		text := strings.TrimSpace(constraint.Description)
		if text == "" {
			switch constraint.Kind {
			case AtLeastOne:
				text = fmt.Sprintf("%s 至少指定一个", dashed(constraint.Flags))
			case ExactlyOne:
				text = fmt.Sprintf("%s 必须且只能指定一个", dashed(constraint.Flags))
			case MutuallyExclusive:
				text = fmt.Sprintf("%s 互斥，最多指定一个", dashed(constraint.Flags))
			}
		}
		lines = append(lines, "  - "+text)
	}
	return "\n\n参数约束：\n" + strings.Join(lines, "\n")
}

func dashed(flags []string) string {
	out := make([]string, len(flags))
	for i, f := range flags {
		out[i] = "--" + f
	}
	return strings.Join(out, "、")
}

func sliceHasValue(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return true
		}
	}
	return false
}
