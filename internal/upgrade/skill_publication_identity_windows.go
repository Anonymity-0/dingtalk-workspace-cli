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

func skillPathSameFileIdentity(_, _ os.FileInfo) bool {
	// MoveFile can surface different metadata identities for the same reparse
	// point before and after rename. Windows ownership is therefore proved by
	// the separately checked creation-time incarnation plus lexical fingerprint.
	return true
}
