# cmdcore 领域模型

本文档描述 `internal/cmdcore` 包的领域模型——类型、概念及其关系。

## 核心模型图

```
┌─────────────────────────────────────────────────────────────────────┐
│                         CommandSpec                                  │
│                    (一个叶子命令的完整契约)                             │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─── CLI 表面 ───┐   ┌─── 参数声明 ───────────────────────────┐   │
│  │ Use            │   │ FlagSpec[]                              │   │
│  │ Short          │   │  ├─ Name / Kind / Default              │   │
│  │ Long           │   │  ├─ Required / MarkRequired            │   │
│  │ Example        │   │  ├─ Aliases[] / EnvVar (回退链)         │   │
│  └────────────────┘   │  ├─ Bind / Transform / OmitEmpty       │   │
│                       │  └─ Enum / Format / SchemaDescription  │   │
│                       │                                        │   │
│                       │ Constraint[]                            │   │
│                       │  ├─ at_least_one                       │   │
│                       │  ├─ exactly_one                        │   │
│                       │  └─ mutually_exclusive                 │   │
│                       │                                        │   │
│                       │ ConstParams map[string]any             │   │
│                       └────────────────────────────────────────┘   │
│                                                                     │
│  ┌─── 安全模型 ──────────────────────────────────────────────────┐  │
│  │                                                               │  │
│  │  Risk (运行时)          Safety (Schema)                       │  │
│  │  ┌──────────────┐      ┌───────────────┐                    │  │
│  │  │ read         │─────▶│ read          │ low/idempotent     │  │
│  │  │ write        │─────▶│ write         │ medium/unknown     │  │
│  │  │ high-risk-   │─────▶│ destructive   │ high/unknown       │  │
│  │  │   write      │      │               │                    │  │
│  │  └──────────────┘      │ high-write    │ high/unknown       │  │
│  │        │               └───────────────┘                    │  │
│  │        │ SafetyDefault()    ▲                               │  │
│  │        └────────────────────┘ (Safety 为空时的推导)           │  │
│  │                                                               │  │
│  │  ConfirmFirst: bool (确认门顺序)                              │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─── Schema 声明 (Agent 可见的元数据) ──────────────────────────┐  │
│  │  SchemaDecl                                                   │  │
│  │  ├─ Title / Description                                      │  │
│  │  ├─ SafetyDecl     {Effect, Risk, Confirmation, Idempotency} │  │
│  │  ├─ DryRunDecl     {PreviewKind, RemoteReads}                │  │
│  │  ├─ InterfaceDecl  {Mode, Availability, Reason, Ref}         │  │
│  │  ├─ SelectionDecl  {AgentSummary, UseWhen, AvoidWhen,        │  │
│  │  │                  Prerequisites, Tips, Examples}            │  │
│  │  ├─ IdentityDecl   {ProductID, CanonicalPath, Aliases, ...}  │  │
│  │  └─ Positionals[]  {Name, Type, Required, Variadic}          │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─── 执行体 (恰好一个) ─────────────────────────────────────────┐  │
│  │  Invoke(Ctx, toolArgs)     ← 单步：框架装配好 args 后派发     │  │
│  │  Orchestrate(Ctx)          ← 多步：自行组装多次调用           │  │
│  │  RunE(cmd, args)           ← 逃生舱：完全自定义               │  │
│  └───────────────────────────────────────────────────────────────┘  │
│                                                                     │
│  ┌─── 钩子 ─────────────────────────────────────────────────────┐  │
│  │  Validate(cmd, args)   ← 条件式业务校验（约束表达不了的）      │  │
│  │  PostMount(cmd)        ← 挂载收尾（设置 Args 等 cobra 属性）   │  │
│  └───────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
```

## 构建与执行流

```
CommandSpec ──── NewCommand() ────▶ cobra.Command ──── 用户执行 ────▶ Ctx
                     │                    │                            │
              构建时检查:              注册产物:                    执行上下文:
              • validateDispatchDecl   • Flags + Aliases            • Str(name)
              • validateSchemaDecl     • Annotations (Schema)       • Int(name)
              • RegisterFlags          • Long (约束 help)           • Bool(name)
              • ValidateConstraintDecls • RunE (管线)               • StrSlice(name)
              • embedContractIntoSchema                            • Changed(name)
              • AnnotateConstraints                                • DryRun() / Yes()
              • PostMount
```

## 领域概念

| 概念 | 类型 | 职责 |
|------|------|------|
| **CommandSpec** | struct | 一个命令的完整契约（声明 + 执行） |
| **FlagSpec** | struct | 一个参数的注册、回退链、绑定规则 |
| **Constraint** | struct | 参数间的关系约束 |
| **Risk** | enum | 运行时确认行为（是否提示用户） |
| **Safety** | enum | Schema 安全元数据等级（Agent 决策依据） |
| **SchemaDecl** | struct | Agent 可见的完整工具规格声明 |
| **SelectionDecl** | struct | Agent 选择该工具的语义指引 |
| **InterfaceDecl** | struct | 工具的接口模式与可用性 |
| **DryRunDecl** | struct | dry-run 能力声明 |
| **SafetyDecl** | struct | Safety 枚举展开后的细粒度覆盖 |
| **IdentityDecl** | struct | 工具在注册表中的身份标识 |
| **PositionalDecl** | struct | 有序位置参数声明 |
| **Ctx** | struct | 执行上下文（类型安全的 flag 读取） |
| **NewCommand** | func | 统一构建器（声明 → cobra.Command） |

## Risk 枚举

控制**运行时确认行为**——是否在派发前提示用户确认。

| 值 | 含义 | 运行时行为 |
|----|------|-----------|
| `""` / `read` | 只读操作 | 不提示 |
| `write` | 变更状态 | 未加 `--yes` 时提示确认 |
| `high-risk-write` | 破坏性/不可逆 | 未加 `--yes` 时提示确认 |

## Safety 枚举

控制 **Schema 元数据等级**——告诉 Agent 操作对系统的影响程度。

| 值 | effect | risk | confirmation | idempotency |
|----|--------|------|--------------|-------------|
| `read` | read | low | not_required | idempotent |
| `write` | write | medium | user_required | unknown |
| `high-write` | write | high | user_required | unknown |
| `destructive` | destructive | high | user_required | unknown |

## Safety 优先级链

三层解析，先命中先生效：

```
SafetyDecl 显式字段  >  Safety 枚举  >  Risk.SafetyDefault()
   (最高: 精确覆盖)     (中: 等级快捷)    (最低: 默认推导)
```

- **SafetyDecl 显式字段**：`SchemaDecl.Safety` 中非空字段直接使用
- **Safety 枚举**：`CommandSpec.Safety` 非空时按表展开填充空字段
- **Risk.SafetyDefault()**：Safety 为空时从 Risk 推导

推导映射（`Risk.SafetyDefault()`）：

```
read            → SafetyRead
write           → SafetyWrite
high-risk-write → SafetyDestructive
```

## FlagSpec 有效值回退链

框架统一的 flag 值解析顺序：

```
显式主 flag (Changed)
    │ 空?
    ▼
隐藏别名 (Changed, 按声明序)
    │ 空?
    ▼
环境变量 (EnvVar)
    │ 空?
    ▼
注册默认值 (Default)
    │ 空?
    ▼
ArgDefault (兜底)
```

各 Kind 的特殊行为：

| Kind | 入参条件 | 回退链 |
|------|----------|--------|
| KindString | 有效值非空（或 !OmitEmpty） | 完整参与 |
| KindInt | 值 ≠ 0（putInt 语义） | 完整参与 |
| KindBool | Changed 时入参（显式 false 也下发） | 不参与别名/env 回退 |
| KindStringSlice | 存在非空元素 | 仅 Changed 的主 flag/alias |

## Constraint 约束

声明式跨 flag 关系，构建时校验合法性，运行时统一执行：

| Kind | 语义 | 错误文案示例 |
|------|------|-------------|
| `at_least_one` | 至少提供一个 | "请至少指定 --a、--b 之一" |
| `exactly_one` | 恰好提供一个 | "请指定 --a、--b 之一" / "只能指定其一" |
| `mutually_exclusive` | 最多提供一个 | "参数 --a、--b 互斥，只能指定其一" |

"是否提供"的判定复用有效值回退链（显式主 flag → 别名 → env），注册默认值不算作已提供。

## SchemaDecl 子结构

### SelectionDecl（Agent 选择指引）

```go
SelectionDecl{
    AgentSummary: "一句话描述工具做什么",
    UseWhen:      []string{"在什么场景下应该选择这个工具"},
    AvoidWhen:    []string{"什么场景不应该用，应该用什么替代"},
    Prerequisites: []string{"使用前提条件"},
    Tips:          []string{"使用技巧"},
    Examples:      []string{"dws dev app create --name Bot --dry-run"},
}
```

### InterfaceDecl（接口模式）

```go
InterfaceDecl{
    Mode:         "composite",  // local / pinned / composite
    Availability: "available",  // available / unavailable
    Reason:       "...",        // composite/unavailable 时的原因
    ProductID:    "...",        // pinned 时的 ref
    RPCName:      "...",        // pinned 时的 ref
}
```

### DryRunDecl（dry-run 能力）

```go
DryRunDecl{
    PreviewKind: "invocation",  // invocation / request / plan
    RemoteReads: false,         // dry-run 时是否发起远端读
}
```

## 执行体三选一

| 执行体 | 适用场景 | 框架做了什么 |
|--------|----------|-------------|
| **Invoke** | 单步 MCP/后端调用 | 框架完成 required→constraint→validate→buildArgs→confirm，传入装配好的 toolArgs |
| **Orchestrate** | 多步编排 | 框架完成 required→constraint→validate→confirm，传入 Ctx 自行组装调用 |
| **RunE** | 逃生舱 | 框架只注册 flag 和 Schema，执行完全自定义 |

## 设计不变量

1. **一个 CommandSpec = 一个叶子命令的全部事实**
2. **声明面绝不调用后端**——cmdcore 是 dispatch-agnostic
3. **执行面绝不发明 CLI 表面**——业务 flag 必须在 Flags 声明
4. **构建时拦截 > 运行时报错**——声明错误 panic 在注册阶段
5. **Safety 和 Risk 独立但可组合**——运行时行为与 Schema 元数据各管一摊
6. **声明即 review**——代码中的 Schema 经 code review 后直接投影，不依赖外部 hint 文件
