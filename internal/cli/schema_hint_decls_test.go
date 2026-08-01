// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cli

import (
	"strings"
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

func TestCrossPlatformCoverageLookupSchemaHintDeclBlankCanonical(t *testing.T) {
	if _, ok := lookupSchemaHintDecl(""); ok {
		t.Fatal("empty canonical must not resolve a hint decl")
	}
	if _, ok := lookupSchemaHintDecl("   "); ok {
		t.Fatal("blank canonical must not resolve a hint decl")
	}
}

func TestCrossPlatformCoverageAttachSchemaHintDeclNilCommandIsNoOp(t *testing.T) {
	// Must not panic and must not register anything.
	attachSchemaHintDecl(nil, testSchemaHintDeclFixture())
}

func TestCrossPlatformCoverageAttachSchemaHintDeclCompletenessPanics(t *testing.T) {
	mustPanic := func(t *testing.T, wantFragment string, decl schemaHintDecl) {
		t.Helper()
		cmd := &cobra.Command{Use: "probe", Short: "probe"}
		defer func() {
			recovered := recover()
			if recovered == nil {
				t.Fatalf("attachSchemaHintDecl must panic for %q", wantFragment)
			}
			message, ok := recovered.(string)
			if !ok || !strings.Contains(message, wantFragment) {
				t.Fatalf("panic = %v, want fragment %q", recovered, wantFragment)
			}
			if HasRuntimeContractFinal(cmd) {
				t.Fatal("panicking attach must not register ContractFinal")
			}
		}()
		attachSchemaHintDecl(cmd, decl)
	}

	t.Run("missing description", func(t *testing.T) {
		decl := testSchemaHintDeclFixture()
		decl.Description = "   "
		mustPanic(t, "missing Description", decl)
	})
	t.Run("missing selection prose", func(t *testing.T) {
		decl := testSchemaHintDeclFixture()
		decl.Selection.UseWhen = nil
		mustPanic(t, "missing Selection prose", decl)
	})
	t.Run("missing interface", func(t *testing.T) {
		decl := testSchemaHintDeclFixture()
		decl.Interface = nil
		mustPanic(t, "missing Interface", decl)
	})
	t.Run("composite without reason", func(t *testing.T) {
		decl := testSchemaHintDeclFixture()
		decl.Interface = &InterfaceSpec{Mode: InterfaceModeComposite, Availability: InterfaceAvailable}
		mustPanic(t, "missing Interface.Reason", decl)
	})
}

func TestCrossPlatformCoverageAttachSchemaHintDeclDefaultsEmptySafetyToRead(t *testing.T) {
	decl := testSchemaHintDeclFixture()
	decl.Safety = SafetySpec{}
	cmd := &cobra.Command{Use: "probe", Short: "probe"}
	t.Cleanup(func() { ClearRuntimeContractFinalForTest(cmd) })
	attachSchemaHintDecl(cmd, decl)
	final, ok := RuntimeContractFinal(cmd)
	if !ok || final.Safety == nil {
		t.Fatalf("ContractFinal after attach = %#v ok=%v", final, ok)
	}
	if final.Safety.Effect != "read" || final.Safety.Risk != "low" ||
		final.Safety.Confirmation != "not_required" || final.Safety.Idempotency != "idempotent" {
		t.Fatalf("defaulted safety = %#v", final.Safety)
	}
	if final.Safety.EffectSource != "corecmd.contract" {
		t.Fatalf("effect source = %q, want corecmd.contract", final.Safety.EffectSource)
	}
}

func TestFirstNonEmptyTrim(t *testing.T) {
	if got := firstNonEmptyTrim("  ", "", "\t"); got != "" {
		t.Fatalf("all-blank firstNonEmptyTrim = %q, want empty", got)
	}
	if got := firstNonEmptyTrim(); got != "" {
		t.Fatalf("no-arg firstNonEmptyTrim = %q, want empty", got)
	}
	if got := firstNonEmptyTrim("", "  value  "); got != "value" {
		t.Fatalf("firstNonEmptyTrim = %q, want value", got)
	}
}
