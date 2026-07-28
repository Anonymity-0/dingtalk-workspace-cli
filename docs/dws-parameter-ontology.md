# DWS 参数字典 / 本体（Parameter Ontology）

> 目的：给"同一概念多种写法"定唯一 canonical，作为参数标准化的单一事实源。
> 状态：**v0 · IM(chat) 试点**（流水线小白鼠）。每个产品的优化流水线以本字典为准归一。
> 归属：canonical 命名由克谨拍板；栩朝维护摸底证据；瑞达/南润/念晨 在各自层对齐；龚睿辅助摸底与真机抽验。

## 0. 两层分治原则（重要）

参数标准化 **不是**把一个概念全局改成一个词——后端 RPC property 本来就随 tool 变。分两层：

1. **Surface 层（CLI flag，人/Agent 面）统一**：一个概念 = 一个 canonical flag，全库永远一致；旧写法保留为 alias（向后兼容）。
2. **Binding 层（flag→RPC property）吸收后端差异**：canonical flag 映射到**该 tool 真实需要的** property（`cid` / `openConversationId` 各归各）。
3. **Lint 层兜底**：凡属身份/资源类概念的 flag，必须 ① 用 canonical 拼写或已注册 alias；② 有非空 binding 指向真实 property。缺任一 = fail。

> **这条 lint 就是能自动抓出那 15 条 chat 真机失败的规则**——它们全都是身份类 flag 的 binding 缺失/错拼写。

## 1. 概念表 · IM(chat)

摸底证据来自 `internal/helpers/chat.go` 的真实 `callMCPTool` 调用点、`internal/shortcut/chat|smart/*.go` 的 flag、`internal/cli/schema_parameter_bindings.json` 的 61 条 `chat.*`。

| 概念 | canonical flag（提议·待克谨拍板） | 现存 flag 别名 | 后端 property（随 tool 变，binding 负责选对） | 备注 |
|---|---|---|---|---|
| 会话标识 | `--conversation` | `--conversation-id` `--open-conversation-id` `--cid` `--id` | `openConversationId`(91) `cid`(23) `openCid`(4) `conversationId`(3) | **15 失败根因**；binding 必须按 tool 选对拼写 |
| 群名（待解析） | `--group` | — | （resolve → 上面的会话标识） | **语义分离**：`--group <名字>`走 resolve，`--conversation <id>`直传，别再混用 |
| 消息标识 | `--message` | `--msg-id` `--open-message-id` `--ref-msg-id` | `openMessageId`(30) `messageId`(25) `openMsgId`(6) `msgId`(6) | ref-类保留独立语义（引用回复） |
| 用户标识 | `--user`（多值 `--users`） | `--to` `--receiver` `--new-owner` `--applicant` `--inviter` | `userId`(90) `openDingTalkId`(21) `receiver`(5) `receiverUid`(2) `applicantUid`(2) `inviterUid`(2) | **card / 入群审批失败根因**：`--receiver`未归到`receiverUid`、`--applicant`未归到`applicantUid` |
| 机器人 | `--robot` | — | `robotCode`(20) | |
| 分页-条数 | `--limit` | `--size` | — | 已有 `--size→--limit` 先例（bindings removals 段） |
| 分页-游标 | `--cursor` | — | `cursor` | |

## 2. 优化流水线（每产品复用，IM 试点验证）

| 步 | 动作 | 自动化程度 | 人评审卡点 |
|---|---|---|---|
| 摸底 | 扫该产品 flag / helper property / binding，产出概念表 | **全自动**（grep/脚本） | — |
| 定 canonical | 每概念定 canonical flag + alias 集 | 半自动（agent 提议） | **克谨拍板命名** |
| 标准化 | flag 改名 + 注册 alias（向后兼容） + 补/修 binding | **agent 可做**（改本产品文件） | 克谨审 diff |
| 入参转绿 | 该产品真机失败 case 复跑 | 自动跑 | 抽验 |
| HINT | selection prose 修 use_when/avoid_when | 半自动 | **B 审 prose** |
| SKILL | 路由校准，与 HINT 对齐 | 半自动 | **A 审路由** |
| shortcut | 裸 CallMCP → 投影+Output；S→G | agent 可做 | **念晨 审** |
| 门禁 | make generate-schema + drift/policy/selection gate + lint | 全自动 | 全绿才 merge |

**Lint 新增规则（本试点产出）**：身份/资源类 flag（会话/消息/用户/部门/…）必须用 canonical 或已注册 alias，且有非空 binding → real property。→ 进 CI，挡住未来所有同类回归。

## 3. 后续产品（fan-out 时各自追加一节）

contact(身份根: userId/openDingTalkId/deptId/mobile) · calendar(time/分页) · aitable(baseId/tableId/recordId/fieldId/roleId + envelope) · todo/mail/doc/drive/oa/sheet/attendance/minutes/…
