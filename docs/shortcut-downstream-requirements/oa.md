# OA Shortcut 下游能力需求

基线：`c0b286c1e10b6fcebdff4ed522d3b1cf30549866`；Comparator：`lark-cli 1.0.87`；审计日期：2026-08-18。

本文只记录脱敏的能力标签与聚合事实。没有资源 ID、业务标题、姓名、邮箱、精确业务时间、trace/request ID 或原始响应。

## 结论与 surface

- 源码 Shortcut：10。
- 公开：1（`+search-forms`）。
- 隐藏：9，其中 unavailable 8；`+done-approvals` 是 available 但隐藏的兼容入口。
- 所有 10 条均声明 `Contract`、`Safety`、`Result` 并启用 unified output；写入口在首次远端调用前由 Runtime 确认门禁阻断。

### Residual-gap 分类

| Shortcut / 用户路线 | 最终分类 | 依据 |
|---|---|---|
| `+search-forms` | fully unlocked | current HEAD exact/raw：已知非空 1、保证零命中 0，稳定身份集合一致；public |
| `+list-executed` | delivery hardened but downstream blocked | known-nonempty exact/raw 7 个稳定身份一致；guaranteed-zero raw 缺 `hasMore`，严格实现拒绝 |
| `+list-submitted`、`+my-initiated` | delivery hardened but downstream blocked | known-nonempty exact/raw 14 个稳定身份一致；guaranteed-zero raw 缺 `hasMore`，严格实现拒绝 |
| `+done-approvals` | routed | available 的隐藏兼容别名；新调用路由到 `+list-executed` |
| `+list-forms` | delivery hardened but downstream blocked | exact 与 raw 均缺 continuation，改变 cursor 后集合高度重叠 |
| `+list-pending` | delivery hardened but downstream blocked | exact 与 raw 均得到 `success=true`，但 `result.values` 为 `null` 而不是合法空数组 |
| `+pending` | unavailable | 隐藏兼容入口路由到同一受阻待办能力，没有独立安全 fixture |
| `+list-cc` | delivery hardened but downstream blocked | exact 与 raw 隔离到错型业务状态与 `null` 集合，不能解释为合法零项 |
| `+approve-by` | unavailable | 复合编排与确认门禁已实现，但没有隔离待办、终态读回和可恢复清理 fixture |
| 已知实例/任务的详情与写操作 | routed | 使用精确 OA 原子命令；必须由用户提供真实目标并单独确认，不等于 Shortcut 已解锁 |

OA 不能作为整体宣称 fully unlocked：源码 10 条中仅 1 条 fully unlocked，1 条兼容路由，6 条 delivery hardened but downstream blocked，2 条业务能力 unavailable；Catalog unavailable 共 8 条。

### Current HEAD 公开 Shortcut 双层发布矩阵

| EVERY public Shortcut | Exact Shortcut | Owning atomic/raw | known-nonempty / guaranteed-zero | current HEAD 结论 |
|---|---|---|---|---|
| `oa +search-forms` | `dws oa +search-forms --query <known-or-random-query> --format json` | `dws oa approval search-forms --query <same-query> --format json` | 已知查询 1/1，稳定 `processCode` 集合相等；随机 UUID 查询 0/0 | `PASS-OA-SEARCH-DOUBLE`；fully unlocked |

该搜索接口一次返回完整匹配集合，不声明 cursor；双层证据验证的是同场景完整集合、稳定身份和独立随机零命中，而不是把已知集合后的空页当零命中。实现与严格响应测试位于 `internal/shortcut/oa/oa.go`、`internal/shortcut/oa/common.go` 和 `internal/shortcut/oa/oa_strict_cross_platform_coverage_test.go`。

| Exact Shortcut 证明 | 标签 | 聚合事实 |
|---|---|---|
| `+list-executed` fail-closed | PASS | current HEAD known-nonempty exact/raw 为 7 且稳定身份集合一致；guaranteed-zero raw 缺 `hasMore`，exact 返回 `missing_pagination` |
| `+list-submitted` fail-closed | PASS | current HEAD known-nonempty exact/raw 为 14 且稳定身份集合一致；guaranteed-zero raw 缺 `hasMore`，exact 返回 `missing_pagination` |
| `+my-initiated` fail-closed | PASS | current HEAD known-nonempty exact/raw 为 14 且稳定身份集合一致；不回退 raw；guaranteed-zero 缺分页时失败 |
| `+search-forms` | PASS | current HEAD 已知非空 1、保证零命中 0；exact/raw 稳定身份集合一致 |
| `+done-approvals`（隐藏兼容） | PASS | 已知非空 7；首屏终态明确 |
| `+list-forms` fail-closed | PASS | 非空页缺 continuation 被拒绝；两个 cursor 的 93/93 项重叠 92 项 |
| `+approve-by` 确认门禁单测 | PASS | 确认前 0 调用；确认后固定为读待办、读任务、写审批、精确任务读回 |
| `+list-pending` fail-closed | PASS | exact 与 raw 均隔离到布尔成功、对象结果、`null` 集合；拒绝伪造空数组 |
| `+pending` fail-closed | PASS | 与主入口一致拒绝 `null` 集合；没有独立宽松回退 |
| `+list-cc` fail-closed | PASS | exact 与 raw 均拒绝错型业务状态及 `null` 集合 |

未列为 fully unlocked 的能力没有安全的已知非空或可逆写 fixture，或下游响应合同本身不合法；`null` 集合不是显式空结果，不能替代合法零项证明。

## Lark 1.0.87 用户任务映射

Lark approval 共 14 个任务；映射按用户结果而不是命令名判断。

| Lark 任务 | DWS 当前路线 | 分类 | 临时处置 |
|---|---|---|---|
| approvals get | `oa approval form-schema` | routed | 使用原子读取 |
| approvals search | `oa +search-forms` | covered | 公开 Shortcut |
| instances initiated | `oa approval list-submitted` | routed | 两条 Shortcut 因 guaranteed-zero 缺分页证据而 unavailable；使用精确原子读取 |
| instances get | `oa approval detail` | routed | 使用原子读取 |
| instances cancel | `oa approval revoke` | routed | 高风险原子写；无可逆 fixture，不新增公开 Shortcut |
| instances cc | `oa approval oa-cc-noticer` | routed | 无安全 fixture，保持原子入口 |
| instances create | `oa approval forecast-process` + `create-instance` | routed | 无可清理实例 fixture，不新增公开 Shortcut |
| tasks query | 四类列表与 `oa approval tasks` | delivery hardened but downstream blocked | 已办/已发起非空可读，但零命中缺分页；待办返回 `null` 集合，抄送返回错型状态与 `null` 集合 |
| tasks add_sign | `oa approval append-task` | routed | 无安全 fixture，保持原子入口 |
| tasks approve | `oa approval approve` / `+approve-by` | routed / unavailable | 原子写需用户确认；复合 Shortcut unavailable |
| tasks reject | `oa approval reject` | routed | 无安全 fixture，保持原子入口 |
| tasks remind | `ding-info` 后串联 DING 发送 | downstream_required | 缺少一个可验证的稳定催办回执合同 |
| tasks rollback | `revert-activities` + `revert-task` | routed | 无可恢复 fixture，保持原子入口 |
| tasks transfer | `redirect-task` | routed | 无安全 fixture，保持原子入口 |

DWS 现有原子能力还覆盖审批记录、表单 Schema、流程预测、评论、可回退节点等产品原生任务。它们当前应由精确原子命令承担；在取得稳定 Result、fixture 与写后验证前，不为增加数量而包装 Shortcut。

## `DS-oa-001`：可证明前进的可见表单分页

- 优先级：P1；类型：adapter defect / contract insufficient；Owner：OA MCP adapter 与业务接口。
- 影响：`oa +list-forms` 当前 unavailable。Exact Shortcut 与同参数 atomic/raw 均显示非空列表，但响应没有 `hasMore`、`nextCursor` 或等价 continuation；改变 cursor 后集合高度重叠，已排除 Shortcut 投影错误。
- 所需合同：
  - 请求明确 `cursor`（首次为 0）与 `pageSize`（1–100）。
  - 成功必须是布尔 `success=true`，并包含显式 `result.processCodeList` 数组。
  - 每个元素必须包含唯一、非空 `processCode` 和非空名称。
  - 必须返回布尔 `hasMore`；`hasMore=true` 时返回非空、与当前值不同且单调前进的 `nextCursor`；`hasMore=false` 时不得返回非空 continuation。
  - cursor 不得被忽略；重复页、回退游标与循环必须返回稳定非成功错误。
  - 明确页面大小上限、总页/总项边界和合法空首页语义。
- 验收：Exact Shortcut 与 atomic/raw 对同一脱敏 fixture 逐页得到不重复稳定身份集合；已知非空和保证零命中均通过；缺失、重复、循环和错型 continuation 都返回非零。
- 临时处置：`+list-forms` 保持 hidden/unavailable；需要精确定位定义时使用公开 `+search-forms`。

## `DS-oa-002`：可重复的审批读写 E2E fixture

- 优先级：P1；类型：tenant-or-fixture；Owner：OA 测试租户/权限与测试数据平台。
- 影响：待办、抄送、任务、同意/拒绝/撤回/转交/加签/退回/催办等用户任务不能完成安全 live proof。当前 raw 待办集合为 `null`；抄送同时存在错型业务状态与 `null` 集合，二者都不是合法空页。
- 所需 fixture：
  - 隔离测试身份与专用审批定义，可创建至少一条待办、一条抄送和一条已办实例。
  - 创建回执返回稳定实例 ID；任务查询返回稳定 taskId 和明确任务状态。
  - 写操作可在隔离资源上完成，且具有确定的恢复、撤回或自动到期路径。
  - fixture 权限最小化，支持在测试结束后验证零残留；不得复用真实业务审批。
- 验收：每个 list/search 同时通过已知非空与保证零命中；每个写在确认前 0 调用，确认后只写一次，按同一实例和 taskId 精确读回终态并完成清理。
- 临时处置：相关 Shortcut 保持 hidden/unavailable；精确原子写仅在用户明确确认并提供真实目标时执行，不记为自动化 E2E PASS。

## `DS-oa-003`：审批写入的稳定终态回执与读回语义

- 优先级：P1；类型：contract insufficient；Owner：OA 业务服务与 MCP adapter。
- 影响：即使写调用返回 nominal success，也难以区分已提交、处理中、已生效与 commit-unknown，阻止安全公开 `+approve-by` 及其他复合写任务。
- 所需合同：
  - 写回执必须包含 `success=true`、稳定 `processInstanceId`、稳定 `taskId`、终态或 typed pending 状态及操作类型。
  - `success=false`、缺 success、空响应、错型 ID、业务错误码冲突必须非零；不得以请求 echo 作为成功证据。
  - 同一 taskId 的查询必须提供明确 pending/approved/rejected/transferred/rolled_back 状态，或明确从待处理集合移除的强一致性窗口。
  - 异步操作返回可执行查询动作、轮询边界和 terminal failure；超时保留 commit-unknown，禁止自动重试非幂等写。
  - 批量/多步操作提供逐项 ledger；任一失败不得返回整体 success。
- 验收：Exact Shortcut 写入后以相同实例与 taskId 读回请求关键状态；回执缺失、读回不一致、异步超时和部分失败均机器可检测；安全 fixture 完成恢复与零残留。
- 临时处置：`+approve-by` 保持 hidden/unavailable；当前实现即使写调用成功，读回无法证明时也返回非成功且标记 execution started。

## `DS-oa-004`：编号列表零命中的分页终态

- 优先级：P1；类型：contract insufficient；Owner：OA MCP adapter 与业务接口。
- 影响：`+list-executed`、`+list-submitted`、`+my-initiated`。current HEAD 的 known-nonempty exact/raw 分别以 7、14、14 个稳定身份完全对齐，且 raw 明确 `hasMore=false`；但 guaranteed-zero-query 的 raw 响应只返回显式空 `values`，缺少 `hasMore`。
- 严格处置：空数组不推导分页终态。Shortcut 对缺 `hasMore` 返回 `missing_pagination`，三条命令均为 hidden/unavailable。
- 验收：同一 guaranteed-zero-query 的 raw 响应必须返回布尔 `hasMore=false`；若将来允许 continuation，则必须提供可执行且严格前进的页码/游标，不能依赖集合长度推断。

## 超越 Lark 的机会

在上述两个下游合同和安全 fixture 就绪后，可把 DingTalk 原生的“流程预测→精确实例创建→节点/任务读回→操作记录审计”组合成一条带逐步 ledger 的 Golden Route。它应保持每一步稳定身份、支持 typed pending/partial/commit-unknown，并在创建后的任一步失败时给出可执行恢复动作；在此之前不发布该复合 Shortcut。
