# Skill 多 skill（multi）切换方案

> 目标：一个月内把 DWS 的 Agent skill 默认安装形态从 mono（单 skill）切换为
> multi（按产品拆分），两套并行期后下掉 mono。
> 本文基于对分发/消费链路的代码级梳理（2026-08-04），所有锚点均可跳转验证。
> 机制调研与 lark-cli 对标细节：[skill-distribution-mechanism.md](skill-distribution-mechanism.md)

## 1. 目标与约束

| 项 | 内容 |
|---|---|
| 终态 | 新装/升级默认铺 multi（`<agent-home>/dingtalk-*/`、`dws-shared/`）；mono 仅 opt-in；最终物理删除 mono |
| 并行期 | 约一个月，mono 保留可切换、可回退 |
| 硬约束 | DWS 无服务端灰度/远程配置；skill 是本地文件分发；安装面至少 7 个且已有漂移 |
| 前置原则 | 先修已存在的 upgrade×multi 双份 bug，再谈切默认 |

## 2. 现状关键事实（梳理结论）

分发侧：

- `dws-skills.zip` 布局：zip 根 = mono 副本（向后兼容）+ `mono/` + `multi/`
  （`scripts/release/post-goreleaser.sh:220-248`）。**产物无需改动**。
- 二进制 `//go:embed all:skills/mono all:skills/multi`（`skills_embed.go:28`），
  `dws skill setup` 默认源就是 embed，**与分发渠道无关**。
- 7 个安装/分发面：install.sh、install.ps1、install-skills.sh、npm install.js、
  Homebrew formula、install-event.sh、install-devapp.sh。
- OSS 只发不读；大陆链路靠 Gitee fallback。

消费侧：

- `dws skill setup`：源优先级 `--source`/env → embed；目标 16 个 agent home
  （父目录门控）；互斥清理 best-effort、失败仅 warning、无备份无回滚
  （`internal/app/skill_setup.go`）。
- `dws upgrade`：`LocateSkillMD` 命中 zip 根 mono → `UpgradeSkillLocations`
  只写 `<agent>/dws/`、不清理 `dingtalk-*`、不更新 `~/.dws/skills`
  （`internal/upgrade/paths.go:119-172`）。**multi 用户升级后 mono+multi 共存**。
- `dws upgrade --rollback` 只回滚二进制，不回滚 skill。
- **无任何已安装模式状态**：`~/.dws/` 无 install manifest。
- `~/.dws/skills` 缓存事实上只写不读（默认 setup 走 embed），且 upgrade 不更新，
  版本漂移不可见。
- `skill setup --target all` 不含 opencode，但 `agentSkillPaths` 含
  （`skill_command.go:119-141` vs `skill_setup.go:20-37`）。
- 市场 skill（`dws skill install`）解压到 agent skills 根，与内置 skill 无 mode
  概念；互斥清理按 `dingtalk-*` 前缀扫描，依赖"市场无同名前缀"这一隐性约定。

## 3. 总体设计

### 3.1 单一事实源收敛（P0c）

agent home 清单目前在 5+ 处各写一份（注释约定 keep in sync，无门禁）。

- Go 侧：`skillSetupAgentHomes` 与 `knownSkillDirs` 合并为一个导出列表
  （放在 `internal/upgrade` 或新 `internal/skillhome` 包），补 opencode，
  setup/upgrade/测试共用。
- 脚本侧：install.sh / install.ps1 / install.js / install-skills.sh /
  install-event.sh / install-devapp.sh 的清单由同一个 JSON 生成或政策脚本
  比对（新增 `scripts/policy/check-agent-homes-sync.sh`，进 `make policy`）。

### 3.2 安装状态文件（P0b）

新增 `~/.dws/skills/state.json`：

```json
{
  "schema_version": 1,
  "mode": "multi",
  "cli_version": "1.0.54",
  "installed_at": "2026-08-04T17:00:00+08:00",
  "source": "embedded",
  "agent_homes": ["/Users/x/.agents/skills", "/Users/x/.cursor/skills"],
  "installed": ["dws-shared", "dingtalk-chat", "dingtalk-doc"],
  "previous": {
    "mode": "mono",
    "cli_version": "1.0.53",
    "backup": "/Users/x/.dws/skills/backup/20260804-1700-mono"
  }
}
```

- 写入方：`dws skill setup`、`dws upgrade`、各安装脚本（shell/ps1/js 各写同构 JSON）。
- 读取方：`dws upgrade`（决定刷新形态）、`dws skill mode`（status/set/rollback）。
- 缺失/损坏时 fail-safe：从磁盘形态反推（有 `dws/` → mono；有 `dingtalk-*` → multi；
  两者都有 → 报 drift，提示显式 `dws skill mode set <mode>` 收敛）。

### 3.3 备份式安装与真回滚（P0a 一部分）

把安装时的 `RemoveAll` 改为 `mv` 到 `~/.dws/skills/backup/<ts>-<mode>/`：

- 全部 agent home 拷贝成功 → 保留最近 2 份备份，清理更老的。
- 任一 agent home 失败 → 自动从备份恢复，非 0 退出，输出"已回滚到 <mode>"。
- 互斥清理同样走备份目录而不是直接删。
- 改变现状"清理失败仅 warning 继续装"的语义：清理/拷贝失败即整体回滚，
  不留半装状态。

### 3.4 upgrade mode-aware（P0a）

- `UpgradeSkillLocations(extractedDir, mode)` 按 state.json 的 mode 走：
  - mono → 现状逻辑（写 `<agent>/dws/`）+ 清 `dingtalk-*` 残留；
  - multi → 从 zip `multi/` 平铺刷新 `<agent>/dingtalk-*` + `dws-shared`，
    清 `dws/` 残留，刷新 `~/.dws/skills/multi` 缓存。
- 复用 setup 的 `agentHomeForMode` / 互斥清理逻辑：下沉到共享包，禁止复制。
- upgrade 成功后更新 state.json（mode 不变，`cli_version`、installed 列表刷新）。
- 无 state 的存量机器：按 3.2 的磁盘反推；推不出（全新机）→ 保持 mono 行为
  （并行期），切默认后 → multi。

### 3.5 模式切换命令（P1）

```bash
dws skill mode                        # status：mode/版本/已装列表/上次切换/备份
dws skill mode set multi              # 切换：备份→互斥清理→安装→记账，失败自动回滚
dws skill mode set mono --yes
dws skill mode rollback               # 回到 state.previous（一条命令回退）
dws skill mode set multi --dry-run    # 预览写删路径
```

- `set` 复用 `skill setup` 的安装实现，外加备份/记账；`skill setup` 保留兼容
  （脚本/CI 在用），内部走同一条 install 函数。
- 切换成功后提示重启 AI 工具（skill 目录由 Agent 进程加载）。

### 3.6 默认切换（P2）

改默认值="multi 默认、mono opt-in"，七个面一起改 + 门禁锁一致性：

| 面 | 改动 |
|---|---|
| `dws skill setup` | 无 `--mode` 时默认 multi（交互选项顺序反转，mono 标 legacy） |
| install.sh / install.ps1 | multi 分支**真正安装**（现在是打印提示跳过）；`DWS_SKILL_MODE=mono` opt-in；TTY 默认项改 multi |
| install-skills.sh / npm install.js | 从零加 mode 支持（env `DWS_SKILL_MODE` / `--skill-mode`），默认 multi |
| Homebrew formula | caveats 改提示 `dws skill setup`（默认即 multi），无需改资源 |
| install-event.sh / install-devapp.sh | 维持单 skill 语义，但改走共享 install 函数 |

文档同步：README/README_zh（`README_zh.md:81` "默认 1"）、
`skills/multi/dingtalk-skill/SKILL.md` 的 🧪 EXPERIMENTAL 措辞下调、
install.sh 内 multi 警告文案、AGENTS.md"生产优先 mono"表述。

### 3.7 灰度与止血（无服务端能力下的替代）

- **L1 渠道灰度（先行）**：beta 轨（GitHub prerelease / npm `beta` dist-tag /
  `dws upgrade --beta`）先切默认 multi，stable 保持 mono。零新增代码。
- **L2 确定性分桶（可选增强）**：随 release 发 `rollout.json`
  （GitHub asset + OSS 同步），安装/升级时 `hash(machine-id) % 100 < pct` 决策
  并粘入 `state.rollout`。规则：本地显式 env/flag 永远优先；拉取失败 fail-safe
  mono（并行期）/ 保持现状（切默认后）；存量机器不被 rollout 改模式。
- **Kill switch**：`rollout.json` pct=0（若上 L2）+ 公告
  `dws skill mode rollback` / `dws skill mode set mono`。依赖 3.3 的备份回滚已落地。
- **可观测**：MCP/登录请求头加 `x-dws-skill-mode: mono|multi`（仿
  `internal/auth/oauth_helpers.go` 的 `x-dws-channel`），否则灰度占比与
  下线判据都无从测量。

### 3.8 安装与升级的语义矩阵（两条路径都要支持 multi）

| 场景 | 时期 | 行为 |
|---|---|---|
| 新装 | 并行期（切默认前） | mono 默认，`DWS_SKILL_MODE=multi` / 交互可选 multi |
| 新装 | 切默认后 | **multi 默认**，`DWS_SKILL_MODE=mono` opt-in；写 state.json |
| 升级（存量 mono） | 并行期 | **保持 mono 不自动迁移**，刷新 mono 包；输出一行提示"可 `dws skill mode set multi` 切换" |
| 升级（存量 multi） | 任意 | **刷新 multi**（P0a）：按 state.json `installed` 增量刷新，清 `dws/` 残留，刷缓存；绝不再写 `dws/` |
| 升级（无 state） | 任意 | 磁盘形态反推；推不出（全新机）跟随当期默认 |
| 升级（存量 mono） | mono 下线版本 | **一次性自动迁移 mono→multi**（备份旧 mono 可 `dws skill mode rollback`），此后 install/upgrade 的 mono 分支报错并指向迁移命令 |
| `dws upgrade --rollback` | 任意 | 只回滚二进制（现状）；skill 回滚统一走 `dws skill mode rollback` |

原则：**升级永远尊重用户当前 mode**（粘性），模式的默认翻转只影响新装与
显式切换；唯一的自动迁移发生在 mono 下线版本，且必须带备份可回滚。

## 4. 阶段与时间线（4 周）

| 周 | 内容 | 出口标准 |
|---|---|---|
| W1 | P0a upgrade mode-aware + P0b state.json + P0c 清单收敛与门禁 + 备份式安装 | upgrade×multi 集成测试（装 multi → upgrade → 无 `dws/` 残留）；备份回滚测试（模拟中途失败）；`make policy` 含 homes 同步检查 |
| W2 | P1 `dws skill mode`（status/set/rollback/dry-run）；beta 轨默认切 multi（L1）；`x-dws-skill-mode` 头 | 双向切换 + 中断恢复手工验收；beta 轨冒烟 |
| W3 | P2 stable 默认切 multi、mono 降为 opt-in；文案翻转；（可选 L2 分桶 5%→20%） | 七面默认行为一致（政策脚本）；issue/请求头占比观察 |
| W4 | （L2 则 50%→100%）；mono 打 deprecation 警告，**不删代码** | 连续 7 天无 multi 相关 P1 |

## 5. mono 下线判据（不满足则不删）

> 判据更新（2026-08-05）：原判据 3「悟空 bundled skill 分发线（dws_res →
> bundled-skills，本仓库外）已切 multi」作废——悟空分发线已于当日决策
> 下线（见 [skill-multi-roadmap.md](skill-multi-roadmap.md)
> 「悟空线下线的影响（2026-08-05）」），无仓外 mono 依赖，mono 下线不再
> 有仓外节奏闸门。判据重新编号如下（原 4/5 顺延为 3/4）。

1. `x-dws-skill-mode=multi` 请求占比 ≥ 90%；
2.  mono 主动 opt-in 率 ≤ 2%（install 脚本/命令埋点）；
3. 连续两周无 multi P1；
4. 已有等价政策门替代 `scripts/policy/check-skill-context-budget.sh` 对
   `skills/mono/SKILL.md` 的依赖；`skills_embed.go` 去掉 `all:skills/mono`；
   存量 `<agent>/dws` 目录有"遇到即迁移清理"逻辑。

满足后单独一个版本窗口物理删除 `skills/mono/`，install/setup/upgrade 中 mono
分支改为报错并指向 `dws skill mode set multi`。

## 6. 风险与对策

| 风险 | 对策 |
|---|---|
| 半装状态（清理成功、拷贝失败） | 3.3 备份式安装，失败整体回滚 |
| multi 用户被 upgrade 塞回 mono | 3.4 mode-aware，P0a 先修 |
| Agent 目录清单继续漂移 | 3.1 单一事实源 + policy 门禁 |
| 互斥清理误伤市场 `dingtalk-*` skill | 清理前校验目录内 SKILL.md frontmatter 属 DWS 产品集，写成测试 |
| 无灰度全切翻车 | beta 轨先行 + 备份回滚 + kill switch 公告命令 |
| 用户不重启 AI 工具读到旧 skill | mode set / setup / install 输出统一提示重启 |
| `~/.dws/skills` 缓存与 embed 版本漂移 | upgrade 时同步刷新对应 mode 缓存；长期可评估废弃缓存 |

## 7. 分发机制对标 lark-cli（2026-08-04 调研）

### 7.1 lark-cli 的实际分发结构

npm 包 `@larksuite/cli` 是一个**薄壳**（package.json `files` 只有
`install.js` / `install-wizard.js` / `run.js` / `checksums.txt`）：

- **二进制**：postinstall 时按平台从 GitHub Releases 下载，SHA256 校验
  （checksums.txt 随 npm 包发布）；镜像链 = GitHub → 用户 registry 派生镜像 →
  npmmirror 兜底，host allowlist + checksum 双保险。`run.js` 在二进制缺失时
  自动补下载；Windows 有 `.old` 崩溃恢复。
- **Skills**：**不进 npm 包、不进二进制**。repo 根 `skills/` 目录即事实源，
  由生态通用安装器 `npx skills add larksuite/cli -y -g`（vercel-labs/skills）
  安装；wizard 首选 `https://open.feishu.cn` 直链、GitHub shorthand 兜底。
- **一键向导** `npx @larksuite/cli@latest install`：run.js 拦截 `install`
  子命令 → install-wizard.js 串联 4 步（npm 全局装/升级 → skills 安装 →
  config init → auth login），每步幂等可跳过（`skills ls -g` 检测 `lark-*`
  已装则跳过）；非 TTY 自动降级为"装完打印后续命令"。

生态安装器 `skills` CLI 的能力（lark-cli 免费获得的）：

- **76 个 agent 的目录清单由生态维护**，自动探测已装 agent；project/global
  两种 scope；
- **symlink 到 canonical 副本（推荐）或 copy** —— symlink 模式下升级
  = 更新 canonical 一份，所有 agent 即时生效；
- `list / find / update / remove` 全套生命周期命令，skill 升级由
  `npx skills update` 承担，**lark-cli 自己不写 skill 刷新逻辑**；
- 发现约定兼容 catalog 布局 `skills/<catalog>/<name>/SKILL.md`。

### 7.2 与 DWS 的关键差异

| 维度 | lark-cli | DWS 现状 |
|---|---|---|
| 分发单元 | repo `skills/` 目录（源码即事实源） | zip + 二进制 embed + 5 处拷贝缓存 |
| 安装器数量 | **1 个**（生态工具，76 agent 清单别人维护） | **7 个**自维护脚本，清单已漂移 |
| 落盘方式 | symlink（canonical 单点更新） | 全量 copy，升级要重写 16 个 home |
| skill 升级 | `npx skills update`（生态承担） | `dws upgrade` 自写，且不识 multi |
| mono/multi 问题 | **不存在** —— 天生按目录多 skill | 互斥清理/模式切换/状态全是自建 |
| npm 包体积 | 薄壳（运行时下载单平台二进制） | 全平台 archive + skills.zip 全打进 tarball |
| 大陆镜像 | registry 派生 + npmmirror | Gitee fallback + OSS（只发不读） |

### 7.3 结论：生态分发通道**已否决**（2026-08-04）

~~借生态安装器做分发通道~~ 方向经评估后**放弃**。否决原因（均为实测）：

1. **无版本固定**：`skills add`（v1.5.21）不支持 tag/ref 安装，而 DWS skill
   从 Cobra 树生成、与 CLI 版本强耦合，错配不可接受；唯一可复现机制
   `skills-lock.json` 仍是 experimental。
2. **依赖 Node + GitHub 可达**：curl|sh / Homebrew / 大陆 Gitee fallback 的
   用户环境大量无 Node，生态通道在这些场景是断的。
3. **mono 会被一起发现**：发布即制造双份，须等 mono 下线才能发布，节奏不合。
4. **悟空 bundled / 离线预装**生态通道永远覆盖不了。（2026-08-05 注：
   悟空分发线已下线，此条约束随之消失；前 3 条否决理由仍成立。）

**决策：分发维持全自维护通道**（zip + embed + 安装脚本），按 P0–P2 落地；
不从 lark-cli 借鉴分发架构。可保留的借鉴点只剩两个本地语义，与生态无关：

1. **wizard 式幂等安装**：`install.sh` 的 multi 分支从"打印提示跳过"改为
   检测 state 可重入的真正安装（参考 install-wizard 的步骤化幂等）。
2. **npm 下载校验**：install.js 如后续薄壳化，可参考其 host allowlist +
   checksum 双保险与镜像链（GitHub → registry 派生 → npmmirror）设计。
   与本次 multi 迁移解耦，不单列任务。

## 8. 工作量评估与任务拆解

> 以 1 名熟悉本仓库的工程师计（人日）。规模基线：涉及生产代码约 5.0k 行
> （Go 3.0k + shell/ps1/js 2.0k），已有测试 1.2k 行可复用。

### 8.1 总体判断

**核心路径 13–17 人日，一人一个月可行但偏紧**。风险不在 Go 而在"shell /
ps1 / js 三语言 × 七面同步"和对应的测试矩阵。可选项（rollout 分桶、npm
薄壳化、symlink、生态通道发布）全部可砍可后置，砍后**最小可行集 8–10
人日**（见 8.4）。

### 8.2 任务拆解（带依赖）

```
P0c 清单收敛 ──┐
P0b state.json ─┼─► P0a upgrade mode-aware ─► P1 skill mode ─► P2 切默认
备份式安装 ────┘         │                        │
                         └─► x-dws-skill-mode 头   └─► beta 轨先切（P2 预演）
```

| # | 任务 | 改动面 | 估时 | 依赖 |
|---|---|---|---|---|
| P0c-1 | agent home 清单下沉共享包（setup/upgrade 共用），补 opencode | `internal/upgrade/paths.go` 或新 `internal/skillhome`（~80 行新代码） | 0.5d | — |
| P0c-2 | `scripts/policy/check-agent-homes-sync.sh`：比对 sh/ps1/js/sh 专项清单与 Go 清单，进 `make policy` | 新脚本 ~120 行 + 各脚本清单改成可解析块 | 1d | P0c-1 |
| P0b-1 | state.json schema + setup 写入（含 installed 列表、agent_homes、previous） | `internal/app/skill_setup.go` +新文件 ~200 行 | 1d | — |
| P0b-2 | 磁盘形态反推（dws/ → mono；dingtalk-* → multi；都有 → drift 报错）+ 单测 | ~120 行 | 0.5–1d | P0b-1 schema |
| P0-3 | 备份式安装：RemoveAll→mv backup、失败自动回滚、保留最近 2 份、改 warning 语义 | setup install 两函数重写 ~150 行 + 测试 | 1.5–2d | — |
| P0a-1 | `UpgradeSkillLocations(dir, mode)`：mono 现状+清残留；multi 从 zip `multi/` 平铺刷新+清 `dws/`+刷缓存 | `internal/upgrade/paths.go` ~150 行 | 1.5d | P0c-1、P0b |
| P0a-2 | upgrade×multi 集成测试（装 multi→upgrade→无 dws/ 残留、dingtalk-* 已刷新） | 测试 ~200 行 | 1d | P0a-1 |
| P0-4 | `x-dws-skill-mode` 请求头（仿 `x-dws-channel`） | `internal/auth/oauth_helpers.go` 附近 ~30 行 | 0.5d | P0b |
| P1-1 | `dws skill mode` status/set/rollback/--dry-run | 新文件 ~350 行 + 测试 | 2–2.5d | P0b、P0-3 |
| P1-2 | beta 轨默认切 multi（版本门控的默认值翻转，stable 不变） | setup/install.sh 默认值逻辑 ~40 行 | 0.5–1d | P1-1 |
| P2-1 | setup 默认翻转 + 交互选项反转 + mono 标 legacy | ~30 行 + 测试更新 | 0.5d | P1 |
| P2-2 | install.sh / install.ps1 的 multi 分支**真装**（幂等、读 state 跳过） | 两个脚本各 ~80 行 | 1.5d | P0b（脚本侧写 state） |
| P2-3 | install.js / install-skills.sh 加 mode 支持（env + flag，默认 multi） | 各 ~60 行 | 1d | P2-2 同批 |
| P2-4 | 文案翻转：README×2、SKILL.md EXPERIMENTAL 下调、install 脚本提示、AGENTS.md | 纯文档 | 0.5d | — |
| W4 | mono deprecation 警告（不删代码）+ 观察 | ~20 行 | 0.5d | P2 |

**小计：核心 13–17 人日**（P0 ≈ 6.5–8.5，P1 ≈ 3–3.5，P2 ≈ 3.5，W4 0.5）。

可选/后置：rollout.json 分桶 2–3d；`setup --link` symlink 1–2d（需逐 agent
验证）；npm 薄壳化 2–3d（独立立项）。

### 8.3 风险最高的两处（先动）

1. **P0a upgrade multi 刷新语义**：additive 安装 vs upgrade 全量刷新之间存在
   一个真实设计题 —— 用户手动 `-x` 排除过的 skill，upgrade 要不要装回来？
   答案：以 state.json 的 `installed` 为准做增量刷新，未装的不得补装
   （否则违背 additive 语义）。这条必须在 P0a 开工前定死。
2. **P0-3 备份回滚改语义**：现有测试断言"清理失败继续装"，改语义会动
   `skill_setup_full_coverage_test.go` 多处；回滚恢复顺序（先恢复再报错）
   要用故障注入测试覆盖。

### 8.4 一个月做不完时的砍法

按价值/成本比从后往前砍：rollout 分桶（L1 beta 轨已够）→ install.js /
install-skills.sh mode 支持（npm 渠道用户量小，可先只改 sh/ps1）。砍后**最小可行集 8–10 人日**：

> P0c-1 + P0b + P0-3 + P0a + P1-1 + P2-1 + P2-2（仅 sh/ps1）+ 文案

即：upgrade 不再制造双份、有状态可回滚、setup 与主流安装脚本默认 multi。
npm/install-skills.sh 维持 mono 显式行为并在输出中标注即将切换。

## 9. 明确不做

- 不做服务端远程配置/灰度平台（用 beta 轨 + rollout.json 替代）。
- 不动 `dws-skills.zip` 产物布局（已含 mono/multi 双树）。
- 不动市场 skill（`dws skill install`）的安装语义。
- 并行期内不删 mono 代码与产物，只降级为 opt-in。
