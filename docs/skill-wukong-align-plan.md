# DWS multi-skill **内容框架**对齐方案（相对 dws-wukong develop）

> 状态：**方案待 owner 按 §7 重新批准后编码**。本文只做内容架构盘点与分期建议，**不含实现**。
>
> 撰写 / 二次收窄：2026-08-05  
> 工作树：`/Users/john/GolandProjects/open-source/dws-multi-skill-align`  
> 分支：`feat/multi-skill-framework-align`（自 `origin/main` @ `a37e6e68`）  
> **本分支范围：只做 skill 内容的这个框架**（目录布局、文档契约、共享内容约定、zip 内内容树合同）。  
> **不做**安装/升级引擎、agent-home、脚本 skill-install 行为翻转。
>
> 对照仓：
>
> | 仓 | 路径 | 基线 |
> |---|---|---|
> | DWS OSS CLI（本工作树） | `dws-multi-skill-align` | `origin/main` |
> | dws-wukong | `~/GolandProjects/open-source/dws-wukong` | `origin/develop` @ `ab76629a`（调研时） |
> | 行为参考（**另一分支**） | `dws-skill-mode-migration` @ `402429ac`/`d5c8982c` | 安装默认 multi / upgrade 强制 multi —— **不在本分支排期** |

---

## 0. TL;DR

1. **本分支 = skill 内容框架**：`skills/mono` 与尤其 `skills/multi` 如何组织（目录、`SKILL.md`、references、共享层、跨 skill 约定），以及 `dws-skills.zip` 里 multi/mono **内容树布局合同**（不是 Go 安装引擎）。
2. **对齐对象**：dws-wukong develop 的 **skill 内容树**（`dingtalk-skills/`、flat `dingtalk-*` + `dws-shared`、references 分层），只取对 DWS 内容组织有用的概念；**不**对齐客户端 `_install.sh` / dual 包 / RewindDesktop。
3. **安装/升级行为**（默认 multi、`LocateSkillsRoot`、`skill_setup`、`paths.go`、install 脚本、`402429ac`/`d5c8982c`）→ **单独 follow-up 分支**，本方案只登记延期，不进 Phase。
4. 可选「内容包元数据」（如树内 `manifest` 仅描述内容版本/布局）与安装运行时 manifest **分开**；后者不在本分支。

### 0.1 IN SCOPE（本分支唯一交付面）

| 类别 | 包含 |
|---|---|
| 内容树结构 | `skills/mono/`（单入口 + `references/products/*`）与 `skills/multi/<name>/`（平铺产品 skill）的目录合同 |
| 单 skill 内部约定 | `SKILL.md` frontmatter / 运行时契约块 / Golden Route；`references/` 分层与索引；可选 `scripts/` 归属规则 |
| 共享内容 | `skills/multi/dws-shared/` 职责边界、被产品 skill 引用的方式、与 mono 共享文的映射关系（文档级） |
| 命名与集合 | `dingtalk-<product>` + `dws-shared`；相对悟空树的「共有 / 仅 DWS」清单（**文档与缺口表**，不强制内容抄库） |
| Zip **内容布局合同** | 发布物中 `mono/`、`multi/`、根 mono 副本各自代表什么内容树（描述与必要时校验「树形状」）；**不改**安装谁、默认装哪棵 |
| 内容架构文档 | 本文件 + 可选短文：multi 内容架构 / mono↔multi 内容映射约定 |

### 0.2 OUT OF SCOPE（本分支不做 → 另分支 / 延期）

| 类别 | 去向 |
|---|---|
| 安装默认 multi、upgrade 有 multi 则强制刷 multi | **Follow-up 分支**（可基于 `402429ac` / `d5c8982c`） |
| `LocateSkillsRoot`、`skill_setup.go`、`internal/upgrade/paths.go` | 同上 |
| `internal/skillhome`（agent-home / 互斥 / stale 安装逻辑） | 同上 |
| `install.sh` / `install.ps1` / `install.js` / `install-skills.sh` 的 skill-install 行为 | 同上 |
| Cherry-pick `402429ac` / `d5c8982c` 作为本分支 Phase 1 | **明确不排期**；§6 仅作指针 |
| 面向安装/运行时的 manifest、state.json、`dws skill mode`、telemetry header | 拒绝或属行为分支；已取消产品仍拒绝 |
| 悟空 `_install.sh` symlink、dual/Qwen/RewindDesktop/pod | 拒绝 |
| Schema catalog / chat shortcut 代码 / whiteboard等非 skill-内容工作 | 拒绝（除非某 reference **正文**本身是本分支内容编辑任务且 owner 点名） |

---

## 1. 内容现状盘点

### 1.1 DWS `skills/mono`（单 skill 内容）

```text
skills/mono/
├── SKILL.md                 # 总入口
├── references/
│   ├── products/<product>.md|
│   ├── best_practices/…
│   └── （全局：error-codes、intent-guide、…）
└── scripts/                 # 可选辅助脚本
```

特点：一个 skill id；产品能力以 `references/products/*` 挂在同一棵树下。

### 1.2 DWS `skills/multi`（多 skill 内容 · 本分支重点）

```text
skills/multi/
├── dws-shared/
│   ├── SKILL.md
│   └── references/          # 全局契约、routing、error-codes、best_practices/_common、…
└── dingtalk-<product>/      # 当前 19 个产品 skill
    ├── SKILL.md             # frontmatter + 运行时契约块 + Golden Route / 意图表
    ├── references/          # 产品内分层（如 chat/、card/）
    └── scripts/             # 部分产品有
```

当前产品集合（调研时）：  
aisearch, aitable, calendar, chat, contact, **dev**, doc, drive, **event**, **hrbrain**, mail, **markdown**, minutes, misc, **pat**, **profile**, **skill**, todo, wiki + `dws-shared`。  
（加粗 = 悟空 `dingtalk-skills/` 无对应目录。）

### 1.3 dws-wukong develop `dingtalk-skills/`（内容树对照）

```text
dingtalk-skills/
├── dws-shared/              # 含 overlays/、package.json 等悟空侧扩展（OSS 未必需要）
└── dingtalk-* /             # 12 产品：aisearch…wiki（无 dev/event/hrbrain/…）
    ├── SKILL.md
    ├── references/
    └── scripts/
```

打包侧另有「bundle 内再套一层 `skills/` + 根 `manifest.json`」——那是 **客户端/Qwen 包形态**，本分支只关心 **子树本身的内容组织**（`SKILL.md` + `references` + 命名），不移植 bundle 外壳。

### 1.4 Zip 内容布局（内容合同，非安装行为）

`dws-skills.zip` 现行内容形状（`post-goreleaser.sh` 注释）：

| Zip 路径 | 内容含义 |
|---|---|
| `<root>/`（SKILL.md + references + …） | mono 树副本（兼容旧消费者看根） |
| `<root>/mono/` | 显式 mono 内容源 |
| `<root>/multi/` | multi 平铺内容源（与 `skills/multi/` 同构） |

本分支若动打包，**仅**为澄清/加固「内容树合同」（例如校验 multi 下每个子目录含 `SKILL.md`、命名符合 `dingtalk-*|dws-shared`）。**不**在此改变「默认安装哪棵树」。

---

## 2. Diff：内容组织已对齐 / 分叉 / 勿移植

### 2.1 已大致同构

| 概念 | DWS multi | 悟空 dingtalk-skills |
|---|---|---|
| 目录名 | `dingtalk-<product>` | 同 |
| 共享层目录 | `dws-shared` | 同 |
| 单 skill 骨架 | `SKILL.md` + `references/`（+ 可选 `scripts/`） | 同 |
| 平铺多 skill | `skills/multi/*` | `dingtalk-skills/*`（再被打进 bundle 的 `skills/*`） |

### 2.2 内容分叉（本分支可讨论）

| 维度 | DWS | 悟空内容树 | 本分支态度 |
|---|---|---|---|
| 产品数量 | 19 + shared | 12 + shared | **保留 DWS 超集**；文档化共有/独有，不合一抄库 |
| mono 形态 | 独立完整树 `skills/mono` | 另有 `dingtalk-workspace/` mono 源 | 文档化 mono↔multi **内容映射**；不删 mono |
| `dws-shared` | 无 overlays/package.json | 有 overlays、package.json | **不**移植悟空 overlay 流水线；可借鉴「共享层只放跨产品契约」边界 |
| references 命名 | 产品自洽（如 `chat/`、`01-*.md` 混用） | 部分产品有 `01-messaging`、`lite-recipes` 等 | 订 **DWS multi 内容约定**（索引方式、是否统一编号），不对齐抄文件名 |
| 运行时契约块 | multi `SKILL.md` 含 `DWS_RUNTIME_CONTRACT` 等标记块 | 文案可能不同源 | 以 DWS 契约为准；只统一 **内容框架标记** 是否每个产品 skill 必有 |
| Zip 外壳 | 三树（根+mono+multi） | bundle = manifest + skills/ | **保持 DWS 三树**；不改为悟空 bundle 外壳 |

### 2.3 内容侧 Reject

- 悟空 symlink 安装记账、dual 包、Qwen overlay、pod 链路  
- 为「像悟空」而删掉 DWS 独有 7 个产品 skill  
- 把 mono 产品文机械搬进 multi 而不经内容评审（若做映射，走内容 Phase，不走安装 Phase）

---

## 3. Goals / Non-goals

### 3.1 Goals（内容框架）

1. **写清并固化** multi 内容目录合同：合法子树名、必有文件、`dws-shared` 边界、产品 skill 最小 `SKILL.md`/references 结构。  
2. **文档化** mono ↔ multi 内容关系（哪些全局文在 shared、哪些在产品树、与 `references/products/*` 的对应），便于后续演进，而非本分支做安装切换。  
3. **对照悟空内容树**产出「共有 12 / 仅 DWS 7」与结构差异表；只吸收有用的组织概念（flat + shared），不抄客户端包。  
4. **（可选）** zip/源树级 **内容元数据**（例如 `multi/CONTENT_LAYOUT.md` 或纯内容 `manifest`：layout 版本、skill 列表）——标明 content-only，**不**驱动 setup/upgrade。  
5. **（可选）** 内容形状校验脚本/测试：multi 下每个 skill 含 `SKILL.md`、命名合法、shared 存在——仍属内容合同，不碰安装默认值。

### 3.2 Non-goals（本分支）

| 项 | 说明 |
|---|---|
| 安装/升级行为翻转 | 另分支；见 §6 |
| Go skill 安装引擎重构 | 另分支 |
| Cherry-pick `402429ac`/`d5c8982c` | 另分支，**不**作本分支 Phase 1 |
| 取消产品（mode / state.json / header） | 仍拒绝 |
| 悟空客户端打包与安装器 | 拒绝 |
| 非 skill 内容的 CLI 功能开发 | 拒绝 |

---

## 4. 分期（全部为内容-only）

> 批准前 **零编码**。下列阶段 **均不包含** install/upgrade Go/脚本行为变更。

### Phase 0 — 方案冻结（本文）

| | |
|---|---|
| **范围** | 本文件；§7 勾选 |
| **文件** | 仅 `docs/skill-wukong-align-plan.md` |
| **验收** | owner 按「内容框架 only」重新批准 |

### Phase 1 — Multi 内容目录合同 + 架构短文

| | |
|---|---|
| **范围** | 成文：`skills/multi` 目录合同（命名、必有 `SKILL.md`、`references/` 约定、`dws-shared` 职责）；与悟空内容树对照表（共有/独有/结构差异）；明确 zip 内 `multi/` 与源树同构的 **内容合同** |
| **可能触达** | `docs/` 下 skill 内容架构短文（新建）；必要时各 `SKILL.md` **仅**目录/约定说明级极小修订（非产品能力扩写） |
| **不碰** | 任何 `internal/app`、`internal/upgrade`、install 脚本行为 |
| **验收** | 文档可独立指导「如何新增一个 dingtalk-* 内容目录」；对照表完整；§0.2 未破线 |

### Phase 2 — Mono↔multi 内容映射与缺口表

| | |
|---|---|
| **范围** | 盘点 mono `references/products/*`（及全局文）与 multi 各树的覆盖/重复/缺口；输出缺口表与「共享层 vs 产品层」归属建议 |
| **可能触达** | 文档；可选在 `dws-shared` 或产品 references **补索引/链接**（内容导航），不做安装逻辑 |
| **验收** | 缺口表经 owner 过目；无安装行为 diff |

### Phase 3 — 内容形状护栏（可选）

| | |
|---|---|
| **范围** | 测试或轻量脚本：断言 multi 子目录命名、`SKILL.md` 存在、必有 shared；可选检查 zip staging 树形状与源树一致（**内容合同**） |
| **可能触达** | `test/unit/*skill*content*` 或 `scripts/policy/check-multi-skill-content.sh` 一类；**不**改 `LocateSkillsRoot`/默认 mode |
| **验收** | 故意破坏目录合同会失败；CI 仅绑内容护栏 |

### Phase 4 — 可选内容包元数据（content-only）

| | |
|---|---|
| **范围** | 若需要：在 `skills/multi/`（或 zip `multi/`）增加 **纯内容** 元数据文件（layout 版本、skill 列表），供人/校验脚本阅读 |
| **禁止** | 被 setup/upgrade/install 脚本读取并改变安装行为（那属行为分支） |
| **验收** | 元数据缺省不影响现行安装；文档写明「非运行时合同」 |

### 延期登记（不在本分支 Phase）

| 主题 | 建议载体 |
|---|---|
| 默认 multi + upgrade always-multi | 新分支，cherry-pick/移植 `402429ac` → `d5c8982c`（skill 行为 diff only） |
| `skillhome` / agent-home / 互斥安装 | 行为/框架分支（安装向） |
| 安装面 skill-install bootstrap | 同上 |

---

## 5. Port / Adapt / Reject（悟空 · **内容树**）

| 悟空内容相关 | 决策 | 说明 |
|---|---|---|
| flat `dingtalk-*` + `dws-shared` 目录模型 | **Port（内容语义）** | DWS 已具备；固化为合同 |
| 单 skill = `SKILL.md` + `references/` + 可选 `scripts/` | **Port** | 已同构 |
| `dws-shared` 放跨产品契约/routing | **Adapt** | 对齐职责边界；不抄 overlays |
| references 编号/文件名习惯 | **Adapt（可选）** | 订 DWS 自己的索引约定，不强制改名大爆发 |
| 12 产品内容正文 | **Reject（抄库）** | 不合一；缺口用文档跟踪 |
| bundle `manifest` + `_install.sh` + 外层 `skills/` | **Reject** | 安装/客户端包形态 |
| `sync-monolith-to-multiskill.py` | **Reject / 另议** | 派生流水线属内容工程化，仅当 Phase 2 证明需要「从 mono 生成 multi」再开题；默认直接维护 multi |
| dual / Qwen overlay / pod | **Reject** | 非内容树 |

---

## 6. 与 `402429ac` / `d5c8982c` 的关系

| 问题 | 结论 |
|---|---|
| 本分支是否 cherry-pick？ | **否** |
| 是否作为 Phase 1？ | **否** |
| 它们是什么？ | 安装/升级 **行为**（默认 multi、upgrade 强制 multi、脚本真装） |
| 何时做？ | **单独 follow-up 分支**（建议仍从 `origin/main` 拉出，例如 `feat/multi-skill-install-default`），且只带 skill 行为 diff、不绑内容大改 |
| 本分支与行为分支顺序？ | 可并行；内容合同稳定有利于行为分支少踩「目录假设」；**不**要求内容分支先合并才能做行为分支 |

---

## 7. 批准清单（内容框架 only · 请重新勾选）

**范围**

- [ ] 认可本分支 **仅** skill **内容**框架（§0.1）  
- [ ] 认可 §0.2：安装/升级/`skill_setup`/`paths`/`skillhome`/install 脚本/`402429ac`·`d5c8982c` **均不在本分支**  
- [ ] 认可取消产品与悟空客户端链路仍拒绝  

**Phase（内容）**

- [ ] **Phase 1**：multi 内容目录合同 + 与悟空内容树对照短文（可执行）  
- [ ] **Phase 2**：mono↔multi 内容映射与缺口表 —— 本迭代做 / 拆后续  
- [ ] **Phase 3**：内容形状护栏（测试/policy）—— 做 / 不做 / 以后  
- [ ] **Phase 4**：纯内容元数据文件 —— 做 / 不做 / 以后  

**Follow-up（仅登记，不批准即不在本分支做）**

- [ ] 知悉安装默认 multi + upgrade always-multi 将走 **另一分支**（可基于 `402429ac`/`d5c8982c`）  

---

## 8. 下一步

**请 owner 按 §7 对「内容框架 only」重新批准。**  
批准前不编码。批准后默认推进 Phase 1 文档/合同；Phase 2–4 按勾选。  
安装/升级 cherry-pick **等待另开分支指令**，不在本方案 Phase 内启动。

---

*内容路径锚点：`skills/mono`、`skills/multi`、wukong `dingtalk-skills/`（`git show origin/develop:…`）。*
