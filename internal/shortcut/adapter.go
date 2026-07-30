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

package shortcut

import (
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cmdcore"
)

// adapter.go bridges the Shortcut framework toward the unified cmdcore typed
// spec. FromShortcut maps a Shortcut's SHARED BASE (flags, constraints, risk,
// help identity) into a cmdcore.CommandSpec so both frameworks can eventually
// share one flag + constraint + risk base.
//
// Scope (Phase 2): this is the typed seam only — it is NOT wired into the live
// mount() path. The Shortcut runtime keeps its own mount/RuntimeContext/Execute
// so the shipped shortcuts are provably unaffected (catalog + `shortcut list`
// unchanged). The following Shortcut semantics are NOT modeled by cmdcore and
// are therefore dropped by this projection — Phase 3 must extend CommandSpec (or
// reject such specs) before wiring it in:
//
//   - multi-step orchestration: Shortcut.Execute(rt) uses RuntimeContext with
//     CallMCPData/CallMCPWriteData; cmdcore's single-step Dispatch(toolArgs)
//     cannot express it. FromShortcut therefore leaves Dispatch/RunE nil, and
//     cmdcore.NewCommand rejects such a spec at run time rather than no-oping.
//   - decline behavior: mount() returns nil when the user declines a risky
//     shortcut, whereas cmdcore.NewCommand returns a typed validation error.
//   - Required semantics: mount() requires the flag to be Changed (message
//     "缺少必填参数 --x"), while cmdcore.ValidateRequired accepts any effective
//     value including a registration Default or env fallback — so a Required
//     shortcut flag with a non-empty Default is always satisfied under cmdcore.
//   - typed defaults: mount() registers Bool/Int/StringSlice defaults parsed
//     from Flag.Default; cmdcore.RegisterFlag hardcodes false/0/nil and treats
//     Default only as a string fallback-chain tail, so bool/slice defaults are
//     lost and an int default would not surface in --help.
//   - Flag.Enum and Flag.Hidden: enum validation stays inside the Shortcut
//     framework, and CommandSpec has no hidden-command/flag field.
//   - Shortcut.Tips: mount() renders them as Example; Example is left empty here.
//   - the "custom" constraint kind (enforced by Shortcut.Validate), plus the
//     required-flag/enum runtime-schema annotations and hidden-flag filtering
//     that annotateRuntimeSchemaContract applies, have no cmdcore counterpart.
//
// Wiring FromShortcut into mount() (Phase 3) must be gated by byte-identical
// `dws schema --all` + `shortcut list` output, exactly as the leaf migration
// was gated by catalog zero-drift.
func FromShortcut(s Shortcut) cmdcore.CommandSpec {
	return cmdcore.CommandSpec{
		Use:   s.Command,
		Short: s.Description,
		// Only the prose part: cmdcore.NewCommand appends its own 参数约束
		// section, so reusing shortcutLongHelp here would render it twice.
		Long:        shortcutIntentProse(s),
		Flags:       fromShortcutFlags(s.Flags),
		Constraints: fromShortcutConstraints(s.Constraints),
		Risk:        cmdcore.Risk(s.risk()),
	}
}

// shortcutIntentProse returns just the intent/description prose that precedes
// the 参数约束 section in shortcutLongHelp, so the constraint section is rendered
// exactly once (by cmdcore.NewCommand).
func shortcutIntentProse(s Shortcut) string {
	prose := strings.TrimSpace(s.Intent)
	if prose == "" {
		prose = strings.TrimSpace(s.Description)
	}
	return prose
}

// fromShortcutFlags maps shortcut.Flag values into cmdcore.FlagSpec, carrying
// the shared-base fields. Usage keeps mount()'s flagHelp decoration (必填 /
// 可选值) so the projected help matches the live shortcut. Enum and Hidden have
// no cmdcore representation (see the FromShortcut doc block).
func fromShortcutFlags(flags []Flag) []cmdcore.FlagSpec {
	if len(flags) == 0 {
		return nil
	}
	out := make([]cmdcore.FlagSpec, 0, len(flags))
	for _, f := range flags {
		out = append(out, cmdcore.FlagSpec{
			Name:     f.Name,
			Usage:    flagHelp(f),
			Kind:     fromShortcutFlagKind(f.Type),
			Default:  f.Default,
			Required: f.Required,
		})
	}
	return out
}

// fromShortcutFlagKind maps the shortcut FlagType to the cmdcore FlagKind; an
// empty type defaults to string, matching the Shortcut framework.
func fromShortcutFlagKind(t FlagType) cmdcore.FlagKind {
	switch t {
	case FlagBool:
		return cmdcore.KindBool
	case FlagInt:
		return cmdcore.KindInt
	case FlagStringSlice:
		return cmdcore.KindStringSlice
	default:
		return cmdcore.KindString
	}
}

// fromShortcutConstraints maps the known relationship constraints into the
// cmdcore base. The shortcut-only "custom" kind (enforced by Shortcut.Validate)
// has no cmdcore equivalent and is dropped.
func fromShortcutConstraints(constraints []Constraint) []cmdcore.Constraint {
	if len(constraints) == 0 {
		return nil
	}
	out := make([]cmdcore.Constraint, 0, len(constraints))
	for _, c := range constraints {
		kind, ok := fromShortcutConstraintKind(c.Kind)
		if !ok {
			continue
		}
		out = append(out, cmdcore.Constraint{
			Kind:        kind,
			Flags:       append([]string(nil), c.Flags...),
			Description: c.Description,
		})
	}
	return out
}

// fromShortcutConstraintKind maps a shortcut ConstraintKind to its cmdcore
// equivalent; the second return is false for kinds cmdcore does not model
// (e.g. ConstraintCustom).
func fromShortcutConstraintKind(k ConstraintKind) (cmdcore.ConstraintKind, bool) {
	switch k {
	case ConstraintAtLeastOne:
		return cmdcore.AtLeastOne, true
	case ConstraintExactlyOne:
		return cmdcore.ExactlyOne, true
	case ConstraintMutuallyExclusive:
		return cmdcore.MutuallyExclusive, true
	default:
		return "", false
	}
}
