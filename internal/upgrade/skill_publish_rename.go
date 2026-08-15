package upgrade

import (
	"errors"
	"fmt"
	"os"
)

// errNoReplaceRenameUnsupported marks a platform that has no atomic no-replace
// rename primitive at all.
var errNoReplaceRenameUnsupported = errors.New("平台不支持原子 no-replace rename")

var skillPathRenameNoReplaceAtomic = renameSkillPathNoReplaceAtomic

// renameSkillPathNoReplace moves source onto destination and never replaces an
// object that already occupies destination.
//
// The atomic primitives (Linux RENAME_NOREPLACE, Darwin RENAME_EXCL) require
// support from the underlying filesystem and report EINVAL/ENOTSUP when it is
// missing — rename(2) lists only ext4, btrfs, tmpfs and cifs, so NFS, FUSE and
// overlayfs homes reject the flag. Failing there would abort the whole install
// on those filesystems, so they degrade to an explicit existence check plus a
// plain rename: the no-clobber contract is preserved, only its atomicity is
// lost. That check-then-rename window is the same exposure a plain rename
// always had, and callers additionally verify identity after publication.
func renameSkillPathNoReplace(source, destination string) error {
	err := skillPathRenameNoReplaceAtomic(source, destination)
	if err == nil {
		return nil
	}
	if !errors.Is(err, errNoReplaceRenameUnsupported) && !isNoReplaceRenameUnsupported(err) {
		return err
	}
	if _, statErr := skillPathLstat(destination); statErr == nil {
		return &os.LinkError{Op: "rename", Old: source, New: destination, Err: os.ErrExist}
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("检查发布目标失败 %s: %w", destination, statErr)
	}
	return skillPathRename(source, destination)
}
