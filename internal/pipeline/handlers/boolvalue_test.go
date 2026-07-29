// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package handlers

import (
	"reflect"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/pipeline"
)

func TestBoolValueHandler(t *testing.T) {
	boolFlags := []pipeline.FlagInfo{{Name: "yes", Shorthand: "y", Type: "bool"}}
	tests := []struct {
		name       string
		args       []string
		flags      []pipeline.FlagInfo
		protected  []string
		want       []string
		correction bool
	}{
		{name: "long false", args: []string{"--yes", "false"}, flags: boolFlags, want: []string{"--yes=false"}, correction: true},
		{name: "long true synonym", args: []string{"--yes", "yes"}, flags: boolFlags, want: []string{"--yes=true"}, correction: true},
		{name: "shorthand zero", args: []string{"-y", "0"}, flags: boolFlags, want: []string{"--yes=false"}, correction: true},
		{name: "preserves following flags", args: []string{"--yes", "off", "--format", "json"}, flags: boolFlags, want: []string{"--yes=false", "--format", "json"}, correction: true},
		{name: "explicit equals stays native", args: []string{"--yes=false"}, flags: boolFlags, want: []string{"--yes=false"}},
		{name: "bare bool stays native", args: []string{"--yes"}, flags: boolFlags, want: []string{"--yes"}},
		{name: "invalid literal is positional", args: []string{"--yes", "maybe"}, flags: boolFlags, want: []string{"--yes", "maybe"}},
		{name: "non bool flag is unchanged", args: []string{"--name", "false"}, flags: []pipeline.FlagInfo{{Name: "name", Type: "string"}}, want: []string{"--name", "false"}},
		{name: "unknown flag is unchanged", args: []string{"--confirm", "false"}, flags: boolFlags, want: []string{"--confirm", "false"}},
		{name: "shorthand cluster is unchanged", args: []string{"-vy", "false"}, flags: boolFlags, want: []string{"-vy", "false"}},
		{name: "protected bool is unchanged", args: []string{"--yes", "false"}, flags: boolFlags, protected: []string{"yes"}, want: []string{"--yes", "false"}},
		{name: "stops at double dash", args: []string{"--", "--yes", "false"}, flags: boolFlags, want: []string{"--", "--yes", "false"}},
		{name: "no specs", args: []string{"--yes", "false"}, want: []string{"--yes", "false"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := &pipeline.Context{
				Args:      append([]string(nil), test.args...),
				FlagSpecs: test.flags,
			}
			for _, protected := range test.protected {
				ctx.ProtectFlag(protected, pipeline.FlagProtectionBlocked)
			}
			handler := BoolValueHandler{}
			if err := handler.Handle(ctx); err != nil {
				t.Fatalf("Handle() error = %v", err)
			}
			if !reflect.DeepEqual(ctx.Args, test.want) {
				t.Fatalf("Args = %#v, want %#v", ctx.Args, test.want)
			}
			wantCorrections := 0
			if test.correction {
				wantCorrections = 1
			}
			if len(ctx.Corrections) != wantCorrections {
				t.Fatalf("Corrections = %#v, want %d", ctx.Corrections, wantCorrections)
			}
			if test.correction {
				correction := ctx.Corrections[0]
				if correction.Handler != "boolvalue" || correction.Kind != "explicit-bool" || correction.Field != "--yes" {
					t.Fatalf("correction metadata = %#v", correction)
				}
			}
		})
	}
}

func TestBoolValueHandlerMeta(t *testing.T) {
	handler := BoolValueHandler{}
	if handler.Name() != "boolvalue" || handler.Phase() != pipeline.PreParse {
		t.Fatalf("handler metadata = %q/%v", handler.Name(), handler.Phase())
	}
}
