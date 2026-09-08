# Chat 最终二进制真实验收记录

2026-09-08 · 分支 `codex/chat-cli-parity-20260908`。39 个检查，38 通过、1 清理终态未确认。功能断言均通过。

| 检查 | 状态 | 核实方式或边界 |
|---|---|---|
| create | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| self-only | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| rename-alias | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| text | PASS | terminal message ID, not task receipt alone |
| text-read | PASS | exact message ID + conversation + self author + unique body |
| markdown | PASS | terminal message ID, not task receipt alone |
| markdown-read | PASS | exact message ID + conversation + self author + unique body |
| at-self | PASS | terminal message ID, not task receipt alone |
| at-self-read | PASS | exact message ID + conversation + self author + unique body |
| at-all | PASS | terminal message ID, not task receipt alone |
| at-all-read | PASS | exact message ID + conversation + self author + unique body |
| edit-auto-context | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| edit-readback | PASS | exact message ID + conversation + self author + unique body |
| edit-wrong-scope | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| edit-unchanged | PASS | exact message ID + conversation + self author + unique body |
| thread-created | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| thread-root | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| thread-read | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| favorite-dedup | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| favorite-thread-read | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| unfavorite | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| favorites-restored | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| history-all-written-IDs | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| group-member | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| group-manager | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| group-group | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| group-topic | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| group-create-sort | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| search-at-self | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| read-user-dedup | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| resource-exact-bytes | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| cleanup-scope | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| cleanup-thread | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| cleanup-at-all | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| cleanup-at-self | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| cleanup-markdown | UNRESOLVED | 撤回超时；之后消息未返回，群解散后重试为业务码11056。缺少终态证据，不能判成功。 |
| cleanup-text | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| cleanup-group | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |
| cleanup-group-absent | PASS | 对应原始调用与断言保存在私有证据；采用准确资源 ID、上下文或字段校验。 |

两个自用测试群均已解散；最终群查询准确空集合且 complete=true。所有原始业务 ID、正文、请求与响应仅在本地受控证据目录。

公开 CLI 调用均带最终二进制 SHA256；记录数量与场景数量不同，发送后轮询、独立读取和对照请求分别保留。

[完整实施报告](implementation.md)列明参数、先前验证与未覆盖条件。
