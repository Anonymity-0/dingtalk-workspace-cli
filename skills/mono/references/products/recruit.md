# 钉钉招聘

`dws recruit` 提供招聘职位列表、职位详情和职位创建能力。组织、业务标识和当前
操作人由登录身份及 MCP Connector 注入，不要要求用户提供或猜测身份字段。

## 查询

```bash
dws recruit job list --keyword "Java" --status open --size 20 --format json
dws recruit job get --job-id JOB_ID --format json
```

列表状态支持 `draft`、`open`、`invalid`、`closed`。首页不传 `--cursor`；
`hasMore=true` 时把响应 `nextCursor` 回填。查询详情前必须从真实列表结果取得
`jobId`。

## 创建

创建是非幂等写操作。先用 `--dry-run` 检查；实际执行必须经过用户明确确认，并
交由 CLI 的确认流程处理：

```bash
dws recruit job create --from ./job.json --dry-run --format json
dws recruit job create --from ./job.json --format json
```

JSON 必填字段为 `name`、`description`、`jobNature`、`requiredEdu`、
`minSalary`、`maxSalary`。后三者为数字，且最低薪资不得大于最高薪资。文件只写
职位对象，不要添加 MCP 信封字段 `atsAddJobParam`。
