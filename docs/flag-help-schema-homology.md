# flag / help / schema 同源

- **状态**：已决策（路径 A + Contract 嵌入 Schema）
- **相关**：[`rfc-command-framework-convergence.md`](rfc-command-framework-convergence.md)、[`schema-dynamic-endpoint-design.md`](schema-dynamic-endpoint-design.md)
- **实现门面**：`LeafSpec` → `cmdcore.CommandSpec` → `cmdcore.NewCommand`

## 1. 决策

采用 **路径 A：Contract / LeafSpec 为 CLI 表面权威**，并且 **Contract 必须嵌入进 Schema**。

「同源」的含义是：同一份 Contract（今日经 LeafSpec / `CommandSpec` 表达）同时决定——且门禁能证明——

1. cobra 实际注册的 flags / required / defaults；
2. `--help` 的 Flags 与「参数约束」段；
3. **嵌入后的** `dws schema` / Catalog（`schema_catalog` / agent metadata 生成物）中的 parameters、关系约束，以及显式 `Risk` 时的 confirmation/effect。

嵌入机制（已落地）：

```text
CommandSpec
  → RegisterFlags + embedContractIntoSchema
  → cobra annotations:
       dws.schema.contract=cmdcore
       dws.schema.property / type / required  (per flag)
       dws.schema.constraints
       dws.schema.risk          (仅当 Risk 字段非空)
  → Schema 组装读取 native_annotation + Contract Risk overlay
  → go:embed schema_catalog / schema_agent_metadata
```

空 `Risk` **不**写入 `dws.schema.risk`，避免把默读盖掉今日 write-guard 叶子的 reviewed Safety。

### 1.1 硬规则：声明 OR 人工标注（禁止纯推断）

每一份进入 help / Schema 的**事实**（parameters 形状、约束、confirmation/effect 等）必须满足以下二者之一，**不得**仅靠生成器或组装期推断：

1. **声明（declare）**：写在 Contract 结构体字段里（见 §1.2），由 `cmdcore.NewCommand` 注册并嵌入；或
2. **人工标注（annotate）**：显式写入 cobra annotation / reviewed hints（见 §1.3），并由 Schema 组装**按标注**投影。

今日写命令在未升格 `Risk` 前，走 annotate 路径（`HOM-S2`）。读命令无确认语义时，二者皆可缺省；一旦存在确认/写副作用语义，则必须 declare 或 annotate。

**不采用**路径 B（以 `schema_mcp_metadata` 生成全部 CLI flag/help/schema）作为主权威。钉钉 MCP meta 不是飞书 OAPI：粒度与 CLI 特有语义（二选一、OmitEmpty、ConstParams、write guard）无法从裸 meta 推出；强行生成会违反「Schema 描述 CLI，不制造 CLI」。

路径 B 仅允许作为 **可选的 1:1 MCP 透传叶子通道**（见 §5），不得覆盖 LeafSpec / Shortcut 主路径。

对外叙事：与飞书 **分层单权威** 同构——API/透传叶可用平台事实，产品 CLI / Shortcut 用手写契约 + 执行体——而不是「全家只有平台 meta」。

### 1.2 声明（declare）：写什么、写在哪、投影到哪

**定义**：声明 = 在 Contract 结构体的**数据字段**上写出事实；`NewCommand` 据此注册 cobra、渲染 help、写入 `dws.schema.*`。钩子闭包（`Validate` / `Call` / `PostMount` / `RunE`）里的逻辑**不算**声明——即使行为正确，也不能单靠钩子让 Schema/help「猜出」该事实。

命令框架边界与今日/目标对应见 RFC [`rfc-command-framework-convergence.md`](rfc-command-framework-convergence.md) **§5.0**（框架上的「声明」定义）；本节给字段级表与示例。

**唯一写入面**（三选一，语义相同）：

| 入口 | 类型 | 归一 |
|---|---|---|
| Leaf 命令 | `helpers.LeafSpec` | `FromLeafSpec` → `cmdcore.CommandSpec` |
| Shortcut | Shortcut 声明（经 `FromShortcut`） | → `cmdcore.CommandSpec` |
| 直接基座 | `cmdcore.CommandSpec` | `cmdcore.NewCommand` |

今日产品 CLI 叶子以 **`LeafSpec` 为声明门面**；字段与 `CommandSpec` 契约面一一对应。

#### 1.2.1 契约字段（算声明）

| 字段 | 声明什么 | 运行时 | 嵌入 Schema / help |
|---|---|---|---|
| `Flags[]`（`FlagSpec` / `LeafFlag`） | 用户可见参数面：名、类型、默认、必填、usage | 注册 cobra flag；装配 toolArgs | `dws.schema.property` / `type` / `required`；`--help` Flags |
| `Constraints[]` | 跨 flag 关系：`at_least_one` / `exactly_one` / `mutually_exclusive` | `ValidateConstraints` | `dws.schema.constraints`；`--help`「参数约束」 |
| `Risk`（**非空**） | 副作用与确认：`write` / `high-risk-write`（或显式 `read`） | `ConfirmRisk`（`--yes` / `--dry-run` 跳过） | `dws.schema.risk` → Safety overlay（`HOM-S1`） |
| `ConstParams` | 固定载荷（不上 flag 表） | 并入 toolArgs；不满足 Required | **不**投影为用户 parameter |
| `Use` / `Short` / `Long` / `Example` | 命令身份文案与示例 | cobra 自身 | help；identity 仍以 registry 为准 |

`FlagSpec` 子字段（声明细节）：

| 子字段 | 作用 | 是否进 Schema parameters |
|---|---|---|
| `Name`, `Usage`, `Kind`, `Default` | 注册名/说明/类型/DefValue | 是（name/type；default 与 cobra 对齐） |
| `Required` / `MarkRequired` | 非空校验 / cobra 硬必填 | 是（`required`） |
| `RequiredHint`, `Aliases`, `EnvVar` | 校验提示、隐藏别名、环境回退 | 否（执行细节；别名不上主 parameter 表） |
| `ArgDefault`, `Bind`, `OmitEmpty`, `Trim`, `Transform` | toolArgs 装配语义 | 否（载荷细节；`Bind` 可进 property 映射，但不另造 flag） |

#### 1.2.2 编排 / 执行字段（不算声明）

| 字段 | 角色 | 禁止用来「冒充」的声明 |
|---|---|---|
| `Validate` | 条件式/领域校验钩子 | 不得在此 `Flags().String(...)` 注册业务 flag；不得只靠钩子表达「必填/互斥」而不写 `Flags`/`Constraints` |
| `Call` / `Invoke` / `Orchestrate` | 执行体 | 不得 `params[k]=…` 装配业务参数（应在 `Flags`/`ConstParams`） |
| `PostMount` | 挂载后收尾（annotate、领域工具注入） | 不得注册业务 flag；分页等横切由领域工具注入并可走 annotate |
| `RunE` | 逃生舱（整段手写） | 表面事实仍须 Flags 声明，或对确认语义做 §1.3 标注 |
| `Server` / `Tool` | MCP 路由 | 不构成 CLI parameter 声明 |

#### 1.2.3 最小声明示例

```go
// 读：无确认语义 → Risk 可空（不写 dws.schema.risk）
NewLeafCommand(LeafSpec{
    Use: "get", Short: "…", Tool: "…",
    Flags: []LeafFlag{
        {Name: "unified-app-id", Usage: "…", Bind: "unifiedAppId", Trim: true, Required: true},
    },
    Call: devAppCall(runner), PostMount: devAppMeta(tool),
})

// 写（升格 Risk = 声明确认）：ConfirmRisk + Schema Safety 同源
NewLeafCommand(LeafSpec{
    Use: "publish", Short: "…", Tool: "…",
    Flags: []LeafFlag{ /* … */ },
    Risk: LeafRiskWrite, // 声明：非空 → dws.schema.risk=write
    Call: devAppCall(runner), PostMount: devAppMeta(tool),
})

// 写（今日未升格 Risk）：确认走 annotate，见 §1.3 —— 不是「声明了写」
NewLeafCommand(LeafSpec{
    Use: "create", /* Flags… */,
    Validate: func(cmd *cobra.Command, _ []string) error {
        return devAppRequireWriteGuard(cmd, "create") // 执行守卫，不是 Contract 声明
    },
    Call: devAppCall(runner),
    PostMount: devAppMetaWrite(tool), // 人工标注 runtime_gate
})
```

**空 `Risk` 的含义**：运行时等同只读确认（不提示），且 **不**嵌入 `dws.schema.risk`。因此「会改状态却留空 Risk」**不是**合法声明；必须要么写非空 `Risk`，要么按 §1.3 标注 gate。

### 1.3 人工标注（annotate）：声明的补充通道

当事实无法或不愿放进 Contract 字段时，必须**显式**落注解 / 评审源，禁止组装期推断：

| 标注手段 | 典型值 | 何时用 |
|---|---|---|
| `cli.AnnotateRuntimeGate` / `devAppMetaWrite` | `dws.schema.runtime_gate=devAppRequireWriteGuard` | 写命令走 write-guard、尚未 `Risk` 升格（`HOM-S2`） |
| `cli.AnnotateRuntimeRisk` | `dws.schema.risk=…` | 非 Leaf 构建路径需手写嵌入 Risk（Leaf 应优先字段声明） |
| `cli.AnnotateRuntimeFlag` / Constraints | 与 embed 同形 | 手写 cobra 叶补齐表面（长期应迁入 Contract） |
| reviewed `schema_hints/metadata` Safety | effect/risk/confirmation | 迁移期遗留；受管命令收敛后让位于 Risk/gate |

标注与声明冲突时：**Contract 声明胜**（路径 A）。标注不得发明未注册的 CLI flag。

### 1.4 Schema 全覆盖（`ToolSpec` 无空洞）

`dws schema` 叶子模型是 `cli.ToolSpec`。命令框架必须为**每一个字段组**指定权威；完整矩阵在 RFC **§5.0.4**，摘要：

| ToolSpec 组 | 权威类 | 框架声明字段 / 其它源 |
|---|---|---|
| Identity | 评审源 | `schema_command_registry` |
| Parameters.`name/type/required/default/property` | **声明**（或同形 annotate） | `Flags` / `Bind` |
| Parameters.`description` | 声明 usage + 可选 hints overlay | `Usage`；hints 不得改 type/required/default |
| Parameters.`interface_*` | 评审源 | MCP meta / bindings；**不造 flag** |
| Constraints | **声明** | `Constraints` |
| Positionals | **声明** 或显式 annotate | 目标 `Args`；禁止推断 |
| Safety.`effect/risk/confirmation` | **声明**非空 `Risk` **或** `runtime_gate` 标注（或迁移期 reviewed Safety） | 空 Risk 不嵌入 |
| Safety.`idempotency` | 评审源 | hints metadata |
| DryRun | 评审源 | dry-run capabilities registry |
| Interface | 评审源 | MCP + agent metadata |
| Selection | 评审源 | `schema_hints/selection` |
| FieldProvenance / Extensions | 组装派生或评审扩展 | 组装器；与 delivered value 一致 |
| （非 Schema parameter）ConstParams | **声明** | 载荷；不上 parameters 表 |

验收：新增 Schema 字段必须同步改 RFC §5.0.4 + 本表；受管写命令 Safety 不得无主。

## 2. 字段归属（每一类恰好一个写入者）

| 字段类 | 权威 | 投影到 |
|---|---|---|
| flags / defaults / required / enum / 关系约束 / 运行时 Risk | Contract（LeafSpec / `CommandSpec` 门面） | cobra、`--help`、Schema `parameters` / constraints / confirmation |
| ConstParams、Bind、OmitEmpty、Transform | 同上（载荷声明，不上 flag 表） | toolArgs；Schema 不把 ConstParams 伪装成用户 flag |
| canonical path / aliases / navigation / exposure | `schema_command_registry`（+ reviewed manual additions） | Schema identity |
| use_when / avoid_when / examples / agent_summary 文案 | `schema_hints/selection` | Schema selection |
| RPC tool 形状、`interface_ref`、interface 描述 | `schema_mcp_metadata` + `schema_parameter_bindings` | Schema `interface_*` 字段；**不得创建 flag** |
| 参数描述 overlay（可选） | `schema_hints/metadata` 仅补充 usage 文案 | 与 Contract/cobra 冲突时 **Contract/cobra 胜** |
| 遗留 Safety 文案（迁移期） | 今日仍可读 `schema_hints/metadata` safety | 受管命令收敛后以 Contract.Risk / runtime_gate 为准（见 §4） |
| dry-run 正能力 | reviewed dry-run registry | Schema `dry_run` |
| positionals | Contract Args / 显式 annotate | Schema `positionals` |
| FieldProvenance | 组装派生 | Schema provenance（与值一致） |

Identity 与 selection **刻意不**由 Contract 取代（RFC 决策 8 / schema 设计硬规则）。**完整无空洞表见 RFC §5.0.4。**

## 3. 当前缺口与目标闭环

已具备：

- Flags / ConstParams / Constraints → 注册、校验、`ConstraintHelp`、运行时 `ConfirmRisk`（cmdcore）；
- Call / Execute 作为执行体；业务参数不得在 Call 内装配（helpers 门禁）；
- **Contract → Schema 注解嵌入**（`embedContractIntoSchema`）：`dws.schema.contract` / property / type / required / constraints；显式 Risk → `dws.schema.risk` 并在 Schema 组装时 overlay Safety。

仍缺：

1. `HOM-P*` / `HOM-D1` 门禁：禁止 hints 静默改写 type/required/default；help ≡ schema parameters；
2. 重新 `make generate-schema` 后把嵌入结果固化进签入 catalog（注意本机 OOM）；identity/selection 仍走评审源。

已落地（写命令确认语义，`HOM-S2` 起点）：

- `AnnotateRuntimeGate` / `dws.schema.runtime_gate`；Leaf `PostMount: devAppMetaWrite`；手写 delete/robot 等同路径显式标注；
- Schema 组装在无 Contract Risk 但有 gate 时 overlay `confirmation=user_required`（`applyContractGateToSafety`）；
- AST/mount 测试：Validate 含 `devAppRequireWriteGuard` ⇒ PostMount 必须 `devAppMetaWrite`；挂载叶须 Risk 或 runtime_gate。

## 4. Schema 投影与 Safety 门禁规划

以下门禁在落地时应作为独立 policy / 测试交付（门禁 ID 稳定，便于 CI 认领）。

| Gate ID | 断言 | 范围 |
|---|---|---|
| `HOM-P1` | 受管 leaf 的 schema `parameters[].name` 集合 ≡ cobra 本地 flag 名集合（排除全局 persistent） | LeafSpec / 未来 Contract 编译命令 |
| `HOM-P2` | schema parameter `type` / `required` / `default` 与 cobra DefValue / MarkFlagRequired / FlagSpec 一致；hints overlay **不得**改写这三项，只可补 description | 同上 |
| `HOM-P3` | schema 关系约束（require_one_of / mutually_exclusive）≡ Contract/Leaf `Constraints` 投影（与 `AnnotateConstraints` 同构） | 声明了 Constraints 的命令 |
| `HOM-S1` | 若 Contract/Leaf `Risk` ∈ {write, high-risk-write}，则 schema `confirmation=user_required`，且 help Safety 行与之同语义 | 使用 Risk 确认的受管命令 |
| `HOM-S2` | 若命令走显式 write guard（如 `devAppRequireWriteGuard`）而非 Risk，则必须人工标注 `dws.schema.runtime_gate`（或等价 reviewed Safety）；Schema 不得呈 `confirmation=not_required`；符合 §1.1 declare OR annotate | 今日 devapp 写命令 |
| `HOM-S3` | `Risk=read`（或空→read）不得投影为 `user_required`，除非有 reviewed exclusion reason | 受管读命令 |
| `HOM-I1` | `interface_ref` 存在时，bindings 覆盖的 CLI flag ⊆ Contract flags；MCP meta **不**引入额外 CLI flag | 有 MCP 绑定的命令 |
| `HOM-D1` | `dws <path> --help` Flags 段与 schema leaf parameters 零未解释增量 | 抽样 + 受管全集逐步扩大 |

落地顺序建议：

1. 先对 LeafSpec 命令实现 `HOM-P1`/`HOM-P2`/`HOM-P3`（反射 cobra 即可起步，因 flag 已由 Contract 注册）；
2. 再收 `HOM-S1`–`S3`（需产品确认 devapp 是否升格 Risk，或保持 guard + 诚实 provenance）；
3. `HOM-I1`/`HOM-D1` 接入 `check-schema-*` / PR 门禁。

## 5. 路径 B 子通道：1:1 MCP 透传叶（可选，非主路径）

仅当同时满足以下条件时，才允许「从 MCP meta 生成 flag/help/schema 参数」：

1. CLI path ↔ 单一 MCP tool **严格 1:1**，无多步、无按名解析、无本地 effect；
2. 无不在 MCP input schema 中的 CLI 特有 flag（含 guard 专用语义 flag 除外的全局 `--yes`/`--dry-run`）；
3. 无 ConstParams / Transform / 跨 flag 约束 / Call 内业务逻辑；
4. Risk/confirmation 在 meta 或并列 reviewed Safety 中有显式来源，不靠生成器猜测；
5. 在 registry 中标记 `surface_kind=mcp_passthrough`（名称可调整），且 **不得**与 LeafSpec/Shortcut 手写定义双注册同一 `cli_path`；
6. 文档与门禁写明：该通道是子集优化，失败时回退/禁止扩张到产品 CLI。

显式排除（永远走路径 A / Shortcut）：

- 全部 `LeafSpec` 命令（含 `dws dev app …`）；
- 全部 `+shortcut` 与 smart 编排；
- 任何需要 `devAppRequireWriteGuard`、cursor 工具注入、或响应投影的命令。

## 6. 非目标

- 不把 selection 文案或 canonical identity 塞进 Contract。
- 不要求删除 LeafSpec 门面。
- 不把「生成 catalog 字节一致」当作运行时同源的充分条件（仍需 `HOM-*` 与差分门禁）。
