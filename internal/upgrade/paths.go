// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package upgrade

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Permission constants following Unix best practices.
const (
	dirPermSecure  os.FileMode = 0o700
	dirPermShared  os.FileMode = 0o755
	filePermBinary os.FileMode = 0o755
	filePermConfig os.FileMode = 0o644
)

// knownSkillDirs lists all known Agent skill directories (relative to $HOME).
// Kept in sync with:
//   - build/npm/install.js                                  AGENT_DIRS
//   - scripts/install.sh                                    for-in list
//   - scripts/install.ps1                                   $AgentDirs
//   - scripts/install-skills.sh                             for-in list
//   - build/homebrew.rb.tmpl                                targets
//   - test/scripts/package_script_test.go                   expectedPackagedSkillTargets
//   - scripts/release/verify-package-managers.sh            HOME_AGENT_PARENTS / HOME_SKILL_TARGETS
//
// The first entry (.agents/skills) is always updated; subsequent entries are
// only updated when their parent directory already exists.
var knownSkillDirs = []string{
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

var (
	upgradeUserHomeDir  = os.UserHomeDir
	upgradeExecutable   = os.Executable
	upgradeEvalSymlinks = filepath.EvalSymlinks
	upgradeCopyDir      = copyDir
	upgradeEnsureDir    = ensureDir
	upgradeRemoveAll    = os.RemoveAll
	upgradeMkdirAll     = os.MkdirAll
	upgradeReadDir      = os.ReadDir
	upgradeStat         = os.Stat
	upgradeBackupStamp  = func() string { return time.Now().UTC().Format("20060102-150405") }
)

// skillBackupSubdir is the user-level directory where skill directories are
// preserved before a layout-changing install/upgrade removes them. Non-
// interactive flows (install scripts, npm postinstall, `dws upgrade`) cannot
// ask for confirmation, so deletions must stay reversible instead.
const skillBackupSubdir = ".dws/skill-backups"

// backupAndRemoveSkillDir moves dir into <homeDir>/.dws/skill-backups/
// <stamp>/<rel> instead of destroying it, and returns the backup path. It is
// fail-safe: a directory that cannot be backed up is NOT removed and the
// error is returned so the caller can surface it (and never install the
// opposite layout next to it silently). Missing paths and regular files are
// no-ops ("", nil).
func backupAndRemoveSkillDir(homeDir, dir string) (string, error) {
	info, err := upgradeStat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("检查技能目录失败 %s: %w", dir, err)
	}
	if !info.IsDir() {
		return "", nil
	}
	rel, relErr := filepath.Rel(homeDir, dir)
	if relErr != nil || rel == "." || strings.HasPrefix(rel, "..") {
		rel = filepath.Base(dir)
	}
	name := strings.NewReplacer(string(filepath.Separator), "-", "/", "-").Replace(rel)
	stamp := upgradeBackupStamp()
	backupRoot := filepath.Join(homeDir, skillBackupSubdir, stamp)
	target := filepath.Join(backupRoot, name)
	for i := 1; ; i++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		backupRoot = filepath.Join(homeDir, skillBackupSubdir, fmt.Sprintf("%s-%d", stamp, i))
		target = filepath.Join(backupRoot, name)
		if i > 1000 {
			return "", fmt.Errorf("备份目录冲突无法解决: %s", target)
		}
	}
	if err := upgradeMkdirAll(backupRoot, dirPermShared); err != nil {
		return "", fmt.Errorf("创建备份目录失败 %s: %w", backupRoot, err)
	}
	if err := upgradeRename(dir, target); err != nil {
		return "", fmt.Errorf("备份技能目录失败 %s: %w", dir, err)
	}
	// Keep the backup directory bounded; a prune failure must not fail the
	// install (the backup itself succeeded).
	_ = pruneSkillBackups(homeDir)
	return target, nil
}

// BackupAndRemoveSkillDir is the exported wrapper over
// backupAndRemoveSkillDir for callers outside the upgrade package (the
// skill-setup channel in internal/app).
func BackupAndRemoveSkillDir(homeDir, dir string) (string, error) {
	return backupAndRemoveSkillDir(homeDir, dir)
}

// skillDirBlacklist contains parent directories whose skills are managed by
// external mechanisms (e.g. IDE extensions) and must NOT be touched by upgrade.
var skillDirBlacklist = []string{
	".real",
}

// SkillDirStatus describes the installation outcome for a single skill directory.
type SkillDirStatus int

const (
	SkillDirOK          SkillDirStatus = iota // successfully installed
	SkillDirSkipped                           // agent not detected, directory skipped
	SkillDirBlacklisted                       // blacklisted, never touched
	SkillDirFailed                            // installation attempted but failed
)

// SkillDirResult holds the per-directory install result.
type SkillDirResult struct {
	Dir    string         // destination directory (e.g. ~/.claude/skills/dws)
	Status SkillDirStatus // outcome
	Err    error          // non-nil when Status == SkillDirFailed
}

// SkillUpgradeResult aggregates the outcome of an UpgradeSkillLocations call.
type SkillUpgradeResult struct {
	Results []SkillDirResult
}

// Succeeded returns directories that were successfully updated.
func (r *SkillUpgradeResult) Succeeded() []SkillDirResult {
	var out []SkillDirResult
	for _, d := range r.Results {
		if d.Status == SkillDirOK {
			out = append(out, d)
		}
	}
	return out
}

// Failed returns directories where installation was attempted but failed.
func (r *SkillUpgradeResult) Failed() []SkillDirResult {
	var out []SkillDirResult
	for _, d := range r.Results {
		if d.Status == SkillDirFailed {
			out = append(out, d)
		}
	}
	return out
}

// UpgradeSkillLocations refreshes skills from extractedDir into agent homes.
// extractedDir may be a multi-skill bundle root (subdirectories each containing
// SKILL.md) or a legacy mono root (SKILL.md at its top level). Callers that
// resolve a release zip usually pass LocateSkillsRoot's result (multi/
// preferred when present).
//
// Package-driven layout (owner decision 2026-08-05 — no disk sticky):
//   - release zip has multi/ → ALWAYS refresh multi (flat product skills +
//     dws-shared), removing mono leftover dws/ and stale dingtalk-*/dws-shared.
//     Existing mono installs are one-shot migrated to multi on upgrade.
//   - legacy zip with no multi tree → mono refresh path (unchanged fallback)
//
// This is not a runtime mode-switch product. Fresh install still defaults to
// multi with opt-in mono via `dws skill setup --mode mono`; subsequent
// upgrades force multi when the package ships it.
//
// Strategy (matches npm install.js installSkillsToHomes):
//   - ~/.agents/skills/ is ALWAYS updated (primary install location)
//   - Other agent dirs (claude, cursor, ...) are updated only when the parent
//     directory exists (e.g. ~/.claude/ exists => user has Claude)
//   - ~/.real/ and other blacklisted paths are NEVER touched
//   - If no location was updated at all, fall back to ~/.agents/skills/
//
// Opposite-mode leftovers are backed up to ~/.dws/skill-backups/ and then
// removed so mono and multi never co-exist after an upgrade; a leftover that
// cannot be backed up is never removed and fails that home. Same-name bundle
// skills are refreshed in place (verified DWS-managed overwrite). Caches
// under ~/.dws/skills/{multi,mono} are refreshed best-effort.
func UpgradeSkillLocations(extractedDir string) (*SkillUpgradeResult, error) {
	homeDir, err := upgradeUserHomeDir()
	if err != nil {
		return nil, err
	}

	multiRoot, skills := resolveMultiBundle(extractedDir)
	if len(skills) > 0 {
		return upgradeMultiSkillLocations(homeDir, multiRoot, skills)
	}
	monoSrc := resolveMonoSkillSrc(extractedDir)
	if monoSrc != "" {
		return upgradeMonoSkillLocations(homeDir, monoSrc)
	}
	return nil, fmt.Errorf("升级包中找不到可安装的 skill 源")
}

// skillBackupKeep limits ~/.dws/skill-backups/ growth: only the newest
// backups are kept.
const skillBackupKeep = 5

// pruneSkillBackups removes the oldest backup directories when more than
// skillBackupKeep remain. Best-effort: a removal failure never aborts, but
// pruning failures are reported so callers can warn the user.
func pruneSkillBackups(homeDir string) error {
	root := filepath.Join(homeDir, skillBackupSubdir)
	entries, err := upgradeReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var firstErr error
	for len(names) > skillBackupKeep {
		old := filepath.Join(root, names[0])
		names = names[1:]
		if err := upgradeRemoveAll(old); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// resolveMultiBundle returns the multi skill root and its skill names when
// extractedDir is itself a bundle, or when a multi/ child of the parent
// extract root carries one (LocateSkillsRoot already prefers that child).
func resolveMultiBundle(extractedDir string) (string, []string) {
	if skills := bundleSkillNames(extractedDir); len(skills) > 0 {
		return extractedDir, skills
	}
	child := filepath.Join(extractedDir, "multi")
	if skills := bundleSkillNames(child); len(skills) > 0 {
		return child, skills
	}
	return "", nil
}

// resolveMonoSkillSrc finds a mono skill tree for the legacy mono-only
// package fallback: the path itself, a sibling mono/ next to a multi root,
// or the extract-root SKILL.md copy that release zips still ship.
func resolveMonoSkillSrc(extractedDir string) string {
	if skillTreeHasRoot(extractedDir) {
		return extractedDir
	}
	sibling := filepath.Join(filepath.Dir(extractedDir), "mono")
	if skillTreeHasRoot(sibling) {
		return sibling
	}
	parent := filepath.Dir(extractedDir)
	if skillTreeHasRoot(parent) {
		return parent
	}
	child := filepath.Join(extractedDir, "mono")
	if skillTreeHasRoot(child) {
		return child
	}
	return ""
}

// upgradeMonoSkillLocations is the legacy mono behavior: one dws/ directory
// per agent home.
func upgradeMonoSkillLocations(homeDir, skillSrc string) (*SkillUpgradeResult, error) {
	result := &SkillUpgradeResult{}

	for i, agentDir := range knownSkillDirs {
		destDir := filepath.Join(homeDir, agentDir, "dws")

		if isBlacklisted(agentDir) {
			result.Results = append(result.Results, SkillDirResult{Dir: destDir, Status: SkillDirBlacklisted})
			continue
		}

		if i > 0 {
			parentGate := filepath.Dir(filepath.Join(homeDir, agentDir))
			if _, err := os.Stat(parentGate); os.IsNotExist(err) {
				result.Results = append(result.Results, SkillDirResult{Dir: destDir, Status: SkillDirSkipped})
				continue
			}
		}

		// Mutual exclusion: installing mono backs up + removes multi leftovers.
		// A base directory that exists but cannot be read fails the home
		// instead of silently installing mono alongside multi.
		if err := cleanupMultiLeftovers(homeDir, filepath.Join(homeDir, agentDir)); err != nil {
			result.Results = append(result.Results, SkillDirResult{Dir: destDir, Status: SkillDirFailed, Err: err})
			continue
		}

		// Refresh the existing mono directory reversibly: back it up instead of
		// hard-deleting, since it may carry user modifications.
		if _, err := backupAndRemoveSkillDir(homeDir, destDir); err != nil {
			result.Results = append(result.Results, SkillDirResult{Dir: destDir, Status: SkillDirFailed, Err: err})
			continue
		}
		if err := upgradeCopyDir(skillSrc, destDir); err != nil {
			result.Results = append(result.Results, SkillDirResult{Dir: destDir, Status: SkillDirFailed, Err: err})
			continue
		}
		result.Results = append(result.Results, SkillDirResult{Dir: destDir, Status: SkillDirOK})
	}

	// Fallback: if nothing succeeded, force the primary location. The multi
	// leftovers under the primary base are the usual reason the primary
	// install failed, so clean them first — failing loud like the multi
	// fallback — instead of letting mono and multi co-exist marked OK.
	if len(result.Succeeded()) == 0 {
		destBase := filepath.Join(homeDir, ".agents", "skills")
		os.MkdirAll(destBase, dirPermShared)
		if err := cleanupMultiLeftovers(homeDir, destBase); err != nil {
			return result, fmt.Errorf("所有技能目录安装失败，回退到主目录清理残留也失败: %w", err)
		}
		dest := filepath.Join(destBase, "dws")
		if _, err := backupAndRemoveSkillDir(homeDir, dest); err != nil {
			return result, fmt.Errorf("所有技能目录安装失败，回退到主目录备份残留失败: %w", err)
		}
		if err := upgradeCopyDir(skillSrc, dest); err != nil {
			return result, fmt.Errorf("所有技能目录安装失败，回退到主目录也失败: %w", err)
		}
		// Replace the earlier failed entry for this dir (if any) or append a new one
		replaced := false
		for idx, r := range result.Results {
			if r.Dir == dest {
				result.Results[idx] = SkillDirResult{Dir: dest, Status: SkillDirOK}
				replaced = true
				break
			}
		}
		if !replaced {
			result.Results = append(result.Results, SkillDirResult{Dir: dest, Status: SkillDirOK})
		}
	}

	// Best-effort: refresh the user-level mono cache so that
	// `dws skill setup --mode mono` fallbacks stay on the upgraded version
	// (symmetric with the multi cache refresh in upgradeMultiSkillLocations).
	refreshSkillCache(homeDir, "mono", skillSrc)

	return result, nil
}

// upgradeMultiSkillLocations installs every skill of the multi bundle into
// each agent home as sibling directories and backs up + removes the
// opposite-mode leftovers. A home is marked failed (and multi is NOT
// installed into it) when leftover backup/removal fails, so mono and multi
// never co-exist.
func upgradeMultiSkillLocations(homeDir, multiRoot string, skills []string) (*SkillUpgradeResult, error) {
	skillSet := make(map[string]bool, len(skills))
	for _, s := range skills {
		skillSet[s] = true
	}

	result := &SkillUpgradeResult{}

	for i, agentDir := range knownSkillDirs {
		destBase := filepath.Join(homeDir, agentDir)

		if isBlacklisted(agentDir) {
			result.Results = append(result.Results, SkillDirResult{Dir: destBase, Status: SkillDirBlacklisted})
			continue
		}

		if i > 0 {
			parentGate := filepath.Dir(destBase)
			if _, err := os.Stat(parentGate); os.IsNotExist(err) {
				result.Results = append(result.Results, SkillDirResult{Dir: destBase, Status: SkillDirSkipped})
				continue
			}
		}

		if err := cleanupOppositeModeLeftovers(homeDir, destBase, skillSet); err != nil {
			result.Results = append(result.Results, SkillDirResult{Dir: destBase, Status: SkillDirFailed, Err: err})
			continue
		}

		failed := false
		for _, name := range skills {
			subDest := filepath.Join(destBase, name)
			if _, err := backupAndRemoveSkillDir(homeDir, subDest); err != nil {
				result.Results = append(result.Results, SkillDirResult{Dir: destBase, Status: SkillDirFailed, Err: err})
				failed = true
				break
			}
			if err := upgradeCopyDir(filepath.Join(multiRoot, name), subDest); err != nil {
				result.Results = append(result.Results, SkillDirResult{Dir: destBase, Status: SkillDirFailed, Err: err})
				failed = true
				break
			}
		}
		if failed {
			continue
		}
		result.Results = append(result.Results, SkillDirResult{Dir: destBase, Status: SkillDirOK})
	}

	// Fallback: if nothing succeeded, force the primary location
	if len(result.Succeeded()) == 0 {
		destBase := filepath.Join(homeDir, ".agents", "skills")
		os.MkdirAll(destBase, dirPermShared)
		if err := cleanupOppositeModeLeftovers(homeDir, destBase, skillSet); err != nil {
			return result, fmt.Errorf("所有技能目录安装失败，回退到主目录清理残留也失败: %w", err)
		}
		for _, name := range skills {
			subDest := filepath.Join(destBase, name)
			if _, err := backupAndRemoveSkillDir(homeDir, subDest); err != nil {
				return result, fmt.Errorf("所有技能目录安装失败，回退到主目录备份残留也失败: %w", err)
			}
			if err := upgradeCopyDir(filepath.Join(multiRoot, name), subDest); err != nil {
				return result, fmt.Errorf("所有技能目录安装失败，回退到主目录也失败: %w", err)
			}
		}
		// Replace the earlier failed entry for this dir (if any) or append a new one
		replaced := false
		for idx, r := range result.Results {
			if r.Dir == destBase {
				result.Results[idx] = SkillDirResult{Dir: destBase, Status: SkillDirOK}
				replaced = true
				break
			}
		}
		if !replaced {
			result.Results = append(result.Results, SkillDirResult{Dir: destBase, Status: SkillDirOK})
		}
	}

	// Best-effort: refresh the user-level caches so that `dws skill setup`
	// fallbacks stay on the upgraded version. The release zip ships both
	// trees, so when the sibling mono/ tree is present the mono cache is
	// refreshed as well.
	refreshSkillCache(homeDir, "multi", multiRoot)
	if monoSrc := filepath.Join(filepath.Dir(multiRoot), "mono"); skillTreeHasRoot(monoSrc) {
		refreshSkillCache(homeDir, "mono", monoSrc)
	}

	return result, nil
}

// refreshSkillCache best-effort mirrors src into ~/.dws/skills/<name>/ so
// that `dws skill setup --mode <name>` can fall back to the cache and stay on
// the upgraded version. All errors are swallowed by design.
func refreshSkillCache(homeDir, name, src string) {
	cacheDir := filepath.Join(homeDir, ".dws", "skills", name)
	os.RemoveAll(cacheDir)
	if err := os.MkdirAll(filepath.Dir(cacheDir), dirPermShared); err == nil {
		_ = upgradeCopyDir(src, cacheDir)
	}
}

// skillTreeHasRoot reports whether dir carries a top-level SKILL.md.
func skillTreeHasRoot(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil && !info.IsDir()
}

// cleanupMultiLeftovers backs up + removes every multi-mode skill directory
// (dingtalk-* or dws-shared) inside one agent home before mono is installed.
// A missing base directory simply means no leftovers; any other read failure
// is reported so mono never silently co-exists with multi. Removal is
// reversible: each leftover is preserved under ~/.dws/skill-backups/ and a
// backup failure aborts the removal for that home.
func cleanupMultiLeftovers(homeDir, baseDir string) error {
	entries, err := upgradeReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取技能目录失败 %s: %w", baseDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() || !isMultiSkillDirName(e.Name()) {
			continue
		}
		stale := filepath.Join(baseDir, e.Name())
		if _, err := backupAndRemoveSkillDir(homeDir, stale); err != nil {
			return fmt.Errorf("备份并清理 multi 残留失败 %s: %w", stale, err)
		}
	}
	return nil
}

// cleanupOppositeModeLeftovers backs up + removes, inside one agent home, the
// legacy mono directory (dws/) and every multi skill directory (dingtalk-* or
// dws-shared) that is not part of the new bundle. The dingtalk- prefix and
// the dws-shared name are reserved for DWS product skills; market-installed
// skills do not use them (see skill_command.go). Removal is reversible: each
// directory is preserved under ~/.dws/skill-backups/ and a backup failure
// aborts the removal for that home.
func cleanupOppositeModeLeftovers(homeDir, destBase string, skillSet map[string]bool) error {
	if _, err := backupAndRemoveSkillDir(homeDir, filepath.Join(destBase, "dws")); err != nil {
		return fmt.Errorf("备份并清理 mono 残留失败 %s: %w", filepath.Join(destBase, "dws"), err)
	}
	entries, err := upgradeReadDir(destBase)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取技能目录失败 %s: %w", destBase, err)
	}
	for _, e := range entries {
		if !e.IsDir() || !isMultiSkillDirName(e.Name()) || skillSet[e.Name()] {
			continue
		}
		stale := filepath.Join(destBase, e.Name())
		if _, err := backupAndRemoveSkillDir(homeDir, stale); err != nil {
			return fmt.Errorf("备份并清理过期技能失败 %s: %w", stale, err)
		}
	}
	return nil
}

// isMultiSkillDirName reports whether name belongs to a DWS multi-mode skill
// directory (product skills use the dingtalk- prefix; dws-shared is the
// mandatory shared bundle).
func isMultiSkillDirName(name string) bool {
	return strings.HasPrefix(name, "dingtalk-") || name == "dws-shared"
}

// bundleSkillNames returns the sorted names of subdirectories of dir that
// contain a SKILL.md. It returns nil when dir itself carries a top-level
// SKILL.md (mono layout) so callers can distinguish the two layouts.
func bundleSkillNames(dir string) []string {
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "SKILL.md")); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// LocateSkillsRoot resolves the skill root inside an extracted dws-skills.zip,
// preferring the multi bundle ({extractDir}/multi) over the legacy mono
// layouts handled by LocateSkillMD.
func LocateSkillsRoot(extractDir string) string {
	multiRoot := filepath.Join(extractDir, "multi")
	if skills := bundleSkillNames(multiRoot); len(skills) > 0 {
		return multiRoot
	}
	return LocateSkillMD(extractDir)
}

// LocateSkillMD finds the directory containing SKILL.md in an extracted zip.
// It handles both flat layouts (SKILL.md at root) and nested layouts (dws/SKILL.md).
func LocateSkillMD(extractDir string) string {
	// Check nested: {extractDir}/dws/SKILL.md
	nested := filepath.Join(extractDir, "dws", "SKILL.md")
	if _, err := os.Stat(nested); err == nil {
		return filepath.Join(extractDir, "dws")
	}

	// Check flat: {extractDir}/SKILL.md
	flat := filepath.Join(extractDir, "SKILL.md")
	if _, err := os.Stat(flat); err == nil {
		return extractDir
	}

	return ""
}

// EnsureUpgradeDirectories creates the directories needed for upgrade operations.
func EnsureUpgradeDirectories() error {
	homeDir, err := upgradeUserHomeDir()
	if err != nil {
		return err
	}

	dirs := []struct {
		path string
		perm os.FileMode
	}{
		{filepath.Join(homeDir, ".dws"), dirPermSecure},
		{filepath.Join(homeDir, ".dws", "data"), dirPermSecure},
		{filepath.Join(homeDir, ".dws", "data", "backups"), dirPermSecure},
		{filepath.Join(homeDir, ".dws", "cache"), dirPermSecure},
		{filepath.Join(homeDir, ".dws", "cache", "downloads"), dirPermSecure},
	}

	for _, d := range dirs {
		if err := upgradeEnsureDir(d.path, d.perm); err != nil {
			return err
		}
	}
	return nil
}

// DownloadCacheDir returns the path for temporary downloads during upgrade.
func DownloadCacheDir() string {
	homeDir, _ := upgradeUserHomeDir()
	return filepath.Join(homeDir, ".dws", "cache", "downloads")
}

// CurrentBinaryPath returns the resolved path of the currently running binary.
func CurrentBinaryPath() (string, error) {
	exe, err := upgradeExecutable()
	if err != nil {
		return "", err
	}
	return upgradeEvalSymlinks(exe)
}

// BinaryName returns the platform-specific binary name.
func BinaryName() string {
	return binaryNameFor(runtime.GOOS)
}

func binaryNameFor(goos string) string {
	if goos == "windows" {
		return "dws.exe"
	}
	return "dws"
}

func isBlacklisted(agentDir string) bool {
	for _, bl := range skillDirBlacklist {
		// agentDir is like ".real/skills" — check if it starts with a blacklisted prefix
		if len(agentDir) >= len(bl) && agentDir[:len(bl)] == bl {
			next := len(bl)
			if next == len(agentDir) || agentDir[next] == '/' {
				return true
			}
		}
	}
	return false
}

func ensureDir(path string, perm os.FileMode) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return os.MkdirAll(path, perm)
	}
	if err != nil {
		return err
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != perm {
		if info.Mode().Perm()&^perm != 0 {
			return os.Chmod(path, perm)
		}
	}
	return nil
}
