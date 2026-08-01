// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package agentmetadata

import (
	"fmt"
	"sort"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/contractfinal"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

var reviewedSelectionFields = map[string]bool{
	"agent_summary": true,
	"use_when":      true,
	"avoid_when":    true,
	"examples":      true,
}

// retainReviewedSelectionCandidates keeps only reviewed candidates
// (reviewed_explicit or contract_final) for Agent selection fields so
// metadata/skill/MCP prose cannot win after selection files have been
// applied. contract_final candidates come from in-code Contract declarations
// and outrank file sources for declared tools.
func retainReviewedSelectionCandidates(file *File) {
	if file == nil {
		return
	}
	for productID, metadata := range file.Products {
		if metadata.fieldCandidates == nil {
			metadata.fieldCandidates = map[string][]FieldCandidateProvenance{}
		}
		for _, field := range []string{"agent_summary", "use_when", "avoid_when"} {
			metadata.fieldCandidates[field] = onlyReviewedExplicitCandidates(metadata.fieldCandidates[field])
		}
		file.Products[productID] = metadata
	}
	for path, metadata := range file.Tools {
		if metadata.fieldCandidates == nil {
			metadata.fieldCandidates = map[string][]FieldCandidateProvenance{}
		}
		for field := range reviewedSelectionFields {
			metadata.fieldCandidates[field] = onlyReviewedExplicitCandidates(metadata.fieldCandidates[field])
		}
		file.Tools[path] = metadata
	}
}

func onlyReviewedExplicitCandidates(candidates []FieldCandidateProvenance) []FieldCandidateProvenance {
	selected := make([]FieldCandidateProvenance, 0, len(candidates))
	for _, candidate := range candidates {
		if isReviewedSelectionRank(precedenceRank(candidate.Precedence)) {
			selected = append(selected, candidate)
		}
	}
	return selected
}

// isReviewedSelectionRank reports whether a rank is a reviewed Agent selection
// source: the reviewed selection files (reviewed_explicit) or an in-code
// Contract final declaration (contract_final, the stronger reviewed source).
func isReviewedSelectionRank(rank int) bool {
	return rank == selectionRankReviewedExplicit || rank == selectionRankContractFinal
}

// isReviewedSelectionPrecedence mirrors isReviewedSelectionRank for provenance
// precedence labels.
func isReviewedSelectionPrecedence(precedence string) bool {
	switch strings.TrimSpace(precedence) {
	case selectionPrecedenceReviewedExplicit, selectionPrecedenceContractFinal:
		return true
	default:
		return false
	}
}

func validateReviewedSelectionDelivery(file File, opts Options) error {
	problems := []string{}
	productIDs := sortedBoolKeys(opts.ProductIDs)
	for _, productID := range productIDs {
		metadata, ok := file.Products[productID]
		if !ok {
			// ProductDecl-registered products may omit selection/ products{}
			// rows; their contract_final prose is applied outside hint files.
			if contract.HasProductDecl(productID) {
				continue
			}
			problems = append(problems, "missing product "+productID)
			continue
		}
		checks := []struct {
			name  string
			valid bool
		}{
			{"agent_summary", strings.TrimSpace(metadata.AgentSummary) != "" && isReviewedSelectionRank(metadata.agentSummaryRank)},
			{"use_when", len(metadata.UseWhen) > 0 && isReviewedSelectionRank(metadata.useWhenRank)},
			{"avoid_when", len(metadata.AvoidWhen) > 0 && isReviewedSelectionRank(metadata.avoidWhenRank)},
		}
		for _, check := range checks {
			if !check.valid {
				problems = append(problems, productID+" "+check.name+" is not reviewed_explicit/contract_final")
			}
		}
		for _, field := range []string{"agent_summary", "use_when", "avoid_when"} {
			provenance, ok := metadata.FieldProvenance[field]
			if !ok || !isReviewedSelectionPrecedence(provenance.Precedence) || selectedCandidateCount(provenance.Candidates) != 1 {
				problems = append(problems, productID+" "+field+" provenance is not one reviewed_explicit/contract_final winner")
			}
			// File-based winners must come from a selection/ source file;
			// contract_final winners are declared in code (no file path).
			if ok && provenance.Precedence != selectionPrecedenceContractFinal && !strings.Contains(provenance.Source, "/selection/") {
				problems = append(problems, productID+" "+field+" source is not selection/: "+provenance.Source)
			}
		}
	}

	expected := expectedCanonicalToolPaths(opts)
	canonicalPaths := make([]string, 0, len(expected))
	for canonical := range expected {
		canonicalPaths = append(canonicalPaths, canonical)
	}
	sort.Strings(canonicalPaths)
	for _, canonical := range canonicalPaths {
		primary := expected[canonical]
		metadata, ok := file.Tools[primary]
		if !ok {
			problems = append(problems, "missing tool "+canonical)
			continue
		}
		checks := []struct {
			name  string
			valid bool
		}{
			{"agent_summary", strings.TrimSpace(metadata.AgentSummary) != "" && isReviewedSelectionRank(metadata.agentSummaryRank)},
			{"use_when", len(metadata.UseWhen) > 0 && isReviewedSelectionRank(metadata.useWhenRank)},
			{"avoid_when", len(metadata.AvoidWhen) > 0 && isReviewedSelectionRank(metadata.avoidWhenRank)},
			{"examples", len(metadata.Examples) > 0 && isReviewedSelectionRank(metadata.examplesRank)},
			{"reviewed", metadata.Reviewed != nil && *metadata.Reviewed && isReviewedSelectionRank(metadata.reviewedRank)},
		}
		for _, check := range checks {
			if !check.valid {
				problems = append(problems, canonical+" "+check.name+" is not reviewed_explicit/contract_final")
			}
		}
		for _, field := range []string{"agent_summary", "use_when", "avoid_when", "examples"} {
			provenance, ok := metadata.FieldProvenance[field]
			if !ok || !isReviewedSelectionPrecedence(provenance.Precedence) || selectedCandidateCount(provenance.Candidates) != 1 {
				problems = append(problems, canonical+" "+field+" provenance is not one reviewed_explicit/contract_final winner")
			}
			// File-based winners must come from a selection/ source file;
			// contract_final winners are declared in code (no file path).
			if ok && provenance.Precedence != selectionPrecedenceContractFinal && !strings.Contains(provenance.Source, "/selection/") {
				problems = append(problems, canonical+" "+field+" source is not selection/: "+provenance.Source)
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("selection Agent delivery invariant failed: %s", strings.Join(problems, "; "))
}

func expectedCanonicalToolPaths(opts Options) map[string]string {
	if len(opts.CanonicalToolPaths) > 0 {
		result := make(map[string]string, len(opts.CanonicalToolPaths))
		for canonical, primary := range opts.CanonicalToolPaths {
			canonical = strings.TrimSpace(canonical)
			primary = normalizeCommandPath(primary)
			if canonical != "" && primary != "" {
				result[canonical] = primary
			}
		}
		return result
	}
	result := map[string]string{}
	for canonical, primary := range opts.ToolPaths {
		canonical = strings.TrimSpace(canonical)
		if !strings.ContainsAny(canonical, " \t") && strings.Contains(canonical, ".") {
			result[canonical] = normalizeCommandPath(primary)
		}
	}
	return result
}

func selectedCandidateCount(candidates []FieldCandidateProvenance) int {
	count := 0
	for _, candidate := range candidates {
		if candidate.Selected {
			count++
		}
	}
	return count
}

// selectionHintCoverageProducts returns product IDs that still need a
// selection/ products{} row. ProductDecl-registered products are exempt.
func selectionHintCoverageProducts(productIDs map[string]bool) map[string]bool {
	if productIDs == nil {
		return nil
	}
	expected := make(map[string]bool, len(productIDs))
	for productID, include := range productIDs {
		if !include {
			continue
		}
		productID = strings.TrimSpace(productID)
		if productID == "" || contract.HasProductDecl(productID) {
			continue
		}
		expected[productID] = true
	}
	return expected
}

func expectedCanonicalToolSet(opts Options) map[string]bool {
	if len(opts.CanonicalToolPaths) > 0 {
		expected := make(map[string]bool, len(opts.CanonicalToolPaths))
		for canonical := range opts.CanonicalToolPaths {
			if canonical = strings.TrimSpace(canonical); canonical != "" {
				expected[canonical] = true
			}
		}
		return expected
	}
	if len(opts.ToolPaths) == 0 {
		return nil
	}
	expected := map[string]bool{}
	for path := range opts.ToolPaths {
		path = strings.TrimSpace(path)
		if !strings.ContainsAny(path, " \t") && strings.Contains(path, ".") {
			expected[path] = true
		}
	}
	return expected
}

func validateSelectionAuthoringContracts(opts Options) error {
	expectedTools := expectedCanonicalToolSet(opts)
	for canonical := range expectedTools {
		bound, ok := opts.BoundCommands.ByCanonical[canonical]
		if ok && contractfinal.HasRuntimeContractFinal(bound.PrimaryCommand) {
			delete(expectedTools, canonical)
		}
	}
	expectedProducts := selectionHintCoverageProducts(opts.ProductIDs)
	if selectionHintCoverageRequired(expectedProducts, expectedTools) {
		missingProducts := make([]string, 0)
		for productID, include := range expectedProducts {
			if include {
				missingProducts = append(missingProducts, productID)
			}
		}
		missingTools := make([]string, 0)
		for tool, include := range expectedTools {
			if include {
				missingTools = append(missingTools, tool)
			}
		}
		sort.Strings(missingProducts)
		sort.Strings(missingTools)
		return fmt.Errorf("ProductDecl/ContractFinal selection coverage incomplete: missing_products=%v missing_tools=%v", missingProducts, missingTools)
	}
	if len(opts.BoundCommands.Commands) == 0 {
		return nil
	}
	_, err := cli.ValidateAgentSelectionContract(opts.BoundCommands)
	return err
}
