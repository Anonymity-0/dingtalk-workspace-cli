# Skill 能力缺口分析与补全计划

> 基于 `feat/skill-mode-migration` 工作区代码级盘点（2026-08-05）。本工作区已把
> install / upgrade / setup 的默认形态翻转为 multi；本文回答两个问题：
> **管理能力上还缺什么**（Part 1）、**multi 内容是否覆盖 mono**（Part 2），
> 并给出优先级排序的实施清单（Part 3）。
> 任务 ID 复用 [skill-multi-migration-plan.md](skill-multi-migration-plan.md) §8.2
> （P0a/P0b/P0c/P0-3/P0-4/P1-1/P1-2/P2-x/W4）；新增项以 `P1-3`、`C1–C7` 编号。
> 机制背景见 [skill-distribution-mechanism.md](skill-distribution-mechanism.md)。

## 0. 结论速览

- **管理面**：默认翻转已落地（setup/四个安装脚本/upgrade 识 multi），但生命周期
  能力仍缺 6/8 项：无状态（state.json）、无备份回滚、无卸载、无故障恢复、
  upgrade 不按已装清单做增量、市场/内置边界靠前缀约定。当前形态 = "能装上
  multi，但装成什么样、出了事怎么退，全靠运气"。
- **内容面**：multi 对产品参考的覆盖**总体是 mono 超集**（chat/event/dev/sheet
  均更详细），但 mono 有 **4 块全局能力在 multi 完全缺失**（recovery 闭环、
  确认门禁协议、Schema 渐进查询教学、LICENSE/NOTICE），另有 1 个脚本
  （`report_inbox_today.py`）未迁移；multi 自身还有 6 项内部不一致（死链、
  orphan 脚本、漏改 EXPERIMENTAL 文案等）。
- **工作量**：管理面 P0–P2 剩余 ≈ 10.5–13 人日；内容补全 C1–C7 ≈ 3.5–4 人日；
  合计 ≈ 14–17 人日，与 plan §8.1 的 13–17 人日口径一致（内容项是新增量）。

---

## Part 1 — 管理能力缺口（skill 生命周期管理）

### 1.1 现状盘点（本工作区实际状态）

**`dws skill setup`（内置 skill 安装）**

| 能力 | 状态 | 锚点 |
|---|---|---|
| mode 选择 | mono / multi；无 `--mode` 时 TTY 交互（multi 为默认项）、非 TTY 默认 multi | `internal/app/skill_setup.go:343-376` |
| 按产品挑选 | `-s/--skill`、`-x/--exclude` 互斥；`dws-shared` 强制包含；未点名的已有 `dingtalk-*` 保留（additive） | `skill_setup.go:101-102`、`254-318`、`209-226`、`640-670` |
| `--dry-run` | 有，预览 mode/来源/目标/子 skill，不写文件（root 持久 flag 注入，`internal/app/flags.go:43`） | `skill_setup.go:156-167` |
| 源 | `--source` / `DWS_SKILL_SOURCE` 显式覆盖（失败不回退）→ 默认 embed（与二进制同版） | `internal/app/skill_setup_embed.go:50-58`、`skills_embed.go:28` |
| 目标 | 16 个 agent home，父目录门控；`--target all` 不含 opencode，但命名 target 含（不对称） | `skill_setup.go:20-37`、`509-522` vs `internal/app/skill_command.go:129` |
| 互斥清理 | 装 mono 删 `dingtalk-*`+`dws-shared`，装 multi 删 `dws/`；**best-effort，失败仅 warning 继续装** | `skill_setup.go:570-608` |
| 备份/记账 | **无**。`RemoveAll` 直删，不写任何状态 | `skill_setup.go:616`、`655` |

**`dws skill search / get / install`（市场 skill）**

- 子命令全集仅 `get / install / search / setup`（+ 隐藏的 find/add 兼容提示），
  **无 status / mode / remove / rollback**（`skill_command.go:202-209`）。
- `install` 解压到 agent skills **根目录**（非 `dws/` 子目录），无 mode 概念、
  无记账、无覆盖保护（`skill_command.go:378-443`、`653-672`）。

**`dws upgrade`（skill 刷新）**

- `LocateSkillsRoot` **优先 zip 内 `multi/`**（`internal/upgrade/paths.go:363-369`，
  接线于 `internal/app/upgrade.go:60`）；`UpgradeSkillLocations` 按包布局自动
  识别 mono/multi（`paths.go:129-139`）。
- multi 路径：平铺刷新 `<agent>/dingtalk-*`+`dws-shared`，清 `dws/` 残留与过期
  `dingtalk-*`，best-effort 刷 `~/.dws/skills/multi` 缓存
  （`paths.go:217-299`、`301-327`、`290-296`）。
- **不读任何已装状态**：zip 恒含 multi/ → 存量 mono 用户升级即被**静默迁移为
  multi**（无确认、无备份、无 state 记录），违反 plan §3.8「升级永远尊重用户
  当前 mode」的粘性原则。`~/.dws/skills/mono` 缓存升级时不刷新。
- skill 安装发生在**二进制替换之后**，skill 失败时二进制已换 → 半升级态
  （`upgrade.go:593-611`）。`dws upgrade --rollback` 只回滚二进制
  （`upgrade.go:160`、`312`）。

**安装脚本（7 面）**

| 面 | 本工作区状态 |
|---|---|
| `scripts/install.sh` | 已默认 multi 且**真装**（`install_multi_skills_to_homes`），mono 为 opt-in；`rm -rf` 直删无备份 |
| `scripts/install.ps1` | 同上（Windows） |
| `scripts/install-skills.sh` | 已加 `DWS_SKILL_MODE`（默认 multi）+ multi 安装 |
| `build/npm/install.js` | 已加 `installMultiSkillsToHomes`（互斥清理 + 平铺） |
| Homebrew formula | 不铺 agent home，caveats 引导 `dws skill setup` |
| `scripts/install-event.sh` / `install-devapp.sh` | 单 skill 专项语义，缓存 `multi/dingtalk-event` / `multi/dingtalk-dev` 子集 |
| 共同缺口 | **均不写 state.json、均无备份**；agent home 清单仍是 sh/ps1/js/Go 多份手写（`paths.go:23-52` 的 keep-in-sync 注释约定，无门禁） |

### 1.2 缺口表（对照完整生命周期）

严重度：🔴 高（可致数据丢失/双份派发/半装态）｜🟡 中（能力缺失但有绕行）｜🟢 低（体验项）

| # | 生命周期项 | 现状 | 严重度 | 补全设计（复用 plan §8 ID） |
|---|---|---|---|---|
| 1 | **status 查询** | **无**。无 state.json、无 `dws skill mode/status`；判断 mode 只能人工看磁盘形态（`dws/` vs `dingtalk-*`） | 🔴 | **P0b-1** 新增 `~/.dws/skills/state.json`（schema_version/mode/cli_version/installed/agent_homes/previous），setup 与安装脚本写入；**P0b-2** 缺失/损坏时磁盘形态反推，双形态并存 → drift 报错；**P1-1** `dws skill mode`（status 子命令展示 mode/版本/已装列表/上次切换/备份） |
| 2 | **切换** | 有（`setup --mode`），但**无状态、无记账**：切完不留 previous，互斥清理失败仅 warning 继续装（`skill_setup.go:600-608`）→ 可能 mono+multi 双份共存、Agent 双份派发 | 🔴 | **P0b** 记账（含 `previous`）；**P0-3** 把「清理失败继续装」改为「失败整体回滚」；**P1-1** `mode set <mode>` 复用 setup 安装实现 + 备份 + 记账，成功后提示重启 AI 工具 |
| 3 | **回滚** | **无备份**。setup/upgrade/安装脚本全部 `RemoveAll`/`rm -rf` 直删（`skill_setup.go:616,655`、`paths.go:181,249,307`、`install.js` `fs.rmSync`）；`upgrade --rollback` 只回二进制 | 🔴 | **P0-3** 备份式安装：`RemoveAll` → `mv` 到 `~/.dws/skills/backup/<ts>-<mode>/`，保留最近 2 份，任一 home 失败自动恢复并非 0 退出；**P1-1** `mode rollback` 一条命令回到 `state.previous` |
| 4 | **版本对齐** | embed 天然同版（setup 默认源，`skill_setup_embed.go:50-58`）✓；但 `~/.dws/skills` 缓存**只写不读**（仅 legacy 回退候选，`skill_setup.go:445-447`），upgrade 只刷 multi 缓存不刷 mono（`paths.go:290-296`），漂移不可见 | 🟡 | **P0b-1** state.json 记 `cli_version` 使漂移可见；**P0a-1** upgrade 按 mode 同步刷新对应缓存（或评估废弃缓存，plan §6 风险表末行） |
| 5 | **卸载** | **无 remove**。skill 子命令仅 get/install/search/setup（`skill_command.go:202-209`）；用户只能手动删目录，且不知道该删哪些（16 个 home × N 个 skill） | 🟡 | **新增 P1-3** `dws skill remove`：按 state.json 的 `agent_homes`×`installed` 精确删除内置 skill（`dingtalk-*`+`dws-shared`+`dws/`），不动市场 skill；`--dry-run` 预览（依赖 P0b） |
| 6 | **局部更新** | setup 侧 additive 语义完整（`-s/-x`，未点名保留）；但 **upgrade 增量语义未按 `state.installed`**：`UpgradeSkillLocations` 用 zip 内 bundle 全集刷新（`paths.go:135-137`、`339-358`），用户 `-x` 排除过的 skill 会被升级装回来 | 🔴 | plan §8.3-1 已定死语义（以 `state.installed` 为准增量刷新、未装不补装）；**P0b-1** 先落 `installed` 列表，**P0a-1** `UpgradeSkillLocations(dir, mode)` 按其过滤 |
| 7 | **故障恢复** | **无**。setup 清理/拷贝失败仅 warning 或按 home 跳过，留半装态；upgrade 的 skill 失败发生在二进制替换之后（`upgrade.go:593-611`），半升级态无自动恢复、无修复命令 | 🔴 | **P0-3** 备份式安装 + 失败整体回滚（改变「warning 继续装」语义，需同步改 `skill_setup_full_coverage_test.go` 多处断言，plan §8.3-2）；**P0b-2** drift 检测给显式收敛指令；**P1-1** `mode rollback` |
| 8 | **市场 vs 内置边界** | **隐性前缀约定**：互斥清理按 `dingtalk-`/`dws-shared` 名称扫描（`paths.go:332-334`、`skill_setup.go:581`），无 SKILL.md frontmatter 校验 —— 市场 skill 若同名前缀会被误删；反向地，市场 `install` 解压无保护，可覆盖内置 `dingtalk-*`（`skill_command.go:653-672`） | 🟡 | plan §6 风险行：清理前校验目录内 SKILL.md frontmatter 属 DWS 产品集并写成测试（落入 **P0-3** 的清理改造）；长期由 state.json 的 `installed` 清单取代前缀扫描（P0b 后续） |

**附：本工作区已完成项**（不再列入缺口）：setup 默认 multi（P2-1）、
install.sh/ps1 真装 multi（P2-2）、install-skills.sh/install.js mode 支持
（P2-3）、README×2 与 dingtalk-skill 文案（P2-4 部分）、upgrade 识别并刷新
multi 包（P0a 的布局识别一半）。**尚未做**：P0c 清单收敛、P0b state.json、
P0-3 备份、P0a 的 mode-aware 一半、P0-4 请求头、P1-1 mode 命令、P1-2 beta 轨。

---

## Part 2 — 内容能力对等（mono vs multi）

### 2.1 树对比总览

| 维度 | mono | multi |
|---|---|---|
| 规模 | 152 文件 / ≈2.5 MB | 244 文件 / ≈3.3 MB（19 个 `dingtalk-*` + `dws-shared`） |
| 入口 | 单 `SKILL.md`（332 行，含全局路由/危险表/Schema 教学） | 每产品一个 `SKILL.md` + `dws-shared` 全局契约（90 行） |
| 全局参考 | `references/` 10 项（intent-guide、global-reference、url-patterns、error-codes、capability-limits、channel-login、field-rules、recovery-guide、best_practices/、products/） | `dws-shared/references/` 9 项 + 各产品 skill 自带 |
| 脚本 | `scripts/` 37 个 | 10 个 skill 带 `scripts/` 共 55 个 |
| 最佳实践 | `best_practices/` 01–11 + lite + `_common` | 按产品分发（01→chat … 11→minutes），`_common` 与 lite 入 `dws-shared` |

### 2.2 产品参考覆盖核对

逐产品比对结论：**multi 覆盖 mono 全部产品参考，且多数为超集**。

| mono 产品参考 | multi 对应物 | 结论 |
|---|---|---|
| aitable.md + aitable/（20）+ aitable-record-ops | `dingtalk-aitable`（同 20 子章节 + field-rules + 06-data-analytics） | ✅ 超集 |
| chat.md + chat-emoji-list | `dingtalk-chat`（+ chat/ 5 个子章节） | ✅ 超集 |
| calendar / contact / doc+doc/ / drive / mail / minutes / todo / wiki / aisearch / hrbrain / pat / markdown | 同名 `dingtalk-*` skill | ✅ 覆盖（doc 侧 mono 的 doc-file-ops/doc-list/doc-permission/doc-search 四篇已迁移为 `dingtalk-doc/references/doc.md:202-207` 的 drive/wiki 迁移表，属刻意重构） |
| event.md（222 行） | `dingtalk-event`（event-im.md 399 行 + 完整订阅治理契约） | ✅ 超集 |
| dev.md（212 行） | `dingtalk-dev`（12 篇 reference 共 584 行） | ✅ 超集（结构重组） |
| oa / attendance / report / sheet / ding / devdoc / agoal | `dingtalk-misc` 对应 reference（sheet 多 3 篇：comment/formula/version） | ✅ 超集 |
| simple.md（devdoc+oa 合集 + 意图判断 + 上下文传递表） | 已拆入 `dingtalk-misc/references/oa.md:309,425` 与 `devdoc.md:15,19` | ✅ 覆盖 |
| 多组织/多账号（SKILL.md:64-70 一节） | `dingtalk-profile` 整个 skill | ✅ 超集 |
| 意图决策树（SKILL.md:101-123） | `dws-shared/references/intent-guide.md`（536 行 ≥ mono 488 行）+ `routing.md` | ✅ 覆盖 |
| best_practices 01–11 / lite / _common | 按产品分发 + `dws-shared/references/best_practices/_common/` | ✅ 覆盖 |
| url-patterns / capability-limits / channel-login / error-codes | `dws-shared/references/` 同名 | ✅ 覆盖 |
| field-rules.md（mono 全局位） | `dingtalk-aitable/references/field-rules.md` | ✅ 合理下沉（内容即 AI 表格字段规则） |

### 2.3 不对等项清单（mono 有、multi 无）

| # | 缺失项 | mono 锚点 | multi 现状证据 | 严重度 | 补齐方式（承载方） |
|---|---|---|---|---|---|
| M1 | **Recovery 闭环**（recovery-guide.md + global-reference §Recovery + 错误处理第 2 步） | `skills/mono/SKILL.md:296,311`、`references/recovery-guide.md`、`global-reference.md:62` | multi 全树 `RECOVERY_EVENT_ID` 零引用；`dws-shared/SKILL.md:83-89` 错误最短路径无 recovery；`dingtalk-dev/SKILL.md:124` 指向「root dws / dws-shared 的错误处理」→ **断链** | 🔴 | **dws-shared**：新增 `references/recovery-guide.md`（从 mono 移植）、SKILL.md 错误最短路径加 recovery 步、`global-reference.md` 补 Recovery 节（→ C1） |
| M2 | **确认门禁协议 + 全局危险操作表** | `skills/mono/SKILL.md:137-187`（危险操作表 + 确认流程 + `confirmation_required` 识别与重试协议） | multi 仅 aitable/doc/misc 三个 SKILL.md 有产品级危险表；`confirmation_required` / 「确认门禁」全树零命中；`dws-shared/SKILL.md:37-38` 只有一行泛化规则 | 🔴 | **dws-shared**：全局确认门禁协议（识别 `confirmation_required`、原始命令追加 `--yes` 重试、`--dry-run` 预览、禁止管道喂答案）+ 危险操作索引指向各产品表（→ C2） |
| M3 | **Schema 渐进查询教学** | `skills/mono/SKILL.md:198-292`（≈95 行：四层查询、`--compact`/`--all` 边界、字段速查、Schema/Help/业务数据边界表、漂移处理） | 各产品 SKILL.md 仅点状提及 leaf Schema（17 处）；`dws-shared/references/global-reference.md:79-89`「命令自省」只有 `--help` | 🟡 | **dws-shared**：新增 Schema 渐进查询章节（改写为 multi 语境，链入渐进加载表）（→ C3） |
| M4 | **scripts/report_inbox_today.py** | `skills/mono/scripts/report_inbox_today.py` | multi 无（misc 仅有 `report_received_today.py`）；mono 文档也未引用它 | 🟢 | **dingtalk-misc**：先验证脚本仍可用 → 迁入 `scripts/` 并在 `report.md` 引用；不可用则连同 mono 侧一起删（→ C5） |
| M5 | **LICENSE / NOTICE** | `skills/mono/LICENSE`、`NOTICE` | multi 20 个 skill 均无 | 🟡 | 每个 multi skill 根复制两份（或 release 打包期注入，`scripts/release/post-goreleaser.sh`）（→ C7） |
| M6 | **aiapp 意图路由** | `skills/mono/SKILL.md:76,101`（产品表 + 决策树有 aiapp 行，但目标 `aiapp.md` 不存在 —— mono 自身死链） | multi 全树无 aiapp 路由/文档；仅 orphan 脚本 `dingtalk-misc/scripts/aiapp_create_and_poll.py` | 🟡 | 决策：**dws-shared/routing.md** + **dingtalk-misc** 产品索引补 aiapp 行并新建 reference，或明确下线该能力并清掉 mono 死链与 orphan 脚本（→ C5） |

### 2.4 multi 自身不一致项（不阻塞对等结论，但阻塞「multi 可独当一面」）

| # | 问题 | 证据 | 处理 |
|---|---|---|---|
| X1 | 16 个 orphan 脚本无任何文档引用（yida×13、finance×2、aiapp×1）；`dws-shared/references/routing.md` 把「宜搭」路由到 dingtalk-misc，但 misc 产品索引无 yida 行、无 yida reference | `skills/multi/dingtalk-misc/scripts/`；`dingtalk-misc/SKILL.md:20-32` | 补文档（misc 产品索引 + reference）或移出发布包（→ C5） |
| X2 | 死链：`dingtalk-chat/SKILL.md:131` 引用 `scripts/extract_media_id.py`，文件不存在（mono 也无） | 同上 | 补脚本或删引用（→ C4） |
| X3 | 死链：`dws-shared/references/routing.md:22` 指向 `dingtalk-misc/references/markdown.md`，实际在 `dingtalk-markdown` | 同上 | 改指 `../../dingtalk-markdown/SKILL.md`（→ C4） |
| X4 | `dingtalk-event/SKILL.md` 缺 `dws-shared` PREREQUISITE 与 `metadata:` 块（其余 19 个 skill 均有） | `skills/multi/dingtalk-event/SKILL.md:1-4` | 补齐（→ C4） |
| X5 | 4 个 skill 仍是 🧪 EXPERIMENTAL + 「生产优先 mono」文案，与本工作区已翻转的默认矛盾（dingtalk-skill 已改，这 4 个漏改） | `dingtalk-profile/SKILL.md:15`、`dingtalk-hrbrain:15`、`dingtalk-markdown:15`、`dingtalk-pat:15` | 统一下调文案（→ C4，即 plan P2-4 剩余量） |
| X6 | `<!-- SAFETY_PREAMBLE_INJECT -->` 标记存在于 5 个 SKILL.md，但仓库内无注入器 | `dingtalk-{pat,hrbrain,markdown,skill,profile}/SKILL.md` | 明确注入方（仓外流程则写注释）或移除标记（→ C4） |

### 2.5 测试与政策门覆盖

| 门面 | 覆盖 | 缺口 |
|---|---|---|
| `test/skill_tests.md` 覆盖表（40-54 行） | 13 产品 ≈256 用例 | 12/13 行引用 **mono 路径**（`references/products/...`），仅 dev 指 multi；`workbench` 行指向不存在的 `workbench.md`（54 行，mono/multi 均无）；`devdoc` 行指 `simple.md`（47 行，multi 无此文件）；未覆盖 doc/drive/mail/minutes/oa/sheet/wiki/aisearch/hrbrain/markdown/pat/profile/skill。multi 默认后需按 multi 路径重写并补产品（→ C6） |
| `scripts/policy/check-skill-context-budget.sh` | 锁 `dingtalk-chat/SKILL.md` ≤14000B + shortcut 区块不膨胀 + mono SKILL.md 不含「充分阅读产品参考文件」回归（9-38 行）；`gen_skill_shortcut_sections.py --check` 同时写 mono 与 multi 的 shortcut 区块 | mono 下线判据（plan §5-4）要求先替代对 `skills/mono/SKILL.md` 的依赖 |
| `make skill-command-integrity`（`check-skill-commands.sh` → `test/skill_static/skill_static_test.go:119-133`） | 静态校验 mono+multi 两树文档中的命令与 flag | 覆盖 OK；X2/X3 类 reference 死链不在其校验面 |

**Part 2 结论**：multi **没有**覆盖 mono 的全部能力 —— 产品参考层面是超集，
但全局能力缺 M1–M3 三块（recovery / 确认门禁 / Schema 教学），加 M4–M6 三个
小项；另有 X1–X6 六项 multi 内部不一致。补齐全部落在 `dws-shared`（M1/M2/M3）、
`dingtalk-misc`（M4/M6/X1）、各产品 skill（X2/X4/X5）与打包流程（M5）。

---

## Part 3 — 优先级排序实施清单

> 估时以 1 名熟悉本仓库的工程师计（人日），口径同 plan §8。
> 「状态」列：✅ 本工作区已完成｜🚧 部分完成｜⬜ 未做。

### 3.1 P0 — 先堵会丢数据/双份派发的洞（≈6.5–8.5d）

| ID | 状态 | 任务 | 文件级改动点 | 估时 | 依赖 |
|---|---|---|---|---|---|
| P0c-1 | ⬜ | agent home 清单下沉共享包，补 opencode 不对称 | 新 `internal/skillhome`（或 `internal/upgrade` 导出）：合并 `paths.go:35-52` `knownSkillDirs` + `skill_setup.go:20-37` `skillSetupAgentHomes` + `skill_command.go:119-141` `agentSkillPaths`；setup/upgrade/测试共用 | 0.5d | — |
| P0c-2 | ⬜ | 清单同步门禁 | 新 `scripts/policy/check-agent-homes-sync.sh`（进 `make policy`）；install.sh:ps1:install.js:install-skills.sh 清单改可解析块 | 1d | P0c-1 |
| P0b-1 | ⬜ | state.json schema + 写入 | 新 `internal/app/skill_state.go`（schema 见 plan §3.2）；`skill_setup.go` `runSkillSetup` 安装成功后记账；install.sh/ps1/install.js 写同构 JSON | 1d | — |
| P0b-2 | ⬜ | 磁盘形态反推 + drift 报错 | `skill_state.go`：`dws/`→mono、`dingtalk-*`→multi、两者都有→报 drift 并提示 `dws skill mode set` 收敛；单测 | 0.5–1d | P0b-1 |
| P0-3 | ⬜ | 备份式安装 + 失败整体回滚 | `skill_setup.go` `installSkillToHomes:610` / `installMultiSkillToHomes:640` / `cleanupMutualExclusion:600`：`RemoveAll`→`mv` 至 `~/.dws/skills/backup/<ts>-<mode>/`，保留 2 份，任一 home 失败自动恢复 + 非 0 退出；清理前加 SKILL.md frontmatter 校验（缺口 #8）；同步改 `skill_setup_full_coverage_test.go` 语义断言 + 故障注入测试 | 1.5–2d | — |
| P0a-1 | 🚧 | upgrade mode-aware | `paths.go:129` `UpgradeSkillLocations(dir, mode)` 按 state.json：mono→现状逻辑+清 `dingtalk-*`+刷 mono 缓存；multi→**按 `state.installed` 增量刷新**（缺口 #6）+清 `dws/`+刷 multi 缓存；无 state 按 P0b-2 反推；复用 P0c-1 共享清单 | 1.5d | P0c-1、P0b |
| P0a-2 | 🚧 | upgrade×multi 集成测试 | `internal/upgrade/paths_multi_test.go`（已有 6 个测试）扩：装 multi→upgrade→无 `dws/` 残留、`-x` 排除项不被装回、mono 存量不被静默迁移 | 1d | P0a-1 |
| P0-4 | ⬜ | `x-dws-skill-mode` 请求头 | `internal/auth/oauth_helpers.go` 附近仿 `x-dws-channel`（≈30 行） | 0.5d | P0b |

### 3.2 P1 — 生命周期命令 + 内容补全（≈7–7.5d，两条线可并行）

**管理命令线**

| ID | 状态 | 任务 | 文件级改动点 | 估时 | 依赖 |
|---|---|---|---|---|---|
| P1-1 | ⬜ | `dws skill mode` status/set/rollback/--dry-run | 新 `internal/app/skill_mode.go`（≈350 行）+ `skill_command.go:202-209` 注册；set 复用 setup 安装实现 + 备份 + 记账；rollback 回 `state.previous`；成功提示重启 AI 工具 | 2–2.5d | P0b、P0-3 |
| P1-3 | ⬜ | `dws skill remove`（新增，缺口 #5） | `skill_mode.go` 或新文件：按 state 的 `agent_homes`×`installed` 精确删内置 skill，`--dry-run` 预览；不碰非 `installed` 目录（市场 skill 安全） | 1d | P0b |

**内容补全线**（对应 Part 2 编号）

| ID | 任务 | 文件级改动点 | 估时 | 依赖 |
|---|---|---|---|---|
| C1 | recovery 闭环进 multi（M1） | 新 `skills/multi/dws-shared/references/recovery-guide.md`；`dws-shared/SKILL.md:83-89` 错误最短路径加 recovery 步；`dws-shared/references/global-reference.md` 补 Recovery 节；`dingtalk-dev/SKILL.md:124` 断链改指 | 0.5d | — |
| C2 | 确认门禁协议 + 危险操作索引（M2） | `dws-shared/SKILL.md` 增「确认门禁」节（移植 mono `SKILL.md:170-187` 协议）+ 危险操作索引表指向各产品 SKILL.md | 0.5d | — |
| C3 | Schema 渐进查询教学（M3） | `dws-shared/SKILL.md` 渐进加载表加一行 + 新 `references/schema-usage.md`（改写 mono `SKILL.md:198-292`） | 0.5d | — |
| C4 | multi 一致性修复（X2/X3/X4/X5/X6 = plan P2-4 剩余量） | `dws-shared/references/routing.md:22` 改指 dingtalk-markdown；`dingtalk-chat/SKILL.md:131` 死链处置；`dingtalk-event/SKILL.md` 补 PREREQUISITE+metadata；`dingtalk-{profile,hrbrain,markdown,pat}/SKILL.md:15` EXPERIMENTAL 文案下调；SAFETY_PREAMBLE_INJECT 标记处置 | 0.5d | — |
| C5 | orphan 脚本与缺失产品处置（X1/M4/M6） | `dingtalk-misc/SKILL.md:20-32` 产品索引补 yida/finance/aiapp 行 + 新 reference（或把 16 个脚本移出发布包）；`report_inbox_today.py` 验证后迁入或删除；aiapp 路由决策落 `dws-shared/references/routing.md` | 0.5–1d | — |
| C6 | skill_tests.md multi 化 + 扩产品 | `test/skill_tests.md:40-54` 覆盖表改 multi 路径；修 workbench（54 行）/devdoc（47 行）死链；补 doc/drive/mail/minutes/oa/sheet/wiki 用例 | 1d | C1–C5（路径稳定后） |
| C7 | LICENSE/NOTICE 进 multi（M5） | 20 个 `skills/multi/*/LICENSE|NOTICE`（或 `scripts/release/post-goreleaser.sh` 打包期注入） | 0.25d | — |

### 3.3 P2 / W4 — 收尾（≈1d）

| ID | 状态 | 任务 | 估时 | 依赖 |
|---|---|---|---|---|
| P1-2 | ⬜ | beta 轨默认切 multi（版本门控；本工作区已全量切，可选择保留全量或回退为 beta 先行） | 0.5–1d | P1-1 |
| P2-1 ~ P2-3 | ✅ | setup / install.sh / install.ps1 / install-skills.sh / install.js 默认 multi | — | 已完成 |
| P2-4 | 🚧 | 文案翻转：README×2、dingtalk-skill 已改 ✅；4 个 EXPERIMENTAL 漏改 → 并入 C4 | — | — |
| W4 | ⬜ | mono deprecation 警告（不删代码）+ mono 下线判据第 4 条（替代 `check-skill-context-budget.sh:10,34-38` 对 mono 的依赖） | 0.5d | P2、C6 |

### 3.4 依赖图与排期建议

```text
P0c-1 ─► P0c-2
P0b-1 ─► P0b-2 ─┐
P0-3 ───────────┼─► P0a-1 ─► P0a-2 ─► P1-1 ─► P1-3
P0b-1 ──────────┴─► P0-4
C1…C5（互相独立，可与 P0 并行）─► C6
```

- **W1**：P0c-1 → P0b-1/P0b-2 → P0-3 → P0a-1/P0a-2（管理面止血）；并行 C1–C4（内容高危项）。
- **W2**：P0-4、P1-1、P1-3；并行 C5、C7。
- **W3**：C6（内容面收口）+ P1-2 beta 决策；`make policy` 全绿。
- **W4**：mono deprecation + 观察（plan §4 原节奏不变）。

合计：P0 ≈ 6.5–8.5d + P1 管理 ≈ 3–3.5d + 内容 C1–C7 ≈ 3.5–4d + P2/W4 ≈ 1d
= **14–17 人日**。砍法同 plan §8.4：内容线可砍 C3/C7（教学与法律文件可后置），
管理线不可砍 P0b/P0-3/P0a（缺了就是现在这副「能装不能管」的样子）。
