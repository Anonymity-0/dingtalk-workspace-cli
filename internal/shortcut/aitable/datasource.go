// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// This file declares the 5 datasource shortcuts for the aitable service:
// create / update / sync / sync-status / get-config. Each maps 1:1 onto a
// datasource MCP tool on the "aitable" server. datasource-type is passed
// through without CLI-side enum validation; source-config is validated as a
// JSON object and passed through as a parsed value.

package aitable

import (
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contract"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
)

// ─────────────────────────────────────────────────────────────
// datasource: 数据源同步管理（server: aitable）
// ─────────────────────────────────────────────────────────────

// DatasourceCreate 为指定 AI 表格创建数据源同步配置（create_datasource）。
var DatasourceCreate = shortcut.Shortcut{
	Service:     "aitable",
	Command:     "+datasource-create",
	Product:     serverMain,
	Description: "为指定 AI 表格创建数据源同步配置，创建一张数据源表并触发首次全量同步。返回新建数据源表 ID 和同步任务 ID。",
	Intent:      "当用户需要将外部数据源（如审批数据）接入 AI 表格时使用。创建后返回的表 ID 可用于后续同步、更新配置或查询状态。",
	Risk:        shortcut.RiskWrite,
	Safety: contract.SafetySpec{
		Effect: "write", Risk: "medium",
		Confirmation: "not_required", Idempotency: "not_idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "aitable",
			Name:           "shortcut_datasource_create",
			CanonicalPath:  "aitable.shortcut_datasource_create",
			CLIPath:        "aitable +datasource-create",
			PrimaryCLIPath: "aitable +datasource-create",
		},
		Description: "为指定 AI 表格创建数据源同步配置，创建一张数据源表并触发首次全量同步。返回新建数据源表 ID 和同步任务 ID。",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "为指定 AI 表格创建数据源同步配置，创建一张数据源表并触发首次全量同步。返回新建数据源表 ID 和同步任务 ID。",
			UseWhen:      []string{"当用户需要将外部数据源接入 AI 表格、创建新的数据源表时"},
			AvoidWhen: []string{
				"目标 Base 已有数据源表且仅需更新配置时（改用 +datasource-update）",
				"仅需触发已有数据源表的同步时（改用 +datasource-sync）",
			},
			Examples: []string{
				`dws aitable +datasource-create --base-id BASE123 --datasource-type OA --source-config '{"processCode":"PROC-XXXX"}'`,
				`dws aitable +datasource-create --base-id BASE123 --datasource-type OA --source-config '{"processCode":"PROC-XXXX"}' --auto`,
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "base-id", Type: shortcut.FlagString, Desc: "目标 Base ID（通过 +base-list / +base-search 获取）", Required: true},
		{Name: "datasource-type", Type: shortcut.FlagString, Desc: "数据源类型，目前支持审批（OA）", Required: true},
		{Name: "source-config", Type: shortcut.FlagString, Desc: "源配置 JSON 字符串，由用户提供", Required: true},
		{Name: "auto", Type: shortcut.FlagBool, Desc: "是否开启自动同步，默认 false"},
		{Name: "field-ids", Type: shortcut.FlagStringSlice, Desc: "需要同步的字段 ID 列表，不传时同步全部字段"},
		{Name: "conflict-strategy", Type: shortcut.FlagInt, Desc: "冲突策略：0=自动重命名（默认），1=返回已有配置"},
	},
	Tips: []string{
		`dws aitable +datasource-create --base-id BASE123 --datasource-type OA --source-config '{"processCode":"PROC-XXXX"}'`,
		`dws aitable +datasource-create --base-id BASE123 --datasource-type OA --source-config '{"processCode":"PROC-XXXX"}' --auto`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		sourceConfig, err := parseJSONObject("source-config", rt.Str("source-config"))
		if err != nil {
			return err
		}
		params := map[string]any{
			"baseId":         rt.Str("base-id"),
			"datasourceType": rt.Str("datasource-type"),
			"sourceConfig":   sourceConfig,
		}
		if rt.Changed("auto") {
			params["auto"] = rt.Bool("auto")
		}
		if rt.Changed("field-ids") {
			params["fieldIds"] = rt.StrSlice("field-ids")
		}
		if rt.Changed("conflict-strategy") {
			params["syncConflictStrategy"] = rt.Int("conflict-strategy")
		}
		data, err := rt.CallMCPData(serverMain, "create_datasource", params)
		if err != nil {
			return err
		}
		return rt.Output(data)
	},
}

// DatasourceUpdate 更新已有数据源表的同步配置（update_datasource_config）。
var DatasourceUpdate = shortcut.Shortcut{
	Service:     "aitable",
	Command:     "+datasource-update",
	Product:     serverMain,
	Description: "更新指定 AI 表格中已有数据源表的同步配置，支持更新源配置、自动同步开关和同步字段选择。更新后触发一次同步。仅适用于数据源表。",
	Intent:      "当用户需要修改已有数据源表的配置（如更换审批模板、调整同步字段、开关自动同步）时使用。",
	Risk:        shortcut.RiskWrite,
	Safety: contract.SafetySpec{
		Effect: "write", Risk: "medium",
		Confirmation: "not_required", Idempotency: "not_idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "aitable",
			Name:           "shortcut_datasource_update",
			CanonicalPath:  "aitable.shortcut_datasource_update",
			CLIPath:        "aitable +datasource-update",
			PrimaryCLIPath: "aitable +datasource-update",
		},
		Description: "更新指定 AI 表格中已有数据源表的同步配置，支持更新源配置、自动同步开关和同步字段选择。更新后触发一次同步。仅适用于数据源表。",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "更新指定 AI 表格中已有数据源表的同步配置，支持更新源配置、自动同步开关和同步字段选择。更新后触发一次同步。仅适用于数据源表。",
			UseWhen:      []string{"当用户需要修改已有数据源表的同步配置时"},
			AvoidWhen: []string{
				"需要创建新数据源表时（改用 +datasource-create）",
				"仅需触发同步不改配置时（改用 +datasource-sync）",
			},
			Examples: []string{
				`dws aitable +datasource-update --base-id BASE123 --table-id TBL456 --auto`,
				`dws aitable +datasource-update --base-id BASE123 --table-id TBL456 --source-config '{"processCode":"PROC-YYYY"}'`,
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "base-id", Type: shortcut.FlagString, Desc: "目标 Base ID", Required: true},
		{Name: "table-id", Type: shortcut.FlagString, Desc: "已存在的数据源表 ID（由 +datasource-create 返回）", Required: true},
		{Name: "source-config", Type: shortcut.FlagString, Desc: "新的源配置 JSON，不传时保持原有配置"},
		{Name: "auto", Type: shortcut.FlagBool, Desc: "是否开启自动同步，不传时保持原有设置"},
		{Name: "field-ids", Type: shortcut.FlagStringSlice, Desc: "需要同步的字段 ID 列表，不传时同步全部字段"},
	},
	Tips: []string{
		`dws aitable +datasource-update --base-id BASE123 --table-id TBL456 --auto`,
		`dws aitable +datasource-update --base-id BASE123 --table-id TBL456 --source-config '{"processCode":"PROC-YYYY"}'`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"baseId":  rt.Str("base-id"),
			"tableId": rt.Str("table-id"),
		}
		if rt.Changed("source-config") {
			sourceConfig, err := parseJSONObject("source-config", rt.Str("source-config"))
			if err != nil {
				return err
			}
			params["sourceConfig"] = sourceConfig
		}
		if rt.Changed("auto") {
			params["auto"] = rt.Bool("auto")
		}
		if rt.Changed("field-ids") {
			params["fieldIds"] = rt.StrSlice("field-ids")
		}
		data, err := rt.CallMCPData(serverMain, "update_datasource_config", params)
		if err != nil {
			return err
		}
		return rt.Output(data)
	},
}

// DatasourceSync 对数据源表触发一次手动同步（run_datasource_sync）。
var DatasourceSync = shortcut.Shortcut{
	Service:     "aitable",
	Command:     "+datasource-sync",
	Product:     serverMain,
	Description: "对指定 AI 表格中的数据源表触发一次手动同步。单次最多 5 张表，每张表独立提交，部分失败不影响其他表。",
	Intent:      "当用户需要手动触发已有数据源表的同步（而非创建或更新配置）时使用。同步任务 ID 可通过 +datasource-sync-status 查询结果。",
	Risk:        shortcut.RiskWrite,
	Safety: contract.SafetySpec{
		Effect: "write", Risk: "medium",
		Confirmation: "not_required", Idempotency: "not_idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "aitable",
			Name:           "shortcut_datasource_sync",
			CanonicalPath:  "aitable.shortcut_datasource_sync",
			CLIPath:        "aitable +datasource-sync",
			PrimaryCLIPath: "aitable +datasource-sync",
		},
		Description: "对指定 AI 表格中的数据源表触发一次手动同步。单次最多 5 张表，每张表独立提交，部分失败不影响其他表。",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "对指定 AI 表格中的数据源表触发一次手动同步。单次最多 5 张表，每张表独立提交，部分失败不影响其他表。",
			UseWhen:      []string{"当用户需要手动触发已有数据源表的数据同步时"},
			AvoidWhen: []string{
				"需要创建新数据源表时（改用 +datasource-create）",
				"需要更新配置时（改用 +datasource-update）",
			},
			Examples: []string{
				`dws aitable +datasource-sync --base-id BASE123 --table-ids TBL1,TBL2`,
				`dws aitable +datasource-sync --base-id BASE123 --table-ids TBL1`,
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "base-id", Type: shortcut.FlagString, Desc: "目标 Base ID", Required: true},
		{Name: "table-ids", Type: shortcut.FlagStringSlice, Desc: "待触发同步的数据源表 ID 列表（1-5 个）", Required: true},
	},
	Tips: []string{
		`dws aitable +datasource-sync --base-id BASE123 --table-ids TBL1,TBL2`,
		`dws aitable +datasource-sync --base-id BASE123 --table-ids TBL1`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"baseId":   rt.Str("base-id"),
			"tableIds": rt.StrSlice("table-ids"),
		}
		data, err := rt.CallMCPData(serverMain, "run_datasource_sync", params)
		if err != nil {
			return err
		}
		return rt.Output(data)
	},
}

// DatasourceSyncStatus 查询数据源表同步任务状态（get_datasource_sync_status）。
var DatasourceSyncStatus = shortcut.Shortcut{
	Service:     "aitable",
	Command:     "+datasource-sync-status",
	Product:     serverMain,
	Description: "查询指定数据源表的同步任务状态。与 +datasource-sync / +datasource-create / +datasource-update 配对使用：这些指令触发同步后返回任务 ID，本指令通过任务 ID 查询最终结果。",
	Intent:      "当用户触发同步后需要查询同步是否完成、成功或失败时使用。支持批量查询（单次最多 5 个任务 ID），不传任务 ID 时查询最近一次同步状态。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "aitable",
			Name:           "shortcut_datasource_sync_status",
			CanonicalPath:  "aitable.shortcut_datasource_sync_status",
			CLIPath:        "aitable +datasource-sync-status",
			PrimaryCLIPath: "aitable +datasource-sync-status",
		},
		Description: "查询指定数据源表的同步任务状态。与 +datasource-sync / +datasource-create / +datasource-update 配对使用：这些指令触发同步后返回任务 ID，本指令通过任务 ID 查询最终结果。",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "查询指定数据源表的同步任务状态。与 +datasource-sync / +datasource-create / +datasource-update 配对使用：这些指令触发同步后返回任务 ID，本指令通过任务 ID 查询最终结果。",
			UseWhen:      []string{"当用户触发同步后需要查询同步任务状态时"},
			AvoidWhen: []string{
				"需要触发同步时（改用 +datasource-sync）",
			},
			Examples: []string{
				`dws aitable +datasource-sync-status --base-id BASE123 --table-id TBL456 --task-ids TASK1,TASK2`,
				`dws aitable +datasource-sync-status --base-id BASE123 --table-id TBL456`,
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "base-id", Type: shortcut.FlagString, Desc: "目标 Base ID", Required: true},
		{Name: "table-id", Type: shortcut.FlagString, Desc: "数据源表 ID", Required: true},
		{Name: "task-ids", Type: shortcut.FlagStringSlice, Desc: "待查询的同步任务 ID 列表（1-5 个），不传时查询最近一次"},
	},
	Tips: []string{
		`dws aitable +datasource-sync-status --base-id BASE123 --table-id TBL456 --task-ids TASK1,TASK2`,
		`dws aitable +datasource-sync-status --base-id BASE123 --table-id TBL456`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"baseId":  rt.Str("base-id"),
			"tableId": rt.Str("table-id"),
		}
		if rt.Changed("task-ids") {
			params["taskIds"] = rt.StrSlice("task-ids")
		}
		data, err := rt.CallMCPData(serverMain, "get_datasource_sync_status", params)
		if err != nil {
			return err
		}
		return rt.Output(data)
	},
}

// DatasourceGetConfig 获取数据源表的同步配置信息（get_datasource_config）。
var DatasourceGetConfig = shortcut.Shortcut{
	Service:     "aitable",
	Command:     "+datasource-get-config",
	Product:     serverMain,
	Description: "获取指定数据源表的同步配置信息，包括源配置、同步模式、自动同步开关和同步状态。仅适用于数据源表。",
	Intent:      "当用户需要查看已有数据源表的配置详情（如确认当前同步的审批模板、字段选择、自动同步状态）时使用。",
	Risk:        shortcut.RiskRead,
	Safety: contract.SafetySpec{
		Effect: "read", Risk: "low",
		Confirmation: "not_required", Idempotency: "idempotent",
	},
	Contract: corecmd.ContractDecl{
		Identity: contract.ToolIdentitySpec{
			ProductID:      "aitable",
			Name:           "shortcut_datasource_get_config",
			CanonicalPath:  "aitable.shortcut_datasource_get_config",
			CLIPath:        "aitable +datasource-get-config",
			PrimaryCLIPath: "aitable +datasource-get-config",
		},
		Description: "获取指定数据源表的同步配置信息，包括源配置、同步模式、自动同步开关和同步状态。仅适用于数据源表。",
		Interface: &contract.InterfaceSpec{
			Mode:         "composite",
			Availability: "available",
			Reason:       "Reviewed built-in shortcut adapter: the executable CLI owns validation, optional multi-step orchestration, output projection, and confirmation; the complete command contract is not represented by one pinned MCP interface_ref.",
		},
		Selection: contract.SelectionSpec{
			AgentSummary: "获取指定数据源表的同步配置信息，包括源配置、同步模式、自动同步开关和同步状态。仅适用于数据源表。",
			UseWhen:      []string{"当用户需要查看已有数据源表的同步配置详情时"},
			AvoidWhen: []string{
				"需要更新配置时（改用 +datasource-update）",
				"需要查询同步任务状态时（改用 +datasource-sync-status）",
			},
			Examples: []string{
				`dws aitable +datasource-get-config --base-id BASE123 --table-id TBL456`,
			},
		},
	},
	Flags: []shortcut.Flag{
		{Name: "base-id", Type: shortcut.FlagString, Desc: "目标 Base ID", Required: true},
		{Name: "table-id", Type: shortcut.FlagString, Desc: "数据源表 ID", Required: true},
	},
	Tips: []string{
		`dws aitable +datasource-get-config --base-id BASE123 --table-id TBL456`,
	},
	Execute: func(rt *shortcut.RuntimeContext) error {
		params := map[string]any{
			"baseId":  rt.Str("base-id"),
			"tableId": rt.Str("table-id"),
		}
		data, err := rt.CallMCPData(serverMain, "get_datasource_config", params)
		if err != nil {
			return err
		}
		return rt.Output(data)
	},
}

func init() {
	shortcut.Register(withReviewedAITableShortcutContracts(
		DatasourceCreate,
		DatasourceUpdate,
		DatasourceSync,
		DatasourceSyncStatus,
		DatasourceGetConfig,
	)...)
}
