//go:build linux

package upgrade

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func skillPathFileIncarnationImpl(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Sprintf("unknown:%T", info.Sys())
	}
	return fmt.Sprintf("%d:%d:%d:%d", stat.Dev, stat.Ino, stat.Ctim.Sec, stat.Ctim.Nsec)
}

func skillPathSameFileIdentityImpl(left, right os.FileInfo) bool {
	return os.SameFile(left, right)
}

// skillPathFileIdentityImpl records device, inode, and birth time. os.SameFile
// on Linux compares only device+inode; ext4/overlayfs recycle those numbers
// immediately, so a concurrent replacement can look like the staged object.
// ctime is omitted: writing a child updates the directory ctime and would
// make an owned dest look foreign.
// skillPathStatx is the seam behind skillPathFileIdentityImpl so tests can
// exercise the kernels/filesystems that do not report STATX_BTIME.
var skillPathStatx = func(path string, stx *unix.Statx_t) error {
	return unix.Statx(unix.AT_FDCWD, path, unix.AT_SYMLINK_NOFOLLOW, unix.STATX_BASIC_STATS|unix.STATX_BTIME, stx)
}

func skillPathFileIdentityImpl(path string) string {
	var stx unix.Statx_t
	if err := skillPathStatx(path, &stx); err != nil {
		return ""
	}
	if stx.Mask&unix.STATX_BTIME == 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d:%d:%d:%d", stx.Dev_major, stx.Dev_minor, stx.Ino, stx.Btime.Sec, stx.Btime.Nsec)
}

// skillPathIdentityProven prefers the Statx token. An empty expected ID means
// identity cannot be proven and the caller must refuse auto-delete.
func skillPathIdentityProven(staged, published os.FileInfo, expected, actual string) bool {
	if expected != "" || actual != "" {
		return expected != "" && expected == actual
	}
	return skillPathSameFileIdentity(staged, published) &&
		skillPathFileIncarnation(staged) == skillPathFileIncarnation(published)
}
