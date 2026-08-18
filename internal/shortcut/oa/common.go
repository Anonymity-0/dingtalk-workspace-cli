// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package oa

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

const oaCompositeReason = "Reviewed OA Shortcut composite: the executable CLI owns strict business-success validation, exact collection paths, stable identity checks, truthful pagination, output projection, and confirmation; no single MCP interface represents the complete command contract."

type oaPageEvidence struct {
	Known   bool
	HasMore bool
	Next    string
}

func oaCollectionResult(collection, description string) *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: json.RawMessage(fmt.Sprintf(
			`{"type":"object","description":%q,"properties":{"count":{"type":"integer","description":"当前响应中的有效审批记录数量"},%q:{"type":"array","description":%q,"items":{"type":"object","description":"严格验证后的 OA 审批业务记录","additionalProperties":true}},"complete":{"type":"boolean","description":"服务端分页证据是否证明结果完整"}},"required":["count",%q,"complete"],"additionalProperties":true}`,
			description, collection, description, collection,
		)),
		SensitivePaths: []string{collection + ".title", collection + ".originatorName", collection + ".formValueVOS", collection + ".userId"},
	}
}

func oaObjectResult(description string) *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: json.RawMessage(fmt.Sprintf(
			`{"type":"object","description":%q,"properties":{"value":{"type":"object","description":"严格验证且身份匹配的 OA 审批对象","additionalProperties":true}},"required":["value"],"additionalProperties":false}`,
			description,
		)),
		SensitivePaths: []string{"value.title", "value.processInstanceTitle", "value.formValueVOS", "value.originatorUserid", "value.operationRecords", "value.userId"},
	}
}

func oaReadSafety() contract.SafetySpec {
	return contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "idempotent"}
}

func oaWriteSafety() contract.SafetySpec {
	return contract.SafetySpec{Effect: "write", Risk: "high", Confirmation: "user_required", Idempotency: "unknown"}
}

func oaContract(command, description, intent string, result *contract.ResultSpec, pagination *contract.PaginationSpec, params []contract.ParamDecl, examples ...string) corecmd.ContractDecl {
	name := "shortcut_" + strings.ReplaceAll(strings.TrimPrefix(command, "+"), "-", "_")
	cliPath := "oa " + command
	return corecmd.ContractDecl{
		Description: description,
		Result:      result,
		Pagination:  pagination,
		Parameters:  params,
		Identity: contract.ToolIdentitySpec{
			ProductID: "oa", Name: name, CanonicalPath: "oa." + name,
			CLIPath: cliPath, PrimaryCLIPath: cliPath,
		},
		Interface: &contract.InterfaceSpec{Mode: contract.InterfaceModeComposite, Availability: contract.InterfaceAvailable, Reason: oaCompositeReason},
		Selection: contract.SelectionSpec{
			AgentSummary: description,
			UseWhen:      []string{intent},
			AvoidWhen:    []string{"只需原始 OA MCP 响应时使用 oa approval 原子命令；缺少稳定实例、任务或表单身份时不要猜测"},
			Examples:     examples,
		},
	}
}

func oaPagePagination(parameter string) *contract.PaginationSpec {
	return &contract.PaginationSpec{
		Kind:                  contract.PaginationKindCursor,
		CursorParameter:       parameter,
		MetaPath:              contract.PaginationMetaPath,
		EndpointExhaustedPath: contract.PaginationExhaustedPath,
		NextTokenPath:         contract.PaginationNextTokenPath,
	}
}

func oaResponseError(operation, reason, message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithOperation(operation),
		apperrors.WithOrigin("mcp"),
		apperrors.WithFailureStage("response_validation"),
		apperrors.WithRetryable(false),
		apperrors.WithReason(reason),
	)
}

// oaRequireSuccess accepts only the two success encodings observed from the
// OA backend. Missing, false, null, and all other encodings fail closed.
func oaRequireSuccess(data map[string]any, operation string) error {
	if len(data) == 0 {
		return oaResponseError(operation, "empty_tool_response", "服务返回空响应，无法证明 OA 调用成功或结果确实为空")
	}
	switch value := data["success"].(type) {
	case bool:
		if value {
			return nil
		}
	case string:
		if value == "true" {
			return nil
		}
	}
	if _, present := data["success"]; !present {
		return oaResponseError(operation, "missing_success", "OA 响应缺少 success 业务状态")
	}
	message := oaFirstString(data, "errorMessage", "errorMsg", "message", "error")
	if message == "" {
		message = "OA 服务未明确返回成功状态"
	}
	return oaResponseError(operation, "remote_failure", message)
}

func oaLookup(object map[string]any, path string) (any, bool) {
	var current any = object
	for _, segment := range strings.Split(path, ".") {
		mapped, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = mapped[segment]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func oaRequireObject(data map[string]any, operation, path string) (map[string]any, error) {
	if err := oaRequireSuccess(data, operation); err != nil {
		return nil, err
	}
	value, present := oaLookup(data, path)
	if !present || value == nil {
		return nil, oaResponseError(operation, "missing_result", fmt.Sprintf("成功响应缺少非空 %s 对象", path))
	}
	object, ok := value.(map[string]any)
	if !ok || len(object) == 0 {
		return nil, oaResponseError(operation, "malformed_result", fmt.Sprintf("响应 %s 应为非空对象，实际为 %T", path, value))
	}
	return object, nil
}

func oaRequireCollection(data map[string]any, operation, path string) ([]map[string]any, error) {
	if err := oaRequireSuccess(data, operation); err != nil {
		return nil, err
	}
	value, present := oaLookup(data, path)
	if !present {
		return nil, oaResponseError(operation, "missing_collection", fmt.Sprintf("成功响应缺少 %s 数组；不能把未知响应结构当作空结果", path))
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, oaResponseError(operation, "malformed_collection", fmt.Sprintf("响应 %s 应为数组，实际为 %T", path, value))
	}
	items := make([]map[string]any, 0, len(raw))
	for index, item := range raw {
		object, ok := item.(map[string]any)
		if !ok || len(object) == 0 {
			return nil, oaResponseError(operation, "malformed_item", fmt.Sprintf("响应 %s[%d] 不是非空对象", path, index))
		}
		items = append(items, object)
	}
	return items, nil
}

func oaFirstString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func oaScalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		return ""
	}
}

func oaIdentity(object map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := oaScalarString(object[key]); value != "" {
			return value
		}
	}
	return ""
}

func oaRequireIdentity(object map[string]any, operation, expected string, keys ...string) error {
	actual := oaIdentity(object, keys...)
	if actual == "" {
		return oaResponseError(operation, "missing_stable_id", "OA 业务对象缺少稳定 ID")
	}
	if expected != "" && actual != expected {
		return oaResponseError(operation, "identity_mismatch", "OA 业务对象 ID 与请求目标不一致")
	}
	return nil
}

func oaProjectForms(data map[string]any, operation, path string) ([]map[string]any, error) {
	items, err := oaRequireCollection(data, operation, path)
	if err != nil {
		return nil, err
	}
	forms := make([]map[string]any, 0, len(items))
	for index, item := range items {
		code := oaIdentity(item, "processCode")
		name := oaFirstString(item, "processName", "name")
		if code == "" || name == "" {
			return nil, oaResponseError(operation, "malformed_item", fmt.Sprintf("表单结果第 %d 项缺少 processCode 或名称", index))
		}
		row := map[string]any{"processCode": code, "name": name}
		if icon := oaFirstString(item, "processIconUrl", "iconUrl"); icon != "" {
			row["iconUrl"] = icon
		}
		forms = append(forms, row)
	}
	return forms, nil
}

func oaProjectInstances(data map[string]any, operation, path string) ([]map[string]any, error) {
	items, err := oaRequireCollection(data, operation, path)
	if err != nil {
		return nil, err
	}
	instances := make([]map[string]any, 0, len(items))
	for index, item := range items {
		id := oaIdentity(item, "processInstanceId")
		if id == "" {
			return nil, oaResponseError(operation, "missing_item_identity", fmt.Sprintf("审批结果第 %d 项缺少 processInstanceId", index))
		}
		row := map[string]any{"processInstanceId": id}
		if title := oaFirstString(item, "title", "processInstanceTitle"); title != "" {
			row["title"] = title
		}
		if status := oaScalarString(item["status"]); status != "" {
			row["status"] = status
		}
		for _, key := range []string{"processCreateTime", "createTime"} {
			if value, present := item[key]; present && value != nil {
				row["createTime"] = value
				break
			}
		}
		instances = append(instances, row)
	}
	return instances, nil
}

func oaHasMorePage(result map[string]any, operation string, itemCount, currentPage int) (oaPageEvidence, error) {
	raw, present := result["hasMore"]
	if !present {
		// Reviewed OA zero-page encoding: get_done_tasks,
		// get_submitted_instances, and get_noticed_instances omit hasMore only
		// when result.values is an explicit empty array. No other missing
		// pagination shape is accepted.
		if itemCount == 0 {
			return oaPageEvidence{Known: true}, nil
		}
		return oaPageEvidence{}, oaResponseError(operation, "missing_pagination", "非空审批列表缺少 hasMore，无法证明结果完整或提供续页凭据")
	}
	hasMore, ok := raw.(bool)
	if !ok {
		return oaPageEvidence{}, oaResponseError(operation, "malformed_pagination", "OA 响应 hasMore 应为布尔值")
	}
	page := oaPageEvidence{Known: true, HasMore: hasMore}
	if hasMore {
		if currentPage <= 0 {
			return oaPageEvidence{}, oaResponseError(operation, "invalid_page", "当前页码无效，无法生成下一页凭据")
		}
		page.Next = strconv.Itoa(currentPage + 1)
	}
	return page, nil
}

func outputOAPage(rt *shortcut.RuntimeContext, collection string, items []map[string]any, page oaPageEvidence) error {
	if !page.Known {
		return oaResponseError("oa/pagination", "missing_pagination", "响应缺少分页终态证据")
	}
	payload := map[string]any{"count": len(items), collection: items, "complete": !page.HasMore}
	if !output.UsesUnifiedResult(rt.Command()) {
		if page.HasMore {
			payload["nextPage"] = page.Next
		}
		return rt.Output(payload)
	}
	pagination, err := output.NewPagination(!page.HasMore, page.Next)
	if err != nil {
		return oaResponseError("oa/pagination", "invalid_pagination", err.Error())
	}
	meta := &output.Meta{Count: output.NewCount(len(items)), Pagination: pagination}
	return output.StoreResult(rt.Command().Context(), output.Success(payload, output.WithMeta(meta)))
}

func validateOAPage(page, limit int) error {
	if page <= 0 {
		return apperrors.NewValidation("--page 必须大于 0")
	}
	if limit <= 0 || limit > 100 {
		return apperrors.NewValidation("--limit 必须在 1 到 100 之间")
	}
	return nil
}
