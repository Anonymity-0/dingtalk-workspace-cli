# 命令框架架构

本文档描述 `internal/cmdcore` 统一命令框架的当前架构，面向框架使用者和维护者。

## 概览

```
用户输入 → cobra 命令树 → cmdcore.NewCommand() → 运行时管线 → 后端派发
```

命令框架将 CLI 命令的**声明**与**执行**分离：

- **声明面** — 数据字段描述命令是什么（flag、约束、风险等级、Schema 元数据）
- **执行面** — 钩子函数描述命令做什么（校验、派发、编排）

框架负责：flag 注册、有效值回退链、required/约束校验、Risk 写确认、toolArgs 装配、Agent Runtime Schema 投影。

## 核心类型

### CommandSpec

统一的类型化命令规格，是框架的核心数据结构：

```go
type CommandSpec struct {
    // 声明面
    Use         string
    Short       string
    Long        string
    Example     string
    Flags       []FlagSpec
    Constraints []Constraint
    Risk        Risk          // 运行时确认行为
    Safety      Safety        // Schema 元数据等级
    ConfirmFirst bool         // 确认门先于参数校验
    ConstParams map[string]any
    Schema      SchemaDecl    // 完整 ToolSpec 载荷

    // 执行面（恰好一个）
    Invoke      func(c *Ctx, toolArgs map[string]any) error  // 单步
    Orchestrate func(c *Ctx) error                           // 多步
    RunE        func(cmd *cobra.Command, args []string) error // 逃生舱

    // 钩子
    Validate    func(cmd *cobra.Command, args []string) error
    PostMount   func(cmd *cobra.Command)
}
```

### Risk 与 Safety（独立枚举）

两个正交维度，组合使用：

| 枚举 | 职责 | 取值 |
|------|------|------|
| **Risk** | 运行时确认行为（是否弹 yes/no） | `read` / `write` / `high-risk-write` |
| **Safety** | Schema 元数据等级（Agent 看到的影响描述） | `read` / `write` / `high-write` / `destructive` |

Safety 为空时，通过 `Risk.SafetyDefault()` 方法推导：

| Risk | → Safety 默认 | Schema 展开 |
|------|--------------|-------------|
| (空) / `read` | `read` | effect:read, risk:low, confirmation:not_required, idempotency:idempotent |
| `write` | `write` | effect:write, risk:medium, confirmation:user_required, idempotency:unknown |
| `high-risk-write` | `destructive` | effect:destructive, risk:high, confirmation:user_required, idempotency:unknown |

显式设置 Safety 可以覆盖默认推导：

```go
// create：运行时 high-risk 确认，但 Schema 标记为 high-write（不可逆但非破坏性）
Risk:   LeafRiskHighWrite,
Safety: LeafSafetyHighWrite,   // write/high 而非 destructive/high
```

优先级链：`explicit SafetyDecl fields > Safety enum > Risk.SafetyDefault()`

### FlagSpec

声明一个 flag 的注册方式、有效值回退链、到 toolArgs 的绑定：

```go
type FlagSpec struct {
    Name     string       // flag 名（kebab-case）
    Usage    string       // --help 文案
    Kind     FlagKind     // String / Int / Bool / StringSlice
    Default  string       // 注册默认值
    Required bool         // 框架校验非空
    Aliases  []string     // 隐藏别名
    EnvVar   string       // 环境变量回退
    Bind     string       // toolArgs 键名（空则用 Name）
    Transform func(string) (any, error)  // 值转换
    // ...更多字段见源码
}
```

### Constraint

跨 flag 关系约束：

```go
type Constraint struct {
    Kind  ConstraintKind  // at_least_one / exactly_one / mutually_exclusive
    Flags []string
}
```

## 有效值回退链

flag 解析按以下顺序取值（先命中先生效）：

```
显式主 flag (Changed) → 隐藏别名 (Changed) → 环境变量 → 注册默认值
                                                            │
                                              ArgDefault ←──┘ (兜底)
```

- KindBool：仅 Changed 时生效，不参与回退链
- KindStringSlice：仅主 flag / alias Changed 时生效，元素恒 TrimSpace
- KindInt：非零才入 toolArgs（putInt 语义）

## 构建时流程

`cmdcore.NewCommand(spec)` 执行以下构建时检查（失败则 panic）：

1. **validateDispatchDecl** — 恰好一个执行体（Invoke/Orchestrate/RunE）
2. **validateSchemaDecl** — Schema 声明完整性（Description、AgentSummary、UseWhen、AvoidWhen、Examples、Safety、Interface）
3. **RegisterFlags** — flag + alias 注册到 cobra
4. **ValidateConstraintDecls** — 约束引用的 flag 必须存在
5. **embedContractIntoSchema** — 投影到 dws.schema.* annotations
6. **AnnotateConstraints** — 约束渲染到 --help
7. **PostMount** — 调用方的挂载收尾钩子

## 运行时流程

生成的 `RunE` 按以下顺序执行：

```
[ConfirmFirst? → ConfirmRisk]        ← devapp 遗留语义：先确认后校验
  │
  ▼
ValidateRequired                     ← 有效值回退链校验
  │
  ▼
ValidateConstraints                  ← 互斥/至少一个/恰好一个
  │
  ▼
Validate hook                        ← 条件式业务校验（可选）
  │
  ▼
BuildArgs                            ← flag → toolArgs 装配
  │
  ▼
ConstParams 合并
  │
  ▼
[!ConfirmFirst? → ConfirmRisk]       ← 默认顺序：校验后确认
  │
  ▼
Invoke(ctx, toolArgs)                ← 单步派发
  或 Orchestrate(ctx)                ← 多步编排
```

## 消费方式

### LeafSpec（MCP 直连叶子命令）

```go
func newDevAppCreateCommand(runner executor.Runner) *cobra.Command {
    return NewLeafCommand(LeafSpec{
        Use:     "create",
        Short:   "创建开放平台企业内部应用",
        Tool:    devAppCreateTool,
        Risk:    LeafRiskHighWrite,
        Safety:  LeafSafetyHighWrite,
        ConfirmFirst: true,
        Flags: []LeafFlag{
            {Name: "name", Usage: "应用名称 (必填)", Bind: "name",
             Trim: true, Required: true, RequiredHint: "--name 为必填"},
        },
        Schema: LeafSchema{
            Description: "创建开放平台企业内部应用",
            DryRun:      &LeafDryRunDecl{PreviewKind: "invocation"},
            Interface:   &LeafInterfaceDecl{Mode: "composite", Availability: "available"},
            Selection: LeafSelectionDecl{
                AgentSummary: "创建钉钉开放平台应用",
                UseWhen:      []string{"需要新建企业内部应用"},
                AvoidWhen:    []string{"应用已存在时用 update"},
                Examples:     []string{`dws dev app create --name "Bot" --dry-run`},
            },
        },
        Call: devAppCall(runner),
    })
}
```

`NewLeafCommand` 经 `FromLeafSpec()` 归一为 `CommandSpec`，再交 `NewCommand()` 构建。

### Shortcut（智能快捷方式，Phase 3 接入）

```go
// typed seam —— 当前未接入运行时 mount()
spec := FromShortcut(Shortcut{
    Service: "chat",
    Command: "+demo",
    Risk:    RiskHighWrite,
    Flags:   []Flag{...},
    Execute: func(rt *RuntimeContext) error { ... },
})
```

## 文件结构

| 文件 | 职责 |
|------|------|
| `internal/cmdcore/cmdcore.go` | 核心类型 + NewCommand 构建器 + 运行时管线 |
| `internal/cmdcore/schema_decl.go` | SchemaDecl 载荷类型 + 声明完整性守卫 |
| `internal/helpers/leaf.go` | LeafSpec 门面 + type alias + FromLeafSpec 映射 |
| `internal/shortcut/adapter.go` | FromShortcut 映射 (typed seam) |

## Schema 投影

声明即 review：代码中的 Schema 声明经过 code review 后直接投影为：

- **Agent Runtime Schema** (dws.schema.* cobra annotations)
- **Agent Metadata** (agent-metadata JSON, 消费方为 Agent 选择器)
- **Catalog** (catalog.json, 命令注册表)
- **Dry-run Capabilities** (声明自动索引为 reviewed 能力)

不再需要外部 hint 文件维护 selection/metadata/dry-run 信息。

## 设计原则

1. **声明 vs 执行分离** — Flags/Constraints/Risk/Safety/Schema 是声明；Invoke/Validate/PostMount 是执行
2. **单一数据源** — 一份声明驱动 --help、Schema、catalog、runtime 校验
3. **框架推导 > 手写** — Safety 从 Risk 推导、Example 从 Schema.Selection.Examples 继承
4. **构建时拦截 > 运行时报错** — 声明不完整在命令注册时 panic，不等到用户触发
5. **向后兼容** — type alias 让现有 LeafSpec 调用方零改动
