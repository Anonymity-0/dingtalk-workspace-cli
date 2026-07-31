// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func testSchemaHintDeclFixture() schemaHintDecl {
	return schemaHintDecl{
		Safety: SafetySpec{
			Effect: "read", Risk: "low",
			Confirmation: "not_required", Idempotency: "idempotent",
		},
		Description: "fixture command for attachSchemaHintDecl freeze test",
		Interface: &InterfaceSpec{
			Mode: InterfaceModeMCP, Availability: InterfaceAvailable,
			Ref: &InterfaceRefSpec{ProductID: "fixture", RPCName: "fixture_probe"},
		},
		Selection: SelectionSpec{
			AgentSummary: "fixture probe",
			UseWhen:      []string{"unit testing attachSchemaHintDecl"},
			AvoidWhen:    []string{"production use"},
			Examples:     []string{"dws fixture probe"},
		},
	}
}

func TestSchemaHintDeclAttachDoesNotReplaceRunE(t *testing.T) {
	ran := false
	cmd := &cobra.Command{
		Use:   "probe",
		Short: "probe",
		RunE: func(*cobra.Command, []string) error {
			ran = true
			return nil
		},
	}
	attachSchemaHintDecl(cmd, testSchemaHintDeclFixture())
	if !HasRuntimeContractFinal(cmd) {
		t.Fatal("expected ContractFinal after attach")
	}
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !ran {
		t.Fatal("RunE body was not invoked — execution must stay frozen")
	}
	ClearRuntimeContractFinalForTest(cmd)
}

func TestSchemaHintDeclLookupCoverage(t *testing.T) {
	// All reviewed tools now author ContractFinal beside the command
	// (DeclareLeafMetadata / Shortcut.Schema). The compiled residual map is empty.
	if got := len(schemaHintDeclsByCanonical); got != 0 {
		t.Fatalf("compiled residual decls = %d, want 0 after full colocation", got)
	}
	if _, ok := lookupSchemaHintDecl("aitable.view_update_aggregate"); ok {
		t.Fatal("aitable.view_update_aggregate must not remain in compiled hint decls")
	}
	if _, ok := lookupSchemaHintDecl("aitable.shortcut_base_list"); ok {
		t.Fatal("aitable.shortcut_base_list must not remain in compiled hint decls")
	}
	if _, ok := lookupSchemaHintDecl("doc.create_document"); ok {
		t.Fatal("doc.create_document must not remain in compiled hint decls")
	}
}
