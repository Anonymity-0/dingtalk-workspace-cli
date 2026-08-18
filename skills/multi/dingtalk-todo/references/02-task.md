# Todo 组合流程

## 循环待办

公开 Shortcut 暂未暴露 recurrence。先取得真实执行人 `userId`，再使用原子命令：

```bash
dws todo task create --title "每日站会" --executors <USER_ID> --due "2026-08-19T10:00:00+08:00" --recurrence "DTSTART:20260819T020000Z\nRRULE:FREQ=DAILY;INTERVAL=1" --format json
```

`--due` 必填。保存创建结果的 `taskId`，再用 `dws todo +get --task-id <TASK_ID> --format json` 回读；创建响应不明时先列表对账，不重放。

## 批量创建

1. 把姓名唯一解析成 `userId`；同名或零匹配先消歧。
2. 生成最多 30 条的 JSON 数组，字段为 `title`、`executors`，可选 `priority`、`due`、`recurrence`。
3. 执行 `python scripts/todo_batch_create.py todos.json`。
4. 只把 ledger 中 `status=verified` 的条目报告为完成；`unknown` 与 `unverified` 按返回的标题/`taskId` 查询对账。

## 指派并通知

1. `dws todo +assign --to "<姓名>" --task "<标题>" --format json`。
2. 仅在返回稳定 `taskId` 且 `verified=true` 后，搜索目标群取得 `openConversationId`。
3. 把已核验的标题与 `taskId` 发到群；Todo 创建失败或未核验时不发送“已创建”通知。

## 会后行动项建待办

1. 用 `dingtalk-minutes` 锁定同一 `taskUuid` 并读取真实待办事项。
2. 缺少责任人或时间时向用户补齐；不要从空白来源编造。
3. 单人使用 `+assign`，多人同一事项使用 `+assign-multi`，多事项使用批量脚本。
4. 保留“听记行动项 → Todo taskId”的逐项映射，只汇报已验证条目。

