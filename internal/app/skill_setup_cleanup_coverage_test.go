// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

// TestCrossPlatformCoverageSkillSetupConfirmPreviewsStaleSkills verifies the
// confirmation prompt lists stale dingtalk-* / dws-shared directories that a
// full (unfiltered) multi install will back up and remove, and that a
// filtered install previews nothing extra.
func TestCrossPlatformCoverageSkillSetupConfirmPreviewsStaleSkills(t *testing.T) {
	testseam.Swap(t, &skillSetupInteractive, func() bool { return false })

	dest := filepath.Join(t.TempDir(), ".claude", "skills")
	stale := filepath.Join(dest, "dingtalk-old")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(stale, "SKILL.md"), []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := markDWSManagedSkillDir(stale); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	ok, err := confirmSkillSetup(&out, skillSetupModeMulti, "src", []string{dest}, []string{"dingtalk-chat"}, false)
	if err == nil || ok || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("confirmSkillSetup = (%v, %v), want non-interactive confirmation error", ok, err)
	}
	if !strings.Contains(out.String(), "将备份并移除过期 skill") {
		t.Fatalf("full install preview must list stale skills, got %q", out.String())
	}
	if !strings.Contains(out.String(), filepath.Join(dest, "dingtalk-old")) {
		t.Fatalf("preview must name the stale directory, got %q", out.String())
	}

	out.Reset()
	ok, err = confirmSkillSetup(&out, skillSetupModeMulti, "src", []string{dest}, []string{"dingtalk-chat"}, true)
	if err == nil || ok {
		t.Fatalf("filtered confirmSkillSetup = (%v, %v), want non-interactive confirmation error", ok, err)
	}
	if strings.Contains(out.String(), "将备份并移除过期 skill") {
		t.Fatalf("filtered install must stay additive in the preview, got %q", out.String())
	}
}

func TestCrossPlatformCoverageSkillSetupManagedMarkerValidationAndWriteFailure(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "dingtalk-custom")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if isManagedDWSMultiSkillDir(dir) {
		t.Fatal("an unmarked dingtalk-* directory must not be treated as DWS-owned")
	}
	if err := os.WriteFile(filepath.Join(dir, managedSkillMarkerName), []byte("someone-else\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if isManagedDWSMultiSkillDir(dir) {
		t.Fatal("a foreign marker value must not be treated as DWS-owned")
	}
	if err := markDWSManagedSkillDir(dir); err != nil {
		t.Fatal(err)
	}
	if !isManagedDWSMultiSkillDir(dir) {
		t.Fatal("the exact DWS marker must prove ownership")
	}
	legacy := filepath.Join(t.TempDir(), legacySharedSkill)
	if !isManagedDWSMultiSkillDir(legacy) {
		t.Fatal("the exact legacy dws-shared name must remain managed")
	}

	testseam.Swap(t, &skillSetupWriteFile, func(string, []byte, os.FileMode) error { return errors.New("marker denied") })
	if err := markDWSManagedSkillDir(dir); err == nil || !strings.Contains(err.Error(), "受管标记") {
		t.Fatalf("markDWSManagedSkillDir error = %v", err)
	}
}

func TestCrossPlatformCoverageSkillSetupEventMigrationMarkerFailures(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		failAt := failAt
		t.Run(fmt.Sprintf("marker-%d", failAt), func(t *testing.T) {
			src := writeMultiSkillSource(t, []string{multiEventSkill, multiMiscSkill})
			calls := 0
			testseam.Swap(t, &skillSetupWriteFile, func(path string, data []byte, mode os.FileMode) error {
				calls++
				if calls == failAt {
					return errors.New("marker denied")
				}
				return os.WriteFile(path, data, mode)
			})
			if _, err := prepareEventMiscMigration(src, t.TempDir()); err == nil || !strings.Contains(err.Error(), "受管标记") {
				t.Fatalf("prepareEventMiscMigration marker failure = %v", err)
			}
		})
	}
}

func TestCrossPlatformCoverageSkillSetupExecuteMarkerFailure(t *testing.T) {
	src := writeMultiSkillSource(t, []string{"dingtalk-a"})
	dest := filepath.Join(t.TempDir(), "skills")
	markerErr := errors.New("marker denied")
	testseam.Swap(t, &skillSetupWriteFile, func(string, []byte, os.FileMode) error { return markerErr })
	plan := &skillSetupPlan{
		Mode:            skillSetupModeMulti,
		Source:          src,
		MultiSkillNames: []string{"dingtalk-a"},
		Targets:         []skillSetupTargetPlan{{Destination: dest}},
	}

	var out, errOut bytes.Buffer
	installed, skipped, err := executeSkillSetupPlan(plan, &out, &errOut)
	if err != nil || installed != 0 || skipped != 1 {
		t.Fatalf("execute marker failure = (%d, %d, %v), want (0, 1, nil)", installed, skipped, err)
	}
	if out.Len() != 0 || !strings.Contains(errOut.String(), "受管标记") {
		t.Fatalf("execute marker output = %q / %q", out.String(), errOut.String())
	}
	if _, statErr := os.Stat(filepath.Join(dest, "dingtalk-a")); !os.IsNotExist(statErr) {
		t.Fatalf("marker failure published an unmarked Skill, stat err=%v", statErr)
	}
	entries, readErr := os.ReadDir(dest)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".dingtalk-a.tmp-") {
			t.Fatalf("marker failure retained staging directory %s", entry.Name())
		}
	}
}

func TestCrossPlatformCoveragePublishManagedSkillFailurePaths(t *testing.T) {
	src := writeMultiSkillSource(t, []string{"dingtalk-a"})
	skillSrc := filepath.Join(src, "dingtalk-a")
	failure := errors.New("publish denied")

	t.Run("mkdir", func(t *testing.T) {
		testseam.Swap(t, &skillSetupPublishTemp, func(string, string) (string, error) { return "", failure })
		err := publishDWSManagedSkillDir(skillSrc, filepath.Join(t.TempDir(), "dingtalk-a"))
		if !errors.Is(err, failure) || !strings.Contains(err.Error(), "staging") {
			t.Fatalf("mkdir error = %v", err)
		}
	})

	t.Run("copy", func(t *testing.T) {
		parent := t.TempDir()
		testseam.Swap(t, &skillSetupCopyDir, func(string, string) error { return failure })
		err := publishDWSManagedSkillDir(skillSrc, filepath.Join(parent, "dingtalk-a"))
		if !errors.Is(err, failure) {
			t.Fatalf("copy error = %v", err)
		}
		if entries, readErr := os.ReadDir(parent); readErr != nil || len(entries) != 0 {
			t.Fatalf("copy failure retained staging: %v, err=%v", entries, readErr)
		}
	})

	t.Run("rename", func(t *testing.T) {
		parent := t.TempDir()
		testseam.Swap(t, &skillSetupPublishRename, func(string, string) error { return failure })
		err := publishDWSManagedSkillDir(skillSrc, filepath.Join(parent, "dingtalk-a"))
		if !errors.Is(err, failure) || !strings.Contains(err.Error(), "发布 Skill") {
			t.Fatalf("rename error = %v", err)
		}
		if entries, readErr := os.ReadDir(parent); readErr != nil || len(entries) != 0 {
			t.Fatalf("rename failure retained staging: %v, err=%v", entries, readErr)
		}
	})

	t.Run("cleanup", func(t *testing.T) {
		markerErr := errors.New("marker denied")
		cleanupErr := errors.New("cleanup denied")
		testseam.Swap(t, &skillSetupWriteFile, func(string, []byte, os.FileMode) error { return markerErr })
		testseam.Swap(t, &skillSetupRemoveAll, func(string) error { return cleanupErr })
		err := publishDWSManagedSkillDir(skillSrc, filepath.Join(t.TempDir(), "dingtalk-a"))
		if !errors.Is(err, markerErr) || !errors.Is(err, cleanupErr) {
			t.Fatalf("cleanup error = %v", err)
		}
	})
}

// TestCrossPlatformCoverageSkillSetupCleanupHomeFailure verifies that
// cleanupMutualExclusion keeps every victim in place with a warning when
// $HOME cannot be resolved, instead of destroying anything.
func TestCrossPlatformCoverageSkillSetupCleanupHomeFailure(t *testing.T) {
	dest := filepath.Join(t.TempDir(), ".agents", "skills")
	victim := filepath.Join(dest, "dws")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(victim, "SKILL.md"), []byte("mono"), 0o644); err != nil {
		t.Fatal(err)
	}
	homeErr := errors.New("home boom")
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return "", homeErr })

	var out, errOut bytes.Buffer
	cleanupMutualExclusion(dest, skillSetupModeMulti, &out, &errOut)
	if !strings.Contains(errOut.String(), "无法解析 HOME，跳过删除") {
		t.Fatalf("expected HOME warning on errOut, got %q", errOut.String())
	}
	if _, err := os.Stat(filepath.Join(victim, "SKILL.md")); err != nil {
		t.Fatalf("victim must survive the HOME failure: %v", err)
	}
}

func TestCrossPlatformCoverageSkillSetupBackupFailureSkipsWholeTarget(t *testing.T) {
	home := t.TempDir()
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
	copyCalls := 0
	testseam.Swap(t, &skillSetupCopyDir, func(string, string) error {
		copyCalls++
		return nil
	})
	failure := errors.New("backup boom")
	testseam.Swap(t, &skillSetupBackupAndRemove, func(_ string, dir string) (string, error) {
		if filepath.Base(dir) == "dws" {
			return "", failure
		}
		return "", nil
	})

	dest := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(filepath.Join(dest, "dws"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := writeMultiSkillSource(t, []string{"dingtalk-a", "dingtalk-shared"})
	var out, errOut bytes.Buffer
	installed, skipped, err := installMultiSkillToHomes(src, []string{"dingtalk-a", "dingtalk-shared"}, []string{dest}, &out, &errOut, false)
	if err != nil || installed != 0 || skipped != 2 {
		t.Fatalf("install = (%d, %d, %v), want (0, 2, nil)", installed, skipped, err)
	}
	if copyCalls != 0 {
		t.Fatalf("backup failure copied %d new Skills", copyCalls)
	}
	if !strings.Contains(errOut.String(), "跳过整个 Agent 目标") {
		t.Fatalf("missing whole-target warning: %q", errOut.String())
	}
}

func TestCrossPlatformCoverageSkillSetupCleanupMutualExclusionBackupFailure(t *testing.T) {
	home := t.TempDir()
	dest := filepath.Join(home, ".agents", "skills")
	victim := filepath.Join(dest, "dws")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("backup boom")
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
	testseam.Swap(t, &skillSetupBackupAndRemove, func(_ string, dir string) (string, error) {
		if dir != victim {
			t.Fatalf("backup victim = %q, want %q", dir, victim)
		}
		return "", failure
	})

	var out, errOut bytes.Buffer
	err := cleanupMutualExclusion(dest, skillSetupModeMulti, &out, &errOut)
	if !errors.Is(err, failure) {
		t.Fatalf("cleanup error = %v, want %v", err, failure)
	}
	if out.Len() != 0 || !strings.Contains(errOut.String(), "互斥清理失败") {
		t.Fatalf("cleanup output = %q / %q", out.String(), errOut.String())
	}
}

func TestCrossPlatformCoverageSkillSetupMonoCleanupFailureSkipsWholeTarget(t *testing.T) {
	home := t.TempDir()
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
	copyCalls := 0
	testseam.Swap(t, &skillSetupCopyDir, func(string, string) error {
		copyCalls++
		return nil
	})
	failure := errors.New("multi backup boom")
	testseam.Swap(t, &skillSetupBackupAndRemove, func(_ string, dir string) (string, error) {
		if filepath.Base(dir) == "dingtalk-a" {
			return "", failure
		}
		return "", nil
	})

	base := filepath.Join(home, ".agents", "skills")
	multi := filepath.Join(base, "dingtalk-a")
	if err := os.MkdirAll(multi, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := markDWSManagedSkillDir(multi); err != nil {
		t.Fatal(err)
	}
	monoSrc := t.TempDir()
	if err := os.WriteFile(filepath.Join(monoSrc, "SKILL.md"), []byte("mono"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	installed, skipped, err := installSkillToHomes(monoSrc, []string{filepath.Join(base, "dws")}, &out, &errOut)
	if err != nil || installed != 0 || skipped != 1 {
		t.Fatalf("install = (%d, %d, %v), want (0, 1, nil)", installed, skipped, err)
	}
	if copyCalls != 0 {
		t.Fatalf("multi cleanup failure copied mono %d times", copyCalls)
	}
	if _, err := os.Stat(multi); err != nil {
		t.Fatalf("multi leftover must survive backup failure: %v", err)
	}
	if !strings.Contains(errOut.String(), "互斥清理失败，跳过整个 Agent 目标") {
		t.Fatalf("missing mono whole-target warning: %q", errOut.String())
	}
}

func TestCrossPlatformCoverageSkillSetupStaleBackupFailureSkipsWholeTarget(t *testing.T) {
	home := t.TempDir()
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
	copyCalls := 0
	testseam.Swap(t, &skillSetupCopyDir, func(string, string) error {
		copyCalls++
		return nil
	})
	failure := errors.New("stale backup boom")
	testseam.Swap(t, &skillSetupBackupAndRemove, func(_ string, dir string) (string, error) {
		if filepath.Base(dir) == "dingtalk-stale" {
			return "", failure
		}
		return "", nil
	})

	dest := filepath.Join(home, ".agents", "skills")
	stale := filepath.Join(dest, "dingtalk-stale")
	if err := os.MkdirAll(stale, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := markDWSManagedSkillDir(stale); err != nil {
		t.Fatal(err)
	}
	src := writeMultiSkillSource(t, []string{"dingtalk-a", "dingtalk-shared"})
	var out, errOut bytes.Buffer
	installed, skipped, err := installMultiSkillToHomes(src, []string{"dingtalk-a", "dingtalk-shared"}, []string{dest}, &out, &errOut, false)
	if err != nil || installed != 0 || skipped != 2 {
		t.Fatalf("install = (%d, %d, %v), want (0, 2, nil)", installed, skipped, err)
	}
	if copyCalls != 0 {
		t.Fatalf("stale backup failure copied %d new Skills", copyCalls)
	}
	if !strings.Contains(errOut.String(), "过期 Skill 备份失败，跳过整个 Agent 目标") {
		t.Fatalf("missing stale whole-target warning: %q", errOut.String())
	}
}

// TestCrossPlatformCoverageSkillSetupInstallHomeFailureSkips verifies both
// install paths skip (never destroy) every target when $HOME cannot be
// resolved for the pre-refresh backup.
func TestCrossPlatformCoverageSkillSetupInstallHomeFailureSkips(t *testing.T) {
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return "", errors.New("home boom") })

	monoSrc := t.TempDir()
	if err := os.WriteFile(filepath.Join(monoSrc, "SKILL.md"), []byte("# mono"), 0o644); err != nil {
		t.Fatal(err)
	}
	monoDest := filepath.Join(t.TempDir(), "agent", "dws")
	if err := os.MkdirAll(monoDest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(monoDest, "SKILL.md"), []byte("# old"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	installed, skipped, err := installSkillToHomes(monoSrc, []string{monoDest}, &out, &errOut)
	if err != nil || installed != 0 || skipped != 1 {
		t.Fatalf("mono install = (%d, %d, %v), want (0, 1, nil)", installed, skipped, err)
	}
	if !strings.Contains(errOut.String(), "无法解析 HOME，跳过刷新") {
		t.Fatalf("expected HOME skip warning, got %q", errOut.String())
	}
	if _, err := os.Stat(filepath.Join(monoDest, "SKILL.md")); err != nil {
		t.Fatalf("existing mono dir must be preserved: %v", err)
	}

	multiSrc := writeMultiSkillSource(t, []string{"dingtalk-a"})
	multiDest := filepath.Join(t.TempDir(), ".claude", "skills")
	if err := os.MkdirAll(filepath.Join(multiDest, "dingtalk-a"), 0o755); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	errOut.Reset()
	installed, skipped, err = installMultiSkillToHomes(multiSrc, []string{"dingtalk-a"}, []string{multiDest}, &out, &errOut, true)
	if err != nil || installed != 0 || skipped != 1 {
		t.Fatalf("multi install = (%d, %d, %v), want (0, 1, nil)", installed, skipped, err)
	}
	if !strings.Contains(errOut.String(), "无法解析 HOME，跳过整个 Agent 目标") {
		t.Fatalf("expected multi HOME skip warning, got %q", errOut.String())
	}
	if _, err := os.Stat(filepath.Join(multiDest, "dingtalk-a")); err != nil {
		t.Fatalf("existing sub skill must be preserved: %v", err)
	}
}

// TestCrossPlatformCoverageSkillSetupRemoveStaleMultiSkillsEdges covers
// removeStaleMultiSkills and its preview companion staleMultiSkillVictims:
// scan failures, the HOME failure, backup failures, and the success path.
func TestCrossPlatformCoverageSkillSetupRemoveStaleMultiSkillsEdges(t *testing.T) {
	dest := filepath.Join(t.TempDir(), ".cursor", "skills")
	keep := []string{"dingtalk-chat"}
	entries := map[string]bool{ // dir entries; README below is a plain file
		"dingtalk-chat":  true, // kept (in bundle)
		"dingtalk-stale": true, // stale product skill
		"dws-shared":     true, // legacy shared name is stale too
		"other-skill":    true, // non-DWS, must survive
	}
	for name := range entries {
		if err := os.MkdirAll(filepath.Join(dest, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := markDWSManagedSkillDir(filepath.Join(dest, "dingtalk-stale")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "README"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out, errOut bytes.Buffer

	// Non-ENOENT scan failure warns; ENOENT is silent.
	testseam.Swap(t, &skillSetupReadDir, func(string) ([]os.DirEntry, error) { return nil, errors.New("scan boom") })
	removeStaleMultiSkills(dest, keep, &out, &errOut)
	if !strings.Contains(errOut.String(), "过期 skill 扫描失败") {
		t.Fatalf("expected scan warning, got %q", errOut.String())
	}
	errOut.Reset()
	testseam.Swap(t, &skillSetupReadDir, func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist })
	removeStaleMultiSkills(dest, keep, &out, &errOut)
	if errOut.Len() != 0 {
		t.Fatalf("ENOENT scan must be silent, got %q", errOut.String())
	}
	testseam.Swap(t, &skillSetupReadDir, os.ReadDir)

	// The preview companion sees the same victims and skips files/kept/non-DWS.
	victims := staleMultiSkillVictims(dest, keep)
	wantVictims := []string{filepath.Join(dest, "dingtalk-stale"), filepath.Join(dest, "dws-shared")}
	if len(victims) != len(wantVictims) {
		t.Fatalf("staleMultiSkillVictims = %v, want %v", victims, wantVictims)
	}

	// HOME failure keeps every stale directory with a warning.
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return "", errors.New("home boom") })
	errOut.Reset()
	removeStaleMultiSkills(dest, keep, &out, &errOut)
	if !strings.Contains(errOut.String(), "无法解析 HOME，跳过删除") {
		t.Fatalf("expected HOME warning, got %q", errOut.String())
	}
	for name := range entries {
		if _, err := os.Stat(filepath.Join(dest, name)); err != nil {
			t.Fatalf("entry %s must survive the HOME failure: %v", name, err)
		}
	}
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return t.TempDir(), nil })

	// Backup failure keeps the stale directory with a warning.
	testseam.Swap(t, &skillSetupBackupAndRemove, func(string, string) (string, error) { return "", errors.New("backup boom") })
	errOut.Reset()
	removeStaleMultiSkills(dest, keep, &out, &errOut)
	if !strings.Contains(errOut.String(), "过期 skill 清理失败（保留原目录") {
		t.Fatalf("expected backup failure warning, got %q", errOut.String())
	}
	for _, stale := range wantVictims {
		if _, err := os.Stat(stale); err != nil {
			t.Fatalf("stale dir must survive the backup failure: %v", err)
		}
	}

	// Success: both stale dirs are backed up and reported; the rest survives.
	testseam.Swap(t, &skillSetupBackupAndRemove, func(_, dir string) (string, error) { return filepath.Join(t.TempDir(), "backup"), nil })
	out.Reset()
	removeStaleMultiSkills(dest, keep, &out, &errOut)
	if count := strings.Count(out.String(), "已备份并清理过期 skill"); count != len(wantVictims) {
		t.Fatalf("expected %d stale cleanup lines, got %d (out=%q)", len(wantVictims), count, out.String())
	}
	if _, err := os.Stat(filepath.Join(dest, "other-skill")); err != nil {
		t.Fatalf("non-DWS dir must survive: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "dingtalk-chat")); err != nil {
		t.Fatalf("bundle skill must survive: %v", err)
	}
}
