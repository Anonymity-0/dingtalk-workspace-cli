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
		Risk: RiskWrite,
		Schema: SchemaDecl{
			Title:       "Create Title",
			Description: "Create Desc",
			Positionals: []PositionalDecl{{Name: "id", Required: true, Index: 0}},
			Safety:      SafetyDecl{Idempotency: "retryable"},
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

func TestSchemaDeclEmptySkipsFinal(t *testing.T) {
	cmd := NewCommand(CommandSpec{
		Use:    "get",
		Short:  "g",
		Flags:  []FlagSpec{{Name: "id", Usage: "id"}},
		Risk:   RiskWrite, // Risk alone is annotation-only; Final needs Schema
		Invoke: func(*Ctx, map[string]any) error { return nil },
	})
	if cli.HasRuntimeContractFinal(cmd) {
		t.Fatal("Risk without Schema must not register Final (keep runtime write light)")
	}
	if _, ok := cli.RuntimeContractRisk(cmd); !ok {
		t.Fatal("Risk must still embed dws.schema.risk")
	}
}
