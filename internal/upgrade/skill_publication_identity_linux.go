//go:build linux

package upgrade

import (
	"fmt"
	"os"
	"syscall"
)

func skillPathFileIncarnation(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Sprintf("unknown:%T", info.Sys())
	}
	return fmt.Sprintf("%d:%d:%d:%d", stat.Dev, stat.Ino, stat.Ctim.Sec, stat.Ctim.Nsec)
}
