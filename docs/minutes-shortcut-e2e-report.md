# DWS Minutes Shortcut E2E 报告

日期：2026-08-10
分支：`codex/minutes-shortcuts`
原则：真实个人登录态、真实 Minutes 服务、真实媒体传输；报告不记录 taskUuid、UID、签名 URL、上传凭证或原文内容。

## 结论

- 27 个公开 Minutes shortcut 已进入同一 public catalog 和最终 Schema。
- 读取类命令只有在响应结构明确、分页完整时才成功；合法的业务空集合与“响应缺字段/解析失败”已分开处理。
- 写入类命令要求明确 acknowledgement，并在能读回的能力上执行最终状态验证；部分写入返回非零及 ledger，不再伪装成普通成功。
- 所有声明 dry-run 的 Minutes 示例都经过全仓执行门禁，证明零 ToolCaller 调用、零 stdin 读取、零确认交互，并输出可审计的 `dry_run=true`、`preview_kind=plan`。
- 两项服务端/数据前提无法制造成功：当前账号是所测听记的创建者，权限申请接口明确拒绝“创建者申请”；speaker insights 任务可创建但现有听记在轮询窗口内不产出结果。两者均正确非零退出并保留诊断/恢复信息。

## 真实数据矩阵

| 能力 | 真实 E2E 结果 | 成功判据 |
|---|---|---|
| `+list-mine` / `+list-shared` / `+list-all` | 通过 | 三个 scope 均返回显式 `itemList`；结构异常不按空列表处理 |
| `+search` | 通过 | 追完 8 页、扫描 148 条，按真实标题精确命中 1 条，`complete=true` |
| `+latest` | 通过 | 从带可比较时间的真实候选中选择最新项，并读取非空 basic info |
| `+detail` | 通过 | 批量 2 条；其中成熟听记逐字稿 8 页、389 段，文件安全落盘且 inline 按配置移除 |
| `+transcript` | 通过 | 8 页、389 段；消费 `nextToken` 后才报告完整，不把第一页当全集 |
| `+action-items` | 通过 | 指定真实听记读取 todos；只有显式 todos 数组允许合法空结果 |
| `+download` | 通过 | 下载 36,223,533 字节；落盘大小与传输结果一致，签名 URL 未进入结果 |
| `+upload` | 通过 | create → HTTPS PUT → complete 重试 → basic read-back；293,978 字节一致 |
| `+update` | 通过 | 标题写入后读回精确相等；随后恢复原标题并再次验证 |
| `+apply-permission` | 服务端前提受限 | 真实接口返回“申请人是创建者，无需申请”；命令非零退出，没有合成成功 |
| `+summary` | 通过 | 写入唯一标记、命令内读回、外部独立读回；最后按原 SHA-256 恢复并验证 |
| `+speaker-replace` | 通过 | 完整逐字稿预检源昵称，替换后重新分页读回；随后反向恢复 |
| `+replace-batch` | 通过 | 隔离上传听记，多规则写入逐项 acknowledgement + 最终逐字稿读回；随后恢复 |
| `+record-start` / `+record-pause` / `+record-resume` | 通过 | 每步真实调用返回显式 command/taskUuid acknowledgement |
| `+record-stop` / `+record-wrap-up` | 通过 | stop acknowledgement 明确；wrap-up 的 basic 产物完成，未就绪产物不会以空值成功 |
| `+upload-and-analyze` | 通过恢复链路 | 首轮上传成功但 transcript 仍为空，19 次轮询后正确 partial/non-zero；用 `--resume-id` 不重复上传，最终 1 段且完成 |
| `+mindmap` | 通过 | 对真实成熟听记 resume 查询到 `taskStatus=1`、`complete=true` |
| `+speaker-insights` | 后端产物受限 | create 返回真实 taskId；当前数据 60 秒内无结果，命令 non-zero 并返回 taskId/taskUuid 恢复句柄 |
| `+prepare-asr` | 通过 | 添加唯一热词并读回；原子清理后再次读回确认移除 |
| `+export-pack` | 通过 | 生成 6 个非空文件（manifest + 5 类产物），manifest 不含凭证/签名 URL |
| `+share` / `+unshare` | 通过 | 在隔离听记上对当前 UID 实际授予再移除，逐成员 ledger `complete=true` |

## 提交前真实服务复验（2026-08-11）

在最终代码与最终二进制上又执行了一轮真实账号 smoke；以下均为真实 MCP、真实 HTTPS 媒体传输和真实写入，不是 mock：

- 生成 4 秒隔离 WAV 并执行本地直传，服务端 create → PUT → complete → basic read-back 通过；随后真实下载 65,320 字节，字节数与上传一致。
- 对隔离听记真实更新标题并读回验证；新上传对象尚无 `fullSummary` 时，`+summary` 明确非零退出，没有把缺字段降级成空摘要或继续覆盖。
- 对隔离听记真实执行 `+share` 再 `+unshare`，两个逐成员 ledger 均完成；创建者执行 `+apply-permission` 被服务端拒绝并保持非零。
- `+export-pack --artifacts basic` 在临时工作目录原子发布；`+upload-and-analyze --resume-id ... --artifacts basic` 证明恢复路径没有重复上传且两个阶段均完成。首次请求遇到网关响应头超时并非零退出，提高单次网络超时后重试成功。
- 真实执行 `+record-start → +record-pause → +record-resume → +record-wrap-up`；pause/resume/stop acknowledgement 与 basic 产物均完成。
- 真实热词集合先增加唯一测试词并读回，再通过 `--sync` 恢复原有 5 项；最终计数一致且测试词不存在。
- 为防止业务数据遗留，额外扫描 195 个可访问候选的摘要，没有发现测试 marker；报告与提交仍不记录任何 taskUuid、成员 UID、组织信息、签名 URL、凭证或业务正文。

## PR 反馈修复后复验（2026-08-11）

- 从真实最新听记内部取得标题但不输出标题或标识，分别执行公开 `+search` 与隐藏兼容 `+minutes-search`；两条路径都真实命中 1 条且 taskUuid 非空，证明历史 Schema/argv 兼容入口不是空壳。
- 重新生成 1 秒隔离 WAV，真实执行 create → HTTPS PUT → complete → basic read-back；32,078 字节一致，`complete=true`、`verified=true`，sessionId/taskUuid 均非空但未记录具体值。
- 首次读取遇到网关响应头超时并明确非零；提高单次超时后才取得上述非空业务结果，未把网络错误或空输出记成通过。
- “服务端已 complete、客户端丢失响应”无法在生产服务上安全且确定地制造，因此增加确定性端到端 fixture：complete 调用在服务端状态标记成功后向客户端返回错误，断言命令返回 `minutes_upload_completion_unknown`、保留 sessionId、`remoteEffect=unknown`，并且 `cancel_upload_session` 调用次数严格为 0。

## E2E 过程中实际发现并修复的问题

1. Minutes 列表真实结构是 `result.itemList`；旧 smart 链路会错误返回“暂无妙记”。现统一由 `minutesdata` 严格解析。
2. `+detail` 和 `+transcript` 旧实现只读第一页。现默认追完 cursor，检测 token 停滞/成环并输出完整性证据。
3. `+replace-batch` 旧实现允许部分写仍返回成功。现默认首错停止，continue 也会在任一失败时返回非零，并增加写前/写后逐字稿验证。
4. 上传后的分析接口曾返回显式空 transcript；旧 workflow 会将其视为 ready。现 summary/transcript 必须满足非空产物合同，超时返回可恢复的 partial result。
5. 新增 dry-run 初版只输出 `dryRun` 且部分命令会先远端读取。示例执行门禁发现后，已统一为可审计 envelope，并保证预览路径零远端调用。

## 不能与 lark-cli 假装对齐的边界

| 能力 | 原因 | DWS 交付决定 |
|---|---|---|
| Minutes Todo 写入 | DWS 只有听记内待办读取，没有 add/update/delete API 与稳定 todo ID | 公开只读 `+action-items`，不注册假 `+todo` |
| Chapter | 没有章节原子 API | 不从摘要推导后冒充平台章节 |
| speaker identity rebinding | DWS 是昵称/可选 UID 替换，不是 speaker_id → open_id 绑定模型 | `+speaker-replace` 明示昵称替换语义 |
| transcript-only word replacement | DWS 原子替换同时影响逐字稿和摘要 | 保留更准确名称 `+replace-batch` 并公开副作用范围 |
| owner/participant 精确搜索 | 服务端 list 只提供归属、关键词和时间 | `+search` 不声明不存在的服务端过滤 |
| Meeting Bot / lifecycle events / artifact index | `conference` 已下线，当前 event 与 Minutes 无稳定 meetingId↔taskUuid 关联 | 不创建空壳 shortcut，等待平台 API |

## 自动化门禁

交付前执行：

```text
make generate-schema
./scripts/policy/check-generated-drift.sh
./scripts/policy/check-schema-catalog.sh
./scripts/policy/check-runtime-confirmation-truth.sh
make test-schema-agent-examples
DWS_PACKAGE_VERSION=0.0.0-test go test ./...
make build
```

真实数据 smoke 不进入普通 CI：它依赖个人登录态并会创建/修改业务数据。所有写入测试使用隔离对象或可验证恢复步骤；报告只保留匿名计数和状态。
