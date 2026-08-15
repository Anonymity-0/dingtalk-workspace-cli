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
	// point before and after rename, so Windows ownership relies on the
	// separately checked creation-time incarnation plus the lexical fingerprint.
	// That pair is strong evidence but not proof: NTFS file tunneling restores
	// the original creation time when a same-named object is recreated in the
	// same directory shortly afterwards, so a byte-identical concurrent
	// replacement can still be attributed to this transaction.
	return true
}
