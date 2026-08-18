# Report Shortcut 下游业务能力需求规格

> 日期：2026-08-18
> DWS 基线：`c0b286c1e10b6fcebdff4ed522d3b1cf30549866`
> 对比基线：lark-cli 1.0.87（无 Report 产品域）
> 范围：仅 Report Shortcut；证据只记录 PASS 标签与聚合事实

## 1. 执行摘要

- 源码 Shortcut 共 4 个；公开 2 个，unavailable 2 个。
- `+template-search` 已完成严格模板集合、已知非空和保证零命中证明，可公开。
- `+inbox-list` 已完成已知非空、保证零项、多页 cursor 严格前进、相邻页稳定身份无重复和明确耗尽证明，可公开。
- `+outbox-list` 在多个有界窗口只得到合法零项结果；缺少已知非空证明，因此保持 unavailable。
- `+report-latest` 依赖完整非空发件箱和精确详情读回；当前没有安全 fixture，因此保持 unavailable，且不再回退原始列表行。
- Report 原子层具有模板、收件箱、发件箱、详情、统计和创建能力；创建缺少可验证删除/回收能力，本轮未执行写操作。

| ID | 优先级 | 类型 | 用户任务 | 当前处置 | 解锁的 Shortcut |
|---|---|---|---|---|---|
| `DS-Report-001` | P1 | tenant-or-fixture / missing capability | 读取自己发出的日志及最新详情 | hidden + unavailable | `+outbox-list`、`+report-latest` |

## 2. 用户任务与证据矩阵

| 用户任务 | Shortcut | Lark CLI 对应 | Exact Shortcut 证据 | Atomic/raw 对照 | 结论 |
|---|---|---|---|---|---|
| 按名称搜索可用日志模板 | `+template-search` | 无 | `PASS-RPT-TEMPLATE-NONEMPTY`：1 项；`PASS-RPT-TEMPLATE-ZERO`：0 项 | `PASS-RPT-TEMPLATE-COLLECTION`：完整集合聚合数量 73 | available / public |
| 分页查看收到的日志 | `+inbox-list` | 无 | `PASS-RPT-INBOX-PAGES`：页大小依次为 20、20、20、20、12，随后明确耗尽；`PASS-RPT-INBOX-ZERO`：0 项 | `PASS-RPT-INBOX-CURSOR`：非终止 cursor 严格递增；首对相邻页稳定身份交集为 0 | available / public |
| 分页查看自己发出的日志 | `+outbox-list` | 无 | `PASS-RPT-OUTBOX-ZERO`：多个有界窗口均为 0 项 | `PASS-RPT-OUTBOX-TERMINAL`：零项页具有明确成功与终止证据 | unavailable |
| 读取自己最新提交的日志详情 | `+report-latest` | 无 | `PASS-RPT-LATEST-ZERO`：发件箱为空时在一次列表调用后停止 | `PASS-RPT-DETAIL-SHAPE`：独立详情能力可返回稳定身份与严格字段数组，但没有安全发件箱目标可串联 | unavailable |

## 3. 已验证分页合同

- `PASS-RPT-INBOX-CURSOR`：所有非终止页均返回正整数、相对请求严格前进的 continuation。
- `PASS-RPT-INBOX-PAGES`：exact Shortcut 沿服务端返回 cursor 读取 5 个非空页，聚合数量 92，末页明确 `hasMore=false`。
- `PASS-RPT-INBOX-OVERLAP`：首对相邻非空页的稳定身份交集为 0。
- `PASS-RPT-INBOX-ZERO`：独立有界查询返回合法 0 项、`complete=true` 与 endpoint exhausted。
- 服务端在终止页仍回显整数 cursor；DWS 只把它当页面收据，不发布 `next_token`。错型或冲突 cursor 仍失败。
- DWS 严格拒绝空响应、缺/错型 `success`、缺/错型集合、坏元素、重复身份、缺 continuation、空页续页和不前进 cursor。

## 4. 下游需求明细

### `DS-Report-001` — 提供可回收的发件箱 E2E fixture

#### 用户结果

用户能够可靠列出自己发出的日志，并在完整候选集合中选出最新一篇，再按稳定 `reportId` 读取身份一致的详情。

#### 当前证据与归因

- `PASS-RPT-OUTBOX-ZERO`：多个独立有界窗口均为合法 0 项；这只能证明零结果合同，不能证明非空项目投影。
- `PASS-RPT-LATEST-ZERO`：组合命令遇到空发件箱后停止，没有调用详情，也没有把空结果伪装成成功详情。
- `PASS-RPT-DETAIL-SHAPE`：在独立安全读取中，详情结果具备稳定身份、时间字段与非空字段集合；统计读取也具备明确成功对象。
- 原子层可以创建日志，但没有对应删除/回收能力。仅凭创建成功回执不能满足写后精确读回、清理和零残留要求，因此本轮没有创建测试数据。

#### 所需 fixture / 能力

- 首选：租户提供专用、无业务内容、定期自动过期的已发送日志 fixture，并保证测试身份可读取。
- 或者：新增精确删除/回收能力，使测试可按创建回执中的稳定身份读回并在 `defer` 清理，最后证明发件箱与详情均无残留。
- 非空发件箱响应必须包含布尔 `success=true`、精确集合、每项稳定 `reportId`、正整数 `createTime` 和真实分页终止证据。
- 详情响应必须返回与请求完全一致的稳定身份；字段集合必须为数组，坏元素不能被跳过。
- 若创建执行结果不确定，必须返回 commit-unknown 分类与安全的核查动作，不能建议盲目重试。

#### 验收标准

1. exact `+outbox-list` 同时证明已知非空与保证零项。
2. 非空项目均有稳定身份与排序时间，分页遍历至终止且无重复。
3. exact `+report-latest` 只调用一次完整列表和一次精确详情，详情身份与所选目标一致。
4. 若用创建产生 fixture：确认前 0 次写调用；确认后稳定回执；按回执精确读回；无条件清理；最终证明零残留。
5. 没有可验证清理能力时，不执行创建，并继续保持相关 Shortcut unavailable。

## 5. Lark 对齐与平台差异

lark-cli 1.0.87 没有 Report/日志产品域，因此 4 个 Report Shortcut 均无一对一 Lark 任务。它们是 DWS 的平台原生增量能力，不应虚构映射：

| DWS 用户任务 | Lark 映射 | 推荐结论 |
|---|---|---|
| 搜索日志模板 | 无 | DWS 原生能力；公开 `+template-search` |
| 查看收到的日志 | 无 | DWS 原生能力；公开 `+inbox-list` |
| 查看发出的日志或最新详情 | 无 | DWS 原生能力；非空链路验证前保持 unavailable |

## 6. 无需下游变更的上游修复

| Shortcut | 已完成修复 | 回归证据 |
|---|---|---|
| 全部 4 个 | 统一 Result/Safety/Contract、unified output；空响应、缺/错型 success、缺集合、坏元素与重复身份全部失败 | `PASS-RPT-STRICT-MATRIX` |
| `+inbox-list`、`+outbox-list` | 发布真实 cursor pagination；缺失、冲突、错型和不前进 cursor 全部失败；终止页不发布回显 cursor | `PASS-RPT-PAGINATION-MATRIX` |
| `+template-search` | 严格验证完整模板集合后本地筛选；不从坏元素或未知形状制造空结果 | `PASS-RPT-TEMPLATE-MATRIX` |
| `+report-latest` | 删除原始列表行回退；仅接受完整候选集合、可排序时间和身份匹配详情 | `PASS-RPT-LATEST-READBACK` |

## 7. 安全与脱敏声明

- 本文不包含真实 ID、标题、人名、邮箱、正文、原始响应、trace/request ID、token 或签名 URL。
- 所有真实证据只以 `PASS-RPT-*` 标签和聚合数量表达；不得用这些标签反查或推断具体资源。
- 写能力因缺少可验证清理合同未执行；不存在为了测试而遗留的 Report 数据。
