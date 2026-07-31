# 命令框架架构

本文档描述 `internal/corecmd` 统一命令框架的当前架构，面向框架使用者和维护者。

## 概览

```
用户输入 → cobra 命令树 → corecmd.New() → 运行时管线 → 后端派发
```

命令框架将 CLI 命令的**声明**与**执行**分离：

- **声明面** — 数据字段描述命令是什么（flag、约束、SafetySpec、Schema 元数据）
- **执行面** — 钩子函数描述命令做什么（校验、派发、编排）

框架负责：flag 注册、有效值回退链、required/约束校验、SafetySpec 确认、toolArgs 装配、Agent Runtime Schema 投影。

## 核心类型

### corecmd.Spec

统一的类型化命令规格，是框架的核心数据结构：

```go
type Spec struct {
    // 声明面
    Use         string
    Short       string
    Long        string
    Example     string
    Flags       []FlagSpec
    Constraints []Constraint
    Safety      cli.SafetySpec // 运行时与 Schema 的单一安全来源
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

### SafetySpec（单一安全来源）

`Spec.Safety` 直接使用现有 Agent Runtime Schema 的 `cli.SafetySpec`：

| 字段 | 职责 |
|------|------|
| `Effect` | 操作影响：read / write / destructive |
| `Risk` | 风险等级：low / medium / high |
| `Confirmation` | 是否需要用户确认：not_required / user_required |
| `Idempotency` | 幂等性：idempotent / retryable / non_idempotent / unknown |

四个字段彼此独立。框架只读取 `Confirmation` 决定运行时确认，其余字段原样发布到 Schema，不从一个字段机械推导另一个。非空 SafetySpec 必须一次声明完整：

```go
Safety: cli.SafetySpec{
    Effect:       "write",
    Risk:         "high",
    Confirmation: "user_required",
    Idempotency:  "unknown",
},
```

完全空值保留历史只读默认 `read/low/not_required/idempotent`；不存在 Risk/Safety 枚举或覆盖优先级链。

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

`corecmd.New(spec)` 执行以下构建时检查（失败则 panic）：

1. **validateDispatchDecl** — 恰好一个执行体（Invoke/Orchestrate/RunE）
2. **validateSafetySpec** — 非空 SafetySpec 的四个独立字段必须完整
3. **validateSchemaDecl** — Schema 声明完整性（Description、AgentSummary、UseWhen、AvoidWhen、Examples、Interface）
4. **RegisterFlags** — flag + alias 注册到 cobra
5. **ValidateConstraintDecls** — 约束引用的 flag 必须存在
6. **embedContractIntoSchema** — 投影到 dws.schema.* annotations
7. **AnnotateConstraints** — 约束渲染到 --help
8. **PostMount** — 调用方的挂载收尾钩子

## 运行时流程

生成的 `RunE` 按以下顺序执行：

```
[ConfirmFirst? → ConfirmSafety]      ← 可选：先确认后校验
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
[!ConfirmFirst? → ConfirmSafety]     ← 默认顺序：校验后确认
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
        Safety: cli.SafetySpec{
            Effect: "write", Risk: "high",
            Confirmation: "user_required", Idempotency: "unknown",
        },
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

`NewLeafCommand` 经 `FromLeafSpec()` 归一为 `corecmd.Spec`，再交 `corecmd.New()` 构建。这是**完全托管模式**：声明 + 执行都归 command。

### 声明元数据模式（既有命令补 Schema）

执行体必须冻结时，用同一套 `LeafSpec` 词汇只声明元数据，写在命令字面量旁：

```go
baseListCmd := &cobra.Command{
    Use: "list", Short: "获取 AI 表格列表",
    RunE: func(cmd *cobra.Command, args []string) error { /* 原执行体不动 */ },
}
DeclareLeafMetadata(baseListCmd, LeafSpec{
    Safety: aitableSafetyRead(),
    Schema: LeafSchema{
        Description: "列出最近访问的 AI 表格 Base。",
        Interface:   aitableMCPInterface("list_bases"),
        Selection: LeafSelectionDecl{
            AgentSummary: "列出最近访问的 AI 表格 Base。",
            UseWhen:      []string{"只需浏览最近打开过的 Base 时"},
            AvoidWhen:    []string{"按名称查找优先 base search"},
            Examples:     []string{"dws aitable base list"},
        },
    },
})
```

`DeclareLeafMetadata` 调用 `corecmd.AttachSchema` 挂 Safety+Schema；不注册 flag、不接管参数投影。当 `Safety.Confirmation=user_required` 时，用**同一份** SafetySpec 包一层 `ConfirmSafety`，保证执行门禁与 Catalog 同源。迁移态入口；新命令仍应走 `NewLeafCommand`。

### Shortcut（智能快捷方式，已接入 live mount）

```go
func mount(s Shortcut) *cobra.Command {
    return corecmd.New(FromShortcut(s))
}

spec := FromShortcut(Shortcut{
    Service: "chat",
    Command: "+demo",
    Risk:    RiskHighWrite,
    Flags:   []Flag{...},
    Execute: func(rt *RuntimeContext) error { ... },
})
```

Shortcut 当前仍保留自身的 `Risk`，adapter 只在边界将它展开成完整
`cli.SafetySpec`；command/Leaf 不再保留该枚举。Shortcut 的 Cobra
type/default/usage provenance 保持不变，command 统一补充 Required、Enum 和关系约束投影。

## 文件结构

| 文件 | 职责 |
|------|------|
| `internal/corecmd/command.go` | 核心类型 + NewCommand 构建器 + 运行时管线 |
| `internal/corecmd/schema_decl.go` | SchemaDecl 载荷类型 + 声明完整性守卫 |
| `internal/helpers/leaf.go` | LeafSpec 门面：`NewLeafCommand`（完全托管）+ `DeclareLeafMetadata`（声明元数据） |
| `internal/shortcut/adapter.go` | FromShortcut 完整映射与 Risk 兼容边界 |
| `internal/shortcut/runner.go` | RuntimeContext；live mount 委托 command |

## Schema 投影

声明即 review：代码中的 Schema 声明经过 code review 后直接投影为：

- **Agent Runtime Schema** (dws.schema.* cobra annotations)
- **Agent Metadata** (agent-metadata JSON, 消费方为 Agent 选择器)
- **Catalog** (catalog.json, 命令注册表)
- **Dry-run Capabilities** (声明自动索引为 reviewed 能力)

不再需要外部 hint 文件维护 selection/metadata/dry-run 信息。

## 设计原则

1. **声明 vs 执行分离** — Flags/Constraints/Safety/Schema 是声明；Invoke/Validate/PostMount 是执行
2. **单一数据源** — 一份声明驱动 --help、Schema、catalog、runtime 校验
3. **安全字段不互推** — Confirmation 单独驱动确认，Effect/Risk/Idempotency 原样发布
4. **构建时拦截 > 运行时报错** — 声明不完整在命令注册时 panic，不等到用户触发
5. **边界兼容** — Shortcut 暂由 adapter 转换，Leaf 直接声明 SafetySpec
