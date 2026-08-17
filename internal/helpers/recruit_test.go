// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
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
	err       error
	text      string
	dryRun    bool
}

func (c *recruitCaptureCaller) CallTool(_ context.Context, productID, tool string, args map[string]any) (*edition.ToolResult, error) {
	c.productID = productID
	c.tool = tool
	c.args = args
	c.calls = append(c.calls, recruitCapturedCall{productID: productID, tool: tool, args: args})
	if c.err != nil {
		return nil, c.err
	}
	text := c.text
	if text == "" {
		text = `{}`
		switch tool {
		case recruitListJobsTool:
			text = `{"jobs":[],"hasMore":false}`
		case recruitCreateJobTool:
			text = `{"jobId":"created-job"}`
		}
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: text}}}, nil
}

func (c *recruitCaptureCaller) Format() string { return "json" }
func (c *recruitCaptureCaller) DryRun() bool   { return c.dryRun }
func (c *recruitCaptureCaller) Fields() string { return "" }
func (c *recruitCaptureCaller) JQ() string     { return "" }

func withRecruitCaller(t *testing.T) *recruitCaptureCaller {
	t.Helper()
	caller := &recruitCaptureCaller{}
	InitDepsForTest(t, caller)
	deps.Out.w = io.Discard
	return caller
}

func prepareRecruitTestCommand(cmd *cobra.Command) *cobra.Command {
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	return cmd
}

func TestRecruitJobListWrapsQueryParameters(t *testing.T) {
	caller := withRecruitCaller(t)
	cmd := prepareRecruitTestCommand(newRecruitJobListCommand())
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
	if caller.args["cursor"] != int64(10) || caller.args["size"] != 30 {
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

func TestRecruitJobListCursorRoundTrip(t *testing.T) {
	caller := withRecruitCaller(t)
	cmd := prepareRecruitTestCommand(newRecruitJobListCommand())
	cmd.SetArgs([]string{"--cursor", "9223372036854775807"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := caller.args["cursor"]; got != int64(9223372036854775807) {
		t.Fatalf("cursor = %#v (%T), want max int64", got, got)
	}

	for _, cursor := range []string{"not-a-cursor", "9223372036854775808"} {
		invalid := prepareRecruitTestCommand(newRecruitJobListCommand())
		invalid.SetArgs([]string{"--cursor", cursor})
		if err := invalid.Execute(); err == nil || !strings.Contains(err.Error(), "大于或等于 0 的整数") {
			t.Fatalf("cursor %q error = %v", cursor, err)
		}
	}
}

func TestRecruitJobListResponseCursorRoundTrip(t *testing.T) {
	caller := withRecruitCaller(t)
	caller.text = `{"success":true,"result":{"list":[],"hasMore":true,"nextCursor":9223372036854775807}}`

	cmd := prepareRecruitTestCommand(newRecruitJobListCommand())
	result, err := recruitResultCall(cmd, recruitListJobsTool, map[string]any{"size": 20})
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := output.EnvelopeFromResult(result)
	if err != nil {
		t.Fatal(err)
	}
	if envelope.Outcome != output.OutcomeSuccess || envelope.Meta == nil || envelope.Meta.Pagination == nil {
		t.Fatalf("envelope = %#v", envelope)
	}
	meta := envelope.Meta
	if got := meta.Pagination.NextToken; got != "9223372036854775807" {
		t.Fatalf("next token = %q, want lossless max int64", got)
	}

	caller.text = `{"jobs":[],"hasMore":false}`
	cmd = prepareRecruitTestCommand(newRecruitJobListCommand())
	cmd.SetArgs([]string{"--cursor", meta.Pagination.NextToken})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := caller.args["cursor"]; got != int64(9223372036854775807) {
		t.Fatalf("replayed cursor = %#v (%T), want max int64", got, got)
	}
}

func TestRecruitJobListDefaultsAndValidation(t *testing.T) {
	caller := withRecruitCaller(t)
	cmd := prepareRecruitTestCommand(newRecruitJobListCommand())
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if caller.args["size"] != 20 {
		t.Fatalf("size = %#v, want 20", caller.args["size"])
	}
	if _, ok := caller.args["cursor"]; ok {
		t.Fatalf("first page unexpectedly sent cursor: %#v", caller.args)
	}

	badStatus := prepareRecruitTestCommand(newRecruitJobListCommand())
	badStatus.SetArgs([]string{"--status", "unknown"})
	if err := badStatus.Execute(); err == nil || !strings.Contains(err.Error(), "draft/open/invalid/closed") {
		t.Fatalf("invalid status error = %v", err)
	}

	badSize := prepareRecruitTestCommand(newRecruitJobListCommand())
	badSize.SetArgs([]string{"--size", "101"})
	if err := badSize.Execute(); err == nil || !strings.Contains(err.Error(), "1 到 100") {
		t.Fatalf("invalid size error = %v", err)
	}
}

func TestRecruitJobGetDispatchesJobID(t *testing.T) {
	caller := withRecruitCaller(t)
	cmd := prepareRecruitTestCommand(newRecruitJobGetCommand())
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
	cmd := prepareRecruitTestCommand(newRecruitJobCreateCommand())
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
	cmd := prepareRecruitTestCommand(newRecruitJobCreateCommand())
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
	bad := prepareRecruitTestCommand(newRecruitJobCreateCommand())
	bad.Flags().Bool("yes", false, "确认创建")
	bad.SetArgs([]string{"--from", badPath, "--yes"})
	if err := bad.Execute(); err == nil || !strings.Contains(err.Error(), "description") {
		t.Fatalf("missing field error = %v", err)
	}
}

func TestRecruitValidationBranches(t *testing.T) {
	statuses, err := transformRecruitStatuses(" open, ,open,closed ")
	if err != nil || !reflect.DeepEqual(statuses, []int{1, 3}) {
		t.Fatalf("statuses = %#v, err = %v", statuses, err)
	}

	negativeCursor := prepareRecruitTestCommand(newRecruitJobListCommand())
	negativeCursor.SetArgs([]string{"--cursor", "-1"})
	if err := negativeCursor.Execute(); err == nil || !strings.Contains(err.Error(), "大于或等于 0") {
		t.Fatalf("negative cursor error = %v", err)
	}

	nullPath := filepath.Join(t.TempDir(), "null.json")
	if err := os.WriteFile(nullPath, []byte("null"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRecruitJobFile(nullPath); err == nil || !strings.Contains(err.Error(), "顶层必须是对象") {
		t.Fatalf("null job error = %v", err)
	}
	if _, err := loadRecruitJobFile(filepath.Join(t.TempDir(), "missing.json")); err == nil || !strings.Contains(err.Error(), "读取职位 JSON 失败") {
		t.Fatalf("missing job file error = %v", err)
	}
	invalidPath := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalidPath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRecruitJobFile(invalidPath); err == nil || !strings.Contains(err.Error(), "不是有效的 JSON 对象") {
		t.Fatalf("invalid job JSON error = %v", err)
	}

	base := map[string]any{
		"name": "Java 工程师", "description": "服务端开发", "jobNature": "FULL_TIME",
		"requiredEdu": float64(1), "minSalary": float64(20000), "maxSalary": float64(35000),
	}
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
		want   string
	}{
		{name: "string type", mutate: func(job map[string]any) { job["name"] = 1 }, want: "name 必须是字符串"},
		{name: "number type", mutate: func(job map[string]any) { job["requiredEdu"] = "本科" }, want: "requiredEdu 必须是数字"},
		{name: "salary range", mutate: func(job map[string]any) { job["minSalary"] = float64(40000) }, want: "minSalary 不能大于 maxSalary"},
	} {
		t.Run(test.name, func(t *testing.T) {
			job := make(map[string]any, len(base))
			for key, value := range base {
				job[key] = value
			}
			test.mutate(job)
			if err := validateRecruitJob(job); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("validation error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestRecruitListResultData(t *testing.T) {
	for _, test := range []struct {
		name string
		data any
		want string
	}{
		{name: "not object", data: []any{}, want: "必须是 JSON 对象"},
		{name: "missing hasMore", data: map[string]any{}, want: "缺少布尔字段 hasMore"},
		{name: "invalid cursor", data: map[string]any{"hasMore": true, "nextCursor": true}, want: "必须是字符串或数字"},
		{name: "missing cursor", data: map[string]any{"hasMore": true}, want: "缺少 nextCursor"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := recruitListResultData(test.data); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}

	data, meta, err := recruitListResultData(map[string]any{
		"jobs": []any{map[string]any{"jobId": "job-1"}}, "hasMore": true, "nextCursor": float64(12),
	})
	if err != nil {
		t.Fatal(err)
	}
	if meta.Count == nil || *meta.Count != 1 || meta.Pagination.EndpointExhausted ||
		meta.Pagination.NextToken != "12" || meta.Pagination.Pages != 1 || meta.Pagination.Items != 1 {
		t.Fatalf("meta = %#v", meta)
	}
	clean := data.(map[string]any)
	if _, exists := clean["hasMore"]; exists || clean["jobs"] == nil {
		t.Fatalf("clean data = %#v", clean)
	}

	_, terminal, err := recruitListResultData(map[string]any{"jobs": []any{}, "hasMore": false, "nextCursor": "ignored"})
	if err != nil || !terminal.Pagination.EndpointExhausted || terminal.Pagination.NextToken != "" {
		t.Fatalf("terminal meta = %#v, err = %v", terminal, err)
	}

	normalized, normalizedMeta, err := recruitListResultData(map[string]any{
		"list":       []any{map[string]any{"jobId": "job-2", "name": "测试工程师"}},
		"hasMore":    false,
		"nextCursor": nil,
		"totalCount": float64(1),
		"success":    true,
		"result":     map[string]any{"source": "business-payload"},
	})
	if err != nil {
		t.Fatal(err)
	}
	normalizedData := normalized.(map[string]any)
	jobs, ok := normalizedData["jobs"].([]any)
	if !ok || len(jobs) != 1 || normalizedData["totalCount"] != float64(1) {
		t.Fatalf("normalized data = %#v", normalizedData)
	}
	if _, exists := normalizedData["list"]; exists {
		t.Fatalf("normalized data retained Connector list field: %#v", normalizedData)
	}
	if normalizedData["success"] != true || !reflect.DeepEqual(
		normalizedData["result"], map[string]any{"source": "business-payload"},
	) {
		t.Fatalf("business success/result fields were unwrapped: %#v", normalizedData)
	}
	if normalizedMeta.Count == nil || *normalizedMeta.Count != 1 || !normalizedMeta.Pagination.EndpointExhausted {
		t.Fatalf("normalized meta = %#v", normalizedMeta)
	}
}

func TestRecruitResultCallPropagatesMCPError(t *testing.T) {
	caller := withRecruitCaller(t)
	caller.err = stderrors.New("transport failed")
	cmd := prepareRecruitTestCommand(newRecruitJobGetCommand())
	if _, err := recruitResultCall(cmd, recruitGetJobTool, map[string]any{"jobId": "job-1"}); err == nil {
		t.Fatal("expected MCP error")
	}
}

func TestRecruitMCPDataDecoderIsLosslessAndStrict(t *testing.T) {
	for _, test := range []struct {
		name    string
		text    string
		wantErr string
	}{
		{name: "empty", text: "   "},
		{name: "invalid JSON", text: "{", wantErr: "解析 list_jobs 返回失败"},
		{name: "multiple values", text: `{} {}`, wantErr: "存在多个 JSON 值"},
		{name: "invalid trailing value", text: `{} {`, wantErr: "解析 list_jobs 返回失败"},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := withRecruitCaller(t)
			caller.text = test.text
			data, err := callRecruitMCPToolData(context.Background(), recruitListJobsTool, map[string]any{})
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
				return
			}
			if err != nil || len(data.(map[string]any)) != 0 {
				t.Fatalf("empty response data = %#v, err = %v", data, err)
			}
		})
	}

	caller := withRecruitCaller(t)
	caller.text = `{"nextCursor":9223372036854775807}`
	data, err := callRecruitMCPToolData(context.Background(), recruitListJobsTool, map[string]any{})
	if err != nil || data.(map[string]any)["nextCursor"] != json.Number("9223372036854775807") {
		t.Fatalf("lossless recruit number data = %#v, err = %v", data, err)
	}
}

func TestRecruitResultCallUnwrapsConnectorEnvelopeForGetAndCreate(t *testing.T) {
	for _, test := range []struct {
		name string
		tool string
		text string
		want map[string]any
	}{
		{
			name: "get detail",
			tool: recruitGetJobTool,
			text: `{"success":true,"result":{"jobId":"job-detail","name":"Java 工程师","status":1}}`,
			want: map[string]any{"jobId": "job-detail", "name": "Java 工程师", "status": json.Number("1")},
		},
		{
			name: "create job",
			tool: recruitCreateJobTool,
			text: `{"success":true,"result":{"jobId":"job-created"}}`,
			want: map[string]any{"jobId": "job-created"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := withRecruitCaller(t)
			caller.text = test.text
			data, err := callRecruitMCPToolData(context.Background(), test.tool, map[string]any{})
			if err != nil {
				t.Fatal(err)
			}
			clean, err := recruitBusinessResultData(data, test.tool)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(clean, test.want) {
				t.Fatalf("normalized data = %#v, want %#v", clean, test.want)
			}
			if _, hasSuccess := clean["success"]; hasSuccess {
				t.Fatalf("normalized data retained Connector envelope: %#v", clean)
			}
			if _, hasResult := clean["result"]; hasResult {
				t.Fatalf("normalized data retained nested result: %#v", clean)
			}
		})
	}
}

func TestRecruitCommandsPublishUnwrappedConnectorResult(t *testing.T) {
	for _, test := range []struct {
		name   string
		leaf   func() *cobra.Command
		args   func(*testing.T) []string
		text   string
		wantID string
	}{
		{
			name:   "get detail",
			leaf:   newRecruitJobGetCommand,
			args:   func(*testing.T) []string { return []string{"--job-id", "job-detail"} },
			text:   `{"success":true,"result":{"jobId":"job-detail","name":"Java 工程师","status":1}}`,
			wantID: "job-detail",
		},
		{
			name: "create job",
			leaf: newRecruitJobCreateCommand,
			args: func(t *testing.T) []string {
				path, _ := writeRecruitJobFixture(t)
				return []string{"--from", path, "--yes"}
			},
			text:   `{"success":true,"result":{"jobId":"job-created"}}`,
			wantID: "job-created",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			caller := withRecruitCaller(t)
			caller.text = test.text
			root := &cobra.Command{Use: "dws", SilenceErrors: true, SilenceUsage: true}
			ctx, _ := output.WithResultStore(context.Background())
			root.SetContext(ctx)
			root.PersistentFlags().String("format", "json", "")
			root.PersistentFlags().String("fields", "", "")
			root.PersistentFlags().String("jq", "", "")
			root.PersistentFlags().Bool("dry-run", false, "")
			root.PersistentFlags().Bool("yes", false, "")
			root.PersistentPostRunE = func(cmd *cobra.Command, _ []string) error {
				_, _, err := output.EmitStoredResult(cmd)
				return err
			}
			leaf := test.leaf()
			root.AddCommand(leaf)
			var stdout bytes.Buffer
			root.SetOut(&stdout)
			root.SetArgs(append([]string{leaf.Name()}, test.args(t)...))
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Data map[string]any `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatalf("decode output %q: %v", stdout.String(), err)
			}
			if envelope.Data["jobId"] != test.wantID {
				t.Fatalf("data = %#v, want top-level jobId %q", envelope.Data, test.wantID)
			}
			if _, exists := envelope.Data["success"]; exists {
				t.Fatalf("public data retained Connector success: %#v", envelope.Data)
			}
			if _, exists := envelope.Data["result"]; exists {
				t.Fatalf("public data retained Connector result: %#v", envelope.Data)
			}
		})
	}
}

func TestRecruitBusinessResultDataRejectsInvalidConnectorEnvelope(t *testing.T) {
	for _, test := range []struct {
		name string
		data any
		want string
	}{
		{name: "not object", data: []any{}, want: "必须是 JSON 对象"},
		{name: "missing result", data: map[string]any{"success": true}, want: "同时包含 success 和 result"},
		{name: "missing success", data: map[string]any{"result": map[string]any{}}, want: "同时包含 success 和 result"},
		{name: "invalid success", data: map[string]any{"success": "true", "result": map[string]any{}}, want: "success 必须是布尔值"},
		{name: "business failure", data: map[string]any{"success": false, "message": "职位不存在", "result": map[string]any{}}, want: "职位不存在"},
		{name: "business failure without message", data: map[string]any{"success": false, "result": map[string]any{}}, want: "Connector 返回 success=false"},
		{name: "invalid result", data: map[string]any{"success": true, "result": nil}, want: "result 必须是 JSON 对象"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := recruitBusinessResultData(test.data, recruitGetJobTool)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
			var businessFailure *recruitBusinessFailure
			isBusinessFailure := stderrors.As(err, &businessFailure)
			wantBusinessFailure := strings.HasPrefix(test.name, "business failure")
			if isBusinessFailure != wantBusinessFailure {
				t.Fatalf("business failure classification = %t, want %t", isBusinessFailure, wantBusinessFailure)
			}
		})
	}
}

func TestRecruitResultCallClassifiesBusinessAndMalformedFailures(t *testing.T) {
	for _, test := range []struct {
		name        string
		text        string
		wantSubtype string
		wantMessage string
		wantHint    bool
	}{
		{
			name:        "business failure",
			text:        `{"success":false,"message":"职位不存在","result":{}}`,
			wantMessage: "职位不存在",
		},
		{
			name:        "malformed envelope",
			text:        `{"success":true}`,
			wantSubtype: "invalid_response",
			wantMessage: "同时包含 success 和 result",
			wantHint:    true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var payload any
			if err := json.Unmarshal([]byte(test.text), &payload); err != nil {
				t.Fatal(err)
			}
			_, responseErr := recruitBusinessResultData(payload, recruitGetJobTool)
			if responseErr == nil {
				t.Fatal("expected response error")
			}
			result := recruitResponseFailure(responseErr)
			envelope, err := output.EnvelopeFromResult(result)
			if err != nil {
				t.Fatal(err)
			}
			if envelope.Outcome != output.OutcomeFailure || envelope.Error == nil {
				t.Fatalf("envelope = %#v", envelope)
			}
			if envelope.Error.Type != "api" || envelope.Error.Subtype != test.wantSubtype ||
				!strings.Contains(envelope.Error.Message, test.wantMessage) {
				t.Fatalf("error = %#v", envelope.Error)
			}
			if (envelope.Error.Hint != "") != test.wantHint {
				t.Fatalf("hint = %q, want present %t", envelope.Error.Hint, test.wantHint)
			}
		})
	}
}

func TestRecruitResultCallRejectsInconsistentPagination(t *testing.T) {
	caller := withRecruitCaller(t)
	caller.text = `{"jobs":[],"hasMore":true}`
	cmd := prepareRecruitTestCommand(newRecruitJobListCommand())
	result, err := recruitResultCall(cmd, recruitListJobsTool, map[string]any{"size": 20})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome() != output.OutcomeFailure {
		t.Fatalf("outcome = %s, want failure", result.Outcome())
	}
}

func TestRecruitResultCallDryRun(t *testing.T) {
	caller := withRecruitCaller(t)
	caller.dryRun = true
	cmd := prepareRecruitTestCommand(newRecruitJobGetCommand())
	result, err := recruitResultCall(cmd, recruitGetJobTool, map[string]any{"jobId": "job-1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome() != output.OutcomeSuccess || len(caller.calls) != 0 {
		t.Fatalf("outcome = %s, calls = %d", result.Outcome(), len(caller.calls))
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

func TestRecruitResultAndPaginationContractsReachContractFinal(t *testing.T) {
	root := newRecruitCommand()
	tests := []struct {
		path        []string
		canonical   string
		resultField string
		pagination  bool
	}{
		{path: []string{"job", "list"}, canonical: "recruit.list_jobs", resultField: "jobs", pagination: true},
		{path: []string{"job", "get"}, canonical: "recruit.get_job_detail", resultField: "jobId"},
		{path: []string{"job", "create"}, canonical: "recruit.create_job", resultField: "jobId"},
	}
	for _, test := range tests {
		t.Run(test.canonical, func(t *testing.T) {
			leaf, _, err := root.Find(test.path)
			if err != nil {
				t.Fatal(err)
			}
			final, ok := contractfinal.RuntimeContractFinal(leaf)
			if !ok || final.Identity == nil || final.Identity.CanonicalPath != test.canonical {
				t.Fatalf("final identity = %#v, found = %v", final.Identity, ok)
			}
			if final.Result == nil {
				t.Fatal("final Result is nil")
			}
			wantOutcomes := []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure}
			if !reflect.DeepEqual(final.Result.Outcomes, wantOutcomes) {
				t.Fatalf("outcomes = %#v, want %#v", final.Result.Outcomes, wantOutcomes)
			}
			var schema struct {
				Type       string                     `json:"type"`
				Properties map[string]json.RawMessage `json:"properties"`
			}
			if err := json.Unmarshal(final.Result.DataSchema, &schema); err != nil {
				t.Fatalf("unmarshal result data_schema: %v", err)
			}
			if schema.Type != "object" || len(schema.Properties[test.resultField]) == 0 {
				t.Fatalf("result data_schema = %s, want object property %q", final.Result.DataSchema, test.resultField)
			}
			if test.pagination {
				if final.Pagination == nil || final.Pagination.Kind != contract.PaginationKindCursor ||
					final.Pagination.CursorParameter != "cursor" || final.Pagination.MetaPath != contract.PaginationMetaPath {
					t.Fatalf("pagination = %#v, want cursor contract", final.Pagination)
				}
			} else if final.Pagination != nil {
				t.Fatalf("pagination = %#v, want nil", final.Pagination)
			}
		})
	}
}
