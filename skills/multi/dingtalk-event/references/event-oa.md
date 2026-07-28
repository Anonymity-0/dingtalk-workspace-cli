# OA 个人审批事件

先读上层 [SKILL.md](../SKILL.md) 的命令规则、调用流和子进程契约。本参考覆盖当前公开的两个 OA 个人事件：审批任务创建和审批实例完成。

实时监听审批事件必须使用 `dws event consume` 长连接，不要轮询 OA 待办或审批实例列表来模拟事件。

## Prerequisite

OA 个人事件使用当前用户 OAuth 登录态。未登录或 token 失效时，先执行：

```bash
dws auth login
```

非默认组织使用全局 `--profile <corpId 或 profile 名>`。事件范围始终是该 OAuth 用户相关的全部 OA 事件，不需要也不接受审批人、发起人或审批模板选择参数。

## Event catalog

| 事件码 | 订阅规则 | 接收语义 | 必填参数 |
|---|---|---|---|
| `user_oa_approval_task_created` | `all` | 审批任务创建，发送给审批人 | 无 |
| `user_oa_approval_instance_finished` | `all` | 审批实例完成，发送给审批单发起人 | 无 |

只承认上表 2 个 OA 事件码。CLI 为每个事件发送 `ruleType=all`、`filterRule={}` 的独立订阅请求；不要添加 `--user`、`--open-dingtalk-id`、`--group`、`--query` 或 `--filter-json`。

## Intent mapping

| 用户说 | 下一步 |
|---|---|
| “监听新的待我审批任务” / “有审批任务创建时通知我” | `dws event consume user_oa_approval_task_created --flatten -f ndjson` |
| “监听我发起的审批何时完成” / “审批实例完成时通知我” | `dws event consume user_oa_approval_instance_finished --flatten -f ndjson` |
| “同时监听审批任务创建和审批完成” | 一个 consume 放入两个 OA event key，不加目标或过滤参数 |
| “查看 OA 事件目录” | `dws event list --category oa` |
| “查看 OA 事件输出字段” | 对对应事件运行 `dws event schema <event_key> --flatten` |

审批任务创建事件只表达“任务已创建并投递给当前审批人”；审批实例完成事件只表达“当前用户发起的审批已完成”。首版不推断审批结果、任务 ID 或实例 ID 的具体 payload 字段。

## Commands

查看稳定的扁平输出 schema：

```bash
dws event schema user_oa_approval_task_created --flatten
dws event schema user_oa_approval_instance_finished --flatten
```

单独监听一种事件：

```bash
dws event consume user_oa_approval_task_created --flatten -f ndjson
dws event consume user_oa_approval_instance_finished --flatten -f ndjson
```

同时监听两种事件：

```bash
dws event consume \
  user_oa_approval_task_created \
  user_oa_approval_instance_finished \
  --flatten \
  -f ndjson
```

双事件 consume 会为两个 event key 分别创建订阅和逻辑 consumer，并共享当前组织的 personal bus、远程连接、stdout 和生命周期。不要给 OA 命令加 `--query` 或 `--filter-json`；这两个 flag 只用于兼容的 IM 消息接收事件。

## Output contract

`--flatten` 模式只承诺以下稳定结构：

```json
{
  "type": "user_oa_approval_task_created",
  "event_id": "...",
  "timestamp": 0,
  "subscribe_id": "...",
  "payload": {}
}
```

- `type` 是当前 event key；`event_id` 可用于去重；`timestamp` 是事件发生时间戳；`subscribe_id` 标识对应的独立订阅。
- `payload` 是开放业务对象，schema 使用 `additionalProperties=true`。未知业务字段原样保留，内部路由字段 `uid/corpid/clientId/filterSubId/bizid/orgId/sourceId` 会被移除。
- 不要假设 `payload` 一定包含 `taskId`、`processInstanceId`、`processCode`、审批结果或完成状态，也不要猜测状态枚举。始终以实际 payload 和 `dws event schema <event_key> --flatten` 为准。
- payload 缺失、为空或无法解析时，consume 会在 stderr 记录 warning，并把原始 transport envelope 写到 stdout，保证事件不被静默丢弃。
- 不传 `--flatten` 时保持兼容 transport envelope，业务 payload 位于 `.data | fromjson`。需要联调完整原始协议时使用不带 `--flatten` 的 `-f raw` 或 `--debug-raw-events`。

## Lifecycle

- 单事件等待 `[event] ready event_key=<key> bus_pid=<pid> subscribe_id=<id>`。
- 双事件先保存两条 `[event] subscription event_key=<key> subscribe_id=<id>`，再等待 `[event] ready event_count=2 bus_pid=<pid>`。
- 临时验证使用 `--max-events 1` 或 `--duration 10m`；任务完成后优雅结束 consume，本次新建的订阅会自动取消。
- 外部停止已有订阅时先运行 `dws event stop <subscribe_id> --dry-run`，确认后再加 `--yes`。不要 `kill -9`，否则会跳过自动退订。
