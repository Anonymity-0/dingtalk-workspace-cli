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

	t.Run("concurrent creation between mkdir and rename is detected", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		seedUpgradeSkill(t, source, "payload", false)
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return errNoReplaceRenameUnsupported })
		renameErr := errors.New("simulated ENOTEMPTY: destination populated by another process")
		testseam.Swap(t, &skillPathRename, func(string, string) error { return renameErr })
		var removeCalled bool
		testseam.Swap(t, &skillPathRemove, func(path string) error {
			if path == destination {
				removeCalled = true
			}
			return os.Remove(path)
		})
		err := renameSkillPathNoReplace(source, destination)
		if !errors.Is(err, renameErr) {
			t.Fatalf("rename failure must surface, got %v", err)
		}
		if !removeCalled {
			t.Fatal("empty destination must be cleaned up after rename failure")
		}
		if _, statErr := os.Lstat(destination); !os.IsNotExist(statErr) {
			t.Fatalf("destination must not exist after cleanup, stat err=%v", statErr)
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
		testseam.Swap(t, &skillPathRename, func(string, string) error { return errors.New("rename failed") })
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
		testseam.Swap(t, &skillPathReadDir, func(string) ([]os.DirEntry, error) { return nil, os.ErrPermission })
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
		originalRename := skillPathRename
		testseam.Swap(t, &skillPathRename, func(src, dst string) error {
			if src == source {
				return os.ErrExist
			}
			if filepath.Base(dst) == "zz-second.md" {
				return os.ErrPermission
			}
			return originalRename(src, dst)
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
		if err != nil || info.Mode().Perm() != 0o755 {
			t.Fatalf("published mode = %v, %v; want 0755", info.Mode(), err)
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
