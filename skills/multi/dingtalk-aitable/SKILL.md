---
name: dingtalk-aitable
description: 钉钉 AI 表格（多维表）。Use when 用户说 AI表格/多维表/数据表/base/table/建表/查记录/写数据/字段/记录增删改查/筛选/排序/公式/模板搜索/批量导入CSV或JSON/导出/仪表盘/图表/上传附件到表格/按字段类型建表。不做电子表格单元格读写（走 dingtalk-misc）、文档编辑（走 dingtalk-doc）；听记待办入表先用 dingtalk-minutes 提取，再由本 skill 写入。命令前缀：dws aitable。
metadata:
  cli_version: ">=0.2.14"
  category: product
  requires:
    bins:
      - dws
---

# 钉钉 AI 表格 Skill

<!-- DWS_RUNTIME_CONTRACT_START -->
## 最小 DWS 执行契约

- 只通过 `dws` CLI 操作钉钉；结构化读取使用 `--format json`，按真实返回判断结果。
- 已知命令直接执行。只有 leaf 参数或安全语义不确定时读取精确 Schema，只有 Cobra flag 不确定时读取精确 leaf Help；不要加载产品级 Catalog 代替选路。
- 不猜命令、flag、字段、ID、账号或时间。后续 ID 必须来自真实返回；零命中、多候选或类型不明时停止并消歧。
- 解析目标、读取上下文和最终执行必须使用同一 profile；不得跨组织复用 userId、openDingTalkId 或 openConversationId。多账号组织只使用明确的 `isOrgCurrent=true` 默认账号；没有默认账号时要求用户指定，禁止选择第一项、最近登录或最近使用账号。
- 不输出或记录 token、refresh token、appSecret、webhook token 等凭据；宿主已注入认证时不要索要凭据。
- 写操作必须符合用户明确意图。是否需要确认以最终 Runtime gate 和 Schema 为准；需要确认时先说明对象、动作与影响，再追加 `--yes`。
- 写后按任务结果契约验证；不能仅凭退出码宣称成功。部分结果、未知投递状态和失败项必须如实保留。
- 时间戳面向用户展示时转换为带时区的可读时间；默认使用当前会话时区，必要时同时保留原值。
- 遇到认证、权限、profile、confirmation 或未知错误时，只加载 `dingtalk-shared` 中对应 reference；不要连续猜测替代命令。
<!-- DWS_RUNTIME_CONTRACT_END -->

<!-- VISIBLE_SHORTCUTS_START -->
## Shortcut 发现（按需）

`aitable` 当前有 93 条公开 shortcut，完整清单保留在 Runtime Catalog 与 Schema，不在高频产品根 Skill 中重复展开。已知意图直接使用下方的优先路由、意图表或任务 reference；命令已选中时直接执行，只在参数/安全语义不确定时读取 leaf Schema，在当前 Cobra flags 不确定时读取 leaf Help。

仅当现有路由和 reference 都无法定位低频能力时，才执行 `dws shortcut list --service aitable --format json` 做最后回退；不要为已知高频意图加载完整 Shortcut Catalog 或产品级 Schema。
<!-- VISIBLE_SHORTCUTS_END -->

## Golden Route

已有 ID 直接使用；完整 URL 先解析；名称先唯一解析为稳定 ID。零命中或多候选时停止，不默认选第一项。

| 用户意图 | 唯一推荐入口 | 关键边界 |
|---|---|---|
| 从 URL 解析稳定 ID | `dws aitable +url-resolve --url <URL>` | 只解析 URL 中已有的 baseId/tableId/viewId/recordId，不做远端名称搜索 |
| 按名称定位 Base/Table | `dws aitable +resolve-base --name <名称>` → `dws aitable +resolve-table --base <ID> --name <表名>` | 默认精确匹配；只有用户明确接受模糊匹配时才加 `--fuzzy` |
| 浏览 Base 下的数据表 | `dws aitable +list-tables --base <ID>` | 只返回 tableId/tableName，不加载字段 |
| 搜索 Base | `dws aitable +base-search --query <关键词>` | 单次返回；不要将 `+base-list`（最近访问）声称为全量目录 |
| 新建 Base 与整套表字段 | `+base-bootstrap` | 高频准确 flags 与 JSON 形状见唯一执行 Reference；不要先调用 Help |
| 已有 Base 新建一张表与字段 | `+table-bootstrap` | 自动按 15 个字段分片并读回验证；替代 `table create` 后连续 `field create` |
| 查询、筛选、排序或分页 | `dws aitable +record-query --base-id <ID> --table-id <ID>` | 单页/精确 ID 优先；明确要求全量时改用原子 `record query --all --page-limit <N>` |
| 新增单条或批量记录 | `dws aitable record create --base-id <ID> --table-id <ID> --records <JSON>` | 当前无 `+record-create`；写前取字段定义，写后按新 ID 回读 |
| 更新已知 recordId | `dws aitable +record-update --base-id <ID> --table-id <ID> --records <JSON>` | 自动分片并读回；只传需修改字段 |
| 按业务唯一键同步 | `dws aitable +record-upsert-by-key --base-id <ID> --table-id <ID> --key-field-id <ID> --key-value <值> --cells <JSON>` | 0 条创建、1 条更新、多条停止；非字符串键改用 `--key-value-json` |
| 按条件批量修改 | `dws aitable +record-bulk-patch --base-id <ID> --table-id <ID> --query <关键词> --patch <JSON> --max-matches <N>` | 也可用 filters/record-ids 选范围；禁止无边界整表写 |
| 删除整个 Base | `dws aitable +base-delete --base-id <ID>` | 先通过只读命令确认真实 ID；按 Runtime confirmation 执行，不用 Drive 删除同名节点 |
| 删除字段 | `dws aitable +field-delete --base-id <ID> --table-id <ID> --field-id <ID>` | 先读取字段目录并确认非主字段；按 Runtime confirmation 执行 |
| 查询/创建记录主键文档 | `dws aitable +record-primary-doc-get|+record-primary-doc-create ...` | create 必须传 primaryDoc 类型的 `--field-id`；正文操作切到 Doc |
| 创建 View / Dashboard / Chart 或导入文件 | 对应 leaf / `+import-*` | 高频准确 flags 见唯一执行 Reference；已有参数直接执行，不调用 Help |
| 调整视图列顺序 | `dws aitable view update visible-fields --base-id <ID> --table-id <ID> --view-id <ID> --field-ids <完整有序IDs>` | 先读取字段和当前完整列数组，固定主字段在首位，写后回读精确校验 |
| 创建/修改图表前取配置 | `dws aitable +chart-widgets-example` | 命令返回所有图表类型示例；已有合法 config 时直接 create/update |
| Base 内 Section/节点移动 | `dws aitable +section-*` | Table/Dashboard/Section 是 Base 内 nsheet 节点，不是独立 Drive 节点 |

### 低频入口

字段配置用 `+field-*`；删记录用 `+record-delete`；附件用 `+attachment-*`。批量分享记录用 `+record-share-links --base <B> --table <T> --record-ids <IDs>`。其余能力使用同名前缀 leaf。

## 当前最短路径

- 已有 ID 直接使用；URL 只解析一次；名称用 `+resolve-base` / `+resolve-table` 唯一解析。filters/sort 缺 fieldId 时才读取字段目录。
- 高频 Base/Table/Field/View/Dashboard/Chart/Import 先读一次 [执行 Reference](references/aitable.md)，其中已有准确 flags；不要再调用 Help。
- Shortcut 已含分片或验证时不重复拆步；已有 Base 新建完整表结构直接用 `+table-bootstrap`。
- 预计不超过 5 条命令的任务直接执行，不创建 TodoWrite/任务计划；复杂任务也只保留必要检查点。

## 记录输入与结果

- `cells` key 用当前 fieldId；大 JSON 用相对 `--records-file`。filters 顶层为 `and|or`，sort 使用 `direction`；复杂条件读 [filter-sort](references/aitable/aitable-filter-sort.md)。
- 按真实字段类型写值，只读字段不得写入。
- 新建从 `data.newRecordIds[]` 取 ID，再用 `+record-query --record-ids` 回读。
- 批量结果检查 completed/failed、verification、checkpoint；`partial_success` 不是完成。全量查询使用原子 `record query --all` 并检查 `hasMore`；只有 `hasMore=false`，或按指定 ID 全命中时，才声称结果完整。
- 写入效果未知时回读，不重放成功批次。

## 安全边界

- 删除不可逆，按 Runtime confirmation 核对真实目标；`base list` 只是最近访问。字段零/多候选、类型不明时停止；多批写保留已完成批次和续跑位置。

## 按需加载

Golden Route 参数足够时不读 reference；否则最多读取一个：

| 触发条件 | Reference |
|---|---|
| 高频 Golden Route 的准确 flags、JSON 形状与错误恢复 | [执行 Reference](references/aitable.md) |
| 记录 CRUD、字段值格式 | [record-ops](references/aitable-record-ops.md) |
| 记录主键文档 | [primary-doc](references/aitable/aitable-primary-doc.md) |
| filters/sort/date 操作符 | [filter-sort](references/aitable/aitable-filter-sort.md) |
| 字段创建或复杂配置 | [field](references/aitable/aitable-field.md) |
| 导入导出任务恢复 | [export-import](references/aitable/aitable-export-import.md) |
| 视图列顺序、筛选、排序、冻结 | [view-config](references/aitable/aitable-view-config.md) |
| Base 内 Section/节点移动或清理 | [section](references/aitable-section.md) |
| 图表配置 | [dashboard-chart](references/aitable/aitable-dashboard-chart.md) |
| 附件、表单、工作流 | 读取 `references/aitable/` 下对应的一个精确文件 |
| 产品边界不明确 | [intent-guide](references/intent-guide.md) |

不连读 reference；低频能力按意图加载。

## 错误最短路径

1. 零/多候选、字段歧义或分页不完整：停止并返回证据；需要后续页时只透传真实 `nextCursor`。
2. 类型错误只复核目标字段，不删字段或丢输入；`partial_success` 从 checkpoint 续跑，未知写入先回读。
3. 错误包含 `actions` / `available_flags` 时只执行其中的 `next_command`；`retryable=false` 时停止，不换猜测 ID 或相似 flag 试参。

## 跨产品边界

- Excel 式单元格、区域和公式操作 → `dingtalk-misc` 的 Sheet。
- Base 作为整体在普通文件夹间移动或做外层存储重命名 → Drive；Base 结构复制/删除，以及 Base 内 Table、Dashboard、Section 的创建、复制、移动、重命名、删除 → AITable。
- 记录主键文档正文 → 取得真实 nodeId 后切 `dingtalk-doc`。
