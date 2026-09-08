// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutes

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func minutesListResult() *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{
			contract.ResultOutcomeSuccess,
			contract.ResultOutcomeFailure,
		},
		DataSchema: json.RawMessage(`{"type":"object","description":"带范围与完整性证据的听记列表","properties":{"scope":{"type":"string","description":"本次列表的产品范围"},"count":{"type":"integer","description":"本次返回的去重听记数量"},"scannedCount":{"type":"integer","description":"标题过滤前扫描到的去重听记数量"},"minutes":{"type":"array","description":"稳定投影后的听记条目","items":{"type":"object","description":"包含稳定 taskUuid 的听记条目","additionalProperties":true}},"pages":{"type":"integer","description":"本次实际读取的页数"},"complete":{"type":"boolean","description":"是否已证明目标产品范围完整"},"nextAction":{"type":"string","description":"当前结果不完整时的安全继续方式"},"scopeLedger":{"type":"array","description":"accessible 聚合时各范围的完整性台账","items":{"type":"object","description":"一个底层范围的分页与结果状态","additionalProperties":true}}},"required":["scope","count","minutes","pages","complete"],"additionalProperties":true}`),
	}
}

func minutesRecordResult() *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{
			contract.ResultOutcomeSuccess,
			contract.ResultOutcomeFailure,
		},
		DataSchema: json.RawMessage(`{"type":"object","description":"带稳定绑定状态的听记录音控制回执","properties":{"accepted":{"type":"boolean","description":"网关是否明确接受录音控制指令"},"command":{"type":"string","description":"已确认执行的录音控制指令"},"bound":{"type":"boolean","description":"回执是否包含可归属于本次录音的稳定 taskUuid"},"controlReady":{"type":"boolean","description":"是否可以安全执行后续 pause/resume/stop 控制"},"taskUuid":{"type":"string","description":"已由回执确认的听记稳定 taskUuid"},"reason":{"type":"string","description":"已受理但无法安全绑定时的停止原因"},"result":{"type":"object","description":"经校验的网关原始业务回执","additionalProperties":true}},"required":["accepted","command","bound","controlReady","result"],"additionalProperties":false}`),
	}
}

func minutesCursorPagination() *contract.PaginationSpec {
	return &contract.PaginationSpec{
		Kind:                  contract.PaginationKindCursor,
		CursorParameter:       "cursor",
		MetaPath:              contract.PaginationMetaPath,
		EndpointExhaustedPath: contract.PaginationExhaustedPath,
		NextTokenPath:         contract.PaginationNextTokenPath,
	}
}

func minutesContract(command, description, useWhen string, avoidWhen []string, examples []string) corecmd.ContractDecl {
	name := "shortcut_" + strings.ReplaceAll(strings.TrimPrefix(command, "+"), "-", "_")
	return corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "minutes",
			Name:           name,
			CanonicalPath:  "minutes." + name,
			CLIPath:        "minutes " + command,
			PrimaryCLIPath: "minutes " + command,
		},
		Description: description,
		Interface: &contract.InterfaceSpec{
			Mode:         contract.InterfaceModeComposite,
			Availability: contract.InterfaceAvailable,
			Reason:       "The executable Shortcut owns validation, orchestration, completeness and verification across one or more Minutes RPCs; no single RPC represents the final command contract.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: description,
			UseWhen:      []string{useWhen},
			AvoidWhen:    avoidWhen,
			Examples:     examples,
		},
	}
}

func withMinutesDryRun(decl corecmd.ContractDecl, kind string, remoteReads bool) corecmd.ContractDecl {
	decl.DryRun = &contract.DryRunSpec{PreviewKind: kind, RemoteReads: remoteReads}
	return decl
}

func withMinutesExportResult(decl corecmd.ContractDecl) corecmd.ContractDecl {
	decl.Result = &contract.ResultSpec{
		Outcomes:   []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomeFailure},
		DataSchema: json.RawMessage(`{"type":"object","description":"听记文本归档发布与签名清理回执；不证明图片离线可用或文件哈希读回","properties":{"operation":{"type":"string","description":"执行操作"},"taskUuid":{"type":"string","description":"源听记标识"},"complete":{"type":"boolean","description":"所选产物收集和当前发布步骤是否完成"},"published":{"type":"boolean","description":"是否已发布归档目录"},"path":{"type":"string","description":"已发布相对目录"},"manifest":{"type":"string","description":"清理台账相对路径"},"sanitized":{"type":"boolean","description":"文本产物是否已清理并通过发布前凭据扫描"},"sanitizationScope":{"type":"string","description":"扫描范围 text_artifacts，不含二进制媒体内容"},"redactionCount":{"type":"integer","description":"全部文本产物中替换的链接或凭据字段次数"},"redactionKinds":{"type":"array","description":"实际发生的清理类型","items":{"type":"string"}},"offlineImagesComplete":{"type":"boolean","description":"是否证明摘要图片离线完整；当前始终为 false"},"files":{"type":"object","description":"逐文件路径、大小、完整性以及文本清理状态和次数","additionalProperties":true}},"additionalProperties":true}`),
	}
	return decl
}

func withMinutesSpeakerResult(decl corecmd.ContractDecl) corecmd.ContractDecl {
	decl.Result = &contract.ResultSpec{
		Outcomes:   []contract.ResultOutcome{contract.ResultOutcomeSuccess, contract.ResultOutcomePending, contract.ResultOutcomeFailure},
		DataSchema: json.RawMessage(`{"type":"object","description":"发言人总结的有界读取结果，创建回执不等于完成","properties":{"operation":{"type":"string","description":"执行操作"},"state":{"type":"string","enum":["pending","ready","failed","unsupported_shape"],"description":"读取状态；pending 也可能表示结果暂不可读取，不保证任务仍在运行"},"complete":{"type":"boolean","description":"只有已确认终态且存在总结正文时为 true"},"taskUuid":{"type":"string","description":"听记稳定标识"},"taskId":{"type":"string","description":"创建或查询实际返回的恢复句柄，未知时为空"},"createStatus":{"type":"string","description":"创建时状态或 resume，不表示当前终态"},"status":{"type":"string","description":"当前查询明确返回的任务状态"},"attempts":{"type":"integer","description":"实际查询次数"},"stage":{"type":"string","description":"停止所在 create 或 poll 阶段"},"reason":{"type":"string","description":"当前状态的判定原因"},"retryable":{"type":"boolean","description":"是否可有界继续读取；永不授权重新 create"},"result":{"type":"object","description":"仅 ready 时返回校验后的总结正文和终态证据","additionalProperties":true},"recovery":{"type":"object","description":"未完成时保留稳定标识和只读恢复方式","properties":{"taskUuid":{"type":"string","description":"原听记标识"},"taskId":{"type":"string","description":"已知恢复句柄"},"nextCommand":{"type":"array","description":"需沿用同一 profile 并完成确认的 resume argv，不含 --yes","items":{"type":"string"}},"nextAction":{"type":"string","description":"恢复限制与确认提示"}},"additionalProperties":false}},"additionalProperties":true}`),
	}
	return decl
}

func withMinutesListResult(decl corecmd.ContractDecl) corecmd.ContractDecl {
	decl.Result = minutesListResult()
	decl.Pagination = minutesCursorPagination()
	return decl
}

// outputMinutesListResult keeps the business payload and the framework cursor
// metadata separate. Legacy projection is retained for defensive direct use,
// while the five published Minutes pagination routes use the unified result.
func outputMinutesListResult(rt *shortcut.RuntimeContext, payload map[string]any, result minutesListCollection, readErr error) error {
	if !output.UsesUnifiedResult(rt.Command()) {
		if err := rt.Output(payload); err != nil {
			return err
		}
		return readErr
	}

	business := make(map[string]any, len(payload))
	for key, value := range payload {
		if key == "endpointExhausted" || key == "nextToken" {
			continue
		}
		business[key] = value
	}

	meta := &output.Meta{Count: output.NewCount(len(result.Rows))}
	pagination, paginationErr := output.NewPagination(result.EndpointExhausted, result.NextToken)
	_, aggregateResult := payload["scopeLedger"]
	if readErr != nil && aggregateResult {
		// The token belongs to one internal mine/shared leg and cannot be passed
		// safely to the public aggregate command's --cursor flag.
		pagination = nil
		paginationErr = fmt.Errorf("aggregate pagination cannot publish an internal scope cursor")
	}
	if paginationErr == nil {
		pagination.Pages = result.Pages
		pagination.Items = len(result.Rows)
		meta.Pagination = pagination
	}
	if readErr != nil {
		details := map[string]any{
			"pages":     result.Pages,
			"itemCount": len(result.Rows),
			"cause":     readErr.Error(),
		}
		if result.NextToken != "" && !aggregateResult {
			details["nextToken"] = result.NextToken
		}
		options := []output.ResultOption{output.WithMeta(meta)}
		hint := "使用 meta.pagination.next_token 继续读取；若该字段缺失，请从未完成的范围首页重试。"
		if paginationErr != nil {
			meta.Pagination = nil
			hint = "当前聚合范围没有可安全复用的公开 cursor；请重新执行同一条 --page-all 命令。"
		}
		return output.StoreResult(rt.Command().Context(), output.Failure(&output.ErrorInfo{
			Type:             "api",
			Subtype:          "minutes_pagination_incomplete",
			Message:          fmt.Sprintf("听记分页读取不完整：已读取 %d 页、%d 条听记", result.Pages, len(result.Rows)),
			Hint:             hint,
			Operation:        "minutes/list_by_keyword_and_time_range",
			Origin:           "mcp",
			Stage:            "pagination",
			ExecutionStarted: boolPointer(true),
			Details:          details,
			TechnicalDetail:  readErr.Error(),
		}, options...))
	}
	if paginationErr != nil {
		return fmt.Errorf("minutes pagination result is invalid: %w", paginationErr)
	}
	return output.StoreResult(rt.Command().Context(), output.Success(business, output.WithMeta(meta)))
}

func boolPointer(value bool) *bool { return &value }

func withMinutesRecordResult(decl corecmd.ContractDecl) corecmd.ContractDecl {
	decl.Result = minutesRecordResult()
	return decl
}

func minutesDryRunPayload(kind, operation string, payload map[string]any) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}
	payload["operation"] = operation
	payload["dry_run"] = true
	payload["dryRun"] = true
	payload["preview_kind"] = kind
	payload["executed"] = false
	return payload
}

// finalizeMinutesShortcuts keeps the human Shortcut declaration and the final
// Agent contract on one source of truth. Custom validation is prose-only in the
// Schema wire, so publish its exact evidence on every affected flag as well.
func finalizeMinutesShortcuts(values ...shortcut.Shortcut) []shortcut.Shortcut {
	finalized := make([]shortcut.Shortcut, len(values))
	for index, value := range values {
		value.Contract.Selection.AgentSummary = value.Description
		value.Contract.Selection.UseWhen = []string{value.Intent}
		for _, constraint := range value.Constraints {
			if constraint.Kind != shortcut.ConstraintCustom {
				continue
			}
			evidence := strings.TrimSpace(constraint.Description)
			if evidence == "" {
				continue
			}
			for flagIndex := range value.Flags {
				flag := &value.Flags[flagIndex]
				if !containsString(constraint.Flags, flag.Name) || strings.Contains(flag.Desc, evidence) {
					continue
				}
				flag.Desc = strings.TrimRight(flag.Desc, "；。 ") + "；约束：" + evidence
			}
			for parameterIndex := range value.Contract.Parameters {
				parameter := &value.Contract.Parameters[parameterIndex]
				if !containsString(constraint.Flags, parameter.Name) || strings.Contains(parameter.Description, evidence) {
					continue
				}
				parameter.Description = strings.TrimRight(parameter.Description, "；。 ") + "；约束：" + evidence
			}
		}
		finalized[index] = value
	}
	return finalized
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
