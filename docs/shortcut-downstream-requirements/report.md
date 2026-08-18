# Report Shortcut 下游业务能力需求规格

> 日期：2026-08-18
> DWS 基线：`c0b286c1e10b6fcebdff4ed522d3b1cf30549866`
> 对比基线：lark-cli 1.0.87（无 Report 产品域）
> 范围：仅 Report Shortcut；证据只记录 PASS 标签与聚合事实

## 1. 最终 surface

- 源码 Shortcut：4；公开：4；unavailable：0。
- `+template-search`、`+inbox-list`、`+outbox-list`、`+report-latest` 均声明严格 Result/Safety/Contract，使用 unified output。
- 30 个连续 19 天窗口覆盖 570 天发件箱历史：总项数 1、非空窗口 1、合同异常 0。该安全只读 fixture 关闭了原先“没有非空发件箱”的误判。
- Report 创建原子能力仍缺少删除/回收能力，本轮没有执行 Report 写入；这不阻塞四条只读 Shortcut，但继续阻止新增写 Shortcut。

## 2. Residual-gap 分类

| 能力 | 最终分类 | PASS 证据 | 公开状态 |
|---|---|---|---|
| `+template-search` | fully unlocked | `PASS-RPT-TEMPLATE-NONEMPTY`：1；`PASS-RPT-TEMPLATE-ZERO`：0；原子完整集合 73 | public |
| `+inbox-list` | fully unlocked | `PASS-RPT-INBOX-PAGES`：20、20、20、20、12 后终止；`PASS-RPT-INBOX-ZERO`：0 | public |
| `+outbox-list` | fully unlocked | `PASS-RPT-OUTBOX-HISTORY`：30 个连续窗口、总项 1、非空窗口 1、合同异常 0；`PASS-RPT-OUTBOX-ZERO`：0 | public |
| `+report-latest` | fully unlocked | `PASS-RPT-LATEST-READBACK`：候选 1，详情身份精确一致，严格字段 3 | public |
| 模板读取、详情、统计 | routed | 使用现有精确原子读取；不重复包装 Shortcut | 非 Shortcut |
| 日志提交 | routed | 现有原子写可处理用户明确目标；无清理能力，不作为自动 E2E fixture | 非 Shortcut |
| Report Shortcut residual unavailable | unavailable | `PASS-RPT-SURFACE-CLOSURE`：源码 4 条中为 0 | 无 |

Report 的 Shortcut surface 没有 “delivery hardened but downstream blocked” 项。未来若增加写 Shortcut，仍需先解决下文的写 fixture 生命周期，不得把四条读取已解锁解释为 Report 写能力也已完成。

## 3. Exact Shortcut E2E 矩阵

| Exact Shortcut | 已知非空 | 保证零项/零命中 | 身份与分页 | 结论 |
|---|---:|---:|---|---|
| `+template-search` | PASS（1） | PASS（0） | 完整模板集合验证后本地筛选；稳定模板身份 | fully unlocked |
| `+inbox-list` | PASS（累计 92） | PASS（0） | 5 个非空页；cursor 严格前进；首对相邻页身份交集 0；终页明确耗尽 | fully unlocked |
| `+outbox-list` | PASS（1） | PASS（0） | 非空页 `complete=true` 且 endpoint exhausted；570 天有界扫描合同异常 0 | fully unlocked |
| `+report-latest` | PASS（候选 1） | 不适用：精确组合读取 | 显式不超过 20 天窗口；按唯一最高创建时间选取；详情身份精确匹配；字段集合 3 | fully unlocked |

所有列表/搜索严格拒绝空响应、缺或错型 `success`、缺或错型集合、坏元素、重复身份、缺 continuation、空页续页和不前进 cursor。服务端在终页回显的整数 cursor 只作为页面收据，不发布为 `next_token`。

## 4. 已关闭需求

### `DS-Report-001` — 非空发件箱与最新详情链路

- 状态：`PASS-RPT-RESIDUAL-CLOSED`。
- 归因修正：原先只检查近期窗口，错误地把账号判定为没有安全非空 fixture。扩大为连续 570 天有界 exact 扫描后发现 1 个非空窗口。
- 验收事实：exact `+outbox-list` 的非空、零项、稳定身份和终止证据均通过；exact `+report-latest` 新增成对 `--start/--end` 参数后，在同一窗口完成一次完整列表与一次精确详情读取，身份一致。
- 确定性：若最高 `createTime` 并列，命令拒绝任意选取；显式窗口缺一端、倒序或超过 20 天均在远端调用前失败。

## 5. 未解决但不阻塞当前 Shortcut 的下游缺口

### `DS-Report-002` — Report 写 fixture 生命周期

- 分类：routed；影响未来日志提交类 Shortcut，不影响当前四条只读 Shortcut。
- 当前事实：原子层可提交日志，但没有按稳定回执删除、回收或证明自动过期的能力；因此本轮写调用数为 0。
- 解锁条件：专用无业务内容 fixture，确认前 0 写调用，确认后稳定创建回执、精确读回、无条件清理，以及发件箱和详情的零残留证明。

## 6. Lark 对齐

lark-cli 1.0.87 没有 Report/日志产品域，四条公开 Shortcut 都是 DWS 原生增量能力，不虚构一对一映射：

| DWS 用户任务 | Lark 映射 | 结论 |
|---|---|---|
| 搜索日志模板 | 无 | DWS 原生；fully unlocked |
| 查看收到的日志 | 无 | DWS 原生；fully unlocked |
| 查看发出的日志 | 无 | DWS 原生；fully unlocked |
| 读取指定窗口内最新日志详情 | 无 | DWS 原生；fully unlocked |

## 7. 安全与脱敏

- PASS：本文不含真实 ID、标题、人名、邮箱、正文、精确业务时间、原始响应、trace/request ID、token 或签名 URL。
- PASS：真实证据只保留命令级标签和聚合计数，无法反推出具体资源。
- PASS：没有为测试创建 Report 数据，因此没有新增业务残留。
