// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutes

import (
	"encoding/json"
	"strings"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

func minutesListResult() *contract.ResultSpec {
	return &contract.ResultSpec{
		Outcomes: []contract.ResultOutcome{
			contract.ResultOutcomeSuccess,
			contract.ResultOutcomePartialFailure,
			contract.ResultOutcomeFailure,
		},
		DataSchema: json.RawMessage(`{"type":"object","description":"带范围与完整性证据的听记列表","properties":{"scope":{"type":"string","description":"本次列表的产品范围"},"count":{"type":"integer","description":"本次返回的去重听记数量"},"scannedCount":{"type":"integer","description":"标题过滤前扫描到的去重听记数量"},"minutes":{"type":"array","description":"稳定投影后的听记条目","items":{"type":"object","description":"包含稳定 taskUuid 的听记条目","additionalProperties":true}},"pages":{"type":"integer","description":"本次实际读取的页数"},"complete":{"type":"boolean","description":"是否已证明目标产品范围完整"},"endpointExhausted":{"type":"boolean","description":"本次调用的服务端分页端点是否耗尽"},"nextToken":{"type":"string","description":"单页预览可继续读取的分页 token"},"nextAction":{"type":"string","description":"当前结果不完整时的安全继续方式"},"scopeLedger":{"type":"array","description":"accessible 聚合时各范围的完整性台账","items":{"type":"object","description":"一个底层范围的分页与结果状态","additionalProperties":true}}},"required":["scope","count","minutes","pages","complete","endpointExhausted"],"additionalProperties":true}`),
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

func withMinutesListResult(decl corecmd.ContractDecl) corecmd.ContractDecl {
	decl.Result = minutesListResult()
	decl.Pagination = minutesCursorPagination()
	return decl
}

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
