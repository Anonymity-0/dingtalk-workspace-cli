# DWS Skill mono→multi 灰度能力设计（rollout capability）

> 本文回答一个问题：**在没有服务端远程配置/灰度平台的前提下，mono→multi
> 默认翻转如何灰度发布、如何观测、如何止血**。关联文档：
> [skill-multi-roadmap.md](skill-multi-roadmap.md)（迁移事实源，本文展开其
> 阶段 3「灰度切流」）、[skill-multi-migration-plan.md](skill-multi-migration-plan.md)
> §3.7（灰度与止血的原始设计）。代码锚点快照：`feat/skill-mode-migration`
> 工作区未 commit 变更（2026-08-05）。

---

## TL;DR 推荐路线

| 步 | 动作 | 层级 | 前置 |
|---|---|---|---|
| 1 | 把当前工作区的「五面默认 multi」拆成**版本门控默认**（beta→multi / stable→mono），同一份代码发 beta 轨先吃 | L1 | 无（本文 §2.1） |
| 2 | 落地阶段 2 四件套：备份式安装、`state.json`、`dws skill mode`、`x-dws-skill-mode` 请求头 | L3 + kill switch | roadmap 阶段 2（P0-3/P0b/P1-1/P0-4） |
| 3 | beta 轨观察 ≥2 周：请求头 multi 占比、issue 流入 | L3 | 步 1+2 |
| 4 | stable 切流：直接 100%（小用户量可接受），或随 stable 连续补丁版发 `rollout.json` 5%→20%→50%→100% 粘性分桶 | L2（可选） | 步 2 的 `state.json` |
| 5 | kill switch 常备：beta 撤回（已有）/ `pct=0` 补丁版（L2）/ 公告 `dws skill mode set mono` + 备份回滚 | — | 步 2 |

---

## 1. 现状盘点：今天可用于灰度的全部旋钮

### 1.1 旋钮总表

| # | 旋钮 | 代码锚点 | 生效范围 | 盲区（谁够不着） |
|---|---|---|---|---|
| K1 | GitHub Release 双轨（stable / prerelease） | `internal/upgrade/github.go:100-106`（`ReleaseTrack`）；`internal/app/upgrade.go:870-875`（`upgradeTrack`）；release.yml 强制版本→轨映射（`release.yml:547` 含 `-beta.` → prerelease） | `dws upgrade --beta` / `--version vX.Y.Z-beta.N` 的二进制+skill 升级 | 不用 `dws upgrade` 的人；Gitee/OSS 镜像用户（见 K3/K4） |
| K2 | npm dist-tag 双轨（`latest` / `beta`） | `release.yml:1761-1762`（prerelease→`beta` tag）；发布防倒退 `release.yml:1828-1835`；撤回脚本 `scripts/release/withdraw-release.sh:652-665`（dist-tag 回拨+deprecate） | `npm i dingtalk-workspace-cli@beta` 的新装/重装 | npm 默认安装（`@latest`）用户无感；npm 装完即走、不再 `npm i` 的存量 |
| K3 | Gitee 镜像（代码 + release 资产） | main 代码镜像 `.github/workflows/mirror-to-gitee.yml:42-82`；release 资产镜像 `release.yml:1957-1965`（`sync-to-gitee.sh`）；安装脚本侧 `DWS_GITEE_REPO` 解析 `scripts/install.sh:18-19`、`scripts/install-skills.sh:21-24` | curl\|sh / install-skills.sh 的国内用户（显式 env 或 GitHub 不可达自动回退） | **`dws upgrade` 不到 Gitee**：upgrade client 只打 GitHub API（`internal/app/upgrade.go:48` → `internal/upgrade/github.go`，包内无任何 Gitee 引用）；Gitee release 是否 prerelease 由镜像脚本原样搬运，无独立轨控 |
| K4 | OSS 镜像（ossutil 同步） | `scripts/release/sync-to-oss.sh:9-14`（`download/<version>/` + `latest.txt`/`beta.txt` 指针）；`release.yml:1897-1908` | 今天：**只写不读**——脚本注释明示「repository installers currently resolve GitHub/Gitee and do not consume these OSS pointers directly」（`sync-to-oss.sh:5-7`） | 所有人（指针无人消费）；但 `latest.txt`/`beta.txt` 是天然的**可变 channel 指针**（见 §2.2 设计复用） |
| K5 | 安装脚本 env / flag | `DWS_SKILL_MODE`：`scripts/install.sh:17`（解析 `install.sh:220-256`）、`scripts/install.ps1:293-332`、`scripts/install-skills.sh:29-33`；npm `--skill-mode` / env `build/npm/install.js:197-214`；`DWS_VERSION` 指定 beta 版 `install.sh:38` | 新装时的逐台显式控制（CI、内推灰度名单） | 只对**执行安装那一刻**生效；装完无持久化（无 state.json），事后无法得知当初怎么装的；`DWS_VERSION=latest` 在 GitHub 侧只解析 stable（`/releases/latest` 永不指向 prerelease，`install.sh:190-198`），beta 必须显式给版本号 |
| K6 | 版本门控默认值（构建期注入） | goreleaser ldflags 注入版本 `.goreleaser.yaml:22`（`internal/app.version=v{{.Version}}`）；`prerelease: auto` `.goreleaser.yaml:68` | 同一 commit，beta build 与 stable build 可表现不同默认值（§2.1 切法 B 的机制） | 脚本面拿不到 Go 变量，需各自从「解析出的版本号」重推导（§2.1）；homebrew formula 只搬 zip 根（mono 布局）到 pkgshare（`build/homebrew.rb.tmpl:26-31`），formula 本身无 mode 概念 |
| K7 | 本地遥测（opt-in） | `internal/shortcut/usage/recorder.go:82-88`（`DWS_USAGE_TRACKING=1`，默认关）；只写本地 `~/.dws/usage.jsonl`（`recorder.go:91`） | 高频命令形状挖掘（shortcut P2） | **不上传任何服务端**：对灰度占比测量零贡献；默认关意味着即使上传也无统计意义 |
| K8 | 请求头通道（已有上行链路） | MCP 请求统一注入点 `internal/app/runner.go:953-1008`（`resolveIdentityHeaders`，接线于 `runner.go:140/240/597`、`internal/app/recovery_command.go:271/337`）；登录权限检查 `internal/auth/oauth_helpers.go:1424-1428`；`x-dws-channel`（`DWS_CHANNEL`）先例 `runner.go:1006-1008`、`oauth_helpers.go:1425-1426` | 服务端（MCP 网关）已能按 header 聚合：`x-dws-agent-id`、`x-dingtalk-dws-agent-code`、`X-Cli-Version` 均在线 | 只有「已登录且发 MCP 请求」的用户可被观测；纯安装未使用、auth 失败前的用户在分子里缺席（占比偏高估，§2.3） |

### 1.2 结构性盲区（任何旋钮都够不着）

| 盲区 | 说明 | 出处 |
|---|---|---|
| ~~悟空 bundled skill 分发线~~（盲区已移除） | 悟空分发线（dws_res → Wukong.app bundled-skills）已于 2026-08-05 决策下线，本盲区随之移除；历史上该线在**本仓库外**、仅有 main CI 成功后的下游触发（`.github/workflows/notify-wukong.yml:13-38`），本仓库默认值翻转管不到它 | 原 roadmap 风险表「悟空线外挂」行（已解除）、阶段 4 原判据 3（已作废） |
| homebrew 用户 | formula 只把 zip 根（mono 副本）stage 进 `pkgshare/skills/dws`（`build/homebrew.rb.tmpl:26-31`），caveats 指向 `dws skill setup`（`:33-38`）；multi 源不在包内，`setup --mode multi` 只能靠 `~/.dws/skills/multi` 缓存 | 同上，K6 |
| 永不升级的存量 mono 用户 | 所有旋钮都作用于「新装/升级/重装」三个时点；不动作的用户一切照旧（这正是灰度的天然保护层） | — |
| 安装后不再运行 `dws` 的用户 | K8 观测不到，占比分母缺失 | §2.3 |

---

## 2. 方案设计（分层）

### 2.1 L1 渠道灰度：beta 轨先吃 multi 默认

目标：**同一代码、不同 release 轨不同默认值**，stable 用户在观察期内完全无感。

#### 切法 A：纯流程（零新增代码，不推荐单独使用）

当前工作区五面已无条件翻转 multi；直接发 beta 即完成「beta 先吃」。
问题：main 上的默认值已是 multi，**下一个 stable 无处可躲**——stable 发布
窗口一到就必须全切，观察期长短不由人；且中途想给 stable 出补丁版（hotfix）
会被迫带上 multi 默认。仅适合「beta 观察期确定短、stable 窗口确定远」的情形。

#### 切法 B：版本门控默认值（推荐）

把五面的默认值从「无条件 multi」改为「beta 版本默认 multi，stable 版本默认
mono」。判据统一用版本号是否含 `-beta.`（release.yml 已强制版本→轨唯一映射，
`release.yml:547`、`.goreleaser.yaml:68`）：

| 面 | 门控取值来源 | 落点 |
|---|---|---|
| `dws skill setup` / `dws upgrade`（Go） | ldflags 注入的 `internal/app.version`（`.goreleaser.yaml:22`），`strings.Contains(version, "-beta.")` | `internal/app/skill_setup.go:354-357`（非交互默认）与 `:364-368`（交互默认项排序） |
| install.sh / install.ps1 / install-skills.sh | 脚本自己解析出的 `$VERSION`（`install.sh:177-198`；Gitee 侧 `install.sh:180-188`）——`case "$VERSION" in *-beta.*)` | `install.sh:220-256`、`install.ps1:293-332`、`install-skills.sh:29` |
| npm install.js | 包内 `package.json` 的 `version`（staging 时由 `stage-npm-package.sh` 写入 release 版本） | `build/npm/install.js:197-214` |

**关键发现——upgrade 路径必须同步门控，否则 stable 观察期形同虚设。**
当前实现的 skill 升级语义是「跟着 zip 产物布局走」：`LocateSkillsRoot` 恒优先
`multi/`（`internal/upgrade/paths.go:363-369`），而新 zip 恒含 `multi/`
（`scripts/release/post-goreleaser.sh:220-248`），于是**任何轨的升级都会把存量
mono 用户自动迁移到 multi**（`upgradeMultiSkillLocations` 会先删 `dws/` 残留，
`paths.go:241-244`）。zip 布局按 D1 决策不做轨差异化（产物零改动），所以门控
必须落在 Go 侧：

```
multi preferred ⟺ (beta build) OR (磁盘已是 multi 形态)
```

即 stable build 下磁盘为 mono 的机器升级继续刷 mono（保留 `upgradeMonoSkillLocations`
路径），磁盘已是 multi 的保持 multi 粘性——与「升级永远尊重用户当前 mode」
（plan §3.8 原则）一致。落点：`internal/upgrade/paths.go:129-139`
（`UpgradeSkillLocations` 分支选择）+ `paths.go:363-369`（`LocateSkillsRoot`
优先序），门控函数下沉到 `internal/upgrade` 供 `internal/app` 复用。

切 stable 时删除门控（等价于当前工作区的无条件翻转），是一个独立小 PR。

#### 风险与回退

| 风险 | 说明 | 回退 |
|---|---|---|
| 门控不可见 | 默认值随版本号变化，review/测试容易漏 | 每面补契约测试（beta→multi / stable→mono），`test/scripts` 已有同构先例（`test/scripts/install_script_test.go:529-576`） |
| beta 用户被升级迁移后无法回 mono | beta 期 `dws upgrade` 会把 mono 存量迁到 multi；今天可用手动出口是 `dws skill setup --mode mono --yes`（mono 缓存由各安装面双写，`install.sh:311-318`、`install.js:216-237`；但 upgrade 只刷 multi 缓存，`paths.go:290-296`——首次经 beta 升级迁移的机器 mono 缓存可能缺失，setup 会要求 `--source`） | beta 期在 upgrade 输出加一行「已迁移至 multi，回退：…」；阶段 2 的 `dws skill mode set mono` + 备份式安装落地前不进 stable（roadmap 风险表第 1 行已是同一结论） |
| beta 轨整体有毒 | 二进制或 skill 包级事故 | 现有撤回链：`scripts/release/withdraw-release.sh`（GitHub release 撤回 + npm deprecate + dist-tag 回拨，`withdraw-release.sh:652-665`）；stable 轨不受影响 |
| 版本字符串被仿造 | 本地 `go build` 无版本注入时 `version=""`，门控落 stable 分支（保守方向，正确） | — |

### 2.2 L2 确定性分桶：随 release 发 `rollout.json`

L1 的粒度是「轨」：beta 全吃、stable 全不吃。stable 切流若要 5%→100% 的
渐进，需要机器级分桶。无服务端，用**随版本分发的只读配置 + 客户端确定性
哈希**替代。

#### rollout.json schema 与发布链路

作为 release 资产随每个版本发出（进 `dist/`）：

```json
{
  "skill_mode": {
    "pct": 20,
    "salt": "skill-mode-2026h2",
    "note": "mono->multi default rollout for stable track"
  }
}
```

- `pct`：0–100，`bucket < pct` 的机器默认 multi。
- `salt`：分桶盐，换盐=重新洗牌（默认不换，保证跨版本粘性可比）。
- 发布链路改动：`scripts/release/post-goreleaser.sh` 生成进 `dist/`；
  **资产命名空间是精确集合**（`scripts/release/verify-release-artifacts.sh:12-38`，
  「public release assets must contain exactly the supported files」），必须把
  `rollout.json` 加进 EXPECTED_ASSETS；stable 晋升门会比较 beta 资产集
  （`release.yml:833-846`），所以引入该资产的那个 beta 起两轨必须同时带。
- checksums.txt 由 dist 自动生成，镜像脚本（Gitee `release.yml:1957-1965`、
  OSS `sync-to-oss.sh:9-14`）整目录搬运，**rollout.json 自动随资产集流到
  Gitee/OSS，无需额外接线**。

#### machine-id 来源：读现成，不新建

`internal/auth/identity.go` 已有稳定 per-install UUID v4 `machineId`
（`identity.go:19-20`、结构体 `:70-76`），持久化在 `~/.dws/identity.json`
（`identity.go:51` + `pkg/config/constants.go:177-188`），惰性生成
（`EnsureExists` `:115-133`），v1 文件透明迁移（`:98-113`）。分桶直接复用：

```
bucket = int(sha256(machineId + "|" + salt)[:8], 16) % 100
```

边界情况：`identity.json` 首建于首次 MCP 请求链路（`runner.go:954`）。灰度
决策发生在 setup/upgrade 时，可能早于任何 MCP 请求——此时按 `EnsureExists`
同款语义**就地惰性创建**（best-effort 持久化，失败则用进程内随机值且当次
不记账，下次重决）。不引入第二套 `~/.dws/install-id`，避免双事实源。

脚本面（sh/ps1）做 sha256 分桶要读 JSON + 哈希，复杂且易错；npm install.js
用 node crypto 是一行。**范围划定：L2 分桶只在 Go 面（`dws upgrade` /
`dws skill setup` / 未来 `dws skill mode`）与 npm install.js 实现**；sh/ps1
停在 L1 版本门控（curl 用户全是新装，渠道轨已够；百分比分桶的主战场是存量
升级，而升级必过 Go 二进制）。

#### state.json 的 rollout 字段

依赖阶段 2 的 `~/.dws/skills/state.json`（P0b，未实现，roadmap 阶段 2）：

```json
{
  "mode": "multi",
  "rollout": {
    "bucket": 37,
    "pct": 20,
    "salt": "skill-mode-2026h2",
    "decision": "multi",
    "decided_at": "2026-08-20T08:00:00Z",
    "decided_by": "rollout.json@v1.4.2",
    "explicit": false
  }
}
```

决策顺序（每台机器只决策一次，粘性）：

1. 本地显式（`DWS_SKILL_MODE` / `--mode` / `--skill-mode` / `dws skill mode set`）
   → 用之，`explicit=true`；
2. `state.json` 已有 `mode` 或磁盘形态可反推（roadmap 阶段 2 既定兜底）
   → 保持现状，不参与分桶；
3. `state.rollout.decision` 已存在 → 复用（pct 后续变化不翻案）；
4. 拉取 `rollout.json`（Go 面：upgrade 已下载本版资产，同 release 再取一个
   小文件；npm：包内自带）→ 算 bucket 决策并记账；
5. 任一步失败 → 当期默认（L1 版本门控结果）。

#### 三条硬规则

| 规则 | 内容 | 理由 |
|---|---|---|
| R1 本地显式优先 | env/flag/命令任何时候压过 rollout 决策；显式选择落 `explicit=true` 后 rollout 永不改它 | 灰度不能覆盖用户意志；也是 kill switch 的用户侧出口 |
| R2 拉取失败 fail-safe | 拉不到/解析失败/字段越界 → 保持现状（存量）或当期默认（新装），**绝不因拉取失败翻模式** | 无服务端下网络面即故障面，故障必须倒向保守侧 |
| R3 存量不被改模式 | 已有 mode（state 或磁盘可推）的机器不参与分桶；pct 只影响「未决策」机器 | pct 从 20 降到 0 不能把已进 multi 的 20% 弹回 mono（那需要 kill switch，不是分桶语义） |

#### 与 Gitee / immutable release 的兼容

- **Gitee**：`dws upgrade` 不读 Gitee（§1.1-K3），rollout.json 经
  reconcile/sync 脚本随资产集镜像到 Gitee release；Gitee 侧安装脚本如需消费，
  走与 `dws-skills.zip` 相同的 Gitee API 资产枚举（`install.sh:180-188` 同
  模式）。一期不消费、只保证镜像不缺失。
- **immutable release 约束**：官方仓已开启不可变 release（`release.yml:802`
  「Immutable releases must be enabled before publishing」），**资产发布后不可
  替换**——调 pct = 发一个新补丁版（beta 线 release.yml 支持连续 beta：
  `release_bump` 在连续 beta 线时被忽略，`release.yml:25-27`）。这决定了
  L2 的调参时延 = 一次发版；追求更快止血见 §2.4。
- （可选远期）OSS `latest.txt`/`beta.txt` 是现成的**可变**指针
  （`sync-to-oss.sh:13-14`），若未来 installer 学会读 OSS，可把
  `rollout-current.json` 放 OSS 变指针后面实现「不发版调 pct」；今天无消费
  方，不建。

### 2.3 L3 可观测性：`x-dws-skill-mode` 请求头

#### 设计

| 项 | 设计 |
|---|---|
| 头名 / 取值 | `x-dws-skill-mode: mono \| multi \| unknown` |
| 注入点 | 唯一收口 `resolveIdentityHeaders`（`internal/app/runner.go:953-1008`），照 `x-dws-channel` 先例（`runner.go:1006-1008`）；登录权限检查侧补同名字段（`internal/auth/oauth_helpers.go:1424-1428` 旁），让未完整登录的 auth 探测也带信号 |
| 取值来源（优先级） | ① `state.json` 的 `mode`（阶段 2 落地后）→ ② 磁盘形态反推：任一已知 agent home 下存在 `dingtalk-*/` 或 `dws-shared/` → `multi`；存在 `dws/` → `mono`；两者皆有（drift）或皆无 → `unknown`；反推逻辑与 state.json 兜底共用同一函数，不写第二份 |
| 进程内开销 | 每进程解析一次缓存（`sync.Once`），读 1–2 个目录，可忽略 |
| unknown 语义 | 诚实上报，不用磁盘漂移硬猜；占比看板把 unknown 单列，mono 占比 = mono/(mono+multi) 与 mono/(all) 双口径 |

服务端聚合（网关按 header 计数出看板）**out of scope**——本文只定义客户端
信号；网关侧已有 `x-dws-agent-id` 等头的聚合先例（CHANGELOG #467），加一个
维度是既有看板的延伸。已知偏差：分母是「发 MCP 请求的活跃用户」，装而不用
者缺席 → multi 占比会偏高估，判读下线判据（roadmap 阶段 4 判据 1「≥90%」）
时按双口径保守取值。

### 2.4 Kill switch：四层止血

| 层 | 手段 | 时延 | 现状/依赖 |
|---|---|---|---|
| S1 beta 轨整体撤回 | `scripts/release/withdraw-release.sh`：GitHub release 撤回 + npm deprecate + `beta` dist-tag 回拨（`:652-665`） | 分钟级 | **今天可用** |
| S2 rollout.json `pct=0` | 发补丁版把 pct 打 0（immutable release 不允许原地改资产，§2.2） | 一次发版（小时级） | 依赖 L2 落地 |
| S3 公告命令 | 公告用户执行 `dws skill mode set mono`（R1 显式优先保证其生效） | 用户触达时延 | **依赖阶段 2 P1-1（未实现）**；今天的等价物是 `dws skill setup --mode mono --yes`，但无记账无备份 |
| S4 备份回滚 | 安装/清理从 `RemoveAll` 直删改为 `mv` 进 `~/.dws/skills/backup/<ts>-<mode>/`，失败整体回滚；`dws skill mode rollback` 一键回 | 用户单机秒级 | **依赖阶段 2 P0-3（未实现）**；现状清理失败仅 warning 继续装（`internal/app/skill_setup.go:598-630`、`internal/upgrade/paths.go:246-259`），半装无兜底 |

二进制侧另有既有的 `dws upgrade --rollback`（`internal/upgrade/rollback.go`，
备份在 `~/.dws/data/backups`、保留 5 份 `:16/:54-63`），但只回滚二进制不回滚
skill 布局（plan §3.8 明示）——skill 止血必须靠 S3/S4，不能指望它。

**结论：备份式安装（S4 前置）落地前，kill switch 只有 S1 完整、S3 半残，
不宜进 stable 全切**——与 roadmap 风险表第 1 行、决策 D2 的结论一致。

---

## 3. 对标（简要）

**npm dist-tag 双轨（lark-cli 类企业内部 CLI 的通行形态）。** 以 npm 仓
registry 为唯一分发面时，灰度即 dist-tag：`latest` 稳态、`beta`/`next` 先行
（`next@canary`、`typescript@beta` 同款模式），安装侧 `npm i pkg@beta` 或
CI 指定 tag 即完成分群；撤回即 `npm dist-tag add pkg@<prev> latest` +
`npm deprecate`。DWS 已完整具备此形态（§1.1-K2），lark-cli 等内部 CLI 在
集团内网 registry 上亦按同一范式运作——差别只在内部 registry 可附带按
员工/部门灰度的下发规则，那是「registry 有服务端」的红利，DWS 面向公网
npm 没有这一层，故需 L2 补齐。

**安装时下载器内版本选择（deno / rustup 模式）。** `curl | sh` 安装器不显式
给版本时，先拉一个**可变 channel 指针文件**（如 deno 的
`dl.deno.land/release-latest.txt`、rustup 的 channel manifest），再按指针下载
真实产物——指针一改全量新装即转向，**不发版即可调流**。DWS 的 OSS
`latest.txt`/`beta.txt`（`sync-to-oss.sh:13-14`）已是同构物，只差安装脚本消费
它；install.sh 今天直接打 GitHub `/releases/latest` 重定向（`install.sh:190-198`），
等价于把 GitHub 当不可调指针用。此模式是指针级灰度，做不到机器级百分比，
需与 L2 分桶叠加。

**双产物并行（VS Code Stable / Insiders 模式）。** 两个渠道各发各的包、用户
自选安装，灰度靠「渠道人口结构」自然形成，无需任何运行时门控。DWS 的
GitHub prerelease + npm `@beta` 已是它的轻量版（同包不同 tag 而非两个包名），
L1 切法 B 的版本门控默认正是把「渠道差异」从纯流程下沉为可测试的代码事实。

---

## 4. 推荐路线与改动点清单

### 4.1 路线（与 roadmap 阶段 2/3 对齐后的排序）

```
L1 版本门控拆分（本工作区之上叠加，先合入）
  → 阶段 2 四件套（备份式安装 / state.json / dws skill mode / x-dws-skill-mode）
  → beta 轨发版先吃（L1 自动生效）+ L3 观察 ≥2 周
  → stable 切流：小步直接 100%（删门控）；若要求渐进再上 L2 分桶
  → 阶段 4 下线判据（roadmap 既定，含 x-dws-skill-mode ≥90%）
```

### 4.2 改动点清单（文件级）

| 项 | 文件 | 改动 | 估时 |
|---|---|---|---|
| L1-a Go 门控函数 | `internal/app/skill_setup.go`（或新 `internal/upgrade/track.go` 下沉共享） | `defaultSkillModeForVersion(version)`：含 `-beta.`→multi 否则 mono；替换 `:354-357` 非交互默认与 `:364-368` 交互排序 | 0.5d |
| L1-b upgrade 同步门控 | `internal/upgrade/paths.go:129-139`（分支）、`:363-369`（LocateSkillsRoot 优先序）、`internal/app/upgrade.go:60/579` seam 周边 | stable build 且磁盘 mono → 走 mono 刷新；磁盘已 multi 保持 multi | 0.5d |
| L1-c 脚本面门控 | `scripts/install.sh:220-256`、`scripts/install.ps1:293-332`、`scripts/install-skills.sh:29`、`build/npm/install.js:197-214` | 默认值解析加 `*-beta.*` 分支 | 0.5d |
| L1-d 契约测试 | `test/scripts/install_script_test.go`、`internal/upgrade/paths_multi_test.go`、`internal/app/skill_setup*_test.go` | 每面断言 beta→multi / stable→mono | 0.5–1d |
| L3 请求头 | `internal/app/runner.go:953-1008`、`internal/auth/oauth_helpers.go:1424-1428` 旁、磁盘反推函数（与 state.json 兜底共享，建议落 `internal/upgrade` 或新 `internal/skillmode`） | 加 `x-dws-skill-mode` 三值 | 0.5d（roadmap P0-4 同估） |
| L2-a rollout.json 进资产 | `scripts/release/post-goreleaser.sh`（生成）、`scripts/release/verify-release-artifacts.sh:12-29`（EXPECTED_ASSETS）、`internal/generator` 无关 | 资产+校验 | 0.5d |
| L2-b 分桶引擎（Go） | 新 `internal/skillmode/rollout.go`（复用 `internal/auth/identity.go:115-141` 的 machineId 获取）、`internal/app/skill_setup.go` / `internal/upgrade/paths.go` 决策点接入 | 拉取→哈希→决策→记 `state.rollout`；三硬规则 | 1–1.5d（依赖 P0b state.json） |
| L2-c npm 侧分桶 | `build/npm/install.js:197-214` 周边 | node crypto 一行哈希 + 包内 rollout.json 读取 | 0.5d |
| S3/S4 依赖（非本文新增，roadmap 阶段 2 已有估时） | 备份式安装 1.5–2d（P0-3）；`dws skill mode` 2–2.5d（P1-1）；state.json 1d + 0.5–1d（P0b-1/2） | — | roadmap 既定 |

合计：L1 ≈ 2–2.5d；L3 ≈ 0.5d；L2 ≈ 2.5–3d（在 state.json 之后）。
L1+L3 是进入 beta 观察期的最小集；L2 只在 stable 需要渐进切流时才启动，
否则删门控一步到位即可。

### 4.3 明确不做（沿用 roadmap/D2，本文补充）

- 不建服务端远程配置/灰度平台；不为灰度单独引入可变配置下发通道（OSS 变
  指针仅作远期可选，今天无消费方）。
- 不动 `dws-skills.zip` 产物布局（D1）；不按轨发不同 zip——轨差异全部落在
  版本门控的客户端行为上。
- 不用遥测做灰度测量（K7 本地 opt-in 无统计意义），观测只走 K8 请求头。
