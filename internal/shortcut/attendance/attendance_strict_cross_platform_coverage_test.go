// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package attendance

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func TestCrossPlatformCoveragePublicAttendanceContractsAreUnifiedAndTyped(t *testing.T) {
	shortcuts := []struct {
		name         string
		rollout      output.RolloutState
		hasResult    bool
		resultSchema string
		identity     string
		cursor       string
	}{
		{CheckResult.Command, CheckResult.OutputRollout, CheckResult.Contract.Result != nil, string(CheckResult.Contract.Result.DataSchema), `"id"`, "offset"},
		{CheckRecord.Command, CheckRecord.OutputRollout, CheckRecord.Contract.Result != nil, string(CheckRecord.Contract.Result.DataSchema), `"id"`, ""},
		{ListApprove.Command, ListApprove.OutputRollout, ListApprove.Contract.Result != nil, string(ListApprove.Contract.Result.DataSchema), `"id"`, ""},
		{GetApproveTemplate.Command, GetApproveTemplate.OutputRollout, GetApproveTemplate.Contract.Result != nil, string(GetApproveTemplate.Contract.Result.DataSchema), `"approveType"`, ""},
		{GetSchedule.Command, GetSchedule.OutputRollout, GetSchedule.Contract.Result != nil, string(GetSchedule.Contract.Result.DataSchema), `"id"`, ""},
		{SearchClass.Command, SearchClass.OutputRollout, SearchClass.Contract.Result != nil, string(SearchClass.Contract.Result.DataSchema), `"classId"`, "page"},
		{GetClass.Command, GetClass.OutputRollout, GetClass.Contract.Result != nil, string(GetClass.Contract.Result.DataSchema), `"shiftVO"`, ""},
		{SearchAdjustmentRule.Command, SearchAdjustmentRule.OutputRollout, SearchAdjustmentRule.Contract.Result != nil, string(SearchAdjustmentRule.Contract.Result.DataSchema), `"ruleId"`, "page"},
		{GetOvertimeRule.Command, GetOvertimeRule.OutputRollout, GetOvertimeRule.Contract.Result != nil, string(GetOvertimeRule.Contract.Result.DataSchema), `"id"`, ""},
		{SearchOvertimeRule.Command, SearchOvertimeRule.OutputRollout, SearchOvertimeRule.Contract.Result != nil, string(SearchOvertimeRule.Contract.Result.DataSchema), `"ruleId"`, "page"},
		{GetSelfSetting.Command, GetSelfSetting.OutputRollout, GetSelfSetting.Contract.Result != nil, string(GetSelfSetting.Contract.Result.DataSchema), `"userId"`, ""},
	}
	for _, item := range shortcuts {
		if item.rollout != output.RolloutUnifiedActive {
			t.Errorf("%s rollout = %q, want unified_active", item.name, item.rollout)
		}
		if !item.hasResult || strings.TrimSpace(item.resultSchema) == "" {
			t.Errorf("%s must publish a non-empty Result", item.name)
		}
		if !json.Valid([]byte(item.resultSchema)) || !strings.Contains(item.resultSchema, item.identity) {
			t.Errorf("%s Result must declare stable identity %s: %s", item.name, item.identity, item.resultSchema)
		}
		if len(attendanceDeclarationByName(item.name).Contract.Result.SensitivePaths) == 0 {
			t.Errorf("%s must publish reviewed sensitive_paths", item.name)
		}
		if strings.Contains(item.resultSchema, `"complete"`) || strings.Contains(item.resultSchema, `"nextPage"`) || strings.Contains(item.resultSchema, `"nextOffset"`) {
			t.Errorf("%s Result must not place pagination controls in business data", item.name)
		}
		if item.cursor == "" {
			if itemPagination := attendanceDeclarationByName(item.name).Contract.Pagination; itemPagination != nil {
				t.Errorf("%s unexpectedly publishes Pagination", item.name)
			}
		} else if pagination := attendanceDeclarationByName(item.name).Contract.Pagination; pagination == nil || pagination.CursorParameter != item.cursor {
			t.Errorf("%s Pagination = %#v, want cursor_parameter=%s", item.name, pagination, item.cursor)
		}
	}
}

func attendanceDeclarationByName(name string) *shortcut.Shortcut {
	for _, declaration := range []*shortcut.Shortcut{&CheckResult, &CheckRecord, &ListApprove, &GetApproveTemplate, &GetSchedule, &SearchClass, &GetClass, &SearchAdjustmentRule, &GetOvertimeRule, &SearchOvertimeRule, &GetSelfSetting} {
		if declaration.Command == name {
			return declaration
		}
	}
	return &shortcut.Shortcut{}
}

func TestCrossPlatformCoverageAttendanceEmptyOnlyLeavesAreUnavailable(t *testing.T) {
	for _, declaration := range []*shortcut.Shortcut{&GetSummary, &ListLeaveTypes, &GetLeaveRecords, &GetCheckinRecord} {
		if declaration.OutputRollout != output.RolloutLegacyOnly || declaration.Contract.Result != nil || declaration.Contract.Pagination != nil {
			t.Errorf("%s unavailable contract = rollout %q result=%v pagination=%v", declaration.Command, declaration.OutputRollout, declaration.Contract.Result, declaration.Contract.Pagination)
		}
		if declaration.Contract.Interface == nil || declaration.Contract.Interface.Availability != "unavailable" || strings.TrimSpace(declaration.Contract.Interface.Reason) == "" {
			t.Errorf("%s must publish a precise unavailable Interface", declaration.Command)
		}
	}
}

func TestCrossPlatformCoverageAttendanceProjectorsDistinguishEmptyFromBroken(t *testing.T) {
	explicitEmpty := map[string]any{
		"success": true,
		"result": map[string]any{
			"items":          []any{},
			"adjustmentList": []any{},
			"atRuleList":     []any{},
		},
	}
	if got, err := searchClassProject(explicitEmpty); err != nil || len(got) != 0 {
		t.Fatalf("explicit empty class list must succeed: got=%v err=%v", got, err)
	}
	if got, err := searchRuleProject(explicitEmpty, "attendance-wukong/test", "result.adjustmentList"); err != nil || len(got) != 0 {
		t.Fatalf("explicit empty rule list must succeed: got=%v err=%v", got, err)
	}
	if got, err := searchGroupProject(explicitEmpty); err != nil || len(got) != 0 {
		t.Fatalf("explicit empty group list must succeed: got=%v err=%v", got, err)
	}

	broken := []map[string]any{
		{},
		{"success": false, "result": []any{}},
		{"success": true, "result": map[string]any{}},
		{"success": true, "result": map[string]any{"items": "not-an-array"}},
		{"success": true, "result": map[string]any{"items": []any{map[string]any{"name": "missing-id"}}}},
	}
	for index, data := range broken {
		if got, err := searchClassProject(data); err == nil {
			t.Errorf("broken class response %d returned success: %v", index, got)
		}
		if got, err := searchGroupProject(data); err == nil {
			t.Errorf("broken group response %d returned success: %v", index, got)
		}
	}

	ruleBroken := map[string]any{
		"success": true,
		"result": map[string]any{
			"adjustmentList": []any{map[string]any{"entityVO": map[string]any{"name": "missing-id"}}},
		},
	}
	if got, err := searchRuleProject(ruleBroken, "attendance-wukong/test", "result.adjustmentList"); err == nil {
		t.Fatalf("rule row without stable ID returned success: %v", got)
	}
}

func TestCrossPlatformCoverageAttendancePaginationRequiresCompletionEvidence(t *testing.T) {
	withoutEvidence := map[string]any{"success": true, "result": map[string]any{"items": []any{}}}
	if _, _, err := attendancePageEvidence(withoutEvidence, "attendance-wukong/test", 1, 20, 0); err == nil {
		t.Fatal("missing totalCount/totalPage must not be reported as a complete page")
	}
	firstPage := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "totalCount": float64(21)}}
	if _, _, err := attendancePageEvidence(firstPage, "attendance-wukong/test", 1, 20, 0); err == nil {
		t.Fatal("nonzero total with empty page must fail closed")
	}
	complete, extra, err := attendancePageEvidence(firstPage, "attendance-wukong/test", 1, 20, 20)
	if err != nil {
		t.Fatalf("valid page evidence: %v", err)
	}
	if complete || extra["nextPage"] != 2 {
		t.Fatalf("first page should be incomplete with nextPage=2: complete=%v extra=%v", complete, extra)
	}
	lastPage := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "totalPage": float64(2)}}
	complete, extra, err = attendancePageEvidence(lastPage, "attendance-wukong/test", 2, 20, 1)
	if err != nil || !complete {
		t.Fatalf("last page should be complete: complete=%v extra=%v err=%v", complete, extra, err)
	}
	conflicting := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "totalPage": float64(1), "totalCount": float64(21)}}
	if _, _, err := attendancePageEvidence(conflicting, "attendance-wukong/test", 1, 20, 20); err == nil {
		t.Fatal("conflicting totalPage/totalCount evidence must fail closed")
	}
	mismatchedPage := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "currentPage": float64(2), "totalCount": float64(0)}}
	if _, _, err := attendancePageEvidence(mismatchedPage, "attendance-wukong/test", 1, 20, 0); err == nil {
		t.Fatal("mismatched currentPage must fail closed")
	}
	negativeTotal := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "totalCount": float64(-1)}}
	if _, _, err := attendancePageEvidence(negativeTotal, "attendance-wukong/test", 1, 20, 0); err == nil {
		t.Fatal("negative pagination evidence must fail closed")
	}
	wrongType := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "totalCount": "0"}}
	if _, _, err := attendancePageEvidence(wrongType, "attendance-wukong/test", 1, 20, 0); err == nil {
		t.Fatal("wrong pagination type must fail closed")
	}
}

func TestCrossPlatformCoverageAttendanceStableIdentityRejectsZeroBlankAndDuplicates(t *testing.T) {
	for _, items := range [][]map[string]any{{{"id": 0}}, {{"id": ""}}, {{"id": 1}, {"id": 1}}} {
		if err := attendanceValidatePositiveIntegerIDs(items, "attendance/test", "id"); err == nil {
			t.Fatalf("invalid integer identity accepted: %#v", items)
		}
	}
	if err := attendanceValidatePositiveIntegerIDs([]map[string]any{{"id": float64(1)}, {"id": int64(2)}}, "attendance/test", "id"); err != nil {
		t.Fatalf("valid integer identities rejected: %v", err)
	}
	if err := attendanceValidateExpectedStrings([]map[string]any{{"approveType": ""}}, "attendance/test", "approveType", "LEAVE"); err == nil {
		t.Fatal("blank string identity accepted")
	}
	if err := attendanceValidateExpectedStrings([]map[string]any{{"approveType": "LEAVE"}, {"approveType": "LEAVE"}}, "attendance/test", "approveType", "LEAVE"); err == nil {
		t.Fatal("duplicate string identity accepted")
	}
}
