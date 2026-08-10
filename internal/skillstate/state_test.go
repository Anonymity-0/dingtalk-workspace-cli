package skillstate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestCrossPlatformCoverageSkillStatePlanMatchesLarkIncrementalModel(t *testing.T) {
	previous := &State{OfficialSkills: []string{"dingtalk-a", "dingtalk-b"}}
	plan := Plan(SyncInput{
		OfficialSkills: []string{"dingtalk-a", "dingtalk-b", "dingtalk-c", "dingtalk-c", ""},
		LocalSkills:    []string{"dingtalk-a", "custom", "dingtalk-a"},
		PreviousState:  previous,
		StateReadable:  true,
	})
	if want := []string{"dingtalk-a", "dingtalk-c"}; !reflect.DeepEqual(plan.ToUpdate, want) {
		t.Fatalf("ToUpdate = %v, want %v", plan.ToUpdate, want)
	}
	if !reflect.DeepEqual(plan.Added, []string{"dingtalk-c"}) || !reflect.DeepEqual(plan.SkippedDeleted, []string{"dingtalk-b"}) {
		t.Fatalf("plan = %#v", plan)
	}
	force := Plan(SyncInput{OfficialSkills: []string{"dingtalk-b", "dingtalk-a"}, Force: true})
	if !reflect.DeepEqual(force.ToUpdate, []string{"dingtalk-a", "dingtalk-b"}) {
		t.Fatalf("force = %#v", force)
	}
	cold := Plan(SyncInput{OfficialSkills: []string{"dingtalk-a", "dingtalk-b"}})
	if !reflect.DeepEqual(cold.ToUpdate, cold.OfficialSkills) {
		t.Fatalf("cold = %#v", cold)
	}
}

func TestCrossPlatformCoverageSkillStateReadWriteRemoveAndErrors(t *testing.T) {
	t.Setenv("DWS_CONFIG_DIR", "")
	home := t.TempDir()
	if state, readable, err := Read(home); err != nil || readable || state != nil {
		t.Fatalf("missing = %#v, %v, %v", state, readable, err)
	}
	want := State{Version: "1.2.3", OfficialSkills: []string{"dingtalk-b", "dingtalk-a", "dingtalk-a"}, UpdatedSkills: []string{"dingtalk-a"}}
	if err := Write(home, want); err != nil {
		t.Fatal(err)
	}
	got, readable, err := Read(home)
	if err != nil || !readable || !reflect.DeepEqual(got.OfficialSkills, []string{"dingtalk-a", "dingtalk-b"}) {
		t.Fatalf("round trip = %#v, %v, %v", got, readable, err)
	}
	if err := Remove(home); err != nil {
		t.Fatal(err)
	}
	if err := Remove(home); err != nil {
		t.Fatal(err)
	}
	badHome := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(Path(badHome)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(badHome), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Read(badHome); err == nil || !strings.Contains(err.Error(), "不可读") {
		t.Fatalf("malformed = %v", err)
	}
	blocked := filepath.Join(t.TempDir(), "home-file")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Write(blocked, State{}); err == nil {
		t.Fatal("blocked write succeeded")
	}
	if _, _, err := Read(blocked); err == nil {
		t.Fatal("blocked read succeeded")
	}
	if err := Remove(blocked); err == nil {
		t.Fatal("blocked remove succeeded")
	}
	configured := t.TempDir()
	t.Setenv("DWS_CONFIG_DIR", "  "+configured+"  ")
	if Path("ignored") != filepath.Join(configured, stateFile) {
		t.Fatal("configured path ignored")
	}
}
