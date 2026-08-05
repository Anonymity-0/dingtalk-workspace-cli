# Mono↔Multi Skill 内容质检规格

> 对照基准：`skills/mono`（单 skill）。被测主体：`skills/multi`。  
> 机读合同：`skills/content-qa/mono-multi-coverage.yaml`。  
> 执行：`make skill-mono-multi-content`（已挂入 `make policy`）。

## 1. 质检矩阵

| ID | 类型 | 输入 | 通过准则 |
|---|---|---|---|
| **G1 形状** | 结构 | `skills/multi/*` | 仅 `dingtalk-*` / `dws-shared`；每目录有 `SKILL.md`；必有 `dws-shared` |
| **G2 结构** | 结构 | 各 `SKILL.md` frontmatter | `name`==目录名；非空 `description`；`category`∈{product,shared}；`requires.bins` 含 `dws` |
| **G3 覆盖** | 覆盖 | mono `references/products/*` 顶层 stem | 每 stem ∈ `coverage` 或 `omit_coverage`；coverage 目标 skill/refs 存在 |
| **G4 漂移** | 漂移 | scripts、成对文件、全局协议 | orphan 脚本 ∈ allowlist；paired 一致；全局协议存在或 ∈ `omit_global` |

已有门禁（继续复用，不替代本矩阵）：`check-skill-commands`、`check-skill-context-budget`、`check-multi-im-skill-chain`、`skill_docs_policy`、whiteboard 成对测试。

## 2. 有意省略 / 延期登记格式

YAML（见 coverage 文件）：

```yaml
omit_coverage:
  - mono: simple
    disposition: covered_by   # covered_by | defer | wontfix
    via: dingtalk-misc        # optional
    reason: "拆入 oa/devdoc…"

omit_global:
  - id: field-rules-global
    mono_path: references/field-rules.md
    expected_multi: dingtalk-aitable/references/field-rules.md
    disposition: covered_by
    reason: "AI 表格字段规则已下沉到 dingtalk-aitable；G3/coverage 不强制全局同名"

orphan_scripts_allowlist:
  - path: dingtalk-misc/scripts/report_received_today.py
    disposition: defer
    reason: "pending report.md reference"
```

**处置原则**：质检失败 → 修**内容**或更新 reviewed omit；**不**改安装/升级默认。

## 3. 缺口基线（相对 mono）

| ID | 项 | disposition | 说明 |
|---|---|---|---|
| M1 | recovery-guide / RECOVERY_EVENT_ID 闭环 | **removed** | 已从 mono/multi skill 文档删除；不做移植 |
| M2 | confirmation_required 全局协议 | **done** | `dws-shared/references/confirmation.md` + SKILL 导航 |
| M3 | Schema 渐进查询教学 | **done** | `dws-shared/references/schema-usage.md` |
| M4 | `report_inbox_today.py` | `defer` / orphan 侧 | 验证后迁 misc 或删 |
| M5 | multi LICENSE/NOTICE | `defer` | 内容或打包注入 |
| M6 | aiapp 路由 vs orphan 脚本 | **done（标明未产品化）** | mono 死链移除；`unsupported-scripts.md` |
| X1 | yida/finance/aiapp orphan scripts | **done（登记）** | 由 unsupported-scripts 具名引用 |
| X2 | chat 死链 `extract_media_id.py` | n/a | 现仅为反模式提及 |
| X3 | routing → markdown 错路径 | **done** | 已指 `dingtalk-markdown`；drive 尾链已修 |
| X4 | event 缺 metadata | **done** | |
| X5 | EXPERIMENTAL「优先 mono」文案 | **done** | 五 skill 已去「优先 mono」 |
| X6 | SAFETY_PREAMBLE_INJECT 无注入器 | **done** | 标记已移除 |

产品面覆盖：见 YAML `coverage`——mono products 均有 multi 承接（misc 聚合 attendance/oa/sheet/…）。

## 4. 与悟空

借鉴 frontmatter / 断链 / requires 等**检查维度**；不运行悟空 bundle zip 校验脚本。覆盖权威始终是 DWS mono。
