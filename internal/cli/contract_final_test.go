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
	"errors"
	"strings"
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

func TestCrossPlatformCoverageRuntimeToolSpecFromContractFinalPassThrough(t *testing.T) {
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
	spec, err := runtimeToolSpecFromContractFinal(entry, mustFinal(t, cmd), runtimeSchemaMetadataSources{})
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

func TestCrossPlatformCoverageRuntimeToolSpecFromContractFinalIdentityMismatchFails(t *testing.T) {
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
			_, err := runtimeToolSpecFromContractFinal(entry, ContractFinalPayload{Identity: &id}, runtimeSchemaMetadataSources{})
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
	if _, err := runtimeToolSpecFromContractFinal(entry, consistent, runtimeSchemaMetadataSources{}); err != nil {
		t.Fatalf("declared identity matching bound entry must pass: %v", err)
	}
}

func TestCrossPlatformCoverageRuntimeToolSpecFromContractFinalRejectsReviewedSelection(t *testing.T) {
	reviewed := true
	entry := runtimeSchemaEntry{
		ProductID: "dev",
		ToolName:  "create_thing",
		Command:   &cobra.Command{Use: "create"},
	}
	_, err := runtimeToolSpecFromContractFinal(entry, ContractFinalPayload{
		Selection: &SelectionSpec{AgentSummary: "sum", Reviewed: &reviewed},
	}, runtimeSchemaMetadataSources{})
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

func TestCrossPlatformCoverageContractFinalNilCommandGuards(t *testing.T) {
	// Nil command registration/lookup must be inert no-ops.
	RegisterRuntimeContractFinal(nil, ContractFinalPayload{Title: "ignored"})
	if _, ok := RuntimeContractFinal(nil); ok {
		t.Fatal("RuntimeContractFinal(nil) must report no payload")
	}
	if HasRuntimeContractFinal(nil) {
		t.Fatal("HasRuntimeContractFinal(nil) must be false")
	}
	ClearRuntimeContractFinalForTest(nil)
}

func TestCrossPlatformCoverageRuntimeContractFinalRejectsForeignStoredValue(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	t.Cleanup(func() { ClearRuntimeContractFinalForTest(cmd) })

	// Defensive branch: a stored value that is not a *ContractFinalPayload
	// (or a typed nil) must fail the read instead of panicking.
	contractFinalByCommand.Store(cmd, "not-a-payload")
	if _, ok := RuntimeContractFinal(cmd); ok {
		t.Fatal("foreign stored value must not decode as ContractFinalPayload")
	}
	contractFinalByCommand.Store(cmd, (*ContractFinalPayload)(nil))
	if _, ok := RuntimeContractFinal(cmd); ok {
		t.Fatal("typed nil payload must not decode as ContractFinalPayload")
	}
}

func TestCrossPlatformCoverageRuntimeToolSpecFromContractFinalIdentityOverridesApplied(t *testing.T) {
	cmd := &cobra.Command{Use: "create"}
	entry := runtimeSchemaEntry{
		ProductID:       "dev",
		SourceProductID: "src",
		ToolName:        "create_thing",
		CLIName:         "create",
		Group:           "dev thing",
		CLIPath:         "dev create",
		PrimaryCLIPath:  "dev create",
		Aliases:         []string{"dev alt"},
		Command:         cmd,
		Source:          "test",
	}
	final := ContractFinalPayload{Identity: &ToolIdentitySpec{
		ProductID:       "dev",
		SourceProductID: "src",
		Name:            "create_thing",
		CLIName:         "create",
		CanonicalPath:   "dev.create_thing",
		CLIPath:         "dev create",
		PrimaryCLIPath:  "dev create",
		Group:           "dev thing",
		Aliases:         []string{"dev alt"},
		Source:          "test",
	}}
	spec, err := runtimeToolSpecFromContractFinal(entry, final, runtimeSchemaMetadataSources{})
	if err != nil {
		t.Fatalf("consistent declared identity with all override fields failed: %v", err)
	}
	if spec.Identity.SourceProductID != "src" {
		t.Fatalf("source_product_id = %q, want src", spec.Identity.SourceProductID)
	}
	if spec.Identity.Group != "dev thing" {
		t.Fatalf("group = %q, want dev thing", spec.Identity.Group)
	}
	if len(spec.Identity.Aliases) != 1 || spec.Identity.Aliases[0] != "dev alt" {
		t.Fatalf("aliases = %v, want [dev alt]", spec.Identity.Aliases)
	}
	if spec.Identity.CanonicalPath != "dev.create_thing" || spec.Identity.Source != "test" {
		t.Fatalf("identity = %#v", spec.Identity)
	}
}

func TestCrossPlatformCoverageRuntimeToolSpecFromContractFinalSameLengthAliasMismatchFails(t *testing.T) {
	entry := runtimeSchemaEntry{
		ProductID:      "dev",
		ToolName:       "create_thing",
		CLIName:        "create",
		CLIPath:        "dev create",
		PrimaryCLIPath: "dev create",
		Aliases:        []string{"dev rm"},
		Command:        &cobra.Command{Use: "create"},
		Source:         "test",
	}
	// Same-length alias sets with different members must be detected by the
	// element-wise comparison, not only by the length fast-path.
	final := ContractFinalPayload{Identity: &ToolIdentitySpec{Aliases: []string{"dev other"}}}
	_, err := runtimeToolSpecFromContractFinal(entry, final, runtimeSchemaMetadataSources{})
	if err == nil || !strings.Contains(err.Error(), "aliases") {
		t.Fatalf("same-length alias mismatch error = %v, want aliases mismatch", err)
	}
}

func TestCrossPlatformCoverageRuntimeToolSpecFromContractFinalSafetyAnnotationFallbacks(t *testing.T) {
	entry := runtimeSchemaEntry{
		ProductID:      "dev",
		ToolName:       "create_thing",
		CLIName:        "create",
		CLIPath:        "dev create",
		PrimaryCLIPath: "dev create",
		Source:         "test",
	}

	// Safety nil + Contract Risk annotation: Risk overlay wins.
	riskCmd := &cobra.Command{Use: "create"}
	AnnotateRuntimeRisk(riskCmd, "write")
	entry.Command = riskCmd
	spec, err := runtimeToolSpecFromContractFinal(entry, ContractFinalPayload{}, runtimeSchemaMetadataSources{})
	if err != nil {
		t.Fatalf("risk-annotated declared leaf failed: %v", err)
	}
	if spec.Safety.Effect != "write" || spec.Safety.Risk != "medium" || spec.Safety.Confirmation != "user_required" {
		t.Fatalf("risk fallback safety = %#v", spec.Safety)
	}
	if spec.Safety.EffectSource != "corecmd.contract" {
		t.Fatalf("risk fallback effect source = %q", spec.Safety.EffectSource)
	}

	// Safety nil + runtime gate annotation: gate overlay wins.
	gateCmd := &cobra.Command{Use: "create"}
	AnnotateRuntimeGate(gateCmd, "devAppRequireWriteGuard")
	entry.Command = gateCmd
	spec, err = runtimeToolSpecFromContractFinal(entry, ContractFinalPayload{}, runtimeSchemaMetadataSources{})
	if err != nil {
		t.Fatalf("gate-annotated declared leaf failed: %v", err)
	}
	if spec.Safety.Confirmation != "user_required" || spec.Safety.Effect != "write" || spec.Safety.Risk != "medium" {
		t.Fatalf("gate fallback safety = %#v", spec.Safety)
	}
	if spec.Safety.EffectSource != "corecmd.contract_gate" {
		t.Fatalf("gate fallback effect source = %q", spec.Safety.EffectSource)
	}
}

func TestCrossPlatformCoverageRuntimeToolSpecFromContractFinalParameterResolutionError(t *testing.T) {
	oldParameters := resolveRuntimeParameters
	t.Cleanup(func() { resolveRuntimeParameters = oldParameters })
	resolveRuntimeParameters = func(*cobra.Command, string, map[string]ParameterSchemaHint, map[string]embeddedMCPParamMeta, RuntimeSchemaConstraints) ([]ParameterSpec, error) {
		return nil, errors.New("parameters failed")
	}
	entry := runtimeSchemaEntry{
		ProductID: "dev",
		ToolName:  "create_thing",
		Command:   &cobra.Command{Use: "create"},
	}
	_, err := runtimeToolSpecFromContractFinal(entry, ContractFinalPayload{}, runtimeSchemaMetadataSources{})
	if err == nil || !strings.Contains(err.Error(), "resolve Contract Schema parameters") {
		t.Fatalf("parameter resolution error = %v, want resolve Contract Schema parameters", err)
	}
}
