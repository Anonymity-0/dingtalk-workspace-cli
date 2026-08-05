# Skill multi 迁移：技术方案（as-implemented）与 Roadmap

> 本文是 multi 迁移的当前事实源：第一部分记录**已落地实现**（代码级锚点，
> 均可跳转验证），第二部分是带实时状态的 roadmap。
> 原始方案 [skill-multi-migration-plan.md](skill-multi-migration-plan.md) 的
> 若干"待做"描述已被后续决策取代（见决策记录 D1/D5）；分发机制调研结论见
> [skill-distribution-mechanism.md](skill-distribution-mechanism.md)。
> 代码快照：`feat/skill-mode-migration` @ `402429ac` + 工作区
> upgrade-force-multi 调整（2026-08-05）。

## 状态速览

| 项 | 内容 |
|---|---|
| 当前阶段 | **阶段 1 / 1.5 ✅**（安装/升级默认 multi 已落地）；**阶段 2 运行时切换产品线 ❌ CANCELLED**（2026-08-05 owner 决策） |
| 硬 deadline | **2026-08-30**：安装/升级默认 multi（✅）+ upgrade **有 multi 包时一次性刷成 multi**（不做磁盘粘性）+ mono 仅安装时 opt-in；mono **物理下线**仍可在独立 retirement 版本推进，但**不再**依赖 `dws skill mode` / 备份回滚产品 |
| 已完成 | 五面默认 multi、`dws skill setup` 默认 multi、互斥清理对称、文案翻转；upgrade 有 `multi/` 时一律刷新 multi（存量 mono 一次性迁移） |
| 下一步 | beta/L1 版本门控与观察（靠 issue/回访，**无** `x-dws-skill-mode` 埋点）；可选 agent-home 清单门禁；内容 C 线并行 |
| 终态 | mono 仅安装时 opt-in；日常 upgrade（含 multi 的包）刷 multi；mono 物理删除仍可在 dedicated retirement 版本收尾（非用户切换命令） |

### 简化设计（2026-08-05 起生效）

1. **安装时一次决定**：默认 multi；`DWS_SKILL_MODE=mono` / `--mode mono` / 安装器 TTY 选 mono 为唯一 opt-in。
2. **装完无运行时切换产品**：不做 `dws skill mode set/rollback`，不做备份式安装 / `state.json` 记账产品面。
3. **升级不做粘性**：release zip 含 `multi/` 时 **一律** 刷新 multi（清 `dws/`、刷产品 skill + 缓存）；仅 legacy 无 multi 树的包回退 mono 路径。存量 mono 在日常 upgrade 上一次性迁到 multi。
4. **mono 下线**：安装侧仍可 opt-in；upgrade 已承担「有 multi 包即迁走」；物理删 mono 树可另议 retirement。
5. **可观测**：`x-dws-skill-mode` 请求头已按 owner 决策移除；灰度靠 issue + 回访。

---

# 第一部分：技术方案（as-implemented）

## 1. 升级：包驱动 multi 刷新（`dws upgrade`）

核心语义：**升级不做磁盘粘性**；产物有 `multi/` 时一次性刷成 multi。
无需 `state.json`，也无运行时切换命令。

- `LocateSkillsRoot` 优先返回 zip 内 `multi/`（`internal/upgrade/paths.go`）。
- `UpgradeSkillLocations`：包内有 multi 技能树 → **始终** `upgradeMultiSkillLocations`
  （平铺 `dingtalk-*` + `dws-shared`，删 mono 残留 `dws/`，清过期 multi skill，
  刷 `~/.dws/skills/multi`）；无 multi 树的 legacy 包才走 mono 刷新。
- 这是 upgrade 上的一次性迁移，不是 `dws skill mode` 产品。

测试：`internal/upgrade/paths_multi_test.go`（含
`TestUpgradeSkillLocationsMonoDiskMigratesToMulti`）+
`internal/app/upgrade_skill_multi_e2e_test.go`。

## 2. 安装默认 multi（四个脚本面）

四个脚本安装面默认值全部为 multi，`DWS_SKILL_MODE=mono` 为统一 opt-in，
互斥清理双向对称。详见阶段 1 落地说明（`scripts/install.sh` /
`install.ps1` / `build/npm/install.js` / `scripts/install-skills.sh`）。

## 3. `dws skill setup` 默认 multi

- 非交互未指定 `--mode` 时默认 multi。
- 交互选项 multi 在前（默认）、mono 标 legacy。
- 仍可用 `dws skill setup --mode mono --yes` **重装**到 mono（这是安装入口，
  不是 lifecycle 切换产品；无备份/state/rollback 命令）。

## 4. 文案翻转

- `README.md` / `README_zh.md`：multi 默认、mono legacy。
- `skills/multi/dingtalk-skill/SKILL.md`：去掉 EXPERIMENTAL。

## 5. 已验证

- `go test ./internal/upgrade ./internal/app ./test/scripts`（阶段 1 基线）+
  upgrade-force-multi 单测 / fake-HOME E2E。
- 脚本面契约测试暴露 `DWS_SKILL_MODE` 与 mono/multi 选项。

## 6. 决策记录

- **D1 升级不依赖 state.json**（仍成立）。不读状态文件；布局由包内容驱动。
- **D2 无服务端灰度**。L1 beta 轨 + issue/回访；L2 `rollout.json` 已砍；
  kill switch = beta 撤回 / 重装 `--mode mono`（无 `skill mode` 命令）。
- **D3 生态分发通道已否决**（`npx skills add` 等）。见
  [skill-distribution-mechanism.md](skill-distribution-mechanism.md)。
- **D4 互斥前缀约定**。`dingtalk-*` / `dws-shared` 属 DWS 产品 skill。
- **D5 无运行时模式切换（2026-08-05）**。取消阶段 2 的备份式安装、
  `state.json`、`dws skill mode`（status/set/rollback）、`x-dws-skill-mode`
  请求头。Mode 只在安装时决定。
- **D6 升级不做粘性（2026-08-05）**。有 multi 包时 upgrade 一次性刷成 multi
  （含存量 mono）；legacy 无 multi 树才回退 mono 路径。

---

# 第二部分：Roadmap

## ✅ 阶段 1 / 1.5（已完成，2026-08-05）

安装/升级默认 multi、五面互斥清理、文案翻转、1.5 review 修复与实机 9/9。
HEAD：`402429ac`。

## ❌ 阶段 2（原切换/状态/备份产品线）— CANCELLED（2026-08-05）

| 原任务 | 状态 | 说明 |
|---|---|---|
| 备份式安装（`~/.dws/skills/backup/...`） | ❌ CANCELLED | 不做运行时切换，无需备份回滚产品 |
| `~/.dws/skills/state.json` | ❌ CANCELLED | 无切换产品；upgrade 按包刷 multi，不需要状态文件 |
| `dws skill mode` status/set/rollback/--dry-run | ❌ CANCELLED | 无运行时切换 UX |
| `x-dws-skill-mode` 请求头 | ❌ CANCELLED | owner 决策移除；观测改 issue/回访 |
| agent home 清单门禁 | ⬜ 可选 | 与切换产品无关，仍可作工程质量项 |
| `upgrade --dry-run` multi 文案 | ⬜ 可选 | 可随 force-multi 语义轻量对齐 |

## 重构后的 8/30 目标

**底线**：新装默认 multi；含 `multi/` 的 release 上 `dws upgrade` **一次性刷成
multi**（含存量 mono）；仅需保持 mono 的用户用安装入口 opt-in 后勿升级到
含 multi 的包，或 retirement 前用 setup 重装；mono 物理删除可另议。

### 建议关键路径（简化）

```text
阶段1默认multi ✅ → upgrade force-multi ✅ → beta/L1（可选）→ 观察(issue/回访)
→ stable 默认已是 multi → 可选 mono retirement（删 mono 树 / 安装入口报错）
```

### mono retirement（原阶段 4，重框）

- **不再**提供 `dws skill mode rollback` 作为用户出口。
- 日常 upgrade 已在有 multi 包时迁走存量 mono；retirement 版本可进一步移除
  install/setup 的 mono 分支与 zip 内 mono 树。
- 下线判据改为：issue/回访无系统性 multi P1、mono opt-in 可接受、政策门
  不再依赖 `skills/mono/SKILL.md`。（原请求头占比判据作废。）

## 悟空线下线的影响（2026-08-05）

悟空 bundled-skill 分发线已下线：mono 下线无仓外节奏闸门；`skills/multi/`
为唯一 multi 事实源。设计资产留档
[skill-wukong-comparison.md](skill-wukong-comparison.md)。

## 明确不做

- ❌ 运行时模式切换（`dws skill mode set/rollback`）。
- ❌ `state.json` / 备份式安装产品面（随 D5 取消）。
- ❌ `x-dws-skill-mode` 请求头。
- ❌ 服务端远程配置 / L2 `rollout.json`。
- ❌ 运行时「模式切换」产品（含 sticky 伪切换）；upgrade 有 multi 时刷 multi
  是包驱动的一次性迁移，不是 switch UX。
- 不动 `dws-skills.zip` 双树布局（仍可含 mono 副本供安装 opt-in）；不动市场
  skill（`dws skill install`）。

---

## 风险表

| 风险 | 现状 | 缓解 |
|---|---|---|
| 半装无备份 | 安装/清理仍 `RemoveAll` | 接受为非切换产品下的已知限制；重装可收敛 |
| mono/multi 漂移 | 无 state；upgrade 有 multi 即刷 multi | 升级后收敛为 multi；安装互斥清理 |
| 用户想换模式 | 无 switch 命令 | 文档引导：`dws skill setup --mode <mono\|multi> --yes` 重装 |
| 8/30 滑期 | 切换产品线已砍，关键路径缩短 | 聚焦 force-multi upgrade + 默认 multi 稳定 |
