# DWS IM / lark-cli GSB Fixture 准备方案

> 配套 Query 集合：[dws-im-gsb-core-query-set.md](dws-im-gsb-core-query-set.md)
> 本方案把账号、群、消息、状态、图片、文件、ID 台账、造数顺序和清理顺序整理成可重复的评测前置。
> 当前状态：本地素材已准备；DWS 当前用户登录有效；真实测试账号、机器人和租户资源尚未创建。

## 1. 先给结论：需要你现场提供什么

真实账号不能伪造，也不应把密码、token 或密钥写进评测仓库。开始造真实群之前，你只需要给出下面这些账号的姓名、邮箱或平台用户 ID。

### 1.1 最小可运行账号集

| 角色 | 数量 | 你需要提供 | 用途 | 是否已有 |
|---|---:|---|---|---|
| `GSB_OWNER` | 1 | 确认是否可使用当前已登录 DWS 用户 | 群主、主要发送者、个人状态操作、清理负责人 | DWS 登录已验证，仍需你确认可用 |
| `GSB_ADMIN` | 1 | 同租户姓名或 userId；Lark 可给 open_id | 管理员设置、转让群主、治理命令、清理兜底 | 待提供 |
| `GSB_MEMBER_A` | 1 | 同租户姓名或 userId | 非 Owner 消息发送者、@我、sender 搜索、话题回复 | 待提供 |
| `GSB_MEMBER_B` | 1 | 同租户姓名或 userId | 未读/已读对照、加群/退群/移除成员、批量成员查询 | 待提供 |

这 4 个账号可以完成绝大多数 DWS Query。账号可以是专门测试号；不要使用客户、离职员工或无法及时配合清理的真实业务账号。

### 1.2 完整覆盖增量

| 角色 | 你需要提供 | 额外覆盖 |
|---|---|---|
| `GSB_EXTERNAL` | 另一个组织的测试账号，以及其目标组织 ID | 外部群升级、跨组织数据授权、跨组织边界分析 |
| `GSB_BOT` | DWS `robotCode` / `openBotId`；Lark app ID。无需提供 appSecret | 机器人发消息、撤回、状态查询、Bot-only Lark API |
| `GSB_WEBHOOK` | 只需确认有可用自定义机器人；token 由你在本机设为环境变量，不要发到对话或写入文件 | Webhook 发送 |
| `GSB_SAME_NAME`（可选） | 与 `GSB_MEMBER_A` 同名的第二个测试号 | `+dm` 等姓名解析歧义与澄清能力 |

### 1.3 现场交接格式

你可以直接按下面格式回复；不需要密码、验证码、access token、appSecret 或 Webhook token。

```text
允许当前 DWS 登录用户作为 GSB_OWNER：是/否
GSB_ADMIN：姓名或 userId
GSB_MEMBER_A：姓名或 userId
GSB_MEMBER_B：姓名或 userId
GSB_EXTERNAL：姓名或 userId（没有可留空）
目标外部组织 ID：（没有可留空）
GSB_BOT：机器人名称或 robotCode（没有可留空）
Lark 是否也准备同样四个账号：是/否
```

## 2. 已准备好的本地素材

所有素材均为合成数据，无生产信息、个人信息或凭证。命令从仓库根目录执行时可直接使用相对路径。

| Fixture | 路径 | 用途 | 已验证属性 |
|---|---|---|---|
| 消息附件图 | `docs/fixtures/im-gsb/gsb-im-dashboard.png` | 图片/文件消息、转发、下载、跨平台资源比较 | PNG，1536×1024 |
| 群头像图 | `docs/fixtures/im-gsb/gsb-im-avatar.png` | 群头像更新、Lark 图片上传 | PNG，1254×1254 |
| 文本报告 | `docs/fixtures/im-gsb/gsb-im-report.txt` | 文件消息、关键词搜索、资源下载 | UTF-8 文本 |
| 结构化数据 | `docs/fixtures/im-gsb/gsb-im-data.csv` | 文件附件、下载后内容校验 | CSV，3 条合成记录 |
| 群公告 | `docs/fixtures/im-gsb/gsb-im-announcement.md` | 公告创建、编辑、读取 | Markdown |
| 互动卡片 | `docs/fixtures/im-gsb/gsb-im-card.json` | Lark interactive 消息；DWS 流式卡片内容参考 | 合法 JSON |
| 消息种子定义 | `docs/fixtures/im-gsb/seed-messages.json` | 统一消息 key、发送角色和目标状态 | 13 类消息 |
| ID 台账模板 | `docs/fixtures/im-gsb/fixture-registry.template.json` | 保存真实账号、群、消息和派生 ID | 合法 JSON，不含秘密 |
| 文件校验和 | `docs/fixtures/im-gsb/checksums.sha256` | 上传前后验证本地素材未变 | SHA-256 |

图片由内置 ImageGen 生成，提示词只要求合成评测图、唯一 Fixture 标识、无品牌/人物/敏感数据。图片不是业务视觉资产，不需要人工修图。

### 2.1 可选大文件

Lark 的大文件分片下载在文件超过 8 MiB 时才有意义。它不属于 DWS 当前 151 个命令面的额外分母，因此默认包不提交一个无意义的大二进制文件。需要测分片时，在 `tmp/` 中生成一次性 9 MiB 文件并上传：

```bash
mkdir -p tmp/im-gsb
dd if=/dev/zero of=tmp/im-gsb/gsb-im-large-9m.bin bs=1048576 count=9
shasum -a 256 tmp/im-gsb/gsb-im-large-9m.bin
```

`tmp/` 已被 Git 忽略；测试完直接删除该一次性文件。

## 3. Fixture 命名与生命周期

每次运行都生成新的 `RUN_ID`，避免搜索结果与旧数据混在一起。

```text
RUN_ID = YYYYMMDD-HHMM-operator
群名   = GSB-IM-<用途>-<RUN_ID>
消息词 = GSB_<语义>_<RUN_ID>
TTL    = 24 小时，最长不超过 7 天
```

例如：

```text
GSB-IM-GOV-20260728-1400-operator
GSB_RELEASE_PLAN_20260728-1400-operator
```

真实 ID 复制到：

```bash
mkdir -p tmp/im-gsb
cp docs/fixtures/im-gsb/fixture-registry.template.json \
  tmp/im-gsb/<RUN_ID>.json
```

`tmp/` 已被 Git 忽略。不要把真实 userId、群 ID、消息 ID、Webhook token 或授权响应提交到仓库。

## 4. 群拓扑

### 4.1 完整 DWS 拓扑

| Key | 群名模板 | 初始成员 | 预置 | 主要用途 |
|---|---|---|---|---|
| `GSB_GOV` | `GSB-IM-GOV-<RUN_ID>` | Owner、Admin、A、B、Bot | Owner/Admin/普通成员分层 | 角色、管理员、成员、机器人、禁言、群设置、群昵称、邀请链接、头像 |
| `GSB_MSG` | `GSB-IM-MSG-<RUN_ID>` | Owner、A、B、Bot | 13 类种子消息 | 搜索、已读、回复、编辑、撤回、转发、reaction、收藏、pin/top、附件、卡片 |
| `GSB_DST` | `GSB-IM-DST-<RUN_ID>` | Owner、A | 空群 | 单条/合并/话题转发目标、群邀请分享目标 |
| `GSB_STATE` | `GSB-IM-STATE-<RUN_ID>` | Owner、B | 未读、免打扰、会话分类、置顶 | 红点、隐藏、top、mute、category、Feed 对齐 |
| `GSB_TOPIC` | `GSB-IM-TOPIC-<RUN_ID>` | Owner、A、B | 话题根消息 + 3 条回复 | topic/thread 查询与转发 |
| `GSB_JOIN` | `GSB-IM-JOIN-<RUN_ID>` | Owner、Admin | 开启入群验证；B 从群外申请 | 入群验证列表与审批 |
| `GSB_EXIT` | `GSB-IM-EXIT-<RUN_ID>` | Owner、B | 无其他业务数据 | B 退群，不污染治理群 |
| `GSB_TRANSFER` | `GSB-IM-TRANSFER-<RUN_ID>` | Owner、Admin | 无其他业务数据 | 群主转让，不影响主 Fixture 的清理权 |
| `GSB_DANGER_DISMISS` | `GSB-IM-DANGER-DISMISS-<RUN_ID>` | Owner、A | 只放一条“可解散”说明 | `dismiss_group` 专用，最后执行 |
| `GSB_DANGER_EXTERNAL` | `GSB-IM-DANGER-EXTERNAL-<RUN_ID>` | Owner、Admin | 内部群、无业务数据 | 不可逆外部群升级专用，最后执行 |

### 4.2 Lark 拓扑

Lark 复用 `GOV`、`MSG`、`DST`、`STATE`、`TOPIC` 五种语义群即可。`TOPIC` 用 `--chat-mode topic` 创建；Feed Group 与 Feed Shortcut 属于用户个人状态，不需要额外群。

### 4.3 为什么危险动作必须单独造群

- 解散群不可恢复，不能拿 `GOV` 或 `MSG` 群做测试。
- 群主转让会改变后续清理权限。
- 外部群升级不可逆，不能用后续还要做内部群设置的群。
- 退群会让执行账号失去后续访问，因此使用独立 `EXIT` 群。

## 5. 消息与状态种子

| Key | 谁发送 | 目标 | 内容/附件 | 后续状态 |
|---|---|---|---|---|
| `MSG_RELEASE` | Owner | MSG | `GSB_RELEASE_PLAN...` | 收藏、Pin、Top；用于关键词搜索 |
| `MSG_SENDER` | Member A | MSG | `GSB_WEEKLY_REPORT...` | 用于 sender + time range 搜索 |
| `MSG_AT_OWNER` | Member A | MSG | @Owner + `GSB_ACTION_REQUIRED` | Owner 暂不打开，制造 @我 + 未读 |
| `MSG_EDIT` | Owner | MSG | 故意写错日期 | 后续编辑为周五 20:00 |
| `MSG_RECALL` | Owner | MSG | 明确标注为撤回目标 | 仅用于撤回 |
| `MSG_FORWARD_1..3` | Owner | MSG | 三条连续结论 | 单条转发和合并转发 |
| `MSG_REACTION` | Member B | MSG | reaction 目标 | Owner 添加/移除 emoji；DWS 可加文字表情 |
| `MSG_THREAD_ROOT` | Owner | TOPIC | 话题根消息 | A/B 分别回复，形成 thread |
| `MSG_IMAGE` | Owner | MSG | `gsb-im-dashboard.png` | 下载、转发、文件一致性校验 |
| `MSG_FILE` | Owner | MSG | `gsb-im-report.txt` | 文件下载与内容校验 |
| `MSG_CARD` | Bot | MSG | 流式/互动卡片 | 更新到 `flow-status=3` |
| `NOTICE` | Owner | GOV | `gsb-im-announcement.md` | 创建、编辑、读取、列表 |
| `BOT_DAILY` | Bot | MSG | `GSB_BOT_DAILY...` | 记录 taskId/processQueryKey，用于状态和撤回 |

### 5.1 需要人工在客户端完成的状态

| 状态 | 为什么不能只靠现有 DWS CLI 自动造 | 操作 |
|---|---|---|
| DWS 内联图片 `mediaId` | `chat message send --file-path` 会发送可下载 file，不生成旧版 inline-image mediaId | 在钉钉客户端向 `GSB_MSG` 发送 `gsb-im-dashboard.png`，把消息 ID 与 mediaId 填入台账 |
| DWS 群头像 `mediaId` | `chat group update-icon` 只消费已有 mediaId，不负责本地上传 | 用有 mediaId 的测试图片或在客户端/上游上传后填入台账 |
| 特别关注消息 | 当前 Chat CLI 只有 `list-focused`，没有设置特别关注人的命令 | Owner 在客户端把 Member A 设为特别关注，然后由 A 发送 `MSG_SENDER` |
| 未读对照 | Owner 执行读命令可能改变客户端状态 | 由 Member A 最后发送 `MSG_AT_OWNER`，Owner 在未读 Query 前不要打开对应会话 |
| 已读人员对照 | 必须由多个真实身份分别打开或不打开消息 | Admin 打开 `MSG_RELEASE`，Member B 保持未读 |
| 入群申请 | 需要一个真实群外申请者触发服务端记录 | 开启 `GSB_JOIN` 入群验证后，由 Member B 从客户端申请 |

Lark 图片可通过 `lark-cli im images create` 上传，因此不需要 DWS 式 mediaId 人工步骤。

## 6. 造数顺序

### Phase 0：账号和权限

1. 你按第 1 节提供账号姓名或 ID。
2. 用通讯录只读命令解析并确认唯一身份。
3. 确认 Owner、Admin、Member A、Member B 均为专用测试账号或明确同意参与。
4. 确认 DWS 当前登录 Owner；Lark 同时检查 user/bot 两种身份。
5. 只申请 Query 所需的最小 scope；Webhook token 只放环境变量。

推荐只读检查：

```bash
./dws auth status --format json
./dws contact user search --query "<姓名>" --format json

lark-cli auth status --json
lark-cli contact +search-user --query "<姓名或邮箱>"
```

### Phase 1：创建 ID 台账

```bash
RUN_ID="<YYYYMMDD-HHMM-operator>"
mkdir -p tmp/im-gsb
cp docs/fixtures/im-gsb/fixture-registry.template.json \
  "tmp/im-gsb/${RUN_ID}.json"
```

每创建一个群、消息、reaction、分类或卡片，就立即回填对应 key；不要等整批执行完再补。

### Phase 2：建安全群

先对每种参数组合运行 `--dry-run`，确认成员与群名，再真实创建：

```bash
./dws chat group create \
  --name "GSB-IM-GOV-${RUN_ID}" \
  --users "<GSB_ADMIN>,<GSB_MEMBER_A>,<GSB_MEMBER_B>" \
  --dry-run

lark-cli im +chat-create \
  --name "GSB-IM-GOV-${RUN_ID}" \
  --users "<ou_admin>,<ou_member_a>,<ou_member_b>" \
  --as user \
  --dry-run
```

去掉 `--dry-run` 执行后，把返回的 DWS `openConversationId/groupId` 和 Lark `chat_id` 写入台账。先建 `GOV/MSG/DST/STATE/TOPIC`，再建 `JOIN/EXIT/TRANSFER`，危险群最后建。

### Phase 3：发送基础消息

DWS 文本与附件：

```bash
./dws chat message send \
  --group "<GSB_MSG_OPEN_CONVERSATION_ID>" \
  --text "GSB_RELEASE_PLAN_${RUN_ID}：发布窗口调整到周五 20:00，请查看置顶说明。" \
  --uuid "gsb-release-${RUN_ID}"

./dws chat message send \
  --group "<GSB_MSG_OPEN_CONVERSATION_ID>" \
  --msg-type file \
  --file-path "docs/fixtures/im-gsb/gsb-im-dashboard.png" \
  --uuid "gsb-image-${RUN_ID}"

./dws chat message send \
  --group "<GSB_MSG_OPEN_CONVERSATION_ID>" \
  --msg-type file \
  --file-path "docs/fixtures/im-gsb/gsb-im-report.txt" \
  --uuid "gsb-file-${RUN_ID}"
```

Member A 的消息必须由 Member A 的已授权 profile/会话发送，不能由 Owner 冒充：

```bash
./dws chat message send \
  --profile "<member-a-profile>" \
  --group "<GSB_MSG_OPEN_CONVERSATION_ID>" \
  --text "<@OWNER_OPEN_DINGTALK_ID> GSB_ACTION_REQUIRED_${RUN_ID}：请在今天 18:00 前确认发布风险。" \
  --at-open-dingtalk-ids "<OWNER_OPEN_DINGTALK_ID>" \
  --uuid "gsb-at-owner-${RUN_ID}"
```

Lark：

```bash
lark-cli im +messages-send \
  --chat-id "<GSB_MSG_CHAT_ID>" \
  --text "GSB_RELEASE_PLAN_${RUN_ID}：发布窗口调整到周五 20:00，请查看置顶说明。" \
  --idempotency-key "gsb-release-${RUN_ID}" \
  --as user

lark-cli im +messages-send \
  --chat-id "<GSB_MSG_CHAT_ID>" \
  --image "docs/fixtures/im-gsb/gsb-im-dashboard.png" \
  --idempotency-key "gsb-image-${RUN_ID}" \
  --as user

lark-cli im +messages-send \
  --chat-id "<GSB_MSG_CHAT_ID>" \
  --file "docs/fixtures/im-gsb/gsb-im-report.txt" \
  --idempotency-key "gsb-file-${RUN_ID}" \
  --as user
```

### Phase 4：造派生状态

按依赖顺序执行：

1. 先查询消息列表并回填所有 message ID。
2. 对 `MSG_RELEASE` 设置 favorite、pin、top。
3. 对 `MSG_REACTION` 添加 emoji，记录 reaction ID。
4. 创建 DWS 文字表情，再添加到 `MSG_REACTION`。
5. 对 `MSG_THREAD_ROOT` 发送 3 条回复。
6. 发送 DWS/Lark 卡片并记录 `bizId/openTaskId/processQueryKey/message_id`。
7. 创建 `GSB_ACTIVE`、`GSB_ARCHIVE` 两个个人分类/Feed Group，把 `STATE` 和 `MSG` 放入不同组。
8. 设置 `STATE` 会话 top、mute、@all mute；保留一个未读会话。
9. 创建公告并记录 `noticeId/dataId`。
10. 最后让 Member A 发送 @Owner 消息，之后 Owner 不再打开 `MSG`，直到未读类 Query 完成。

### Phase 5：危险与一次性 Query

以下必须放在普通评测之后：

1. `GSB_EXIT`：Member B 退群。
2. `GSB_TRANSFER`：Owner 把群主转给 Admin。
3. `GSB_DANGER_EXTERNAL`：展示不可逆影响，取得明确确认后升级。
4. `GSB_DANGER_DISMISS`：展示不可恢复影响，取得明确确认后解散。
5. 跨组织授权：只授权目标测试组织和有限 TTL，不使用 `--all`。

## 7. Query → Fixture 覆盖映射

### 7.1 稳定 Schema 78 / 78

| Primary Fixture | Query IDs |
|---|---|
| `GOV / ACCOUNT / BOT` | DWS-S001、S003、S005、S009、S018、S019、S020、S024、S025、S029、S035、S041、S042、S044、S046、S050、S051、S052、S057、S061、S062、S063、S066、S069、S070、S071、S072、S073、S074、S076 |
| `DANGER / EXIT / TRANSFER` | DWS-S012、S038、S078 |
| `MSG / TOPIC / NOTICE / FILE` | DWS-S002、S004、S006、S007、S008、S011、S013、S014、S015、S016、S022、S026、S027、S028、S030、S031、S033、S036、S037、S039、S040、S043、S045、S047、S048、S049、S053、S054、S055、S056、S058、S059、S060、S064、S067、S068、S077 |
| `STATE / CATEGORY` | DWS-S010、S017、S021、S023、S032、S034、S065、S075 |

### 7.2 兼容/runnable 31 / 31

| Primary Fixture | Query IDs |
|---|---|
| `STATE / CATEGORY` | DWS-C001–C005、C007–C009、C020–C024、C028–C029 |
| `AUTH / EXTERNAL` | DWS-C006、C010 |
| `GOV / JOIN / MEMBER / DST` | DWS-C011–C014、C019、C031 |
| `NOTICE` | DWS-C015–C018 |
| `MSG` | DWS-C025–C027 |
| 本地纯文本，无远程 Fixture | DWS-C030 |

### 7.3 Smart 与 Lark 差异项

| Primary Fixture | Query IDs |
|---|---|
| `ACCOUNT + MSG` | DWS-P001、P002 |
| `GOV / DST` | DWS-P003、P004 |
| `STATE` | DWS-P005 |
| `Lark STATE / FEED` | LARK-X001、X007 |
| `Lark GOV` | LARK-X002 |
| `Lark BOT MESSAGE` | LARK-X003–X005 |
| 本地图片 | LARK-X006 |

这张映射覆盖核心 Query 文档中的 121 个唯一 Query；一个 Query 可能还会消费其他辅助 Fixture，但 Primary Fixture 不重复计数。

## 8. 清理顺序

清理必须按造数的反向依赖执行，并由台账驱动，不按模糊群名批量删除。

1. 导出评测结果和必要响应摘要；去除 token、手机号、邮箱和真实姓名。
2. 完成未读、已读、搜索和下载类 Query。
3. 取消 reaction、文字表情、favorite、pin、top 和 Feed Shortcut。
4. 删除/移出个人 category、Feed Group 项，再删除空分组。
5. 完成 bot 消息撤回、普通消息撤回、公告编辑等一次性 mutation。
6. 从群中移除机器人和临时成员。
7. 清理 `EXIT/TRANSFER/JOIN` 等改变成员关系的群。
8. 对仍存在的 DWS 临时群逐个展示群名和 ID，取得一次明确清理确认后再解散。
9. Lark skill 当前没有解散群 API：由 Owner 在客户端删除临时群，或移除成员后退出。
10. 把台账 `status` 改为 `cleaned`，记录 `completed_at`；任何失败资源写入 `remaining_resources`。

不要预先把 `--yes` 写进清理脚本。DWS/Lark 出现确认门禁时，应展示准确目标并等待确认。

## 9. Ready Gate

| Gate | 通过条件 | 当前状态 |
|---|---|---|
| `R0_LOCAL_ASSETS` | 8 个被校验素材/模板和 checksum 文件存在；JSON 可解析、PNG 可读、checksum 匹配 | 已通过 |
| `R1_ACCOUNTS_MIN` | Owner/Admin/Member A/Member B 均已解析为唯一账号 | 等你提供 3 个账号并确认 Owner |
| `R2_ACCOUNTS_FULL` | External、Bot、Webhook 可用 | 可选增强，待提供 |
| `R3_AUTH` | DWS owner token 有效；Lark user/bot identity 与 scope 已确认 | DWS 已通过；Lark 待确认 |
| `R4_GROUPS` | 所有群名带 RUN_ID，ID 已写台账 | 未创建 |
| `R5_MESSAGES` | 13 类消息均有可追溯 ID | 未创建 |
| `R6_STATES` | unread/read/pin/top/favorite/reaction/thread/category 已验证 | 未创建 |
| `R7_MANUAL` | mediaId、特别关注、入群申请已人工完成 | 未完成 |
| `R8_DANGER` | 独立危险群已建，普通 Query 已先完成 | 未创建 |
| `R9_CLEANUP_OWNER` | Owner/Admin 均能执行最终清理 | 待账号确认 |

只有 `R0–R6` 通过后才开始正式跑普通 Query；`R7` 只阻塞对应小部分 Query；`R8` 通过且得到当次确认后才跑两个不可逆 Query。

## 10. 本地复核结果

| 检查 | 结果 |
|---|---:|
| 稳定 Schema Query → Fixture 映射 | 78 / 78 |
| 兼容/runnable Query → Fixture 映射 | 31 / 31 |
| DWS Smart Query → Fixture 映射 | 5 / 5 |
| Lark 差异 Query → Fixture 映射 | 7 / 7 |
| 唯一 Query 总映射 | 121 / 121 |
| JSON 语法 | 3 / 3 通过 |
| SHA-256 | 8 / 8 通过 |
| PNG 可读性 | 2 / 2 通过 |

图片尺寸：

- `gsb-im-dashboard.png`：1536×1024 RGB PNG。
- `gsb-im-avatar.png`：1254×1254 RGB PNG。

真实资源 Ready Gate 尚未通过的原因只有账号、机器人、权限和需要多人动作的状态没有被伪造。收到第 1 节的账号交接信息后，才能安全创建真实临时群并继续回填台账。
