package upgrade

import (
	"crypto/sha256"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func TestCrossPlatformCoverageSkillPublicationNoClobberAndOwnedRollback(t *testing.T) {
	t.Run("existing destination is never replaced", func(t *testing.T) {
		root := t.TempDir()
		staged := filepath.Join(root, "staged")
		destination := filepath.Join(root, "destination")
		seedUpgradeSkill(t, staged, "new", false)
		seedUpgradeSkill(t, destination, "concurrent", false)

		if _, err := PublishSkillPathNoReplace(staged, destination); err == nil {
			t.Fatal("expected no-replace publication to fail")
		}
		assertUpgradeSkillContent(t, destination, "concurrent")
		assertUpgradeSkillContent(t, staged, "new")
	})

	t.Run("owned publication is removed", func(t *testing.T) {
		root := t.TempDir()
		staged := filepath.Join(root, "staged")
		destination := filepath.Join(root, "destination")
		seedUpgradeSkill(t, staged, "new", false)
		publication, err := PublishSkillPathNoReplace(staged, destination)
		if err != nil {
			t.Fatal(err)
		}
		if err := RollbackSkillPathPublications([]SkillPathPublication{publication}); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(destination); !os.IsNotExist(err) {
			t.Fatalf("owned publication remains: %v", err)
		}
	})

	t.Run("concurrent replacement is quarantined instead of deleted", func(t *testing.T) {
		root := t.TempDir()
		staged := filepath.Join(root, "staged")
		destination := filepath.Join(root, "destination")
		seedUpgradeSkill(t, staged, "new", false)
		publication, err := PublishSkillPathNoReplace(staged, destination)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(destination); err != nil {
			t.Fatal(err)
		}
		// Even a byte-for-byte identical replacement is not owned by this
		// transaction: inode identity, not content alone, is authoritative.
		seedUpgradeSkill(t, destination, "new", false)

		err = RollbackSkillPathPublications([]SkillPathPublication{publication})
		if err == nil || !strings.Contains(err.Error(), "拒绝删除非本事务") {
			t.Fatalf("replacement rollback error = %v", err)
		}
		assertUpgradeSkillContent(t, destination, "new")
	})
}

func TestCrossPlatformCoverageSkillPublicationFailureEdges(t *testing.T) {
	failure := errors.New("injected publication failure")

	t.Run("publish identity", func(t *testing.T) {
		if _, err := PublishSkillPathNoReplace(filepath.Join(t.TempDir(), "missing"), filepath.Join(t.TempDir(), "dest")); err == nil {
			t.Fatal("expected identity failure")
		}
	})

	t.Run("publish fingerprint", func(t *testing.T) {
		root := t.TempDir()
		staged := filepath.Join(root, "staged")
		seedUpgradeSkill(t, staged, "value", false)
		originalLstat := skillPathLstat
		calls := 0
		testseam.Swap(t, &skillPathLstat, func(path string) (os.FileInfo, error) {
			calls++
			if calls == 2 {
				return nil, failure
			}
			return originalLstat(path)
		})
		if _, err := PublishSkillPathNoReplace(staged, filepath.Join(root, "dest")); !errors.Is(err, failure) {
			t.Fatal("expected fingerprint failure")
		}
	})

	t.Run("rollback temp", func(t *testing.T) {
		testseam.Swap(t, &skillPathMkdirTemp, func(string, string) (string, error) { return "", failure })
		if err := RollbackSkillPathPublications([]SkillPathPublication{{Destination: "dest"}}); !errors.Is(err, failure) {
			t.Fatalf("temp error = %v", err)
		}
	})

	t.Run("missing destination cleanup", func(t *testing.T) {
		if err := RollbackSkillPathPublications([]SkillPathPublication{{Destination: filepath.Join(t.TempDir(), "missing")}}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing destination cleanup failure", func(t *testing.T) {
		testseam.Swap(t, &skillPathRemoveAll, func(string) error { return failure })
		if err := RollbackSkillPathPublications([]SkillPathPublication{{Destination: filepath.Join(t.TempDir(), "missing")}}); !errors.Is(err, failure) {
			t.Fatalf("cleanup error = %v", err)
		}
	})

	t.Run("quarantine rename", func(t *testing.T) {
		destination := filepath.Join(t.TempDir(), "destination")
		seedUpgradeSkill(t, destination, "value", false)
		testseam.Swap(t, &skillPathRename, func(string, string) error { return failure })
		if err := RollbackSkillPathPublications([]SkillPathPublication{{Destination: destination}}); !errors.Is(err, failure) {
			t.Fatalf("quarantine error = %v", err)
		}
	})

	t.Run("owned remove", func(t *testing.T) {
		root := t.TempDir()
		staged := filepath.Join(root, "staged")
		destination := filepath.Join(root, "destination")
		seedUpgradeSkill(t, staged, "value", false)
		publication, err := PublishSkillPathNoReplace(staged, destination)
		if err != nil {
			t.Fatal(err)
		}
		originalRemove := skillPathRemoveAll
		testseam.Swap(t, &skillPathRemoveAll, func(path string) error {
			if filepath.Base(path) == "payload" {
				return failure
			}
			return originalRemove(path)
		})
		if err := RollbackSkillPathPublications([]SkillPathPublication{publication}); !errors.Is(err, failure) {
			t.Fatalf("remove error = %v", err)
		}
	})

	t.Run("quarantined fingerprint", func(t *testing.T) {
		root := t.TempDir()
		staged := filepath.Join(root, "staged")
		destination := filepath.Join(root, "destination")
		seedUpgradeSkill(t, staged, "value", false)
		publication, err := PublishSkillPathNoReplace(staged, destination)
		if err != nil {
			t.Fatal(err)
		}
		originalLstat := skillPathLstat
		testseam.Swap(t, &skillPathLstat, func(path string) (os.FileInfo, error) {
			if filepath.Base(path) == "payload" {
				return nil, failure
			}
			return originalLstat(path)
		})
		if err := RollbackSkillPathPublications([]SkillPathPublication{publication}); !errors.Is(err, failure) {
			t.Fatalf("fingerprint error = %v", err)
		}
	})

	t.Run("mismatch restore and cleanup", func(t *testing.T) {
		root := t.TempDir()
		staged := filepath.Join(root, "staged")
		destination := filepath.Join(root, "destination")
		seedUpgradeSkill(t, staged, "published", false)
		publication, err := PublishSkillPathNoReplace(staged, destination)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(destination); err != nil {
			t.Fatal(err)
		}
		seedUpgradeSkill(t, destination, "replacement", false)
		originalRemove := skillPathRemoveAll
		testseam.Swap(t, &skillPathRemoveAll, func(path string) error {
			if strings.Contains(filepath.Base(path), ".destination.rollback-") {
				return failure
			}
			return originalRemove(path)
		})
		if err := RollbackSkillPathPublications([]SkillPathPublication{publication}); !errors.Is(err, failure) {
			t.Fatalf("mismatch cleanup error = %v", err)
		}
		assertUpgradeSkillContent(t, destination, "replacement")
	})

	t.Run("mismatch restore failure", func(t *testing.T) {
		root := t.TempDir()
		staged := filepath.Join(root, "staged")
		destination := filepath.Join(root, "destination")
		seedUpgradeSkill(t, staged, "published", false)
		publication, err := PublishSkillPathNoReplace(staged, destination)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(destination); err != nil {
			t.Fatal(err)
		}
		seedUpgradeSkill(t, destination, "replacement", false)
		testseam.Swap(t, &skillPathRenameNoReplace, func(string, string) error { return failure })
		if err := RollbackSkillPathPublications([]SkillPathPublication{publication}); !errors.Is(err, failure) || !strings.Contains(err.Error(), "并发对象保留") {
			t.Fatalf("mismatch restore error = %v", err)
		}
	})

	t.Run("fingerprint read failures and lexical types", func(t *testing.T) {
		t.Run("directory read", func(t *testing.T) {
			dir := t.TempDir()
			testseam.Swap(t, &skillPathReadDir, func(string) ([]os.DirEntry, error) { return nil, failure })
			if _, err := fingerprintSkillPath(dir); !errors.Is(err, failure) {
				t.Fatalf("read dir error = %v", err)
			}
		})
		t.Run("recursive child", func(t *testing.T) {
			dir := t.TempDir()
			child := filepath.Join(dir, "child")
			if err := os.WriteFile(child, []byte("value"), 0o644); err != nil {
				t.Fatal(err)
			}
			originalLstat := skillPathLstat
			testseam.Swap(t, &skillPathLstat, func(path string) (os.FileInfo, error) {
				if path == child {
					return nil, failure
				}
				return originalLstat(path)
			})
			if _, err := fingerprintSkillPath(dir); !errors.Is(err, failure) {
				t.Fatalf("child error = %v", err)
			}
		})
		t.Run("symlink read", func(t *testing.T) {
			link := filepath.Join(t.TempDir(), "link")
			if err := os.Symlink("missing", link); err != nil {
				t.Skipf("symlink unavailable: %v", err)
			}
			testseam.Swap(t, &skillPathReadlink, func(string) (string, error) { return "", failure })
			if _, err := fingerprintSkillPath(link); !errors.Is(err, failure) {
				t.Fatalf("readlink error = %v", err)
			}
		})
		t.Run("file digest", func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(file, []byte("value"), 0o644); err != nil {
				t.Fatal(err)
			}
			testseam.Swap(t, &skillPathOpen, func(string) (*os.File, error) { return nil, failure })
			if _, err := fingerprintSkillPath(file); !errors.Is(err, failure) {
				t.Fatalf("digest error = %v", err)
			}
		})
		t.Run("special", func(t *testing.T) {
			testseam.Swap(t, &skillPathLstat, func(string) (os.FileInfo, error) {
				return skillPathFakeInfo{mode: os.ModeNamedPipe}, nil
			})
			if err := fingerprintSkillPathInto(sha256.New(), "special"); err == nil {
				t.Fatal("expected special path error")
			}
		})
	})
}

func TestCrossPlatformCoverageUpgradeRollbackRetainsConcurrentReplacement(t *testing.T) {
	home := t.TempDir()
	base := filepath.Join(home, ".agents", "skills")
	first := filepath.Join(base, "dingtalk-first")
	second := filepath.Join(base, "dingtalk-second")
	seedUpgradeSkill(t, first, "old first", false)
	seedUpgradeSkill(t, second, "old second", false)
	stageRoot := filepath.Join(base, ".stage")
	stagedFirst := filepath.Join(stageRoot, "dingtalk-first")
	stagedSecond := filepath.Join(stageRoot, "dingtalk-second")
	seedUpgradeSkill(t, stagedFirst, "new first", false)
	seedUpgradeSkill(t, stagedSecond, "new second", false)

	failure := errors.New("injected second publication failure")
	originalPublish := upgradePublishSkillPath
	calls := 0
	testseam.Swap(t, &upgradePublishSkillPath, func(staged, destination string) (SkillPathPublication, error) {
		calls++
		if calls == 2 {
			if err := os.RemoveAll(first); err != nil {
				t.Fatal(err)
			}
			seedUpgradeSkill(t, first, "concurrent", false)
			return SkillPathPublication{}, failure
		}
		return originalPublish(staged, destination)
	})

	err := publishStagedSkillSet(home, []stagedSkillDir{
		{staged: stagedFirst, dest: first},
		{staged: stagedSecond, dest: second},
	}, []string{first, second})
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "拒绝删除非本事务") {
		t.Fatalf("transaction error = %v", err)
	}
	assertUpgradeSkillContent(t, first, "concurrent")
	assertUpgradeSkillContent(t, second, "old second")
	matches, globErr := filepath.Glob(filepath.Join(home, ".dws", "skill-backups", "*", ".agents-skills-dingtalk-first", "SKILL.md"))
	if globErr != nil || len(matches) != 1 {
		t.Fatalf("retained old backup = %v, %v", matches, globErr)
	}
	content, readErr := os.ReadFile(matches[0])
	if readErr != nil || string(content) != "old first" {
		t.Fatalf("retained backup content = %q, %v", content, readErr)
	}
}
