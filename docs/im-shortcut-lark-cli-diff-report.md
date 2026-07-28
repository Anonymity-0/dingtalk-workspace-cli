# DWS IM Shortcut 与 lark-cli 差异报告

> 日期：2026-07-28
> DWS 基线：分支 `codex/im-shortcut-optimization`，91 个已审阅 Chat Shortcut，其中 88 个公开、3 个确认下层失败后标记 `unavailable` 并隐藏
> Lark 基线：lark-cli 1.0.78，`lark-im` Skill 当前推荐 21 个 IM Shortcut
> 比较对象：用户意图、身份与权限边界、输入参数、结果形状、分页/富化、安全和错误恢复；不以命令名称相似作为等价依据。

## 1. 结论

| 指标 | 结果 | 解释 |
|---|---:|---|
| DWS 当前公开 Chat Shortcut | 88 | 88 个 reviewed + available 入口公开；disposition 继续标识 Smart、Adapter、Schema 投影和兼容别名 |
| DWS Runtime Schema Shortcut 投影 | 42 / 88 | 其余 46 项通过 Shortcut Catalog 公开执行，并以精确 reviewed exclusion 等待逐项 Schema curation |
| DWS 源码 Shortcut 语义 | 91 / 91 | 88 个公开、3 个隐藏；全部映射到 Query 并保留显式 availability |
| Lark 推荐 IM Shortcut | 21 | 当前 `lark-im` Skill 与 `lark-cli im --help` 一致 |
| Lark Shortcut 可路由到 DWS 公开 Shortcut | 14 / 21 | 包含 Smart/Adapter 和已公开的稳定 leaf 投影 |
| Lark Shortcut 仍需路由到 DWS 原子 leaf | 5 / 21 | 建群、回复和三类收藏操作在 DWS 没有同构 Shortcut 名称 |
| 部分等价、需 recipe 或能力降级 | 2 / 21 | `+chat-update`、`+feed-group-query-item` |
| GSB 契约覆盖 | DWS 197/197；Lark 55/55 | 无 missing/stale expectation；不代表两边都完成真实业务执行 |

核心判断：

1. 两边在群列表、成员、消息历史、搜索、资源下载和 thread 读取上高度类似，但分页、身份、资源类型和富化深度不同。
2. DWS 现在公开 88 个可用入口，并用 disposition 区分默认 Smart 路由与 1:1 Schema 投影；lark-cli 的推荐 Shortcut 更接近“稳定 API recipe + 统一身份/分页/富化”。
3. 原先隐藏的 79 个 Shortcut 中，76 个已打开；3 个已用原生命令复现下层失败，保留实现但标记 `unavailable`，不暴露给用户或 Agent。
4. Runtime Schema 已投影其中 42 个公开入口；其余 46 个并未隐藏，只是等待逐项 selection/metadata curation。GSB 对两套目录中的同一可执行入口去重。
5. Pin、Top、Flag、Feed Shortcut、Mute、Moderation 等名称相近但对象不同，是 Agent 最容易误选的区域。
6. 本报告的 DWS 侧包含真实执行证据；Lark 侧本轮只做实时 Help/Schema/Skill 契约复核，没有执行 Lark 业务写操作。

## 2. 等价关系定义

| 关系 | 判定标准 |
|---|---|
| 直接近等价 | 用户意图和主要结果形状一致，只存在平台 ID、分页或字段差异 |
| 同意图、边界不同 | 用户目标相同，但身份、认证、输入抽象或返回完整性不同 |
| DWS Smart / Lark Recipe | DWS 单个 Shortcut 完成名称解析或多目标编排；Lark 需要 Contact + IM 多步执行 |
| Lark Shortcut / DWS Leaf | Lark 提供推荐 Shortcut；DWS 对应意图当前只有 Runtime Schema 原子命令 |
| 部分等价 | 只能覆盖部分字段或需“列出后本地过滤”等降级 recipe |
| 单侧能力 | 另一侧当前没有同构能力，不能拼造命令 |

## 3. DWS 12 个 Smart / Semantic 代表入口的逐项对照

| DWS Shortcut | lark-cli 对应 | 关系 | 相同点 | 关键差异 |
|---|---|---|---|---|
| `+at-me` | `+messages-search --is-at-me` | 同意图、DWS 专项化 | 跨会话找 @我的消息 | DWS 自动构造时间窗并提供专项稳定投影；Lark 复用通用搜索和更丰富过滤器 |
| `+broadcast` | 多次 `+messages-send --user-id` | DWS Smart / Lark Recipe | 向多位用户发送相同内容 | DWS 按多人姓名逐一解析、拒绝歧义并隔离单人失败；Lark 无直接 broadcast Shortcut |
| `+chat-members-list` | `+chat-members-list` | 直接近等价 | 用户/机器人分桶，支持过滤成员类型 | DWS 可按群名唯一解析并组合两个下层工具；Lark 支持 user/bot 身份、`--page-all` 和服务端截断提示 |
| `+chat-messages` | `+chat-messages-list` | 直接近等价 | 群聊/单聊历史、时间范围、分页、sender、reaction、update time | DWS 统一 DingTalk 群与两类单聊目标并保留引用/threadId；Lark 支持 user/bot、`--download-resources` 和更完整的自动富化 |
| `+conversation-info` | `schema im.chats.get` / `im chats get` | DWS Semantic / Lark Raw | 查询会话详情 | DWS 统一群 openConversationId 与单聊 openDingTalkId；Lark 当前没有同名推荐 Shortcut，按 raw API 路由 |
| `+dm` | `+messages-send --user-id` | DWS Smart / Lark Shortcut | 给单个用户发消息 | DWS 接受姓名并调用 Contact 唯一解析；Lark IM Shortcut 接受 open_id，身份可选 user/bot，支持富媒体和幂等键 |
| `+messages-resource-download` | `+messages-resources-download` | 直接近等价 | 获取消息资源并安全落盘 | DWS 当前只支持 mediaId，默认不覆盖、临时文件和原子发布；Lark 支持 image/file、大文件 8MB 分片和 Content-Type 扩展名 |
| `+my-groups` | `+chat-list` | 同意图、边界不同 | 列出当前身份加入的群并分页 | DWS 提供本地群类型过滤和稳定 DingTalk 字段；Lark 支持 user/bot、group/p2p、排序、`--exclude-muted`、`--page-all` |
| `+search-msg` | `+messages-search` | 直接近等价 | 关键词、时间范围、消息身份和分页 | DWS 自动构造最近 N 天窗口并保留引用/reaction；Lark 支持 sender/chat/attachment 等更多过滤和 mget/chat 富化 |
| `+send-to-group` | `+chat-search` → `+messages-send --chat-id` | DWS Smart / Lark Recipe | 按可读群名找到群后发消息 | DWS 内置唯一匹配、歧义拒绝和 dry-run；Lark 需要显式两步，但发送格式和 user/bot 身份更丰富 |
| `+thread-replies` | `+threads-messages-list` | 直接近等价 | 按 thread 读取回复、分页和消息正文 | DWS 接受 threadId 并兼容 topicId，已用真实第二成员回复验证非空投影；Lark 可从 om_/omt_ 自动解析 thread、支持 user/bot |
| `+unread-chats` | 无直接等价 | DWS 单侧能力 | — | Lark `+chat-list` 没有同构“仅未读会话”筛选 |

### 3.1 相似度最高的六组

1. `+chat-members-list` ↔ `+chat-members-list`
2. `+chat-messages` ↔ `+chat-messages-list`
3. `+messages-resource-download` ↔ `+messages-resources-download`
4. `+search-msg` ↔ `+messages-search`
5. `+thread-replies` ↔ `+threads-messages-list`
6. `+my-groups` ↔ `+chat-list`

它们适合继续做参数、分页、富化和输出 ontology 对齐，但不应强行统一平台 ID 或认证模型。

## 4. Lark 21 个推荐 Shortcut 在 DWS 中的完整路由

| lark-cli Shortcut | DWS 推荐路由 | 关系 | 备注 |
|---|---|---|---|
| `+chat-create` | `dws chat group create` | Lark Shortcut / DWS Leaf | 同为建群；DWS 当前 91 项中没有同构 Shortcut |
| `+chat-list` | `dws chat +my-groups` | 公开语义对应 | 群/p2p、身份和过滤选项不同 |
| `+chat-members-list` | `dws chat +chat-members-list` | 直接近等价 | 两边都输出 user/bot 分桶 |
| `+chat-messages-list` | `dws chat +chat-messages` | 直接近等价 | Lark 多资源自动下载更强；DWS target 归一更强 |
| `+chat-search` | `dws chat +chat-search` | 公开 leaf 投影 | 与 `dws chat search` 同能力，公开目录保留兼容入口 |
| `+chat-update` | `dws chat group rename` | 部分等价 | 群名称可对应；Lark 同时支持 description，DWS 需按具体设置另选 leaf |
| `+messages-mget` | `dws chat +messages-mget` | 公开 leaf 投影 | Lark 自动展开 thread 和 sender/reaction 富化；DWS 保留稳定兼容投影 |
| `+messages-reply` | `dws chat message reply` | Lark Shortcut / DWS Leaf | 都支持回复；Lark 支持更多富媒体、thread 和幂等选项 |
| `+messages-resources-download` | `dws chat +messages-resource-download` | 直接近等价 | 资源类型、分片和文件名处理不同 |
| `+messages-search` | `dws chat +search-msg` | 直接近等价 | Lark 过滤器更广；DWS 时间窗和 DingTalk 投影更稳定 |
| `+messages-send` | `+dm` / `+send-to-group` / `dws chat message send` | 同意图、边界重构 | DWS 按自然语言目标拆入口；Lark 用 user/bot + chat-id/user-id 统一发送 |
| `+threads-messages-list` | `dws chat +thread-replies` | 直接近等价 | thread 标识和分页参数不同 |
| `+flag-create` | `dws chat message add-favorite` | Lark Shortcut / DWS Leaf | 都是个人消息收藏；不要映射到 Pin |
| `+flag-cancel` | `dws chat message remove-favorite` | Lark Shortcut / DWS Leaf | Lark 可 best-effort 双层取消；DWS 是明确收藏层 |
| `+flag-list` | `dws chat message list-favorites` | Lark Shortcut / DWS Leaf | Lark 还处理 feed-layer thread flag |
| `+feed-shortcut-create` | `dws chat +conversation-set-top` | 公开 leaf 投影 | 都影响用户侧边栏；不是消息 Pin |
| `+feed-shortcut-remove` | `dws chat +conversation-set-top --off` | 公开 leaf 投影 | Lark 有逐项失败 ledger；DWS 为单会话开关 |
| `+feed-shortcut-list` | `dws chat +conversation-list-top` | 公开语义适配 | 两边都是用户个人会话置顶视图；DWS 额外把 `singleChat` 规范化为 `group/direct`，支持按会话类型过滤 |
| `+feed-group-list` | `dws chat +category-list` | 公开 leaf 投影 | 都是用户会话分组/标签 |
| `+feed-group-list-item` | `dws chat +category-list-conversations` | 公开 leaf 投影 | Lark 会富化 chat_name；DWS 返回 DingTalk 会话投影 |
| `+feed-group-query-item` | 分组列表后按会话 ID 本地过滤 | 部分等价 | DWS 当前没有同构的“按多个 feed ID 精确查项”语义入口 |

## 5. 名称相似但不能当作相同能力

| 易混概念 | DWS | Lark | 正确判断 |
|---|---|---|---|
| 消息 Pin | `message set-pin-msg` / `list-pin-msg` | `pins.create/delete/list` | 群内消息 Pin，基本同类 |
| 消息 Top | `message set-top-msg` | 无独立同构能力 | 不能因为 Lark 有 Pin 就宣称完全等价 |
| 会话置顶 | `chat set-top` / `list-top-conversations` | `+feed-shortcut-create/remove/list` | 用户侧边栏状态，不是消息 Pin |
| 消息收藏 | `add-favorite/remove-favorite/list-favorites` | `+flag-create/cancel/list` | 个人收藏层，不是 Feed Shortcut |
| 会话免打扰 | `chat mute` / `+conversation-mute` | `chat.user_setting.batch_update is_muted` | 当前用户的通知偏好 |
| 屏蔽 @所有人 | `mute-at-all` | `chat.user_setting.batch_update is_mute_at_all` | 当前用户偏好，不是群禁言 |
| 全员/成员禁言 | `group-mute` / `mute-member` | `chat.moderation.update` | 群治理权限，影响成员发言 |
| 群更新 | `group rename` | `+chat-update` | 名称可对应；description 等字段不能自动推断 |
| 个人群备注 | `group update-alias` | 无直接同构 | 只对当前用户可见，不是群名称 |
| 个人群昵称 | `group update-nick` | `chat.nickname.update/delete` | 都是自己的群内昵称，与群名无关 |
| 自定义群角色 | `group-role ...` | 无直接同构 | Lark managers 只是管理员，不等于任意业务角色 |
| Emoji Reaction | `add/remove-emoji` | `reactions.create/delete/list` | 基本同类 |
| 文字表情 | `create/add/remove-text-emotion` | 无直接同构 | DingTalk 自定义文字回应，不能降级为固定 emoji 后声称等价 |
| 身份 | current user / app bot / Webhook 分 leaf | `--as user` / `--as bot` | 认证、权限和回执不同；Webhook 不应机械并入同一个身份枚举 |

## 6. 平台差异

### 6.1 DWS 更强或更原生的语义

- 姓名解析单聊、群名解析发送、多人姓名 broadcast，能够直接处理 Agent 常见的自然语言目标。
- 专项 `+at-me`、`+unread-chats`，减少模型自己拼时间窗或本地筛选。
- DingTalk 自定义群角色、入群审批记录、新成员历史可见范围、数字群号、红包提醒开关。
- 自定义文字表情、全量/单会话红点、隐藏会话、标记未读等 DingTalk 状态语义。
- 88 个可用 Shortcut 公开，同时保留 `disposition` 和 `semantic_delta`；3 个当前不可执行的入口明确隐藏，避免 Agent 路由到已知失败能力。

### 6.2 lark-cli 更强或更成熟的语义

- `+messages-send` / `+messages-reply` 在同一契约下支持 user/bot、文本/Markdown/post/media/card 和幂等键。
- 消息列表支持 `--download-resources`，资源下载支持 image/file 和大文件分片。
- `+messages-search` 支持 sender、chat、attachment、时间范围和 `--page-all`，并做 mget/chat 批量富化。
- Feed Shortcut、Flag、Feed Group 三类个人组织对象边界清晰，批量操作有 partial-failure ledger。
- 原生 IM 还提供应用内/电话/短信加急、图片上传、批量用户会话设置和 moderation 只读/写接口。

### 6.3 当前 DWS 的真实缺口

| 缺口 | 当前证据 | 建议 |
|---|---|---|
| Thread 自动造数 | 已用真实第二成员回复完成 1/1 非空验证；当前 MCP 仍无回复写接口 | 保留人工 Fixture，待上游有 writer 后纳入全自动回归 |
| 消息资源类型 | 仅 mediaId | 支持 fileId/文件消息、分片下载和 Content-Type 扩展名 |
| 列消息自动下载资源 | 当前需单独调用资源 Shortcut | 参考 Lark 增加默认关闭的 `--download-resources`，单资源失败隔离 |
| 搜索过滤深度 | 关键词/时间窗为主 | 增加 sender/chat/attachment 过滤、page-all 和富化失败 ledger |
| 发送身份体验 | user/bot/webhook 原子 leaf 与公开投影并存，Smart 主要面向 current user | 可做显式身份路由 Smart Shortcut，但不得隐藏认证、scope 和回执差异 |
| 三个下层错误 | 两项 1002、一项合并转发权限 | 作为上游 MCP/服务问题跟踪，不在 Shortcut 层吞错或伪造降级 |

## 7. 对齐建议

### P0：保持当前方向

1. 保持 88 个可用 Shortcut 公开，并让 Agent 优先选择 `primary_smart` / `semantic_adapter`；3 个已知下层失败入口在修复和复测前保持 `unavailable`。`schema_leaf` / `alias_internal` 用于精确或兼容路由，不与 Smart 入口等权竞争。
2. 继续使用统一 Message/Page 投影，保证 sender、quote、reaction、updateTime、threadId 和分页完整性。
3. 对所有名字相似的状态能力，在 Hint/Skill 中明确对象层：message、conversation、feed、group moderation、personal preference。

### P1：值得跟进 lark-cli 的效果优化

1. 给历史消息、mget 和 thread 读取增加可选资源自动下载。
2. 扩展资源下载到 fileId、大文件分片、扩展名推断和局部失败 ledger。
3. 给消息搜索增加 sender/chat/attachment 过滤和自动分页。
4. 让发送/回复返回统一 delivery identity、idempotency 和 partial failure 结构。
5. 为分类/会话置顶提供清晰的批量 recipe，但保留 DingTalk 与 Lark 对象差异。

### 不建议复制

- 不把 `receive_id_type` 机械暴露给所有 DWS Shortcut；它是接口 binding 事实。
- 不把 current user、app bot、Webhook 合成一个模糊身份而丢失权限和回执语义。
- 不把 Pin、Top、Flag、Feed Shortcut 合并成一个“置顶/收藏”万能 Shortcut。
- 不再为了追平 lark-cli 名称新增重复包装；现有 91 个已审阅语义通过 availability 与 disposition 分别管理可用性和选择优先级。

## 8. 验证证据

- [DWS IM Shortcut 分层测试报告](im-shortcut-test-report.md)
- [DWS IM GSB Query 集](dws-im-gsb-core-query-set.md)
- [本轮 GSB 197/197 与 lark-cli 55/55 契约复核结论](dws-im-gsb-core-query-set.md#101-2026-07-28-契约复核结果)
- [IM 优化设计](im-optimization-design.md)
- 本机实时命令：`lark-cli --version`、`lark-cli im --help`、逐路径 `lark-cli schema/--help`
- 本机 Lark Skill：`$HOME/.agents/skills/lark-im/SKILL.md`

## 9. 使用边界

契约对齐只证明命令存在、Help/Schema 可解析和 Query 映射完整。DWS 的 79 项真实成功、0 项待证空结果、9 项 Fixture 阻塞和 3 项下层错误应以测试报告为准；本轮没有对 Lark 业务账号执行对应的 21 项 live 写入，因此不能把 Lark 55/55 契约覆盖解释为真实调用成功率。
