# DING Shortcut 下游业务能力需求规格

> 日期：2026-08-20
> 当前 main 基线：`b6eaf3c5af77b26cf1b1559fb7ea173fd0c5b971`
> 对比基线：lark-cli 1.0.87
> 范围：Shortcut-only DING surface

## 1. 最终 surface

- 源码 Shortcut：5；公开：1；unavailable：4。
- 公开 `+receiver-status` fully unlocked；其余 4 条保持 fail-closed，不能把 DING 整体写效果称为完成。
- 读取严格化：PASS。只接受显式布尔成功、准确结果路径、正确集合类型、合法元素和稳定 DING 身份。
- 本地可修缺口：PASS。`app/sms/call` 已在原子 helper 映射为下游要求的 `APP/SMS/PHONE`，未知类型在调用前拒绝。
- 写门禁：PASS。3 个写 Shortcut 均需用户确认，且当前 capability gate 不发起远端调用；隔离诊断只通过精确 atomic/raw 路径执行。

## 2. Residual-gap 分类

| 能力 | 最终分类 | 归因 | Shortcut 状态 |
|---|---|---|---|
| `+receiver-status` | fully unlocked | exact 稳定 DING 身份读取返回 1 项且全部匹配 | public |
| `+list` | delivery hardened but downstream blocked | 真实 continuation 不前进并返回重复边界项；本地不能安全猜测跳游标或去重 | unavailable |
| `+send-personal --type app` | delivery hardened but downstream blocked | 稳定 DING 回执已证明；接收项没有稳定接收人身份，撤回无可查询终态 | unavailable |
| `+send-by-message --type app` | delivery hardened but downstream blocked | 源消息和 DING 稳定身份已证明；接收人与清理终态仍不可证明 | unavailable |
| `+send-personal/+send-by-message --type sms/call` | unavailable | 没有隔离号码、显式成本授权和通知残留核验 fixture，本轮未执行 | unavailable |
| `+recall-personal` | delivery hardened but downstream blocked | 撤回只返回布尔成功，不返回目标或终态；撤回后精确接收状态仍保留记录 | unavailable |
| Lark `urgent_app/sms/phone` 用户任务 | routed | 精确原子命令存在；仅在用户明确目标、提醒方式与成本后执行，不等于 Shortcut 已解锁 | 非公开 Shortcut 路线 |

## 3. Lark 用户任务映射

| 用户任务 | DWS 路线 | lark-cli 1.0.87 | 结论 |
|---|---|---|---|
| 查询 DING 历史 | `ding +list` | 无直接任务 | delivery hardened but downstream blocked |
| 查询 DING 接收状态 | `ding +receiver-status` | `im messages read_users` | fully unlocked；平台身份模型不同 |
| 基于消息发送应用内加急 | `ding message send-by-message --type app` | `im messages urgent_app` | routed；Shortcut unavailable |
| 基于消息发送短信加急 | `ding message send-by-message --type sms` | `im messages urgent_sms` | routed，需成本授权；Shortcut unavailable |
| 基于消息发送电话加急 | `ding message send-by-message --type call` | `im messages urgent_phone` | routed，需成本授权；Shortcut unavailable |
| 新建个人 DING | `ding message send-personal` | 无直接任务 | routed；Shortcut unavailable |
| 撤回个人 DING | `ding message recall-personal` | 无直接任务 | routed；Shortcut unavailable |

## 4. Exact Shortcut 与隔离 raw 证据

下表中 `+receiver-status` 是本轮 current-HEAD 发布证据；其余非公开入口为先前同租户隔离诊断，只用于说明下游缺口，不计入本轮公开能力 PASS。

| 能力 | Exact Shortcut | Atomic/raw 隔离事实 | 结论 |
|---|---|---|---|
| `+list` | 已知非空 50；保证零项 0；真实续页返回 `stalled_cursor` | continuation 页仍为 50 项、下一游标不前进、相邻稳定身份交集 1；尝试包含式边界后仍不前进 | downstream blocked |
| `+receiver-status` | 精确身份返回 1，全部 DING 身份匹配 | 集合为数组、业务成功为布尔值 | fully unlocked |
| `+send-personal` | capability gate，远端调用 0 | APP 自收件：稳定 DING 回执 1、接收状态 1、DING 身份匹配；稳定接收人身份字段 0 | downstream blocked |
| `+send-by-message` | capability gate，远端调用 0 | 自发源消息稳定任务/消息/会话身份均通过；APP DING 回执 1、接收状态 1；稳定接收人身份字段 0 | downstream blocked |
| `+recall-personal` | capability gate，远端调用 0 | 撤回业务成功为布尔值，但目标回执 0；撤回后精确接收状态仍为 1 | downstream blocked |

### Current HEAD 公开 Shortcut 双层发布矩阵

| EVERY public Shortcut | Exact Shortcut | Owning atomic/raw | 稳定身份与结果 | current HEAD 结论 |
|---|---|---|---|---|
| `ding +receiver-status` | `dws ding +receiver-status --ding-id <stable-id> --format json` | 先以 `dws ding message list --type ALL --format json` 取得同一稳定 ID，再执行 `dws ding message receiver-status --ding-id <same-id> --format json` | 原子来源页 50；exact/raw 接收项均为 1；请求 DING ID、每项 DING ID 与完整接收行集合一致 | `PASS-DING-RECEIVER-DOUBLE`；fully unlocked |

这是精确稳定 ID 读取，不是 list/search，因此不制造随机不存在 ID 的“零命中”。实现与严格响应测试位于 `internal/shortcut/ding/ding.go`、`internal/shortcut/ding/common.go` 和 `internal/shortcut/ding/ding_strict_cross_platform_coverage_test.go`。

隔离 APP fixture 撤回后，`SEND`、`DELETED`、`ALL` 首屏均未出现目标；但 `+list` 的 continuation 已被证明停滞，首屏缺席不能升级为全局零残留证明。源聊天消息也在执行撤回后仍可按稳定身份读取，因此不能把“记录仍可查询”擅自解释为已删除或未删除。

## 5. 下游需求

### `DS-DING-001` — 稳定接收人身份读回

- 优先级：P1；类型：contract insufficient。
- 已关闭部分：`PASS-DING-WRITE-RECEIPT`，两种 APP 发送路径均返回唯一稳定 `openDingId`；提醒类型枚举映射已在本地修正。
- 剩余缺口：接收状态项只有 DING 身份、确认状态和显示名，没有可与请求 `openDingTalkId` 集合精确比对的稳定接收人身份。
- 验收：返回逐接收人的稳定身份和 typed 受理状态；多接收人集合与请求集合精确一致；缺项、重复项、额外项和错型均机器可检测。

### `DS-DING-002` — 可查询撤回终态

- 优先级：P1；类型：missing capability。
- 当前事实：撤回响应不含目标身份或终态；精确接收状态在撤回后仍保留；列表首屏缺席不能越过坏分页证明全局终态。
- 验收：按同一 `openDingId` 返回 typed `recalled` 终态或提供专用只读查询；重复撤回语义稳定；能够区分完成、处理中、部分撤回和 commit-unknown，并证明无残留通知。

### `DS-DING-003` — 游标边界与前进语义

- 优先级：P2；类型：contract insufficient。
- 当前事实：首屏 50 项；真实 continuation 再返回 50 项，下一游标不前进，跨页身份交集 1。对包含式边界加一仍返回不前进页面，排除安全的本地 `+1` 修补。
- 验收：连续页凭据严格前进；跨页无重复，或合同明确包含式边界且可证明去重不遗漏；终页明确 `hasMore=false`。

## 6. Fixture、安全与脱敏

- PASS：APP 仅使用当前账号自收件、合成无业务正文与立即撤回；没有执行短信或电话提醒。
- PASS：没有提交用户、组织、会话、消息、DING 的真实 ID，也没有标题、正文、人名、邮箱、电话、精确业务时间、trace/request ID、token 或原始响应。
- PASS：文档只保留命令级标签与聚合计数；DING 写 Shortcut 仍为 unavailable，未把原子诊断回执当成公开能力完成。
