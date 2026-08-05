# DWS multi-skill 框架对齐方案（相对 dws-wukong develop）

> 状态：**方案待 owner 批准后编码**。本文只做对齐盘点与分期建议，不含实现。
>
> 撰写日期：2026-08-05  
> 工作树：`/Users/john/GolandProjects/open-source/dws-multi-skill-align`  
> 分支：`feat/multi-skill-framework-align`（自 `origin/main` @ `a37e6e68`）  
> 对照仓：
>
> | 仓 | 路径 | 基线 |
> |---|---|---|
> | DWS OSS CLI（本工作树） | `dws-multi-skill-align` | `origin/main` |
> | 既有迁移试验分支 | `dws-skill-mode-migration` | `feat/skill-mode-migration` @ `d5c8982c` |
> | dws-wukong | `~/GolandProjects/open-source/dws-wukong` | `origin/develop` @ `ab76629a`（调研时） |
>
> 配套留档（勿当权威；悟空线已决策下线）：  
> `feat/skill-mode-migration` 上的 `docs/skill-wukong-comparison.md`、  
> `docs/skill-architecture-optimization.md`、`docs/skill-multi-roadmap.md`。

---

## 0. TL;DR（给 owner 的结论）

1. **DWS OSS 与悟空是两套分发链路**：OSS 走 `dws-skills.zip` + embed + 安装脚本/`dws skill setup`/`dws upgrade`；悟空走 `dws_res_*.zip` + `dingtalk-workspace(-bundle).zip` + 桌面端 bundled-skills。内容树也不同源（DWS multi 19 产品 vs 悟空 `dingtalk-skills/` 12 产品）。
2. **值得对齐的是「框架语义」**：flat multi 命名（`dingtalk-*` + `dws-shared`）、安装默认 multi、upgrade 有 multi 包时一次性刷 multi、安装面行为一致、减少清单/互斥逻辑多份拷贝。
3. **不值得原样移植的是悟空客户端产物形态**：`manifest.json` + `scripts/_install.sh` symlink 提升、`package-dual` / Qwen overlay、RewindDesktop 单 zip 消费约定——这些是 Qwen Work / 悟空端内契约，不是 OSS CLI 的消费模型。
4. **既有 `402429ac` / `d5c8982c` 已把默认 multi + upgrade 强制 multi 做完**，且已声明取消 `dws skill mode` / `state.json` / `x-dws-skill-mode`。建议在 main 上 **cherry-pick / 精简移植这两笔**，再做框架收敛（`internal/skillhome`），而不是从零重写。
5. **编码前需 owner 拍板**：是否引入 zip 内轻量 `manifest.json`（仅版本自描述）；Phase 1 是否立刻把脚本降级为 `dws skill setup` bootstrap（还是先行为对齐、后引擎合一）。

---

## 1. 现状盘点

### 1.1 DWS open-source CLI（`origin/main`）

| 层 | 现状 | 锚点 |
|---|---|---|
| 源树 | `skills/mono/` + `skills/multi/`（19 产品 + `dws-shared`） | `skills/` |
| embed | `//go:embed all:mono all:multi` | `skills/embed.go` |
| 发布 zip | `dws-skills.zip` = 根 mono 副本 + `mono/` + `multi/`（三树恒含） | `scripts/release/post-goreleaser.sh` |
| `dws skill setup` | `--mode mono\|multi`；**非交互默认 mono**；multi 标 EXPERIMENTAL | `internal/app/skill_setup.go` |
| `dws upgrade` | 刷新已装 skill；**尚未**「有 multi 包则强制刷 multi」 | `internal/upgrade/paths.go`（main 约 273 行，无 multi 专用刷新路径） |
| 安装面 | `install.sh` / `install.ps1` / `install-skills.sh` / npm `install.js`；`DWS_SKILL_MODE`，**非 TTY 默认 mono**；multi 多为提示/半装 | `scripts/`、`build/npm/` |
| Agent home 清单 | Go setup / upgrade / skill install / 各脚本各写一份；**opencode 已漂移**（`skill install` 有，setup/upgrade 无） | 见架构优化留档 §1 |
| 取消产品（已决策） | 无 `dws skill mode`；无 `state.json` 产品面；无 `x-dws-skill-mode` 头 | owner 2026-08-05 |

### 1.2 dws-wukong `develop` multi-skill 打包

| 件 | 作用 | 锚点（develop） |
|---|---|---|
| `dingtalk-skills/` | multi 源树：12 产品 + `dws-shared`（由仓内 mono 机械派生） | 仓根 |
| `scripts/build-bundle.sh` | 打 `dingtalk-workspace.zip`：**manifest.json + scripts/_install.sh + skills/\*** | `scripts/build-bundle.sh` |
| `scripts/_install.sh` | Qwen Work：把 `skills/*` **symlink 提升**到一层 skills 根；`.dws-multiskill-current` / `.dws-multiskill-links` 记账 | `scripts/_install.sh` |
| `manifest.json` | `{"version": "..."}`（overlay 时可加 distribution 等） | bundle 根 |
| `bundle` / `bundle-platform` / `package-dual` | multi bundle（+ 可选 mono 兜底双包） | `Makefile` |
| `real-platform` | 正式发版仍偏 mono `dingtalk-workspace.zip`（历史结论；以 develop Makefile 为准复核） | `Makefile` |
| 消费方 | Qwen Work Cloud 吃 bundle+`_install.sh`；RewindDesktop/Wukong.app 仍按「一 zip 一 skill」接线（历史对比文档） | 仓外 |

> 注：2026-08-05 决策「悟空 bundled-skill 分发线下线」。对齐目标变为 **吸收对 OSS 有用的框架概念**，而不是把 OSS 接到悟空发版链路上。

### 1.3 既有迁移分支（参考，非本分支基线）

`feat/skill-mode-migration` 上两笔核心提交：

| Commit | 摘要 |
|---|---|
| `402429ac` | 安装四面 + `dws skill setup` **默认 multi**；multi 真装；全量安装清 stale；upgrade 侧开始 prefer multi |
| `d5c8982c` | **upgrade 有 multi 包时始终刷 multi**（无磁盘 sticky / 无运行时切换）；文档同步取消项 |

另有大量设计留档（roadmap / architecture / wukong comparison）。本分支 **不从该 branch 继续开发**，只复用其结论与可 cherry-pick 的代码。

---

## 2. Diff：已对齐 / 分叉 / 勿移植

### 2.1 已大致同构（形态层面）

| 概念 | DWS | 悟空 |
|---|---|---|
| 产品 skill 命名 | `dingtalk-<product>` | 同 |
| 共享层 | `dws-shared` | 同 |
| 平铺目录 + 每目录 `SKILL.md` | `skills/multi/<name>/` | bundle 内 `skills/<name>/` |
| 双树并存过渡 | zip 含 `mono/` + `multi/` | dual 包含 monolith zip + bundle zip（可选） |

### 2.2 关键分叉

| 维度 | DWS main | 悟空 develop | 影响 |
|---|---|---|---|
| 默认安装布局 | **mono** | 端内仍偏 mono；Qwen 侧可装 bundle | OSS 用户默认仍拿单 skill |
| upgrade 布局策略 | 未强制 multi | 端内灰度替换单 zip；bundle 有自描述安装器 | OSS upgrade 可能 mono/multi 共存或刷回 mono |
| 包自描述 | zip **无** manifest | bundle **有** version manifest | OSS 缓存/升级难观测版本 |
| 安装器 | 多语言拷贝到各 agent home | `_install.sh` symlink 提升到单一 skills 根 | 消费模型不同 |
| 内容集合 | 19 产品 | 12 产品 | **不求内容合一**（悟空线下线后更无必要） |
| 事实源 | 本仓直接维护 multi | 从仓内 mono 派生 | 不共享 git 树 |
| 清单/互斥逻辑 | 4 语言 × 多面拷贝，已漂移 | 主要在 `_install.sh` + 端内 Rust | OSS 维护成本高 |

### 2.3 悟空侧应 **拒绝原样移植** 的部分

见 §5「reject」行。核心原则：

- OSS 不消费 `dingtalk-workspace-bundle.zip`，也不跑 Qwen 的 symlink 提升。
- 不做运行时 mode 产品、不做 state.json 产品面、不做 telemetry header。
- 不把 RewindDesktop / pod / Gaea 灰度链路拉进本仓。

---

## 3. 对齐目标与非目标

### 3.1 Goals（本分支批准后实施）

1. **行为**：安装默认 multi；`DWS_SKILL_MODE=mono` / `dws skill setup --mode mono` 仍为 opt-in；**upgrade 在包含 `multi/` 时始终刷 multi**（一次性迁移旧 mono，无 sticky）。
2. **框架**：收敛 agent-home 清单、保留前缀、互斥/stale 规则到单一 Go 事实源（建议包名 `internal/skillhome`）；setup 与 upgrade 共用；脚本清单用标记块 + policy 比对防漂移。
3. **命名/布局约定**：文档与代码统一「multi = flat `dingtalk-*` + `dws-shared`」；全量安装清 stale，带 `-s/-x` 保持 additive。
4. **（可选）包自描述**：在不破坏现有三树 zip 布局的前提下，评估增加轻量 `manifest.json`（仅 version / layout 字段），供升级与诊断读取——**需 owner 单独拍板**。
5. **测试**：setup/upgrade 单测 + install.sh fake-HOME 冒烟；不引入直播遥测。

### 3.2 Non-goals（明确不做）

| 项 | 原因 |
|---|---|
| `dws skill mode` 生命周期切换 | 已取消 |
| `~/.dws/skills/state.json` 产品面 | 已取消（upgrade 按包内容驱动即可） |
| `x-dws-skill-mode` 请求头 | 已取消 |
| 移植 `_install.sh` symlink 模型为默认 | 架构评估已否决 symlink canonical；与多 agent home 拷贝模型冲突 |
| 移植 `package-dual` / Qwen overlay / platform-overlays | 客户端/云端分发专用 |
| 强行统一 DWS multi(19) 与悟空 dingtalk-skills(12) 内容 | 悟空线内容同源问题 MOOT |
| 大改 `dws-skills.zip` 三树布局 | 兼容承诺（D1）；除非 owner 批准「加文件不改树」 |
| 从 `feat/skill-mode-migration` 整支 merge | 基线旧、文档噪音大；应 cherry-pick 行为提交 |

---

## 4. 分期建议（Phase 0..N）

> 批准前 **零编码**。下列阶段供拍板范围与顺序。

### Phase 0 — 方案冻结与移植策略（本文）

| | |
|---|---|
| **范围** | 本文件；确认 port/adapt/reject；确认是否做 zip manifest |
| **改动文件** | 仅 `docs/skill-wukong-align-plan.md` |
| **风险** | 无 |
| **验收** | owner 书面批准 Goals/Non-goals 与 Phase 1 范围 |

### Phase 1 — 行为对齐：默认 multi + upgrade 强制 multi（P0）

| | |
|---|---|
| **范围** | 复用 `402429ac` + `d5c8982c` 的**行为部分**：setup 默认 multi；install.sh/ps1/install-skills.sh/install.js 真装 multi 且默认 multi；`LocateSkillsRoot` prefer `multi/`；`UpgradeSkillLocations` 有 multi 则始终刷 multi 并清 mono leftover / stale |
| **可能触达** | `internal/app/skill_setup.go` (+tests)、`internal/upgrade/paths.go` (+`paths_multi_test.go`)、`internal/app/upgrade*.go`、`scripts/install.sh`、`scripts/install.ps1`、`scripts/install-skills.sh`、`build/npm/install.js`、`test/scripts/install_script_test.go`、`test/scripts/install_js_smoke.mjs`、README(_zh) 相关段落；可选精简移植 roadmap 文档 |
| **风险** | 中：curl\|sh 非 TTY 用户从 mono 翻到 multi；需文档与 CHANGELOG 说清；install.sh 注意避免再引入死代码双定义 |
| **验收** | ① 非交互 `dws skill setup --yes` → multi；② fake-HOME `install.sh` 默认装出 `dingtalk-*` 无 `dws/`；③ 已装 mono 后 upgrade 含 multi 的包 → homes 仅 multi、无 `dws/`；④ `DWS_SKILL_MODE=mono` / `--mode mono` 仍可用；⑤ 无 `skill mode` / state.json / header 新 API |

### Phase 2 — 框架收敛：`internal/skillhome` + 清单门禁（P0/P1）

| | |
|---|---|
| **范围** | 抽出 AgentHomes（**补齐 opencode**）、保留前缀/名称、布局与互斥/stale 规则；setup/upgrade 改引用；脚本清单包在 `DWS-AGENT-HOMES-BEGIN/END`；新增 `scripts/policy/check-agent-homes-sync.sh`（可先 warn-only） |
| **可能触达** | 新 `internal/skillhome/`、`skill_setup.go`、`paths.go`、`skill_command.go`（断言对齐）、各 install 脚本清单块、`Makefile` policy |
| **风险** | 低–中：补 opencode 会改变「`--target all` / upgrade」覆盖面（这是修漂移，需在 CHANGELOG 写明） |
| **验收** | Go 侧仅一处清单权威；policy 能检出人为改脚本漏改；现有 setup/upgrade 测试绿 |

### Phase 3 — 安装面降重：脚本 → bootstrap + 冻结 fallback（P1，可选加快）

| | |
|---|---|
| **范围** | install.sh / ps1 / install.js / install-skills.sh：优先 `dws skill setup --mode <resolved> --yes`；失败才走**冻结**内嵌拷贝；不再演进脚本内安装语义 |
| **可能触达** | 上述脚本 + CI smoke；**暂不动** install-event / install-devapp（语义仍双装，需单独收敛） |
| **风险** | 中：二进制不可用、`DWS_SKILLS_ONLY=1`、npm ignore-scripts 等路径必须 fallback 可靠 |
| **验收** | 脚本冒烟与 Phase 1 行为一致；语义变更只改 Go 引擎一处即可反映到四面 |

### Phase 4 — 可选：zip/embed 轻量 manifest（adapt 悟空，非必须）

| | |
|---|---|
| **范围** | 在 `dws-skills.zip` **不破坏三树**前提下增加根级或 `multi/manifest.json`（`version` + `layout: multi-flat`）；打包脚本写入；setup/upgrade **忽略非 skill 目录**；诊断可打印版本 |
| **可能触达** | `post-goreleaser.sh`、可选 `skills/multi/manifest.json` 模板、Locate/list 过滤、文档 |
| **风险** | 低（若严格「只加文件」）；中（若有工具假设 multi/ 下全是 skill 目录——需审计 `listMultiSkillNames`） |
| **验收** | 旧消费方不解 manifest 仍能装；新路径能读到 version；无 symlink 安装器 |

### Phase 5 — 清理与 mono 退役准备（后置）

| | |
|---|---|
| **范围** | 删脚本死代码与重复拷贝；统一 EXPERIMENTAL 文案；评估何时物理删 `skills/mono`（独立 retirement，不绑本对齐） |
| **风险** | 删 mono 影响大，需独立 RFC/版本窗 |
| **验收** | 另案 |

---

## 5. Port / Adapt / Reject 表（悟空产物）

| 悟空产物 / 概念 | 决策 | 说明 |
|---|---|---|
| flat `skills/<dingtalk-*>` + `dws-shared` 布局 | **Port（语义）** | OSS 已有 `skills/multi/`；保持命名与平铺约定 |
| 「有 multi 则装/刷 multi」包驱动策略 | **Port（行为）** | 对齐 `402429ac`/`d5c8982c`；非 sticky |
| `manifest.json`（version） | **Adapt（可选 Phase 4）** | 只取自描述版本；不引入 distribution/overlay 字段除非有 OSS 消费者 |
| `scripts/_install.sh` symlink 提升 | **Reject** | Qwen 单根 skills 模型；与 OSS 多 agent home 拷贝冲突；架构文档已否决 symlink canonical |
| `.dws-multiskill-current` / `.dws-multiskill-links` | **Reject** | 依附 symlink 安装器 |
| `build-bundle.sh` / `bundle-platform` / `package-dual` | **Reject** | 产出 `dingtalk-workspace-bundle.zip`，OSS 不消费 |
| Qwen `platform-overlays` / `apply-platform-overlay.py` | **Reject** | 云端分发专用 |
| `sync-monolith-to-multiskill.py` 派生流水线 | **Reject** | OSS 直接维护 `skills/multi/`；无需从 mono 再派生 |
| `dingtalk-skills/` 12 产品内容集 | **Reject（内容合一）** | 集合不同；悟空线下线后无统一压力 |
| `validate-multiskill-bundle.py` | **Adapt（思路）** | 可借鉴「打包后断言 SKILL.md 数量/命名」做 OSS policy，不必抄脚本 |
| RewindDesktop / pod / Gaea 更新链 | **Reject** | 仓外客户端；非本仓范围 |
| `replace-wukong-skill.sh` | **Reject** | 悟空运维脚本 |

---

## 6. 既有提交如何复用（`402429ac` / `d5c8982c`）

### 6.1 不要做的事

- **不要** `git merge feat/skill-mode-migration`（历史基线旧、含大量已过时/已取消设计叙述与中间态）。
- **不要**在本分支继续堆「skill mode / state.json / header」实现。

### 6.2 推荐做法

1. 本分支保持基于 **`origin/main`**（已建好）。
2. Phase 1 对两笔提交执行 **`git cherry-pick`（或等价手工移植）**：
   - 先 `402429ac`（默认 multi + 安装面真装 + setup/upgrade 初版 multi 路径）
   - 再 `d5c8982c`（upgrade always-multi、去掉 sticky 叙述）
3. Cherry-pick 后立刻做：
   - 解决与 main 新进文件的冲突（main 已前进大量 schema/命令框架提交）。
   - **删掉/不采纳**任何重新引入取消产品的片段（预期这两笔已是「取消后」版本，仍需 diff 审查）。
   - 去掉多余文档噪音或改为指向本文；若保留 roadmap，须与 Non-goals 一致。
4. Phase 2+ 在 cherry-pick 稳定后再做 `internal/skillhome`，避免「大行为变更 + 大重构」同一提交搅在一起。

### 6.3 与架构优化文档的关系

`docs/skill-architecture-optimization.md`（迁移分支）中的：

- **采纳方向**：L1 `skillhome`、脚本 bootstrap、清单 policy、保留前缀正式化、修 opencode 漂移。
- **不采纳**：以 state.json 为权威安装状态（已取消）；symlink canonical。

即：**Phase 1 = 迁移分支行为提交；Phase 2–3 = 架构优化的去 state 子集。**

---

## 7. 建议的批准清单（请 owner 勾选）

- [ ] Goals / Non-goals（§3）认可  
- [ ] Phase 1 立即做（cherry-pick 两笔行为提交）  
- [ ] Phase 2（`skillhome` + opencode + policy）是否与 Phase 1 同 PR 或拆 PR  
- [ ] Phase 3（脚本 bootstrap）是否本迭代做，还是行为对齐后再做  
- [ ] Phase 4（zip `manifest.json`）做 / 不做 / 以后再说  
- [ ] 文档：是否把迁移分支多份 skill-*.md 精简进本分支，或只保留本文 + CHANGELOG  

---

## 8. 下一步

**等待 owner 批准本文后再编码。**  
批准后默认执行顺序：Phase 1 → 测试 →（可选同 PR 或后续）Phase 2 → Phase 3；Phase 4 单独决策。

---

*盘点锚点均可按 §1 路径与 wukong `git show origin/develop:…` 复查。*
