package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	skillSetupResolveMode    = resolveSkillSetupMode
	skillSetupResolveSource  = resolveSkillSetupSourceOrEmbedded
	skillSetupResolveTargets = resolveSkillSetupTargets
	skillSetupListMulti      = listMultiSkillNames
	skillSetupFilterMulti    = filterMultiSkillNames
	skillSetupConfirm        = confirmSkillSetup
	skillSetupInstallMono    = installSkillToHomes
	skillSetupInstallMulti   = installMultiSkillToHomes
	skillSetupCopyDir        = copyDir
	skillSetupRunForm        = (*huh.Form).Run
	skillSetupInteractive    = isInteractiveTerminal
	skillSetupReadDir        = os.ReadDir
	skillSetupStat           = os.Stat
	skillSetupExecutable     = os.Executable
	skillSetupGetwd          = os.Getwd
	skillSetupUserHomeDir    = os.UserHomeDir
	skillSetupRemoveAll      = os.RemoveAll
	skillSetupMkdirAll       = os.MkdirAll
	skillSetupWalk           = filepath.Walk
	skillSetupRel            = filepath.Rel
	skillSetupReadlink       = os.Readlink
	skillSetupOpen           = os.Open
	skillSetupOpenFile       = os.OpenFile
	skillSetupCopy           = io.Copy
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

不带 --mode 时进入交互式询问；不带 --target 时铺到所有检测到的 Agent 目录。
skill 源默认取二进制内嵌的版本（升级二进制即升级 skill）；--source / DWS_SKILL_SOURCE 可显式覆盖。`,
		Example: `  dws skill setup                                       # 交互式
  dws skill setup --mode mono --yes                     # 非交互装 mono
  dws skill setup --mode multi --target claude          # multi 全装到 ~/.claude/skills/
  dws skill setup --mode multi -s aitable -s calendar   # 只装 aitable + calendar
  dws skill setup --mode multi -x live -x devdoc        # 安装除 live、devdoc 外的其余 skill
  dws skill setup --source /path/to/repo                # 显式指定 skill 源`,
		DisableAutoGenTag: true,
		RunE:              runSkillSetup,
	}
	cmd.Flags().String("mode", "", "skill 模式：mono | multi（不指定则交互询问）")
	cmd.Flags().String("target", "all", "目标 Agent：all | "+supportedTargets())
	cmd.Flags().String("source", "", "skill 源目录（默认使用二进制内嵌的 skill 源，与当前版本一致）")
	cmd.Flags().Bool("yes", false, "跳过所有确认提示")
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
	var multiSkillNames []string
	if mode == skillSetupModeMulti {
		allMultiSkillNames, listErr := skillSetupListMulti(skillSrc)
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
		// dws-shared carries the global rules every product skill declares as a
		// PREREQUISITE; it must ship even when --skill / --exclude narrows the set.
		multiSkillNames = ensureMandatorySharedSkill(filtered, allMultiSkillNames)
	}

	// --dry-run：仅预览将安装的内容与目标目录，不写入任何文件、不弹确认。
	if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
		fmt.Fprintf(out, "[DRY-RUN] 预览（不写入任何文件）：mode=%s，来源 %s\n", mode, skillSrc)
		fmt.Fprintln(out, "将安装到：")
		for _, d := range dests {
			fmt.Fprintf(out, "  - %s\n", d)
		}
		if mode == skillSetupModeMulti && len(multiSkillNames) > 0 {
			fmt.Fprintf(out, "子 skill：%s\n", strings.Join(multiSkillNames, ", "))
		}
		return nil
	}

	// filtered 决定 multi 安装的清理语义：带 -s/--skill 或 -x/--exclude
	// 时保持 additive（不动未列出的 sibling）；全量安装与 install.sh /
	// install.js 对齐，清掉不在 bundle 内的过期 dingtalk-* / dws-shared。
	filtered := len(includeRaw) > 0 || len(excludeRaw) > 0

	if !autoYes {
		ok, err := skillSetupConfirm(out, mode, skillSrc, dests, multiSkillNames, filtered)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(out, "已取消。")
			return nil
		}
	}

	var installed, skipped int
	switch mode {
	case skillSetupModeMono:
		installed, skipped, err = skillSetupInstallMono(skillSrc, dests, out, errOut)
	case skillSetupModeMulti:
		installed, skipped, err = skillSetupInstallMulti(skillSrc, multiSkillNames, dests, out, errOut, filtered)
	default:
		return fmt.Errorf("内部错误：未知 mode %q", mode)
	}
	if err != nil {
		return err
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
// dws-shared that was never installed.
const multiSharedSkill = "dws-shared"

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

func confirmSkillSetup(out io.Writer, mode, src string, dests []string, multiSkillNames []string, filtered bool) (bool, error) {
	fmt.Fprintf(out, "\n📦 将安装 skill：\n  mode: %s\n  source: %s\n", mode, src)
	if mode == skillSetupModeMulti {
		fmt.Fprintf(out, "  将装 %d 个独立 skill（按子目录平铺到 <agent-home>/<skill-name>/）：\n", len(multiSkillNames))
		for _, n := range multiSkillNames {
			fmt.Fprintf(out, "    · %s\n", n)
		}
	}
	fmt.Fprintln(out, "  destinations:")
	for _, d := range dests {
		fmt.Fprintf(out, "    - %s\n", d)
	}
	// 列出互斥清理：装 mode 前要把对面 mode 的残留删掉
	fmt.Fprintln(out, "  互斥清理（确认后才执行）：")
	for _, d := range dests {
		victims, _ := mutualExclusionVictims(d, mode) // 预览只读，扫描失败不阻塞确认
		for _, victim := range victims {
			fmt.Fprintf(out, "    × 将删除 %s\n", victim)
		}
	}
	// 全量 multi 安装还会清掉不在 bundle 内的过期 dingtalk-* / dws-shared
	// （removeStaleMultiSkills）；这些删除同样必须先进入确认预览。带
	// -s/-x 的 filtered 安装是 additive 语义，不会动未列出的 sibling。
	if mode == skillSetupModeMulti && !filtered {
		for _, d := range dests {
			for _, victim := range staleMultiSkillVictims(d, multiSkillNames) {
				fmt.Fprintf(out, "    × 将删除过期 skill %s\n", victim)
			}
		}
	}

	if !skillSetupInteractive() {
		return true, nil
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
			if e.IsDir() && (strings.HasPrefix(e.Name(), multiSkillPrefix) || e.Name() == multiSharedSkill) {
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
		}
		return nil, nil
	}
	return nil, nil
}

// cleanupMutualExclusion best-effort removes the opposite-mode leftovers.
// Failures — including a failed victim scan — emit a warning to errOut but
// never abort the install.
func cleanupMutualExclusion(dest, mode string, out, errOut io.Writer) {
	victims, scanErr := mutualExclusionVictims(dest, mode)
	if scanErr != nil {
		fmt.Fprintf(errOut, "  ⚠️  互斥清理扫描失败（继续安装） %s: %v\n", dest, scanErr)
	}
	for _, victim := range victims {
		if err := skillSetupRemoveAll(victim); err != nil {
			fmt.Fprintf(errOut, "  ⚠️  互斥清理失败（继续安装） %s: %v\n", victim, err)
			continue
		}
		fmt.Fprintf(out, "  × 已清理对面模式残留 %s\n", victim)
	}
}

func installSkillToHomes(src string, dests []string, out, errOut io.Writer) (installed, skipped int, err error) {
	sort.Strings(dests)
	for _, dest := range dests {
		// 先做互斥清理：装 mono 前先把同级 dingtalk-* 子目录全部干掉
		cleanupMutualExclusion(dest, skillSetupModeMono, out, errOut)

		if err := skillSetupRemoveAll(dest); err != nil {
			fmt.Fprintf(errOut, "  ✗ 清理失败 %s: %v\n", dest, err)
			skipped++
			continue
		}
		if err := skillSetupMkdirAll(filepath.Dir(dest), 0o755); err != nil {
			fmt.Fprintf(errOut, "  ✗ 父目录创建失败 %s: %v\n", dest, err)
			skipped++
			continue
		}
		if err := skillSetupCopyDir(src, dest); err != nil {
			fmt.Fprintf(errOut, "  ✗ 拷贝失败 %s: %v\n", dest, err)
			skipped++
			continue
		}
		fmt.Fprintf(out, "  ✓ %s\n", dest)
		installed++
	}
	return installed, skipped, nil
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
	sort.Strings(dests)
	for _, dest := range dests {
		// 互斥清理：装 multi 前先把 dest/dws/ 整个删除（mono 残留）
		cleanupMutualExclusion(dest, skillSetupModeMulti, out, errOut)

		if err := skillSetupMkdirAll(dest, 0o755); err != nil {
			fmt.Fprintf(errOut, "  ✗ Agent 目录创建失败 %s: %v\n", dest, err)
			skipped += len(skillNames)
			continue
		}

		if !filtered {
			removeStaleMultiSkills(dest, skillNames, out, errOut)
		}

		for _, name := range skillNames {
			subSrc := filepath.Join(src, name)
			subDest := filepath.Join(dest, name)
			if err := skillSetupRemoveAll(subDest); err != nil {
				fmt.Fprintf(errOut, "  ✗ 清理失败 %s: %v\n", subDest, err)
				skipped++
				continue
			}
			if err := skillSetupCopyDir(subSrc, subDest); err != nil {
				fmt.Fprintf(errOut, "  ✗ 拷贝失败 %s: %v\n", subDest, err)
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
// not part of the bundle. It is the read-only preview companion of
// removeStaleMultiSkills; scan failures degrade to a nil list so the
// confirmation prompt is never blocked by an unreadable agent home.
func staleMultiSkillVictims(dest string, keep []string) []string {
	entries, err := skillSetupReadDir(dest)
	if err != nil {
		return nil
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
		if !strings.HasPrefix(e.Name(), multiSkillPrefix) && e.Name() != multiSharedSkill {
			continue
		}
		victims = append(victims, filepath.Join(dest, e.Name()))
	}
	sort.Strings(victims)
	return victims
}

// removeStaleMultiSkills deletes dingtalk-* / dws-shared directories under
// dest that are not part of the current bundle. Best-effort: scan/removal
// failures warn on errOut and never abort the install.
func removeStaleMultiSkills(dest string, keep []string, out, errOut io.Writer) {
	entries, err := skillSetupReadDir(dest)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(errOut, "  ⚠️  过期 skill 扫描失败（继续安装） %s: %v\n", dest, err)
		}
		return
	}
	keepSet := make(map[string]bool, len(keep))
	for _, n := range keep {
		keepSet[n] = true
	}
	for _, e := range entries {
		if !e.IsDir() || keepSet[e.Name()] {
			continue
		}
		if !strings.HasPrefix(e.Name(), multiSkillPrefix) && e.Name() != multiSharedSkill {
			continue
		}
		stale := filepath.Join(dest, e.Name())
		if err := skillSetupRemoveAll(stale); err != nil {
			fmt.Fprintf(errOut, "  ⚠️  过期 skill 清理失败（继续安装） %s: %v\n", stale, err)
			continue
		}
		fmt.Fprintf(out, "  × 已清理过期 skill %s\n", stale)
	}
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
