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

package helpers

import (
	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/cmdcore"
)

// leaf.go 是叶子命令的统一构建框架（命令框架的 Leaf 门面）。
//
// 声明 vs 执行（框架规则，详见 RFC §5.0 / 同源文档 §1.2）：
//
//   - 声明 = LeafSpec 数据字段：Flags、Constraints、非空 Risk、ConstParams、
//     Use/Short/Long/Example。经 FromLeafSpec → cmdcore.NewCommand 注册并
//     嵌入 dws.schema.*。
//   - 执行 = Validate / Call / RunE / PostMount。钩子消费已装配参数，不得
//     发明 CLI 表面（业务 flag/params）。
//   - 标注 = 声明字段装不下时的显式注解（如 write-guard 的 runtime_gate）。
//
// LeafSpec 把共性收敛为声明式结构：命令声明 flag 集合与绑定，框架统一校验、
// 装配与投影。默认派发走 MCP 直连；非 MCP 命令通过 Call 注入派发器。复杂
// 命令可用 RunE 逃生舱。
//
// 收敛纪律（Phase 2）：flag 注册、有效值回退链、required/约束、Risk 写确认、
// toolArgs 装配、Schema 投影与帮助渲染均在 internal/cmdcore；本文件只做
// LeafSpec→CommandSpec 映射与 dispatch 闭包。等价性由 catalog 漂移门禁与
// leaf/risk/约束单测共同兜底。
//
// 迁移纪律：迁入 LeafSpec 时 flag 名、默认值、usage、MarkFlagRequired、
// required 错误格式、toolArgs 键与值必须逐字保持一致。

// LeafFlagKind 是 flag 的值类型（cmdcore.FlagKind 的别名）。
type LeafFlagKind = cmdcore.FlagKind

const (
	// LeafString 字符串 flag（默认）。
	LeafString = cmdcore.KindString
	// LeafInt 整型 flag；仅在值 != 0 时进入 toolArgs（putInt 语义）。
	LeafInt = cmdcore.KindInt
	// LeafBool 布尔 flag；仅在用户显式提供（Changed）时进入 toolArgs，显式
	// false 也下发；不参与别名/env 回退链。
	LeafBool = cmdcore.KindBool
	// LeafStringSlice 字符串列表 flag；仅在存在非空元素时进入 toolArgs，元素
	// 恒 TrimSpace 后过滤空串。
	LeafStringSlice = cmdcore.KindStringSlice
)

// LeafFlag 声明一个 flag 的注册方式与到 MCP toolArgs 的绑定
// （cmdcore.FlagSpec 的别名，字段含义见 cmdcore 定义）。
type LeafFlag = cmdcore.FlagSpec

// LeafRisk 声明叶子命令的副作用等级（cmdcore.Risk 的别名）。取值与 shortcut
// 框架的 Risk 逐字一致。
type LeafRisk = cmdcore.Risk

const (
	// LeafRiskRead 只读操作，从不提示。空值即视为 LeafRiskRead。
	LeafRiskRead = cmdcore.RiskRead
	// LeafRiskWrite 变更状态；未加 --yes 时提示确认。
	LeafRiskWrite = cmdcore.RiskWrite
	// LeafRiskHighWrite 破坏性/不可逆操作；未加 --yes 时提示确认。
	LeafRiskHighWrite = cmdcore.RiskHighWrite
)

// LeafConstraintKind 是跨 flag 关系约束的类型（cmdcore.ConstraintKind 的
// 别名）。取值与 shortcut 框架的 ConstraintKind 逐字一致。
type LeafConstraintKind = cmdcore.ConstraintKind

const (
	// LeafAtLeastOne 要求 Flags 至少提供一个。
	LeafAtLeastOne = cmdcore.AtLeastOne
	// LeafExactlyOne 要求 Flags 必须且只能提供一个。
	LeafExactlyOne = cmdcore.ExactlyOne
	// LeafMutuallyExclusive 允许 Flags 中最多提供一个。
	LeafMutuallyExclusive = cmdcore.MutuallyExclusive
)

// LeafConstraint 声明一组 flag 的关系约束（cmdcore.Constraint 的别名）。框架
// 在 required 校验之后、Validate 钩子之前统一执行；「是否提供」的判定复用有效
// 值回退链（显式主 flag → 别名 → env），即只传兼容别名同样视为已提供。约束
// 同时投影到 Agent Runtime Schema 并渲染进 --help 的「参数约束」段。
type LeafConstraint = cmdcore.Constraint

// LeafSchema 是 Schema 最终载荷声明（cmdcore.SchemaDecl 别名）。
type LeafSchema = cmdcore.SchemaDecl

// LeafSpec 是命令框架的 Leaf 声明门面（映射为 cmdcore.CommandSpec）。
//
// 声明面 = Schema 最终数据源：Flags（含 parameter Schema 字段）、Constraints、
// Risk、ConstParams、Use/Short/Long/Example、Schema（ToolSpec 各组）。
// Schema 组装透传嵌入值，声明路径不再引入评审并行字段。
//
// 执行面（不算声明）：Validate、Call、RunE、PostMount；Server/Tool 仅路由。
type LeafSpec struct {
	Use     string
	Short   string
	Long    string
	Example string

	// Server 非空时走 callMCPToolOnServer（显式 server 路由），否则走
	// callMCPTool（按 product 路由）。Call 非空时两者都被忽略。不是 CLI 声明。
	Server string
	Tool   string
	Flags  []LeafFlag
	// Constraints 是跨 flag 的关系约束（至少一个 / 恰好一个 / 互斥），由
	// cmdcore 统一校验并投影到 Runtime Schema 与 --help。复杂的条件式校验
	// 仍放 Validate 钩子（钩子本身不是约束声明）。
	Constraints []LeafConstraint

	// Risk 声明副作用等级，驱动 ConfirmRisk（对齐 shortcut）。非空时嵌入
	// dws.schema.risk 并合成 Schema.Safety。
	Risk LeafRisk

	// ConstParams 是与 flag 无关的固定载荷（如 precheckOnly），在 flag 装配
	// 之后并入 toolArgs。载荷声明，不上用户 flag 表；从不满足 Required。
	ConstParams map[string]any

	// Schema 是 ToolSpec 最终载荷（identity/selection/safety/dry_run/…）。
	Schema LeafSchema

	// Call 是执行体：非空时替代默认 MCP 派发。toolArgs 已由 Flags/ConstParams
	// 装配完成；Call 不应再写业务参数。分页等横切由领域工具处理，不进声明。
	Call func(cmd *cobra.Command, tool string, args map[string]any) error

	// Validate 是编排钩子（条件式校验），不是声明面。单 flag 转换用
	// LeafFlag.Transform；可声明的互斥/至少一个应写 Constraints。
	Validate func(cmd *cobra.Command, args []string) error

	// RunE 非空时完全自定义执行体（逃生舱）；表面事实仍须 Flags/Schema 声明。
	RunE func(cmd *cobra.Command, args []string) error

	// PostMount 是挂载收尾钩子（领域工具等），不是声明面。
	// 业务 flag 必须写在 Flags；分页由领域工具注入。
	PostMount func(cmd *cobra.Command)
}

// NewLeafCommand 按 LeafSpec 构建叶子命令：经 FromLeafSpec 归一为
// cmdcore.CommandSpec 后交由统一构建器 cmdcore.NewCommand 编排（flag 注册、
// 约束声明检查、Schema 投影、帮助渲染、required/约束校验、Risk 写确认、
// toolArgs 装配）。本函数只保留 LeafSpec→CommandSpec 的映射与 MCP dispatch
// 闭包（callMCPTool/OnServer/Call）。所有 LeafSpec 命令（含 devapp 全部叶子）
// 由此统一流经 cmdcore 单一 spec 路径。
func NewLeafCommand(spec LeafSpec) *cobra.Command {
	return cmdcore.NewCommand(FromLeafSpec(spec))
}

// FromLeafSpec 把 LeafSpec 归一为统一的 cmdcore.CommandSpec。契约字段
// （Flags/Constraints/Risk）与编排钩子（Validate/PostMount/RunE）直接透传；
// dispatch 收敛为一个闭包：Call 优先，其次显式 Server 路由，最后按 product
// 自动路由。RunE 逃生舱存在时不设 Dispatch（与旧行为一致）。
func FromLeafSpec(spec LeafSpec) cmdcore.CommandSpec {
	cs := cmdcore.CommandSpec{
		Use:         spec.Use,
		Short:       spec.Short,
		Long:        spec.Long,
		Example:     spec.Example,
		Flags:       spec.Flags,
		Constraints: spec.Constraints,
		Risk:        spec.Risk,
		ConstParams: spec.ConstParams,
		Schema:      spec.Schema,
		Validate:    spec.Validate,
		PostMount:   spec.PostMount,
		RunE:        spec.RunE,
	}
	if spec.RunE == nil {
		cs.Invoke = func(c *cmdcore.Ctx, toolArgs map[string]any) error {
			if spec.Call != nil {
				return spec.Call(c.Command(), spec.Tool, toolArgs)
			}
			if spec.Server != "" {
				return callMCPToolOnServer(spec.Server, spec.Tool, toolArgs)
			}
			return callMCPTool(spec.Tool, toolArgs)
		}
	}
	return cs
}
