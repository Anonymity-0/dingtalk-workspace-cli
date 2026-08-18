// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package report

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/spf13/cobra"
)

type reportCoverageCall struct {
	tool string
	args map[string]any
}

type reportCoverageCaller struct {
	responses map[string][]string
	history   []reportCoverageCall
}

func (caller *reportCoverageCaller) CallTool(_ context.Context, _, tool string, args map[string]any) (*edition.ToolResult, error) {
	caller.history = append(caller.history, reportCoverageCall{tool: tool, args: args})
	queue := caller.responses[tool]
	if len(queue) == 0 {
		return nil, errors.New("missing Report fake response for " + tool)
	}
	caller.responses[tool] = queue[1:]
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: queue[0]}}}, nil
}

func (*reportCoverageCaller) Format() string { return "json" }
func (*reportCoverageCaller) DryRun() bool   { return false }
func (*reportCoverageCaller) Fields() string { return "" }
func (*reportCoverageCaller) JQ() string     { return "" }

func runReportCoverage(t *testing.T, declaration shortcut.Shortcut, caller *reportCoverageCaller, args ...string) (*cobra.Command, error) {
	t.Helper()
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(declaration))
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	return cmd, cmd.Execute()
}

func TestCrossPlatformCoverageReportContractsAreStrictTypedAndUnified(t *testing.T) {
	declarations := []shortcut.Shortcut{InboxList, OutboxList, TemplateSearch, ReportLatest}
	for _, declaration := range declarations {
		if declaration.Contract.Empty() || declaration.Contract.Result == nil {
			t.Errorf("%s lacks Contract/Result", declaration.Command)
		}
		if declaration.Safety.Effect != "read" || declaration.Safety.Confirmation != "not_required" {
			t.Errorf("%s safety=%+v", declaration.Command, declaration.Safety)
		}
		if declaration.OutputRollout != output.RolloutUnifiedActive {
			t.Errorf("%s rollout=%q", declaration.Command, declaration.OutputRollout)
		}
		if declaration.Contract.Interface == nil || declaration.Contract.Interface.Availability != "available" {
			t.Errorf("%s interface=%+v", declaration.Command, declaration.Contract.Interface)
		}
	}
	if InboxList.Contract.Pagination == nil || OutboxList.Contract.Pagination == nil {
		t.Fatal("Report list shortcuts must publish cursor pagination")
	}
	if TemplateSearch.Contract.Pagination != nil || ReportLatest.Contract.Pagination != nil {
		t.Fatal("non-paginated Report shortcuts published pagination")
	}
}

func TestCrossPlatformCoverageReportListResponseMatrix(t *testing.T) {
	valid := map[string]any{
		"success": true,
		"result": map[string]any{
			"report_list": []any{map[string]any{"reportId": "report-1", "templateName": "fixture", "createTime": float64(10)}},
		},
		"hasMore": false,
	}
	entries, page, err := reportProjectEntries(valid, "report/get_received_report_list")
	if err != nil || len(entries) != 1 || page.HasMore || entries[0]["createTime"] != int64(10) {
		t.Fatalf("valid projection entries=%#v page=%+v err=%v", entries, page, err)
	}
	empty := map[string]any{"success": true, "result": map[string]any{"report_list": []any{}}, "hasMore": false, "cursor": nil}
	if entries, page, err := reportProjectEntries(empty, "report/get_received_report_list"); err != nil || len(entries) != 0 || page.HasMore {
		t.Fatalf("terminal empty entries=%#v page=%+v err=%v", entries, page, err)
	}
	terminalReceipt := map[string]any{"success": true, "result": map[string]any{"report_list": []any{map[string]any{"reportId": "report-1"}}}, "hasMore": false, "nextCursor": float64(1)}
	if entries, page, err := reportProjectEntries(terminalReceipt, "report/get_received_report_list"); err != nil || len(entries) != 1 || page.HasMore || page.Next != "" {
		t.Fatalf("terminal cursor receipt entries=%#v page=%+v err=%v", entries, page, err)
	}
	fixtures := map[string]map[string]any{
		"empty response":     map[string]any{},
		"missing success":    map[string]any{"result": map[string]any{"report_list": []any{}}, "hasMore": false},
		"wrong success":      map[string]any{"success": "true", "result": map[string]any{"report_list": []any{}}, "hasMore": false},
		"remote failure":     map[string]any{"success": false, "result": map[string]any{"report_list": []any{}}, "hasMore": false},
		"missing result":     map[string]any{"success": true, "hasMore": false},
		"wrong result":       map[string]any{"success": true, "result": "bad", "hasMore": false},
		"missing collection": map[string]any{"success": true, "result": map[string]any{}, "hasMore": false},
		"wrong collection":   map[string]any{"success": true, "result": map[string]any{"report_list": map[string]any{}}, "hasMore": false},
		"bad item":           map[string]any{"success": true, "result": map[string]any{"report_list": []any{"bad"}}, "hasMore": false},
		"empty item":         map[string]any{"success": true, "result": map[string]any{"report_list": []any{map[string]any{}}}, "hasMore": false},
		"missing identity":   map[string]any{"success": true, "result": map[string]any{"report_list": []any{map[string]any{"createTime": float64(1)}}}, "hasMore": false},
		"wrong identity":     map[string]any{"success": true, "result": map[string]any{"report_list": []any{map[string]any{"reportId": float64(1)}}}, "hasMore": false},
		"duplicate identity": map[string]any{"success": true, "result": map[string]any{"report_list": []any{map[string]any{"reportId": "same"}, map[string]any{"reportId": "same"}}}, "hasMore": false},
		"wrong optional":     map[string]any{"success": true, "result": map[string]any{"report_list": []any{map[string]any{"reportId": "report-1", "createTime": "1"}}}, "hasMore": false},
		"missing pagination": map[string]any{"success": true, "result": map[string]any{"report_list": []any{map[string]any{"reportId": "report-1"}}}},
		"wrong pagination":   map[string]any{"success": true, "result": map[string]any{"report_list": []any{}}, "hasMore": "false"},
		"empty continuation": map[string]any{"success": true, "result": map[string]any{"report_list": []any{}}, "hasMore": true, "nextCursor": float64(2)},
		"missing continuation": map[string]any{
			"success": true, "result": map[string]any{"report_list": []any{map[string]any{"reportId": "report-1"}}}, "hasMore": true,
		},
		"wrong continuation": map[string]any{
			"success": true, "result": map[string]any{"report_list": []any{map[string]any{"reportId": "report-1"}}}, "hasMore": true, "nextCursor": "2",
		},
		"wrong terminal cursor": map[string]any{
			"success": true, "result": map[string]any{"report_list": []any{}}, "hasMore": false, "nextCursor": "2",
		},
		"conflicting has more": map[string]any{
			"success": true, "result": map[string]any{"report_list": []any{}, "hasMore": true}, "hasMore": false,
		},
		"conflicting continuation": map[string]any{
			"success": true, "result": map[string]any{"report_list": []any{map[string]any{"reportId": "report-1"}}, "cursor": float64(3)}, "hasMore": true, "nextCursor": float64(2),
		},
	}
	for name, fixture := range fixtures {
		if projected, _, projectErr := reportProjectEntries(fixture, "report/get_received_report_list"); projectErr == nil {
			t.Errorf("%s returned success: %#v", name, projected)
		}
	}
	if err := reportValidateContinuation(reportPageEvidence{HasMore: true, Next: "2"}, 2, "report/list"); err == nil {
		t.Fatal("stalled continuation returned success")
	}
}

func TestCrossPlatformCoverageReportTemplateResponseMatrix(t *testing.T) {
	valid := map[string]any{"success": true, "items": []any{
		map[string]any{"report_template_id": "template-1", "report_template_name": "Fixture Weekly", "last_modified_time": float64(2)},
		map[string]any{"report_template_id": "template-2", "report_template_name": "Fixture Daily"},
	}}
	templates, err := reportProjectTemplates(valid, "report/get_available_report_templates")
	if err != nil || len(templates) != 2 || templates[0]["lastModifiedTime"] != int64(2) {
		t.Fatalf("valid templates=%#v err=%v", templates, err)
	}
	for name, fixture := range map[string]map[string]any{
		"empty response":     {},
		"missing success":    {"items": []any{}},
		"wrong success":      {"success": float64(1), "items": []any{}},
		"missing collection": {"success": true},
		"wrong collection":   {"success": true, "items": map[string]any{}},
		"bad item":           {"success": true, "items": []any{"bad"}},
		"missing id":         {"success": true, "items": []any{map[string]any{"report_template_name": "fixture"}}},
		"missing name":       {"success": true, "items": []any{map[string]any{"report_template_id": "template-1"}}},
		"wrong name":         {"success": true, "items": []any{map[string]any{"report_template_id": "template-1", "report_template_name": float64(1)}}},
		"duplicate id":       {"success": true, "items": []any{map[string]any{"report_template_id": "same", "report_template_name": "one"}, map[string]any{"report_template_id": "same", "report_template_name": "two"}}},
		"wrong modified":     {"success": true, "items": []any{map[string]any{"report_template_id": "template-1", "report_template_name": "fixture", "last_modified_time": "2"}}},
	} {
		if projected, projectErr := reportProjectTemplates(fixture, "report/get_available_report_templates"); projectErr == nil {
			t.Errorf("%s returned success: %#v", name, projected)
		}
	}
	if projected, err := reportProjectTemplates(map[string]any{"success": true, "items": []any{}}, "report/templates"); err != nil || len(projected) != 0 {
		t.Fatalf("explicit empty template collection=%#v err=%v", projected, err)
	}
}

func TestCrossPlatformCoverageReportDetailResponseMatrix(t *testing.T) {
	valid := map[string]any{"success": true, "result": map[string]any{
		"report_Id": "report-2", "report_name": "fixture", "createTime": float64(2),
		"report_content": []any{map[string]any{"key": "field", "value": "value", "sort": float64(1), "type": float64(2)}},
	}}
	detail, err := reportProjectDetail(valid, "report/get_report_entry_details", "report-2")
	if err != nil || detail["reportId"] != "report-2" || len(detail["fields"].([]map[string]any)) != 1 {
		t.Fatalf("valid detail=%#v err=%v", detail, err)
	}
	for name, fixture := range map[string]map[string]any{
		"empty response":     {},
		"missing result":     {"success": true},
		"empty result":       {"success": true, "result": map[string]any{}},
		"missing id":         {"success": true, "result": map[string]any{"report_content": []any{}}},
		"identity mismatch":  {"success": true, "result": map[string]any{"report_Id": "other", "report_content": []any{}}},
		"missing collection": {"success": true, "result": map[string]any{"report_Id": "report-2"}},
		"wrong collection":   {"success": true, "result": map[string]any{"report_Id": "report-2", "report_content": map[string]any{}}},
		"bad field":          {"success": true, "result": map[string]any{"report_Id": "report-2", "report_content": []any{"bad"}}},
		"missing field key":  {"success": true, "result": map[string]any{"report_Id": "report-2", "report_content": []any{map[string]any{"value": "value", "sort": float64(1), "type": float64(2)}}}},
		"wrong field type":   {"success": true, "result": map[string]any{"report_Id": "report-2", "report_content": []any{map[string]any{"key": "field", "value": "value", "sort": "1", "type": float64(2)}}}},
	} {
		if projected, projectErr := reportProjectDetail(fixture, "report/get_report_entry_details", "report-2"); projectErr == nil {
			t.Errorf("%s returned success: %#v", name, projected)
		}
	}
}

func TestCrossPlatformCoverageReportExactShortcutsProjectUnifiedData(t *testing.T) {
	templateCaller := &reportCoverageCaller{responses: map[string][]string{
		"get_available_report_templates": {`{"success":true,"items":[{"report_template_id":"template-1","report_template_name":"Fixture Weekly","last_modified_time":2},{"report_template_id":"template-2","report_template_name":"Fixture Daily"}]}`},
	}}
	cmd, err := runReportCoverage(t, TemplateSearch, templateCaller, "--query", "weekly")
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if code, emitted, emitErr := output.EmitStoredResult(cmd); emitErr != nil || !emitted || code != 0 {
		t.Fatalf("emit=(%d,%t,%v)", code, emitted, emitErr)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	if data["count"] != float64(1) || len(data["templates"].([]any)) != 1 {
		t.Fatalf("template search data=%#v", data)
	}
	if _, leaked := data["items"]; leaked {
		t.Fatalf("template search leaked raw collection: %#v", data)
	}

	inboxCaller := &reportCoverageCaller{responses: map[string][]string{
		"get_received_report_list": {`{"success":true,"result":{"report_list":[{"reportId":"report-1","createTime":1}]},"hasMore":false,"cursor":null}`},
	}}
	cmd, err = runReportCoverage(t, InboxList, inboxCaller,
		"--start", "2026-07-01T00:00:00+08:00", "--end", "2026-07-02T00:00:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	cmd.SetOut(&stdout)
	if _, emitted, emitErr := output.EmitStoredResult(cmd); emitErr != nil || !emitted {
		t.Fatalf("inbox emit=(%t,%v)", emitted, emitErr)
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data = envelope["data"].(map[string]any)
	if data["count"] != float64(1) || data["complete"] != true {
		t.Fatalf("inbox data=%#v", data)
	}
}

func TestCrossPlatformCoverageReportLatestRequiresCompleteOrderedListAndExactReadback(t *testing.T) {
	missingOrder := &reportCoverageCaller{responses: map[string][]string{
		"get_send_report_list": {`{"success":true,"result":{"report_list":[{"reportId":"report-1"}]},"hasMore":false}`},
	}}
	if _, err := runReportCoverage(t, ReportLatest, missingOrder); err == nil || len(missingOrder.history) != 1 {
		t.Fatalf("missing order err=%v history=%v", err, missingOrder.history)
	}

	incomplete := &reportCoverageCaller{responses: map[string][]string{
		"get_send_report_list": {`{"success":true,"result":{"report_list":[{"reportId":"report-1","createTime":1}]},"hasMore":true,"nextCursor":20}`},
	}}
	if _, err := runReportCoverage(t, ReportLatest, incomplete); err == nil || len(incomplete.history) != 1 {
		t.Fatalf("incomplete err=%v history=%v", err, incomplete.history)
	}

	tied := &reportCoverageCaller{responses: map[string][]string{
		"get_send_report_list": {`{"success":true,"result":{"report_list":[{"reportId":"report-1","createTime":2},{"reportId":"report-2","createTime":2}]},"hasMore":false}`},
	}}
	if _, err := runReportCoverage(t, ReportLatest, tied); err == nil || len(tied.history) != 1 {
		t.Fatalf("tied latest err=%v history=%v", err, tied.history)
	}

	caller := &reportCoverageCaller{responses: map[string][]string{
		"get_send_report_list":     {`{"success":true,"result":{"report_list":[{"reportId":"report-1","createTime":1},{"reportId":"report-2","createTime":2}]},"hasMore":false}`},
		"get_report_entry_details": {`{"success":true,"result":{"report_Id":"report-2","report_name":"fixture","createTime":2,"report_content":[{"key":"field","value":"value","sort":1,"type":2}]}}`},
	}}
	cmd, err := runReportCoverage(t, ReportLatest, caller,
		"--start", "2026-07-01T00:00:00+08:00", "--end", "2026-07-20T00:00:00+08:00")
	if err != nil {
		t.Fatal(err)
	}
	if len(caller.history) != 2 || caller.history[0].tool != "get_send_report_list" || caller.history[1].tool != "get_report_entry_details" || caller.history[1].args["report_id"] != "report-2" {
		t.Fatalf("exact call history=%#v", caller.history)
	}
	if caller.history[0].args["startTime"] != int64(1782835200000) || caller.history[0].args["endTime"] != int64(1784476800000) {
		t.Fatalf("explicit range args=%#v", caller.history[0].args)
	}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if _, emitted, emitErr := output.EmitStoredResult(cmd); emitErr != nil || !emitted {
		t.Fatalf("latest emit=(%t,%v)", emitted, emitErr)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	projected := envelope["data"].(map[string]any)["report"].(map[string]any)
	_, leaked := projected["success"]
	if projected["reportId"] != "report-2" || leaked {
		t.Fatalf("latest projection=%s", stdout.String())
	}
}

func TestCrossPlatformCoverageReportValidationRejectsInvalidRangesBeforeMCP(t *testing.T) {
	tests := []struct {
		declaration shortcut.Shortcut
		args        []string
	}{
		{InboxList, []string{"--start", "2026-07-02T00:00:00+08:00", "--end", "2026-07-01T00:00:00+08:00"}},
		{InboxList, []string{"--start", "2026-07-01T00:00:00+08:00", "--end", "2026-07-02T00:00:00+08:00", "--size", "21"}},
		{OutboxList, []string{"--modified-start", "2026-07-01T00:00:00+08:00"}},
		{ReportLatest, []string{"--start", "2026-07-01T00:00:00+08:00"}},
		{ReportLatest, []string{"--start", "2026-07-22T00:00:00+08:00", "--end", "2026-07-01T00:00:00+08:00"}},
	}
	for _, test := range tests {
		caller := &reportCoverageCaller{responses: map[string][]string{}}
		if _, err := runReportCoverage(t, test.declaration, caller, test.args...); err == nil || len(caller.history) != 0 {
			t.Errorf("%s args=%v err=%v calls=%v", test.declaration.Command, test.args, err, caller.history)
		}
	}
}
