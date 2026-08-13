package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func TestSkillSetupCanonicalTargetsAndAgentCapabilities(t *testing.T) {
	home := t.TempDir()
	testseam.Swap(t, &skillSetupUserHomeDir, func() (string, error) { return home, nil })
	testseam.Swap(t, &skillSetupAgentHomes, []string{
		".agents/skills", ".codex/skills", ".claude/skills", ".openclaw/skills",
	})
	for _, parent := range []string{".codex", ".claude", ".openclaw"} {
		if err := os.MkdirAll(filepath.Join(home, parent), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	dests, err := resolveSkillSetupTargets("all", skillSetupModeMulti)
	if err != nil {
		t.Fatal(err)
	}
	canonical := filepath.Join(home, ".agents", "skills")
	if len(dests) != 4 || dests[0] != canonical {
		t.Fatalf("targets = %v", dests)
	}

	src := t.TempDir()
	for _, name := range []string{"dingtalk-chat", "dingtalk-shared"} {
		dir := filepath.Join(src, name)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	oldCodex := filepath.Join(home, ".codex", "skills", "dingtalk-chat")
	if err := os.MkdirAll(oldCodex, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldCodex, "SKILL.md"), []byte("beta.6"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan, err := buildSkillSetupPlan(skillSetupModeMulti, src, dests, []string{"dingtalk-chat", "dingtalk-shared"}, false)
	if err != nil {
		t.Fatal(err)
	}
	installed, skipped, err := executeSkillSetupPlan(plan, &bytes.Buffer{}, &bytes.Buffer{})
	if err != nil || skipped != 0 || installed != 6 { // canonical + two linked Agents, two Skills each
		t.Fatalf("execute = installed %d skipped %d err %v", installed, skipped, err)
	}
	if _, err := os.Lstat(oldCodex); !os.IsNotExist(err) {
		t.Fatalf("Codex duplicate remains: %v", err)
	}
	for _, name := range []string{"dingtalk-chat", "dingtalk-shared"} {
		if _, err := os.Stat(filepath.Join(canonical, name, "SKILL.md")); err != nil {
			t.Fatalf("canonical %s missing: %v", name, err)
		}
		for _, agent := range []string{".claude", ".openclaw"} {
			link := filepath.Join(home, agent, "skills", name)
			info, err := os.Lstat(link)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("link %s = %#v, %v", link, info, err)
			}
		}
	}
	// Re-running setup must recognize the existing links as already correct;
	// canonical refreshes in place without turning links into copied trees.
	plan, err = buildSkillSetupPlan(skillSetupModeMulti, src, dests, []string{"dingtalk-chat", "dingtalk-shared"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := executeSkillSetupPlan(plan, &bytes.Buffer{}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	for _, agent := range []string{".claude", ".openclaw"} {
		info, err := os.Lstat(filepath.Join(home, agent, "skills", "dingtalk-chat"))
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("idempotent setup replaced %s link: %#v, %v", agent, info, err)
		}
	}
}
