// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/upgrade"
)

// TestUpgradeSkillLocationsMonoSeedMigratesToMulti is the fake-HOME E2E for
// the 2026-08-05 owner decision: upgrade is not disk-sticky. Seeding a mono
// layout then calling UpgradeSkillLocations with a multi bundle must install
// product skills, remove dws/, and leave non-DWS dirs alone.
func TestUpgradeSkillLocationsMonoSeedMigratesToMulti(t *testing.T) {
	home := t.TempDir()
	setTestHome(t, home)
	upgrade.SwapUserHomeDirForTest(t, func() (string, error) { return home, nil })

	agentsBase := filepath.Join(home, ".agents", "skills")
	if err := os.MkdirAll(filepath.Join(agentsBase, "dws"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsBase, "dws", "SKILL.md"), []byte("old mono"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(agentsBase, "other-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsBase, "other-skill", "SKILL.md"), []byte("not dws"), 0o644); err != nil {
		t.Fatal(err)
	}

	extract := t.TempDir()
	multiRoot := filepath.Join(extract, "multi")
	for _, name := range []string{"dingtalk-chat", "dws-shared"} {
		dir := filepath.Join(multiRoot, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := upgrade.UpgradeSkillLocations(multiRoot)
	if err != nil {
		t.Fatalf("UpgradeSkillLocations() error = %v", err)
	}
	if failed := result.Failed(); len(failed) != 0 {
		t.Fatalf("expected 0 failures, got %v", failed)
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "dws")); !os.IsNotExist(err) {
		t.Fatalf("mono leftover dws/ must be gone, stat err=%v", err)
	}
	for _, name := range []string{"dingtalk-chat", "dws-shared"} {
		if _, err := os.Stat(filepath.Join(agentsBase, name, "SKILL.md")); err != nil {
			t.Errorf("multi skill missing: %s: %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(agentsBase, "other-skill", "SKILL.md")); err != nil {
		t.Errorf("non-DWS dir should be preserved: %v", err)
	}
}
