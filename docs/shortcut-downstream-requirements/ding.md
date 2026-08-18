# DING Shortcut 下游业务能力需求规格

> 日期：2026-08-18
> DWS 基线：`c0b286c1e10b6fcebdff4ed522d3b1cf30549866`
> 对比基线：lark-cli 1.0.87
> 范围：Shortcut-only DING surface

## 1. 执行摘要

- 源码 Shortcut：5；公开：1；unavailable：4。
- 读取严格化：PASS。`+list` 与 `+receiver-status` 只接受显式布尔成功、准确结果路径、正确集合类型和合法元素。
- 分页真实性：PASS（拒绝虚假成功）。`+list` 仅在 `hasMore` 与数值 `nextCursor` 一致且游标前进时发布续页信息；真实续页返回停滞游标，严格实现返回 `stalled_cursor`，因此该能力保持 unavailable。
- 真实读取：PASS。`+list` 已执行已知非空页和保证零项页；`+receiver-status` 已对列表中稳定身份执行精确非空查询。
- 写安全：PASS。3 个写 Shortcut 均需确认，确认前 0 次下游调用；因缺少可逆 fixture、精确接收人读回和零残留清理，确认后也由本地能力门禁停止。
- 脱敏：PASS。本文仅保留聚合数量与 PASS 标签，不保存 ID、标题、人名、正文、原始响应或请求追踪信息。

## 2. 用户任务与 Lark 映射

| 用户任务 | DWS Shortcut | lark-cli 1.0.87 | 状态 |
|---|---|---|---|
| 查询 DING 历史并取得稳定身份 | `ding +list` | 无直接任务 | unavailable；真实续页游标停滞 |
| 查询 DING 接收状态 | `ding +receiver-status` | `im messages read_users` | PASS / public；平台身份模型不同 |
| 基于已有消息发送应用内加急 | `ding +send-by-message --type app` | `im messages urgent_app` | unavailable |
| 基于已有消息发送短信加急 | `ding +send-by-message --type sms` | `im messages urgent_sms` | unavailable |
| 基于已有消息发送电话加急 | `ding +send-by-message --type call` | `im messages urgent_phone` | unavailable |
| 以用户身份新建 DING | `ding +send-personal` | 无直接任务 | unavailable；DWS 产品原生机会 |
| 撤回用户身份 DING | `ding +recall-personal` | 无直接任务 | unavailable；DWS 产品原生机会 |

## 3. 真实 E2E 聚合矩阵

| Exact Shortcut | 已知非空 | 保证零项 | 身份/分页 | 写后读回 | 清理 | 结论 |
|---|---:|---:|---|---|---|---|
| `+list` | PASS（50） | PASS（0） | PASS（严格拒绝）；续页游标停滞 | 不适用 | 不适用 | unavailable |
| `+receiver-status` | PASS（1） | 不适用：精确 ID 查询 | PASS：全部返回项身份匹配 | 不适用 | 不适用 | public |
| `+send-personal` | 不适用 | 不适用 | 不适用 | BLOCKED | BLOCKED | unavailable |
| `+send-by-message` | 不适用 | 不适用 | 不适用 | BLOCKED | BLOCKED | unavailable |
| `+recall-personal` | 不适用 | 不适用 | 不适用 | BLOCKED | BLOCKED | unavailable |

## 4. 下游需求

### DS-DING-001 — 稳定写回执与接收人身份

- 优先级：P1；类型：contract insufficient。
- 影响：`+send-personal`、`+send-by-message`。
- 需要明确布尔业务成功、稳定 `openDingId`、逐接收人稳定身份与受理状态；集合必须始终存在且类型固定，不能用空响应或仅 HTTP 成功表示业务成功。
- 写后读取必须能按同一 `openDingId` 返回稳定接收人身份，以精确比对请求集合，而不是仅返回显示名。
- 幂等键重复提交必须返回同一资源或稳定冲突语义，并标明是否已开始执行与是否可安全重试。
- 验收：确认前 0 调用；确认后写入一次；按稳定身份精确读回全部接收人；清理后再次读取证明无残留。

### DS-DING-002 — 可验证撤回终态

- 优先级：P1；类型：missing capability。
- 影响：`+recall-personal` 及两种发送能力的清理闭环。
- 撤回应返回稳定目标身份与明确终态，或提供按该身份查询撤回状态的只读接口。
- `success=true` 单一回执不足以证明接收端通知已撤回；需要可查询的 terminal 状态，并区分未开始、已完成、部分撤回与未知效果。
- 验收：创建专用可回滚 fixture；精确撤回；按同一身份读回终态；重复撤回具有稳定幂等语义；无通知残留。

### DS-DING-003 — 游标边界语义

- 优先级：P2；类型：contract insufficient。
- 影响：`+list` 的跨页聚合调用方。
- 当前真实续页返回的 `nextCursor` 未严格前进，且基线相邻页还观察到 1 个稳定身份边界重复项。下游需声明 `nextCursor` 是包含式还是排除式边界，并保证续页凭据可以推进。
- 验收：连续两页均返回合法分页事实；游标严格前进；跨页无重复，或合同明确要求调用方按稳定身份去重；终页明确 `hasMore=false` 且无虚假续页凭据。

## 5. Fixture、权限与清理

| 需求 | 安全资源模型 | 到期/回收 | 状态 |
|---|---|---|---|
| 应用内 DING 写 fixture | 专用测试接收人和无业务正文 | 用后立即撤回并读回终态 | BLOCKED |
| 短信/电话 DING fixture | 隔离租户、显式成本授权、专用号码 | 用后核对计费与通知残留 | BLOCKED |
| 消息转 DING fixture | 专用测试会话与可删除源消息 | 撤回 DING 后删除源消息 | BLOCKED |
| 接收状态读取 fixture | 由公开列表取得稳定 DING 身份 | 只读，无残留 | PASS |

## 6. 安全与脱敏声明

- PASS：不含用户、组织、租户、profile、消息、会话或 DING 的真实 ID。
- PASS：不含标题、正文、人名、邮箱、电话、精确业务时间、trace/request ID、token 或原始响应。
- PASS：真实证据仅记录命令级标签和聚合计数；未将本地诊断响应提交到仓库。
