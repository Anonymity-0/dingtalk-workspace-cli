// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

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
	decl, ok := lookupSchemaHintDecl("doc.create_document")
	if !ok {
		t.Fatal("expected doc.create_document in compiled hint decls")
	}
	attachSchemaHintDecl(cmd, decl)
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
	if got := len(schemaHintDeclsByCanonical); got < 800 {
		t.Fatalf("compiled decls = %d, want >= 800", got)
	}
	if _, ok := lookupSchemaHintDecl("aitable.view_update_aggregate"); !ok {
		t.Fatal("missing aitable.view_update_aggregate")
	}
}
