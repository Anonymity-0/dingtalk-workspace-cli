# 便捷层投影保真：上下层数据必须比对（警示案例）

## 一句话铁律

**任何做了 reshape/投影的 shortcut，都必须对比「上层投影输出」与「下层原始后端数据」。**
只看上层永远发现不了「投影把非空数据吃成空」这类失分点。

## 警示案例

| Shortcut | 底层工具 | 真实响应结构 | 旧投影结果 | 症状 |
|----------|---------|-------------|-----------|------|
| `contact +list-roles` | `get_org_labels` | `result[]` 是**角色分组**，角色嵌在每组 `labels[]` 里 | `count=0, roles=[]` | exit 0、无 error 信封，**静默返空**（底层 57 个角色） |
| `oa +list-forms` | `list_user_visible_process` | 表单在 `result.processCodeList[]` | `count=0, forms=[]` | exit 0、无 error 信封，**静默返空**（底层 93 张表单） |

两处根因都在投影层的 list 定位器：

- `listRolesResolveList` 命中 `result` 是数组就直接返回——返回的是**分组对象**（`{groupName, labels}`），
  没有下钻进 `labels[]`；分组对象取不到 `labelId/name`，每行被丢弃 → 空。
- `oaFormResolveList` 探测的容器 key 里**漏了真实键 `processCodeList`** → 定位不到列表 → 空。

危害：`exit 0` + 无 error 信封 → Agent 误判「无数据」并据此做错决策。这一处硬伤会同源连累
多个评估维度。

## 修复

- `internal/shortcut/contact/contact.go`：新增 `listRolesFlattenGroups`，把分组数组
  （`result[] = {groupName, labels[]}`）平坦化成角色列表；已是扁平列表时原样返回，兼容两种结构。
- `internal/shortcut/oa/oa.go`：`oaFormResolveList` 的探测 key 补上 `processCodeList`。
- 守卫单测（喂真实嵌套结构、断言非空，防回归）：
  - `internal/shortcut/contact/list_roles_projection_test.go`
  - `internal/shortcut/oa/list_forms_projection_test.go`

## 判定规则升级：从体系上堵住这个盲区

`scripts/shortcut_real_result.py` 过去只看上层：`exit==0` 且无 error 信封 → `real-ok`，
所以「投影返空」会被判成通过。现在新增**上下层比对**：

- `count_projection_items(stdout)`：数出上层投影条目数（列表型投影）。
- `backend_record_count(raw)`：估算下层原始后端的业务条目数。
- `compare_layers(...)`：上层为空、下层有数据 → 判定投影吃数据。
- `classify_real_status(..., backend_raw=...)`：采集到下层时，上下层不一致直接判 `real-error`。
- `classify_failure(...)`：新增类别 `projection-data-loss`（`cli-projection-fix-needed`），
  即便进程 exit 0、status 被记成 real-ok 也会被翻出来。
- `projection_audit(result)`：对只读/列表类 shortcut 强制比对——
  - 下层有数据、上层空 → `projection-data-loss`；
  - 上层空但**未采集下层** → `empty-projection-unverified`（**必须补采下层再比对，禁止直接判 real-ok**）。

规则自测：`python3 scripts/shortcut_real_result_test.py`。

## 待办：其余投影 resolver 需按真实后端结构逐一比对

同类风险存在于所有「硬编码容器 key / 不下钻嵌套」的 resolver。以下文件的 `*ResolveList` /
`*Project` 需要在真实后端下跑一遍、用上下层比对确认不丢数据（无测试文件的包尤其优先）：

- `internal/shortcut/attendance/attendance.go`（无单测）
- `internal/shortcut/calendar/calendar.go`（无单测）
- `internal/shortcut/mail/mail.go`（无单测）
- `internal/shortcut/wiki/wiki.go`（无单测）
- `internal/shortcut/minutes/minutes.go`（无单测）
- `internal/shortcut/doc/doc.go`、`internal/shortcut/drive/drive.go`（无单测）
- `internal/shortcut/report/report.go`、`internal/shortcut/sheet/sheet.go`、
  `internal/shortcut/todo/todo.go`、`internal/shortcut/devapp/devapp.go`（无单测）
- `internal/shortcut/oa/oa.go` 的 `oaInstanceResolveList`（list-pending/list-executed 等，需复核真实键）

验证方式：真实账号下跑 shortcut 拿上层输出，同时用对应 leaf 命令（或原始 MCP 响应）拿下层，
交给 `projection_audit` / `classify_real_status(backend_raw=...)` 比对。
