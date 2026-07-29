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
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cmdcore"
)

// adapter.go bridges the Shortcut framework toward the unified cmdcore typed
// spec. FromShortcut maps a Shortcut's SHARED BASE (flags, constraints, risk,
// help identity) into a cmdcore.CommandSpec so both frameworks can eventually
// share one flag + constraint + risk base.
//
// Scope (Phase 2): this is the typed seam only — it is NOT wired into the live
// mount() path. The Shortcut runtime keeps its own mount/RuntimeContext/Execute
// so the 376 shipped shortcuts are provably unaffected (catalog + `shortcut
// list` unchanged). The following Shortcut-specific semantics are intentionally
// NOT modeled by cmdcore yet and are therefore not carried here:
//
//   - multi-step orchestration: Shortcut.Execute(rt) uses RuntimeContext with
//     CallMCPData/CallMCPWriteData; cmdcore's single-step Dispatch(toolArgs)
//     cannot express it. FromShortcut leaves Dispatch/RunE nil.
//   - decline behavior: mount() returns nil when the user declines a risky
//     shortcut, whereas cmdcore.NewCommand returns a typed validation error.
//     Unifying this is a Phase 3 decision.
//   - Flag.Enum / Flag.Hidden and the "custom" constraint kind: shortcut-only
//     extras validated inside the Shortcut framework; cmdcore's base does not
//     model them, so they are dropped from the projection.
//
// Wiring FromShortcut into mount() (Phase 3) must be gated by byte-identical
// `dws schema --all` + `shortcut list` output, exactly as the leaf migration
// was gated by catalog zero-drift.
func FromShortcut(s Shortcut) cmdcore.CommandSpec {
	return cmdcore.CommandSpec{
		Use:         s.Command,
		Short:       s.Description,
		Long:        shortcutLongHelp(s),
		Flags:       fromShortcutFlags(s.Flags),
		Constraints: fromShortcutConstraints(s.Constraints),
		Risk:        cmdcore.Risk(s.risk()),
	}
}

// fromShortcutFlags maps shortcut.Flag values into cmdcore.FlagSpec, carrying
// only the shared-base fields (name, kind, default, usage, required). Enum and
// Hidden are shortcut-only extras and are not represented in the cmdcore base.
func fromShortcutFlags(flags []Flag) []cmdcore.FlagSpec {
	if len(flags) == 0 {
		return nil
	}
	out := make([]cmdcore.FlagSpec, 0, len(flags))
	for _, f := range flags {
		out = append(out, cmdcore.FlagSpec{
			Name:     f.Name,
			Usage:    f.Desc,
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
			Flags:       c.Flags,
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
