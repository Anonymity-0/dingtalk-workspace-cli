---
name: dingtalk-mcp
description: 钉钉 MCP 开发平台服务与工具管理，含从 API 接口材料一键搭建 MCP。触发一：用户说 MCP服务/MCP工具/MCP开发脚手架/MCP发布/MCP调试/获取MCP地址/mcpId/toolId/actionId/versionId/mcp_tool/mcp_service。触发二：用户给出 API 文档、OpenAPI/Swagger、Postman Collection、curl 样例或任何 HTTP 接口描述，说「做成 MCP」「把这个接口给 agent/AI 用」「建个 MCP 工具」「接口变成工具」——即使没提 MCP 也用本 skill。命令前缀：dws connector mcp。
cli_version: "1.0.40+"
metadata:
  category: product
  stability: experimental
  requires:
    bins:
      - dws
---

# 钉钉 MCP 开发管理 Skill

> 🧪 **EXPERIMENTAL · 试验版 / Preview** — multi 模式当前未达 stable 标准；接口、命名、跨 skill 引用后续可能调整。生产 / 共享环境请优先使用 mono 模式（`dws skill setup --mode mono`）。

> **PREREQUISITE:** Read the `dws-shared` skill first for auth, global flags, product routing, URL preflight, error codes, and safety rules. The `dws` binary must be on PATH.

<!-- SAFETY_PREAMBLE_INJECT -->

> ⚠️ **命令可用性以当前 dws 二进制为准**。服务发现已下线，本文档随内置 skill 发布；如果 `dws <cmd> --help` 不存在，说明当前版本未暴露该命令。若命令存在但调用失败，请按错误中的 endpoint 或 tool 提示确认静态端点目录和后端工具注册。实际调用前可用 `dws <cmd> --help` 或 `--dry-run` 验证。

## MUST DO

1. 执行前先看 `dws connector mcp --help`、目标分组 `--help`、叶子命令 `--help`，按当前二进制 flag 构造命令。
2. 所有命令加 `--format json`。
3. 创建、更新、删除、调试、发布都先 `--dry-run --format json` 预览，再在用户明确确认后加 `--yes --format json`。
4. `mcpId`、`toolId`（G-ACT- 开头，即工具列表返回的 actionId，0714 契约起 tool 系命令入参统一叫 toolId）、`versionId` 必须来自 `service list/get`、`tool list/get`、`tool versions` 或上游命令返回，禁止猜。
5. 发布前必须先 `tool debug` 调试通过；调试草稿必须显式传草稿 `--version-id`。**debug 通过的标准是返回真实业务数据**（如查北京要真返回经纬度），不是「没报错」；业务结果在返回**顶层 `toolOutput`** 字段（V4 起平铺）。**调试带鉴权的工具必须显式传 `--credential-id`**——debug 不使用 `credential bind` 绑定的凭证，不传则按无凭证直调，症状全是误导性假错。
6. `url get` 返回的是按调用者**个人身份**生成的实例地址（非组织公共地址），其中 `?key=` 是个人敏感凭证，勿外发共享，不写入文档、代码、日志或回答全文。
7. 删除服务前必须先 `service get` + `tool list` 核对；服务下还有工具时先逐个删除工具。
8. **写 `--input-mappings` / `--output-mappings` 前先读 [mapping-rules.md](references/mapping-rules.md)**——映射是最大坑源：位置名 Pascal 写错、express 用错字段、出参 rules 的 source 未在 apiOutputs 声明范围内（UI 标「变量已失效」）都**静默失效不报错**，只有 debug 真跑才暴露；出参三件套（api-outputs/tool-outputs/output-mappings）0720 起必填——漏传 apiOutputs＝下游复杂结构字段被整段吞掉（调用还"成功"），CLI 已硬拦。**CLI 双闸**：create/update 时 mappings 引用的字段必须在同批提交的 apiInputs/apiOutputs/toolInputs/toolOutputs 里可解析（整体透传也必须带 apiOutputs）；publish 前自动读回草稿复验出参 rules 可解析性，不过直接拒发。
9. **两种 update 语义（V5 起，勿混用）**：http 型走 `tool update`＝**全量提交**（漏传即清空，必须先 `tool get` 读回——**读回即三段式可直接改**——在其基础上整包提交；仅 timeout/only-original-keys 例外：漏传=保留原值）；hsf 型走 `tool update-hsf`＝**部分更新**（只传要改的，未传保持原值），hsf 工具误走 http 版会被平台拒（hsf_tool_update_via_http）。判型看 `tool get` 返回的 `toolType`（http/hsf）。
10. **debug 全通过 ≠ 任务完成（发布闸门后的交付防呆）**：用户目标含「让我能调用 / 给 agent 用 / 接入客户端」时，debug 通过只是进入**待发布确认**状态——必须显式列出待发布工具清单请求发布确认（「嗯/继续」不算），且此后每次汇报都要带「仍未发布、客户端不可用」直到发布完成。执行 `url get` 前先 `tool list` 回读已发布数量：**0 个已发布工具时 `url get --source PUBLISHED` 仍返回 success+完整 URL（平台缺口 Aone 84799867），该 URL 调 `tools/list` 报泛化 `PARAM_ERROR 参数不能为空`、`tools/call` 返回 `not found the specified tool` 且 `isError=false`**——这样的 URL 不是可用交付物，禁止交付。只有 `publish → refresh → tools/list → tools/call 真调一个工具` 全通过，才能宣称 MCP/CLI 闭环完成。

## 领域模型

```
MCP 开发脚手架（mcpdev 管理面）
├── MCP 服务 Service（mcpId）
│   ├── 服务元信息：name / description / icon / introduction
│   ├── 服务命令名：serverName（kebab-case，DWS 动态命令一级路径）
│   ├── 下游鉴权配置 + 凭证账号（密钥不回显）
│   ├── 开发协作者成员
│   ├── 接入地址：mcpURL / mcpJSON（PUBLISHED / MARKET，可能含敏感 ?key=）
│   └── MCP 工具 Tool（toolId；toolType=http 配 httpInfo / hsf 配 hsfInfo）
│       ├── 身份字段：name / title / description / status
│       ├── HTTP 适配：method / url / auth
│       ├── 三段式定义：apiInputs/apiOutputs + toolInputs/toolOutputs + mappings
│       └── 工具版本 Version（versionId / versionNo / status）
├── 调试 Debug：用 value 真实执行工具，草稿调试必须显式传 versionId
├── 发布 Publish：草稿转正，发布后使用方可调用
└── DWS 动态命令面：发现已发布 MCP 后生成 <service-or-tool> <tool>，同时保留 connector mcp published <service-or-tool> <tool> 调试路径
```

- 服务是工具容器，先建服务再建工具。服务名业务语义、组织内唯一，禁 test/临时 占位名（用户明说测试用途除外）。
- 工具 `name` = snake_case 动词开头、≤32 字符、服务内唯一；`title` = 中文 ≤30 字；`description` 按四要素写（见 mapping-rules.md §7）。
- `mcpId` 定位服务；`toolId` 定位工具（G-ACT- 开头；平台底层字段名 actionId，语义相同）；`versionId` 定位工具版本。
- 工具定义是三段式：真实 HTTP 入参出参（apiInputs/apiOutputs）、暴露给 LLM 的入参出参（toolInputs/toolOutputs）、两者之间的映射规则。
- 工具创建或更新后只是草稿，必须调试通过并发布后才对使用方生效。**publish ≠ 上架市场**：publish 后企业内即可用，`url get --source PUBLISHED` 即可自助取地址。
- `draft` 只有草稿；`published` 只有线上版本；`published_with_draft` 线上有发布版同时存在更新草稿。
- `tool debug` 不传 `versionId` 时，已发布工具默认调线上版本；调试草稿必须传草稿 `versionId`。
- `service list` 返回的合法 ASCII `serverName` 是 DWS 动态命令一级路径；缺失或不合法时稳定回退 `mcp-<mcpId>`，没有 mcpId 才退到工具 `name`。禁止用中文服务名生成命令；不要凭 `mcpId` 手拼接入地址。

## Shortcut

按用户目标直接走下面的快捷方案；每一步仍要遵守 dry-run、确认和回读规则。

| 目标 | 快捷方案 |
|------|----------|
| **从 API 材料一键建 MCP（最高频）** | 收齐材料/业务目标/鉴权方式（缺就问）→ 按 [api-to-tool.md](references/api-to-tool.md) 拆三段式 + 设计整表给用户过目 → `service create`（`--name` 中文显示名 + **`--server-name` 必填**：kebab-case，就是顶层动态命令名，漏设则退化为 `mcp-<mcpId>`）→ 逐个 `tool create`（先建最简单的一个，`tool get` 读回核对再建其余）→ 逐个 `tool debug` 真跑（校验真实业务数据）→ **debug 全通过＝进入待发布确认状态（不是完成）**：列待发布清单请求用户发布确认（「嗯/继续」不算）→ 确认后逐个 `tool publish` → `url get --source PUBLISHED` → `connector mcp refresh` → `tools/list`+真调一个工具通过＝闭环完成（MUST DO 10） |
| 从零创建 MCP 服务 | `service create --dry-run`（**必带 `--server-name`**，kebab-case 一级命令名）→ 用户确认 → `service create --yes` → 记录返回 `mcpId` |
| 给服务新增 HTTP 工具 | 读 mapping-rules.md → `tool create --dry-run` → 用户确认 → `tool create --yes` → `tool get` 取 `toolId/versionId` 并核对 rules |
| 从 HSF 方法建工具（hsf 型） | `hsf method-list --interface-name <接口全限定名>` 发现方法与 DTO 字段名 → `tool create-hsf`（hsfInfo 三元组；无需 api-inputs/outputs，服务端自动生成；⚠️DTO 含 corpId/userId 必须显式写两条 $.system_node.* 注入映射）→ debug → publish |
| 验证草稿工具能跑 | `tool get` 取草稿 `versionId` → `tool debug --version-id <versionId> --dry-run` → 用户确认 → `tool debug --version-id <versionId> --yes` → 核验返回真实业务数据 |
| debug 失败排查 | 大概率映射问题：位置名大小写（Pascal）/ express 字段用错 / 漏映射 → 按 mapping-rules.md 修 → `tool update`（全量）→ 再 debug；同一工具自动修最多 2 轮，仍不行按 [mcp.md](references/mcp.md) §故障定位 五步法排查后升级给用户 |
| 发布工具并可调用 | 确认最近一次 debug 成功 → `tool publish --dry-run` → 明确说明发布后使用方可调用 → 用户明确确认（「嗯/继续」不算）→ `tool publish --yes` → `tool get` 回读状态 |
| 获取客户端接入地址 | **取址前先 `tool list` 回读已发布数量：published=0 时 URL 照样返回但不可用（MUST DO 10，Aone 84799867），先走发布流程**；已发布未上架用 `url get --source PUBLISHED`；已上架市场用 `url get --source MARKET`；输出中的 `?key=` 只脱敏展示；**只返回 success 无 mcpUrl＝取址失败**（服务已删/不可用，平台缺口 Aone 84417179），先 `service get` 核实 |
| 配置下游接口鉴权 | `auth get` 查现状 → **先按 mcp.md「鉴权方式选型」选类型（静态 API key=SIGNATURE 自定义字段+直引）** → `auth save --dry-run` → 用户确认 → `auth save --yes`；auth save 只存「说明书」，真实密钥不要放鉴权配置，改用 `credential save` 存**开发者内置凭证** |
| 管理凭证账号 | `credential list/get` 查账号元信息 → `credential save --content-file` 保存密钥（开发者内置凭证：归属当前用户+当前 MCP，密钥由开发者提供，配置调试与实例运行时两个场景都用它）→ `credential debug` 验证 → `credential bind` 选用（**仅对正式实例生效**；`tool debug` 不吃 bind，调试必须显式 `--credential-id`）；`credential unbind` 解绑实例凭证（bind 逆操作，delete 报 credential_in_use 时先 unbind 再删） |
| 管理开发协作者 | `member list` 查现状 → `member add/remove --user-ids <staffId,...> --dry-run` → 用户确认 → `--yes` |
| 探测指定 MCP 地址 | 将含凭证地址放入 `DINGTALK_MCPDEV_MCP_URL`，执行 `dws connector mcp inspect --format json`；读取协议版本、服务能力和完整工具 Schema，不调用业务工具 |
| 生成/刷新 DWS 动态命令 | 工具发布后执行 `dws connector mcp refresh --format json` → 检查 `partial/failedServices/cacheUpdated` → 检查 `dws <service-or-tool> --help` 和 `dws connector mcp published --help`；部分失败时健康服务正常更新、失败服务保留旧缓存 |
| 续作已有服务/工具 | `service list --keyword` 找 `mcpId` → `tool list --mcp-id` 找 `toolId` → 再执行目标操作 |
| 编辑已发布工具 | `tool get` 读当前定义（**读回即三段式**，判型看 toolType）→ http 型全量构造 `tool update --dry-run`（漏传即清空）/ hsf 型 `tool update-hsf` 只传要改的（部分更新）→ 调试草稿 versionId → 用户确认后发布 |
| 删除工具或服务 | 先 `tool get` 或 `service get` + `tool list` 核对影响面 → 用户明确确认 → 删除命令加 `--yes` |

## 意图表

| 用户说 | 命令 |
|--------|------|
| "把这个接口做成 MCP / 给 agent 用" | 走 Shortcut 第一行全流程（从 API 材料一键建 MCP） |
| "列出 MCP 服务 / 找回 mcpId 或 serverName" | `dws connector mcp service list --keyword <关键词> --format json`；读取顶层 `services[].mcpId/serverName` |
| "查看 MCP 服务详情" | `dws connector mcp service get --mcp-id <mcpId> --format json` |
| "创建 MCP 服务" | `dws connector mcp service create --name <中文显示名> --server-name <kebab命令名> --description <描述> --dry-run --format json` |
| "列出某服务工具 / 找回 toolId" | `dws connector mcp tool list --mcp-id <mcpId> --page-size 100 --format json` |
| "读取工具定义 / 找 versionId" | `dws connector mcp tool get --mcp-id <mcpId> --tool-id <toolId> --format json` |
| "调试工具草稿" | `dws connector mcp tool debug --mcp-id <mcpId> --tool-id <toolId> --version-id <versionId> --value '{"city_name":"北京"}' --dry-run --format json`（value=符合 toolInputs 的测试入参，从设计阶段的材料示例值来，不要传空 `{}` 走过场；带鉴权的工具加 `--credential-id <id>`） |
| "发布工具" | `dws connector mcp tool publish --mcp-id <mcpId> --tool-id <toolId> --dry-run --format json` |
| "获取 MCP 接入地址" | `dws connector mcp url get --mcp-id <mcpId> --source PUBLISHED --format json` |
| "配置/查询 MCP 鉴权" | `dws connector mcp auth get\|save` |
| "保存/查询/调试/绑定/删除 MCP 凭证" | `dws connector mcp credential save\|list\|get\|debug\|bind\|delete` |
| "查询/新增/移除 MCP 协作者" | `dws connector mcp member list\|add\|remove` |
| "把 HSF 接口/方法做成工具" | `dws connector mcp hsf method-list` 发现方法 → `tool create-hsf`（详见 mcp.md HSF 节） |
| "解绑实例凭证 / 删凭证被 credential_in_use 拦" | `dws connector mcp credential unbind --mcp-id <mcpId>` → 再 `credential delete` |
| "读取这个 MCP 的协议和工具元数据" | 设置 `DINGTALK_MCPDEV_MCP_URL` 后执行 `dws connector mcp inspect --format json` |

## 详细参考

- 命令面、生命周期、故障定位：[mcp.md](references/mcp.md)
- 从 API 材料拆三段式工具定义（建新 MCP 时读）：[api-to-tool.md](references/api-to-tool.md)
- 映射规则（写 `--input-mappings`/`--output-mappings` 前必读）：[mapping-rules.md](references/mapping-rules.md)
- express 表达式函数全集（7 组 82 个，复杂数据变换才查）：[expression-functions.md](references/expression-functions.md)
