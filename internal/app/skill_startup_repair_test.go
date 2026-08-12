// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	upgradepkg "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/upgrade"
	"github.com/spf13/cobra"
)

func TestCrossPlatformCoverageStartupRepairsNestedUpgradeLayout(t *testing.T) {
	home := t.TempDir()
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
	upgradepkg.SwapUserHomeDirForTest(t, func() (string, error) { return home, nil })
	testseam.Swap(t, &skillSetupAgentHomes, []string{".agents/skills", ".codex/skills"})
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(home, ".agents", "skills", "dws", "multi", "dingtalk-chat")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "SKILL.md"), []byte("old nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := repairNestedMultiSkillLayout(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(home, ".codex", "skills", "dingtalk-chat", "SKILL.md")); err != nil {
		t.Fatalf("canonical Codex Skill missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, ".agents", "skills", "dws")); !os.IsNotExist(err) {
		t.Fatalf("nested generic layout remains: %v", err)
	}
}

func TestCrossPlatformCoverageStartupRepairIgnoresLegitimateMonoAndReportsFailure(t *testing.T) {
	home := t.TempDir()
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
	testseam.Swap(t, &skillSetupAgentHomes, []string{".agents/skills"})
	mono := filepath.Join(home, ".agents", "skills", "dws")
	if err := os.MkdirAll(mono, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mono, "SKILL.md"), []byte("valid mono"), 0o644); err != nil {
		t.Fatal(err)
	}
	called := false
	testseam.Swap(t, &repairNestedSkillUpgrade, func(string, upgradepkg.SkillUpgradeOptions) (*upgradepkg.SkillUpgradeResult, error) {
		called = true
		return nil, errors.New("unexpected")
	})
	if err := repairNestedMultiSkillLayout(); err != nil || called {
		t.Fatalf("valid mono repair = %v, called=%v", err, called)
	}

	nested := filepath.Join(mono, "multi", "dingtalk-chat")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "SKILL.md"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := repairNestedMultiSkillLayout(); err == nil {
		t.Fatal("repair failure was not surfaced")
	}
}

func TestCrossPlatformCoverageStartupRepairSkipsExplicitSkillManagers(t *testing.T) {
	upgradeCmd := &cobra.Command{Use: "upgrade"}
	if shouldRepairNestedSkillLayout(upgradeCmd) {
		t.Fatal("upgrade must manage its own Skill lifecycle")
	}
	skillCmd := &cobra.Command{Use: "skill"}
	setupCmd := &cobra.Command{Use: "setup"}
	skillCmd.AddCommand(setupCmd)
	if shouldRepairNestedSkillLayout(setupCmd) {
		t.Fatal("skill setup must manage its own Skill lifecycle")
	}
	if !shouldRepairNestedSkillLayout(&cobra.Command{Use: "version"}) || !shouldRepairNestedSkillLayout(nil) {
		t.Fatal("ordinary commands must trigger repair detection")
	}
}

func TestCrossPlatformCoverageStartupRepairErrorBranchesAndRootWarning(t *testing.T) {
	failure := errors.New("repair failure")
	t.Run("HOME failure", func(t *testing.T) {
		testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return "", failure })
		if err := repairNestedMultiSkillLayout(); !errors.Is(err, failure) {
			t.Fatalf("HOME error = %v", err)
		}
	})

	t.Run("embedded extraction failure", func(t *testing.T) {
		home := t.TempDir()
		testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
		testseam.Swap(t, &skillSetupAgentHomes, []string{".agents/skills"})
		nested := filepath.Join(home, ".agents", "skills", "dws", "multi", "dingtalk-chat")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nested, "SKILL.md"), []byte("nested"), 0o644); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &embeddedSkillStat, func(string) (os.FileInfo, error) { return nil, failure })
		if err := repairNestedMultiSkillLayout(); !errors.Is(err, failure) {
			t.Fatalf("embedded extraction error = %v", err)
		}
	})

	t.Run("failed Agent result", func(t *testing.T) {
		home := t.TempDir()
		testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
		testseam.Swap(t, &skillSetupAgentHomes, []string{".agents/skills"})
		nested := filepath.Join(home, ".agents", "skills", "dws", "multi", "dingtalk-chat")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(nested, "SKILL.md"), []byte("nested"), 0o644); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &repairNestedSkillUpgrade, func(string, upgradepkg.SkillUpgradeOptions) (*upgradepkg.SkillUpgradeResult, error) {
			return &upgradepkg.SkillUpgradeResult{Results: []upgradepkg.SkillDirResult{{Status: upgradepkg.SkillDirFailed, Err: failure}}}, nil
		})
		if err := repairNestedMultiSkillLayout(); err == nil {
			t.Fatal("failed Agent repair succeeded")
		}
	})

	t.Run("root warning", func(t *testing.T) {
		testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return "", failure })
		root := newRootCommandWithEngine(context.Background(), nil, false, true)
		cmd := &cobra.Command{Use: "version"}
		cmd.SetContext(context.Background())
		var stderr bytes.Buffer
		cmd.SetErr(&stderr)
		if err := root.PersistentPreRunE(cmd, nil); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(stderr.String(), "自动修复失败") {
			t.Fatalf("root repair warning missing: %q", stderr.String())
		}
	})
}
