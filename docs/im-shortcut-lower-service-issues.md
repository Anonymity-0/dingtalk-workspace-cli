# IM Shortcut 下层服务待修问题

> 实测日期：2026-07-28
> 范围：仅记录已用真实会话/真实消息复现，并已排除 Shortcut 与原生 CLI 参数装配差异的 3 项问题。
> 安全：文档不记录真实 `openConversationId`、`openMessageId`、用户标识或凭证。

## 结论

| 优先级 | Shortcut | IM tool | 实测结果 | 当前发布状态 |
|---|---|---|---|---|
| P1 | `chat +conversation-mute-at-all` | `im/update_at_all_notification_off` | `1002` / `business_error` | 隐藏、不可用 |
| P1 | `chat +conversation-mute-red-envelope` | `im/update_red_env_notification_off` | `1002` / `business_error` | 隐藏、不可用 |
| P1 | `chat +messages-combine-forward` | `im/combine_forward_messages` | `COMBINE_FORWARD_ERROR` / `permission` | 隐藏、不可用 |

最新可供下层检索的真实请求：

| 时间（Asia/Shanghai） | IM tool | trace ID | 原始业务消息 |
|---|---|---|---|
| `2026-07-28 19:37:38` | `im/update_at_all_notification_off` | `2166caec17852386586833548e04ed` | `系统繁忙，请稍后再试` |
| `2026-07-28 19:37:39` | `im/update_red_env_notification_off` | `2166caec17852386599041699e04ee` | `系统繁忙，请稍后再试` |
| `2026-07-28 19:41:52` | `im/combine_forward_messages` | `2127d89817852389134833142e07bd` | `[AUTH_PERMISSION_DENIED] Permission denied` |

三项均满足：

1. Shortcut dry-run 已验证实际 tool 和参数键与当前 live MCP Schema 一致；
2. 使用原生 CLI 路径直接调用同一 tool，结果相同；
3. 使用同一真实会话或消息的相邻能力可以成功，因此不是无效测试数据导致的普遍失败。

## 1. `update_at_all_notification_off` 返回 `1002`

### 真实场景

- 当前登录用户可访问的真实群会话；
- 分别执行关闭 `@所有人` 提醒和恢复提醒；
- 两个方向均失败，因此没有产生需要回滚的状态变更。

### 下层请求契约

```json
{
  "openConversationId": "<real-openConversationId>",
  "mute": true
}
```

恢复方向：

```json
{
  "openConversationId": "<same-real-openConversationId>",
  "mute": false
}
```

### 实际错误

```json
{
  "category": "api",
  "message": "系统繁忙，请稍后再试",
  "reason": "business_error",
  "server_key": "im",
  "server_error_code": "1002",
  "trace_id": "2166caec17852386586833548e04ed"
}
```

两次调用（`mute=true`、`mute=false`）均返回相同结果。

### 复现命令

```bash
dws chat +conversation-mute-at-all \
  --conversation-id <real-openConversationId> \
  --yes

dws chat +conversation-mute-at-all \
  --conversation-id <real-openConversationId> \
  --off \
  --yes
```

原生路径：

```bash
dws chat mute-at-all \
  --conversation-id <real-openConversationId> \
  --yes
```

### 已排除

- dry-run 命中 `update_at_all_notification_off`；
- 参数键严格为 `openConversationId`、`mute`，未再发送旧的冗余 `cid`；
- Shortcut 与原生路径使用相同 payload；
- 同一会话的普通免打扰 `im/update_notification_off` 已真实正向、反向成功。

### 请下层协助确认

- `1002` 在该 tool 中的精确业务含义；
- 当前用户身份是否被错误地按机器人身份或无权身份校验；
- `openConversationId` 到内部 `cid` 的转换是否成功；
- 该接口是否还有未体现在 MCP Schema 中的群类型、成员角色或权限前置条件。

## 2. `update_red_env_notification_off` 返回 `1002`

### 真实场景

- 当前登录用户可访问的真实群会话；
- 分别执行关闭红包提醒和恢复提醒；
- 两个方向均失败，因此没有产生需要回滚的状态变更。

### 下层请求契约

```json
{
  "openConversationId": "<real-openConversationId>",
  "mute": true
}
```

恢复方向：

```json
{
  "openConversationId": "<same-real-openConversationId>",
  "mute": false
}
```

### 实际错误

```json
{
  "category": "api",
  "message": "系统繁忙，请稍后再试",
  "reason": "business_error",
  "server_key": "im",
  "server_error_code": "1002",
  "trace_id": "2166caec17852386599041699e04ee"
}
```

两次调用（`mute=true`、`mute=false`）均返回相同结果。

### 复现命令

```bash
dws chat +conversation-mute-red-envelope \
  --conversation-id <real-openConversationId> \
  --yes

dws chat +conversation-mute-red-envelope \
  --conversation-id <real-openConversationId> \
  --off \
  --yes
```

原生路径：

```bash
dws chat mute-red-envelope \
  --conversation-id <real-openConversationId> \
  --yes
```

### 已排除

- dry-run 命中 `update_red_env_notification_off`；
- 参数键严格为 `openConversationId`、`mute`，未再发送旧的冗余 `cid`；
- Shortcut 与原生路径使用相同 payload；
- 同一会话的普通免打扰及其他会话设置写操作可以成功。

### 请下层协助确认

- `1002` 在该 tool 中的精确业务含义；
- 当前用户身份及会话类型是否满足真实接口要求；
- `openConversationId` 是否在 MCP/IM 网关处正确转换；
- 红包提醒能力是否仅对特定客户端、群类型或租户开放，但 Schema 未声明这一限制。

## 3. `combine_forward_messages` 返回权限错误

### 真实场景

- 源会话：当前用户可访问的真实群；
- 源消息：在源会话中创建的两条真实消息，两个 `openMessageId` 均有效；
- 目标会话：另一个当前用户可访问的真实群；
- 单条消息转发 `im/forward_message` 在同类测试数据上成功；
- 合并转发失败。

### 下层请求契约

```json
{
  "srcOpenCid": "<real-source-openConversationId>",
  "srcOpenMessageIds": [
    "<real-openMessageId-1>",
    "<real-openMessageId-2>"
  ],
  "destOpenCid": "<real-destination-openConversationId>",
  "uuid": "<idempotency-key>"
}
```

### 实际错误

```json
{
  "category": "api",
  "message": "[AUTH_PERMISSION_DENIED] Permission denied",
  "reason": "business_error",
  "server_key": "im",
  "server_error_code": "COMBINE_FORWARD_ERROR",
  "message_class": "permission",
  "trace_id": "2127d89817852389134833142e07bd"
}
```

### 复现命令

```bash
dws chat +messages-combine-forward \
  --src-conversation-id <real-source-openConversationId> \
  --msg-ids <real-openMessageId-1>,<real-openMessageId-2> \
  --dest-conversation-id <real-destination-openConversationId> \
  --uuid <new-idempotency-key> \
  --yes
```

原生路径：

```bash
dws chat message combine-forward \
  --src-conversation-id <real-source-openConversationId> \
  --msg-ids <real-openMessageId-1>,<real-openMessageId-2> \
  --dest-conversation-id <real-destination-openConversationId> \
  --uuid <new-idempotency-key>
```

### 已排除

- 不是“只有一条消息”的非法合并场景；
- 不是源、目标会话相同导致；
- 源会话、目标会话和两条源消息都是真实资源；
- dry-run 命中 `combine_forward_messages`，参数键和数组类型与 MCP Schema 一致；
- Shortcut 与原生路径使用相同 payload；
- 同类资源上的单条消息转发已成功。

### 请下层协助确认

- permission 判定具体缺少哪一种权限：源消息读取、源群成员、目标群发言、消息作者，还是应用/用户 scope；
- tool 是否错误使用应用身份执行，而该能力要求用户身份；
- 是否存在未在 MCP Schema 中声明的消息类型、消息作者、群类型或跨群限制；
- 是否能返回可操作的细分错误码，而不是统一的 `COMBINE_FORWARD_ERROR`。

## 修复验收

下层修复后，DWS 侧会用相同真实场景复测：

1. 两个提醒开关都必须通过 `true → false` 双向测试；
2. 合并转发必须用至少两条真实消息和不同目标群验证；
3. Shortcut 与原生命令都通过；
4. 通过后再把对应 Shortcut 从 `unavailable` 恢复公开。
