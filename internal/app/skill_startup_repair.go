// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package app

import (
	"fmt"
	"path/filepath"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/upgrade"
	"github.com/spf13/cobra"
)

var repairNestedSkillUpgrade = upgrade.UpgradeSkillLocationsWithOptions

// repairNestedMultiSkillLayout closes the compatibility gap where an older
// running dws process replaces itself with a newer binary, but then installs
// the new release bundle with its legacy mono copier. That produces the
// impossible layout <agent>/dws/multi/<skill>/SKILL.md. The new binary cannot
// change the already-running old process, so its next invocation repairs this
// exact malformed layout from the bundle embedded in the new binary.
//
// A legitimate mono install has no dws/multi product tree and is untouched.
func repairNestedMultiSkillLayout() error {
	home, err := skillSetupUserHomeDir()
	if err != nil {
		return err
	}
	found := false
	for _, rel := range skillSetupAgentHomes {
		nested := filepath.Join(home, rel, "dws", "multi")
		if isSkillSourceRoot(nested, skillSetupModeMulti) {
			found = true
			break
		}
	}
	if !found {
		return nil
	}

	source, cleanup, err := materializeEmbeddedSkillSource(skillSetupModeMulti)
	if err != nil {
		return fmt.Errorf("提取当前版本内嵌 multi Skill 失败: %w", err)
	}
	defer cleanup()
	result, err := repairNestedSkillUpgrade(source, upgrade.SkillUpgradeOptions{Version: RawVersion()})
	if err != nil {
		return err
	}
	if failed := result.Failed(); len(failed) > 0 {
		return fmt.Errorf("%d 个 Agent 目录修复失败", len(failed))
	}
	return nil
}

func shouldRepairNestedSkillLayout(cmd *cobra.Command) bool {
	if cmd == nil {
		return true
	}
	if cmd.Name() == "upgrade" {
		return false
	}
	return !(cmd.Name() == "setup" && cmd.Parent() != nil && cmd.Parent().Name() == "skill")
}
