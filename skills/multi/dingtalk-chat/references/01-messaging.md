# 消息任务级流程

只在单个 Golden Route 不能完成任务、需要跨步骤传递真实结果时读取本文件。简单姓名/群名文本发送、单会话读取和跨会话搜索直接按根 Skill 执行。

## 选择路线

1. 先选择任务语义最窄的 Shortcut。
2. 只有 Shortcut 暂不接受自然目标或目标类型时，才用一个只读 leaf/Shortcut 解析 ID。
3. 解析全部完成并消歧后再写入；不要边解析边产生部分副作用。
4. 后续步骤只使用真实返回字段，不从名称、URL 或上下文猜 ID。

## 群聊消息

已知群 ID 时直接读取：

```bash
dws chat +chat-messages --group <openConversationId> --format json
```

只有群名时，读取历史直接用 `+chat-messages --chat-query <群名>`，普通文本发送直接用 `+send-to-group`。其它尚不接受群名的高级动作才先用 `+chat-search --query <群名>`；只有唯一候选才把 `openConversationId` 传给下一步。查询结果需要资源时在读取命令上加 `--download-resources`，不要让 Agent 手工遍历资源引用。按姓名读取单聊同理使用 `+chat-messages --user-query <姓名>`。

## 发送消息

- 姓名 + 简单文本：`+dm`。
- 群名 + 简单文本：`+send-to-group`。
- 已知 ID、文件、Bot、Webhook、复杂 @ 或幂等：`+messages-send`。
- 姓名 + 文件/高级控制：`+messages-send --as user --user-query <姓名> --file <相对路径>`。
- 群名 + 文件/高级控制：`+messages-send --as user --chat-query <群名> --file <相对路径>`。

`--user-query` 和 `--chat-query` 会在 CLI 内运行真实只读解析；零命中或多候选时在上传或发送前停止。Bot/Webhook 不接受这两个自然目标参数。

文件直接交给 `+messages-send --file`。不要恢复“独立上传 → 提取 mediaId → 发送”的旧默认链路。

## 创建群聊

`+chat-create` 同时接受 `--users` 稳定 ID 和 `--member-query` 姓名/花名。自然成员解析、候选消歧、稳定 ID 去重和创建前预检都由 CLI 完成：

```text
传入全部姓名
→ 对零命中和多候选统一消歧
→ 按稳定 ID 去重
→ 全部成功后执行一次 +chat-create
```

任一成员未唯一解析时不会读取当前用户或创建群；`--dry-run` 也走同一解析链。不要用群名预搜索伪装幂等，因为业务上允许同名群。

## 机器人消息

已知 `robotCode` 时使用 `+messages-send --as bot`。未知机器人、机器人入群、批量群发或撤回读取 [chat-bot.md](chat/chat-bot.md)。Bot 不继承 user 的文件能力；只使用 leaf Schema 明确发布的文本/Markdown 能力。

## 引用与转发

- 引用回复：`+messages-reply`；优先继续使用结果中的 `messageId`、`conversationId`、`deliveryStatus` 和 `referencedMessage`，未知投递状态不得写成成功送达。
- 单条转发：`+messages-forward`。
- 合并转发：`+messages-combine-forward`。
- 话题转发：`+messages-forward-topic`。

先用 `+chat-messages`、`+search-msg` 或 `+messages-mget` 取得真实 `messageId` 和 conversation/thread 上下文。引用或合并消息中的子消息优先使用自己的 `messageId`，不要拿父消息 ID 代替。

## 上下文传递表

| 上一步 | 真实返回 | 下一步用途 |
|---|---|---|
| `+chat-search` | `openConversationId` | 高级发送、读取、群管理 |
| `dingtalk-contact` 唯一用户解析 | `userId` / `openDingTalkId` | 单聊、建群、@、按发送者搜索 |
| `+messages-send` | `openTaskId` / 投递结果 | 查询投递状态；不是回复/撤回消息 ID |
| `+chat-messages` / `+search-msg` / `+messages-mget` | `messageId`、conversation/thread、`resourceRefs` | 回复、转发、撤回、资源下载 |
| `+chat-create` | `openConversationId` | 新群后续消息与群管理 |
| 分页查询 | `hasMore` / `nextCursor` / `complete` | 继续翻页和完整性判断 |

## 完成判断

- 写操作检查任务级结果或可查询状态，不只看退出码。
- 读取检查 `complete`、`hasMore` 和 `failures`。
- 下载检查每项 ledger；单项失败不抹掉已取得消息。
- 投递状态未知时报告 unknown 并保留幂等键，不自动换目标重发。
