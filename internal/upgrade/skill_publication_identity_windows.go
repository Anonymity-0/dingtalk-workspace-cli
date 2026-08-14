//go:build windows

package upgrade

import (
	"fmt"
	"os"
	"syscall"
)

func skillPathFileIncarnation(info os.FileInfo) string {
	stat, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return fmt.Sprintf("unknown:%T", info.Sys())
	}
	return fmt.Sprintf("%d:%d", stat.CreationTime.HighDateTime, stat.CreationTime.LowDateTime)
}
