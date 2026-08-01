// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"testing"
)

func TestProductDeclRegistryRoundTrip(t *testing.T) {
	t.Cleanup(func() { contract.ClearProductDeclForTest("sample") })
	contract.ClearProductDeclForTest("sample")

	if contract.HasProductDecl("sample") {
		t.Fatal("contract.HasProductDecl before register must be false")
	}
	contract.RegisterProductDecl(contract.ProductDecl{})
	if contract.HasProductDecl("") {
		t.Fatal("empty ID must not register")
	}

	contract.RegisterProductDecl(contract.ProductDecl{
		ID: " sample ",
		Selection: contract.ProductSelectionDecl{
			AgentSummary: "Manage samples",
			UseWhen:      []string{"target is a sample"},
			AvoidWhen:    []string{"target is another product"},
		},
	})
	if !contract.HasProductDecl("sample") {
		t.Fatal("contract.HasProductDecl after register must be true")
	}
	got, ok := contract.LookupProductDecl("sample")
	if !ok || got.ID != "sample" || got.Selection.AgentSummary != "Manage samples" {
		t.Fatalf("contract.LookupProductDecl = %#v, ok=%v", got, ok)
	}
	ids := contract.RegisteredProductDeclIDs()
	found := false
	for _, id := range ids {
		if id == "sample" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("contract.RegisteredProductDeclIDs missing sample: %#v", ids)
	}

	selection, provenance := contract.ProductSelectionFromDecl(got)
	if selection.AgentSummary != "Manage samples" || selection.AgentSummarySource != contract.ProductDeclSourceRef {
		t.Fatalf("contract.ProductSelectionFromDecl selection = %#v", selection)
	}
	for _, field := range []string{"agent_summary", "use_when", "avoid_when"} {
		prov, ok := provenance[field]
		if !ok || prov.Precedence != "contract_final" || prov.Source != contract.ProductDeclProvenanceSource {
			t.Fatalf("field %s provenance = %#v", field, prov)
		}
	}

	contract.ClearProductDeclForTest("sample")
	if contract.HasProductDecl("sample") {
		t.Fatal("contract.ClearProductDeclForTest must remove registration")
	}
}

func TestProductDeclRegisterPanicsOnIncompleteSelection(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for incomplete contract.ProductDecl")
		}
	}()
	contract.RegisterProductDecl(contract.ProductDecl{ID: "broken"})
}
