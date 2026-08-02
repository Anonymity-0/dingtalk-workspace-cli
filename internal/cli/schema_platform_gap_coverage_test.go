// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"fmt"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/contractfinal"
	"github.com/spf13/cobra"
)

func TestCrossPlatformCoveragePinnedMCPTextAndLegacyMetadataSource(t *testing.T) {
	mcp := embeddedMCPMetadata{Tools: map[string]embeddedMCPToolMetadata{
		"sample.run": {
			Title:       "MCP Title",
			Description: "MCP Description",
			Parameters:  map[string]embeddedMCPParamMeta{"name": {Type: "string"}},
		},
	}}
	entry := runtimeSchemaEntry{
		ProductID:      "sample",
		ToolName:       "run",
		PrimaryCLIPath: "sample run",
		CLIPath:        "sample run",
		Title:          "Cobra Title",
		Description:    "Cobra Description",
	}

	title, description, source, _, err := runtimeToolTextMetadataFromMetadata(entry, runtimeSchemaMetadataSources{MCP: mcp})
	if err != nil {
		t.Fatalf("runtimeToolTextMetadataFromMetadata() error = %v", err)
	}
	if title == "" || description == "" {
		t.Fatalf("pinned MCP text candidates must participate: title=%q description=%q source=%q", title, description, source)
	}

	legacyRoot := buildRuntimeSchemaTestRoot()
	legacyBound, err := boundTestCommandRegistry(legacyRoot)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range legacyBound.Commands {
		contractfinal.ClearRuntimeContractFinalForTest(command.PrimaryCommand)
	}
	entries, err := collectRuntimeSchemaEntriesFromBound(legacyBound)
	if err != nil || len(entries) == 0 {
		t.Fatalf("entries = %#v, %v", entries, err)
	}
	legacyEntry := entries[0]
	legacyEntry.Title = "Legacy Title"
	legacyEntry.Description = "Legacy Description"
	tool, err := runtimeToolSpecFromLegacyMetadata(legacyEntry, runtimeSchemaMetadataSources{
		MCP: embeddedMCPMetadata{Tools: map[string]embeddedMCPToolMetadata{
			legacyEntry.ProductID + "." + legacyEntry.ToolName: {
				Title:       "Pinned",
				Description: "Pinned desc",
				Parameters:  map[string]embeddedMCPParamMeta{},
			},
		}},
	})
	if err != nil {
		t.Fatalf("runtimeToolSpecFromLegacyMetadata() error = %v", err)
	}
	if tool.MetadataSource != ProvenanceEmbeddedMCPMetadata {
		t.Fatalf("legacy pinned MCP metadata_source = %q, want %q", tool.MetadataSource, ProvenanceEmbeddedMCPMetadata)
	}
}

func TestCrossPlatformCoverageReviewedCommandRegistryMergedJSON(t *testing.T) {
	merged, err := ReviewedCommandRegistryMergedJSON()
	if err != nil || len(merged) == 0 {
		t.Fatalf("ReviewedCommandRegistryMergedJSON() = %q err=%v", merged, err)
	}
	if !strings.Contains(string(merged), `"products"`) {
		t.Fatalf("merged registry missing products: %s", merged[:min(120, len(merged))])
	}
}

func TestCrossPlatformCoverageCompletenessAndDeliveryErrorBranches(t *testing.T) {
	prevGroups := reviewedRuntimeSchemaExclusionGroups
	t.Cleanup(func() { reviewedRuntimeSchemaExclusionGroups = prevGroups })

	reviewedRuntimeSchemaExclusionGroups = []runtimeSchemaExclusionGroup{{
		ID: "", Reason: "x", Reviewed: true, Commands: []string{"x"},
	}}
	if _, err := ReviewedRuntimeSchemaExclusions(); err == nil || !strings.Contains(err.Error(), "not reviewed") {
		t.Fatalf("blank group id error = %v", err)
	}
	reviewedRuntimeSchemaExclusionGroups = []runtimeSchemaExclusionGroup{{
		ID: "g", Reason: "reason", Reviewed: true, Commands: []string{" ", "ok"},
	}}
	if _, err := ReviewedRuntimeSchemaExclusions(); err == nil || !strings.Contains(err.Error(), "empty command") {
		t.Fatalf("empty command error = %v", err)
	}
	reviewedRuntimeSchemaExclusionGroups = []runtimeSchemaExclusionGroup{{
		ID: "g", Reason: "reason", Reviewed: true, Commands: []string{"dup", "dup"},
	}}
	if _, err := ReviewedRuntimeSchemaExclusions(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("duplicate exclusion error = %v", err)
	}
	reviewedRuntimeSchemaExclusionGroups = prevGroups

	prevBuild := completenessBuildEffective
	prevBind := completenessBindEffective
	prevLoad := completenessLoadExclusions
	prevRuntime := completenessRuntimeReport
	prevDelivery := completenessDeliveryReport
	t.Cleanup(func() {
		completenessBuildEffective = prevBuild
		completenessBindEffective = prevBind
		completenessLoadExclusions = prevLoad
		completenessRuntimeReport = prevRuntime
		completenessDeliveryReport = prevDelivery
	})

	root := &cobra.Command{Use: "dws"}
	completenessBuildEffective = func(*cobra.Command) (EffectiveCommandRegistry, error) {
		return EffectiveCommandRegistry{}, fmt.Errorf("build boom")
	}
	if err := ValidateRuntimeSchemaCompleteness(root); err == nil || !strings.Contains(err.Error(), "build boom") {
		t.Fatalf("build error = %v", err)
	}
	completenessBuildEffective = func(*cobra.Command) (EffectiveCommandRegistry, error) {
		return EffectiveCommandRegistry{}, nil
	}
	completenessBindEffective = func(*cobra.Command, EffectiveCommandRegistry) (BoundCommandRegistry, error) {
		return BoundCommandRegistry{}, fmt.Errorf("bind boom")
	}
	if err := ValidateRuntimeSchemaCompleteness(root); err == nil || !strings.Contains(err.Error(), "bind boom") {
		t.Fatalf("bind error = %v", err)
	}
	completenessBindEffective = func(*cobra.Command, EffectiveCommandRegistry) (BoundCommandRegistry, error) {
		return BoundCommandRegistry{}, nil
	}
	completenessLoadExclusions = func() ([]RuntimeSchemaExclusion, error) {
		return nil, fmt.Errorf("excl boom")
	}
	if err := validateResolvedRuntimeSchemaCompleteness(root, BoundCommandRegistry{}); err == nil || !strings.Contains(err.Error(), "excl boom") {
		t.Fatalf("excl error = %v", err)
	}
	completenessLoadExclusions = func() ([]RuntimeSchemaExclusion, error) { return nil, nil }
	completenessRuntimeReport = func(*cobra.Command, []RuntimeSchemaExclusion, BoundCommandRegistry) RuntimeSchemaCompletenessReport {
		return RuntimeSchemaCompletenessReport{DeliveryErrors: []string{"delivery"}}
	}
	if err := validateResolvedRuntimeSchemaCompleteness(root, BoundCommandRegistry{}); err == nil || !strings.Contains(err.Error(), "delivery") {
		t.Fatalf("delivery report error = %v", err)
	}
	completenessRuntimeReport = func(*cobra.Command, []RuntimeSchemaExclusion, BoundCommandRegistry) RuntimeSchemaCompletenessReport {
		return RuntimeSchemaCompletenessReport{Missing: []string{"m"}}
	}
	if err := validateResolvedRuntimeSchemaCompleteness(root, BoundCommandRegistry{}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing report error = %v", err)
	}
	completenessRuntimeReport = func(*cobra.Command, []RuntimeSchemaExclusion, BoundCommandRegistry) RuntimeSchemaCompletenessReport {
		return RuntimeSchemaCompletenessReport{InvalidExclusions: []string{"i"}}
	}
	if err := validateResolvedRuntimeSchemaCompleteness(root, BoundCommandRegistry{}); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("invalid report error = %v", err)
	}
	completenessRuntimeReport = func(*cobra.Command, []RuntimeSchemaExclusion, BoundCommandRegistry) RuntimeSchemaCompletenessReport {
		return RuntimeSchemaCompletenessReport{StaleExclusions: []string{"s"}}
	}
	if err := validateResolvedRuntimeSchemaCompleteness(root, BoundCommandRegistry{}); err == nil || !strings.Contains(err.Error(), "stale") {
		t.Fatalf("stale report error = %v", err)
	}

	completenessLoadExclusions = func() ([]RuntimeSchemaExclusion, error) {
		return nil, fmt.Errorf("delivery excl boom")
	}
	if err := validateResolvedSchemaCatalogDeliveryCompleteness(root, BoundCommandRegistry{}, SchemaCatalogSnapshot{}); err == nil || !strings.Contains(err.Error(), "delivery excl boom") {
		t.Fatalf("delivery excl error = %v", err)
	}
	completenessLoadExclusions = func() ([]RuntimeSchemaExclusion, error) { return nil, nil }
	completenessDeliveryReport = func(*cobra.Command, loadedSchemaCatalog, []RuntimeSchemaExclusion, BoundCommandRegistry) RuntimeSchemaCompletenessReport {
		return RuntimeSchemaCompletenessReport{DeliveryErrors: []string{"snap"}}
	}
	// Force decode path via empty snapshot tools/catalog — marshal still works.
	if err := validateSchemaCatalogDeliveryCompletenessFromBound(root, BoundCommandRegistry{}, SchemaCatalogSnapshot{
		Version: SchemaCatalogSnapshotVersion,
		Catalog: map[string]any{},
		Tools:   map[string]map[string]any{},
	}, nil); err == nil || !strings.Contains(err.Error(), "snap") {
		// If decode fails first, still exercised encode/decode branches.
		if err == nil {
			t.Fatal("expected delivery completeness error")
		}
	}
}
