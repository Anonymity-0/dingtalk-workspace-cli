// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package agentmetadata

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/contractfinal"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/spf13/cobra"
)

func reviewedSelectionProvenance(source string) map[string]FieldProvenance {
	return map[string]FieldProvenance{
		"agent_summary": {
			Precedence: selectionPrecedenceReviewedExplicit,
			Source:     source,
			Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceReviewedExplicit, Source: source}},
		},
		"use_when": {
			Precedence: selectionPrecedenceReviewedExplicit,
			Source:     source,
			Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceReviewedExplicit, Source: source}},
		},
		"avoid_when": {
			Precedence: selectionPrecedenceReviewedExplicit,
			Source:     source,
			Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceReviewedExplicit, Source: source}},
		},
	}
}

func TestCrossPlatformCoverageManualHintsRetainAndValidateReviewedSelection(t *testing.T) {
	file := &File{
		Products: map[string]ProductMetadata{
			"sample": {
				AgentSummary: "summary", agentSummaryRank: selectionRankReviewedExplicit,
				UseWhen: []string{"use"}, useWhenRank: selectionRankReviewedExplicit,
				AvoidWhen: []string{"avoid"}, avoidWhenRank: selectionRankReviewedExplicit,
				fieldCandidates: map[string][]FieldCandidateProvenance{
					"agent_summary": {
						{Value: "summary", Precedence: selectionPrecedenceReviewedExplicit, Source: "hints/selection/sample.json", Selected: true},
						{Value: "skill", Precedence: selectionPrecedenceSkill, Source: "skill"},
					},
				},
				FieldProvenance: reviewedSelectionProvenance("hints/selection/sample.json"),
			},
		},
		Tools: map[string]ToolMetadata{
			"sample run": {
				AgentSummary: "tool", agentSummaryRank: selectionRankContractFinal,
				UseWhen: []string{"use"}, useWhenRank: selectionRankContractFinal,
				AvoidWhen: []string{"avoid"}, avoidWhenRank: selectionRankContractFinal,
				Examples: []string{"dws sample run"}, examplesRank: selectionRankContractFinal,
				Reviewed: boolPtr(true), reviewedRank: selectionRankContractFinal,
				fieldCandidates: map[string][]FieldCandidateProvenance{
					"examples": {
						{Value: []string{"dws sample run"}, Precedence: selectionPrecedenceContractFinal, Source: contractFinalOrigin, Selected: true},
						{Value: []string{"other"}, Precedence: selectionPrecedenceSkill, Source: "skill"},
					},
				},
				FieldProvenance: map[string]FieldProvenance{
					"agent_summary": {Precedence: selectionPrecedenceContractFinal, Source: contractFinalOrigin, Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceContractFinal, Source: contractFinalOrigin}}},
					"use_when":      {Precedence: selectionPrecedenceContractFinal, Source: contractFinalOrigin, Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceContractFinal, Source: contractFinalOrigin}}},
					"avoid_when":    {Precedence: selectionPrecedenceContractFinal, Source: contractFinalOrigin, Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceContractFinal, Source: contractFinalOrigin}}},
					"examples":      {Precedence: selectionPrecedenceContractFinal, Source: contractFinalOrigin, Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceContractFinal, Source: contractFinalOrigin}}},
				},
			},
		},
	}
	retainReviewedSelectionCandidates(file)
	if len(file.Products["sample"].fieldCandidates["agent_summary"]) != 1 {
		t.Fatalf("product candidates = %#v", file.Products["sample"].fieldCandidates["agent_summary"])
	}
	if len(file.Tools["sample run"].fieldCandidates["examples"]) != 1 {
		t.Fatalf("tool candidates = %#v", file.Tools["sample run"].fieldCandidates["examples"])
	}
	if !isReviewedSelectionRank(selectionRankReviewedExplicit) || !isReviewedSelectionRank(selectionRankContractFinal) {
		t.Fatal("reviewed rank helpers changed")
	}
	if got := onlyReviewedExplicitCandidates([]FieldCandidateProvenance{
		{Precedence: selectionPrecedenceSkill},
		{Precedence: selectionPrecedenceContractFinal},
	}); len(got) != 1 {
		t.Fatalf("filtered candidates = %#v", got)
	}

	opts := Options{
		ProductIDs:         map[string]bool{"sample": true},
		CanonicalToolPaths: map[string]string{"sample.run": "sample run"},
	}
	if err := validateReviewedSelectionDelivery(*file, opts); err != nil {
		t.Fatalf("valid reviewed delivery: %v", err)
	}
	if err := validateReviewedSelectionDelivery(File{}, Options{ProductIDs: map[string]bool{"missing": true}}); err == nil ||
		!strings.Contains(err.Error(), "missing product missing") {
		t.Fatalf("missing product error = %v", err)
	}
	if err := validateReviewedSelectionDelivery(File{Tools: map[string]ToolMetadata{}}, Options{
		CanonicalToolPaths: map[string]string{"sample.run": "sample run"},
	}); err == nil || !strings.Contains(err.Error(), "missing tool sample.run") {
		t.Fatalf("missing tool error = %v", err)
	}
}

func TestCrossPlatformCoverageManualHintsCoverageHelpers(t *testing.T) {
	t.Cleanup(func() { contract.ClearProductDeclForTest("declared") })
	contract.RegisterProductDecl(contract.ProductDecl{
		ID: "declared",
		Selection: contract.ProductSelectionDecl{
			AgentSummary: "Declared",
			UseWhen:      []string{"use"},
			AvoidWhen:    []string{"avoid"},
		},
	})
	if products := selectionHintCoverageProducts(map[string]bool{"declared": true, "hinted": true}); products["declared"] || !products["hinted"] {
		t.Fatalf("selectionHintCoverageProducts = %#v", products)
	}
	if got := expectedCanonicalToolPaths(Options{CanonicalToolPaths: map[string]string{"sample.run": " sample run "}}); got["sample.run"] != "sample run" {
		t.Fatalf("expectedCanonicalToolPaths canonical = %#v", got)
	}
	if got := expectedCanonicalToolPaths(Options{ToolPaths: map[string]string{"sample run": "sample run", "sample.run": "sample run"}}); got["sample.run"] != "sample run" {
		t.Fatalf("expectedCanonicalToolPaths tool paths = %#v", got)
	}
	if got := expectedCanonicalToolSet(Options{ToolPaths: map[string]string{"sample.run": "sample run", "sample run": "sample run"}}); !got["sample.run"] || got["sample run"] {
		t.Fatalf("expectedCanonicalToolSet = %#v", got)
	}
	if got := expectedCanonicalToolSet(Options{CanonicalToolPaths: map[string]string{"sample.run": "sample run"}}); !got["sample.run"] {
		t.Fatalf("expectedCanonicalToolSet canonical = %#v", got)
	}
	if err := validateSelectionAuthoringContracts(Options{BoundCommands: cli.BoundCommandRegistry{Commands: []cli.BoundCommandSpec{}}}); err != nil {
		t.Fatalf("empty bound registry: %v", err)
	}
	selected := true
	if selectedCandidateCount([]FieldCandidateProvenance{{Selected: true}, {}}) != 1 {
		t.Fatal("selectedCandidateCount changed")
	}
	if _, err := parseHintSources(nil, nil, Options{HintsDir: "schema_hints"}, nil, sourceTracker{}); err == nil ||
		!strings.Contains(err.Error(), "schema_hints/ is retired") {
		t.Fatalf("parseHintSources error = %v", err)
	}
	if _, err := parseHintSources(&File{}, nil, Options{}, nil, sourceTracker{}); err != nil {
		t.Fatalf("empty hints dir: %v", err)
	}
	_ = selected
}

func TestCrossPlatformCoveragePipelineSelectionCoverageAndGenerateFromRoot(t *testing.T) {
	if err := ValidateSelectionHints("", "", RegistryProjection{
		CanonicalToolPaths: map[string]string{"sample.run": "sample run"},
		ProductIDs:         map[string]bool{"sample": true},
	}); err == nil || !strings.Contains(err.Error(), "selection coverage incomplete") {
		t.Fatalf("ValidateSelectionHints error = %v", err)
	}
	if !selectionHintCoverageRequired(map[string]bool{"sample": true}, nil) {
		t.Fatal("product coverage should require hints")
	}
	if selectionHintCoverageRequired(nil, map[string]bool{"sample.run": false}) {
		t.Fatal("all-false tool coverage should not require hints")
	}
	if got := resolvePipelineRootPath("root", filepath.Join("..", "abs")); !strings.Contains(got, "abs") {
		t.Fatalf("resolvePipelineRootPath relative = %q", got)
	}
	if _, _, _, err := GenerateFromCommandRoot(".", nil, Options{}); err == nil || !strings.Contains(err.Error(), "root is nil") {
		t.Fatalf("nil root error = %v", err)
	}
}

func TestCrossPlatformCoverageContractFinalDeclarationsMergeToolMetadata(t *testing.T) {
	declared := &cobra.Command{Use: "run"}
	cli.RegisterRuntimeContractFinal(declared, contract.ContractFinalPayload{
		Selection: &contract.SelectionSpec{
			AgentSummary:  "Declared summary",
			UseWhen:       []string{"use declared"},
			AvoidWhen:     []string{"avoid declared"},
			Prerequisites: []string{},
			Tips:          []string{"tip"},
			WorkflowRefs:  []string{"wf"},
			Examples:      []string{"dws sample run"},
		},
		Safety: &contract.SafetySpec{
			Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "idempotent",
		},
		Interface: &contract.InterfaceSpec{
			Mode: contract.InterfaceModeMCP, Availability: contract.InterfaceAvailable,
			Ref: &contract.InterfaceRefSpec{ProductID: "sample", RPCName: "run"},
		},
	})
	t.Cleanup(func() { contractfinal.ClearRuntimeContractFinalForTest(declared) })

	file := &File{Tools: map[string]ToolMetadata{}}
	if err := applyContractFinalDeclarations(file, Options{
		BoundCommands: cli.BoundCommandRegistry{ByCanonical: map[string]cli.BoundCommandSpec{
			"sample.run": {PrimaryCommand: declared},
		}},
		CanonicalToolPaths: map[string]string{"sample.run": "sample run"},
	}); err != nil {
		t.Fatalf("applyContractFinalDeclarations: %v", err)
	}
	tool := file.Tools["sample run"]
	if tool.AgentSummary != "Declared summary" || tool.Effect != "read" || tool.InterfaceMode != contract.InterfaceModeMCP {
		t.Fatalf("merged tool = %#v", tool)
	}
	if metadata, ok := contractFinalToolMetadata(&cobra.Command{Use: "missing"}); ok || metadata.AgentSummary != "" {
		t.Fatalf("missing overlay metadata = %#v, ok=%v", metadata, ok)
	}
}

func boolPtr(v bool) *bool { return &v }

func TestCrossPlatformCoverageManualHintsReviewedDeliveryProductDeclSkip(t *testing.T) {
	t.Cleanup(func() { contract.ClearProductDeclForTest("declared-only") })
	contract.RegisterProductDecl(contract.ProductDecl{
		ID: "declared-only",
		Selection: contract.ProductSelectionDecl{
			AgentSummary: "Declared only",
			UseWhen:      []string{"use"},
			AvoidWhen:    []string{"avoid"},
		},
	})
	if err := validateReviewedSelectionDelivery(File{}, Options{
		ProductIDs: map[string]bool{"declared-only": true},
	}); err != nil {
		t.Fatalf("ProductDecl skip: %v", err)
	}
}

func TestCrossPlatformCoverageManualHintsReviewedDeliveryProductFailures(t *testing.T) {
	badProduct := ProductMetadata{
		AgentSummary: "summary", agentSummaryRank: selectionRankSkill,
		UseWhen: []string{"use"}, useWhenRank: selectionRankReviewedExplicit,
		AvoidWhen: []string{"avoid"}, avoidWhenRank: selectionRankReviewedExplicit,
		FieldProvenance: reviewedSelectionProvenance("hints/selection/sample.json"),
	}
	if err := validateReviewedSelectionDelivery(File{Products: map[string]ProductMetadata{"sample": badProduct}}, Options{
		ProductIDs: map[string]bool{"sample": true},
	}); err == nil || !strings.Contains(err.Error(), "agent_summary is not reviewed") {
		t.Fatalf("bad agent_summary rank = %v", err)
	}

	badProduct = ProductMetadata{
		AgentSummary: "summary", agentSummaryRank: selectionRankReviewedExplicit,
		UseWhen: []string{}, useWhenRank: selectionRankReviewedExplicit,
		AvoidWhen: []string{"avoid"}, avoidWhenRank: selectionRankReviewedExplicit,
		FieldProvenance: reviewedSelectionProvenance("hints/selection/sample.json"),
	}
	if err := validateReviewedSelectionDelivery(File{Products: map[string]ProductMetadata{"sample": badProduct}}, Options{
		ProductIDs: map[string]bool{"sample": true},
	}); err == nil || !strings.Contains(err.Error(), "use_when is not reviewed") {
		t.Fatalf("empty use_when = %v", err)
	}

	badProduct = ProductMetadata{
		AgentSummary: "summary", agentSummaryRank: selectionRankReviewedExplicit,
		UseWhen: []string{"use"}, useWhenRank: selectionRankReviewedExplicit,
		AvoidWhen: []string{}, avoidWhenRank: selectionRankReviewedExplicit,
		FieldProvenance: reviewedSelectionProvenance("hints/selection/sample.json"),
	}
	if err := validateReviewedSelectionDelivery(File{Products: map[string]ProductMetadata{"sample": badProduct}}, Options{
		ProductIDs: map[string]bool{"sample": true},
	}); err == nil || !strings.Contains(err.Error(), "avoid_when is not reviewed") {
		t.Fatalf("empty avoid_when = %v", err)
	}

	badProvenance := ProductMetadata{
		AgentSummary: "summary", agentSummaryRank: selectionRankReviewedExplicit,
		UseWhen: []string{"use"}, useWhenRank: selectionRankReviewedExplicit,
		AvoidWhen: []string{"avoid"}, avoidWhenRank: selectionRankReviewedExplicit,
		FieldProvenance: map[string]FieldProvenance{
			"agent_summary": {Precedence: selectionPrecedenceSkill, Candidates: []FieldCandidateProvenance{{Selected: true}}},
			"use_when":      reviewedSelectionProvenance("hints/selection/sample.json")["use_when"],
			"avoid_when":    reviewedSelectionProvenance("hints/selection/sample.json")["avoid_when"],
		},
	}
	if err := validateReviewedSelectionDelivery(File{Products: map[string]ProductMetadata{"sample": badProvenance}}, Options{
		ProductIDs: map[string]bool{"sample": true},
	}); err == nil || !strings.Contains(err.Error(), "agent_summary provenance") {
		t.Fatalf("bad provenance = %v", err)
	}

	badSource := ProductMetadata{
		AgentSummary: "summary", agentSummaryRank: selectionRankReviewedExplicit,
		UseWhen: []string{"use"}, useWhenRank: selectionRankReviewedExplicit,
		AvoidWhen: []string{"avoid"}, avoidWhenRank: selectionRankReviewedExplicit,
		FieldProvenance: map[string]FieldProvenance{
			"agent_summary": {Precedence: selectionPrecedenceReviewedExplicit, Source: "skill.md", Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceReviewedExplicit}}},
			"use_when":      {Precedence: selectionPrecedenceReviewedExplicit, Source: "hints/selection/sample.json", Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceReviewedExplicit}}},
			"avoid_when":    {Precedence: selectionPrecedenceReviewedExplicit, Source: "hints/selection/sample.json", Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceReviewedExplicit}}},
		},
	}
	if err := validateReviewedSelectionDelivery(File{Products: map[string]ProductMetadata{"sample": badSource}}, Options{
		ProductIDs: map[string]bool{"sample": true},
	}); err == nil || !strings.Contains(err.Error(), "source is not selection/") {
		t.Fatalf("bad source = %v", err)
	}
}

func TestCrossPlatformCoverageManualHintsReviewedDeliveryToolFailures(t *testing.T) {
	reviewedTool := func(mut func(*ToolMetadata)) ToolMetadata {
		tool := ToolMetadata{
			AgentSummary: "tool", agentSummaryRank: selectionRankReviewedExplicit,
			UseWhen: []string{"use"}, useWhenRank: selectionRankReviewedExplicit,
			AvoidWhen: []string{"avoid"}, avoidWhenRank: selectionRankReviewedExplicit,
			Examples: []string{"dws sample run"}, examplesRank: selectionRankReviewedExplicit,
			Reviewed: boolPtr(true), reviewedRank: selectionRankReviewedExplicit,
			FieldProvenance: reviewedSelectionProvenance("hints/selection/sample.json"),
		}
		tool.FieldProvenance["examples"] = FieldProvenance{
			Precedence: selectionPrecedenceReviewedExplicit, Source: "hints/selection/sample.json",
			Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceReviewedExplicit}},
		}
		if mut != nil {
			mut(&tool)
		}
		return tool
	}

	if err := validateReviewedSelectionDelivery(File{Tools: map[string]ToolMetadata{
		"sample run": reviewedTool(func(t *ToolMetadata) { t.agentSummaryRank = selectionRankSkill }),
	}}, Options{CanonicalToolPaths: map[string]string{"sample.run": "sample run"}}); err == nil ||
		!strings.Contains(err.Error(), "sample.run agent_summary is not reviewed") {
		t.Fatalf("tool agent_summary = %v", err)
	}

	if err := validateReviewedSelectionDelivery(File{Tools: map[string]ToolMetadata{
		"sample run": reviewedTool(func(t *ToolMetadata) { t.UseWhen = nil }),
	}}, Options{CanonicalToolPaths: map[string]string{"sample.run": "sample run"}}); err == nil ||
		!strings.Contains(err.Error(), "sample.run use_when is not reviewed") {
		t.Fatalf("tool use_when = %v", err)
	}

	if err := validateReviewedSelectionDelivery(File{Tools: map[string]ToolMetadata{
		"sample run": reviewedTool(func(t *ToolMetadata) { t.AvoidWhen = nil }),
	}}, Options{CanonicalToolPaths: map[string]string{"sample.run": "sample run"}}); err == nil ||
		!strings.Contains(err.Error(), "sample.run avoid_when is not reviewed") {
		t.Fatalf("tool avoid_when = %v", err)
	}

	if err := validateReviewedSelectionDelivery(File{Tools: map[string]ToolMetadata{
		"sample run": reviewedTool(func(t *ToolMetadata) { t.Examples = nil }),
	}}, Options{CanonicalToolPaths: map[string]string{"sample.run": "sample run"}}); err == nil ||
		!strings.Contains(err.Error(), "sample.run examples is not reviewed") {
		t.Fatalf("tool examples = %v", err)
	}

	if err := validateReviewedSelectionDelivery(File{Tools: map[string]ToolMetadata{
		"sample run": reviewedTool(func(t *ToolMetadata) { t.Reviewed = boolPtr(false) }),
	}}, Options{CanonicalToolPaths: map[string]string{"sample.run": "sample run"}}); err == nil ||
		!strings.Contains(err.Error(), "sample.run reviewed is not reviewed") {
		t.Fatalf("tool reviewed = %v", err)
	}

	if err := validateReviewedSelectionDelivery(File{Tools: map[string]ToolMetadata{
		"sample run": reviewedTool(func(t *ToolMetadata) {
			t.FieldProvenance["examples"] = FieldProvenance{Precedence: selectionPrecedenceSkill, Candidates: []FieldCandidateProvenance{{Selected: true}}}
		}),
	}}, Options{CanonicalToolPaths: map[string]string{"sample.run": "sample run"}}); err == nil ||
		!strings.Contains(err.Error(), "sample.run examples provenance") {
		t.Fatalf("tool examples provenance = %v", err)
	}

	if err := validateReviewedSelectionDelivery(File{Tools: map[string]ToolMetadata{
		"sample run": reviewedTool(func(t *ToolMetadata) {
			t.FieldProvenance["agent_summary"] = FieldProvenance{
				Precedence: selectionPrecedenceReviewedExplicit, Source: "skill.md",
				Candidates: []FieldCandidateProvenance{{Selected: true, Precedence: selectionPrecedenceReviewedExplicit}},
			}
		}),
	}}, Options{CanonicalToolPaths: map[string]string{"sample.run": "sample run"}}); err == nil ||
		!strings.Contains(err.Error(), "sample.run agent_summary source is not selection/") {
		t.Fatalf("tool bad source = %v", err)
	}
}

func TestCrossPlatformCoverageSelectionHintCoverageProductsSkips(t *testing.T) {
	if got := selectionHintCoverageProducts(map[string]bool{"": true, "skip": false, "hinted": true}); got["hinted"] != true || got[""] || got["skip"] {
		t.Fatalf("selectionHintCoverageProducts = %#v", got)
	}
}

func TestCrossPlatformCoverageValidateSelectionAuthoringContractsPaths(t *testing.T) {
	if err := validateSelectionAuthoringContracts(Options{
		ProductIDs:    map[string]bool{"missing-product": true},
		ToolPaths:     map[string]string{"missing.tool": "missing tool"},
		BoundCommands: cli.BoundCommandRegistry{Commands: []cli.BoundCommandSpec{}},
	}); err == nil || !strings.Contains(err.Error(), "missing_products") {
		t.Fatalf("missing coverage error = %v", err)
	}

	declared := &cobra.Command{Use: "run"}
	cli.RegisterRuntimeContractFinal(declared, contract.ContractFinalPayload{
		Selection: &contract.SelectionSpec{
			AgentSummary: "Declared",
			UseWhen:      []string{"use"},
			AvoidWhen:    []string{"avoid"},
			Examples:     []string{"dws covered run"},
		},
	})
	t.Cleanup(func() { contractfinal.ClearRuntimeContractFinalForTest(declared) })
	if err := validateSelectionAuthoringContracts(Options{
		CanonicalToolPaths: map[string]string{"covered.run": "covered run"},
		BoundCommands: cli.BoundCommandRegistry{
			ByCanonical: map[string]cli.BoundCommandSpec{"covered.run": {PrimaryCommand: declared}},
		},
	}); err != nil {
		t.Fatalf("ContractFinal-covered tool: %v", err)
	}
}

func TestCrossPlatformCoveragePipelineGenerateFromCommandRootErrors(t *testing.T) {
	origBuild := pipelineBuildEffectiveRegistry
	origBind := pipelineBindEffectiveRegistry
	origGenerate := pipelineGenerateMetadata
	t.Cleanup(func() {
		pipelineBuildEffectiveRegistry = origBuild
		pipelineBindEffectiveRegistry = origBind
		pipelineGenerateMetadata = origGenerate
	})

	if got := resolvePipelineRootPath("/root", "/abs/meta"); got != "/abs/meta" {
		t.Fatalf("absolute path = %q", got)
	}

	root := &cobra.Command{Use: "partial"}
	_, _, _, err := GenerateFromCommandRoot("", root, Options{})
	if err == nil || !strings.Contains(err.Error(), "bind effective CommandRegistry") {
		t.Fatalf("bind failure = %v", err)
	}

	pipelineBuildEffectiveRegistry = func(*cobra.Command) (cli.EffectiveCommandRegistry, error) {
		return cli.EffectiveCommandRegistry{}, fmt.Errorf("build boom")
	}
	_, _, _, err = GenerateFromCommandRoot(".", root, Options{})
	if err == nil || !strings.Contains(err.Error(), "build effective CommandRegistry") {
		t.Fatalf("build failure = %v", err)
	}
	pipelineBuildEffectiveRegistry = origBuild

	pipelineBindEffectiveRegistry = func(*cobra.Command, cli.EffectiveCommandRegistry) (cli.BoundCommandRegistry, error) {
		return cli.BoundCommandRegistry{}, fmt.Errorf("bind boom")
	}
	_, _, _, err = GenerateFromCommandRoot(".", root, Options{})
	if err == nil || !strings.Contains(err.Error(), "bind effective CommandRegistry") {
		t.Fatalf("stub bind failure = %v", err)
	}
	pipelineBindEffectiveRegistry = origBind

	plain := &cobra.Command{Use: "missing"}
	pipelineBuildEffectiveRegistry = func(*cobra.Command) (cli.EffectiveCommandRegistry, error) {
		spec := cli.CommandSpec{
			CanonicalPath:  "missing.tool",
			PrimaryCLIPath: "missing tool",
			Visibility:     cli.SchemaVisibilityPublic,
		}
		return cli.EffectiveCommandRegistry{
			Commands:    []cli.CommandSpec{spec},
			ByCanonical: map[string]cli.CommandSpec{"missing.tool": spec},
		}, nil
	}
	pipelineBindEffectiveRegistry = func(*cobra.Command, cli.EffectiveCommandRegistry) (cli.BoundCommandRegistry, error) {
		return cli.BoundCommandRegistry{
			ByCanonical: map[string]cli.BoundCommandSpec{"missing.tool": {PrimaryCommand: plain}},
		}, nil
	}
	_, _, _, err = GenerateFromCommandRoot(".", root, Options{})
	if err == nil || !strings.Contains(err.Error(), "selection coverage incomplete") {
		t.Fatalf("coverage failure = %v", err)
	}

	pipelineBuildEffectiveRegistry = func(*cobra.Command) (cli.EffectiveCommandRegistry, error) {
		return cli.EffectiveCommandRegistry{}, nil
	}
	pipelineBindEffectiveRegistry = func(*cobra.Command, cli.EffectiveCommandRegistry) (cli.BoundCommandRegistry, error) {
		return cli.BoundCommandRegistry{}, nil
	}
	pipelineGenerateMetadata = func(Options) (File, Stats, error) {
		return File{}, Stats{}, fmt.Errorf("generate boom")
	}
	_, _, _, err = GenerateFromCommandRoot(".", &cobra.Command{Use: "ok"}, Options{})
	if err == nil || !strings.Contains(err.Error(), "generate in-memory Agent metadata") {
		t.Fatalf("generate failure = %v", err)
	}
}
