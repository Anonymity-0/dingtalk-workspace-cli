---
name: dingtalk-todo
description: 钉钉待办 / TODO。Use when 用户说 创建待办/TODO/任务提醒/指派任务/标记完成/查待办/紧急待办/循环待办/批量建待办/逾期待办。不做日报周报（走 dingtalk-misc）、审批（走 dingtalk-misc）、日程（走 dingtalk-calendar）。命令前缀：dws todo。
metadata:
  cli_version: ">=0.2.14"
  category: product
  requires:
    bins:
      - dws
---

# 钉钉待办 Skill

## 执行前提

执行任何 `dws` 操作前，先完整读取 [`dingtalk-shared`](../dingtalk-shared/SKILL.md)。已命中下表时直接走 Golden Route；只有需要低频原子能力或参数细节时，才读取 [todo.md](references/todo.md)。一次最多加载一个操作 Reference。

<!-- VISIBLE_SHORTCUTS_START -->
## Shortcuts（无专用脚本/recipe 时优先）

以下 shortcut 同时进入公开 catalog 与 Runtime Schema。先按本 skill 的意图表、脚本和 recipe 路由：存在精确覆盖该场景的专用脚本/recipe 时按其执行；否则用户意图命中时，shortcut 优先于手写原子命令。命令已选中时直接执行；只在参数或安全语义不确定时读取 Agent leaf Schema（例如 `dws schema --cli-path "todo +<shortcut>" --compact --format json`），在当前 Cobra flags 不确定时读取 `dws todo <shortcut> --help`。只有参数映射、接口绑定或 provenance 审计才省略 `--compact`。仅当现有路由和 reference 都无法定位低频能力时，才用 `dws shortcut list --service todo --format json` 批量发现。

| Shortcut | 风险 | 适用场景 |
|---|---|---|
| `dws todo +assign` | write | 按姓名给某人创建并指派一条待办（自动解析 userId） |
| `dws todo +assign-multi` | write | 把一条待办按姓名一次性指派给多个人（自动把每个姓名解析成 userId） |
| `dws todo +comment` | write | 添加待办评论并读回验证 |
| `dws todo +create` | write | 创建待办并读回验证 |
| `dws todo +created-todos` | read | 列出我创建的待办（我作为创建人 creator 发起的待办，而非分配给我执行的） |
| `dws todo +due-today` | read | 列出我今天到期的待办 |
| `dws todo +get` | read | 查询待办详情 |
| `dws todo +get-my-tasks` | read | 查询当前组织下我的待办列表 |
| `dws todo +get-related-tasks` | read | 一次性列出与我相关的全部待办（我作为创建人/执行人/参与人三种角色的并集，按 taskId 去重） |
| `dws todo +list-attachment` | read | 查询待办任务的附件列表 |
| `dws todo +list-comment` | read | 查询待办评论列表 |
| `dws todo +list-sub` | read | 查询子待办列表 |
| `dws todo +overdue` | read | 列出我已过期未完成的待办 |
| `dws todo +remind` | write | 给自己创建一条带可选截止时间的待办 |
| `dws todo +reminder` | write | 设置或清除待办提醒（仅终端回执） |
| `dws todo +search` | read | 搜索与我相关的全部待办 |
| `dws todo +todo-done` | write | 按标题关键词把我的某条待办标记完成（自动定位 taskId） |
| `dws todo +update` | write | 更新待办并读回验证 |
<!-- VISIBLE_SHORTCUTS_END -->

## Golden Routes

| 用户意图 | 首选入口 | 关键结果 / 边界 |
|---|---|---|
| “给我自己记一条待办” | `dws todo +remind --task "<标题>" [--at "<截止ISO>"] --format json` | 自动解析当前用户；`--at` 是截止时间，不是提醒时间 |
| “给张三建待办” | `dws todo +assign --to "张三" --task "<标题>" --format json` | 姓名必须唯一解析后才创建 |
| “给张三、李四建同一条待办” | `dws todo +assign-multi --to "张三,李四" --task "<标题>" --format json` | 任一姓名不唯一则零写入 |
| 已有 `userId`，创建普通待办 | `dws todo +create --title "<标题>" --executors <USER_ID> [--due "<截止ISO>"] [--priority 10\|20\|30\|40] --format json` | 返回稳定 `taskId`，并读回核验标题 |
| 今天到期 / 已逾期 | `dws todo +due-today --format json` / `dws todo +overdue --format json` | 均有界拉全分页；空集合也是成功结果 |
| 当前组织下我的执行待办 | `dws todo +get-my-tasks --all --status false --format json` | `--all` 达到 40 页仍未耗尽会失败，不伪装完整 |
| 与我相关的全部待办 | `dws todo +get-related-tasks --format json` | 创建人、执行人、参与人三种角色并集，按 `taskId` 去重 |
| 按标题关键词查询 | `dws todo +search --query "<关键词>" --format json` | 搜索与 list 不混用；跨全部分页匹配 |
| 已知 `taskId` 查详情 | `dws todo +get --task-id <TASK_ID> --format json` | 详情必须回传同一个稳定 `taskId` |
| 已知 `taskId` 完成 / 重开 | `dws todo +complete --task-id <TASK_ID> --format json` / `dws todo +reopen ...` | 先读当前状态，避免重复写，再读回核验 |
| 只记得标题，标记完成 | `dws todo +todo-done --task "<关键词>" --format json` | 仅唯一命中时写；零个或多个候选均停止 |
| 修改标题、截止时间或优先级 | `dws todo +update --task-id <TASK_ID> ... --format json` | 至少指定一个待改字段；写后逐字段核验 |
| 设置独立提醒 | `dws todo +reminder --task-id <TASK_ID> --base-time customTime --at "<提醒ISO>" --format json` | 上游无提醒查询接口，只能返回终端写回执，`verified=false` |
| 基于截止时间提前提醒 | `dws todo +reminder --task-id <TASK_ID> --base-time dueTime --due-date-offset -30 --format json` | 待办必须已有截止时间；偏移单位为分钟 |
| 清除全部提醒 | `dws todo +reminder --task-id <TASK_ID> --clear --format json` | 清除写操作；不能与提醒参数混用 |
| 批量创建 | `python scripts/todo_batch_create.py <todos.json>` | 单批最多 30 条；保留逐项 `taskId`、verified/unverified/unknown 状态，不盲重试 |
| 今天/明天/本周汇总 | `python scripts/todo_daily_summary.py today\|tomorrow\|week` | 走 `+get-my-tasks --all`，只纳入范围内且有截止时间的未完成待办 |

## 低频原子能力

以下场景没有等价的公开 Shortcut，确认目标 ID 后再读取 [todo.md](references/todo.md) 对应小节：

- 循环待办、子待办创建；
- 删除待办、评论、附件或标签；
- 增删执行人、参与人；
- 上传附件；
- 标签创建、更新、绑定与查询。

删除类操作必须由用户确认；不要在存储示例里写 `--yes`。附件上传会真实传输本地文件，不能用来试探权限。

## 关键约束

- 标题、URL、展示序号都不是 `taskId`。已知 ID 直接行动；未知 ID 用列表/搜索定位，零匹配或多匹配时停止。
- 待办公开命令统一使用 `--task-id`；`--id` / `--ids` 只是隐藏兼容别名，不要写入新命令或示例。
- 优先级：低=10、普通=20、较高/高/重要=30、紧急/最高/P0=40。
- `--due` / `+remind --at` 表示 deadline；独立 reminder 必须走 `+reminder`。
- 创建和评论是非幂等写。超时、缺少稳定 ID 或读回失败时保留“可能已提交/未核验”状态，先查询对账，禁止盲目重放。
- 所有命令加 `--format json`；写 Shortcut 按 Runtime 安全契约确认，确认前不得自行附加 `--yes`。
- 会后行动项来自听记时先走 `dingtalk-minutes`；OA 审批走 `dingtalk-misc`；时间块和会议走 `dingtalk-calendar`。

## 按需参考

- [局部意图消歧](references/intent-guide.md)
- [轻量流程](references/lite-recipes.md)
- [组合流程](references/02-task.md)
- [完整命令参考](references/todo.md)
