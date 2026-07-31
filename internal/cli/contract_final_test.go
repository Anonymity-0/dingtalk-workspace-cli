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

package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestContractFinalTypedRegistryNoJSON(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	t.Cleanup(func() { ClearRuntimeContractFinalForTest(cmd) })

	RegisterRuntimeContractFinal(cmd, ContractFinalPayload{
		Title: "T",
		Safety: &SafetySpec{
			Effect: "write", Confirmation: "user_required", Idempotency: "retryable",
		},
		Selection: &SelectionSpec{AgentSummary: "sum", UseWhen: []string{"u"}},
		Identity:  &ToolIdentitySpec{ProductID: "p", Name: "n"},
	})
	if cmd.Annotations != nil {
		if _, ok := cmd.Annotations["dws.schema.final"]; ok {
			t.Fatal("must not write JSON annotation dws.schema.final")
		}
	}
	got, ok := RuntimeContractFinal(cmd)
	if !ok || got.Title != "T" || got.Safety == nil || got.Safety.Idempotency != "retryable" {
		t.Fatalf("got %#v ok=%v", got, ok)
	}
	if got.Selection == nil || got.Selection.Reviewed != nil {
		t.Fatalf("selection must not carry reviewed fields: %#v", got.Selection)
	}
}

func TestRuntimeToolSpecFromContractFinalPassThrough(t *testing.T) {
	cmd := &cobra.Command{Use: "create", Short: "s", Long: "l"}
	t.Cleanup(func() { ClearRuntimeContractFinalForTest(cmd) })
	cmd.Flags().String("mode", "", "usage")
	AnnotateRuntimeFlag(cmd, "mode", "mode", "string", false, "")
	RegisterRuntimeContractFinal(cmd, ContractFinalPayload{
		Title: "Final Title",
		Safety: &SafetySpec{
			Effect: "write", Confirmation: "user_required", Idempotency: "none",
		},
		DryRun: &DryRunSpec{PreviewKind: DryRunPreviewInvocation},
		Selection: &SelectionSpec{
			AgentSummary: "from contract",
			UseWhen:      []string{"create things"},
		},
		Identity: &ToolIdentitySpec{
			ProductID: "dev", Name: "create_thing", CanonicalPath: "dev.create_thing",
		},
	})

	entry := runtimeSchemaEntry{
		ProductID:      "dev",
		ToolName:       "create_thing",
		CLIName:        "create",
		CLIPath:        "dev create",
		PrimaryCLIPath: "dev create",
		ProductName:    "Dev",
		Command:        cmd,
		Source:         "test",
	}
	spec, err := runtimeToolSpecFromContractFinal(entry, mustFinal(t, cmd))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Title != "Final Title" {
		t.Fatalf("title = %q", spec.Title)
	}
	if spec.Safety.Confirmation != "user_required" || spec.Safety.Idempotency != "none" {
		t.Fatalf("safety = %#v", spec.Safety)
	}
	if spec.DryRun == nil || spec.DryRun.PreviewKind != DryRunPreviewInvocation {
		t.Fatalf("dry_run = %#v", spec.DryRun)
	}
	if spec.Selection.AgentSummary != "from contract" {
		t.Fatalf("selection = %#v", spec.Selection)
	}
	if spec.MetadataSource != "corecmd.contract" {
		t.Fatalf("metadata_source = %q", spec.MetadataSource)
	}
	if len(spec.Parameters) != 1 || spec.Parameters[0].Name != "mode" {
		t.Fatalf("parameters = %#v", spec.Parameters)
	}
}

func TestRuntimeToolSpecFromContractFinalIdentityMismatchFails(t *testing.T) {
	entry := runtimeSchemaEntry{
		ProductID:      "dev",
		ToolName:       "create_thing",
		CLIName:        "create",
		CLIPath:        "dev create",
		PrimaryCLIPath: "dev create",
		Command:        &cobra.Command{Use: "create"},
		Source:         "test",
	}
	for name, id := range map[string]ToolIdentitySpec{
		"wrong name":           {Name: "delete_thing"},
		"wrong canonical path": {CanonicalPath: "dev.delete_thing"},
		"wrong product":        {ProductID: "other"},
		"wrong cli path":       {CLIPath: "dev delete"},
		"wrong aliases":        {Aliases: []string{"dev rm"}},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := runtimeToolSpecFromContractFinal(entry, ContractFinalPayload{Identity: &id})
			if err == nil {
				t.Fatal("declared identity conflicting with bound entry must fail assembly")
			}
		})
	}
	consistent := ContractFinalPayload{Identity: &ToolIdentitySpec{
		ProductID: "dev", Name: "create_thing", CLIName: "create",
		CanonicalPath: "dev.create_thing", CLIPath: "dev create",
		PrimaryCLIPath: "dev create", Source: "test",
	}}
	if _, err := runtimeToolSpecFromContractFinal(entry, consistent); err != nil {
		t.Fatalf("declared identity matching bound entry must pass: %v", err)
	}
}

func TestRuntimeToolSpecFromContractFinalRejectsReviewedSelection(t *testing.T) {
	reviewed := true
	entry := runtimeSchemaEntry{
		ProductID: "dev",
		ToolName:  "create_thing",
		Command:   &cobra.Command{Use: "create"},
	}
	_, err := runtimeToolSpecFromContractFinal(entry, ContractFinalPayload{
		Selection: &SelectionSpec{AgentSummary: "sum", Reviewed: &reviewed},
	})
	if err == nil {
		t.Fatal("declaration payload carrying Reviewed must fail assembly (reviewed is legacy-path only)")
	}
}

func mustFinal(t *testing.T, cmd *cobra.Command) ContractFinalPayload {
	t.Helper()
	final, ok := RuntimeContractFinal(cmd)
	if !ok {
		t.Fatal("missing final")
	}
	return final
}
