package upgrade

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

// TestCrossPlatformCoverageNoReplaceRenameFallback pins the degradation path for
// filesystems that reject RENAME_NOREPLACE / RENAME_EXCL (NFS, FUSE, overlayfs).
// Those installs must still publish, and must still refuse to clobber an
// occupied destination.
func TestCrossPlatformCoverageNoReplaceRenameFallback(t *testing.T) {
	unsupported := []error{syscall.EINVAL, syscall.ENOSYS, errNoReplaceRenameUnsupported}

	t.Run("publishes when the filesystem rejects the flag", func(t *testing.T) {
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

	t.Run("still refuses an occupied destination", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "source")
		destination := filepath.Join(dir, "destination")
		for _, path := range []string{source, destination} {
			if err := os.WriteFile(path, []byte(filepath.Base(path)), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		testseam.Swap(t, &skillPathRenameNoReplaceAtomic, func(string, string) error { return syscall.EINVAL })
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

	t.Run("cross-device errors still reach the copy fallback", func(t *testing.T) {
		if isNoReplaceRenameUnsupported(syscall.EXDEV) {
			t.Fatal("EXDEV must not be treated as an unsupported flag")
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
				return syscall.EXDEV
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
}
