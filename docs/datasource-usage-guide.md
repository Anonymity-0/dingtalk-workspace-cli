# AI 表格数据源指令使用指南

## 概述

dws 新增了 7 个 AI 表格数据源同步管理指令，用于将外部数据源（一期支持审批数据）接入 AI 表格，实现数据的自动同步。

所有指令均通过 `dws aitable +datasource-*` 前缀调用，操作对象是 AI 表格中的"数据源表"——一种由数据源同步创建的特殊数据表。

## 指令速览

| 指令 | 用途 | 读写 | 风险 |
|------|------|------|------|
| `+datasource-list-sources` | 列出数据源类型可用的来源信息（OA 返回 result/processCode、sourceType、sourceUrl） | 读 | low |
| `+datasource-get-fields` | 获取数据源来源的可同步字段结构 | 读 | low |
| `+datasource-create` | 创建数据源表并触发首次同步 | 写 | medium |
| `+datasource-update` | 更新已有数据源表的同步配置 | 写 | medium |
| `+datasource-sync` | 手动触发一次同步 | 写 | medium |
| `+datasource-sync-status` | 查询同步任务状态 | 读 | low |
| `+datasource-get-config` | 查看数据源表配置 | 读 | low |

## 前置条件

1. **登录认证**：执行 `dws auth login` 确保已登录
2. **获取 Base ID**：通过 `dws aitable +base-list` 或 `dws aitable +base-search --query "关键词"` 获取目标 AI 表格的 Base ID

---

## 1. 列出数据源可用来源

```
dws aitable +datasource-list-sources [flags]
```

列出指定数据源类型可用的来源信息。OA 审批类型返回当前 Base 可用的审批数据源条目（`sources` 数组，当前通常为单条），用于构造 `+datasource-create` / `+datasource-update` / `+datasource-get-fields` 的 `--source-config`。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--base-id` | string | 是 | 目标 Base ID |
| `--datasource-type` | string | 是 | 数据源类型，目前支持审批（OA） |

### 示例

```bash
# 列出审批数据源来源，获取 result（即 processCode）
dws aitable +datasource-list-sources \
  --base-id BASE123 \
  --datasource-type OA
```

### 返回值

返回 `sources` 数组，每个条目包含：

| 字段 | 说明 |
|------|------|
| `result` | 数据源标识，OA 场景即 `processCode`，用于 `--source-config` 中的 `processCode` |
| `sourceType` | 数据源类型编号（OA 对应内部枚举值 2） |
| `iconUrl` | OA 审批图标 URL，创建/更新 `sourceConfig` 时须原样透传 |
| `url` | OA 审批跳转链接，创建/更新 `sourceConfig` 时须原样透传 |
| `sourceUrl` | 数据源访问链接，可选 |

将 `result` 作为 `processCode`，并将 `iconUrl`、`url` 原样填入 `--source-config` 即可创建或更新数据源。

---

## 2. 获取数据源可同步字段

```
dws aitable +datasource-get-fields [flags]
```

获取指定数据源来源（如某个审批模板）的可同步字段列表，包括字段 ID、字段名称、字段类型和是否主键等信息。用于创建数据源前选择需要同步的字段。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--base-id` | string | 是 | 目标 Base ID |
| `--datasource-type` | string | 是 | 数据源类型，目前支持审批（OA） |
| `--source-config` | string | 是 | 源配置 JSON 字符串，结构同 `+datasource-create` 的 `--source-config` |

### 示例

```bash
# 获取某审批模板的可同步字段
dws aitable +datasource-get-fields \
  --base-id BASE123 \
  --datasource-type OA \
  --source-config '{"processCode":"PROC-XXXX","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}'
```

### 返回值

返回可同步字段列表，每个字段包含字段 ID、名称、类型和是否主键。字段 ID 可用于 `+datasource-create` / `+datasource-update` 的 `--field-ids` 参数。

---

## 3. 创建数据源表

```
dws aitable +datasource-create [flags]
```

为指定 AI 表格创建数据源同步配置，自动创建一张数据源表并触发首次全量同步。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--base-id` | string | 是 | 目标 Base ID（通过 `+base-list` / `+base-search` 获取） |
| `--datasource-type` | string | 是 | 数据源类型，目前支持审批（OA） |
| `--source-config` | string | 是 | 源配置 JSON 字符串（格式见下方） |
| `--auto` | bool | 否 | 是否开启自动同步，默认 false |
| `--field-ids` | stringSlice | 否 | 需要同步的字段 ID 列表，不传时同步全部字段 |
| `--conflict-strategy` | int | 否 | 冲突策略：0=覆盖（默认），1=跳过 |

### source-config 格式（审批类）

审批数据源的 `--source-config` 是一个 JSON 对象字符串，包含以下字段：

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `processCode` | string | 是 | 审批模板编码，对应 `+datasource-list-sources` 返回的 `result` |
| `dataType` | string | 是 | 数据时间范围类型：`time_range` / `start_time` / `recent_time` |
| `iconUrl` | string | 是 | OA 审批图标 URL，须从 `+datasource-list-sources` 结果原样透传 |
| `url` | string | 是 | OA 审批跳转链接，须从 `+datasource-list-sources` 结果原样透传 |
| `recentDays` | string | 当 dataType=recent_time 时必填 | 近 N 天：`7d` / `30d` / `1y` |
| `startDate` | string | 当 dataType=time_range 或 start_time 时必填 | 起始日期，格式 `yyyy-MM-dd` |
| `endDate` | string | 当 dataType=time_range 时必填 | 结束日期，格式 `yyyy-MM-dd` |
| `name` | string | 否 | 数据源表显示名称（用作创建的表名） |
| `keepRemovedFields` | bool | 否 | 是否保留已删除字段，默认 false |

> 注：`splitParentTableField`、`enableDataSyncOaDetailList` 等字段为下游内部字段，无需传入，下游自动处理。

按 `dataType` 选择对应的时间参数组合：

| dataType | 需要的时间字段 | 说明 |
|----------|----------------|------|
| `recent_time` | `recentDays` | 同步近 N 天数据（7d/30d/1y） |
| `start_time` | `startDate` | 同步从某日期至今的数据 |
| `time_range` | `startDate` + `endDate` | 同步指定日期范围内的数据 |

### 示例

```bash
# 基本创建——同步近 30 天审批数据
dws aitable +datasource-create \
  --base-id BASE123 \
  --datasource-type OA \
  --source-config '{"processCode":"PROC-XXXX","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}'

# 指定日期范围创建并开启自动同步
dws aitable +datasource-create \
  --base-id BASE123 \
  --datasource-type OA \
  --source-config '{"processCode":"PROC-XXXX","dataType":"time_range","startDate":"2025-01-01","endDate":"2025-12-31","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}' \
  --auto

# 指定同步字段（仅同步部分字段，field-ids 可通过 +datasource-get-fields 获取）
dws aitable +datasource-create \
  --base-id BASE123 \
  --datasource-type OA \
  --source-config '{"processCode":"PROC-XXXX","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}' \
  --field-ids fldAAA,fldBBB,fldCCC
```

### 返回值

创建成功后返回新建数据源表 ID 和同步任务 ID，后续操作需要用到这两个 ID。

---

## 4. 更新数据源配置

```
dws aitable +datasource-update [flags]
```

更新已有数据源表的同步配置，支持更新源配置、自动同步开关和同步字段选择。更新后会自动触发一次同步。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--base-id` | string | 是 | 目标 Base ID |
| `--table-id` | string | 是 | 已存在的数据源表 ID（由 `+datasource-create` 返回） |
| `--source-config` | string | 否 | 新的源配置 JSON 字符串，不传时保持原有配置。结构同 `+datasource-create` |
| `--auto` | bool | 否 | 是否开启自动同步，不传时保持原有设置 |
| `--field-ids` | stringSlice | 否 | 需要同步的字段 ID 列表，不传时同步全部字段 |

### 示例

```bash
# 更换审批模板并调整时间范围
dws aitable +datasource-update \
  --base-id BASE123 \
  --table-id TBL456 \
  --source-config '{"processCode":"PROC-YYYY","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}'

# 开启自动同步
dws aitable +datasource-update \
  --base-id BASE123 \
  --table-id TBL456 \
  --auto

# 更新同步字段范围
dws aitable +datasource-update \
  --base-id BASE123 \
  --table-id TBL456 \
  --field-ids fldAAA,fldDDD
```

> 注意：`--table-id` 指向的是数据源表（由 `+datasource-create` 创建），不是普通数据表。

---

## 5. 触发手动同步

```
dws aitable +datasource-sync [flags]
```

对已有数据源表触发一次手动同步。单次最多 5 张表，每张表独立提交，部分失败不影响其他表。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--base-id` | string | 是 | 目标 Base ID |
| `--table-ids` | stringSlice | 是 | 待触发同步的数据源表 ID 列表（1-5 个） |

### 示例

```bash
# 同步单张表
dws aitable +datasource-sync \
  --base-id BASE123 \
  --table-ids TBL1

# 批量同步多张表（逗号分隔，最多 5 个）
dws aitable +datasource-sync \
  --base-id BASE123 \
  --table-ids TBL1,TBL2,TBL3
```

### 返回值

返回每个表的同步任务 ID，可通过 `+datasource-sync-status` 查询最终结果。

---

## 6. 查询同步状态

```
dws aitable +datasource-sync-status [flags]
```

查询数据源表的同步任务状态。与 `+datasource-sync` / `+datasource-create` / `+datasource-update` 配对使用——这些指令触发同步后返回任务 ID，本指令通过任务 ID 查询最终结果。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--base-id` | string | 是 | 目标 Base ID |
| `--table-id` | string | 是 | 数据源表 ID |
| `--task-ids` | stringSlice | 否 | 待查询的同步任务 ID 列表（1-5 个），不传时查询最近一次 |

### 示例

```bash
# 查询最近一次同步状态
dws aitable +datasource-sync-status \
  --base-id BASE123 \
  --table-id TBL456

# 按任务 ID 查询（批量，最多 5 个）
dws aitable +datasource-sync-status \
  --base-id BASE123 \
  --table-id TBL456 \
  --task-ids TASK1,TASK2
```

---

## 7. 获取数据源配置

```
dws aitable +datasource-get-config [flags]
```

获取指定数据源表的同步配置信息，包括源配置、同步模式、自动同步开关和同步状态。

### 参数

| 参数 | 类型 | 必填 | 说明 |
|------|------|------|------|
| `--base-id` | string | 是 | 目标 Base ID |
| `--table-id` | string | 是 | 数据源表 ID |

### 示例

```bash
dws aitable +datasource-get-config \
  --base-id BASE123 \
  --table-id TBL456
```

---

## 典型工作流

### 场景一：从零接入审批数据

```bash
# 0. 获取 Base ID
dws aitable +base-search --query "我的项目表"

# 1. 列出可用审批数据源来源，获取 result（即 processCode）
dws aitable +datasource-list-sources \
  --base-id BASE123 \
  --datasource-type OA
# → 返回 sources[0].result=PROC-XXXX

# 2. 查看可同步字段（可选，用于指定 field-ids）
dws aitable +datasource-get-fields \
  --base-id BASE123 \
  --datasource-type OA \
  --source-config '{"processCode":"PROC-XXXX","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}'

# 3. 创建数据源表（创建后自动触发首次同步）
dws aitable +datasource-create \
  --base-id BASE123 \
  --datasource-type OA \
  --source-config '{"processCode":"PROC-XXXX","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}'
# → 返回 tableId=TBL456, taskId=TASK001

# 4. 查询首次同步是否完成
dws aitable +datasource-sync-status \
  --base-id BASE123 \
  --table-id TBL456 \
  --task-ids TASK001

# 5. 确认配置
dws aitable +datasource-get-config \
  --base-id BASE123 \
  --table-id TBL456
```

### 场景二：更换审批模板后重新同步

```bash
# 1. 更新源配置（更新后自动触发一次同步）
dws aitable +datasource-update \
  --base-id BASE123 \
  --table-id TBL456 \
  --source-config '{"processCode":"PROC-NEW","dataType":"recent_time","recentDays":"30d","iconUrl":"https://example.com/icon.png","url":"https://example.com/oa"}'

# 2. 查询同步状态
dws aitable +datasource-sync-status \
  --base-id BASE123 \
  --table-id TBL456
```

### 场景三：手动触发日常同步

```bash
# 仅触发同步，不修改配置
dws aitable +datasource-sync \
  --base-id BASE123 \
  --table-ids TBL456

# 查询结果
dws aitable +datasource-sync-status \
  --base-id BASE123 \
  --table-id TBL456
```

### 场景四：开启自动同步后确认

```bash
# 1. 更新配置，开启自动同步
dws aitable +datasource-update \
  --base-id BASE123 \
  --table-id TBL456 \
  --auto

# 2. 确认配置已更新
dws aitable +datasource-get-config \
  --base-id BASE123 \
  --table-id TBL456
# → 返回中应显示 auto=true
```

---

## 通用选项

以下全局选项可在所有指令中使用：

| 选项 | 说明 |
|------|------|
| `-f, --format` | 输出格式：json（默认）/ table / raw / pretty / ndjson / csv |
| `--jq` | jq 表达式过滤输出（如 `.tableId` 或 `.status`） |
| `--fields` | 筛选输出字段（逗号分隔） |
| `--dry-run` | 预览操作内容，不实际执行 |
| `--profile` | 指定组织或账号 |
| `--timeout` | HTTP 请求超时时间（秒，默认 30） |
| `--debug` | 显示调试日志 |
| `-v, --verbose` | 显示详细日志 |

### 输出过滤示例

```bash
# 只取 tableId
dws aitable +datasource-create ... --jq '.tableId'

# 只取同步状态
dws aitable +datasource-sync-status ... --jq '.status'

# table 格式查看
dws aitable +datasource-get-config ... -f table
```

---

## 注意事项

1. **推荐流程**：先 `+datasource-list-sources` 获取 `result`（即 processCode），再 `+datasource-get-fields` 查看可同步字段，最后 `+datasource-create` 创建数据源表。

2. **数据源表 vs 普通数据表**：`+datasource-create` 创建的是"数据源表"，它由数据源同步驱动数据写入。`+datasource-update` 和 `+datasource-sync` 仅适用于数据源表，不可对普通数据表使用。

3. **datasource-type 透传**：CLI 层不对 `--datasource-type` 做枚举校验，目前一期仅支持 `OA`（审批）。后续支持其他类型时由服务端控制，CLI 无需修改。

4. **source-config 格式**：`--source-config` 必须是合法 JSON 字符串。审批数据源需要 `processCode`（对应 `+datasource-list-sources` 返回的 `result`）、`dataType`（时间范围类型）、`iconUrl` 与 `url`（须从 `+datasource-list-sources` 结果原样透传），并按 `dataType` 提供对应的时间参数（`recentDays` / `startDate` / `endDate`）。

5. **同步限制**：`+datasource-sync` 单次最多 5 张表；`+datasource-sync-status` 单次最多查询 5 个任务 ID。

6. **创建即同步**：`+datasource-create` 和 `+datasource-update` 在操作完成后会自动触发一次同步，无需额外调用 `+datasource-sync`。

7. **冲突策略**：创建时如果目标 Base 下已存在数据源表，`--conflict-strategy 0`（默认）会覆盖已有配置，`--conflict-strategy 1` 则会跳过（保留已有配置）。

8. **自动同步**：`--auto` 开启后，数据源表会按服务端策略自动定期同步。关闭后仅能通过 `+datasource-sync` 手动触发。注意：同步频率由服务端策略控制，CLI 暂不支持自定义频率。
