# Skill multi 迁移：技术方案（as-implemented）与 Roadmap

> 本文是 multi 迁移的当前事实源：第一部分记录**已落地实现**（代码级锚点，
> 均可跳转验证），第二部分是带实时状态的 roadmap。
> 原始方案 [skill-multi-migration-plan.md](skill-multi-migration-plan.md) 的
> §3.4/§3.6 等"待做"描述已被本文第一部分取代（实际实现与方案有差异，见
> 决策记录 D1）；分发机制调研结论见
> [skill-distribution-mechanism.md](skill-distribution-mechanism.md)。
> 代码快照：`feat/skill-mode-migration` 工作区未 commit 变更（2026-08-05）。

## 状态速览

| 项 | 内容 |
|---|---|
| 当前阶段 | **阶段 1 / 1.5 均 ✅ 完成（2026-08-05）**；当前处于「**待 commit + 阶段 2 开工前**」，全部变更在工作区未 commit |
| 硬 deadline | **全部切换必须 2026-08-30 前完成**（今日 8/5，剩 25 天）。"全部切换完" = 安装/升级默认 multi（✅）+ 存量 mono 用户被迁移 + **mono 物理下线**（embed/源码/产物中删除）——即阶段 4 也须在 8/30 前落地，不只是切默认 |
| 已完成 | 升级默认 multi、安装脚本四面默认 multi、`dws skill setup` 默认 multi、dws-shared 清理漏洞修复、文案翻转；阶段 1.5 review 0 P0、6 项 P1/P2 已修、五面实测 9/9 PASS（详见 roadmap 阶段 1.5） |
| 下一步 | commit（8/5）→ 阶段 2 工程项：备份式安装 + `state.json` + `dws skill mode` 命令 + `x-dws-skill-mode` 请求头 + agent home 清单门禁（8/6–8/12） |
| 终态 | 阶段 4 判据全满足后物理下线 mono（判据见 roadmap 阶段 4，均为 DWS 自身指标）；悟空分发线已于 2026-08-05 下线，mono 下线无仓外节奏闸门 |

---

# 第一部分：技术方案（as-implemented）

## 1. 升级默认 multi（`dws upgrade`）

核心语义：**升级跟着产物布局走，清理跟着磁盘现状走**，无需任何状态文件。

- 入口 `UpgradeSkillLocations(extractedDir)` 先用 `bundleSkillNames` 探测
  解压产物布局：含子 skill 目录 → multi 分支；顶层有 `SKILL.md` → mono
  分支（`internal/upgrade/paths.go:129-139`、探测函数
  `internal/upgrade/paths.go:339-358`）。
- `LocateSkillsRoot` 优先返回 zip 内的 `multi/` 树，缺失才回退
  `LocateSkillMD` 的 mono 布局（`internal/upgrade/paths.go:363-369`）；
  `internal/app/upgrade.go` 的 `locateUpgradeSkill` seam 已换绑到
  `LocateSkillsRoot`（`internal/app/upgrade.go:60`，调用点
  `internal/app/upgrade.go:579`）。新 zip 恒含 `multi/`，故**升级默认即
  multi 刷新**；老 zip 自然落回 mono 行为。
- multi 刷新 `upgradeMultiSkillLocations`
  （`internal/upgrade/paths.go:217-299`）：按 `knownSkillDirs` 父目录门控
  平铺安装；每个 agent home 先走 `cleanupOppositeModeLeftovers` 删 mono
  残留 `dws/` 与不在新包内的过期 multi skill
  （`internal/upgrade/paths.go:306-327`），清理失败则该 home 标记失败、
  **不装 multi**，保证 mono/multi 绝不共存；全部失败时回退强制写
  `~/.agents/skills`（`internal/upgrade/paths.go:262-288`）。
- multi 装完后 best-effort 刷新 `~/.dws/skills/multi` 缓存，让
  `dws skill setup --mode multi` 的缓存回退源跟上已升级版本
  （`internal/upgrade/paths.go:290-296`）。
- mono 分支保留原行为并补上对称互斥：装 mono 前删 `dingtalk-*` 与
  `dws-shared`（`internal/upgrade/paths.go:162-179`，判定函数
  `isMultiSkillDirName` 含 `dws-shared`，`internal/upgrade/paths.go:332-334`）。

测试（`internal/upgrade/paths_multi_test.go`，fake home 注入
`upgradeUserHomeDir`，`paths_multi_test.go:14-21`）：

| 用例 | 覆盖 |
|---|---|
| `TestLocateSkillsRootPrefersMulti` / `...FallsBackToMono` | zip 根有 mono 副本时 multi 优先；纯 mono 布局回退（`paths_multi_test.go:42-62`） |
| `TestUpgradeSkillLocationsMulti` | multi 刷新 + `dws/` 与过期 `dingtalk-old` 清理 + 非 DWS 目录保留 + multi 缓存刷新（`paths_multi_test.go:87-163`） |
| `TestUpgradeSkillLocationsMultiFallbackPrimary` | 无任何 agent 父目录时回退 `~/.agents/skills`（`paths_multi_test.go:165-183`） |
| `TestUpgradeSkillLocationsMonoStillWorks` | mono 回退路径：装 mono 并清 `dingtalk-*`/`dws-shared` 残留（`paths_multi_test.go:185-218`） |

## 2. 安装默认 multi（四个脚本面）

四个脚本安装面默认值全部翻转为 multi（加上 §3 的 `dws skill setup` 合计
五面），`DWS_SKILL_MODE=mono` 为统一 opt-in 出口，
互斥清理双向对称（装 multi 清 `dws/` + 过期 `dingtalk-*`/`dws-shared`；
装 mono 清 `dingtalk-*` + `dws-shared`）。

| 面 | mode 解析 | multi 真装 | mono 侧互斥清理 |
|---|---|---|---|
| `scripts/install.sh` | `resolve_skill_mode`：env → TTY 交互（`1)=multi（默认） 2)=mono`）→ 非 TTY 兜底 multi（`install.sh:220-256`，交互项 `install.sh:240-249`） | 本地源 `install.sh:291-297`、zip 路径 `install.sh:778-791`，平铺实现 `install_multi_skills_to_homes`（`install.sh:511-545`） | `install.sh:394-399`（含 `dws-shared`） |
| `scripts/install.ps1` | `Resolve-SkillMode` 同构（`install.ps1:293-332`，交互项 `install.ps1:314-325`） | 本地源 `install.ps1:417-423`、zip 路径 `install.ps1:671-683`，`Install-MultiSkillsToHomes`（`install.ps1:522-567`） | `install.ps1:487-493`（含 `dws-shared`） |
| `build/npm/install.js` | `resolveSkillMode`：`DWS_SKILL_MODE` / `--skill-mode` flag，默认 multi（`install.js:197-214`）；分发点 `install.js:270-278` | `installMultiSkillsToHomes`（`install.js:153-176`） | `install.js:130-137`（含 `dws-shared`） |
| `scripts/install-skills.sh` | `DWS_SKILL_MODE` 默认 multi，非法值即报错退出（`install-skills.sh:29-32`） | `install-skills.sh:339-340` | `install-skills.sh:271-276`（含 `dws-shared`） |

三处 shell/ps1/js 均保留双缓存：装完把 `mono/` 与 `multi/` 都刷进
`~/.dws/skills/{mono,multi}`，供 `dws skill setup` 无 `--source` 时回退
（如 `install.sh:311-318`、`install.js:216-235`）。

## 3. `dws skill setup` 默认 multi

- 非交互（`--yes` 或无 TTY）未指定 `--mode` 时默认 multi 并打印说明
  （`internal/app/skill_setup.go:354-357`）。
- 交互选项 multi 在前标"默认"、mono 标"legacy"
  （`internal/app/skill_setup.go:364-368`）；命令 Long 文案同步翻转，
  EXPERIMENTAL 措辞全删（`internal/app/skill_setup.go:75-87`）。
- **dws-shared 漏洞修复**：mono 侧互斥清理此前只扫 `dingtalk-*`，
  `dws-shared` 漏网导致切回 mono 后残留。现 `mutualExclusionVictims` 的
  mono 分支同时匹配 `multiSharedSkill`
  （`internal/app/skill_setup.go:581`，常量定义
  `internal/app/skill_setup.go:205`），与 §2 的四个脚本面合计五面同步
  修复。

## 4. 文案翻转

- `README.md:67-83` / `README_zh.md:67-83`：multi 标默认、mono 标
  legacy，快速安装/TTY/环境变量/事后切换四条路径说明同步更新。
- `skills/multi/dingtalk-skill/SKILL.md:15`：改为"multi 为默认安装模式"，
  EXPERIMENTAL 措辞删除。

## 5. 已验证

- `go test ./internal/upgrade ./internal/app ./test/scripts` 全绿。
- 脚本面有契约测试 `TestInstallScriptsExposeSkillModeSelection`
  （`test/scripts/install_script_test.go:529-576`，断言 sh/ps1 暴露
  `DWS_SKILL_MODE` 与 mono/multi 选项）；ps1/js 的**行为级**自动化测试
  仍缺，见文末风险表。
- fake HOME E2E：双向切换 multi→mono→multi 互斥清理干净，无双份共存。

## 6. 决策记录

- **D1 升级不依赖 state.json**。方案 §3.4 原设计是 upgrade 读
  `state.json` 的 mode 决定刷新形态；实际实现改为探测 zip 产物布局
  （`LocateSkillsRoot` 优先 `multi/`）+ 按各 agent home 磁盘现状做互斥
  清理，无需状态文件即可收敛到单模式。`state.json` 降级为阶段 2 的
  可观测/回滚增强，不再是正确性前提。zip 布局不变（根 mono 副本 +
  `mono/` + `multi/`，`scripts/release/post-goreleaser.sh:220-248`），
  **产物零改动**。
- **D2 无服务端灰度**。DWS 无远程配置能力：beta 轨先行（L1 渠道灰度）
  + 确定性分桶 `rollout.json` 原仅作可选 L2（8/30 倒排下已砍，见第二部分
  「压缩点与代价」）；kill switch = 公告命令
  （`dws skill mode set mono` / `rollback`）+ 备份回滚。**备份尚未实现，
  是阶段 2 的 P0**——在备份落地前，kill switch 只剩公告一半。
- **D3 生态分发通道已否决**。`npx skills add` / vercel-labs skills CLI
  不做分发通道：无版本固定（实测 v1.5.21 不支持 tag/ref）、依赖
  Node+GitHub 可达、mono 会被一起发现、悟空 bundled/离线场景覆盖不了。
  （2026-08-05 注：悟空分发线已下线，第四点约束随之消失；前三点否决
  理由仍成立。）调研留档 [skill-distribution-mechanism.md](skill-distribution-mechanism.md)
  §5–§6。
- **D4 互斥前缀约定**。`dingtalk-*` 与 `dws-shared` 保留给 DWS 产品
  skill，市场 skill（`dws skill install`）不使用此前缀；互斥清理依赖此
  约定（代码注释 `internal/upgrade/paths.go:301-305`、
  `internal/app/skill_setup.go:197-205`）。阶段 2 起由清单门禁兜底漂移。

---

# 第二部分：Roadmap

## ✅ 阶段 1（已完成，2026-08-05）

| 项 | 锚点 |
|---|---|
| 升级默认 multi（布局探测 + 互斥清理 + 缓存刷新 + seam 换绑） | 本文第一部分 §1 |
| 安装脚本四面默认 multi（sh / ps1 / js / install-skills.sh 真装 + 默认翻转） | 本文第一部分 §2 |
| `dws skill setup` 默认 multi + EXPERIMENTAL 文案删除 | 本文第一部分 §3 |
| mono 互斥清理补 `dws-shared`（五面同步） | 本文第一部分 §2/§3 |
| 文案翻转（README×2 + dingtalk-skill SKILL.md） | 本文第一部分 §4 |

## ✅ 阶段 1.5（已完成，2026-08-05）

- 变更 review：**0 P0**；6 项 P1/P2 已修——install.js 空 multi 守卫、
  缓存守卫、EXPERIMENTAL 归零、mono SKILL.md 校验、确认预览、testseam。
- `dws skill setup` 全量安装新增过期清理语义（不在新包内的过期 skill
  一并清理）；`--filter`（filtered）路径仍保持 additive 语义不变。
- 全通道实机安装测试：curl|sh / ps1 / npm / install-skills.sh / setup
  五面 **9/9 PASS**；node 侧补 4 场景冒烟测试。
- 产出**未 commit**：全部变更在工作区待提交，commit 是 8/30 倒排关键
  路径的第一环（8/5）。

## ⬜ 阶段 2–4：8/30 倒排排期（硬 deadline 2026-08-30）

> 硬约束（2026-08-05 项目主决策）：**全部切换必须 2026-08-30 前完成**
> （今日 8/5，剩 25 天）。"全部切换完"的含义：所有安装/升级路径默认
> multi（阶段 1 ✅）＋存量 mono 用户被迁移＋mono 物理下线（embed/源码/
> 产物中删除）——即原阶段 4 也要在 8/30 前落地，不只是切默认。

### 关键路径（每一环滑期都直接吃 8/30 缓冲）

**commit（8/5）→ 阶段 2 工程项（备份回滚 / `state.json` / `dws skill mode` /
埋点，8/6–8/12）→ beta 发布含 L1 版本门控（8/13–8/14）→ beta 观察期
（压缩到 1 周，8/14–8/21，靠 `x-dws-skill-mode` 占比判断）→ stable 全切
（8/21–8/22）→ 观察 + 下线判据核对（8/22–8/27）→ mono 物理下线版本
（8/27–8/29）→ 8/30 缓冲区（1 天）。**

灰度结论不变：**备份回滚落地前不进 stable 全切**——备份式安装是阶段 2
工程项第一项（8/6 开工），回滚路径必须先于 8/21 可用。

### 每周里程碑

#### 第 1 周（8/5–8/9）：commit + 阶段 2 开工

- 交付物：
  - **8/5 commit** 工作区全部变更（阶段 1/1.5 成果固化，关键路径第一环）；
  - 备份式安装（RemoveAll→mv backup + 失败整体回滚，≈1.5–2d）；
  - `state.json`（≈1–1.5d）开工并主体完成；
  - agent home 清单单一事实源 + `check-agent-homes-sync.sh` 门禁
    （≈1.5d，可并入本周）。
- 出口标准：备份式安装 + `state.json` 主体进 review；清单门禁进
  `make policy`。
- 风险：commit 拖到 8/6 之后 → 关键路径整体右移，直接消耗 8/30 唯一的
  1 天缓冲。

#### 第 2 周（8/10–8/16）：阶段 2 收尾 + beta 发布

- 交付物：
  - `dws skill mode`（status / set / rollback / --dry-run，≈2–2.5d）；
  - `x-dws-skill-mode` 请求头埋点（≈0.5d，观察期数据前提，必须先于
    beta 上车）；
  - `upgrade --dry-run` 文案对齐 multi（≈0.5d）；
  - L1 版本门控拆分（≈2–2.5d，8/12 起与阶段 2 收尾交叠）；
  - **8/13–8/14 beta 发布**（GitHub prerelease / npm `beta` dist-tag /
    `dws upgrade --beta`，默认 multi，stable 保持观察）。
- 出口标准：beta 可下载、默认 multi、`x-dws-skill-mode` 上报在服务端
  可见。
- 风险：L1 门控 ≈2–2.5d 压在 2 天窗口内，beta 极易滑到 8/15+；**8/14
  beta 未出即触发降级方案**（见下）。

#### 第 3 周（8/17–8/23）：beta 观察（1 周）+ stable 全切

- 交付物：
  - beta 观察期收尾（8/14–8/21，从原 2 周**压缩到 1 周**，靠
    `x-dws-skill-mode` 占比与 issue 流入判断，不靠时长堆样本）；
  - **8/21–8/22 stable 全切 + 公告**（kill switch 命令
    `dws skill mode set mono` / `rollback` 随公告发出）。
- 出口标准：beta 期间无 multi 相关 P1；multi 请求占比趋势正常；stable
  全量默认 multi。
- 风险：观察期压缩 → 样本量减半；发现 P1 立即 kill switch 回滚，并把
  stable 全切右移进降级路径。

#### 第 4 周（8/24–8/30）：判据核对 + mono 物理下线 + 缓冲

- 交付物：
  - 8/22–8/27 观察 + 下线判据核对（4 条 DWS 自身指标，见下；判据 3
    「连续两周无 P1」从 beta 发布 8/14 起算，8/27 满两周）；
  - **8/27–8/29 mono 物理下线版本**（≈2–3d）：删 `skills/mono/`、
    `skills_embed.go` 去 `all:skills/mono`、`check-skill-context-budget.sh`
    替代门、存量 `<agent>/dws/` 目录遇到即迁移、下线版本 upgrade 一次性
    自动迁移带备份；
  - 8/30 缓冲区（1 天）。
- 出口标准：mono 从源码 / embed / 产物中物理消失；存量 mono 用户升级
  即自动迁移到 multi、可 `dws skill mode rollback` 回滚。
- 风险：判据未全满足但仍须 8/30 前收口的，走降级方案；自动迁移有备份
  兜底，回滚路径在阶段 2 已实现。

### 并行泳道：内容 C 线（8/6–8/20，不卡关键路径）

能力补全内容线 ≈4–5d 工作量，与关键路径并行推进、不占其资源：
recovery 闭环、`confirmation_required`、Schema 教学、orphan 脚本、死链、
`dingtalk-event` PREREQUISITE、LICENSE/NOTICE。8/20 前收口；不进入
stable 全切 / mono 下线的阻塞条件。

### 压缩点与代价

| 压缩点 | 原计划 | 现计划 | 代价与补偿 |
|---|---|---|---|
| beta 观察期 | 2 周 | **1 周**（8/14–8/21） | 样本量减半；靠 `x-dws-skill-mode` 占比 + issue 流入双指标补偿，出现 P1 立即 kill switch |
| L2 确定性分桶（`rollout.json`） | 可选 ≈2.5–3d | **砍掉**（时间不够） | L1 beta 轨 + 公告 kill switch + 备份回滚已覆盖灰度语义 |
| 全程缓冲 | — | 仅 8/30 一天 | 任一环滑期即触发降级方案 |

### 降级方案（8/14 beta 未出：整体右移，8/30 不动）

1. **先保 stable 全切 8/21**：压缩甚至跳过 beta 观察，直接 stable 全切 +
   kill switch 公告（备份回滚此时已落地，回滚路径完整）；
2. **mono 物理删除顺延**到下一个版本窗口，但 **8/30 前必须发出
   deprecation 公告 + upgrade 一次性自动迁移（带备份）**。

即 8/30 的底线 =「stable 全切完成 + 存量 mono 自动迁移发出」；mono
物理删除是最先让位的环节。

### 8/30 倒排甘特（文字版）

```text
            8/5────────────────────8/30
日(个位)     56789012345678901234567890
关键路径
commit      ◆·························
阶段2工程项  ·███████··················   (8/6–8/12)
beta+L1门控 ········◆◆················   (8/13–8/14 发布)
beta观察    ·········▒▒▒▒▒▒▒▒·········   (8/14–8/21，1 周)
stable全切  ················◆◆········   (8/21–8/22)
观察+判据   ·················██████···   (8/22–8/27)
mono下线    ······················███·   (8/27–8/29 物理删除)
缓冲        ·························▪   (8/30)
并行泳道
内容 C 线   ·░░░░░░░░░░░░░░░··········   (8/6–8/20，≈4–5d 工作量)

图例：◆ 里程碑 / █ 执行 / ▒ 观察（靠埋点占比判断） / ░ 并行不占关键路径 / ▪ 缓冲
```

### 阶段 2 工程项明细（排期 8/6–8/12；估时引自 plan §8.2）

| 任务 | 内容 | 估时（plan 编号） |
|---|---|---|
| 备份式安装 | 安装/清理的 `RemoveAll` 改为 `mv` 到 `~/.dws/skills/backup/<ts>-<mode>/`；任一 agent home 失败自动回滚、非 0 退出；保留最近 2 份。改变"清理失败仅 warning 继续装"语义（现状 `internal/app/skill_setup.go:598-608`），不留半装状态 | 1.5–2d（P0-3） |
| `~/.dws/skills/state.json` | mode / cli_version / installed / previous；setup、upgrade、各安装脚本同构写入；缺失/损坏时按磁盘形态反推兜底（有 `dws/` → mono，有 `dingtalk-*` → multi，都有 → 报 drift） | 1d + 0.5–1d（P0b-1/2） |
| `dws skill mode` | `status` / `set <mode>` / `rollback` / `--dry-run`；`set` 复用 setup 安装实现 + 备份/记账 | 2–2.5d（P1-1） |
| `x-dws-skill-mode` 请求头 | 仿 `x-dws-channel`（`internal/auth/oauth_helpers.go:1425-1426`），MCP/登录请求带上 `mono\|multi`，灰度占比与下线判据的测量前提 | 0.5d（P0-4） |
| agent home 清单单一事实源 + 门禁 | 清单目前 5+ 处各写一份、靠注释约定 keep in sync（如 `internal/upgrade/paths.go:23-52`）；下沉共享清单 + 新增 `scripts/policy/check-agent-homes-sync.sh` 进 `make policy` | 0.5d + 1d（P0c-1/2） |
| `upgrade --dry-run` 文案对齐 multi | 预览输出按 multi 平铺布局描述写删路径 | 0.5d 内 |

### mono 下线判据（阶段 4，8/22–8/27 核对；判据全满足才动，均为 DWS 自身指标）

> 判据更新（2026-08-05）：原判据 3「悟空 bundled skill 分发线（dws_res →
> bundled-skills，本仓库外）已切 multi」作废——悟空分发线已于当日决策
> 下线，无仓外 mono 依赖，mono 下线不再有仓外节奏闸门，只取决于下列
> DWS 自身指标（原判据 4/5 顺延为 3/4）。详见文末
> 「悟空线下线的影响（2026-08-05）」。

1. `x-dws-skill-mode=multi` 请求占比 ≥ 90%；
2. mono 主动 opt-in 率 ≤ 2%；
3. 连续两周无 multi 相关 P1（从 beta 发布 8/14 起算，8/27 满两周，与
   核对窗口对齐）；
4. 已有等价政策门替代 `scripts/policy/check-skill-context-budget.sh` 对
   `skills/mono/SKILL.md` 的依赖；`skills_embed.go` 去掉 `all:skills/mono`
   （现状 `skills_embed.go:28`）；存量 `<agent>/dws` 目录有"遇到即迁移
   清理"逻辑。

满足后于 **8/27–8/29** 单独版本窗口物理删除 `skills/mono/`（动作本身
≈2–3d）；**下线版本的 upgrade 对存量 mono 一次性自动迁移到 multi（带
备份、可 `dws skill mode rollback` 回滚）**，此后 install/setup/upgrade 的
mono 分支报错并指向迁移命令。

## 悟空线下线的影响（2026-08-05）

悟空（dws-wukong）bundled-skill 分发线（dws_res → bundled-skills）已于
2026-08-05 决策下线，对本次 mono→multi 迁移的影响：

- **仓外 mono 依赖清零**：mono 下线判据不再含任何本仓库外的分发线，
  阶段 4 的启动时机完全由 DWS 自身指标（占比 / opt-in 率 / P1 观察 /
  政策门替代）决定，不再受悟空客户端改造与发版节奏闸门约束。
- **设计资产留档**：悟空 bundle 的自描述打包（`manifest.json` +
  `scripts/_install.sh`）与 symlink 提升消费设计仍具参考价值，留档于
  [skill-wukong-comparison.md](skill-wukong-comparison.md)，供未来
  DWS 打包形态演进时参考。
- **唯一 multi 事实源**：悟空 `dingtalk-skills/` 派生树随之作废，DWS
  `skills/multi/` 成为唯一的 multi skill 事实源，原"两套 multi 树内容
  同源"问题不再需要决策。

## 明确不做

- 不做服务端远程配置/灰度平台（D2：beta 轨 + 公告 kill switch 替代）。
- 不做 L2 确定性分桶 `rollout.json`（8/30 倒排下时间不够，已砍；L1 beta
  轨 + kill switch + 备份回滚已覆盖灰度语义）。
- 不动 `dws-skills.zip` 产物布局（D1：已含 mono/multi 双树，零改动）。
- 不动市场 skill（`dws skill install`）的安装语义（D4 前缀约定不变）。
- 并行期内（至 8/27 mono 物理下线版本前）不删 mono 代码与产物，只降级
  为 opt-in；8/27–8/29 按倒排物理删除。

---

## 风险表

| 风险 | 现状 | 缓解计划 |
|---|---|---|
| 半装状态无备份 | 安装/清理仍是 `RemoveAll` 直删，清理失败仅 warning 继续装（`internal/app/skill_setup.go:598-630`、`internal/upgrade/paths.go:246-259`），中途失败即半装 | 阶段 2 备份式安装（RemoveAll→mv backup + 失败整体回滚，8/6–8/8）落地前，不进入 stable 全切（8/21–8/22） |
| 无状态可漂移 | 无 `state.json`，mode 只能靠磁盘形态反推；`~/.dws/skills` 缓存与 embed 版本可能漂移 | 阶段 2 `state.json`（8/6–8/9）+ 磁盘反推兜底；upgrade 已会刷 multi 缓存（`internal/upgrade/paths.go:290-296`）缓解一半 |
| 悟空线外挂（已解除，2026-08-05） | 悟空 bundled skill 分发线（dws_res → bundled-skills）已于 2026-08-05 决策下线，原"仓外节奏卡住 mono 下线"风险移除 | 无仓外依赖遗留；bundle 自描述打包（manifest.json + _install.sh）与 symlink 消费设计留档 [skill-wukong-comparison.md](skill-wukong-comparison.md) 供未来打包参考 |
| ps1/js 无自动化测试覆盖 | `test/scripts` 只断言脚本暴露 mode 选项（`install_script_test.go:529-576`），ps1/js 的真实安装/清理行为无行为级测试 | 阶段 1.5 已兜底：五面实测 9/9 PASS + node 冒烟 4 场景；阶段 2 清单门禁（check-agent-homes-sync.sh）锁住五面清单漂移，行为测试随备份式安装一起补 |
| 8/30 硬 deadline 滑期 | 关键路径缓冲仅 1 天（8/30）；beta 观察已压到 1 周、L2 分桶已砍，无可再压缩环节 | 任一环滑期即触发降级方案（见 roadmap「降级方案」）：8/14 beta 未出 → 先保 stable 全切 8/21，mono 物理删除顺延下一个版本窗口，但 8/30 前必发 deprecation + upgrade 一次性自动迁移（带备份） |
