# 受控渠道与阿里巴巴组织登录

## 使用场景

在以下任一场景读取本参考：

- 目标组织是阿里巴巴；
- 登录返回 `CHANNEL_REQUIRED`、`channel_not_allowed`、`enterprise_not_authorized` 或“应用暂不受信任”；
- 用户提到渠道码、`DWS_CHANNEL`、渠道白名单或渠道归因；
- 需要判断 `DWS_CHANNEL` 与 `DINGTALK_DWS_AGENTCODE` 的边界。

## 核心契约

- 将 `DWS_CHANNEL` 作为产品/分发渠道 `channelCode`。CLI 在登录权限检查和后续 MCP 请求中把它发送为 `x-dws-channel`。
- 将 `DINGTALK_DWS_AGENTCODE` 作为执行 Agent 身份。两者是独立维度，禁止互相回填或复用。
- 在受控渠道组织中，把 `DWS_CHANNEL` 同时加到 `auth login` 和每一条后续 `dws` 命令。只在单条命令作用域设置，禁止写入 shell profile 或对其他组织全局导出。
- 仅使用与真实宿主/业务场景匹配的已登记渠道。禁止为了通过登录随机尝试其他渠道或伪装成别的产品。
- 把静态 `channelCode` 视为公开路由标识，不视为密钥或可信归因凭证。长期方案必须由服务端校验宿主身份并签发短期、绑定组织和渠道的会话凭证。

## 当前本机映射

当前 Codex 中的 DWS skill/办公能力验证场景使用：

| 项目 | 值 |
|---|---|
| 目标组织 | 阿里巴巴 |
| 稳定 profile | `dingd8e1123006514592:04061459256343` |
| 渠道 | EI智能体评测 |
| `DWS_CHANNEL` | `18451e165920b301ade00efae99b2c253e1e900b` |

登录：

```bash
DWS_CHANNEL='18451e165920b301ade00efae99b2c253e1e900b' \
  dws auth login \
  --profile 'dingd8e1123006514592:04061459256343' \
  --format json
```

后续命令：

```bash
DWS_CHANNEL='18451e165920b301ade00efae99b2c253e1e900b' \
  dws <product> <command> \
  --profile 'dingd8e1123006514592:04061459256343' \
  --format json
```

2026-07-22 已用 CLI v1.0.54 验证：不带渠道码时阿里巴巴组织登录被拒；带上述渠道码后 OAuth 登录成功，随后 `minutes list all` 调用成功。

## 组织白名单

当前服务端渠道配置组织白名单：

```text
793652894
515819978
21001
```

该数字组织白名单属于服务端控制面，不等同于本地 `profile` 中的字符串 `corpId`。禁止自行推导两者的映射。

## 排查顺序

1. 运行 `dws profile list --format json`，解析目标组织的稳定 `profile`。
2. 按真实宿主从登记表选择渠道；当前本机 Codex 规则命中“EI智能体评测”。
3. 使用命令级 `DWS_CHANNEL` 重新执行 `dws auth login --profile ... --format json`。
4. 使用相同 `DWS_CHANNEL` 和 `profile` 执行一个最小只读产品命令验证。
5. 若仍失败，加 `--verbose` 重试一次并按原始服务端错误分类；禁止轮询尝试整张渠道表。
