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

func canonicalFailurePlan(t *testing.T) (string, *skillSetupPlan) {
	t.Helper()
	home := t.TempDir()
	src := t.TempDir()
	skill := filepath.Join(src, "dingtalk-chat")
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("chat"), 0o644); err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(home, ".agents", "skills")
	dependent := filepath.Join(home, ".claude", "skills")
	plan, err := buildSkillSetupPlan(skillSetupModeMulti, src, []string{canonical, dependent}, []string{"dingtalk-chat"}, true)
	if err != nil {
		t.Fatal(err)
	}
	return home, plan
}

func TestCrossPlatformCoverageSkillSetupCanonicalHomeBackupAndPublishFailures(t *testing.T) {
	t.Run("home", func(t *testing.T) {
		_, plan := canonicalFailurePlan(t)
		canonical := filepath.Join(plan.Targets[0].Destination, "dingtalk-chat")
		if err := os.MkdirAll(canonical, 0o755); err != nil {
			t.Fatal(err)
		}
		plan.Targets[0].Backups = []skillSetupBackup{{Path: canonical, Reason: skillSetupBackupReplace}}
		testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return "", errors.New("home denied") })
		if _, _, err := executeSkillSetupPlan(plan, &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "canonical Skill 刷新中止") {
			t.Fatalf("home failure = %v", err)
		}
	})

	t.Run("backup", func(t *testing.T) {
		home, plan := canonicalFailurePlan(t)
		victim := filepath.Join(plan.Targets[0].Destination, "dingtalk-chat")
		if err := os.MkdirAll(victim, 0o755); err != nil {
			t.Fatal(err)
		}
		plan.Targets[0].Backups = []skillSetupBackup{{Path: victim, Reason: skillSetupBackupReplace}}
		testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
		testseam.Swap(t, &skillSetupBackupAndRemove, func(string, string) (string, error) { return "", errors.New("backup denied") })
		if _, _, err := executeSkillSetupPlan(plan, &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "canonical Skill 备份失败") {
			t.Fatalf("backup failure = %v", err)
		}
	})

	t.Run("publish", func(t *testing.T) {
		home, plan := canonicalFailurePlan(t)
		testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
		testseam.Swap(t, &skillSetupPublishRename, func(string, string) error { return errors.New("publish denied") })
		if _, _, err := executeSkillSetupPlan(plan, &bytes.Buffer{}, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "canonical Skill 发布失败") {
			t.Fatalf("publish failure = %v", err)
		}
	})
}

func TestCrossPlatformCoverageSkillSetupLinkResolutionFailures(t *testing.T) {
	home, plan := canonicalFailurePlan(t)
	canonicalTarget := filepath.Join(plan.Targets[0].Destination, "dingtalk-chat")
	if err := os.MkdirAll(canonicalTarget, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(canonicalTarget, "SKILL.md"), []byte("chat"), 0o644); err != nil {
		t.Fatal(err)
	}
	linked := plan.Targets[1]

	t.Run("physical-parent", func(t *testing.T) {
		testseam.Swap(t, &skillSetupEvalSymlinks, func(path string) (string, error) {
			if path == linked.Destination {
				return "", errors.New("parent denied")
			}
			return filepath.EvalSymlinks(path)
		})
		if _, _, err := stageSkillSetupTarget(plan, linked); err == nil || !strings.Contains(err.Error(), "物理目录") {
			t.Fatalf("physical parent error = %v", err)
		}
	})

	t.Run("canonical-target", func(t *testing.T) {
		testseam.Swap(t, &skillSetupEvalSymlinks, func(path string) (string, error) {
			if path == canonicalTarget {
				return "", errors.New("canonical denied")
			}
			return filepath.EvalSymlinks(path)
		})
		if _, _, err := stageSkillSetupTarget(plan, linked); err == nil || !strings.Contains(err.Error(), "解析 canonical") {
			t.Fatalf("canonical target error = %v", err)
		}
	})

	t.Run("relative-path", func(t *testing.T) {
		testseam.Swap(t, &skillSetupRel, func(string, string) (string, error) { return "", errors.New("relative denied") })
		if _, _, err := stageSkillSetupTarget(plan, linked); err == nil || !strings.Contains(err.Error(), "相对链接") {
			t.Fatalf("relative path error = %v", err)
		}
	})

	_ = home
}
