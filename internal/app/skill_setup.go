package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/skillstate"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/upgrade"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

// skillSetupAgentHomes is the ordered list of agent home subdirectories
// where dws skills get installed. Mirrors install.sh / install.ps1 /
// build/npm/install.js so that `dws skill setup` and the install scripts
// agree on the install footprint.
var skillSetupAgentHomes = []string{
	".agents/skills",
	".claude/skills",
	".cursor/skills",
	".qoder/skills",
	".qoderwork/skills",
	".gemini/skills",
	".codex/skills",
	".github/skills",
	".windsurf/skills",
	".augment/skills",
	".cline/skills",
	".amp/skills",
	".kiro/skills",
	".trae/skills",
	".openclaw/skills",
	".hermes/skills",
}

const (
	skillSetupModeMono  = "mono"
	skillSetupModeMulti = "multi"
)

var (
	skillSetupResolveMode     = resolveSkillSetupMode
	skillSetupResolveSource   = resolveSkillSetupSourceOrEmbedded
	skillSetupResolveTargets  = resolveSkillSetupTargets
	skillSetupListMulti       = listMultiSkillNames
	skillSetupFilterMulti     = filterMultiSkillNames
	skillSetupBuildPlan       = buildSkillSetupPlan
	skillSetupConfirmPlan     = confirmSkillSetupPlan
	skillSetupExecutePlan     = executeSkillSetupPlan
	skillSetupCopyDir         = copyDir
	skillSetupRunForm         = (*huh.Form).Run
	skillSetupInteractive     = isInteractiveTerminal
	skillSetupReadDir         = os.ReadDir
	skillSetupStat            = os.Stat
	skillSetupExecutable      = os.Executable
	skillSetupGetwd           = os.Getwd
	skillSetupUserHomeDir     = os.UserHomeDir
	skillSetupRemoveAll       = os.RemoveAll
	skillSetupBackupAndRemove = upgrade.BackupAndRemoveSkillDir
	skillSetupMkdirAll        = os.MkdirAll
	skillSetupWalk            = filepath.Walk
	skillSetupRel             = filepath.Rel
	skillSetupReadlink        = os.Readlink
	skillSetupOpen            = os.Open
	skillSetupOpenFile        = os.OpenFile
	skillSetupCopy            = io.Copy
	skillSetupWriteState      = skillstate.Write
	skillSetupRemoveState     = skillstate.Remove
	skillSetupNow             = time.Now
)

type skillSetupBackup struct {
	Path   string
	Reason string
}

type skillSetupTargetPlan struct {
	Destination string
	Backups     []skillSetupBackup
}

type skillSetupPlan struct {
	Mode            string
	Source          string
	MultiSkillNames []string
	Filtered        bool
	Targets         []skillSetupTargetPlan
}

const (
	skillSetupBackupMutual  = "opposite layout"
	skillSetupBackupStale   = "stale official Skill"
	skillSetupBackupReplace = "same-name Skill"
)

func newSkillSetupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "安装 dws 自身 skill 到 Agent 目录",
		Long: `安装 dws 自身 skill 文档到 AI Agent 目录（如 ~/.claude/skills/、~/.cursor/skills/ 等）。

支持两种模式：
  multi                 多 skill（默认）—— 按产品拆 N 个独立 skill（dingtalk-*）
  mono                  单 skill（legacy）—— 总入口 SKILL.md + references/products/

multi 模式支持按产品挑选：
  -s/--skill   只装指定子 skill（可重复，短名 aitable 或全名 dingtalk-aitable 均可）
  -x/--exclude 从全装里剔除指定子 skill（可重复，与 --skill 互斥）
  用 -s/-x 挑选时未列出的已有 dingtalk-* skill 会保留（additive 叠加语义）；
  不带过滤条件的全量安装会清理不在 bundle 内的过期 dingtalk-* / dws-shared。
  setup 成功后记录本次官方清单；后续 dws upgrade 会刷新仍在本地的官方 skill，
  自动加入新版新增 skill，并跳过本地已删除的旧 skill。dws upgrade --force 恢复全量。
清理与备份（本命令可能移除的目录）：
  · 安装任一模式前会清理对面模式残留：装 mono 移除 <agent-home>/dingtalk-*，
    装 multi 移除 <agent-home>/dws/；全量 multi 安装还会移除不在 bundle 内的
    过期 dingtalk-* / dws-shared。
  · 被移除的目录与同名旧 skill 会先备份到 ~/.dws/skill-backups/<时间戳>/；
    备份失败时保留原目录并跳过该目标，绝不静默删除。
  · 所有将被移除的目录都会在确认前逐条列出。

不带 --mode 时进入交互式询问；不带 --target 时铺到所有检测到的 Agent 目录。
skill 源默认取二进制内嵌的版本（升级二进制即升级 skill）；--source / DWS_SKILL_SOURCE 可显式覆盖。`,
		Example: `  dws skill setup                                             # 交互式
  dws skill setup --mode mono --yes                         # 非交互装 mono
  dws skill setup --mode multi --target claude --yes        # multi 全装到 ~/.claude/skills/
  dws skill setup --mode multi -s aitable -s calendar --yes # 只装 aitable + calendar
  dws skill setup --mode multi -x live -x devdoc --yes      # 安装除 live、devdoc 外的其余 skill
  dws skill setup --source /path/to/repo                # 显式指定 skill 源`,
		DisableAutoGenTag: true,
		RunE:              runSkillSetup,
	}
	cmd.Flags().String("mode", "", "skill 模式：mono | multi（不指定则交互询问）")
	cmd.Flags().String("target", "all", "目标 Agent：all | "+supportedTargets())
	cmd.Flags().String("source", "", "skill 源目录（默认使用二进制内嵌的 skill 源，与当前版本一致）")
	cmd.Flags().Bool("yes", false, "跳过确认提示（仅供脚本使用；删除操作仍会先备份到 ~/.dws/skill-backups/）")
	cmd.Flags().StringSliceP("skill", "s", nil, "multi 模式：仅安装指定子 skill（可重复，接受短名 aitable 或全名 dingtalk-aitable）")
	cmd.Flags().StringSliceP("exclude", "x", nil, "multi 模式：从全装中剔除指定子 skill（可重复，与 --skill 互斥）")
	return cmd
}

func runSkillSetup(cmd *cobra.Command, _ []string) error {
	mode, _ := cmd.Flags().GetString("mode")
	target, _ := cmd.Flags().GetString("target")
	source, _ := cmd.Flags().GetString("source")
	autoYes, _ := cmd.Flags().GetBool("yes")
	includeRaw, _ := cmd.Flags().GetStringSlice("skill")
	excludeRaw, _ := cmd.Flags().GetStringSlice("exclude")

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	mode, err := skillSetupResolveMode(mode, autoYes, out)
	if err != nil {
		return err
	}

	if mode == skillSetupModeMono && (len(includeRaw) > 0 || len(excludeRaw) > 0) {
		return fmt.Errorf("--skill / --exclude 仅在 --mode multi 下有效（mono 只有一个 skill，无需挑选）")
	}

	skillSrc, srcCleanup, err := skillSetupResolveSource(source, mode)
	if err != nil {
		return err
	}
	defer srcCleanup()

	dests, err := skillSetupResolveTargets(target, mode)
	if err != nil {
		return err
	}

	// multi 模式枚举 src 下的子 skill 名，供确认信息与安装步骤共用
	var multiSkillNames, allMultiSkillNames []string
	if mode == skillSetupModeMulti {
		var listErr error
		allMultiSkillNames, listErr = skillSetupListMulti(skillSrc)
		if listErr != nil {
			return listErr
		}
		if len(allMultiSkillNames) == 0 {
			return fmt.Errorf("multi 模式下 %s 内未发现含 SKILL.md 的子目录", skillSrc)
		}
		filtered, filterErr := skillSetupFilterMulti(allMultiSkillNames, includeRaw, excludeRaw)
		if filterErr != nil {
			return filterErr
		}
		// dingtalk-shared carries the global rules every product skill declares as a
		// PREREQUISITE; it must ship even when --skill / --exclude narrows the set.
		multiSkillNames = ensureMandatorySharedSkill(filtered, allMultiSkillNames)
	}

	// filtered 决定 multi 安装的清理语义：带 -s/--skill 或 -x/--exclude
	// 时保持 additive（不动未列出的 sibling）；全量安装与 install.sh /
	// install.js 对齐，清掉不在 bundle 内的过期 dingtalk-* / dws-shared。
	filtered := len(includeRaw) > 0 || len(excludeRaw) > 0
	plan, err := skillSetupBuildPlan(mode, skillSrc, dests, multiSkillNames, filtered)
	if err != nil {
		return fmt.Errorf("无法完整计算 Skill 安装计划: %w", err)
	}

	// --dry-run 与交互确认消费同一份计划，所以展示的备份路径
	// 与执行阶段传给 BackupAndRemove 的路径完全一致。
	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		fmt.Fprintf(out, "[DRY-RUN] 预览（不写入任何文件）：mode=%s\n", plan.Mode)
		renderSkillSetupPlan(out, plan)
		return nil
	}

	if !autoYes {
		ok, err := skillSetupConfirmPlan(out, plan)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, "已取消。")
			return nil
		}
	}

	installed, skipped, err := skillSetupExecutePlan(plan, out, errOut)
	if err != nil {
		return err
	}
	if installed > 0 {
		home, homeErr := skillSetupUserHomeDir()
		if homeErr != nil {
			return fmt.Errorf("Skill 已安装，但无法解析 HOME 以保存更新状态: %w", homeErr)
		}
		if mode == skillSetupModeMulti {
			selected := make(map[string]bool, len(multiSkillNames))
			for _, name := range multiSkillNames {
				selected[name] = true
			}
			var stateSkipped []string
			for _, name := range allMultiSkillNames {
				if !selected[name] {
					stateSkipped = append(stateSkipped, name)
				}
			}
			state := skillstate.State{
				Version:              RawVersion(),
				OfficialSkills:       allMultiSkillNames,
				UpdatedSkills:        multiSkillNames,
				SkippedDeletedSkills: stateSkipped,
				UpdatedAt:            skillSetupNow().UTC().Format(time.RFC3339),
			}
			if stateErr := skillSetupWriteState(home, state); stateErr != nil {
				return fmt.Errorf("Skill 已安装，但保存后续增量更新状态失败: %w", stateErr)
			}
		} else if stateErr := skillSetupRemoveState(home); stateErr != nil {
			return fmt.Errorf("mono Skill 已安装，但清理 multi 更新状态失败: %w", stateErr)
		}
	}
	fmt.Fprintf(out, "\n✅ Skill 安装完成（mode=%s, installed=%d, skipped=%d）\n", mode, installed, skipped)
	return nil
}

// multiSkillPrefix is the canonical prefix for every per-product skill
// bundle in skills/multi/ (e.g. dingtalk-aitable, dingtalk-calendar).
const multiSkillPrefix = "dingtalk-"

// multiSharedSkill is the shared, non-product skill that every per-product
// skill declares as a PREREQUISITE. It must always be installed in multi mode
// regardless of --skill / --exclude, otherwise the product skills reference a
// dingtalk-shared that was never installed.
const multiSharedSkill = "dingtalk-shared"

// legacySharedSkill is the pre-rename directory name of the shared skill.
// Installations created before the dws-shared -> dingtalk-shared rename keep a
// dws-shared directory on disk; cleanup paths must still recognize it so a
// full install / mode switch removes it instead of leaving an orphaned,
// unreferenced skill next to the new dingtalk-shared.
const legacySharedSkill = "dws-shared"

// isDWSMultiSkillName reports whether name belongs to a DWS multi-mode skill
// directory: product skills use the dingtalk- prefix and the shared bundle is
// dingtalk-shared (or its legacy pre-rename name dws-shared).
func isDWSMultiSkillName(name string) bool {
	return strings.HasPrefix(name, multiSkillPrefix) ||
		name == multiSharedSkill ||
		name == legacySharedSkill
}

// ensureMandatorySharedSkill guarantees the shared dependency skill is included
// whenever it exists in the source, even if --skill / --exclude narrowed it out.
func ensureMandatorySharedSkill(selected, all []string) []string {
	hasShared := false
	for _, n := range all {
		if n == multiSharedSkill {
			hasShared = true
			break
		}
	}
	if !hasShared {
		return selected
	}
	for _, n := range selected {
		if n == multiSharedSkill {
			return selected
		}
	}
	return append([]string{multiSharedSkill}, selected...)
}

// normalizeMultiSkillName accepts either the short form (aitable) or the
// full form (dingtalk-aitable) and returns the canonical full form.
// Empty input returns "". Comparison is case-insensitive.
func normalizeMultiSkillName(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ""
	}
	if strings.HasPrefix(n, multiSkillPrefix) {
		return n
	}
	return multiSkillPrefix + n
}

// filterMultiSkillNames narrows `all` by include / exclude lists:
//
//   - include + exclude are mutually exclusive (both → error)
//   - names accept short or full form; normalized before matching
//   - unknown names → error, with the available list inlined for discovery
//   - both lists empty → return `all` (install everything)
//   - exclude that drops every name → error (avoid silent no-op install)
//
// The caller threads whether a filter was used into installMultiSkillToHomes:
// filtered installs stay additive (already-installed dingtalk-* siblings are
// left untouched); a full unfiltered install also removes stale dingtalk-* /
// dws-shared directories that are no longer part of the bundle.
func filterMultiSkillNames(all, include, exclude []string) ([]string, error) {
	if len(include) > 0 && len(exclude) > 0 {
		return nil, fmt.Errorf("--skill 与 --exclude 不能同时使用")
	}

	available := make(map[string]struct{}, len(all))
	for _, n := range all {
		available[n] = struct{}{}
	}

	validate := func(raw []string, flagName string) ([]string, error) {
		var normalized []string
		var unknown []string
		seen := make(map[string]bool)
		for _, r := range raw {
			n := normalizeMultiSkillName(r)
			if n == "" {
				continue
			}
			if _, ok := available[n]; !ok {
				unknown = append(unknown, r)
				continue
			}
			if !seen[n] {
				seen[n] = true
				normalized = append(normalized, n)
			}
		}
		if len(unknown) > 0 {
			return nil, fmt.Errorf("%s 中的以下名称在 multi 源中找不到：%s\n可用列表（共 %d 个）：%s",
				flagName, strings.Join(unknown, ", "), len(all), strings.Join(all, ", "))
		}
		return normalized, nil
	}

	if len(include) > 0 {
		names, err := validate(include, "--skill")
		if err != nil {
			return nil, err
		}
		sort.Strings(names)
		return names, nil
	}
	if len(exclude) > 0 {
		excluded, err := validate(exclude, "--exclude")
		if err != nil {
			return nil, err
		}
		excludedSet := make(map[string]bool, len(excluded))
		for _, n := range excluded {
			excludedSet[n] = true
		}
		var out []string
		for _, n := range all {
			if !excludedSet[n] {
				out = append(out, n)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("--exclude 把全部 %d 个子 skill 都剔除了，没有可装的", len(all))
		}
		return out, nil
	}
	return all, nil
}

// listMultiSkillNames returns sorted names of subdirectories under src that
// contain a SKILL.md file (i.e. valid multi-mode skill bundles).
func listMultiSkillNames(src string) ([]string, error) {
	entries, err := skillSetupReadDir(src)
	if err != nil {
		return nil, fmt.Errorf("无法读取 multi skill 源目录 %s: %w", src, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := skillSetupStat(filepath.Join(src, e.Name(), "SKILL.md")); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// resolveSkillSetupMode resolves the mode either from the flag or via an
// interactive prompt. If no TTY is available and no mode was given, returns
// an error rather than silently picking a default.
func resolveSkillSetupMode(mode string, autoYes bool, out io.Writer) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	switch mode {
	case skillSetupModeMono, skillSetupModeMulti:
		return mode, nil
	case "":
		// fall through to interactive prompt
	default:
		return "", fmt.Errorf("不支持的 --mode 值: %s（可选 mono / multi）", mode)
	}

	if autoYes || !skillSetupInteractive() {
		fmt.Fprintln(out, "未指定 --mode，非交互环境下默认使用 multi")
		return skillSetupModeMulti, nil
	}

	var choice string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("选择 dws skill 安装模式").
				Description("multi = 按产品拆分（默认）\nmono = 单 skill 入口（legacy）").
				Options(
					huh.NewOption("multi — 多 skill（默认）", skillSetupModeMulti),
					huh.NewOption("mono — 单 skill（legacy）", skillSetupModeMono),
				).
				Value(&choice),
		),
	)
	if err := skillSetupRunForm(form); err != nil {
		return "", fmt.Errorf("交互式选择中止: %w", err)
	}
	return choice, nil
}

// resolveSkillSetupSource finds the local skill source directory for the
// given mode ("mono" or "multi"). Explicit overrides (--source flag or
// DWS_SKILL_SOURCE) win and never fall back to another source; without an
// override the ordered candidate list (binary-adjacent, working directory,
// ~/.dws/skills user cache) is probed for a valid skill root of that mode.
func resolveSkillSetupSource(explicit, mode string) (string, error) {
	subdir := mode // "mono" or "multi"

	// An explicit override (--source flag or DWS_SKILL_SOURCE) wins, and an
	// override that does not contain a skill root is an error — never a
	// silent fallback to another source the user did not ask for.
	var overrides []string
	if explicit != "" {
		overrides = append(overrides, explicit, filepath.Join(explicit, "skills", subdir))
	}
	if env := strings.TrimSpace(os.Getenv("DWS_SKILL_SOURCE")); env != "" {
		overrides = append(overrides, env, filepath.Join(env, "skills", subdir))
	}
	if len(overrides) > 0 {
		for _, c := range overrides {
			if isSkillSourceRoot(c, mode) {
				return c, nil
			}
		}
		hint := strings.Join(overrides, "\n  - ")
		return "", fmt.Errorf("未找到 %s 模式的 skill 源目录（--source / DWS_SKILL_SOURCE 显式指定时不回退到内嵌源），已尝试：\n  - %s", mode, hint)
	}

	// No explicit override: legacy fallback only — embedded materialization
	// is handled by resolveSkillSetupSourceOrEmbedded (skill_setup_embed.go),
	// the wrapper that callers use. This branch is reachable only when the
	// wrapper passes through with an empty explicit/env (legacy direct call).
	candidates := skillSourceCandidates("", subdir)
	for _, c := range candidates {
		if isSkillSourceRoot(c, mode) {
			return c, nil
		}
	}

	hint := strings.Join(candidates, "\n  - ")
	return "", fmt.Errorf("未找到 %s 模式的 skill 源目录，已尝试：\n  - %s\n\n请用 --source 显式指定包含 skills/%s 的仓库根目录", mode, hint, mode)
}

// skillSourceCandidates returns the ordered list of paths to probe for a
// skill source root, given an optional explicit override and the mode
// subdir (mono or multi).
func skillSourceCandidates(explicit, subdir string) []string {
	var roots []string
	if explicit != "" {
		// allow either repo root or already-resolved skills/<mode> dir
		roots = append(roots, explicit, filepath.Join(explicit, "skills", subdir))
	}
	if env := strings.TrimSpace(os.Getenv("DWS_SKILL_SOURCE")); env != "" {
		roots = append(roots, env, filepath.Join(env, "skills", subdir))
	}
	if exe, err := skillSetupExecutable(); err == nil {
		exeDir := filepath.Dir(exe)
		roots = append(roots,
			filepath.Join(exeDir, "skills", subdir),
			filepath.Join(exeDir, "..", "skills", subdir),
			filepath.Join(exeDir, "..", "share", "skills", "dws"),
		)
	}
	if wd, err := skillSetupGetwd(); err == nil {
		roots = append(roots, filepath.Join(wd, "skills", subdir))
	}
	// User-level cache populated by install.sh / install.ps1 / npm install.js
	// from the dws-skills.zip release asset. Lets `dws skill setup` find a
	// source even when the user has no source checkout on disk.
	if home, err := skillSetupUserHomeDir(); err == nil {
		roots = append(roots, filepath.Join(home, ".dws", "skills", subdir))
	}
	return roots
}

func isSkillSourceRoot(path, mode string) bool {
	if path == "" {
		return false
	}
	switch mode {
	case skillSetupModeMono:
		fi, err := skillSetupStat(filepath.Join(path, "SKILL.md"))
		return err == nil && !fi.IsDir()
	case skillSetupModeMulti:
		entries, err := skillSetupReadDir(path)
		if err != nil {
			return false
		}
		for _, e := range entries {
			if e.IsDir() {
				if _, err := skillSetupStat(filepath.Join(path, e.Name(), "SKILL.md")); err == nil {
					return true
				}
			}
		}
		return false
	}
	return false
}

// resolveSkillSetupTargets returns the list of absolute Agent home destinations.
// If target == "all", returns every agent home whose parent directory exists.
// Otherwise returns the single matching home (whether or not it currently exists).
//
// 末段约定：
//   - mono  → <agent-home>/dws   （单 skill，整个 src 拷成一个 dws 目录）
//   - multi → <agent-home>       （安装时把 src 下每个子目录拷成兄弟 skill）
func resolveSkillSetupTargets(target, mode string) ([]string, error) {
	home, err := skillSetupUserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("无法解析用户 HOME: %w", err)
	}

	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" || target == "all" {
		return detectExistingAgentHomes(home, mode), nil
	}

	rel, ok := agentSkillPaths[target]
	if !ok {
		return nil, fmt.Errorf("不支持的 --target 值: %s（可选 all, %s）", target, supportedTargets())
	}
	return []string{agentHomeForMode(filepath.Join(home, rel), mode)}, nil
}

// agentHomeForMode appends the mode-specific tail segment to an agent home base.
func agentHomeForMode(base, mode string) string {
	if mode == skillSetupModeMulti {
		return base
	}
	return filepath.Join(base, "dws")
}

func detectExistingAgentHomes(home, mode string) []string {
	var out []string
	for i, rel := range skillSetupAgentHomes {
		base := filepath.Join(home, rel)
		parent := filepath.Dir(base)
		if i > 0 {
			if _, err := skillSetupStat(parent); errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		out = append(out, agentHomeForMode(base, mode))
	}
	return out
}

func buildSkillSetupPlan(mode, src string, dests, multiSkillNames []string, filtered bool) (*skillSetupPlan, error) {
	if mode != skillSetupModeMono && mode != skillSetupModeMulti {
		return nil, fmt.Errorf("内部错误：未知 mode %q", mode)
	}
	plan := &skillSetupPlan{
		Mode:            mode,
		Source:          src,
		MultiSkillNames: append([]string(nil), multiSkillNames...),
		Filtered:        filtered,
	}
	sort.Strings(plan.MultiSkillNames)
	sortedDests := append([]string(nil), dests...)
	sort.Strings(sortedDests)
	for _, dest := range sortedDests {
		target := skillSetupTargetPlan{Destination: dest}
		seen := map[string]bool{}
		add := func(path, reason string) {
			if seen[path] {
				return
			}
			seen[path] = true
			target.Backups = append(target.Backups, skillSetupBackup{Path: path, Reason: reason})
		}
		mutual, err := mutualExclusionVictims(dest, mode)
		if err != nil {
			return nil, err
		}
		for _, path := range mutual {
			add(path, skillSetupBackupMutual)
		}
		if mode == skillSetupModeMulti && !filtered {
			stale, staleErr := staleMultiSkillVictimsWithError(dest, multiSkillNames)
			if staleErr != nil {
				return nil, staleErr
			}
			for _, path := range stale {
				add(path, skillSetupBackupStale)
			}
		}
		var replacements []string
		if mode == skillSetupModeMono {
			replacements = []string{dest}
		} else {
			for _, name := range plan.MultiSkillNames {
				replacements = append(replacements, filepath.Join(dest, name))
			}
		}
		for _, path := range replacements {
			info, statErr := skillSetupStat(path)
			if statErr != nil {
				if errors.Is(statErr, os.ErrNotExist) {
					continue
				}
				return nil, fmt.Errorf("检查将被替换的 Skill 失败 %s: %w", path, statErr)
			}
			if info.IsDir() {
				add(path, skillSetupBackupReplace)
			}
		}
		sort.Slice(target.Backups, func(i, j int) bool { return target.Backups[i].Path < target.Backups[j].Path })
		plan.Targets = append(plan.Targets, target)
	}
	return plan, nil
}

func renderSkillSetupPlan(out io.Writer, plan *skillSetupPlan) {
	fmt.Fprintf(out, "📦 将安装 skill：\n  mode: %s\n  source: %s\n", plan.Mode, plan.Source)
	if plan.Mode == skillSetupModeMulti {
		fmt.Fprintf(out, "  将装 %d 个独立 skill（按子目录平铺到 <agent-home>/<skill-name>/）：\n", len(plan.MultiSkillNames))
		for _, name := range plan.MultiSkillNames {
			fmt.Fprintf(out, "    · %s\n", name)
		}
	}
	fmt.Fprintln(out, "  destinations:")
	for _, target := range plan.Targets {
		fmt.Fprintf(out, "    - %s\n", target.Destination)
	}
	fmt.Fprintln(out, "  将备份并移除（先保存到 ~/.dws/skill-backups/）：")
	count := 0
	for _, target := range plan.Targets {
		for _, backup := range target.Backups {
			switch backup.Reason {
			case skillSetupBackupMutual:
				fmt.Fprintf(out, "    × 将备份并移除对面模式残留 %s\n", backup.Path)
			case skillSetupBackupStale:
				fmt.Fprintf(out, "    × 将备份并移除过期 skill %s\n", backup.Path)
			default:
				fmt.Fprintf(out, "    × 将备份并移除同名 Skill %s\n", backup.Path)
			}
			count++
		}
	}
	if count == 0 {
		fmt.Fprintln(out, "    (无)")
	}
}

func confirmSkillSetup(out io.Writer, mode, src string, dests []string, multiSkillNames []string, filtered bool) (bool, error) {
	plan, err := buildSkillSetupPlan(mode, src, dests, multiSkillNames, filtered)
	if err != nil {
		return false, err
	}
	return confirmSkillSetupPlan(out, plan)
}

func confirmSkillSetupPlan(out io.Writer, plan *skillSetupPlan) (bool, error) {
	fmt.Fprintln(out)
	renderSkillSetupPlan(out, plan)

	if !skillSetupInteractive() {
		return false, fmt.Errorf("非交互环境无法确认目录迁移；请先用 --dry-run 预览，确认后显式传入 --yes")
	}

	var confirm bool
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Title("确认安装？").
				Affirmative("继续").
				Negative("取消").
				Value(&confirm),
		),
	)
	if err := skillSetupRunForm(form); err != nil {
		return false, fmt.Errorf("确认中止: %w", err)
	}
	return confirm, nil
}

// mutualExclusionVictims returns the paths that should be removed before
// installing into dest under the given mode, to prevent leftover files from
// the opposite mode from co-existing.
//
//   - mono dest is <agent-home>/dws  → multi 残留是 <agent-home>/dingtalk-*
//   - multi dest is <agent-home>     → mono 残留是 <agent-home>/dws
//
// A scan failure (e.g. unreadable agent home) is returned as a non-nil error
// so callers can surface a warning instead of silently skipping cleanup.
func mutualExclusionVictims(dest, mode string) ([]string, error) {
	switch mode {
	case skillSetupModeMono:
		// dest = <agent-home>/dws → agent-home = parent
		agentHome := filepath.Dir(dest)
		entries, err := skillSetupReadDir(agentHome)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, nil
			}
			return nil, fmt.Errorf("扫描 multi 残留失败 %s: %w", agentHome, err)
		}
		var victims []string
		for _, e := range entries {
			if e.IsDir() && isDWSMultiSkillName(e.Name()) {
				victims = append(victims, filepath.Join(agentHome, e.Name()))
			}
		}
		sort.Strings(victims)
		return victims, nil
	case skillSetupModeMulti:
		// dest = <agent-home> → mono 残留是 dest/dws
		monoPath := filepath.Join(dest, "dws")
		if _, err := skillSetupStat(monoPath); err == nil {
			return []string{monoPath}, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("检查 mono 残留失败 %s: %w", monoPath, err)
		}
		return nil, nil
	}
	return nil, nil
}

// cleanupMutualExclusion backs up + removes the opposite-mode
// leftovers. Removals are reversible: each victim is moved to
// ~/.dws/skill-backups/<stamp>/ first (skillSetupBackupAndRemove), and a
// victim whose backup fails is left in place with a warning instead of being
// destroyed. A failure is returned so the caller skips the complete Agent
// target and never installs both layouts together.
func cleanupMutualExclusion(dest, mode string, out, errOut io.Writer) error {
	victims, scanErr := mutualExclusionVictims(dest, mode)
	if scanErr != nil {
		fmt.Fprintf(errOut, "  ⚠️  互斥清理扫描失败（跳过整个 Agent 目标） %s: %v\n", dest, scanErr)
		return scanErr
	}
	if len(victims) == 0 {
		return nil
	}
	home, homeErr := skillSetupUserHomeDir()
	if homeErr != nil {
		for _, victim := range victims {
			fmt.Fprintf(errOut, "  ⚠️  无法解析 HOME，跳过删除（保留） %s: %v\n", victim, homeErr)
		}
		return homeErr
	}
	for _, victim := range victims {
		backup, err := skillSetupBackupAndRemove(home, victim)
		if err != nil {
			fmt.Fprintf(errOut, "  ⚠️  互斥清理失败（保留原目录，跳过整个 Agent 目标） %s: %v\n", victim, err)
			return err
		}
		fmt.Fprintf(out, "  × 已备份并清理对面模式残留 %s → %s\n", victim, backup)
	}
	return nil
}

func installSkillToHomes(src string, dests []string, out, errOut io.Writer) (installed, skipped int, err error) {
	plan, planErr := buildSkillSetupPlan(skillSetupModeMono, src, dests, nil, false)
	if planErr != nil {
		return 0, len(dests), planErr
	}
	return executeSkillSetupPlan(plan, out, errOut)
}

// installMultiSkillToHomes installs each subdir of src (dingtalk-*) into
// dest as a sibling skill directory. installed/skipped is counted per
// (agent-home × sub-skill) pair so the user sees granular progress.
//
// filtered mirrors whether runSkillSetup saw -s/--skill or -x/--exclude:
// a filtered install stays additive and never touches siblings outside the
// requested set; a full (unfiltered) install additionally removes stale
// dingtalk-* / dws-shared directories that are no longer in the bundle,
// matching install.sh / install.ps1 / install.js / upgrade paths.
func installMultiSkillToHomes(src string, skillNames []string, dests []string, out, errOut io.Writer, filtered bool) (installed, skipped int, err error) {
	plan, planErr := buildSkillSetupPlan(skillSetupModeMulti, src, dests, skillNames, filtered)
	if planErr != nil {
		return 0, len(skillNames) * len(dests), planErr
	}
	return executeSkillSetupPlan(plan, out, errOut)
}

func executeSkillSetupPlan(plan *skillSetupPlan, out, errOut io.Writer) (installed, skipped int, err error) {
	home, homeErr := skillSetupUserHomeDir()
	perTarget := 1
	if plan.Mode == skillSetupModeMulti {
		perTarget = len(plan.MultiSkillNames)
	}
	for _, target := range plan.Targets {
		backupFailed := false
		for _, planned := range target.Backups {
			if homeErr != nil {
				if plan.Mode == skillSetupModeMono {
					fmt.Fprintf(errOut, "  ✗ 无法解析 HOME，跳过刷新（保留原目录） %s: %v\n", target.Destination, homeErr)
				} else {
					fmt.Fprintf(errOut, "  ✗ 无法解析 HOME，跳过整个 Agent 目标 %s: %v\n", target.Destination, homeErr)
				}
				backupFailed = true
				break
			}
			backup, backupErr := skillSetupBackupAndRemove(home, planned.Path)
			if backupErr != nil {
				switch planned.Reason {
				case skillSetupBackupMutual:
					fmt.Fprintf(errOut, "  ✗ 互斥清理失败，跳过整个 Agent 目标 %s: %v\n", target.Destination, backupErr)
				case skillSetupBackupStale:
					fmt.Fprintf(errOut, "  ✗ 过期 Skill 备份失败，跳过整个 Agent 目标 %s: %v\n", target.Destination, backupErr)
				default:
					fmt.Fprintf(errOut, "  ✗ 备份失败，跳过整个 Agent 目标 %s: %v\n", target.Destination, backupErr)
				}
				backupFailed = true
				break
			}
			switch planned.Reason {
			case skillSetupBackupMutual:
				fmt.Fprintf(out, "  × 已备份并清理对面模式残留 %s → %s\n", planned.Path, backup)
			case skillSetupBackupStale:
				fmt.Fprintf(out, "  × 已备份并清理过期 skill %s → %s\n", planned.Path, backup)
			default:
				fmt.Fprintf(out, "  × 已备份并移除同名 Skill %s → %s\n", planned.Path, backup)
			}
		}
		if backupFailed {
			skipped += perTarget
			continue
		}
		if plan.Mode == skillSetupModeMono {
			if mkdirErr := skillSetupMkdirAll(filepath.Dir(target.Destination), 0o755); mkdirErr != nil {
				fmt.Fprintf(errOut, "  ✗ 父目录创建失败 %s: %v\n", target.Destination, mkdirErr)
				skipped++
				continue
			}
			if copyErr := skillSetupCopyDir(plan.Source, target.Destination); copyErr != nil {
				fmt.Fprintf(errOut, "  ✗ 拷贝失败 %s: %v\n", target.Destination, copyErr)
				skipped++
				continue
			}
			fmt.Fprintf(out, "  ✓ %s\n", target.Destination)
			installed++
			continue
		}
		if mkdirErr := skillSetupMkdirAll(target.Destination, 0o755); mkdirErr != nil {
			fmt.Fprintf(errOut, "  ✗ Agent 目录创建失败 %s: %v\n", target.Destination, mkdirErr)
			skipped += perTarget
			continue
		}
		for _, name := range plan.MultiSkillNames {
			subDest := filepath.Join(target.Destination, name)
			if copyErr := skillSetupCopyDir(filepath.Join(plan.Source, name), subDest); copyErr != nil {
				fmt.Fprintf(errOut, "  ✗ 拷贝失败 %s: %v\n", subDest, copyErr)
				skipped++
				continue
			}
			fmt.Fprintf(out, "  ✓ %s\n", subDest)
			installed++
		}
	}
	return installed, skipped, nil
}

// staleMultiSkillVictims lists the dingtalk-* / dws-shared directories under
// dest that a full (unfiltered) multi install would delete because they are
// not part of the bundle.
func staleMultiSkillVictims(dest string, keep []string) []string {
	victims, _ := staleMultiSkillVictimsWithError(dest, keep)
	return victims
}

func staleMultiSkillVictimsWithError(dest string, keep []string) ([]string, error) {
	entries, err := skillSetupReadDir(dest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("扫描过期 Skill 失败 %s: %w", dest, err)
	}
	keepSet := make(map[string]bool, len(keep))
	for _, n := range keep {
		keepSet[n] = true
	}
	var victims []string
	for _, e := range entries {
		if !e.IsDir() || keepSet[e.Name()] {
			continue
		}
		if !isDWSMultiSkillName(e.Name()) {
			continue
		}
		victims = append(victims, filepath.Join(dest, e.Name()))
	}
	sort.Strings(victims)
	return victims, nil
}

// removeStaleMultiSkills backs up + removes dingtalk-* / dws-shared
// directories under dest that are not part of the current bundle. Removal is
// reversible: each stale directory is moved to ~/.dws/skill-backups/<stamp>/
// first, and a backup failure keeps the directory in place with a warning.
// A scan or backup failure is returned so callers do not write a new bundle
// into a partially reconciled Agent target.
func removeStaleMultiSkills(dest string, keep []string, out, errOut io.Writer) error {
	entries, err := skillSetupReadDir(dest)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		fmt.Fprintf(errOut, "  ⚠️  过期 skill 扫描失败（跳过整个 Agent 目标） %s: %v\n", dest, err)
		return err
	}
	keepSet := make(map[string]bool, len(keep))
	for _, n := range keep {
		keepSet[n] = true
	}
	var stales []string
	for _, e := range entries {
		if !e.IsDir() || keepSet[e.Name()] {
			continue
		}
		if !isDWSMultiSkillName(e.Name()) {
			continue
		}
		stales = append(stales, filepath.Join(dest, e.Name()))
	}
	if len(stales) == 0 {
		return nil
	}
	home, homeErr := skillSetupUserHomeDir()
	if homeErr != nil {
		for _, stale := range stales {
			fmt.Fprintf(errOut, "  ⚠️  无法解析 HOME，跳过删除（保留） %s: %v\n", stale, homeErr)
		}
		return homeErr
	}
	for _, stale := range stales {
		backup, err := skillSetupBackupAndRemove(home, stale)
		if err != nil {
			fmt.Fprintf(errOut, "  ⚠️  过期 skill 清理失败（保留原目录，跳过整个 Agent 目标） %s: %v\n", stale, err)
			return err
		}
		fmt.Fprintf(out, "  × 已备份并清理过期 skill %s → %s\n", stale, backup)
	}
	return nil
}

func copyDir(src, dst string) error {
	return skillSetupWalk(src, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := skillSetupRel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if info.IsDir() {
			return skillSetupMkdirAll(target, info.Mode())
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// resolve symlink target and copy the underlying file
			resolved, err := skillSetupReadlink(path)
			if err != nil {
				return err
			}
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(path), resolved)
			}
			return copyFileContent(resolved, target, info.Mode())
		}
		return copyFileContent(path, target, info.Mode())
	})
}

func copyFileContent(src, dst string, mode os.FileMode) error {
	if err := skillSetupMkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := skillSetupOpen(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := skillSetupOpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode&os.ModePerm)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = skillSetupCopy(out, in)
	return err
}

func isInteractiveTerminal() bool {
	return isCharDevice(os.Stdin) && isCharDevice(os.Stdout) && isCharDevice(os.Stderr)
}

func isCharDevice(file *os.File) bool {
	if file == nil {
		return false
	}
	fi, err := file.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
