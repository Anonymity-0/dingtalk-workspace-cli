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
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/spf13/cobra"
)

func TestAgentMetadataFixtureLoadsSplitDomains(t *testing.T) {
	// Production no longer embeds or ships schema_agent_metadata/*.json.
	// Runtime Agent metadata must stay empty; selection completeness now lives
	// in schema_catalog (see TestDeliverySchemaCatalogSelectionCompleteness).
	metadata := runtimeAgentMetadata()
	if len(metadata.Tools) != 0 || len(metadata.Products) != 0 || len(metadata.Domains) != 0 {
		t.Fatalf("retired Agent metadata snapshot must be empty: %#v", metadata)
	}
	// Temporary MapFS fixture only — exercises the retired split-domain loader
	// seam without depending on a committed schema_agent_metadata/ directory.
	fixture := fstest.MapFS{
		"schema_agent_metadata/index.json":  {Data: []byte(`{"domains":["sample"],"coverage":{"tools_with_metadata":1}}`)},
		"schema_agent_metadata/sample.json": {Data: []byte(`{"product_id":"sample","tools":{"sample.get":{"agent_summary":"S","use_when":["u"],"avoid_when":["a"],"examples":["dws sample get"],"interface_mode":"local","availability":"available"}}}`)},
	}
	loaded := loadAgentMetadataFixtureFrom(fixture)
	if len(loaded.Tools) != 1 || loaded.Tools["sample.get"].AgentSummary != "S" {
		t.Fatalf("fixture loader = %#v", loaded)
	}
}

// TestDeliverySchemaCatalogSelectionCompleteness replaces the retired
// schema_agent_metadata/*.json split-domain coverage gate: every delivered
// Catalog tool must carry non-empty selection routing, interface disposition,
// and examples that never bypass confirmation with --yes.
func TestDeliverySchemaCatalogSelectionCompleteness(t *testing.T) {
	if !deliverySchemaCatalogAvailable() {
		t.Fatalf("delivery schema Catalog is unavailable: %v", deliverySchemaCatalogError())
	}
	loaded := mustDeliverySchemaCatalogMaps(t)
	products := map[string]struct{}{}
	for canonical, tool := range loaded.Snapshot.Tools {
		product := schemaString(tool["product_id"])
		if product == "" {
			t.Errorf("tool %s missing product_id", canonical)
			continue
		}
		products[product] = struct{}{}
		if len(schemaStringSlice(tool["use_when"])) == 0 ||
			len(schemaStringSlice(tool["avoid_when"])) == 0 ||
			len(schemaStringSlice(tool["examples"])) == 0 {
			t.Errorf("tool %s has incomplete selection metadata: use_when=%v avoid_when=%v examples=%v",
				canonical, tool["use_when"], tool["avoid_when"], tool["examples"])
		}
		if schemaString(tool["interface_mode"]) == "" || schemaString(tool["availability"]) == "" {
			t.Errorf("tool %s has incomplete interface disposition: mode=%q availability=%q",
				canonical, schemaString(tool["interface_mode"]), schemaString(tool["availability"]))
		}
		for _, example := range schemaStringSlice(tool["examples"]) {
			if strings.Contains(" "+example+" ", " --yes ") {
				t.Errorf("tool %s example bypasses confirmation: %q", canonical, example)
			}
		}
	}
	if len(products) < 2 {
		t.Fatalf("catalog products = %d, want multi-product delivery", len(products))
	}
	if _, ok := loaded.Snapshot.Tools["calendar.create_calendar_event"]; !ok {
		t.Fatalf("calendar.create_calendar_event missing from catalog tools (%d total)", len(loaded.Snapshot.Tools))
	}
	if got, want := len(loaded.Snapshot.Tools), len(loaded.Index.CanonicalPaths()); got != want {
		t.Fatalf("catalog tools = %d, typed index = %d", got, want)
	}
}

func TestAgentMetadataTypedAccessorRoundTripsProvenance(t *testing.T) {
	const encoded = `{
  "product_id": "calendar",
  "tools": {
    "calendar attendee update": {
      "agent_summary": "Update one attendee",
      "agent_summary_source": "reviewed-selection",
      "use_when": ["change an attendee"],
      "avoid_when": ["read attendees"],
      "prerequisites": ["event id"],
      "tips": ["verify the attendee id"],
      "effect": "write",
      "effect_source": "agent-hint",
      "risk": "low",
      "confirmation": "not_required",
      "idempotency": "non_idempotent",
      "workflow_refs": ["calendar-update"],
      "examples": ["dws calendar attendee update --event-id e1"],
      "reviewed": true,
      "source_refs": ["internal/cli/schema_hints/calendar.json"],
      "interface_ref": {"product_id": "calendar", "rpc_name": "update_attendee"},
      "interface_mode": "mcp",
      "availability": "available",
      "interface_reason": "reviewed RPC mapping",
      "field_provenance": {
        "risk": {
          "value": "low",
          "source": "reviewed.json",
          "precedence": "reviewed_explicit",
          "resolution": "highest_precedence",
          "review_reason": "reviewed downgrade",
          "candidates": [
            {"value": "low", "source": "reviewed.json", "precedence": "reviewed_explicit", "review_reason": "reviewed downgrade", "selected": true},
            {"value": "high", "source": "imported.json", "precedence": "imported", "selected": false}
          ],
          "overridden_candidates": [
            {"value": "medium", "source": "generated-default", "precedence": "inference_or_default", "selected": false}
          ]
        }
      }
    }
  }
}`
	var fragment agentMetadataDomain
	if err := json.Unmarshal([]byte(encoded), &fragment); err != nil {
		t.Fatalf("decode generated Agent metadata: %v", err)
	}
	roundTrip, err := json.Marshal(fragment)
	if err != nil {
		t.Fatalf("encode generated Agent metadata: %v", err)
	}
	var decoded agentMetadataDomain
	if err := json.Unmarshal(roundTrip, &decoded); err != nil {
		t.Fatalf("round-trip generated Agent metadata: %v", err)
	}

	metadataFixture := agentMetadata{
		Products: map[string]agentProductMetadata{},
		Tools:    decoded.Tools,
	}

	safety, interfaceSpec, selection, provenance, ok := agentToolContractForPathsFromMetadata(metadataFixture, "missing", " calendar   attendee update ")
	if !ok {
		t.Fatal("typed Agent metadata lookup failed")
	}
	if safety != (contract.SafetySpec{Effect: "write", EffectSource: "agent-hint", Risk: "low", Confirmation: "not_required", Idempotency: "non_idempotent"}) {
		t.Fatalf("safety = %#v", safety)
	}
	if interfaceSpec.Ref == nil || interfaceSpec.Ref.ProductID != "calendar" || interfaceSpec.Ref.RPCName != "update_attendee" || interfaceSpec.Mode != "mcp" || interfaceSpec.Availability != "available" || interfaceSpec.Reason != "reviewed RPC mapping" {
		t.Fatalf("interface = %#v", interfaceSpec)
	}
	if selection.AgentSummary != "Update one attendee" || selection.MetadataSource != ProvenanceEmbeddedSkillMetadata || selection.Reviewed == nil || !*selection.Reviewed || len(selection.Examples) != 1 {
		t.Fatalf("selection = %#v", selection)
	}
	risk := provenance["risk"]
	if string(risk.Value) != `"low"` || risk.Source != "reviewed.json" || risk.Precedence != "reviewed_explicit" || risk.Resolution != "highest_precedence" || risk.ReviewReason != "reviewed downgrade" {
		t.Fatalf("risk provenance = %#v", risk)
	}
	if len(risk.Candidates) != 2 || risk.Candidates[0].Selected == nil || !*risk.Candidates[0].Selected || risk.Candidates[1].Selected == nil || *risk.Candidates[1].Selected {
		t.Fatalf("risk candidates = %#v", risk.Candidates)
	}
	if string(risk.Candidates[1].Value) != `"high"` || risk.Candidates[1].Source != "imported.json" {
		t.Fatalf("overridden risk candidate = %#v", risk.Candidates[1])
	}
	if len(risk.OverriddenCandidates) != 1 || string(risk.OverriddenCandidates[0].Value) != `"medium"` || risk.OverriddenCandidates[0].Precedence != "inference_or_default" {
		t.Fatalf("legacy overridden candidates were dropped: %#v", risk.OverriddenCandidates)
	}

	// Accessors return detached typed values; callers cannot mutate the
	// embedded snapshot and accidentally change a later schema response.
	interfaceSpec.Ref.ProductID = "mutated"
	selection.UseWhen[0] = "mutated"
	risk.Candidates[0].Value[0] = 'x'
	provenance["risk"] = risk
	_, interfaceAgain, selectionAgain, provenanceAgain, _ := agentToolContractForPathsFromMetadata(metadataFixture, "calendar attendee update")
	if interfaceAgain.Ref.ProductID != "calendar" || selectionAgain.UseWhen[0] != "change an attendee" || string(provenanceAgain["risk"].Candidates[0].Value) != `"low"` {
		t.Fatalf("typed accessor leaked mutable state: interface=%#v selection=%#v provenance=%#v", interfaceAgain, selectionAgain, provenanceAgain)
	}

}

func TestCrossPlatformCoverageAgentMetadataInterfaceProvenance(t *testing.T) {
	selected := true
	legacyRef := func(value string) contract.FieldProvenance {
		raw, _ := json.Marshal(value)
		return contract.FieldProvenance{
			Value:      raw,
			Source:     "agent-metadata.json",
			Precedence: "explicit",
			Resolution: "highest_precedence",
			Candidates: []contract.FieldCandidateProvenance{{
				Value:      append(json.RawMessage(nil), raw...),
				Source:     "agent-metadata.json",
				Precedence: "explicit",
				Selected:   &selected,
			}},
		}
	}
	mode := resolvedFieldProvenance("local", "reviewed.json", "", "reviewed_explicit", "highest_precedence", "reviewed local wrapper")

	metadataFixture := agentMetadata{
		Products: map[string]agentProductMetadata{},
		Tools: map[string]agentToolMetadata{
			"calendar event get": {
				InterfaceRef:  &embeddedMCPInterfaceRef{ProductID: "calendar", RPCName: "get_event"},
				InterfaceMode: "mcp",
				Availability:  "available",
				FieldProvenance: map[string]contract.FieldProvenance{
					"interface_ref": legacyRef("calendar.get_event"),
				},
			},
			"calendar helper run": {
				InterfaceMode:   "local",
				Availability:    "available",
				InterfaceReason: "reviewed local wrapper",
				FieldProvenance: map[string]contract.FieldProvenance{
					"interface_ref":  legacyRef("<none>"),
					"interface_mode": mode,
				},
			},
			"calendar helper inspect": {
				InterfaceMode: "local",
				Availability:  "available",
				FieldProvenance: map[string]contract.FieldProvenance{
					"interface_mode": mode,
				},
			},
		},
	}

	_, mcpInterface, _, mcpProvenance, ok := agentToolContractForPathsFromMetadata(metadataFixture, "calendar event get")
	if !ok {
		t.Fatal("mcp metadata lookup failed")
	}
	wantRef := `{"product_id":"calendar","rpc_name":"get_event"}`
	if got := string(mcpProvenance["interface_ref"].Value); got != wantRef {
		t.Fatalf("typed interface_ref winner = %s, want %s", got, wantRef)
	}
	if got := string(mcpProvenance["interface_ref"].Candidates[0].Value); got != wantRef {
		t.Fatalf("typed interface_ref candidate = %s, want %s", got, wantRef)
	}
	if err := validateFinalFieldProvenance("calendar.event_get", "interface_ref", mcpProvenance["interface_ref"], mcpInterface.Ref); err != nil {
		t.Fatalf("typed mcp provenance = %v", err)
	}
	for _, field := range []string{"interface_reason", "agent_summary"} {
		if _, exists := mcpProvenance[field]; exists {
			t.Fatalf("typed adapter invented absent %s provenance: %#v", field, mcpProvenance[field])
		}
	}

	_, localInterface, _, provenance, ok := agentToolContractForPathsFromMetadata(metadataFixture, "calendar helper run")
	if !ok {
		t.Fatal("calendar helper run metadata lookup failed")
	}
	if got := string(provenance["interface_ref"].Value); got != "null" {
		t.Fatalf("calendar helper run interface_ref winner = %s, want null", got)
	}
	if err := validateFinalFieldProvenance("calendar helper run", "interface_ref", provenance["interface_ref"], localInterface.Ref); err != nil {
		t.Fatalf("calendar helper run typed null provenance = %v", err)
	}

	_, _, _, provenance, ok = agentToolContractForPathsFromMetadata(metadataFixture, "calendar helper inspect")
	if !ok {
		t.Fatal("calendar helper inspect metadata lookup failed")
	}
	if _, exists := provenance["interface_ref"]; exists {
		t.Fatalf("typed adapter repaired missing interface_ref provenance: %#v", provenance["interface_ref"])
	}
}

func TestCrossPlatformCoverageAgentMetadataInterfaceConflict(t *testing.T) {
	selected := true
	wrong, _ := json.Marshal("calendar.wrong_rpc")
	provenance := contract.FieldProvenance{
		Value:      wrong,
		Source:     "bad.json",
		Precedence: "explicit",
		Resolution: "highest_precedence",
		Candidates: []contract.FieldCandidateProvenance{{
			Value: wrong, Source: "bad.json", Precedence: "explicit", Selected: &selected,
		}},
	}
	projected := projectAgentInterfaceRefProvenance(provenance, &contract.InterfaceRefSpec{ProductID: "calendar", RPCName: "get_event"})
	if string(projected.Value) != string(wrong) {
		t.Fatalf("conflicting winner was rewritten: %s", projected.Value)
	}
	if err := validateFinalFieldProvenance("calendar.event_get", "interface_ref", projected, &contract.InterfaceRefSpec{ProductID: "calendar", RPCName: "get_event"}); err == nil {
		t.Fatal("conflicting interface_ref provenance unexpectedly validated")
	}
}

func TestCrossPlatformCoverageAgentProductSelectionAccessor(t *testing.T) {
	provenance := resolvedFieldProvenance("Document operations", "manual", "manual.json", ProvenanceReviewedManual, "highest_precedence", "reviewed")
	metadataFixture := agentMetadata{
		Products: map[string]agentProductMetadata{
			"doc": {
				AgentSummary:       "Document operations",
				AgentSummarySource: "reviewed-doc-routing",
				UseWhen:            []string{"create a document", "create a document"},
				AvoidWhen:          []string{"manage a spreadsheet"},
				SourceRefs:         []string{"z.md", "a.md"},
				FieldProvenance:    map[string]contract.FieldProvenance{"agent_summary": provenance},
			},
		},
		Tools: map[string]agentToolMetadata{},
	}

	selection, ok := agentProductSelectionForIDsFromMetadata(metadataFixture, "missing", " doc ")
	if !ok || selection.AgentSummary != "Document operations" || selection.AgentSummarySource != "reviewed-doc-routing" || selection.MetadataSource != ProvenanceEmbeddedSkillMetadata {
		t.Fatalf("product selection = %#v, ok=%v", selection, ok)
	}
	if len(selection.UseWhen) != 1 || len(selection.SourceRefs) != 2 || selection.SourceRefs[0] != "a.md" {
		t.Fatalf("normalized product selection = %#v", selection)
	}
	_, deliveredProvenance, ok := agentProductContractForIDsFromMetadata(metadataFixture, "doc")
	if !ok || string(deliveredProvenance["agent_summary"].Value) != `"Document operations"` {
		t.Fatalf("product provenance = %#v, ok=%v", deliveredProvenance, ok)
	}
	selection.UseWhen[0] = "mutated"
	deliveredProvenance["agent_summary"] = contract.FieldProvenance{}
	again, _ := agentProductSelectionForIDsFromMetadata(metadataFixture, "doc")
	if again.UseWhen[0] != "create a document" {
		t.Fatalf("product accessor leaked mutable state: %#v", again)
	}
	_, againProvenance, _ := agentProductContractForIDsFromMetadata(metadataFixture, "doc")
	if len(againProvenance["agent_summary"].Value) == 0 {
		t.Fatal("product accessor leaked mutable provenance state")
	}

}

func TestRuntimeSchemaIncludesAgentMetadata(t *testing.T) {
	agentFixture := agentMetadata{
		Version:    1,
		SourceHash: "sha256:test",
		Products: map[string]agentProductMetadata{
			"doc": {
				AgentSummary:       "创建、读取和维护钉钉文档",
				AgentSummarySource: "test-source",
				UseWhen:            []string{"需要创建或读取文档"},
				SourceRefs:         []string{"skills/mono/SKILL.md"},
			},
		},
		Tools: map[string]agentToolMetadata{
			"doc create": {
				UseWhen:         []string{"新建文档"},
				AvoidWhen:       []string{"只需读取文档时"},
				Effect:          "write",
				EffectSource:    "command-verb",
				Examples:        []string{"dws doc create --title test"},
				SourceRefs:      []string{"skills/mono/references/products/doc.md"},
				InterfaceMode:   "local",
				Availability:    "available",
				InterfaceReason: "test local implementation",
			},
		},
	}
	mcpFixture := embeddedMCPMetadata{Tools: map[string]embeddedMCPToolMetadata{}}

	root := buildRuntimeSchemaTestRoot()
	leaf, err := runtimeSchemaPayloadForTestWithMetadata(root, []string{"doc.create_document"}, agentFixture, mcpFixture)
	if err != nil {
		t.Fatalf("runtimeSchemaPayloadForTest(leaf): %v", err)
	}
	if leaf["effect"] != "write" || leaf["agent_metadata_source"] != ProvenanceEmbeddedSkillMetadata {
		t.Fatalf("leaf Agent metadata = %#v", leaf)
	}
	if leaf["interface_mode"] != "local" || leaf["availability"] != "available" || leaf["interface_reason"] != "test local implementation" {
		t.Fatalf("leaf interface disposition = %#v", leaf)
	}
	if examples, _ := leaf["examples"].([]string); len(examples) != 1 {
		t.Fatalf("leaf examples = %#v", leaf["examples"])
	}

	catalog, err := runtimeSchemaPayloadForTestWithMetadata(root, nil, agentFixture, mcpFixture)
	if err != nil {
		t.Fatalf("runtimeSchemaPayloadForTest(catalog): %v", err)
	}
	summary, _ := catalog["agent_metadata"].(map[string]any)
	// Catalog-level Agent summary is derived from assembled products/tools
	// (ContractFinal / ProductDecl), not from the inject fixture source_hash.
	if summary["source"] != ProvenanceEmbeddedSkillMetadata {
		t.Fatalf("catalog Agent metadata summary = %#v", summary)
	}
	if summary["tools_with_metadata"] == nil || summary["products_with_metadata"] == nil {
		t.Fatalf("catalog Agent metadata summary missing coverage fields: %#v", summary)
	}
	products, _ := catalog["products"].([]map[string]any)
	doc := findSchemaProduct(products, "doc")
	if useWhen, _ := doc["use_when"].([]string); len(useWhen) != 1 {
		t.Fatalf("doc product use_when = %#v", doc["use_when"])
	}
	tools, _ := doc["tools"].([]map[string]any)
	if len(tools) != 1 || tools[0]["effect"] != "write" {
		t.Fatalf("doc tool summaries = %#v", tools)
	}
	if _, exists := tools[0]["examples"]; exists {
		t.Fatalf("product summary must not include examples: %#v", tools[0])
	}

	registry, err := schemaRegistryForTestWithMetadata(root, agentFixture, mcpFixture)
	if err != nil {
		t.Fatalf("schemaRegistryForTest(): %v", err)
	}
	compact, err := registry.ToOverviewPayload()
	if err != nil {
		t.Fatalf("ToOverviewPayload(): %v", err)
	}
	compactProducts, _ := compact["products"].([]map[string]any)
	compactDoc := findSchemaProduct(compactProducts, "doc")
	if compactDoc["agent_summary"] != "创建、读取和维护钉钉文档" {
		t.Fatalf("compact product summary = %#v", compactDoc)
	}
	if _, exists := compactDoc["agent_source_refs"]; exists {
		t.Fatalf("compact product must omit provenance: %#v", compactDoc)
	}
	if _, exists := compactDoc["use_when"]; exists {
		t.Fatalf("compact product with summary must omit routing expansion: %#v", compactDoc)
	}
}

func TestRuntimeSchemaAllPayloadContainsFullLeafParameters(t *testing.T) {
	// Synthetic fixture has no ContractFinal/ProductDecl; exercise the
	// test-isolated legacy assembly path (production fails closed).
	registry, err := schemaRegistryForTestWithMetadata(buildRuntimeSchemaTestRoot(), emptyAgentMetadata(), embeddedMCPMetadata{})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := runtimeSchemaAllPayloadFromRegistry(registry)
	if err != nil {
		t.Fatal(err)
	}
	products := schemaMapSlice(payload["products"])
	doc := findSchemaProduct(products, "doc")
	tools := schemaMapSlice(doc["tools"])
	if len(tools) != 1 {
		t.Fatalf("runtime full export tools = %#v", tools)
	}
	if got := schemaString(tools[0]["canonical_path"]); got != "doc.create_document" {
		t.Fatalf("canonical path = %q", got)
	}
	parameters, ok := tools[0]["parameters"].(map[string]any)
	if !ok || parameters["title"] == nil {
		t.Fatalf("runtime full export parameters = %#v", tools[0]["parameters"])
	}
}

func schemaTestInt(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	case json.Number:
		parsed, _ := typed.Int64()
		return int(parsed)
	case string:
		parsed, _ := json.Number(typed).Int64()
		return int(parsed)
	default:
		return 0
	}
}

func TestRuntimeSchemaUsesVersionedInterfaceRef(t *testing.T) {
	agentFixture := agentMetadata{
		Tools: map[string]agentToolMetadata{
			"doc create": {
				InterfaceRef:  &embeddedMCPInterfaceRef{ProductID: "documents", RPCName: "create_doc_v2"},
				InterfaceMode: "mcp",
				Availability:  "available",
			},
		},
		Products: map[string]agentProductMetadata{},
	}
	mcpFixture := embeddedMCPMetadata{
		Tools: map[string]embeddedMCPToolMetadata{
			"documents.create_doc_v2": {
				Parameters: map[string]embeddedMCPParamMeta{
					"title": {Description: "MCP document title"},
				},
			},
		},
	}

	payload, err := runtimeSchemaPayloadForTestWithMetadata(buildRuntimeSchemaTestRoot(), []string{"doc.create_document"}, agentFixture, mcpFixture)
	if err != nil {
		t.Fatal(err)
	}
	ref, _ := payload["interface_ref"].(map[string]any)
	if ref["product_id"] != "documents" || ref["rpc_name"] != "create_doc_v2" {
		t.Fatalf("interface_ref = %#v", payload["interface_ref"])
	}
	parameters, _ := payload["parameters"].(map[string]any)
	title, _ := parameters["title"].(map[string]any)
	if title["interface_description"] != "MCP document title" {
		t.Fatalf("title metadata = %#v", title)
	}
}

func TestMCPRequiredParticipatesInSourcePrecedence(t *testing.T) {
	required := true
	agentFixture := emptyAgentMetadata()
	mcpFixture := embeddedMCPMetadata{
		Tools: map[string]embeddedMCPToolMetadata{
			"sample.list_items": {
				Parameters: map[string]embeddedMCPParamMeta{
					"limit": {Required: &required},
				},
			},
		},
	}

	root := &cobra.Command{Use: "dws"}
	list := &cobra.Command{Use: "list", Run: func(*cobra.Command, []string) {}}
	list.Flags().Int("limit", 0, "optional page size")
	AttachRuntimeSchema(list, "sample", "list_items", "test")
	sample := &cobra.Command{Use: "sample"}
	sample.AddCommand(list)
	root.AddCommand(sample)

	payload, err := runtimeSchemaPayloadForTestWithMetadata(root, []string{"sample.list_items"}, agentFixture, mcpFixture)
	if err != nil {
		t.Fatal(err)
	}
	parameters, _ := payload["parameters"].(map[string]any)
	limit, _ := parameters["limit"].(map[string]any)
	if limit["required"] != true {
		t.Fatalf("MCP required candidate did not win over the default: %#v", limit)
	}
}

func TestMCPDefaultDoesNotOverrideCLIDefault(t *testing.T) {
	agentFixture := emptyAgentMetadata()
	mcpFixture := embeddedMCPMetadata{
		Tools: map[string]embeddedMCPToolMetadata{
			"sample.list_items": {
				Parameters: map[string]embeddedMCPParamMeta{
					"limit": {Default: "50"},
				},
			},
		},
	}

	root := &cobra.Command{Use: "dws"}
	list := &cobra.Command{Use: "list", Run: func(*cobra.Command, []string) {}}
	list.Flags().Int("limit", 10, "optional page size")
	AttachRuntimeSchema(list, "sample", "list_items", "test")
	sample := &cobra.Command{Use: "sample"}
	sample.AddCommand(list)
	root.AddCommand(sample)

	payload, err := runtimeSchemaPayloadForTestWithMetadata(root, []string{"sample.list_items"}, agentFixture, mcpFixture)
	if err != nil {
		t.Fatal(err)
	}
	parameters, _ := payload["parameters"].(map[string]any)
	limit, _ := parameters["limit"].(map[string]any)
	if limit["default"] != "10" || limit["interface_default"] != "50" {
		t.Fatalf("CLI and interface defaults were not separated: %#v", limit)
	}
}

func findSchemaProduct(products []map[string]any, id string) map[string]any {
	for _, product := range products {
		if product["id"] == id {
			return product
		}
	}
	return nil
}

func buildRuntimeSchemaTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "dws"}
	create := &cobra.Command{Use: "create", Short: "Create document", Run: func(*cobra.Command, []string) {}}
	create.Flags().String("title", "", "Document title")
	AttachRuntimeSchema(create, "doc", "create_document", "runtime:doc")
	AnnotateRuntimeFlag(create, "title", "title", "string", true, "")
	doc := &cobra.Command{Use: "doc", Short: "Docs"}
	doc.AddCommand(create)
	root.AddCommand(doc)
	return root
}
