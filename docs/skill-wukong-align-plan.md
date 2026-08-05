# DWS multi-skill 框架对齐方案（相对 dws-wukong develop）

> 状态：**方案待 owner 按 §7 重新批准后编码**。本文只做对齐盘点与分期建议，不含实现。
>
> 撰写 / 收窄：2026-08-05  
> 工作树：`/Users/john/GolandProjects/open-source/dws-multi-skill-align`  
> 分支：`feat/multi-skill-framework-align`（自 `origin/main` @ `a37e6e68`）  
> **本分支范围声明：只做 skills 本身 + skill 框架**（见 §0.1 / §3）。
>
> 对照仓：
>
> | 仓 | 路径 | 基线 |
> |---|---|---|
> | DWS OSS CLI（本工作树） | `dws-multi-skill-align` | `origin/main` |
> | 既有迁移试验分支 | `dws-skill-mode-migration` | `feat/skill-mode-migration` @ `d5c8982c` |
> | dws-wukong | `~/GolandProjects/open-source/dws-wukong` | `origin/develop` @ `ab76629a`（调研时） |
>
> 配套留档（勿当权威；悟空线已决策下线；**非本分支交付物**）：  
> `feat/skill-mode-migration` 上的 `docs/skill-wukong-comparison.md`、  
> `docs/skill-architecture-optimization.md`、`docs/skill-multi-roadmap.md`。

---

## 0. TL;DR

1. **本分支只对齐 skill 内容树与 skill 分发/安装框架**，不碰 schema / shortcut / 白板 / 无关 CI / 悟空客户端发版链。
2. **值得对齐的框架语义**：flat multi（`dingtalk-*` + `dws-shared`）、安装默认 multi、upgrade 有 `multi/` 时强制刷 multi、`LocateSkillsRoot`、互斥/stale、agent-home 清单与共享常量。
3. **悟空侧只吸收 skill 打包概念**（flat 布局、可选 version manifest 思路）；拒绝 symlink `_install.sh`、dual/Qwen/RewindDesktop/pod。
4. **复用 `402429ac` / `d5c8982c` 时只取 skill 框架/行为 diff**；提交内的大段 roadmap/对比文档默认不整包带进本分支（见 §6）。
5. **编码前需 §7 勾选批准**（范围已收窄，请重新确认）。

### 0.1 本分支 IN SCOPE（唯一交付面）

| 类别 | 包含 |
|---|---|
| Skill 内容树 | `skills/mono/`、`skills/multi/`（命名、布局、必要文案；**不做**与悟空 12 产品内容硬同步） |
| Skill 打包布局 | `dws-skills.zip` 内 mono/multi 树选择；可选轻量 skill `manifest`（Phase 4，另批）；`skills/embed.go` |
| Skill 框架（Go） | discovery / `dws skill setup` / upgrade 的 skill 刷新路径；`LocateSkillsRoot`；互斥与 stale 清理；agent-home 安装落点；建议的 `internal/skillhome` 共享常量与 helpers |
| Skill 安装路径（脚本） | **仅**各安装面中的 **skill-install 路径**（mode 解析、拷贝/清理 multi\|mono、缓存 `~/.dws/skills/{mono,multi}`、调用 `dws skill setup`）；**不**重写二进制安装、PATH、shell 补全等非 skill 逻辑 |
| 测试 / 文档 | setup/upgrade/skill-install 单测与 fake-HOME skill 冒烟；本方案 + 必要 CHANGELOG/README **skill 段落** |

### 0.2 本分支 OUT OF SCOPE（拒绝或延后）

| 类别 | 处理 |
|---|---|
| 非 skill 的 CLI 产品工作（schema catalog、chat shortcut、whiteboard、无关 helpers 等） | **拒绝**纳入本分支 |
| 与 skill 无关的 CI / policy / Makefile 大改 | **拒绝**；仅允许为 skill 清单门禁新增极窄 policy（Phase 2） |
| 悟空客户端 / RewindDesktop / pod / `real-platform` / Gaea 发版链 | **拒绝**（仓外；且悟空 bundled 线已下线） |
| 已取消产品：`dws skill mode`、`state.json` 产品面、`x-dws-skill-mode` | **Non-goals**（不变） |
| 「整份 install.sh/ps1/npm 安装器重写」 | **拒绝**；Phase 3 只改 **skill-install 框架路径**（bootstrap → `dws skill setup` + 冻结 skill 拷贝 fallback） |
| 悟空 `dingtalk-skills` 内容集与 OSS multi(19) 强行合一 | **拒绝** |
| 整支 merge `feat/skill-mode-migration` | **拒绝**；只精简移植 skill 行为 |

---

## 1. 现状盘点（skill 相关）

### 1.1 DWS OSS（`origin/main`）— skill 面

| 层 | 现状 | 锚点 |
|---|---|---|
| 源树 | `skills/mono/` + `skills/multi/`（19 产品 + `dws-shared`） | `skills/` |
| embed | `//go:embed all:mono all:multi` | `skills/embed.go` |
| 发布 zip | `dws-skills.zip` = 根 mono 副本 + `mono/` + `multi/` | `scripts/release/post-goreleaser.sh`（skills 段） |
| `dws skill setup` | `--mode mono\|multi`；**非交互默认 mono**；multi 标 EXPERIMENTAL | `internal/app/skill_setup.go` |
| upgrade · skill | 刷新已装 skill；**尚未**「有 multi 包则强制刷 multi」 | `internal/upgrade/paths.go` skill 路径 |
| 安装面 · skill 路径 | `DWS_SKILL_MODE`；非 TTY 默认 mono；multi 多为提示/半装 | `install.sh` / `install.ps1` / `install-skills.sh` / `build/npm/install.js` 的 skill 段 |
| Agent home 清单 | setup / upgrade / `skill install` / 脚本 skill 段多份拷贝；**opencode 漂移** | `skill_setup.go`、`paths.go`、`skill_command.go` |
| 取消产品 | 无 mode 切换 / state.json 产品 / telemetry header | owner 2026-08-05 |

### 1.2 dws-wukong `develop` — **仅 skill 打包/框架概念**

| 件 | 与本分支关系 |
|---|---|
| `dingtalk-skills/` flat 树 + `dws-shared` | **概念对齐**（命名/平铺）；内容集不移植 |
| `build-bundle.sh` → `manifest.json` + `skills/*` | **布局概念**可参考；产物名/双包流程不移植 |
| `_install.sh` symlink 提升 | **Reject**（消费模型不同） |
| `bundle-platform` / `package-dual` / Qwen overlay / pod | **Out of scope**（客户端/云分发） |

> 对齐目标：吸收对 **DWS skill 分发**有用的框架概念，不是接入悟空发版链路。

### 1.3 既有迁移提交（仅作 skill 行为来源）

| Commit | Skill 行为摘要 | 本分支取舍 |
|---|---|---|
| `402429ac` | setup/安装面默认 multi；multi 真装；全量清 stale；upgrade prefer multi | **取 skill 代码 + 必要测试**；大段 skill-*.md / 对比文档默认不带或另筛 |
| `d5c8982c` | upgrade 有 multi 始终刷 multi；去掉 sticky；文档同步取消项 | **取 `paths.go` / 测试 / 取消项结论**；roadmap 长文不整包迁入 |

---

## 2. Diff（skill 框架视角）

### 2.1 已同构

| 概念 | DWS | 悟空 skill 包 |
|---|---|---|
| `dingtalk-<product>` + `dws-shared` | `skills/multi/` | bundle `skills/` |
| 每目录 `SKILL.md` 平铺 | 是 | 是 |
| 过渡期双树 | zip `mono/`+`multi/` | dual 可选（本分支不实现 dual） |

### 2.2 分叉（本分支要收）

| 维度 | DWS main | 目标（本分支） |
|---|---|---|
| 默认 skill 布局 | mono | **multi**（mono opt-in） |
| upgrade skill 策略 | 未强制 multi | 包含 `multi/` → **始终刷 multi** |
| 清单/互斥 | 多份拷贝、opencode 漂移 | Go 单一事实源（`skillhome`） |
| 包自描述 | 无 | 可选轻量 manifest（Phase 4） |

### 2.3 明确不在本分支解决

- 悟空端内如何加载 bundle、RewindDesktop、pod 版本号  
- Schema / 命令框架 / 非 skill 产品面  
- 整安装器（二进制、shell 集成）重写  

---

## 3. Goals / Non-goals（收窄后）

### 3.1 Goals

1. **Skill 行为**：安装默认 multi；`--mode mono` / `DWS_SKILL_MODE=mono` opt-in；upgrade 在 `LocateSkillsRoot` 见到 `multi/` 时始终刷 multi 并清 mono leftover / stale（无 sticky）。
2. **Skill 框架**：`internal/skillhome`（或等价）收敛 AgentHomes、保留前缀、互斥/stale、布局规则；setup 与 upgrade 的 skill 路径共用；脚本 **skill 段**清单可 policy 比对。
3. **Skill 内容/布局约定**：统一 multi = flat `dingtalk-*` + `dws-shared`；全量安装清 stale，`-s/-x` 保持 additive；不强制改产品 SKILL 正文（除非安装框架所需的极小文案）。
4. **可选**：`dws-skills.zip` 增加轻量 skill `manifest.json`（不破坏三树）——§7 另批。
5. **测试**：skill setup/upgrade 单测 + 安装面 **skill 路径** fake-HOME 冒烟。

### 3.2 Non-goals / 本分支拒绝

| 项 | 原因 |
|---|---|
| `dws skill mode` / `state.json` 产品 / `x-dws-skill-mode` | 已取消 |
| `_install.sh` symlink canonical | 与多 agent-home 拷贝模型冲突 |
| 悟空 dual/Qwen/RewindDesktop/pod | 非 skill-OSS 框架；仓外 |
| 内容集 19↔12 合一 | MOOT / 非目标 |
| 大改 zip 三树布局 | 兼容承诺；除非「只加 manifest」 |
| 非 skill CLI / schema / shortcut / whiteboard | **本分支不做** |
| 整份 installer 重写 | 只动 skill-install 路径 |
| 整支 merge 迁移分支 | 噪音大；只精简移植 |

---

## 4. 分期（均限制在 skill 内容 + skill 框架）

> 批准前 **零编码**。

### Phase 0 — 方案冻结（本文）

| | |
|---|---|
| **范围** | 本文件；§7 勾选 |
| **文件** | 仅 `docs/skill-wukong-align-plan.md` |
| **验收** | owner 按收窄后的 §7 重新批准 |

### Phase 1 — Skill 行为对齐（P0）

| | |
|---|---|
| **范围** | 移植 `402429ac`+`d5c8982c` 的 **skill 框架/行为**：setup 默认 multi；各面 **skill-install 路径**真装 multi；`LocateSkillsRoot` prefer `multi/`；upgrade skill 路径 always-multi + 互斥清理 |
| **可能触达** | `internal/app/skill_setup.go` (+tests)、`internal/upgrade/paths.go` (+multi tests)、upgrade 中调用 skill 刷新的薄封装、`scripts/install*.sh|ps1` 与 `build/npm/install.js` 的 **skill 段**、对应 `test/scripts/*skill*` / install smoke、README/CHANGELOG **skill 相关句** |
| **明确不碰** | 二进制下载/安装逻辑、非 skill CI、schema、产品 helpers、悟空发版脚本 |
| **Cherry-pick 噪音** | 见 §6.2：默认 **不**带入迁移分支整包 `docs/skill-*.md`；若需留档只保留与 skill 框架直接相关的短文或链接到本文 |
| **风险** | 中：默认布局翻转影响新装用户；需 CHANGELOG skill 段说明 |
| **验收** | ① `dws skill setup --yes` → multi；② fake-HOME 下 skill-install 默认 `dingtalk-*`、无 `dws/`；③ mono 机 upgrade 含 multi 包 → 仅 multi；④ mono opt-in 仍可用；⑤ 无取消产品 API；⑥ diff 无非 skill 产品文件 |

### Phase 2 — Skill 框架收敛：`skillhome` + 清单门禁

| | |
|---|---|
| **范围** | AgentHomes（补 opencode）、保留前缀、布局/互斥/stale；setup/upgrade/skill 命令断言对齐；脚本 **skill 清单块**标记 + 窄 policy |
| **可能触达** | `internal/skillhome/`、`skill_setup.go`、`paths.go`、`skill_command.go`、各脚本 skill 清单块、可选 `scripts/policy/check-agent-homes-sync.sh` |
| **不碰** | 非 skill policy 链大改造 |
| **验收** | Go 侧 skill home 单一权威；skill 相关测试绿；opencode 漂移修复有说明 |

### Phase 3 — Skill-install 路径降重（非整安装器重写）

| | |
|---|---|
| **范围** | 各安装面的 **skill-install 段**：优先 `dws skill setup --mode <resolved> --yes`；失败则 **冻结的 skill 拷贝 fallback**；二进制安装等逻辑保持不动 |
| **可能触达** | install.sh/ps1/js、install-skills.sh 的 skill 函数；skill 冒烟测试 |
| **暂缓** | install-event / install-devapp 的特殊双装语义（另案，仍属 skill 但非本阶段必做） |
| **验收** | skill 行为与 Phase 1 一致；非 skill 安装路径无行为回归 |

### Phase 4 — 可选：skill 包轻量 manifest

| | |
|---|---|
| **范围** | `dws-skills.zip` 根或 `multi/manifest.json`（version + layout）；list/Locate **跳过非 skill 目录** |
| **不碰** | 悟空 bundle 文件名、`_install.sh`、distribution overlay |
| **验收** | 旧消费方仍可装；可选读到 version |

### Phase 5 — Skill 清理 / mono 退役（后置、另批）

| | |
|---|---|
| **范围** | skill 脚本死代码、EXPERIMENTAL 文案；**物理删 `skills/mono`** 另开 retirement，不绑本对齐必达 |
| **验收** | 另案 |

---

## 5. Port / Adapt / Reject（悟空 · skill 相关）

| 悟空产物 / 概念 | 决策 | 说明 |
|---|---|---|
| flat `dingtalk-*` + `dws-shared` | **Port（语义）** | 已有 `skills/multi/` |
| 有 multi 则装/刷 multi | **Port（行为）** | 来自迁移两笔的 skill 部分 |
| `manifest.json` version | **Adapt（可选 Phase 4）** | 仅 skill 包自描述 |
| `_install.sh` symlink | **Reject** | 非 OSS skill home 模型 |
| `.dws-multiskill-*` 记账 | **Reject** | 依附 symlink |
| `build-bundle` / dual / Qwen overlay | **Reject / Out of scope** | 非 DWS skill zip 消费 |
| mono→multi 派生脚本 | **Reject** | OSS 直接维护 multi |
| 12 产品内容集 | **Reject** | 不合一 |
| bundle 校验脚本思路 | **Adapt（可选）** | 可做 OSS skill 打包断言，不必抄文件 |
| RewindDesktop / pod / Gaea / replace-wukong-skill | **Reject** | 客户端运维，非本分支 |

---

## 6. 既有提交复用（收窄）

### 6.1 禁止

- `git merge feat/skill-mode-migration`
- 引入取消产品实现
- 借机改 schema / shortcut / 非 skill CI

### 6.2 Cherry-pick / 移植规则

1. 基线保持 **`origin/main`**。
2. 允许 cherry-pick `402429ac` → `d5c8982c`，但落地时按路径过滤：
   - **保留**：`internal/app/skill_setup*.go`、`internal/upgrade/paths*.go`、upgrade 中 skill 刷新相关、安装脚本/npm 的 **skill-install 段**、对应测试、README 中 skill 默认说明的最小 diff。
   - **默认丢弃或另议**：迁移分支新增的整包 `docs/skill-architecture-optimization.md`、`skill-multi-roadmap.md`、`skill-wukong-comparison.md`、`skill-rollout-capability.md`、`skill-capability-completion.md`、`skill-distribution-mechanism.md`、`skill-multi-migration-plan.md` 等——**不以本分支交付**；结论已收敛进本文。若 owner 要留一份短 skill 文档，只保留与框架直接相关的摘录并指向本文。
   - **审查**：丢弃任何非 skill 文件误带；确认无 state.json / mode 命令 / header。
3. Phase 2+（`skillhome`）在 Phase 1 行为稳定后做，避免行为+重构搅在同一大提交。

### 6.3 与架构优化留档的关系（只采纳 skill 子集）

- **采纳**：skillhome、skill-install bootstrap、清单 policy、保留前缀、修 opencode。  
- **不采纳**：state.json 权威、symlink canonical、整安装器 L3 叙事中超出 skill 路径的部分。

---

## 7. 批准清单（收窄后 · 请 owner 重新勾选）

**范围确认**

- [ ] 认可本分支 **仅** skills 内容树 + skill 框架（§0.1）；§0.2 全部保持 out of scope  
- [ ] 认可 Non-goals：无 mode 切换 / state.json 产品 / telemetry header；无悟空客户端/pod 工作  

**Phase**

- [ ] **Phase 1**：移植两笔提交的 **skill 行为/框架代码**（可执行）  
- [ ] cherry-pick 时 **默认不带入**迁移分支整包 skill 长文档（只保留本文 + 必要 README/CHANGELOG skill 句）——同意 / 要保留某几份（请注明）  
- [ ] **Phase 2**（`skillhome` + opencode + skill 清单 policy）：与 Phase 1 **同 PR** / **拆 PR**  
- [ ] **Phase 3**（仅 skill-install 路径 bootstrap，非整 install.sh 重写）：本迭代做 / 行为对齐后再做 / 不做  
- [ ] **Phase 4**（skill zip 轻量 manifest）：做 / 不做 / 以后再说  

**显式不批准即不做**

- [ ] 确认不在本分支做：schema catalog、chat/whiteboard 等非 skill 产品、无关 CI、悟空 RewindDesktop/pod 发版  

---

## 8. 下一步

**请 owner 按 §7 重新批准。**  
批准前不编码。批准后默认：Phase 1（skill-only diff）→ 测试 → 再按勾选推进 Phase 2/3；Phase 4 单独决策。

---

*盘点锚点可按 §1 与 wukong `git show origin/develop:…` 复查。*
