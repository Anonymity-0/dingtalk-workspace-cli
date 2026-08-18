# Attendance Shortcut 下游业务能力需求规格

> 日期：2026-08-18
> DWS 基线：`effde762277ad717a246e38b237a8297fa49aab7`
> 对比基线：Lark CLI 1.0.87
> 范围：Attendance Shortcut only；不修改 DWS 产品 Skill 或 multi Skill。

## 1. 执行摘要

- Attendance 共审核 35 个源码 Shortcut；17 个具备公开条件，18 个保持 hidden/unavailable。
- `+check-result` 已覆盖 Lark CLI 当前唯一 Attendance 用户任务 `attendance user_tasks query`，并提供打卡流水、审批、排班、班次、规则、设置、假期和个人视图等更宽能力。
- 已确认 4 组下游需求：补卡规则详情返回空结果、报表合同不足、打卡结果分页缺少确定终止证据、缺少安全可回收的管理员/写操作 fixture。
- 班次详情曾因未公开而缺失，现已通过“搜索非空结果 → 同一稳定 ID 详情”闭环恢复；严格响应校验、统一 Result 和空结果判定均属于上游修复，不要求下游改动。

| ID | 优先级 | 类型 | 用户任务 | 当前状态 | 建议 Owner | 解锁的 Shortcut |
|---|---|---|---|---|---|---|
| `DS-ATTENDANCE-001` | P1 | business-service defect / contract insufficient | 搜索后读取补卡规则详情 | unavailable | Attendance Wukong 规则服务 | `+get-adjustment-rule` |
| `DS-ATTENDANCE-002` | P1 | business-service defect / contract insufficient | 发现报表列并查询考勤/假期报表 | unavailable | Attendance 报表服务 / MCP adapter | `+list-report-columns`, `+query-report-data`, `+query-report-leave` |
| `DS-ATTENDANCE-003` | P2 | contract insufficient | 可靠翻完打卡结果 | partial | Attendance 打卡查询服务 | `+check-result` 完整分页 |
| `DS-ATTENDANCE-004` | P1 | tenant-or-fixture / permission | 验证考勤组、全局设置、余额和写操作 | blocked / unavailable | Attendance 产品测试基础设施 / 权限 Owner | 14 个读写 Shortcut |

## 2. 用户任务与能力缺口总览

| 用户任务 / Golden Route | DWS Shortcut | Lark CLI 对应 | 当前能力 | 缺口分类 | 临时处置 |
|---|---|---|---|---|---|
| 批量查询员工打卡结果 | `attendance +check-result` | `attendance user_tasks query` | covered；分页终止为保守推断 | contract insufficient | 返回 `complete`/`nextOffset`，不声明框架 cursor 分页 |
| 搜索并读取班次 | `+search-class` → `+get-class` | 无同级入口 | covered | 无 | 公开，严格身份闭环 |
| 搜索并读取补卡规则 | `+search-adjustment-rule` → `+get-adjustment-rule` | 无同级入口 | partial | business-service defect | 只公开搜索；详情 unavailable |
| 发现字段并查询考勤报表 | `+list-report-columns` → `+query-report-data` | 无同级入口 | unavailable | contract insufficient | 两个入口均保持 hidden/unavailable |
| 查询假期报表 | `+query-report-leave` | 无同级入口 | unavailable | business-service defect | hidden/unavailable |
| 搜索并读取考勤组 | `+search-group` → `+get-group` | 无同级入口 | blocked | tenant-or-fixture | 无已知非空安全 fixture，全部 hidden |
| 查询企业全局设置和假期余额 | `+get-global-setting`, `+get-leave-balance` | 无同级入口 | blocked | permission / fixture | hidden/unavailable |
| 修改排班、班次、考勤组、假期和打卡结果 | 9 个写 Shortcut | 无同级入口 | unsafe to verify | tenant-or-fixture / contract insufficient | hidden/unavailable，不以 dry-run 记通过 |

## 3. 下游需求明细

### `DS-ATTENDANCE-001` — 让搜索得到的补卡规则可被稳定读取

#### A. 用户任务与现状

- 用户任务：先按名称浏览补卡规则，再用结果中的稳定主键读取完整规则。
- canonical Shortcut：`attendance +search-adjustment-rule`、`attendance +get-adjustment-rule`。
- atomic/raw route：`attendance adjustment search`、`attendance adjustment get`。
- Exact Shortcut 与 atomic/raw 均使用搜索返回的同一候选主键；搜索明确成功且非空，详情调用明确 `success=true`，但 `result=null`。
- 已排除上游空数组投影、整数解析和候选字段遗漏：多个可作为候选的数值字段均未得到非空详情；加班规则的相邻搜索→详情闭环正常。
- 置信度：高。仍需下游确认“搜索 ID 与详情 ID 不同”还是详情服务未返回对象。
- 安全证据句柄：`ATT-DETAIL-NULL-01`；仓库不保存 raw body、资源 ID 或 trace。

#### B. 需要下游提供的合同

- 明确 `get_adjustment_rule` 列表项中哪个字段是 `get_adjustment_rule_detail.adjustmentId` 的稳定主键；名称和类型必须在 Schema 中一致。
- 对存在且有权限的规则返回 `success=true` 和非空对象 `result`，对象必须回显同一稳定规则 ID。
- 对不存在、已删除、无权限、租户未开通分别返回稳定的 typed error；不得以 `success=true + result=null` 表示任一失败。
- 如详情接口不受支持，提供可发现的 capability/feature 状态，或在搜索结果中返回足以完成详情任务的完整对象并声明字段稳定性。
- 改动应 additive/versioned；旧字段保留兼容期，禁止静默改变现有 ID 的语义。

#### C. 验收标准

1. 创建或选择隔离规则，atomic search 非空并取得稳定 ID。
2. atomic detail 和 exact `+get-adjustment-rule` 均返回同一 ID 的非空对象。
3. 不存在 ID、无权限和已删除 ID 分别返回非零 typed error。
4. 上游恢复公开后，搜索→详情 E2E 通过且仓库/远端无测试残留。

#### D. 临时处置

`+get-adjustment-rule` 保持 hidden/unavailable 并从 Agent Schema 排除；`+search-adjustment-rule` 不再承诺详情入口可用。

### `DS-ATTENDANCE-002` — 提供可发现、可验证的考勤报表合同

#### A. 用户任务与现状

- Golden Route：列出企业可查询报表列 → 选择稳定列 ID → 查询一批员工的列值；另一路径按假期类型查询时长报表。
- canonical Shortcut：`+list-report-columns`、`+query-report-data`、`+query-report-leave`。
- atomic/raw operations：`get_report_columns`、`get_report_columns_value`、`get_leave_time_by_leave_names`。
- 观察：列发现与假期报表调用均退出码 0 且 payload 为 JSON `null`；使用未经验证的列 ID 查询列值仅得到显式空数组，不能证明列 ID 有效或查询正确。
- 已排除上游投影丢失：原子调用本身即返回 `null`；Shortcut 现已拒绝把 `null` 当作合法空集合。
- 置信度：高。权限/租户功能可能是触发条件，但接口没有返回可区分的状态。
- 安全证据句柄：`ATT-REPORT-NULL-01`。

#### B. 需要下游提供的合同

- `get_report_columns`：成功时必须返回显式列数组；每项含稳定 `columnId`、显示名、值类型、单位、支持的日期/人员范围和是否需要管理员权限。
- 合法无列必须是 `success=true + result=[]`；未开通、无权限和服务异常必须是不同 typed error，不得返回裸 `null`。
- `get_report_columns_value`：返回值必须绑定请求的用户集合、列 ID 和时间范围；未知列返回 `COLUMN_NOT_FOUND`，不能静默得到空数组。
- `get_leave_time_by_leave_names`：返回显式数组并包含稳定用户身份、假期类型标识、单位和数值；合法零记录为显式空数组。
- 列值和假期报表若分页，必须提供 page/cursor、hasMore 和终止证据；批量用户存在部分失败时返回逐项 ledger 与整体 partial status。
- 提供安全 capability discovery：租户是否开通、调用身份所需权限、最大用户数、最大列数、最大时间跨度。

#### C. 验收标准

1. 管理员测试租户中列发现有已知非空和明确空租户两组 E2E。
2. 使用发现的同一 `columnId` 执行 atomic 与 exact Shortcut，返回与请求用户/区间绑定的非空值。
3. 未知列、无权限、未开通和超范围分别产生稳定非零错误。
4. 假期报表至少覆盖已知非空、合法空和未知假期类型。
5. 分页/partial 分支和远端零残留通过。

#### D. 临时处置

三个报表 Shortcut 均保持 hidden/unavailable；不得用 `null`、请求 echo 或未验证列产生的空数组标记 PASS。

### `DS-ATTENDANCE-003` — 为打卡结果提供确定的分页终止证据

#### A. 用户任务与现状

- `+check-result` 已真实返回非空打卡结果并覆盖 Lark 任务；当前接口只接受 `offset/limit`，响应缺少稳定总量、hasMore 或 nextOffset。
- DWS 只能在返回条数小于 limit 时证明结束；满页时保守输出 `complete=false` 和建议 nextOffset，不能声明全量完成。
- 安全证据句柄：`ATT-CHECK-PAGE-01`。

#### B. 需要下游提供的合同

- 响应增加 `hasMore` 与 `nextOffset`，或 `totalCount`；这些字段必须与同一快照/排序一致。
- 固定稳定排序键和同 offset 重放语义；说明并发新增/修改是否可能造成重复或漏项。
- 空页且 `hasMore=true` 必须仍给出前进 token/offset；重复或倒退 offset 为协议错误。
- 声明最大 limit、最大时间跨度和超过上限的 typed validation error。

#### C. 验收标准与临时处置

- 验收覆盖多页、最后一页、零记录、满页但仍有下一页、重复 token/offset 和并发变更。
- 下游完成前，DWS 保留数据层 `complete/nextOffset`，不伪造 cursor PaginationSpec，也不把满页当完整结果。

### `DS-ATTENDANCE-004` — 建立可回收的 Attendance 管理员与写操作测试资源

#### A. 用户任务与现状

- 受影响读取：`+search-group`、`+get-group`、`+get-group-filtered`、`+get-global-setting`、`+get-leave-balance`。
- 受影响写入：`+import-schedule`、`+create-class`、`+update-class`、`+update-group-members`、`+create-group`、`+update-group`、`+update-leave-type`、`+save-leave-balance`、`+boss-check`。
- 当前安全身份没有已知非空考勤组 fixture；全局设置被权限拒绝；余额读取没有可验证结果。写操作会影响真实员工规则，且部分资源缺删除/恢复能力，因此未执行生产数据写入。
- 这不是对业务接口必然有 bug 的结论，而是可测试性和权限前置不足。
- 安全证据句柄：`ATT-FIXTURE-GAP-01`。

#### B. 需要的测试基础设施与合同

- 提供隔离租户或专用测试组织，包含：管理员测试身份、两个无业务含义测试成员、一个可删除考勤组、一个可删除班次、一个可恢复假期类型、可控排班与打卡结果。
- 只授予完成相应接口所需的最小 scopes；提供 capability discovery，区分权限不足、功能未开通和资源不存在。
- 写接口返回稳定资源 ID、逐项结果、幂等/commit-unknown 语义；所有更新支持精确读回。
- 为不可删除的企业设置提供 snapshot/restore 或专用 reset API；余额和 BOSS 改签必须能恢复原值。
- Fixture 有 TTL、Owner 和自动清理告警；日志只保留受控 evidence handle，不输出业务内容或身份值。

#### C. 验收标准与临时处置

1. 考勤组搜索有已知非空和保证零命中；详情绑定同一 ID。
2. create→get→update→restore/delete 覆盖班次、考勤组与排班。
3. 成员、余额和打卡结果写入均有 before/after 精确读回并恢复原值。
4. 未确认时远程写调用为 0；任一 partial/commit-unknown 非零退出。
5. 测试结束远端和本地均零残留。

在完整 fixture 到位前，相关 Shortcut 保持 hidden/unavailable。

## 4. Lark 对齐与平台差异

| Lark 用户任务 | 所需下游能力 | 可精确对齐 | 平台差异 | DWS 推荐结论 |
|---|---|---|---|---|
| `attendance user_tasks query` 查询打卡结果 | 现有 `query_check_result`；最好补分页终止证据 | yes，分页完整性 partial | Lark 当前没有同级的排班、规则、报表和企业设置任务 | 保留 `+check-result` 为主对齐入口，报告分页边界 |

无法对齐的不是 DWS 缺入口，而是部分钉钉管理面缺少可验证下游合同或安全 fixture；不能为追求同名率伪造成功。

## 5. 超越 Lark 的产品机会

| 产品原生能力 | 所需下游支持 | 可形成的 DWS Shortcut | 安全/验证要求 | 优先级 |
|---|---|---|---|---|
| 异常考勤处置队列 | 稳定异常记录 ID、原因、关联审批、处理状态、分页和可恢复更正 | `attendance +exceptions` / `+resolve-exception` | 读写分离；更正确认；写后同 ID 终态读回；可恢复 | P2 |
| 跨员工考勤汇总 | 可按组织/成员批量聚合迟到、缺卡、加班、请假并给出统计口径版本 | `attendance +team-summary` | 最小权限、聚合脱敏、口径版本、分页完整性 | P2 |
| 规则影响预览 | 更新班次/考勤组/假期前返回受影响成员与日期范围，不提交写入 | `attendance +rule-impact-preview` | 只读、稳定影响计数、无副作用、与最终写请求同参数语义 | P1 |

## 6. 无需下游变更的上游修复

| Shortcut | 上游根因 | 已完成修复 | 回归证据 |
|---|---|---|---|
| 所有公开 Attendance 集合查询 | 容错 projector 可能把缺字段、错型或坏元素投成 `[]` | 共享严格 success/result/collection 校验；显式空数组才合法 | 单元负向矩阵；真实非空与零命中 E2E |
| `+search-class`, `+search-adjustment-rule`, `+search-overtime-rule` | 嵌套 `shiftVO/entityVO` 导致身份投影风险 | 固定审核路径、展开 wrapper、要求稳定 ID、公开分页完成证据 | 非空搜索、零命中、详情闭环和坏 item 回归 |
| `+get-class` | 能力存在但未公开、无 Result/严格读回 | 补 Contract/Safety/Result，搜索 ID 精确读回并恢复公开 | exact live E2E |
| `+my-attendance`, `+this-month` | 直接透传 raw data，缺严格空结果边界 | 统一结果 envelope 并复用严格打卡流水校验 | exact live E2E 与 contract gate |

## 7. 安全与脱敏声明

- 本文不含真实用户、组织、租户、profile、规则、排班、考勤组或打卡记录 ID。
- 本文不含 trace/request ID、token、签名 URL、邮箱、电话、业务标题正文或真实日程内容。
- Raw 响应仅在仓库外临时目录中处理并已删除；本文只保留不可反查的证据句柄和聚合事实。
- 进入 Git 前必须扫描最终树、未跟踪文件和 `origin/main..HEAD` 全部历史。
