# DWS Multi IM 优化方案

> 状态：Phase 0–4 已在当前分支实施；无下层能力的项目已以 typed 负面边界收口
>
> 日期：2026-08-03
>
> DWS 口径：当前分支 `codex/multi-im-optimization` 及其工作树；
> 设计对比基线为 `017258e5b49517e3787a3d373f90953c128eaf58`
>
> 对照基线：本机当前安装的 `lark-im`、`lark-shared`、`lark-contact`、
> `lark-event`
>
> 详细证据：[Lark IM 与 DWS Multi IM Skill 设计对比](lark-im-vs-dws-multi-im-design-review.md)

## 1. 一句话结论

DWS Multi IM 已在不推翻 Golden Route、不复制 Lark 全量手册的前提下，完成
Runtime、Schema、Skill 根入口、task references、Event 子产品和发布脚本链路的统一。
Lark 中值得借鉴的精确 task reference、身份能力矩阵、Card 工作流和结果合同
已转化为 DWS 的 typed descriptor 和 drift gate，而不是独立手写事实表。

目标不是“看起来像 Lark”，而是：

```text
任何合法加载路径
→ 选择同一个任务级入口
→ 使用同一个 typed resolver / Runtime gate / DTO
→ 得到可验证、可继续编排的结果
```

### 1.1 2026-08-03 实施检查点

当前工作区已完成全部计划交付：

- 新增 9 条 reviewed 高频 IM intent route fixture，并把 policy 接入 `make policy`；
- policy 从 CommandRegistry 绑定真实 Cobra leaf、从 `ResolveMeta` 读取 confirmation，校验
  marker、selection 正负场景、fallback 安全、source-ref 路径/锚点和脚本合同；
- Chat/Event 根 Skill、任务 references、Mono 入口统一到 Golden Shortcut；原子 leaf 只保留
  显式底层 fallback；
- Chat/Event selection 已收窄，旧单文件 CommandRegistry provenance 已迁移到当前 product shard；
- Shared 的公开 flag、默认 JSON、无统一成功 envelope、typed error/retry/profile 合同已按当前
  Runtime 修正；
- 3 个 Chat 第二 Runtime 入口、1 个公共 helper 及其 Mono 副本已退役；生成器不再发布它们，policy
  用精确退役路径防止回流；
- 当前分支的群名 resolver 已由并行开发补齐全量分页、停滞游标和安全页数上限保护，并通过
  Chat/targetresolver 回归测试。
- `+chat-messages` 已支持有界 `--page-all`、typed continuation、消息去重、完整性 ledger 和
  工作区内 `.json` 原子导出；`+conversation-list --page-all` 同样遵守服务端页上限并公开截断。
- Bot 多群发送已下沉 `+messages-send --groups|--groups-file`，统一确认一次并输出
  `im.batch-write.v1` 逐目标 ledger；
- `+group-members` 与 `+chat-members-list` 已实现用户桶全量分页、稳定 ID 去重、用户/机器人
  分桶、有界 continuation 和中途失败的 partial ledger；`+chat-create` 支持显式 owner 或
  歧义安全的 owner 自然查询；
- 身份、消息结果、Card 流程和 IM 能力边界均有 typed descriptor；
  [`contracts.md`](../skills/multi/dingtalk-chat/references/contracts.md) 的标记区块由 policy 与 Runtime
  descriptor 逐字节比对；
- Card 已拆为 create/update/callback/schema 任务 reference；Event IM 已拆为 keys、lifecycle、
  output/handoff、operations，Chat/Event 交接 ID 映射由 policy 锁定；
- 评测中高频出现的群搜索历史路径/位置参数、`--chat`、单值 `--message-ids` 和冗余
  `aisearch --type person` 已加入不改变业务语义的兼容层；稳定 ID 类型误用在下层调用前失败。
- 群机器人、邀请链接和指定群 `@我` 已接受自然群目标并复用同一歧义安全 resolver；撤回可由
  单个消息 ID 只读补齐会话上下文。

对下层当前不存在的 Thread writer、Bot 富媒体、Card action callback 和资源断点续传，
本轮没有伪造实现；它们已进入 typed 负面能力矩阵，Schema/Skill 明确引导到当前可用替代路径。
当平台提供真实下层接口时，再按 `Runtime → Schema → task reference → root navigation` 新增正向能力。

## 2. 当前基础与主要问题

### 2.1 已经做对的部分

当前分支已经完成 IM 高频主链升级：

- Chat 根 Skill 只保留 Golden Route、关键结果语义和按需 reference 导航；
- `+dm`、`+send-to-group`、`+messages-send`、`+chat-messages`、`+search-msg`、
  `+chat-create` 和 `event +listen-im` 复用 typed target resolver；
- 自然目标的零命中和多候选会在写入前停止，解析和执行复用当前 profile；
- 消息读取具有 `complete`、`hasMore`、`failures` 和资源下载 ledger；`+chat-messages` 已支持
  有界 `--page-all` 与原子文件导出，`+search-msg` 支持游标全量搜索和批量富化；
- `+listen-im` 能从自然目标和事件意图确定 EventKey 集合，并复用一个 consume 生命周期；
- 参数、安全、selection 和 interface 由 reviewed Schema 输入与 Runtime 交付。

这里不把“显式稳定 ID 必然在写入前完成跨 profile 校验”列为现有能力。当前 Runtime 没有为
任意显式 ID 携带可验证的 profile provenance；这类 ID 的来源仍由调用方负责，也可能到下游
权限或组织边界才被拒绝。

这些设计不应回退。

### 2.2 基线审计发现的问题

以下是实施前基线的问题清单。第 1–9 项和可由当前下层支撑的 Runtime 等价能力均已处理；
第 10 项中尚无平台接口的部分以显式负面合同收口，不作为本轮未完成项。

1. `chat-message.md`、`chat-group.md`、`chat-bot.md`、`chat-conversation.md` 和
   `event-im.md` 仍会把 Agent 从 Golden Shortcut 带回手工查 ID 或原子命令；
2. 某些原子写 leaf 的 confirmation 比对应 Golden Shortcut 更弱，reference 回流可能绕开
   typed resolver、稳定 DTO 或 `typed_yes` gate；
3. Chat 发布包携带 3 个 Python 入口和 1 个公共 helper，其中两个入口直接选择第一个搜索结果；现已从
   Multi/Mono 发布树删除，功能由 typed Shortcut 接管；
4. Mono/Multi 存在同名脚本副本且内容已经分叉；现已取消这组脚本的生成与发布，
   policy 对全部退役路径 fail closed；
5. Shared 的全局输出、flag 和错误文档部分落后于当前 Runtime；
6. 现有 policy 主要证明根 Skill 预算和 Shortcut Runtime，不证明完整加载链一致；
7. reviewed selection 中仍有 Shortcut 与原子工具同时声明同一高频正向场景，修 reference
   不能单独消除 Schema 选路歧义；
8. `selection/chat.json` 当前仍有约 120 个 `source_refs` 指向已不存在的旧版单文件
   `schema_command_registry.json`；已迁移到 `schema_command_registry/products/chat.json`，
   Event 同步迁移，并由 policy 校验锚点；
9. Mono reference 仍直接推荐或列出待迁移脚本；现已迁移到 Runtime 命令并清理发布路径；
10. DWS 缺少 Lark 已形成闭环的任意 Card 编译/callback、富 Thread 回复等能力；现已对
    DWS 真实支持的 streaming-text Card create/update 建立子产品 reference 和 typed workflow，
    对 callback/Thread writer 建立显式不支持边界。

## 3. 优化原则

### 3.1 一个意图只能有一个默认入口

同一用户终点可以有底层 fallback，但不能在不同文件中出现两个并列默认答案。

例如：

```text
按姓名发消息       → +dm
高级/Bot/Webhook   → +messages-send
建群               → +chat-create
引用回复           → +messages-reply
监听个人 IM        → event +listen-im
```

原子命令只在 Shortcut 未暴露的特殊字段、原始响应或底层运维控制场景出现，并必须写明
`fallback_reason`、能力损失和确认差异。

高频意图使用 reviewed、机器可读的路由清单固定默认工具与允许 fallback，例如：

```json
{
  "intent_id": "chat.create",
  "preferred_tool": "chat.shortcut_chat_create",
  "allowed_fallbacks": [
    {
      "tool": "chat.create_group_conversation",
      "reason_code": "shortcut_missing_required_field"
    }
  ]
}
```

清单建议位于
`scripts/policy/multi-im-skill-chain/testdata/intent_routes.json`，reference 只写精确的
`intent_id` marker。它只拥有高频任务的选路断言，不创建命令身份、CLI path、参数或安全事实：
canonical tool 必须由 reviewed CommandRegistry 解析，confirmation 必须由 `ResolveMeta` 读取，
selection 正负场景仍由 `schema_hints/selection/<product>.json` 拥有。

### 3.2 Skill 负责选路，Runtime 负责确定性执行

Skill 不应自己完成以下工作：

- 搜索后选择 `users[0]` 或 `groups[0]`；
- 手工拼接跨步骤 ID；
- 自己兼容多代 JSON envelope；
- 用脚本重新实现分页、下载、重试、幂等和 partial failure；
- 根据文案猜测 confirmation。

这些都应由 typed resolver、Shortcut、稳定 DTO 和最终 Schema/Runtime gate 承担。

### 3.3 Reference 只拥有一种主要职责

每个 reference 应明确属于以下一种类型：

| 类型 | 内容 | 不应包含 |
|---|---|---|
| Task reference | 默认入口、任务边界、结果、错误和 continuation | 全量原子 Catalog |
| Platform semantics | 对象区别、身份限制、平台不可推导规则 | 重复完整 flags 表 |
| Atomic fallback | Shortcut 未覆盖的底层能力 | 高频意图选路 |
| Static resource | emoji、枚举、Card 组件等受控数据 | 业务工作流 |

### 3.4 不改变现有稳定 Schema wire contract

优化应复用当前 CommandRegistry、ToolSpec、Schema Catalog 和 `ResolveMeta` 单一来源。
文档对齐不构成修改 JSON wire contract、重建第二套 identity source 或从生成物反向合并的
理由。

### 3.5 Profile 保证分层表达

- 自然目标：resolver 在当前 profile 内完成唯一解析，并由同一 RuntimeContext 执行；
- Agent 编排：reference 和 script 禁止缓存后跨 profile 复用解析结果；
- 显式稳定 ID：没有 provenance 时不承诺本地预写拦截，只能说明其来源由调用方负责；
- 若未来需要硬保证，应引入携带 profile/organization provenance 的 typed target，并让写入
  Shortcut 验证它；不能只靠 Skill 文案宣称已经实现。

## 4. 从 Lark 借鉴什么

### 4.1 借鉴精确 task reference

Lark 为高阶 Shortcut 提供精确 reference，每个文件通常包含：

```text
适用任务
→ 身份/权限
→ 参数边界
→ 输出字段与完整性
→ 常见错误
→ 后续动作
```

DWS 不需要为 97 个 Chat Shortcut 各建一个文件，但高频且语义复杂的任务应采用这个
模板。优先覆盖：

- 消息发送；
- 消息读取/搜索/资源；
- 引用回复与转发；
- 建群与批量成员解析；
- Bot/Webhook；
- 实时 IM 监听；
- Card 创建/更新及未来 callback。

### 4.2 借鉴显式身份能力矩阵

Lark 会明确 user/bot 的 token、可见性、群成员关系和 Scope 差异。DWS 已在 Runtime 中
校验真实能力，但不能再由 Skill 手写一份独立事实表。应先在 `internal/shortcut/chat/` 抽取
typed identity capability descriptor，至少表达 identity、target kinds、content kinds、自然目标
解析支持和失败恢复类别，并让现有校验逻辑与测试共同约束它。

短矩阵由该 descriptor 与最终 leaf Schema/Help 受控生成或校验后再投影到 Skill。过渡期只链接
精确 leaf Schema，不复制 user/bot/webhook 能力表。矩阵只描述 DWS Runtime 已发布事实，不从
Lark 推导平台能力，也不成为新的身份或参数来源。

### 4.3 借鉴结果合同和富化说明

Lark 对 reaction、更新时间、thread、资源下载、分页和缺字段语义有精确说明。DWS 已有更强
的完整性 ledger，应把它固定为所有消息读取 Shortcut 的公共合同：

- 稳定消息投影字段；
- reaction/updateTime/resourceRefs 的来源；
- `--no-reactions` 与 `--download-resources` 的成本边界；
- `complete=false`、`hasMore=true`、单项 failure 的含义；
- 子消息 ID 与父 conversation fallback 规则；
- 下载的相对路径、默认不覆盖、原子落盘和跨主机 header 规则。

发布公共文档前，应在 `internal/shortcut/chatmsg/` 抽取 typed message result contract，使 Runtime
输出、分页/完整性测试和文档投影共享同一组字段定义。生成的 Markdown 片段只能单向发布到
Skill，不能反向作为 Catalog、参数或 Runtime 输入。过渡期 references 只链接精确 leaf Schema
并说明当前单页边界，不复制一张可能漂移的字段表。

### 4.4 借鉴 Card 子产品化

Lark Card 不是一个散落的 JSON 示例，而是独立的 workflow + schema + style + components +
callback 子树。DWS 若继续扩展卡片能力，也应采用同样的分层思想：

```text
card/
├── create.md          # 创建/校验/发送流程
├── update.md          # 流式状态与结束状态
├── callback.md        # 当前负面边界；平台真实支持后升级
├── schema.md          # DWS 卡片模型，不复制 Lark JSON
├── style.md           # 可选的 Agent 设计规则
└── components/        # 仅收录 DWS Runtime 真正支持的组件
```

在 callback 或任意组件下层未真实支持前，Skill 必须明确 `avoid_when`，不能通过文档先行
伪造能力。

### 4.5 借鉴“脚本不是第二 Runtime”

Lark IM 没有产品脚本重新解析 CLI 输出；分页、富化、下载、目标解析和事件生命周期都在
Shortcut/CLI 内。DWS 应采用同一原则：

- 完全被 Shortcut 覆盖的脚本删除；
- 仍有通用价值的批量流程下沉为 typed Runtime Shortcut；
- 只有暂时无法下沉的纯结果格式转换才允许脚本存在，而且只能消费稳定 DTO，不调用原子
  写命令，也不能自行做候选选择。

### 4.6 借鉴精确 Event 运维合同

保留 Lark 值得借鉴的 ready marker、stdin/SIGTERM 干净退出、结构化错误和禁止 `kill -9`
规则；继续使用 DWS 更适合 Agent 的多 EventKey 单生命周期和自然目标编译，不退回
“一 EventKey 一进程”。

## 5. 不应照搬什么

- 不复制 Lark 的 `open_id`、`chat_id`、`message_id`、Scope 和 EventKey；
- 不假设 DWS user/bot 对富媒体、Thread、Card 的支持与 Lark 对称；
- 不把 Lark Feed Group、Feed Shortcut、Flag 字段直接映射成 DWS category/Top/Favorite；
- 不强制所有正常 Chat 任务预加载完整 `dws-shared`；
- 不在 DWS 根 Skill 放完整原子 API、静态 Scope 表或 97 条 Shortcut Catalog；
- 不为文件数量对齐而机械创建 58 个 reference；
- 不采用一 EventKey 一进程的 Lark 实现限制；
- 不因借鉴 Lark 而修改 DWS 已发布的稳定 Schema wire contract。

## 6. 按文件的实施结果

### 6.1 Chat 根 Skill

文件：[`skills/multi/dingtalk-chat/SKILL.md`](../skills/multi/dingtalk-chat/SKILL.md)

保持了小根入口，没有重新展开完整 Shortcut 表，并已补充：

1. identity/message/Card/capability boundary 的单一 typed contract reference；
2. 消息分页/导出、Bot 多群 ledger、owner 选择和 Card 负面边界的精确入口；
3. 只包含 `intent_id` 的导航校验 marker，默认工具从 reviewed 路由清单解析；
4. Card create/update/callback/schema 子产品的直达链接。

### 6.2 Chat references

| 文件 | 实施结果 |
|---|---|
| [`01-messaging.md`](../skills/multi/dingtalk-chat/references/01-messaging.md) | 作为任务级编排主 reference，已接入 identity/result/fallback 合同 |
| [`chat-message.md`](../skills/multi/dingtalk-chat/references/chat/chat-message.md) | 默认发送/回复/读取/搜索已收敛到 Golden Shortcut；明确 Thread writer、callback 和 resume 的负面边界 |
| [`chat-group.md`](../skills/multi/dingtalk-chat/references/chat/chat-group.md) | 建群统一到 `+chat-create`；新增 owner 选择、成员全量分页和用户/机器人分桶语义 |
| [`chat-bot.md`](../skills/multi/dingtalk-chat/references/chat/chat-bot.md) | Bot/Webhook 发送统一到 `+messages-send`；补多群 ledger，明确 Bot 富媒体当前不支持 |
| [`chat-conversation.md`](../skills/multi/dingtalk-chat/references/chat/chat-conversation.md) | 按会话 Top/消息 Top/Favorite/category 分流，自然目标统一走 typed resolver |
| [`chat.md`](../skills/multi/dingtalk-chat/references/chat.md) | 已降级为低频原子 Catalog，不与高频 Golden Route 竞争 |
| [`intent-guide.md`](../skills/multi/dingtalk-chat/references/intent-guide.md) | 保留对象层级消歧和禁止回流原子写路径的负例 |
| `lite-recipes.md` | 已删除无信息中转层 |
| [`chat-emoji-list.md`](../skills/multi/dingtalk-chat/references/chat-emoji-list.md) | 已标注 reviewed snapshot 来源、更新日期和维护方式 |
| [`contracts.md`](../skills/multi/dingtalk-chat/references/contracts.md) | 新增 typed 身份、消息结果、Card 和能力边界投影，受 byte-exact policy 约束 |
| [`card/`](../skills/multi/dingtalk-chat/references/card/) | 新增 create/update/callback/schema 四个子产品 reference |

### 6.3 Event

已实施文件：

- [`skills/multi/dingtalk-event/SKILL.md`](../skills/multi/dingtalk-event/SKILL.md)
- [`event-im.md`](../skills/multi/dingtalk-event/references/event-im.md)
- [`event-im-keys.md`](../skills/multi/dingtalk-event/references/event-im-keys.md)
- [`event-im-lifecycle.md`](../skills/multi/dingtalk-event/references/event-im-lifecycle.md)
- [`event-im-output.md`](../skills/multi/dingtalk-event/references/event-im-output.md)
- [`event-im-operations.md`](../skills/multi/dingtalk-event/references/event-im-operations.md)

实施结果：

1. 保留 `+listen-im` 为消息/reaction/read/recall 的唯一高频入口；
2. 自动回复示例统一使用 `dws chat +messages-send`；
3. 将旧宽 reference 拆为 EventKey/Filter、lifecycle、output/handoff、operations/troubleshooting；
4. 根 Skill 只保留 ready、bounded/unbounded、干净退出等不可省略的宿主合同；
5. 订阅保护文件、重试预算和紧急恢复进入 operations reference；
6. 增加 Chat/Event 交接 drift gate，确保 `conversation_id` 和
   `sender_open_dingtalk_id` 只映射到对应稳定 ID 参数。

### 6.4 Shared、Profile、Contact 与 AI Search

| 文件/产品 | 实施结果 |
|---|---|
| `dws-shared/global-reference.md` | 已按当前 Help/Runtime 修复默认输出、全局 flag 和 envelope |
| `dws-shared/error-codes.md` | 已修复 Chat text/flag、confirmation 和 profile 错误恢复说明 |
| `dws-shared/SKILL.md` | 已将最小契约覆盖说明收窄到真实发布边界 |
| `dingtalk-profile` | 继续拥有组织/profile 选择；Chat reference 只引用，不复制 |
| `dingtalk-contact` | 明确资料查询与完整手机号职责；普通 DM 不再要求先 Contact |
| `dingtalk-aisearch` | 保留人员语义和跨源搜索；普通发消息/读群不再成为强制前置 |

### 6.5 产品 scripts

| 退役 script | Runtime 替代 |
|---|---|
| `chat_history_with_user.py` | `+chat-messages --user-query --page-all --output *.json` |
| `chat_export_messages.py` | `+chat-messages --chat-query --page-all --output *.json` |
| `bot_broadcast.py` | `+messages-send --as bot --groups|--groups-file`，输出 `im.batch-write.v1` |

3 个入口和 1 个公共 helper 的 Multi/Mono 共 8 个发布路径已删除，生成器不再复制；
退役清单受 policy 精确路径校验。
替代能力已覆盖目标唯一解析、分页终止/停滞防护、页数/条数上限、partial failure、
完整性 ledger、安全输出路径、默认不覆盖、原子发布和批量写逐项结果。

### 6.6 Schema 与 Runtime

1. 不因文档修改机械改 reviewed CommandRegistry；当前分支的根清单位于
   [`schema_command_registry/registry.json`](../internal/cli/schema_command_registry/registry.json)，
   Chat owning shard 位于
   [`products/chat.json`](../internal/cli/schema_command_registry/products/chat.json)。只有
   canonical identity、primary path、alias 或 visibility 真正变化时才修改；
2. selection 路由变化写入
   [`schema_hints/selection/chat.json`](../internal/cli/schema_hints/selection/chat.json)；
   高频路由调整必须同时处理 Shortcut 和对应原子工具：Shortcut 承接普通正向场景，原子工具
   只保留未覆盖字段/原始响应场景，并在 `avoid_when` 中明确普通任务走 Shortcut；
3. confirmation、runtime gate、interface 或参数 overlay 变化写入
   [`schema_hints/metadata/chat.json`](../internal/cli/schema_hints/metadata/chat.json)；
4. Event 同理修改各自 owning block；
5. typed resolver、消息投影、下载 ledger 和 Event consume 保持共享组件，不在每个 Shortcut
   复制实现；
6. 修复 `selection/chat.json` 中指向旧版 `schema_command_registry.json` 的失效
   `source_refs`，并增加精确路径存在性校验；
7. 新 Card/Thread/资源能力必须先有 Runtime 与 typed contract，再写 Skill reference。

## 7. 分阶段实施结果

### Phase 0：冻结正确主链（已完成）

- 9 个高频 intent 已有 reviewed route fixture 和精确 marker；
- canonical path 由 CommandRegistry 解析，confirmation 由 `ResolveMeta` 读取；
- policy 会拒绝孤儿/重复 marker、非法 fallback、安全降级、失效 source-ref 和根导航断链。

### Phase 1：selection、文档与 provenance 收敛（已完成）

- Chat/Event 根入口、冲突 references、Mono 产品索引和 Shared 合同已统一；
- Shortcut 承接普通正向场景，原子 leaf 仅保留特殊字段/原始响应 fallback；
- Chat/Event `source_refs` 已迁移到当前 product shard。

### Phase 2：Runtime 等价能力与脚本退役（已完成）

- `+chat-messages` 具备有界全量分页、typed continuation、去重、停滞防护、partial ledger
  和安全 `.json` 原子导出；
- Bot broadcast 已下沉为 `+messages-send` 多群模式和 `im.batch-write.v1`；
- 3 个 Chat 入口、1 个公共 helper 及其 Mono 副本（共 8 个路径）已删除，policy 防止重新发布。

### Phase 3：精确 references 与 typed 合同（已完成）

- 发送、读取/资源、回复/转发、建群、Bot/Webhook 已按任务边界收敛；
- identity/message/Card/capability boundary 从 typed Runtime descriptor 投影到 `contracts.md`；
- Event IM 宽文件已拆为 4 个按需 reference，Card 已建立 4 个子产品 reference。

### Phase 4：真实能力闭环（已完成可实现范围）

| 能力 | 完成状态 |
|---|---|
| 群成员全量分页/分桶/partial result | 已实现，并将 `+chat-members-list` 纳入 reviewed Schema |
| 显式 owner/自然 owner 建群 | 已实现并去重 owner/member |
| streaming-text Card create/update | 已建立 typed workflow、flow-status validation 和精确 references |
| 身份矩阵 | 已实现 user/bot/webhook 目标、内容、自然解析、幂等和 ledger 合同 |
| Thread writer | 下层无接口；typed boundary 明确 `supported=false` |
| Bot 富媒体 | 下层当前仅 text/markdown；typed boundary 明确禁止宣告支持 |
| Card action callback | 下层无 callback 消费链；callback reference 只描述负面边界 |
| 资源断点续传 | 下层无 Range/resume；保留原子落盘与默认不覆盖 |

这些负面项不是文档遗漏，而是当前平台真实能力的 fail-closed 交付。

## 8. 已落地门禁

本轮已新增：

```text
scripts/policy/check-multi-im-skill-chain.sh
scripts/policy/multi-im-skill-chain/main.go
scripts/policy/multi-im-skill-chain/main_test.go
scripts/policy/multi-im-skill-chain/testdata/intent_routes.json
```

该布局沿用现有 `scripts/policy/*/main.go` 约定，已接入 `make policy`。
`intent_routes.json` 是 reviewed policy fixture，不是 CommandRegistry、参数或安全元数据来源。

至少检查：

1. Chat/Event 根 Skill 的所有相对 reference 链接可解析；
2. 每个受控 reference marker 都能精确映射到唯一 manifest intent，且没有孤儿或重复 ID；
3. manifest 的 preferred/fallback canonical tool 都能经当前 CommandRegistry 绑定到可执行
   Cobra leaf；
4. 标记区块中的默认 CLI path 与 preferred tool 一致，不出现未允许的原子 fallback；
5. Shortcut 与对应原子工具的 selection 正负场景符合 manifest 的默认/例外关系；
6. 写路径的 Runtime confirmation 通过 `ResolveMeta` 获取并与文档声明一致；
7. `source_refs` 中的仓库路径和锚点可解析，不再接受旧 CommandRegistry 路径；
8. 退役脚本的 Multi/Mono 精确路径不得再出现；
9. Chat/Event 交接必须使用稳定 conversation/sender ID；
10. identity/message/Card/capability boundary 文档片段必须与 typed descriptor 逐字节一致；
11. Shared 文档中的 flag/example 能通过真实 Cobra command tree；
12. 新 `+chat-members-list` leaf 必须持续绑定真实 Cobra/Schema 命令；
13. 只扫描 manifest 和 marker 声明的精确文件/区块，不用自然语言相似度，也不使用会隐藏
    未来命令的通配豁免。

## 9. 验证矩阵

当前分支的最终验证矩阵为：

```bash
python3 scripts/gen_skill_shortcut_sections.py --check
./scripts/policy/check-multi-im-skill-chain.sh
./scripts/policy/check-skill-context-budget.sh
./scripts/policy/check-skill-commands.sh
./scripts/policy/check-runtime-confirmation-truth.sh
./scripts/policy/check-schema-catalog.sh
./scripts/policy/check-generated-drift.sh

go test ./internal/shortcut/targetresolver -count=1
go test ./internal/shortcut/chat ./internal/shortcut/smart ./internal/app \
  -run 'Test.*(NaturalTarget|MessageView|MessagesSend|MessagesReply|ChatCreate|ListenIM)' \
  -count=1
go test ./internal/app -run '^(TestEventListenIM|Test.*ListenIM.*)$' -count=1
```

由于本轮修改了 Agent-visible command、selection、metadata 和参数，以下生成与交付校验为必跑项：

```bash
make generate-schema
./scripts/policy/check-runtime-confirmation-truth.sh
./scripts/policy/check-generated-drift.sh
./scripts/policy/check-schema-catalog.sh
```

真实写审计只在需要验证对应能力时运行，不能用 live audit 代替上述确定性门禁。

### 9.1 本轮最终结果

2026-08-03 在当前稳定工作树上复验：

- `make generate-schema` 成功：26 产品、849 工具、159 组参数别名；
- Multi IM policy 通过：9 个 reviewed intent；
- Skill 命令面通过：1009 条可执行 path；
- Runtime confirmation truth 通过：137 个 gated 工具；
- Schema Catalog、CommandRegistry 和两次独立生成漂移检查通过；
- `DWS_PACKAGE_VERSION=0.0.0-test go test -p 1 ./...` 通过；默认包级并行复跑曾因
  `internal/helpers` 的共享测试环境竞争失败一次，该包隔离强制复跑通过；
- `go build -o /tmp/dws-multi-im-build ./cmd` 通过。

### 9.2 真实环境端到端审计

2026-08-03 使用当前分支构建的 CLI、有效个人登录态和真实 DingTalk 后端完成两组审计。
报告只保留命令名、下层工具名、数量和脱敏错误类别，不落业务 payload：

```bash
python3 scripts/run_chat_shortcut_live_audit.py \
  --dws /tmp/dws-multi-im-final \
  --out-dir /tmp/dws-im-live-audit-019fc653/read-current-branch \
  --timeout 90 --max-group-probes 8 --max-member-probes 8

python3 scripts/run_chat_shortcut_live_write_audit.py \
  --dws /tmp/dws-multi-im-final \
  --out-dir /tmp/dws-im-live-audit-019fc653/write-current-branch \
  --timeout 90 --yes-live
```

结果：

- 读链路共 34 项：28 项 `pass`，2 项 `pass_empty`，4 项 `fixture_blocked`，无投影
  不一致、下层错误或超时；
- 写链路共 57 项：46 项 `pass`，11 项 `external_fixture_required`，无执行错误；
- 写审计使用单用户临时群，并在 `finally` 中恢复可逆开关、解散临时群；按 4 类审计
  名称前缀复查，残留为 0；
- 未覆盖项都保留精确 fixture 原因：数字 `groupId`、真实 `openTaskId`、媒体消息、配置
  完成的机器人/Webhook、第二测试成员和待审批入群请求；不使用业务数据伪造通过；
- `+conversation-mute-at-all` 和 `+conversation-mute-red-envelope` 均在真实环境通过，旧 Mono
  reference 的“当前不可用”结论已移除；真实前置条件是先开启总免打扰，否则平台返回
  `NotificationOffNotEnabled`；连续审计两个子开关时，红包应先于 @所有人，或在恢复
  @所有人通知后重新开启总免打扰；
- `+at-me` 曾因上层同时提供 `messages`/`items` 兼容别名被审计器重复计数，现固定选择一个
  canonical message projection；该问题属于审计器误报，不是 Runtime 数据丢失；
- 首轮 `+messages-list-unread-conversations` 出现一次瞬时后端超时，同一原子入口和最终
  完整复测均通过，未将瞬时故障掩盖为 contract-only。

真实审计补充确定性门禁，但不替代 Schema、policy、单元测试和全量 Go 测试。

## 10. 完成定义

DWS Multi IM 优化完成应同时满足：

- 根 Skill、所有精确 references 和相关子产品对同一意图选择同一默认入口；
- 不存在自然目标取第一候选、reference 指示跨 profile 复用解析结果或第二 Runtime 猜
  envelope；没有 typed provenance 时，不对任意显式稳定 ID 宣称本地跨 profile 硬校验；
- 写操作的 confirmation 在文档、Schema、Runtime 和最终交付路径一致；
- 单页和全量消息读取、资源下载始终暴露完整性/失败 ledger；全量替代脚本时具备明确分页
  上限、终止和导出合同；
- 高频任务不需要预加载完整 Shared、产品 Catalog 或原子命令全集；
- 原子 fallback 有明确原因和能力损失说明；
- Shortcut/原子 selection 不为同一普通任务同时声明正向默认入口；
- reviewed `source_refs` 指向当前存在的 authored source 或可验证 Runtime evidence；
- 退役脚本不在 Mono/Multi 发布树中，且不会被生成器重新引入；
- Card、Thread、Bot、资源等能力只发布下层真实支持的正向范围，其余由 typed
  负面边界防止误选；
- 所有生成、Schema、policy 和 focused Runtime tests 通过；
- 不改变未版本化的稳定 Schema wire contract。

## 11. Review 交接

当前工作树已同时包含路由/文档、Runtime 等价能力、脚本退役和真实能力边界四类改动。
Review 建议按以下四个逻辑分组查看，但不应将它们当作未完成的后续 PR：

1. reviewed intent route、selection/metadata、CommandRegistry 与生成 Catalog；
2. typed resolver、message/member/card/identity contracts 与 Runtime tests；
3. Chat/Event/Card references、Shared/Mono 消费者与 typed drift policy；
4. 退役脚本、生成器发布边界与防回流门禁。
