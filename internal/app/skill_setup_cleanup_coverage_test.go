// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"bytes"
	"errors"
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

	var out bytes.Buffer
	ok, err := confirmSkillSetup(&out, skillSetupModeMulti, "src", []string{dest}, []string{"dingtalk-chat"}, false)
	if err != nil || !ok {
		t.Fatalf("confirmSkillSetup = (%v, %v), want (true, nil)", ok, err)
	}
	if !strings.Contains(out.String(), "将备份并移除过期 skill") {
		t.Fatalf("full install preview must list stale skills, got %q", out.String())
	}
	if !strings.Contains(out.String(), filepath.Join(dest, "dingtalk-old")) {
		t.Fatalf("preview must name the stale directory, got %q", out.String())
	}

	out.Reset()
	ok, err = confirmSkillSetup(&out, skillSetupModeMulti, "src", []string{dest}, []string{"dingtalk-chat"}, true)
	if err != nil || !ok {
		t.Fatalf("filtered confirmSkillSetup = (%v, %v), want (true, nil)", ok, err)
	}
	if strings.Contains(out.String(), "将备份并移除过期 skill") {
		t.Fatalf("filtered install must stay additive in the preview, got %q", out.String())
	}
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
	if !strings.Contains(errOut.String(), "无法解析 HOME，跳过刷新") {
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
