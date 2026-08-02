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
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli/runtimeannotate"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

//go:embed schema_mcp_metadata.json
var embeddedMCPMetadataJSON []byte

const (
	// Annotation keys are owned by runtimeannotate; aliases keep assembly readers stable.
	runtimeSchemaProductAnnotation          = runtimeannotate.AnnotationProduct
	runtimeSchemaToolAnnotation             = runtimeannotate.AnnotationTool
	runtimeSchemaSourceAnnotation           = runtimeannotate.AnnotationSource
	runtimeSchemaTitleAnnotation            = runtimeannotate.AnnotationTitle
	runtimeSchemaDescAnnotation             = runtimeannotate.AnnotationDescription
	runtimeSchemaMetaAnnotation             = runtimeannotate.AnnotationMetaSource
	runtimeSchemaExcludeAnnotation          = runtimeannotate.AnnotationExclude
	runtimeSchemaRulesAnnotation            = runtimeannotate.AnnotationConstraints
	runtimeSchemaArgsAnnotation             = runtimeannotate.AnnotationPositionals
	runtimeSchemaFlagPropertyAnnotation     = runtimeannotate.AnnotationFlagProperty
	runtimeSchemaFlagTypeAnnotation         = runtimeannotate.AnnotationFlagType
	runtimeSchemaFlagDescriptionAnnotation  = runtimeannotate.AnnotationDescription
	runtimeSchemaFlagRequiredAnnotation     = runtimeannotate.AnnotationFlagRequired
	runtimeSchemaFlagRequiredWhenAnnotation = runtimeannotate.AnnotationFlagReqWhen
	runtimeSchemaFlagExampleAnnotation      = runtimeannotate.AnnotationFlagExample
	runtimeSchemaContractAnnotation         = runtimeannotate.AnnotationContract
	runtimeSchemaRiskAnnotation             = runtimeannotate.AnnotationRisk
	runtimeSchemaRuntimeGateAnnotation      = runtimeannotate.AnnotationRuntimeGate
)

type embeddedMCPMetadata struct {
	Version        int                                `json:"version"`
	Source         string                             `json:"source"`
	SourceRevision string                             `json:"source_revision,omitempty"`
	SourceHash     string                             `json:"source_hash"`
	Coverage       embeddedMCPMetadataCoverage        `json:"coverage,omitempty"`
	Tools          map[string]embeddedMCPToolMetadata `json:"tools"`
}

type embeddedMCPMetadataCoverage struct {
	SurfaceScope     string   `json:"surface_scope,omitempty"`
	SourceServices   int      `json:"source_services,omitempty"`
	SnapshotServices int      `json:"snapshot_services,omitempty"`
	MissingServices  []string `json:"missing_services,omitempty"`
	SourceTools      int      `json:"source_tools,omitempty"`
	SurfaceTools     int      `json:"surface_tools,omitempty"`
	MatchedTools     int      `json:"matched_tools,omitempty"`
	AliasedTools     int      `json:"aliased_tools,omitempty"`
	UnmatchedTools   int      `json:"unmatched_tools,omitempty"`
}

type embeddedMCPInterfaceRef struct {
	ProductID string `json:"product_id"`
	RPCName   string `json:"rpc_name"`
}

type embeddedMCPToolMetadata struct {
	Title        string                          `json:"title,omitempty"`
	Description  string                          `json:"description,omitempty"`
	Parameters   map[string]embeddedMCPParamMeta `json:"parameters,omitempty"`
	InterfaceRef *embeddedMCPInterfaceRef        `json:"interface_ref,omitempty"`
}

type embeddedMCPParamMeta struct {
	Type         string   `json:"type,omitempty"`
	Description  string   `json:"description,omitempty"`
	Default      string   `json:"default,omitempty"`
	Format       string   `json:"format,omitempty"`
	Enum         []string `json:"enum,omitempty"`
	Required     *bool    `json:"required,omitempty"`
	RequiredWhen string   `json:"required_when,omitempty"`
}

var runtimeEmbeddedMCPMetadataLazy struct {
	once     sync.Once
	metadata embeddedMCPMetadata
}

var runtimeEmbeddedMCPMetadataLazyLoadCount atomic.Uint64

var runtimeSchemaConstraintsByCanonical = map[string]RuntimeSchemaConstraints{}

// RegisterRuntimeSchemaConstraints records reviewed cross-parameter CLI rules
// independently from the generated Catalog, preventing stale Catalog data
// from becoming the source of its own next generation.
func RegisterRuntimeSchemaConstraints(canonicalPath string, constraints RuntimeSchemaConstraints) {
	canonicalPath = strings.TrimSpace(canonicalPath)
	constraints = normalizeRuntimeSchemaConstraints(constraints)
	if canonicalPath == "" || runtimeSchemaConstraintsEmpty(constraints) {
		return
	}
	runtimeSchemaConstraintsByCanonical[canonicalPath] = constraints
}

func loadEmbeddedMCPMetadata() embeddedMCPMetadata {
	var metadata embeddedMCPMetadata
	if err := json.Unmarshal(embeddedMCPMetadataJSON, &metadata); err != nil {
		return embeddedMCPMetadata{Tools: map[string]embeddedMCPToolMetadata{}}
	}
	if metadata.Tools == nil {
		metadata.Tools = map[string]embeddedMCPToolMetadata{}
	}
	return metadata
}

// runtimeMCPMetadata parses the pinned interface snapshot only when Schema
// assembly first requests it. Normal command construction and execution do
// not cross this boundary.
func runtimeMCPMetadata() embeddedMCPMetadata {
	runtimeEmbeddedMCPMetadataLazy.once.Do(func() {
		runtimeEmbeddedMCPMetadataLazyLoadCount.Add(1)
		runtimeEmbeddedMCPMetadataLazy.metadata = loadEmbeddedMCPMetadata()
	})
	return runtimeEmbeddedMCPMetadataLazy.metadata
}

func interfaceMetadataSummaryFrom(metadata embeddedMCPMetadata) map[string]any {
	summary := map[string]any{
		"source":      strings.TrimSpace(metadata.Source),
		"version":     metadata.Version,
		"source_hash": strings.TrimSpace(metadata.SourceHash),
		"tool_count":  len(metadata.Tools),
	}
	if revision := strings.TrimSpace(metadata.SourceRevision); revision != "" {
		summary["source_revision"] = revision
	}
	if metadata.Coverage.SurfaceTools > 0 {
		summary["coverage"] = metadata.Coverage
	}
	return summary
}

// applyContractRiskToSafety overlays Schema Safety fields from an embedded
// Contract Risk value. Path A: Contract wins effect/risk/confirmation for the
// managed surface; other Safety fields (e.g. idempotency) are preserved.
func applyContractRiskToSafety(base contract.SafetySpec, contractRisk string) contract.SafetySpec {
	out := base
	switch strings.TrimSpace(contractRisk) {
	case "write":
		out.Effect = "write"
		out.EffectSource = "corecmd.contract"
		out.Risk = "medium"
		out.Confirmation = "user_required"
	case "high-risk-write":
		out.Effect = "destructive"
		out.EffectSource = "corecmd.contract"
		out.Risk = "high"
		out.Confirmation = "user_required"
	case "read":
		out.Effect = "read"
		out.EffectSource = "corecmd.contract"
		out.Risk = "low"
		out.Confirmation = "not_required"
	}
	return out
}

// applyContractGateToSafety ensures a write-guard annotation cannot leave Schema
// claiming confirmation is not required. Reviewed effect/risk are kept when set.
func applyContractGateToSafety(base contract.SafetySpec, gate string) contract.SafetySpec {
	out := base
	if strings.TrimSpace(gate) == "" {
		return out
	}
	out.Confirmation = "user_required"
	if out.Effect == "" || out.Effect == "read" {
		out.Effect = "write"
		out.EffectSource = "corecmd.contract_gate"
	}
	if out.Risk == "" || out.Risk == "low" {
		out.Risk = "medium"
	}
	return out
}

type runtimeSchemaEntry struct {
	ProductID       string
	SourceProductID string
	ProductName     string
	ToolName        string
	CLIName         string
	Group           string
	CLIPath         string
	Title           string
	Description     string
	Source          string
	MetadataSource  string
	Command         *cobra.Command
	PrimaryCLIPath  string
	Aliases         []string
	IsAlias         bool
	IdentityField   contract.FieldProvenance
}

func collectRuntimeSchemaEntries(root *cobra.Command) ([]runtimeSchemaEntry, error) {
	effective, err := BuildEffectiveCommandRegistry(root)
	if err != nil {
		return nil, err
	}
	bound, err := BindEffectiveCommandRegistry(root, effective)
	if err != nil {
		return nil, err
	}
	return collectRuntimeSchemaEntriesFromBound(bound)
}

// collectRuntimeSchemaEntriesFromBound is the sole identity hand-off into the
// Schema assembler. It never scans annotations to discover commands: the
// reviewed registry has already selected the exact command set and the binder
// has already proved that every path resolves to a runnable Cobra leaf.
func collectRuntimeSchemaEntriesFromBound(bound BoundCommandRegistry) ([]runtimeSchemaEntry, error) {
	entries := make([]runtimeSchemaEntry, 0, len(bound.Commands))
	for _, command := range bound.Commands {
		productID, toolName, ok := splitManualSchemaCanonicalPath(command.CanonicalPath)
		if !ok {
			return nil, fmt.Errorf("bound Schema command has invalid canonical path %q", command.CanonicalPath)
		}
		if command.Visibility != SchemaVisibilityPublic {
			continue
		}
		leaf := command.PrimaryCommand
		AnnotateRuntimeConstraints(leaf, runtimeSchemaConstraintsByCanonical[command.CanonicalPath])

		parts := splitSchemaPathTokens(command.PrimaryCLIPath)
		group := ""
		if len(parts) > 2 {
			group = strings.Join(parts[1:len(parts)-1], ".")
		}
		productName := ""
		if top := topLevelCommand(leaf); top != nil {
			productName = strings.TrimSpace(top.Short)
		}
		entries = append(entries, runtimeSchemaEntry{
			ProductID:       productID,
			SourceProductID: command.SourceProductID,
			ProductName:     productName,
			ToolName:        toolName,
			CLIName:         leaf.Name(),
			Group:           group,
			CLIPath:         command.PrimaryCLIPath,
			Title:           runtimeCommandTitle(leaf),
			Description:     runtimeCommandDescription(leaf),
			Source:          command.Source,
			MetadataSource:  runtimeCommandMetadataSource(leaf),
			Command:         leaf,
			PrimaryCLIPath:  command.PrimaryCLIPath,
			Aliases:         append([]string(nil), command.Aliases...),
			IdentityField:   commandRegistryIdentityProvenance(command),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ProductID != entries[j].ProductID {
			return entries[i].ProductID < entries[j].ProductID
		}
		if entries[i].ToolName != entries[j].ToolName {
			return entries[i].ToolName < entries[j].ToolName
		}
		return entries[i].CLIPath < entries[j].CLIPath
	})
	return entries, nil
}

func embeddedMCPMetadataForEntryFrom(entry runtimeSchemaEntry, agentMetadata embeddedAgentMetadata, mcpMetadata embeddedMCPMetadata) (embeddedMCPToolMetadata, bool) {
	// Production authority: ContractFinal Interface.Ref remaps CLI canonical
	// names onto pinned MCP tool keys (e.g. reply_personal_message →
	// chat.send_personal_message). Agent-metadata inject is CI-dump only.
	if entry.Command != nil {
		if final, ok := RuntimeContractFinal(entry.Command); ok && final.Interface != nil && final.Interface.Ref != nil {
			if metadata, found := mcpMetadataForInterfaceRef(mcpMetadata, final.Interface.Ref.ProductID, final.Interface.Ref.RPCName); found {
				return metadata, true
			}
		}
	}
	paths := []string{
		entry.PrimaryCLIPath,
		entry.CLIPath,
		entry.ProductID + "." + entry.ToolName,
	}
	paths = append(paths, entry.Aliases...)
	if toolMetadata, ok := lookupAgentToolMetadataFrom(agentMetadata, paths...); ok && toolMetadata.InterfaceRef != nil {
		if metadata, found := mcpMetadataForInterfaceRef(mcpMetadata, toolMetadata.InterfaceRef.ProductID, toolMetadata.InterfaceRef.RPCName); found {
			return metadata, true
		}
	}
	for _, key := range []string{
		entry.SourceProductID + "." + entry.ToolName,
		entry.ProductID + "." + entry.ToolName,
	} {
		key = strings.Trim(key, ".")
		if key == "" {
			continue
		}
		if meta, ok := mcpMetadata.Tools[key]; ok {
			return meta, true
		}
	}
	return embeddedMCPToolMetadata{}, false
}

func mcpMetadataForInterfaceRef(mcpMetadata embeddedMCPMetadata, productID, rpcName string) (embeddedMCPToolMetadata, bool) {
	productID = strings.TrimSpace(productID)
	rpcName = strings.TrimSpace(rpcName)
	key := strings.Trim(productID+"."+rpcName, ".")
	if key == "" {
		return embeddedMCPToolMetadata{}, false
	}
	metadata, exists := mcpMetadata.Tools[key]
	if !exists {
		return embeddedMCPToolMetadata{}, false
	}
	metadata.InterfaceRef = &embeddedMCPInterfaceRef{ProductID: productID, RPCName: rpcName}
	return metadata, true
}

func runtimeSchemaAnnotations(cmd *cobra.Command) (productID, toolName, source string) {
	if cmd == nil || cmd.Annotations == nil {
		return "", "", ""
	}
	productID = strings.TrimSpace(cmd.Annotations[runtimeSchemaProductAnnotation])
	toolName = strings.TrimSpace(cmd.Annotations[runtimeSchemaToolAnnotation])
	source = strings.TrimSpace(cmd.Annotations[runtimeSchemaSourceAnnotation])
	if source == "" && productID != "" {
		source = "runtime:" + productID
	}
	return productID, toolName, source
}

func runtimeSchemaExcluded(cmd *cobra.Command) bool {
	return cmd != nil && cmd.Annotations != nil &&
		strings.EqualFold(strings.TrimSpace(cmd.Annotations[runtimeSchemaExcludeAnnotation]), "true")
}

func runtimeCommandTitle(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	if cmd.Annotations != nil {
		if title := strings.TrimSpace(cmd.Annotations[runtimeSchemaTitleAnnotation]); title != "" {
			return title
		}
	}
	return strings.TrimSpace(cmd.Short)
}

func runtimeCommandDescription(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	if cmd.Annotations != nil {
		if desc := strings.TrimSpace(cmd.Annotations[runtimeSchemaDescAnnotation]); desc != "" {
			return desc
		}
	}
	if desc := strings.TrimSpace(cmd.Long); desc != "" {
		return desc
	}
	return strings.TrimSpace(cmd.Short)
}

func runtimeCommandMetadataSource(cmd *cobra.Command) string {
	if cmd == nil || cmd.Annotations == nil {
		return ""
	}
	return strings.TrimSpace(cmd.Annotations[runtimeSchemaMetaAnnotation])
}

func commandPathParts(cmd *cobra.Command) []string {
	parts := []string{}
	for c := cmd; c != nil && c.HasParent(); c = c.Parent() {
		parts = append([]string{c.Name()}, parts...)
	}
	return parts
}

func topLevelCommand(cmd *cobra.Command) *cobra.Command {
	var top *cobra.Command
	for c := cmd; c != nil && c.HasParent(); c = c.Parent() {
		top = c
	}
	return top
}

func schemaProductToolCount(product map[string]any) int {
	switch value := product["tool_count"].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	}
	if tools, ok := product["tools"].([]map[string]any); ok {
		return len(tools)
	}
	if tools, ok := product["tools"].([]any); ok {
		return len(tools)
	}
	return 0
}

func runtimeToolTextMetadataFromMetadata(entry runtimeSchemaEntry, metadata runtimeSchemaMetadataSources) (title, description, metadataSource string, provenance map[string]contract.FieldProvenance, err error) {
	baseSource := "cobra_help"
	baseTitle := runtimeSchemaStringCandidate(entry.Title, baseSource)
	baseDescription := runtimeSchemaStringCandidate(entry.Description, baseSource)
	if strings.TrimSpace(entry.MetadataSource) != "" {
		baseSource = strings.TrimSpace(entry.MetadataSource)
		baseTitle = runtimeSchemaStringCandidateAtRank(entry.Title, baseSource, runtimeSchemaRankNativeAnnotation, "native_annotation")
		baseDescription = runtimeSchemaStringCandidateAtRank(entry.Description, baseSource, runtimeSchemaRankNativeAnnotation, "native_annotation")
	}
	titleCandidates := []runtimeSchemaFieldCandidate{baseTitle}
	descriptionCandidates := []runtimeSchemaFieldCandidate{baseDescription}
	embeddedMeta, hasEmbeddedMeta := embeddedMCPMetadataForEntryFrom(entry, metadata.Agent, metadata.MCP)
	if hasEmbeddedMeta {
		titleCandidates = append(titleCandidates, runtimeSchemaStringCandidate(embeddedMeta.Title, "mcp_metadata"))
		descriptionCandidates = append(descriptionCandidates, runtimeSchemaStringCandidate(embeddedMeta.Description, "mcp_metadata"))
	}
	titleWinner, err := resolveRuntimeSchemaField("title", titleCandidates...)
	if err != nil {
		return "", "", "", nil, err
	}
	descriptionWinner, err := resolveRuntimeSchemaField("description", descriptionCandidates...)
	if err != nil {
		return "", "", "", nil, err
	}
	title, _ = titleWinner.Value.(string)
	description, _ = descriptionWinner.Value.(string)
	selectedSource := descriptionWinner.Source
	if selectedSource == "" {
		selectedSource = titleWinner.Source
	}
	switch selectedSource {
	case "mcp_metadata", "pinned_mcp_metadata":
		metadataSource = "embedded-mcp-metadata"
	case "cobra_help":
		metadataSource = ""
	default:
		metadataSource = strings.TrimSpace(selectedSource)
	}
	provenance = map[string]contract.FieldProvenance{}
	if titleWinner.Present {
		provenance["title"] = runtimeSchemaFieldProvenance(titleWinner)
	}
	if descriptionWinner.Present {
		provenance["description"] = runtimeSchemaFieldProvenance(descriptionWinner)
	}
	if metadataSource != "" {
		provenance["metadata_source"] = runtimeSchemaFieldProvenance(runtimeSchemaStringCandidate(metadataSource, "metadata_source_resolution"))
	}
	return strings.TrimSpace(title), strings.TrimSpace(description), strings.TrimSpace(metadataSource), provenance, nil
}

type runtimeSchemaFieldCandidate struct {
	Value        any
	Present      bool
	Source       string
	Rank         int
	Precedence   string
	Resolution   string
	ReviewReason string
	Compared     []runtimeSchemaFieldCandidate
}

const (
	runtimeSchemaRankDefault          = 0
	runtimeSchemaRankDerived          = 50
	runtimeSchemaRankInference        = 100
	runtimeSchemaRankMCP              = 400
	runtimeSchemaRankCobraHelp        = 450
	runtimeSchemaRankCobraDefault     = 600
	runtimeSchemaRankCobraContract    = 610
	runtimeSchemaRankNativeAnnotation = 620
	runtimeSchemaRankTypedMetadata    = 630
	runtimeSchemaRankConstraint       = 640
	runtimeSchemaRankVersionedBinding = 650
	// ParamDecl.Property (dws.schema.property) outranks residual versioned
	// binding candidates (active bindings JSON is empty after Phase 2).
	// Mapping exclusions stay highest so an explicit "no MCP property" review
	// cannot be overridden by a leaf ParamDecl that still carries a Property.
	runtimeSchemaRankParamDeclProperty = 655
	runtimeSchemaRankMappingExclusion  = 660

	runtimeSchemaPrecedenceDefault          = "default"
	runtimeSchemaPrecedenceDerived          = "derived_resolution"
	runtimeSchemaPrecedenceInference        = "inference"
	runtimeSchemaPrecedenceMCP              = "mcp_metadata"
	runtimeSchemaPrecedenceCobraHelp        = "cobra_help"
	runtimeSchemaPrecedenceCobra            = "cobra_contract"
	runtimeSchemaPrecedenceNativeAnnotation = "native_annotation"
	runtimeSchemaPrecedenceTypedMetadata    = "typed_metadata"
	runtimeSchemaPrecedenceConstraint       = "command_constraint"
	runtimeSchemaPrecedenceVersionedBinding = "versioned_binding"
	runtimeSchemaPrecedenceMappingExclusion = "reviewed_mapping_exclusion"
)

func runtimeSchemaCandidate(value any, present bool, source string) runtimeSchemaFieldCandidate {
	rank, precedence := runtimeSchemaSourcePriority(source)
	return runtimeSchemaStringCandidateAtPriority(value, present, source, rank, precedence)
}

func runtimeSchemaStringCandidateAtRank(value any, source string, rank int, precedence string) runtimeSchemaFieldCandidate {
	present := true
	if text, ok := value.(string); ok {
		value = strings.TrimSpace(text)
		present = value != ""
	}
	return runtimeSchemaStringCandidateAtPriority(value, present, source, rank, precedence)
}

func runtimeSchemaStringCandidateAtPriority(value any, present bool, source string, rank int, precedence string) runtimeSchemaFieldCandidate {
	return runtimeSchemaFieldCandidate{
		Value:      value,
		Present:    present,
		Source:     strings.TrimSpace(source),
		Rank:       rank,
		Precedence: strings.TrimSpace(precedence),
		Resolution: "highest_precedence",
	}
}

// resolveRuntimeSchemaCandidate is the only scalar resolver used while
// assembling the typed runtime contract. Call-site order is deliberately
// irrelevant: source rank selects the winner, values never do, and two
// disagreeing candidates at the same rank fail closed instead of silently
// depending on merge order.
func resolveRuntimeSchemaCandidate(field string, candidates ...runtimeSchemaFieldCandidate) (runtimeSchemaFieldCandidate, error) {
	present := make([]runtimeSchemaFieldCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Present {
			candidate.Compared = nil
			present = append(present, candidate)
		}
	}
	if len(present) == 0 {
		return runtimeSchemaFieldCandidate{}, nil
	}
	// Sort the complete candidate set, not just the winning rank. This keeps
	// both winner selection and diagnostics independent from call-site order.
	sort.Slice(present, func(i, j int) bool {
		left, right := present[i], present[j]
		if left.Rank != right.Rank {
			return left.Rank > right.Rank
		}
		if left.Source != right.Source {
			return left.Source < right.Source
		}
		if left.Precedence != right.Precedence {
			return left.Precedence < right.Precedence
		}
		leftValue, _ := json.Marshal(left.Value)
		rightValue, _ := json.Marshal(right.Value)
		if comparison := bytes.Compare(leftValue, rightValue); comparison != 0 {
			return comparison < 0
		}
		return left.ReviewReason < right.ReviewReason
	})

	// A higher-ranked winner must not hide an internally contradictory lower
	// rank. Every precedence layer is a reviewed contract in its own right, so
	// conflicting scalar values at any rank fail closed.
	for start := 0; start < len(present); {
		end := start + 1
		for end < len(present) && present[end].Rank == present[start].Rank {
			end++
		}
		reference := present[start]
		for _, candidate := range present[start+1 : end] {
			if !reflect.DeepEqual(candidate.Value, reference.Value) {
				referenceValue, _ := json.Marshal(reference.Value)
				candidateValue, _ := json.Marshal(candidate.Value)
				return runtimeSchemaFieldCandidate{}, fmt.Errorf(
					"%s has conflicting equal-precedence sources %s=%s and %s=%s",
					strings.TrimSpace(field), reference.Source, referenceValue, candidate.Source, candidateValue,
				)
			}
		}
		start = end
	}

	winner := present[0]
	winner.Compared = present
	return winner, nil
}

// resolveRuntimeSchemaField keeps field resolution replaceable in focused
// tests without changing the deterministic production resolver.
var resolveRuntimeSchemaField = resolveRuntimeSchemaCandidate

// resolveRequiredProjection merges required candidates with a Cobra hard-required
// floor. Other Schema fields still use value-neutral resolveRuntimeSchemaCandidate;
// required alone cannot project optional when the executable flag is MarkFlagRequired.
func resolveRequiredProjection(cobraHard bool, candidates ...runtimeSchemaFieldCandidate) (runtimeSchemaFieldCandidate, error) {
	winner, err := resolveRuntimeSchemaField("required", candidates...)
	if err != nil {
		return runtimeSchemaFieldCandidate{}, err
	}
	if !cobraHard {
		return winner, nil
	}
	if required, _ := winner.Value.(bool); required {
		return winner, nil
	}

	floor := runtimeSchemaCandidate(true, true, "cobra_hard_required")
	floor.Resolution = "cobra_hard_required_floor"
	compared := make([]runtimeSchemaFieldCandidate, 0, len(winner.Compared)+1)
	compared = append(compared, floor)
	for _, candidate := range winner.Compared {
		if candidate.Source == "cobra_hard_required" {
			continue
		}
		copyCandidate := candidate
		copyCandidate.Compared = nil
		compared = append(compared, copyCandidate)
	}
	floor.Compared = compared
	return floor, nil
}

func runtimeSchemaFieldProvenance(candidate runtimeSchemaFieldCandidate) contract.FieldProvenance {
	if !candidate.Present {
		return contract.FieldProvenance{}
	}
	value, err := json.Marshal(candidate.Value)
	if err != nil {
		value = json.RawMessage("null")
	}
	provenance := contract.FieldProvenance{
		Value:        value,
		Source:       candidate.Source,
		Precedence:   candidate.Precedence,
		Resolution:   candidate.Resolution,
		ReviewReason: candidate.ReviewReason,
	}
	compared := candidate.Compared
	if len(compared) == 0 {
		copyCandidate := candidate
		copyCandidate.Compared = nil
		compared = []runtimeSchemaFieldCandidate{copyCandidate}
	}
	provenance.Candidates = make([]contract.FieldCandidateProvenance, 0, len(compared))
	for idx, item := range compared {
		selected := idx == 0
		value, err := json.Marshal(item.Value)
		if err != nil {
			// Runtime Schema candidates are closed scalar values (string/bool).
			// Keep provenance structurally valid if a future adapter violates
			// that contract; the resolved typed field remains authoritative.
			value = json.RawMessage("null")
		}
		provenance.Candidates = append(provenance.Candidates, contract.FieldCandidateProvenance{
			Value:        value,
			Source:       item.Source,
			Precedence:   item.Precedence,
			ReviewReason: item.ReviewReason,
			Selected:     &selected,
		})
	}
	return provenance
}

func runtimeSchemaSourcePriority(source string) (int, string) {
	switch strings.TrimSpace(source) {
	case "reviewed_mapping_exclusion":
		return runtimeSchemaRankMappingExclusion, runtimeSchemaPrecedenceMappingExclusion
	case "require_one_of_constraint":
		return runtimeSchemaRankConstraint, runtimeSchemaPrecedenceConstraint
	case "versioned_parameter_binding":
		return runtimeSchemaRankVersionedBinding, runtimeSchemaPrecedenceVersionedBinding
	case "typed_parameter_metadata":
		return runtimeSchemaRankTypedMetadata, runtimeSchemaPrecedenceTypedMetadata
	case "native_annotation":
		return runtimeSchemaRankNativeAnnotation, runtimeSchemaPrecedenceNativeAnnotation
	case "cobra_hard_required", "cobra_nonzero_default", "cobra_flag_type", "cobra_usage":
		if strings.TrimSpace(source) == "cobra_nonzero_default" {
			return runtimeSchemaRankCobraDefault, runtimeSchemaPrecedenceCobra
		}
		return runtimeSchemaRankCobraContract, runtimeSchemaPrecedenceCobra
	case "mcp_metadata", "pinned_mcp_metadata":
		return runtimeSchemaRankMCP, runtimeSchemaPrecedenceMCP
	case "cobra_help":
		return runtimeSchemaRankCobraHelp, runtimeSchemaPrecedenceCobraHelp
	case "flag_name_inference", "usage_required_inference", "usage_format_inference":
		return runtimeSchemaRankInference, runtimeSchemaPrecedenceInference
	case "metadata_source_resolution":
		return runtimeSchemaRankDerived, runtimeSchemaPrecedenceDerived
	case "default", "effect-default", "risk-default":
		return runtimeSchemaRankDefault, runtimeSchemaPrecedenceDefault
	default:
		return runtimeSchemaRankDerived, "source_order"
	}
}

func runtimeSchemaStringCandidate(value, source string) runtimeSchemaFieldCandidate {
	value = strings.TrimSpace(value)
	return runtimeSchemaCandidate(value, value != "", source)
}

func runtimeSchemaAnnotatedBoolCandidate(flag *pflag.Flag, annotation, source string) runtimeSchemaFieldCandidate {
	raw := firstFlagAnnotation(flag, annotation)
	if raw == "" {
		return runtimeSchemaFieldCandidate{}
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return runtimeSchemaFieldCandidate{}
	}
	return runtimeSchemaCandidate(value, true, source)
}

func runtimeFlagCobraHardRequired(flag *pflag.Flag) bool {
	return flag != nil && len(flag.Annotations[cobra.BashCompOneRequiredFlag]) > 0
}

func runtimeSchemaParameterMappingKey(canonicalPath, flagName string) string {
	return strings.TrimSpace(canonicalPath) + " --" + strings.TrimSpace(flagName)
}

// runtimeSchemaParameterMappingCandidates resolves the two reviewed,
// versioned property-mapping inputs. An exclusion is an explicit statement
// that the CLI parameter is not a direct MCP property: it therefore supplies
// a present empty candidate (rather than allowing name inference to survive)
// and keeps the review reason in provenance.
func runtimeSchemaParameterMappingCandidates(snapshot schemaParameterBindingSnapshot, canonicalPath, flagName string) (runtimeSchemaFieldCandidate, runtimeSchemaFieldCandidate, error) {
	binding := strings.TrimSpace(snapshot.Bindings[strings.TrimSpace(canonicalPath)][strings.TrimSpace(flagName)])
	bindingCandidate := runtimeSchemaStringCandidate(binding, "versioned_parameter_binding")
	reason, excluded := snapshot.MappingExclusions[runtimeSchemaParameterMappingKey(canonicalPath, flagName)]
	if !excluded {
		return bindingCandidate, runtimeSchemaFieldCandidate{}, nil
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return runtimeSchemaFieldCandidate{}, runtimeSchemaFieldCandidate{}, fmt.Errorf("reviewed mapping exclusion has no reason")
	}
	if binding != "" {
		return runtimeSchemaFieldCandidate{}, runtimeSchemaFieldCandidate{}, fmt.Errorf("versioned binding %q conflicts with reviewed mapping exclusion", binding)
	}
	exclusionCandidate := runtimeSchemaCandidate("", true, "reviewed_mapping_exclusion")
	exclusionCandidate.ReviewReason = reason
	return runtimeSchemaFieldCandidate{}, exclusionCandidate, nil
}

// runtimeCommandParameterSpecs resolves every source into the typed contract
// model. Most fields use value-neutral source precedence: a higher-priority
// source may intentionally raise or lower type/mapping/description semantics.
// required is different: Cobra MarkFlagRequired is a hard floor that cannot be
// lowered by manual/hint overlays (see resolveRequiredProjection).
func runtimeCommandParameterSpecs(cmd *cobra.Command, canonicalPath string, embeddedParams map[string]embeddedMCPParamMeta, constraints RuntimeSchemaConstraints) ([]ParameterSpec, error) {
	if cmd == nil {
		return nil, nil
	}
	params := make([]ParameterSpec, 0)
	var resolveErr error
	metadata := runtimeSchemaParameterMetadataByCanonical[canonicalPath]
	bindingSnapshot, err := schemaParameterBindingData()
	if err != nil {
		return nil, fmt.Errorf("load reviewed Schema parameter bindings: %w", err)
	}
	inherited := metadata.Inherited
	visitRuntimeCommandFlags(cmd, inherited, func(flag *pflag.Flag) {
		if resolveErr != nil || flag == nil || flag.Hidden || flag.Name == "help" || isGenericPayloadFlag(flag) {
			return
		}
		bindingProperty, excludedProperty, err := runtimeSchemaParameterMappingCandidates(bindingSnapshot, canonicalPath, flag.Name)
		if err != nil {
			resolveErr = fmt.Errorf("flag --%s: %w", flag.Name, err)
			return
		}
		propertyWinner, err := resolveRuntimeSchemaField("property",
			excludedProperty,
			bindingProperty,
			runtimeSchemaStringCandidate(firstFlagAnnotation(flag, runtimeSchemaFlagBindingPropertyAnnotation), "versioned_parameter_binding"),
			// ParamDecl.Property is authored on the leaf and applied at assembly
			// via ApplyParamDecls. It outranks any residual versioned binding
			// candidate; active bindings JSON rows are empty after Phase 2.
			runtimeSchemaStringCandidateAtRank(
				firstFlagAnnotation(flag, runtimeSchemaFlagPropertyAnnotation),
				"native_annotation",
				runtimeSchemaRankParamDeclProperty,
				runtimeSchemaPrecedenceNativeAnnotation,
			),
			runtimeSchemaStringCandidate(lowerCamelFlagName(flag.Name), "flag_name_inference"),
		)
		if err != nil {
			resolveErr = fmt.Errorf("flag --%s: %w", flag.Name, err)
			return
		}
		property, _ := propertyWinner.Value.(string)
		embeddedParam, hasEmbeddedParam := embeddedMCPParamMeta{}, false
		if strings.TrimSpace(property) != "" {
			embeddedParam, hasEmbeddedParam = lookupEmbeddedMCPParam(embeddedParams, property, flag.Name)
		}
		flagName := flag.Name

		paramType := runtimeFlagCLIType(flag)
		interfaceTypeWinner, err := resolveRuntimeSchemaField("interface_type",
			runtimeSchemaStringCandidate(firstFlagAnnotation(flag, runtimeSchemaFlagTypeAnnotation), "native_annotation"),
			runtimeSchemaStringCandidate(embeddedParam.Type, "mcp_metadata"),
			runtimeSchemaStringCandidateAtRank(paramType, "cobra_flag_type", runtimeSchemaRankInference, "fallback"),
		)
		if err != nil {
			resolveErr = fmt.Errorf("flag --%s: %w", flag.Name, err)
			return
		}
		interfaceType, _ := interfaceTypeWinner.Value.(string)

		descriptionWinner, err := resolveRuntimeSchemaField("description",
			runtimeSchemaStringCandidate(firstFlagAnnotation(flag, runtimeSchemaFlagDescriptionAnnotation), "native_annotation"),
			runtimeSchemaStringCandidate(flag.Usage, "cobra_usage"),
			runtimeSchemaStringCandidate(embeddedParam.Description, "mcp_metadata"),
			runtimeSchemaCandidate("", true, "default"),
		)
		if err != nil {
			resolveErr = fmt.Errorf("flag --%s: %w", flag.Name, err)
			return
		}
		description, _ := descriptionWinner.Value.(string)
		interfaceDescription := ""
		if hasEmbeddedParam {
			interfaceDescription = strings.TrimSpace(embeddedParam.Description)
		}

		// Required uses field-level safe merge: higher sources may raise required, but
		// Cobra MarkFlagRequired cannot be projected away as optional.
		constraintRequired := runtimeSchemaFieldCandidate{}
		if runtimeSchemaRequireOneOfContains(constraints, flag.Name, flagName, property) {
			constraintRequired = runtimeSchemaCandidate(false, true, "require_one_of_constraint")
		}
		mcpRequired := runtimeSchemaFieldCandidate{}
		if hasEmbeddedParam && embeddedParam.Required != nil {
			mcpRequired = runtimeSchemaCandidate(*embeddedParam.Required, true, "mcp_metadata")
		}
		usageRequired := usageImpliesRequired(flag.Usage)
		cobraDefaultOptional := (runtimeFlagDefault(flag) != "" || usageImpliesDefault(flag.Usage)) && !usageRequired
		typedRequired := false
		for _, name := range metadata.Required {
			if strings.TrimSpace(name) == flag.Name {
				typedRequired = true
				break
			}
		}
		cobraHardRequired := runtimeFlagCobraHardRequired(flag)
		requiredWinner, err := resolveRequiredProjection(cobraHardRequired,
			constraintRequired,
			runtimeSchemaCandidate(true, typedRequired, "typed_parameter_metadata"),
			runtimeSchemaAnnotatedBoolCandidate(flag, runtimeSchemaFlagMetadataRequiredAnnotation, "typed_parameter_metadata"),
			runtimeSchemaAnnotatedBoolCandidate(flag, runtimeSchemaFlagRequiredAnnotation, "native_annotation"),
			runtimeSchemaCandidate(true, cobraHardRequired, "cobra_hard_required"),
			runtimeSchemaCandidate(false, cobraDefaultOptional, "cobra_nonzero_default"),
			runtimeSchemaCandidate(usageRequired, usageRequired, "usage_required_inference"),
			mcpRequired,
			runtimeSchemaCandidate(false, true, "default"),
		)
		if err != nil {
			resolveErr = fmt.Errorf("flag --%s: %w", flag.Name, err)
			return
		}
		if requiredWinner.Source == "default" {
			requiredWinner.Resolution = "fallback"
		}
		required, _ := requiredWinner.Value.(bool)

		fieldProvenance := map[string]contract.FieldProvenance{
			"type":        runtimeSchemaFieldProvenance(runtimeSchemaStringCandidate(paramType, "cobra_flag_type")),
			"description": runtimeSchemaFieldProvenance(descriptionWinner),
			"required":    runtimeSchemaFieldProvenance(requiredWinner),
		}
		parameter := ParameterSpec{
			Name:            flagName,
			Type:            paramType,
			Description:     description,
			Property:        property,
			Required:        required,
			CLIRequired:     runtimeFlagCobraHardRequired(flag),
			FieldProvenance: fieldProvenance,
		}
		// An explicit reviewed mapping exclusion is a present winner whose
		// value is intentionally empty. Preserve its provenance even though
		// the wire payload omits property; otherwise final delivery cannot
		// distinguish reviewed absence from an accidentally dropped field.
		if propertyWinner.Present {
			fieldProvenance["property"] = runtimeSchemaFieldProvenance(propertyWinner)
		}
		if parameter.CLIRequired {
			fieldProvenance["cli_required"] = runtimeSchemaFieldProvenance(
				runtimeSchemaCandidate(true, true, "cobra_hard_required"),
			)
		}
		if interfaceDescription != "" && interfaceDescription != description {
			parameter.InterfaceDescription = interfaceDescription
		}
		if interfaceType != "" && interfaceType != paramType {
			parameter.InterfaceType = interfaceType
			fieldProvenance["interface_type"] = runtimeSchemaFieldProvenance(interfaceTypeWinner)
		}
		requiredWhenWinner, err := resolveRuntimeSchemaField("required_when",
			runtimeSchemaStringCandidate(metadata.RequiredWhen[flag.Name], "typed_parameter_metadata"),
			runtimeSchemaStringCandidate(firstFlagAnnotation(flag, runtimeSchemaFlagMetadataRequiredWhenAnnotation), "typed_parameter_metadata"),
			runtimeSchemaStringCandidate(firstFlagAnnotation(flag, runtimeSchemaFlagRequiredWhenAnnotation), "native_annotation"),
			runtimeSchemaStringCandidate(embeddedParam.RequiredWhen, "mcp_metadata"),
			runtimeSchemaCandidate("", true, "default"),
		)
		if err != nil {
			resolveErr = fmt.Errorf("flag --%s: %w", flag.Name, err)
			return
		}
		requiredWhen, _ := requiredWhenWinner.Value.(string)
		parameter.RequiredWhen = requiredWhen
		fieldProvenance["required_when"] = runtimeSchemaFieldProvenance(requiredWhenWinner)
		if def := runtimeFlagDefault(flag); def != "" {
			parameter.Default = runtimeSchemaJSONString(def)
		}
		if hasEmbeddedParam {
			interfaceDefault := strings.TrimSpace(embeddedParam.Default)
			if interfaceDefault != "" && interfaceDefault != runtimeFlagDefault(flag) {
				parameter.InterfaceDefault = runtimeSchemaJSONString(interfaceDefault)
			}
		}
		formatWinner, err := resolveRuntimeSchemaField("format",
			runtimeSchemaStringCandidate(metadata.Formats[flag.Name], "typed_parameter_metadata"),
			runtimeSchemaStringCandidate(firstFlagAnnotation(flag, runtimeSchemaFlagMetadataFormatAnnotation), "typed_parameter_metadata"),
			runtimeSchemaStringCandidate(firstFlagAnnotation(flag, "x-cli-format"), "native_annotation"),
			runtimeSchemaStringCandidate(embeddedParam.Format, "mcp_metadata"),
			runtimeSchemaStringCandidate(inferredRuntimeFlagFormat(flag), "usage_format_inference"),
			runtimeSchemaCandidate("", true, "default"),
		)
		if err != nil {
			resolveErr = fmt.Errorf("flag --%s: %w", flag.Name, err)
			return
		}
		if format, _ := formatWinner.Value.(string); format != "" {
			parameter.Format = format
			fieldProvenance["format"] = runtimeSchemaFieldProvenance(formatWinner)
		}

		enumWinner, err := resolveRuntimeSchemaField("enum",
			runtimeSchemaEnumCandidate(metadata.Enums[flag.Name], "typed_parameter_metadata"),
			runtimeSchemaEnumCandidate(runtimeFlagEnumAnnotation(flag, runtimeSchemaFlagMetadataEnumAnnotation), "typed_parameter_metadata"),
			runtimeSchemaEnumCandidate(runtimeFlagEnum(flag), "native_annotation"),
			runtimeSchemaEnumCandidate(embeddedParam.Enum, "mcp_metadata"),
			runtimeSchemaCandidate([]string{}, true, "default"),
		)
		if err != nil {
			resolveErr = fmt.Errorf("flag --%s: %w", flag.Name, err)
			return
		}
		if enum, _ := enumWinner.Value.([]string); len(enum) > 0 {
			parameter.Enum = append([]string(nil), enum...)
			fieldProvenance["enum"] = runtimeSchemaFieldProvenance(enumWinner)
		}

		exampleWinner, err := resolveRuntimeSchemaField("example",
			runtimeSchemaStringCandidate(metadata.Examples[flag.Name], "typed_parameter_metadata"),
			runtimeSchemaStringCandidate(firstFlagAnnotation(flag, runtimeSchemaFlagMetadataExampleAnnotation), "typed_parameter_metadata"),
			runtimeSchemaStringCandidate(firstFlagAnnotation(flag, runtimeSchemaFlagExampleAnnotation), "native_annotation"),
			runtimeSchemaCandidate("", true, "default"),
		)
		if err != nil {
			resolveErr = fmt.Errorf("flag --%s: %w", flag.Name, err)
			return
		}
		if example, _ := exampleWinner.Value.(string); example != "" {
			parameter.Example = runtimeSchemaJSONString(example)
			fieldProvenance["example"] = runtimeSchemaFieldProvenance(exampleWinner)
		}
		params = append(params, parameter)
	})
	if resolveErr != nil {
		return nil, resolveErr
	}
	if len(params) == 0 {
		return nil, nil
	}
	sort.Slice(params, func(i, j int) bool { return params[i].Name < params[j].Name })
	return params, nil
}

func runtimeSchemaJSONString(value string) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

// runtimeCommandParameters is the compatibility wire adapter for callers that
// have not yet moved to ToolSpec. Resolution happens only in the typed path;
// this wrapper serializes the resulting ParameterSpecs without re-merging or
// re-interpreting any source.
var runtimeCommandParameterSpecsForPayload = runtimeCommandParameterSpecs

func runtimeCommandParameters(cmd *cobra.Command, canonicalPath string, embeddedParams map[string]embeddedMCPParamMeta, constraints RuntimeSchemaConstraints) (map[string]any, error) {
	specs, err := runtimeCommandParameterSpecsForPayload(cmd, canonicalPath, embeddedParams, constraints)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, nil
	}
	parameters := make(map[string]any, len(specs))
	for _, spec := range specs {
		payload, payloadErr := spec.ToPayload()
		if payloadErr != nil {
			return nil, fmt.Errorf("serialize Schema parameter %q: %w", spec.Name, payloadErr)
		}
		parameters[spec.Name] = payload
	}
	return parameters, nil
}

func runtimeSchemaRequireOneOfContains(constraints RuntimeSchemaConstraints, names ...string) bool {
	wanted := map[string]bool{}
	for _, name := range names {
		if name = strings.TrimSpace(name); name != "" {
			wanted[name] = true
		}
	}
	for _, group := range constraints.RequireOneOf {
		for _, name := range group {
			if wanted[strings.TrimSpace(name)] {
				return true
			}
		}
	}
	return false
}

func runtimeCommandFlag(cmd *cobra.Command, name string) *pflag.Flag {
	return runtimeannotate.CommandFlag(cmd, name)
}

func visitRuntimeCommandFlags(cmd *cobra.Command, inheritedNames []string, visit func(*pflag.Flag)) {
	if cmd == nil || visit == nil {
		return
	}
	root := cmd.Root()
	rootPersistent := map[*pflag.Flag]bool{}
	ancestorPersistent := map[*pflag.Flag]bool{}
	if root != nil {
		root.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
			rootPersistent[flag] = true
		})
	}
	for parent := cmd.Parent(); parent != nil && parent != root; parent = parent.Parent() {
		parent.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
			ancestorPersistent[flag] = true
		})
	}
	allowedInherited := map[string]bool{}
	for _, name := range inheritedNames {
		if name = strings.TrimSpace(name); name != "" {
			allowedInherited[name] = true
		}
	}
	seen := map[string]bool{}
	visitSet := func(flags *pflag.FlagSet) {
		flags.VisitAll(func(flag *pflag.Flag) {
			if flag == nil || rootPersistent[flag] || seen[flag.Name] ||
				(ancestorPersistent[flag] && !allowedInherited[flag.Name]) {
				return
			}
			seen[flag.Name] = true
			visit(flag)
		})
	}
	visitSet(cmd.Flags())
	visitSet(cmd.PersistentFlags())
	for parent := cmd.Parent(); parent != nil && parent != root; parent = parent.Parent() {
		parent.PersistentFlags().VisitAll(func(flag *pflag.Flag) {
			if flag != nil && allowedInherited[flag.Name] && !seen[flag.Name] {
				seen[flag.Name] = true
				visit(flag)
			}
		})
	}
}

func runtimeCommandConstraints(cmd *cobra.Command) RuntimeSchemaConstraints {
	return runtimeannotate.CommandConstraints(cmd)
}

func runtimeCommandPositionals(cmd *cobra.Command) []contract.RuntimeSchemaPositional {
	return runtimeannotate.CommandPositionals(cmd)
}

func normalizeRuntimeSchemaConstraints(constraints RuntimeSchemaConstraints) RuntimeSchemaConstraints {
	return runtimeannotate.NormalizeConstraints(constraints)
}

func normalizeRuntimeSchemaGroups(groups [][]string, minimum int) [][]string {
	// Test-facing thin wrapper over NormalizeConstraints group rules.
	c := RuntimeSchemaConstraints{}
	switch minimum {
	case 1:
		c.RequireOneOf = groups
	default:
		c.MutuallyExclusive = groups
	}
	out := runtimeannotate.NormalizeConstraints(c)
	if minimum == 1 {
		return out.RequireOneOf
	}
	return out.MutuallyExclusive
}

func runtimeSchemaConstraintsEmpty(constraints RuntimeSchemaConstraints) bool {
	return runtimeannotate.ConstraintsEmpty(constraints)
}

func lookupEmbeddedMCPParam(params map[string]embeddedMCPParamMeta, property, flagName string) (embeddedMCPParamMeta, bool) {
	if len(params) == 0 {
		return embeddedMCPParamMeta{}, false
	}
	if meta, ok := params[property]; ok {
		return meta, true
	}
	if meta, ok := params[flagName]; ok {
		return meta, true
	}
	return embeddedMCPParamMeta{}, false
}

func isGenericPayloadFlag(flag *pflag.Flag) bool {
	if flag == nil {
		return false
	}
	switch flag.Name {
	case "json":
		return strings.TrimSpace(flag.Usage) == "Base JSON object payload for this tool invocation"
	case "params":
		return strings.TrimSpace(flag.Usage) == "Additional JSON object payload merged after --json"
	default:
		return false
	}
}

func runtimeFlagCLIType(flag *pflag.Flag) string {
	switch flag.Value.Type() {
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "count":
		return "integer"
	case "float32", "float64":
		return "number"
	case "bool":
		return "boolean"
	case "stringSlice", "stringArray", "intSlice", "int32Slice", "int64Slice",
		"uintSlice", "float32Slice", "float64Slice", "boolSlice", "durationSlice":
		return "array"
	default:
		return "string"
	}
}

func runtimeFlagRequiredState(flag *pflag.Flag) (bool, bool) {
	// This helper reports the projected Schema annotation first. Cobra's
	// executable marker is retained as a lower-priority observation; the typed
	// contract exposes both candidates when an explicit overlay lowers it.
	if raw := firstFlagAnnotation(flag, runtimeSchemaFlagRequiredAnnotation); raw != "" {
		required, err := strconv.ParseBool(raw)
		if err == nil {
			return required, true
		}
	}
	if runtimeFlagCobraHardRequired(flag) {
		return true, true
	}
	usage := strings.ToLower(strings.TrimSpace(flag.Usage))
	if usageImpliesRequired(usage) {
		return true, true
	}
	return false, false
}

func usageImpliesRequired(usage string) bool {
	usage = strings.ToLower(strings.TrimSpace(usage))
	if usage == "" {
		return false
	}
	for _, conditional := range []string{
		"可选", "时必填", "下必填", "二选一", "至少传一个", "至少填一个", "至少提供一项",
		"at least one", "required when", "required if", "conditionally required",
	} {
		if strings.Contains(usage, conditional) {
			return false
		}
	}
	return strings.Contains(usage, "required") || strings.Contains(usage, "必填")
}

func usageImpliesDefault(usage string) bool {
	usage = strings.ToLower(strings.TrimSpace(usage))
	return strings.Contains(usage, "默认") || strings.Contains(usage, "default")
}

func lowerCamelFlagName(flagName string) string {
	flagName = strings.TrimSpace(flagName)
	parts := strings.FieldsFunc(flagName, func(r rune) bool { return r == '-' || r == '_' })
	if len(parts) == 0 {
		return flagName
	}
	if len(parts) == 1 {
		return parts[0]
	}
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	out := strings.ToLower(parts[0])
	for _, part := range parts[1:] {
		lower := strings.ToLower(part)
		out += strings.ToUpper(lower[:1]) + lower[1:]
	}
	return out
}

func runtimeFlagDefault(flag *pflag.Flag) string {
	def := strings.TrimSpace(flag.DefValue)
	if def == "" || def == "0s" || def == "[]" || def == "{}" {
		return ""
	}
	switch flag.Value.Type() {
	case "bool":
		if def == "false" {
			return ""
		}
	case "int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "count",
		"float32", "float64":
		if def == "0" {
			return ""
		}
	}
	return def
}

func firstFlagAnnotation(flag *pflag.Flag, key string) string {
	if flag == nil || flag.Annotations == nil {
		return ""
	}
	values := flag.Annotations[key]
	if len(values) == 0 {
		return ""
	}
	return strings.TrimSpace(values[0])
}

func runtimeFlagEnum(flag *pflag.Flag) []string {
	return runtimeFlagEnumAnnotation(flag, "x-cli-enum")
}

func runtimeFlagEnumAnnotation(flag *pflag.Flag, annotation string) []string {
	if flag == nil || flag.Annotations == nil {
		return nil
	}
	values := flag.Annotations[annotation]
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func runtimeSchemaEnumCandidate(values []string, source string) runtimeSchemaFieldCandidate {
	// Normalize at the candidate boundary exactly as ParameterSpec does. This
	// keeps the selected provenance value byte-for-byte aligned with the final
	// delivery even when an authored enum repeats a value.
	clean := stableUniqueStrings(values)
	return runtimeSchemaCandidate(clean, len(clean) > 0, source)
}

func inferredRuntimeFlagFormat(flag *pflag.Flag) string {
	if flag == nil {
		return ""
	}
	usage := strings.ToLower(strings.TrimSpace(flag.Usage))
	if strings.Contains(usage, "iso-8601") || strings.Contains(usage, "rfc3339") {
		return "date-time"
	}
	if strings.Contains(usage, "a1") {
		return "a1-range"
	}
	return ""
}

func strconvQuote(value string) string {
	return "\"" + strings.ReplaceAll(value, "\"", "\\\"") + "\""
}

// ─── --compact mode ──────────────────────────────────────────────────────────

// schemaCompactStripKeys are top-level tool/product keys removed in --compact mode.
var schemaCompactStripKeys = map[string]bool{
	// provenance / debug
	"agent_metadata_source": true,
	"agent_source_refs":     true,
	"agent_summary_source":  true,
	"effect_source":         true,
	"metadata_source":       true,
	"source":                true,
	"agent_metadata":        true,
	"interface_metadata":    true,
	"field_provenance":      true,
	"reviewed":              true,
	// redundant with canonical_path / cli_path
	"name":              true,
	"path":              true,
	"cli_name":          true,
	"primary_cli_path":  true,
	"is_alias":          true,
	"has_parameters":    true,
	"parameter_count":   true,
	"product_id":        true,
	"display":           true,
	"title":             true,
	"group":             true,
	"source_product_id": true,
	"aliases":           true,
	"catalog_hash":      true,
	"surface_hash":      true,
	"workflow_refs":     true,
	"prerequisites":     true,
	"tips":              true,
	"interface_ref":     true,
}

// schemaCompactParamStripKeys are per-parameter keys removed in --compact mode.
var schemaCompactParamStripKeys = map[string]bool{
	"interface_description": true,
	"interface_type":        true,
	"property":              true,
	"field_provenance":      true,
}

// stripSchemaPayloadCompact walks a schema payload map and removes provenance,
// debug and redundant keys so that only agent-essential fields remain.
// It operates recursively on nested maps, slices, and parameter objects.
func stripSchemaPayloadCompact(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	result := make(map[string]any, len(payload))
	for k, v := range payload {
		if schemaCompactStripKeys[k] {
			continue
		}
		if k == "parameters" {
			result[k] = stripSchemaParametersCompact(v)
			continue
		}
		result[k] = stripSchemaValueCompact(v)
	}
	return result
}

func stripSchemaParametersCompact(value any) any {
	parameters, ok := value.(map[string]any)
	if !ok {
		return stripSchemaValueCompact(value)
	}
	result := make(map[string]any, len(parameters))
	for name, raw := range parameters {
		parameter, ok := raw.(map[string]any)
		if !ok {
			result[name] = stripSchemaValueCompact(raw)
			continue
		}
		// Parameter names are contract data. They may legitimately be
		// "name", "path", "source", or another key that is redundant only
		// at tool/product level, so never run them through the top-level key
		// filter.
		result[name] = stripSchemaParamCompact(parameter)
	}
	return result
}

func stripSchemaValueCompact(v any) any {
	switch val := v.(type) {
	case map[string]any:
		// Check if this looks like a parameter object (has "type" or "required" or "description")
		_, isParam := val["required"]
		if !isParam {
			_, isParam = val["type"]
		}
		if isParam {
			return stripSchemaParamCompact(val)
		}
		return stripSchemaPayloadCompact(val)
	case []map[string]any:
		result := make([]map[string]any, len(val))
		for i, item := range val {
			result[i] = stripSchemaPayloadCompact(item)
		}
		return result
	case []any:
		result := make([]any, len(val))
		for i, item := range val {
			result[i] = stripSchemaValueCompact(item)
		}
		return result
	default:
		return v
	}
}

func stripSchemaParamCompact(param map[string]any) map[string]any {
	result := make(map[string]any, len(param))
	for k, v := range param {
		if schemaCompactParamStripKeys[k] {
			continue
		}
		result[k] = v
	}
	return result
}
