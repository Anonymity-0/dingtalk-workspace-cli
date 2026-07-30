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

package cmdcore

import (
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
)

func TestNewCommandEmbedsFullSchemaDeclAsFinalSource(t *testing.T) {
	cmd := NewCommand(CommandSpec{
		Use:   "create",
		Short: "short",
		Long:  "long",
		Flags: []FlagSpec{
			{
				Name: "mode", Usage: "mode usage", Bind: "mode",
				Enum: []string{"a", "b"}, Format: "token", Example: "a",
				RequiredWhen: "when x", SchemaDescription: "schema desc",
			},
		},
		Safety: cli.SafetySpec{
			Effect: "write", Risk: "medium",
			Confirmation: "user_required", Idempotency: "retryable",
		},
		Schema: SchemaDecl{
			Title:       "Create Title",
			Description: "Create Desc",
			Positionals: []PositionalDecl{{Name: "id", Required: true, Index: 0}},
			DryRun:      &DryRunDecl{PreviewKind: "invocation", RemoteReads: true},
			Interface: &InterfaceDecl{
				Mode: "mcp", Availability: "available",
				ProductID: "dev", RPCName: "create_thing",
			},
			Selection: SelectionDecl{
				AgentSummary: "summary",
				UseWhen:      []string{"when create"},
				AvoidWhen:    []string{"when read"},
				Examples:     []string{"dws create --mode a"},
			},
			Identity: IdentityDecl{
				ProductID: "dev", Name: "create_thing",
				CLIPath: "dev create", CanonicalPath: "dev.create_thing",
			},
		},
		Invoke: func(*Ctx, map[string]any) error { return nil },
	})

	if cmd.Annotations != nil {
		if _, ok := cmd.Annotations["dws.schema.final"]; ok {
			t.Fatal("framework must convert typed SchemaDecl; must not write JSON dws.schema.final")
		}
	}
	final, ok := cli.RuntimeContractFinal(cmd)
	if !ok {
		t.Fatal("expected typed ContractFinal registration")
	}
	if final.Title != "Create Title" || final.Description != "Create Desc" {
		t.Fatalf("title/desc = %q %q", final.Title, final.Description)
	}
	if final.Safety == nil || final.Safety.Confirmation != "user_required" || final.Safety.Idempotency != "retryable" {
		t.Fatalf("safety = %#v", final.Safety)
	}
	if final.DryRun == nil || final.DryRun.PreviewKind != "invocation" || !final.DryRun.RemoteReads {
		t.Fatalf("dry_run = %#v", final.DryRun)
	}
	if final.Interface == nil || final.Interface.Mode != "mcp" || final.Interface.Ref == nil || final.Interface.Ref.RPCName != "create_thing" {
		t.Fatalf("interface = %#v", final.Interface)
	}
	if final.Selection == nil || final.Selection.AgentSummary != "summary" || len(final.Selection.UseWhen) != 1 {
		t.Fatalf("selection = %#v", final.Selection)
	}
	if final.Identity == nil || final.Identity.ProductID != "dev" || final.Identity.Name != "create_thing" {
		t.Fatalf("identity = %#v", final.Identity)
	}
	if len(final.Positionals) != 1 || final.Positionals[0].Name != "id" {
		t.Fatalf("positionals = %#v", final.Positionals)
	}

	flag := cmd.Flags().Lookup("mode")
	if flag == nil {
		t.Fatal("missing mode flag")
	}
	if got := flag.Annotations["dws.schema.description"]; len(got) == 0 || got[0] != "schema desc" {
		t.Fatalf("description annotation = %#v", flag.Annotations["dws.schema.description"])
	}
	if got := flag.Annotations["dws.schema.required_when"]; len(got) == 0 || got[0] != "when x" {
		t.Fatalf("required_when = %#v", flag.Annotations["dws.schema.required_when"])
	}
	if got := flag.Annotations["x-cli-format"]; len(got) == 0 || got[0] != "token" {
		t.Fatalf("format = %#v", flag.Annotations["x-cli-format"])
	}
	if got := flag.Annotations["x-cli-enum"]; len(got) != 2 {
		t.Fatalf("enum = %#v", flag.Annotations["x-cli-enum"])
	}
}

func TestNewCommandPanicsOnPartialSchemaDecl(t *testing.T) {
	for _, tc := range []struct {
		name    string
		schema  SchemaDecl
		wantSub string
	}{
		{"missing description", SchemaDecl{
			Selection: SelectionDecl{AgentSummary: "s", UseWhen: []string{"u"}, AvoidWhen: []string{"a"}, Examples: []string{"dws x"}},
		}, "Schema.Description"},
		{"missing selection", SchemaDecl{
			Description: "d",
		}, "Schema.Selection.AgentSummary"},
		{"missing examples", SchemaDecl{
			Description: "d",
			Interface:   &InterfaceDecl{Mode: "mcp", Availability: "available", ProductID: "dev", RPCName: "get_thing"},
			Selection:   SelectionDecl{AgentSummary: "s", UseWhen: []string{"u"}, AvoidWhen: []string{"a"}},
		}, "Schema.Selection.Examples"},
		{"missing interface", SchemaDecl{
			Description: "d",
			Selection:   SelectionDecl{AgentSummary: "s", UseWhen: []string{"u"}, AvoidWhen: []string{"a"}, Examples: []string{"dws x"}},
		}, "Schema.Interface"},
		{"composite without reason", SchemaDecl{
			Description: "d",
			Interface:   &InterfaceDecl{Mode: "composite", Availability: "available"},
			Selection:   SelectionDecl{AgentSummary: "s", UseWhen: []string{"u"}, AvoidWhen: []string{"a"}, Examples: []string{"dws x"}},
		}, "Schema.Interface.Reason"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				recovered := recover()
				if recovered == nil {
					t.Fatal("partial SchemaDecl must panic at construction")
				}
				if msg, _ := recovered.(string); !strings.Contains(msg, tc.wantSub) {
					t.Fatalf("panic = %v, want mention %s", recovered, tc.wantSub)
				}
			}()
			NewCommand(CommandSpec{
				Use:    "x",
				Short:  "x",
				Schema: tc.schema,
				Invoke: func(*Ctx, map[string]any) error { return nil },
			})
		})
	}
}

func TestNewCommandDerivesHelpExampleFromDeclaredSelection(t *testing.T) {
	schema := SchemaDecl{
		Description: "desc",
		Interface:   &InterfaceDecl{Mode: "mcp", Availability: "available", ProductID: "dev", RPCName: "create_thing"},
		Selection: SelectionDecl{
			AgentSummary: "summary",
			UseWhen:      []string{"when"},
			AvoidWhen:    []string{"avoid"},
			Examples:     []string{"dws create --mode a", "dws create --mode b --dry-run"},
		},
	}
	cmd := NewCommand(CommandSpec{
		Use:    "create",
		Short:  "short",
		Safety: testWriteSafety(),
		Schema: schema,
		Invoke: func(*Ctx, map[string]any) error { return nil },
	})
	want := "  dws create --mode a\n  dws create --mode b --dry-run"
	if cmd.Example != want {
		t.Fatalf("derived Example = %q, want %q", cmd.Example, want)
	}

	schema.Selection.Examples = []string{"dws create --mode a"}
	explicit := NewCommand(CommandSpec{
		Use:     "create",
		Short:   "short",
		Example: "  dws create --custom",
		Safety:  testWriteSafety(),
		Schema:  schema,
		Invoke:  func(*Ctx, map[string]any) error { return nil },
	})
	if explicit.Example != "  dws create --custom" {
		t.Fatalf("authored Example must win over derivation, got %q", explicit.Example)
	}
}

func TestNewCommandSafetySpecPassThrough(t *testing.T) {
	schema := func() SchemaDecl {
		return SchemaDecl{
			Description: "desc",
			Interface:   &InterfaceDecl{Mode: "mcp", Availability: "available", ProductID: "dev", RPCName: "op"},
			Selection: SelectionDecl{
				AgentSummary: "s", UseWhen: []string{"u"}, AvoidWhen: []string{"a"}, Examples: []string{"dws x"},
			},
		}
	}
	build := func(spec CommandSpec) *cli.SafetySpec {
		cmd := NewCommand(spec)
		final, ok := cli.RuntimeContractFinal(cmd)
		if !ok || final.Safety == nil {
			t.Fatalf("expected declared safety, final=%#v ok=%v", final, ok)
		}
		return final.Safety
	}

	declared := cli.SafetySpec{
		Effect: "write", Risk: "low",
		Confirmation: "not_required", Idempotency: "non_idempotent",
	}
	if got := build(CommandSpec{Use: "w", Short: "w", Safety: declared, Schema: schema(),
		Invoke: func(*Ctx, map[string]any) error { return nil }}); got.Effect != declared.Effect ||
		got.Risk != declared.Risk || got.Confirmation != declared.Confirmation ||
		got.Idempotency != declared.Idempotency {
		t.Fatalf("SafetySpec must pass through without cross-field inference: %#v", got)
	}
	// A wholly empty declaration preserves the historical read-only default.
	if got := build(CommandSpec{Use: "r", Short: "r", Schema: schema(),
		Invoke: func(*Ctx, map[string]any) error { return nil }}); got.Effect != "read" || got.Risk != "low" ||
		got.Confirmation != "not_required" || got.Idempotency != "idempotent" {
		t.Fatalf("empty Safety must use read default, = %#v", got)
	}
}

func TestSchemaDeclEmptySkipsFinal(t *testing.T) {
	cmd := NewCommand(CommandSpec{
		Use:    "get",
		Short:  "g",
		Flags:  []FlagSpec{{Name: "id", Usage: "id"}},
		Safety: testWriteSafety(),
		Invoke: func(*Ctx, map[string]any) error { return nil },
	})
	if cli.HasRuntimeContractFinal(cmd) {
		t.Fatal("Safety without Schema must not register Final (keep runtime write light)")
	}
	if _, ok := cmd.Annotations["dws.schema.risk"]; ok {
		t.Fatal("Safety must not use the removed dws.schema.risk annotation")
	}
}

func TestNewCommandRejectsPartialSafetySpec(t *testing.T) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("partial SafetySpec must panic at construction")
		}
		if msg, _ := recovered.(string); !strings.Contains(msg, "Safety.Confirmation") {
			t.Fatalf("panic = %v, want missing Safety.Confirmation", recovered)
		}
	}()
	NewCommand(CommandSpec{
		Use:    "partial",
		Safety: cli.SafetySpec{Effect: "write", Risk: "medium", Idempotency: "unknown"},
		Invoke: func(*Ctx, map[string]any) error { return nil },
	})
}
