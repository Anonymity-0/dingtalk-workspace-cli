package app

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/skillstate"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	"github.com/charmbracelet/huh"
)

func TestCrossPlatformCoverageSkillSetupPlanPreviewDeclineAndExecutionMatch(t *testing.T) {
	home := t.TempDir()
	dest := filepath.Join(home, ".claude", "skills")
	source := writeMultiSkillSource(t, []string{"dingtalk-a", "dingtalk-shared"})
	for _, name := range []string{"dws", "dingtalk-a", "dingtalk-stale"} {
		if err := os.MkdirAll(filepath.Join(dest, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	testseam.Swap(t, &skillSetupResolveMode, func(mode string, _ bool, _ io.Writer) (string, error) { return mode, nil })
	testseam.Swap(t, &skillSetupResolveSource, func(string, string) (string, func(), error) { return source, func() {}, nil })
	testseam.Swap(t, &skillSetupResolveTargets, func(string, string) ([]string, error) { return []string{dest}, nil })
	testseam.Swap(t, &skillSetupListMulti, func(string) ([]string, error) {
		return []string{"dingtalk-a", "dingtalk-shared"}, nil
	})
	testseam.Swap(t, &skillSetupFilterMulti, filterMultiSkillNames)
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
	testseam.Swap(t, &skillSetupInteractive, func() bool { return true })
	testseam.Swap(t, &skillSetupRunForm, func(*huh.Form) error { return nil })
	testseam.Swap(t, &skillSetupWriteState, func(string, skillstate.State) error { return nil })

	wantBackups := []string{
		filepath.Join(dest, "dingtalk-a"),
		filepath.Join(dest, "dingtalk-stale"),
		filepath.Join(dest, "dws"),
	}

	// Dry-run must disclose every exact path and perform no backup or copy.
	backupCalls, copyCalls := []string{}, 0
	testseam.Swap(t, &skillSetupBackupAndRemove, func(_ string, path string) (string, error) {
		backupCalls = append(backupCalls, path)
		return "backup", nil
	})
	testseam.Swap(t, &skillSetupCopyDir, func(string, string) error { copyCalls++; return nil })
	dryRunCmd := skillSetupCoverageCommand(t, skillSetupModeMulti, false)
	var dryRunOut bytes.Buffer
	dryRunCmd.SetOut(&dryRunOut)
	if err := dryRunCmd.Root().PersistentFlags().Set("dry-run", "true"); err != nil {
		t.Fatal(err)
	}
	if err := dryRunCmd.RunE(dryRunCmd, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range wantBackups {
		if strings.Count(dryRunOut.String(), path) != 1 {
			t.Fatalf("dry-run path %s count != 1:\n%s", path, dryRunOut.String())
		}
	}
	if len(backupCalls) != 0 || copyCalls != 0 {
		t.Fatalf("dry-run mutated backup=%v copy=%d", backupCalls, copyCalls)
	}

	// The real confirmation renderer discloses the same paths. Its default
	// negative answer must leave backup and copy at zero calls.
	declineCmd := skillSetupCoverageCommand(t, skillSetupModeMulti, false)
	var declineOut bytes.Buffer
	declineCmd.SetOut(&declineOut)
	if err := declineCmd.RunE(declineCmd, nil); err != nil {
		t.Fatal(err)
	}
	for _, path := range wantBackups {
		if strings.Count(declineOut.String(), path) != 1 {
			t.Fatalf("confirmation path %s count != 1:\n%s", path, declineOut.String())
		}
	}
	if len(backupCalls) != 0 || copyCalls != 0 {
		t.Fatalf("declined confirmation mutated backup=%v copy=%d", backupCalls, copyCalls)
	}

	// Explicit confirmation executes exactly the paths rendered from the plan.
	var confirmedPlan *skillSetupPlan
	testseam.Swap(t, &skillSetupConfirmPlan, func(out io.Writer, plan *skillSetupPlan) (bool, error) {
		confirmedPlan = plan
		renderSkillSetupPlan(out, plan)
		return true, nil
	})
	confirmCmd := skillSetupCoverageCommand(t, skillSetupModeMulti, false)
	if err := confirmCmd.RunE(confirmCmd, nil); err != nil {
		t.Fatal(err)
	}
	var planned []string
	for _, target := range confirmedPlan.Targets {
		for _, backup := range target.Backups {
			planned = append(planned, backup.Path)
		}
	}
	if !reflect.DeepEqual(planned, wantBackups) || !reflect.DeepEqual(backupCalls, wantBackups) {
		t.Fatalf("planned=%v executed=%v want=%v", planned, backupCalls, wantBackups)
	}
	if copyCalls != 2 {
		t.Fatalf("copy calls = %d, want 2", copyCalls)
	}

	// A filtered multi plan replaces only selected same-name skills and leaves
	// unselected siblings out of the backup set.
	filtered, err := buildSkillSetupPlan(skillSetupModeMulti, source, []string{dest}, []string{"dingtalk-a"}, true)
	if err != nil {
		t.Fatal(err)
	}
	var filteredPaths []string
	for _, backup := range filtered.Targets[0].Backups {
		filteredPaths = append(filteredPaths, backup.Path)
	}
	if !reflect.DeepEqual(filteredPaths, []string{filepath.Join(dest, "dingtalk-a"), filepath.Join(dest, "dws")}) {
		t.Fatalf("filtered backups = %v", filteredPaths)
	}
}

func TestCrossPlatformCoverageSkillSetupMonoPlanIncludesSameNameTarget(t *testing.T) {
	dest := filepath.Join(t.TempDir(), ".agents", "skills", "dws")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	plan, err := buildSkillSetupPlan(skillSetupModeMono, "source", []string{dest}, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Targets) != 1 || len(plan.Targets[0].Backups) != 1 || plan.Targets[0].Backups[0].Path != dest || plan.Targets[0].Backups[0].Reason != skillSetupBackupReplace {
		t.Fatalf("mono plan = %#v", plan)
	}
}

func TestCrossPlatformCoverageSkillSetupPlanDeduplicatesAndFailsClosed(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(dest, "dws"), 0o755); err != nil {
		t.Fatal(err)
	}
	// "dws" is synthetic but makes the mutual-exclusion target and selected
	// same-name target overlap, pinning path deduplication in the plan itself.
	plan, err := buildSkillSetupPlan(skillSetupModeMulti, "source", []string{dest}, []string{"dws"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Targets[0].Backups) != 1 || plan.Targets[0].Backups[0].Path != filepath.Join(dest, "dws") {
		t.Fatalf("deduplicated plan = %#v", plan)
	}

	failure := errors.New("scan denied")
	monoDest := filepath.Join(t.TempDir(), "agent", "dws")
	if err := os.MkdirAll(filepath.Dir(monoDest), 0o755); err != nil {
		t.Fatal(err)
	}
	testseam.Swap(t, &skillSetupStat, func(string) (os.FileInfo, error) { return nil, failure })
	if _, err := buildSkillSetupPlan(skillSetupModeMono, "source", []string{monoDest}, nil, false); err == nil || !strings.Contains(err.Error(), "\u68c0\u67e5\u5c06\u88ab\u66ff\u6362") {
		t.Fatalf("replacement stat error = %v", err)
	}

	testseam.Swap(t, &skillSetupStat, func(string) (os.FileInfo, error) { return nil, os.ErrNotExist })
	testseam.Swap(t, &skillSetupReadDir, func(string) ([]os.DirEntry, error) { return nil, failure })
	if _, err := buildSkillSetupPlan(skillSetupModeMulti, "source", []string{dest}, []string{"dingtalk-a"}, false); err == nil || !strings.Contains(err.Error(), "\u626b\u63cf\u8fc7\u671f") {
		t.Fatalf("stale scan error = %v", err)
	}
}

func TestCrossPlatformCoverageSkillSetupEmptyCleanupPlansAreNoOps(t *testing.T) {
	dest := t.TempDir()
	var out, errOut bytes.Buffer
	if err := cleanupMutualExclusion(dest, skillSetupModeMulti, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if err := removeStaleMultiSkills(dest, []string{"dingtalk-a"}, &out, &errOut); err != nil {
		t.Fatal(err)
	}
	if out.Len() != 0 || errOut.Len() != 0 {
		t.Fatalf("empty cleanup output = %q / %q", out.String(), errOut.String())
	}
}

func TestCrossPlatformCoverageSkillSetupSameNameBackupFailureSkipsTarget(t *testing.T) {
	home := t.TempDir()
	dest := filepath.Join(home, ".agents", "skills")
	plan := &skillSetupPlan{
		Mode:            skillSetupModeMulti,
		Source:          "source",
		MultiSkillNames: []string{"dingtalk-a"},
		Targets: []skillSetupTargetPlan{{
			Destination: dest,
			Backups: []skillSetupBackup{{
				Path:   filepath.Join(dest, "dingtalk-a"),
				Reason: skillSetupBackupReplace,
			}},
		}},
	}
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
	testseam.Swap(t, &skillSetupBackupAndRemove, func(string, string) (string, error) {
		return "", errors.New("backup denied")
	})
	copyCalls := 0
	testseam.Swap(t, &skillSetupCopyDir, func(string, string) error { copyCalls++; return nil })
	var out, errOut bytes.Buffer
	installed, skipped, err := executeSkillSetupPlan(plan, &out, &errOut)
	if err != nil || installed != 0 || skipped != 1 || copyCalls != 0 {
		t.Fatalf("same-name failure = (%d, %d, %v), copy=%d", installed, skipped, err, copyCalls)
	}
	if !strings.Contains(errOut.String(), "备份失败，跳过整个 Agent 目标") {
		t.Fatalf("same-name warning = %q", errOut.String())
	}
}
