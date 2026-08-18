// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package smart

import (
	"encoding/json"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/responsecheck"
)

func attendanceRecordsResult() *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes:   []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: json.RawMessage(`{"type":"object","description":"严格校验的个人考勤打卡流水","properties":{"count":{"type":"integer","description":"有效打卡流水数量"},"records":{"type":"array","description":"个人打卡流水","items":{"type":"object","description":"打卡流水记录","additionalProperties":true}},"complete":{"type":"boolean","description":"当前时间窗内结果是否完整"}},"required":["count","records","complete"],"additionalProperties":false}`),
	}
}

func outputStrictAttendanceRecords(rt *shortcut.RuntimeContext, data map[string]any) error {
	records, err := responsecheck.RequireObjectCollection(data, "attendance-wukong/query_check_record", "result")
	if err != nil {
		return err
	}
	return rt.Output(map[string]any{
		"count":    len(records),
		"records":  records,
		"complete": true,
	})
}
