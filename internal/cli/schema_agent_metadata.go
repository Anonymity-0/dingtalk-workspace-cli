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

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
)

// schema_agent_metadata/*.json is retired as a shipped artifact. Production
// Agent selection / safety / interface authority is leaf ContractFinal and
// ProductDecl via RegisterSchemaSourceRoot → ResolveSchemaBuild.
// InstallBuildTimeAgentMetadataJSON is a CI/local dump helper for
// cmd_schema_catalog only; it is not a production source.

// Wire provenance / metadata_source labels (#602 Catalog contract).
// Const identifiers may be renamed; the string values must not change.
const (
	ProvenanceEmbeddedSkillMetadata = "embedded-skill-metadata"
	ProvenanceEmbeddedMCPMetadata   = "embedded-mcp-metadata"
	ProvenanceReviewedManual        = "reviewed_manual"
)

type agentMetadata struct {
	Version     int                             `json:"version"`
	SourceHash  string                          `json:"source_hash"`
	SurfaceHash string                          `json:"surface_hash,omitempty"`
	Coverage    agentMetadataCoverage           `json:"coverage"`
	Products    map[string]agentProductMetadata `json:"products"`
	Domains     []string                        `json:"domains"`
	Tools       map[string]agentToolMetadata    `json:"tools"`
}

type agentMetadataCoverage struct {
	SurfaceProducts        int `json:"surface_products,omitempty"`
	ProductsWithMetadata   int `json:"products_with_metadata"`
	SurfaceTools           int `json:"surface_tools,omitempty"`
	ToolsWithMetadata      int `json:"tools_with_metadata"`
	ToolsWithSummary       int `json:"tools_with_agent_summary,omitempty"`
	ToolsWithUseWhen       int `json:"tools_with_use_when,omitempty"`
	ToolsWithAvoidWhen     int `json:"tools_with_avoid_when,omitempty"`
	ToolsWithExamples      int `json:"tools_with_examples,omitempty"`
	ToolsWithInterfaceMode int `json:"tools_with_interface_mode,omitempty"`
	UnmatchedSkillTools    int `json:"unmatched_skill_tools,omitempty"`
	UnreviewedSkillTools   int `json:"unreviewed_skill_tools,omitempty"`
}

type agentProductMetadata struct {
	AgentSummary       string                              `json:"agent_summary,omitempty"`
	AgentSummarySource string                              `json:"agent_summary_source,omitempty"`
	UseWhen            []string                            `json:"use_when,omitempty"`
	AvoidWhen          []string                            `json:"avoid_when,omitempty"`
	SourceRefs         []string                            `json:"source_refs,omitempty"`
	FieldProvenance    map[string]contract.FieldProvenance `json:"field_provenance,omitempty"`
}

type agentToolMetadata struct {
	AgentSummary       string                              `json:"agent_summary,omitempty"`
	AgentSummarySource string                              `json:"agent_summary_source,omitempty"`
	UseWhen            []string                            `json:"use_when,omitempty"`
	AvoidWhen          []string                            `json:"avoid_when,omitempty"`
	Prerequisites      []string                            `json:"prerequisites,omitempty"`
	Tips               []string                            `json:"tips,omitempty"`
	Effect             string                              `json:"effect,omitempty"`
	EffectSource       string                              `json:"effect_source,omitempty"`
	Risk               string                              `json:"risk,omitempty"`
	Confirmation       string                              `json:"confirmation,omitempty"`
	Idempotency        string                              `json:"idempotency,omitempty"`
	WorkflowRefs       []string                            `json:"workflow_refs,omitempty"`
	Examples           []string                            `json:"examples,omitempty"`
	Reviewed           *bool                               `json:"reviewed,omitempty"`
	SourceRefs         []string                            `json:"source_refs,omitempty"`
	InterfaceRef       *embeddedMCPInterfaceRef            `json:"interface_ref,omitempty"`
	InterfaceMode      string                              `json:"interface_mode,omitempty"`
	Availability       string                              `json:"availability,omitempty"`
	InterfaceReason    string                              `json:"interface_reason,omitempty"`
	FieldProvenance    map[string]contract.FieldProvenance `json:"field_provenance,omitempty"`
}

var runtimeAgentMetadataLazy struct {
	once     sync.Once
	metadata agentMetadata
}

var runtimeAgentMetadataLazyLoadCount atomic.Uint64

var (
	buildTimeAgentMetadataMu       sync.Mutex
	buildTimeAgentMetadataOverride *agentMetadata
)

// InstallBuildTimeAgentMetadataJSON installs generator-produced Agent metadata
// for cmd_schema_catalog CI/local dump assembly only. Production binaries never
// call this; production authority remains leaf ContractFinal / ProductDecl.
// The dump helper injects an in-memory snapshot so schema_agent_metadata/ is
// neither committed nor embedded.
func InstallBuildTimeAgentMetadataJSON(data []byte) error {
	var metadata agentMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return fmt.Errorf("decode build-time Agent metadata: %w", err)
	}
	if metadata.Products == nil {
		metadata.Products = map[string]agentProductMetadata{}
	}
	if metadata.Tools == nil {
		metadata.Tools = map[string]agentToolMetadata{}
	}
	buildTimeAgentMetadataMu.Lock()
	defer buildTimeAgentMetadataMu.Unlock()
	copied := metadata
	buildTimeAgentMetadataOverride = &copied
	return nil
}

// ClearBuildTimeAgentMetadata removes a cmd_schema_catalog dump-helper injection.
func ClearBuildTimeAgentMetadata() {
	buildTimeAgentMetadataMu.Lock()
	defer buildTimeAgentMetadataMu.Unlock()
	buildTimeAgentMetadataOverride = nil
}

// runtimeAgentMetadata returns build-time injected Agent metadata when the
// CI dump helper installed one; otherwise an empty snapshot. Shipped
// binaries no longer embed schema_agent_metadata/*.json; production Agent
// facts come from ContractFinal / ProductDecl.
func runtimeAgentMetadata() agentMetadata {
	buildTimeAgentMetadataMu.Lock()
	override := buildTimeAgentMetadataOverride
	buildTimeAgentMetadataMu.Unlock()
	if override != nil {
		return *override
	}
	runtimeAgentMetadataLazy.once.Do(func() {
		runtimeAgentMetadataLazyLoadCount.Add(1)
		runtimeAgentMetadataLazy.metadata = emptyAgentMetadata()
	})
	return runtimeAgentMetadataLazy.metadata
}

func emptyAgentMetadata() agentMetadata {
	return agentMetadata{
		Products: map[string]agentProductMetadata{},
		Tools:    map[string]agentToolMetadata{},
	}
}

// agentToolContractForPathsFromMetadata is the sole typed adapter from generated Agent
// metadata to runtime contract assembly. Path resolution happens once; all
// consumers receive the same resolved safety, interface, selection and
// provenance values without performing downstream map merges.
func agentToolContractForPathsFromMetadata(source agentMetadata, paths ...string) (contract.SafetySpec, contract.InterfaceSpec, contract.SelectionSpec, map[string]contract.FieldProvenance, bool) {
	metadata, ok := lookupAgentToolMetadataFrom(source, paths...)
	if !ok {
		return contract.SafetySpec{}, contract.InterfaceSpec{}, contract.SelectionSpec{}, nil, false
	}
	safety := contract.SafetySpec{
		Effect:       strings.TrimSpace(metadata.Effect),
		EffectSource: strings.TrimSpace(metadata.EffectSource),
		Risk:         strings.TrimSpace(metadata.Risk),
		Confirmation: strings.TrimSpace(metadata.Confirmation),
		Idempotency:  strings.TrimSpace(metadata.Idempotency),
	}
	interfaceSpec := contract.InterfaceSpec{
		Mode:         strings.TrimSpace(metadata.InterfaceMode),
		Availability: strings.TrimSpace(metadata.Availability),
		Reason:       strings.TrimSpace(metadata.InterfaceReason),
	}
	if metadata.InterfaceRef != nil {
		interfaceSpec.Ref = &contract.InterfaceRefSpec{
			ProductID: strings.TrimSpace(metadata.InterfaceRef.ProductID),
			RPCName:   strings.TrimSpace(metadata.InterfaceRef.RPCName),
		}
	}
	selection := agentToolSelection(metadata)
	provenance := resolvedAgentToolProvenance(metadata.FieldProvenance, interfaceSpec, selection)
	return safety, interfaceSpec, selection, provenance, true
}

// resolvedAgentToolProvenance is the typed source adapter for generated Agent
// metadata. The generator stores concrete interface refs in its compact
// "product.rpc" identity form and stores reviewed no-ref dispositions as JSON
// null. The final typed contract stores an object or JSON null, so project the
// concrete compact identity here. Missing provenance is never synthesized:
// constructors and snapshot loaders remain validate-only and fail closed.
func resolvedAgentToolProvenance(source map[string]contract.FieldProvenance, interfaceSpec contract.InterfaceSpec, selection contract.SelectionSpec) map[string]contract.FieldProvenance {
	out := cloneFieldProvenance(source)
	if provenance, ok := out["interface_ref"]; ok {
		out["interface_ref"] = projectAgentInterfaceRefProvenance(provenance, interfaceSpec.Ref)
	}
	return out
}

func projectAgentInterfaceRefProvenance(provenance contract.FieldProvenance, ref *contract.InterfaceRefSpec) contract.FieldProvenance {
	finalValue, _ := json.Marshal(ref)
	legacy := "<none>"
	if ref != nil {
		legacy = strings.TrimSpace(ref.ProductID) + "." + strings.TrimSpace(ref.RPCName)
	}
	legacyValue, _ := json.Marshal(legacy)
	project := func(value json.RawMessage) json.RawMessage {
		if string(value) == string(legacyValue) || string(value) == string(finalValue) {
			return append(json.RawMessage(nil), finalValue...)
		}
		// Keep a disagreeing source value untouched. ToolSpec validation will
		// then fail instead of this adapter laundering a resolver conflict.
		return value
	}
	provenance.Value = project(provenance.Value)
	for index := range provenance.Candidates {
		candidate := &provenance.Candidates[index]
		if candidate.Selected != nil && *candidate.Selected {
			candidate.Value = project(candidate.Value)
		}
	}
	return provenance
}

// agentProductSelectionForIDsFromMetadata exposes generated product routing
// through the same typed contract.SelectionSpec used by ToolSpec.
func agentProductSelectionForIDsFromMetadata(source agentMetadata, ids ...string) (contract.SelectionSpec, bool) {
	selection, _, ok := agentProductContractForIDsFromMetadata(source, ids...)
	return selection, ok
}

func agentProductContractForIDsFromMetadata(source agentMetadata, ids ...string) (contract.SelectionSpec, map[string]contract.FieldProvenance, bool) {
	for _, id := range ids {
		metadata, ok := source.Products[strings.TrimSpace(id)]
		if !ok {
			continue
		}
		selection := contract.SelectionSpec{
			AgentSummary:       strings.TrimSpace(metadata.AgentSummary),
			AgentSummarySource: strings.TrimSpace(metadata.AgentSummarySource),
			UseWhen:            cloneOptionalStrings(metadata.UseWhen),
			AvoidWhen:          cloneOptionalStrings(metadata.AvoidWhen),
			SourceRefs:         cloneOptionalStrings(metadata.SourceRefs),
			MetadataSource:     ProvenanceEmbeddedSkillMetadata,
		}.Normalized()
		return selection, cloneFieldProvenance(metadata.FieldProvenance), true
	}
	return contract.SelectionSpec{}, nil, false
}

func agentToolSelection(metadata agentToolMetadata) contract.SelectionSpec {
	var reviewed *bool
	if metadata.Reviewed != nil {
		value := *metadata.Reviewed
		reviewed = &value
	}
	return contract.SelectionSpec{
		AgentSummary:       strings.TrimSpace(metadata.AgentSummary),
		AgentSummarySource: strings.TrimSpace(metadata.AgentSummarySource),
		UseWhen:            cloneOptionalStrings(metadata.UseWhen),
		AvoidWhen:          cloneOptionalStrings(metadata.AvoidWhen),
		Prerequisites:      cloneOptionalStrings(metadata.Prerequisites),
		Tips:               cloneOptionalStrings(metadata.Tips),
		WorkflowRefs:       cloneOptionalStrings(metadata.WorkflowRefs),
		Examples:           cloneOptionalStrings(metadata.Examples),
		Reviewed:           reviewed,
		SourceRefs:         cloneOptionalStrings(metadata.SourceRefs),
		MetadataSource:     ProvenanceEmbeddedSkillMetadata,
	}.Normalized()
}

func cloneFieldProvenance(source map[string]contract.FieldProvenance) map[string]contract.FieldProvenance {
	if len(source) == 0 {
		return nil
	}
	out := make(map[string]contract.FieldProvenance, len(source))
	for field, provenance := range source {
		provenance.Value = append(json.RawMessage(nil), provenance.Value...)
		provenance.Candidates = cloneFieldCandidates(provenance.Candidates)
		provenance.OverriddenCandidates = cloneFieldCandidates(provenance.OverriddenCandidates)
		out[field] = provenance
	}
	return out
}

func cloneFieldCandidates(source []contract.FieldCandidateProvenance) []contract.FieldCandidateProvenance {
	if len(source) == 0 {
		return nil
	}
	out := make([]contract.FieldCandidateProvenance, len(source))
	for index, candidate := range source {
		candidate.Value = append(json.RawMessage(nil), candidate.Value...)
		if candidate.Selected != nil {
			value := *candidate.Selected
			candidate.Selected = &value
		}
		out[index] = candidate
	}
	return out
}

func lookupAgentToolMetadataFrom(source agentMetadata, paths ...string) (agentToolMetadata, bool) {
	seen := map[string]bool{}
	for _, path := range paths {
		for _, candidate := range []string{
			strings.TrimSpace(path),
			strings.Join(splitSchemaPathTokens(path), " "),
		} {
			if candidate == "" || seen[candidate] {
				continue
			}
			seen[candidate] = true
			if metadata, ok := source.Tools[candidate]; ok {
				return metadata, true
			}
		}
	}
	return agentToolMetadata{}, false
}

func agentMetadataSummaryFrom(metadata agentMetadata) map[string]any {
	summary := map[string]any{
		"source":                 ProvenanceEmbeddedSkillMetadata,
		"version":                metadata.Version,
		"source_hash":            strings.TrimSpace(metadata.SourceHash),
		"products_with_metadata": len(metadata.Products),
		"tools_with_metadata":    len(metadata.Tools),
	}
	if metadata.SurfaceHash != "" {
		summary["surface_hash"] = metadata.SurfaceHash
	}
	coverage := metadata.Coverage
	if coverage.SurfaceProducts > 0 {
		summary["surface_products"] = coverage.SurfaceProducts
	}
	if coverage.SurfaceTools > 0 {
		summary["surface_tools"] = coverage.SurfaceTools
	}
	if coverage.ToolsWithSummary > 0 {
		summary["tools_with_agent_summary"] = coverage.ToolsWithSummary
	}
	if coverage.UnmatchedSkillTools > 0 {
		summary["unmatched_skill_tools"] = coverage.UnmatchedSkillTools
	}
	return summary
}

// agentMetadataSummaryFromProducts publishes Catalog-level Agent coverage from
// the assembled Schema surface (ContractFinal / ProductDecl). This keeps
// runtime delivery and CI dumps hash-aligned without requiring build-time
// Agent metadata inject as a published summary source.
func agentMetadataSummaryFromProducts(products []ProductSpec) map[string]any {
	productsWith := 0
	toolsWith := 0
	toolsWithSummary := 0
	surfaceTools := 0
	for _, product := range products {
		if productHasPublishedAgentMetadata(product) {
			productsWith++
		}
		for _, tool := range product.Tools {
			surfaceTools++
			if toolHasPublishedAgentMetadata(tool) {
				toolsWith++
			}
			if strings.TrimSpace(tool.Selection.AgentSummary) != "" {
				toolsWithSummary++
			}
		}
	}
	summary := map[string]any{
		"source":                 ProvenanceEmbeddedSkillMetadata,
		"version":                1,
		"source_hash":            "",
		"products_with_metadata": productsWith,
		"tools_with_metadata":    toolsWith,
	}
	if len(products) > 0 {
		summary["surface_products"] = len(products)
	}
	if surfaceTools > 0 {
		summary["surface_tools"] = surfaceTools
	}
	if toolsWithSummary > 0 {
		summary["tools_with_agent_summary"] = toolsWithSummary
	}
	return summary
}

func productHasPublishedAgentMetadata(product ProductSpec) bool {
	return strings.TrimSpace(product.Selection.AgentSummary) != "" ||
		len(product.Selection.UseWhen) > 0 ||
		len(product.Selection.AvoidWhen) > 0
}

func toolHasPublishedAgentMetadata(tool ToolSpec) bool {
	return strings.TrimSpace(tool.Selection.AgentSummary) != "" ||
		len(tool.Selection.UseWhen) > 0 ||
		len(tool.Selection.AvoidWhen) > 0 ||
		len(tool.Selection.Examples) > 0
}
