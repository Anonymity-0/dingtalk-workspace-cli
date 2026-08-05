# DWS multi-skill vs 悟空（dws-wukong）分发线对比

> ⚠️ **留档注记（2026-08-05）**：悟空（dws-wukong）bundled-skill 分发线
> 已于 2026-08-05 决策下线。本文自此仅作**历史调研留档**，不再作为任何
> 对齐依据——文中的「判据 #3」「对齐清单（§7.1）」「待确认问题（§7.2）」
> 等均随悟空线下线而 MOOT。其中 bundle 的自描述打包设计
> （`manifest.json` + `scripts/_install.sh`）与 symlink 提升消费方式
> 可作为未来打包方案参考保留。
>
> 撰写日期：2026-08-05。聚焦 **multi-skill** 主题：DWS 侧（本工作区
> `feat/skill-mode-migration`）已把 multi 翻转为全通道默认，而 mono 下线
> 原判据 #3 曾要求"悟空 bundled skill 分发线（dws_res → bundled-skills）
> 已切 multi"（[skill-multi-roadmap.md](skill-multi-roadmap.md) 阶段 4、
> [skill-multi-migration-plan.md](skill-multi-migration-plan.md) §5；
> 该判据已于 2026-08-05 随悟空线下线作废）。
> 本文盘清悟空线现状、两边差异、切换影响与对齐清单。
>
> 配套阅读：[skill-multi-roadmap.md](skill-multi-roadmap.md)（DWS 侧
> as-implemented 事实源）、[skill-distribution-mechanism.md](skill-distribution-mechanism.md)
> （DWS 分发/消费链路调研）。

## 0. 摘要（TL;DR）

- **DWS 侧**：multi（`skills/multi/`，19 个 `dingtalk-*` 产品 skill +
  `dws-shared`）已是安装（4 脚本面）、升级、`dws skill setup` 五面默认；
  产物 `dws-skills.zip` 恒含"根 mono 副本 + `mono/` + `multi/`"三树，
  二进制 embed 双树（`skills_embed.go:28`）。**产物零改动即完成默认翻转**。
- **悟空侧**：multi 打包能力**已合入 dws-wukong `develop`**（merge
  `9eb801e5`，"DWS MultiSkill 与 Qwen Work Cloud 六平台打包"），但
  **正式发版 target `make real-platform` 仍只打 mono**
  `dingtalk-workspace.zip`；multi 以 `dingtalk-workspace-bundle.zip` 双包
  形态存在（`bundle-platform` / `package-dual`），本地有 2026-07-03 的双包
  实测产物，但**未随已发布版本出门**（`release/0.2.97`–`0.2.99` 均不含该
  merge）。
- **关键缺口在客户端**：RewindDesktop（悟空桌面端）构建期
  `download_binary.py` 只认 `dingtalk-workspace.zip` 单 zip；运行时
  `dws_update.rs` 灰度更新也只 upsert 单个 `dingtalk-workspace` skill；
  全仓 grep 无 `dingtalk-workspace-bundle` / T4b 处理。**端内尚无消费
  multi bundle 的代码路径**。
- **悟空线的 multi 与 DWS 的 multi 是两套独立维护的树**（dws-wukong
  `dingtalk-skills/` 12 产品 + `dws-shared`，由本仓 mono 机械派生；DWS
  `skills/multi/` 19 产品 + `dws-shared`），内容靠 SOP 人工对齐，无自动
  同源。判据 #3 的"切 multi"首先要解决的是**打包形态 + 端内加载**，
  内容同源是紧随其后的问题。
- 结论（历史）：判据 #3 远未满足。对齐需要 dws-wukong 仓（发版 target）、
  RewindDesktop 仓（构建期 + 运行时两条加载路径）两侧改动，详见 §7 清单。
  **（2026-08-05 MOOT：悟空线下线，判据 #3 已作废——见
  [skill-multi-roadmap.md](skill-multi-roadmap.md) 阶段 4 判据更新；
  上述对齐工作不再需要。）**

## 1. 证据源与版本快照

本地仓库（均为真实磁盘证据，非仅凭文档）：

| 仓库 | 本地路径 | 核查时状态 |
|---|---|---|
| DWS（本仓） | `~/GolandProjects/open-source/dws-skill-mode-migration` | `feat/skill-mode-migration`，含未 commit 的阶段 1 变更 |
| dws-wukong 主仓 | `~/GolandProjects/open-source/dws-wukong` | checkout `codex/deploy-qwenwork-dev`（落后 develop 602 commits）；本文 Makefile/脚本结论均以 `develop` 分支内容（`git show develop:...`）为准 |
| dws-wukong multiSkill 工作区 | `~/GolandProjects/open-source/dws-wukong-multiSkill` | detached @ `9eb801e5`（multiSkill merge 本体），`target/` 有 2026-07 实测产物 |
| RewindDesktop | `~/IdeaProjects/RewindDesktop` | checkout `dws/0.2.98`；`develop` 上 `DEFAULT_DWS_RES_URL` = pod `0.2.96` |

关键版本事实：

| 事实 | 证据 |
|---|---|
| multiSkill merge `9eb801e5` 已入 `develop` 与本地 `release/0.2.100` | `git branch --contains 9eb801e5` |
| `release/0.2.97` / `0.2.98` / `0.2.99` **不含** multiSkill merge；其 `real-platform` 不打 bundle | `git merge-base --is-ancestor` + `git show origin/release/0.2.99:Makefile` |
| 当前发给悟空的 pod 包为 mono：`dws_res_mac.zip` 内仅 `dingtalk-workspace.zip`（单 skill，`SKILL.md`+`references/products/*`）+ 双架构二进制 | 主仓 `target/dws_res_mac.zip`（2026-06-01）`unzip -l` 实测 |
| 双包形态已实测：`dws_res_mac/` 同时含 `dingtalk-workspace.zip`（924K）与 `dingtalk-workspace-bundle.zip`（1.27M） | multiSkill 工作区 `target/dws_res_mac.zip`（2026-07-03）实测 |
| multi bundle 内部布局：`manifest.json` + `scripts/_install.sh` + `skills/<name>/SKILL.md...` 平铺目录 | multiSkill 工作区 `target/dingtalk-workspace.zip`（2026-07-24）实测 |

**本地未能验证**（详见 §7.2 问题清单）：pod 线上当前包内容（需内网
SSO）；`/Applications/Wukong.app` 未安装在本机（bundled-skills 目录不存在，
无法核对在端真实 zip）；RewindDesktop 端"T4b 二选一"灰度逻辑（全仓无匹配，
疑似未开发或在平台侧）；Qwen Work Cloud 六平台打包的实际发布状态。

## 2. 分发链路对比

### 2.1 链路全景

DWS 侧（本仓，multi 已默认）：

```text
skills/mono/ ─┬─ go:embed all:skills/mono all:skills/multi (skills_embed.go:28)
skills/multi/ ┘      │  `dws skill setup` 默认源（embed 优先）
              │
              └─ scripts/release/post-goreleaser.sh:220-248 ──► dws-skills.zip
                 （根 = mono 副本 + mono/ + multi/，三树恒含）
                        │
        GitHub Release / Gitee / OSS / npm tarball / Homebrew / 专项脚本
                        │
   install.sh · install.ps1 · install-skills.sh · npm install.js（四面默认 multi）
   + `dws skill setup`（第五面默认 multi）+ `dws upgrade`（布局探测→multi 刷新）
                        │
              各 agent home 平铺 dingtalk-*/（互斥清理 dws/ 与过期 skill）
              + ~/.dws/skills/{mono,multi} 双缓存
```

悟空侧（dws-wukong + RewindDesktop，发版线仍 mono）：

```text
上游 CLI 仓 ../dingtalk-workspace-cli（go.mod replace，sync-upstream 按
CLI_UPSTREAM_TAG 重建 release 分支）──► 只提供 Go 代码，不提供 skill 内容

dws-wukong 仓内 skill 内容（自维护）：
  dingtalk-workspace/（mono 源，含 overlays/real）
      ├─ build-workspace-zip ──► dingtalk-workspace.zip（mono，单 skill）
      └─ scripts/sync-monolith-to-multiskill.py ──► dingtalk-skills/（multi 派生树）
             └─ scripts/build-bundle.sh ──► dingtalk-workspace-bundle.zip
                  （manifest.json + scripts/_install.sh + skills/<name>/）

make real-platform（发版默认）──► dws_res_{mac,win}.zip
      = dws 二进制 + dingtalk-workspace.zip（仅 mono）
make bundle-platform / package-dual（已合入 develop，未用于正式发版）
      = dws 二进制 + dingtalk-workspace.zip（mono 兜底）+ dingtalk-workspace-bundle.zip
                        │
        pod.alibaba-inc.com zipUpload（SSO 浏览器上传，版本号独立递增）
                        │
        RewindDesktop scripts/download_binary.py（DEFAULT_DWS_RES_URL 手工对齐）
                        │
        Wukong.app Contents/Resources/resources/
            ├─ dws/bin/dws（二进制）
            └─ bundled-skills/dingtalk-workspace.zip（原样拷贝的单 zip）
                        │
        运行时两条路：
        a) 启动同步 initialize_bundled_skills_from_resources
           → 解 zip 到中央技能库 ~/.real/.skills/bundled/dingtalk-workspace
           （及 ~/.real/users/*/.skills/bundled）
        b) 灰度自更新 dws_update.rs：Gaea 开关 wukong/dws_auto_update_enabled_v2
           → LWP /r/Adaptor/DwsGrayI/getLatest 取 {version,url,sha256}
           → 下载 dws_res → 换 seed 二进制 + 换 bundled zip + upsert 单 skill
```

### 2.2 链路对照表

| 环节 | DWS（本仓） | 悟空线 | 锚点（悟空侧） |
|---|---|---|---|
| skill 事实源 | `skills/mono` + `skills/multi`（同仓双树，multi 19 产品 + dws-shared） | dws-wukong 仓 `dingtalk-workspace/`（mono 源）→ 派生 `dingtalk-skills/`（12 产品 + dws-shared）；**与上游 skills/ 无自动同步** | `scripts/sync-monolith-to-multiskill.py` docstring |
| 二进制与 skill 的版本耦合 | embed 进二进制，天然同版 | 二进制来自上游 tag（`go.mod:58` replace + `sync-upstream` pin `CLI_UPSTREAM_TAG`）；skill 在 dws-wukong 仓随 `VERSION`/`main.go` 双写发版 | dws-wukong `Makefile` `sync-upstream` |
| 打包产物 | `dws-skills.zip`：根 mono 副本 + `mono/` + `multi/`（`post-goreleaser.sh:220-248`）；embed 双树 | `dws_res_{mac,win}.zip`：二进制 + `dingtalk-workspace.zip`（mono）；双包 target 已存在但未上发版线 | dws-wukong `Makefile` `real-platform` / `bundle-platform` / `package-mac-dual` |
| 渠道 | GitHub/Gitee/OSS/npm/Homebrew/专项脚本，7 个安装面 | pod zipUpload（SSO）→ RewindDesktop `download_binary.py` → 客户端 bundle | release skill 文档 + `download_binary.py:72-84` |
| 端内安装 | 4 脚本 + `dws skill setup` 平铺到 16 个 agent home（父目录门控） | 构建期拷贝 zip 进 app 资源；启动时解到 `~/.real/.skills/bundled/` | `startup.rs:486-554` |
| 运行时更新 | `dws upgrade`（布局探测→multi 刷新 + 互斥清理 + 缓存刷新） | 客户端灰度自更新（Gaea + LWP），整包替换 seed 二进制 + skill zip | `dws_update.rs:107,27-28,440-545` |
| 灰度能力 | 无服务端：beta 轨（L1）+ 可选 `rollout.json` 分桶（L2）+ 公告 kill switch（决策 D2） | 有服务端：Gaea 开关 + LWP getLatest + pod 版本号 | 决策记录 D2 vs `dws_update.rs` |

## 3. skill 布局对比

### 3.1 三种形态

| 形态 | 布局 | 消费方 |
|---|---|---|
| DWS mono | 单 skill：`SKILL.md` + `references/` + `scripts/`，装进 `<agent-home>/dws/` | DWS legacy 安装面；悟空 `dingtalk-workspace.zip` 同构（多 `plugins/`、real overlay） |
| DWS multi | 平铺目录树：`multi/dingtalk-<product>/{SKILL.md,references,scripts}` + `multi/dws-shared/`，无 manifest、无安装器，拷贝即平铺到 agent home | DWS 五面默认；`dws upgrade` 探测 `multi/` 树（`internal/upgrade/paths.go:363-369`） |
| 悟空 multi bundle | zip 内 `manifest.json`（`{"version":...}`）+ `scripts/_install.sh` + `skills/<name>/` 平铺目录（含 `dws-shared`） | **Qwen Work Cloud 已消费**（`_install.sh` 把 `skills/*` 以 symlink 提升到一层 skills 根，供 Codex/OpenCode 扫描；`.dws-multiskill-current` + `.dws-multiskill-links` 记账）；**Wukong.app 尚未消费** |

注意两种 multi 的"平铺"语义不同：DWS multi 是**裸目录树**，由安装面自己
拷贝到各 agent home；悟空 bundle 是**自描述包**（manifest + 安装脚本），
由消费方解包后提升。`dws-skills.zip` 的 `multi/` 树与
`dingtalk-workspace-bundle.zip` 的 `skills/` 树**布局同构但内容不同源**
（见 §3.3）。

### 3.2 悟空客户端加载约定（RewindDesktop 实测）

构建期（`scripts/download_binary.py`）：

- `DWS_RES_WORKSPACE_ZIP = "dingtalk-workspace.zip"`（`:103`），
  `resolve_dws_res_contents`（`:1366-1380`）强制 dws_res 内必须同时有
  平台二进制和**这个文件名的 zip**——`dingtalk-workspace-bundle.zip` 会被
  原样忽略（不报错，但也不使用）。
- `sync_dingtalk_workspace_bundle`（`:1422-1432`）把该 zip **原样拷贝**到
  `tauri-app/src-tauri/resources/bundled-skills/dingtalk-workspace.zip`，不解包。

运行时（`tauri-app/src-tauri/src/skills/startup.rs`）：

- `collect_bundled_skill_sources`（`:486-506`）扫描 `resources/bundled-skills/`：
  **每个子目录或每个 `.zip` = 一个 skill**，`skill_id = 文件主干名`
  （`dingtalk-workspace.zip` → skill id `dingtalk-workspace`）。
  该扫描**天然支持多 skill 平铺**（放 `dingtalk-mail.zip`、`dingtalk-doc.zip`
  就会被分别注册），但**不认识嵌套 bundle**：若把
  `dingtalk-workspace-bundle.zip` 丢进去，只会被当成一个名叫
  `dingtalk-workspace-bundle` 的单 skill 解开（内容是 manifest+skills/ 目录，
  不会被提升）。
- `sync_bundled_skill_source`（`:533-554`）解 zip 拷贝进中央技能库；
  `cleanup_removed_bundled_skills`（`:508-531`）会删除 bundled-skills 里
  已不存在的 bundled skill（mono→multi 切换时可自动清掉旧
  `dingtalk-workspace`，前提是 store 记录完好）。
- 灰度自更新 `dws_update.rs:440-545` 硬编码单 skill：
  `upsert_bundled_skill_from_source(store, DWS_WORKSPACE_SKILL_ID, …)`
  （`DWS_WORKSPACE_SKILL_ID = "dingtalk-workspace"`），换包 = 替换
  `bundled-skills/dingtalk-workspace.zip` + 重 upsert 这一个 skill。

结论：端内**构建期与运行时两条路都按"单 zip 单 skill"接线**；要支持
multi，二选一：(a) 端内学会解 bundle（manifest + 提升子 skill），或
(b) 打包侧把每个子 skill 打成独立 zip 平铺进 dws_res，复用现有平铺扫描
（仅需把 `dws_update.rs` 的单 skill upsert 改为遍历）。详见 §6.2。

### 3.3 两套 multi 树的集合差异

| | DWS `skills/multi/` | dws-wukong `dingtalk-skills/`（develop） |
|---|---|---|
| 产品 skill 数 | 19 | 12 |
| 共有（12 个） | aisearch, aitable, calendar, chat, contact, doc, drive, mail, minutes, misc, todo, wiki | 同左 |
| 仅 DWS 有（7 个） | dev, event, hrbrain, markdown, pat, profile, skill | — |
| 共享层 | `dws-shared` | `dws-shared`（内容独立维护） |
| 内容来源 | 本仓直接维护 | 由本仓 mono `dingtalk-workspace/` 经 `sync-monolith-to-multiskill.py` 机械派生（链接改写 + misc 桶归并） |
| 场景 skill | 不涉及 | `dingtalk-products-skills/` 23 个 scenario skill **仅保留源码，不进统一发布包**（`build-bundle.sh` 头注释） |

含义：即使悟空线明天切到 multi 形态，其 multi **内容**与 DWS multi 也不
一致（少 7 个产品、各自演化）。判据 #3 只要求"分发线切 multi"（形态），
但长期看内容同源（或明确的子集契约）需要一并决策。
**（2026-08-05 MOOT：悟空线下线，"DWS skills/multi(19) vs 悟空
dingtalk-skills(12) 分歧"的长期统一问题随之作废，无需统一；DWS
`skills/multi/` 成为唯一 multi 事实源。）**

## 4. 版本对齐对比

| 维度 | DWS | 悟空线 |
|---|---|---|
| skill↔CLI 耦合 | embed 进二进制，`dws skill setup` 装的就是本二进制版本（`skills_embed.go` + `skill_setup_embed.go`） | 二进制版本 = 上游 tag（`sync-upstream` pin）；skill 版本 = dws-wukong `Makefile VERSION` + `main.go version` 双写；**两者只通过"同一次发版动作"对齐，无结构性强约束** |
| 产物版本 | `dws-skills.zip` 随 goreleaser 与二进制同 tag 发布 | pod 版本号独立于 dws 版本（右most 段 +1 递增，mac/win 各自一条线，如 mac `0.2.28.0`、win `0.2.2`）；RewindDesktop `DEFAULT_DWS_RES_URL` 手工改指 |
| 端内版本事实源 | — | 客户端以 `dws --version` 输出为准（`get_dws_version_sync`，失败回退 `versions.json`）；skill zip 无独立版本概念（mono zip 内无 manifest） |
| multi 包版本 | 无 manifest；版本 = 所属 zip/二进制版本 | bundle 内 `manifest.json` 带 `version`（`build-bundle.sh` 由 `$(VERSION)` 写入）——**multi 形态反而第一次给 skill 包带来了显式版本号** |
| 升级时的布局兼容 | 新 zip 恒含 `multi/`，`LocateSkillsRoot` 优先 multi；老 zip 自然落回 mono（决策 D1，产物零改动） | dws_res 布局固定（二进制 + workspace zip）；双包形态下 mono zip 保留作兜底（`package-mac-dual` 注释："端内 validate 必含，bundle 缺失时兜底"） |

## 5. 更新 / 回滚对比

| 维度 | DWS | 悟空线 |
|---|---|---|
| 更新触发 | 用户主动 `dws upgrade` / 重装脚本 | 两条：(a) 随客户端版本更新（app 内嵌资源替换 + 启动同步）；(b) 运行时灰度自更新（Gaea `dws_auto_update_enabled_v2` + LWP getLatest → 下载整包 → 换 seed 二进制 + 换 bundled zip + upsert skill） |
| skill 刷新语义 | 布局探测（multi 优先）+ 按 home 互斥清理（删 `dws/`、过期 `dingtalk-*`/`dws-shared`），清理失败则该 home 不装，杜绝共存（`internal/upgrade/paths.go:217-299`） | 启动同步按 bundled-skills 现状全量对账（新增拷贝、缺失删除，`startup.rs:508-554`）；自更新路径是单 zip 替换 + 单 skill upsert（`dws_update.rs:462-513`） |
| 回滚 | `dws upgrade --rollback` 只回二进制不回 skill；备份式安装 + `dws skill mode rollback` 是阶段 2 P0，**尚未落地** | 无显式回滚命令；事实回滚 = Gaea 开关关闭/改指旧 pod 版本重新下发，或客户端版本回退；`replace-wukong-skill.sh`（仅存在于 `codex/deploy-qwenwork-dev` 分支）提供人工替换 + 重启 |
| 半装保护 | 现状 `RemoveAll` 直删、失败仅 warning（风险表已列，阶段 2 改备份式） | zip 整体替换 + preflight 可写性检查，失败提示重启；粒度为整个 skill 包，无子 skill 级半装概念 |
| 离线/内网 | embed 兜底（无网可 setup） | app 内嵌 zip 兜底；灰度通道依赖内网 LWP/pod 可达 |

## 6. 切换影响分析

### 6.1 DWS 切 multi 默认 / 最终删 mono 对悟空线的影响点

| # | 影响点 | 评估 |
|---|---|---|
| 1 | DWS 删 `skills/mono` 后上游 embed 只剩 multi；悟空二进制由上游 tag 构建，`dws skill setup` 行为随之变 | **低**。悟空客户端不从 embed 装 skill（走 bundled zip）；但悟空用户在端内手动跑 `dws skill setup` 时会得到 multi——行为变化需在悟空侧公告 |
| 2 | `dws-skills.zip` 布局（根 mono + mono/ + multi/） | **零影响**。悟空线不消费 `dws-skills.zip`；DWS 侧也已承诺不动该产物（决策 D1、"明确不做"） |
| 3 | dws-wukong 的 mono 内容源 `dingtalk-workspace/` 与上游 `skills/mono` 本就各自维护 | **低（但需注意）**。上游删 mono 不会直接打破 dws-wukong 构建；但两边 mono 的"内容漂移对照基准"消失，dws-wukong mono 将彻底成为孤儿副本，加速与上游 CLI 能力的文档漂移 |
| 4 | mono 下线判据 #3 反向卡住 DWS 侧进度 | **高（流程性）**。悟空线一天不切 multi，DWS 就不能物理删 `skills/mono/`（roadmap 风险表"悟空线外挂"行）。**（已解除 2026-08-05：悟空线下线，判据 #3 作废，DWS 侧进度不再受仓外闸门约束）** |

### 6.2 悟空线"吃 multi"的改造选项

**选项 A：端内解 bundle（dws-wukong 现有 bundle 产物直接被消费）**

- dws-wukong 侧：`real-platform` 改打（或加打）`dingtalk-workspace-bundle.zip`
  ——`bundle-platform` / `package-dual` 已就绪，基本零新开发。
- RewindDesktop 侧（主要工作量）：
  - `download_binary.py`：`resolve_dws_res_contents` 接受/校验
    `dingtalk-workspace-bundle.zip`，同步进 `resources/bundled-skills/`；
  - `startup.rs`：识别 bundle（`manifest.json` + `skills/*`），把每个子
    skill 注册为独立 bundled skill（逻辑等价于 `_install.sh` 的提升，
    但落在中央技能库）；
  - `dws_update.rs`：灰度自更新从"单 skill upsert"改为"bundle 全量对账"
    （可复用启动同步的对账逻辑）。
- 优点：与 Qwen Work Cloud 已消费的包形态一致，一份产物两个端；bundle 自
  带 `manifest.json` 版本号。
- 缺点：端内要新增 bundle 解析/提升代码与测试；`skills/` 嵌套布局与现有
  "一 zip 一 skill"约定不同，需谨慎处理迁移期（旧 mono skill 清理）。

**选项 B：打包侧拆 zip（端内零新格式）**

- dws-wukong 侧：新增 target 把 `dingtalk-skills/` 每个子 skill 打成独立
  zip（`dingtalk-mail.zip` … `dws-shared.zip`）平铺进 dws_res。
- RewindDesktop 侧：`download_binary.py` 改为同步多个 zip；
  `startup.rs` 现有平铺扫描**零改动**（自动注册每个 zip）；
  `dws_update.rs` 仍需从单 skill upsert 改为遍历。
- 优点：复用端内现有"一 zip 一 skill"约定，启动路径几乎不动。
- 缺点：与 Qwen Work 的 bundle 形态分叉（一份内容两种包）；dws_res 内文件
  数膨胀；`dws-shared` 作为独立 skill id 出现在用户可见列表里需要确认
  端内展示策略。

**选项 C（过渡态，事实已在用）**：双包并存——mono zip 兜底 + bundle 灰度，
端内按灰度二选一。`package-dual` 的注释已写明此意图（"端内 T4b 二选一，
monolith 端内 validate 必含，bundle 缺失时兜底"），但**端内 T4b/灰度选择
逻辑在 RewindDesktop 尚未找到实现**，当前双包发出去也只会用 mono。

### 6.3 mono 下线判据 #3 的建议验收方式

> **（2026-08-05 MOOT：判据 #3 已随悟空线下线作废，本节验收清单不再
> 适用，仅留档。）**

判据原文："悟空 bundled skill 分发线（dws_res → bundled-skills，本仓库
外）已切 multi"（原 [skill-multi-migration-plan.md](skill-multi-migration-plan.md)
§5-3；该判据已于 2026-08-05 作废，plan §5 已重新编号）。建议按以下
可核查项验收（全绿才算满足）：

1. **发版**：dws-wukong 正式 release 流程（release skill 文档中的
   `make real-platform` 路径）产出的 dws_res 内含 multi 形态包（bundle 或
   平铺 zip 集），且 pod 上当前版本即为该形态。
2. **构建期消费**：RewindDesktop `develop` 的 `download_binary.py` 把 multi
   形态同步进 `bundled-skills/`（不再是只认 `dingtalk-workspace.zip`）。
3. **运行时消费**：悟空端启动同步后，`~/.real/.skills/bundled/` 下出现
   `dingtalk-*` 多 skill（而非单个 `dingtalk-workspace`）；灰度自更新路径
   同样支持多 skill 对账。
4. **实机回归**：全新安装悟空 → 技能列表出现各 `dingtalk-*` skill 且
   路由正常；从 mono 旧版升级 → 旧 `dingtalk-workspace` 被清理、无双份
   派发；灰度通道下发一次 multi 包 → 更新后无残留。
5. **回退预案**：悟空侧保留 mono 兜底产物或快速重发能力，直至 DWS 删
   mono 窗口关闭。

## 7. 结论

### 7.1 对齐清单（悟空侧待办，按优先级）

> **（2026-08-05 MOOT：悟空线下线，本清单整体不再需要执行，仅留档。）**

| 优先级 | 事项 | 仓库/位置 | 备注 |
|---|---|---|---|
| P0 | 决策端内 multi 消费方案（选项 A 解 bundle vs 选项 B 平铺 zip） | RewindDesktop + dws-wukong 联合 | 建议 A：与 Qwen Work 已消费形态一致，且 bundle 带版本 manifest |
| P0 | `download_binary.py` 支持 multi 形态同步进 bundled-skills | RewindDesktop `scripts/download_binary.py:103,1366-1432` | 选项 A 下识别 `dingtalk-workspace-bundle.zip` |
| P0 | 启动同步/技能库支持 bundle 解包与子 skill 注册 | RewindDesktop `tauri-app/src-tauri/src/skills/startup.rs:486-554` | 含旧 mono skill 的迁移清理（现有 `cleanup_removed_bundled_skills` 可复用语义） |
| P0 | 灰度自更新支持 multi（单 skill upsert → 多 skill 对账） | RewindDesktop `.../dws_update.rs:440-545` | 否则运行时更新会把 multi 打回 mono |
| P1 | 正式发版 target 切 multi（`real-platform` 改打/加打 bundle，或改用 `bundle-platform`/`package-dual`） | dws-wukong `Makefile` | 打包能力已在 develop，缺的是设为默认 + release skill 文档同步更新 |
| P1 | 端内灰度选择逻辑落地（双包二选一/T4b，或确认直接全量切） | RewindDesktop（未找到现有实现） | 若选选项 C 过渡则必须 |
| P1 | 两套 multi 树的内容同源策略：dws-wukong `dingtalk-skills/`（12 产品）vs DWS `skills/multi/`（19 产品） | dws-wukong + 本仓 | **MOOT（2026-08-05）**：悟空线下线，无需统一；原备注：至少明确"悟空子集"契约与同步 SOP 的归属；7 个缺失产品（dev/event/hrbrain/markdown/pat/profile/skill）是否需要进悟空 |
| P2 | `replace-wukong-skill.sh` 等运维脚本支持 bundle 形态 | dws-wukong `scripts/deploy/` | 当前只在特性分支且只处理单 zip |
| P2 | 悟空侧公告：端内 `dws skill setup` 行为随上游 embed 变化 | dws-wukong 发版流程 | 对应 §6.1 影响点 1 |
| P2 | 判据 #3 验收清单（§6.3）写入 DWS roadmap 并跟踪 | 本仓 `docs/skill-multi-roadmap.md` | 阶段 3 期间启动 |

### 7.2 待确认问题（本地无法闭环，需找人/仓库确认）

| # | 问题 | 建议确认方 |
|---|---|---|
| 1 | pod 线上当前 `dws_res_mac/win` 的版本与内部构成（是否已有人发过双包） | pod.alibaba-inc.com（需内网 SSO）/ 悟空发版 owner |
| 2 | "端内 T4b 二选一"灰度逻辑是否已存在（在哪个仓库/平台），还是仅写在 Makefile 注释里的规划 | RewindDesktop 团队 / 悟空端内灰度平台 owner |
| 3 | LWP `/r/Adaptor/DwsGrayI/getLatest` 服务端返回的下载 URL 指向何处（pod？另一制品库？），multi 包下发是否需要服务端配合改造 | Adaptor/DwsGrayI 服务端 owner |
| 4 | Qwen Work Cloud 六平台打包的发布状态与其对 bundle 的消费方式是否可作为悟空端改造的直接参照 | dws-wukong 仓 owner（merge `9eb801e5` 提交者） |
| 5 | 悟空端内是否允许 `dws-shared` 作为独立 bundled skill 暴露（名称/展示/路由策略），还是应内联进各产品 skill | RewindDesktop 技能库 owner |
| 6 | dws-wukong `dingtalk-skills/` 与上游 `skills/multi/` 的长期关系：保持派生自本仓 mono，还是改为从上游 multi 同步（**MOOT 2026-08-05**：悟空线下线，无需统一） | dws-wukong + DWS 双侧 owner 联合决策 |
| 7 | 悟空线切换的目标时间窗（决定 DWS 阶段 4 判据 #3 的最早可满足点）（**MOOT 2026-08-05**：判据 #3 已作废） | 悟空发版 owner |

---

*本文所有"已验证"结论均可按 §1 的仓库路径与文中锚点复查；未能本地验证
的项集中在 §7.2。*
