// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package devapp

import (
	"bytes"
	"context"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type paginationCaller struct{ text string }

func (c *paginationCaller) CallTool(context.Context, string, string, map[string]any) (*edition.ToolResult, error) {
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: c.text}}}, nil
}
func (*paginationCaller) Format() string { return "json" }
func (*paginationCaller) DryRun() bool   { return false }
func (*paginationCaller) Fields() string { return "" }
func (*paginationCaller) JQ() string     { return "" }

func TestDevAppListProjectionPreservesPaginationEvidence(t *testing.T) {
	items := []map[string]any{{"id": "one"}}
	for _, tc := range []struct {
		name string
		data map[string]any
	}{
		{name: "top level", data: map[string]any{"hasMore": true, "nextCursor": "next"}},
		{name: "nested result", data: map[string]any{"result": map[string]any{"hasMore": true, "nextCursor": "next"}}},
		{name: "exhausted", data: map[string]any{"hasMore": false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := devAppListProjection(tc.data, "items", items)
			if got["hasMore"] != devAppPaginationCandidates(tc.data)[len(devAppPaginationCandidates(tc.data))-1]["hasMore"] && tc.name == "nested result" {
				t.Fatalf("hasMore not preserved: %#v", got)
			}
			if tc.name != "exhausted" && got["nextCursor"] != "next" {
				t.Fatalf("nextCursor not preserved: %#v", got)
			}
			if got["count"] != 1 {
				t.Fatalf("projection count=%#v", got["count"])
			}
		})
	}
	if got := devAppPaginationCandidates(nil); got != nil {
		t.Fatalf("nil candidates=%#v", got)
	}
	deep := map[string]any{"content": map[string]any{"result": map[string]any{"hasMore": true, "nextCursor": "deep"}}}
	if got := devAppListProjection(deep, "items", nil); got["nextCursor"] != "deep" {
		t.Fatalf("deep projection=%#v", got)
	}
}

func TestFrameworkPaginatedShortcutExecutionKeepsCursorEvidence(t *testing.T) {
	caller := &paginationCaller{text: `{"content":{"result":{"hasMore":true,"nextCursor":"next","list":[]}}}`}
	helpers.InitDepsForTest(t, caller)
	helpers.GetFormatter().SetWriters(&bytes.Buffer{}, &bytes.Buffer{})
	for _, tc := range []struct {
		declaration shortcut.Shortcut
		args        []string
	}{
		{ListApp, nil},
		{PermissionList, []string{"--unified-app-id", "app"}},
		{EventList, []string{"--unified-app-id", "app"}},
		{VersionList, []string{"--unified-app-id", "app"}},
	} {
		cmd := corecmd.New(shortcut.FromShortcut(tc.declaration))
		cmd.SetArgs(tc.args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%s: %v", tc.declaration.Command, err)
		}
	}
}
