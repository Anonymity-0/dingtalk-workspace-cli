//go:build darwin

package upgrade

import (
	"fmt"
	"os"
	"syscall"
)

func skillPathFileIncarnationImpl(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Sprintf("unknown:%T", info.Sys())
	}
	return fmt.Sprintf("%d:%d:%d:%d:%d:%d:%d", stat.Dev, stat.Ino, stat.Gen,
		stat.Ctimespec.Sec, stat.Ctimespec.Nsec, stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}

func skillPathSameFileIdentityImpl(left, right os.FileInfo) bool {
	return os.SameFile(left, right)
}

func skillPathFileIdentityImpl(path string) string {
	var stat syscall.Stat_t
	if err := syscall.Lstat(path, &stat); err != nil {
		return ""
	}
	return fmt.Sprintf("%d:%d:%d:%d", stat.Dev, stat.Ino, stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
}

func skillPathIdentityProven(staged, published os.FileInfo, expected, actual string) bool {
	if expected != "" || actual != "" {
		return expected != "" && expected == actual
	}
	return skillPathSameFileIdentity(staged, published) &&
		skillPathFileIncarnation(staged) == skillPathFileIncarnation(published)
}
