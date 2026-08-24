---
category: Security
---

- **DWS OpenAPI 逃生舱** ([Aone #84603971](https://project.aone.alibaba-inc.com/v2/project/2125919/req/84603971)) — 保持 `dws api <METHOD> <PATH>`、五种 HTTP method、既有 flags/defaults、App Token 缓存、双域名 Token 注入、原始成功 JSON 与分页 page 数组兼容；新增 `--params/--data @file`、单文件流式 `--file [field=]path` multipart、camelCase 分页字段和基于官方 `open.dingtalk.com/llms.txt` 的 misc/mono Agent 发现指南。Client ID/Client Secret 只按完整 flags、完整环境变量或完整 app config 成对解析，禁止跨来源拼接；Raw API 的单次 flags/env 不持久化 AppSecret，成功 OAuth 登录则持久化其实际使用的完整 pair。Client Secret 统一到 `appsecret:<clientID>`，自动迁移历史明文及 `client-secret:` 槽位，新旧值冲突时 fail closed；内部不再回填 `DWS_CLIENT_ID/DWS_CLIENT_SECRET`。安全收紧包括拒绝 HTTP、非 443 端口、跨域或 HTTPS 降级重定向，安全化服务端下载文件名，并对 JSON/错误响应执行大小上限、对二进制执行流式临时文件原子替换。
