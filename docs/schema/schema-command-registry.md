# schema_command_registry — 评审身份册

这份目录就留着当「身份册」用手改：**不删、不生成、不搬进 Contract**。

## 管什么

- `canonical_path`（稳定 ID）
- `cli_path`（主路径）
- `aliases`（正式别名表）
- 可选 `visibility`

实例（本目录真实条目）：

- `drive.copy_document` ↔ `drive copy`
- `sheet.table_get` → `sheet table-get`，另有别名 `sheet table-read`

## 不管什么

Safety / Selection / 参数语义 → 写在叶上 Contract（`ContractDecl` / `ProductDecl`），不进 registry。

同属 `internal/cli` 的 **评审输入**（`param_concepts.json`、exclusions、
`schema_parameter_mapping_ledger.go`、MCP pin）与本目录并列、职责分开——不要合并进
registry，也不要把它们抬成 Catalog 声明权威。总表见 `AGENTS.md`「Reviewed inputs /
评审输入」。

**别名三层消歧**（勿混用）：`FlagSpec.Aliases` = 叶上 flag 同义词；本目录
`aliases` = 命令路径正式别名；`param_concepts.json` = argv 概念/同义归一。Identity
是 pin（与 Cobra 对齐），不是第二身份源。

## 为什么不能删

**Cobra 不够当身份源。** Cobra 只保证「现在能不能跑、有哪些 flag」。它不负责稳定的
canonical_path、正式别名表、产品导航、visibility。叶名一改、别名一挪，若没有独立
评审册，Agent 侧身份会跟着漂移。

**Contract 也不能顶替。** 叶上 ContractDecl 管 Safety / Selection / 参数语义。Identity
若以 Contract 为准，等于每个产品作者都能改 canonical，和「单一评审身份」冲突；现在的
规则是 Contract 身份只能与 registry 对齐，不能覆盖。

**装配硬依赖它。** `BuildEffectiveCommandRegistry` = registry ∩ 活 Cobra 叶 → 再 Bind →
再装 ToolSpec。没有 registry，就没有 Effective 集合，也就没有「每个 Agent 可见叶恰好
一条 ToolSpec」的契约。

**和 Catalog 不是一类东西。** committed Catalog / hints 可以删（已删），因为那是
投影/旁路。registry 是输入，不是 dump。

**若强行「只靠 Cobra」会怎样：** Agent 身份随代码重构抖动；别名/导航/隐藏策略失去
评审落点；反向完备性（registry ↔ Cobra ↔ exclusions）失去一边锚点。

## 能防什么漂移

| 漂移类型 | 拦截机制 |
|---|---|
| 重构改名（叶名/canonical 动了） | registry 里的 canonical_path 与活 Cobra 做交集校验；代码一改名、registry 没改 → 三向检查 CI 红。身份变化必须走 registry 人工评审，不能随重构搭车 |
| 别名漂移（挪动/增删别名） | 别名只存在于 registry；代码侧动别名而不同步 → CI 红 |
| 可见性漂移（命令变可见/隐藏） | registry ↔ Cobra ↔ exclusions 三向锚定，单边变动即失败 |
| 静黙漂移（任何漏网的） | `ReviewedCommandRegistrySourceHash` + surface/catalog hash 双哈希——dump 与 CI 基线对哈希，任何未评审的身份变化都以哈希不匹配现形 |

边界：语义（Safety/Selection/参数）与行为（命令做什么）的漂移不归 registry 管——
语义归叶上 Contract + homology 门禁，行为归产品测试。registry 只锚「它是谁、在哪、
叫什么」。

**它到底防住了什么（精确版）：**

1. 它拦不住「有人把名字改了」——git、CI、评审册都拦不住编辑发生；有意义的拦截点
   从来只有「能不能合入」。它把失败点从「生产环境静黙漂移，数周后才发现」提前到
   「CI 当场红，合入前爆炸」。
2. 只靠 Cobra 时漂移在原则上不可发现：检测漂移需要第二个独立真源比对，没有
   registry，「有意的改名」和「事故的漂移」在信息上无法区分。
3. 它真正做的：把静黙漂移变成必须留痕的评审事件——身份要动，必须在评审册里留
   一行 diff，过一道人审。
4. 合谋情形（代码+册子同 PR 一起改）防不住，这是设计终点：机器负责让漂移可发现
   且留痕，人负责在留痕处做判断；再往上是 CODEOWNERS / required review 的地盘。

## 怎么办（日常维护）

- 新上 Schema / 改路径别名 → 改 `products/<product>.json`
- 只改文案参数 → 不动 registry
- 可跑但不给 Agent → 写 exclusions（`internal/cli/schema_command_exclusions.go`）
- 只加 Cobra 或只写 Contract：help 能跑，但 Schema 绑不上，CI 会报 Missing
- 三者分工不能互相替代：registry（身份）/ Contract（语义）/ Cobra（可执行面）

## 怎么写入（agent / 人工通用）

**条目格式**（`products/<product>.json` 的 `tools` 数组，一条命令一个对象）：

```json
{ "canonical_path": "drive.copy_document", "cli_path": "drive copy" }
{ "canonical_path": "sheet.table_get", "cli_path": "sheet table-get", "aliases": ["sheet table-read"] }
```

- 必填：`canonical_path`（`<product>.<name>`，稳定 ID，起盘后不改）、`cli_path`（主路径，指向活 Cobra 叶）
- 可选：`aliases`（正式别名表，全库现仅 9 条在用）、`visibility`（**deprecated / dormant**：`public` 默认省略 / `compat` / `internal`；当前产品分片零显式赋值，「可跑但不对 Agent 可见」写 `schema_command_exclusions.go`，不要新开 visibility 策略）、`source_product_id`（跨产品归源）
- 未知字段会被 schema 与加载层双重拒绝——只能写这五个字段

**写入步骤**：

1. 只在「新上 Schema 命令 / 改路径或别名」时改 `products/<product>.json`；文案、参数、Safety/Selection 一律不动 registry（那些归叶上 Contract）
2. 本地验证（全部要绿）：
   - `go test -count=1 -run 'TestCommandRegistry|TestDecodeCommandRegistry' ./internal/cli/`（格式契约）
   - `./scripts/policy/check-schema-command-registry.sh`（schema 校验）
   - `./scripts/policy/check-command-surface.sh --strict`（registry ↔ Cobra ↔ exclusions 三向一致）
3. 三向锚定自检：新命令只加 Cobra 或只写 Contract → help 能跑但 Schema 绑不上，CI 报 Missing；registry 加了但 Cobra 叶不存在 → 同样失败

## 什么时候可以变薄，而不是删除

- 条目继续按产品分片（已是）
- 机械字段少写、只保留身份/导航最小集
- 长期若做「从声明推导导航」也只能是生成候选 + 人类评审写入 registry，不能变成
  运行时从 Cobra 猜身份

一句话：Catalog 可运行时现装；身份必须先有一份审过的册。`schema_command_registry/`
就是那份册。
