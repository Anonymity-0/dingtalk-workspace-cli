# DWS IM 能力与语义层优化设计

> 状态：Active Plan（Runtime Schema 平台基线已关闭，IM 纵向能力持续实施）
>
> 日期：2026-07-27；最近复核：2026-07-28
>
> 适用范围：`dws chat`、Chat Semantic Shortcut、Chat Schema/Hints/Skills、个人 IM 事件消费，以及必要的上游 MCP 契约协同
>
> 当前 DWS 基线：`main@2dfc39f0`
>
> lark-cli 对比基线：`main@56c9a2a`、稳定版 `v1.0.77`

## 1. 摘要

一句话目标：

> **保留 Runtime Schema 作为完整原子能力面，同时公开全部 91 个已审阅、
> 当前可用的 Chat Shortcut；用 disposition 明确区分 Smart 主入口、
> Semantic Adapter、Schema leaf 投影和兼容别名，并继续重点优化“读消息、
> 搜消息、资源下载、按人名发送、真实送达、监听后回复”的实际效果。**

### 1.1 用户现在遇到什么问题

| 用户意图 | 当前问题 | 优化后 |
|---|---|---|
| “查这个群昨天的消息” | 需要自己补时间、方向和 cursor；不同命令输出不同；可能 exit 0 却错误返空 | 自动选择时间窗和方向、有界翻页，返回统一 Message/Page；shape 不认识就明确报错 |
| “搜索小王发的那条消息” | Agent 需要判断 userId/openDingTalkId、再拼搜索和 mget | 目标解析、搜索、详情与会话/发送人富化由一个语义入口完成 |
| “把这张本地图片发到群里” | 本地图片会被当作 file 附件；上传、发送和下载没有统一资源模型 | 解析群名/ID，按内联图片上传并发送；不支持时明确报 capability error，不静默降级 |
| “消息发成功了吗” | `openTaskId` 只表示已受理，容易被误认为已送达 | 返回 accepted/pending/succeeded/failed，并可有界等待真实结果 |
| “监听群消息并回复” | Event 与 Chat 的消息、身份、资源字段不同，需要 Agent 自己拼接和防循环 | 统一 Event Payload/ResourceRef，提供过滤、去重、防循环、限速和清理 recipe |
| “下载消息里的文件” | mediaId 需要人工提取；路径可覆盖或逃逸；失败结果不统一 | 自动提取资源，限制安全目录，不覆盖，校验大小/checksum，单文件失败不丢主结果 |

### 1.2 每一层到底负责什么

| 层 | 本轮职责 | 不负责 |
|---|---|---|
| Raw CLI + Runtime Schema | 完整发布 78 个原子 IM 工具；保证 path、flag、binding、约束和 safety 正确 | 不为每个原子工具再复制一个 Shortcut |
| Semantic Shortcut | 公开全部 reviewed + available 入口；用 disposition 表达 Smart、Adapter、Schema 投影和兼容关系，并让 Smart 入口获得默认路由优先级 | 不把公开等同于等权候选，不掩盖原子 binding、权限、Fixture 或下层错误 |
| Hint | 让 Agent 在原子命令和语义入口之间选对；参数和安全事实与 runtime 一致 | 不发明命令、flag 或后端能力 |
| Skill / Recipe | 把自然语言意图路由到 Golden Path；处理跨 Chat/Event/Contact/Drive/DING 场景 | 不复制完整命令表和易漂移执行事实 |
| Typed IM Adapter | 吸收 MCP/OpenAPI 字段差异，输出统一 Message/Page/Resource/Delivery/Error | 不改变上游原生 ID，不掩盖 shape drift |
| Event | 稳定消费、统一 payload、资源引用和监听回复闭环 | 不用轮询历史消息伪装实时事件 |

### 1.3 具体要交付什么

| 顺序 | 交付包 | 具体改动 | 用户可感知结果 |
|---:|---|---|---|
| 1 | P0 语义底座 | 参数 ADR；TargetRef/IdentityRef；Message/Page 最小模型；typed projection error；Shortcut 语义价值审计 | ID 不再乱猜，错误空结果能被识别；知道哪些 Shortcut 应保留、合并、隐藏或改成 Smart |
| 2 | P1 高保真读取/搜索 | 统一 list/search/mget/thread adapter；默认时间窗；自动分页；发送人/会话/reaction/reply 富化；局部失败 | “查记录/搜消息”一次调用得到可用结果，不再手工翻页和补详情 |
| 3 | P2 资源与媒体 | Media Resolver；本地/URL/media key；内联图片；安全下载；MIME/文件名/checksum | 本地图片真正以内联图片发送，文件可安全往返并校验 |
| 4 | P4 个人事件闭环 | 16 类 payload 归一；ResourceRef；queue/last-error 指标；listen-and-reply recipe | 先完成稳定监听、过滤和退出；回复尾项复用 P3 的发送安全能力 |
| 5 | P3 发送与送达 | send/reply 内容模型；dry-run/idempotency；目标摘要；DeliveryStatus 与等待 | 发送前知道目标和计划，发送后区分“受理”和“真实送达” |
| 6 | P5/P6 治理与长尾 | 群治理按意图审计；高风险 gate；Skill mono/multi 一致；清理 metadata 差异 | 钉钉特色治理能力可被准确选择，高风险动作行为一致 |

### 1.4 Shortcut 的明确决策

本轮最终实现决策是：对原有 89 个及新增 2 个 Chat Shortcut 完成逐项
语义审计后，将 91 个 `reviewed + available` 入口全部公开。

- `primary_smart`：有目标解析、分页、资源处理或跨命令编排价值，作为
  Agent 默认优先候选；
- `semantic_adapter`：统一目标或输出语义，优先于同义原子路径；
- `schema_leaf`：公开的稳定原子能力投影，适合精确意图，不与 Smart
  入口等权竞争；
- `alias_internal`：为已有调用方保留的公开兼容别名，明确指向 primary；
- 真机 evidence 与 visibility 正交：Fixture 缺失和下层服务错误继续在
  测试报告中暴露，但不再机械隐藏 reviewed + available 命令；
- Runtime Schema/CLI 仍是完整原子能力事实源，Shortcut 公开不改变
  binding、权限或安全语义。

这一最终决策替代本文早期“逐意图发布、不批量解锁”的阶段性建议；后文
保留相关文字时，应将其理解为历史规划背景，而不是当前可见性规则。

第一批建议只建设或升级五个高价值语义入口：

1. 高保真 message history；
2. 高保真 message search；
3. resource download；
4. name-aware send；
5. listen-and-reply recipe。

### 1.5 建议马上开始的三个变更

| 变更 | 做什么 | 主要位置 | 验收 |
|---|---|---|---|
| Change 1：Shortcut 语义价值审计与全量公开 | 给 91 项补 `semantic_delta` 与 smart/adapter/leaf/alias 结论；改 Catalog 生成规则，分离 visibility、availability、evidence | `internal/shortcut/`、`scripts/gen_shortcut_public_catalog.py` | 91/91 reviewed + available 项公开；每项有明确处置；`live_blocked` 不再等于不可用 |
| Change 2：统一读取内核 | 抽取 list/search/mget/thread 共用 adapter、Message/Page、默认时间窗、分页和 `projection_shape_drift` | `internal/helpers/chat*.go`、`internal/shortcut/chatmsg/` 或新 typed IM package | raw 非空不得 normalized 为空；截断必提示；富化失败保留主消息 |
| Change 3：两个读取语义入口 | 把重复候选收敛为 history 和 search 两个 primary，接入统一读取内核；同步 Hint 和 Skill | `internal/shortcut/chat/`、`schema_hints/selection/chat.json`、Chat Skill | 自然语言“查记录/搜消息”Top-1 选对；一次调用返回可用结果；旧同义入口转 alias/internal |

完成这三个变更后再进入 Media Resolver 和资源 E2E，避免继续在不稳定的
消息模型上叠加发送、下载和 Event 逻辑。

## 2. 背景与现状

### 2.1 当前命令面

当前 `dws schema chat --compact --format json` 发布 78 个 Agent-visible IM 工具：

| 维度 | 当前数量 |
|---|---:|
| read | 32 |
| write | 44 |
| destructive | 2 |
| MCP | 65 |
| composite | 13 |
| confirmation=not_required | 76 |
| confirmation=user_required | 2 |
| availability=available | 78 |

Chat Shortcut 当前状态：

| 维度 | 当前数量 |
|---|---:|
| 已注册 Chat Shortcut | 91 |
| 公开 Shortcut | 91 |
| 隐藏 Shortcut | 0 |
| 已完成真实只读执行 | 33 个非空/有效结果 + 1 个合法空结果 |
| 已完成真实写入执行 | 43 个通过并回滚 |
| 其余真实执行状态 | 11 个外部 Fixture、3 个下层服务错误 |

以下 47 项是设计基线时期的旧隐藏真机结果分类，现不再决定可见性：

| 原因 | 数量 | 是否能直接证明命令不可用 |
|---|---:|---|
| missing-real-resource | 18 | 否 |
| backend-or-mcp-error | 17 | 否，需要区分 DWS 与上游 |
| input-or-business-validation | 9 | 否，需要区分负向用例与 binding 错误 |
| auth-or-permission | 3 | 否，只证明当前身份无权限 |

当前公开 Catalog 由 `status == real-ok` 决定。缺少测试群、消息、机器人、管理员权限或后端临时条件时，即使 CLI 契约正确，也会从 Help、Shortcut list 和 Skill 中被隐藏。

### 2.2 当前优势

DWS 已具备比通用消息 CLI 更宽的钉钉原生 IM 能力：

- 当前用户、机器人、Webhook 三种消息发送路径。
- 消息回复、编辑、撤回、单条/合并/话题转发、已读与发送状态。
- 收藏、关注、@我、未读、pin、top、reaction、文字表情。
- 群创建、解散、成员、管理员、自定义角色、机器人、邀请链接、入群审核。
- 会话分类、智能分类、免打扰、群昵称、群设置、外部群升级。
- 16 类当前用户身份的个人实时 IM 事件，支持多事件联合消费。
- `dws dev connect` 的本地 Agent 桥接、会话记忆、去重、速率限制和审批卡片能力。

这些原子能力继续由 Runtime Schema 完整发布。只有能提供参数收敛、目标
解析、编排、富化、安全或错误恢复增益的高价值意图，才进一步建设
Semantic Shortcut；其余由 Skill 直接路由到原子 leaf。

### 2.3 当前主要问题

#### 2.3.1 Shortcut 公开状态与真机证据耦合

旧规则把“当前测试环境未成功”错误地等同于“产品不应公开”。这会导致：

- 缺 fixture 的命令被隐藏；
- 权限型能力无法被具备权限的真实用户发现；
- 后端修复后 Catalog 仍可能长期停留旧状态；
- 已经实现的 `+chat-messages`、`+search-msg`、`+thread-replies` 等语义入口不可见；
- Skill 由公开 Catalog 生成后同步失去这些能力。

当前实现已经解除 evidence/availability 对 visibility 的机械耦合，并在
91 项逐项 reviewed 后全部公开。Agent 选择质量由 disposition、selection
语义和测试证据控制，而不是再次把 leaf 投影隐藏。

#### 2.3.2 投影可能静默丢失数据

多个 Shortcut 通过硬编码候选 key 定位列表和字段。未识别响应结构时返回空数组，进程仍 exit 0。对 Agent 而言，这与真实空结果无法区分。

本设计要求：

> 下层已确认存在业务数据时，上层投影不得静默为空；无法识别 shape 必须返回类型化 `projection_shape_drift`。

相关警示案例见 [shortcut-projection-fidelity.md](shortcut-projection-fidelity.md)。

#### 2.3.3 参数与输出缺少统一本体

当前同一概念可能使用：

- 会话：`group`、`conversation-id`、`open-conversation-id`、`cid`、`id`；
- 消息：`message-id`、`msg-id`、`open-message-id`、`ref-msg-id`；
- 用户：`user`、`users`、`open-dingtalk-id`、`receiver`、`new-owner`；
- 分页：`limit`、`size`、`count`、`cursor`、`page-size`、`page-token`；
- 时间：本地时间字符串、ISO-8601、秒或毫秒时间戳。

后端 property 本来可以随工具变化，但这些变化应由 binding/adapter 吸收，不应泄漏到 Surface。

#### 2.3.4 高频场景缺少统一读取和媒体链

现有 `search-advanced` 已支持关键词、发送者、@、会话和时间范围，但缺少：

- 自动分页；
- 批量获取完整消息；
- 群名、会话类型和发送人富化；
- reaction、reply、thread、forward 的统一输出；
- 可选资源下载；
- 单项富化失败隔离。

当前用户身份发送本地图片时按 file 附件发送，无法得到内联图片体验。资源下载支持任意本地输出路径和覆盖写入，缺少安全路径、分片、重试、MIME 与文件名推断。

#### 2.3.5 Raw/Schema 与 Shortcut 安全体验不一致

原子命令的普通 write 多数为 `confirmation=not_required`，但 Shortcut runner 对所有 write 和 high-risk-write 都进行交互确认。同一业务动作只因 Agent 选择了不同执行路径，就可能出现阻塞差异。

### 2.4 2026-07-27 outcome-graded 实测证据

外部评测《CLI评测-IM领域详细对比报告-20260727》使用
`dws v1.0.54` 与 `lark v1.0.77`，覆盖 T0～T7、文件往返、群管理、
卡片、加急和表情等场景。报告给出 dws 85.0、lark 89.9 的域深度分。
该分数只代表该版本、Profile 和评测模型下的相对结果，不作为跨版本
产品能力的绝对分数；原始 outcome 和复现步骤作为本设计的 evidence
输入。

#### 2.4.1 直接采纳的证据

| 实测结论 | 设计处置 | 优先级 |
|---|---|---:|
| 便捷层可能 exit 0 静默返空 | projection invariant、typed `projection_shape_drift`、observed-shape fixture | P0/P1 |
| `userId`、`openDingTalkId`、`openConversationId`、`mediaId` 增加 Agent 寻址负担 | TargetRef/IdentityRef、canonical flag、兼容 alias、name-aware resolver | P0 |
| `message list` 需要显式时间与方向才能稳定得到预期窗口 | 合理默认时间窗、明确 direction、标准分页和截断 | P1 |
| 搜索实时可命中，历史“索引不可用”判断属于错误测法 | 撤销错误 blocker；继续优化搜索输出深度而不是重复修索引 | P1 |
| 当前用户身份可查询已读状态 | 保留并产品化为 DWS 差异化读取能力 | P1/P5 |
| 图片/文件往返是 Agent 高频任务，DWS 媒体体验显著弱于 Lark | 资源与媒体 E2E 提升到 P2 | P2 |
| `openTaskId` 只代表受理，真实送达需要查询发送状态 | 标准 DeliveryStatus、可选等待/轮询 Smart Shortcut | P3 |
| 流式卡片、按姓名直发、@我聚合、DING、会话分类是 DWS 优势 | 在 Golden Path 和 Skill 中明确保留，不因对齐而降级 | P3/P5 |

#### 2.4.2 需要修正的归因

| 报告归因 | 修正 |
|---|---|
| “能力覆盖打平，因此差距全在 D4a” | 16 个粗粒度能力块只能证明“有入口”，不能证明分页、富化、错误恢复、资源安全、Event、Hint 和 Skill 深度对等 |
| “给 DWS ID 加 Lark 式前缀” | 上游原生 ID 不改写；使用 typed reference 或显式 target type 提供自描述语义 |
| “Lark Shortcut 统一使用 `receive_id_type`” | `receive_id_type` 主要是 API 层概念；当前 Lark `+messages-send` 使用 `--chat-id` / `--user-id` 并由 binding 推导 |
| “三种发送命令应合并为 `--as user\|bot\|webhook`” | user、app bot、Webhook 的认证、目标、风险和回执不同；保留独立原子 CLI leaf，可增加一个负责身份路由的 Smart Shortcut |
| “媒体问题只是 AppKey 门禁” | 当前代码已将旧 `chat media upload` 下线；本地文件可发送，但本地图片只能作为 file 附件，真正缺口是受支持的内联图片上传与统一 Media Resolver |
| “Shortcut 越少越容易选择” | 数量不是唯一指标，但无语义增益的 1:1 Shortcut 会增加选择噪音；只保留有明确 semantic delta 的入口 |

#### 2.4.3 转为 Golden Scenario 的评测用例

以下场景进入 BASE-01，保留版本、身份、参数、原始响应、规范输出和
期望 outcome：

1. 当前用户发送消息 → `openTaskId` → 查询真实送达状态；
2. 群消息读取的默认时间窗、older/newer 方向与分页；
3. 消息搜索实时命中，并返回完整消息和会话上下文；
4. userId/openDingTalkId/openConversationId/name 的目标解析；
5. 本地图片内联发送、本地文件发送、资源下载和 md5 校验；
6. 当前用户已读状态；
7. 流式卡片、@我聚合和 DING 等 DWS 优势场景。

## 3. 设计目标与非目标

### 3.1 目标

1. 建立 API、CLI、Semantic Shortcut、Hint、Skill 和 Event 之间清晰、单向、可验证的契约。
2. Runtime Schema 保证原子 IM 能力完整；91 个已审阅 Shortcut 全部公开，
   并用 disposition 让高价值语义入口获得明确的选择优先级。
3. 分离 visibility、availability 和 evidence；Fixture 或下层错误必须可见，
   但不机械改写 reviewed + available Shortcut 的公开状态。
4. 建立统一的 Message、Conversation、Target、Identity、Resource、Pagination、DeliveryStatus 和 PartialError typed model。
5. 形成消息读取、搜索、媒体上传、发送、回复、资源下载和事件消费的高质量 Golden Path。
6. 在保持现有 wire compatibility 的前提下统一参数和输出。
7. 对投影丢失、分页截断、局部富化失败和文件安全建立强制门禁。
8. 保留并增强钉钉特有的群治理、个人事件和本地 Agent 桥接优势。

### 3.2 非目标

1. 不以命令数量代替效果指标；91 项全量公开后仍以可执行率、选择正确率、
   投影保真和真实结果为准。
2. 不再机械生成新的 1:1 包装来追平 lark-cli 名称；已有 Schema leaf
   投影保留公开，并通过 disposition 与 Smart 主入口区分。
3. 不把 Shortcut 合并进 Runtime Schema 作为本期前置；两套 Catalog 可以暂时保留，但必须相互导航、共享参数与安全事实。
4. 不在未版本化的情况下破坏现有 JSON wire contract。
5. 不把上游 MCP 缺能力、权限不足或缺测试 fixture 伪装成 DWS CLI 可独立修复的问题。
6. 不手工修改生成的 `schema_agent_metadata/` 或 `schema_catalog.json`。

## 4. 核心设计原则

### 4.1 Shortcut 是正式语义层

语义执行层分为三种形态：

| 类型 | 定义 | 价值 |
|---|---|---|
| `semantic_adapter` | 可只绑定一个执行路径，但必须比原子 leaf 增加可验证的语义能力 | 参数归一、强校验、标准输出、安全或错误恢复 |
| `smart_composite` | 编排多个调用或本地处理步骤 | 目标解析、分页、富化、聚合、资源处理、轮询、局部失败 |
| `recipe` | 跨产品、长运行或需要用户决策的场景流程 | 自然语言路由、确认、监听、跨产品协作 |

调用 API 的数量不用于判断价值，但仅做改名和透传不构成 semantic delta。
原子能力的完整发现由 Runtime Schema 负责。

### 4.2 双向完整性

IM 能力面必须满足：

1. 每个公开原子 CLI leaf 进入 Runtime Schema 或 exact reviewed exclusion；
2. 每个公开 Shortcut 必须声明相对原子 leaf 的 `semantic_delta`；
3. 每个 Shortcut 必须绑定真实可运行的 CLI/API/MCP 路径；
4. 一个 Smart Shortcut 可以绑定多个 CLI/API/MCP 路径；
5. 同一用户意图只保留一个 primary，兼容名称作为 alias/deprecated；
6. Skill 必须能在“直接原子 leaf”和“Semantic Shortcut”之间做明确选择。

### 4.3 Golden Path 决定优先级，不决定能力边界

Golden Path 获得：

- Skill 默认路由；
- 更完整的 observed-shape、live 和语义选择测试；
- 标准分页、富化、错误恢复和资源闭环；
- 更严格的性能和可用性目标。

长尾原子能力仍然通过 Schema 公开、可发现、可执行。当用户表达精确
操作时，Agent 可以直接选择对应 CLI leaf，不要求创建长尾 Shortcut。

### 4.4 公开状态与证据状态分离

Shortcut 至少包含两个正交维度：

Availability 回答“运行时是否支持”，Visibility 则是经过审阅的显式产品
决策。当前 Chat 策略为：`reviewed=true` 且 `availability=available` 的
91 项全部公开；semantic relation 通过 disposition 表达并参与 Agent
路由优先级，不再作为隐藏 leaf/alias 的开关。

#### Availability

| 状态 | 含义 | 默认公开 |
|---|---|---|
| `available` | 契约有效，当前发行版支持 | 是 |
| `runtime_gated` | 契约有效，但依赖 edition、身份、权限或运行时条件 | 是，展示条件 |
| `unavailable` | 当前发行版真实不可执行 | 否 |
| `deprecated` | 已有稳定替代路径 | 否或仅兼容显示 |

#### Evidence

| 状态 | 含义 |
|---|---|
| `contract_valid` | Cobra、flags、constraints、binding、风险契约通过 |
| `mock_verified` | observed response shape 与投影测试通过 |
| `live_verified` | 指定身份与 fixture 真机通过 |
| `live_blocked` | 缺 fixture、权限、索引延迟或后端条件 |
| `live_failed` | 真机执行暴露确定性产品或后端问题 |

`live_blocked` 不得机械改写 Availability。

### 4.5 Surface 与 Binding 分治

- Surface 统一人和 Agent 使用的 flag。
- Binding/adapter 吸收 `openConversationId`、`cid`、`openMsgId` 等后端差异。
- Native annotation 只做一致性断言，不作为身份或参数回退源。
- 未确认后端真实 property 前，禁止机械改拼写。

### 4.6 输出兼容优先

- 内部使用 typed model。
- 现有 serializer 保持已有字段。
- 新标准字段优先做兼容性新增。
- 无法兼容的结构变化必须通过明确版本或新 projection mode 发布。
- Raw payload 不作为正常输出；仅在诊断模式输出脱敏 evidence。

### 4.7 安全事实独立解析

`effect`、`risk`、`confirmation`、`idempotency` 分别解析，不从其他字段机械推导。

- 普通、用户明确请求的 write 不额外制造阻塞式 CLI 确认。
- destructive/high-risk 且 runtime gate 要求确认时，必须 `--yes`。
- Raw、CLI、Shortcut、Schema、Hint 和 Skill 的最终安全语义必须一致。

### 4.8 Event 身份面分离

当前个人事件使用当前用户 OAuth 身份。未来 bot/app 事件必须建立独立 Catalog、Schema 和 Skill 路由，不得把 bot/app callback 伪装成个人事件。

## 5. 目标架构

```text
用户 / Agent 意图
  │
  ├─ Skill / Recipe
  │    └─ 选择高价值语义入口或精确原子 leaf
  │
  ├─ Semantic Shortcut Registry
  │    ├─ semantic_adapter
  │    ├─ smart_composite
  │    └─ selection / safety / evidence / output contract
  │
  ├─ CLI Command Registry + Cobra
  │    ├─ canonical CLI path
  │    ├─ flags / constraints
  │    └─ executable truth
  │
  ├─ IM Adapter / Typed Normalizer
  │    ├─ identity binding
  │    ├─ request mapping
  │    ├─ response shape normalization
  │    └─ typed errors / partial failures
  │
  └─ MCP / OpenAPI / Personal Event Stream
```

### 5.1 Semantic Shortcut Registry

建议在现有 `Shortcut` 声明模型上逐步补齐以下类型信息：

```go
type SemanticKind string

const (
    SemanticAdapter SemanticKind = "semantic_adapter"
    SmartComposite SemanticKind = "smart_composite"
    Recipe         SemanticKind = "recipe"
)

type EvidenceStatus string

const (
    ContractValid EvidenceStatus = "contract_valid"
    MockVerified EvidenceStatus = "mock_verified"
    LiveVerified EvidenceStatus = "live_verified"
    LiveBlocked  EvidenceStatus = "live_blocked"
    LiveFailed   EvidenceStatus = "live_failed"
)

type ShortcutSpec struct {
    Service          string
    Command          string
    CanonicalID      string
    Kind             SemanticKind
    SemanticDelta    []string
    Bindings         []ExecutionBinding
    Parameters       []SemanticParameterSpec
    Constraints      []Constraint
    OutputRef        string
    Effect           string
    Risk             string
    Confirmation     string
    Idempotency      string
    Availability     string
    Evidence         []EvidenceRecord
    Selection        SelectionSpec
}
```

这里的类型只表达目标结构，实际实现应复用现有 `Shortcut`、`Flag`、`Constraint` 和 RuntimeContext，避免一次性重写框架。

### 5.2 单向发布

Shortcut 数据流应保持单向：

```text
Shortcut source declarations
  + reviewed semantic metadata
  + exact execution bindings
  + evidence records
      ↓
Typed Shortcut Registry
      ↓
runtime mounting / shortcut list / public catalog / Skill visible table
```

运行时、Skill 生成器和公开 Catalog 不得分别重新解释风险、参数或公开状态。

## 6. 标准数据模型

### 6.1 Message Normalized V1

内部 typed model 建议至少包含：

```json
{
  "message_id": "string",
  "conversation_id": "string",
  "conversation_type": "group|o2o|topic|unknown",
  "sender": {
    "display_name": "string",
    "user_id": "string",
    "open_dingtalk_id": "string",
    "sender_type": "user|bot|system|unknown"
  },
  "create_time": "RFC3339 string",
  "create_time_raw": "backend value",
  "message_type": "text|markdown|image|file|audio|video|card|reply|forward|unknown",
  "text": "string",
  "resources": [],
  "reply": {},
  "thread": {},
  "forwarded": [],
  "reactions": [],
  "warnings": []
}
```

原则：

- 缺失字段不伪造。
- 加密消息输出明确状态，不输出无意义密文。
- card/post 尽可能输出可读文本，同时保留类型。
- forwarded、reply、thread 使用递归或引用结构，但必须限制深度和数量。
- 后端已有 sender name 时优先使用，不为补名字强制发起大量联系人调用。

### 6.2 Resource Normalized V1

```json
{
  "resource_id": "string",
  "media_id": "string",
  "resource_type": "image|file|audio|video|unknown",
  "name": "string",
  "mime_type": "string",
  "size_bytes": 0,
  "checksum": {
    "algorithm": "md5|sha256",
    "value": "string"
  },
  "source": "existing_key|local|url|message",
  "downloadable": true,
  "download_hint": {
    "message_id": "string",
    "conversation_id": "string"
  },
  "local_path": "string",
  "error": false
}
```

### 6.3 Pagination Normalized V1

Serializer 在兼容旧字段的同时，统一提供：

```json
{
  "count": 20,
  "has_more": true,
  "next_token": "string",
  "truncated": false,
  "page_count": 1,
  "warnings": []
}
```

### 6.4 Partial failure

消息主体、reaction、发送人、会话上下文、forward 子消息和资源下载分别处理：

- 主消息获取失败：命令失败；
- 可选富化失败：主结果保留，在 `warnings`/`partial_errors` 中记录；
- 批次部分失败：只标记失败项；
- 禁止捕获错误后伪装成空列表；
- warning 必须结构化，并可在 stderr 输出简短诊断。

### 6.5 Delivery Status V1

DWS 当前用户发送可能先返回 `openTaskId`，它表示任务受理而不是最终送达。
标准发送结果需要显式区分：

```json
{
  "accepted": true,
  "delivery_state": "accepted|pending|succeeded|failed|unknown",
  "task_id": "string",
  "message_id": "string",
  "failure_reason": "string",
  "checked_at": "RFC3339 string"
}
```

- 原子 send 不伪造最终送达；
- 支持送达查询的身份面应返回 `task_id` 和下一步提示；
- Smart Shortcut 可提供 `--wait-delivery` 和有界轮询；
- 超时返回 `pending/unknown`，不得改写为 succeeded；
- 不支持送达查询的后端明确 `delivery_state=accepted`，并说明只代表受理。

## 7. 参数本体与迁移

参数标准化沿用 [dws-parameter-ontology.md](dws-parameter-ontology.md) 的 Surface/Binding 分治原则。本设计补充以下决策约束。

### 7.1 推荐概念

| 概念 | 推荐 Surface | 兼容 alias | 备注 |
|---|---|---|---|
| 任意会话 ID | `--conversation-id` | `--open-conversation-id`、`--cid`、`--id` | 单聊/群聊共同使用 |
| 群名或待解析群引用 | `--group` | 现有名称型 flag | 语义层负责解析；不得与 ID 静默混淆 |
| 消息 ID | `--message-id` | `--msg-id`、`--open-message-id` | 引用消息可保留 `--ref-message-id` |
| 内部用户 ID | `--user-id` | `--user` | 与 openDingTalkId 严格区分 |
| 开放身份 | `--open-dingtalk-id` | 现有精确别名 | 外部联系人、跨组织、机器人 |
| 机器人编码 | `--robot-code` | `--robot` | 与 openBotId 分离 |
| 搜索关键词 | `--query` | `--keyword` | 对外统一 |
| 开始/结束时间 | `--start` / `--end` | 旧 time/range flags | Surface 使用 ISO-8601 |
| 单页数量 | `--page-size` | `--limit`、`--size`、`--count` | 分阶段迁移 |
| 分页 token | `--page-token` | `--cursor` | 后端 cursor 由 binding 处理 |
| 自动分页 | `--page-all` | 无 | 必须配最大页数 |
| 自动分页上限 | `--page-limit` | 无 | 防止无界调用 |
| 幂等键 | `--idempotency-key` | `--uuid` | 统一长度/格式校验 |

`--conversation-id` 与现有参数字典中的 `--conversation` 需要在实施前完成一次 ADR 决策。未决策前不得机械批量改名。

### 7.2 TargetRef 与 IdentityRef

DingTalk 上游 ID 不具有 Lark `ou_`、`oc_`、`om_` 一类稳定类型前缀，
CLI 不应修改、拼接或伪造原生 ID。语义层使用 typed reference 让目标
和执行身份自描述：

```text
TargetRef {
  kind: user | conversation
  id_type: user_id | open_dingtalk_id | open_conversation_id | name
  id: string
  display_name?: string
  source: explicit | resolved
}

IdentityRef {
  kind: current_user | app_bot | webhook
  auth_surface: oauth | app | webhook_token
  identity_id?: string
}
```

规则：

- Raw/CLI leaf 可以继续使用精确的 `--user-id`、`--open-dingtalk-id`、
  `--conversation-id`；
- 高频 Smart Shortcut 可以提供 `--target` 与枚举化 `--target-type`，
  由 binding 映射到真实 leaf；
- `--target-type=auto` 只允许基于明确 flag、typed ref 或无歧义 observed
  格式推导，无法判断时必须要求补充，不可猜测；
- `receive_id_type` 属于接口 binding 事实，不机械暴露为所有 Shortcut
  的统一 Surface；
- user、app bot、Webhook 使用不同 IdentityRef。Webhook 是独立认证和
  传输路径，不伪装成普通 `--as` 身份切换；
- 标准输出同时返回 `target.kind`、`target.id_type`、`target.id`，已有
  原始 ID 字段按兼容策略保留。

### 7.3 名称解析与 ID 输入

名称和 ID 不应共用一个无法判断语义的参数：

- 已知 ID：传明确 ID flag；
- 只有群名：使用 name-aware Semantic Shortcut；
- 只有人名：先通过 aisearch/contact 解析，重名必须确认；
- `--user-id` 与 `--open-dingtalk-id` 严格二选一；
- 写操作不得自动跨组织搜索后直接执行。

### 7.4 迁移策略

1. 新主 flag 与旧 alias 同时注册。
2. Help 只展示主 flag，alias 隐藏。
3. Schema/ShortcutSpec 只发布主 flag，并在 provenance/compatibility 中记录 alias。
4. 运行时统一归一到 canonical value。
5. 绑定测试确认每个 tool 的真实后端 property。
6. 至少经过一个兼容周期后才能考虑删除 alias。

## 8. 功能与 CLI 优化

### 8.1 读取平面

#### IM-READ-01：统一消息转换器

范围：

- `message list`
- `message list-all`
- `message list-by-ids` / mget
- `message search` / `search-advanced`
- topic/thread replies
- @me、focused、favorites、pin 等消息列表

验收：

- 共用 Message Normalized V1；
- 同一真实消息在不同入口的核心字段一致；
- 未识别 shape 返回 `projection_shape_drift`；
- raw non-empty → normalized non-empty invariant；
- card、forward、reply、encrypted 等真实 observed shape 有测试。

#### IM-READ-02：统一分页

范围：

- `page-size/page-token/page-all/page-limit`；
- 下一页 token；
- 截断提示；
- 最大页数与请求间隔；
- 后端不支持自动分页时明确说明。

验收：

- `--page-all` 永不无界；
- 因 page-limit 截断时 `truncated=true`；
- 单页模式保留 next token；
- 空结果与未知 shape 可区分。

#### IM-READ-03：高保真消息搜索

流程：

```text
search
  → auto paginate
  → batch mget details
  → conversation context
  → sender/reaction enrichment
  → normalized output
```

可选富化失败不得丢失搜索命中。

#### IM-READ-04：消息上下文

补充：

- reply/reference context；
- thread reply 摘要或展开；
- merge/forward 子消息；
- reaction；
- conversation name/type；
- sender display name；
- update/read status（后端支持时）。

### 8.2 写入与媒体平面

#### IM-WRITE-01：统一发送能力

当前用户、bot、Webhook 保留不同原子 CLI leaf，但共享：

- TargetRef/IdentityRef；
- target validation；
- text/markdown/content 选择；
- mention 归一；
- idempotency；
- dry-run 计划；
- 标准发送结果；
- typed errors。

不强行用一个 `--as` 参数合并语义不同、权限不同、响应不同的发送后端。
真正语义相同的入口通过 primary/alias 收敛；高频统一入口可以是
Smart Shortcut。精确身份和传输方式继续由原子 leaf 表达，不再为每条
路径额外复制 1:1 Shortcut。

#### IM-WRITE-02：内联图片与富媒体

当前基线中，历史 `chat media upload` 已下线为兼容 stub；本地路径可
通过 `message send --msg-type file --file-path` 发送，但图片会显示为
可下载 file，不会生成 mediaId 或渲染为内联 image。因此目标不是恢复
旧 AppKey 门禁，而是建立受支持、可测试的资源上传与发送闭环。

支持：

- 已有 mediaId；
- cwd/workspace 相对本地图片；
- 本地文件；
- URL 图片/文件；
- 音频和视频的类型/大小校验；
- 上传失败时明确失败或经评审的降级行为。

URL 下载必须经过 SSRF、安全域名和大小限制。

验收：

- local/URL/existing key 使用同一个 Media Resolver；
- 本地图片默认按内联 image 发送，显式 `--as-file` 才降级为附件；
- 上游暂不支持内联上传时返回 typed capability error，不静默改成 file；
- 上传结果返回标准 `ResourceRef`、size、MIME、checksum 和 source；
- 图片/文件发送后可通过消息读取和资源下载完成 md5 闭环。

#### IM-WRITE-03：富媒体回复

在现有引用回复基础上支持标准消息内容和媒体 resolver。引用 ID、引用发送者、目标会话必须严格验证。

#### IM-WRITE-04：幂等与安全

- 幂等键统一校验；
- destructive/high-risk 才强制 runtime confirmation；
- 普通 write 的 Raw 与 Shortcut 行为一致；
- dry-run 不上传、不发送、不注入 `--yes`。

### 8.3 资源下载

#### IM-RESOURCE-01：安全文件路径

- 默认只允许 workspace/cwd 相对路径；
- 拒绝绝对路径和 `..`；
- 若产品确需绝对路径，使用显式、经评审的 opt-in；
- 默认 no-clobber；
- 显式 `--overwrite` 才允许覆盖；
- 临时文件写完并校验后原子 rename。

#### IM-RESOURCE-02：可靠下载

- Content-Disposition 文件名；
- MIME 扩展名推断；
- 唯一 basename；
- Range/chunk 下载；
- 重试与超时；
- Content-Range、大小和完成度校验；
- 失败清理临时文件。

#### IM-RESOURCE-03：批量和自动下载

读取类 Shortcut 可提供 `--download-resources`：

- 默认关闭；
- 固定安全目录；
- 有界并发；
- 单资源失败隔离；
- 输出 `local_path` 或 `error=true`；
- 主消息列表不因下载失败而丢失。

## 9. Semantic Shortcut 优化

### 9.1 完整性审计

建立 IM Semantic Value Matrix：

| 字段 | 说明 |
|---|---|
| intent_id | 稳定用户意图身份 |
| shortcut | `chat +...` |
| kind | semantic_adapter / smart_composite / recipe |
| semantic_delta | 相对原子 Schema leaf 新增的语义价值 |
| primary_cli_path | 主要 CLI 绑定 |
| extra_bindings | 额外 API/MCP/本地步骤 |
| parameters | 标准参数 |
| output_ref | 标准输出模型 |
| safety | effect/risk/confirmation/idempotency |
| selection | use_when/avoid_when/examples |
| availability | available/runtime_gated/unavailable/deprecated |
| evidence | contract/mock/live |
| public | 最终是否公开 |

审计输出：

- 只做原子 leaf 改名/透传、没有 semantic delta；
- Shortcut 有、执行绑定不清；
- 语义重复但无 primary；
- 一个 Shortcut 混合不相容语义；
- 仅因真机 fixture 被隐藏；
- 值得升级为 Smart Shortcut；
- 应改为 alias/deprecated/internal；
- 缺参数约束；
- 缺投影或标准输出；
- 缺安全事实或与 runtime 不一致。

### 9.2 语义重复与 primary/alias 处置

命令或 Shortcut 数量不能直接证明冗余。每组疑似重复入口必须按以下
分类评审：

| 分类 | 判定 | 处置 |
|---|---|---|
| 同一语义、同一效果、同一主要输出 | 仅名称或历史路径不同 | 选一个 primary，其余 alias/deprecated |
| 同一用户目标、不同身份或认证 | current user / app bot / Webhook | 原子 leaf 保持独立；只在高频场景增加统一身份路由 Shortcut |
| 同一资源、不同操作范围 | 全群禁言 / 单成员禁言 / @all 免打扰 | 原子 leaf 保持精确；有自然语言歧义时由 Skill 路由 |
| 同一数据集合、不同稳定视图 | 全部会话 / 置顶会话 / 分类会话 | 优先一个 Smart Shortcut + 过滤器；精确原子 leaf 继续由 Schema 暴露 |
| 多个调用才能完成高频意图 | 名称解析、上传发送、发送后查送达 | 增加 smart_composite，底层能力保留在 Schema，不重复包 Shortcut |

审计必须输出 `semantic_relation=primary|alias|distinct|composite` 和理由。
禁止仅以“数量多”“Lark 命令少”或底层 API 数量作为合并依据。

### 9.3 Semantic Shortcut Definition of Done

每个公开 Shortcut 至少满足：

1. 稳定 canonical semantic identity；
2. 非空 `semantic_delta`，且至少属于参数归一、目标解析、编排、分页、
   富化、资源处理、安全增强或错误恢复之一；
3. 面向用户和 Agent 的名称与描述；
4. 明确 use_when、avoid_when，以及何时直接使用原子 leaf；
5. 标准主参数和隐藏兼容 alias；
6. required、enum、mutually-exclusive、require-together 等约束；
7. ID、时间、分页、内容或状态校验；
8. 标准输出和 typed error；
9. safety 与 runtime 一致；
10. dry-run 或明确不支持原因；
11. contract、observed-shape 和 selection test；
12. evidence status。

只做命令改名、flag 改名或无损透传，且以上语义增益均为空的，不应作为
新的公开 Shortcut。

### 9.4 公开机制重构

旧逻辑：

```text
real-ok → public
其他 → hidden
```

新逻辑：

```text
contract valid
  + runnable binding
  + semantic_delta 非空
  + availability != unavailable/deprecated
    → public

live result
    → evidence only
```

公开 Catalog 应展示或输出 evidence，而不是使用 evidence 删除能力。

### 9.5 原 47 个隐藏 Shortcut 的最终处置

这些入口已经完成逐项审阅并全部公开。四类处置继续作为 Agent 路由和
兼容关系，而不再作为 visibility 开关：

| 处置 | 判定 | 示例方向 | 发布动作 |
|---|---|---|---|
| 升级为 Smart | 高频意图，需要解析、翻页、富化、资源或轮询 | history、search、resource download、name-aware send | 公开，并作为 primary 默认候选 |
| 保留 Semantic Adapter | 虽只绑定一个 leaf，但确有标准参数、强校验、标准输出或安全增益 | 少数稳定、用户直接表达的入口 | 公开，优先于同义原子路径 |
| Schema leaf 投影 | 只做改名/透传，或属于低频精确操作 | reaction、pin/top、单项群设置等原子操作 | 公开，用于精确意图，低于 Smart 路由优先级 |
| Alias/Internal relation | 与 primary 重复，或只为历史兼容 | 同义名称、旧参数形态 | 公开兼容，但必须明确 primary |

审计顺序：

1. 先处理读取候选：`+chat-messages`、`+messages-list`、`+search-msg`、
   `+thread-replies`、`+messages-resource-url`，收敛为 history、search 和
   resource download 三个 primary；
2. 再处理发送候选，收敛为 name-aware send，并保留 user/bot/Webhook
   原子 leaf 的身份差异；
3. reaction、forward、conversation setting 和群治理逐族判断是否有
   semantic delta；没有则直接由 Schema/Skill 承担；
4. destructive/high-risk 候选只有在确实需要语义编排时才做 Shortcut，
   且必须具备 runtime gate、目标摘要、dry-run 和恢复说明；
5. 真机缺 fixture 标记为 `live_blocked`，下层错误标记为 `live_failed`；
   二者都不会让 reviewed + available 命令再次隐藏。

### 9.6 Smart Shortcut

第一阶段只建设以下五个高价值入口：

| Smart Shortcut | 组合价值 |
|---|---|
| 高保真 messages search | 自动分页、mget、会话、发送人、reaction 富化 |
| 高保真 message history | 群/单聊统一、分页、标准 Message |
| resource download | 从消息提取资源、安全批量下载、局部失败 |
| name-aware send | 人名/群名解析、歧义确认、上传、发送 |
| listen-and-reply recipe | event consume、过滤、去重、防循环、安全回复 |

Skill 根据用户意图在这五个入口和 Runtime Schema 原子 leaf 之间选择。

## 10. Hint 优化

### 10.1 Selection

文件：`internal/cli/schema_hints/selection/chat.json`

任务：

- send/reply/list/search/mget/download 的 use_when/avoid_when 去冲突；
- 区分搜索群、搜索消息、按 ID 获取消息；
- 区分单聊/群聊、当前用户/bot/Webhook；
- 区分查询历史与实时事件；
- 区分会话 mute、全员 mute、成员 mute；
- 区分执行 recall 与监听 recall；
- Golden Path 获得默认正向场景；
- 长尾精确操作保留原子 leaf 的精确正向场景；
- 每工具至少一个正向和一个负向场景。

### 10.2 Metadata

文件：`internal/cli/schema_hints/metadata/chat.json`

任务：

- effect、risk、confirmation、idempotency 逐项复核；
- runtime gate 与真实确认点一致；
- parameter overlay 引用真实 runnable leaf 和真实 flag；
- 13 个 composite interface 逐个审计；
- 能绑定真实 pinned MCP 的不得继续标 composite；
- 无单一 pinned interface 的保留明确 reason；
- 审计 metadata 79 项与 selection/Schema 78 项的差异；
- `chat.get_group_members` 等孤立记录必须明确保留原因或删除。

### 10.3 CommandRegistry

仅在以下情况修改：

- canonical identity 变化；
- primary CLI path 变化；
- alias 变化；
- navigation 变化。

参数描述、Skill、selection、safety 或 metadata-only 变化不得机械改写 CommandRegistry。

### 10.4 Shortcut Hint

Shortcut 应拥有与 Runtime ToolSpec 同等级的 selection 和 safety 事实，但保持独立的 typed registry。短期通过 ShortcutSpec 发布；是否进入统一 `dws schema` 需要单独 ADR 和版本设计。

## 11. Skill 优化

### 11.1 顶层 Skill 只承担路由

`skills/multi/dingtalk-chat/SKILL.md` 顶层保留：

- 适用/不适用边界；
- Golden Path；
- 身份决策；
- 名称解析与多组织规则；
- 安全规则；
- Chat/Event/Contact/AISearch/Drive/DING/Mail 跨产品路由；
- Topic index。

原子命令事实来自 Runtime Schema，高价值语义入口来自 typed Shortcut
Catalog；Skill 不手工维护两套完整命令表。

### 11.2 参考文档拆分

将大 Chat reference 按主题拆分：

```text
references/
  messaging-send-reply.md
  messaging-read-search.md
  messaging-media.md
  messaging-reactions-forward.md
  conversations.md
  groups-members.md
  groups-admin.md
  cards-bots-webhooks.md
```

每个 reference 只包含该主题的：

- semantic shortcut；
- 原子回退路径；
- 参数和输出；
- 常见错误；
- 真实限制；
- 不适用场景。

### 11.3 Golden Path 路由

| 用户意图 | Skill 默认 |
|---|---|
| 发消息给人/群 | name-aware send 或精确 Semantic Shortcut |
| 查聊天记录 | 高保真 history |
| 搜历史消息 | 高保真 messages search |
| 按消息 ID 查 | mget |
| 下载消息资源 | resource download |
| 实时监听 | dingtalk-event |
| 监听后回复 | listen-and-reply recipe |
| 群治理精确操作 | 对应 Runtime Schema 原子 leaf；只有存在 semantic delta 时使用 Shortcut |

### 11.4 Mono/Multi 一致性

- 共享结构化场景事实；
- 自动校验命令、flags、Shortcut visibility 和安全；
- mono/multi 可以有不同上下文密度，但不得有不同执行事实；
- Skill 升级 stable 前必须通过 dispatch、selection、examples 和真机 Golden Scenario。

## 12. 事件消费优化

### 12.1 保留现有稳定契约

以下契约不得破坏：

- `event consume` 为长连接；
- stdout 只输出事件；
- stderr 输出 ready/status/debug/error；
- ready line 包含 event_key、bus_pid、subscribe_id；
- 受控退出输出 exited line；
- `--max-events`、`--duration` 可有界退出；
- 新建订阅在干净退出时自动清理；
- `--subscribe-id` 复用语义保持；
- `event schema <key>` 描述 payload；
- `dws schema "event consume"` 描述 CLI 参数。

### 12.2 Event Payload Normalized V1

当前 16 类个人事件应进一步收敛为共享模型：

```json
{
  "event_key": "string",
  "event_time": "RFC3339 string",
  "subscription_id": "string",
  "conversation_id": "string",
  "message_id": "string",
  "sender": {},
  "operator": {},
  "message": {},
  "resource_refs": [],
  "operation": "receive|read|recall|reaction",
  "raw_event_type": "string"
}
```

现有 flatten 输出保持兼容；新字段以兼容新增或版本化 schema 提供。

### 12.3 媒体事件闭环

媒体消息事件应输出结构化 resource reference，Skill 路由到安全 resource download，而不是要求 Agent 从 content 文本中解析 mediaId。

### 12.4 新事件面

在上游支持且身份边界明确时逐步增加：

- card action；
- bot added/deleted；
- user joined/left/removed；
- chat updated/disbanded；
- bot/app message receive。

bot/app 事件使用独立 Catalog 和 Skill，不与个人事件合并命名。

### 12.5 监听并回复

先以 recipe 提供：

```text
consume
  → wait ready
  → normalize event
  → filter
  → dedup
  → rate limit
  → resolve reply target
  → semantic send
  → graceful shutdown
```

安全要求：

- self-loop 防护；
- message ID 去重；
- sender/group allowlist；
- 最大并发和速率；
- 最大重试；
- 写操作遵循 profile/组织边界；
- 消费结束时清理订阅；
- 不用轮询历史模拟事件。

### 12.6 可观测性

`event status` 或 metrics 增加：

- connection state；
- reconnect count；
- last event time；
- received/dropped count；
- queue depth；
- active consumers；
- subscription age；
- last error category。

## 13. API/MCP 协同边界

问题必须先归因：

| 根因 | DWS 责任 | 上游责任 |
|---|---|---|
| Cobra flag/constraint 错误 | 修 CLI | 无 |
| flag→property binding 错误 | 修 adapter/binding | 提供真实契约证据 |
| projection shape 错误 | 修 normalizer | 提供 observed response |
| MCP 未透传参数 | 建契约测试与分单 | 修 MCP |
| MCP response envelope 冲突 | 兼容明确版本 | 修稳定契约 |
| OpenAPI 不支持 | 不伪造能力 | 产品/开放平台决策 |
| fixture/权限不足 | 标 live_blocked | 提供 fixture/权限 |

涉及 `dingtalk-supcon-mono` 的任务应使用同一个 case ID、tool、参数形状、trace_id、期望 contract 和脱敏 observed response，避免两个仓库各自维护模糊描述。

## 14. 工作包

状态定义：

- `CLOSED`：精确 DoD 已由 `main` 代码与门禁证明，后续变更由回归门禁守护；
- `PARTIAL`：已有可复用实现，但尚未满足完整 DoD；
- `OPEN`：未进入 `main`，或核心出口尚未实现；
- `BLOCKED`：已完成本地可做部分，剩余能力依赖明确的上游契约。

Schema 被合入只关闭 Schema 自身的身份、参数、选择和发布契约，不会自动
关闭 CLI 运行时能力、Shortcut 语义层、Skill、Normalized Output 或 Event
业务 payload。

### 14.1 Runtime Schema 平台基线

| ID | 任务 | 状态 | 关闭证据 |
|---|---|---|---|
| SCH-01 | Reviewed CommandRegistry 与 Cobra 精确绑定 | `CLOSED` | canonical/primary/alias/navigation 单一评审源；不存在、不可运行和冲突路径 fail closed |
| SCH-02 | Schema 双向完整性与精确 exclusion | `CLOSED` | 603 个 registry command 与 603 个最终 ToolSpec 完整交付；公开 Cobra leaf 必须进入 Schema 或 exact exclusion |
| SCH-03 | Typed ToolSpec、SchemaRegistry 与单向发布 | `CLOSED` | leaf、summary、`schema --all` 和 Catalog 共用同一 resolved typed registry |
| SCH-04 | Metadata/Selection 分治与参数投影 | `CLOSED` | reviewed selection、metadata、parameter binding 按确定优先级解析并携带 provenance |
| SCH-05 | 生成漂移、Help 参数与安全事实门禁 | `CLOSED` | 2026-07-28 在 `main@2dfc39f0` 通过 generated drift、Schema Catalog、Help exact-set 和 runtime confirmation truth |

### 14.2 基础契约

| ID | 任务 | 依赖 | 状态 | Done |
|---|---|---|---|---|
| BASE-01 | IM Golden Scenarios | 无 | `OPEN` | 固定七类核心场景与 fixtures |
| BASE-02 | 参数 ADR | 无 | `OPEN` | canonical/alias/binding 规则通过评审 |
| BASE-03 | Message/Resource/Page/Delivery model | BASE-02 | `OPEN` | typed model 与兼容 serializer |
| BASE-04 | Shortcut publication policy | 无 | `CLOSED` | reviewed visibility + availability 决定发布；91/91 Chat Shortcut 公开，evidence 独立展示 |
| BASE-05 | 效果指标基线 | BASE-01 | `OPEN` | TSR 子指标可重复计算 |
| BASE-06 | TargetRef/IdentityRef | BASE-02 | `OPEN` | 原生 ID 不改写，目标和执行身份 typed、自描述 |

### 14.3 CLI 与能力

| ID | 任务 | 依赖 | 状态 | Done |
|---|---|---|---|---|
| CLI-01 | 拆分 `chat.go` | 无 | `OPEN` | 按域拆 builder，行为不变 |
| CLI-02 | Typed IM adapter | BASE-02/03 | `OPEN` | 后端字段差异集中吸收 |
| CLI-03 | Message normalizer | BASE-03 | `OPEN` | 读取命令共享 |
| CLI-04 | Pagination | BASE-03 | `OPEN` | page-all/page-limit/truncated |
| CLI-05 | Error taxonomy | BASE-03 | `OPEN` | typed projection/partial errors |
| CLI-06 | Media resolver | BASE-02 | `OPEN` | local/URL/key 统一 |
| CLI-07 | Safe file I/O | 无 | `OPEN` | relative/no-traversal/no-clobber |
| CLI-08 | Dry-run/idempotency | BASE-02 | `OPEN` | 写链计划与校验一致 |
| CLI-09 | Target resolver | BASE-06 | `OPEN` | user/conversation/name 输入稳定映射到真实 leaf |
| CLI-10 | Delivery status | BASE-03 | `OPEN` | accepted/pending/succeeded/failed 语义与有界轮询 |

### 14.4 Shortcut

| ID | 任务 | 依赖 | 状态 | Done |
|---|---|---|---|---|
| SC-01 | Semantic Value Matrix | BASE-04 | `CLOSED` | 91 个 Shortcut 均有 reviewed semantic_delta 与 smart/adapter/leaf/alias 处置 |
| SC-02 | Typed ShortcutSpec | BASE-04 | `OPEN` | runtime/list/catalog/skill 同源 |
| SC-03 | 读取语义入口收敛 | CLI-03/04 | `OPEN` | history/search 两个 primary 替代重复读取候选 |
| SC-04 | 发送与资源语义入口 | CLI-06/08 | `OPEN` | resource download 与 name-aware send 达到 Shortcut DoD |
| SC-05 | 高风险候选审计 | safety gate | `OPEN` | 默认走原子 leaf；仅保留确有编排价值且安全完整的 Shortcut |
| SC-06 | Semantic delta gate | SC-01 | `CLOSED` | 91/91 均有非空 semantic_delta；disposition 控制路由优先级 |
| SC-07 | Smart read/search/resource | CLI-03/04/07 | `OPEN` | Golden Path 完成 |
| SC-08 | Semantic relation 审计 | SC-01 | `CLOSED` | primary/adapter/leaf/alias 均有 reviewed 理由，兼容 alias 指向公开 primary |

### 14.5 Hint 与 Skill

| ID | 任务 | 依赖 | 状态 | Done |
|---|---|---|---|---|
| HINT-01 | Golden selection | SC-03/07 | `PARTIAL` | 78/78 Chat Tool 已有 reviewed 正负场景并通过确定性覆盖门禁；Golden Path 默认选择与语义 smoke 仍随 SC-03/07 验收 |
| HINT-02 | Safety/interface | CLI/SC runtime | `PARTIAL` | Runtime Schema 的 safety/interface/final delivery 已关闭；Shortcut 与原子路径一致性仍未关闭 |
| HINT-03 | Parameter overlays | BASE-02 | `CLOSED` | 当前 17 个 Chat overlay 均绑定真实 runnable leaf/flag，Help exact-set 与生成门禁通过 |
| HINT-04 | metadata 差异清理 | SC-01 | `OPEN` | metadata 仍为 79 项、selection/Schema 为 78 项；`chat.get_group_members` 尚无明确 review reason 或删除处置 |
| SKILL-01 | Chat 路由重写 | SC/HINT | `OPEN` | 顶层 Skill 精简为决策层 |
| SKILL-02 | reference 拆分 | CLI/SC 稳定 | `OPEN` | 主题化参考 |
| SKILL-03 | Chat/Event recipe | Event model | `OPEN` | 监听回复闭环 |
| SKILL-04 | mono/multi 校验 | SKILL-01/02 | `OPEN` | 执行事实一致 |

### 14.6 Event

| ID | 任务 | 依赖 | 状态 | Done |
|---|---|---|---|---|
| EVT-01 | Event Payload V1 | BASE-03 | `PARTIAL` | 16 类个人事件已有 typed schema/flatten output；仍需共享 Message/Identity/Resource/operation V1 |
| EVT-02 | 媒体 resource refs | EVT-01/CLI-07 | `OPEN` | 可直接下载 |
| EVT-03 | card action | 上游支持 | `BLOCKED` | typed callback |
| EVT-04 | lifecycle events | 上游支持 | `PARTIAL` | 个人身份已新增群更新、成员加入/退出、解散；bot/app 独立身份面仍依赖上游 |
| EVT-05 | metrics/backpressure | 无 | `PARTIAL` | connection/reconnect/last-event、received/dropped、active consumers 已可观测；queue depth 与 last error category 未完成 |
| EVT-06 | listen-and-reply | EVT-01/SC send | `OPEN` | 安全 recipe |
| EVT-07 | live fixtures | EVT-01 | `PARTIAL` | 多事件、生命周期、stop/cleanup 自动化测试已具备；16 类真实触发与清理证据未完成 |

### 14.7 2026-07-28 关闭审计

本轮基于 `main@2dfc39f0` 关闭：

- `SCH-01`～`SCH-05`：Runtime Schema 平台基线；
- `HINT-03`：当前 Chat 参数 overlay 与真实 CLI flag 的绑定完整性。
- `BASE-04`、`SC-01`、`SC-06`、`SC-08`：91 个 Chat Shortcut 的公开策略、
  语义矩阵、非空 semantic_delta 与 relation 审计。

以下项目不得因 Schema 合入而关闭：

- `BASE-02/03/06` 与 `CLI-02`～`CLI-10`：Schema 能描述参数，但没有建立
  IM 的 TargetRef、统一 adapter、Normalized Output、媒体与送达运行时；
- `SC-02`～`SC-05`、`SC-07`：typed spec 全链同源、读取/发送黄金链路、
  高风险语义体验和效果优化仍需继续；公开完成不等于这些效果项已关闭；
- `HINT-01/02`：Schema 侧已完成的选择覆盖和安全事实，不等于 Smart
  Shortcut 默认路由或 Shortcut/原子路径安全一致；
- `SKILL-01`～`SKILL-04`：Skill 尚未改造成由 Runtime Schema +
  typed Shortcut Catalog 共同驱动的场景路由；
- `EVT-01`～`EVT-07`：Event Phase 3 降低了剩余成本，但没有完成
  ResourceRef、统一 payload、监听回复和全量 live fixture。

关闭证据：

```text
generated drift check: ok
schema catalog check: ok (25 products, 603 tools)
runtime confirmation truth ok (80 gated)
Chat selection: 78/78 reviewed，且每项至少一个 use_when/avoid_when/example
Chat parameter overlay: 17 个 owning block，全部通过 runnable leaf/flag 门禁
```

## 15. ROI 优先级与实施顺序

### 15.1 ROI 口径

ROI 用于决定实施顺序，不用于删除能力或缩小 Semantic Shortcut
覆盖面。为了让排序可复算，所有工作包使用 1～5 分的相同量表：

| 符号 | 维度 | 1 分 | 5 分 |
|---|---|---|---|
| R | 用户场景覆盖 Reach | 小众、低频、单命令 | 高频核心意图、跨多命令 |
| T | TSR 增益 | 只改善外观或文案 | 同时提升选择、参数、执行、输出 |
| U | 公共复用 Reuse | 单一 leaf 专用 | 被读取、搜索、发送、资源、事件复用 |
| S | 对齐/超越价值 Strategic | 无明显竞品或差异化价值 | 补齐 Lark 核心领先点或形成 DWS 独有优势 |
| C | 交付确定性 Certainty | 强上游依赖、无 fixture | 已有实现证据、observed shape 和测试条件 |
| E | 实现成本 Effort | 1=小 | 5=大且跨仓/高风险 |

加权价值与成本因子：

```text
WeightedValue =
  0.25 × R
  + 0.30 × T
  + 0.20 × U
  + 0.15 × S
  + 0.10 × C

CostFactor(E) = {1: 1.00, 2: 0.80, 3: 0.60, 4: 0.45, 5: 0.30}

ROI Index = round(100 × WeightedValue / 5 × CostFactor(E))
```

ROI Index 用于同一规划周期内的相对排序：

| Index | 解释 |
|---:|---|
| ≥70 | 立即执行的系统性杠杆 |
| 55–69 | 下一核心迭代 |
| 35–54 | 高价值纵向切片 |
| 25–34 | 中价值或差异化建设 |
| <25 | 上游依赖型或发布后长尾 |

Safety、契约完整性和数据不丢失是准入门槛，不因为 ROI 高而跳过。
分数是规划指数，不是财务收益估算；生产 TSR、使用频次、工期或上游
状态变化后必须重算。

### 15.2 ROI 总表

对比基线为 `lark-cli main@56c9a2a`、稳定版 `v1.0.77`。Lark 的主要
优势是少量高频 Shortcut 的参数、分页、富化、媒体和错误恢复深度；
DWS 的潜在优势是更宽的 Semantic Shortcut 覆盖、钉钉原生群治理和
当前用户身份的个人 IM 事件。2026-07-27 outcome 实测进一步提高了
TargetRef、投影保真和媒体 E2E 的收益评分。

#### 15.2.1 评分明细

| 优先级 | 工作包 | R | T | U | S | C | E | ROI Index |
|---|---|---:|---:|---:|---:|---:|---:|---:|
| P0 | 薄契约、TargetRef 与公开机制 | 5 | 5 | 5 | 4 | 5 | 2 | **78** |
| P1 | 读取 / 搜索黄金链路 | 5 | 5 | 5 | 5 | 4 | 3 | **59** |
| P2 | 资源与媒体 E2E | 5 | 5 | 4 | 5 | 4 | 3 | **56** |
| P3 | 富媒体发送 / 回复与交付语义 | 5 | 4 | 4 | 4 | 4 | 4 | **38** |
| P4 | 个人事件监听 → 安全回复 | 3 | 4 | 4 | 5 | 4 | 3 | **47** |
| P5 | 钉钉原生治理语义化 | 3 | 3 | 4 | 5 | 4 | 4 | **32** |
| P6 | 高风险长尾与稳定化 | 2 | 3 | 4 | 3 | 4 | 4 | **27** |
| P7 | Bot 卡片与生命周期事件 | 2 | 3 | 3 | 4 | 2 | 5 | **17** |

评分依据：

- P0 同时解决评测暴露的 ID 寻址、静默返空、Semantic 选择和公开机制，
  且多数为本地契约工作，确定性最高；
- P1 覆盖所有读取型高频意图，并为搜索、事件和 Skill 提供公共消息模型；
- P2 的图片/文件往返是 outcome 实测中的明确阻塞，且 ResourceRef、
  Media Resolver 和安全 I/O 被读取、发送与事件复用，因此紧随 P1；
- P4 的多事件联合消费、16 类个人事件、生命周期输出和可观测性底座已经
  进入 `main`，因此交付确定性由 3 提升到 4、剩余成本由 4 降到 3；
  payload/resource/reply/live fixture 仍需完成，重算后的 ROI 为 47；
- P3 用户覆盖高，但依赖 P0 TargetRef 和 P2 Media Resolver，且涉及多
  身份、幂等与送达状态，成本高于 P2；
- P5 多数底层能力已存在，战略差异化高，但单能力频次低于 Golden Path；
- P6 是发布完整性工作，用户直接感知较弱但确定性高；
- P7 受上游事件支持、权限和 live fixture 约束，当前不应阻塞现有优势。

#### 15.2.2 执行总表

| 优先级 | 纵向工作包 | ROI | 主要任务 | 关键出口 |
|---|---|---:|---|---|
| P0 | 薄契约、TargetRef 与公开机制 | 78 | BASE-01/02/04/05/06、CLI-09、SC-01/02/06/08、Message/Page 最小模型 | typed target/identity；91 个 Shortcut 完成 smart/adapter/leaf/alias 处置并公开；TSR 基线可重复 |
| P1 | 读取 / 搜索黄金链路 | 59 | BASE-03、CLI-02/03/04/05、SC-03/07、HINT-01、SKILL-01 读取路由；HINT-03 仅做变更回归 | history/search 两个 primary；默认时间窗、分页、富化、partial failure 和投影保真完整 |
| P2 | 资源与媒体 E2E | 56 | Resource model、CLI-06/07、SC-04 media slice、SC-07 resource、相关 Hint/Skill | local/URL/key、内联图片、结构化回执、安全下载和 md5 往返 |
| P4 | 个人事件监听 → 安全回复 | 47 | EVT-01/02/05/06/07、SKILL-03 | 16 类事件收敛到共享 payload；资源闭环；防循环、有界消费和可观测清理 |
| P3 | 富媒体发送 / 回复与交付语义 | 38 | CLI-08/10、SC-04、HINT-02、SKILL-01 写入路由；HINT-03 仅做变更回归 | name-aware send；富媒体 reply；dry-run/idempotency；accepted 与 delivered 分离 |
| P5 | 钉钉原生治理语义化 | 32 | SC-01/05/06、HINT-01/02、SKILL-01/02 | 原子 leaf 直接路由；只为确有 semantic delta 的治理意图保留 Shortcut |
| P6 | 高风险长尾与稳定化 | 27 | SC-05、CLI-01、HINT-04、SKILL-02/04、全量真机矩阵 | 高风险原子/语义路径安全事实一致；mono/multi 一致；发布门禁完整 |
| P7 | Bot 卡片与生命周期事件 | 17 | EVT-03/04、对应 CLI/Shortcut/Skill | 上游支持时发布独立身份面；command schema 与 event payload schema 分治 |

ROI 排序不改变职责边界：原子 leaf 由 Runtime Schema 完整交付，
Shortcut 只覆盖明确的高价值语义。按剩余 ROI 的当前建议顺序为
`P0 → P1 → P2 → P4 → P3 → P5 → P6 → P7`；P4 可先完成 payload、
resource 和 fixture，listen-and-reply 尾项仍复用 P3 的发送安全能力。

### 15.3 任务分类

任务按交付责任划分为以下八类。一个纵向工作包必须覆盖相关类别，
不能只交付 CLI 实现后把参数、Hint、Skill 或必要的 Semantic Shortcut
留到最终阶段。

| 分类 | 范围 | 主要任务 / ID | 首要优先级 | Definition of Done |
|---|---|---|---|---|
| 功能 / 能力补充 | 读取、搜索、媒体、资源、群治理和场景闭环 | 读取富化、自动分页、内联图片、富媒体回复、安全下载、个人事件回复 | P1–P5 | Golden Scenario 可端到端完成，失败路径可解释 |
| API / CLI cmd 优化 | Cobra leaf、adapter、运行时校验和错误恢复 | CLI-02～10；CLI-01 仅随业务切片拆分 | P0–P3；纯重构 P6 | flag 可执行，binding 正确，错误 typed，dry-run 与真实执行同构 |
| 出入参标准化 | Surface 参数、Binding 映射、typed output 和兼容 serializer | BASE-02/03/06、CLI-02/03/04/05/09、HINT-03 | P0–P3 | TargetRef/canonical/alias 清晰；后端 property 不泄漏；旧输出兼容；分页与 partial failure 标准化 |
| Semantic Shortcut | 高价值语义入口、Smart Shortcut、公开状态和安全事实 | SC-01～08 | P0–P6 | 每个公开 Shortcut 有非空 semantic_delta、runnable binding、selection、标准输出与安全事实；无增益项回到 Schema leaf |
| Hint 优化 | selection、safety、interface、parameter overlay | HINT-01～04 | 随 P1–P7 同步 | 正负选择场景、参数和安全事实与最终 ToolSpec/runtime 一致 |
| Skill 优化 | 顶层路由、主题 reference、Chat/Event recipe、mono/multi | SKILL-01～04 | 随 P1、P3、P4、P5 同步 | Skill 只路由到真实公开能力；不复制漂移的执行事实；示例可执行 |
| 事件消费补充 | Event Payload、媒体 refs、card/lifecycle、消费可靠性 | EVT-01～07 | P4、P7 | command schema 与 payload schema 分离；ready/consume/exit/cleanup 稳定 |
| 质量与发布治理 | 指标、fixtures、observed shape、跨仓 case 和发布门禁 | BASE-01/05、outcome 报告用例、测试矩阵、跨仓 issue contract | 全程 | TSR 可重复；projection invariant；真机失败能区分 DWS、上游、权限和 fixture |

### 15.4 执行规则

1. **薄底座而非大前置工程。** P0 建议控制在 5～7 个工作日；只建立
   P1 所需的参数 ADR、Message/Page 最小模型、公开机制和指标，不等待
   Resource/Event 全模型完成。
2. **纵向切片交付。** 每个能力族必须同时完成 CLI、出入参、Hint、
   Skill、测试和 evidence；只有存在 semantic delta 时才增加 Shortcut。
3. **禁止机械 1:1 Shortcut。** 原子覆盖由 Schema 负责；Shortcut 必须
   提交 semantic_delta。没有增益的现有项改为 leaf/alias/internal。
4. **全量公开、分级路由。** 91 个 reviewed + available Shortcut 全部
   公开；history、search、resource download、name-aware send 和
   listen-and-reply 继续作为效果优化与默认选择的优先纵向切片。
5. **Hint 和 Skill 不作为最终扫尾。** 每个 P1～P7 工作包都必须带上
   对应选择、参数、安全和路由变更；P6 只清理全局一致性和遗留差异，
   P7 只发布已得到上游契约和 fixture 支持的事件能力。
6. **纯重构不单独占据高优先级。** `chat.go` 按读取、资源、发送、治理
   等正在修改的域渐进拆分；不先做大规模行为不变重构。
7. **上游依赖不阻塞现有超越点。** card action 和 bot lifecycle 可先做
   可行性与契约设计，但不得阻塞个人事件监听→回复和钉钉原生治理能力。

### 15.5 P0：薄契约与公开机制

范围：

- 固定七类 IM Golden Scenarios 和 TSR 子指标基线；
- 评审参数 Surface/Binding ADR；
- 定义 TargetRef/IdentityRef 和兼容迁移规则；
- 建立 Target resolver，原生 ID 不改写；
- 建立 Shortcut semantic-value/availability/evidence 三维发布规则；
- 完成 91 个 Shortcut 的 Semantic Value Matrix；
- 完成 keep/smart/leaf/alias semantic relation 审计；
- 建立最小 Typed ShortcutSpec；
- 只定义 P1 所需的 Message/Page model；
- 加入 raw non-empty → normalized non-empty 投影 invariant。

出口：

- `live_blocked` 不再机械隐藏有语义价值的 contract-valid Shortcut；
- 目标和执行身份可自描述，无法判型时不猜测；
- 91 个现有 Shortcut 均有 semantic_delta、binding、availability、
  evidence 和明确处置；
- 78 个 Agent-visible 原子工具继续由 Runtime Schema 完整发布，不要求
  Shortcut mapping；
- P1 的选择、参数、执行和输出指标有可重复基线。

### 15.6 P1：读取 / 搜索黄金链路

范围：

- `messages-list`、`chat-messages`、`thread-replies`、
  `search-advanced/search-msg` 共用 Message normalizer；
- list/mget/thread/search 共享 typed adapter；
- `message list` 提供合理的默认时间窗和明确方向；
- 自动分页、page limit、next cursor 和 truncated 提示；
- sender/conversation/reaction/forward/reply/thread 富化；
- 单项富化失败保留主结果并发布 partial failure；
- 将重复读取候选收敛为 history/search 两个 primary；
- 同步完成读取类参数 Hint、selection Hint 和 Chat Skill 默认路由。

出口：

- raw non-empty → normalized non-empty 100%；
- 未识别 observed shape 返回 `projection_shape_drift`，不得伪装为空结果；
- optional enrichment 失败时主结果保留率 100%；
- 截断结果提示率 100%；
- 读取 Golden Scenarios 首次调用得到可用结果；
- history/search 两个语义入口达到 Shortcut DoD，原子读取 leaf 保持可用。

### 15.7 P2：资源与媒体 E2E

范围：

- 完成 Resource Normalized V1；
- local/URL/existing key 统一 Media Resolver；
- 建立受支持的图片/文件上传路径，不恢复已下线的旧 media upload；
- 本地图片默认按内联 image 发送，显式选择才按 file 附件发送；
- 上传结果返回 ResourceRef、MIME、size、checksum 和 source；
- relative/no-traversal/no-clobber；
- 分片、重试、超时、大小验证和失败清理；
- Content-Disposition、MIME 和扩展名推断；
- 消息读取可选自动下载；
- 为 Event resource refs 提供相同下载入口。

出口：

- 本地图片、文件和已有 media key 可通过统一语义输入处理；
- 图片/文件完成 send→read→download→checksum 端到端闭环；
- 不支持内联上传时返回 typed capability error，不静默降级；
- 下载路径不可逃逸 workspace；
- 默认不覆盖现有文件；
- 部分或失败下载不遗留伪成功文件；
- 读取输出和 Event payload 使用同一 ResourceRef。

### 15.8 P3：富媒体发送 / 回复与交付语义

范围：

- text/markdown/card/image/file/video/audio 统一内容选择；
- reply 与 send 共享媒体能力；
- idempotency key、dry-run、目标摘要和输入约束；
- 标准 Delivery Status；
- 可选 `--wait-delivery` 与有界轮询；
- 普通 write 与原子命令 confirmation 一致；
- 发布 name-aware send；reply/forward/reaction 仅在存在 semantic delta
  时保留 Shortcut，否则由 Schema leaf 直接执行。

出口：

- send/reply 共享 P2 Media Resolver 和标准内容模型；
- dry-run 能完整描述上传、转换和发送步骤；
- 重试不会制造无约束重复消息；
- accepted、pending、succeeded、failed 不再混淆；
- 普通 write 不因 Shortcut 路径产生额外确认漂移。

### 15.9 P4：个人事件监听 → 安全回复

范围：

- Event Payload Normalized V1；
- 16 类个人事件共享身份、会话、消息和时间字段；
- 媒体事件发布 ResourceRef；
- received/dropped/queue/reconnect/last-error 指标；
- listen-and-reply recipe；
- 防循环、去重、有界消费和清理；
- Chat/Event Skill 对齐。

出口：

- 16 类事件 payload 契约稳定；
- 监听→回复场景不需要 Agent 自行猜测消息和会话 ID；
- ready→consume→reply→exit→cleanup 有自动化 fixture；
- command schema 与 event payload schema 无混淆。

### 15.10 P5：钉钉原生治理语义化

91 个 Shortcut 已全量公开；后续仍按以下能力族推进效果、选择和安全审计：

1. reaction、文字表情、pin、top；
2. 会话已读、未读、隐藏、免打扰和分类；
3. 转发、合并转发和话题转发；
4. 群成员、管理员、自定义角色和群主转让；
5. 群机器人、Webhook、入群审核、邀请链接和外部群升级。

每个能力族先保证原子 Schema leaf 可准确选择和执行。只有存在参数收敛、
目标解析、组合操作或错误恢复价值时，才按 SC-06 新建或保留 Shortcut；
Hint、Skill、mock/observed-shape 和权限错误语义仍需同步交付。

### 15.11 P6：高风险长尾与稳定化

- 审计高风险 Shortcut，默认回到精确原子 leaf；
- 被保留的高风险语义入口补齐 runtime gate、目标摘要、`--yes`、dry-run
  和恢复语义；
- 清理 metadata 79/78 差异；
- 完成 mono/multi 一致性；
- 随业务改动完成剩余 `chat.go` 域拆分；
- 执行全量 contract、observed-shape、selection、event lifecycle 和真机矩阵；
- 完成版本发布与 evidence freshness 检查。

### 15.12 P7：Bot 卡片与生命周期事件

- 先确认上游是否支持 card action 和目标 bot lifecycle；
- 为 bot/app 身份面单独定义订阅、payload、scope 和消费限制；
- 不把个人事件 key 与 bot/app 事件 key 合并；
- 上游未支持时保留 exact unavailable/runtime-gated 状态和 issue contract；
- 上游支持后再进入正式 Schema、Shortcut、Skill 和 live fixture。

## 16. 测试与门禁

### 16.1 分层测试

| 层 | 目的 |
|---|---|
| Contract | Cobra path、flags、required、constraints、binding、risk |
| Unit | adapter、normalizer、pagination、media、path safety |
| Observed shape | 使用脱敏真实响应验证投影 |
| Integration | 真实 command→MCP stub→normalized output |
| Live UAT | 真实身份和 fixture |
| Selection | use_when/avoid_when 与同产品候选 |
| Event lifecycle | ready→event→exit→cleanup |

### 16.2 强制不变量

1. 每个 Shortcut 绑定真实 runnable path。
2. 每个公开 Shortcut 有非空、可测试的 semantic_delta；无增益时不得进入
   Agent 默认候选。
3. 下层业务列表非空时，上层投影不得为空。
4. 未采集下层数据的空投影不得直接标记 live verified。
5. `confirmation=user_required` 与 metadata runtime gate 数量和值一致。
6. Shortcut 与对应原子命令的安全事实一致。
7. 示例通过真实 Cobra argv 和约束校验。
8. `--page-all` 有界。
9. 下载路径不能逃逸工作目录。
10. 生成流程不读取旧 Catalog 作为输入。
11. 上游原生 ID 不被添加、删除或改写前缀。
12. TargetRef 无法无歧义判型时返回 typed validation error，不猜测。
13. 本地图片请求内联发送时不得静默降级为 file 附件。
14. `accepted/pending` 不得投影为 `succeeded`。
15. 媒体 Golden Scenario 的 send/read/download checksum 必须一致。

### 16.3 发布命令

```bash
go build ./cmd
DWS_PACKAGE_VERSION=0.0.0-test go test ./...
go generate ./internal/cli
./scripts/policy/check-generated-drift.sh
./scripts/policy/check-schema-catalog.sh
./scripts/policy/check-runtime-confirmation-truth.sh
go test ./internal/app -run '^TestSheetFinalSchemaConfirmationMatchesRuntimeGuards$' -count=1
```

Shortcut 的新增门禁应包含：

```bash
python3 scripts/shortcut_real_result_test.py
```

真机测试应输出 contract/mock/live/live-blocked 统计，不再只输出 public/followup 二元结果。

## 17. 效果指标

北极星继续使用意图达成率 TSR：

```text
选择正确
  × 参数正确
  × 执行成功
  × 输出可用
```

IM 专项指标：

| 指标 | 说明 |
|---|---|
| Golden intent coverage | 五个首批高价值意图有可执行语义入口的比例 |
| Semantic delta pass ratio | 公开 Shortcut 通过 semantic-value gate 的比例 |
| Selection Top-1 | 自然语言场景首选正确率 |
| First-run success | 无补参、无换命令的成功率 |
| Projection preservation | 下层记录被上层保留的比例 |
| Empty correctness | 真空结果与 shape drift 区分率 |
| Pagination visibility | 截断结果有明确提示的比例 |
| Enrichment isolation | 可选富化失败时主结果保留率 |
| Media task success | 上传/发送/下载完整成功率 |
| Media checksum roundtrip | send→read→download 后 checksum 一致率 |
| Target resolution success | name/typed ID 在首次调用中解析到正确目标的比例 |
| Target ambiguity safety | 歧义目标被阻止并要求补充的比例 |
| Delivery truthfulness | accepted/pending/succeeded/failed 与真实状态一致率 |
| Semantic relation coverage | 疑似重复入口具有 primary/alias/distinct/composite 结论的比例 |
| Evidence freshness | live evidence 距当前版本的时间 |
| Event lifecycle success | ready、消费、退出、退订完整成功率 |

不使用 Shortcut 数量作为单独效果指标：既不追求越少越好，也不追求
leaf 级覆盖率；关注高价值意图覆盖和 semantic delta 质量。

## 18. 文件与所有权边界

| 工作流 | 主要文件 |
|---|---|
| CLI/API | `internal/helpers/chat*.go` |
| Semantic Shortcut | `internal/shortcut/chat/`、`internal/shortcut/smart/` |
| Message projection | `internal/shortcut/chatmsg/`，后续 typed IM package |
| Shortcut framework | `internal/shortcut/types.go`、`runner.go`、`public_catalog.go` |
| Shortcut publication | `scripts/gen_shortcut_public_catalog.py`、Catalog/evidence source |
| Command identity | `internal/cli/schema_command_registry.json` |
| Parameter binding | `internal/cli/schema_parameter_bindings.json` |
| Hint metadata | `internal/cli/schema_hints/metadata/chat.json` |
| Hint selection | `internal/cli/schema_hints/selection/chat.json` |
| Event | `internal/event/personal/`、`consume/`、`bus/`、`internal/app/event_*` |
| Skill | `skills/multi/dingtalk-chat/`、`dingtalk-event/`、`skills/mono/` |
| 上游契约 | `dingtalk-supcon-mono` 对应 MCP/tool 实现 |

并行修改时按工作流隔离文件。生成动作只在各输入完成评审后统一执行。

## 19. 风险与应对

| 风险 | 应对 |
|---|---|
| 无语义增益的 Shortcut 增加选择混淆 | semantic_delta gate；优先 Schema leaf；同一意图只保留一个 primary |
| 参数改名破坏脚本 | 主 flag + 隐藏 alias + 兼容周期 |
| typed model 引发 wire 变化 | 内部 typed、外部兼容 serializer、必要时版本化 |
| 自动分页请求过多 | page-limit、delay、截断提示 |
| 富化降低性能 | 默认策略、批量、有界并发、可关闭 |
| 媒体 URL 带来 SSRF | URL 校验、大小限制、超时和安全域名策略 |
| Target auto 推导误发 | 只接受显式类型或无歧义 typed ref；写前目标摘要 |
| 伪造 Lark 式 ID 前缀破坏上游调用 | 原生 ID 原值保留，类型放入 TargetRef 字段 |
| 合并 user/bot/webhook 导致权限语义丢失 | 原子 leaf 与 IdentityRef 保持独立；Smart Shortcut 只做显式身份路由 |
| 内联图片能力不足时静默降级 | typed capability error；仅显式 `--as-file` 降级 |
| 受理被误认为送达 | Delivery Status 分层；有界轮询；超时保持 pending/unknown |
| 写入语义入口带来误操作 | 目标校验、dry-run、准确 risk/confirmation；无编排价值时直接使用原子 leaf |
| 真机环境不稳定 | contract/mock/live 分层，live-blocked 不隐藏 |
| 上游返回结构变化 | observed-shape contract、projection drift error |
| Chat/Event 身份混淆 | personal 与 bot/app Catalog 分离 |

## 20. 待评审决策

1. 会话 ID 的 canonical flag 最终使用 `--conversation-id` 还是现有参数本体草案中的 `--conversation`。
2. ShortcutSpec 是否复用 Schema 的 ParameterSpec 类型，还是保留独立类型并共享更底层的 Constraint/Parameter primitives。
3. Shortcut 是否在后续版本进入统一 `dws schema`；本期默认保持独立 Catalog。
4. Normalized V1 是只做内部 model + 兼容输出，还是增加显式 projection/version flag。
5. 普通 write Shortcut 的确认策略切换是否需要一个兼容开关或直接统一到最终 metadata。
6. 自动资源下载的默认目录、单文件大小、总大小和并发上限。
7. bot/app IM 事件是否由 `dws event` 同一顶层命令承载，还是单独身份子树。
8. 已决议：原有 89 个及本轮新增 2 个 Shortcut 全部公开，并分别标注为
   Smart、Adapter、Schema leaf 投影或兼容 alias；该 relation 影响路由优先级。
9. 高频 Smart Shortcut 使用 `--target/--target-type`，还是只发布分类型
   `--user-id/--open-dingtalk-id/--conversation-id`。
10. 内联图片上传由现有 MCP 新能力、OpenAPI 本地 adapter，还是新的统一
    Resource service 承载；未确定前不得恢复旧 media upload。
11. 是否增加统一发送 Smart Shortcut；若增加，Webhook 是否继续保持独立
    入口（本设计默认保持独立）。

## 21. 相关材料

- [DWS 参数字典 / 本体](dws-parameter-ontology.md)
- [Shortcut 总体规划](shortcut-plan.md)
- [Shortcut P2 设计](shortcut-p2-design.md)
- [Shortcut 与 lark-cli 对齐矩阵](shortcut-lark-alignment.md)
- [DWS IM 场景 Shortcut 分层测试报告](im-shortcut-test-report.md)
- [DWS IM Shortcut 与 lark-cli 差异报告](im-shortcut-lark-cli-diff-report.md)
- [Shortcut 投影保真警示](shortcut-projection-fidelity.md)
- [Shortcut 后端/MCP 问题](shortcut-backend-mcp-issues.md)
- 外部 outcome evidence：《CLI评测-IM领域详细对比报告-20260727》
  （dws v1.0.54 vs lark v1.0.77；原始报告未纳入仓库）
- [dingtalk-chat Skill](../skills/multi/dingtalk-chat/SKILL.md)
- [dingtalk-event Skill](../skills/multi/dingtalk-event/SKILL.md)
- [lark-cli IM shortcuts at 56c9a2a](https://github.com/larksuite/cli/tree/56c9a2afd881317c3dcf2d3d5148b09041e3ea7a/shortcuts/im)
