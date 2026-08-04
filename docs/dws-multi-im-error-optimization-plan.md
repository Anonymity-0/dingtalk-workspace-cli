# DWS Multi IM 错误专项优化方案

> 日期：2026-08-03  
> 分支：`codex/multi-im-optimization`  
> 当前提交：`30314311`  
> 评测基线：`2a8c6c87`，155 条 accepted 轨迹、74 次失败调用

## 1. 结论

74 次失败已经能够完整归因，不应继续使用“API 业务错误”或“其他命令失败”这类粗分类：

| 根因 | 次数 | 判断 |
|---|---:|---|
| 群名解析歧义 | 37 | Runtime 正确 fail-closed；评测夹具和 Agent 消歧流程问题 |
| 稳定 ID 进入群名参数 | 17 | Shortcut 目标参数合同不一致 |
| 群名进入稳定 ID 参数并打到后端 | 7 | Shortcut 目标参数合同不一致，且缺少调用前校验 |
| 不存在的命令或 flag | 11 | 选路/Help 漂移及缺少无歧义兼容别名 |
| `auth whoami` 不存在 | 1 | MCP 错误误导后的错误恢复分支 |
| MCP 内部依赖不可用 | 1 | 后端 `queryToolMeta` 内部连接拒绝；CLI 错分为业务错误 |
| **合计** | **74** | |

其中真正带 `NETWORK_ERROR / UNAVAILABLE / Connection refused` 证据的只有 1 次。它不是用户侧公网故障，也不是 `+conversation-list` 参数拼装错误，而是 MCP 后端内部依赖不可用。

## 2. 本地复测

### 2.1 真实 MCP

使用当前提交重新构建 CLI，执行：

```bash
dws chat +conversation-list --page-all --format json --timeout 60
```

6 轮已验证结果全部为：

```text
exit=0
complete=true
count=100
failedCount=0
```

此外，评测提交 `2a8c6c87` 重新构建后同一命令也成功；该提交到当前提交之间，`+conversation-list` 的 MCP 工具名、参数和分页代码没有变化。

### 2.2 原始失败确定性回放

本地 loopback MCP 返回评测中的等价内容：

```json
{
  "success": false,
  "server_error_code": "NETWORK_ERROR",
  "technical_detail": "McpService.queryToolMeta ... StatusCode.UNAVAILABLE ... Connection refused",
  "retryable": true
}
```

当前 CLI 稳定输出：

```text
category=api
reason=business_error
message=business error: success=false
hint=Check required parameters and values
server_error_code=NETWORK_ERROR
retryable=true
```

这证明需要修的是中央错误分类：诊断字段已经保留，但没有参与 `reason/message/hint` 决策。

### 2.3 其他代表错误复测

当前 Runtime 可稳定复现：

1. `+chat-members-list --group <cid>` 在调用后端前失败，并提示“请使用 --group 或 --chat”，但当前命令的稳定 ID 参数实际是 `--conversation-id`；提示与命令合同矛盾。
2. `+chat-messages --group <群名>` 把群名直接交给只接受 CID 的下层，后端返回 `openCid or cid is required`；应在 CLI 内先解析，不能打到后端。
3. `+search-msg --text ...` 返回 `unknown_flag`；当前正式参数是 `--query`。

## 3. P0：修中央错误分类

### 3.1 分类顺序

修改 `internal/app/runner.go`，在通用 `business_error` 之前执行结构化分类：

```text
ExtractServerDiagnostics
→ ClassifyServerFailure
→ 构造 typed Error
→ 无已知类别才回退 business_error
```

建议新增独立分类器，例如 `internal/app/server_failure_classifier.go`。输入为 MCP 内容和 `ServerDiagnostics`，输出：

```go
type ServerFailureClass struct {
    Reason           string
    Origin           string
    Stage            string
    ExecutionStarted *bool
    Hint             string
    Actions          []string
}
```

首批规则：

| 条件 | reason | origin | stage |
|---|---|---|---|
| `NETWORK_ERROR` 且 detail 含 `queryToolMeta`、`UNAVAILABLE` | `backend_dependency_unavailable` | `mcp_gateway` | `tool_metadata_lookup` |
| `PARAM_ERROR`、缺 required 参数 | `invalid_request` | `dingtalk_api` 或 `mcp_gateway` | `tool_validation` |
| 权限/Scope | `permission_denied` | 实际返回方 | `authorization` |
| 资源不存在 | `resource_not_found` | `dingtalk_api` | `tool_execution` |
| 无结构化证据 | `business_error` | `unknown` | `unknown` |

兼容期可以用 `technical_detail` 补充 stage，但 `server_error_code` 必须是主分类依据。原始 `trace_id`、`technical_detail` 和 `retryable` 原样保留。

### 3.2 重试边界

当前响应没有稳定的 `execution_started` 字段，因此第一阶段只修分类和恢复提示，不对所有工具自动重试。

- 后端应优先在 `queryToolMeta` 内部做短重试，因为它明确发生在业务执行前；
- CLI 只有在后端提供 `execution_started=false` 后，才可对读写命令统一安全重试；
- 过渡期最多只对最终 Schema 标记为只读的命令重试一次；
- 写命令执行状态未知时禁止自动重试。

## 4. P0：统一群目标参数合同

24 次失败来自同一个设计问题：不同 Shortcut 对 `--group` 的含义不同。

统一公开合同：

| 参数 | 统一语义 |
|---|---|
| `--group` | 接受群名或 `openConversationId`；CLI 自动识别 |
| `--conversation-id` | 显式稳定 ID |
| `--chat-query` | 显式自然群名 |
| `--chat` / `--open-conversation-id` | 只作为无歧义兼容别名，不作为文档默认入口 |

新增共享入口，例如：

```go
ResolveChatTarget(rt, directValue, queryValue)
```

规则：

1. directValue 形似 CID：直接返回，不进行群名搜索；
2. directValue 不是 CID：按自然群名走完整分页唯一解析；
3. queryValue 形似 CID：规范化为稳定 ID，而不是送入群名搜索；
4. 零命中或多命中继续 fail-closed；
5. 任何自然群名都不得直接发送到只接受 CID 的后端工具。

首批应用：

- `internal/shortcut/smart/group_members.go`
- `internal/shortcut/smart/chat_messages.go`
- `internal/shortcut/smart/search_msg.go`
- 复核 `send_to_group.go`、`at_me.go` 和 `chat/chat_group.go`，消除同类局部实现。

同时修改 `ResolveChat` 的类型错误：不要输出与调用命令不一致的具体 flag，改为 typed `target_type_mismatch`，并提示读取当前 leaf Help。

## 5. P0：处理 11 次命令/flag 漂移

只增加语义唯一、不会制造第二入口的兼容项：

| 观察到的错误 | 决策 |
|---|---|
| `+search-msg --text`（3 次） | 增加 `--text` → `--query` 隐藏兼容别名 |
| `+search-msg --text-query`（1 次） | 增加 `--text-query` → `--query` 隐藏兼容别名 |
| `+chat-messages --page-size`（1 次） | 增加 `--page-size` → `--limit` 隐藏兼容别名 |
| `+chat-messages --open-conversation-id`（1 次） | 增加稳定 ID 兼容别名 |
| `+chat-members-list --chat`（2 次） | 增加稳定 ID 兼容别名 |
| `+chat-members-list --open-conversation-id`（2 次） | 增加稳定 ID 兼容别名 |
| `+chat-rename`（1 次） | 作为 `+chat-update` 的隐藏兼容别名，`--group` 复用群名/CID 唯一解析；Skill 默认仍只推荐 reviewed `chat group rename --id --name` |

所有新增 flag 必须进入真实 Cobra、Shortcut constraints、Schema 参数投影和示例校验；不能只改 Skill。

`auth whoami` 不建议补一个语义不清的新命令。MCP 内部依赖错误修正后，恢复动作应直接提示“稍后重试并保留 trace ID”，不得诱导 Agent 检查认证。认证确需检查时只推荐现有 `dws auth status` 和 `dws profile list`。

## 6. P1：同名群与评测口径

37 次 `resolution_ambiguous` 是安全机制正常工作，不能通过默认第一个候选来消除。

需要同时处理两层：

1. 真实产品：继续返回结构化 candidates，Agent 展示候选并停止写入；可在下层真实提供时增加群主、群类型等可理解的区分字段。
2. 固定评测：所有要求命中 `dws测试群01/02` 的 case 都必须在 accepted run 前保证目标名称唯一，而不是只处理 H15/H18/H20；夹具状态和恢复记录继续留在 Prompt 外。

评测报告应把错误维度分开：

```text
agent_error
request_contract_error
expected_resolution_stop
business_error
infrastructure_error
recovered_infrastructure_error
```

若环境真实存在同名群，`resolution_ambiguous` 应计入安全停机事实，不应伪装成业务成功；若 case 设计要求唯一群，则应判夹具失败并重跑，而不是污染 Agent clean。

## 7. 测试与验收门禁

### 7.1 错误分类

- `success=false + NETWORK_ERROR + queryToolMeta/UNAVAILABLE` → `backend_dependency_unavailable`；
- `success=false + PARAM_ERROR` → `invalid_request`；
- 无诊断字段的 `success=false` 保持 `business_error`；
- JSON 输出中的 `reason/hint/retryable/server_error_code/trace_id` 与 typed error 一致；
- 不再对内部依赖错误提示“检查参数或认证”。

### 7.2 目标合同

对 `+chat-members-list`、`+chat-messages`、`+search-msg` 分别覆盖：

- `--group <cid>` 不搜索，且下层收到同一 CID；
- `--group <name>` 唯一解析后下层只收到 CID；
- `--conversation-id <cid>` 不搜索；
- `--chat-query <name>` 唯一解析；
- `--chat-query <cid>` 不再触发“ID 被当群名”；
- 多候选时下层业务调用为 0；
- 群名绝不进入 `openCid/cid/openConversationId` 参数。

### 7.3 兼容参数

- 每个兼容 alias 与 canonical flag 产生完全相同的 invocation；
- alias 与 canonical 同时出现时按 constraints 拒绝，禁止静默覆盖；
- Help 只突出 canonical flag，Schema 保留 Agent 可执行事实；
- Skill 和 reference 不把兼容 alias 写成新的默认路线。

### 7.4 最终门禁

```bash
gofmt -w <modified-go-files>
DWS_PACKAGE_VERSION=0.0.0-test go test ./internal/errors ./internal/transport ./internal/app ./internal/shortcut/targetresolver ./internal/shortcut/chat ./internal/shortcut/smart
go generate ./internal/cli
./scripts/policy/check-generated-drift.sh
./scripts/policy/check-schema-catalog.sh
./scripts/policy/check-skill-commands.sh
DWS_PACKAGE_VERSION=0.0.0-test go test ./...
```

真实环境至少再跑：

```bash
dws chat +conversation-list --page-all --format json
dws chat +chat-members-list --group <真实群名或CID> --format json
dws chat +chat-messages --group <真实群名或CID> --limit 5 --format json
dws chat +search-msg --chat-query <真实群名> --query <关键词> --format json
```

验收不是简单要求 74 变 0，而是：

- 1 次 MCP 内部故障被正确分类并进入独立可靠性指标；
- 24 次目标类型错误在 CLI 内确定性消除；
- 11 次无歧义 flag 漂移不再发生；
- 1 次错误认证分支不再由误导 hint 触发；
- 37 次同名群由正确夹具或显式用户消歧处理，Runtime 继续禁止猜第一个候选。

## 8. 优化实施拆分

优化按错误归属拆成四个可以独立验证的交付单元，避免 Runtime、Skill 和评测夹具混在同一个改动里。

### 8.1 交付 A：错误语义与恢复动作

范围：

- `internal/app/runner.go`
- `internal/transport/diagnostics.go`
- `internal/errors/`
- 新增中央 server failure classifier 及测试

交付结果：

1. `NETWORK_ERROR + queryToolMeta/UNAVAILABLE` 不再落入 `business_error`；
2. JSON 输出具备稳定的 `origin/stage/reason`；
3. 参数、权限、资源和基础设施错误给出不同恢复动作；
4. 未确认 `execution_started=false` 前不对写请求自动重试；
5. 错误提示不再诱导 Agent 执行无关认证命令。

该交付只改变错误解释和恢复合同，不改变 `+conversation-list` 的请求拼装。

### 8.2 交付 B：统一群目标 Runtime

范围：

- `internal/shortcut/targetresolver/`
- `internal/shortcut/smart/group_members.go`
- `internal/shortcut/smart/chat_messages.go`
- `internal/shortcut/smart/search_msg.go`
- 其他调用 `ResolveChat` 的 Chat Shortcut

交付结果：

1. 群名和 CID 在进入下层前完成类型规范化；
2. 稳定 CID 永远不进入 `search_groups`；
3. 自然群名永远不直接进入 `openCid/cid/openConversationId`；
4. 相同的参数名在高频 Shortcut 中表达相同语义；
5. 零命中、多候选和类型不匹配都使用 typed resolution error。

这是消除 24 次目标类型错误的主要 Runtime 改动。

### 8.3 交付 C：兼容参数、Schema 与 Skill 收敛

范围：

- 对应 Shortcut 的真实 Cobra flags/constraints
- `internal/cli/schema_hints/metadata/chat.json`
- `internal/cli/schema_hints/selection/chat.json`
- `skills/multi/dingtalk-chat/SKILL.md`
- 精确 Chat references
- 生成的 Schema/Agent metadata/Catalog

交付结果：

1. 增加经过评审的无歧义兼容 flag，但 Help/Skill 只推荐 canonical flag；
2. 修改群名只推荐现有 reviewed `chat group rename --id --name`；已观测的 `+chat-rename` 仅作为隐藏兼容别名，不成为第二默认入口；
3. `unknown command/unknown flag` 后只查精确 leaf Help并最多重试一次；
4. 示例必须通过真实 Cobra 和 Schema constraints；
5. Skill 不保存固定测试群名、CID 或评测 case 文案。

### 8.4 交付 D：评测夹具与 judge 分层

范围：评测仓库或 runner，不进入 DWS Runtime 业务逻辑。

交付结果：

1. 所有依赖指定测试群名的 case 在 accepted run 前保证目标唯一；
2. 夹具创建、隔离和恢复有稳定 ID 审计记录，但不注入 Prompt；
3. `expected_resolution_stop`、`request_contract_error` 和 `infrastructure_error` 分开统计；
4. infrastructure retry 即使恢复成功，也保留独立可靠性计数；
5. clean 不再把正确的安全停机和错误请求拼装混成一种失败。

## 9. 评测拟合方案

### 9.1 拟合对象

只拟合以下稳定模式：

- 错误结构：`server_error_code`、MCP stage、retryability；
- 意图结构：群名、稳定 CID、搜索关键词、分页参数；
- 命令合同：canonical flag、无歧义兼容 alias、constraints；
- 安全语义：多候选禁止默认第一项、写操作禁止不安全重试；
- 结果合同：complete/partial/failures 和 typed error。

禁止拟合：

- `H4/H8/...` 等 case 编号；
- `dws测试群01/02` 字面值；
- 评测中出现的固定 CID、用户 ID 或 trace ID；
- 针对某条 Prompt 的字符串分支；
- 为提高 clean 而把非零退出统一改为成功。

### 9.2 数据集拆分

采用三层数据集：

| 数据集 | 用途 | 内容 |
|---|---|---|
| observed | 修复已观察问题 | 当前 155 条 accepted、74 次失败调用 |
| variant | 验证泛化 | 替换群名、CID、关键词、flag 顺序、是否显式指定 ID |
| safety holdout | 防止过拟合 | 同名群、零命中、跨 profile ID、写请求执行状态未知、分页中途失败 |

Runtime 单元测试和本地 MCP E2E 使用合成稳定 ID，例如 `cid-test-a`，不得依赖真实评测资源。真实环境 smoke 只验证集成连通性和输出合同，不把真实业务数据写入测试快照。

### 9.3 变体矩阵

每个群目标型 Shortcut 至少覆盖：

| 维度 | 变体 |
|---|---|
| 目标输入 | 唯一群名、CID、模糊群名、同名群、零命中 |
| 参数入口 | canonical flag、兼容 alias、canonical+alias 冲突 |
| 命令类型 | 只读、普通写、高风险写 |
| 后端错误 | 参数错误、权限错误、资源不存在、限流、内部依赖不可用 |
| 执行阶段 | 调用前、元数据查询、业务执行、分页后续页 |
| 结果状态 | complete、partial、fail-closed、retry recovered |

兼容 alias 只需证明与 canonical invocation 等价，不为每个 alias 复制完整业务测试矩阵。

### 9.4 回归顺序

```text
typed unit tests
→ fake MCP CLI E2E
→ Schema/Skill policy
→ observed 31×5 回放
→ variant 集
→ safety holdout
→ 小规模真实环境 smoke
```

只有前四层通过后才看 clean 指标。若 clean 提升但 safety holdout 出现自动选第一个群、重复写或 partial 冒充 complete，则判优化失败。

### 9.5 指标

主指标：

- 请求合同错误数；
- 首次 canonical 命令/flag 命中率；
- 自然目标到稳定 ID 的一次成功率；
- false success、错误写入和重复写入；
- 错误分类准确率；
- complete/partial 真实性。

分层可靠性指标：

- MCP/网关基础设施失败率；
- 安全自动重试恢复率；
- 未恢复 infrastructure error 数；
- trace ID 可关联率。

次级指标：

- clean run 比例；
- 每 case 业务 CLI 次数；
- unknown command/flag 后的纠错次数；
- Token 和执行时长。

clean 不能替代主指标，也不能通过过滤真实失败直接提升。

## 10. 预期收敛与发布判定

基于当前 74 次失败，实施后的预期归宿是：

| 当前错误 | 目标状态 |
|---|---|
| MCP 内部依赖失败 1 次 | 正确分类；满足安全条件时恢复，否则进入 infrastructure 指标 |
| 错误认证分支 1 次 | 由精确恢复动作消除 |
| 目标类型错误 24 次 | Runtime 规范化后消除 |
| 命令/flag 漂移 11 次 | canonical 路由或兼容 alias 消除 |
| 同名群歧义 37 次 | 唯一夹具下正常执行；真实歧义下继续安全停机 |

发布必须同时满足：

1. observed 集中的确定性合同错误归零；
2. variant 集不依赖固定群名和 CID；
3. safety holdout 的误选、误写和重复写为 0；
4. MCP 基础设施错误分类准确，不再显示参数/认证误导；
5. 全仓、Schema、Skill 和真实只读 smoke 全部通过；
6. 评测报告同时给出 clean、业务正确性、安全停机和基础设施可靠性，不再只给一个混合分数。

## 11. 当前分支实施与验证记录

> 实施日期：2026-08-03  
> 实施分支：`codex/multi-im-optimization`

### 11.1 已实施

1. 中央错误分类已在两条 HTTP MCP 失败路径生效：
   - `NETWORK_ERROR` / `queryToolMeta` / `UNAVAILABLE` 分类为 `backend_dependency_unavailable`；
   - 输出 `origin` / `stage` / `reason`，保留 trace、server code 和 retryable；
   - 后端未显式返回时不猜测 `execution_started`；
   - multi-profile 聚合保留错误恢复语义、resolution candidates 和诊断字段。
2. 群目标合同已收敛到 `ResolveChatTarget`：
   - CID 不进入 `search_groups`；
   - 群名唯一解析后才传入业务工具；
   - 零命中、多候选、不完整分页继续 fail-closed；
   - 已应用到成员、会话消息、消息搜索、@ 我、群机器人/邀请、按群发送和群重命名。
3. 11 次命令/flag 漂移的 Runtime 兼容已补齐：
   - `--text` / `--text-query`、`--page-size`、`--open-conversation-id`、成员命令的 `--chat`；
   - `+chat-rename` 作为 `+chat-update` 的隐藏兼容别名，群名依然在写前唯一解析；
   - canonical 与 alias 同时出现时拒绝静默覆盖。
4. Chat Skill、群/消息 reference、reviewed selection hints 与生成 Schema Catalog 已同步。

### 11.2 自动验证

- 本地可控 MCP 重放通过真实 `executeInvocation → HTTP MCP → success=false → typed Error` 路径；
- 聚焦错误、resolver、Chat Shortcut 和 app 回归通过；
- `go generate ./internal/cli` 通过；
- `check-generated-drift.sh` 通过；
- `check-schema-catalog.sh` 通过：26 products / 849 tools；
- `check-skill-commands.sh` 通过：1009 executable command paths；
- Skill `quick_validate.py` 通过；
- Agent 示例合同通过：1047 total / 1023 contract / 24 dry-run；
- `DWS_PACKAGE_VERSION=0.0.0-test go test ./...` 全仓通过；
- `go build -o <temp>/dws ./cmd` 真实构建通过。仓库文档中的裸 `go build ./cmd` 会因根目录已有 `cmd/` 目录而报输出同名冲突，因此验证时显式指定了临时 `-o`。

### 11.3 真实环境 smoke

使用当前登录 profile 和本次构建产物执行，不保存真实群名或 CID 到仓库：

- `+conversation-list --page-all`：完整返回；
- `+chat-members-list --group <真实群名>`：唯一解析成功，`complete=true`；
- `+chat-members-list --open-conversation-id <真实CID>`：兼容 alias 成功；
- `+chat-messages --group <真实群名>` 与 `--open-conversation-id/--page-size`：均成功，单页返回正确标记 `complete=false / hasMore=true / stopReason=single_page`；
- `+search-msg --chat-query <真实CID> --text-query <无命中词>`：不搜群名，搜索完整返回；
- `+chat-rename --group <真实CID> ... --dry-run`：返回 `executed=false`且 invocation 使用正确 CID，未修改真实群。

### 11.4 仓库外后续项

评测夹具唯一性与 judge 分层属于评测 runner，不在本 DWS CLI 仓库中。Runtime 修复后应重跑 31×5 observed、variant 和 safety holdout；真实同名群仍必须保持 `resolution_ambiguous` 安全停机，不以默认第一个候选换 clean。
