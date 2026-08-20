// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package helpers

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type aitableDatasourceCaller struct {
	calls []aitableTestCall
}

func (c *aitableDatasourceCaller) CallTool(_ context.Context, server, tool string, args map[string]any) (*edition.ToolResult, error) {
	c.calls = append(c.calls, aitableTestCall{server: server, tool: tool, args: args})
	return &edition.ToolResult{Content: []edition.ContentBlock{{
		Type: "text",
		Text: `{"status":"success","data":{"tableId":"tbl_test","taskId":"task_test"}}`,
	}}}, nil
}

func (*aitableDatasourceCaller) Format() string { return "json" }
func (*aitableDatasourceCaller) DryRun() bool   { return false }
func (*aitableDatasourceCaller) Fields() string { return "" }
func (*aitableDatasourceCaller) JQ() string     { return "" }

func runAitableDatasourceCommand(t *testing.T, args ...string) (*aitableDatasourceCaller, error) {
	t.Helper()
	testseam.Protect(t, &os.Args)

	caller := &aitableDatasourceCaller{}
	InitDepsForTest(t, caller)
	deps.Out.w = io.Discard
	deps.Out.errW = io.Discard
	os.Args = append([]string{"dws", "aitable", "datasource"}, args...)

	root := newAitableCommand()
	root.SetArgs(append([]string{"datasource"}, args...))
	return caller, root.Execute()
}

func TestAitableDatasourceSyncRejectsMissingTableIDs(t *testing.T) {
	_, err := runAitableDatasourceCommand(t, "sync", "--base-id", "BASE123")
	if err == nil || !strings.Contains(err.Error(), "table-ids") {
		t.Fatalf("error = %v, want table-ids required", err)
	}
}

func TestAitableDatasourceSyncRejectsTooManyTableIDs(t *testing.T) {
	_, err := runAitableDatasourceCommand(t, "sync", "--base-id", "BASE123",
		"--table-ids", "T1,T2,T3,T4,T5,T6")
	if err == nil || !strings.Contains(err.Error(), "1-5") {
		t.Fatalf("error = %v, want 1-5 limit", err)
	}
}

func TestAitableDatasourceSyncRejectsEmptyTableIDs(t *testing.T) {
	_, err := runAitableDatasourceCommand(t, "sync", "--base-id", "BASE123",
		"--table-ids", "")
	if err == nil || !strings.Contains(err.Error(), "table-ids") {
		t.Fatalf("error = %v, want table-ids error", err)
	}
}

func TestAitableDatasourceSyncAcceptsBoundaryFive(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "sync", "--base-id", "BASE123",
		"--table-ids", "T1,T2,T3,T4,T5")
	if err != nil {
		t.Fatalf("5 table-ids should succeed: %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "run_datasource_sync" {
		t.Fatalf("unexpected calls: %#v", caller.calls)
	}
}

func TestAitableDatasourceSyncAcceptsSingleTableID(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "sync", "--base-id", "BASE123",
		"--table-ids", "T1")
	if err != nil {
		t.Fatalf("single table-id should succeed: %v", err)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
}

func TestAitableDatasourceSyncStatusRejectsTooManyTaskIDs(t *testing.T) {
	_, err := runAitableDatasourceCommand(t, "sync-status",
		"--base-id", "BASE123", "--table-id", "TBL456",
		"--task-ids", "TK1,TK2,TK3,TK4,TK5,TK6")
	if err == nil || !strings.Contains(err.Error(), "at most 5") {
		t.Fatalf("error = %v, want at most 5 limit", err)
	}
}

func TestAitableDatasourceSyncStatusAcceptsFiveTaskIDs(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "sync-status",
		"--base-id", "BASE123", "--table-id", "TBL456",
		"--task-ids", "TK1,TK2,TK3,TK4,TK5")
	if err != nil {
		t.Fatalf("5 task-ids should succeed: %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "get_datasource_sync_status" {
		t.Fatalf("unexpected calls: %#v", caller.calls)
	}
}

func TestAitableDatasourceSyncStatusWithoutTaskIDs(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "sync-status",
		"--base-id", "BASE123", "--table-id", "TBL456")
	if err != nil {
		t.Fatalf("no task-ids should succeed: %v", err)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(caller.calls))
	}
	if _, ok := caller.calls[0].args["taskIds"]; ok {
		t.Fatal("taskIds should not be set when --task-ids not provided")
	}
}

func TestAitableDatasourceSyncStatusRejectsMissingTableID(t *testing.T) {
	_, err := runAitableDatasourceCommand(t, "sync-status", "--base-id", "BASE123")
	if err == nil || !strings.Contains(err.Error(), "table-id") {
		t.Fatalf("error = %v, want table-id required", err)
	}
}

func TestAitableDatasourceGetConfigRejectsMissingTableID(t *testing.T) {
	_, err := runAitableDatasourceCommand(t, "get-config", "--base-id", "BASE123")
	if err == nil || !strings.Contains(err.Error(), "table-id") {
		t.Fatalf("error = %v, want table-id required", err)
	}
}

func TestAitableDatasourceGetConfigSuccess(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "get-config",
		"--base-id", "BASE123", "--table-id", "TBL456")
	if err != nil {
		t.Fatalf("get-config should succeed: %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "get_datasource_config" {
		t.Fatalf("unexpected calls: %#v", caller.calls)
	}
	if caller.calls[0].args["tableId"] != "TBL456" {
		t.Fatalf("tableId = %v, want TBL456", caller.calls[0].args["tableId"])
	}
}

func TestAitableDatasourceListSourcesSuccess(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "list-sources",
		"--base-id", "BASE123", "--datasource-type", "OA")
	if err != nil {
		t.Fatalf("list-sources should succeed: %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "list_datasource_sources" {
		t.Fatalf("unexpected calls: %#v", caller.calls)
	}
}

func TestAitableDatasourceListSourcesRejectsMissingType(t *testing.T) {
	_, err := runAitableDatasourceCommand(t, "list-sources", "--base-id", "BASE123")
	if err == nil || !strings.Contains(err.Error(), "datasource-type") {
		t.Fatalf("error = %v, want datasource-type required", err)
	}
}

func TestAitableDatasourceGetFieldsRejectsMissingSourceConfig(t *testing.T) {
	_, err := runAitableDatasourceCommand(t, "get-fields",
		"--base-id", "BASE123", "--datasource-type", "OA")
	if err == nil || !strings.Contains(err.Error(), "source-config") {
		t.Fatalf("error = %v, want source-config required", err)
	}
}

func TestAitableDatasourceGetFieldsSuccess(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "get-fields",
		"--base-id", "BASE123", "--datasource-type", "OA",
		"--source-config", `{"processCode":"PROC-XXXX","name":"采购申请","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}`)
	if err != nil {
		t.Fatalf("get-fields should succeed: %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "get_datasource_fields" {
		t.Fatalf("unexpected calls: %#v", caller.calls)
	}
	if caller.calls[0].args["sourceConfig"] != `{"processCode":"PROC-XXXX","name":"采购申请","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}` {
		t.Fatalf("sourceConfig not passed as raw string: %v", caller.calls[0].args["sourceConfig"])
	}
}

func TestAitableDatasourceCreateRejectsMissingSourceConfig(t *testing.T) {
	_, err := runAitableDatasourceCommand(t, "create",
		"--base-id", "BASE123", "--datasource-type", "OA")
	if err == nil || !strings.Contains(err.Error(), "source-config") {
		t.Fatalf("error = %v, want source-config required", err)
	}
}

func TestAitableDatasourceCreateSuccess(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "create",
		"--base-id", "BASE123", "--datasource-type", "OA",
		"--source-config", `{"processCode":"PROC-XXXX","name":"采购申请","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}`)
	if err != nil {
		t.Fatalf("create should succeed: %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "create_datasource" {
		t.Fatalf("unexpected calls: %#v", caller.calls)
	}
	if v, ok := caller.calls[0].args["auto"]; !ok || v != false {
		t.Fatalf("auto = %v, want false when not provided", v)
	}
}

func TestAitableDatasourceCreateWithAuto(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "create",
		"--base-id", "BASE123", "--datasource-type", "OA",
		"--source-config", `{"processCode":"PROC-XXXX","name":"采购申请","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}`,
		"--auto")
	if err != nil {
		t.Fatalf("create with --auto should succeed: %v", err)
	}
	if v, ok := caller.calls[0].args["auto"]; !ok || v != true {
		t.Fatalf("auto = %v, want true", v)
	}
}

func TestAitableDatasourceUpdateRejectsMissingTableID(t *testing.T) {
	_, err := runAitableDatasourceCommand(t, "update", "--base-id", "BASE123")
	if err == nil || !strings.Contains(err.Error(), "table-id") {
		t.Fatalf("error = %v, want table-id required", err)
	}
}

func TestAitableDatasourceUpdateWithAutoOnly(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "update",
		"--base-id", "BASE123", "--table-id", "TBL456", "--auto")
	if err != nil {
		t.Fatalf("update with --auto only should succeed: %v", err)
	}
	if len(caller.calls) != 1 || caller.calls[0].tool != "update_datasource_config" {
		t.Fatalf("unexpected calls: %#v", caller.calls)
	}
	if v, ok := caller.calls[0].args["auto"]; !ok || v != true {
		t.Fatalf("auto = %v, want true", v)
	}
}

func TestAitableDatasourceUpdateWithSourceConfig(t *testing.T) {
	caller, err := runAitableDatasourceCommand(t, "update",
		"--base-id", "BASE123", "--table-id", "TBL456",
		"--source-config", `{"processCode":"PROC-YYYY","name":"出差申请","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}`)
	if err != nil {
		t.Fatalf("update with source-config should succeed: %v", err)
	}
	if caller.calls[0].args["sourceConfig"] != `{"processCode":"PROC-YYYY","name":"出差申请","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}` {
		t.Fatalf("sourceConfig not passed as raw string: %v", caller.calls[0].args["sourceConfig"])
	}
	if v, ok := caller.calls[0].args["auto"]; !ok || v != false {
		t.Fatalf("auto = %v, want false when not provided", v)
	}
}
