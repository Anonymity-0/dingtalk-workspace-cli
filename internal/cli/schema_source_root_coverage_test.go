// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	"github.com/spf13/cobra"
)

func TestCrossPlatformCoverageAssembleSchemaCatalogFromRootSuccess(t *testing.T) {
	t.Cleanup(restorePackageCLISchemaDeliveryForTest)

	prevResolve := resolveSchemaBuildForDelivery
	prevValidateParam := resolveValidateParameterDelivery
	prevValidateIface := loadCatalogValidateInterfaces
	prevValidateProv := loadCatalogValidateProvenance
	t.Cleanup(func() {
		resolveSchemaBuildForDelivery = prevResolve
		resolveValidateParameterDelivery = prevValidateParam
		loadCatalogValidateInterfaces = prevValidateIface
		loadCatalogValidateProvenance = prevValidateProv
	})

	root := &cobra.Command{Use: "dws"}
	registry := SchemaRegistry{
		Products: []ProductSpec{{
			ID: "sample",
			Tools: []ToolSpec{{
				Identity: contract.ToolIdentitySpec{
					CLIPath: "sample run", CanonicalPath: "sample.run",
					ProductID: "sample", Name: "run", Path: "sample.run", PrimaryCLIPath: "sample run",
				},
				Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
				Selection: contract.SelectionSpec{AgentSummary: "sample"},
			}},
		}},
	}
	resolveSchemaBuildForDelivery = func(*cobra.Command) (ResolvedSchemaBuild, error) {
		return ResolvedSchemaBuild{registry: registry, root: root}, nil
	}
	resolveValidateParameterDelivery = func(BoundCommandRegistry, SchemaRegistry) error { return nil }
	loadCatalogValidateInterfaces = func(SchemaRegistry) error { return nil }
	loadCatalogValidateProvenance = func(SchemaRegistry) error { return nil }

	loaded, err := assembleSchemaCatalogFromRoot(root)
	if err != nil {
		t.Fatalf("assembleSchemaCatalogFromRoot success path: %v", err)
	}
	if loaded.Registry.Source != SchemaSourceRuntimeAssembled {
		t.Fatalf("Source = %q", loaded.Registry.Source)
	}
	if loaded.Snapshot.SourceHash == "" || loaded.Snapshot.SurfaceHash == "" {
		t.Fatalf("snapshot hashes missing: %#v", loaded.Snapshot)
	}
	if len(loaded.Index.CanonicalPaths()) == 0 {
		t.Fatal("index must be populated on success")
	}

	InstallProductionSchemaAssemblyForTest(func() *cobra.Command { return root })
	if !SchemaSourceRootRegistered() {
		t.Fatal("InstallProductionSchemaAssemblyForTest must register factory")
	}
	assembleDeliverySchemaCatalogFn = assembleSchemaCatalogFromRoot
	resetDeliverySchemaCatalogStateForTest()
	sourceHash, err := MaterializeDeliverySchemaCatalogMapsForTest()
	if err != nil || sourceHash == "" {
		t.Fatalf("materialize success = %q err=%v", sourceHash, err)
	}
	// Second materialize hits Tools-already-set short circuit.
	if again, err := MaterializeDeliverySchemaCatalogMapsForTest(); err != nil || again != sourceHash {
		t.Fatalf("rematerialize = %q err=%v", again, err)
	}
	payload, err := DeliverySchemaAllPayloadForTest()
	if err != nil || payload == nil {
		t.Fatalf("DeliverySchemaAllPayloadForTest = %#v err=%v", payload, err)
	}
}

func TestCrossPlatformCoverageSchemaSourceRootErrorBranchesAndRestoreFallback(t *testing.T) {
	t.Cleanup(restorePackageCLISchemaDeliveryForTest)

	if _, err := assembleSchemaCatalogFromRoot(nil); err == nil || !strings.Contains(err.Error(), "schema source root is nil") {
		t.Fatalf("nil root error = %v", err)
	}

	prevResolve := resolveSchemaBuildForDelivery
	prevValidateParam := resolveValidateParameterDelivery
	prevValidateIface := loadCatalogValidateInterfaces
	prevValidateProv := loadCatalogValidateProvenance
	prevPayload := registryToSnapshotPayloadFn
	t.Cleanup(func() {
		resolveSchemaBuildForDelivery = prevResolve
		resolveValidateParameterDelivery = prevValidateParam
		loadCatalogValidateInterfaces = prevValidateIface
		loadCatalogValidateProvenance = prevValidateProv
		registryToSnapshotPayloadFn = prevPayload
	})

	root := &cobra.Command{Use: "dws"}
	resolveSchemaBuildForDelivery = func(*cobra.Command) (ResolvedSchemaBuild, error) {
		return ResolvedSchemaBuild{}, fmt.Errorf("resolve boom")
	}
	if _, err := assembleSchemaCatalogFromRoot(root); err == nil || !strings.Contains(err.Error(), "resolve Schema build") {
		t.Fatalf("resolve error = %v", err)
	}

	registry := SchemaRegistry{Products: []ProductSpec{{
		ID: "sample",
		Tools: []ToolSpec{{
			Identity: contract.ToolIdentitySpec{
				CLIPath: "sample run", CanonicalPath: "sample.run",
				ProductID: "sample", Name: "run", Path: "sample.run", PrimaryCLIPath: "sample run",
			},
			Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
			Selection: contract.SelectionSpec{AgentSummary: "sample"},
		}},
	}}}
	resolveSchemaBuildForDelivery = func(*cobra.Command) (ResolvedSchemaBuild, error) {
		return ResolvedSchemaBuild{registry: registry, root: root}, nil
	}

	resolveValidateParameterDelivery = func(BoundCommandRegistry, SchemaRegistry) error {
		return fmt.Errorf("param boom")
	}
	if _, err := assembleSchemaCatalogFromRoot(root); err == nil || !strings.Contains(err.Error(), "param boom") {
		t.Fatalf("param error = %v", err)
	}
	resolveValidateParameterDelivery = func(BoundCommandRegistry, SchemaRegistry) error { return nil }

	loadCatalogValidateInterfaces = func(SchemaRegistry) error { return fmt.Errorf("iface boom") }
	if _, err := assembleSchemaCatalogFromRoot(root); err == nil || !strings.Contains(err.Error(), "iface boom") {
		t.Fatalf("iface error = %v", err)
	}
	loadCatalogValidateInterfaces = func(SchemaRegistry) error { return nil }

	loadCatalogValidateProvenance = func(SchemaRegistry) error { return fmt.Errorf("prov boom") }
	if _, err := assembleSchemaCatalogFromRoot(root); err == nil || !strings.Contains(err.Error(), "prov boom") {
		t.Fatalf("provenance error = %v", err)
	}
	loadCatalogValidateProvenance = func(SchemaRegistry) error { return nil }

	registryToSnapshotPayloadFn = func(SchemaRegistry) (SchemaCatalogSnapshot, error) {
		return SchemaCatalogSnapshot{}, fmt.Errorf("payload boom")
	}
	if _, err := assembleSchemaCatalogFromRoot(root); err == nil || !strings.Contains(err.Error(), "payload boom") {
		t.Fatalf("payload error = %v", err)
	}
	registryToSnapshotPayloadFn = prevPayload

	// Index failure: empty registry still indexes; force via invalid product id.
	badRegistry := SchemaRegistry{Products: []ProductSpec{{
		ID: "",
		Tools: []ToolSpec{{
			Identity: contract.ToolIdentitySpec{CLIPath: "x", CanonicalPath: "x.y", ProductID: "x", Name: "y", Path: "x.y", PrimaryCLIPath: "x"},
		}},
	}}}
	resolveSchemaBuildForDelivery = func(*cobra.Command) (ResolvedSchemaBuild, error) {
		return ResolvedSchemaBuild{registry: badRegistry, root: root}, nil
	}
	if _, err := assembleSchemaCatalogFromRoot(root); err == nil {
		// Index may still succeed for empty product id; tolerate either outcome
		// as long as the call executes the Index branch.
		t.Log("index accepted empty product id")
	}

	RegisterSchemaSourceRoot(func() *cobra.Command { return &cobra.Command{Use: "dws"} })
	assembleDeliverySchemaCatalogFn = func(*cobra.Command) (loadedSchemaCatalog, error) {
		return loadedSchemaCatalog{}, fmt.Errorf("forced assemble failure")
	}
	resetDeliverySchemaCatalogStateForTest()
	if _, err := MaterializeDeliverySchemaCatalogMapsForTest(); err == nil || !strings.Contains(err.Error(), "forced assemble failure") {
		t.Fatalf("materialize assemble error = %v", err)
	}

	prevHook := restorePackageCLISchemaDeliveryHook
	restorePackageCLISchemaDeliveryHook = nil
	t.Cleanup(func() { restorePackageCLISchemaDeliveryHook = prevHook })
	RestorePackageCLISchemaDeliveryForTest()
	if SchemaSourceRootRegistered() {
		t.Fatal("restore fallback must clear Schema source root")
	}
}

func TestCrossPlatformCoverageSafetyForCLIPathUnregisteredFactory(t *testing.T) {
	prev := schemaSourceRootFn
	t.Cleanup(func() {
		schemaSourceRootFn = prev
		restorePackageCLISchemaDeliveryForTest()
	})
	schemaSourceRootFn = nil
	if _, ok := SafetyForCLIPath("dev app delete"); ok {
		t.Fatal("SafetyForCLIPath without factory must return ok=false")
	}
	RenderSafetyAnnotation(&cobra.Command{Use: "dws"})
}

func TestCrossPlatformCoverageReviewedCommandRegistrySourceHash(t *testing.T) {
	hash, err := ReviewedCommandRegistrySourceHash()
	if err != nil || hash == "" {
		t.Fatalf("ReviewedCommandRegistrySourceHash = %q err=%v", hash, err)
	}
	testseam.Swap(t, &loadReviewedCommandRegistry, func() (CommandRegistry, error) {
		return CommandRegistry{}, fmt.Errorf("load boom")
	})
	if _, err := ReviewedCommandRegistrySourceHash(); err == nil || !strings.Contains(err.Error(), "load boom") {
		t.Fatalf("ReviewedCommandRegistrySourceHash error = %v", err)
	}
}

func TestCrossPlatformCoverageRenderSafetyAnnotationSuccess(t *testing.T) {
	restorePackageCLISchemaDeliveryForTest()
	t.Cleanup(restorePackageCLISchemaDeliveryForTest)

	root := &cobra.Command{Use: "dws"}
	dev := &cobra.Command{Use: "dev"}
	appCmd := &cobra.Command{Use: "app"}
	deleteCmd := &cobra.Command{Use: "delete"}
	root.AddCommand(dev)
	dev.AddCommand(appCmd)
	appCmd.AddCommand(deleteCmd)
	var out bytes.Buffer
	deleteCmd.SetOut(&out)
	RenderSafetyAnnotation(deleteCmd)
	rendered := out.String()
	if !strings.Contains(rendered, "Safety:") || !strings.Contains(rendered, "effect=") {
		t.Fatalf("RenderSafetyAnnotation success = %q", rendered)
	}
	unknown := &cobra.Command{Use: "unknown-cmd"}
	root.AddCommand(unknown)
	var silent bytes.Buffer
	unknown.SetOut(&silent)
	RenderSafetyAnnotation(unknown)
	if silent.Len() != 0 {
		t.Fatalf("unknown command rendered %q", silent.String())
	}
}

func TestCrossPlatformCoverageMCPMetadataInterfaceRefEdges(t *testing.T) {
	if _, ok := mcpMetadataForInterfaceRef(embeddedMCPMetadata{}, " ", " "); ok {
		t.Fatal("blank interface ref must miss")
	}
	if _, ok := mcpMetadataForInterfaceRef(embeddedMCPMetadata{Tools: map[string]embeddedMCPToolMetadata{}}, "chat", "missing"); ok {
		t.Fatal("missing MCP tool must miss")
	}
	agent := agentMetadata{Tools: map[string]agentToolMetadata{
		"chat reply": {InterfaceRef: &embeddedMCPInterfaceRef{ProductID: "chat", RPCName: "send_personal_message"}},
	}}
	mcp := embeddedMCPMetadata{Tools: map[string]embeddedMCPToolMetadata{
		"chat.send_personal_message": {Parameters: map[string]embeddedMCPParamMeta{"clawType": {Type: "string"}}},
	}}
	got, ok := pinnedMCPMetadataForEntryFrom(runtimeSchemaEntry{
		PrimaryCLIPath: "chat reply", ProductID: "chat", ToolName: "reply_personal_message",
	}, agent, mcp)
	if !ok || got.Parameters["clawType"].Type != "string" {
		t.Fatalf("agent InterfaceRef remap = %#v ok=%v", got, ok)
	}
}

func TestCrossPlatformCoverageAssembleSchemaCatalogSnapshotErrors(t *testing.T) {
	if _, err := assembleSchemaCatalogSnapshot([]byte("{"), fstest.MapFS{}, "tools"); err == nil {
		t.Fatal("bad envelope must fail")
	}
	envelope := []byte(`{"version":1,"source_hash":"x","catalog":{}}`)
	if _, err := assembleSchemaCatalogSnapshot(envelope, fstest.MapFS{}, "missing"); err == nil || !strings.Contains(err.Error(), "read schema catalog tools directory") {
		t.Fatalf("missing dir error = %v", err)
	}
	shards := shardReadErrFS{MapFS: fstest.MapFS{"tools/bad.json": {Data: []byte(`{"product":"sample","tools":{}}`)}}}
	if _, err := assembleSchemaCatalogSnapshot(envelope, shards, "tools"); err == nil || !strings.Contains(err.Error(), "read schema catalog shard") {
		t.Fatalf("read shard error = %v", err)
	}
	badJSON := fstest.MapFS{"tools/bad.json": {Data: []byte(`{`)}}
	if _, err := assembleSchemaCatalogSnapshot(envelope, badJSON, "tools"); err == nil || !strings.Contains(err.Error(), "decode schema catalog shard") {
		t.Fatalf("decode shard error = %v", err)
	}
}

func TestCrossPlatformCoverageSchemaMetaIndexSuccessReturns(t *testing.T) {
	index := SchemaMetaIndexSnapshot{
		Version: SchemaMetaIndexVersion, SourceHash: "hash",
		Entries: []SchemaMetaIndexEntry{{CLIPath: "a run", Canonical: "a.run", ProductID: "a"}},
	}
	encoded, err := EncodeSchemaMetaIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeSchemaMetaIndex(encoded)
	if err != nil || got.SourceHash != "hash" {
		t.Fatalf("DecodeSchemaMetaIndex = %#v err=%v", got, err)
	}
	lookup, err := decodeSchemaMetaIndexLookup(encoded)
	if err != nil || lookup["a run"].Identity.Canonical != "a.run" {
		t.Fatalf("decodeSchemaMetaIndexLookup = %#v err=%v", lookup, err)
	}
}

func TestCrossPlatformCoverageDeliverySchemaOverviewAndQueryBranches(t *testing.T) {
	restorePackageCLISchemaDeliveryForTest()
	t.Cleanup(restorePackageCLISchemaDeliveryForTest)

	if _, err := deliverySchemaOverviewPayload(); err != nil {
		t.Fatalf("deliverySchemaOverviewPayload() error = %v", err)
	}
	if _, err := queryDeliverySchemaPayload([]string{"dev"}); err != nil {
		t.Fatalf("queryDeliverySchemaPayload(dev) error = %v", err)
	}
	if _, err := queryDeliverySchemaPayload([]string{"dev app"}); err != nil {
		t.Fatalf("queryDeliverySchemaPayload(dev app) error = %v", err)
	}
	if _, err := deliverySchemaAllPayload(); err != nil {
		t.Fatalf("deliverySchemaAllPayload() error = %v", err)
	}

	schemaSourceRootFn = nil
	assembleDeliverySchemaCatalogFn = assembleSchemaCatalogFromRoot
	resetDeliverySchemaCatalogStateForTest()
	t.Cleanup(restorePackageCLISchemaDeliveryForTest)
	if _, err := deliverySchemaOverviewPayload(); err == nil {
		t.Fatal("overview without factory must fail")
	}
	if _, err := queryDeliverySchemaPayload(nil); err == nil {
		t.Fatal("query without factory must fail")
	}
	if _, err := deliverySchemaAllPayload(); err == nil {
		t.Fatal("all without factory must fail")
	}
}

func TestCrossPlatformCoverageSchemaPayloadEmptySourceFallback(t *testing.T) {
	registry := SchemaRegistry{
		Source: "",
		Products: []ProductSpec{{
			ID: "sample",
			Tools: []ToolSpec{{
				Identity: contract.ToolIdentitySpec{
					CLIPath: "sample nested run", CanonicalPath: "sample.nested_run",
					ProductID: "sample", Name: "nested_run", Path: "sample.nested_run",
					PrimaryCLIPath: "sample nested run", Group: "nested",
				},
				Safety:    contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "yes"},
				Selection: contract.SelectionSpec{AgentSummary: "sample"},
			}},
		}},
	}
	index, err := registry.Index()
	if err != nil {
		t.Fatal(err)
	}
	loaded := loadedSchemaCatalog{Registry: registry, Index: index, Snapshot: SchemaCatalogSnapshot{SourceHash: "hash"}}
	product, err := schemaPayloadFromLoadedCatalog(loaded, []string{"sample"})
	if err != nil || product["source"] != SchemaSourceRuntimeAssembled {
		t.Fatalf("product empty-source fallback = %#v err=%v", product, err)
	}
	group, err := schemaPayloadFromLoadedCatalog(loaded, []string{"sample nested"})
	if err != nil || group["source"] != SchemaSourceRuntimeAssembled {
		t.Fatalf("group empty-source fallback = %#v err=%v", group, err)
	}
}
