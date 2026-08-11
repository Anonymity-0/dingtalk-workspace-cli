package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/skillstate"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func TestCrossPlatformCoverageIncrementalSkillUpgradeMatchesLarkModel(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", "")
	home := withFakeHome(t)
	testseam.Swap(t, &knownSkillDirs, []string{".agents/skills"})
	testseam.Swap(t, &upgradeNow, func() time.Time { return time.Date(2026, 8, 10, 1, 2, 3, 0, time.UTC) })
	base := filepath.Join(home, ".agents", "skills")
	for _, name := range []string{"dingtalk-a", "dingtalk-shared"} {
		if err := os.MkdirAll(filepath.Join(base, name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(base, name, "SKILL.md"), []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := skillstate.Write(home, skillstate.State{OfficialSkills: []string{"dingtalk-a", "dingtalk-b", "dingtalk-shared"}}); err != nil {
		t.Fatal(err)
	}
	multi := writeMultiBundle(t, t.TempDir(), "dingtalk-a", "dingtalk-b", "dingtalk-c", "dingtalk-shared")
	result, err := UpgradeSkillLocationsWithOptions(multi, SkillUpgradeOptions{Version: "1.1.0"})
	if err != nil || len(result.Failed()) != 0 {
		t.Fatalf("incremental = %#v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(base, "dingtalk-b")); !os.IsNotExist(err) {
		t.Fatalf("deleted old Skill restored: %v", err)
	}
	for _, name := range []string{"dingtalk-a", "dingtalk-c", "dingtalk-shared"} {
		if _, err := os.Stat(filepath.Join(base, name, "SKILL.md")); err != nil {
			t.Fatal(err)
		}
	}
	state, readable, err := skillstate.Read(home)
	if err != nil || !readable || !reflect.DeepEqual(state.AddedOfficialSkills, []string{"dingtalk-c"}) || !reflect.DeepEqual(state.SkippedDeletedSkills, []string{"dingtalk-b"}) {
		t.Fatalf("state = %#v, %v, %v", state, readable, err)
	}
	result, err = UpgradeSkillLocationsWithOptions(multi, SkillUpgradeOptions{Force: true})
	if err != nil || len(result.Failed()) != 0 {
		t.Fatalf("force = %#v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(base, "dingtalk-b", "SKILL.md")); err != nil {
		t.Fatalf("force restore: %v", err)
	}
}

func TestCrossPlatformCoverageIncrementalSkillUpgradeFallbacksAndHelpers(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", "")
	testseam.Swap(t, &knownSkillDirs, []string{".agents/skills"})
	multi := writeMultiBundle(t, t.TempDir(), "dingtalk-a", "dingtalk-b", "dingtalk-shared")
	home := withFakeHome(t)
	statePath := skillstate.Path(home)
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(statePath, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := UpgradeSkillLocationsWithOptions(multi, SkillUpgradeOptions{})
	if err != nil || len(result.Succeeded()) != 1 {
		t.Fatalf("cold fallback = %#v, %v", result, err)
	}
	home2 := t.TempDir()
	testseam.Swap(t, &upgradeUserHomeDir, func() (string, error) { return home2, nil })
	testseam.Swap(t, &upgradeWriteSkillState, func(string, skillstate.State) error { return errors.New("denied") })
	result, err = UpgradeSkillLocationsWithOptions(multi, SkillUpgradeOptions{})
	if err == nil || !strings.Contains(err.Error(), "状态未写入") || len(result.Succeeded()) != 1 {
		t.Fatalf("write failure = %#v, %v", result, err)
	}
	home3 := t.TempDir()
	base := filepath.Join(home3, ".agents", "skills")
	if err := os.MkdirAll(filepath.Join(base, "dingtalk-a", "SKILL.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "dingtalk-b"), []byte("file"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := listInstalledOfficialSkills(home3, []string{"dingtalk-a", "dingtalk-b"}); err != nil || len(got) != 0 {
		t.Fatalf("filtered local = %v, %v", got, err)
	}
	originalReadDir := upgradeReadDir
	testseam.Swap(t, &upgradeReadDir, func(path string) ([]os.DirEntry, error) {
		if path == base {
			return nil, errors.New("read denied")
		}
		return originalReadDir(path)
	})
	if _, err := listInstalledOfficialSkills(home3, []string{"dingtalk-a"}); err == nil {
		t.Fatal("blocked local discovery succeeded")
	}
	if got := ensureMandatoryUpgradeShared([]string{"dingtalk-a"}, []string{"dingtalk-a"}); !reflect.DeepEqual(got, []string{"dingtalk-a"}) {
		t.Fatal(got)
	}
	if got := ensureMandatoryUpgradeShared([]string{"dingtalk-a", "dingtalk-shared"}, []string{"dingtalk-shared"}); !reflect.DeepEqual(got, []string{"dingtalk-a", "dingtalk-shared"}) {
		t.Fatal(got)
	}
	if got := ensureMandatoryUpgradeShared([]string{"dingtalk-a"}, []string{"dingtalk-a", "dingtalk-shared"}); !reflect.DeepEqual(got, []string{"dingtalk-a", "dingtalk-shared"}) {
		t.Fatal(got)
	}
	if got := skippedOfficialSkills([]string{"dingtalk-shared", "dingtalk-b", "dingtalk-a"}, []string{"dingtalk-shared", "dingtalk-a"}); !reflect.DeepEqual(got, []string{"dingtalk-b"}) {
		t.Fatal(got)
	}
}
