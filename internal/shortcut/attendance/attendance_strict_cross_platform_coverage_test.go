// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package attendance

import (
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestCrossPlatformCoveragePublicAttendanceContractsAreUnifiedAndTyped(t *testing.T) {
	shortcuts := []struct {
		name         string
		rollout      output.RolloutState
		hasResult    bool
		resultSchema string
	}{
		{CheckResult.Command, CheckResult.OutputRollout, CheckResult.Contract.Result != nil, string(CheckResult.Contract.Result.DataSchema)},
		{CheckRecord.Command, CheckRecord.OutputRollout, CheckRecord.Contract.Result != nil, string(CheckRecord.Contract.Result.DataSchema)},
		{ListApprove.Command, ListApprove.OutputRollout, ListApprove.Contract.Result != nil, string(ListApprove.Contract.Result.DataSchema)},
		{GetApproveTemplate.Command, GetApproveTemplate.OutputRollout, GetApproveTemplate.Contract.Result != nil, string(GetApproveTemplate.Contract.Result.DataSchema)},
		{GetSchedule.Command, GetSchedule.OutputRollout, GetSchedule.Contract.Result != nil, string(GetSchedule.Contract.Result.DataSchema)},
		{SearchClass.Command, SearchClass.OutputRollout, SearchClass.Contract.Result != nil, string(SearchClass.Contract.Result.DataSchema)},
		{GetClass.Command, GetClass.OutputRollout, GetClass.Contract.Result != nil, string(GetClass.Contract.Result.DataSchema)},
		{GetAdjustmentRule.Command, GetAdjustmentRule.OutputRollout, GetAdjustmentRule.Contract.Result != nil, string(GetAdjustmentRule.Contract.Result.DataSchema)},
		{SearchAdjustmentRule.Command, SearchAdjustmentRule.OutputRollout, SearchAdjustmentRule.Contract.Result != nil, string(SearchAdjustmentRule.Contract.Result.DataSchema)},
		{GetOvertimeRule.Command, GetOvertimeRule.OutputRollout, GetOvertimeRule.Contract.Result != nil, string(GetOvertimeRule.Contract.Result.DataSchema)},
		{SearchOvertimeRule.Command, SearchOvertimeRule.OutputRollout, SearchOvertimeRule.Contract.Result != nil, string(SearchOvertimeRule.Contract.Result.DataSchema)},
		{SearchGroup.Command, SearchGroup.OutputRollout, SearchGroup.Contract.Result != nil, string(SearchGroup.Contract.Result.DataSchema)},
		{GetSummary.Command, GetSummary.OutputRollout, GetSummary.Contract.Result != nil, string(GetSummary.Contract.Result.DataSchema)},
		{GetSelfSetting.Command, GetSelfSetting.OutputRollout, GetSelfSetting.Contract.Result != nil, string(GetSelfSetting.Contract.Result.DataSchema)},
		{QueryReportData.Command, QueryReportData.OutputRollout, QueryReportData.Contract.Result != nil, string(QueryReportData.Contract.Result.DataSchema)},
		{ListLeaveTypes.Command, ListLeaveTypes.OutputRollout, ListLeaveTypes.Contract.Result != nil, string(ListLeaveTypes.Contract.Result.DataSchema)},
		{GetLeaveRecords.Command, GetLeaveRecords.OutputRollout, GetLeaveRecords.Contract.Result != nil, string(GetLeaveRecords.Contract.Result.DataSchema)},
		{GetCheckinRecord.Command, GetCheckinRecord.OutputRollout, GetCheckinRecord.Contract.Result != nil, string(GetCheckinRecord.Contract.Result.DataSchema)},
	}
	for _, item := range shortcuts {
		if item.rollout != output.RolloutUnifiedActive {
			t.Errorf("%s rollout = %q, want unified_active", item.name, item.rollout)
		}
		if !item.hasResult || strings.TrimSpace(item.resultSchema) == "" {
			t.Errorf("%s must publish a non-empty Result", item.name)
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
	if _, _, err := attendancePageEvidence(withoutEvidence, "attendance-wukong/test", 1, 20); err == nil {
		t.Fatal("missing totalCount/totalPage must not be reported as a complete page")
	}
	firstPage := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "totalCount": float64(21)}}
	complete, extra, err := attendancePageEvidence(firstPage, "attendance-wukong/test", 1, 20)
	if err != nil {
		t.Fatalf("valid page evidence: %v", err)
	}
	if complete || extra["nextPage"] != 2 {
		t.Fatalf("first page should be incomplete with nextPage=2: complete=%v extra=%v", complete, extra)
	}
	lastPage := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "totalPage": float64(2)}}
	complete, extra, err = attendancePageEvidence(lastPage, "attendance-wukong/test", 2, 20)
	if err != nil || !complete {
		t.Fatalf("last page should be complete: complete=%v extra=%v err=%v", complete, extra, err)
	}
	conflicting := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "totalPage": float64(1), "totalCount": float64(21)}}
	if _, _, err := attendancePageEvidence(conflicting, "attendance-wukong/test", 1, 20); err == nil {
		t.Fatal("conflicting totalPage/totalCount evidence must fail closed")
	}
	mismatchedPage := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "currentPage": float64(2), "totalCount": float64(0)}}
	if _, _, err := attendancePageEvidence(mismatchedPage, "attendance-wukong/test", 1, 20); err == nil {
		t.Fatal("mismatched currentPage must fail closed")
	}
	negativeTotal := map[string]any{"success": true, "result": map[string]any{"items": []any{}, "totalCount": float64(-1)}}
	if _, _, err := attendancePageEvidence(negativeTotal, "attendance-wukong/test", 1, 20); err == nil {
		t.Fatal("negative pagination evidence must fail closed")
	}
}
