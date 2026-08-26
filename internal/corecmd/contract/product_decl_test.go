// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package contract

import "testing"

func TestCrossPlatformCoverageProductDeclRegistryRoundTrip(t *testing.T) {
	t.Cleanup(func() { ClearProductDeclForTest("sample") })
	ClearProductDeclForTest("sample")

	if HasProductDecl("sample") {
		t.Fatal("HasProductDecl before register must be false")
	}
	RegisterProductDecl(ProductDecl{})
	if HasProductDecl("") {
		t.Fatal("empty ID must not register")
	}

	RegisterProductDecl(ProductDecl{
		ID: " sample ",
		HelpReferences: HelpReferences{
			RelatedSkills: []string{" dingtalk-sample ", "dingtalk-sample"},
			Documentation: []HelpDocumentation{
				SkillDocumentation(" Sample guide ", "dingtalk-sample", "references/sample.md"),
			},
		},
		Selection: ProductSelectionDecl{
			AgentSummary: "Manage samples",
			UseWhen:      []string{"target is a sample"},
			AvoidWhen:    []string{"target is another product"},
		},
	})
	if !HasProductDecl("sample") {
		t.Fatal("HasProductDecl after register must be true")
	}
	got, ok := LookupProductDecl("sample")
	if !ok || got.ID != "sample" || got.Selection.AgentSummary != "Manage samples" {
		t.Fatalf("LookupProductDecl = %#v, ok=%v", got, ok)
	}
	if len(got.HelpReferences.RelatedSkills) != 1 || got.HelpReferences.RelatedSkills[0] != "dingtalk-sample" ||
		len(got.HelpReferences.Documentation) != 1 || got.HelpReferences.Documentation[0].Label != "Sample guide" ||
		got.HelpReferences.Documentation[0].URL != "https://github.com/DingTalk-Real-AI/dingtalk-workspace-cli/blob/main/skills/multi/dingtalk-sample/references/sample.md" {
		t.Fatalf("normalized HelpReferences = %#v", got.HelpReferences)
	}
	got.HelpReferences.RelatedSkills[0] = "mutated"
	got.HelpReferences.Documentation[0].Label = "mutated"
	again, _ := LookupProductDecl("sample")
	if again.HelpReferences.RelatedSkills[0] != "dingtalk-sample" || again.HelpReferences.Documentation[0].Label != "Sample guide" {
		t.Fatalf("LookupProductDecl leaked mutable HelpReferences: %#v", again.HelpReferences)
	}
	ids := RegisteredProductDeclIDs()
	found := false
	for _, id := range ids {
		if id == "sample" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("RegisteredProductDeclIDs missing sample: %#v", ids)
	}

	selection, provenance := ProductSelectionFromDecl(got)
	if selection.AgentSummary != "Manage samples" || selection.AgentSummarySource != ProductDeclSourceRef {
		t.Fatalf("ProductSelectionFromDecl selection = %#v", selection)
	}
	for _, field := range []string{"agent_summary", "use_when", "avoid_when"} {
		prov, ok := provenance[field]
		if !ok || prov.Precedence != "contract_final" || prov.Source != ProductDeclProvenanceSource {
			t.Fatalf("field %s provenance = %#v", field, prov)
		}
	}

	ClearProductDeclForTest("sample")
	if HasProductDecl("sample") {
		t.Fatal("ClearProductDeclForTest must remove registration")
	}
}

func TestCrossPlatformCoverageProductDeclLookupRejectsWrongType(t *testing.T) {
	t.Cleanup(func() { productDecls.Delete("broken-type") })
	productDecls.Store("broken-type", "not-a-product-decl")
	if _, ok := LookupProductDecl("broken-type"); ok {
		t.Fatal("LookupProductDecl must reject non-ProductDecl values")
	}
}

func TestCrossPlatformCoverageProductDeclRegisterPanicsOnIncompleteSelection(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for incomplete ProductDecl")
		}
	}()
	RegisterProductDecl(ProductDecl{ID: "broken"})
}

func TestCrossPlatformCoverageProductDeclRejectsInvalidHelpDocumentation(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for non-HTTPS Help documentation")
		}
	}()
	RegisterProductDecl(ProductDecl{
		ID: "invalid-help",
		Selection: ProductSelectionDecl{
			AgentSummary: "invalid Help fixture",
			UseWhen:      []string{"test invalid Help"},
			AvoidWhen:    []string{"production"},
		},
		HelpReferences: HelpReferences{
			Documentation: []HelpDocumentation{{Label: "invalid", URL: "http://example.com"}},
		},
	})
}
