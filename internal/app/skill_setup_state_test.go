package app

import (
	"errors"
	"io"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/skillstate"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func TestCrossPlatformCoverageSkillSetupPersistsOfficialSnapshot(t *testing.T) {
	home := t.TempDir()
	testseam.Swap(t, &skillSetupResolveMode, func(mode string, _ bool, _ io.Writer) (string, error) { return mode, nil })
	testseam.Swap(t, &skillSetupResolveSource, func(string, string) (string, func(), error) { return "source", func() {}, nil })
	testseam.Swap(t, &skillSetupResolveTargets, func(string, string) ([]string, error) { return []string{filepath.Join(home, "skills")}, nil })
	testseam.Swap(t, &skillSetupListMulti, func(string) ([]string, error) {
		return []string{"dingtalk-a", "dingtalk-b", "dingtalk-shared"}, nil
	})
	testseam.Swap(t, &skillSetupFilterMulti, filterMultiSkillNames)
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
	testseam.Swap(t, &skillSetupExecutePlan, func(plan *skillSetupPlan, _ io.Writer, _ io.Writer) (int, int, error) {
		if plan.Mode == skillSetupModeMono {
			return 1, 0, nil
		}
		return 2, 0, nil
	})
	testseam.Swap(t, &skillSetupNow, func() time.Time {
		return time.Date(2026, 8, 10, 1, 2, 3, 0, time.UTC)
	})

	var saved skillstate.State
	testseam.Swap(t, &skillSetupWriteState, func(_ string, state skillstate.State) error {
		saved = state
		return nil
	})
	cmd := skillSetupCoverageCommand(t, skillSetupModeMulti, true)
	if err := cmd.Flags().Set("skill", "a"); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved.OfficialSkills, []string{"dingtalk-a", "dingtalk-b", "dingtalk-shared"}) ||
		!reflect.DeepEqual(saved.UpdatedSkills, []string{"dingtalk-shared", "dingtalk-a"}) {
		t.Fatalf("saved = %#v", saved)
	}

	testseam.Swap(t, &skillSetupWriteState, func(string, skillstate.State) error { return errors.New("denied") })
	cmd = skillSetupCoverageCommand(t, skillSetupModeMulti, true)
	if err := cmd.RunE(cmd, nil); err == nil || !strings.Contains(err.Error(), "信息快照失败") {
		t.Fatalf("write-state error = %v", err)
	}

	testseam.Swap(t, &skillSetupRemoveState, func(string) error { return errors.New("denied") })
	cmd = skillSetupCoverageCommand(t, skillSetupModeMono, true)
	if err := cmd.RunE(cmd, nil); err == nil || !strings.Contains(err.Error(), "清理 multi") {
		t.Fatalf("remove-state error = %v", err)
	}

	testseam.Swap(t, &skillSetupRemoveState, func(string) error { return nil })
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return "", errors.New("no home") })
	cmd = skillSetupCoverageCommand(t, skillSetupModeMono, true)
	if err := cmd.RunE(cmd, nil); err == nil || !strings.Contains(err.Error(), "无法解析 HOME") {
		t.Fatalf("home error = %v", err)
	}
}

func TestCrossPlatformCoverageSkillSetupPartialInstallDoesNotWriteState(t *testing.T) {
	home := t.TempDir()
	testseam.Swap(t, &skillSetupResolveMode, func(mode string, _ bool, _ io.Writer) (string, error) { return mode, nil })
	testseam.Swap(t, &skillSetupResolveSource, func(string, string) (string, func(), error) { return "source", func() {}, nil })
	testseam.Swap(t, &skillSetupResolveTargets, func(string, string) ([]string, error) {
		return []string{filepath.Join(home, "skills")}, nil
	})
	testseam.Swap(t, &skillSetupListMulti, func(string) ([]string, error) {
		return []string{"dingtalk-a", "dingtalk-b", "dingtalk-shared"}, nil
	})
	testseam.Swap(t, &skillSetupFilterMulti, filterMultiSkillNames)
	testseam.Swap(t, &skillSetupExecutePlan, func(*skillSetupPlan, io.Writer, io.Writer) (int, int, error) {
		return 2, 1, nil
	})

	writes := 0
	testseam.Swap(t, &skillSetupWriteState, func(string, skillstate.State) error {
		writes++
		return nil
	})
	cmd := skillSetupCoverageCommand(t, skillSetupModeMulti, true)
	err := cmd.RunE(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "Skill 安装不完整") || !strings.Contains(err.Error(), "skipped=1") {
		t.Fatalf("partial setup error = %v", err)
	}
	if writes != 0 {
		t.Fatalf("partial setup wrote %d complete state snapshot(s)", writes)
	}
}
