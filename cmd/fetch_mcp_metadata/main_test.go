// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package main

import (
	"reflect"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/transport"
)

func TestLoadRegistryInterfaceRefsUsesSplitRegistry(t *testing.T) {
	refs := loadRegistryInterfaceRefs()
	if len(refs) == 0 {
		t.Fatal("loadRegistryInterfaceRefs() returned no reviewed commands")
	}

	got, ok := refs["calendar.list_calendars"]
	if !ok {
		t.Fatal("calendar.list_calendars missing from reassembled split registry")
	}
	if got["product_id"] != "calendar" || got["rpc_name"] != "list_calendars" {
		t.Fatalf("calendar.list_calendars ref = %#v", got)
	}
}

func TestMergeLiveMCPToolRefreshesExistingMetadata(t *testing.T) {
	const canonical = "calendar.list_calendars"
	reviewedRef := map[string]any{
		"product_id": "calendar-helper",
		"rpc_name":   "list_user_calendars",
	}
	allTools := map[string]map[string]any{
		canonical: {
			"title":         "old title",
			"description":   "old description",
			"interface_ref": reviewedRef,
			"parameters": map[string]any{
				"stale": map[string]any{"type": "string"},
			},
		},
	}
	live := transport.ToolDescriptor{
		Name:        "list_calendars",
		Title:       "new title",
		Description: "new description",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cursor": map[string]any{
					"type":        "string",
					"description": "next page cursor",
				},
			},
			"required": []any{"cursor"},
		},
	}
	fallbackRef := map[string]string{
		"product_id": "calendar",
		"rpc_name":   "list_calendars",
	}

	mergeLiveMCPTool(allTools, canonical, live, fallbackRef)

	got := allTools[canonical]
	if got["title"] != "new title" || got["description"] != "new description" {
		t.Fatalf("live metadata was not refreshed: %#v", got)
	}
	if !reflect.DeepEqual(got["interface_ref"], reviewedRef) {
		t.Fatalf("interface_ref = %#v, want reviewed mapping %#v", got["interface_ref"], reviewedRef)
	}
	params, ok := got["parameters"].(map[string]map[string]any)
	if !ok {
		t.Fatalf("parameters type = %T, want refreshed parameter map", got["parameters"])
	}
	if _, stale := params["stale"]; stale {
		t.Fatalf("stale parameter survived refresh: %#v", params)
	}
	if cursor := params["cursor"]; cursor["type"] != "string" || cursor["description"] != "next page cursor" || cursor["required"] != true {
		t.Fatalf("cursor parameter = %#v", cursor)
	}
}

func TestBuildCoverageReportsFailedServices(t *testing.T) {
	got := buildCoverage(26, []string{"doc", "sheet"}, 800, 813)
	if got["source_services"] != 26 {
		t.Fatalf("source_services = %v, want 26", got["source_services"])
	}
	if got["snapshot_services"] != 24 {
		t.Fatalf("snapshot_services = %v, want 24 (26 sources - 2 failures)", got["snapshot_services"])
	}
	if !reflect.DeepEqual(got["missing_services"], []string{"doc", "sheet"}) {
		t.Fatalf("missing_services = %#v, want failed service IDs", got["missing_services"])
	}
	if got["source_tools"] != 800 || got["surface_tools"] != 813 || got["matched_tools"] != 813 {
		t.Fatalf("tool counts = %#v", got)
	}
}

func TestBuildCoverageFullSnapshotHasNoMissingServices(t *testing.T) {
	got := buildCoverage(26, nil, 813, 813)
	if got["snapshot_services"] != 26 {
		t.Fatalf("snapshot_services = %v, want 26", got["snapshot_services"])
	}
	if !reflect.DeepEqual(got["missing_services"], []string{}) {
		t.Fatalf("missing_services = %#v, want empty non-nil slice", got["missing_services"])
	}
}
