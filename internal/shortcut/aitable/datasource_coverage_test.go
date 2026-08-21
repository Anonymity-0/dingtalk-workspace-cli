// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package aitable

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type datasourceCoverageCaller struct {
	err  error
	resp string
}

func (c *datasourceCoverageCaller) CallTool(_ context.Context, _, _ string, _ map[string]any) (*edition.ToolResult, error) {
	if c.err != nil {
		return nil, c.err
	}
	text := c.resp
	if text == "" {
		text = `{"status":"success","data":{}}`
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: text}}}, nil
}

func (c *datasourceCoverageCaller) Format() string { return "json" }
func (c *datasourceCoverageCaller) DryRun() bool   { return false }
func (c *datasourceCoverageCaller) Fields() string { return "" }
func (c *datasourceCoverageCaller) JQ() string     { return "" }

func runDatasourceShortcutCLI(t *testing.T, caller *datasourceCoverageCaller, args ...string) error {
	t.Helper()
	helpers.InitDepsForTest(t, caller)
	root := newPlatformCoverageRoot()
	root.SetArgs(append([]string{"aitable"}, args...))
	return root.Execute()
}

// ── DatasourceCreate error paths ─────────────────────────────────────────────

func TestDatasourceCreateRejectsInvalidSourceConfig(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{},
		"+datasource-create", "--base-id", "BASE1", "--datasource-type", "OA",
		"--source-config", "not-json")
	if err == nil || !strings.Contains(err.Error(), "source-config") {
		t.Fatalf("error = %v, want source-config parse error", err)
	}
}

func TestDatasourceCreateRejectsInvalidAutoSyncSetting(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{},
		"+datasource-create", "--base-id", "BASE1", "--datasource-type", "OA",
		"--source-config", `{"processCode":"P","name":"N","dataType":"recent_time","iconUrl":"u","url":"v"}`,
		"--auto-sync-setting", "not-json")
	if err == nil || !strings.Contains(err.Error(), "auto-sync-setting") {
		t.Fatalf("error = %v, want auto-sync-setting parse error", err)
	}
}

func TestDatasourceCreatePropagatesCallMCPError(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{err: errors.New("mcp-error")},
		"+datasource-create", "--base-id", "BASE1", "--datasource-type", "OA",
		"--source-config", `{"processCode":"P","name":"N","dataType":"recent_time","iconUrl":"u","url":"v"}`)
	if err == nil || !strings.Contains(err.Error(), "mcp-error") {
		t.Fatalf("error = %v, want mcp-error", err)
	}
}

// ── DatasourceUpdate error paths ─────────────────────────────────────────────

func TestDatasourceUpdateRejectsInvalidSourceConfig(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{},
		"+datasource-update", "--base-id", "BASE1", "--table-id", "TBL1",
		"--source-config", "not-json")
	if err == nil || !strings.Contains(err.Error(), "source-config") {
		t.Fatalf("error = %v, want source-config parse error", err)
	}
}

func TestDatasourceUpdateRejectsInvalidAutoSyncSetting(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{},
		"+datasource-update", "--base-id", "BASE1", "--table-id", "TBL1",
		"--auto-sync-setting", "not-json")
	if err == nil || !strings.Contains(err.Error(), "auto-sync-setting") {
		t.Fatalf("error = %v, want auto-sync-setting parse error", err)
	}
}

func TestDatasourceUpdateRejectsNoChanges(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{},
		"+datasource-update", "--base-id", "BASE1", "--table-id", "TBL1")
	if err == nil || !strings.Contains(err.Error(), "至少需要一个配置变更") {
		t.Fatalf("error = %v, want no-changes error", err)
	}
}

func TestDatasourceUpdatePropagatesCallMCPError(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{err: errors.New("mcp-error")},
		"+datasource-update", "--base-id", "BASE1", "--table-id", "TBL1", "--auto")
	if err == nil || !strings.Contains(err.Error(), "mcp-error") {
		t.Fatalf("error = %v, want mcp-error", err)
	}
}

// ── DatasourceSync error paths ────────────────────────────────────────────────

func TestDatasourceSyncShortcutRejectsTooManyTableIDs(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{},
		"+datasource-sync", "--base-id", "BASE1", "--table-ids", "T1,T2,T3,T4,T5,T6")
	if err == nil || !strings.Contains(err.Error(), "1-5") {
		t.Fatalf("error = %v, want 1-5 limit", err)
	}
}

func TestDatasourceSyncShortcutPropagatesCallMCPError(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{err: errors.New("mcp-error")},
		"+datasource-sync", "--base-id", "BASE1", "--table-ids", "TBL1")
	if err == nil || !strings.Contains(err.Error(), "mcp-error") {
		t.Fatalf("error = %v, want mcp-error", err)
	}
}

// ── DatasourceSyncStatus error paths ─────────────────────────────────────────

func TestDatasourceSyncStatusShortcutRejectsTooManyTaskIDs(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{},
		"+datasource-sync-status", "--base-id", "BASE1", "--table-id", "TBL1",
		"--task-ids", "T1,T2,T3,T4,T5,T6")
	if err == nil || !strings.Contains(err.Error(), "1-5") {
		t.Fatalf("error = %v, want 1-5 limit", err)
	}
}

func TestDatasourceSyncStatusShortcutPropagatesCallMCPError(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{err: errors.New("mcp-error")},
		"+datasource-sync-status", "--base-id", "BASE1", "--table-id", "TBL1",
		"--task-ids", "TASK1")
	if err == nil || !strings.Contains(err.Error(), "mcp-error") {
		t.Fatalf("error = %v, want mcp-error", err)
	}
}

// ── DatasourceGetConfig error path ───────────────────────────────────────────

func TestDatasourceGetConfigShortcutPropagatesCallMCPError(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{err: errors.New("mcp-error")},
		"+datasource-get-config", "--base-id", "BASE1", "--table-id", "TBL1")
	if err == nil || !strings.Contains(err.Error(), "mcp-error") {
		t.Fatalf("error = %v, want mcp-error", err)
	}
}

// ── DatasourceListSources error path ─────────────────────────────────────────

func TestDatasourceListSourcesShortcutPropagatesCallMCPError(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{err: errors.New("mcp-error")},
		"+datasource-list-sources", "--base-id", "BASE1", "--datasource-type", "OA")
	if err == nil || !strings.Contains(err.Error(), "mcp-error") {
		t.Fatalf("error = %v, want mcp-error", err)
	}
}

// ── DatasourceGetFields error paths ──────────────────────────────────────────

func TestDatasourceGetFieldsShortcutRejectsInvalidSourceConfig(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{},
		"+datasource-get-fields", "--base-id", "BASE1", "--datasource-type", "OA",
		"--source-config", "not-json")
	if err == nil || !strings.Contains(err.Error(), "source-config") {
		t.Fatalf("error = %v, want source-config parse error", err)
	}
}

func TestDatasourceGetFieldsShortcutPropagatesCallMCPError(t *testing.T) {
	err := runDatasourceShortcutCLI(t, &datasourceCoverageCaller{err: errors.New("mcp-error")},
		"+datasource-get-fields", "--base-id", "BASE1", "--datasource-type", "OA",
		"--source-config", `{"processCode":"P","name":"N","dataType":"recent_time","iconUrl":"u","url":"v"}`)
	if err == nil || !strings.Contains(err.Error(), "mcp-error") {
		t.Fatalf("error = %v, want mcp-error", err)
	}
}
