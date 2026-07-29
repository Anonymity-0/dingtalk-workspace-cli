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
	"testing"

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

	if cs.Use != "+demo" || cs.Short != "演示" || cs.Long == "" {
		t.Fatalf("identity = %q/%q/%q", cs.Use, cs.Short, cs.Long)
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
	if name.Name != "name" || name.Usage != "名称" || !name.Required || name.Default != "d" {
		t.Fatalf("name flag base fields = %#v", name)
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
