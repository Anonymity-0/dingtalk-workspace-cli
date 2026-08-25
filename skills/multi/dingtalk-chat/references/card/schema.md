# 卡片 Schema

DWS 公开两类卡片命令契约：

- `chat +messages-send-card` / `chat +messages-update-card`：仍是 `im.streaming-card.v1`
  Shortcut 工作流，只处理 streaming text。
- `chat message send-card` / `chat message update-card`：原子命令支持
  `--card-engine streaming|a2ui`，默认 `streaming`。

streaming 不是任意组件 Schema：

- target：group、direct user、direct openDingTalkId；
- content：streaming text；
- lifecycle：create 可选串联 update，后续按 `bizId` update；
- flowStatus：1–5；
- callback：不支持。

A2UI 原子命令规则：

- `send-card --card-engine a2ui` 调用 `im.create_a2ui_card`。
- `update-card --card-engine a2ui` 调用 `im.update_a2ui_card`。
- `--content` 必须是 JSON 字符串数组，例如 `'["message1","message2"]'`；
  CLI 解析为 `a2uiMessages`，send-card 额外生成 `fallbackText`，值为数组元素按换行拼接。
- 群聊目标写入 `target.openConversationId`；单聊目标写入 `target.receiverUid`，由 MCP
  server 根据现有单聊目标转换或补齐。
- A2UI `flowStatus` 接受 1–9：1 PROCESSING、2 INPUTTING、3 FINISH、
  4 EXECUTING、5 ERROR、6 ABORTED、7 TIMEOUT、8 CONFIRMING、9 CONFIRMED。
- A2UI send-card 在 CLI 侧自动生成 `requestId`、`bizCardId`，并固定
  `protocolVersion="1.0"`；创建默认 `flowStatus=1(PROCESSING)`。

参数、required 和 confirmation 读取
`dws schema --cli-path "chat +messages-send-card" --compact -f json` 或
`dws schema --cli-path "chat message send-card" --compact -f json`。不要把 Lark card JSON 字段翻译成
未发布的 DWS flags；A2UI 内容复用 `--content`，不要发明 `--a2ui-content`。
