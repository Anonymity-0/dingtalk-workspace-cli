// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package mail

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

const mailCompositeReason = "Reviewed Mail Shortcut composite: the executable CLI owns strict response validation, pagination evidence, stable-identity checks, output projection, and confirmation; no single MCP interface represents the complete command contract."

func mailCollectionResult(collection, description string) *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: json.RawMessage(fmt.Sprintf(
			`{"type":"object","description":%q,"properties":{"count":{"type":"integer","description":"当前响应中的有效记录数量"},%q:{"type":"array","description":%q,"items":{"type":"object","description":"严格校验后的邮件业务记录","additionalProperties":true}},"complete":{"type":"boolean","description":"服务端分页证据是否证明结果完整"},"nextCursor":{"type":"string","description":"结果未完整时的下一页游标"}},"required":["count",%q,"complete"],"additionalProperties":false}`,
			description, collection, description, collection,
		)),
	}
}

func mailObjectResult(description string) *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: json.RawMessage(fmt.Sprintf(
			`{"type":"object","description":%q,"properties":{"value":{"type":"object","description":"严格校验且身份匹配的邮件业务对象","additionalProperties":true}},"required":["value"],"additionalProperties":false}`,
			description,
		)),
		SensitivePaths: []string{"value.body", "value.markdownBody", "value.subject", "value.from", "value.toRecipients", "value.ccRecipients", "value.bccRecipients"},
	}
}

func mailReadSafety() contract.SafetySpec {
	return contract.SafetySpec{Effect: "read", Risk: "low", Confirmation: "not_required", Idempotency: "idempotent"}
}

func mailWriteSafety(idempotency string) contract.SafetySpec {
	return contract.SafetySpec{Effect: "write", Risk: "medium", Confirmation: "user_required", Idempotency: idempotency}
}

func mailReadContract(command, description, intent string, result *contract.ResultSpec, flags []contract.ParamDecl, examples ...string) corecmd.ContractDecl {
	name := "shortcut_" + strings.ReplaceAll(strings.TrimPrefix(command, "+"), "-", "_")
	cliPath := "mail " + command
	return corecmd.ContractDecl{
		Description: description,
		Result:      result,
		Identity: contract.ToolIdentitySpec{
			ProductID: "mail", Name: name, CanonicalPath: "mail." + name,
			CLIPath: cliPath, PrimaryCLIPath: cliPath,
		},
		Interface: &contract.InterfaceSpec{Mode: contract.InterfaceModeComposite, Availability: contract.InterfaceAvailable, Reason: mailCompositeReason},
		Selection: contract.SelectionSpec{
			AgentSummary: description,
			UseWhen:      []string{intent},
			AvoidWhen:    []string{"只需摘要列表时使用 mail +triage 或 +recent-mail；需要原始响应时改用对应原子命令"},
			Examples:     examples,
		},
		Parameters: flags,
	}
}

func mailWriteContract(command, description, intent string, flags []contract.ParamDecl, examples ...string) corecmd.ContractDecl {
	contractDecl := mailReadContract(command, description, intent, mailObjectResult(description), flags, examples...)
	contractDecl.Selection.AvoidWhen = []string{"用户尚未确认写入内容或目标时不要执行；只需读取时使用对应只读 shortcut"}
	return contractDecl
}

func mailResponseError(operation, reason, message string) error {
	return apperrors.NewAPI(message,
		apperrors.WithOperation(operation),
		apperrors.WithOrigin("mcp"),
		apperrors.WithFailureStage("response_validation"),
		apperrors.WithRetryable(false),
		apperrors.WithReason(reason),
	)
}

// mailRequireSuccess accepts only the two success encodings observed from the
// Mail backend: boolean true and the exact string "true". Missing, false, or
// malformed status values fail closed.
func mailRequireSuccess(data map[string]any, operation string) error {
	if len(data) == 0 {
		return mailResponseError(operation, "empty_tool_response", "服务返回空响应，无法证明调用成功或结果确实为空")
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
		return mailResponseError(operation, "missing_success", "响应缺少 success 业务状态")
	}
	message := mailFirstString(data, "errorMessage", "errorMsg", "message", "error")
	if message == "" {
		message = "服务未明确返回成功状态"
	}
	return mailResponseError(operation, "remote_failure", message)
}

func mailRequireCollection(data map[string]any, operation, path string) ([]map[string]any, error) {
	if err := mailRequireSuccess(data, operation); err != nil {
		return nil, err
	}
	value, present := mailLookup(data, path)
	if !present {
		return nil, mailResponseError(operation, "missing_collection", fmt.Sprintf("成功响应缺少 %s 数组；不能把未知响应结构当作空结果", path))
	}
	raw, ok := value.([]any)
	if !ok {
		return nil, mailResponseError(operation, "malformed_collection", fmt.Sprintf("响应 %s 应为数组，实际为 %T", path, value))
	}
	items := make([]map[string]any, 0, len(raw))
	for index, item := range raw {
		object, ok := item.(map[string]any)
		if !ok || len(object) == 0 {
			return nil, mailResponseError(operation, "malformed_item", fmt.Sprintf("响应 %s[%d] 不是非空对象", path, index))
		}
		items = append(items, object)
	}
	return items, nil
}

func mailRequireObject(data map[string]any, operation, path string) (map[string]any, error) {
	if err := mailRequireSuccess(data, operation); err != nil {
		return nil, err
	}
	value, present := mailLookup(data, path)
	if !present || value == nil {
		return nil, mailResponseError(operation, "missing_result", fmt.Sprintf("成功响应缺少非空 %s 对象", path))
	}
	object, ok := value.(map[string]any)
	if !ok || len(object) == 0 {
		return nil, mailResponseError(operation, "malformed_result", fmt.Sprintf("响应 %s 应为非空对象，实际为 %T", path, value))
	}
	return object, nil
}

func mailRequireIdentity(object map[string]any, operation, expected string, keys ...string) error {
	actual := mailFirstString(object, keys...)
	if actual == "" {
		return mailResponseError(operation, "missing_stable_id", "业务对象缺少稳定 ID")
	}
	if expected != "" && actual != expected {
		return mailResponseError(operation, "identity_mismatch", "业务对象 ID 与请求目标不一致")
	}
	return nil
}

func mailValidateRows(items []map[string]any, operation string, identityKeys ...string) error {
	for index, item := range items {
		if mailFirstString(item, identityKeys...) == "" {
			return mailResponseError(operation, "missing_item_identity", fmt.Sprintf("结果第 %d 项缺少稳定 ID", index))
		}
	}
	return nil
}

func mailProjectCollection(data map[string]any, operation, path string, identityKeys []string, fields map[string][]string) ([]map[string]any, error) {
	items, err := mailRequireCollection(data, operation, path)
	if err != nil {
		return nil, err
	}
	if err := mailValidateRows(items, operation, identityKeys...); err != nil {
		return nil, err
	}
	projected := make([]map[string]any, 0, len(items))
	for _, item := range items {
		row := make(map[string]any, len(fields))
		for outputName, candidates := range fields {
			for _, candidate := range candidates {
				if value, present := item[candidate]; present && value != nil {
					row[outputName] = value
					break
				}
			}
		}
		projected = append(projected, row)
	}
	return projected, nil
}

func mailPage(data map[string]any, operation, prefix string) (bool, string, error) {
	container := data
	if prefix != "" {
		value, present := mailLookup(data, prefix)
		if !present {
			return false, "", mailResponseError(operation, "missing_pagination", "分页响应缺少已审核的分页对象")
		}
		var ok bool
		container, ok = value.(map[string]any)
		if !ok {
			return false, "", mailResponseError(operation, "malformed_pagination", "分页容器不是对象")
		}
	}
	nextValue, nextPresent := container["nextCursor"]
	next, nextOK := nextValue.(string)
	if !nextPresent || !nextOK {
		return false, "", mailResponseError(operation, "missing_pagination", "响应缺少字符串 nextCursor，无法证明结果是否完整")
	}
	if raw, present := container["hasMore"]; present {
		hasMore, ok := mailBool(raw)
		if !ok {
			return false, "", mailResponseError(operation, "malformed_pagination", "hasMore 应为布尔值")
		}
		terminal := strings.TrimSpace(next) == "" || strings.TrimSpace(next) == "$"
		if hasMore && terminal {
			return false, "", mailResponseError(operation, "missing_next_cursor", "hasMore=true 但 nextCursor 为空")
		}
		if !hasMore && !terminal {
			return false, "", mailResponseError(operation, "conflicting_pagination", "hasMore=false 但 nextCursor 非空")
		}
		if !hasMore {
			return true, "", nil
		}
		return false, next, nil
	}
	if strings.TrimSpace(next) == "" || strings.TrimSpace(next) == "$" {
		return true, "", nil
	}
	return false, next, nil
}

func mailBool(value any) (bool, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		if typed == "true" {
			return true, true
		}
		if typed == "false" {
			return false, true
		}
	}
	return false, false
}

func mailLookup(object map[string]any, path string) (any, bool) {
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

func mailFirstString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func mailCollectionPayload(collection string, items []map[string]any, complete bool, next string) map[string]any {
	payload := map[string]any{"count": len(items), collection: items, "complete": complete}
	if !complete {
		payload["nextCursor"] = next
	}
	return payload
}

func hardenPublicMailContracts() {
	collections := []struct {
		declaration *shortcut.Shortcut
		collection  string
		description string
	}{
		{&ThreadList, "threads", "严格校验的邮件会话列表"},
		{&FolderList, "folders", "严格校验的邮箱文件夹列表"},
		{&TagList, "tags", "严格校验的邮件标签列表"},
		{&UserSearch, "users", "严格校验的企业邮箱用户搜索结果"},
		{&TemplateList, "templates", "严格校验的邮件模板列表"},
		{&ContactList, "contacts", "严格校验的邮件联系人列表"},
	}
	for _, item := range collections {
		item.declaration.OutputRollout = output.RolloutUnifiedActive
		item.declaration.Contract.Result = mailCollectionResult(item.collection, item.description)
		item.declaration.Contract.Interface = &contract.InterfaceSpec{Mode: contract.InterfaceModeComposite, Availability: contract.InterfaceAvailable, Reason: mailCompositeReason}
	}
}
