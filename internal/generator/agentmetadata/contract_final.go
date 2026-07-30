// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package agentmetadata

import (
	"sort"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/spf13/cobra"
)

// applyContractFinalDeclarations merges each registered Contract final
// overlay (cmdcore.SchemaDecl pass-through) into the matching tool record as
// the top-precedence candidate. Declared tools stop depending on selection
// hint files: the in-code declaration is the single final source for every
// field it carries. Non-declared tools are untouched.
//
// Runs after hint reconciliation so rank ordering decides the winner; the
// contract rank outranks every file/manual source.
func applyContractFinalDeclarations(file *File, opts Options) error {
	if file == nil || len(opts.BoundCommands.ByCanonical) == 0 {
		return nil
	}
	canonicals := make([]string, 0, len(opts.BoundCommands.ByCanonical))
	for canonical := range opts.BoundCommands.ByCanonical {
		canonicals = append(canonicals, canonical)
	}
	sort.Strings(canonicals)
	for _, canonical := range canonicals {
		bound := opts.BoundCommands.ByCanonical[canonical]
		metadata, ok := contractFinalToolMetadata(bound.PrimaryCommand)
		if !ok {
			continue
		}
		primary := normalizeCommandPath(opts.CanonicalToolPaths[canonical])
		if primary == "" {
			continue
		}
		merged, err := mergeToolMetadata(file.Tools[primary], metadata, primary)
		if err != nil {
			return err
		}
		file.Tools[primary] = merged
	}
	return nil
}

// contractFinalToolMetadata maps a registered Contract final overlay to a
// ToolMetadata candidate whose fields carry the contract_final rank/origin.
// Absent payload sections stay absent (never authored-empty); declared
// sections map field-by-field, preserving explicit empty-list authorship.
func contractFinalToolMetadata(command *cobra.Command) (ToolMetadata, bool) {
	payload, ok := cli.RuntimeContractFinal(command)
	if !ok {
		return ToolMetadata{}, false
	}
	metadata := ToolMetadata{
		AgentSummarySource: "cmdcore.SchemaDecl",
		SourceRefs:         []string{"cmdcore.SchemaDecl"},
		reviewedRank:       selectionRankContractFinal,
		reviewedOrigin:     contractFinalOrigin,
	}
	// Declarations live in reviewed code, so the Agent reviewed flag is
	// assembly-derived true (mirrors the catalog pass-through).
	reviewed := true
	metadata.Reviewed = &reviewed

	if selection := payload.Selection; selection != nil {
		if summary := strings.TrimSpace(selection.AgentSummary); summary != "" {
			metadata.AgentSummary = summary
			metadata.agentSummaryPresent = true
			metadata.agentSummaryRank = selectionRankContractFinal
			metadata.agentSummaryOrigin = contractFinalOrigin
		}
		for _, list := range []struct {
			value   []string
			out     *[]string
			present *bool
			rank    *int
			origin  *string
		}{
			{selection.UseWhen, &metadata.UseWhen, &metadata.useWhenPresent, &metadata.useWhenRank, &metadata.useWhenOrigin},
			{selection.AvoidWhen, &metadata.AvoidWhen, &metadata.avoidWhenPresent, &metadata.avoidWhenRank, &metadata.avoidWhenOrigin},
			{selection.Prerequisites, &metadata.Prerequisites, &metadata.prerequisitesPresent, &metadata.prerequisitesRank, &metadata.prerequisitesOrigin},
			{selection.Tips, &metadata.Tips, &metadata.tipsPresent, &metadata.tipsRank, &metadata.tipsOrigin},
			{selection.WorkflowRefs, &metadata.WorkflowRefs, &metadata.workflowRefsPresent, &metadata.workflowRefsRank, &metadata.workflowRefsOrigin},
			{selection.Examples, &metadata.Examples, &metadata.examplesPresent, &metadata.examplesRank, &metadata.examplesOrigin},
		} {
			if list.value == nil {
				continue
			}
			*list.out = list.value
			*list.present = true
			*list.rank = selectionRankContractFinal
			*list.origin = contractFinalOrigin
		}
	}

	if safety := payload.Safety; safety != nil {
		if effect := strings.TrimSpace(safety.Effect); effect != "" {
			metadata.Effect = effect
			metadata.EffectSource = contractFinalOrigin
			metadata.effectPresent = true
			metadata.effectRank = selectionRankContractFinal
			metadata.effectOrigin = contractFinalOrigin
		}
		if risk := strings.TrimSpace(safety.Risk); risk != "" {
			metadata.Risk = risk
			metadata.riskPresent = true
			metadata.riskRank = selectionRankContractFinal
			metadata.riskOrigin = contractFinalOrigin
		}
		if confirmation := strings.TrimSpace(safety.Confirmation); confirmation != "" {
			metadata.Confirmation = confirmation
			metadata.confirmationPresent = true
			metadata.confirmationRank = selectionRankContractFinal
			metadata.confirmationOrigin = contractFinalOrigin
		}
		if idempotency := strings.TrimSpace(safety.Idempotency); idempotency != "" {
			metadata.Idempotency = idempotency
			metadata.idempotencyPresent = true
			metadata.idempotencyRank = selectionRankContractFinal
			metadata.idempotencyOrigin = contractFinalOrigin
		}
	}

	if iface := payload.Interface; iface != nil {
		if mode := strings.TrimSpace(iface.Mode); mode != "" {
			metadata.InterfaceMode = mode
			metadata.interfaceModePresent = true
			metadata.interfaceModeRank = selectionRankContractFinal
			metadata.interfaceModeOrigin = contractFinalOrigin
		}
		if availability := strings.TrimSpace(iface.Availability); availability != "" {
			metadata.Availability = availability
			metadata.availabilityPresent = true
			metadata.availabilityRank = selectionRankContractFinal
			metadata.availabilityOrigin = contractFinalOrigin
		}
		if reason := strings.TrimSpace(iface.Reason); reason != "" {
			metadata.InterfaceReason = reason
			metadata.interfaceReasonPresent = true
			metadata.interfaceReasonRank = selectionRankContractFinal
			metadata.interfaceReasonOrigin = contractFinalOrigin
		}
		if iface.Ref != nil {
			metadata.InterfaceRef = &InterfaceRef{ProductID: iface.Ref.ProductID, RPCName: iface.Ref.RPCName}
			metadata.interfaceRefPresent = true
			metadata.interfaceRefRank = selectionRankContractFinal
			metadata.interfaceRefOrigin = contractFinalOrigin
		}
	}
	return metadata, true
}
