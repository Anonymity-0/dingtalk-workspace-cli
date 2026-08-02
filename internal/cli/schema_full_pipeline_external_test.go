// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli_test

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/app"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
)

// Exercise the complete production source-to-snapshot path from an external
// test package. Keeping this here (rather than in a generator package) means
// Go's normal per-package coverage accounting attributes the exercised Schema
// assembly code to internal/cli.
func TestCrossPlatformCoverageProductionSchemaSourcePipeline(t *testing.T) {
	root := app.NewSchemaSourceRootCommand()
	resolved, err := cli.ResolveSchemaBuild(root)
	if err != nil {
		t.Fatalf("ResolveSchemaBuild() error = %v", err)
	}
	if resolved.CommandCount() == 0 || resolved.RegistryHash() == "" {
		t.Fatalf("resolved build is empty: commands=%d hash=%q", resolved.CommandCount(), resolved.RegistryHash())
	}
	snapshot, err := cli.BuildSchemaCatalogSnapshot(resolved, cli.SchemaCatalogBuildOptions{
		RegistryHash: resolved.RegistryHash(),
	})
	if err != nil {
		t.Fatalf("BuildSchemaCatalogSnapshot() error = %v", err)
	}
	if len(snapshot.Tools) == 0 {
		t.Fatal("production Schema snapshot contains no tools")
	}
	registry, err := cli.AssembleSchemaRegistry(app.NewSchemaSourceRootCommand())
	if err != nil {
		t.Fatalf("AssembleSchemaRegistry() error = %v", err)
	}
	if len(registry.Products) == 0 {
		t.Fatal("assembled production Schema registry contains no products")
	}
	if err := cli.ValidateEmbeddedRuntimeSchemaCompleteness(app.NewSchemaSourceRootCommand()); err != nil {
		t.Fatalf("ValidateEmbeddedRuntimeSchemaCompleteness() error = %v", err)
	}
	root = app.NewSchemaSourceRootCommand()
	effective, err := cli.BuildEffectiveCommandRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	bound, err := cli.BindEffectiveCommandRegistry(root, effective)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cli.ValidateAgentSelectionContract(bound); err != nil {
		t.Fatalf("ValidateAgentSelectionContract() error = %v", err)
	}
	if _, _, err := cli.BuildAgentSelectionEvalFixture(bound); err != nil {
		t.Fatalf("BuildAgentSelectionEvalFixture() error = %v", err)
	}
	if _, err := cli.ValidateEmbeddedManualAgentExampleDelivery(bound, registry); err != nil {
		t.Fatalf("ValidateEmbeddedManualAgentExampleDelivery() error = %v", err)
	}
	if err := cli.ValidateSchemaParameterBindingDelivery(bound, registry); err != nil {
		t.Fatalf("ValidateSchemaParameterBindingDelivery() error = %v", err)
	}
	exclusions, err := cli.EmbeddedRuntimeSchemaExclusions()
	if err != nil {
		t.Fatal(err)
	}
	report := cli.RuntimeSchemaCompleteness(root, exclusions)
	if len(report.Missing)+len(report.InvalidExclusions)+len(report.StaleExclusions)+len(report.DeliveryErrors) != 0 {
		t.Fatalf("RuntimeSchemaCompleteness() = %#v", report)
	}
	if capabilities, err := cli.ReviewedDryRunCapabilities(); err != nil || len(capabilities) == 0 {
		t.Fatalf("ReviewedDryRunCapabilities() = %d, %v", len(capabilities), err)
	}
	// Phase 2: active binding tuples are empty; property delivery is ParamDecl.
	if bindings, err := cli.EmbeddedSchemaParameterBindings(); err != nil || len(bindings) != 0 {
		t.Fatalf("EmbeddedSchemaParameterBindings() = %d, %v; want empty active map after Phase 2", len(bindings), err)
	}
	if counts := cli.RuntimeSchemaMetadataLoadCounts(); counts.AgentMetadata == 0 || counts.MCPMetadata == 0 {
		t.Fatalf("RuntimeSchemaMetadataLoadCounts() = %#v", counts)
	}
}
