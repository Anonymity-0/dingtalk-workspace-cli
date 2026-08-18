// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package attendance

import (
	"encoding/json"
	"fmt"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut/responsecheck"
)

func attendanceObjectResult(description string) *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: json.RawMessage(fmt.Sprintf(
			`{"type":"object","description":%q,"properties":{"value":{"description":"严格校验后的考勤业务结果"}},"required":["value"],"additionalProperties":false}`,
			description,
		)),
	}
}

func attendanceCollectionResult(collection, description string) *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: json.RawMessage(fmt.Sprintf(
			`{"type":"object","description":%q,"properties":{"count":{"type":"integer","description":"当前响应中的有效业务记录数量"},%q:{"type":"array","description":%q,"items":{"type":"object","description":"考勤业务记录","additionalProperties":true}},"complete":{"type":"boolean","description":"现有服务端证据是否证明结果已经完整"},"page":{"type":"integer","description":"当前页码"},"limit":{"type":"integer","description":"请求的分页大小"},"totalCount":{"type":"integer","description":"服务端报告的总记录数"},"totalPage":{"type":"integer","description":"服务端报告的总页数"},"nextPage":{"type":"integer","description":"结果未完整时建议请求的下一页"},"nextOffset":{"type":"integer","description":"结果未完整时建议请求的下一偏移量"}},"required":["count",%q,"complete"],"additionalProperties":false}`,
			description, collection, description, collection,
		)),
	}
}

func hardenPublicAttendanceContracts() {
	collections := []struct {
		declaration *shortcut.Shortcut
		collection  string
		description string
	}{
		{&CheckResult, "records", "严格校验的员工打卡结果"},
		{&CheckRecord, "records", "严格校验的员工打卡流水"},
		{&ListApprove, "approvals", "严格校验的考勤审批记录"},
		{&GetApproveTemplate, "templates", "严格校验的考勤审批模板"},
		{&GetSchedule, "schedules", "严格校验的员工排班记录"},
		{&SearchClass, "classes", "严格校验的班次搜索结果"},
		{&SearchAdjustmentRule, "rules", "严格校验的补卡规则搜索结果"},
		{&SearchOvertimeRule, "rules", "严格校验的加班规则搜索结果"},
		{&SearchGroup, "groups", "严格校验的考勤组搜索结果"},
		{&ListLeaveTypes, "leaveTypes", "严格校验的假期类型列表"},
		{&GetLeaveRecords, "records", "严格校验的假期余额变更记录"},
		{&GetCheckinRecord, "records", "严格校验的签到记录"},
	}
	for _, item := range collections {
		item.declaration.OutputRollout = output.RolloutUnifiedActive
		item.declaration.Contract.Result = attendanceCollectionResult(item.collection, item.description)
	}
	objects := []struct {
		declaration *shortcut.Shortcut
		description string
	}{
		{&GetClass, "严格校验的班次详情"},
		{&GetAdjustmentRule, "严格校验的补卡规则详情"},
		{&GetOvertimeRule, "严格校验的加班规则详情"},
		{&GetSummary, "严格校验的个人考勤统计摘要"},
		{&GetSelfSetting, "严格校验的个人考勤设置"},
		{&QueryReportData, "严格校验的考勤报表数据"},
	}
	for _, item := range objects {
		item.declaration.OutputRollout = output.RolloutUnifiedActive
		item.declaration.Contract.Result = attendanceObjectResult(item.description)
	}
}

func attendanceCallCollection(
	rt *shortcut.RuntimeContext,
	product, tool string,
	params map[string]any,
	collection string,
	complete bool,
	extra map[string]any,
	paths ...string,
) error {
	data, err := rt.CallMCPData(product, tool, params)
	if err != nil {
		return err
	}
	items, err := responsecheck.RequireObjectCollection(data, product+"/"+tool, paths...)
	if err != nil {
		return err
	}
	payload := map[string]any{
		"count":    len(items),
		collection: items,
		"complete": complete,
	}
	for key, value := range extra {
		payload[key] = value
	}
	return rt.Output(payload)
}

func attendanceCallValue(rt *shortcut.RuntimeContext, product, tool string, params map[string]any) error {
	data, err := rt.CallMCPData(product, tool, params)
	if err != nil {
		return err
	}
	value, err := responsecheck.RequireResult(data, product+"/"+tool)
	if err != nil {
		return err
	}
	return rt.Output(map[string]any{"value": value})
}

func attendanceCallObject(rt *shortcut.RuntimeContext, product, tool string, params map[string]any) error {
	data, err := rt.CallMCPData(product, tool, params)
	if err != nil {
		return err
	}
	value, err := responsecheck.RequireSingleObjectResult(data, product+"/"+tool)
	if err != nil {
		return err
	}
	return rt.Output(map[string]any{"value": value})
}

func attendanceCollectionPayload(collection string, items []map[string]any, complete bool, extra map[string]any) map[string]any {
	payload := map[string]any{
		"count":    len(items),
		collection: items,
		"complete": complete,
	}
	for key, value := range extra {
		payload[key] = value
	}
	return payload
}

func attendancePageEvidence(data map[string]any, operation string, page, limit int) (bool, map[string]any, error) {
	result, err := responsecheck.RequireObjectResult(data, operation)
	if err != nil {
		return false, nil, err
	}
	extra := map[string]any{"page": page, "limit": limit}
	if currentPage, ok := attendanceInt(result["currentPage"]); ok && currentPage != page {
		return false, nil, responsecheck.Error(operation, "pagination_page_mismatch", "服务端 currentPage 与请求页码不一致")
	}
	var evidence []bool
	if totalPage, ok := attendanceInt(result["totalPage"]); ok {
		if totalPage < 0 {
			return false, nil, responsecheck.Error(operation, "invalid_pagination_evidence", "服务端 totalPage 不能为负数")
		}
		extra["totalPage"] = totalPage
		evidence = append(evidence, page >= totalPage)
	}
	if totalCount, ok := attendanceInt(result["totalCount"]); ok {
		if totalCount < 0 {
			return false, nil, responsecheck.Error(operation, "invalid_pagination_evidence", "服务端 totalCount 不能为负数")
		}
		extra["totalCount"] = totalCount
		evidence = append(evidence, page*limit >= totalCount)
	}
	if len(evidence) == 0 {
		return false, nil, responsecheck.Error(operation, "missing_pagination_evidence", "分页响应缺少 totalCount/totalPage，无法证明当前页是否完整")
	}
	complete := evidence[0]
	for _, candidate := range evidence[1:] {
		if candidate != complete {
			return false, nil, responsecheck.Error(operation, "conflicting_pagination_evidence", "服务端 totalCount 与 totalPage 对当前页是否完成给出矛盾证据")
		}
	}
	if !complete {
		extra["nextPage"] = page + 1
	}
	return complete, extra, nil
}

func attendancePageInput(rt *shortcut.RuntimeContext) (int, int, error) {
	page, limit := rt.Int("page"), rt.Int("limit")
	if page < 1 {
		return 0, 0, fmt.Errorf("--page 必须大于等于 1")
	}
	if limit < 1 || limit > 200 {
		return 0, 0, fmt.Errorf("--limit 必须在 1 到 200 之间")
	}
	return page, limit, nil
}

func attendanceInt(value any) (int, bool) {
	switch number := value.(type) {
	case int:
		return number, true
	case int32:
		return int(number), true
	case int64:
		return int(number), true
	case float64:
		if number != float64(int(number)) {
			return 0, false
		}
		return int(number), true
	default:
		return 0, false
	}
}
