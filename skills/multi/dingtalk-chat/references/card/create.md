# 创建流式卡片

用户明确只创建、不更新或不填正文时，使用原生 `dws chat message send-card`，只执行
`create_and_send_card`；群聊创建可通过 `--at-open-dingtalk-ids` 或 `--at-all` 设置
艾特对象。用户同时提供正文或明确要求创建后写入/完成时，才使用
`dws chat +messages-send-card`。Shortcut 的群目标传 `--group`；单聊 userId 传
`--receiver`，Runtime 会唯一解析为 openDingTalkId；已有 openDingTalkId 时传
`--receiver-open-dingtalk-id`。三种目标严格三选一。

- 只传目标：优先用原生 `message send-card` 创建卡片并从真实结果取得 `bizId`，供后续更新。
- 同时传 `--content`：Runtime 串行执行 create → 从返回提取 `bizId` → update；默认
  `--flow-status 3`。
- 群聊可传 `--at-open-dingtalk-ids` 或 `--at-all`；艾特对象只进入初始
  `create_and_send_card`。同一次调用带 `--content` 时，Runtime 将 create 返回的
  `atTag` 自动加在正文前，再调用 `update_streaming_card`；调用方不要拼 ID
  或艾特占位符。
- 原生 `message send-card --dry-run` 只输出单步创建计划；Shortcut 的 `--dry-run` 仍执行只读 userId 解析，并按是否提供 `--content` 输出一步或两步计划，均不执行写入。
- 用户没有提供正文时不得自行编造 `--content`，也不得擅自增加 update。

创建成功但自动更新失败时，错误会保留真实 `bizId`。不要重复创建；使用该 `bizId` 继续
`+messages-update-card` 或人工处理。

当前内容仅为 streaming text，不接受 Lark Card JSON、组件树或按钮 callback。

```bash
dws chat +messages-send-card --group <openConversationId> --at-open-dingtalk-ids <mentionedOpenDingTalkId> --content "请确认"
dws chat +messages-send-card --group <openConversationId> --at-all --content "请大家确认"
```
