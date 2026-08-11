# Skill multi 迁移：技术方案（as-implemented）与 Roadmap

> 本文是 multi 迁移的当前事实源，记录 PR #922 在 2026-08-11 的最终实现。
> 原始设计过程见 [skill-multi-migration-plan.md](skill-multi-migration-plan.md)，
> 分发机制调研见
> [skill-distribution-mechanism.md](skill-distribution-mechanism.md)。两份文档是历史
> 输入；发生冲突时以本文和代码为准。

## 状态速览

| 项 | 内容 |
|---|---|
| 当前阶段 | 默认 multi、安全迁移、官方 Skill 全量覆盖升级已落地；独立 `dws skill mode` lifecycle 命令不建设 |
| 硬 deadline | 2026-08-30：安装/升级默认 multi；mono 仅 opt-in；物理下线在独立 retirement 版本推进 |
| 已完成 | 五面默认 multi、互斥清理、删除前备份、setup 确认/`--dry-run`、官方清单全量覆盖升级 |
| 下一步 | beta/L1 观察、可选 agent-home 清单门禁、mono retirement |
| 终态 | multi 为唯一默认布局；每次升级全量覆盖预制 Skill，不持久化本地删除选择 |

## 1. 最终语义

### 1.1 安装和模式切换

- 新装默认 multi；`DWS_SKILL_MODE=mono`、`--skill-mode mono` 或
  `dws skill setup --mode mono` 为 legacy opt-in。
- 不新增 `dws skill mode status|set|rollback` 产品面。需要切换时重新执行
  `dws skill setup --mode <mono|multi>`。
- setup 的 `--dry-run` 和交互确认消费同一份精确计划。非交互环境必须显式
  `--yes`，公开可复制示例不携带确认绕过参数。
- 对面布局、过期受管 Skill 和同名官方目标在替换前都移入
  `~/.dws/skill-backups/<时间戳>/`；备份失败时保留原目录并跳过整个 Agent
  目标，避免 mono/multi 混装。
- 每个预制 multi Skill 安装成功后写入 `.dws-managed` 所有权标记。互斥与
  stale 清理只接受该标记（以及精确 legacy 名 `dws-shared`），绝不以
  `dingtalk-*` 前缀推断所有权；同前缀市场/用户 Skill 保留。

### 1.2 升级布局与 Skill 集合

- `LocateSkillsRoot` 优先返回 release zip 内的 `multi/`。包中存在 multi 树时，
  存量 mono 布局会迁移为 multi；仅 legacy mono-only 包走 mono 回退路径。
- 布局和 Skill 集合都由当前升级包驱动，不写 mode sticky 状态，也不根据本地
  目录缺失推断排除意图。每次 upgrade 都安装并覆盖本版官方全量集合。
- 因此，本地删除或通过 setup 临时排除的预制 Skill 会在下一次 upgrade 恢复，
  新版新增官方 Skill 也会自动加入；`--force` 主要用于重装当前 CLI 版本。
- `dingtalk-shared` 随官方全量集合始终安装。

### 1.3 状态与缓存

- multi setup/upgrade 成功后写入信息快照 `~/.dws/skills-state.json`；设置
  `DWS_CONFIG_DIR` 时写到该目录下的 `skills-state.json`。
- 状态记录当前版本官方清单、本次更新集合和更新时间，不参与安装集合求解，
  也不决定 mono/multi 布局。
- `~/.dws/skills/{mono,multi}` 是 setup 的本地回退源。所有渠道先在缓存同级
  staging 目录完成复制，再通过 rename 发布；复制或发布失败时保留/恢复旧缓存。

## 2. 分发入口

以下入口默认 multi，并保持相同的布局互斥与失败保护：

| 入口 | 默认/opt-in | 缓存与备份 |
|---|---|---|
| `dws skill setup` | 默认 multi；`--mode mono` opt-in | embed 为主源；确认后备份替换 |
| `scripts/install.sh` | 默认 multi；`DWS_SKILL_MODE=mono` | 双缓存原子刷新；目录替换前备份 |
| `scripts/install.ps1` | 同上 | 同上 |
| `scripts/install-skills.sh` | 默认 multi；环境变量 opt-in | `DWS_CACHE_ROOT` 下双缓存原子刷新 |
| npm `install.js` | 默认 multi；环境变量或 `--skill-mode` opt-in | `~/.dws/skills` 双缓存原子刷新 |

Homebrew 不直接铺 Agent home，安装后由 `dws skill setup` 完成布局安装。

## 3. 已验证的安全边界

- 空或损坏的 multi bundle 不进入 multi 安装分支，也不会覆盖有效缓存。
- 备份失败不复制新布局；扫描或 HOME 解析失败会让相应目标失败。
- setup 拒绝确认时零文件操作；显式确认只处理预览中列出的路径。
- Go、npm、Shell、PowerShell 的缓存复制失败均保留旧缓存。
- 多 Agent 目标彼此独立；一个目标失败不阻止其他目标尝试。upgrade 仅在无失败
  且至少一个目标成功时更新状态；setup 在至少一个目标安装成功后记录本次快照。
  npm、Shell 和 PowerShell 安装器完成其余目标尝试后，只要有目标失败就以非零
  状态结束，不再输出整体安装成功。

## 4. 决策记录

- **D1：布局和集合都由包驱动。** state 不做 mode sticky，也不参与升级集合
  求解；它仅保存官方清单信息快照。
- **D2：无服务端灰度。** 使用 beta 轨、issue 和主动回访观察；不实现
  `rollout.json` 或 `x-dws-skill-mode`。
- **D3：不采用 `npx skills add` 作为主分发通道。** DWS 继续维护 zip、embed
  和平台安装器，确保版本绑定、无 Node 环境和国内下载链路可用。
- **D4：不新增 mode lifecycle 命令。** setup 保留双向重装能力；删除前备份是
  安全机制，不等于提供自动 rollback 产品。
- **D5：mono retirement 独立推进。** 当前版本仍保留 mono 源和 opt-in 入口。

## 5. Roadmap

### 已完成

- 默认 multi 和五面安装入口对齐。
- mono/multi 互斥、过期目录与同名目录删除前备份。
- setup 精确计划、`--dry-run`、非交互确认门禁。
- 官方清单信息快照与普通/`--force` 全量覆盖升级语义。
- 缓存 staged publish 与旧缓存恢复。
- `.dws-managed` 所有权标记与同前缀市场/用户 Skill 保留门禁。

### 后续可选

- 收敛各语言的 Agent home 清单并增加 policy 门禁。
- 增加显式的备份查看/恢复 UX；当前恢复副本已存在，但没有自动 rollback 命令。
- 满足观察期、mono opt-in 率和内容策略门禁后，单独删除 mono 产物与入口。

## 6. mono 下线判据

1. 连续两周无 multi 相关 P1；
2. mono 主动 opt-in 已降到可接受范围；
3. multi 内容与 mono 能力完全对等；
4. policy 不再依赖 `skills/mono/SKILL.md`；
5. 存量 `<agent>/dws` 可通过常规 upgrade 安全迁移，并留有恢复副本。

## 7. 风险与缓解

| 风险 | 缓解 |
|---|---|
| 多目标复制中断 | 旧目录保存在 `skill-backups`，目标报告失败；后续可补自动 rollback UX |
| mono/multi 漂移 | 安装互斥清理；备份失败时不铺相反布局；upgrade 有 multi 包时迁移布局 |
| 预制 Skill 被用户删除或修改 | 下一次 upgrade 从官方 bundle 全量恢复并覆盖 |
| 状态文件损坏 | 明确回退全量并重写有效状态，不从目录猜测 CLI 参数意图 |
| 缓存复制失败 | 同级 staging 完整复制后发布；失败保留或恢复旧缓存 |
| 无服务端灰度 | beta 轨先行、issue/回访观察、必要时撤回 release 并引导重装 mono |
