package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

type mockSpecialInfo struct{}

func (mockSpecialInfo) Name() string       { return "source" }
func (mockSpecialInfo) Size() int64        { return 0 }
func (mockSpecialInfo) Mode() os.FileMode  { return os.ModeNamedPipe }
func (mockSpecialInfo) ModTime() time.Time { return time.Time{} }
func (mockSpecialInfo) IsDir() bool        { return false }
func (mockSpecialInfo) Sys() interface{}   { return nil }

// assertSourceConsumed accepts both publication shapes: a rename that
// consumed the source entirely, and the child-move slow path that leaves an
// emptied source shell behind as its identity-changed signal.
func assertSourceConsumed(t *testing.T, source string) {
	t.Helper()
	info, err := os.Lstat(source)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		t.Fatalf("source stat = %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("leftover source must be a shell directory, mode=%v", info.Mode())
	}
	entries, readErr := os.ReadDir(source)
	if readErr != nil || len(entries) != 0 {
		t.Fatalf("leftover source shell must be empty, entries=%v, err=%v", entries, readErr)
	}
}

// TestCrossPlatformCoverageNoReplaceRenameFallback pins the degradation path for
// filesystems that reject RENAME_NOREPLACE / RENAME_EXCL (NFS, FUSE, overlayfs).
// Those installs must still publish, and must still refuse to clobber an
// occupied destination. The fallback uses atomic no-clobber primitives
// (os.Mkdir for directories, os.Link for files) instead of a TOCTOU-prone
// check-then-rename.
func TestCrossPlatformCoverageNoReplaceRenameFallback(t *testing.T) {
	unsupported := testNoReplaceUnsupportedErrors()

	t.Run("publishes a file when the filesystem rejects the flag", func(t *testing.T) {
		for _, unsupportedErr := range unsupported {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			destination := filepath.Join(dir, "destination")
			if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
				t.Fatal(err)
			}
			testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return unsupportedErr })
			if err := renameSkillPathNoReplace(source, destination); err != nil {
				t.Fatalf("fallback must publish for %v, got %v", unsupportedErr, err)
			}
			if data, err := os.ReadFile(destination); err != nil || string(data) != "payload" {
				t.Fatalf("destination = %q, %v", data, err)
			}
			if _, err := os.Lstat(source); !os.IsNotExist(err) {
				t.Fatalf("source must be gone, stat err=%v", err)
			}
		}
	})

	t.Run("publishes a directory when the filesystem rejects the flag", func(t *testing.T) {
		for _, unsupportedErr := range unsupported {
			dir := t.TempDir()
			source := filepath.Join(dir, "source")
			destination := filepath.Join(dir, "destination")
			seedUpgradeSkill(t, source, "payload", false)
			testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return unsupportedErr })
			if err := renameSkillPathNoReplace(source, destination); err != nil {
				t.Fatalf("fallback must publish for %v, got %v", unsupportedErr, err)
			}
			assertUpgradeSkillContent(t, destination, "payload")
			assertSourceConsumed(t, source)
		}
	})

	t.Run("still refuses an occupied destination for files", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		for _, path := range []string{source, destination} {
			if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("occupied destination must report ErrExist, got %v", err)
		}
		if data, readErr := os.ReadFile(destination); readErr != nil || string(data) != "destination" {
			t.Fatalf("destination must be untouched, got %q, %v", data, readErr)
		}
		if _, statErr := os.Lstat(source); statErr != nil {
			t.Fatalf("source must be preserved: %v", statErr)
		}
	})

	t.Run("file link failure with non-EEXIST error surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		testseam.Swap(t, &skillPathLink, func(string, string) error { return os.ErrPermission })
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("link error must surface, got %v", err)
		}
		if _, statErr := os.Lstat(source); statErr != nil {
			t.Fatalf("source must be preserved: %v", statErr)
		}
	})

	t.Run("file source removal failure retracts the linked destination", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		originalRemove := skillPathRemove
		testseam.Swap(t, &skillPathRemove, func(path string) error {
			if path == source {
				return os.ErrPermission
			}
			return originalRemove(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "移动 Skill 文件失败") {
			t.Fatalf("source removal failure must surface, got %v", err)
		}
		if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
			t.Fatalf("linked destination must be retracted, stat err=%v", statErr)
		}
		if content, readErr := os.ReadFile(source); readErr != nil || string(content) != "payload" {
			t.Fatalf("source must stay intact, content=%q err=%v", content, readErr)
		}
	})

	t.Run("file source removal failure with unremovable destination keeps both", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		testseam.Swap(t, &skillPathRemove, func(string) error { return os.ErrPermission })
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "撤回已链接 Skill 文件失败") {
			t.Fatalf("retract failure must be reported, got %v", err)
		}
		if _, statErr := os.Lstat(destination); statErr != nil {
			t.Fatalf("destination must be reported as retained, stat err=%v", statErr)
		}
		if _, statErr := os.Lstat(source); statErr != nil {
			t.Fatalf("source must stay intact, stat err=%v", statErr)
		}
	})

	t.Run("file retract refuses a replaced destination", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		testseam.Swap(t, &skillPathSameFileIdentity, func(_, _ os.FileInfo) bool { return false })
		originalFileIdentity := skillPathFileIdentity
		stagedID := ""
		testseam.Swap(t, &skillPathFileIdentity, func(path string) string {
			id := originalFileIdentity(path)
			if stagedID == "" {
				stagedID = id
				return id
			}
			return id + ":swapped"
		})
		originalRemove := skillPathRemove
		testseam.Swap(t, &skillPathRemove, func(path string) error {
			if path == source {
				return os.ErrPermission
			}
			return originalRemove(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "待撤回 Skill 文件已被替换") {
			t.Fatalf("replaced destination must be refused, got %v", err)
		}
		if _, statErr := os.Lstat(destination); statErr != nil {
			t.Fatalf("replaced destination must be retained, stat err=%v", statErr)
		}
	})

	t.Run("file fallback source stat failure surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		calls := 0
		originalLstat := skillPathLstat
		testseam.Swap(t, &skillPathLstat, func(path string) (os.FileInfo, error) {
			calls++
			if calls > 1 {
				return nil, os.ErrPermission
			}
			return originalLstat(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "读取源 Skill 身份失败") {
			t.Fatalf("file fallback stat failure must surface, got %v", err)
		}
		if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
			t.Fatalf("destination must stay unlinked, stat err=%v", statErr)
		}
	})

	t.Run("file retract destination stat failure surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		linked := false
		originalLink := skillPathLink
		testseam.Swap(t, &skillPathLink, func(src, dst string) error {
			err := originalLink(src, dst)
			linked = err == nil
			return err
		})
		originalLstat := skillPathLstat
		testseam.Swap(t, &skillPathLstat, func(path string) (os.FileInfo, error) {
			if path == destination && linked {
				return nil, os.ErrPermission
			}
			return originalLstat(path)
		})
		originalRemove := skillPathRemove
		testseam.Swap(t, &skillPathRemove, func(path string) error {
			if path == source {
				return os.ErrPermission
			}
			return originalRemove(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "确认待撤回 Skill 文件失败") {
			t.Fatalf("retract stat failure must surface, got %v", err)
		}
		if _, statErr := os.Lstat(destination); statErr != nil {
			t.Fatalf("destination must be retained when its state is unreadable, stat err=%v", statErr)
		}
	})

	t.Run("refuses an occupied directory destination via mkdir EEXIST", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		seedUpgradeSkill(t, destination, "existing", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("occupied destination must report ErrExist, got %v", err)
		}
		assertUpgradeSkillContent(t, destination, "existing")
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("foreign different-name entry in the claim aborts before child moves", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		// A concurrent writer lands inside the claim between the mkdir claim
		// and the fast rename: the rename fails, and the claim re-read must
		// see the foreign entry and abort with the destination retained
		// instead of falling through to per-child moves whose rollback would
		// delete it.
		originalRename := skillPathRename
		testseam.Swap(t, &skillPathRename, func(src, dst string) error {
			if src == source && dst == destination {
				if err := os.WriteFile(filepath.Join(destination, "concurrent-user-data.txt"), []byte("foreign\n"), 0o644); err != nil {
					return err
				}
				return os.ErrExist
			}
			return originalRename(src, dst)
		})
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("foreign claim entry must abort with ErrExist, got %v", err)
		}
		got, readErr := os.ReadFile(filepath.Join(destination, "concurrent-user-data.txt"))
		if readErr != nil || string(got) != "foreign\n" {
			t.Fatalf("foreign entry must survive at the destination: %q, %v", got, readErr)
		}
		if _, statErr := os.Lstat(filepath.Join(destination, "SKILL.md")); !os.IsNotExist(statErr) {
			t.Fatalf("no source child may be moved after a foreign entry is seen: %v", statErr)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("foreign same-name file in the claim is never replaced", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		// The claim is empty when read, so the child moves start; the
		// concurrent file uses the same name as a source child and the
		// per-child link primitive must fail atomically instead of replacing
		// it.
		originalLink := skillPathLink
		testseam.Swap(t, &skillPathLink, func(oldname, newname string) error {
			if filepath.Base(newname) == "SKILL.md" {
				if err := os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("concurrent\n"), 0o644); err != nil {
					return err
				}
			}
			return originalLink(oldname, newname)
		})
		forceFastClaimRenameFailure(t, source, destination)
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("same-name concurrent file must abort with ErrExist, got %v", err)
		}
		got, readErr := os.ReadFile(filepath.Join(destination, "SKILL.md"))
		if readErr != nil || string(got) != "concurrent\n" {
			t.Fatalf("concurrent file must stay byte-identical: %q, %v", got, readErr)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("foreign different-name entry mid child-move aborts and is retained", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		forceFastClaimRenameFailure(t, source, destination)
		// The live re-read after the child moves observes an entry count above
		// the source's: a different-named foreign object landed mid-move. The
		// publication aborts, the moved children return to the source, and the
		// claim is only removed while empty — so the foreign entry survives.
		claimReads := 0
		originalReadDir := skillPathReadDir
		testseam.Swap(t, &skillPathReadDir, func(path string) ([]os.DirEntry, error) {
			entries, err := originalReadDir(path)
			if err == nil && path == destination {
				claimReads++
				if claimReads == 2 {
					if err := os.WriteFile(filepath.Join(destination, "concurrent-mid.txt"), []byte("mid\n"), 0o644); err != nil {
						return nil, err
					}
					return originalReadDir(path)
				}
			}
			return entries, err
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "并发目录项") {
			t.Fatalf("mid-move foreign entry must abort, got %v", err)
		}
		got, readErr := os.ReadFile(filepath.Join(destination, "concurrent-mid.txt"))
		if readErr != nil || string(got) != "mid\n" {
			t.Fatalf("mid-move foreign entry must survive: %q, %v", got, readErr)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("a genuine error is not retried", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		calls := 0
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error {
			calls++
			return os.ErrPermission
		})
		if err := renameSkillPathNoReplace(source, filepath.Join(dir, "destination")); !errors.Is(err, os.ErrPermission) {
			t.Fatalf("permission error must surface, got %v", err)
		}
		if calls != 1 {
			t.Fatalf("atomic attempts = %d, want 1", calls)
		}
	})

	t.Run("source stat failure surfaces without rename", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(dir, "destination")
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		testseam.Swap(t, &skillPathLstat, func(string) (os.FileInfo, error) { return nil, os.ErrPermission })
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("stat error must surface, got %v", err)
		}
		if _, statErr := os.Lstat(source); statErr != nil {
			t.Fatalf("source must be preserved: %v", statErr)
		}
	})

	t.Run("cross-device errors still reach the copy fallback", func(t *testing.T) {
		crossDevice := testCrossDeviceError()
		if isNoReplaceRenameUnsupported(crossDevice) {
			t.Fatal("cross-device error must not be treated as an unsupported flag")
		}
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(dir, "backup", "destination")
		original := skillPathRenameNoReplaceAtomic
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(from, to string) error {
			if from == source {
				return crossDevice
			}
			return original(from, to)
		})
		if err := moveSkillPathRecoverably(source, destination); err != nil {
			t.Fatalf("cross-device move = %v", err)
		}
		if data, err := os.ReadFile(filepath.Join(destination, "SKILL.md")); err != nil || string(data) != "payload" {
			t.Fatalf("cross-device destination = %q, %v", data, err)
		}
	})

	t.Run("mkdir failure with non-EEXIST error surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		testseam.Swap(t, &skillPathMkdir, func(string, os.FileMode) error { return os.ErrPermission })
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("mkdir error must surface, got %v", err)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("child move failure rolls back and surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		forceFastClaimRenameFailure(t, source, destination)
		testseam.Swap(t, &skillPathLink, func(string, string) error { return os.ErrPermission })
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "降级发布 Skill 目录失败") {
			t.Fatalf("slow-path failure must surface, got %v", err)
		}
		if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
			t.Fatalf("claim must be removed after rollback, stat err=%v", statErr)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("directory fallback source stat failure surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		// The dispatch lstat must succeed so the directory fallback runs;
		// its own identity probe then fails.
		calls := 0
		originalLstat := skillPathLstat
		testseam.Swap(t, &skillPathLstat, func(path string) (os.FileInfo, error) {
			calls++
			if calls > 1 {
				return nil, os.ErrPermission
			}
			return originalLstat(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "读取源 Skill 身份失败") {
			t.Fatalf("fallback stat failure must surface, got %v", err)
		}
		if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
			t.Fatalf("destination must stay unclaimed, stat err=%v", statErr)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("directory fallback readdir failure removes the claim", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		forceFastClaimRenameFailure(t, source, destination)
		testseam.Swap(t, &skillPathReadDir, func(path string) ([]os.DirEntry, error) {
			if path == source {
				return nil, os.ErrPermission
			}
			return os.ReadDir(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "读取源 Skill 目录失败") {
			t.Fatalf("readdir failure must surface, got %v", err)
		}
		if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
			t.Fatalf("claim must be removed after readdir failure, stat err=%v", statErr)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("directory fallback child failure returns already moved children", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "first", false)
		if err := os.WriteFile(filepath.Join(source, "zz-second.md"), []byte("second"), 0o644); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		forceFastClaimRenameFailure(t, source, destination)
		originalLink := skillPathLink
		testseam.Swap(t, &skillPathLink, func(oldname, newname string) error {
			if filepath.Base(newname) == "zz-second.md" {
				return os.ErrPermission
			}
			return originalLink(oldname, newname)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "迁移 Skill 目录项失败") {
			t.Fatalf("child failure must surface, got %v", err)
		}
		if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
			t.Fatalf("claim must be removed after rollback, stat err=%v", statErr)
		}
		assertUpgradeSkillContent(t, source, "first")
		if content, readErr := os.ReadFile(filepath.Join(source, "zz-second.md")); readErr != nil || string(content) != "second" {
			t.Fatalf("second child must stay in the source, content=%q err=%v", content, readErr)
		}
	})

	t.Run("directory fallback chmod failure rolls back", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		forceFastClaimRenameFailure(t, source, destination)
		originalChmod := skillPathChmod
		testseam.Swap(t, &skillPathChmod, func(path string, mode os.FileMode) error {
			if path == destination {
				return os.ErrPermission
			}
			return originalChmod(path, mode)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "恢复已发布 Skill 目录权限失败") {
			t.Fatalf("chmod failure must surface, got %v", err)
		}
		if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
			t.Fatalf("claim must be removed after chmod rollback, stat err=%v", statErr)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("directory slow path moves children into the claim", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		sourceInfo, err := os.Lstat(source)
		if err != nil {
			t.Fatal(err)
		}
		sourceMode := sourceInfo.Mode().Perm()
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		calls := 0
		originalRename := skillPathRename
		testseam.Swap(t, &skillPathRename, func(src, dst string) error {
			calls++
			if calls == 1 {
				return os.ErrExist
			}
			return originalRename(src, dst)
		})
		if err := renameSkillPathNoReplace(source, destination); err != nil {
			t.Fatalf("slow path must publish, got %v", err)
		}
		assertUpgradeSkillContent(t, destination, "payload")
		assertSourceConsumed(t, source)
		info, err := os.Lstat(destination)
		if err != nil || info.Mode().Perm() != sourceMode {
			t.Fatalf("published mode = %v, %v; want source mode %v", info.Mode(), err, sourceMode)
		}
	})

	t.Run("directory publish when first rename replaces empty dir", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		originalRename := skillPathRename
		testseam.Swap(t, &skillPathRename, func(src, dst string) error {
			_ = os.Remove(dst)
			return originalRename(src, dst)
		})
		if err := renameSkillPathNoReplace(source, destination); err != nil {
			t.Fatalf("publish must succeed when rename replaces empty dir, got %v", err)
		}
		assertUpgradeSkillContent(t, destination, "payload")
	})

	t.Run("non-regular non-directory source fails safely", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.WriteFile(source, []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		testseam.Swap(t, &skillPathLstat, func(string) (os.FileInfo, error) { return mockSpecialInfo{}, nil })
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, errNoReplaceRenameUnsupported) {
			t.Fatalf("unsupported error must surface for special files, got %v", err)
		}
	})
}

// TestCrossPlatformCoverageNoReplaceRenameFallbackNeverUnlinksClaim locks in
// the invariant the unlink→rename redesign provides. The previous design
// removed its mkdir claim and retried a plain rename, opening a window in
// which a foreign directory could appear at the destination and be silently
// overwritten. The claim is now held for the whole transaction, so the
// destination is never unlinked during a successful publish and a concurrent
// creator can only ever lose the initial mkdir race. This is asserted
// platform-independently by forcing the slow path and recording every remove.
func TestCrossPlatformCoverageNoReplaceRenameFallbackNeverUnlinksClaim(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	destination := filepath.Join(dir, "destination")
	seedUpgradeSkill(t, source, "payload", false)
	testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })

	// Force the slow path: refuse the fast rename, then move children for real.
	originalRename := skillPathRename
	first := true
	testseam.Swap(t, &skillPathRename, func(src, dst string) error {
		if first && src == source {
			first = false
			return os.ErrExist
		}
		return originalRename(src, dst)
	})

	originalRemove := skillPathRemove
	var removed []string
	testseam.Swap(t, &skillPathRemove, func(path string) error {
		removed = append(removed, path)
		return originalRemove(path)
	})

	if err := renameSkillPathNoReplace(source, destination); err != nil {
		t.Fatalf("slow path must publish, got %v", err)
	}
	for _, path := range removed {
		if path == destination {
			t.Fatalf("destination claim must never be unlinked during publish, removed=%v", removed)
		}
	}
	assertUpgradeSkillContent(t, destination, "payload")
	assertSourceConsumed(t, source)
}

// forceFastClaimRenameFailure makes the plain rename of source onto the just
// claimed destination fail so the child-move slow path runs, while every
// later rename (the children themselves) executes for real.
func forceFastClaimRenameFailure(t *testing.T, source, destination string) {
	t.Helper()
	originalRename := skillPathRename
	testseam.Swap(t, &skillPathRename, func(src, dst string) error {
		if src == source && dst == destination {
			return os.ErrExist
		}
		return originalRename(src, dst)
	})
}

// TestCrossPlatformCoverageDegradedChildPrimitives pins every branch of the
// per-child atomic no-clobber primitives the degraded slow path dispatches
// to: symlink children (success, EEXIST, creation failure, source-removal
// retraction, readlink failure), nested directory shells, the child stat
// gate, the claim re-read gate, and the empty-only claim cleanup.
func TestCrossPlatformCoverageDegradedChildPrimitives(t *testing.T) {
	forceSlowPath := func(t *testing.T, source, destination string) {
		t.Helper()
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		forceFastClaimRenameFailure(t, source, destination)
	}

	t.Run("symlink child publishes through the slow path", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("payload"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("SKILL.md", filepath.Join(source, "self-link")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		forceSlowPath(t, source, destination)
		if err := renameSkillPathNoReplace(source, destination); err != nil {
			t.Fatalf("slow path must publish a symlink child, got %v", err)
		}
		if target, err := os.Readlink(filepath.Join(destination, "self-link")); err != nil || target != "SKILL.md" {
			t.Fatalf("published link = %q, %v; want target SKILL.md", target, err)
		}
		assertSourceConsumed(t, source)
	})

	t.Run("symlink child refuses a concurrent same-name link", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("foreign-target", filepath.Join(source, "self-link")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		forceSlowPath(t, source, destination)
		originalSymlink := skillPathSymlink
		testseam.Swap(t, &skillPathSymlink, func(target, linkname string) error {
			if err := os.Symlink("concurrent-target", linkname); err != nil {
				return err
			}
			return originalSymlink(target, linkname)
		})
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("same-name concurrent link must abort with ErrExist, got %v", err)
		}
		if target, readErr := os.Readlink(filepath.Join(destination, "self-link")); readErr != nil || target != "concurrent-target" {
			t.Fatalf("concurrent link must be preserved, target=%q, %v", target, readErr)
		}
		if target, readErr := os.Readlink(filepath.Join(source, "self-link")); readErr != nil || target != "foreign-target" {
			t.Fatalf("source link must be preserved, target=%q, %v", target, readErr)
		}
	})

	t.Run("symlink creation failure surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("target", filepath.Join(source, "self-link")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		forceSlowPath(t, source, destination)
		testseam.Swap(t, &skillPathSymlink, func(string, string) error { return os.ErrPermission })
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "发布 Skill 链接失败") {
			t.Fatalf("link creation failure must surface, got %v", err)
		}
	})

	t.Run("symlink source readlink failure surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("target", filepath.Join(source, "self-link")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		forceSlowPath(t, source, destination)
		testseam.Swap(t, &skillPathReadlink, func(string) (string, error) { return "", os.ErrPermission })
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "读取源 Skill 链接失败") {
			t.Fatalf("readlink failure must surface, got %v", err)
		}
	})

	t.Run("symlink source removal failure retracts the published link", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		if err := os.MkdirAll(source, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("target", filepath.Join(source, "self-link")); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}
		forceSlowPath(t, source, destination)
		linkSource := filepath.Join(source, "self-link")
		testseam.Swap(t, &skillPathRemove, func(path string) error {
			if path == linkSource {
				return os.ErrPermission
			}
			return os.Remove(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "移动 Skill 链接失败") {
			t.Fatalf("link source removal failure must surface, got %v", err)
		}
		if _, statErr := os.Lstat(filepath.Join(destination, "self-link")); !os.IsNotExist(statErr) {
			t.Fatalf("unconfirmed published link must be retracted, stat err=%v", statErr)
		}
		if target, readErr := os.Readlink(linkSource); readErr != nil || target != "target" {
			t.Fatalf("source link must be preserved, target=%q, %v", target, readErr)
		}
	})

	t.Run("nested directory shell removal failure surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		if err := os.MkdirAll(filepath.Join(source, "references"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, "references", "guide.md"), []byte("guide"), 0o644); err != nil {
			t.Fatal(err)
		}
		forceSlowPath(t, source, destination)
		// Force the slow path at every depth: a rename whose destination is
		// inside the claim must never succeed, so the nested directory also
		// takes the child-move route on platforms that would otherwise allow
		// replacing an empty directory claim.
		originalRename := skillPathRename
		testseam.Swap(t, &skillPathRename, func(src, dst string) error {
			if dst == destination || strings.HasPrefix(dst, destination+string(filepath.Separator)) {
				return os.ErrExist
			}
			return originalRename(src, dst)
		})
		nested := filepath.Join(source, "references")
		testseam.Swap(t, &skillPathRemove, func(path string) error {
			if path == nested {
				return os.ErrPermission
			}
			return os.Remove(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "清理已迁移 Skill 子目录失败") {
			t.Fatalf("nested shell removal failure must surface, got %v", err)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("child stat failure surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		forceSlowPath(t, source, destination)
		originalLstat := skillPathLstat
		testseam.Swap(t, &skillPathLstat, func(path string) (os.FileInfo, error) {
			if strings.HasPrefix(path, source+string(filepath.Separator)) {
				return nil, os.ErrPermission
			}
			return originalLstat(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "读取源 Skill 目录项失败") {
			t.Fatalf("child stat failure must surface, got %v", err)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("claim re-read failure aborts without deleting the destination", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		forceSlowPath(t, source, destination)
		originalReadDir := skillPathReadDir
		testseam.Swap(t, &skillPathReadDir, func(path string) ([]os.DirEntry, error) {
			if path == destination {
				return nil, os.ErrPermission
			}
			return originalReadDir(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "降级发布 Skill 目录失败") {
			t.Fatalf("claim re-read failure must surface, got %v", err)
		}
		if _, statErr := os.Lstat(destination); statErr != nil {
			t.Fatalf("unreadable claim must be retained, stat err=%v", statErr)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("missing claim cleanup after a failed child move is a no-op", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		if err := moveSkillDirChildrenIntoClaim(source, destination, 0o755); err == nil {
			t.Fatal("a child move into a missing claim must fail")
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("cleanup of an unreadable claim reports the failure", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		if err := os.Mkdir(destination, 0o755); err != nil {
			t.Fatal(err)
		}
		claimReads := 0
		originalReadDir := skillPathReadDir
		testseam.Swap(t, &skillPathReadDir, func(path string) ([]os.DirEntry, error) {
			entries, err := originalReadDir(path)
			if err == nil && path == destination {
				claimReads++
				// The direct call's only claim read is removeClaimIfEmpty's,
				// after the injected child-move failure.
				if claimReads == 1 {
					return nil, os.ErrPermission
				}
			}
			return entries, err
		})
		testseam.Swap(t, &skillPathLink, func(string, string) error { return os.ErrPermission })
		err := moveSkillDirChildrenIntoClaim(source, destination, 0o755)
		if err == nil || !strings.Contains(err.Error(), "检查降级发布目标失败") {
			t.Fatalf("unreadable claim cleanup must surface, got %v", err)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})
}

func TestCrossPlatformCoverageDegradedChildPrimitiveErrors(t *testing.T) {
	forceSlowPath := func(t *testing.T, source, destination string) {
		t.Helper()
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		forceFastClaimRenameFailure(t, source, destination)
	}

	t.Run("nested directory publish failure propagates", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		if err := os.MkdirAll(filepath.Join(source, "references"), 0o755); err != nil {
			t.Fatal(err)
		}
		forceSlowPath(t, source, destination)
		// The nested directory's own claim is occupied: a concurrent
		// same-name directory already sits at <claim>/references, so the
		// nested mkdir returns EEXIST and the child error must propagate.
		originalMkdir := skillPathMkdir
		testseam.Swap(t, &skillPathMkdir, func(path string, mode os.FileMode) error {
			if path == filepath.Join(destination, "references") {
				return os.ErrExist
			}
			return originalMkdir(path, mode)
		})
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, os.ErrExist) {
			t.Fatalf("nested claim occupation must propagate ErrExist, got %v", err)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("special file child fails safely", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		if err := os.WriteFile(filepath.Join(source, "fifo"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		forceSlowPath(t, source, destination)
		special := filepath.Join(source, "fifo")
		originalLstat := skillPathLstat
		testseam.Swap(t, &skillPathLstat, func(path string) (os.FileInfo, error) {
			if path == special {
				return mockSpecialInfo{}, nil
			}
			return originalLstat(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, errNoReplaceRenameUnsupported) {
			t.Fatalf("special file child must fail safely, got %v", err)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})
}

func TestCrossPlatformCoverageDegradedRollbackEdges(t *testing.T) {
	forceSlowPath := func(t *testing.T, source, destination string) {
		t.Helper()
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		forceFastClaimRenameFailure(t, source, destination)
	}

	t.Run("failed cleanup of an empty claim surfaces", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		if err := os.Mkdir(destination, 0o755); err != nil {
			t.Fatal(err)
		}
		testseam.Swap(t, &skillPathLink, func(string, string) error { return os.ErrPermission })
		testseam.Swap(t, &skillPathRemove, func(path string) error {
			if path == destination {
				return os.ErrPermission
			}
			return os.Remove(path)
		})
		err := moveSkillDirChildrenIntoClaim(source, destination, 0o755)
		if err == nil || !strings.Contains(err.Error(), "清理降级发布目标失败") {
			t.Fatalf("empty-claim cleanup failure must surface, got %v", err)
		}
		assertUpgradeSkillContent(t, source, "payload")
	})

	t.Run("rollback child rename failure is reported alongside the move failure", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "first", false)
		if err := os.WriteFile(filepath.Join(source, "zz-second.md"), []byte("second"), 0o644); err != nil {
			t.Fatal(err)
		}
		forceSlowPath(t, source, destination)
		originalLink := skillPathLink
		testseam.Swap(t, &skillPathLink, func(oldname, newname string) error {
			if filepath.Base(newname) == "zz-second.md" {
				return os.ErrPermission
			}
			return originalLink(oldname, newname)
		})
		originalRename := skillPathRename
		testseam.Swap(t, &skillPathRename, func(src, dst string) error {
			if src == source && dst == destination {
				return os.ErrExist
			}
			if strings.HasPrefix(src, destination+string(filepath.Separator)) {
				return os.ErrPermission
			}
			return originalRename(src, dst)
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "迁移 Skill 目录项失败") || !strings.Contains(err.Error(), "permission denied") {
			t.Fatalf("rollback rename failure must be reported with the move failure, got %v", err)
		}
		if got, readErr := os.ReadFile(filepath.Join(destination, "SKILL.md")); readErr != nil || string(got) != "first" {
			t.Fatalf("stranded moved child must stay at the claim with its content: %q, %v", got, readErr)
		}
	})

	t.Run("live claim re-read failure rolls the moved children back", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		forceSlowPath(t, source, destination)
		claimReads := 0
		originalReadDir := skillPathReadDir
		testseam.Swap(t, &skillPathReadDir, func(path string) ([]os.DirEntry, error) {
			entries, err := originalReadDir(path)
			if err == nil && path == destination {
				claimReads++
				if claimReads == 2 {
					return nil, os.ErrPermission
				}
			}
			return entries, err
		})
		err := renameSkillPathNoReplace(source, destination)
		if err == nil || !strings.Contains(err.Error(), "确认降级发布目标失败") {
			t.Fatalf("live claim re-read failure must surface, got %v", err)
		}
		assertUpgradeSkillContent(t, source, "payload")
		if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
			t.Fatalf("emptied claim must be cleaned after rollback, stat err=%v", statErr)
		}
	})
}
