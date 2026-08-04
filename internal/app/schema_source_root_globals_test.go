// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package app

import (
	"context"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cli"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/mcptypes"
)

// schemaSourceRootMarkerCaller is a recognizable ToolCaller used to detect
// InitDeps clobbering when NewSchemaSourceRootCommand builds the tree.
type schemaSourceRootMarkerCaller struct{}

func (schemaSourceRootMarkerCaller) CallTool(context.Context, string, string, map[string]any) (*edition.ToolResult, error) {
	return &edition.ToolResult{}, nil
}
func (schemaSourceRootMarkerCaller) Format() string { return "json" }
func (schemaSourceRootMarkerCaller) DryRun() bool   { return false }
func (schemaSourceRootMarkerCaller) Fields() string { return "" }
func (schemaSourceRootMarkerCaller) JQ() string     { return "" }

// TestSchemaSourceRootPreservesRuntimeDepsAndPluginEndpoints ensures Schema
// assembly (NewSchemaSourceRootCommand / ResolveMeta delivery) does not call
// helpers.InitDeps or SetDynamicServers, which would wipe a live process's
// caller and plugin endpoints.
func TestSchemaSourceRootPreservesRuntimeDepsAndPluginEndpoints(t *testing.T) {
	const (
		pluginID       = "schema-source-root-plugin-marker"
		pluginEndpoint = "https://schema-source-root-plugin.example/mcp"
	)

	previousCaller := helpers.GetCaller()
	dynamicMu.RLock()
	previousEndpoints := dynamicEndpoints
	previousProducts := dynamicProducts
	previousAliases := dynamicAliases
	previousToolEndpoints := dynamicToolEndpoints
	dynamicMu.RUnlock()
	t.Cleanup(func() {
		helpers.InitDeps(previousCaller)
		dynamicMu.Lock()
		dynamicEndpoints = previousEndpoints
		dynamicProducts = previousProducts
		dynamicAliases = previousAliases
		dynamicToolEndpoints = previousToolEndpoints
		dynamicMu.Unlock()
	})

	marker := schemaSourceRootMarkerCaller{}
	helpers.InitDeps(marker)
	SetDynamicServers(nil)
	AppendDynamicServer(mcptypes.ServerDescriptor{
		Key:      pluginID,
		Endpoint: pluginEndpoint,
		CLI: mcptypes.CLIOverlay{
			ID:      pluginID,
			Command: pluginID,
		},
	})

	root := NewSchemaSourceRootCommand()
	if root == nil {
		t.Fatal("NewSchemaSourceRootCommand returned nil")
	}

	if got := helpers.GetCaller(); got != marker {
		t.Fatalf("helpers.GetCaller() after NewSchemaSourceRootCommand = %T (%p), want marker caller preserved", got, got)
	}
	gotEndpoint, ok := directRuntimeEndpoint(pluginID, "")
	if !ok || gotEndpoint != pluginEndpoint {
		t.Fatalf("plugin endpoint after NewSchemaSourceRootCommand = %q ok=%v, want %q preserved", gotEndpoint, ok, pluginEndpoint)
	}

	resolved, err := cli.ResolveSchemaBuild(root)
	if err != nil {
		t.Fatalf("ResolveSchemaBuild after declaration-only root: %v", err)
	}
	if resolved.CommandCount() == 0 {
		t.Fatal("ResolveSchemaBuild command count is 0")
	}

	// Delivery path also builds NewSchemaSourceRootCommand via the factory.
	registerSchemaRuntimeDelivery()
	meta, ok := cli.ResolveMeta("dev app delete")
	if !ok || meta.Identity.Canonical == "" {
		t.Fatalf("ResolveMeta after registerSchemaRuntimeDelivery = %#v ok=%v", meta, ok)
	}
	if got := helpers.GetCaller(); got != marker {
		t.Fatalf("helpers.GetCaller() after ResolveMeta delivery = %T, want marker caller preserved", got)
	}
	gotEndpoint, ok = directRuntimeEndpoint(pluginID, "")
	if !ok || gotEndpoint != pluginEndpoint {
		t.Fatalf("plugin endpoint after ResolveMeta delivery = %q ok=%v, want %q preserved", gotEndpoint, ok, pluginEndpoint)
	}
}
