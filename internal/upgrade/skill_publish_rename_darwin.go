//go:build darwin

package upgrade

import "golang.org/x/sys/unix"

func renameSkillPathNoReplace(source, destination string) error {
	return unix.RenamexNp(source, destination, unix.RENAME_EXCL)
}
