---
name: dingtalk-chat
description: 钉钉群聊与消息。Use when 用户提到 发消息/编辑或撤回已发送消息/单聊/群聊/建群/普通群升级外部群/群昵称/会话分组/群成员管理/@消息/搜索聊天记录/话题回复/收藏消息/机器人群发/Webhook通知/发送或下载消息图片与文件。Distinct from dingtalk-ding(紧急DING消息/短信/电话)、dingtalk-mail(邮件)、dingtalk-edu-group(班级群)。命令前缀：dws chat。
cli_version: ">=0.2.14"
metadata:
  category: product
  stability: experimental
  requires:
    bins:
      - dws
---

# 钉钉群聊 / 消息 Skill

> 🧪 **EXPERIMENTAL · 试验版 / Preview** — multi 模式当前未达 stable 标准。全部 dingtalk-* skill 已通过 dispatch verifier，但接口、命名、跨 skill 引用后续可能调整；生产 / 共享环境请优先使用 mono 模式（`dws skill setup --mode mono`）。问题请提 issue 反馈。

> **PREREQUISITE:** Read the `dws-shared` skill first for auth, global flags, product routing, URL preflight, error codes, and safety rules. The `dws` binary must be on PATH.

<!-- SAFETY_PREAMBLE_INJECT -->

> ⚠️ **命令可用性以当前 dws 二进制为准**。服务发现已下线，本文档随内置 skill 发布；如果 `dws <cmd> --help` 不存在，说明当前版本未暴露该命令。若命令存在但调用失败，请按错误中的 endpoint 或 tool 提示确认静态端点目录和后端工具注册。实际调用前可用 `dws <cmd> --help` 或 `--dry-run` 验证。


> 命令参考：[chat.md](references/chat.md)；表情：[chat-emoji-list.md](references/chat-emoji-list.md)；剧本：[01-messaging.md](references/01-messaging.md)。

<!-- VISIBLE_SHORTCUTS_START -->
## Shortcuts（无专用脚本/recipe 时优先）

以下 shortcut 同时进入公开 catalog 与 Runtime Schema。先按本 skill 的意图表、脚本和 recipe 路由：存在精确覆盖该场景的专用脚本/recipe 时按其执行；否则用户意图命中时，shortcut 优先于手写原子命令。用 leaf Schema（例如 `dws schema --cli-path "chat +<shortcut>" --format json`）读取 Agent 选择、参数、约束、风险和确认语义；用 `dws shortcut list --service chat --format json` 批量发现；最后以 `dws chat <shortcut> --help` 核对当前 Cobra flags。

| Shortcut | 风险 | 适用场景 |
|---|---|---|
| `dws chat +at-me` | write | 查最近 @我 的消息（自动算时间窗，投影发送人/时间/内容/会话） |
| `dws chat +bot-find` | read | 搜索全部可用机器人（含他人/官方，返回 openDingTalkId 可发单聊） |
| `dws chat +bot-search` | read | 搜索当前用户自己创建的机器人 |
| `dws chat +broadcast` | write | 按姓名逐一给多个人群发同一条单聊消息（自动解析 userId、逐个发送） |
| `dws chat +category-add-conversation` | write | 将会话移动到指定的自定义分组中 |
| `dws chat +category-create` | write | 创建用户自定义会话分组 |
| `dws chat +category-delete` | high-risk-write | 删除用户自定义会话分组 |
| `dws chat +category-list` | read | 获取用户自定义会话分组 |
| `dws chat +category-list-conversations` | read | 拉取指定自定义会话分组下的会话 |
| `dws chat +category-remove-conversation` | write | 将会话从指定的自定义分组中移出 |
| `dws chat +category-rename` | write | 更新用户自定义会话分组的名称 |
| `dws chat +chat-add-bot` | write | 将机器人添加到群中 |
| `dws chat +chat-audit-join` | write | 审批入群验证（通过/拒绝/删除/忽略/拉黑） |
| `dws chat +chat-bots` | read | 查看群内所有机器人 |
| `dws chat +chat-create` | write | 以当前用户身份创建钉钉群聊 |
| `dws chat +chat-dismiss` | high-risk-write | 解散群聊（不可逆，需群主权限） |
| `dws chat +chat-get-by-id` | read | 根据群号获取群聊信息 |
| `dws chat +chat-invite-url` | read | 获取群邀请链接 |
| `dws chat +chat-list-all` | read | 分页拉取我加入的所有群列表 |
| `dws chat +chat-list-join-requests` | read | 分页拉取入群验证记录 |
| `dws chat +chat-list-mine` | read | 拉取我创建/管理的群 |
| `dws chat +chat-members-get` | read | 根据成员 openDingTalkId 批量查询群成员详情 |
| `dws chat +chat-members-list` | read | 列出群成员并把用户与机器人分桶（支持群名语义解析） |
| `dws chat +chat-messages` | write | 拉取某个会话（群聊或单聊）的消息列表并投影出发言人/文本/时间 |
| `dws chat +chat-mute` | write | 全员禁言 / 取消全员禁言 |
| `dws chat +chat-mute-member` | write | 指定群成员禁言 / 取消禁言 |
| `dws chat +chat-quit` | write | 退出群聊 |
| `dws chat +chat-remove-bot` | high-risk-write | 从群内移除机器人 |
| `dws chat +chat-role-add` | write | 添加群身份 |
| `dws chat +chat-role-list` | read | 拉取会话的群身份列表 |
| `dws chat +chat-role-query-user` | read | 查询群成员的群身份 |
| `dws chat +chat-role-remove` | high-risk-write | 删除群身份 |
| `dws chat +chat-role-remove-user` | write | 移除用户的指定群身份 |
| `dws chat +chat-role-set-user` | write | 设置用户的群身份（覆盖该用户的全部群身份） |
| `dws chat +chat-role-update` | write | 更新群身份名称 |
| `dws chat +chat-search` | read | 按关键词搜索群聊 |
| `dws chat +chat-set-admin` | write | 设置 / 取消群管理员 |
| `dws chat +chat-set-history` | write | 设置新成员入群可查看历史消息范围 |
| `dws chat +chat-transfer-owner` | write | 转让群主 |
| `dws chat +chat-update` | write | 更新群名称（仅名称，不支持 description） |
| `dws chat +chat-update-alias` | write | 设置群备注（仅自己可见） |
| `dws chat +chat-update-icon` | write | 更新群头像 |
| `dws chat +chat-update-nick` | write | 设置当前用户在群内的群昵称 |
| `dws chat +chat-update-settings` | write | 更新群设置（settingKey + status） |
| `dws chat +conversation-clear-all-red-point` | write | 清除所有会话红点（全部已读） |
| `dws chat +conversation-clear-messages` | high-risk-write | 清空当前用户指定会话的聊天记录（仅本人视角，不可逆） |
| `dws chat +conversation-clear-red-point` | write | 清除会话红点 |
| `dws chat +conversation-hide` | write | 会话列表中隐藏会话（收到新消息会重新出现） |
| `dws chat +conversation-info` | read | 获取会话信息（群聊传 --group，单聊传 --open-dingtalk-id） |
| `dws chat +conversation-list` | read | 分页获取当前用户的全部会话列表（单聊+群聊） |
| `dws chat +conversation-list-top` | read | 拉取置顶会话列表，可只看群聊或单聊 |
| `dws chat +conversation-mark-read` | write | 标记消息已读（该消息及之前的消息都标记为已读） |
| `dws chat +conversation-mark-unread` | write | 标记会话为未读 |
| `dws chat +conversation-mute` | write | 会话消息免打扰（支持单聊/群聊） |
| `dws chat +conversation-set-top` | write | 批量会话置顶 / 取消置顶（最多 10 个） |
| `dws chat +dm` | write | 按姓名直接给某人发单聊消息（自动解析 userId） |
| `dws chat +feed-group-query-item` | read | 在会话分组结果中按会话 ID 精确查询多项 |
| `dws chat +flag-cancel` | write | 取消收藏一条或多条消息（最多 10 条） |
| `dws chat +flag-create` | write | 收藏一条或多条消息（最多 10 条） |
| `dws chat +flag-list` | read | 分页查询当前用户收藏的消息 |
| `dws chat +group-members` | read | 按群名列出群成员（自动搜群解析 openConversationId） |
| `dws chat +messages-add-emoji` | write | 对消息添加 emoji 表情回应 |
| `dws chat +messages-add-text-emotion` | write | 对消息添加文字表情回应 |
| `dws chat +messages-batch-recall-by-bot` | write | 机器人撤回单聊消息 |
| `dws chat +messages-batch-send-by-bot` | write | 机器人批量向用户发送单聊 Markdown 消息 |
| `dws chat +messages-combine-forward` | write | 合并转发多条消息 |
| `dws chat +messages-create-text-emotion` | write | 创建文字表情（获取 emotionId） |
| `dws chat +messages-forward` | write | 转发单条消息 |
| `dws chat +messages-forward-topic` | write | 转发话题消息到目标会话 |
| `dws chat +messages-list` | read | 拉取群聊会话消息 |
| `dws chat +messages-list-direct` | read | 拉取单聊会话消息 |
| `dws chat +messages-list-pin` | read | 拉取会话中钉住的消息列表 |
| `dws chat +messages-list-unread-conversations` | read | 获取有未读消息的会话列表 |
| `dws chat +messages-mget` | write | 根据消息 ID 批量查询消息（最多 50 条） |
| `dws chat +messages-query-send-status` | read | 查询消息发送状态 |
| `dws chat +messages-read-status` | read | 查询消息的已读/未读状态 |
| `dws chat +messages-recall` | write | 撤回当前用户发送的消息 |
| `dws chat +messages-recall-by-bot` | write | 机器人撤回群消息 |
| `dws chat +messages-remove-emoji` | write | 移除消息的 emoji 表情回应 |
| `dws chat +messages-remove-text-emotion` | write | 移除消息的文字表情回应 |
| `dws chat +messages-reply` | write | 以当前用户身份引用回复消息（自动补全原发送者） |
| `dws chat +messages-resource-download` | write | 安全下载消息资源（图片/视频/语音/文件）到本地 |
| `dws chat +messages-resource-url` | read | 获取消息资源（图片/视频/语音）下载链接 |
| `dws chat +messages-send` | write | 统一发送文本、Markdown、当前用户文件或已有 mediaId 图片 |
| `dws chat +messages-send-by-bot` | write | 机器人向群聊发送 Markdown 消息 |
| `dws chat +messages-send-by-webhook` | write | 自定义机器人 Webhook 发送群消息 |
| `dws chat +messages-send-card` | write | 创建流式卡片，可在同一次调用中写入内容并结束 |
| `dws chat +messages-set-pin` | write | 钉住消息（Pin） |
| `dws chat +messages-set-top` | write | 置顶消息 |
| `dws chat +messages-unset-pin` | write | 取消钉住消息（Unpin） |
| `dws chat +messages-unset-top` | write | 取消置顶消息 |
| `dws chat +messages-update-card` | write | 流式更新卡片内容（最后一次 --flow-status 应为 3） |
| `dws chat +my-groups` | read | 列出我加入的群，可按类型过滤并投影关键字段 |
| `dws chat +search-msg` | write | 多维搜索消息，可全量翻页并批量富化详情 |
| `dws chat +send-to-group` | write | 按群名直接给群发消息（自动搜群解析 openConversationId） |
| `dws chat +thread-replies` | write | 拉取某条话题消息的全部回复并投影出发言人/文本/时间 |
| `dws chat +unread-chats` | read | 列出我有未读消息的会话（投影会话名/未读数/会话ID） |
<!-- VISIBLE_SHORTCUTS_END -->

## IM Shortcut 优先路由

- 统一发送优先用 `chat +messages-send`：通过 `--as user|bot|webhook` 选择身份，并只传该身份真实支持的目标、内容和凭据。仅在需要原子命令的特定返回结构或兼容参数时，才降级到 `chat message send` / `send-by-bot` / `send-by-webhook`。
- 消息查询按意图选择：指定会话用 `+chat-messages`，跨维度过滤/全量翻页用 `+search-msg`，@我用 `+at-me`，已知消息 ID 批量富化用 `+messages-mget`，已知 thread/topic ID 读取回复用 `+thread-replies`。
- 查询结果中的 `resourceRefs` 是可继续执行的资源上下文。引用、回复或合并转发中的子消息必须使用该子消息返回的 `messageId`；子消息缺会话 ID 时才继承父消息的 `openConversationId`，不要把所有资源重新绑定到父消息。
- `+at-me`、`+chat-messages`、`+messages-mget`、`+search-msg`、`+thread-replies` 的 `--download-resources` 会写本地文件；`+messages-resource-download` 始终写本地文件。它们按 leaf Schema 发布为 `write/user_required`，先确认目标目录和覆盖策略，获得用户确认后才加 `--yes`。
- `+messages-send` 会按身份规范化 @ 占位符：user 使用 `<@id>` / `<@all>`，bot/webhook 使用 `@id` / `@手机号` / `@all`；只需声明 `--at-*` / `--at-all`，缺失占位符会自动补齐。

## 意图表

| 用户说 | 命令 |
|--------|------|
| "发消息给张三" | `dws chat +messages-send --as user --open-dingtalk-id <id> --text "<内容>"` |
| "发到XX群" | `dws chat +chat-search --query "<群名>"` → `dws chat +messages-send --as user --chat-id <openConversationId> --text "<内容>"` |
| "建群" / "拉人进群" | `dws chat +chat-create` / `dws chat group members add` |
| "改群名" / "踢人" | `dws chat group rename` / `dws chat group members remove --yes`（踢人不可逆，确认目标后加 --yes；踢群主会被 CLI 拦截，需先 `transfer-owner`）|
| "@我消息" | `dws chat +at-me` |
| "查群聊记录" | `dws chat +chat-messages --group <openConversationId>` |
| "收藏/取消收藏这条消息" | `dws chat +flag-create` / `dws chat +flag-cancel` |
| "查看我收藏的消息" | `dws chat +flag-list` |
| "用机器人发消息" | `dws chat +messages-send --as bot --robot-code <code> --chat-id <id> --text "<内容>"` |
| "Webhook 推一条" | `dws chat +messages-send --as webhook --webhook-token <token> --text "<内容>"` |
| "编辑 / 修改已发送消息" | `dws chat message edit --conversation-id <openConversationId> --msg-id <openMessageId> --text "<新内容>"` |
| "撤回我发的消息" | `dws chat message recall`（撤回当前用户发送的消息）|
| "撤回机器人消息" | `dws chat message recall-by-bot --robot-code <code> --group <openConversationId> --keys <processQueryKey>`（撤回机器人发的）|
| "把普通群升级为外部群" | `dws chat group upgrade-to-external --group <openConversationId> --dry-run`，确认后加 `--yes` |
| "清除我的群昵称" | `dws chat group update-nick --group <openConversationId>`（省略 `--nick`） |
| "这个会话属于哪些分组" | `dws chat category list-by-conv --group <openConversationId>` |
| "批量查询分组信息" | `dws chat category batch-info --category-ids <id1>,<id2>` |

> **注**：`chat message send` 的 `--title` 可选（不传时用正文首行作标题）；`send-by-bot` / `send-by-webhook` 的 `--title` 必填。

## 评测高频硬约束

- `chat message edit` 必须同时提供会话 ID 与消息 ID；`--text` 和 `--content` 二选一，`--title` 只与 `--text` 搭配。
- `chat group upgrade-to-external` 只适用于 `NORMAL_GROUP`，不可逆且仅群主可执行。先 `--dry-run`，获得用户明确确认后再加 `--yes`。
- `chat group update-nick` 不传 `--nick` 表示清除当前用户的群昵称，不是参数缺失。
- 会话分组 ID 是数值 ID；按会话反查用 `category list-by-conv`，按多个分组 ID 查详情用 `category batch-info`。

## 跨产品协作

- 收件人是人名 → 先用 `dingtalk-contact` 或 `dingtalk-aisearch` 拿 `openDingTalkId` / `userId`
- 要发本地图片/文件 → 直接用 `dws chat message send --msg-type file --file-path <本地路径>`；图片会作为可下载的文件附件发送，不会内联渲染。只有上游已提供有效 mediaId 时才用 `--msg-type image --media-id`，DWS CLI 不能把本地文件转换成 mediaId
- 紧急升级（应用内/短信/电话）→ 切到 `dingtalk-ding`
- 发邮件 → 切到 `dingtalk-mail`
