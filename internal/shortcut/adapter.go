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
	"fmt"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cmdcore"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/spf13/cobra"
)

// adapter.go is the compatibility boundary between Shortcut declarations and
// the unified cmdcore runtime. Shortcut keeps its RuntimeContext and Execute
// hooks because they own MCP orchestration, while cmdcore owns command/flag
// construction, declarative validation, Schema annotations and confirmation.
//
// Shortcut.Risk remains the legacy runtime confirmation source when Safety is
// empty. Explicit Safety overrides Risk expansion; Schema is pass-through into
// ContractFinal when authored.
func FromShortcut(s Shortcut) cmdcore.CommandSpec {
	safety := s.Safety
	if !safetySpecDeclared(safety) {
		safety = shortcutSafetySpec(s.risk())
	}
	return cmdcore.CommandSpec{
		Use:     s.Command,
		Short:   s.Description,
		Example: shortcutExamples(s.Tips),
		Hidden:  s.Hidden,
		// Only the prose part: cmdcore.NewCommand appends its own 参数约束
		// section, so the adapter must not pre-render it.
		Long:        shortcutIntentProse(s),
		Flags:       fromShortcutFlags(s.Flags),
		Constraints: fromShortcutConstraints(s.Constraints),
		Safety:      safety,
		Schema:      s.Schema,
		// Preserve the shipped Shortcut Catalog provenance: Cobra remains the
		// source for type/default/usage, while cmdcore adds Required/Enum/rules.
		ParameterProjection: cmdcore.ProjectCobraParameters,
		Validate:            fromShortcutValidate(s),
		// Multi-step body: cmdcore stays backend-agnostic, so the shortcut's own
		// RuntimeContext — which owns CallMCPData/CallMCPWriteData/Output — is
		// built here from the Ctx's command.
		Orchestrate: func(c *cmdcore.Ctx) error {
			if s.Execute == nil {
				return apperrors.NewInternal(fmt.Sprintf(
					"shortcut %s %s 未实现 Execute", s.Service, s.Command))
			}
			return s.Execute(&RuntimeContext{cmd: c.Command(), shortcut: s})
		},
	}
}

func safetySpecDeclared(safety cli.SafetySpec) bool {
	return strings.TrimSpace(safety.Effect) != "" ||
		strings.TrimSpace(safety.Risk) != "" ||
		strings.TrimSpace(safety.Confirmation) != "" ||
		strings.TrimSpace(safety.Idempotency) != ""
}

func shortcutExamples(tips []string) string {
	if len(tips) == 0 {
		return ""
	}
	return "  " + strings.Join(tips, "\n  ")
}

func fromShortcutValidate(s Shortcut) func(*cobra.Command, []string) error {
	if s.Validate == nil {
		return nil
	}
	return func(cmd *cobra.Command, _ []string) error {
		return s.Validate(&RuntimeContext{cmd: cmd, shortcut: s})
	}
}

// shortcutSafetySpec is the temporary compatibility boundary while the live
// Shortcut framework still owns its legacy Risk enum. cmdcore and Leaf do not
// retain that enum: the adapter expands it once into the existing Schema model.
func shortcutSafetySpec(risk Risk) cli.SafetySpec {
	switch risk {
	case RiskWrite:
		return cli.SafetySpec{
			Effect: "write", Risk: "medium",
			Confirmation: "user_required", Idempotency: "unknown",
		}
	case RiskHighWrite:
		return cli.SafetySpec{
			Effect: "destructive", Risk: "high",
			Confirmation: "user_required", Idempotency: "unknown",
		}
	default:
		return cli.SafetySpec{
			Effect: "read", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		}
	}
}

// shortcutIntentProse returns just the intent/description prose so the
// constraint section is rendered exactly once by cmdcore.NewCommand.
func shortcutIntentProse(s Shortcut) string {
	prose := strings.TrimSpace(s.Intent)
	if prose == "" {
		prose = strings.TrimSpace(s.Description)
	}
	return prose
}

// fromShortcutFlags maps every Shortcut flag fact into cmdcore.
// ValidationShortcut preserves declaration-order Required/Enum checks, the
// historical "the token itself must be present" contract and its exact
// missing-flag message even when a registration default exists.
func fromShortcutFlags(flags []Flag) []cmdcore.FlagSpec {
	if len(flags) == 0 {
		return nil
	}
	out := make([]cmdcore.FlagSpec, 0, len(flags))
	for _, f := range flags {
		out = append(out, cmdcore.FlagSpec{
			Name:           f.Name,
			Usage:          flagHelp(f),
			Kind:           fromShortcutFlagKind(f.Type),
			Default:        f.Default,
			Hidden:         f.Hidden,
			Required:       f.Required,
			ValidationMode: cmdcore.ValidationShortcut,
			RequiredError:  fmt.Sprintf("缺少必填参数 --%s：%s", f.Name, f.Desc),
			Enum:           append([]string(nil), f.Enum...),
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

// fromShortcutConstraints maps both generic relationships and custom
// declaration/help facts. Custom runtime checks remain in Shortcut.Validate.
func fromShortcutConstraints(constraints []Constraint) []cmdcore.Constraint {
	if len(constraints) == 0 {
		return nil
	}
	out := make([]cmdcore.Constraint, 0, len(constraints))
	for _, c := range constraints {
		kind, ok := fromShortcutConstraintKind(c.Kind)
		if !ok {
			panic(fmt.Sprintf("unknown shortcut constraint kind %q", c.Kind))
		}
		out = append(out, cmdcore.Constraint{
			Kind:        kind,
			Flags:       append([]string(nil), c.Flags...),
			Description: c.Description,
		})
	}
	return out
}

// fromShortcutConstraintKind maps a shortcut ConstraintKind to cmdcore.
func fromShortcutConstraintKind(k ConstraintKind) (cmdcore.ConstraintKind, bool) {
	switch k {
	case ConstraintAtLeastOne:
		return cmdcore.AtLeastOne, true
	case ConstraintExactlyOne:
		return cmdcore.ExactlyOne, true
	case ConstraintMutuallyExclusive:
		return cmdcore.MutuallyExclusive, true
	case ConstraintCustom:
		return cmdcore.Custom, true
	default:
		return "", false
	}
}
