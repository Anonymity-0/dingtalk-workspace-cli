// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package cli

import (
	"sort"
	"testing"
)

func TestRuntimeToolTextPrefersCobraHelpOverGenericMCPMetadata(t *testing.T) {
	entry := runtimeSchemaEntry{
		ProductID:      "aitable",
		ToolName:       "view_get_filter",
		Title:          "获取视图 filter 配置",
		Description:    "获取指定视图当前的筛选规则数组。",
		PrimaryCLIPath: "aitable view get filter",
	}
	metadata := runtimeSchemaMetadataSources{MCP: embeddedMCPMetadata{Tools: map[string]embeddedMCPToolMetadata{
		"aitable.view_get_filter": {
			Title:       "get_views",
			Description: "获取数据表的全部视图。",
		},
	}}}

	title, description, metadataSource, provenance, err := runtimeToolTextMetadataFromMetadata(entry, metadata)
	if err != nil {
		t.Fatal(err)
	}
	if title != entry.Title || description != entry.Description {
		t.Fatalf("tool text = %q / %q, want Cobra Help %q / %q", title, description, entry.Title, entry.Description)
	}
	if metadataSource != "" {
		t.Fatalf("metadata source = %q, want Cobra Help", metadataSource)
	}
	for _, field := range []string{"title", "description"} {
		if provenance[field].Source != "cobra_help" {
			t.Fatalf("%s provenance = %#v, want cobra_help", field, provenance[field])
		}
	}
}

func TestProductionRegisterSchemaHintsHasNoSubstantiveOverlays(t *testing.T) {
	// Framework expectation after ParamDecl / ContractFinal migration: production
	// RegisterSchemaHints must not publish tool_schema_hint overlays. Temporary
	// registry injection is allowed only inside unit-test fixtures.
	if got := len(defaultSchemaHintRegistry.tools); got != 0 {
		paths := make([]string, 0, got)
		for path := range defaultSchemaHintRegistry.tools {
			paths = append(paths, path)
		}
		sort.Strings(paths)
		t.Fatalf("production schema hint registry has %d tool overlay(s), want 0; paths=%v", got, paths)
	}
}

func TestRuntimeToolTextKeepsReviewedHintAboveCobraHelp(t *testing.T) {
	// Production RegisterSchemaHints maps are empty; inject a temporary
	// reviewed tool_schema_hint so this precedence unit test stays self-contained.
	originalHints := defaultSchemaHintRegistry
	t.Cleanup(func() { defaultSchemaHintRegistry = originalHints })
	defaultSchemaHintRegistry = newSchemaHintRegistry()
	const wantDescription = "查询 AI 表格记录。默认返回单页；传 --all 时自动翻页累计全部记录。"
	RegisterSchemaHints("aitable", map[string]ToolSchemaHint{
		"query_records": {Description: wantDescription},
	})

	entry := runtimeSchemaEntry{
		ProductID:      "aitable",
		ToolName:       "query_records",
		Title:          "查询记录",
		Description:    "CLI 查询记录说明。",
		PrimaryCLIPath: "aitable record query",
	}
	metadata := runtimeSchemaMetadataSources{MCP: embeddedMCPMetadata{Tools: map[string]embeddedMCPToolMetadata{
		"aitable.query_records": {
			Title:       "query_records",
			Description: "通用 RPC 查询说明。",
		},
	}}}

	title, description, metadataSource, provenance, err := runtimeToolTextMetadataFromMetadata(entry, metadata)
	if err != nil {
		t.Fatal(err)
	}
	if title != entry.Title {
		t.Fatalf("title = %q, want Cobra Help title %q", title, entry.Title)
	}
	if description != wantDescription {
		t.Fatalf("description = %q, want reviewed hint %q", description, wantDescription)
	}
	if metadataSource != "tool-schema-hint" {
		t.Fatalf("metadata source = %q, want tool-schema-hint", metadataSource)
	}
	if provenance["title"].Source != "cobra_help" || provenance["description"].Source != "tool_schema_hint" {
		t.Fatalf("provenance = %#v", provenance)
	}
}
