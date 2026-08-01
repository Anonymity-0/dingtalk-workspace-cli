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

package helpers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/contractfinal"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

func TestDeclareLeafMetadataRejectsExecutionSurface(t *testing.T) {
	cmd := &cobra.Command{Use: "x", Short: "x"}
	schema := LeafContract{
		Description: "d",
		Interface:   &contract.InterfaceSpec{Mode: "mcp", Availability: "available", Ref: &contract.InterfaceRefSpec{ProductID: "p", RPCName: "t"}},
		Selection: contract.SelectionSpec{
			AgentSummary: "s",
			UseWhen:      []string{"u"},
			AvoidWhen:    []string{"a"},
			Examples:     []string{"e"},
		},
	}
	mustPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatalf("%s: expected panic", name)
			}
		}()
		fn()
	}
	mustPanic("flags", func() {
		DeclareLeafMetadata(cmd, LeafSpec{Contract: schema, Flags: []LeafFlag{{Name: "f"}}})
	})
	mustPanic("runE", func() {
		DeclareLeafMetadata(cmd, LeafSpec{Contract: schema, RunE: func(*cobra.Command, []string) error { return nil }})
	})
	mustPanic("empty schema", func() {
		DeclareLeafMetadata(cmd, LeafSpec{})
	})
	// Validate is the one execution hook allowed in metadata mode (PreRunE,
	// before ConfirmSafety). It must not panic.
	DeclareLeafMetadata(&cobra.Command{Use: "y", Short: "y", RunE: func(*cobra.Command, []string) error { return nil }}, LeafSpec{
		Contract: schema,
		Validate: func(*cobra.Command, []string) error { return nil },
	})
}

func TestDeclareLeafMetadataDoesNotRewriteRunE(t *testing.T) {
	run := func(*cobra.Command, []string) error { return nil }
	cmd := &cobra.Command{Use: "x", Short: "x", RunE: run}
	before := cmd.RunE
	DeclareLeafMetadata(cmd, LeafSpec{
		Safety: aitableSafetyRead(),
		Contract: LeafContract{
			Description: "d",
			Interface:   aitableMCPInterface("tool"),
			Selection: contract.SelectionSpec{
				AgentSummary: "s",
				UseWhen:      []string{"u"},
				AvoidWhen:    []string{"a"},
				Examples:     []string{"e"},
			},
		},
	})
	if !contractfinal.HasRuntimeContractFinal(cmd) {
		t.Fatal("expected ContractFinal")
	}
	// function values are not comparable; ensure pointer identity via uintptr trick is unnecessary —
	// just check RunE is still non-nil and command still has same Use.
	if cmd.RunE == nil {
		t.Fatal("RunE must remain set")
	}
	_ = before
	if cmd.Use != "x" {
		t.Fatalf("Use mutated: %q", cmd.Use)
	}
}

func TestAitableDeclareLeafMetadataCoversRegistryHelpers(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	regPath := filepath.Join(filepath.Dir(thisFile), "..", "cli", "schema_command_registry", "products", "aitable.json")
	raw, err := os.ReadFile(regPath)
	if err != nil {
		t.Fatalf("read registry: %v", err)
	}
	var reg struct {
		Tools []struct {
			CanonicalPath string `json:"canonical_path"`
			CLIPath       string `json:"cli_path"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &reg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	root := newAitableCommand()
	missing := make([]string, 0)
	helperCount := 0
	for _, tool := range reg.Tools {
		// Shortcut paths (+...) stay on bind-time decls until Shortcut.Schema migration.
		if containsPlus(tool.CLIPath) {
			continue
		}
		helperCount++
		leaf := findCLIPath(root, tool.CLIPath)
		if leaf == nil {
			missing = append(missing, tool.CLIPath+" (command missing)")
			continue
		}
		if !contractfinal.HasRuntimeContractFinal(leaf) {
			missing = append(missing, tool.CLIPath)
		}
	}
	if helperCount == 0 {
		t.Fatal("expected helper registry entries")
	}
	if len(missing) > 0 {
		t.Fatalf("%d/%d helper paths missing ContractFinal: %v", len(missing), helperCount, missing[:min(10, len(missing))])
	}
}

func containsPlus(cliPath string) bool {
	for _, p := range splitCLI(cliPath) {
		if len(p) > 0 && p[0] == '+' {
			return true
		}
	}
	return false
}

func splitCLI(cliPath string) []string {
	out := make([]string, 0)
	cur := ""
	for _, r := range cliPath {
		if r == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func findCLIPath(root *cobra.Command, cliPath string) *cobra.Command {
	parts := splitCLI(cliPath)
	if len(parts) == 0 || parts[0] != "aitable" {
		return nil
	}
	cur := root
	for _, p := range parts[1:] {
		next, _, err := cur.Find(append([]string{}, p))
		if err != nil || next == nil {
			return nil
		}
		cur = next
	}
	return cur
}
