// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0

package agentmetadata

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
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
			problems = append(problems, "missing product "+productID)
			continue
		}
		if strings.TrimSpace(metadata.AgentSummary) == "" || metadata.agentSummaryRank != selectionRankReviewedExplicit {
			problems = append(problems, productID+" agent_summary is not reviewed_explicit")
		}
		if len(metadata.UseWhen) == 0 || metadata.useWhenRank != selectionRankReviewedExplicit {
			problems = append(problems, productID+" use_when is not reviewed_explicit")
		}
		if len(metadata.AvoidWhen) == 0 || metadata.avoidWhenRank != selectionRankReviewedExplicit {
			problems = append(problems, productID+" avoid_when is not reviewed_explicit")
		}
		for _, field := range []string{"agent_summary", "use_when", "avoid_when"} {
			provenance, ok := metadata.FieldProvenance[field]
			if !ok || provenance.Precedence != selectionPrecedenceReviewedExplicit || selectedCandidateCount(provenance.Candidates) != 1 {
				problems = append(problems, productID+" "+field+" provenance is not one reviewed_explicit winner")
			}
			if ok && !strings.Contains(provenance.Source, "/selection/") {
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
	selectionRoot := filepath.Join(resolvePath(opts.Root, opts.HintsDir), "selection")
	hints, err := cli.LoadAgentHintsFromSelectionForValidation(os.DirFS(selectionRoot))
	if err != nil {
		return fmt.Errorf("load selection Agent hints: %w", err)
	}
	expectedTools := expectedCanonicalToolSet(opts)
	// Declared tools carry their selection fields in the Contract final
	// overlay (cmdcore.SchemaDecl); they are exempt from hint-file coverage.
	// Hint rows for them may still exist during migration, but are no longer
	// required and never win over the declaration.
	for canonical := range expectedTools {
		bound, ok := opts.BoundCommands.ByCanonical[canonical]
		if ok && cli.HasRuntimeContractFinal(bound.PrimaryCommand) {
			delete(expectedTools, canonical)
		}
	}
	if err := cli.ValidateManualAgentHintSet(hints, opts.ProductIDs, expectedTools); err != nil {
		return fmt.Errorf("validate selection Agent hints: %w", err)
	}
	if opts.MaxExamples > 0 {
		for canonical, hint := range hints.Tools {
			if len(hint.Examples) > opts.MaxExamples {
				return fmt.Errorf("selection Agent hint %s has %d reviewed examples, exceeding max-examples=%d", canonical, len(hint.Examples), opts.MaxExamples)
			}
		}
	}
	if err := cli.ValidateManualAgentHintExamples(opts.BoundCommands, hints); err != nil {
		return fmt.Errorf("validate selection Agent hint examples: %w", err)
	}
	if _, err := cli.ValidateManualAgentSelectionContract(opts.BoundCommands, hints); err != nil {
		return fmt.Errorf("validate selection Agent selection contract: %w", err)
	}
	return nil
}
