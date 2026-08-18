---
name: dingtalk-aitable
description: 钉钉 AI 表格（多维表）。Use when 用户说 AI表格/多维表/数据表/base/table/建表/查记录/写数据/字段/记录增删改查/筛选/排序/公式/模板搜索/批量导入CSV或JSON/导出/仪表盘/图表/上传附件到表格/按字段类型建表/数据源/创建数据源/更新数据源配置/触发数据源同步/查询同步状态/获取数据源配置/审批数据同步。不做电子表格单元格读写（走 dingtalk-misc）、文档编辑（走 dingtalk-doc）；听记待办入表先用 dingtalk-minutes 提取，再由本 skill 写入。命令前缀：dws aitable。
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
- 写操作必须符合用户明确意图。是否需要确认以最终 Runtime gate 和 Schema 为准；本轮用户已明确要求执行、目标与影响无歧义的非破坏性写操作时，该明确指令就是本次确认，首次调用直接携带 Runtime 所需的 `--yes`，不先制造 `confirmation_required`。删除、停用自动化等破坏性或高风险动作仍须先说明对象、动作与影响并取得独立确认。
- 写后按任务结果契约验证；不能仅凭退出码宣称成功。部分结果、未知投递状态和失败项必须如实保留。
- 时间戳面向用户展示时转换为带时区的可读时间；默认使用当前会话时区，必要时同时保留原值。
- 遇到认证、权限、profile、confirmation 或未知错误时，只加载 `dingtalk-shared` 中对应 reference；不要连续猜测替代命令。
<!-- DWS_RUNTIME_CONTRACT_END -->

<!-- VISIBLE_SHORTCUTS_START -->
## Shortcut 发现（按需）

`aitable` 当前有 93 条公开 shortcut，完整清单保留在 Runtime Catalog 与 Schema，不在高频产品根 Skill 中重复展开。已知意图直接使用下方的优先路由、意图表或任务 reference；命令已选中时直接执行，只在参数/安全语义不确定时读取 leaf Schema，在当前 Cobra flags 不确定时读取 leaf Help。

| Shortcut | 风险 | 适用场景 |
|---|---|---|
| `dws aitable +advperm-disable` | high-risk-write | 关闭指定 Base 的高级权限总开关（所有自定义角色失效） |
| `dws aitable +advperm-enable` | write | 开启指定 Base 的高级权限总开关 |
| `dws aitable +attachment-put` | write | 准备凭证、实际 PUT 本地文件、写入 attachment 单元格并读回验证 |
| `dws aitable +attachment-remove` | high-risk-write | 从 attachment 字段清空全部或按文件名移除，写前确保剩余项具有可重写 fileToken，并读回验证 |
| `dws aitable +attachment-upload` | write | 为 attachment 字段申请 OSS 直传地址（uploadUrl / fileToken） |
| `dws aitable +base-bootstrap` | write | 一次创建 Base、数据表和字段，逐层读回验证并在中断时报告已知副作用 |
| `dws aitable +base-copy` | write | 复制 AI 表格到指定目录（可仅复制结构） |
| `dws aitable +base-delete` | high-risk-write | 删除指定 Base（不可逆） |
| `dws aitable +base-get` | read | 获取指定 Base 的目录信息（tables / dashboards summary） |
| `dws aitable +base-get-primary-doc-id` | read | 根据 baseId/tableId/recordId 获取主键文档的 dentryUuid |
| `dws aitable +base-list` | read | 获取当前用户可访问的 AI 表格 Base 列表（最近访问，支持游标分页） |
| `dws aitable +base-schema-snapshot` | read | 读取 Base、全部数据表、字段和视图的可复用结构快照，并严格校验每层响应 |
| `dws aitable +base-search` | read | 按名称关键词搜索 AI 表格 Base |
| `dws aitable +base-update` | write | 更新 Base 名称（可选备注） |
| `dws aitable +chart-delete` | high-risk-write | 删除指定 chart 及其布局项（不可逆） |
| `dws aitable +chart-get` | read | 获取指定 chart 的详细信息 |
| `dws aitable +chart-share-get` | read | 查询 chart 的分享配置 |
| `dws aitable +chart-share-update` | write | 开启/关闭 chart 分享并可设置分享类型 |
| `dws aitable +chart-update` | write | 更新指定 chart 的配置或布局（--config 必填） |
| `dws aitable +chart-widgets-example` | read | 获取所有图表类型的 widget config 示例 |
| `dws aitable +dashboard-arrange` | write | 对指定仪表盘做服务端智能布局重排 |
| `dws aitable +dashboard-config-example` | read | 获取 dashboard config 的结构示例 |
| `dws aitable +dashboard-delete` | high-risk-write | 删除指定 dashboard（级联删除其 chart，不可逆） |
| `dws aitable +dashboard-get` | read | 获取指定 dashboard 的详细信息（含 charts summary） |
| `dws aitable +dashboard-share-get` | read | 查询 dashboard 的分享配置 |
| `dws aitable +dashboard-share-update` | write | 开启/关闭 dashboard 分享并可设置分享类型 |
| `dws aitable +dashboard-update` | write | 更新指定 dashboard 的配置 |
| `dws aitable +datasource-create` | write | 为指定 AI 表格创建数据源同步配置并触发首次同步 |
| `dws aitable +datasource-get-config` | read | 获取数据源表的同步配置信息 |
| `dws aitable +datasource-sync` | write | 对已有数据源表触发一次手动同步（单次最多 5 张表） |
| `dws aitable +datasource-sync-status` | read | 查询数据源表的同步任务状态（支持批量查询） |
| `dws aitable +datasource-update` | write | 更新已有数据源表的同步配置并触发同步 |
| `dws aitable +datasource-get-fields` | read | 获取指定数据源下可供同步的字段列表（用于决定 field-ids） |
| `dws aitable +datasource-list-sources` | read | 列出指定 Base 下可用的数据源条目（OA 审批模板等） |
| `dws aitable +export-data` | read | 导出 AI 表格数据（创建导出任务或按 taskId 续等） |
| `dws aitable +field-delete` | high-risk-write | 删除指定字段（不可逆） |
| `dws aitable +field-get` | read | 批量获取字段详情（含类型相关完整配置） |
| `dws aitable +field-update` | write | 更新字段名称 / 配置 / AI 配置（类型不可改） |
| `dws aitable +find-record` | read | 在指定多维表里按关键词查记录（只读） |
| `dws aitable +form-delete` | high-risk-write | 删除指定表单视图（不可逆） |
| `dws aitable +form-field-hide` | write | 切换表单字段的隐藏/显示状态 |
| `dws aitable +form-field-list` | read | 列出表单视图当前可见的字段及其配置 |
| `dws aitable +form-field-update` | write | 更新表单字段的必填状态或描述 |
| `dws aitable +form-list` | read | 列出指定数据表下的所有表单视图 |
| `dws aitable +form-share-get` | read | 读取视图当前的分享表单配置 |
| `dws aitable +form-share-update` | write | 开启或关闭指定视图的分享表单 |
| `dws aitable +form-update` | write | 更新表单标题 / 描述 |
| `dws aitable +import-data` | write | 将已上传文件导入 AI 表格（新建表或追加到已有表） |
| `dws aitable +import-upload` | write | 为导入任务申请 OSS 直传地址（uploadUrl / importId） |
| `dws aitable +list-tables` | read | 列出某个多维表(base)里的所有数据表（只读，投影 tableId/tableName） |
| `dws aitable +record-bulk-patch` | high-risk-write | 完整查询目标记录后批量合并同一组 cells，自动分片并逐条读回验证 |
| `dws aitable +record-delete` | high-risk-write | 批量删除记录（不可逆），自动按 100 条分片并逐批确认记录已不存在 |
| `dws aitable +record-history-list` | read | 按 recordId 查询单条记录的变更历史 |
| `dws aitable +record-primary-doc-create` | write | 为记录创建主键文档（幂等），fieldId 须为 primaryDoc 类型 |
| `dws aitable +record-primary-doc-get` | read | 查询记录关联的主键文档 nodeId |
| `dws aitable +record-query` | read | 查询表格记录（按 ID 取 / 条件筛选 / 关键词 / 分页） |
| `dws aitable +record-query-empty` | read | 扫描并过滤出完全没填用户字段的空行 |
| `dws aitable +record-share-links` | read | 批量（可 >20 条）获取多维表记录分享链接：去重+分片+合并 |
| `dws aitable +record-share-url` | read | 按 recordId 批量获取记录分享链接，单次最多 20 条 |
| `dws aitable +record-update` | write | 批量更新记录，自动按 100 条分片并逐批读回验证 |
| `dws aitable +record-upsert` | write | 按 recordId 自动拆分 create/update，按 100 条分片并读回验证 |
| `dws aitable +record-upsert-by-key` | write | 按唯一字段值有则更新、无则创建记录，并读回验证 |
| `dws aitable +resolve-base` | read | 按名称搜索多维表 Base 并解析出唯一 baseId（只读） |
| `dws aitable +resolve-table` | read | 在某个多维表 Base 内按名称解析出唯一的数据表 tableId（只读） |
| `dws aitable +role-create` | write | 在指定 Base 下创建自定义角色 |
| `dws aitable +role-delete` | high-risk-write | 删除 Base 下指定的自定义角色（不可逆） |
| `dws aitable +role-get` | read | 获取单个角色的完整配置 |
| `dws aitable +role-list` | read | 列出指定 Base 下的全部角色 |
| `dws aitable +role-update` | write | 按 PATCH 语义增量更新自定义角色 |
| `dws aitable +section-create` | write | 在指定 Base 下创建文件夹（组织 table / dashboard） |
| `dws aitable +section-delete` | high-risk-write | 删除指定文件夹（不可逆） |
| `dws aitable +section-list-empty` | read | 列出指定 Base 下所有没有子节点的空文件夹 |
| `dws aitable +section-list-nodes` | read | 列出指定 Base 当前版本下的全部 nsheet 节点 |
| `dws aitable +section-move-node` | write | 把任意 nsheet 节点移动到目标文件夹下（可选调整位置） |
| `dws aitable +section-rename` | write | 重命名指定文件夹 |
| `dws aitable +section-reorder` | write | 在当前父文件夹下调整文件夹的展示顺序 |
| `dws aitable +table-copy` | write | 跨 Base 同步复制一张表的可创建字段结构，并可同步复制全部记录 |
| `dws aitable +table-delete` | high-risk-write | 删除指定数据表（不可逆） |
| `dws aitable +table-get` | read | 批量获取指定数据表的表级信息、字段目录与视图目录 |
| `dws aitable +table-update` | write | 更新数据表名称 / 备注 / 行命名规则 |
| `dws aitable +template-search` | read | 按名称关键词搜索 AI 表格模板 |
| `dws aitable +url-resolve` | read | 解析 AI 表格 URL 中的 baseId/tableId/viewId/recordId |
| `dws aitable +view-delete` | high-risk-write | 删除指定视图（不可逆） |
| `dws aitable +view-duplicate` | write | 复制视图，生成配置相同的新视图 |
| `dws aitable +view-get` | read | 获取视图完整信息（列顺序、筛选、排序、分组等） |
| `dws aitable +view-get-frozen-cols` | read | 获取视图当前冻结的左侧列数 |
| `dws aitable +view-get-lock` | read | 获取视图锁定状态 |
| `dws aitable +view-get-row-height` | read | 获取视图单元格行高（像素） |
| `dws aitable +view-lock` | write | 锁定视图（默认）或解锁（--off） |
| `dws aitable +view-preset-apply` | write | 按视图精确名称幂等创建或更新预设，并读回校验类型和 config |
| `dws aitable +view-set-fill-color-rule` | write | 全量覆盖 Grid 视图的条件填色规则（传 '[]' 清空） |
| `dws aitable +view-set-frozen-cols` | write | 设置视图冻结列数（0 表示取消冻结） |
| `dws aitable +view-set-row-height` | write | 设置视图单元格行高（像素，合法档位 32/56/88/128） |
| `dws aitable +view-update` | write | 更新视图名称 / 描述 / 配置（visibleFieldIds、filter、sort、group 等） |
| `dws aitable +workflow-deploy` | write | 创建或更新完整 workflow-dsl/v1，强制检查 valid/flowId，并可启用后验证 RUNNING 状态 |
| `dws aitable +workflow-disable` | high-risk-write | 禁用指定 Base 中的自动化工作流（影响业务自动化） |
| `dws aitable +workflow-enable` | write | 启用指定 Base 中的自动化工作流 |
| `dws aitable +workflow-get` | read | 获取单个自动化工作流的详细信息 |
| `dws aitable +workflow-list` | read | 列出指定 Base 中的自动化工作流（分页） |
<!-- VISIBLE_SHORTCUTS_END -->

## Golden Route

已有 ID 直接使用；完整 URL 先解析；名称先唯一解析为稳定 ID。零命中或多候选时停止，不默认选第一项。

| 用户意图 | 唯一推荐入口 | 关键边界 |
|---|---|---|
| 从 URL 解析稳定 ID | `dws aitable +url-resolve --url <URL>` | 只解析 URL 中已有的 baseId/tableId/viewId/recordId，不做远端名称搜索 |
| 按名称唯一定位并操作 Base/Table | `dws aitable +resolve-base --name <名称>` → `dws aitable +resolve-table --base <ID> --name <表名>` | 默认精确匹配；只有用户明确接受模糊匹配时才加 `--fuzzy` |
| 浏览 Base 下的数据表 | `dws aitable +list-tables --base <ID>` | 只返回 tableId/tableName，不加载字段 |
| 搜索 Base 候选或检查是否存在 | `dws aitable +base-search --query <关键词>` | 用户说“搜索/找一下/候选/如果没有就创建”时直接走本入口，不先调用 `+resolve-base`；AITable 上下文中的 Base 名称不得路由到 `dws aisearch person` |
| 新建 Base 与整套表字段 | `dws aitable +base-bootstrap --name <名称> --tables '[{"name":"<表名>","fields":[{"fieldName":"<字段名>","type":"text"}]}]'` | 表对象键必须是 `name`，不是 `tableName`；字段使用 `fieldName/type/config`；参数已足够时直接执行，不读 Reference 或 Help |
| 已有 Base 新建一张表与字段 | `dws aitable +table-bootstrap --base-id <ID> --name <表名> --fields '<JSON数组>'` | 字段使用 `fieldName/type/config`；自动按 15 个字段分片并读回验证 |
| 读取字段目录或完整配置 | `dws aitable field list --base-id <B> --table-id <T>` / `dws aitable +field-get --base-id <B> --table-id <T>` | 只需 fieldId/name/type 用 `field list`；需要 config 用 `+field-get`；不存在 `+field-list` 或 `+list-fields` |
| 查询、筛选、排序或字段投影 | `dws aitable +record-query --base-id <ID> --table-id <ID> [--record-ids <IDs>] [--field-ids <IDs>] [--filters <JSON>] [--sort <JSON>] [--query <关键词>]` | 用户要求“只返回/仅查看”指定字段时必须传对应 `--field-ids`，不能只在最终文本删列；明确要求全量时改用原子 `record query --all --page-limit <N>` |
| 查询一条记录的变更历史 | `dws aitable +record-history-list --base-id <ID> --table-id <ID> --record-id <ID>` | 已知 recordId 时直接执行；不要调用 Help、产品 Catalog 或全量 Schema 寻找 history 命令 |
| 新增单条或批量记录 | `dws aitable record create --base-id <ID> --table-id <ID> --records <JSON>` | 当前无 `+record-create`；写前取字段定义，写后按新 ID 回读 |
| 更新已知 recordId | `dws aitable +record-update --base-id <ID> --table-id <ID> --records <JSON>` | 自动分片并读回；只传需修改字段 |
| 按业务唯一键同步 | `dws aitable +record-upsert-by-key --base-id <ID> --table-id <ID> --key-field-id <ID> --key-value <值> --cells <JSON>` | 0 条创建、1 条更新、多条停止；非字符串键改用 `--key-value-json` |
| 按条件批量修改 | `dws aitable +record-bulk-patch --base-id <ID> --table-id <ID> --query <关键词> --patch <JSON> --max-matches <N>` | 也可用 filters/record-ids 选范围；禁止无边界整表写 |
| 删除整个 Base | `dws aitable +base-delete --base-id <ID>` | 先通过只读命令确认真实 ID；按 Runtime confirmation 执行，不用 Drive 删除同名节点 |
| 删除字段 | `dws aitable +field-delete --base-id <ID> --table-id <ID> --field-id <ID>` | 先读取字段目录并确认非主字段；按 Runtime confirmation 执行 |
| 查询/创建记录主键文档 | `dws aitable +record-primary-doc-get|+record-primary-doc-create ...` | create 必须传 primaryDoc 类型的 `--field-id`；正文操作切到 Doc |
| 生成记录分享链接并发送给联系人 | `dws aitable +record-share-links --base <B> --table <T> --record-ids <IDs>` → `dws chat +dm --to <姓名> --text <完整链接文本>` | AITable 只生成链接；用户要求“发送”时必须加载 `dingtalk-chat` 并对每位收件人完成真实发送，不能停在联系人解析 |
| 创建 View / Dashboard / Chart 或导入文件 | 对应 leaf / `+import-*` | 根 Skill 参数足够则直接执行；复杂配置最多读取一个对应操作 Reference，不读取通用索引 |
| 调整视图列顺序 | `dws aitable view update visible-fields --base-id <ID> --table-id <ID> --view-id <ID> --field-ids <完整有序IDs>` | 先读取字段和当前完整列数组，固定主字段在首位，写后回读精确校验 |
| 创建/修改图表前取配置 | `dws aitable +chart-widgets-example` | 命令返回所有图表类型示例；已有合法 config 时直接 create/update |
| Base 内 Section/节点移动 | `dws aitable +section-*` | Table/Dashboard/Section 是 Base 内 nsheet 节点，不是独立 Drive 节点 |

### 常用 leaf 直达

参数已知时直接执行，不探测 Help/Catalog：Base 查看/改名用 `+base-get` / `+base-update`；模板搜索用 `+template-search`，再把真实 templateId 交给 `base create --template-id`；Table 查看/更新用 `+table-get` / `+table-update`；视图创建/复制用 `view create` / `+view-duplicate`；仪表盘创建/更新/读回用 `dashboard create` / `+dashboard-update` / `+dashboard-get`；表单分享用 `+form-share-update` / `+form-share-get`；查看自动化用 `+workflow-list`。

### 低频入口

字段配置用 `+field-*`；删记录用 `+record-delete`；附件用 `+attachment-*`。批量分享记录用 `+record-share-links --base <B> --table <T> --record-ids <IDs>`。其余能力使用同名前缀 leaf。

## 当前最短路径

- 已有 ID 直接使用；URL 只解析一次；“唯一定位并操作”用 `+resolve-base` / `+resolve-table`，“搜索候选/存在性检查”直接用 `+base-search`，两条路径不要串行探测。filters/sort 缺 fieldId 时才读取字段目录。
- Golden Route 已给出准确命令和参数时直接执行；不预读或默认读取通用 `references/aitable.md`。只有操作参数、JSON 结构或恢复语义确实缺失时，才读取下方一个精确操作 Reference。
- Shortcut 已含分片或验证时不重复拆步；已有 Base 新建完整表结构直接用 `+table-bootstrap`。
- 单产品线性任务直接执行，不创建 TodoWrite；只有跨产品或多个独立分支的长任务才建计划，并且只在阶段切换时更新，不在每条 CLI 后刷新状态。
- 用户要求资源名带当前时间戳时只取一次并在 Base、Table、Dashboard 等名称中复用同一值；不要为每个资源分别取时间。
- JSON 已返回所需字段时立即复用；不得为寻找同一字段改用 `--verbose`、`raw`、`pretty` 重复请求。

## 记录输入与结果

- `cells` key 用当前 fieldId；大 JSON 用相对 `--records-file`。filters 顶层为 `and|or`，sort 使用 `direction`；复杂条件读 [filter-sort](references/aitable/aitable-filter-sort.md)。
- 建表字段类型使用真实枚举：单选为 `singleSelect`；人民币货币字段使用 `type:"currency"` 和 `config:{"currencyType":"CNY","formatter":"FLOAT_2"}`，不要猜 `select` 或 `config.symbol`。
- 用户限定返回字段时，先复用当前字段目录中的真实 fieldId，最终 `+record-query` 必须带 `--field-ids <ID1,ID2>`；工具层投影是业务要求和 token 控制的一部分，不能用最终答复二次过滤替代。
- 按真实字段类型写值，只读字段不得写入。
- 新建从 `data.newRecordIds[]` 取 ID，再用 `+record-query --record-ids` 回读；若用户同时限定列，回读命令一并传 `--field-ids`。
- 批量结果检查 completed/failed、verification、checkpoint；`partial_success` 不是完成。全量查询使用原子 `record query --all` 并检查 `hasMore`；只有 `hasMore=false`，或按指定 ID 全命中时，才声称结果完整。
- 写入效果未知时回读，不重放成功批次。

## 安全边界

- 删除不可逆，按 Runtime confirmation 核对真实目标；`base list` 只是最近访问。字段零/多候选、类型不明时停止；多批写保留已完成批次和续跑位置。

## 按需加载

每个 Case 最多读取一个操作 Reference。Golden Route 参数足够时读取零个并直接执行；一旦读取了一个 Reference，本 Case 不再读取第二个 Reference、通用 `aitable.md`、产品级 Catalog 或 Help。

| 触发条件 | Reference |
|---|---|
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

通用 `references/aitable.md` 仅保留为兼容索引，不是默认入口；正常 Case 不预读。低频能力按意图选择一个最精确的 Reference，禁止连读。

## 错误最短路径

1. 零/多候选、字段歧义或分页不完整：停止并返回证据；需要后续页时只透传真实 `nextCursor`。
2. 类型错误只复核目标字段，不删字段或丢输入；`partial_success` 从 checkpoint 续跑，未知写入先回读。
3. 错误包含 `actions` / `available_flags` 时只执行其中的 `next_command`；同一操作最多做一次有证据的参数修正。`retryable=false` 或目标 ID 类型不符时停止，不把 Drive/Wiki/Space/子节点 ID 轮流代入试错。

## 跨产品边界

- Excel 式单元格、区域和公式操作 → `dingtalk-misc` 的 Sheet。
- Base 作为整体在普通文件夹间移动或做外层存储重命名 → Drive；Base 结构复制/删除，以及 Base 内 Table、Dashboard、Section 的创建、复制、移动、重命名、删除 → AITable。
- 记录主键文档正文 → 取得真实 nodeId 后切 `dingtalk-doc`。
