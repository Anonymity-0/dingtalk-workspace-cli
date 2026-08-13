package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func TestCanonicalSkillLayoutMigratesUniversalCopiesAndLinksOtherAgents(t *testing.T) {
	home := withFakeHome(t)
	testseam.Swap(t, &knownSkillDirs, []string{
		".agents/skills", ".codex/skills", ".claude/skills", ".openclaw/skills",
	})

	for _, parent := range []string{".codex", ".claude", ".openclaw"} {
		if err := os.MkdirAll(filepath.Join(home, parent), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// beta.6 wrote a physical copy into Codex. The canonical migration must
	// remove it without treating a matching user/market name as DWS-owned.
	oldCodex := filepath.Join(home, ".codex", "skills", "dingtalk-chat")
	if err := os.MkdirAll(oldCodex, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oldCodex, "SKILL.md"), []byte("beta.6"), 0o644); err != nil {
		t.Fatal(err)
	}

	multiRoot := writeMultiBundle(t, t.TempDir(), "dingtalk-chat", "dingtalk-shared")
	result, err := UpgradeSkillLocationsWithOptions(multiRoot, SkillUpgradeOptions{Version: "1.0.58-beta.6"})
	if err != nil || len(result.Failed()) != 0 {
		t.Fatalf("upgrade = %#v, %v", result, err)
	}

	canonical := filepath.Join(home, ".agents", "skills")
	for _, name := range []string{"dingtalk-chat", "dingtalk-shared"} {
		if _, err := os.Stat(filepath.Join(canonical, name, "SKILL.md")); err != nil {
			t.Fatalf("canonical %s missing: %v", name, err)
		}
		if _, err := os.Lstat(filepath.Join(home, ".codex", "skills", name)); !os.IsNotExist(err) {
			t.Fatalf("universal Codex duplicate remains for %s: %v", name, err)
		}
		for _, agent := range []string{".claude", ".openclaw"} {
			link := filepath.Join(home, agent, "skills", name)
			info, err := os.Lstat(link)
			if err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("%s link = %#v, %v", link, info, err)
			}
			if _, err := os.Stat(filepath.Join(link, "SKILL.md")); err != nil {
				t.Fatalf("%s does not resolve canonical content: %v", link, err)
			}
		}
	}
	result, err = UpgradeSkillLocationsWithOptions(multiRoot, SkillUpgradeOptions{Version: "1.0.58-beta.6"})
	if err != nil || len(result.Failed()) != 0 {
		t.Fatalf("idempotent upgrade = %#v, %v", result, err)
	}
	for _, agent := range []string{".claude", ".openclaw"} {
		info, err := os.Lstat(filepath.Join(home, agent, "skills", "dingtalk-chat"))
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("idempotent upgrade replaced %s link: %#v, %v", agent, info, err)
		}
	}
}

func TestCanonicalSkillLinksFallBackToCopies(t *testing.T) {
	home := withFakeHome(t)
	testseam.Swap(t, &knownSkillDirs, []string{".agents/skills", ".claude/skills"})
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	testseam.Swap(t, &upgradeSymlink, func(string, string) error { return errors.New("links unavailable") })

	multiRoot := writeMultiBundle(t, t.TempDir(), "dingtalk-chat")
	result, err := UpgradeSkillLocations(multiRoot)
	if err != nil || len(result.Failed()) != 0 {
		t.Fatalf("upgrade = %#v, %v", result, err)
	}
	dest := filepath.Join(home, ".claude", "skills", "dingtalk-chat")
	info, err := os.Lstat(dest)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("copy fallback = %#v, %v", info, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err != nil {
		t.Fatalf("copy fallback content missing: %v", err)
	}
}
