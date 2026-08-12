// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type recruitCapturedCall struct {
	productID string
	tool      string
	args      map[string]any
}

type recruitCaptureCaller struct {
	productID string
	tool      string
	args      map[string]any
	calls     []recruitCapturedCall
}

func (c *recruitCaptureCaller) CallTool(_ context.Context, productID, tool string, args map[string]any) (*edition.ToolResult, error) {
	c.productID = productID
	c.tool = tool
	c.args = args
	c.calls = append(c.calls, recruitCapturedCall{productID: productID, tool: tool, args: args})
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: `{}`}}}, nil
}

func (c *recruitCaptureCaller) Format() string { return "json" }
func (c *recruitCaptureCaller) DryRun() bool   { return false }
func (c *recruitCaptureCaller) Fields() string { return "" }
func (c *recruitCaptureCaller) JQ() string     { return "" }

func withRecruitCaller(t *testing.T) *recruitCaptureCaller {
	t.Helper()
	caller := &recruitCaptureCaller{}
	InitDepsForTest(t, caller)
	deps.Out.w = io.Discard
	return caller
}

func TestRecruitJobListWrapsQueryParameters(t *testing.T) {
	caller := withRecruitCaller(t)
	cmd := newRecruitJobListCommand()
	cmd.SetArgs([]string{
		"--keyword", "Java", "--status", "open,draft", "--creator-user-ids", "u1,u2",
		"--campus=false", "--cursor", "10", "--size", "30",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if caller.productID != recruitServerID || caller.tool != recruitListJobsTool {
		t.Fatalf("dispatch = %s/%s, want %s/%s", caller.productID, caller.tool, recruitServerID, recruitListJobsTool)
	}
	if caller.args["cursor"] != 10 || caller.args["size"] != 30 {
		t.Fatalf("pagination args = %#v", caller.args)
	}
	param, ok := caller.args["param"].(map[string]any)
	if !ok {
		t.Fatalf("param = %#v, want object", caller.args["param"])
	}
	if param["keyword"] != "Java" || param["campus"] != false {
		t.Fatalf("param = %#v", param)
	}
	statuses, ok := param["statusList"].([]int)
	if !ok || len(statuses) != 2 || statuses[0] != 1 || statuses[1] != 0 {
		t.Fatalf("statusList = %#v", param["statusList"])
	}
	creators, ok := param["creatorUserIds"].([]string)
	if !ok || len(creators) != 2 || creators[0] != "u1" || creators[1] != "u2" {
		t.Fatalf("creatorUserIds = %#v", param["creatorUserIds"])
	}
}

func TestRecruitJobListDefaultsAndValidation(t *testing.T) {
	caller := withRecruitCaller(t)
	cmd := newRecruitJobListCommand()
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if caller.args["size"] != 20 {
		t.Fatalf("size = %#v, want 20", caller.args["size"])
	}
	if _, ok := caller.args["cursor"]; ok {
		t.Fatalf("first page unexpectedly sent cursor: %#v", caller.args)
	}

	badStatus := newRecruitJobListCommand()
	badStatus.SetArgs([]string{"--status", "unknown"})
	if err := badStatus.Execute(); err == nil || !strings.Contains(err.Error(), "draft/open/invalid/closed") {
		t.Fatalf("invalid status error = %v", err)
	}

	badSize := newRecruitJobListCommand()
	badSize.SetArgs([]string{"--size", "101"})
	if err := badSize.Execute(); err == nil || !strings.Contains(err.Error(), "1 到 100") {
		t.Fatalf("invalid size error = %v", err)
	}
}

func TestRecruitJobGetDispatchesJobID(t *testing.T) {
	caller := withRecruitCaller(t)
	cmd := newRecruitJobGetCommand()
	cmd.SetArgs([]string{"--job-id", "job-1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if caller.productID != recruitServerID || caller.tool != recruitGetJobTool || caller.args["jobId"] != "job-1" {
		t.Fatalf("captured call = %s/%s %#v", caller.productID, caller.tool, caller.args)
	}
}

func writeRecruitJobFixture(t *testing.T) (string, map[string]any) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "job.json")
	job := map[string]any{
		"name": "Java 工程师", "description": "服务端开发", "jobNature": "FULL_TIME",
		"requiredEdu": 1, "minSalary": 20000, "maxSalary": 35000,
	}
	data, err := json.Marshal(job)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, job
}

func TestRecruitJobCreateRequiresConfirmationBeforeRemoteCall(t *testing.T) {
	caller := withRecruitCaller(t)
	path, _ := writeRecruitJobFixture(t)
	cmd := newRecruitJobCreateCommand()
	cmd.Flags().Bool("yes", false, "确认创建")
	cmd.SetIn(strings.NewReader(""))
	cmd.SetArgs([]string{"--from", path})
	err := cmd.Execute()
	var appErr *apperrors.Error
	if !stderrors.As(err, &appErr) || appErr.Reason != "confirmation_required" {
		t.Fatalf("create without confirmation error = %T %v, want confirmation_required", err, err)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("MCP calls before confirmation = %d: %#v, want none", len(caller.calls), caller.calls)
	}
}

func TestRecruitJobCreateCallsRemoteOnceWhenConfirmed(t *testing.T) {
	caller := withRecruitCaller(t)
	path, _ := writeRecruitJobFixture(t)
	cmd := newRecruitJobCreateCommand()
	cmd.Flags().Bool("yes", false, "确认创建")
	cmd.SetArgs([]string{"--from", path, "--yes"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("MCP calls after confirmation = %d: %#v, want exactly one", len(caller.calls), caller.calls)
	}
	call := caller.calls[0]
	if call.productID != recruitServerID || call.tool != recruitCreateJobTool {
		t.Fatalf("dispatch = %s/%s, want %s/%s", call.productID, call.tool, recruitServerID, recruitCreateJobTool)
	}
	wantArgs := map[string]any{"atsAddJobParam": map[string]any{
		"name": "Java 工程师", "description": "服务端开发", "jobNature": "FULL_TIME",
		"requiredEdu": float64(1), "minSalary": float64(20000), "maxSalary": float64(35000),
	}}
	if !reflect.DeepEqual(call.args, wantArgs) {
		t.Fatalf("args = %#v, want %#v", call.args, wantArgs)
	}
}

func TestRecruitCreateValidatesJobFile(t *testing.T) {
	withRecruitCaller(t)
	badPath := filepath.Join(t.TempDir(), "bad-job.json")
	if err := os.WriteFile(badPath, []byte(`{"name":"缺字段"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	bad := newRecruitJobCreateCommand()
	bad.Flags().Bool("yes", false, "确认创建")
	bad.SetArgs([]string{"--from", badPath, "--yes"})
	if err := bad.Execute(); err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("missing field error = %v", err)
	}
}

func TestRecruitCommandTree(t *testing.T) {
	root := newRecruitCommand()
	for _, path := range [][]string{{"job", "list"}, {"job", "get"}, {"job", "create"}} {
		cmd, _, err := root.Find(path)
		if err != nil || cmd == root || cmd.Name() != path[len(path)-1] {
			t.Fatalf("Find(%v) = %v, %v", path, cmd, err)
		}
	}
}
