# Chat：DWS 可直接优化项开发与验证

**分支：** `codex/chat-cli-parity-20260908`。保留原命令入口。

[交给 IM 团队的独立文档](im-handoff.md)已剔除 CLI 可改项。本文记录本轮代码工作、验证证据和不能外推的边界。

## 1. D01–D18 实施台账

| ID / L项 | 本轮实现 | 参数与场景 | 验证依据及边界 |
|---|---|---|---|
| D01 · L03/L18/L24 | 严格区分明确空集合/未知结构；成员末页清游标；分组缺项进入unresolved | --cursor/--page-token、--single-page、--member-types；错误形状/丢游标/缺项 | 成员真实单人阳性；分组准确CID阳性+不存在目标仍unresolved；形状/分页异常回归 |
| D02 · L07 | 已读用户过滤先精确userId→OpenID，去重后调用已验证target字段 | --users、--message-id、可选--conversation-id | 真实userId重复输入仍只返回本人一条；错配会话非零失败 |
| D03 · L01 | 取消创建名称和成员强制要求，明确传已验证本人，保留后端默认名 | name/members省略、显式值、写入确认 | 无name/members真实创建成功；独立成员读取精确只有本人 |
| D04 · L02/L03/L12/L21/L22/L23 | 保留时间/成员/Bot/资源/置顶/分组原生字段；区分内容类型与ID类型 | createAt/lastMsgCreateAt/owner OpenID、singleChat、resourceIdType/contentType | 真实排序、成员、资源SHA256和置顶详情；未返回字段不编造 |
| D05 · L04/L12/L13/L14/L15/L16/L17/L21/L22/L23/L24 | 补同名命令/参数alias，保持原入口；显式读取别名参数值 | chat-id/sort-order/file-key/at-chatter-ids/feed-group-id/feed-id/page-token/thread/user-id | Cobra↔Schema identity与payload等价回归；真实跨历史/搜索/下载/分组命令 |
| D06 · L02/L03/L04/L05/L13/L15 | 有界自动分页、可中断等待、成员起点/单页；历史恢复token绑定上下文 | page-all/page-limit/page-delay/cursor/page-token/limit/nextPageToken | 历史小页2/大页100准确ID集合一致；空可续页；跨会话token拒绝；中断和范围回归 |
| D07 · L04/L13 | 搜索支持单边/仅时间/all-time；群历史asc默认读取准确群创建时间，使用UTC带时区边界 | start/end/start-time/end-time/all-time/order/sort-order | 真实升序/分页正例；保留默认7天和单聊asc显式start；错误/冲突时间回归 |
| D08 · L04 | 群历史每页明确mark_as_read=false | 所有群历史读取/续页/补查 | 真实读取未清除未读的既有审计证据；当前请求字段回归。不宣称读取者逐消息回执已可查 |
| D09 · L05/L13 | 精确成员/管理者/群模式组合过滤；group/p2p映射；仅开放已验证file搜索类型 | member-ids/is-manager/chat-modes/conversation-type/chat-type/message-type | 自用群成员和管理者阳性、topic阴性；群/单聊结果不交叉；已知文件消息命中；未知type拒绝 |
| D10 · L02/L05/L13 | 按已返回创建/活跃时间/成员数稳定排序；标记returned_items范围 | sort=create_time/active_time/member_count | 真实字段和排序单调性；满页complete=false；相同值稳定ID及坏字段回归 |
| D11 · L07/L08/L12/L15/L16/L17 | 消息精确归属共享预检；省略会话可自动补，显式错配仍阻止 | message-id/conversation-id/open-conversation-id/type | 编辑、收藏、已读、图片下载真实自动定位；假/错消息和资源零错误写入回归 |
| D12 · L04/L09/L13/L15/L18 | 共享Reaction与按需Thread富化；历史/搜索/收藏共用有界实现 | no-reactions/no-enrich/with-threads；Thread10条/最多50次查询与500回复 | 真实Thread回复ID在历史/搜索/收藏读回；共享预算覆盖空Thread；缺项/错误不伪装完整 |
| D13 · L14 | 个人纯文本与Markdown分流，text采用正确content JSON | text/markdown/msg-type/as/uuid/at-open-dingtalk-ids/at-all/ai-tag | 原生text、Markdown、@本人/@all正文准确读回；@本人索引准确命中；视觉与@all提醒待验 |
| D14 · L12 | 资源归属/类型验证、扩展名回退、Range重试/刷新URL、原子落盘；后续200整文件安全重启 | type=image/file/mediaId/fileId、file-key、message-id、output、part-size/retries/retry-delay/overwrite | 真实图片原始SHA256一致，fileId独立下载；禁止覆盖/显式覆盖；503重试、版本变化、200重启回归 |
| D15 · L21/L24 | 置顶/指定分组查询可补会话详情，失败保留基础项和富化失败 | no-detail、page-token、feed-id | 真实base/details准确会话ID集合一致；正向详情读取；失败/错CID回归 |
| D16 · L24 | 分组缺项用反向成员关系证明；不存在分组不能假判notFound | feed-group-id/category-id、feed-id/conversation-ids、exclude-muted | 真实准确目标去重保序；原始列表未知耗尽下的阴性仍unresolved |
| D17 · L11 | 显式普通消息→Thread→回复；已有Thread直接复用；中途失败保留恢复信息 | create-thread/reply-in-thread/thread-id/as、源message-id | 新自用群普通消息转换并发送成功，独立读回准确Thread回复ID；不自动悄悄转换 |
| D18 · L01/L06/L10/L11/L14/L15 | 更新Help/Schema描述，区分发送者名单和阅读者状态、原生text与媒体降级 | 命令选择、主体与类型边界 | Cobra/Schema反向完整性、别名和同构检查；原接口保留，统一入口收敛另行讨论 |

没有把改了alias或增加组合查询直接标为整条Lark shortcut完全对齐。Bot身份、阅读者本人状态、原生富消息/附件编辑、公开可发现群及Feed模型等依然按IM文档分项处理。

## 2. 真实场景验收原则与结果

- 使用已核实的个人profile；新建只有本人的群，成员独立读取确认后才发送测试消息。每条写入等到发送任务给出真实消息ID，再按ID、会话、作者和唯一正文回读。
- 搜索用已知消息ID阳性、不同类型集合对照、确定零命中词和不同页大小集合相等。HTTP成功或空数组本身不计为阳性。
- 对真实失败保留原始调用；修后单独复测，不覆盖成一次通过。下载按原始SHA256验证；编辑/收藏/Thread按准确ID验证。
- 测试输出和安全证据在仓库外私有目录；本报告只公开场景说明和汇总，不复制业务ID、签名URL或账号凭据。

## 3. 仍不能宣称已覆盖的场景

| 范围 | 当前边界 | 处置 |
|---|---|---|
| Bot/Webhook多身份、无效或未开通模板 | 当前Bot样本400004，没有合格Bot阳性夹具 | IM提供主体/权限/模板；不能按个人查询结果外推 |
| text视觉、@all提醒、原生audio/video/富消息附件区 | text正文与@本人索引有证据；原始详情缺少原生type/提及字段 | IM合同与客户端验收；不宣称所有媒体/提醒完全对齐 |
| 会话/群搜索全量及大群、权限变化、同毫秒大量消息 | 既有两项分页BUG仍未修；本轮小页/大页阳性不证明所有租户数据集 | 保留complete/unknown，交IM受控大数据集验收 |
| 大文件长期断流、自然签名过期、跨进程续传 | 真实图片/小文件、分段与整文件回退已验；故障注入覆盖重试/版本/清理 | 未实测自然过期；不提供跨进程resume承诺 |
| 统一入口数量和默认策略 | 原接口和alias保留；新副作用需要显式create-thread | 按此前约定，审计完成后再讨论入口收敛 |

## 4. 检查状态

本轮 D01–D18 可由 DWS 处理的改动已完成；原入口保留，本 PR 保留原入口，不包含发布或下游接口修改。以下结果对应最终源代码与最终二进制。

| 检查 | 结果与范围 |
|---|---|
| 构建 | `make build` 通过，运行时载荷和本地签名完成 |
| 完整 Go 测试 | `DWS_PACKAGE_VERSION=0.0.0-test go test -p 4 ./...` 通过；含 app、mock MCP、脚本和 smoke |
| Schema / 同构 / 反向完整性 | 通过；31 个产品、1373 个工具，包含命令别名与运行时确认一致性 |
| 生成漂移与组装确定性 | 通过；没有新增或提交生成 Catalog / MCP pin |
| Agent 示例 | `make test-schema-agent-examples` 通过，含符合声明条件的真实 dry-run |
| 变更代码覆盖 | 100.0000%，1671 条可执行语句；包含新增生产文件，未调低门禁。按平台门禁命名的测试覆盖异常、重试、参数冲突和部分失败 |
| 格式与差异 | 所有变更 Go 文件 gofmt 检查通过；`git diff --check` 通过 |
| 最终二进制真实验收 | 39 个检查：38 PASS、1 撤回终态未确认。功能断言全部通过；这不是 39 个不同 API 或所有主体全场景覆盖 |

最终真实验收覆盖创建默认值、本人独占群、重命名别名、text/Markdown/提及正文、编辑自动定位与错会话拦截、显式 Thread 转换与回复读回、收藏去重和恢复、历史已知 ID 全集、群筛选正负例、@本人检索、已读目标转换去重、资源准确字节及清理。每次公开 CLI 调用均记录二进制 SHA256，与交付 manifest 一致。此前小页/大页、fileId、分组/置顶及故障对照的证据另行保留，不冒充这 39 项的最终闭环。

**清理边界：** 两个本轮自用测试群均已解散，准确群名查询均验证不再返回。最终群的一条 Markdown 撤回首先返回 `TIMEOUT_ERROR`；随后按准确 ID 读取未返回该消息，解散后用已知会话重试返回业务码 `11056`（`listRoles null`）。无法区分已撤回、不可见或群状态影响，因此保留为终态未确认，不宣称该条撤回成功。其余消息撤回与收藏恢复均有验证证据；没有向其他成员发送测试消息。

**本地验证环境说明：** macOS 曾对部分临时 Go 二进制报 SIGKILL。最终部分测试/策略调用使用临时 ad-hoc 签名和 `go run -exec` 启动同一源码生成器；未改变仓库策略、测试断言、TLS 验证或覆盖阈值。此前失败日志保留，最终通过日志另存。

代码覆盖率只证明被测代码执行覆盖，不证明所有业务模型或所有下游场景均已验收。第 3 节列出的 Bot、客户端提醒、原生媒体、规模与过期条件仍有明确待验范围。

[最终验收逐项记录](verification.md)提供不含业务 ID 的 39 项检查表。

## 5. L01–L24 指令对照

| ID | lark-cli | DWS | 下游剩余事项 |
|---|---|---|---|
| L01 | `lark-cli im +chat-create` | `dws chat +chat-create` / `dws chat +chat-add-bot` | —；R01/R06/R07 |
| L02 | `lark-cli im +chat-list` | `dws chat +chat-list` / `dws chat +conversation-list` | B01；R01/R08/R11 |
| L03 | `lark-cli im +chat-members-list` | `dws chat +chat-members-list` / `dws chat +group-members` | —；R01/R09/R11 |
| L04 | `lark-cli im +chat-messages-list` | `dws chat +chat-messages` | —；R01/R11/R17 |
| L05 | `lark-cli im +chat-search` | `dws chat +chat-search` | B02；R01/R06/R10 |
| L06 | `lark-cli im +chat-update` | `dws chat +chat-update` | —；R01/R06/R07 |
| L07 | `lark-cli im +message-read-users` | `dws chat +messages-read-status` | —；R14；V05 |
| L08 | `lark-cli im +messages-edit` | `dws chat +messages-edit`（个人编辑） | —；R03 |
| L09 | `lark-cli im +messages-mget` | `dws chat +messages-mget` | —；R01/R15/R17 |
| L10 | `lark-cli im +messages-read-status` | `dws chat +messages-read-status` | —；R02 |
| L11 | `lark-cli im +messages-reply` | `dws chat +messages-reply` | —；R01/R03/R04/R05/R16/R18 |
| L12 | `lark-cli im +messages-resources-download` | `dws chat +messages-resource-download` | —；R01/R17；V01/V05/V06 |
| L13 | `lark-cli im +messages-search` | `dws chat +search-msg` / `dws chat +at-me` | —；R01/R10；V02 |
| L14 | `lark-cli im +messages-send` | `dws chat +messages-send` / `dws chat +messages-send-card` / `dws chat +messages-update-card` | —；R03/R04/R05/R16/R18 |
| L15 | `lark-cli im +threads-messages-list` | `dws chat +thread-replies` | —；R01/R11/R17；V03 |
| L16 | `lark-cli im +flag-create` | `dws chat +flag-create` | —；R12 |
| L17 | `lark-cli im +flag-cancel` | `dws chat +flag-cancel` | —；R12 |
| L18 | `lark-cli im +flag-list` | `dws chat +flag-list` | —；R11/R12 |
| L19 | `lark-cli im +feed-shortcut-create` | `dws chat +feed-shortcut-create` | —；R13 |
| L20 | `lark-cli im +feed-shortcut-remove` | `dws chat +feed-shortcut-remove` | —；无新增阻塞需求 |
| L21 | `lark-cli im +feed-shortcut-list` | `dws chat +feed-shortcut-list` | —；R11/R12；V04 |
| L22 | `lark-cli im +feed-group-list` | `dws chat +feed-group-list` | —；R11/R12 |
| L23 | `lark-cli im +feed-group-list-item` | `dws chat +feed-group-list-item` | —；R11/R12 |
| L24 | `lark-cli im +feed-group-query-item` | `dws chat +feed-group-query-item` | —；R11/R12 |
