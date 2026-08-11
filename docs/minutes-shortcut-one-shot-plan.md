# DWS 听记（Minutes）Shortcut 一次性建设方案

> 目标：一次性补齐可诚实对齐 lark-cli 的 Minutes 体验，修复当前隐藏能力的正确性与可发现性，并把 DWS 已有的录音、上传、热词、脑图、发言人总结和权限原子能力组合成更强的业务闭环。

> 实施状态（2026-08-10）：本方案的 27 个公开 shortcut 已落地；公开目录、最终 Schema、Help 与运行时声明已同源。真实账号 E2E 结果、恢复路径和服务端边界见 [Minutes Shortcut E2E 报告](./minutes-shortcut-e2e-report.md)。下文第 1～2 节保留开发前基线，用于解释为什么需要本轮改造，不代表交付后的命令面。

## 1. 基线、范围与结论

### 1.1 复核基线（2026-08-10）

- DWS：`origin/main@2bc4ded9`，工作分支 `codex/minutes-shortcuts`。
- lark-cli：`main@199762bc0ce027948c2e7217a4ccd500c7020fb5`。
- 产品分析输入：内部 DWS 听记 Minutes × lark-cli 对比报告（访问链接含组织信息，已从仓库版本移除）。
- lark-cli 源码基线：[larksuite/cli](https://github.com/larksuite/cli) 及其 [Minutes shortcuts](https://github.com/larksuite/cli/tree/main/shortcuts/minutes)。

本轮不是按名称补壳，而是按下面五层逐项对齐：输入解析、校验与安全、实际编排、完整性/失败语义、输出与 Agent Contract。只有五层都等价，才标记为“等价”；底层 API 语义不同的命令必须标记“部分对齐”或“不对齐”。

### 1.2 当前真实命令面

| 项目 | 当前状态 |
|---|---|
| DWS Minutes 原子 ToolSpec | 34 个 |
| DWS Minutes built-in shortcut | 13 个：6 个公开、7 个隐藏 |
| DWS Minutes Schema ToolSpec | 41 个，其中 7 个 shortcut；隐藏的 `+minutes-search` 意外进入 Schema |
| lark-cli Minutes shortcut | 10 个公开命令 |
| lark-cli 相关会议面 | VC 8 个公开 shortcut、1 个隐藏 `+notes`；另有 Minutes generated event 和 VC events |

DWS 当前公开 6 个：

- `+list-mine`、`+list-shared`、`+list-all`
- `+detail`
- `+record-start`
- `+replace-batch`

DWS 当前隐藏 7 个：

- `+record-pause`、`+record-resume`、`+record-stop`
- `+minutes-search`、`+latest-minutes`、`+transcript`、`+action-items`

### 1.3 lark-cli 本轮实际参照面

| 命令 | 需要对齐的关键体验 |
|---|---|
| `+search` | query/owner/participant/起止时间；至少一个过滤条件；page size 1–30；用户/机器人身份；稳定投影 |
| `+download` | 最多 50 个 minute token；音频/视频；url-only；单文件/目录输出；覆盖保护；批量去重和逐项错误 |
| `+upload` | 接受 Drive `file_token` 创建 Minutes；本地文件需先走 Drive upload |
| `+update` | 更新标题 |
| `+apply-permission` | 当前用户申请 view/edit |
| `+summary` | 完整替换摘要；content/file/stdin；dry-run；建议先读取当前内容 |
| `+todo` | 基于 todo_id 的 add/update/delete，支持单项和 JSON/file/stdin 混合批处理 |
| `+speaker-replace` | 把转写中的 speaker_id/用户标识重新绑定到 Lark open_id；要求 speaker list 预检 |
| `+word-replace` | 批量原子 JSON；file/stdin；重复 source 校验；作用域是逐字稿 |
| `+detail` | 最多 50 个 token；选择 summary/todo/chapter/transcript/keyword；逐字稿落盘；wait-ready 与 processing/permission 指引 |

这 10 个命令是“体验上限参照”，不是同名率 KPI。DWS 若底层对象模型不同，就保留 DWS 名称并在 Contract 中说明差异。

### 1.4 DWS 当前 34 个原子能力

- 列表与读取：mine/shared/all、batch/basic info、summary、keywords、transcription、todos、audio/video URL、tag list/query、audio memo list、hot-word list。
- 录音控制：start、pause、resume、stop。
- 上传状态机：create、complete、cancel。
- AI 派生产物：mind-graph create/status、speaker-summary create/get。
- 内容修改：update title、update summary、replace text、speaker replace。
- 权限：add、apply、remove。
- ASR 词表：hot-word add/delete。

这套原子能力足够完成搜索、传输、录音闭环、AI 派生和分享组合；不足以完成 Todo 写入、章节、会议 Bot、会议事件和会议产物索引。

### 1.5 需要纠正的旧结论

1. 最新 main 已有 `minutes permission apply`，因此 lark `+apply-permission` 已是可建设项，不再属于平台缺口。
2. 最新 main 还新增了 `hot-word delete` 和 `audio-memo list`，可用于 ASR 准备和语音备忘场景。
3. 当前代码并没有可执行的 Minutes `+upload`，旧文档里的“已建”属于过期描述。
4. `minutes get audio` 声明返回“音频/视频 OSS 链接”，不应继续简单描述为“只有音频”；但服务端没有发布稳定的媒体类型字段，因此本地下载必须按响应头/文件名识别，不能假装有可靠的 audio/video 枚举。
5. `conference` 已明确下线，当前 `event` 只支持个人 IM 事件。会议 Bot、会议事件和 meeting-to-minutes 索引不能靠 CLI 编排伪造。
6. 当前 Agent Schema 采用“声明即 Catalog”，旧报告中的 `schema_command_registry`、`schema_hints` 等路径已经退役，严禁重新引入。

## 2. 当前必须先修的正确性问题

### P0-1：隐藏的“最新听记”链路会把真实结果误判为空

`internal/shortcut/smart/latest_minutes.go` 没有解析真实列表结构 `result.itemList`。它同时影响：

- `+latest-minutes`
- `+minutes-search`
- `+transcript`
- `+action-items`

而公开列表实现 `internal/shortcut/minutes/minutes.go` 已正确支持 `itemList`，并有 `list_projection_test.go` 作为真实响应证据。

修复要求：

- 列表解包、字段投影、时间比较与 UUID 提取只保留一套 Minutes 域实现，公开/隐藏命令共同复用。
- 覆盖 `result.itemList`、空列表、非对象条目、不同时间字段、缺 UUID、数字/字符串时间和列表响应漂移测试。
- 不再把“空结果”包装成 validation error；空搜索是成功结果，输出 `count: 0` 和完整性信息。

### P0-2：`+detail` 的逐字稿不是完整逐字稿

当前 `+detail` 只调用一次 `get_minutes_transcription`，没有传递并消费 cursor。逐字稿原子命令明确支持分页，而且原子命令 Help/selection 已要求“默认拉取全部原文”，因此当前实现和最终交付语义可能静默停在第一页。

修复要求：

- 默认拉全逐字稿，跟随 `nextToken`，对 cursor 停滞、成环、声明有下一页但无 token 做硬失败。
- 提供 `--single-page`/`--cursor` 的显式单页模式，但输出必须带 `complete`、`hasMore`、`nextCursor`。
- 跨页按稳定 segment ID 去重；没有 ID 时采用稳定复合键并在输出中声明去重策略。
- `--direction=1` 的全局倒序必须在合并所有页后完成，不能只倒序单页。
- 完整逐字稿默认安全落盘并在 payload 返回路径、字符数和 segment 数；只有显式 inline 且未超过上限时才把全文塞进终端/JSON，避免 Agent 上下文失控。
- 原子 `get transcription` 的 Help、执行与 Schema selection 也要通过最终装配测试证明一致，不能只修 shortcut。

### P0-3：批量文字替换允许无提示的部分写入

当前 `+replace-batch` 某条失败后继续执行其余替换，最终仍返回普通成功 payload，可能留下用户没有预期的部分写入。

修复要求：

- 默认 `--failure-policy=stop`，首个失败后停止，输出 `applied/failed/unattempted` 明细并返回非零错误。
- 显式 `--failure-policy=continue` 才允许继续，最终只要存在失败仍返回可机器识别的 partial failure。
- 写前完整预检所有规则、重复源词、空替换、输入大小和目标听记；支持 `--json`、`@file`、stdin。
- `--dry-run` 输出确定性执行计划和差异，不执行任何写入。
- 不宣传为事务或可回滚：底层没有批量原子写和 undo API。

### P0-4：隐藏命令与 Schema 可见性不一致

`+minutes-search` 因单独声明 Contract 而进入 Schema，其他 6 个隐藏命令却不可发现；测试甚至固化了“Schema 比公开 shortcut 多 1”的异常计数。

修复要求：

- 新增 `internal/shortcut/semantic_catalog_minutes.json`，让 Minutes 的 public/alias/deprecated 决策和其他已评审产品一致。
- 在 `semantic_catalog.go`、`scripts/gen_shortcut_public_catalog.py` 接入该文件，并新增 Minutes 语义目录测试。
- 每个公开 shortcut 都声明完整 `Safety`、`ContractDecl`、参数、约束和 selection；兼容别名只改变 CLI view，不创建第二份 ToolSpec。
- 删除隐藏 Schema 泄漏的特殊计数与测试前提，改为“公开目录 = Agent 可发现 shortcut”的一致性断言。

## 3. lark-cli 10 个 Minutes shortcut 对齐矩阵

状态定义：`等价` 表示核心平台语义一致；`超越` 表示先满足等价基础再增加 DWS 能力；`部分` 表示可以交付但必须公开差异；`不可诚实对齐` 表示缺底层平台能力，禁止造同名空壳。

| lark 命令 | DWS 现状 | 本轮目标 | 状态与边界 |
|---|---|---|---|
| `+search` | 3 个 list shortcut；隐藏 `+minutes-search` 只搜 mine、固定 20 条且解析有 bug | 新建主命令 `+search`：`--scope mine/shared/all`、query、起止时间、cursor/limit、`--page-all/--page-limit`、跨域去重和完整性 ledger；`+minutes-search` 变成隐藏 alias，保留 3 个 list 命令兼容 | **部分**：DWS API 没有 owner/participant 过滤；不在本地假装服务端可精确筛选 |
| `+download` | 只有单条 OSS URL 查询 | 新建安全本地下载：单条或最多 50 个 taskUuid、`--url-only`、output/output-dir、no-clobber、原子发布、大小限制、SSRF/重定向/DNS 防护、签名 URL 脱敏、逐条失败 ledger | **超越**：复用 `internal/localio` 的安全落盘能力；媒体类型仅按真实响应识别 |
| `+upload` | 有 create/complete/cancel 原子命令，无 shortcut | 新建本地文件直传：stat/大小/MIME 校验 → create → HTTPS PUT → complete；失败自动 cancel；有界重试、dry-run、签名 URL 脱敏；输出 session/taskUuid | **超越**：lark 先上传 Drive 再用 file_token，DWS 可从本地媒体直接完成闭环 |
| `+update` | 有 `update title` 原子命令 | 新建安全标题更新：支持 taskUuid/听记 URL、读前校验、dry-run diff、确认和确定性结果 | **等价并增强安全** |
| `+apply-permission` | 最新 main 已有 `permission apply --policy` | 新建语义化权限申请：`--permission view/download/edit` 映射 policy 4/3/2，支持 URL/ID，展示申请对象和策略 | **等价**；映射必须用契约测试固定，不能让用户记数字 |
| `+summary` | 有完整覆盖式 `update summary` 原子命令，当前原子 Contract 无确认 | 新建安全摘要更新：先读现状、支持 content/file/stdin、Markdown 与图片引用校验、diff/preview、确认后整段覆盖 | **超越**：把全量覆盖风险变成显式 read-modify-validate-write 流程 |
| `+todo` | 只有 `get todos` | 不发布写入型 `+todo`；保留并公开只读 `+action-items` | **不可诚实对齐**：DWS 没有 Minutes Todo add/update/delete API，不能借 Todo 产品创建普通任务冒充听记内待办 |
| `+speaker-replace` | 原子命令按发言人昵称替换，语义不是身份绑定 | 新建带逐字稿预检的 `+speaker-replace`：列出匹配发言人、拒绝零/多义匹配、展示影响范围并确认 | **部分**：lark 把 speaker_id 绑定 open_id；DWS 是昵称/可选 UID 替换，不能宣称身份重绑 |
| `+word-replace` | `+replace-batch` 多规则调用原子 replace | 继续以 `+replace-batch` 为 DWS 主命令，补 JSON/file/stdin、dry-run 和正确 partial failure；不新增公开同名 alias | **部分且范围更宽**：DWS 原子能力会同时改逐字稿和摘要，lark 只改逐字稿，不能用同名掩盖副作用差异 |
| `+detail` | 单 taskUuid，5 类产物，逐字稿只取一页 | 升级为单条或最多 50 个目标、选择产物、逐字稿完整分页、安全文件输出、媒体 URL、失败 ledger、可用时有界 wait-ready | **部分且多维超越**：DWS 增加 keywords/media/batch，但没有原生 chapter；无稳定状态时只能对可重试错误做有界等待 |

## 4. 隐藏 7 个 shortcut 的最终处置

| 当前隐藏命令 | 处置 | 公开语义 |
|---|---|---|
| `+record-pause` | 修 Contract 后公开 | 暂停已知 taskUuid 的录音；写操作、用户确认、幂等性按后端事实声明 |
| `+record-resume` | 修 Contract 后公开 | 恢复已暂停录音；先验证目标状态，失败不伪成功 |
| `+record-stop` | 修 Contract 后公开 | 停止录音；高影响写操作，明确停止后不可继续的真实语义 |
| `+minutes-search` | 作为 `+search` 的隐藏兼容 alias | 不保留独立 Schema identity，不再形成第二套搜索实现 |
| `+latest-minutes` | 改为主命令 `+latest` 的隐藏兼容 alias | 在指定 scope/query/time 中选最新听记并返回详情；输出 chosen candidate 和排序依据 |
| `+transcript` | 修复后公开 | `--id` 指定目标；不传时取 mine 最新；默认全量分页，可写文件；输出完整性 |
| `+action-items` | 修复后公开 | `--id` 指定目标；不传时取 mine 最新；只读听记提取待办，不冒充 Todo 写入 |

所有“默认最新”命令都复用统一 resolver：显式 ID/URL 优先；否则使用 scope/query/time 搜索并按可比较创建时间选择。解析不出时间时只接受后端明确有序的结果，否则返回候选要求消歧，不随意取第一条。

## 5. 基于 DWS 现有原子能力做出的超越项

这些能力不是为了增加命令数量，而是把多次手工调用变成有完整状态、补偿和失败语义的业务闭环。

### 5.1 `+record-wrap-up`

`record stop → 有界等待产物可用 → detail/export`。用于会议结束后立即收口录音、逐字稿、摘要、关键词和待办。必须输出每一阶段的状态；stop 成功但产物等待超时时返回 partial failure，绝不把已停止录音伪装为整体失败后可重试。

### 5.2 `+upload-and-analyze`

`本地上传 → complete → 有界等待 → detail`，可选 `--mindmap`、`--speaker-insights`。这是相对 lark 最明确的超越：无需先把本地媒体变成 Drive file_token，且上传失败会 cancel，分析失败会保留已创建的 taskUuid 供恢复。

### 5.3 `+mindmap`

`mind-graph create → status 轮询 → 输出结果`。有 timeout、interval、退避和最终 job 状态；超时不重复创建任务，返回可继续轮询的标识。

### 5.4 `+speaker-insights`

`speaker summary create → get 轮询 → 结构化投影`。支持选择发言人范围；创建成功后的读取失败必须返回恢复句柄。

### 5.5 `+prepare-asr`

录音/上传前执行热词 list/diff/add/delete，再进入 `record-start` 或 `upload`。默认只增不删；删除必须由显式 `--sync` 与确认触发。输出变更计划和最终热词集合。

### 5.6 `+export-pack`

把 basic/summary/keywords/transcript/todos/media 下载到一个受控目录，并生成 `manifest.json`：记录 taskUuid、产物完整性、文件大小、失败项和生成时间，不记录签名 URL/token。采用临时目录构建，达到所选必需产物门槛后再原子发布目录或明确输出 partial 包。

### 5.7 `+share` / `+unshare`

组合人员解析与 `permission add/remove`，支持多成员去重、写前计划、确认和逐成员 ledger。默认首错停止；继续模式显式开启。不能宣称事务或回滚，已成功的权限变更必须在错误输出中列清楚。

## 6. 明确不做“伪对齐”的平台边界

| lark/会议能力 | 不能对齐的原因 | 可接受的后续解锁条件 |
|---|---|---|
| Minutes Chapter | DWS 没有章节原子 API；从摘要本地生成不是同一平台产物 | 后端提供 chapter 读取；或另命名为本地 `+analyze` 派生产物并标来源 |
| Minutes Todo 写入 | 只有听记待办读取，没有 add/update/delete | Minutes Todo 写 API 与稳定 todo ID |
| speaker identity rebinding | DWS 替换昵称/UID，不具备 speaker_id → 用户身份绑定模型 | 发言人列表、稳定 speaker ID 和 rebind API |
| transcript-only word replacement | 当前原子替换同时影响逐字稿和摘要 | 后端提供 scope=transcript 或独立 API |
| owner/participant 精确搜索 | list API 只有归属、关键字和时间 | 服务端 owner/participant filter；否则只能标注为本地候选过滤且不承诺全集 |
| Meeting Artifact Index | `conference` 已下线；calendar、event 与 Minutes 间没有 meetingId↔taskUuid 连接 | 会议历史/详情 API 或 Minutes generated event 暴露稳定关联 ID |
| 独立 Note 对象 | DWS 没有 note ID、正文、权限和 transcript 对象模型 | 独立 Note 产品 API |
| Meeting Bot | 没有 join/leave/active meeting/events/send-message 原子能力 | 会议 Bot API 与会中授权模型 |
| Minutes/VC 生命周期事件 | 当前 event 只支持个人 IM | 上游事件目录提供 minutes generated、meeting start/end 等事件及稳定 payload |

这些缺口写入用户文档和 shortcut selection 的 `avoid_when`，但不注册 unavailable 的可执行空命令；Schema 中也不伪造 interface_ref。

## 7. 最终目标命令面

### 7.1 对齐与兼容面

```text
dws minutes
├── +search                 # 主搜索；+minutes-search 为隐藏 alias
├── +list-mine              # 兼容的 scope facade
├── +list-shared
├── +list-all
├── +latest                 # +latest-minutes 为隐藏 alias
├── +detail
├── +transcript
├── +action-items           # 只读
├── +download
├── +upload
├── +update
├── +apply-permission
├── +summary
├── +speaker-replace
├── +replace-batch
├── +record-start
├── +record-pause
├── +record-resume
└── +record-stop
```

### 7.2 DWS 差异化闭环

```text
├── +record-wrap-up
├── +upload-and-analyze
├── +mindmap
├── +speaker-insights
├── +prepare-asr
├── +export-pack
├── +share
└── +unshare
```

不把 `+todo`、`+word-replace`、`+chapter`、`+meeting-*` 注册成公开命令，原因见第 6 节。

## 8. 实现架构与文件落点

### 8.1 域内复用，而不是继续堆独立 smart 文件

在 `internal/shortcut/minutes/` 收口 Minutes 能力：

- `response.go`：唯一的 list/artifact/permission/upload 响应解析与投影。
- `pagination.go`：列表、逐字稿分页、cursor 防停滞/成环、去重和 completeness ledger。
- `resolver.go`：taskUuid、听记 URL、latest 搜索与消歧。
- `transfer.go`：上传状态机和下载编排；下载复用 `internal/localio`，上传复用已有 HTTPS/OSS 客户端安全策略。
- `read_shortcuts.go`：search/latest/detail/transcript/action-items/download。
- `write_shortcuts.go`：upload/update/apply-permission/summary/speaker/replace/record/share。
- `workflow_shortcuts.go`：wrap-up/analyze/mindmap/insights/prepare-asr/export-pack。

迁移 `internal/shortcut/smart` 中 Minutes 专属实现；旧名称只作为主命令的 `Shortcut.Aliases`，不能重复注册一份执行体或 Contract。

### 8.2 统一执行结果信封

所有批量/多阶段命令输出同一结构：

```json
{
  "operation": "minutes.upload_and_analyze",
  "complete": false,
  "summary": {"total": 4, "succeeded": 3, "failed": 1, "unattempted": 0},
  "stages": [],
  "results": [],
  "recovery": {"taskUuid": "...", "nextAction": "..."}
}
```

规则：

- 只要发生部分写或所选必需产物不完整，命令返回非零，同时保留结构化结果。
- signed URL、authorization header、upload credential、token 不进入输出、日志或测试快照。
- 读 fan-out 可继续收集失败项；写 fan-out 默认首错停止，只有显式 continue 才继续。
- dry-run 只显示本地可确定计划，不调用写接口，也不声称远端验证已通过。

### 8.3 Contract 与生成边界

- 每个主命令通过 `Safety` / `ContractDecl` / `contract.ParamDecl` 声明最终 Schema 事实。
- 新增 `semantic_catalog_minutes.json` 作为 Minutes shortcut 可见性和语义关系的评审输入。
- 保留 `+list-*` 等兼容 facade 的独立 identity；真正的拼写兼容用 `Aliases`，alias lookup 只改变 `cli_path/is_alias`。
- 不新增 `schema_hints/`、`schema_command_registry/`、`schema_mcp_metadata.json` 或提交生成 Catalog。
- `make generate-schema` 只刷新参数别名并验证运行时装配确定性。

## 9. 一次性开发顺序

所有工作在一个分支完成，但按依赖顺序提交，避免前层错误污染后续流程。

### A. 契约冻结与回归夹具

1. 固定 DWS 34 个原子 ToolSpec、lark 10 个 Minutes shortcut 与本方案矩阵。
2. 为真实 `result.itemList`、transcript 多页、upload create/complete/cancel、audio URL、permission policy 建匿名 fixture。
3. 新增 Minutes semantic catalog 和目录一致性测试，先让当前隐藏泄漏显式失败。

### B. 共享域基础设施

1. 合并 list parser 和 target resolver。
2. 实现通用 cursor pager、完整性 ledger、partial failure 类型和敏感字段脱敏。
3. 接入 `internal/localio`，补 Minutes 文件名、批量路径与媒体响应解析。
4. 实现 upload 状态机及 cancel 补偿，使用可注入 HTTP/MCP seam。

### C. 修现有命令

1. 修复 latest/search/transcript/action-items 的 `itemList` 问题。
2. 升级 `+detail` 全量分页、批量和文件输出。
3. 修复 `+replace-batch` 的部分写与输入能力。
4. 为 record pause/resume/stop 和其他拟公开命令补全 Safety/Contract。

### D. 补齐 lark 对齐面

依次交付 `+search`、`+download`、`+upload`、`+update`、`+apply-permission`、`+summary`、`+speaker-replace`，再挂载兼容 aliases。每个命令完成后同时补 help、Schema、selection、fixture 和 failure-path 测试，不留“代码存在但仍隐藏”的中间态。

### E. 交付 DWS 超越面

基于已验证基础设施实现 record-wrap-up、upload-and-analyze、mindmap、speaker-insights、prepare-asr、export-pack、share/unshare；每个流程都要覆盖“前一步成功、后一步失败”的恢复信息。

### F. 文档与发布收口

1. 更新 Minutes mono/multi Skill 和 `docs/shortcut-lark-alignment.md` 的过期数据。
2. 生成 public shortcut catalog；清除隐藏 Schema 泄漏和旧计数断言。
3. 文档列出第 6 节平台边界，确保“未对齐”不是遗漏而是有证据的产品决定。

## 10. 测试与政策门槛

### 10.1 单元与组件测试

- 列表：`result.itemList`、空页、字段漂移、mine/shared 合并去重、时间范围与 scope。
- 分页：多页、空 token、停滞、成环、重复 segment、倒序、page limit、上下文取消。
- resolver：taskUuid、URL、latest、无结果、多候选、无法比较时间。
- 下载：HTTPS、私网/loopback/DNS rebinding/redirect 拒绝、路径逃逸、symlink 替换、no-clobber、大小上限、临时文件清理、签名 URL 脱敏。
- 上传：create→PUT→complete、PUT 重试、complete 失败、cancel 成功/失败、超时、本地文件变化、credential 脱敏。
- 写命令：dry-run 无远端调用、确认门、首错停止、显式 continue、partial ledger、recovery handle。
- Contract：每个 public shortcut 的 help/Schema/summary 同源；aliases 不产生第二份合同。

涉及 macOS 覆盖门的测试按仓库规则命名为 `TestCrossPlatformCoverage*`。

### 10.2 集成测试

使用本地 fake MCP + `httptest`，覆盖：

- 本地文件上传完整状态机和失败补偿。
- OSS 下载的安全重定向与原子发布。
- detail 多产物 fan-out、逐字稿多页和 partial failure。
- record-wrap-up 的 stop 成功/等待超时。
- upload-and-analyze 的 upload 成功/分析失败恢复。
- summary、replace、share 的读前校验与部分写输出。

只有有安全隔离、明确测试账号和可恢复 fixture 时才增加真实环境 smoke；不能因为缺真实账号 fixture 就把已评审能力继续隐藏。

### 10.3 必过命令

```bash
gofmt -w <modified-go-files>
go generate ./internal/cli
make generate-schema
./scripts/policy/check-generated-drift.sh
./scripts/policy/check-schema-catalog.sh
./scripts/policy/check-runtime-confirmation-truth.sh
make test-schema-agent-examples
DWS_PACKAGE_VERSION=0.0.0-test go test ./...
make build
```

另加 Minutes 聚焦回归，覆盖最终 app 装配、Help、Schema、alias 和 runtime confirmation；不能只测 generator 或孤立 registry。

## 11. 一次性完成的验收定义

全部满足才算完成：

1. P0 四类问题均有回归测试，旧隐藏链路不再漏 `itemList`，逐字稿不会静默截断。
2. 第 3 节中所有“等价/超越/部分”项都有可执行主命令、真实底层调用、完整 Contract 和失败语义。
3. 第 4 节 7 个隐藏命令全部按表处置；没有“可执行但目录/Schema 状态不明”的命令。
4. 第 5 节 8 个 DWS 闭环全部输出阶段状态、partial failure 和可恢复句柄。
5. 第 6 节不可对齐项在文档与 selection 中清楚说明，仓库里没有同名假实现。
6. 所有写操作默认安全，`--yes` 不进入存储示例；批量写不静默部分成功。
7. `dws schema --all`、leaf、summary 和 alias view 使用同一 ToolSpec；无 retired Schema 输入回流。
8. 全量测试、Schema policy、runtime confirmation truth、example gate 和 build 全绿。

## 12. 实施时的硬性非目标

- 不为了命令数量对齐而创建空壳、离线 mock 或错误 interface_ref。
- 不把普通 Todo、Calendar event 或 IM event 冒充 Minutes Todo/Meeting event。
- 不从 summary 推导 chapter 后宣称它是平台章节。
- 不承诺底层没有提供的事务、回滚、speaker identity 或媒体类型。
- 不顺带修改用户未纳入本分支的文件；当前未跟踪的 `docs/doc-shortcut-business-review.html` 保持原样。
