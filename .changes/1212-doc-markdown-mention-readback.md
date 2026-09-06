---
category: Fixed
---

- **markdown @人 写后回读误报** — 写入含 `[@姓名](alidocs-mcp://doc/mention?openDingTalkId=…)` 的 markdown 时，`doc +create` 与 `doc +update --command append|overwrite` 会以 `doc_write_verification_failed` 报错，而内容其实已正确写入。原因是写后回读把写入原文与服务端改写后的正文比对，而服务端会把该私有协议改写成钉钉个人资料链接。现在写后回读改为**按位置配对**：只有预期正文中写了 mention 私有协议的那个位置，才允许回读侧是个人资料链接；其余链接——包括作者自己写的普通个人资料链接——仍保留完整目标并严格比对。显示文本与节点顺序照旧参与比对，漏写、改标签或顺序错乱依旧判定失败。原子命令 `doc update` 无写后回读，行为不变。已知限制：同显示文本的多个 mention 若目标互换，本地无法判别（`openDingTalkId` 与改写后的 `staffId` 不同值且无本地映射）。
