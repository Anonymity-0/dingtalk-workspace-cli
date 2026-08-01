// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/spf13/cobra"
)

func TestValidateSchemaRegistryAgainstCommandRegistryChecksFullIdentity(t *testing.T) {
	tool := ToolSpec{Identity: contract.ToolIdentitySpec{
		ProductID:       "sample",
		SourceProductID: "implementation_a",
		Name:            "run",
		CanonicalPath:   "sample.run",
		Path:            "sample.run",
		CLIPath:         "sample run",
		PrimaryCLIPath:  "sample run",
		Source:          "reviewed_command_registry",
	}}
	registry, err := SchemaRegistryFromRuntime("test", []ProductSpec{{ID: "sample", Tools: []ToolSpec{tool}}})
	if err != nil {
		t.Fatal(err)
	}

	base := CommandSpec{
		CanonicalPath:   "sample.run",
		SourceProductID: "implementation_a",
		PrimaryCLIPath:  "sample run",
		Visibility:      SchemaVisibilityPublic,
		Source:          "reviewed_command_registry",
	}
	for name, test := range map[string]struct {
		mutate func(*CommandSpec)
		want   string
	}{
		"source product": {
			mutate: func(spec *CommandSpec) { spec.SourceProductID = "implementation_b" },
			want:   "source product",
		},
		"identity source": {
			mutate: func(spec *CommandSpec) { spec.Source = "stale_identity_source" },
			want:   "identity source",
		},
	} {
		t.Run(name, func(t *testing.T) {
			expected := cloneCommandSpec(base)
			test.mutate(&expected)
			effective, err := newEffectiveCommandRegistry([]CommandSpec{expected})
			if err != nil {
				t.Fatal(err)
			}
			err = validateSchemaRegistryAgainstCommandRegistry(registry, effective)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validation error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateSchemaRegistryAgainstCommandRegistryRejectsAliasViewAsCanonical(t *testing.T) {
	baseTool := ToolSpec{Identity: contract.ToolIdentitySpec{
		ProductID:      "sample",
		Name:           "run",
		CanonicalPath:  "sample.run",
		Path:           "sample.run",
		CLIPath:        "sample run",
		PrimaryCLIPath: "sample run",
		Aliases:        []string{"sample execute"},
		Source:         "reviewed_command_registry",
	}}
	effective, err := newEffectiveCommandRegistry([]CommandSpec{{
		CanonicalPath:  "sample.run",
		PrimaryCLIPath: "sample run",
		Aliases:        []string{"sample execute"},
		Visibility:     SchemaVisibilityPublic,
		Source:         "reviewed_command_registry",
	}})
	if err != nil {
		t.Fatal(err)
	}

	for name, test := range map[string]struct {
		mutate func(*contract.ToolIdentitySpec)
		want   string
	}{
		"alternate cli path": {
			mutate: func(identity *contract.ToolIdentitySpec) { identity.CLIPath = "sample execute" },
			want:   "must equal primary_cli_path",
		},
		"alias marker": {
			mutate: func(identity *contract.ToolIdentitySpec) { identity.IsAlias = true },
			want:   "must have is_alias=false",
		},
	} {
		t.Run(name, func(t *testing.T) {
			tool := baseTool
			test.mutate(&tool.Identity)
			registry := SchemaRegistry{Products: []ProductSpec{{ID: "sample", Tools: []ToolSpec{tool}}}}
			err := validateSchemaRegistryAgainstCommandRegistry(registry, effective)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validation error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestAssembleSchemaRegistryFailClosedMissingContractFinal(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	leaf := &cobra.Command{Use: "run", Short: "Run", Run: func(*cobra.Command, []string) {}}
	AttachRuntimeSchema(leaf, "sample", "run", "test")
	product := &cobra.Command{Use: "sample"}
	product.AddCommand(leaf)
	root.AddCommand(product)

	t.Cleanup(func() { contract.ClearProductDeclForTest("sample") })
	contract.RegisterProductDecl(contract.ProductDecl{
		ID: "sample",
		Selection: contract.ProductSelectionDecl{
			AgentSummary: "Sample product",
			UseWhen:      []string{"sample routing"},
			AvoidWhen:    []string{"not sample"},
		},
	})

	_, err := schemaRegistryForTest(root)
	if err == nil || !strings.Contains(err.Error(), "missing RuntimeContractFinal") {
		t.Fatalf("production assembly error = %v, want missing RuntimeContractFinal", err)
	}
}

func TestAssembleSchemaRegistryFailClosedMissingProductDecl(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	leaf := &cobra.Command{Use: "run", Short: "Run", Run: func(*cobra.Command, []string) {}}
	AttachRuntimeSchema(leaf, "orphan", "run", "test")
	RegisterRuntimeContractFinal(leaf, contract.ContractFinalPayload{
		Title:       "Orphan run",
		Description: "Has ContractFinal but no ProductDecl",
		Selection:   &contract.SelectionSpec{AgentSummary: "orphan leaf"},
	})
	t.Cleanup(func() {
		contract.ClearRuntimeContractFinalForTest(leaf)
		contract.ClearProductDeclForTest("orphan")
	})
	product := &cobra.Command{Use: "orphan"}
	product.AddCommand(leaf)
	root.AddCommand(product)

	_, err := schemaRegistryForTest(root)
	if err == nil || !strings.Contains(err.Error(), "missing ProductDecl") {
		t.Fatalf("production assembly error = %v, want missing ProductDecl", err)
	}
}

func TestAssembleSchemaRegistryAllowingLegacyIsolatesOverlayPath(t *testing.T) {
	root := buildRuntimeSchemaTestRoot()
	bound, err := boundTestCommandRegistry(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range bound.Commands {
		contract.ClearRuntimeContractFinalForTest(command.PrimaryCommand)
	}
	agent := embeddedAgentMetadata{
		Version: 1,
		Products: map[string]agentProductMetadata{
			"doc": {AgentSummary: "docs", UseWhen: []string{"read docs"}},
		},
		Tools: map[string]agentToolMetadata{
			"doc create": {Effect: "write", InterfaceMode: "local", Availability: "available", InterfaceReason: "test"},
		},
	}
	registry, err := assembleSchemaRegistryFromBoundAllowingLegacy(bound, runtimeSchemaMetadataSources{Agent: agent})
	if err != nil {
		t.Fatalf("legacy-isolated assembly: %v", err)
	}
	if len(registry.Products) != 1 || registry.Products[0].ID != "doc" {
		t.Fatalf("legacy-isolated products = %#v", registry.Products)
	}
}

func TestAssembleSchemaRegistryRequiresContractFinalAndProductDecl(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	leaf := &cobra.Command{Use: "run", Short: "Run sample", Long: "Run the sample tool", Run: func(*cobra.Command, []string) {}}
	AttachRuntimeSchema(leaf, "sample", "run", "test")
	RegisterRuntimeContractFinal(leaf, contract.ContractFinalPayload{
		Title:       "Sample run",
		Description: "Declared sample tool",
		Safety: &contract.SafetySpec{
			Effect: "read", Risk: "low", Confirmation: "none", Idempotency: "idempotent",
		},
		Interface: &contract.InterfaceSpec{
			Mode: "local", Availability: "available", Reason: "test local leaf",
		},
		Selection: &contract.SelectionSpec{
			AgentSummary: "Run a sample tool",
			UseWhen:      []string{"need sample run"},
			AvoidWhen:    []string{"need other product"},
		},
	})
	t.Cleanup(func() {
		contract.ClearRuntimeContractFinalForTest(leaf)
		contract.ClearProductDeclForTest("sample")
	})
	contract.RegisterProductDecl(contract.ProductDecl{
		ID: "sample",
		Selection: contract.ProductSelectionDecl{
			AgentSummary: "Sample product",
			UseWhen:      []string{"sample routing"},
			AvoidWhen:    []string{"not sample"},
		},
	})
	product := &cobra.Command{Use: "sample"}
	product.AddCommand(leaf)
	root.AddCommand(product)

	registry, err := schemaRegistryForTest(root)
	if err != nil {
		t.Fatalf("declared production assembly: %v", err)
	}
	if len(registry.Products) != 1 || len(registry.Products[0].Tools) != 1 {
		t.Fatalf("registry = %#v", registry.Products)
	}
	tool := registry.Products[0].Tools[0]
	if tool.MetadataSource != "corecmd.contract" {
		t.Fatalf("metadata_source = %q, want corecmd.contract", tool.MetadataSource)
	}
	if registry.Products[0].Selection.AgentSummary != "Sample product" {
		t.Fatalf("product selection = %#v", registry.Products[0].Selection)
	}
}
