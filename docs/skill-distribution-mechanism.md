# Skill 分发与消费机制调研（含 lark-cli 对标）

> 配套方案：[skill-multi-migration-plan.md](skill-multi-migration-plan.md)
> 调研日期：2026-08-04，基于 `feat/skill-mode-migration` 工作区代码级梳理 +
> lark-cli（larksuite/cli@main）公开仓库调研。

## 1. DWS 分发链路（源树 → 制品 → 渠道）

```
skills/mono/ ─┬─ go build ──► embed.FS（skills_embed.go:28）
skills/multi/ ┘                    │  dws skill setup 默认源
              │
              └─ post-goreleaser.sh:220-248 ──► dws-skills.zip
                 布局：zip 根 = mono 副本（向后兼容）+ mono/ + multi/
                        │
        ┌───────┬───────┼────────┬─────────┬──────────┐
     GitHub   Gitee    OSS     npm      Homebrew   专项脚本
     Release  (镜像)  (只发    tarball  (cellar    install-event/
        │       │     不读)     │       不铺agent) install-devapp
        │       │              │
   install.sh/ps1          postinstall
   install-skills.sh       install.js
        │                      │
        ├─► 各 agent home ◄────┤   永远 mono：<agent>/dws/
        └─► ~/.dws/skills/{mono,multi} 缓存
```

渠道清单（7 个安装/分发面）：

| 面 | skill 行为 | mode 概念 |
|---|---|---|
| `scripts/install.sh` | 装 mono 到 agent homes + 双缓存；选 multi **只打印提示、连缓存都跳过** | 有（半残） |
| `scripts/install.ps1` | 同 install.sh（Windows） | 有（半残） |
| `scripts/install-skills.sh` | 只装 mono，缓存 mono+multi | 无 |
| `build/npm/install.js` | 永远 mono；缓存 mono+multi | 无 |
| Homebrew formula | 整包 zip 进 cellar，不铺 agent、不写缓存，caveats 提示手动 setup | 无 |
| `scripts/install-event.sh` | 装 mono + 缓存 `multi/dingtalk-event`；多 `.config/opencode/skills` | 无 |
| `scripts/install-devapp.sh` | 缓存 `multi/dingtalk-dev` | 无 |

产物事实：`dws-skills.zip` 已含 mono/multi 双树，**切 multi 不需要改 release 产物**。

## 2. DWS 消费链路（源 → agent 目录）

### 2.1 `dws skill setup`（`internal/app/skill_setup.go`）

- 源优先级：`--source` / `DWS_SKILL_SOURCE`（失败不回退）→ **embed 默认**
  （`skill_setup_embed.go:50-58`）；legacy 候选（exe 旁/cwd/`~/.dws/skills`）
  仅在绕过 wrapper 直连时才走。
- 目标：`skillSetupAgentHomes` 16 个 agent home，父目录门控（i=0 `.agents`
  无条件，其余需 `~/.claude` 这类父目录存在）。`--target all` **不含
  opencode**，但 `agentSkillPaths` 命名 target 含（不对称，测试只锁单向）。
- 布局：mono → `<agent-home>/dws/`；multi → `<agent-home>/` 平铺兄弟目录。
- 互斥清理：装 mono 删 `dingtalk-*`、装 multi 删 `dws/`；**best-effort，
  失败仅 warning 继续装**（`cleanupMutualExclusion:611-620`），无备份。
- multi 过滤：`-s/--skill` 与 `-x/--exclude` 互斥；`dws-shared` 强制包含；
  未点名的已有 `dingtalk-*` 保留（additive）。

### 2.2 `dws upgrade`（`internal/upgrade/paths.go`）

- `LocateSkillMD` 命中 zip 根 mono → `UpgradeSkillLocations` 只写
  `<agent>/dws/`：**不识 multi、不清理 `dingtalk-*`、不更新 `~/.dws/skills`**。
- 后果：multi 用户升级后 **mono + multi 共存**，Agent 双份派发。
- `--rollback` 只回滚二进制，不回滚 skill。

### 2.3 市场 skill（`dws skill install`）

解压到 agent skills **根目录**（非 `dws/` 子目录），无 mode 概念；互斥清理按
`dingtalk-*` 前缀扫描，依赖"市场无同名前缀"隐性约定。

### 2.4 状态与缓存

- **无任何已安装模式状态**：`~/.dws/` 有 auth/backups/cache，无 install
  manifest；判断 mode 只能看磁盘形态。
- `~/.dws/skills` 缓存事实**只写不读**（默认 setup 走 embed），upgrade 不更新，
  版本漂移不可见。

## 3. lark-cli 分发机制（larksuite/cli@main）

npm 包 `@larksuite/cli` 是薄壳（`files` 仅 install.js / install-wizard.js /
run.js / checksums.txt）：

- **二进制**：postinstall 按平台从 GitHub Releases 下载，SHA256 校验
  （checksums.txt 随 npm 包发）；镜像链 GitHub → 用户 registry 派生镜像 →
  npmmirror 兜底；host allowlist + checksum 双保险；`run.js` 缺二进制自动补下，
  Windows 有 `.old` 崩溃恢复。
- **Skills**：不进 npm 包、不进二进制。repo 根 `skills/` 即事实源，由生态
  安装器 `npx skills add larksuite/cli -y -g`（vercel-labs/skills）安装；
  wizard 首选 `https://open.feishu.cn` 直链，GitHub shorthand 兜底。
- **一键向导** `npx @larksuite/cli@latest install`：run.js 拦截 `install` →
  install-wizard.js 串联 4 步（npm 全局装/升级 → skills → config init →
  auth login），每步幂等（`skills ls -g` 检测 `lark-*` 已装则跳过）；
  非 TTY 降级为"装完打印后续命令"。

生态安装器 `skills` CLI 提供的能力：

- 76 个 agent 目录清单生态维护，自动探测；project/global 双 scope；
- **symlink canonical（推荐）或 copy**；symlink 下升级 = 更新一处；
- `list / find / update / remove` 全生命周期；**skill 升级由
  `npx skills update` 承担，lark-cli 自己不写 skill 刷新逻辑**；
- 发现约定兼容 catalog 布局 `skills/<catalog>/<name>/SKILL.md`。

## 4. 对标：DWS vs lark-cli

| 维度 | lark-cli | DWS 现状 |
|---|---|---|
| 分发单元 | repo `skills/`（源码即事实源） | zip + embed + 5 处拷贝缓存 |
| 安装器 | 1 个（生态工具） | 7 个自维护脚本，已漂移 |
| 落盘 | symlink 单点更新 | 全量 copy，升级重写 16 home |
| skill 升级 | `npx skills update`（生态承担） | `dws upgrade` 自写，不识 multi |
| mono/multi | 不存在此问题（天生多 skill） | 互斥/切换/状态全自建 |
| npm 包 | 薄壳运行时下载 | 全平台 archive + zip 全打进 tarball |
| 大陆镜像 | registry 派生 + npmmirror | Gitee fallback + OSS 只发不读 |

## 5. "分发外包给生态，自己只维护源码目录"是什么

职责切分：skill 的分发/安装/升级/卸载交给生态标准化工具（`npx skills`，
"agent skill 界的 npm"），DWS 只保证 repo 里 `skills/` 符合 agentskills.io
规范。它消掉的正是 DWS 现在自维护的四块问题：

1. **N 份事实源**（zip 三拷贝 + embed + 缓存 + 16 home）→ 只剩 repo 目录一份；
2. **7 个自写安装器** → 零个，agent 清单别人维护（新 agent 自动支持）；
3. **自建升级/切换/回滚**（UpgradeSkillLocations、互斥清理、方案中的
   state.json/备份/`dws skill mode`）→ symlink 模式更新 canonical 一处；
4. **lark-cli 因此根本没有 mono/multi 之争**，也没有 P0 要修的那些 bug。

### 代价与前提（不是免费午餐）

| 风险 | 说明 | 对策 |
|---|---|---|
| 版本错配 | DWS skill 从 Cobra 树生成，与 CLI 版本强耦合；embed 天然同版，生态安装从主分支拉可能错配。**已实测**：`skills add`（v1.5.21）无 tag/ref 版本固定参数，`@` 后接的是 skill 名而非 git ref；唯一的可复现机制是 project 级 `skills-lock.json`（experimental_install） | 短期：发布说明引导"skill 随 CLI 升级（`skills update`）"；中期：release 时推一个 `release/vX.Y.Z` 镜像分支或专用 skills 镜像仓供按版本安装；或接受错配（skill 内容为文档，错配成本=提到不存在的命令） |
| 网络可达 | 依赖 npx + GitHub；大陆/内网现靠 Gitee fallback | `skills` CLI 支持任意 git URL，Gitee 镜像仓兜底，链路需验证 |
| 离线/打包场景 | 悟空 bundled-skills、企业预装镜像生态通道覆盖不了（2026-08-05 注：悟空分发线已于当日下线，「悟空 bundled-skills」一项不再适用；企业预装镜像约束仍在） | 自维护打包保留一条 |
| 生态工具策略漂移 | 清单/发现约定/默认值被动跟随 | 作为增量通道而非唯一通道，保留 embed 兜底 |

## 6. 结论

- **生态分发通道已否决**（2026-08-04）：无版本固定（`skills add` 不支持
  tag/ref，实测 v1.5.21）、依赖 Node + GitHub 可达、mono 会被一起发现、
  悟空/离线场景覆盖不了。分发维持全自维护。（2026-08-05 注：悟空分发线
  已于当日下线，「悟空场景覆盖不了」一条随之失效；其余否决理由与结论
  不变。）
- 一个月内：按方案 P0–P2 修自维护通道并切 multi 默认（zip + embed +
  安装脚本，产物布局不变）。
- 终态即"自维护通道修好之后"的形态，不再向 lark-cli 的生态外包形态收敛。
- 调研保留备查：`npx -y skills add . --list` 实测可发现全部 21 个 skill
  （若未来生态工具补齐版本固定能力，可重新评估本决策）。
