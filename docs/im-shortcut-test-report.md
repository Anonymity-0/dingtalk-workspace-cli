# DWS IM 场景 Shortcut 分层测试报告

> 日期：2026-07-28
> 分支：`codex/im-shortcut-optimization`
> 范围：DWS `chat` 服务 91 个源码 Shortcut；上层 Shortcut 输出与同次调用下层 MCP 响应；写操作 dry-run 与真实可回滚场景；GSB 契约覆盖。
> 隐私：报告不保存真实业务正文、人员 ID、群 ID、Token 或下载 URL。

## 1. 结论

| 指标 | 结果 | 解释 |
|---|---:|---|
| Shortcut 总量 | 91 | 原 89 项，加上 `+chat-members-list`、`+messages-resource-download` |
| 真实业务执行通过 | 79 / 91 | 34 个真实读取 + 45 个真实写入；写入场景均执行回滚或解散临时群 |
| 合法空结果但未完成非空证明 | 0 / 91 | 原 3 个空结果均已通过实际造数得到上层/下层一致的非空结果 |
| 外部 Fixture 阻塞 | 9 / 91 | 需要机器人、待审批记录、群主转让目标或 Webhook token；不计产品失败 |
| 下层服务错误 | 3 / 91 | 两个通知开关返回 1002；合并转发返回权限错误；原生命令同样失败 |
| 写 Shortcut dry-run | 57 / 57 | 参数、目标工具和写入阻断全部通过 |
| 当前公开 Chat Shortcut | 88 / 91 | 88 个 reviewed + available 入口公开；3 个已确认下层失败的入口标记 `unavailable` 并隐藏 |
| Runtime Schema Shortcut 投影 | 42 / 88 | 其余 46 个公开入口以精确 reviewed exclusion 等待逐项 Schema curation |
| GSB 契约覆盖 | DWS 197 / 197 | Schema 78/78、兼容 30/30、runnable parent 1/1、公开 Shortcut 88/88 |

公开状态现在由显式 reviewed 决策与 `availability` 决定；`disposition` 只描述路由关系。缺 Fixture 不会机械隐藏 contract-valid 命令；但已用原生命令复现、确认当前真实不可执行的 3 项标记为 `unavailable`，不会暴露给用户或 Agent。GSB 对 Runtime Schema 中重复投影的 42 个 Shortcut 去重，避免同一入口既计入原子 Schema 又计入公开 Shortcut。

## 2. 测试方法

1. 通过真实 `dws chat +...` 入口执行；使用 `DWS_DUMP_RAW=1` 捕获同一次调用的下层 MCP 响应，避免上下层使用不同时间点的数据。
2. 读取类比较列表数量、reaction、threadId、引用消息和分页完整性；下层非空而上层为空视为投影失败。
3. 写入类使用 DWS 创建的临时群、消息、分类、角色和 Pin 场景；可逆设置恢复原值，临时普通群和内部群已解散。
4. 无法安全造数的第二成员、企业机器人和 Webhook 场景标记 `BLOCKED_FIXTURE`，不伪造 pass。
5. 所有 57 个写 Shortcut 额外执行 `--dry-run --yes`，确认写工具未被真正调用且参数形状正确。
6. 使用 `dws-im-gsb-eval quick` 分别统计契约覆盖与真实执行；两者不合并为一个百分比。

## 3. 三个“合法空结果”的造数复核

| Shortcut | 造数动作 | 上层 / 下层 | 结论 |
|---|---|---:|---|
| `+chat-role-list` | 创建临时自定义角色 | 1 / 1 | 非空通过；角色已删除 |
| `+messages-list-pin` | Pin 一条带 threadId 的真实消息 | 1 / 1；threadId 1 / 1 | 非空通过；Pin 已撤销 |
| `+thread-replies` | 创建话题根消息，由真实第二成员回复后复测 | 1 / 1 | 非空通过；回复正文和 threadId 投影一致 |

因此，原来的三个空结果已全部完成实际造数复核，上层与下层都得到一致的非空结果。

## 4. 测试中发现并修复的问题

- 分类标题真实上限为 15 个 Unicode 字符；Shortcut、原子 CLI、测试 Fixture 统一校验。
- 修复按群名解析的精确匹配，避免模糊搜索选错群。
- 消息、引用消息、Pin 和 Smart 投影统一保留 `threadId`，兼容多种下层字段名。
- `+thread-replies` 以 `--thread-id` 为主参数，保留 `--topic-id` 兼容。
- `+thread-replies` 在真实第二成员回复后得到上层/下层 1/1，关闭原“合法空”疑点。
- `+chat-mute-member` 不再使用下层当前无效的 `uids`；先把 userId 解析为群成员 `openDingTalkId`，再调用 `openDingTalkIds`，已完成真实禁言和回滚。
- `+chat-set-admin` 已使用真实第二成员完成设置管理员与回滚。
- `+conversation-list-top` 将下层 `singleChat` 规范化为 `direct` / `group`，新增 `--type all|group|direct` 本地过滤；实测 all=4、direct=2、group=2。
- 两个专项免打扰 Shortcut 删除下层 Schema 不接受的冗余 `cid` 参数；修复后仍能复现原生命令的服务端 1002。
- `+chat-set-history` 使用服务端真实可用枚举并验证回滚。
- 新增 `+chat-members-list`：群名唯一解析，用户/机器人分桶，保留分页完整性。
- 新增 `+messages-resource-download`：HTTPS、相对路径、默认不覆盖、临时文件、原子发布、Content-Length 校验和 symlink 逃逸保护。
- 资源下载曾以真实 4,309,928 字节媒体完成上下层、文件大小和 checksum 一致验证；最终 symlink 加固后当前账号已无可见 mediaId，最新扫描保持 blocked，未冒充二次 live pass。
- Code Review 将 dry-run 的实际读取能力改为显式只允许 `get_`、`list_`、`query_`、`search_`、`unread_` 前缀；未知工具 fail-closed，不能因漏进写工具名单而真实执行。
- Code Review 补齐 91 项 reviewed risk，并在加载、生成和 builtin 覆盖测试中校验风险枚举及运行时一致性。
- Code Review 将 lark-cli 契约探测改为顺序执行，避免多个进程争用 discovery/auth cache 锁产生假超时。

## 5. 91 项逐条结果

说明：91 项都已逐项审阅，其中 88 项公开，3 项因实测确认当前不可执行而标记 `unavailable` 并隐藏。`schema_leaf` 表示 Shortcut 是原子能力的稳定投影，`alias_internal` 表示兼容别名，`primary_smart` / `semantic_adapter` 表示具有额外解析或编排；这些处置不单独决定可见性。

| Shortcut | 可见性 | 语义处置 | 实测状态 | 下层证据 / 阻塞原因 |
|---|---|---|---|---|
| `+at-me` | 公开 | `primary_smart` | PASS（真实读） | chat/search_at_me_message |
| `+bot-find` | 公开 | `schema_leaf` | PASS（真实读） | bot/search_bots |
| `+bot-search` | 公开 | `schema_leaf` | PASS（真实读） | bot/search_my_robots |
| `+broadcast` | 公开 | `primary_smart` | PASS（真实写并回滚） | contact/search_contact_by_key_word, chat/send_personal_message |
| `+category-add-conversation` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/add_conv_to_categories |
| `+category-create` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/create_conv_category |
| `+category-delete` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/delete_conv_category |
| `+category-list` | 公开 | `schema_leaf` | PASS（真实读） | im/list_user_define_conv_categories |
| `+category-list-conversations` | 公开 | `schema_leaf` | PASS（真实读） | im/list_conversations_by_category |
| `+category-remove-conversation` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/remove_conv_from_categories |
| `+category-rename` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/rename_conv_category |
| `+chat-add-bot` | 公开 | `schema_leaf` | BLOCKED_FIXTURE | 需要已配置企业机器人 |
| `+chat-audit-join` | 公开 | `schema_leaf` | BLOCKED_FIXTURE | 需要待审批入群申请和第二操作人 |
| `+chat-bots` | 公开 | `schema_leaf` | PASS（真实读） | bot/list_group_bots |
| `+chat-dismiss` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/dismiss_group |
| `+chat-get-by-id` | 公开 | `schema_leaf` | PASS（真实读） | im/get_conv_info_by_group_id |
| `+chat-invite-url` | 公开 | `schema_leaf` | PASS（真实读） | im/get_group_invite_url |
| `+chat-list-all` | 公开 | `alias_internal` | PASS（真实读） | im/list_my_groups_pagination |
| `+chat-list-join-requests` | 公开 | `schema_leaf` | PASS（真实读） | im/list_apply_join_group_records |
| `+chat-list-mine` | 公开 | `schema_leaf` | PASS（真实读） | im/list_owned_or_admin_groups |
| `+chat-members-get` | 公开 | `schema_leaf` | PASS（真实读） | im/list_group_member_by_ids |
| `+chat-members-list` | 公开 | `primary_smart` | PASS（真实读） | im/search_groups, chat/get_group_members, bot/list_group_bots |
| `+chat-messages` | 公开 | `primary_smart` | PASS（真实读） | chat/list_conversation_message_v2 |
| `+chat-mute` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/set_group_mute, im/set_group_mute |
| `+chat-mute-member` | 公开 | `schema_leaf` | PASS（真实写并回滚） | contact/get_user_info_by_user_id, chat/get_group_members, im/set_group_member_mute_list |
| `+chat-quit` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/quit_group |
| `+chat-remove-bot` | 公开 | `schema_leaf` | BLOCKED_FIXTURE | 需要已配置企业机器人 |
| `+chat-role-add` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/add_custom_group_role |
| `+chat-role-list` | 公开 | `schema_leaf` | PASS（真实读） | im/list_custom_group_roles |
| `+chat-role-query-user` | 公开 | `schema_leaf` | PASS（真实读） | im/query_custom_user_roles |
| `+chat-role-remove` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/remove_custom_group_role |
| `+chat-role-remove-user` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/remove_custom_user_roles |
| `+chat-role-set-user` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/set_custom_user_roles |
| `+chat-role-update` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/update_custom_group_role |
| `+chat-search` | 公开 | `schema_leaf` | PASS（真实读） | im/search_groups |
| `+chat-set-admin` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/set_group_admin |
| `+chat-set-history` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/update_show_history_msg_option, im/update_show_history_msg_option |
| `+chat-transfer-owner` | 公开 | `schema_leaf` | BLOCKED_FIXTURE | 需要可安全转让后再转回的群主目标 |
| `+chat-update-alias` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/update_user_group_alias |
| `+chat-update-icon` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/update_group_icon |
| `+chat-update-nick` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/update_group_nick |
| `+chat-update-settings` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/update_group_settings, im/update_group_settings |
| `+conversation-clear-all-red-point` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/clear_all_red_point |
| `+conversation-clear-messages` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/clear_conversation_messages |
| `+conversation-clear-red-point` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/clear_conversation_red_point |
| `+conversation-hide` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/hide_conversation |
| `+conversation-info` | 公开 | `semantic_adapter` | PASS（真实读） | chat/get_conversation_info |
| `+conversation-list` | 公开 | `schema_leaf` | PASS（真实读） | im/list_all_conversations |
| `+conversation-list-top` | 公开 | `semantic_adapter` | PASS（真实读：all=4、direct=2、group=2） | chat/list_top_conversations；支持 `--type all|group|direct` |
| `+conversation-mark-read` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/mark_message_read |
| `+conversation-mark-unread` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/mark_conversation_unread |
| `+conversation-mute` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/update_notification_off, im/update_notification_off |
| `+conversation-mute-at-all` | 隐藏（`unavailable`） | `schema_leaf` | LOWER_ERROR | 下层 1002；原生命令同样失败 |
| `+conversation-mute-red-envelope` | 隐藏（`unavailable`） | `schema_leaf` | LOWER_ERROR | 下层 1002；原生命令同样失败 |
| `+conversation-set-top` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/set_top_conversation, im/set_top_conversation |
| `+dm` | 公开 | `primary_smart` | PASS（真实写并回滚） | contact/search_contact_by_key_word, chat/send_personal_message |
| `+group-members` | 公开 | `alias_internal` | PASS（真实读） | im/search_groups, chat/get_group_members |
| `+messages-add-emoji` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/add_emoji_reaction |
| `+messages-add-text-emotion` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/add_text_emotion |
| `+messages-batch-recall-by-bot` | 公开 | `schema_leaf` | BLOCKED_FIXTURE | 需要 robotCode 和真实 processQueryKey |
| `+messages-batch-send-by-bot` | 公开 | `schema_leaf` | BLOCKED_FIXTURE | 需要 robotCode |
| `+messages-combine-forward` | 隐藏（`unavailable`） | `schema_leaf` | LOWER_ERROR | 下层 COMBINE_FORWARD_ERROR / permission；原生命令同样失败 |
| `+messages-create-text-emotion` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/create_text_emotion |
| `+messages-forward` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/forward_message |
| `+messages-forward-topic` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/forward_topic |
| `+messages-list` | 公开 | `alias_internal` | PASS（真实读） | chat/list_conversation_message_v2 |
| `+messages-list-direct` | 公开 | `alias_internal` | PASS（真实读） | chat/list_individual_chat_message |
| `+messages-list-pin` | 公开 | `schema_leaf` | PASS（真实读） | im/list_pin_messages |
| `+messages-list-unread-conversations` | 公开 | `alias_internal` | PASS（真实读） | chat/unread_message_conversation_list |
| `+messages-mget` | 公开 | `schema_leaf` | PASS（真实读） | im/list_messages_by_ids |
| `+messages-query-send-status` | 公开 | `schema_leaf` | PASS（真实读） | im/query_message_send_status |
| `+messages-read-status` | 公开 | `schema_leaf` | PASS（真实读） | im/query_msg_read_status |
| `+messages-recall` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/recall_message |
| `+messages-recall-by-bot` | 公开 | `schema_leaf` | BLOCKED_FIXTURE | 需要 robotCode 和真实 processQueryKey |
| `+messages-remove-emoji` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/remove_emoji_reaction |
| `+messages-remove-text-emotion` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/remove_text_emotion |
| `+messages-resource-download` | 公开 | `primary_smart` | PASS（真实读） | im/get_resource_download_url |
| `+messages-resource-url` | 公开 | `schema_leaf` | PASS（真实读） | im/get_resource_download_url |
| `+messages-send-by-bot` | 公开 | `schema_leaf` | BLOCKED_FIXTURE | 需要 robotCode |
| `+messages-send-by-webhook` | 公开 | `schema_leaf` | BLOCKED_FIXTURE | 需要测试 Webhook token |
| `+messages-send-card` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/create_and_send_card |
| `+messages-set-pin` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/set_pin_message |
| `+messages-set-top` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/set_top_message |
| `+messages-unset-pin` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/unset_pin_message |
| `+messages-unset-top` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/unset_top_message |
| `+messages-update-card` | 公开 | `schema_leaf` | PASS（真实写并回滚） | im/update_streaming_card |
| `+my-groups` | 公开 | `semantic_adapter` | PASS（真实读） | im/list_my_groups_pagination |
| `+search-msg` | 公开 | `primary_smart` | PASS（真实读） | chat/search_messages_by_keyword |
| `+send-to-group` | 公开 | `primary_smart` | PASS（真实写并回滚） | im/search_groups, chat/send_personal_message |
| `+thread-replies` | 公开 | `primary_smart` | PASS（真实读） | chat/list_topic_replies；真实回复后上层/下层 1/1 |
| `+unread-chats` | 公开 | `semantic_adapter` | PASS（真实读） | chat/unread_message_conversation_list |

## 6. 尚未闭环的 12 项

### 6.1 需要外部 Fixture（9 项）

| Fixture | 涉及 Shortcut |
|---|---|
| 已配置企业机器人 | `+chat-add-bot`、`+chat-remove-bot` |
| 可安全转让后再转回的群主目标 | `+chat-transfer-owner` |
| 待审批入群申请 + 第二操作人 | `+chat-audit-join` |
| robotCode | `+messages-send-by-bot`、`+messages-batch-send-by-bot` |
| robotCode + processQueryKey | `+messages-recall-by-bot`、`+messages-batch-recall-by-bot` |
| 测试 Webhook token | `+messages-send-by-webhook` |

### 6.2 下层服务错误（3 项）

| Shortcut | Shortcut 参数状态 | 原生复现 | 下层结果 |
|---|---|---|---|
| `+conversation-mute-at-all` | 已与 live MCP Schema 精确一致 | 是 | IM 1002 |
| `+conversation-mute-red-envelope` | 已与 live MCP Schema 精确一致 | 是 | IM 1002 |
| `+messages-combine-forward` | 已使用两条真实消息和不同目标群 | 是 | `COMBINE_FORWARD_ERROR / permission` |

完整错误码、时间和 traceId 见 [下层服务问题协查单](im-shortcut-lower-service-issues.md)。

## 7. 工程门禁

- 88 个公开 Shortcut 的逐命令 `--help`：88 / 88 成功；3 个 `unavailable` 入口不出现在 Shortcut Catalog 或 `dws chat --help` 的公开列表中。
- Code Review 后 57 个写 Shortcut 的真实 Cobra `--dry-run --yes`：57 / 57 成功，未产生写 MCP 响应。
- `dws-im-gsb-eval quick`：DWS 197 / 197、lark-cli 55 / 55，无 missing/stale expectation。
- `DWS_PACKAGE_VERSION=0.0.0-test GOMAXPROCS=1 GOGC=20 GOMEMLIMIT=384MiB go test -p 1 ./...`：通过。
- `go vet ./...`：通过。
- `GOMAXPROCS=1 GOGC=20 GOMEMLIMIT=512MiB go build -p 1 -o /tmp/dws-im-shortcut-review ./cmd`：通过。仓库已有 `cmd/` 目录，不能使用默认输出名执行裸 `go build ./cmd`。
- `check-generated-drift.sh`：通过。
- `check-schema-catalog.sh`：通过，26 products / 813 tools。
- `git diff --check`：通过。

## 8. 证据文件

- 本地脱敏读取审计：`tmp/im-shortcut-live-audit/nonempty-fixtures-rerun-20260728/chat-read-live-audit.md`
- 本地脱敏写入审计：`tmp/im-shortcut-live-audit/write-live-two-message-20260728/chat-write-live-audit.md`
- 本地 57 项 Code Review 后 dry-run 审计：`tmp/im-shortcut-live-audit/write-dry-run-review-20260728/chat-write-dry-run-audit.md`
- [IM GSB Query 集与 197/197 契约复核结论](dws-im-gsb-core-query-set.md#101-2026-07-28-契约复核结果)
- [3 个下层服务问题协查单](im-shortcut-lower-service-issues.md)
- [IM 优化设计](im-optimization-design.md)
- [IM GSB Query 集](dws-im-gsb-core-query-set.md)
