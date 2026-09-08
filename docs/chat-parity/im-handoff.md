# IM 团队交付文档：Chat 下游缺陷与能力合同

2026-09-08｜对照 lark-cli `7fd6ef3c07182257ce776cdc5a614e122d5bd4b3`｜DWS 基线 `36680f062a47ac8f65cd41cf40f0d423403cc8c4`。

覆盖 L01–L24。本文仅列需要 IM / MCP 适配 / OpenAPI / 存储团队处理的事项，可由 CLI 独立修改的命名、参数映射、默认值、分页等待、投影、组合查询、消息定位和下载恢复等均已剔除。现有接口应优先补足合同或暴露已存在能力；只有确认不存在后才进入新接口建设。

## 1. 两项已复现的下游 BUG

| 编号 | 相关指令 | 下游接口 | 原子对照与影响 | 修复验收 | 证据句柄 |
|---|---|---|---|---|---|
| B01 / IM-12-B1 · P0 | L02 | `im/list_all_conversations` | limit20返回20、hasMore=false且无cursor；limit100前20项同序且另有80项；已证明20条不是终点，遍历会静默漏会话 | 固定201项，20/100完整遍历集合一致，无漏重 | CHAT-B01 |
| B02 / IM-12-S1 · P0 | L05 | `im/search_groups` | 固定已知2个测试群：limit1第一页1项且有cursor；原样续页0项结束；limit20返回2项，重复两次；相同查询小页遍历漏群，不经过CLI过滤也复现 | 同一数据集limit1返回1+1，limit20返回2，ID集合一致 | CHAT-B02 |

两项均已隔离到原子接口，不能以调用方局部排序或扩大页大小作为修复。每次对照须固定同一身份、查询条件和静态数据快照；同时返回稳定业务错误码，禁止把过期游标或限流伪装成合法空终点。

## 2. 待确认的合同与观察

以下是已观察到的行为，不全部等同于已定责的业务 BUG。请 IM 明确正式合同后确认修复或补充文档。

| 编号 | 范围 | 观察 | 需要确认 |
|---|---|---|---|
| V01 | L01/L12 | required与运行时默认矛盾：Schema必填，但省略名称/资源上下文的部分样本成功 | R07/R17：请IM明确正式默认与条件必填 |
| V02 | L13 | 搜索枚举/未知参数被忽略：p2p混合群/单聊、未知messageType不报错；有效group_chat/single_chat已证明 | R10：枚举未发布前不把所有Lark枚举拒绝/忽略当IM业务BUG |
| V03 | L15 | forward=true结果仍倒序：给早起点2条仍与false同序；无起点为空 | R11：区分遍历方向/时间边界与输出顺序；请求与业务含义有矛盾，待IM确认后定缺陷 |
| V04 | L21 | limit2仍返4：与limit100同4项，未看到cursor | R11：若limit是硬限制则BUG；若接口全量返回则合同说明需修，不宣称已证分页丢失 |
| V05 | L07/L12 | 错配消息/会话仍成功：已读状态和资源URL样本未校验提供的上下文 | R14/R17：先确认参数用途/匹配保证 |
| V06 | L12 | 错误If-Range仍206：同一文件在错误ETag条件下返回片段 | R17：由IM协调下载/存储服务确认条件请求支持；不直接定为IM业务逻辑BUG，需明确可依赖的响应验证字段 |
| V07 | L11 | 未知replyMsgType也成功：无效类型及富内容候选均可能变成文本 | R03/R18：确认拒绝还是降级合同；缺少原生type/资源验收不能宣称支持 |
| V08 | L13 | 确定无命中的搜索原子响应仅含hasMore=false/nextCursor，没有消息集合 | 发布合法空响应：明确messages或conversationMessagesList为空数组；区分失败、字段遗漏与零命中。证据 CHAT-N08 |
| V09 | L14 | 新建自用群原生text/@本人/@all正文均独立读回；详情原始响应不含type或提及状态；@本人搜索已命中准确消息ID | 提供原生type与实际提及目标读回合同，或可独立验证的提醒事件。正文成功不等于提醒语义已验收。证据 CHAT-N09 |
| V10 | L04 | 同一历史起点limit2得到空消息数组、hasMore=true及时间游标，limit5得到非空结果 | 明确limit是否包含被过滤系统消息，允许空可续页的合同、游标进展和时区；不把空页作为结束。证据 CHAT-N10 |
| V11 | L12 | 后续Range携带ETag时可能返回200整文件；错误If-Range曾返回206 | IM协调存储/CDN明确条件请求、版本及200/206语义；不能仅凭状态码证明版本一致。证据 CHAT-N11 |
| V12 | 消息撤回 / 终态读回 | 本人消息撤回返回TIMEOUT_ERROR；其后按准确ID批读未返回；群解散后按已知上下文重试为业务码11056（listRoles null） | 明确撤回超时终态查询、撤回/无权/群解散的逐ID原因，以及安全重试合同。当前无法确认撤回成功；不直接定责为业务BUG。证据 CHAT-N12 |

## 3. 统一交付合同

- 接口必须声明 user/bot/app/webhook 身份、scope、组织与父资源约束；能力不可用要可发现，并区分功能未开通和权限不足。
- 列表成功必须有明确集合；空、未知、不完整、无权分别表达。分页需明确上限、快照/游标有效期与上下文绑定，不根据短页或字段缺失推断全量。
- 写入区分受理、持久成功、部分失败及结果未知。异步返回可查询任务句柄和最终稳定资源ID；超时后提供不会导致重复写的恢复办法。
- 错误至少区分非法参数、身份/权限、资源不存在、冲突/版本过期、限流、服务失败、暂不支持；标明可重试性、退避及安全下一步。
- 下列输入/输出是待 IM 确认的目标合同，不表示现有接口已经支持。建议兼容增加字段/新版本；不得无版本改写已有字段含义。

## 4. 18组下游需求明细

### R01 · 独立Bot身份的创建、目录、成员、历史、搜索、Thread和下载
- 关联：L01–L05/L06/L09/L12–L15；沿用需求号 IM-01；优先级 P1；类型：能力接入/权限确认。
- 需要 IM 交付：交付可调用MCP/原子/OpenAPI、主体/profile/scope和有效Bot样本；不能用个人代查当Bot。
- 接口/身份：im/list_all_conversations、im/search_groups、im/search_messages、im/list_messages_by_ids、chat/get_group_members、chat/list_topic_replies；应用身份入口待确认。
- 输入与边界：actor/app配置引用、conversationId/messageIds/threadId、scope、time/limit/cursor；禁止忽略actor后切个人。
- 输出与成功语义：实际执行主体与资源归属、逐项权限原因、明确items及耗尽；入群前可读起点。
- 验收：提供个人与有效Bot各一套授权样本；同一资源切身份、未入群、退群、跨租户、过期凭据分别验收；同请求切个人/Bot权限范围可解释，未入群/权限不足明确失败。
- 证据：CHAT-R01。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R02 · 当前阅读者本人对收到消息的只读状态
- 关联：L10；沿用需求号 IM-02；优先级 P0；类型：确认现有路线；无则新增。
- 需要 IM 交付：1–50条逐项状态及unknown/无权；现发送者接口1002不适用。
- 接口/身份：现有 im/query_msg_read_status 仅消息发送者可调用；请确认阅读者接口，未发现已接入等价路线。
- 输入与边界：当前阅读者身份；1–50个messageId，必要时conversationId；只读操作。
- 输出与成功语义：每条readerReadStatus=read/unread/unknown及reason，原始消息ID；无权/不支持与false分开。
- 验收：收到消息前后各读一次：状态查询不得改变读回执；与客户端已读/未读两类真实样本比对；查询收到消息成功且不修改读状态；未知不填false。
- 证据：CHAT-R02。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R03 · Bot自发消息编辑、原生post/富回复、独立附件区
- 关联：L08/L11/L14；沿用需求号 IM-03/IM-12-P2；优先级 P0；类型：正文/编辑协议接入。
- 需要 IM 交付：给真实内容/资源写入Schema和保留、替换、清空语义、作者/时间窗口。
- 接口/身份：个人编辑路线已存在；Bot自发消息编辑及附件区独立修改的正式MCP/OpenAPI入口待IM提供。
- 输入与边界：Bot/作者身份、messageId、会话；正文类型与schema版本；附件省略/数组/空数组分别为保持/替换/清空。
- 输出与成功语义：稳定消息ID、内容版本、实际原生type与资源集合；作者限制、可编辑窗口及部分修改是否原子。
- 验收：用Bot自发文本/富消息验证正文编辑；有附件→保持→替换→清空；错误作者和超窗口明确失败；独立读回type与资源，省略和[]不同，未知类型不静默变文本。
- 证据：CHAT-R03。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R04 · 原生音视频、封面/时长/上传转码
- 关联：L11/L14；沿用需求号 IM-08；优先级 P1；类型：已有OpenAPI接入。
- 需要 IM 交付：确认已知官方路线的凭据、媒体ID、原生模板、群/单聊/回复支持范围。
- 接口/身份：已知Bot媒体OpenAPI路线需IM确认可用凭据、模板和MCP接入；不得将普通文件发送认作原生媒体。
- 输入与边界：应用主体、媒体上传坐标、音频时长/格式、视频封面/尺寸/转码任务、群或单聊及引用目标。
- 输出与成功语义：原生media type、资源ID、转码/发送状态与最终messageId；失败不能静默降级file。
- 验收：有效音视频在客户端可播放且气泡类型正确；上传/转码失败、格式不支持、回复及单聊场景分别验收；真实可播放且媒体气泡正确；不能file降级算成功。
- 证据：CHAT-R04。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R05 · 互动卡片发送更新、动作回调
- 关联：L11/L14；沿用需求号 IM-11；优先级 P1；类型：卡片协议/生命周期。
- 需要 IM 交付：A2UI与原生卡片区别、组件/版本、回调订阅及事件去重。
- 接口/身份：卡片平台/A2UI发送更新及动作事件的统一可调用路线待确认。
- 输入与边界：卡片schema版本、组件与事件定义、业务实例ID、更新版本、回调订阅主体。
- 输出与成功语义：实际卡片ID/消息ID、版本、事件ID/动作ID、发起者及去重键；失效动作明确状态。
- 验收：发送→按钮操作→回调→更新全链路；重复回调、乱序、过期版本、无权操作均有明确语义；按钮到事件关联可追踪；失败/过期/重复动作清晰。
- 证据：CHAT-R05。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R06 · 群描述、public/private可发现范围、Bot管理者
- 关联：L01/L05/L06；沿用需求号 IM-04；优先级 P1；类型：产品模型与接口确认。
- 需要 IM 交付：说明钉钉对应模型，已有接口先给出；无对应模型时明确差异。
- 接口/身份：chat/create_group_conversation 与群设置、发现接口；description/public-private/Bot管理员对应模型待确认。
- 输入与边界：独立群描述、可见性/可发现范围、Bot owner/admin主体；平台若没有对应对象应明确unsupported。
- 输出与成功语义：独立description字段；可见/加入/邀请权限分离；实际owner/admin身份类型。
- 验收：未加入但可发现公开群阳性和不可见群阴性；公告、备注不得代替description；Bot管理操作校验权限；描述独立读写，未加入公开群范围可验证；公告/备注不替代描述。
- 证据：CHAT-R06。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R07 · 创建默认值、owner/名称成员限制、邀请链接与可见性
- 关联：L01/L06；沿用需求号 IM-04/IM-12；优先级 P1；类型：权限/边界合同。
- 需要 IM 交付：发布空名/[]/省略语义，修正4013071错误说明；给真实长度和分享权限合同。
- 接口/身份：chat/create_group_conversation、群名更新及群邀请链接相关接口。
- 输入与边界：name省略/空串；groupMembers省略/null/[]；owner默认与成员包含关系；名称长度计算方式；分享有效期和权限。
- 输出与成功语义：准确必填/条件必填声明与实际默认；名称过长错误而非含糊4013071；邀请链接和会话跳转链接分开。
- 验收：发布正式边界合同；空名默认、单人创建、owner非本人、名称边界、私有群邀请与无权分享；检查实际持久状态；不因默认值改动更新群名；失败保留已创建群；邀请与跳转分别验。
- 证据：CHAT-R07。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R08 · description/chat_status/单聊对端/类型枚举/会话跳转
- 关联：L02；沿用需求号 IM-12-F1；优先级 P1；类型：字段来源合同。
- 需要 IM 交付：提供准确字段源、主体ID和可执行链接。
- 接口/身份：im/list_all_conversations、chat/get_conversation_info。
- 输入与边界：个人目录/详情读取；明确群与单聊模型、owner/对端ID命名空间。
- 输出与成功语义：description/chat_status、singleChat对端ID、完整类型枚举、会话跳转字段；不可得明确null/omitted含义。
- 验收：群/单聊/话题群/外部会话逐类核对；对端不得由owner推断；跳转必须打开准确会话；未知不伪造normal或false；owner不冒充单聊对端。
- 证据：CHAT-R08。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R09 · 成员固定批次、Bot全集及ID转换
- 关联：L03；沿用需求号 IM-12-M1；优先级 P1；类型：分页/ID/覆盖合同。
- 需要 IM 交付：明确是否可控页大小、Bot过滤/封顶、OpenID/userId/unionId映射。
- 接口/身份：chat/get_group_members、bot/list_group_bots、身份解析接口。
- 输入与边界：cursor类型、页大小支持与固定批次上限；user/bot过滤；OpenID/userId/unionId命名空间。
- 输出与成功语义：真实hasMore/nextCursor或承诺无截断全量；机器人类型、范围、封顶及外部身份转换错误。
- 验收：至少超过用户固定一页的群和多类机器人群；跨页集合无漏重；不支持的ID转换逐项明确；用户500批次可续；Bot未知范围不当全集；外部身份逐项说明。
- 证据：CHAT-R09。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R10 · 高级附件、发送者排除、@all、搜索匹配与排序
- 关联：L05/L13；沿用需求号 IM-05/IM-12-S2；优先级 P0；类型：搜索筛选合同/扩展。
- 需要 IM 交付：发布有效枚举/AND-OR/默认值与非法值行为；明确更新/活跃定义。
- 接口/身份：im/search_groups、im/search_messages。
- 输入与边界：messageType/searchConvType等有效枚举；atMe/atUserIds/atOpenDingTakIds/@all；sender包含/排除、onlyRobotMessages；AND/OR与排序定义。
- 输出与成功语义：实际生效过滤、原生消息type/附件类型/提及ID、稳定命中ID；非法枚举报错；合法空结果显式[]。
- 验收：对每个参数准备已知命中和确定不应命中的真实ID；未知枚举不能被忽略；消息text筛选不得混入文件；全局排序验证完整集合；稳定ID正负样本；false不猜成排除；返回字段存在不等于全局筛选。
- 证据：CHAT-R10。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R11 · 全量上限、分页快照、Thread方向、置顶limit和分组耗尽
- 关联：L02/L03/L07/L15/L18/L21–L24；沿用需求号 IM-05/IM-12-T1/TOP1/F2；优先级 P0；类型：完整性/顺序合同。
- 需要 IM 交付：明确无截断全量或返回可续游标；方向/同毫秒边界/版本绑定。
- 接口/身份：目录、成员、已读、chat/list_topic_replies、im/list_message_favorites、置顶与category读取。
- 输入与边界：limit语义、cursor与上下文/快照绑定、时间精度/时区、forward遍历方向与最终顺序、same-millisecond边界。
- 输出与成功语义：实际耗尽或明确truncated；合法空页可续游标；列表一律显式[]；hasMore=true必须可续，unknown不得false。
- 验收：固定跨多页集合，对比小页与大页ID集；同毫秒、系统消息过滤形成的空页、权限变化、过期cursor；Thread首/末页顺序；置顶limit2返回4须明确合同；固定集合跨页一致；单页升序取最早项；未知耗尽不报完整。
- 证据：CHAT-R11。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R12 · message/feed双层、Thread Feed、更新窗口和删除墓碑
- 关联：L16–L18/L21–L24；沿用需求号 IM-06；优先级 P1；类型：产品模型/新增或暴露。
- 需要 IM 交付：先确认平台对象；已有则提供独立ID/层级、updateTime、deleted与cursor。
- 接口/身份：im/add_message_favorite、im/remove_message_favorite、im/list_message_favorites；置顶/分组Feed模型入口待确认。
- 输入与边界：message/feed对象类型、独立ID、只取消一层/两层；更新时间窗口、增量游标、删除墓碑。
- 输出与成功语义：独立对象身份与层级、updateTime与deleted、稳定增量cursor；明确平台不支持的Feed类型。
- 验收：消息层与Feed层分别创建/取消且互不误伤；Thread Feed、删除后增量消费、窗口边界重复消除；单层取消互不影响；删除可增量消费；createAt不冒充updateTime。
- 证据：CHAT-R12。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R13 · 个人置顶会话head/tail/相对位置
- 关联：L19；沿用需求号 IM-07；优先级 P1；类型：确认排序路线；无则新增。
- 需要 IM 交付：提供位置写入、有序读回与并发语义；不是群内toolbar排序。
- 接口/身份：im/set_top_conversation已有置顶开关；位置写入入口待确认。
- 输入与边界：个人会话置顶目标、head/tail或相对会话位置、预期版本与并发冲突策略。
- 输出与成功语义：有序会话列表及版本，位置变更结果；置顶幂等与不存在目标错误。
- 验收：至少3个受控置顶会话首尾插入、移动、重复调用和并发；不得依赖取消再置顶的隐含副作用；至少3项首尾插入及重复/并发，不靠取消再置顶副作用。
- 证据：CHAT-R13。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R14 · 发送者已读名单完整性与Bot发送句柄
- 关联：L07；沿用需求号 IM-09/IM-12-R1；优先级 P1；类型：接入/结果范围合同。
- 需要 IM 交付：个人接收者快照/人数分页；Bot processQueryKey与主体会话绑定。
- 接口/身份：im/query_msg_read_status；Bot已读人OpenAPI的句柄接入待确认。
- 输入与边界：发送者个人/Bot主体、messageId与会话绑定；Bot processQueryKey映射；过滤目标及cursor/limit。
- 输出与成功语义：发送时接收者快照、逐项阅读状态与原因、分页/总数含义；不把当前成员列表当发送时接收人。
- 验收：大群跨页、发送后入退群、已撤回/过期句柄、错误主体/会话；过滤1人不能返回其他人；超过一页、过期、入退群；当前群成员不替代发送时接收人。
- 证据：CHAT-R14。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R15 · 原生批读逐项成功/失败与稳定ID错误码
- 关联：L09；沿用需求号 IM-12-MGET1；优先级 P1；类型：下游体验增强。
- 需要 IM 交付：覆盖无效、不存在、无权、撤回、限流等；减少调用方隔离失败项的额外请求。
- 接口/身份：im/list_messages_by_ids。
- 输入与边界：1–50条有序messageId集合；合法ID、不存在、畸形、无权、撤回混合；认证与限流属于请求级错误。
- 输出与成功语义：每个请求ID明确success/not_found/forbidden/recalled/invalid/unknown，保留成功数据；不得整批失去成功项。
- 验收：真ID+畸形ID+不存在ID混批；成功ID集合与单查一致；认证/网络失败不得被拆解成资源不存在；混合ID保留成功项，认证/网络错误不误归坏ID；属于原生批读增强，不是缺失整个批读功能。
- 证据：CHAT-R15。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R16 · uuid范围/TTL/首次回执、有效robotCode模板
- 关联：L11/L14；沿用需求号 IM-12-P1/P3；优先级 P1；类型：幂等/身份合同。
- 需要 IM 交付：明确重复请求语义、模板与profile映射，提供可用Bot样本。
- 接口/身份：chat/send_personal_message、发送状态查询、Bot发送/回复与模板路线。
- 输入与边界：uuid作用域/TTL/相同键不同body；profile-app-robotCode/template匹配；异步openTaskId。
- 输出与成功语义：首次/重复受理回执、最终SUCCESS/FAILED/UNKNOWN、真实messageId；commit-unknown安全恢复。
- 验收：相同键重放与冲突body、多进程、过TTL、状态超时；提供至少一个可用Bot模板。现400004样本只证明当前配置不可用；同键重试不重复；当前400004不能作为全平台Bot不可用结论。
- 证据：CHAT-R16。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R17 · 资源类型、归属、签名过期、Range版本与转发/文件夹坐标
- 关联：L04/L09/L12–L15/L18；沿用需求号 IM-10/IM-12-DL1；优先级 P1；类型：资源/富化协议。
- 需要 IM 交付：保留content type与ID type；给ETag/总长/过期/子项定位合同。
- 接口/身份：IM消息/资源读取与 get_resource_download_url，Drive文件下载及存储/CDN。
- 输入与边界：resourceIdType=mediaId/fileId与contentType=image/file等分离；消息归属；签名有效期、Range/If-Range、资源版本。
- 输出与成功语义：实际resourceType、resourceIdType、可验证父消息/会话；文件名、长度、ETag、过期错误；转发/文件夹子项坐标。
- 验收：不同图片/文件及大文件；正确和错误If-Range、自然过期、403/429、断流、跳转、版本改变；200整文件与206分段不得混淆；Range不得混版本；无上下文样本不外推全资源；正文片段不是附件。
- 证据：CHAT-R17。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

### R18 · 消息content结构、type支持枚举和原生结果字段
- 关联：L11/L14；沿用需求号 IM-12-P4/IM-12-P2；优先级 P1；类型：正式类型Schema与读回。
- 需要 IM 交付：请发布各type真实协议和未知值错误；text内层content的既有成功样本不代替完整类型合同。
- 接口/身份：chat/send_personal_message、回复/编辑和im/list_messages_by_ids。
- 输入与边界：公开text/markdown/post/引用/媒体的content JSON schema；支持的replyMsgType；提及字段正式定义。
- 输出与成功语义：实际native type、已生效提及目标/@all、正文及附件区、引用关系；未支持type明确拒绝或显式降级。
- 验收：原生text已能独立读回正文；必须进一步证明Markdown字面显示、@自身/@all提醒及富内容类型；未知replyMsgType成功不能当原生类型支持；未知replyMsgType被接受不能证明支持；原生type/资源可读回核验。
- 证据：CHAT-R18。涉及未提供有效Bot、特殊租户或媒体样本的分支，状态为待验，不宣称能力已不存在。

## 5. L01–L24 对照与下游归属

| ID | lark-cli 指令 | DWS现有入口 | 下游事项 |
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

L20既定个人会话取消置顶任务没有新增阻塞需求。L08的个人编辑不等价于Bot作者编辑；L10阅读者状态不等价于发送者查询已读人。这两项身份/目标差异须独立处理。

## 6. 验收数据与团队协作

| 数据/环境 | 用途 | 责任与回收 |
|---|---|---|
| 静态多页会话/群搜索/成员数据集；含系统消息与同毫秒消息 | B01/B02、R09/R11；小页/大页稳定ID集合对照 | IM提供可复现的受控租户，固定时间快照 |
| 当前有效Bot、模板、profile-app映射、群与单聊样本 | R01/R03/R04/R14/R16 | IM/开放平台提供最小权限配置与失效时间 |
| 收到但未读、收到已读、本人发送三类消息 | R02/R14，逐消息状态与无副作用读取 | IM提供可独立观测回执的测试账号 |
| 有附件/富正文/卡片、原生音视频、超大资源、即将过期URL | R03/R04/R05/R17/R18 | 业务/媒体/存储Owner协作，测试后回收 |
| 至少3个置顶会话、分组与Thread Feed | R12/R13 | 在专用账号执行，记录并恢复准确顺序和成员关系 |

建议逐项回复：正式Owner、现有可调用接口或缺失结论、精确Schema/权限、错误码、已知限制、测试环境、验收时间。每项回归须同时跑公开DWS指令和同场景原子请求；通过稳定资源ID核对结果。

## 7. 证据与分发边界

本文不包含账号、组织/profile、真实会话/消息/用户ID、token、签名URL、业务正文或trace/requestId。CHAT-*为不透明审计证据句柄；原始输入输出和复现映射单独保存在本地受控目录，按授权渠道提供给Owner。本文已可转交，但本轮未向任何人发送、未提交到业务系统。
