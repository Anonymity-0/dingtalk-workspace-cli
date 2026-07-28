# MCP 服务与工具管理

命令入口：

```bash
dws connector mcp --help
dws connector mcp service --help
dws connector mcp tool --help
dws connector mcp url --help
```

## 领域模型

这套能力分成两层，不要混用：

- `mcpdev` 管理面：`dws connector mcp service/tool/url ...`，用于创建、编辑、调试、发布 MCP 服务和工具。
- 已发布 MCP 调用面：服务发布后，DWS 通过发现接口或真实接入地址的 `tools/list` 生成固定 DWS 动态命令，同时保留 `dws connector mcp published <service-or-tool> <tool>` 调试路径。

| 对象 | 主键 | 说明 |
|------|------|------|
| MCP 服务 Service | `mcpId` | 工具容器，承载服务名称、描述、图标、详情介绍和接入地址 |
| MCP 工具 Tool | `toolId` | 服务下的单个工具（G-ACT- 开头；平台底层字段名 actionId，0714 契约起工具入参统一叫 toolId），包含工具身份、HTTP 定义、LLM 入参出参和映射规则 |
| 工具版本 Version | `versionId` | 草稿、发布版或历史版本；调试草稿、回滚验证时必须明确版本 |
| 工具定义 Definition | 无独立主键 | 三段式定义：`apiInputs/apiOutputs`、`toolInputs/toolOutputs`、`inputMappings/outputMappings` |
| 调试记录 Debug | 无稳定主键 | 用一组 `value` 真实执行工具；写操作必须用测试数据 |
| 接入地址 Endpoint | `mcpURL` / `mcpJSON` | 客户端配置入口，可能含敏感 `?key=`；只脱敏展示 |
| DWS 动态命令 | 命令路径 | 由已发布 MCP 的 `tools/list` 投影成 `dws <service-or-tool> <tool>`；同时保留 `dws connector mcp published <service-or-tool> <tool>` |

对象关系：

```text
MCP Service (mcpId)
├── service metadata
├── AuthConfig（下游鉴权说明书，不含凭证值）
├── Credential（凭证账号，密钥不回显）
├── Member（开发协作者 staffId）
├── Endpoint(PUBLISHED / MARKET)
└── Tool(toolId)
    ├── identity: name / title / description / status
    ├── HTTP adapter: method / url / auth
    ├── Definition
    │   ├── apiInputs / apiOutputs       # 真实 HTTP 参数和响应
    │   ├── toolInputs / toolOutputs     # 暴露给 LLM 的参数和响应
    │   └── inputMappings / outputMappings
    └── Version(versionId, versionNo, status)
```

工具生命周期：

```text
service create
  → tool create/update
  → draft
  → tool debug --version-id <draftVersionId>   # 真跑验证，返回真实业务数据才算通过
  → tool publish                                # publish = 企业内可用（≠ 上架市场）
  → published
  → url get --source PUBLISHED
  → connector mcp refresh
  → <service-or-tool> <tool>
  → connector mcp published <service-or-tool> <tool>
```

`published_with_draft` 是最容易误判的状态：它表示线上仍是旧发布版，同时有一份新草稿。不传 `--version-id` 调试时会命中线上旧版本；要验证正在编辑的内容，必须从 `tool get` 取草稿 `versionId` 后传给 `tool debug`。

### 三段式工具定义

`tool create` / `tool update` 用同一套三段式入参。`tool update` 是全量提交语义，漏传字段等于清空。

| 段 | 作用 | 例子 |
|----|------|------|
| `apiInputs` / `apiOutputs` | HTTP 接口真实参数和响应字段 | `query.wd`、`headers.User-Agent`、`body.items` |
| `toolInputs` / `toolOutputs` | 暴露给 LLM 的参数和响应字段，可裁剪、改名、防呆 | `keyword`、`city`、`raw` |
| `inputMappings` / `outputMappings` | LLM 字段和 HTTP 字段之间的映射 | `$.node_start.keyword` → `$.Query.wd` |

字段结构统一用 `{key,title,type,required,description,children}`（type：string/number/boolean/object/array；object/array 的子结构放 `children` 递归；**array 字段的 items 不能为空**）。

- `toolInputs` 的 `key` 对应 `inputMappings.source`（`$.node_start.<key>`）；`apiInputs` 的字段位置对应 `inputMappings.target`（`$.<位置>.<key>`）。
- **平台不支持 enum/default/example 等 JSON Schema 标准属性**——枚举、默认值、示例一律写进字段 `description` 文本。
- create/update 必填集（0720 第六刀终态）：`mcpId / name / title / description / --http-info / --api-inputs / --tool-inputs / --input-mappings` + **出参三件套 `--api-outputs / --tool-outputs / --output-mappings`** 全部必填（create 11 项、update 12 项；`--tool-outputs` 不做精修显式传 `[]`）。CLI 已硬拦缺项；平台运行时不强拦，但**漏传 apiOutputs 的故障形态**＝工具建成且调用看似成功、下游复杂结构字段被精确裁剪整段吞掉、UI 出参映射标「变量已失效」。`toolType` 参数已移除（多传被静默忽略）；旧键 `http` 仍报 `business_error_invalid_params`。
- 工具 `description` 有 DB 列长限制（约 700 字符）：`tool create/update` 草稿不报错，**`tool publish` 才报 Data too long**——描述超长要在发布前收敛。

映射规则的完整格式（JSONPath 写法、Pascal 位置名、reference/fixed/express 三型、出参透传、系统参数、数组双规则）见 **[mapping-rules.md](mapping-rules.md)**，写 `--input-mappings` / `--output-mappings` 前必读。速记三条最大的坑：

- target 位置名必须 Pascal（`$.Query.*` / `$.Body.*`），全大写/全小写**静默失效**；
- `express` 类型的表达式必须放 `expression` 字段（不是 `source`），放错被静默存成 `{}`；
- `outputMappings`（必填）二选一：整体透传 `{"type":"reference","source":"$.node_service_activator.Body","target":"$"}`，或字段级精修（裁剪/改名/[*]/嵌套/系统变量，见 mapping-rules.md §5）。⚠️出参 rules 的 source 必须在 apiOutputs 声明范围内，否则 UI 标「变量已失效」（红线#13）。（直连脚手架的旧调用方注记：平台运行时不强拦省略/[]，行为=返回 `{"Body":{整包响应体}}` 包壳——勿依赖。）
- `corpId` / `userId` 等调用者身份由系统参数注入（`$.system_node.*`），不要做成 toolInput 让 LLM 传。

### HSF 型工具（V5：方法发现 → create-hsf → update-hsf 部分更新）

与 http 型的关键差异（其余 toolInputs/inputMappings/toolOutputs/outputMappings 同名同构）：

- **apiInputs/apiOutputs 无需提供**——服务端按 HSF 方法 schema 自动生成（读回照常有）；
- `--hsf-info`＝三元组 `{interfaceName, methodName, version(可省，缺省1.0.0)}`，与 http 版 `--http-info` 对仗；
- **建前必跑 `hsf method-list`**：返回每方法的 `inputSchema`/`outputSchema`——inputMappings 的 DTO 字段名**唯一权威来源**（字段名写错=静默忽略不报错）、出参映射 source 的依据；
- **身份注入必须显式写两条映射**（服务端不自动补，漏写=运行时参数错误）：`{"target":"$.<DTO简名>.corpId","type":"reference","source":"$.system_node.ddDataCorpId"}` 与 `{"target":"$.<DTO简名>.userId","type":"reference","source":"$.system_node.operateUserId"}`；
- 映射语法差异：入参 target=`$.<DTO简名>.<字段>`（如 `$.SearchMCPRequest.name`，**不是** http 的 Pascal 位置名）；出参 source **无 `.Body.` 前缀**：`$.node_service_activator.result…`（写操作类最简=只透 success/errorCode/errorMsg 三条）；
- **update-hsf＝部分更新**：只传要改的字段，未传保持原值——与 http 版全量提交完全相反；⚠️`--hsf-info` 的 interfaceName 与 methodName 必须同时给才会切换方法（只给其一被静默忽略）；⚠️`--output-mappings` 传 `[]`＝清空既有映射（勿用 [] 表达「不改」）；
- timeout/only-original-keys 两参数（http/hsf 通用，可选）：timeout 1-180 秒缺省 10，**一旦设置无法清回默认**（update 漏传=保留原值、传 0 被拒）；only-original-keys 开启后出参只含下游实际返回的 key（无 null 占位），update 侧漏传=保留原值、要关闭须显式传 false；
- 仅对开通灰度的组织可用（未开通报 no_permission）。

### 从 API 材料到工具定义

用户给 API 材料（OpenAPI/Postman/curl/文档）要求「做成 MCP / 给 agent 用」时，按 **[api-to-tool.md](api-to-tool.md)** 拆解：信息对齐（材料/业务目标/鉴权，缺就问）→ 按材料类型提取三段式 → 工具侧加工（裁剪/改名/约束写 desc/身份走注入）→ 一个语义动作一个工具 → 设计整表给用户过目再动手建。只读接口建议先真跑一次取真实响应（反推 apiOutputs + 生成 debug 测试入参）。

### 发现与动态命令

动态命令不是 mcpdev 管理命令。它来自已发布 MCP 的 `tools/list`，由 DWS 投影成固定命令：

```text
dws <service-or-tool-slug> <tool-slug> [flags]
```

一级命令命名优先级：合法 ASCII `serverName` → `mcp-<mcpId>` → 工具 `name`。中文服务名和非法 `serverName` 不进入命令路径；旧缓存中的中文一级路径会自动迁移为 `mcp-<mcpId>`。DWS 同时保留调试路径：

```text
dws connector mcp published <service-or-tool-slug> <tool-slug> [flags]
```

刷新命令：

```bash
dws connector mcp refresh --format json
dws <service-or-tool-slug> --help
dws connector mcp published --help
```

缓存规则：

- DWS 会缓存已发布 MCP 的工具描述，TTL 10 分钟。
- 动态命令的 `dws schema <service>.<tool>` 只读取同一份命令缓存，不实时调用发现接口或 `tools/list`；没有缓存时动态命令不注册，残留命令缺少 schema 缓存时明确返回不可用。
- 静态管理命令的 schema（如 `connector.mcp.service.create`）使用其原有 schema 来源，不受动态命令缓存是否存在影响。
- `refresh` 会主动拉取预发/线上 Portal 发现接口并重建缓存；`--timeout` 同时作为发现请求和单个 MCP `tools/list` 的超时。
- MCP 工具发现使用有限并发和独立超时：单个服务失败不会取消后续服务；健康服务更新，失败服务保留上次缓存。
- 返回中的 `partial` 表示部分成功，`failedServices` 给出失败的 mcpId/脱敏端点/原因，`cacheUpdated` 表示合并后的缓存已持久化。
- 发布或更新工具后**最迟 10 分钟（缓存 TTL）自动生效**——「立即出现」只是缓存恰好过期的巧合，不是保证；需要立即可用就执行 `refresh`，再看 `published --help`。
- `service list` 返回顶层 `services[].serverName`（V4 起分页平铺，无 result 信封）；它是 kebab-case CLI 一级命令名，未设置时为空。动态发现必须优先使用该字段，不能始终从中文服务名推导。
- 所有刷新只使用发现接口返回的真实 hash `mcpUrl`；不会根据 `mcpId` 猜 `/server/org-{mcpId}`。需要找回地址时使用 `url get --source PUBLISHED`。

指定接入地址的只读探测：

```bash
DINGTALK_MCPDEV_MCP_URL='<含凭证的 MCP 地址>' dws connector mcp inspect --format json
```

`inspect` 依次执行 `initialize`、`notifications/initialized`、`tools/list`，返回协商协议版本、服务能力和完整工具 Schema，不调用任何业务工具。优先通过环境变量传入含 `?key=` 的地址，避免凭证出现在命令参数中；输出中的地址会自动脱敏。

### 鉴权、凭证与成员

```bash
# 服务级下游鉴权配置
dws connector mcp auth get --mcp-id <mcpId> --format json
dws connector mcp auth save --mcp-id <mcpId> --auth-type NO_AUTH --dry-run --format json
dws connector mcp auth save --mcp-id <mcpId> --auth-type TOKEN --token-auth-config '<JSON>' --dry-run --format json

# 凭证账号：密钥优先从文件或 stdin 读取，避免进入 shell history
dws connector mcp credential list --mcp-id <mcpId> --format json
dws connector mcp credential get --mcp-id <mcpId> --credential-id <id> --format json
dws connector mcp credential save --mcp-id <mcpId> --name <账号名> --content-file credentials.json --dry-run --format json
dws connector mcp credential debug --mcp-id <mcpId> --credential-id <id> --dry-run --format json
dws connector mcp credential bind --mcp-id <mcpId> --credential-id <id> --dry-run --format json
dws connector mcp credential unbind --mcp-id <mcpId> --dry-run --format json
dws connector mcp credential delete --mcp-id <mcpId> --credential-id <id> --dry-run --format json

# 开发协作者，--user-ids 传 staffId，不传姓名
dws connector mcp member list --mcp-id <mcpId> --format json
dws connector mcp member add --mcp-id <mcpId> --user-ids <staffId1,staffId2> --dry-run --format json
dws connector mcp member remove --mcp-id <mcpId> --user-ids <staffId1,staffId2> --dry-run --format json
```

#### 鉴权方式选型（先选对类型再动手）

| 下游接口要什么 | 选型 |
|---|---|
| 无鉴权 | `NO_AUTH` |
| 用户名+密码（HTTP Basic） | `BASIC`（运行时注入 `Authorization: Basic base64(user:pass)`） |
| **静态 API key**（key 原文放 query 或 header，如 `?api_key=xxx`） | **`SIGNATURE` 自定义字段 + 直引**（样本见下） |
| 动态换 token（先拿 key/secret 换 access_token 再调业务接口） | `TOKEN`（注入位 authHeaders/authQuery/authBody 三选一 + 失效自动刷新） |
| 自定义签名算法（时间戳/摘要等需表达式计算） | `SIGNATURE` 自定义字段 + funcValue 表达式 |

**静态 API key（query 位）实测样本**（真调 NASA APOD 通过）：

```json
{"signatureAuthConfig": {
  "authFields": [{"dataId":"apiKey","description":"API Key","type":"password","required":true}],
  "authQuery": [{"key":"api_key","type":"authField","value":"#(\"apiKey\")"}],
  "testRequest": {"method":"GET","url":"https://<业务接口>"}
}}
```

key 放 header 时把 `authQuery` 换成 `authHeaders`（key=<头名>）；凭证 content=`{"apiKey":"<真实key>"}`。

> **⚠️ 建带鉴权 MCP 的顺序铁律（实测踩坑）**：`auth save`（配鉴权）→ `credential save`（存凭证）→ **`credential bind`（绑定生效凭证）** → 建工具 → **`tool publish`（发布）**。**凭证必须在 `tool publish` 之前 bind**——若先 publish 后 bind，发布实例会卡在**草稿态（DRAFT）**：`url get` 恒返回 `success:true` 但**无 mcpUrl**，且**事后再 bind 也救不回**，只能重建工具按正确顺序重发布。无鉴权（NO_AUTH）服务无此约束，建好发布直接可取 url。

- auth save 存的是「说明书」（authFields 字段定义、换取与注入方式），不含密钥值；按选型只传对应类型的配置对象。
- TOKEN 型注入位按下游要求三选一：`authHeaders`（token 放请求头）/ `authQuery`（token 放 query 参数）/ `authBody`。**query 位完整模板**（已实测跑通，按下游实际字段替换占位符）：

```json
{
  "authFields": [{"dataId":"appKey","type":"string","required":true},{"dataId":"appSecret","type":"password","required":true}],
  "fetchTokenRequest": {"method":"GET","url":"https://<换token接口>","query":[
    {"key":"<换token入参key名>","type":"authField","value":"#(\"appKey\")"},
    {"key":"<换token入参secret名>","type":"authField","value":"#(\"appSecret\")"}]},
  "authQuery": [{"key":"<业务接口的token参数名>","type":"text","value":"$.Body.<token在换token响应中的字段路径>"}],
  "tokenExpireRules": [{"func":"EQ(${@(\"Body/<错误码字段>\")},\"<token失效错误码>\")"}],
  "refreshToken": true,
  "testRequest": {"method":"POST","url":"https://<业务接口>"}
}
```

  要点：`authQuery[].value` 用 `$.Body.<字段>` 引用换 token 响应；失效判据按下游真实返回写——HTTP 状态码型用 `${@("$/statusCode")}`，业务错误码型用 `${@("Body/<字段>")}`。header 位注入同结构，把 `authQuery` 换成 `authHeaders` 即可。
- `credential save` 保存的是**开发者内置凭证**：归属「当前用户 + 当前 MCP」，密钥由开发者提供（不是使用者的凭证），配置调试与实例运行时两个场景都用它。`content` key 必须与 `auth get` 返回的 `authFields[].dataId` 一致；密钥不会回显，dry-run 也只显示脱敏占位。TOKEN 型 save 会真实调用换 token 接口验证密钥（密钥无效则 save 整体失败），其他类型纯落库。
- `credential debug` 会真实调用鉴权配置中的 `testRequest`，返回平铺结构 `{executeSuccess, executeErrorMsg, detail}`（V4 起 detail 在顶层）；**外层 success=true 只代表流程走完，不代表鉴权通过**——必须解析顶层 `detail.response`（statusCode 2xx 且业务无 errcode 才算真通过），由 AI 判读后给出明确结论，拿不准时人工复核。
- `credential bind` 只是「选用」：**仅对正式实例运行时生效**。`tool debug` 不使用 bind 绑定的凭证——调试带鉴权的工具必须在 `tool debug` 显式传 `--credential-id`，不传则按无凭证直调（TOKEN 注入配置原文、BASIC 静默不注入，报错全是误导性假症状）。
- 删除已绑定的凭证会被平台拦 `credential_in_use`（V5 护栏）——先 `credential unbind` 解绑再删；unbind 幂等（已配鉴权但未绑定时也返回 success），⚠️服务从未配置鉴权/已恢复 NO_AUTH 时返回 NOT_FOUND「MCP不存在」——实为实例凭证配置记录不存在，并非服务不存在，勿被文案误导。移除成员前先 `member list` 核对；删除类都必须获得明确确认。

## Shortcut

### 从 API 材料一键建 MCP（终端用户最高频）

```text
1. 信息对齐：API 材料 / 业务目标 / 鉴权方式，缺就问
2. 按「从 API 材料到工具定义」拆三段式，映射按 mapping-rules.md 写
3. 设计整表给用户过目（工具清单/入参/映射/测试入参）
4. service create --dry-run → 确认 → --yes，记录 mcpId
5. 先建最简单的一个工具：tool create → tool get 读回核对（rules 位置名/条数）
6. 结构没问题再建其余；每建一个 tool list 反查一个，失败即停
7. 逐个 tool debug（草稿传 --version-id）：校验返回真实业务数据
8. debug 失败 → 对照 mapping-rules.md 修映射 → tool update → 再 debug（最多 2 轮，仍失败升级用户）
9. 用户明确确认后逐个 tool publish --yes
10. url get --source PUBLISHED 取接入地址（?key= 脱敏）
11. connector mcp refresh → 确认动态命令出现 → 真实调用验证一次
```

### 从零创建并发布一个只读 HTTP 工具

```text
1. service create --dry-run
2. service create --yes
3. tool create --dry-run
4. tool create --yes
5. tool get 或 tool list 取 toolId/versionId
6. tool debug --version-id <草稿versionId> --dry-run
7. tool debug --version-id <草稿versionId> --yes
8. tool publish --dry-run
9. 用户明确确认“发布后使用方可调用”
10. tool publish --yes
11. tool get 回读状态
12. url get --source PUBLISHED 获取接入地址
13. connector mcp refresh --format json
14. <service-or-tool> --help 和 connector mcp published --help 确认动态命令出现
```

### 只更新一个已有工具草稿

```text
1. tool list --mcp-id <mcpId> 找 toolId
2. tool get --mcp-id <mcpId> --tool-id <toolId> 读取当前定义和草稿 versionId
3. tool update --dry-run（全量提交，别漏字段）
4. tool update --yes
5. tool get 取新草稿 versionId
6. tool debug --version-id <草稿versionId> --yes
7. 用户确认后 tool publish --yes
```

注意（V5 起）：第 2 步**读回即三段式**——`tool` 对象含 `toolType`（http/hsf）、`httpInfo`/`hsfInfo`、`timeout`/`onlyOriginalKeys` 与六件套（apiInputs/toolInputs/inputMappings/apiOutputs/toolOutputs/outputMappings），可直接在读回基础上修改后提交（旧版 4 个 *Json 存储串已裁撤，无需再做逆向翻译）。**判型看 toolType 再选 update 工具**：http 型走 `tool update`（全量提交，漏传即清空；仅 timeout/only-original-keys 漏传=保留原值）；hsf 型走 `tool update-hsf`（部分更新，只传要改的）——hsf 工具误走 http 版被平台拒（hsf_tool_update_via_http）。

### 只验证线上工具

```text
1. tool list --mcp-id <mcpId> 找 toolId
2. tool get 读 toolInputs，构造真实测试入参（如 '{"city_name":"北京"}'；不要传空 {} 走过场）
3. tool debug --mcp-id <mcpId> --tool-id <toolId> --value '<测试入参JSON>' --dry-run
4. 确认后 tool debug --mcp-id <mcpId> --tool-id <toolId> --value '<测试入参JSON>' --yes

带鉴权的工具必须加 --credential-id <id>（debug 不使用 bind 绑定的凭证）。
```

线上验证可以不传 `--version-id`；草稿验证必须传。

### 发布后生成 DWS 动态命令

```text
1. tool publish --yes 成功
2. url get --mcp-id <mcpId> --source PUBLISHED 确认服务已有真实接入地址
3. connector mcp refresh --format json
4. <service-or-tool> --help 和 connector mcp published --help
5. 优先使用 <service-or-tool> <tool> --format json 调用；必要时使用 connector mcp published <service-or-tool> <tool> --format json 调试
```

如果 `refresh` 返回 `count=0` 或 `published --help` 没有子命令，先确认 Portal 发现接口是否返回该企业已发布 MCP；不要改用猜测的 `org-{mcpId}` 地址。

### 找回接入地址

```text
1. service list --keyword <服务名关键词> 找 mcpId
2. url get --mcp-id <mcpId> --source PUBLISHED
3. 若服务已上架市场，改用 --source MARKET
```

返回值含 `?key=`，回答里只说“已获取”或脱敏展示。

### 跨会话续作

```text
1. service list --keyword <服务名关键词> --format json
2. tool list --mcp-id <mcpId> --page-size 100 --format json
3. tool get --mcp-id <mcpId> --tool-id <toolId> --format json
4. 根据 status 决定调试草稿、验证线上、发布或更新
```

禁止凭记忆使用 `mcpId` / `toolId` / `versionId`。

### 故障定位

```text
1. schema connector.mcp.<leaf> 看当前 MCP tools/list 暴露的参数说明
2. 目标命令 --help 看当前二进制 flag
3. 加 --dry-run 看实际 tools/call 参数
4. 加 --verbose 重试一次获取 trace_id / technical_detail
5. service get / tool get / tool versions 回读平台状态
```

常见定位结论：

- `mcp_not_found`：当前 mcpdev endpoint 或当前登录组织下没有该 `mcpId`。
- `mcp_id_initializing`（MCP 数据初始化中）：存量数据补齐中，**等 10 秒重试一次**即可。
- `no_draft_to_publish`：没有可发布草稿，先 `tool update` 或确认 toolId。
- `business_error_invalid_params`：常见于旧契约调用——工具入参用了旧键 `actionId`（应为 `toolId`），或 create/update 用了旧键 `http`（应为 `httpInfo`）。升级 dws 后 CLI flag 对应 `--tool-id`/`--http-info`。
- `tool_already_listed_in_market`：已上架市场的工具不允许删除，需先在市场下架。
- 调试「成功」但接口报缺参/返回空：映射静默失效——位置名大小写（须 Pascal）/ express 用了 `source` 字段（须 `expression`）/ 漏映射，见 mapping-rules.md。
- 调试通过但发布后调用旧逻辑：调试时漏传草稿 `versionId`，实际测了线上旧版本。
- 接入地址 `tools/list` 报 `-32603 [PARAM_ERROR] 参数不能为空`、`tools/call` 返回 `not found the specified tool` 且 `isError=false`：**真实根因大概率是该服务 0 个已发布工具**（全是 draft）——`url get --source PUBLISHED` 在 0 发布时仍返回完整 URL（平台缺口 Aone 84799867）；先 `tool list` 核对发布数，不要朝参数方向排查。
- `service list` 偶发返回空列表：索引瞬态，重试一次即可。
- 动态命令没出现：发现接口未返回 MCP，或接入地址 `tools/list` 不可访问。

## 服务

```bash
# 列服务 / 查服务
dws connector mcp service list --keyword <关键词> --format json
dws connector mcp service get --mcp-id <mcpId> --format json

# 创建 / 修改 / 删除服务
dws connector mcp service create --name <服务名> --server-name <kebab-case> --description <描述> --dry-run --format json
dws connector mcp service update --mcp-id <mcpId> --description <新描述> --dry-run --format json
dws connector mcp service delete --mcp-id <mcpId> --dry-run --format json
```

服务名用业务语义命名且组织内唯一，不要用 test、临时 等占位名。删除服务不可恢复，删除前先 `service get` 和 `tool list` 核对；服务下还有工具时会被拒绝。

列表类命令返回**顶层平铺**分页（V4 起，无 result/list 信封）：语义化数组 + 分页字段同级——`service list`→`services[]`、`tool list`→`tools[]`、`tool versions`→`versions[]`、`credential list`→`credentials[]`，旁边是 `hasMore/nextCursor/totalCount`。用 `hasMore` 判断翻页，不要用「返回条数==pageSize」启发式。

所有工具出参的通用行为（V4 出参映射）：**值为 null 的字段会被映射引擎整个省略**，不会出现 `"字段": null`——出参里缺某字段＝该值为空，不代表工具异常；判断成败一律看 `success`/`errorCode`。

## 工具

```bash
# 列工具 / 读工具 / 版本历史
dws connector mcp tool list --mcp-id <mcpId> --page-size 100 --format json
dws connector mcp tool get --mcp-id <mcpId> --tool-id <toolId> --format json
dws connector mcp tool versions --mcp-id <mcpId> --tool-id <toolId> --format json

# 创建 / 更新工具：复杂字段使用 JSON 字符串
dws connector mcp tool create --mcp-id <mcpId> --name <snake_case_name> --title <标题> --description <描述> --http-info '{"method":"GET","url":"https://example.com","auth":{"type":"NO_AUTH"}}' --api-inputs '{...}' --tool-inputs '[...]' --input-mappings '[...]' --api-outputs '{...}' --tool-outputs '[]' --output-mappings '[{"target":"$","type":"reference","source":"$.node_service_activator.Body"}]' --dry-run --format json
dws connector mcp tool update --mcp-id <mcpId> --tool-id <toolId> <同 create 全量字段（0720 起含出参三件套）> --dry-run --format json

# 调试 / 发布 / 删除（value=符合 toolInputs 的测试入参，不要传空 {} 走过场）
dws connector mcp tool debug --mcp-id <mcpId> --tool-id <toolId> --version-id <versionId> --value '{"city_name":"北京"}' --credential-id <credentialId> --dry-run --format json
dws connector mcp tool publish --mcp-id <mcpId> --tool-id <toolId> --dry-run --format json
dws connector mcp tool delete --mcp-id <mcpId> --tool-id <toolId> --dry-run --format json

# HSF 型工具（V5）：方法发现 → 建造 → 部分更新
dws connector mcp hsf method-list --interface-name <接口全限定名> --format json
dws connector mcp tool create-hsf --mcp-id <mcpId> --name <snake_case> --title <标题> --description <描述> --hsf-info '{"interfaceName":"…","methodName":"…","version":"1.0.0"}' --tool-inputs '[...]' --input-mappings '[...]' --tool-outputs '[]' --output-mappings '[...]' --dry-run --format json
dws connector mcp tool update-hsf --mcp-id <mcpId> --tool-id <toolId> --description <只传要改的> --dry-run --format json
```

工具定义是三段式（结构详见上文「三段式工具定义」，映射详见 mapping-rules.md）：

- `apiInputs` / `apiOutputs`：HTTP 接口真实入参和出参。
- `toolInputs` / `toolOutputs`：暴露给 LLM 的入参和出参，可以裁剪、改名、写防呆描述。
- `inputMappings` / `outputMappings`：工具字段与真实接口字段的映射。
- 创建/更新工具前，把 `name` / `title` / `description` 和 `toolInputs` 展示给用户复核。

复杂字段直接传 JSON：

| flag | 类型 | MCP 入参 |
|------|------|----------|
| `--http-info` | object | `httpInfo` |
| `--api-inputs` | object | `apiInputs` |
| `--api-outputs` | object | `apiOutputs` |
| `--tool-inputs` | array | `toolInputs` |
| `--tool-outputs` | array | `toolOutputs` |
| `--input-mappings` | array | `inputMappings` |
| `--output-mappings` | array | `outputMappings` |
| `--value` | object | `value` |

## 调试与发布

- `tool debug` 会真实执行目标接口；写接口必须用测试数据（先让用户指定测试资源）。
- 调试带鉴权的工具必须显式传 `--credential-id`（debug 不使用 `credential bind` 绑定的凭证，不传按无凭证直调）。
- **通过标准 = 返回真实业务数据**（如查天气要真返回温度数值），不是「没报错」——映射静默失效时命令不报错但接口收到空参数。
- 调试返回为平铺结构（V4 起）：业务结果在**顶层 `toolOutput`**（不再嵌在 result.outputValue），同级有 `executeSuccess/toolInput/rawOutput/time`；出参精修是否生效直接看 `toolOutput` 形状。
- 不传 `--version-id` 时，如果工具曾发布过，会调已发布版本，不会自动调最新草稿。
- 要调试正在编辑的草稿，必须从 `tool get` 取草稿 `versionId` 后显式传 `--version-id`。
- 发布前必须至少调试通过一次。
- 发布前向用户复述工具名与作用，并明确说明发布后使用方可调用；用户明确同意后再加 `--yes`。
- 发布后立即 `tool get` 回读 `status`，必要时 `tool debug` 不传 `versionId` 验证线上版本。

## 接入地址

```bash
dws connector mcp url get --mcp-id <mcpId> --source PUBLISHED --format json
dws connector mcp url get --mcp-id <mcpId> --source MARKET --format json
```

- **publish ≠ 上架市场**：publish 后企业内即可用，`--source PUBLISHED` 直接取到「已发布未上架」服务的可用地址——无需上架、无需外部工具，全程自助闭环。
- 已上架市场传 `--source MARKET`。
- 返回的是按调用者**个人身份**生成的实例地址（非组织级公共地址）。
- ⚠️已知缺口（已反馈 Aone 84417179）：对**已删除/不可用**的服务取址不报错，返回 `success:true` 且无 mcpUrl/mcpJson 字段（空成功）。**只返回 success 而无 mcpUrl ＝ 取址失败**——先 `service get` 核实服务是否存在，绝不能当成功处理。
- 返回的 URL 或 JSON config 含个人化 `?key=`，按敏感凭证处理，勿外发共享，只能给当前用户本地配置使用。
- 代码或命令需要接入地址时优先使用 `url get` 或发现接口返回值，不要自行拼接服务 URL。

## Schema

静态管理命令需要确认 JSON 参数结构时用：

```bash
dws schema connector.mcp.service.create --format json
dws schema connector.mcp.tool.create --format json
dws schema connector.mcp.tool.update --format json
dws schema connector.mcp.tool.debug --format json
```

已发布 MCP 的动态命令 schema 来自 10 分钟命令缓存：

```bash
dws schema <service-or-tool-slug>.<tool-slug> --format json
```

该命令不会为补 schema 偷偷访问远程服务。缓存不存在时动态命令不注册；命令存在但 schema 缓存损坏或缺失时返回 `cached MCP schema is unavailable`。这不影响上面的静态管理命令 schema。
