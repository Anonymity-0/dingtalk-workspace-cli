// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/spf13/cobra"
)

func TestCrossPlatformCoverageResolveSchemaBuildAndAssembleEdges(t *testing.T) {
	if _, err := ResolveSchemaBuild(nil); err == nil {
		t.Fatal("nil root must fail ResolveSchemaBuild")
	}
	if _, err := AssembleSchemaRegistry(nil); err == nil {
		t.Fatal("nil root must fail AssembleSchemaRegistry")
	}
	if _, err := AssembleSchemaRegistryFromBound(BoundCommandRegistry{}); err == nil {
		t.Log("empty bound assemble returned nil error")
	}

	registry := SchemaRegistry{
		Source: SchemaSourceRuntimeAssembled,
		Products: []ProductSpec{{
			ID: "sample",
			Tools: []ToolSpec{{
				Identity: contract.ToolIdentitySpec{
					CLIPath: "sample run", CanonicalPath: "sample.run", ProductID: "sample",
					Name: "run", Path: "sample.run", PrimaryCLIPath: "sample run",
				},
				Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
				Selection: contract.SelectionSpec{AgentSummary: "sample"},
			}},
		}},
	}
	if _, err := runtimeSchemaAllPayloadFromRegistry(registry); err != nil {
		t.Fatalf("runtimeSchemaAllPayloadFromRegistry() error = %v", err)
	}
	if _, _, err := assembleTypedSchemaCatalog([]byte("{"), nil, "tools"); err == nil {
		t.Fatal("bad catalog envelope must fail")
	}
}

func TestCrossPlatformCoverageBuildSchemaCatalogSnapshotValidationErrors(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	resolved := ResolvedSchemaBuild{
		root:      root,
		bound:     BoundCommandRegistry{},
		effective: EffectiveCommandRegistry{},
		registry: SchemaRegistry{
			Products: []ProductSpec{{
				ID: "sample",
				Tools: []ToolSpec{{
					Identity: contract.ToolIdentitySpec{
						CLIPath: "sample run", CanonicalPath: "sample.run", ProductID: "sample",
						Name: "run", Path: "sample.run", PrimaryCLIPath: "sample run",
					},
					Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
					Selection: contract.SelectionSpec{AgentSummary: "s"},
				}},
			}},
		},
	}
	if _, err := BuildSchemaCatalogSnapshot(ResolvedSchemaBuild{}, SchemaCatalogBuildOptions{}); err == nil || !strings.Contains(err.Error(), "ResolveSchemaBuild") {
		t.Fatalf("nil root snapshot error = %v", err)
	}
	if _, err := BuildSchemaCatalogSnapshot(resolved, SchemaCatalogBuildOptions{RegistryHash: "disagree"}); err == nil || !strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("hash disagree error = %v", err)
	}

	restore := func() {
		buildCatalogValidateParameterBindings = ValidateSchemaParameterBindingDelivery
		buildCatalogValidateDryRun = ValidateReviewedDryRunCapabilityDelivery
		buildCatalogValidateExamples = ValidateAgentExampleDelivery
		buildCatalogValidateCompleteness = validateResolvedRuntimeSchemaCompleteness
		buildCatalogValidateRegistry = validateSchemaRegistryAgainstCommandRegistry
		buildCatalogValidateInterfaces = validateSchemaRegistryInterfaces
		buildCatalogValidateAgentMetadata = validateSchemaRegistryAgentMetadata
		buildCatalogValidateProvenance = validateFinalSchemaProvenanceCoverage
		buildCatalogValidateDelivery = ValidateSchemaDeliveryInvariants
		buildCatalogValidateFinalCompleteness = validateResolvedSchemaCatalogDeliveryCompleteness
	}
	t.Cleanup(restore)

	type hook struct {
		name string
		set  func()
	}
	passBindings := func() {
		buildCatalogValidateParameterBindings = func(BoundCommandRegistry, SchemaRegistry) error { return nil }
	}
	passDryRun := func() {
		buildCatalogValidateDryRun = func(SchemaRegistry) error { return nil }
	}
	passExamples := func() {
		buildCatalogValidateExamples = func(BoundCommandRegistry, SchemaRegistry) (AgentExampleExecutionPlan, error) {
			return AgentExampleExecutionPlan{}, nil
		}
	}
	passCompleteness := func() {
		buildCatalogValidateCompleteness = func(*cobra.Command, BoundCommandRegistry) error { return nil }
	}
	passRegistry := func() {
		buildCatalogValidateRegistry = func(SchemaRegistry, EffectiveCommandRegistry) error { return nil }
	}
	passInterfaces := func() {
		buildCatalogValidateInterfaces = func(SchemaRegistry) error { return nil }
	}
	passAgent := func() {
		buildCatalogValidateAgentMetadata = func(SchemaRegistry) error { return nil }
	}
	passProvenance := func() {
		buildCatalogValidateProvenance = func(SchemaRegistry) error { return nil }
	}
	passUpTo := func(stage string) {
		passBindings()
		if stage == "param" {
			return
		}
		passDryRun()
		if stage == "dryrun" {
			return
		}
		passExamples()
		if stage == "examples" {
			return
		}
		passCompleteness()
		if stage == "completeness" {
			return
		}
		passRegistry()
		if stage == "registry" {
			return
		}
		passInterfaces()
		if stage == "iface" {
			return
		}
		passAgent()
		if stage == "agent" {
			return
		}
		passProvenance()
	}

	hooks := []hook{
		{"param", func() {
			buildCatalogValidateParameterBindings = func(BoundCommandRegistry, SchemaRegistry) error {
				return fmt.Errorf("param boom")
			}
		}},
		{"dryrun", func() {
			passUpTo("param")
			buildCatalogValidateDryRun = func(SchemaRegistry) error { return fmt.Errorf("dryrun boom") }
		}},
		{"examples", func() {
			passUpTo("dryrun")
			buildCatalogValidateExamples = func(BoundCommandRegistry, SchemaRegistry) (AgentExampleExecutionPlan, error) {
				return AgentExampleExecutionPlan{}, fmt.Errorf("examples boom")
			}
		}},
		{"completeness", func() {
			passUpTo("examples")
			buildCatalogValidateCompleteness = func(*cobra.Command, BoundCommandRegistry) error {
				return fmt.Errorf("completeness boom")
			}
		}},
		{"registry", func() {
			passUpTo("completeness")
			buildCatalogValidateRegistry = func(SchemaRegistry, EffectiveCommandRegistry) error {
				return fmt.Errorf("registry boom")
			}
		}},
		{"iface", func() {
			passUpTo("registry")
			buildCatalogValidateInterfaces = func(SchemaRegistry) error { return fmt.Errorf("iface boom") }
		}},
		{"agent", func() {
			passUpTo("iface")
			buildCatalogValidateAgentMetadata = func(SchemaRegistry) error { return fmt.Errorf("agent boom") }
		}},
		{"prov", func() {
			passUpTo("agent")
			buildCatalogValidateProvenance = func(SchemaRegistry) error { return fmt.Errorf("prov boom") }
		}},
		{"delivery", func() {
			passUpTo("prov")
			buildCatalogValidateDelivery = func(SchemaRegistry, SchemaCatalogSnapshot) error {
				return fmt.Errorf("delivery boom")
			}
		}},
		{"final", func() {
			passUpTo("prov")
			buildCatalogValidateDelivery = func(SchemaRegistry, SchemaCatalogSnapshot) error { return nil }
			buildCatalogValidateFinalCompleteness = func(*cobra.Command, BoundCommandRegistry, SchemaCatalogSnapshot) error {
				return fmt.Errorf("final boom")
			}
		}},
	}
	for _, h := range hooks {
		restore()
		h.set()
		if _, err := BuildSchemaCatalogSnapshot(resolved, SchemaCatalogBuildOptions{}); err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("%s error = %v", h.name, err)
		}
	}
}

func TestCrossPlatformCoverageDeliveryInvariantErrorBranches(t *testing.T) {
	path, left, right := firstSchemaJSONDifference(map[string]any{"a": 1}, map[string]any{"a": 2})
	if path == "" || left == "" || right == "" {
		t.Fatalf("map difference empty: %q %q %q", path, left, right)
	}
	path, _, _ = firstSchemaJSONDifference([]any{1}, []any{1, 2})
	if path == "" {
		t.Fatal("list length difference empty")
	}
	path, _, _ = firstSchemaJSONDifference(map[string]any{"k": "v"}, "scalar")
	if path == "" {
		t.Fatal("map vs scalar difference empty")
	}
	path, _, _ = firstSchemaJSONDifference([]any{"x"}, map[string]any{"x": 1})
	if path == "" {
		t.Fatal("list vs map difference empty")
	}
	long := strings.Repeat("x", 300)
	if got := compactSchemaDiagnosticValue(long); !strings.HasSuffix(got, "...") {
		t.Fatalf("compact long value = %q", got)
	}

	prevRegistry := deliveryRegistryPayload
	prevSchema := deliverySchemaPayload
	prevOverview := deliveryOverviewPayload
	t.Cleanup(func() {
		deliveryRegistryPayload = prevRegistry
		deliverySchemaPayload = prevSchema
		deliveryOverviewPayload = prevOverview
	})

	registry := SchemaRegistry{
		Source: SchemaSourceRuntimeAssembled,
		Products: []ProductSpec{{
			ID: "sample",
			Tools: []ToolSpec{{
				Identity: contract.ToolIdentitySpec{
					CLIPath: "sample run", CanonicalPath: "sample.run", ProductID: "sample",
					Name: "run", Path: "sample.run", PrimaryCLIPath: "sample run",
				},
				Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
				Selection: contract.SelectionSpec{AgentSummary: "s"},
			}},
		}},
	}
	payload, err := registry.ToSnapshotPayload()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := SchemaCatalogSnapshot{
		Version:     SchemaCatalogSnapshotVersion,
		SurfaceHash: "sha256:test",
		Catalog:     payload.Catalog,
		Tools:       payload.Tools,
	}
	snapshot.SourceHash = schemaCatalogSnapshotHash(snapshot)

	if err := schemaSourceSnapshotInvariantErrors(registry, SchemaCatalogSnapshot{
		Catalog: map[string]any{"products": []any{}},
		Tools:   map[string]map[string]any{},
	}); len(err) == 0 {
		t.Fatal("mismatched catalog must report invariant problems")
	}

	deliveryRegistryPayload = func(SchemaRegistry) (map[string]any, error) {
		return nil, fmt.Errorf("all boom")
	}
	if problems := schemaDeliveryInvariantErrors(loadedSchemaCatalog{Registry: registry, Snapshot: snapshot}); len(problems) == 0 {
		t.Fatal("expected --all render failure")
	}
	deliveryRegistryPayload = prevRegistry
	deliverySchemaPayload = func(loadedSchemaCatalog, []string) (map[string]any, error) {
		return nil, fmt.Errorf("list boom")
	}
	if problems := schemaDeliveryInvariantErrors(loadedSchemaCatalog{Registry: registry, Snapshot: snapshot}); len(problems) == 0 {
		t.Fatal("expected list query failure")
	}
	deliverySchemaPayload = prevSchema
	deliveryOverviewPayload = func(SchemaRegistry) (map[string]any, error) {
		return nil, fmt.Errorf("overview boom")
	}
	if problems := schemaDeliveryInvariantErrors(loadedSchemaCatalog{Registry: registry, Snapshot: snapshot}); len(problems) == 0 {
		t.Fatal("expected overview failure")
	}
	deliveryOverviewPayload = prevOverview

	broken := snapshot
	broken.Catalog = map[string]any{"products": []any{}}
	if err := ValidateSchemaDeliveryInvariants(registry, broken); err == nil {
		t.Fatal("broken snapshot must fail delivery invariants")
	}
	if err := validateSchemaSnapshotDeliveryInvariants(broken); err == nil {
		t.Fatal("broken snapshot must fail snapshot-only invariants")
	}
}

func TestCrossPlatformCoverageCommandRegistryValidateAndHashEdges(t *testing.T) {
	if _, err := ValidateCommandRegistrySource([]byte(`{`)); err == nil {
		t.Fatal("bad JSON must fail ValidateCommandRegistrySource")
	}
	merged, err := ReviewedCommandRegistryMergedJSON()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateCommandRegistrySource(merged); err != nil {
		t.Fatalf("embedded registry must validate: %v", err)
	}
	var mutated map[string]any
	if err := json.Unmarshal(merged, &mutated); err != nil {
		t.Fatal(err)
	}
	mutated["version"] = 999
	raw, _ := json.Marshal(mutated)
	if _, err := ValidateCommandRegistrySource(raw); err == nil {
		t.Fatal("mutated registry must disagree with embedded")
	}
	hash, err := ReviewedCommandRegistrySourceHash()
	if err != nil || !strings.HasPrefix(hash, "sha256:") {
		t.Fatalf("ReviewedCommandRegistrySourceHash() = %q err=%v", hash, err)
	}
	left := CommandRegistry{Commands: []CommandSpec{{
		CanonicalPath: "a.one", PrimaryCLIPath: "a one", Visibility: SchemaVisibilityPublic,
	}}}
	right := CommandRegistry{Commands: []CommandSpec{{
		CanonicalPath: "a.two", PrimaryCLIPath: "a two", Visibility: SchemaVisibilityPublic,
	}}}
	if equalCommandRegistries(left, right) {
		t.Fatal("different registries must not equal")
	}
	empty := CommandRegistry{}
	if empty.SourceHash() == "" {
		t.Fatal("empty registry hash must be non-empty")
	}
	_ = EffectiveCommandRegistry{Commands: left.Commands}.SourceHash()
}

func TestCrossPlatformCoverageResolveAssembleInjectionErrors(t *testing.T) {
	root := &cobra.Command{Use: "dws"}
	prevEff := resolveEffectiveCommandRegistry
	prevBound := resolveBoundCommandRegistry
	prevAsm := resolveAssembleSchemaRegistry
	prevParam := resolveValidateParameterDelivery
	prevBind := assembleValidateBindings
	prevCollect := assembleCollectEntries
	prevTool := assembleRuntimeToolSpec
	prevTyped := assembleTypedRegistry
	prevMarshal := assembleMarshalRaw
	t.Cleanup(func() {
		resolveEffectiveCommandRegistry = prevEff
		resolveBoundCommandRegistry = prevBound
		resolveAssembleSchemaRegistry = prevAsm
		resolveValidateParameterDelivery = prevParam
		assembleValidateBindings = prevBind
		assembleCollectEntries = prevCollect
		assembleRuntimeToolSpec = prevTool
		assembleTypedRegistry = prevTyped
		assembleMarshalRaw = prevMarshal
	})

	resolveEffectiveCommandRegistry = func(*cobra.Command) (EffectiveCommandRegistry, error) {
		return EffectiveCommandRegistry{}, fmt.Errorf("eff boom")
	}
	if _, err := ResolveSchemaBuild(root); err == nil || !strings.Contains(err.Error(), "eff boom") {
		t.Fatalf("effective error = %v", err)
	}
	resolveEffectiveCommandRegistry = func(*cobra.Command) (EffectiveCommandRegistry, error) {
		return EffectiveCommandRegistry{}, nil
	}
	resolveBoundCommandRegistry = func(*cobra.Command, EffectiveCommandRegistry) (BoundCommandRegistry, error) {
		return BoundCommandRegistry{}, fmt.Errorf("bound boom")
	}
	if _, err := ResolveSchemaBuild(root); err == nil || !strings.Contains(err.Error(), "bound boom") {
		t.Fatalf("bound error = %v", err)
	}
	resolveBoundCommandRegistry = func(*cobra.Command, EffectiveCommandRegistry) (BoundCommandRegistry, error) {
		return BoundCommandRegistry{}, nil
	}
	resolveAssembleSchemaRegistry = func(BoundCommandRegistry) (SchemaRegistry, error) {
		return SchemaRegistry{}, fmt.Errorf("asm boom")
	}
	if _, err := ResolveSchemaBuild(root); err == nil || !strings.Contains(err.Error(), "asm boom") {
		t.Fatalf("assemble error = %v", err)
	}
	resolveAssembleSchemaRegistry = func(BoundCommandRegistry) (SchemaRegistry, error) {
		return SchemaRegistry{}, nil
	}
	resolveValidateParameterDelivery = func(BoundCommandRegistry, SchemaRegistry) error {
		return fmt.Errorf("param boom")
	}
	if _, err := AssembleSchemaRegistry(root); err == nil || !strings.Contains(err.Error(), "param boom") {
		t.Fatalf("param delivery error = %v", err)
	}
	resolveValidateParameterDelivery = prevParam
	resolveAssembleSchemaRegistry = prevAsm

	assembleValidateBindings = func() error { return fmt.Errorf("bindings boom") }
	if _, err := AssembleSchemaRegistryFromBound(BoundCommandRegistry{}); err == nil || !strings.Contains(err.Error(), "bindings boom") {
		t.Fatalf("bindings error = %v", err)
	}
	assembleValidateBindings = func() error { return nil }
	assembleCollectEntries = func(BoundCommandRegistry) ([]runtimeSchemaEntry, error) {
		return nil, fmt.Errorf("collect boom")
	}
	if _, err := assembleSchemaRegistryFromBound(BoundCommandRegistry{}, runtimeSchemaMetadataSources{}); err == nil || !strings.Contains(err.Error(), "collect boom") {
		t.Fatalf("collect error = %v", err)
	}
	assembleCollectEntries = func(BoundCommandRegistry) ([]runtimeSchemaEntry, error) {
		return []runtimeSchemaEntry{{ProductID: "p", ToolName: "t", ProductName: "P", CLIPath: "p t", PrimaryCLIPath: "p t"}}, nil
	}
	// assembleRuntimeToolSpec is only consulted on the production (non-legacy)
	// path; AllowingLegacy calls runtimeToolSpecAllowingLegacy directly.
	assembleRuntimeToolSpec = func(runtimeSchemaEntry, runtimeSchemaMetadataSources) (ToolSpec, error) {
		return ToolSpec{}, fmt.Errorf("tool boom")
	}
	if _, err := assembleSchemaRegistryFromBound(BoundCommandRegistry{}, runtimeSchemaMetadataSources{}); err == nil || !strings.Contains(err.Error(), "tool boom") {
		t.Fatalf("tool error = %v", err)
	}
	t.Cleanup(func() { contract.ClearProductDeclForTest("p") })
	contract.RegisterProductDecl(contract.ProductDecl{
		ID: "p",
		Selection: contract.ProductSelectionDecl{
			AgentSummary: "P product",
			UseWhen:      []string{"p routing"},
			AvoidWhen:    []string{"not p"},
		},
	})
	assembleRuntimeToolSpec = func(runtimeSchemaEntry, runtimeSchemaMetadataSources) (ToolSpec, error) {
		return ToolSpec{
			Identity: contract.ToolIdentitySpec{
				CLIPath: "p t", CanonicalPath: "p.t", ProductID: "p", Name: "t", Path: "p.t", PrimaryCLIPath: "p t",
			},
			Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
			Selection: contract.SelectionSpec{AgentSummary: "s"},
		}, nil
	}
	assembleTypedRegistry = func(string, []ProductSpec) (SchemaRegistry, error) {
		return SchemaRegistry{}, fmt.Errorf("typed boom")
	}
	if _, err := assembleSchemaRegistryFromBound(BoundCommandRegistry{}, runtimeSchemaMetadataSources{}); err == nil || !strings.Contains(err.Error(), "typed boom") {
		t.Fatalf("typed error = %v", err)
	}
	assembleTypedRegistry = func(string, []ProductSpec) (SchemaRegistry, error) {
		return SchemaRegistry{Products: []ProductSpec{{ID: "p"}}}, nil
	}
	calls := 0
	assembleMarshalRaw = func(any) (json.RawMessage, error) {
		calls++
		if calls == 1 {
			return nil, fmt.Errorf("iface boom")
		}
		return json.RawMessage(`{}`), nil
	}
	if _, err := assembleSchemaRegistryFromBound(BoundCommandRegistry{}, runtimeSchemaMetadataSources{}); err == nil || !strings.Contains(err.Error(), "iface boom") {
		t.Fatalf("iface marshal error = %v", err)
	}
	calls = 0
	assembleMarshalRaw = func(any) (json.RawMessage, error) {
		calls++
		if calls == 2 {
			return nil, fmt.Errorf("agent boom")
		}
		return json.RawMessage(`{}`), nil
	}
	if _, err := assembleSchemaRegistryFromBound(BoundCommandRegistry{}, runtimeSchemaMetadataSources{}); err == nil || !strings.Contains(err.Error(), "agent boom") {
		t.Fatalf("agent marshal error = %v", err)
	}
}

func TestCrossPlatformCoverageDeliveryInvariantProjectionMismatches(t *testing.T) {
	prevIndex := deliveryIndexResolve
	prevTool := deliveryToolPayload
	prevSummary := deliveryToolSummary
	prevSchema := deliverySchemaPayload
	prevRegistry := deliveryRegistryPayload
	prevOverview := deliveryOverviewPayload
	t.Cleanup(func() {
		deliveryIndexResolve = prevIndex
		deliveryToolPayload = prevTool
		deliveryToolSummary = prevSummary
		deliverySchemaPayload = prevSchema
		deliveryRegistryPayload = prevRegistry
		deliveryOverviewPayload = prevOverview
	})

	registry := SchemaRegistry{
		Source: SchemaSourceRuntimeAssembled,
		Products: []ProductSpec{{
			ID: "sample",
			Tools: []ToolSpec{{
				Identity: contract.ToolIdentitySpec{
					CLIPath: "sample run", CanonicalPath: "sample.run", ProductID: "sample",
					Name: "run", Path: "sample.run", PrimaryCLIPath: "sample run",
					Aliases: []string{"sample r"},
				},
				Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
				Selection: contract.SelectionSpec{AgentSummary: "s"},
			}},
		}},
	}
	index, err := registry.Index()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := registry.ToSnapshotPayload()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := SchemaCatalogSnapshot{
		Version:     SchemaCatalogSnapshotVersion,
		SurfaceHash: "sha256:surface",
		Catalog:     payload.Catalog,
		Tools:       payload.Tools,
	}
	snapshot.SourceHash = schemaCatalogSnapshotHash(snapshot)
	loaded := loadedSchemaCatalog{Snapshot: snapshot, Registry: registry, Index: index}

	deliveryIndexResolve = func(SchemaIndex, string) (ToolSpec, bool) { return ToolSpec{}, false }
	if problems := schemaDeliveryInvariantErrors(loaded); len(problems) == 0 {
		t.Fatal("lost index resolve must report")
	}
	deliveryIndexResolve = prevIndex
	deliveryToolPayload = func(ToolSpec) (map[string]any, error) { return nil, fmt.Errorf("tool boom") }
	if problems := schemaDeliveryInvariantErrors(loaded); len(problems) == 0 {
		t.Fatal("tool payload failure must report")
	}
	deliveryToolPayload = prevTool
	deliveryToolSummary = func(ToolSpec) (map[string]any, error) { return nil, fmt.Errorf("summary boom") }
	if problems := schemaDeliveryInvariantErrors(loaded); len(problems) == 0 {
		t.Fatal("summary failure must report")
	}
	deliveryToolSummary = prevSummary

	mutated := loaded
	mutated.Snapshot.Tools = map[string]map[string]any{}
	if problems := schemaDeliveryInvariantErrors(mutated); len(problems) == 0 {
		t.Fatal("missing full tools must report")
	}
	mutated = loaded
	mutated.Snapshot.SurfaceHash = ""
	deliverySchemaPayload = func(loadedSchemaCatalog, []string) (map[string]any, error) {
		return map[string]any{"catalog_hash": loaded.Snapshot.SourceHash, "surface_hash": "extra", "products": []any{}}, nil
	}
	if problems := schemaDeliveryInvariantErrors(mutated); len(problems) == 0 {
		t.Fatal("unexpected surface_hash must report")
	}
	deliverySchemaPayload = func(loadedSchemaCatalog, []string) (map[string]any, error) {
		return map[string]any{"catalog_hash": "wrong", "products": []any{}}, nil
	}
	mutated = loaded
	if problems := schemaDeliveryInvariantErrors(mutated); len(problems) == 0 {
		t.Fatal("catalog_hash mismatch must report")
	}
	deliverySchemaPayload = prevSchema
	deliveryOverviewPayload = func(SchemaRegistry) (map[string]any, error) {
		return map[string]any{"kind": "schema", "level": "products", "count": 0, "tool_count": 0, "products": []any{}}, nil
	}
	if problems := schemaDeliveryInvariantErrors(loaded); len(problems) == 0 {
		t.Fatal("overview mismatch must report")
	}
	deliveryOverviewPayload = prevOverview

	if problem := schemaAliasViewProblem(map[string]any{"cli_path": "a"}, map[string]any{"cli_path": "b", "is_alias": true}, "a"); problem == "" {
		t.Fatal("alias cli_path mismatch must report")
	}
	if problem := schemaAliasViewProblem(map[string]any{"cli_path": "a"}, map[string]any{"cli_path": "a"}, "a"); problem == "" {
		t.Fatal("missing is_alias must report")
	}
	_, problems := schemaDeliveryToolsByCanonical(map[string]any{
		"products": []any{map[string]any{"tools": []any{map[string]any{"name": "x"}, map[string]any{"canonical_path": "a.b"}, map[string]any{"canonical_path": "a.b"}}}},
	}, "view")
	if len(problems) < 2 {
		t.Fatalf("duplicate/missing canonical problems = %v", problems)
	}
	_ = schemaOverviewPayloadFromCatalog(map[string]any{
		"kind": "schema", "source": "runtime",
		"products": []any{
			map[string]any{"id": "p1", "agent_summary": "s", "tools": []any{map[string]any{"canonical_path": "p1.t"}}},
			map[string]any{"id": "p2", "use_when": []any{"u"}, "tools": []any{}},
			map[string]any{"id": "p3", "description": "d", "tools": []any{}},
		},
	})
}

func TestCrossPlatformCoverageCompletenessValidSnapshotReportBranches(t *testing.T) {
	prevReport := completenessDeliveryReport
	prevCollect := completenessCollectEntries
	prevIface := loadCatalogValidateInterfaces
	prevProv := loadCatalogValidateProvenance
	t.Cleanup(func() {
		completenessDeliveryReport = prevReport
		completenessCollectEntries = prevCollect
		loadCatalogValidateInterfaces = prevIface
		loadCatalogValidateProvenance = prevProv
	})
	loadCatalogValidateInterfaces = func(SchemaRegistry) error { return nil }
	loadCatalogValidateProvenance = func(SchemaRegistry) error { return nil }

	registry := SchemaRegistry{
		Source: SchemaSourceRuntimeAssembled,
		Products: []ProductSpec{{
			ID: "sample",
			Tools: []ToolSpec{{
				Identity: contract.ToolIdentitySpec{
					CLIPath: "sample run", CanonicalPath: "sample.run", ProductID: "sample",
					Name: "run", Path: "sample.run", PrimaryCLIPath: "sample run",
				},
				Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
				Selection: contract.SelectionSpec{AgentSummary: "s"},
			}},
		}},
	}
	payload, err := registry.ToSnapshotPayload()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := SchemaCatalogSnapshot{
		Version: SchemaCatalogSnapshotVersion,
		Catalog: payload.Catalog,
		Tools:   payload.Tools,
	}
	snapshot.SourceHash = schemaCatalogSnapshotHash(snapshot)

	root := &cobra.Command{Use: "dws"}
	for _, tc := range []struct {
		report RuntimeSchemaCompletenessReport
		want   string
	}{
		{RuntimeSchemaCompletenessReport{DeliveryErrors: []string{"snap"}}, "snap"},
		{RuntimeSchemaCompletenessReport{Missing: []string{"m"}}, "missing"},
		{RuntimeSchemaCompletenessReport{InvalidExclusions: []string{"i"}}, "invalid"},
		{RuntimeSchemaCompletenessReport{StaleExclusions: []string{"s"}}, "stale"},
	} {
		completenessDeliveryReport = func(*cobra.Command, loadedSchemaCatalog, []RuntimeSchemaExclusion, BoundCommandRegistry) RuntimeSchemaCompletenessReport {
			return tc.report
		}
		if err := validateSchemaCatalogDeliveryCompletenessFromBound(root, BoundCommandRegistry{}, snapshot, nil); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("report %q error = %v", tc.want, err)
		}
	}

	completenessCollectEntries = func(*cobra.Command) ([]runtimeSchemaEntry, error) {
		return nil, fmt.Errorf("collect boom")
	}
	report := RuntimeSchemaCompleteness(root, nil)
	if len(report.DeliveryErrors) == 0 || !strings.Contains(report.DeliveryErrors[0], "collect boom") {
		t.Fatalf("collect delivery errors = %#v", report.DeliveryErrors)
	}
}

func TestCrossPlatformCoverageRuntimeParameterMetadataApply(t *testing.T) {
	canonical := "coverage.param.meta." + t.Name()
	t.Cleanup(func() {
		delete(runtimeSchemaParameterMetadataByCanonical, canonical)
	})
	RegisterRuntimeSchemaParameterMetadata(canonical, RuntimeSchemaParameterMetadata{
		Required:     []string{"name", "missing"},
		RequiredWhen: map[string]string{"name": "other=true"},
		Formats:      map[string]string{"name": "uuid"},
		Enums:        map[string][]string{"name": {"a", "b"}},
		Examples:     map[string]string{"name": "demo"},
	})
	cmd := &cobra.Command{Use: "meta"}
	cmd.Flags().String("name", "", "name")
	applyRuntimeSchemaParameterMetadata(cmd, canonical)
	applyRuntimeSchemaParameterMetadata(cmd, "coverage.param.meta.absent")
	defs := RuntimeSchemaParameterMetadataDefinitions()
	if _, ok := defs[canonical]; !ok {
		t.Fatal("definitions missing registered metadata")
	}
	RegisterRuntimeSchemaParameterMetadata("", RuntimeSchemaParameterMetadata{Required: []string{"x"}})
}

func TestOverallCoverageGapSchemaCommandAndFieldResolve(t *testing.T) {
	cmd := NewSchemaCommand(nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"drive", "--cli-path", "drive list"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("cli-path with positional arg must fail")
	}
	cmd = NewSchemaCommand(nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"drive", "--all"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("--all with path must fail")
	}
	prevCatalog := schemaCommandCatalogError
	t.Cleanup(func() { schemaCommandCatalogError = prevCatalog })
	schemaCommandCatalogError = func() error { return fmt.Errorf("catalog boom") }
	cmd = NewSchemaCommand(nil)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"list"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "catalog boom") {
		t.Fatalf("catalog load error = %v", err)
	}

	left := runtimeSchemaStringCandidateAtPriority("one", true, "same_src", 5, "same_p")
	right := runtimeSchemaStringCandidateAtPriority("two", true, "same_src", 5, "same_p")
	if _, err := resolveRuntimeSchemaCandidate("field", left, right); err == nil {
		t.Fatal("equal-rank value conflict must fail")
	}
	sameA := runtimeSchemaStringCandidateAtPriority("same", true, "same_src", 5, "same_p")
	sameA.ReviewReason = "z-reason"
	sameB := runtimeSchemaStringCandidateAtPriority("same", true, "same_src", 5, "same_p")
	sameB.ReviewReason = "a-reason"
	if winner, err := resolveRuntimeSchemaCandidate("order", sameA, sameB); err != nil || winner.ReviewReason != "a-reason" {
		t.Fatalf("review-reason tie-break = %#v err=%v", winner, err)
	}

	schemaCommandCatalogError = prevCatalog
	// Delivery is installed by TestMain; exercise success + compact branches.
	for _, args := range [][]string{
		{},
		{"--all"},
		{"--cli-path", "dev", "--compact"},
	} {
		success := NewSchemaCommand(nil)
		success.SetOut(&bytes.Buffer{})
		success.SetErr(&bytes.Buffer{})
		success.SetArgs(args)
		if err := success.Execute(); err != nil {
			t.Fatalf("schema %v error = %v", args, err)
		}
	}

	prevResolver := resolveRuntimeSchemaField
	t.Cleanup(func() { resolveRuntimeSchemaField = prevResolver })
	resolveRuntimeSchemaField = func(string, ...runtimeSchemaFieldCandidate) (runtimeSchemaFieldCandidate, error) {
		return runtimeSchemaFieldCandidate{
			Value: false, Present: true, Source: "reviewed",
			Compared: []runtimeSchemaFieldCandidate{
				{Value: false, Present: true, Source: "reviewed"},
				{Value: true, Present: true, Source: "cobra_hard_required"},
				{Value: false, Present: true, Source: "other"},
			},
		}, nil
	}
	floor, err := resolveRequiredProjection(true)
	if err != nil || floor.Value != true || floor.Resolution != "cobra_hard_required_floor" {
		t.Fatalf("cobra hard floor = %#v err=%v", floor, err)
	}
	resolveRuntimeSchemaField = func(string, ...runtimeSchemaFieldCandidate) (runtimeSchemaFieldCandidate, error) {
		return runtimeSchemaCandidate(true, true, "reviewed"), nil
	}
	if got, err := resolveRequiredProjection(true); err != nil || got.Value != true {
		t.Fatalf("already-required projection = %#v err=%v", got, err)
	}
	if got, err := resolveRequiredProjection(false); err != nil || got.Value != true {
		t.Fatalf("soft required projection = %#v err=%v", got, err)
	}
}

func TestOverallCoverageGapDeliveryCompletenessAndDryRun(t *testing.T) {
	prevIndex := deliveryIndexResolve
	prevTool := deliveryToolPayload
	prevSummary := deliveryToolSummary
	prevSchema := deliverySchemaPayload
	prevRegistry := deliveryRegistryPayload
	t.Cleanup(func() {
		deliveryIndexResolve = prevIndex
		deliveryToolPayload = prevTool
		deliveryToolSummary = prevSummary
		deliverySchemaPayload = prevSchema
		deliveryRegistryPayload = prevRegistry
	})

	registry := SchemaRegistry{
		Source: SchemaSourceRuntimeAssembled,
		Products: []ProductSpec{{
			ID: "sample",
			Tools: []ToolSpec{{
				Identity: contract.ToolIdentitySpec{
					CLIPath: "sample group run", CanonicalPath: "sample.group.run", ProductID: "sample",
					Name: "group.run", Path: "sample.group.run", PrimaryCLIPath: "sample group run",
					Aliases: []string{"sample group r"},
				},
				Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
				Selection: contract.SelectionSpec{AgentSummary: "s"},
			}},
		}},
	}
	index, err := registry.Index()
	if err != nil {
		t.Fatal(err)
	}
	payload, err := registry.ToSnapshotPayload()
	if err != nil {
		t.Fatal(err)
	}
	snapshot := SchemaCatalogSnapshot{
		Version: SchemaCatalogSnapshotVersion, Catalog: payload.Catalog, Tools: payload.Tools,
	}
	snapshot.SourceHash = schemaCatalogSnapshotHash(snapshot)
	loaded := loadedSchemaCatalog{Snapshot: snapshot, Registry: registry, Index: index}
	root := &cobra.Command{Use: "dws"}
	sample := &cobra.Command{Use: "sample"}
	group := &cobra.Command{Use: "group"}
	run := &cobra.Command{Use: "run", Run: func(*cobra.Command, []string) {}}
	group.AddCommand(run)
	sample.AddCommand(group)
	root.AddCommand(sample)

	identity := map[string]runtimeSchemaResolvedIdentity{
		"sample group run": {CanonicalPath: "sample.group.run", Source: "reviewed"},
		"sample group r":   {CanonicalPath: "other.run", Source: "reviewed"},
	}
	deliveryIndexResolve = func(SchemaIndex, string) (ToolSpec, bool) { return ToolSpec{}, false }
	report := schemaCatalogDeliveryCompletenessAgainstLoadedAndIdentity(root, loaded, nil, identity, []string{"map boom"})
	if len(report.DeliveryErrors) == 0 {
		t.Fatal("lost index / mapping errors must surface")
	}
	deliveryIndexResolve = prevIndex
	deliverySchemaPayload = func(loadedSchemaCatalog, []string) (map[string]any, error) {
		return nil, fmt.Errorf("query boom")
	}
	report = schemaCatalogDeliveryCompletenessAgainstLoadedAndIdentity(root, loaded, nil, map[string]runtimeSchemaResolvedIdentity{
		"sample group run": {CanonicalPath: "sample.group.run", Source: "reviewed"},
	}, nil)
	if len(report.DeliveryErrors) == 0 {
		t.Fatal("query failures must surface in completeness")
	}
	deliverySchemaPayload = func(_ loadedSchemaCatalog, args []string) (map[string]any, error) {
		return map[string]any{"canonical_path": "wrong." + strings.Join(args, ".")}, nil
	}
	report = schemaCatalogDeliveryCompletenessAgainstLoadedAndIdentity(root, loaded, nil, map[string]runtimeSchemaResolvedIdentity{
		"sample group run": {CanonicalPath: "sample.group.run", Source: "reviewed"},
	}, nil)
	if len(report.DeliveryErrors) == 0 {
		t.Fatal("canonical mismatch must surface")
	}
	deliverySchemaPayload = prevSchema
	deliveryToolPayload = func(ToolSpec) (map[string]any, error) { return nil, fmt.Errorf("render boom") }
	report = schemaCatalogDeliveryCompletenessAgainstLoadedAndIdentity(root, loaded, nil, map[string]runtimeSchemaResolvedIdentity{
		"sample group run": {CanonicalPath: "sample.group.run", Source: "reviewed"},
	}, nil)
	if len(report.DeliveryErrors) == 0 {
		t.Fatal("tool render failures must surface")
	}
	deliveryToolPayload = prevTool

	deliveryRegistryPayload = func(SchemaRegistry) (map[string]any, error) { return nil, fmt.Errorf("all boom") }
	if problems := schemaRegistryProjectionErrors(loaded); len(problems) == 0 {
		t.Fatal("--all render failure must surface")
	}
	deliveryRegistryPayload = func(SchemaRegistry) (map[string]any, error) {
		return map[string]any{"products": []any{map[string]any{"tools": []any{
			map[string]any{"name": "missing"},
			map[string]any{"canonical_path": "sample.group.run"},
			map[string]any{"canonical_path": "sample.group.run"},
		}}}}, nil
	}
	if problems := schemaRegistryProjectionErrors(loaded); len(problems) < 2 {
		t.Fatalf("duplicate/missing canonical problems = %v", problems)
	}
	deliveryRegistryPayload = prevRegistry
	deliveryToolPayload = func(ToolSpec) (map[string]any, error) { return nil, fmt.Errorf("tool boom") }
	if problems := schemaRegistryProjectionErrors(loaded); len(problems) == 0 {
		t.Fatal("tool projection render failure must surface")
	}
	deliveryToolPayload = prevTool
	deliveryToolSummary = func(ToolSpec) (map[string]any, error) { return nil, fmt.Errorf("summary boom") }
	if problems := schemaRegistryProjectionErrors(loaded); len(problems) == 0 {
		t.Fatal("group summary render failure must surface")
	}
	deliveryToolSummary = prevSummary

	bound := BoundCommandRegistry{Commands: []BoundCommandSpec{
		{CommandSpec: CommandSpec{CanonicalPath: "a.one", PrimaryCLIPath: "a one", Visibility: SchemaVisibilityPublic, Source: "s"}},
		{CommandSpec: CommandSpec{CanonicalPath: "a.two", PrimaryCLIPath: "a one", Visibility: SchemaVisibilityPublic, Source: "s"}},
		{CommandSpec: CommandSpec{CanonicalPath: "b.hidden", PrimaryCLIPath: "b hide", Visibility: SchemaVisibilityInternal, Source: "s"}},
		{CommandSpec: CommandSpec{CanonicalPath: "c.blank", PrimaryCLIPath: " ", Visibility: SchemaVisibilityPublic, Source: "s"}},
	}}
	if _, conflicts := runtimeSchemaIdentityByBound(bound); len(conflicts) == 0 {
		t.Fatal("path ownership conflicts must surface")
	}

	t.Cleanup(clearDeclaredDryRunCapabilitiesForTest)
	restore := setReviewedDryRunCapabilityGroupsForTest([]dryRunCapabilityGroup{
		{PreviewKind: contract.DryRunPreviewPlan, CanonicalPaths: []string{"gap.run"}},
		{PreviewKind: contract.DryRunPreviewPlan, CanonicalPaths: []string{"gap.run"}},
	})
	t.Cleanup(restore)
	if _, err := loadManualDryRunCapabilities(); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("cross-group duplicate dry-run error = %v", err)
	}
	restore = setReviewedDryRunCapabilityGroupsForTest([]dryRunCapabilityGroup{
		{PreviewKind: contract.DryRunPreviewPlan, CanonicalPaths: []string{"gap.run"}},
	})
	t.Cleanup(restore)
	if err := ValidateReviewedDryRunCapabilityDelivery(SchemaRegistry{}); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("missing dry-run delivery error = %v", err)
	}
	mismatch := SchemaRegistry{Products: []ProductSpec{{Tools: []ToolSpec{{
		Identity: contract.ToolIdentitySpec{CanonicalPath: "gap.run"},
		DryRun:   &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewDiff},
	}}}}}
	if err := ValidateReviewedDryRunCapabilityDelivery(mismatch); err == nil || !strings.Contains(err.Error(), "dry-run capability") {
		t.Fatalf("mismatched dry-run delivery error = %v", err)
	}
	unreviewed := SchemaRegistry{Products: []ProductSpec{{Tools: []ToolSpec{{
		Identity: contract.ToolIdentitySpec{CanonicalPath: "other.run"},
		DryRun:   &contract.DryRunSpec{PreviewKind: contract.DryRunPreviewPlan},
	}}}}}
	if err := ValidateReviewedDryRunCapabilityDelivery(unreviewed); err == nil || !strings.Contains(err.Error(), "unreviewed") {
		t.Fatalf("unreviewed dry-run delivery error = %v", err)
	}

	walkLeafCommands(root, func(*cobra.Command) {})
	hidden := &cobra.Command{Use: "hidden", Hidden: true}
	root.AddCommand(hidden)
	walkLeafCommands(root, func(*cobra.Command) {})
	if hasRuntimeSchemaCommand(nil) {
		t.Fatal("nil command must not report runtime schema")
	}
	_ = agentMetadataSummaryFrom(agentMetadata{
		Version: 1, SourceHash: "h", SurfaceHash: "s",
		Products: map[string]agentProductMetadata{"p": {}},
		Tools:    map[string]agentToolMetadata{"t": {}},
		Coverage: agentMetadataCoverage{
			SurfaceProducts: 1, SurfaceTools: 2, ToolsWithSummary: 3, UnmatchedSkillTools: 4,
		},
	})

	merged, err := ReviewedCommandRegistryMergedJSON()
	if err != nil {
		t.Fatal(err)
	}
	prevLoad := loadReviewedCommandRegistry
	t.Cleanup(func() { loadReviewedCommandRegistry = prevLoad })
	loadReviewedCommandRegistry = func() (CommandRegistry, error) { return CommandRegistry{}, fmt.Errorf("embed boom") }
	if _, err := ValidateCommandRegistrySource(merged); err == nil || !strings.Contains(err.Error(), "embed boom") {
		t.Fatalf("embed load error = %v", err)
	}
	alt, err := newCommandRegistry([]CommandSpec{{
		CanonicalPath: "gap.only", SourceProductID: "gap", PrimaryCLIPath: "gap only", Visibility: SchemaVisibilityPublic,
	}})
	if err != nil {
		t.Fatal(err)
	}
	loadReviewedCommandRegistry = func() (CommandRegistry, error) { return alt, nil }
	if _, err := ValidateCommandRegistrySource(merged); err == nil || !strings.Contains(err.Error(), "disagrees") {
		t.Fatalf("semantic disagree error = %v", err)
	}

	prevBind := schemaParameterBindingData
	t.Cleanup(func() { schemaParameterBindingData = prevBind })
	schemaParameterBindingData = func() (schemaParameterBindingSnapshot, error) {
		return schemaParameterBindingSnapshot{Bindings: map[string]map[string]string{"a.b": {"flag": "prop"}}}, nil
	}
	if got, err := LoadSchemaParameterBindings(); err != nil || got["a.b"]["flag"] != "prop" {
		t.Fatalf("LoadSchemaParameterBindings() = %#v err=%v", got, err)
	}

	if _, err := buildInterfaceRegistry(map[string]embeddedMCPToolMetadata{"": {}}); err == nil {
		t.Fatal("empty canonical interface registry must fail")
	}
	if _, err := buildInterfaceRegistry(map[string]embeddedMCPToolMetadata{"a.b": {}}); err == nil {
		t.Fatal("missing interface_ref must fail")
	}
	if _, err := buildInterfaceRegistry(map[string]embeddedMCPToolMetadata{
		"a.b":   {InterfaceRef: &embeddedMCPInterfaceRef{ProductID: "a", RPCName: "b"}},
		" a.b ": {InterfaceRef: &embeddedMCPInterfaceRef{ProductID: "a", RPCName: "b"}},
	}); err == nil {
		t.Fatal("conflicting canonical paths must fail")
	}
	if _, err := buildInterfaceRegistry(map[string]embeddedMCPToolMetadata{
		"a.b": {InterfaceRef: &embeddedMCPInterfaceRef{ProductID: "", RPCName: "b"}},
	}); err == nil {
		t.Fatal("incomplete interface_ref must fail")
	}
	if err := validateSchemaRegistryInterfacesWithMetadata(SchemaRegistry{Products: []ProductSpec{{
		Tools: []ToolSpec{{Identity: contract.ToolIdentitySpec{}, Interface: contract.InterfaceSpec{Mode: "bogus"}}},
	}}}, embeddedMCPMetadata{}); err == nil {
		t.Fatal("invalid interface disposition must fail")
	}
	if err := validateToolInterfaceRef("x", "mcp", nil, InterfaceRegistry{}); err == nil {
		t.Fatal("nil interface_ref must fail")
	}
}

func TestOverallCoverageGapRuntimeParamsAndAgentMetadata(t *testing.T) {
	prevSpecs := runtimeCommandParameterSpecsForPayload
	t.Cleanup(func() { runtimeCommandParameterSpecsForPayload = prevSpecs })
	runtimeCommandParameterSpecsForPayload = func(*cobra.Command, string, map[string]embeddedMCPParamMeta, RuntimeSchemaConstraints) ([]ParameterSpec, error) {
		return nil, fmt.Errorf("specs boom")
	}
	if _, err := runtimeCommandParameters(&cobra.Command{Use: "run"}, "sample.run", nil, RuntimeSchemaConstraints{}); err == nil {
		t.Fatal("parameter specs error must surface")
	}
	runtimeCommandParameterSpecsForPayload = func(*cobra.Command, string, map[string]embeddedMCPParamMeta, RuntimeSchemaConstraints) ([]ParameterSpec, error) {
		return []ParameterSpec{{Name: "ok", Type: "string"}}, nil
	}
	payload, err := runtimeCommandParameters(&cobra.Command{Use: "run"}, "sample.run", nil, RuntimeSchemaConstraints{})
	if err != nil || payload["ok"] == nil {
		t.Fatalf("parameter payload = %#v err=%v", payload, err)
	}

	flags := &cobra.Command{Use: "run"}
	flags.Flags().String("need", "", "required value")
	_ = flags.MarkFlagRequired("need")
	flag := flags.Flags().Lookup("need")
	setFlagAnnotation(flag, runtimeSchemaFlagRequiredAnnotation, "not-bool")
	if required, present := runtimeFlagRequiredState(flag); !required || !present {
		t.Fatalf("cobra hard required fallback = %v/%v", required, present)
	}

	if _, err := collectRuntimeSchemaEntriesFromBound(BoundCommandRegistry{Commands: []BoundCommandSpec{
		{CommandSpec: CommandSpec{CanonicalPath: "bad", Visibility: SchemaVisibilityPublic, PrimaryCLIPath: "bad"}},
	}}); err == nil {
		t.Fatal("invalid canonical must fail collect")
	}
	hidden := &cobra.Command{Use: "hide", Run: func(*cobra.Command, []string) {}}
	entries, err := collectRuntimeSchemaEntriesFromBound(BoundCommandRegistry{Commands: []BoundCommandSpec{
		{CommandSpec: CommandSpec{CanonicalPath: "sample.hide", Visibility: SchemaVisibilityInternal, PrimaryCLIPath: "sample hide"}, PrimaryCommand: hidden},
	}})
	if err != nil || len(entries) != 0 {
		t.Fatalf("internal visibility entries = %#v err=%v", entries, err)
	}

	tool := ToolSpec{
		Identity: contract.ToolIdentitySpec{
			CLIPath: "sample group run", CanonicalPath: "sample.group.run", ProductID: "sample",
			Name: "group.run", Path: "sample.group.run", PrimaryCLIPath: "sample group run",
			Aliases: []string{"ghost group run"},
		},
		Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
		Selection: contract.SelectionSpec{AgentSummary: "s"},
	}
	registry := SchemaRegistry{Source: SchemaSourceRuntimeAssembled, Products: []ProductSpec{{ID: "sample", Tools: []ToolSpec{tool}}}}
	index, err := registry.Index()
	if err != nil {
		t.Fatal(err)
	}
	payloadSnap, err := registry.ToSnapshotPayload()
	if err != nil {
		t.Fatal(err)
	}
	loaded := loadedSchemaCatalog{
		Registry: registry, Index: index,
		Snapshot: SchemaCatalogSnapshot{Version: SchemaCatalogSnapshotVersion, Catalog: payloadSnap.Catalog, Tools: payloadSnap.Tools},
	}
	loaded.Snapshot.SourceHash = schemaCatalogSnapshotHash(loaded.Snapshot)
	if problems := schemaRegistryProjectionErrors(loaded); len(problems) == 0 {
		t.Fatal("ghost group product miss must surface")
	}

	prevMeta := finalSchemaAgentMetadata
	t.Cleanup(func() { finalSchemaAgentMetadata = prevMeta })
	finalSchemaAgentMetadata = func() agentMetadata {
		return agentMetadata{Tools: map[string]agentToolMetadata{
			"sample.group.run": {},
			"sample group run": {},
		}}
	}
	if err := validateSchemaRegistryAgentMetadata(registry); err == nil || !strings.Contains(err.Error(), "both resolve") {
		t.Fatalf("duplicate agent metadata keys error = %v", err)
	}
	finalSchemaAgentMetadata = func() agentMetadata {
		return agentMetadata{Tools: map[string]agentToolMetadata{"sample.group.run": {}}}
	}
	if err := validateSchemaRegistryAgentMetadata(registry); err != nil {
		t.Fatalf("matching agent metadata should pass: %v", err)
	}

	badTool := tool
	badTool.Identity.CanonicalPath = "sample.run"
	if err := validateFinalSchemaProvenanceCoverage(SchemaRegistry{Products: []ProductSpec{{
		ID: "sample", Selection: contract.SelectionSpec{AgentSummary: "p", UseWhen: []string{"u"}, AvoidWhen: []string{"a"}},
		Tools: []ToolSpec{badTool},
	}}}); err == nil {
		t.Fatal("invalid tool provenance coverage must fail")
	}

	restore := setReviewedDryRunCapabilityGroupsForTest([]dryRunCapabilityGroup{
		{PreviewKind: "bogus", CanonicalPaths: []string{"gap.run"}},
	})
	t.Cleanup(restore)
	if _, err := reviewedDryRunCapability("gap.run"); err == nil {
		t.Fatal("invalid dry-run preview kind must fail capability load")
	}
	if err := ValidateReviewedDryRunCapabilityDelivery(SchemaRegistry{}); err == nil {
		t.Fatal("invalid dry-run registry must fail delivery validation")
	}
}
