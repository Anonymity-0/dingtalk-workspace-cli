# Skill 分发/安装架构优化方案

> 目标：把 DWS Agent skill 的"装在哪、怎么装、装完记什么"从 7+ 个安装面的
> N 份漂移拷贝，收敛为 **Go 侧单一事实源 + 单一安装引擎 + 脚本侧 bootstrap**。
> 本文基于 feat/skill-mode-migration 工作区（multi 化落地后）的代码级盘点
> （2026-08-05），所有锚点均可跳转验证。
> 前置文档：[skill-multi-migration-plan.md](skill-multi-migration-plan.md)（下称《迁移计划》）、
> [skill-distribution-mechanism.md](skill-distribution-mechanism.md)。

## 1. 问题量化

### 1.1 复制矩阵（能力 × 面）

multi 化之后，同一块逻辑在各安装面的拷贝数（✓ = 独立拷贝一份，数字 = 份数）：

| 能力 \ 面 | install.sh | install.ps1 | npm install.js | install-skills.sh | install-event.sh | install-devapp.sh(+ps1) | Homebrew caveats | `dws skill setup` (Go) | `dws upgrade` (Go) |
|---|---|---|---|---|---|---|---|---|---|
| agent home 清单 | ✓×3 | ✓ | ✓ | ✓×2 | ✓ | ✓×2 | — | ✓ | ✓ |
| 互斥清理（mono↔multi） | ✓×2（其中 1 份死代码） | ✓ | ✓ | ✓ | — | — | — | ✓ | ✓ |
| mode 解析（env/flag/交互） | ✓ | ✓ | ✓ | — | — | — | — | ✓ | 布局嗅探 |
| stale `dingtalk-*` 清理 | ✓ | ✓ | ✓ | ✓ | — | — | — | ✗（additive，不清） | ✓ |
| `~/.dws/skills` 缓存写 | ✓ | ✓ | ✓ | — | ✓ | ✓ | — | —（只读回退） | ✓（仅 multi） |
| 安装状态（state.json） | 无 | 无 | 无 | 无 | 无 | 无 | — | 无 | 无 |

清单锚点（agent home 清单，全仓共 **15 份**）：

| # | 位置 | 备注 |
|---|---|---|
| 1 | `internal/app/skill_setup.go:20-37` | `skillSetupAgentHomes`，16 项，**无 opencode** |
| 2 | `internal/app/skill_command.go:119-141` | `agentSkillPaths`，17 项，含 opencode（`:129`） |
| 3 | `internal/upgrade/paths.go:35-52` | `knownSkillDirs`，16 项，**无 opencode** |
| 4 | `scripts/install.sh:370-386` | mono 安装循环内联清单 |
| 5 | `scripts/install.sh:439-455` | multi 清单（**死代码**，见 1.2） |
| 6 | `scripts/install.sh:516-532` | multi 清单（生效） |
| 7 | `scripts/install.ps1:44-61` | `$AgentDirs` |
| 8 | `build/npm/install.js:11-28` | `AGENT_DIRS` |
| 9 | `scripts/install-skills.sh:166-182` | multi 清单 |
| 10 | `scripts/install-skills.sh:247-261` | mono 清单 |
| 11 | `scripts/install-event.sh:103-107` | 17 项，**含 `.config/opencode/skills`**（`:107`） |
| 12 | `scripts/install-devapp.sh:87-91` | 含 opencode（`:91`） |
| 13 | `scripts/install-devapp.ps1:106` | 含 opencode |
| 14 | `test/scripts/package_script_test.go` | `expectedPackagedSkillTargets` |
| 15 | `scripts/release/verify-package-managers.sh:75-76` | `HOME_AGENT_PARENTS` / `HOME_SKILL_TARGETS` |

互斥清理逻辑共 **≥8 份**：`skill_setup.go:570-608`、`paths.go:306-327` +
`paths.go:163-179`（mono 分支内联）、`install.sh:394-399` + `install.sh:557-570`
+ 死代码 `install.sh:477-486`、`install.ps1:488-498` + `install.ps1:562-570`、
`install.js:130-137` + `install.js:165-178`、`install-skills.sh:207-220`。

mode 解析共 **4 份**：`skill_setup.go:343-376`、`install.sh:220-257`、
`install.ps1:293-332`、`install.js:197-214`（各自实现 env 优先级、非法值报错、
TTY 交互、非 TTY 默认值，措辞与行为细节已不完全一致）。

stale `dingtalk-*` 清理共 **5 份**：`paths.go:317-325`、`install.sh:561-567`、
`install.ps1:562-570`、`install.js:168-172`、`install-skills.sh:211-220`。

缓存写共 **6 份**：`install.sh:326-344`（multi）+ `install.sh:349-360`（mono）、
`install.ps1:446-471`、`install.js:221-237`、`install-event.sh:165-169`、
`install-devapp.sh:82-84`、`paths.go:290-296`（upgrade 只刷 multi）。

### 1.2 已发生的漂移实例（不是假设，是现状）

1. **opencode 清单漂移**：`dws skill install` 支持 opencode 目标
   （`skill_command.go:129`），install-event / install-devapp 也往
   `~/.config/opencode/skills` 装（`install-event.sh:107`、
   `install-devapp.sh:91`），但 `dws skill setup --target all` 与
   `dws upgrade` 的清单都不含它（`skill_setup.go:20-37`、
   `paths.go:35-52`）。结果：opencode 用户能收到 event/dev skill，
   却永远收不到内置 skill 的安装与升级。
2. **install.sh 死代码**：`install_multi_skills_to_homes` 定义了两次
   （`install.sh:434` 与 `install.sh:511`），`_install_multi_to_base`
   同样两次（`install.sh:473` 与 `install.sh:549`）。bash 后定义覆盖先定义，
   第一对（434-505）整体是死代码——本次 multi 化自己引入的重复。
3. **"镜像"注释与实现已不符**：`install.sh:507-510` 注释声称 multi 安装
   "mirroring `dws skill setup --mode multi`"，但脚本会删除 bundle 里不存在
   的 stale `dingtalk-*`（`install.sh:561-567`），而 setup 是 additive 语义、
   只清 `dws/`（`skill_setup.go:587-593`）。注释说镜像，行为已分叉。
4. **互斥语义自相矛盾**：install-event.sh 把 multi 的 `dingtalk-event` 和
   mono 的 `dws` **同时**装进同一批 agent home（`install-event.sh:171-172`），
   与其他所有面"mono/multi 互斥"的语义直接冲突。
5. **缓存只写不读、版本不可见**：`dws skill setup` 默认走二进制 embed
   （`skill_setup_embed.go:50-59`），`~/.dws/skills` 只是 legacy 回退候选
   （`skill_setup.go:442-447`）；6 个写入方没有任何版本戳，upgrade 只刷
   multi 不刷 mono（`paths.go:292-296`），缓存与 embed 漂移不可见。
6. **保留前缀规则已双份**：`dingtalk-` / `dws-shared` 的保留约定在
   `skill_setup.go:199`（`multiSkillPrefix`）、`skill_setup.go:205`
   （`multiSharedSkill`）与 `paths.go:332-334`（`isMultiSkillDirName`）
   各写一份，互斥清理依赖"市场 skill 不用该前缀"这一隐性约定。

### 1.3 维护成本论证（本次 multi 化实证）

本次"把 7 个面全部 multi 化"这一个语义变更，工作区改动为 **13 个文件、
+759/−166 行**（`git diff --stat`），横跨 Go / POSIX sh / PowerShell /
JavaScript 四种语言：

| 文件 | 改动量 | 改了什么 |
|---|---|---|
| `scripts/install.sh` | +256/−… | mode 解析、multi 安装×2（含死代码）、互斥清理、缓存写 |
| `scripts/install.ps1` | +169 | 同上，PowerShell 再写一遍 |
| `build/npm/install.js` | +87 | 同上，JavaScript 再写一遍 |
| `scripts/install-skills.sh` | +101 | multi 安装 + stale 清理 |
| `internal/upgrade/paths.go` | +206 | upgrade multi 刷新 + 互斥清理 + 缓存 |
| `internal/app/skill_setup.go` 等 Go 侧 | ~+50 | setup multi 语义 |
| 测试与文档 | 其余 | 4 个测试文件 + README×2 + SKILL.md |

即：**一个语义变更 = 4 种语言 × 7+ 处同步编辑**，且仍然漏了 opencode、
制造了死代码、留下了语义分叉（1.2）。新增一个 agent home 今天要改 15 份
清单中的至少 11 份（另 4 份是测试/校验）。这不是"以后会漂移"，而是
**每一次改动都在当场制造漂移**。

## 2. 目标架构

### 2.1 分层总览

```text
┌──────────────────────────────────────────────────────────────────┐
│ L3 分发面（7 个，全部退化为 bootstrap）                            │
│   install.sh / install.ps1 / npm install.js / install-skills.sh /│
│   install-event.sh / install-devapp.sh / Homebrew caveats        │
│   职责：装二进制 → 调用 setup(mode=X, non_interactive=true)        │
│         └─ 失败 → 冻结的内嵌 fallback 拷贝（一档，不再演进）        │
├──────────────────────────────────────────────────────────────────┤
│ L2 单一安装引擎（Go，唯二入口）                                    │
│   dws skill setup  ─┐                                            │
│   dws upgrade      ─┴─► 共用同一 install 实现（含互斥清理/记账）   │
├──────────────────────────────────────────────────────────────────┤
│ L1 单一事实源：internal/skillhome（新共享包）                      │
│   AgentHomes() 清单 + 布局规则（mono→<home>/dws，multi→平铺）      │
│   + 互斥/ stale 清理规则 + 保留前缀常量 + state.json 读写          │
├──────────────────────────────────────────────────────────────────┤
│ L0 事实数据                                                       │
│   //go:embed skills/{mono,multi}（skills_embed.go:28，默认源）     │
│   ~/.dws/skills/state.json（权威安装状态）                         │
│   ~/.dws/skills/{mono,multi}（显式回退源，带版本戳，唯一写方=Go）  │
└──────────────────────────────────────────────────────────────────┘
        ▲ 门禁：scripts/policy/check-agent-homes-sync.sh（进 make policy）
        比对各脚本可解析清单块 ↔ skillhome 导出清单
```

### 2.2 单一事实源：`internal/skillhome` 共享包

新建 `internal/skillhome`，把今天散在 3 处 Go 代码里的规则合并导出，
setup / upgrade / 测试共用（对应《迁移计划》P0c-1）：

| 导出物 | 收敛的现有拷贝 |
|---|---|
| `AgentHomes()`（含 opencode，带"首项必装/其余父目录门控"元数据） | `skill_setup.go:20-37`、`paths.go:35-52`，并与 `skill_command.go:119-141` 互相断言 |
| `HomeForMode(base, mode)` 布局规则 | `skill_setup.go:502-507`（`agentHomeForMode`） |
| `MutualExclusionVictims(home, mode)` / stale 判定 | `skill_setup.go:570-596`、`paths.go:306-327` |
| `ReservedSkillPrefixes` / `ReservedSkillNames` 常量 | `skill_setup.go:199/205`、`paths.go:332-334` |
| `State` 读写（state.json，见 2.4） | 新增（《迁移计划》P0b-1） |

脚本侧不生成代码（sh/ps1/js 三语言片段生成成本高、措辞差异大），改为
**可解析标记块 + policy 比对**：每个脚本的清单包在
`# DWS-AGENT-HOMES-BEGIN` / `# DWS-AGENT-HOMES-END` 注释块内，新增
`scripts/policy/check-agent-homes-sync.sh` 提取 7 个脚本块与
`skillhome` 导出清单（`go run ./internal/skillhome/cmd/dump` 或
`dws skill setup --print-agent-homes` 之类调试出口）逐一比对，挂进
`make policy`（Makefile:86-97 现有政策链）。对应《迁移计划》P0c-2。

### 2.3 安装逻辑单引擎化：脚本退化为 bootstrap

**终态形态**：每个安装脚本只做两件事——装二进制、以已解析 mode 非交互调用
setup 引擎（记作 `setup(mode=<resolved>, non_interactive=true)`；mode 解析仍允许脚本做，因为它
要处理自己的 env/flag；也可以更进一步把 `DWS_SKILL_MODE` 透传给 setup）。
执行失败（二进制跑不起来、setup 非 0 退出）时，降级到**冻结的内嵌
fallback 拷贝逻辑一档**——即今天的拷贝实现原样保留但标记"不再演进"，
语义变更只改 Go 引擎。

各面降级可行性分析：

| 面 | 二进制可得性 | 可行性 | 风险与对策 |
|---|---|---|---|
| install.sh | `main` 先 `install_binary` 后 `install_skills`（`install.sh:830-831`），二进制就在 `$INSTALL_DIR` | ✅ 安装器以已解析 mode 非交互调用 setup 引擎 | `DWS_SKILLS_ONLY=1` 时不装二进制 → 先 `command -v dws`，无则走 fallback。curl 下载不带 quarantine 属性，macOS 可直接执行。风险低 |
| install.ps1 | 同序（`Install-Binary`:337 → `Install-Skills`:622，main 入口 `:700`） | ✅ 调 `& $installDir\dws.exe skill setup ...` | ExecutionPolicy 约束的是 .ps1 脚本本身，**子进程 dws.exe 不受其限制**；AppLocker/WDAC 环境可能拦未签名二进制——但那种环境 dws 本身也跑不了，fallback 拷贝仍必要。风险低-中 |
| npm install.js | postinstall 已把平台二进制解到 `vendor/`（`install.js:260`）再装 skill（`:271-280`） | ✅ 通过 `execFileSync` 以已解析 mode 非交互调用 setup 引擎 | `npm i --ignore-scripts` 时 postinstall 整体不跑（现状如此，非新增风险）；部分 CI 以 root 跑生命周期脚本权限怪异；Windows 用 `dws.exe`。风险中，fallback 必须保留 |
| install-skills.sh | skills-only 面，不装二进制 | ✅ `command -v dws` 有则调引擎，无则 fallback | 本质是"刷新 skill"路径，有 dws 才谈得上刷新。风险低 |
| install-event.sh | 装二进制 + 单 skill（dingtalk-event + mono dws） | ⚠️ 有条件 | `EVENT_VERSION` 可与已装二进制版本不同（尤其 `DWS_SKILLS_ONLY=1`），embed 与目标 zip 可能错配；且当前同时装 mono+multi 的语义（`:171-172`）需先按 1.2-4 收敛。**最后迁移**，迁移前先把 dingtalk-event 纳入 embed 并改成 `-s event` 调引擎 |
| install-devapp.sh(+ps1) | 同上（dingtalk-dev） | ⚠️ 有条件 | 同 install-event。风险中-高，最后迁移 |
| Homebrew caveats | 只打印提示 | ✅ 已是终态 | `homebrew.rb.tmpl:33-36` 现状就是"Run `dws skill setup`"，无需改，是其他面的样板 |

配套收益：《迁移计划》P2-2（sh/ps1 的 multi 分支真装，1.5d）与 P2-3
（install.js / install-skills.sh 加 mode 支持，1d）在单引擎方案下**大幅
缩水**——脚本只需把 mode 透传给引擎，不再在 sh/ps1/js 里写安装逻辑。

### 2.4 状态与缓存

- **state.json 权威化**（《迁移计划》P0b 原样采纳）：
  `~/.dws/skills/state.json` 记录 `schema_version / mode / cli_version /
  installed / agent_homes / previous`，写方只有 `dws skill setup` 与
  `dws upgrade`（脚本侧不再各自写状态——它们调引擎，引擎记账）；缺失/损坏
  时按磁盘形态反推（有 `dws/` → mono；有 `dingtalk-*` → multi；都有 →
  报 drift 要求显式收敛）。
- **`~/.dws/skills` 缓存收敛**：二选一，本文建议后者——
  1. ~~被 state 取代，删除缓存~~：激进；embed 之外的"无源码机回退源"场景
     （`skill_setup.go:442-447` 注释描述的场景）会断。
  2. **明确为 setup 回退源 + 版本戳**（采纳）：缓存目录写入
     `cli_version` 戳文件（或并入 state.json 的 `cache_version` 字段）；
     **写缓存收敛到 Go 侧唯一实现**（setup/upgrade 共用），install.sh /
     ps1 / js / event / devapp 的 6 份缓存写逻辑（1.1 矩阵）随 P1/P3 删除；
     upgrade 补齐 mono 缓存刷新（修 `paths.go:292-296` 只刷 multi 的不对称）。
     mono 下线版本再评估整体废弃缓存。

### 2.5 市场 skill 边界正式化

把"`dingtalk-*` 前缀与 `dws-shared` / `dws` 名称为 DWS 内置保留"从隐性约定
变成共享常量 + 测试：

- `skillhome.ReservedSkillPrefixes = ["dingtalk-"]`、
  `skillhome.ReservedSkillNames = ["dws-shared", "dws"]`，替换
  `skill_setup.go:199/205` 与 `paths.go:332-334` 两份私有拷贝；所有互斥/
  stale 清理只认这两个常量。
- 测试一：扫描 `skills/multi/` 实际目录名，断言全部命中保留规则（防新增
  产品 skill 破坏前缀约定）。
- 测试二：市场 skill 安装路径（`skill_command.go:488-498` 的目标解析）若
  产物名命中保留名则拒绝安装——把"市场无同名前缀"从祈祷变成门禁。

## 3. 迁移步骤（渐进、不破坏存量用户）

| 阶段 | 内容 | 风险 |
|---|---|---|
| **P0 清单 + policy** | 建 `internal/skillhome`：AgentHomes（**补 opencode**，修 1.2-1）、布局/互斥/保留名常量；setup/upgrade 切到共享包；脚本清单包标记块；新增 `check-agent-homes-sync.sh` 进 `make policy`；删 install.sh 死代码（1.2-2） | 低。纯收敛不改行为；policy 脚本初期可能误报 → 先 warn-only 跑一个版本再转 hard-fail |
| **P1 setup 单引擎 + 脚本降级** | setup/upgrade 安装实现合一（互斥清理、stale 语义统一为"引擎内一种"，修 1.2-3）；install.sh / install.ps1 / install.js / install-skills.sh 改为 bootstrap + 冻结 fallback；install-event/devapp 暂不动 | 中。bootstrap 首版要在三语言 CI 矩阵上验证"引擎失败→fallback"链路；fallback 标记冻结，防止继续演进出第 9 份拷贝 |
| **P2 state 权威 + 缓存收敛** | state.json 读写进 skillhome，setup/upgrade 记账；缓存加版本戳、写方收敛到 Go、upgrade 补刷 mono 缓存；install-event/devapp 语义收敛（mono+multi 双装改为引擎语义）后同样 bootstrap 化 | 中。存量机器无 state → 磁盘反推逻辑必须有故障注入测试；event/devapp 版本错配场景需保留 zip 直装逃生门 |
| **P3 删除遗留拷贝** | 删除：脚本侧 6 份缓存写、install.sh/ps1/js 内被引擎接管的安装函数（保留冻结 fallback 一档）、Go 侧 `mutualExclusionVictims` 等被 skillhome 吸收的私有拷贝；观察一个版本后评估 fallback 是否可再降档 | 低-中。每删一处先确认 policy 与测试不再引用；fallback 删除是独立决策，不与本阶段捆绑 |

与《迁移计划》的先后关系：本文 P0 即计划 P0c，应**最先做**；P1 与计划
P0a/P0b 并行不冲突（引擎合一会让 P0a 的 mode-aware upgrade 少写一份代码）；
P2 吸收计划 P0b；P3 在计划 P2 切默认之后执行。

## 4. 明确不做

- **不重写成 symlink canonical**（canonical 一份 + 各 agent home 软链）：
  已随生态分发通道一并评估否决，决策与实测原因见
  《迁移计划》[§7.3](skill-multi-migration-plan.md)（无版本固定、依赖
  Node/GitHub 可达、mono 会被一起发现、悟空 bundled/离线预装覆盖不了；
  其中悟空分发线已于 2026-08-05 下线，该条约束随之消失）。
  本地 symlink 模式（`setup --link`）同样维持计划 §8.2"可选/后置"定位，
  不在本方案内。
- **不动 `dws-skills.zip` 产物布局**：zip 根 mono 副本 + `mono/` + `multi/`
  双树保持不变（`scripts/release/post-goreleaser.sh:220-248`）。
- **不动市场 skill**：`dws skill install` 的安装语义、目标解析
  （`skill_command.go:488-498`）不变；2.5 只新增保留名拒绝门禁与测试，
  不改变既有安装行为。
- **不删 fallback**：脚本侧内嵌拷贝逻辑降级为冻结档保留，不追求"脚本零
  拷贝"的纯净化。

## 5. 收益/成本与任务映射

### 5.1 收益/成本表

| 项 | 现状 | 目标 | 量化收益 |
|---|---|---|---|
| agent home 清单 | 15 份拷贝，已漂移（opencode） | 1 份 Go 源 + policy 比对 | 新增 agent 从"改 11 处"变"改 1 处" |
| 安装/互斥/stale 逻辑 | ≥8 份，4 种语言，语义已分叉 | 1 个 Go 引擎 | 语义变更从 4 语言 × 7 面（本次实证 13 文件 +759 行）变 1 包 |
| mode 解析 | 4 份，行为细节不一 | 脚本只做透传，引擎一处实现 | 非法值/默认值行为天然一致 |
| 缓存写 | 6 份，无版本戳，只写不读 | Go 唯一写方 + 版本戳 | 缓存漂移可观测、可判废 |
| 安装状态 | 无 | state.json 权威 | upgrade 粘性、回滚、drift 检测的前提 |
| 市场边界 | 隐性约定，2 份前缀拷贝 | 共享常量 + 2 个测试 | 互斥清理误伤市场 skill 的风险归零 |
| 成本 | — | P0≈1.5d，P1≈3-4d，P2≈2d，P3≈1d | 合计 7.5-8.5 人日；其中 P0/P2 与《迁移计划》P0b/P0c 重叠，净新增约 4-5 人日 |

### 5.2 与《迁移计划》§8 任务 ID 映射

| 本文阶段 | 对应计划任务 | 关系 |
|---|---|---|
| P0 清单 + policy | P0c-1（清单下沉共享包+补 opencode）、P0c-2（check-agent-homes-sync.sh） | 原样采纳，包名定为 `internal/skillhome` |
| P1 引擎合一 | P0a-1（UpgradeSkillLocations mode-aware）的前置简化 | 引擎合一后 P0a-1 不复写互斥/清理逻辑 |
| P1 脚本 bootstrap | **取代** P2-2（sh/ps1 multi 真装，1.5d）、P2-3（install.js/install-skills.sh mode 支持，1d） | 脚本不再写安装逻辑，两个任务缩水为"透传 mode + 调引擎 + 冻结 fallback"，合计从 2.5d 降至 ~1d |
| P2 state 权威 | P0b-1（state.json schema+setup 写入）、P0b-2（磁盘反推） | 原样采纳；缓存版本戳与写方收敛为本文新增 |
| P2 event/devapp 收敛 | 新增（修 1.2-4 语义矛盾） | 依赖 P1 引擎稳定 |
| P3 删除遗留 | 新增 | 在计划 P2 切默认之后执行 |
| （不在本文） | P0-3 备份式安装、P0-4 请求头、P1-1 `dws skill mode`、P1-2/P2-1 默认翻转、P2-4 文案 | 仍按计划执行，与本文正交；备份式安装落地时直接写进引擎，天然覆盖所有面 |
