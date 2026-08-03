# Multi IM 优化完整执行方案

> 状态：方案已实施并完成端到端验证
>
> 日期：2026-08-03
>
> 适用范围：DWS Multi Skill 的 IM 发送、读取、搜索、群创建、回复、资源下载与个人 IM 事件监听
>
> 实现分支：`codex/multi-im-optimization`（基于最新 `origin/main`）
>
> Token 口径：`tiktoken 0.12.0`、`o200k_base`

## 0. 执行摘要

本方案的核心不是继续增加 Shortcut，也不是盲目追求 Skill 更短，而是提高每个控制 Token 对正确选路、正确执行、安全和结果判断的贡献，并把仍需 Agent 手工完成的名称解析、分页、富化、资源处理和事件映射下沉到 CLI。

目标路径统一为：

```text
激活 dingtalk-chat
→ 从有限 Golden Route 中选择最小充分的意图入口
→ CLI 内部完成自然目标解析、参数校验、分页、富化和执行
→ 返回稳定、可证明完整性的任务级 DTO
```

优化顺序冻结为：

```text
PR 1：Skill 路由收敛与 shared 冷启动解耦
→ PR 2：统一 Resolver 与发送执行内核
→ PR 3：读取、搜索、消息 DTO 与资源闭环
→ PR 4：扩展建群与回复闭环
→ PR 5：IM 事件高阶入口
→ 后续：依赖下层的新能力
```

其中 PR 1 收益最大、风险最低：不需要新增后端能力，就能消除 Skill 仍在推荐的旧多步链路。PR 2 是功能层 Token 价值密度和模型轮次收益最大的改造。

### 0.1 实施结果

截至 2026-08-03，本方案的 PR 1～PR 5 已在同一实现分支完成：

| 阶段 | 已实现结果 |
|---|---|
| PR 1 | Chat 根 Skill 改为 Golden Route；最小运行契约由单一 authored source 注入 Chat/Shared；正常单产品任务不再要求完整读取 Shared；卡片、Bot、复杂转发与运维细节按需加载 |
| PR 2 | 新增共享 typed target resolver、结构化 candidates 错误和批量全量预检；`+dm`、`+send-to-group` 与 `+messages-send` 复用发送执行内核；高级 user 发送支持 `--user-query/--chat-query` |
| PR 3 | `+chat-messages` 支持 `--chat-query/--user-query`，`+search-msg` 支持批量 `--chat-query/--sender-query`；list/search/mget/thread 共享 `MessageViewV1` 投影和 `im.message-list.v1` 完整性契约；资源失败并入任务级 ledger |
| PR 4 | `+chat-create --member-query` 在读取当前用户和创建群前完成全部成员解析；`+messages-reply` 返回 `im.message-reply.v1` 后续上下文并保证 dry-run 不触达写 transport |
| PR 5 | 新增 `event +listen-im`，把 kind/events/自然目标确定性编译为 EventKey 集合，并复用既有订阅、bus、ready、NDJSON、回滚和清理生命周期；Event 根 Skill 收敛为 Golden Route |

Schema 暴露同步完成：`+chat-create`、`+messages-reply` 和 `event +listen-im` 已进入 reviewed CommandRegistry、selection 与 metadata；生成 Catalog 由源输入重新生成，没有手改输出快照。

### 0.2 验证结果

截至 2026-08-03，最终工作树通过以下验证：

| 验证层 | 命令 / 结果 |
|---|---|
| 构建 | `go build -o /tmp/dws-multi-im-codex-build ./cmd` 通过 |
| 全仓测试 | `DWS_PACKAGE_VERSION=0.0.0-test go test ./...` 通过；90 个包、13,545 个测试/子测试通过事件，命令退出码为 0 |
| 真实 CLI 子进程 E2E | `go test ./test/mock_mcp -run '^TestMultiIME2E_NaturalTargetsCompletenessAndWriteBoundaries$' -count=1 -v` 通过 9 个场景 |
| E2E 覆盖 | 自然用户解析后恰好一次写；文件真实 PUT→commit→发送；用户歧义零写；资源通过受信任 HTTPS 实际原子落盘；第二页失败保留结果并 `complete=false`；Event 自然群 dry-run 零订阅；自然群完整读取；建群 dry-run 零创建；回复 continuation context |
| Event lifecycle E2E | facade 直通真实 lifecycle，覆盖多订阅 ready、正常退出全部清理、第二项创建失败回滚第一项且不启动 consumer |
| 真实钉钉组织 smoke | 当前 OAuth 身份创建仅含自己的临时测试群，真实发送后 delivery=`SUCCESS`，再由 `+chat-messages` 读回同一消息且 `complete=true`；`event +listen-im` 创建两个真实订阅并输出 ready，bounded 退出后两类订阅均为空；最后成功解散临时群 |
| Schema 生成漂移 | `go generate ./internal/cli`、`check-generated-drift.sh` 通过；两次独立生成一致 |
| Schema 合同 | `check-schema-catalog.sh` 通过：26 个产品、848 个工具；Runtime confirmation truth 为 137 个 gated 工具 |
| Agent 示例 | `make test-schema-agent-examples` 通过：1,045 个示例，其中 24 个走真实 dry-run |
| Skill 冷启动门禁 | `check-skill-context-budget.sh` 通过：Chat 根 Skill 8,346 bytes，最小 Runtime 契约 1,531 bytes；`check-skill-commands.sh` 校验 1,003 条可执行路径 |
| 精确 Token | 使用冻结的 `tiktoken 0.12.0 / o200k_base`：Chat 根 Skill 2,217；最小 Runtime 契约 380；Event 根 Skill 2,041；完整 Shared 1,493（正常单产品任务不冷加载） |
| 变更卫生 | `git diff --check` 通过 |

这里的 E2E 是隔离认证目录下启动真实 `app.Execute()` 子进程，经过真实 Cobra 解析和 HTTP JSON-RPC transport 访问本地 mock MCP；它验证 CLI 进程边界、调用顺序、输出 DTO 和写副作用，但不会向真实钉钉组织发送消息或创建群。

真实组织 smoke 与本地 E2E 分开执行：只创建带时间戳且仅含当前用户的临时群，不邀请其他用户；消息、读取、事件 ready/清理验证完成后立即解散。文档不记录真实 conversation/message/subscription ID 或凭证。

### 0.3 固定场景执行证据

| 场景 | 业务 CLI | 内部关键调用 | 结果护栏 |
|---|---:|---|---|
| 已知/自然用户发送 | 1 | resolve → send×1 | 成功只写一次 |
| 多人同名发送 | 1 | resolve | 写副作用 0，返回 candidates |
| 群名文件发送 | 1 | resolve → init → HTTP PUT → commit → send | 上传字节一致，最终只发送一次 |
| 跨会话分页搜索 | 1 | search page 1 → page 2 failure | 保留首屏，`complete=false`、failure ledger |
| 群名读取并下载 | 1 | resolve → list → resource URL → HTTPS download | 受信任域、相对路径、原子落盘、`complete=true` |
| 姓名建群 dry-run | 1 | resolve all → current profile | create 副作用 0 |
| 群事件监听 dry-run | 1 | resolve → compile EventKeys | subscription 副作用 0 |
| 群事件真实 lifecycle | 1 | create×2 → ready → bounded exit → delete×2 | 正常清理；第二项失败时回滚第一项 |
| 引用回复 | 1 | send×1 | 返回 continuation context |

上述场景均不需要泛化 `schema`/`help` 查询，证明 Golden Route 已把用户可见业务命令收敛为 1。模型轮次属于外部 Agent host 指标；仓库内以 reviewed selection fixture 和命令级 E2E 作为可复现门禁，Ark live-model 评测仍按仓库约定保持显式 opt-in，不作为普通 CI 或本轮合并前置条件。

本方案同时冻结以下约束：

1. 不通过删减安全事实、确认门禁或结果完整性换取 Token。
2. 不为已有能力重复新增同义 Shortcut。
3. 不做未版本化的公开输出 DTO 破坏性变更。
4. 不机械修改 `schema_command_registry.json`；只有稳定 identity、主路径或 alias 真正变化时才评审修改。
5. 不手改生成的 Schema Catalog 或 Agent metadata。
6. Token 预算是防膨胀护栏，不是要求删除所有上下文；能显著降低误路由、误写或 false success 的内容应保留。
7. Golden Route 负责公开意图分流；“统一”只发生在内部 Resolver、执行内核、DTO 和错误契约层，不强行收敛成一个万能公开命令。

---

## 1. 背景与当前事实

> 本章 1.1～1.4 冻结的是实施前基线；其中“当前”均指从 `origin/main` 切出实现分支时的状态，不代表完成后的工作树。完成态和验证证据以 0.1、0.2 为准。

### 1.1 当前冷启动成本

三套 IM Skill 根加载体量如下：

| 体系 | 根加载文件 | 行数 | Tokens |
|---|---|---:|---:|
| Mono | `mono/SKILL.md` | 319 | 8,914 |
| Multi | `dingtalk-chat` + `dws-shared` | 229 | 4,922 |
| lark-cli | `lark-im` + `lark-shared` | 460 | 8,449 |

Multi 的总量已经低于 Mono 和 lark-cli，但这不代表当前加载结构合理。Multi 的 4,922 Tokens 中仍有重复路由、旧 SOP 和无条件 shared 串行读取；lark-cli 的 8,449 Tokens 也不应成为 Multi 的目标预算。应吸收的是 lark-im 已验证的任务闭环、结果契约和渐进披露方式，而不是复制其根文件篇幅。

当前 Multi Chat 根加载由两个文件组成：

| 文件 | 行数 | Tokens | 说明 |
|---|---:|---:|---|
| `skills/multi/dingtalk-chat/SKILL.md` | 139 | 3,402 | IM 路由、意图表和旧 SOP |
| `skills/multi/dws-shared/SKILL.md` | 90 | 1,520 | 全局执行、安全、路由和错误契约 |
| 合计 | 229 | 4,922 | 当前正常 IM 强制冷启动 |

其中 `dingtalk-chat` 的“标准 SOP”约 1,551 Tokens，是当前产品根 Skill 最大单块；它与前面的 Shortcut 优先路由、意图表存在重复甚至冲突。

### 1.2 当前 Runtime 已具备的高频能力

| 能力 | 当前入口 | 已有行为 |
|---|---|---|
| 按姓名发单聊 | `chat +dm` | 姓名搜索、唯一性判断、使用 `openDingTalkId` 发送 |
| 按群名发消息 | `chat +send-to-group` | 群名搜索、精确匹配优先、歧义拒绝 |
| 统一发送 | `chat +messages-send` | user/bot/webhook、文本/Markdown、user 文件上传、幂等、@约束 |
| 指定会话读消息 | `chat +chat-messages` | 群/单聊统一浏览语义、默认最近时间、稳定消息投影、可选资源下载 |
| 跨会话搜索 | `chat +search-msg` | 多维过滤、默认近 7 天、翻页、mget 富化、失败 ledger |
| 批量取消息 | `chat +messages-mget` | 批量详情、reaction、资源引用和可选下载 |
| 单资源下载 | `chat +messages-resource-download` | 安全相对路径、默认不覆盖、原子落盘 |
| 引用回复 | `chat +messages-reply` | 自动读取原发送者、可选幂等键 |
| 置顶会话 | `chat +conversation-list-top` | group/direct 规范化、保留 unresolved 项 |
| 创建群聊 | `chat +chat-create` | 当前用户入群并设为群主、基础群类型和话题模式 |
| 事件监听 | `event consume` | 多 EventKey、NDJSON、ready marker、状态与清理 |

因此，当前首要问题不是 CLI 缺少高频入口，而是 Skill 仍指导 Agent 绕开这些入口。

### 1.3 当前 Skill 与 Runtime 的主要冲突

| 用户意图 | Skill 旧路线 | 已有最短路线 | 直接代价 |
|---|---|---|---|
| 给姓名发消息 | `aisearch person` → 原子 `message send` | `chat +dm` | 多一次业务命令和模型继续轮次 |
| 给群名发消息 | `chat search` → `+messages-send` | `chat +send-to-group` | 多一次业务命令和 ID 搬运 |
| 已知目标发送文件 | 独立上传 → 提取 mediaId → 发送 | `chat +messages-send --file ...` | 多次调用、旧链路漂移 |
| 读最近聊天记录 | 旧 SOP 强制 `--time` | `chat +chat-messages` 可自动取当前时间边界 | Agent 需要额外构造参数 |
| Webhook 发送 | 原子 `send-by-webhook` | `chat +messages-send --as webhook` | 身份入口分叉 |
| 群消息下载 | 读消息 → 手工识别资源 → 单独下载 | `+chat-messages --download-resources` | Agent 手工 join，失败语义不统一 |

### 1.4 当前实现仍有的真实缺口

1. `+dm` 与 `+send-to-group` 已完成名称解析，但仍各自直接调用发送接口，没有完全复用 `+messages-send` 的统一内容、幂等、AI 标记和输出逻辑。
2. 当前 person/group resolver 能拒绝零命中和多命中，但错误主要是人类可读字符串，尚未形成稳定结构化 candidates 契约。
3. `+chat-messages`、`+search-msg`、`+chat-create` 和事件监听仍要求调用者先提供 ID。
4. `+chat-create` 与 `+messages-reply` 已存在；后续任务是扩展闭环，而不是再次创建同义命令。
5. 不同消息读取入口的核心字段已较接近，但尚未冻结兼容的公共 Message DTO 和完整性字段。
6. `dws-shared` 既是共享入口又被产品 Skill 当作每次必须完整读取的文本依赖，导致清晰单产品任务仍多一个串行读取阶段。

### 1.5 从 lark-im 吸收什么

吸收原则是“迁移设计能力，不迁移平台假设”。

| lark-im 中值得吸收的设计 | DWS 当前基础 | Multi 的落点 | 阶段 |
|---|---|---|---|
| 根 Skill 以用户意图选择任务级入口 | 已有多个高阶 Shortcut，但旧 SOP 仍优先 | Golden Route 成为唯一默认路由 | PR 1 |
| 已知 leaf 直接执行，参数和安全由 leaf/Runtime 提供 | Schema/Runtime 已能交付参数和 confirmation | 根 Skill 删除整张参数、身份和 Scope 表 | PR 1 |
| 认证、权限和错误恢复按失败类型加载 | shared 当前无条件读取 | 最小公共契约内嵌，完整 shared 按需 | PR 1 |
| 自然姓名/群名解析进入 CLI | `+dm`、`+send-to-group` 已有局部实现 | 统一 typed Resolver，扩展到 send/read/search/create/event | PR 2～5 |
| 歧义返回 candidates，禁止默认第一个 | 当前能拒绝歧义，但主要是错误字符串 | 稳定 resolution envelope 和退出语义 | PR 2 |
| 发送入口统一承载身份、文件和幂等 | `+messages-send` 已具备主要能力 | 让 `+dm`、`+send-to-group` 复用同一执行器 | PR 2 |
| 读取命令直接保留 reaction、引用和资源语义 | Multi 已有投影、mget 和下载基础 | 统一 MessageView、完整性和 failure ledger | PR 3 |
| 搜索、富化、分页是一个任务闭环 | `+search-msg` 已具备大部分链路 | 增加自然 sender/chat 目标和稳定 DTO | PR 3 |
| 建群先完成全部成员解析再写入 | `+chat-create` 目前只接受 IDs | `--member-query` 全量预检 | PR 4 |
| 普通引用回复与 Thread 回复语义分离 | `+messages-reply` 已存在 | 先补引用回复闭环，Thread 等真实 writer | PR 4/后续 |
| 事件入口按用户语义映射底层 EventKey | 已有 `event consume` 控制面 | `+listen-im` 作为确定性 facade | PR 5 |

以下内容不得直接从 lark-im 照搬：

1. Lark 的 `open_id`、`chat_id`、资源 key、消息字段和事件名；DWS 必须保留自己的稳定 identity 和下层接口事实。
2. 群 description、初始 Bot、visibility、owner/manager、Thread writer 等尚未确认的能力。
3. Bot/Webhook 与 user 完全等价的富媒体能力假设。
4. 把嵌套 sender/resources 或 `items` 顶层结构无版本替换到当前公开 DTO。
5. lark-im 的完整根文件体量、全量命令表或平台特有 SOP。
6. 依赖 Agent 手工调用 contact/chat search 再搬运 ID 的多步流程；应吸收结果语义，但把确定性步骤下沉到 CLI。

最终对齐目标不是命令名相同，而是同一类高频意图在两套 CLI 中都具备：短路由、唯一解析、完整结果、可恢复错误和可证明的安全边界。

---

## 2. 目标、指标与非目标

### 2.1 北极星目标

高频 IM 请求应满足：

```text
一个明确用户意图
→ 一个最小充分的 Golden Shortcut
→ 最多一个用户可见业务 CLI 命令
→ 无 Agent 手工搬运 ID、分页或拼接资源结果
```

优化优先级固定为：

```text
任务正确完成
→ 安全且无错误副作用
→ 结果完整、可继续使用
→ 首次选路正确、少返工
→ 在满足以上条件后减少无效控制 Token 和模型轮次
```

不能因为根 Skill 变短，却导致更多 Schema/Help 查询、更多纠错轮次或更高 false success；这种变化属于 Token 搬家，不属于优化。

### 2.2 价值与效率目标

主指标优先于体量指标：

| 类型 | 指标 | 目标 |
|---|---|---|
| 任务价值 | 首次路由正确率 | 不低于基线，并逐 PR 提升 |
| 任务价值 | 任务成功率、完整结果率 | 不低于基线 |
| 安全价值 | 多候选误写、false success | 0 |
| Token 价值 | 已加载根 Skill 内容均能映射到路由、安全、结果判断或错误恢复场景 | 100% 可解释 |
| Token 价值 | 因 Skill 缺失而追加的 Schema/Help/纠错 Token | 不高于基线 |
| 执行效率 | 用户可见业务 CLI 和模型继续轮次 | 在质量护栏内下降 |

体量和轮次作为次级量化目标：

按每轮固定 20K 基础输入计算：

| 指标 | 当前 | 最终目标 |
|---|---:|---:|
| 核心场景平均模型轮次 | 4.67 | ≤3.7 |
| 平均累计输入 | 111.7K | ≤90K |
| 平均常驻上下文 | 25.7K | 23～24.5K |
| 已知目标发送 | 4 轮 | 3 轮 |
| 唯一姓名发送 | 6 轮 | 3～4 轮 |
| 群名读取并下载 | 5 轮 | 3～4 轮 |
| IM 事件监听 | 5 轮 | 3～4 轮 |

业务消息正文、文件内容和最终用户回答不计入控制 Token。

### 2.3 Skill 预算

| 内容 | 目标预算 |
|---|---:|
| `dingtalk-chat/SKILL.md` | ≤2.5K Tokens |
| 正常单产品 IM 强制 shared | 0 |
| 内嵌最小全局契约 | 300～500 Tokens |
| 单个任务 reference | ≤1.5K Tokens |
| 高频任务 reference | 默认不读取 |

Token 使用 `o200k_base` 计量；CI 同时保留稳定字节预算，防止 tokenizer 环境差异让硬门禁失效。这些预算是评审触发线：超过预算必须解释新增内容防止了什么失败；低于预算也不能作为内容充分的证明。

根 Skill 中每个保留区块至少要承担一种明确作用：

- 选择正确任务入口；
- 阻止高代价误操作；
- 解释 Runtime 输出中无法从 Schema 推导的关键语义；
- 把已知失败精确路由到按需 reference。

不承担上述作用的内容迁出；承担作用但表达重复的内容合并；能够由 leaf Schema/Runtime 稳定交付的内容不再复制。

### 2.4 质量与安全护栏

效率优化必须同时满足：

- 高频场景任务成功率不低于优化前基线；
- 多候选写操作副作用为 0；
- 所有写操作继续遵循最终 Runtime/Schema 的 confirmation truth；
- 读取、搜索和下载不得把部分结果标记为完整成功；
- `--dry-run` 与真实执行使用同一目标解析链；
- 不泄露 token、appSecret、webhook token 或内部原始错误载荷；
- 不发生跨 profile、跨组织 ID 串用；
- 不通过隐藏命令、未公开 Shortcut 或猜测接口绕过能力边界。

### 2.5 非目标

本轮不做：

1. 删除旧原子命令或已发布 Shortcut。
2. 为了命名统一机械重写 CommandRegistry identity 或主路径。
3. 把完整 Shortcut Catalog、原子 API 清单或 Scope 表重新放入根 Skill。
4. 在没有下层能力的情况下承诺 Bot 富媒体、Thread 写入、群 description、初始机器人或断点续传。
5. 无版本地把现有 `sender` 字段从字符串改成对象，或把 `messages` 顶层数组改名为 `items`。
6. 同时重构所有 DWS Multi Skill；本方案先验证 IM 路径。

---

## 3. 目标架构

### 3.1 四层职责

| 层 | 负责 | 不负责 |
|---|---|---|
| 产品根 Skill | 触发边界、Golden Route、最小不可推导语义 | 全量命令、完整参数表、旧 SOP |
| 最小全局契约 | CLI 使用、安全底线、ID 不猜测、写后验证 | 产品路由大全、认证手册、错误码大全 |
| Task reference | 低频复杂流程、卡片、复杂转发、Bot 管理、事件运维 | 高频命令选择 |
| Runtime + leaf Schema/Help | 参数、约束、身份矩阵、确认、接口与执行 | 自然语言产品路由教程 |

### 3.2 目标加载路径

清晰单产品任务：

```text
dingtalk-chat 根 Skill（含最小全局契约）
→ 一个 Golden Shortcut
→ 完成
```

低频复杂任务：

```text
dingtalk-chat 根 Skill
→ 精确 task reference
→ 已知 leaf
→ 仅在参数/安全不确定时查询 leaf Schema
→ 仅在 Cobra flag 不确定时查询 leaf Help
```

异常路径：

```text
认证/权限/profile/未知错误
→ 按错误类型读取 dws-shared 对应 reference
→ 修复或停止
```

### 3.3 Golden Route

根 Skill 的核心只保留以下高频路线：

| 意图 | 唯一推荐入口 | 说明 |
|---|---|---|
| 按姓名发文本/Markdown | `chat +dm` | PR 2 后内部委托统一发送器 |
| 按群名发文本/Markdown | `chat +send-to-group` | PR 2 后内部委托统一发送器 |
| 已知目标、文件、Bot、Webhook | `chat +messages-send` | 身份和内容矩阵由 Runtime 校验 |
| 指定会话读取 | `chat +chat-messages` | 最近消息无需强制 `--time` |
| 跨会话搜索 | `chat +search-msg` | 支持多维过滤、翻页、富化 |
| 读取并下载附件 | 查询命令加 `--download-resources` | 不另起手工下载循环 |
| 查看置顶会话 | `chat +conversation-list-top` | group/direct/unresolved 契约 |
| 监听 IM 事件 | 切换 `dingtalk-event` | PR 5 后优先 `event +listen-im` |

以下能力作为次级直接入口保留，但不扩大成完整命令表：

| 意图 | 入口 |
|---|---|
| 已知消息 ID 批量读取 | `chat +messages-mget` |
| 已知资源引用单独下载 | `chat +messages-resource-download` |
| 引用回复 | `chat +messages-reply` |
| 创建基础群 | `chat +chat-create` |

### 3.4 降级顺序

```text
Golden Shortcut
→ 精确 task reference 中的专用 Shortcut
→ 已知原子 leaf
→ leaf Schema / leaf Help
→ 最后才使用低频发现接口
```

已知意图禁止调用：

- 产品级 Schema；
- `schema --all`；
- 完整 `shortcut list --service chat`；
- 父级 Help 代替 leaf Help；
- 旧原子 SOP 作为默认路线。

### 3.5 Golden Route 与内部统一的边界

Golden Route 和内部统一解决的是不同层次的问题：

```text
用户意图层：按任务分流，入口少而明确
    +dm / +send-to-group / +messages-send
    +chat-messages / +search-msg / +messages-mget
                         ↓
Runtime 内部层：复用同一 Resolver、执行内核、投影和错误契约
```

公开层不建设一个同时承担发送、读取、搜索、下载和事件的万能命令。内部层也不允许每个 Shortcut 各自复制名称解析、分页、消息投影或错误处理。

选择公开入口时采用“最小充分能力”原则，而不是“argv 最短”或“功能最多”原则：

| 意图 | 公开入口 | 选择理由 |
|---|---|---|
| 给姓名发简单文本/Markdown | `+dm` | 语义窄，参数空间小，误选身份概率低 |
| 给群名发简单文本/Markdown | `+send-to-group` | 群目标明确，不暴露无关身份矩阵 |
| 文件、Bot、Webhook、复杂 @、已知 ID 或高级发送控制 | `+messages-send` | 需要统一发送的完整能力矩阵 |
| 浏览一个指定会话的消息 | `+chat-messages` | 以会话和时间窗口为中心 |
| 跨会话或按多维条件检索 | `+search-msg` | 以查询条件、分页和富化为中心 |
| 已知一组消息 ID 获取详情 | `+messages-mget` | 精确读取，不应伪装成搜索 |

因此，`+messages-send --user-query/--chat-query` 可以支持“自然目标 + 高级发送能力”，但不会取代简单文本场景的 `+dm` 和 `+send-to-group`。三个入口共享内部发送内核，不共享同一份公开参数复杂度。

---

## 4. Skill 与 shared 设计

### 4.1 `dingtalk-chat` 目标结构

建议根文件固定为：

```text
Frontmatter / 触发边界
→ 最小 DWS 执行契约
→ Golden Route
→ 三条关键结果语义
→ 低频 reference 导航
→ 最短错误分流
```

只保留三类无法完全由 leaf Schema 表达的结果语义：

1. `openTaskId` 是发送任务 ID，不是回复/撤回使用的消息 ID。
2. 消息读取默认保留 reaction、`updateTime`、引用、转发和 `resourceRefs`；`--no-reactions` 可关闭 reaction 输出。
3. Favorite、消息 Pin、消息 Top、会话 Top 是不同对象层级，不能互换。

不在根 Skill 重复完整身份矩阵；只说明如何选择 `+messages-send`。具体 target/content/credential 互斥关系以 leaf Schema 与 Runtime 为唯一事实源。

### 4.2 必须删除或迁移的内容

- 七条旧原子 SOP 的命令细节；
- 人名先查 ID 的手工流程；
- 群名先查会话 ID 的手工流程；
- 文件先上传再提取 mediaId 的旧默认流程；
- 聊天记录强制 `--time`；
- Webhook 默认原子发送路线；
- 完整 Shortcut 表；
- 完整 flags、互斥和 required 说明；
- 卡片 payload、复杂转发、Bot 管理和事件运维细节。

### 4.3 shared 冷启动解耦

正常 IM 请求不再强制完整读取 `dws-shared/SKILL.md`。但不能简单删除 `MUST read` 而丢失全局安全契约。

冻结的交付方式为：

1. 新建一份单一来源的最小公共契约，例如：
   `skills/multi/dws-shared/references/runtime-contract.md`。
2. 由生成器把该区块同步到 `dingtalk-chat/SKILL.md` 的受控 marker 中。
3. `dws-shared/SKILL.md` 继续作为泛称 DWS、跨产品、认证/profile/错误路由入口，但不再是清晰 IM 任务的强制文本依赖。
4. 策略门禁校验源文件与内嵌区块一致，禁止人工漂移。

最小契约只包括：

- 只通过 `dws` 操作；
- 结构化读取使用 JSON；
- 不猜命令、flag、ID 或字段；
- 后续 ID 来自真实返回；
- 不输出凭据；
- 高影响操作遵循 Runtime confirmation；
- 写后按任务契约验证；
- 未知命令只查精确 leaf Help。

### 4.4 shared 按需入口

仅在以下条件加载 shared 或其 reference：

| 条件 | 内容 |
|---|---|
| 登录态或权限失败 | `global-reference.md` / 对应错误章节 |
| profile 或组织不明确 | `dingtalk-profile` 与 shared profile 规则 |
| confirmation 错误 | Runtime confirmation 处理协议 |
| 未知 CLI/业务错误 | `error-codes.md` 对应章节 |
| 跨产品流程 | `workflow-routing.md` |
| 泛称 DWS、产品不明 | `routing.md` |

### 4.5 Task reference 策略

根 Skill 不加载下列低频内容：

- 流式卡片生命周期；
- 合并转发、话题转发和复杂引用；
- Bot 搜索、入群、管理员和撤回；
- Favorite/Pin/Top 的完整原子命令；
- 事件 ready、cooldown、terminal hold、status/stop 运维；
- 兼容媒体链路和异常恢复。

只有现有 reference 超过单任务预算且无法精确定位时才继续拆文件，避免为了“细粒度”制造大量无路由价值的小文档。

---

## 5. 统一 Resolver 契约

### 5.1 目标

所有自然目标解析必须复用同一套内部组件：

```text
ResolveUser(profile, query, requiredIdentity)
ResolveChat(profile, query)
ResolveBot(profile, query)
ResolveUsers(profile, queries, requiredIdentity)
```

Resolver 只做确定性解析，不执行最终写操作。

### 5.2 类型化结果

建议内部结构：

```json
{
  "status": "resolved",
  "entityType": "user",
  "query": "张三",
  "matchType": "exact",
  "selected": {
    "id": "userId-or-openDingTalkId",
    "userId": "...",
    "openDingTalkId": "...",
    "name": "张三",
    "organization": "..."
  },
  "candidates": [],
  "profile": "corpId:userId"
}
```

状态固定为：

- `resolved`
- `ambiguous`
- `not_found`
- `forbidden`
- `upstream_failure`

### 5.3 匹配规则

1. 去除首尾空白，按产品允许的大小写规则比较。
2. 按稳定 ID 去重，禁止同一对象的重复行制造假歧义。
3. 用户解析按稳定 ID 去重后必须只有一个可用候选；即使其中一个显示名 exact，也不能隐藏同名或外部联系人候选。
4. 群聊解析仅有一个 exact match 时优先 exact；多个 exact match 仍为 `ambiguous`。
5. 群聊没有 exact 时，只有一个可用候选才允许 resolved。
6. 多候选禁止默认第一项、最近使用项或第一组织项。
7. 下游需要 `openDingTalkId` 时，不得选择只有 userId 且无法转换的候选；反之亦然。
8. 已提供稳定 ID 时直接使用，不触发名称搜索。

### 5.4 结构化错误

零命中和多命中应返回非零退出码和机器可读细节：

```json
{
  "ok": false,
  "error": {
    "type": "resolution",
    "subtype": "ambiguous",
    "entityType": "chat",
    "query": "项目群",
    "candidates": [
      {
        "name": "项目群",
        "openConversationId": "..."
      }
    ]
  }
}
```

禁止只把候选拼进不可解析的错误字符串。面向人的 `message` 可以保留，但不是唯一数据面。

### 5.5 Profile 与组织一致性

- 解析和最终执行必须使用同一个 `--profile`。
- Resolver 输出记录 profile identity，执行器在写入前校验一致。
- 不缓存或跨调用复用其它 profile 下的 userId/openDingTalkId/openConversationId。
- 多组织名称搜索遵循现有 profile 选择规则；没有明确默认账号时先停止并要求用户选择。

### 5.6 批量解析

`ResolveUsers` 采用全量预检：

```text
解析所有查询
→ 收集 resolved / ambiguous / not_found
→ 任意一项未 resolved，则整体停止
→ 全部 resolved 后才进入写操作
```

创建群等不可接受半完成的操作禁止“成功一个先写一个”。

### 5.7 Dry-run 一致性

`--dry-run` 必须：

1. 运行与真实执行相同的只读 Resolver；
2. 运行相同的 target/content/identity 校验；
3. 输出最终 resolved target 和将调用的下层动作；
4. 在第一个写调用前停止；
5. 不因 dry-run 使用另一套模糊匹配逻辑。

现有 `CallMCPData` 的 dry-run 只读通道继续作为基础；新增 resolver 所使用的下层工具必须进入明确的只读分类，不能用名称猜测绕过写保护。

---

## 6. 分层发送路由与统一执行内核

### 6.1 自然目标参数

扩展 `chat +messages-send`：

```bash
--user-query
--chat-query
```

示例：

```bash
dws chat +messages-send \
  --as user \
  --user-query "张三" \
  --text "请查收"

dws chat +messages-send \
  --as user \
  --chat-query "项目群" \
  --file ./report.pdf \
  --idempotency-key <key>
```

目标严格且只能选择一个：

```text
--chat-id / --group
--chat-query
--user
--user-query
--open-dingtalk-id
```

Bot 和 Webhook 不自动继承 user 的自然目标或文件能力；是否支持由现有身份矩阵和真实下层能力决定。

这些自然目标参数服务于“自然目标 + 文件/复杂 @/幂等等高级发送”场景，不把 `+messages-send` 提升为所有发送意图的唯一公开入口。简单姓名文本继续走 `+dm`，简单群名文本继续走 `+send-to-group`。

### 6.2 统一内部发送器

将当前发送路径收敛为一个内部执行器：

```text
resolve target（如需要）
→ validate identity/content/target
→ normalize @ placeholders
→ resolve/upload file（如需要）
→ attach idempotency / AI tag
→ dispatch lower transport
→ normalize delivery result
```

`+dm` 和 `+send-to-group` 保留稳定 CLI path，但不再各自维护发送实现：

```text
+dm
→ ResolveUser
→ unified send executor

+send-to-group
→ ResolveChat
→ unified send executor
```

这里的“统一”指 transport 前的校验、内容构造、幂等、AI tag、执行和结果归一化，不要求三个公开命令接受完全相同的 flags。

### 6.3 内容与身份边界

- user：文本、Markdown、已有 mediaId 图片、安全相对路径文件/音频/视频；
- bot：仅发布真实下层支持的文本/Markdown；
- webhook：目标由 token 所属群决定，只发布真实支持的文本/Markdown；
- 不把 user 文件能力伪装为 bot/webhook 等价能力；
- 不在 Skill 复制完整矩阵，由 leaf Schema/Runtime 交付。

### 6.4 幂等与重试

- 已提供 idempotency key 时原样透传；
- Agent 或 facade 重试必须复用原 key；
- 未提供时不得宣称具备跨进程幂等；
- 解析成功但发送结果未知时，返回明确 delivery/unknown 状态，不自动换目标重发；
- 不因错误 envelope 误判成功而重复发送。

### 6.5 Dry-run 输出

```json
{
  "identity": "user",
  "resolvedTarget": {
    "type": "chat",
    "name": "项目群",
    "openConversationId": "..."
  },
  "contentType": "file",
  "file": "./report.pdf",
  "idempotencyKey": "...",
  "willSend": false
}
```

### 6.6 兼容策略

- 保留 `+dm`、`+send-to-group`、旧 ID flags 和原子命令；
- 不为统一发送新建第二个 canonical Shortcut；
- 新增 flags 属于现有 Cobra leaf 的兼容扩展，不修改稳定 command identity；
- 只有主路径或 alias 真正调整时才评审 CommandRegistry；
- Skill 始终按最小充分意图选路；PR 2 完成后也不把简单文本意图统一改为 `+messages-send`。

---

## 7. 读取、搜索与资源闭环

读取侧不新增一个万能 `+messages-read`。`+chat-messages`、`+search-msg` 和 `+messages-mget` 的任务语义不同，继续作为三个公开入口；它们只在内部共享自然目标 Resolver、分页基元、`MessageViewV1` 投影、资源处理和完整性契约。

### 7.1 扩展 `+chat-messages`

新增：

```bash
--chat-query
--user-query
```

示例：

```bash
dws chat +chat-messages \
  --chat-query "项目群" \
  --download-resources
```

内部路径：

```text
解析 chat/user
→ 获取消息
→ 投影 sender/text/time/identity
→ 保留 reaction/引用/转发/resourceRefs
→ 计算分页完整性
→ 可选安全下载
→ 输出稳定 DTO
```

省略 `--time` 继续默认以当前时间为边界向前读取；Skill 不再强制 Agent 构造时间。

### 7.2 扩展 `+search-msg`

新增：

```bash
--sender-query
--chat-query
```

其中 `--sender-query` 应支持重复或列表输入，先解析全部发送者，再执行一次搜索；任何名称歧义都在搜索前停止。

继续保留：

- 时间范围和默认近 7 天；
- 多会话、多发送人；
- @我/@对象；
- 消息类型和机器人来源；
- `--page-all` 与页数上限；
- mget 批量富化；
- `--no-enrich`、`--no-reactions`；
- partial failure ledger；
- `--download-resources`。

### 7.3 Message DTO 兼容策略

目标是让 list/search/mget/thread reply 共享一个内部 `MessageViewV1` 构造器，但不得无版本破坏现有公开输出。

第一阶段保留已有字段类型和顶层名称：

```json
{
  "contractVersion": "im.message-list.v1",
  "messages": [
    {
      "messageId": "...",
      "conversationId": "...",
      "threadId": "...",
      "sender": "张三",
      "senderId": "...",
      "senderType": "user",
      "messageType": "text",
      "text": "...",
      "createTime": "...",
      "updateTime": "...",
      "reactions": {},
      "quotedMessage": {},
      "forwarded": [],
      "resourceRefs": []
    }
  ]
}
```

兼容规则：

1. `sender` 暂时保持现有标量类型，不直接替换为对象。
2. 新的 `senderId`、`senderType` 采用增量字段。
3. `resourceRefs` 保留现有名称；如需新 `resources` 结构，必须增量发布或引入显式 DTO 版本。
4. 顶层 `messages` 不无版本改名为 `items`。
5. 原字段删除或类型变化必须走独立兼容性评审。

如果未来确需嵌套 sender/resources 对象，使用显式 `v2` projection 或新主版本，而不是静默切换。

### 7.4 完整性字段

所有分页读取和搜索逐步统一为：

```json
{
  "messages": [],
  "complete": true,
  "hasMore": false,
  "nextCursor": "",
  "pagesFetched": 1,
  "enrichedCount": 0,
  "failedCount": 0,
  "failures": []
}
```

规则：

- `hasMore=true` 且未继续翻页时，`complete=false`；
- 下层缺失可靠分页字段时，不能证明完整，`complete=false`；
- 达到 page limit 且仍有后续页，`complete=false`；
- mget 富化部分失败，保留已有消息并记录 failures；
- 单资源下载失败不丢弃消息；
- 不捕获错误后动态降级为虚假的完整成功。

### 7.5 资源下载契约

- 路径必须是工作目录内安全相对路径；
- 默认不覆盖；
- 显式 `--overwrite` 才允许覆盖；
- 临时文件完成后原子 rename；
- 按稳定资源 key 去重；
- 单资源失败隔离；
- 返回每项 `messageId`、resource key/type、local path、size/error；
- 下载保持 `read/not_required`，不得制造无意义的 `--yes`；
- 资源引用中的子消息 ID 优先使用子消息自己的 `messageId`。

---

## 8. 建群与回复闭环

### 8.1 扩展现有 `+chat-create`

`+chat-create` 已存在，本阶段增加自然成员解析，而不是创建同义命令。

目标示例：

```bash
dws chat +chat-create \
  --name "项目群" \
  --member-query "张三,李四" \
  --type INTERNAL
```

保留现有：

- `--users`；
- `--type INTERNAL|EXTERNAL|NORMAL`；
- `--thread`；
- 当前用户自动加入并设为群主；
- `openCid` 规范化为 `openConversationId`。

新增：

- `--member-query`；
- 批量解析预检；
- dry-run 成员清单与最终下层参数；
- 重复 ID 去重；
- 结构化 ambiguity/not-found 错误。

以下能力只有下层真实支持并完成 Schema/Runtime 评审后才能加入：

- 群 description；
- 初始机器人；
- 指定其他 owner；
- public/private 或其它 Lark 专属语义；
- 服务端幂等键。

不得通过群名预搜索把“禁止同名群”伪装成幂等，因为业务上可能合法存在同名群。

### 8.2 扩展现有 `+messages-reply`

当前入口已能根据原消息补全发送者；本阶段目标是补齐后续可执行上下文：

```bash
dws chat +messages-reply \
  --message-id <id> \
  --conversation-id <id> \
  --text "收到"
```

期望结果包含：

- 新消息 ID（下层可提供时）；
- conversation ID；
- thread ID（适用时）；
- delivery 状态；
- idempotency key；
- 原消息解析结果或来源。

`--reply-in-thread` 只有在确认真实 Thread writer 下层能力后才发布。普通引用回复和 Thread 回复必须是两个明确模式，不能用同一个 flag 猜测下层行为。

### 8.3 写操作安全

- 目标解析、原消息读取和参数校验全部先于写调用；
- 多候选或缺失原消息时不产生副作用；
- confirmation 由最终 Runtime gate 与 Schema 决定，不根据 `risk` 文案机械推导；
- dry-run 必须在写调用前停止；
- 部分成功时返回已创建对象和恢复上下文，禁止统一报成“完全失败”。

---

## 9. IM 事件高阶入口

### 9.1 定位

新增一个高层 facade：

```bash
dws event +listen-im --kind at-me
dws event +listen-im --kind sender --user-query "张三"
dws event +listen-im --kind group --chat-query "项目群"
dws event +listen-im \
  --events message,reaction,recall \
  --chat-query "项目群"
```

它不是新事件系统，而是现有 `event consume` 的确定性编译层。

### 9.2 建议参数

```text
--kind at-me|sender|group|all-direct|all-group
--events message,reaction,read,recall
--user / --open-dingtalk-id / --user-query
--chat-id / --chat-query
--max-events
--timeout
--profile
```

`kind`、target 和 events 的组合由 Runtime 校验；不兼容组合在创建订阅前失败。

### 9.3 确定性映射

示例：

| 用户语义 | EventKey |
|---|---|
| @我消息 | `user_im_message_receive_at` |
| 指定人的单聊消息 | `user_im_message_receive_o2o` |
| 指定人发给我的所有消息 | `user_im_message_receive_user` |
| 指定群消息 | `user_im_message_receive_group` |
| 指定群 reaction | `user_im_message_reaction_group` |
| 指定群撤回 | `user_im_message_recall_group` |
| 指定单聊已读 | `user_im_message_read_o2o` |

映射表由代码和测试维护，根 Skill 只保留高频用户语义，不展开全部 EventKey。

### 9.4 执行与生命周期

内部必须复用现有 event 控制面：

- 多个兼容事件合并为一次命令、一个消费进程；
- 自动启用 `--flatten -f ndjson`；
- stdout 只输出事件 NDJSON；
- stderr 输出 subscription/ready/status/error；
- 等待真实 ready marker，不用 `sleep` 猜测；
- 部分订阅创建失败时回滚已创建订阅；
- stdin、SIGINT、SIGTERM 和 timeout 触发有序清理；
- 尊重 `in_flight`、`cooldown`、`terminal_hold`；
- `--profile` 在目标解析、订阅、status 和 stop 全程一致；
- bounded 结束不泄漏订阅。

由于这是持续输出命令，不应简单套用只在结束时 `rt.Output` 一次的普通 Shortcut 模板。优先设计为复用 `event consume` 内部 runner 的 composite facade，避免通过 shell 启动第二个 `dws`。

### 9.5 `dingtalk-event` Skill 精简

根 Skill只保留：

- 监听对象选择；
- `+listen-im` 高频示例；
- 有界/无界监听；
- 何时进入运维 reference；
- chat 写操作与 event 监听的边界。

全部 EventKey 表、ready 细节、重试状态和 status/stop 运维迁入按需 reference。

---

## 10. 分阶段实施计划

### PR 1：Skill 路由收敛与 shared 解耦

#### 范围

只修改 Skill、reference、生成器和策略门禁；不改变 CLI 行为、公开参数或业务接口。

#### 工作项

1. 将 `dingtalk-chat` 重写为 Golden Route。
2. 删除旧原子 SOP 和旧文件上传路线。
3. 修正 `--time`、Webhook、资源下载说明。
4. 移除正常 IM 对完整 `dws-shared` 的强制读取。
5. 建立最小公共契约单一来源和受控注入区块。
6. 将卡片、复杂转发、Bot 管理、事件运维迁入精确 reference。
7. 增加禁止旧路线回流的静态策略。

#### 验收

- 已知 ID 发送：读产品 Skill → 执行一个业务命令；
- 姓名发送直接选择 `+dm`；
- 群名发送直接选择 `+send-to-group`；
- 最近聊天记录不强制 `--time`；
- Webhook 默认选择 `+messages-send --as webhook`；
- 文件不再推荐独立 mediaId 上传链；
- 同一意图只有一个主入口；
- 根 Skill ≤2.5K Tokens；
- 正常 IM 强制 shared 为 0；
- 所有示例通过真实 Cobra 路径和 flag 校验。

#### 回滚边界

PR 仅改变 Agent 路由文本，可独立回滚，不影响 CLI 兼容性。

### PR 2：统一 Resolver 与发送执行内核

#### 范围

- 内部 typed resolver；
- 结构化 resolution error；
- `+messages-send --user-query/--chat-query`；
- `+dm`、`+send-to-group` 委托统一发送内核；
- dry-run parity；
- Schema/Help/selection metadata 同步。

#### 验收

- 唯一姓名发送只执行一个用户可见业务命令；
- 群名发送只执行一个用户可见业务命令；
- 群名发送文件可由 `+messages-send --chat-query --file` 一步完成；
- 多候选副作用为 0，并返回结构化 candidates；
- 已知 ID 不触发 resolver；
- dry-run 与真实执行选择同一目标；
- `+dm`、`+send-to-group` 经统一发送内核产生等价的下层发送结果；
- AI tag、幂等和内容验证不因入口不同而漂移。
- 简单姓名/群名文本仍分别选择 `+dm`、`+send-to-group`，不会因为内部统一而暴露完整身份参数矩阵。

#### 兼容性

只增加可选 flags 和内部复用；保留已有命令与参数。

### PR 3：读取、搜索、DTO 与资源闭环

#### 范围

- `+chat-messages --chat-query/--user-query`；
- `+search-msg --sender-query/--chat-query`；
- 公共 `MessageViewV1` 投影；
- 完整性字段；
- 资源下载 ledger 统一；
- 兼容性 fixture。

#### 验收

- 群名读取并下载资源只需要一个业务命令；
- 跨群搜索某人消息只需要一个业务命令；
- 单资源失败不影响其它消息；
- 翻页和富化结束后能证明 `complete=true`；
- list/search/mget/thread 的核心字段一致；
- Agent 不再手工 join sender/chat/resource；
- 现有公开 DTO 字段类型和顶层名称保持兼容。

### PR 4：扩展建群与回复闭环

#### 范围

- 扩展现有 `+chat-create --member-query`；
- 批量解析全量预检；
- dry-run 成员计划；
- 扩展 `+messages-reply` 输出上下文；
- Thread writer 若下层仍缺失则只保留需求，不虚假发布。

#### 验收

- 使用姓名创建群不需 Agent 手工查 ID；
- 任一成员歧义时不创建群；
- 当前用户仍在成员列表并作为群主；
- 回复结果可继续用于查询、撤回和资源操作；
- 普通引用回复与 Thread 回复不混淆；
- 不承诺下层不存在的 description/bot/visibility 能力。

### PR 5：IM 事件高阶入口

#### 范围

- `event +listen-im`；
- kind/events 到 EventKey 的映射；
- user/chat natural resolution；
- 多事件合并；
- ready、bounded、取消、回滚和清理；
- `dingtalk-event` 根 Skill 精简。

#### 验收

- 已知监听意图不调用 `event list`；
- 不需要字段推断时不调用 `event schema`；
- 多兼容事件使用一个命令和一个进程；
- 等待 ready marker，不使用 sleep；
- 正常、超时、取消和部分失败均不泄漏订阅；
- 控制模型轮次 ≤4；
- stdout/stderr 契约保持稳定。

---

## 11. 后续依赖下层的能力（明确非本轮遗留）

以下项目依赖尚未存在或未确认的下层能力，不阻塞前五个 PR，也不属于本轮完成定义：

| 优先级 | 能力 | 当前处理 |
|---|---|---|
| P1 | Thread 回复 writer | 未确认前不发布 `--reply-in-thread` |
| P1 | 异步 delivery status / `--wait` | 先保留 `openTaskId` 和明确未知状态 |
| P1 | 图片、文件、音视频统一资源元数据 | PR 3 先规范当前可识别资源 |
| P2 | Bot 富媒体发送 | 不把 user 能力映射给 bot |
| P2 | 大文件分片、Range、断点续传 | 保持当前安全下载边界 |
| P2 | 卡片按钮回调与完整生命周期 | 独立 card/event reference |
| P3 | Moderation、owner/manager/visibility | 仅发布真实 DWS 下层子集 |
| P3 | 事件触发发送幂等关联 | 待 delivery/event correlation 下层支持 |

---

## 12. 评测设计

### 12.1 固定回归场景

1. 已知 ID 发文本；
2. 唯一姓名发文本；
3. 多人同名发消息；
4. 群名发文件；
5. 跨群搜索某人最近消息；
6. 群名读取历史并下载资源；
7. 使用姓名创建群；
8. 监听某群消息和 reaction。

### 12.2 每场景记录

```text
task_success
routing_correct
first_pass_success
model_calls
skill_files_loaded
reference_files_loaded
schema_calls
help_calls
business_cli_calls
internal_read_calls
internal_write_calls
resident_control_tokens
cumulative_input_tokens
ambiguous_target_behavior
side_effect_count
complete
false_success
correction_rounds
```

### 12.3 指标定义

- `model_calls`：初始决策及每次工具结果后的模型继续调用；
- `business_cli_calls`：Agent 显式执行的用户可见 `dws` 业务命令；Shortcut 内部 MCP 调用不另算模型轮次；
- `resident_control_tokens`：本任务进入上下文的 Skill、shared、reference、Schema 和 Help 去重总量；
- `cumulative_input_tokens`：每轮模型输入中固定基础上下文与已驻留控制文本的累计暴露；
- `first_pass_success`：首次选定入口和参数后，无需改走其它命令或补查泛化 Schema/Help 即完成任务；
- `correction_rounds`：因路由、参数、身份或结果解释不足产生的额外模型继续轮次；
- `false_success`：结果不完整、写入未知或部分失败却被报告为完整成功；
- `side_effect_count`：测试期望之外的创建、发送、更新、订阅或删除次数。

Token 价值通过场景证据评审，而不是仅计算“越少越好”：根 Skill 每个区块必须标注它支持的评测场景或阻止的失败；删除区块后若 Schema/Help fallback、纠错轮次或失败率上升，即使冷启动 Token 下降也不能合入。

### 12.4 分阶段预期路径

| 场景 | PR 1 后 | 最终目标 |
|---|---|---|
| 已知 ID 发文本 | `+messages-send` | 1 个业务命令，3 轮 |
| 唯一姓名发文本 | `+dm` | 1 个业务命令，3～4 轮 |
| 多人同名发送 | `+dm` 无写入失败 | 结构化 candidates，副作用 0 |
| 群名发文件 | 仍可能需 ID | `+messages-send --chat-query --file` 一步 |
| 跨群搜某人 | 仍可能需先解析 | `+search-msg --sender-query` 一步 |
| 群名读并下载 | 仍可能需先解析 | `+chat-messages --chat-query --download-resources` 一步 |
| 姓名建群 | 仍需 IDs | `+chat-create --member-query` 一步 |
| 群事件监听 | 切 `dingtalk-event` | `event +listen-im --chat-query` 一步 |

### 12.5 硬门禁

- 任务成功率、首次路由正确率和完整结果率不得低于基线；
- 冷启动 Token 下降但 Schema/Help fallback 或纠错轮次上升，不算通过；
- 高频已知意图 `schema_calls=0`；
- 高频已知意图 `help_calls=0`；
- 唯一姓名/群名最终只执行一个业务 CLI 命令；
- 多候选写操作 `side_effect_count=0`；
- 不允许完整 Shortcut Catalog 进入上下文；
- 不允许根 Skill 恢复旧原子 SOP；
- 不允许正常 Chat 强制读取完整 shared；
- 不允许 partial result 标记 `complete=true`；
- 不允许生成文件被手工编辑。

---

## 13. 测试与 CI 门禁

### 13.1 Skill 静态门禁

扩展 `check-skill-context-budget.sh` 或增加专用检查：

1. `dingtalk-chat` 字节与 Token 预算；
2. Golden Route 精确命令集合；
3. 同一意图不得出现两个“优先/必须”入口；
4. 禁止旧模式文本回流，例如：
   - 发消息必须先 `aisearch person`；
   - 文件必须先 `dt_media_upload`；
   - `+chat-messages` 必须传 `--time`；
   - Webhook 默认走原子命令；
5. 禁止 Chat 完整 Shortcut 表重新展开；
6. 最小公共契约生成区块必须与源一致；
7. 所有示例通过真实 Cobra path/flag/constraint 校验。

### 13.2 Resolver 单元测试

至少覆盖：

- 零命中；
- 单一模糊候选；
- 单一 exact；
- 多个 exact；
- 重复 ID 去重；
- 外部联系人只有 openDingTalkId；
- 下游要求 userId/openDingTalkId 的差异；
- 群 exact 优先和同名群歧义；
- profile 不一致；
- upstream error；
- dry-run 与真实解析相同；
- 已知 ID 不调用搜索。

### 13.3 Shortcut 行为测试

- `+dm`、`+send-to-group`、`+messages-send` 使用同一发送器；
- user/bot/webhook 目标矩阵；
- 文件路径与内容类型；
- idempotency 和 AI tag；
- 解析失败前无写调用；
- send/read/search/create 的 dry-run 计划；
- Message DTO fixture 和向后兼容；
- 分页、富化、资源 partial failure；
- 群创建全量预检；
- 回复结果的后续上下文。

### 13.4 Event 测试

- kind/events/target 映射；
- 多事件去重和顺序；
- ready marker；
- 部分订阅失败回滚；
- timeout/SIGINT/SIGTERM 清理；
- dry-run 不创建订阅；
- profile 一致；
- stdout 只包含 NDJSON；
- bounded 结束无泄漏。

### 13.5 Schema 与生成门禁

每个改变 Cobra flags、Runtime Schema 或 Agent selection 的 PR 必须：

```bash
go generate ./internal/cli
./scripts/policy/check-generated-drift.sh
./scripts/policy/check-schema-catalog.sh
./scripts/policy/check-runtime-confirmation-truth.sh
make skill-command-integrity skill-context-budget
DWS_PACKAGE_VERSION=0.0.0-test go test ./...
```

并重点运行：

```bash
go test ./internal/shortcut/... -count=1
go test ./internal/app -run 'Test(Chat|Event|Schema|ManualAgentExample)' -count=1
make test-schema-agent-examples
```

示例中不得存储 `--yes`；所有公开示例必须通过真实 `BoundCommand` 的路径、flag、required 和跨参数约束。

### 13.6 Live 验证

在具备隔离测试账号和明确授权时运行：

- 唯一/歧义姓名；
- 同名群；
- user 文本与文件；
- bot/webhook 文本；
- 消息分页和下载；
- 事件多订阅和清理。

Live 报告必须脱敏，不保存 token、webhook token、原始私人消息或下载内容。

---

## 14. 生成与单一来源

### 14.1 Command identity

- 已有 `+dm`、`+send-to-group`、`+messages-send`、`+chat-messages`、`+search-msg`、`+chat-create`、`+messages-reply` 保持 identity 和主路径；
- 新增可选 flag 不机械改写 CommandRegistry；
- `event +listen-im` 是新公开 leaf，需要评审 registry identity、主路径、selection 和 metadata；
- alias 只有存在真实兼容需求时才加入。

### 14.2 参数与安全来源

- Cobra tree：命令存在性与接受的 flags；
- Runtime constraints：互斥、至少一个、条件组合；
- metadata：接口、安全、runtime gate 和必要参数 overlay；
- selection：Agent 选择语义；
- Skill：Golden Route，不重复参数真相。

### 14.3 输出 DTO 来源

消息投影应有单一内部 builder 和共享 fixture。list/search/mget/thread 不各自维护字段别名、reaction、quoted、forwarded 和 resource 解析逻辑。

### 14.4 生成文件

只能修改源输入和生成器，再运行生成：

- 不手改 `schema_catalog/`；
- 不手改 `schema_agent_metadata/`；
- 不以旧 Catalog 为生成输入；
- 生成前后必须通过 byte guard 和 drift check。

---

## 15. 风险与缓解

| 风险 | 表现 | 缓解 |
|---|---|---|
| Skill 过度精简 | 低频能力找不到 | 精确 reference + 最后回退发现，不恢复全量 Catalog |
| 移除 shared 后安全丢失 | 产品 Skill 缺全局底线 | 单一来源最小契约生成注入 + hash/marker 门禁 |
| Resolver 误选人/群 | 发错对象 | exact/unique 规则、结构化 candidates、写前停止 |
| 批量解析半完成 | 建出成员不完整的群 | 全量预检，全成功后单次写入 |
| dry-run 与真实执行漂移 | 预览目标不同 | 同一 resolver、同一校验、只在写调用前分叉 |
| DTO 统一导致兼容破坏 | 脚本解析失败 | 共享内部模型、增量字段、显式版本、禁止改字段类型 |
| facade 继续膨胀 | 又出现大量同义入口 | 只扩展五个主入口，保留兼容 wrapper，不再横向新增 |
| 虚假对齐 Lark | 广告下层不支持能力 | `availability/interface_reason` 如实发布，能力进入后续清单 |
| 事件订阅泄漏 | 进程结束后仍订阅 | ready/rollback/cancel/timeout/cleanup 全路径测试 |
| 跨 profile ID 串用 | 错组织操作 | Resolver 结果携带 profile，执行前一致性校验 |
| 输出富化过度 | 延迟和业务数据 Token 增长 | 保留 `--no-enrich`、`--no-reactions`、下载 opt-in |
| 安全从 Skill 下沉后漂移 | confirmation 不一致 | Runtime gate truth + final embedded Schema 语义测试 |

---

## 16. 冻结决策与待确认项

### 16.1 已冻结

1. PR 顺序为 Skill → Resolver/Send → Read/Search → Create/Reply → Event。
2. 正常 IM 不再完整读取 shared。
3. 最小公共契约采用单一来源生成注入，不人工复制。
4. `+dm`、`+send-to-group` 保留兼容，但内部复用统一发送器。
5. 自然目标 canonical flag 使用 `--user-query`、`--chat-query`、`--sender-query`、`--member-query`。
6. 多候选永不默认第一项。
7. dry-run 运行真实只读解析。
8. `+chat-create`、`+messages-reply` 是扩展项，不重复新增。
9. 消息 DTO 只做兼容性增量；嵌套对象需要显式新版本。
10. `event +listen-im` 复用现有事件控制面，不另建订阅系统。
11. Token 预算是防膨胀护栏，北极星是任务价值、正确性、安全和完整性。
12. Golden Route 保留多个最小充分意图入口；发送和读取只在内部执行内核与数据契约层统一。
13. 简单姓名文本走 `+dm`，简单群名文本走 `+send-to-group`；高级发送才选择 `+messages-send`。

### 16.2 实施前仍需确认

| 项目 | 需要确认的事实 |
|---|---|
| Thread writer | 下层是否能稳定返回新消息 ID、thread ID 和 delivery 状态 |
| 群创建幂等 | 下层是否支持服务端幂等键；否则不宣称幂等 |
| 群 description/初始 Bot | DWS 下层是否真实支持，不照搬 Lark 参数 |
| sender DTO v2 | 是否有消费者需要嵌套对象，是否值得显式版本 |
| event bounded 模式 | `--max-events`、`--timeout` 的退出码和 cleanup 语义 |
| Resolution envelope | 是否扩展通用 `apperrors`，供其它产品复用 |
| Token CI | 使用固定 tokenizer 依赖还是字节硬门禁 + Token 报告双轨 |

未确认项不得阻塞 PR 1，也不得在 Skill 中提前广告。

---

## 17. PR 评审清单

### 路由

- [ ] 高频意图只有一个主入口；
- [ ] 已知意图不需要产品 Schema、Catalog 或 Help；
- [ ] 原子命令只作为明确降级；
- [ ] Skill 没有广告当前下层不可用能力。

### Resolver

- [ ] 已知 ID 跳过解析；
- [ ] exact、unique、ambiguous、not_found 行为有测试；
- [ ] candidates 结构化且不泄露敏感信息；
- [ ] profile 全链路一致；
- [ ] dry-run 与真实执行同解析结果。

### 安全

- [ ] 多候选写操作副作用为 0；
- [ ] confirmation 与最终 Runtime gate/Schema 一致；
- [ ] 文件路径、覆盖和原子落盘规则未放宽；
- [ ] partial success 不被报告为完整成功；
- [ ] 事件异常退出完成清理。

### 兼容性

- [ ] 未删除旧 CLI path/flag；
- [ ] 未机械改写 CommandRegistry；
- [ ] 未改变现有 DTO 字段类型；
- [ ] 新字段为增量或显式版本；
- [ ] alias 有真实兼容理由。

### 生成与验证

- [ ] 修改了正确的 authored source；
- [ ] 未手改 Catalog/Agent metadata；
- [ ] generation、drift、Schema policy 通过；
- [ ] Skill 命令完整性和 context budget 通过；
- [ ] 单元、dry-run、fixture、事件清理测试通过；
- [ ] 评测表记录了模型轮次和控制 Token 变化。

---

## 18. 最终验收标准

全部五个 PR 完成后，必须同时达到：

1. 八个固定高频场景的任务成功率、首次路由正确率和完整结果率不低于基线。
2. 多候选写操作副作用和 false success 为 0。
3. 每个根 Skill 区块都能映射到路由、安全、结果判断或错误恢复证据。
4. 八个固定高频场景平均模型轮次 ≤3.7。
5. 平均累计输入 ≤90K。
6. 平均常驻上下文 23～24.5K。
7. 高频已知意图 Schema/Help 调用为 0。
8. 自然姓名/群名场景只执行一个用户可见业务命令。
9. 群名读取并下载可一命令完成。
10. 姓名建群可一命令完成，任一歧义时不创建。
11. 多事件监听使用一个 facade 和一个消费进程。
12. 读取/搜索 partial failure 不产生 false success。
13. 消息 DTO 保持兼容并共享同一内部投影实现。
14. Safety、confirmation、Schema、Help 和真实 Runtime 无漂移。

如果效率指标达到但任务成功率、安全或完整性退化，则该阶段不算通过。

---

## 附录 A：目标根 Skill 骨架

```markdown
# 钉钉群聊 / 消息 Skill

## 最小执行契约

<!-- 由共享源生成；约 300～500 Tokens -->

## Golden Route

| 用户意图 | 唯一入口 |
|---|---|
| 按姓名发文本 | `dws chat +dm ...` |
| 按群名发文本 | `dws chat +send-to-group ...` |
| 已知目标/文件/Bot/Webhook | `dws chat +messages-send ...` |
| 读指定会话 | `dws chat +chat-messages ...` |
| 跨会话搜索 | `dws chat +search-msg ...` |
| 读并下载资源 | 查询命令加 `--download-resources` |
| 查看置顶会话 | `dws chat +conversation-list-top` |
| 监听事件 | 切 `dingtalk-event` |

## 关键结果语义

- openTaskId 不是消息 ID。
- reaction/updateTime/resourceRefs 默认随稳定投影返回。
- Favorite/Pin/Message Top/Conversation Top 不可混用。

## 低频能力

- 卡片、复杂转发、Bot 管理、事件运维按需读取精确 reference。
- 已知 leaf 直接执行；仅在参数/安全不确定时读取 leaf Schema。

## 错误分流

- resolution candidates：交给用户消歧，不猜。
- auth/profile/confirmation：读取对应 shared reference。
- unknown command/flag：精确 leaf Help，最多修正一次。
```

## 附录 B：评测记录模板

```markdown
| 场景 | 成功 | 主入口 | 模型轮次 | Skill | Ref | Schema | Help | 业务 CLI | 常驻 Tokens | 累计输入 | 副作用 | complete | false success |
|---|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|---|
| 已知 ID 发文本 |  |  |  |  |  |  |  |  |  |  |  |  |  |
| 唯一姓名发文本 |  |  |  |  |  |  |  |  |  |  |  |  |  |
| 多人同名发送 |  |  |  |  |  |  |  |  |  |  |  |  |  |
| 群名发文件 |  |  |  |  |  |  |  |  |  |  |  |  |  |
| 跨群搜某人 |  |  |  |  |  |  |  |  |  |  |  |  |  |
| 群名读并下载 |  |  |  |  |  |  |  |  |  |  |  |  |  |
| 姓名创建群 |  |  |  |  |  |  |  |  |  |  |  |  |  |
| 群消息+reaction 监听 |  |  |  |  |  |  |  |  |  |  |  |  |  |
```

## 附录 C：实施总路线

```text
先修 Skill，使 Agent 使用现有最小充分路由
→ 再统一 Resolver，消除名称到 ID 的 Agent 手工步骤
→ 再统一发送内核，以及读取/搜索共享的投影和完整性契约
→ 扩展现有建群与回复入口
→ 最后封装长连接事件入口
→ 下层能力成熟后再补 Thread、Bot 富媒体和大文件能力
```

最终判断标准不是 Shortcut 数量，而是：用户的高频 IM 意图是否能由一个稳定入口完成，Agent 是否不再承担本可由 CLI 确定性完成的工作。
