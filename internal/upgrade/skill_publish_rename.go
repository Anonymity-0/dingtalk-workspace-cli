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
// on those filesystems, so the fallback uses truly atomic no-clobber primitives
// that work on every POSIX filesystem: os.Mkdir for directories (fails with
// EEXIST if the path is occupied) and os.Link for regular files (fails with
// EEXIST if the path is occupied). Neither primitive has a TOCTOU window.
func renameSkillPathNoReplace(source, destination string) error {
	err := skillPathRenameNoReplaceAtomic(source, destination)
	if err == nil {
		return nil
	}
	if !errors.Is(err, errNoReplaceRenameUnsupported) && !isNoReplaceRenameUnsupported(err) {
		return err
	}
	return renameSkillPathNoReplaceFallback(source, destination)
}

// renameSkillPathNoReplaceFallback is used when the kernel no-replace rename
// flag is unavailable. It dispatches on the source type to pick an atomic
// no-clobber primitive that does not require filesystem support.
func renameSkillPathNoReplaceFallback(source, destination string) error {
	info, err := skillPathLstat(source)
	if err != nil {
		return fmt.Errorf("读取源 Skill 身份失败 %s: %w", source, err)
	}
	switch {
	case info.IsDir():
		return renameSkillDirNoReplaceFallback(source, destination)
	case info.Mode().IsRegular():
		return renameSkillFileNoReplaceFallback(source, destination)
	default:
		return &os.LinkError{
			Op: "rename", Old: source, New: destination,
			Err: fmt.Errorf("%w: 源路径类型不支持降级发布", errNoReplaceRenameUnsupported),
		}
	}
}

// renameSkillDirNoReplaceFallback atomically claims the destination with
// mkdir (which fails with EEXIST if the path is already occupied) and then
// replaces the empty, self-owned directory with a plain rename. On Linux
// rename(2) replaces an empty directory directly. On Darwin and Windows
// rename(2) refuses to replace any existing directory, so the empty dir is
// removed and the rename retried. If someone populated the claimed directory
// between mkdir and remove, Remove fails (ENOTEMPTY) and no data is lost.
// If someone creates a new path at the destination between remove and rename,
// the second Rename detects it and fails. Neither step has a TOCTOU window
// that could silently clobber data.
func renameSkillDirNoReplaceFallback(source, destination string) error {
	if err := skillPathMkdir(destination, 0o700); err != nil {
		if os.IsExist(err) {
			return &os.LinkError{Op: "rename", Old: source, New: destination, Err: os.ErrExist}
		}
		return fmt.Errorf("声明 Skill 发布目标失败 %s: %w", destination, err)
	}
	if err := skillPathRename(source, destination); err == nil {
		return nil
	}
	if rmErr := skillPathRemove(destination); rmErr != nil {
		return fmt.Errorf("清理已声明 Skill 目标失败 %s: %w", destination, rmErr)
	}
	if err := skillPathRename(source, destination); err != nil {
		return fmt.Errorf("发布 Skill 到已声明目标失败 %s: %w", destination, err)
	}
	return nil
}

// renameSkillFileNoReplaceFallback uses os.Link, which atomically fails if the
// destination exists, then removes the source to complete the move.
func renameSkillFileNoReplaceFallback(source, destination string) error {
	if err := skillPathLink(source, destination); err != nil {
		if os.IsExist(err) {
			return &os.LinkError{Op: "rename", Old: source, New: destination, Err: os.ErrExist}
		}
		return fmt.Errorf("发布 Skill 文件失败 %s: %w", destination, err)
	}
	return skillPathRemove(source)
}
