// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

// withFakeHome swaps upgradeUserHomeDir to a temp dir for the test duration.
func withFakeHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	testseam.Swap(t, &upgradeUserHomeDir, func() (string, error) { return home, nil })
	return home
}

// writeMultiBundle creates a multi-skill bundle root with the given skills.
func writeMultiBundle(t *testing.T, root string, skills ...string) string {
	t.Helper()
	multi := filepath.Join(root, "multi")
	for _, name := range skills {
		dir := filepath.Join(multi, name)
		if err := os.MkdirAll(filepath.Join(dir, "references"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "references", "guide.md"), []byte("guide "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return multi
}

func TestLocateSkillsRootPrefersMulti(t *testing.T) {
	extract := t.TempDir()
	// Legacy mono copy at zip root plus the multi bundle: multi must win.
	os.WriteFile(filepath.Join(extract, "SKILL.md"), []byte("# mono"), 0o644)
	multiRoot := writeMultiBundle(t, extract, "dingtalk-chat", "dws-shared")

	got := LocateSkillsRoot(extract)
	if got != multiRoot {
		t.Errorf("LocateSkillsRoot() = %q, want multi root %q", got, multiRoot)
	}
}

func TestLocateSkillsRootFallsBackToMono(t *testing.T) {
	extract := t.TempDir()
	os.WriteFile(filepath.Join(extract, "SKILL.md"), []byte("# mono"), 0o644)

	got := LocateSkillsRoot(extract)
	if got != extract {
		t.Errorf("LocateSkillsRoot() = %q, want mono flat root %q", got, extract)
	}
}

func TestBundleSkillNamesLayouts(t *testing.T) {
	// Mono layout (top-level SKILL.md) is not a bundle.
	mono := t.TempDir()
	os.WriteFile(filepath.Join(mono, "SKILL.md"), []byte("# mono"), 0o644)
	os.MkdirAll(filepath.Join(mono, "references"), 0o755)
	if got := bundleSkillNames(mono); got != nil {
		t.Errorf("mono layout: bundleSkillNames() = %v, want nil", got)
	}

	// Multi bundle: sorted subdir names containing SKILL.md.
	extract := t.TempDir()
	multi := writeMultiBundle(t, extract, "dingtalk-wiki", "dingtalk-chat", "dws-shared")
	want := []string{"dingtalk-chat", "dingtalk-wiki", "dws-shared"}
	if got := bundleSkillNames(multi); !reflect.DeepEqual(got, want) {
		t.Errorf("multi layout: bundleSkillNames() = %v, want %v", got, want)
	}

	// Missing directory.
	if got := bundleSkillNames(filepath.Join(extract, "nope")); got != nil {
		t.Errorf("missing dir: bundleSkillNames() = %v, want nil", got)
	}
}

func TestUpgradeSkillLocationsMulti(t *testing.T) {
	home := withFakeHome(t)

	// .agents always installs; .claude installs (parent exists); .cursor skipped.
	agentsBase := filepath.Join(home, ".agents", "skills")
	claudeBase := filepath.Join(home, ".claude", "skills")
	for _, base := range []string{agentsBase, claudeBase} {
		if err := os.MkdirAll(base, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Pre-existing state in both homes: mono leftover, a stale dingtalk skill
	// no longer in the bundle, and a non-DWS dir that must survive.
	for _, base := range []string{agentsBase, claudeBase} {
		os.MkdirAll(filepath.Join(base, "dws"), 0o755)
		os.WriteFile(filepath.Join(base, "dws", "SKILL.md"), []byte("old mono"), 0o644)
		os.MkdirAll(filepath.Join(base, "dingtalk-old"), 0o755)
		os.WriteFile(filepath.Join(base, "dingtalk-old", "SKILL.md"), []byte("stale"), 0o644)
		os.MkdirAll(filepath.Join(base, "other-skill"), 0o755)
		os.WriteFile(filepath.Join(base, "other-skill", "SKILL.md"), []byte("not dws"), 0o644)
	}

	extract := t.TempDir()
	multiRoot := writeMultiBundle(t, extract, "dingtalk-chat", "dws-shared")
	// The release zip ships both trees; the mono sibling must refresh the
	// mono cache too, so --mode mono fallbacks stay on the upgraded version.
	if err := os.MkdirAll(filepath.Join(extract, "mono"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extract, "mono", "SKILL.md"), []byte("# mono sibling"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := UpgradeSkillLocations(multiRoot)
	if err != nil {
		t.Fatalf("UpgradeSkillLocations() error = %v", err)
	}
	if failed := result.Failed(); len(failed) != 0 {
		t.Fatalf("expected 0 failures, got %v", failed)
	}

	for _, base := range []string{agentsBase, claudeBase} {
		if _, err := os.Stat(filepath.Join(base, "dws")); !os.IsNotExist(err) {
			t.Errorf("mono leftover still present: %s", filepath.Join(base, "dws"))
		}
		if _, err := os.Stat(filepath.Join(base, "dingtalk-old")); !os.IsNotExist(err) {
			t.Errorf("stale skill still present: %s", filepath.Join(base, "dingtalk-old"))
		}
		if _, err := os.Stat(filepath.Join(base, "other-skill", "SKILL.md")); err != nil {
			t.Errorf("non-DWS dir should be preserved: %v", err)
		}
		for _, name := range []string{"dingtalk-chat", "dws-shared"} {
			if _, err := os.Stat(filepath.Join(base, name, "SKILL.md")); err != nil {
				t.Errorf("installed skill missing: %s/%s: %v", base, name, err)
			}
			if _, err := os.Stat(filepath.Join(base, name, "references", "guide.md")); err != nil {
				t.Errorf("installed skill references missing: %s/%s: %v", base, name, err)
			}
		}
	}

	// .cursor has no parent dir: must be skipped and untouched.
	cursorBase := filepath.Join(home, ".cursor", "skills")
	if _, err := os.Stat(cursorBase); !os.IsNotExist(err) {
		t.Errorf(".cursor should not be created, stat err = %v", err)
	}

	// Succeeded entries report the agent home base in multi mode.
	succeeded := result.Succeeded()
	if len(succeeded) != 2 {
		t.Fatalf("Succeeded() len = %d, want 2 (%v)", len(succeeded), result.Results)
	}
	wantDirs := map[string]bool{agentsBase: true, claudeBase: true}
	for _, d := range succeeded {
		if !wantDirs[d.Dir] {
			t.Errorf("unexpected succeeded dir %q", d.Dir)
		}
	}

	// Multi cache refreshed under the fake home.
	if _, err := os.Stat(filepath.Join(home, ".dws", "skills", "multi", "dingtalk-chat", "SKILL.md")); err != nil {
		t.Errorf("multi cache not refreshed: %v", err)
	}
	// Mono sibling tree refreshed the mono cache too.
	if _, err := os.Stat(filepath.Join(home, ".dws", "skills", "mono", "SKILL.md")); err != nil {
		t.Errorf("mono cache not refreshed from sibling mono tree: %v", err)
	}
}

func TestUpgradeSkillLocationsMultiFallbackPrimary(t *testing.T) {
	home := withFakeHome(t)
	// No agent parent dirs at all: only .agents (index 0) is attempted and the
	// primary fallback must also land there.
	extract := t.TempDir()
	multiRoot := writeMultiBundle(t, extract, "dingtalk-chat")

	result, err := UpgradeSkillLocations(multiRoot)
	if err != nil {
		t.Fatalf("UpgradeSkillLocations() error = %v", err)
	}
	if len(result.Failed()) != 0 {
		t.Fatalf("expected 0 failures, got %v", result.Failed())
	}
	primary := filepath.Join(home, ".agents", "skills", "dingtalk-chat", "SKILL.md")
	if _, err := os.Stat(primary); err != nil {
		t.Errorf("primary fallback missing: %v", err)
	}
}

func TestUpgradeSkillLocationsMonoOnlyPackageStillWorks(t *testing.T) {
	home := withFakeHome(t)
	mono := t.TempDir()
	os.WriteFile(filepath.Join(mono, "SKILL.md"), []byte("# mono"), 0o644)

	// Legacy mono-only package (no multi/): fall back to mono refresh even
	// when the disk already has dws/.
	agentsBase := filepath.Join(home, ".agents", "skills")
	os.MkdirAll(filepath.Join(agentsBase, "dws"), 0o755)
	os.WriteFile(filepath.Join(agentsBase, "dws", "SKILL.md"), []byte("old mono"), 0o644)
	os.MkdirAll(filepath.Join(agentsBase, "other-skill"), 0o755)
	os.WriteFile(filepath.Join(agentsBase, "other-skill", "SKILL.md"), []byte("not dws"), 0o644)

	result, err := UpgradeSkillLocations(mono)
	if err != nil {
		t.Fatalf("UpgradeSkillLocations() error = %v", err)
	}
	if len(result.Failed()) != 0 {
		t.Fatalf("expected 0 failures, got %v", result.Failed())
	}
	dest := filepath.Join(home, ".agents", "skills", "dws", "SKILL.md")
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("mono install missing: %v", err)
	}
	if string(data) != "# mono" {
		t.Errorf("mono content = %q, want refreshed package", data)
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "other-skill", "SKILL.md")); err != nil {
		t.Errorf("non-DWS dir should be preserved: %v", err)
	}

	// Mono install refreshes the mono cache so --mode mono fallbacks stay on
	// the upgraded version.
	if _, err := os.Stat(filepath.Join(home, ".dws", "skills", "mono", "SKILL.md")); err != nil {
		t.Errorf("mono cache not refreshed: %v", err)
	}
}

// TestUpgradeSkillLocationsMonoDiskMigratesToMulti pins the 2026-08-05
// decision: upgrade does NOT stick to disk. A mono-only home is one-shot
// migrated to multi when the release zip has multi/ (LocateSkillsRoot input).
func TestUpgradeSkillLocationsMonoDiskMigratesToMulti(t *testing.T) {
	home := withFakeHome(t)
	agentsBase := filepath.Join(home, ".agents", "skills")
	os.MkdirAll(filepath.Join(agentsBase, "dws"), 0o755)
	os.WriteFile(filepath.Join(agentsBase, "dws", "SKILL.md"), []byte("old mono"), 0o644)
	os.MkdirAll(filepath.Join(agentsBase, "other-skill"), 0o755)
	os.WriteFile(filepath.Join(agentsBase, "other-skill", "SKILL.md"), []byte("not dws"), 0o644)

	extract := t.TempDir()
	multiRoot := writeMultiBundle(t, extract, "dingtalk-chat", "dws-shared")
	if err := os.MkdirAll(filepath.Join(extract, "mono"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(extract, "mono", "SKILL.md"), []byte("# mono sibling"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := UpgradeSkillLocations(multiRoot)
	if err != nil {
		t.Fatalf("UpgradeSkillLocations() error = %v", err)
	}
	if len(result.Failed()) != 0 {
		t.Fatalf("expected 0 failures, got %v", result.Failed())
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "dws")); !os.IsNotExist(err) {
		t.Fatalf("mono leftover dws/ must be removed after multi upgrade, stat err=%v", err)
	}
	for _, name := range []string{"dingtalk-chat", "dws-shared"} {
		if _, err := os.Stat(filepath.Join(agentsBase, name, "SKILL.md")); err != nil {
			t.Errorf("multi skill missing after mono→multi migration: %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "other-skill", "SKILL.md")); err != nil {
		t.Errorf("non-DWS dir should be preserved: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".dws", "skills", "multi", "dingtalk-chat", "SKILL.md")); err != nil {
		t.Errorf("multi cache not refreshed: %v", err)
	}
}

// TestUpgradeSkillLocationsEmptyDiskInstallsMulti pins fresh/empty homes:
// with a multi package, upgrade installs multi (install default) and never
// writes dws/.
func TestUpgradeSkillLocationsEmptyDiskInstallsMulti(t *testing.T) {
	home := withFakeHome(t)
	extract := t.TempDir()
	multiRoot := writeMultiBundle(t, extract, "dingtalk-chat", "dws-shared")

	result, err := UpgradeSkillLocations(multiRoot)
	if err != nil {
		t.Fatalf("UpgradeSkillLocations() error = %v", err)
	}
	if len(result.Failed()) != 0 {
		t.Fatalf("expected 0 failures, got %v", result.Failed())
	}
	agentsBase := filepath.Join(home, ".agents", "skills")
	for _, name := range []string{"dingtalk-chat", "dws-shared"} {
		if _, err := os.Stat(filepath.Join(agentsBase, name, "SKILL.md")); err != nil {
			t.Errorf("empty-disk multi install missing %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "dws")); !os.IsNotExist(err) {
		t.Errorf("empty-disk multi upgrade must not create dws/, stat err=%v", err)
	}
}

// TestUpgradeSkillLocationsMultiDiskRefreshes pins an already-multi home:
// product skills are refreshed, stale dingtalk-* removed, non-DWS kept,
// and dws/ stays absent.
func TestUpgradeSkillLocationsMultiDiskRefreshes(t *testing.T) {
	home := withFakeHome(t)
	agentsBase := filepath.Join(home, ".agents", "skills")
	os.MkdirAll(filepath.Join(agentsBase, "dingtalk-chat"), 0o755)
	os.WriteFile(filepath.Join(agentsBase, "dingtalk-chat", "SKILL.md"), []byte("OLD chat"), 0o644)
	os.MkdirAll(filepath.Join(agentsBase, "dingtalk-stale"), 0o755)
	os.WriteFile(filepath.Join(agentsBase, "dingtalk-stale", "SKILL.md"), []byte("stale"), 0o644)
	os.MkdirAll(filepath.Join(agentsBase, "other-skill"), 0o755)
	os.WriteFile(filepath.Join(agentsBase, "other-skill", "SKILL.md"), []byte("not dws"), 0o644)

	extract := t.TempDir()
	multiRoot := writeMultiBundle(t, extract, "dingtalk-chat", "dws-shared")

	result, err := UpgradeSkillLocations(multiRoot)
	if err != nil {
		t.Fatalf("UpgradeSkillLocations() error = %v", err)
	}
	if len(result.Failed()) != 0 {
		t.Fatalf("expected 0 failures, got %v", result.Failed())
	}
	data, err := os.ReadFile(filepath.Join(agentsBase, "dingtalk-chat", "SKILL.md"))
	if err != nil {
		t.Fatalf("refreshed chat missing: %v", err)
	}
	if string(data) != "# dingtalk-chat" {
		t.Errorf("chat content = %q, want refreshed package", data)
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "dws-shared", "SKILL.md")); err != nil {
		t.Errorf("dws-shared missing after multi refresh: %v", err)
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "dingtalk-stale")); !os.IsNotExist(err) {
		t.Errorf("stale multi skill should be removed, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "dws")); !os.IsNotExist(err) {
		t.Errorf("multi refresh must not create dws/, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "other-skill", "SKILL.md")); err != nil {
		t.Errorf("non-DWS dir should be preserved: %v", err)
	}
}

// TestUpgradeSkillLocationsMonoFallbackAfterCopyFailure pins the mono primary
// fallback: when the main-loop copy into ~/.agents/skills/dws fails, the
// fallback retries the primary location and reports success (legacy
// mono-only package path).
func TestUpgradeSkillLocationsMonoFallbackAfterCopyFailure(t *testing.T) {
	home := withFakeHome(t)
	agentsBase := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(filepath.Join(agentsBase, "dws"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsBase, "dws", "SKILL.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	origCopy := upgradeCopyDir
	copyAttempts := 0
	testseam.Swap(t, &upgradeCopyDir, func(src, dst string) error {
		copyAttempts++
		if copyAttempts == 1 {
			return errors.New("injected copy failure")
		}
		return origCopy(src, dst)
	})

	mono := t.TempDir()
	if err := os.WriteFile(filepath.Join(mono, "SKILL.md"), []byte("# mono"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := UpgradeSkillLocations(mono)
	if err != nil {
		t.Fatalf("UpgradeSkillLocations() error = %v", err)
	}
	if failed := result.Failed(); len(failed) != 0 {
		t.Fatalf("fallback should replace the failed entry with OK, got failed=%v", failed)
	}
	if got := len(result.Succeeded()); got != 1 {
		t.Fatalf("Succeeded() len = %d, want 1 (%v)", got, result.Results)
	}
	data, err := os.ReadFile(filepath.Join(agentsBase, "dws", "SKILL.md"))
	if err != nil {
		t.Fatalf("mono fallback install missing: %v", err)
	}
	if string(data) != "# mono" {
		t.Errorf("mono content = %q, want refreshed package", data)
	}
}

// TestUpgradeSkillLocationsMonoReadDirErrorFailsHome pins the F4 fix: a base
// directory that exists but cannot be read must mark the home failed instead
// of silently installing mono alongside the multi leftovers.
func TestUpgradeSkillLocationsMonoReadDirErrorFailsHome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based permission injection is unix-only")
	}
	home := withFakeHome(t)

	// .agents/skills is unreadable; .claude installs fine so the primary
	// fallback never runs and the per-home failure is observable as-is.
	agentsBase := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(agentsBase, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(agentsBase, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(agentsBase, 0o755) })

	mono := t.TempDir()
	if err := os.WriteFile(filepath.Join(mono, "SKILL.md"), []byte("# mono"), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := UpgradeSkillLocations(mono)
	if err != nil {
		t.Fatalf("UpgradeSkillLocations() error = %v (fallback should not run: .claude succeeded)", err)
	}
	failed := result.Failed()
	if len(failed) != 1 {
		t.Fatalf("Failed() len = %d, want 1 (%v)", len(failed), result.Results)
	}
	wantDir := filepath.Join(agentsBase, "dws")
	if failed[0].Dir != wantDir || failed[0].Err == nil {
		t.Fatalf("failed entry = %#v, want dir %q with non-nil err", failed[0], wantDir)
	}
	if !strings.Contains(failed[0].Err.Error(), "读取技能目录失败") {
		t.Fatalf("failed err should mention the read failure, got %v", failed[0].Err)
	}
	if got := len(result.Succeeded()); got != 1 {
		t.Fatalf("Succeeded() len = %d, want 1 (.claude)", got)
	}
	// Mono must NOT have been laid down next to the unreadable multi state.
	// Restore permissions first: stat-ing inside a 000 dir fails with EACCES
	// regardless of whether the child exists.
	if err := os.Chmod(agentsBase, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "dws")); !os.IsNotExist(err) {
		t.Errorf("mono must not be installed into the unreadable home, stat err=%v", err)
	}
}

// TestUpgradeSkillLocationsMultiFallbackCleanupFailure pins the fail-loud
// semantics of the multi fallback: when leftover cleanup fails even at the
// primary location, UpgradeSkillLocations returns an error instead of
// installing multi next to the stale skills.
func TestUpgradeSkillLocationsMultiFallbackCleanupFailure(t *testing.T) {
	home := withFakeHome(t)
	agentsBase := filepath.Join(home, ".agents", "skills")
	staleDir := filepath.Join(agentsBase, "dingtalk-stale")
	if err := os.MkdirAll(staleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staleDir, "SKILL.md"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}

	origRemove := upgradeRemoveAll
	testseam.Swap(t, &upgradeRemoveAll, func(p string) error {
		if strings.HasSuffix(p, "dingtalk-stale") {
			return errors.New("injected cleanup failure")
		}
		return origRemove(p)
	})

	extract := t.TempDir()
	multiRoot := writeMultiBundle(t, extract, "dingtalk-chat")

	result, err := UpgradeSkillLocations(multiRoot)
	if err == nil {
		t.Fatal("multi fallback cleanup failure must return an error")
	}
	if !strings.Contains(err.Error(), "回退到主目录清理残留也失败") {
		t.Fatalf("error should mention the fallback cleanup failure, got %v", err)
	}
	if failed := result.Failed(); len(failed) != 1 {
		t.Fatalf("Failed() len = %d, want 1 (%v)", len(failed), result.Results)
	}
	// The stale skill was never removed and no partial multi install landed.
	if _, err := os.Stat(staleDir); err != nil {
		t.Errorf("stale dir should survive the failed cleanup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "dingtalk-chat")); !os.IsNotExist(err) {
		t.Errorf("multi skill must not be installed when cleanup failed, stat err=%v", err)
	}
}
