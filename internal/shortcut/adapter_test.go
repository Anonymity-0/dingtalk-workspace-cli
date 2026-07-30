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
	"testing"

	"github.com/spf13/pflag"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cmdcore"
)

// TestCrossPlatformCoverageFromShortcutMapsSharedBase verifies FromShortcut
// projects a Shortcut's shared-base fields (identity, flags of every kind,
// known constraints, risk) into a cmdcore.CommandSpec, and that shortcut-only
// extras (Enum/Hidden, the custom constraint) plus multi-step dispatch are not
// modeled (Dispatch/RunE stay nil).
func TestCrossPlatformCoverageFromShortcutMapsSharedBase(t *testing.T) {
	s := Shortcut{
		Service:     "chat",
		Command:     "+demo",
		Description: "演示",
		Risk:        RiskHighWrite,
		Flags: []Flag{
			{Name: "name", Type: FlagString, Desc: "名称", Required: true, Default: "d", Enum: []string{"a", "b"}, Hidden: true},
			{Name: "count", Type: FlagInt, Desc: "数量"},
			{Name: "flag", Type: FlagBool, Desc: "开关"},
			{Name: "ids", Type: FlagStringSlice, Desc: "列表"},
			{Name: "note", Desc: "空类型默认 string"},
		},
		Constraints: []Constraint{
			{Kind: ConstraintExactlyOne, Flags: []string{"name", "count"}, Description: "二选一"},
			{Kind: ConstraintAtLeastOne, Flags: []string{"flag", "ids"}},
			{Kind: ConstraintMutuallyExclusive, Flags: []string{"name", "flag"}},
			{Kind: ConstraintCustom, Flags: []string{"note"}, Description: "自定义由 Validate 保证"},
		},
	}

	cs := FromShortcut(s)

	if cs.Use != "+demo" || cs.Short != "演示" {
		t.Fatalf("identity = %q/%q", cs.Use, cs.Short)
	}
	// Long carries ONLY the intent prose: cmdcore.NewCommand renders the
	// 参数约束 section, so it must not already be present here.
	if strings.Contains(cs.Long, "参数约束") {
		t.Fatalf("Long must not pre-render 参数约束: %q", cs.Long)
	}
	if cs.Risk != cmdcore.RiskHighWrite {
		t.Fatalf("risk = %q, want high-risk-write", cs.Risk)
	}
	if cs.Dispatch != nil || cs.RunE != nil {
		t.Fatal("dispatch/RunE must stay nil (multi-step Execute not modeled)")
	}

	if len(cs.Flags) != 5 {
		t.Fatalf("flags len = %d, want 5", len(cs.Flags))
	}
	wantKinds := []cmdcore.FlagKind{cmdcore.KindString, cmdcore.KindInt, cmdcore.KindBool, cmdcore.KindStringSlice, cmdcore.KindString}
	for i, want := range wantKinds {
		if cs.Flags[i].Kind != want {
			t.Fatalf("flag[%d].Kind = %v, want %v", i, cs.Flags[i].Kind, want)
		}
	}
	name := cs.Flags[0]
	if name.Name != "name" || !name.Required || name.Default != "d" {
		t.Fatalf("name flag base fields = %#v", name)
	}
	// Usage keeps mount()'s flagHelp decoration (必填 / 可选值).
	if name.Usage != flagHelp(s.Flags[0]) {
		t.Fatalf("name usage = %q, want flagHelp %q", name.Usage, flagHelp(s.Flags[0]))
	}
	if !strings.Contains(name.Usage, "可选值") {
		t.Fatalf("enum decoration lost from usage: %q", name.Usage)
	}

	// custom constraint dropped; the other three carried in order.
	if len(cs.Constraints) != 3 {
		t.Fatalf("constraints len = %d, want 3 (custom dropped)", len(cs.Constraints))
	}
	if cs.Constraints[0].Kind != cmdcore.ExactlyOne || cs.Constraints[0].Description != "二选一" {
		t.Fatalf("constraint[0] = %#v", cs.Constraints[0])
	}
	if cs.Constraints[1].Kind != cmdcore.AtLeastOne || cs.Constraints[2].Kind != cmdcore.MutuallyExclusive {
		t.Fatalf("constraint kinds = %#v", cs.Constraints)
	}
	// The projected Flags slice must be a copy, not an alias of the registry's.
	cs.Constraints[0].Flags[0] = "mutated"
	if s.Constraints[0].Flags[0] != "name" {
		t.Fatal("constraint Flags slice is aliased, not copied")
	}
}

// TestCrossPlatformCoverageFromShortcutMatchesMountSurface pins the projection
// against the live mount() surface: flag set (names/types/usage) and rendered
// Long must agree. This is the class of test that catches a double-rendered
// 参数约束 or a lost flagHelp decoration before Phase 3 wires the adapter in.
func TestCrossPlatformCoverageFromShortcutMatchesMountSurface(t *testing.T) {
	s := Shortcut{
		Service:     "chat",
		Command:     "+surface",
		Description: "表层对比",
		Intent:      "用于校验投影与 mount 的一致性",
		Risk:        RiskWrite,
		Flags: []Flag{
			{Name: "group", Type: FlagString, Desc: "群 ID", Required: true},
			{Name: "mode", Type: FlagString, Desc: "模式", Enum: []string{"a", "b"}},
			{Name: "count", Type: FlagInt, Desc: "数量"},
		},
		Constraints: []Constraint{{Kind: ConstraintAtLeastOne, Flags: []string{"group", "mode"}}},
		Execute:     func(*RuntimeContext) error { return nil },
	}

	mounted := mount(s)
	projected := cmdcore.NewCommand(FromShortcut(s))

	mounted.Flags().VisitAll(func(want *pflag.Flag) {
		got := projected.Flags().Lookup(want.Name)
		if got == nil {
			t.Fatalf("projected command missing flag --%s", want.Name)
		}
		if got.Value.Type() != want.Value.Type() {
			t.Fatalf("flag --%s type = %s, want %s", want.Name, got.Value.Type(), want.Value.Type())
		}
		if got.Usage != want.Usage {
			t.Fatalf("flag --%s usage = %q, want %q", want.Name, got.Usage, want.Usage)
		}
	})
	projected.Flags().VisitAll(func(extra *pflag.Flag) {
		if mounted.Flags().Lookup(extra.Name) == nil {
			t.Fatalf("projected command has extra flag --%s", extra.Name)
		}
	})

	if projected.Long != mounted.Long {
		t.Fatalf("Long mismatch:\nprojected=%q\nmounted  =%q", projected.Long, mounted.Long)
	}
	if n := strings.Count(projected.Long, "参数约束"); n != 1 {
		t.Fatalf("参数约束 rendered %d times: %q", n, projected.Long)
	}
}

// TestCrossPlatformCoverageFromShortcutEmpty covers the empty flag/constraint
// short-circuits and the read-risk default.
func TestCrossPlatformCoverageFromShortcutEmpty(t *testing.T) {
	cs := FromShortcut(Shortcut{Service: "x", Command: "+bare", Description: "bare"})
	if cs.Flags != nil {
		t.Fatalf("empty flags = %#v, want nil", cs.Flags)
	}
	if cs.Constraints != nil {
		t.Fatalf("empty constraints = %#v, want nil", cs.Constraints)
	}
	if cs.Risk != cmdcore.RiskRead {
		t.Fatalf("default risk = %q, want read", cs.Risk)
	}
}
