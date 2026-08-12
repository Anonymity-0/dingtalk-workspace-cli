# 钉钉招聘

`dws recruit` 查询和创建钉钉招聘职位。当前公开命令只覆盖职位列表、职位详情和
职位创建；候选人、面试、Offer、职位修改、开放或关闭尚未作为稳定命令发布。

所有命令都应加 `--format json`。组织、业务标识和当前操作人由登录身份及 MCP
Connector 注入，不要要求用户提供或猜测 `corpId`、`bizCode`、`opUserId`。

## 查询职位列表

```bash
dws recruit job list --format json
dws recruit job list --keyword "Java" --status open --size 20 --format json
dws recruit job list --job-ids JOB_ID_1,JOB_ID_2 --format json
```

可选筛选包括 `--job-ids`、`--required-edu`、`--status`、`--job-nature`、
`--campus`、`--start-modified-time`、`--end-modified-time`、
`--creator-user-ids`、`--keyword`、`--category`。状态接受：

- `draft`：草稿
- `open`：招聘中
- `invalid`：已失效
- `closed`：已关闭/完成

`--size` 默认 20，范围 1–100。首页不传 `--cursor`；响应 `hasMore=true` 时，
把 `nextCursor` 回填到下一次查询。

## 查询职位详情

先从列表结果取得真实 `jobId`，不得编造：

```bash
dws recruit job get --job-id JOB_ID --format json
```

## 创建职位

创建是非幂等远端写入。先准备只包含职位对象的 UTF-8 JSON 文件，并用
`--dry-run` 检查完整调用；向用户展示职位名称、性质、薪资和负责人，取得明确
确认后，交由 CLI 的确认流程执行实际创建。不要在存储示例中加入确认绕过参数。

```bash
dws recruit job create --from ./job.json --dry-run --format json
dws recruit job create --from ./job.json --format json
```

最小 `job.json`：

```json
{
  "name": "Java 开发工程师",
  "description": "负责服务端系统开发",
  "jobNature": "FULL_TIME",
  "requiredEdu": 1,
  "minSalary": 20000,
  "maxSalary": 35000
}
```

六个字段均为必填；`requiredEdu`、`minSalary`、`maxSalary` 必须为数字，且
`minSalary` 不得大于 `maxSalary`。可选字段以当前
`dws recruit job create --help` 和 leaf Schema 为准；不要把 MCP 信封字段
`atsAddJobParam` 写进文件，CLI 会自动包装。
