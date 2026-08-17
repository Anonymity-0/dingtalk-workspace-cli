package upgrade

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// renameSkillDirNoReplaceFallback claims the destination once with mkdir and
// never unlinks it for the remainder of the transaction. The fast path
// renames the source over the claim; on platforms that permit replacing an
// empty directory (Linux) that rename only ever touches the claim we own.
// Platforms that refuse to rename over any directory (macOS returns EEXIST
// even for an empty target) take the slow path instead: the source children
// are moved into the claim one by one, so the destination stays occupied by
// us the whole time. Removing the claim and retrying the rename — the
// previous design — opened an unlink→rename window in which a foreign
// directory could appear and be silently overwritten, breaking the
// no-replace contract. The destination is therefore never deleted here: a
// concurrent creator either loses the mkdir race (EEXIST) or finds the path
// claimed until publication completes. The slow path is not
// all-or-nothing-visible, which is acceptable on the degraded filesystems
// that need it; every other property of the no-replace contract holds.
func renameSkillDirNoReplaceFallback(source, destination string) error {
	sourceInfo, err := skillPathLstat(source)
	if err != nil {
		return fmt.Errorf("读取源 Skill 身份失败 %s: %w", source, err)
	}
	if err := skillPathMkdir(destination, 0o700); err != nil {
		if os.IsExist(err) {
			return &os.LinkError{Op: "rename", Old: source, New: destination, Err: os.ErrExist}
		}
		return fmt.Errorf("声明 Skill 发布目标失败 %s: %w", destination, err)
	}
	if err := skillPathRename(source, destination); err == nil {
		return nil
	}
	if err := moveSkillDirChildrenIntoClaim(source, destination, sourceInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("降级发布 Skill 目录失败 %s: %w", destination, err)
	}
	return nil
}

// moveSkillDirChildrenIntoClaim moves every top-level child of source into
// the claimed destination and restores the source mode on the claim. Any
// failure moves the already relocated children back and removes only the
// claim, leaving the source intact. Each child rename targets a path that
// does not exist inside the empty claim, so no step can replace a foreign
// object. The emptied source shell is intentionally left behind: callers of
// renameSkillPathNoReplace use its presence to tell that the publication was
// a child move into a fresh claim (new identity) rather than a rename that
// consumed the source (identity preserved), and moveSkillPathRecoverably
// removes it once the move is confirmed.
func moveSkillDirChildrenIntoClaim(source, destination string, sourceMode os.FileMode) error {
	entries, err := skillPathReadDir(source)
	if err != nil {
		_ = skillPathRemove(destination)
		return fmt.Errorf("读取源 Skill 目录失败 %s: %w", source, err)
	}
	moved := make([]string, 0, len(entries))
	rollback := func() {
		for i := len(moved) - 1; i >= 0; i-- {
			_ = skillPathRename(filepath.Join(destination, moved[i]), filepath.Join(source, moved[i]))
		}
		_ = skillPathRemove(destination)
	}
	for _, entry := range entries {
		name := entry.Name()
		if err := skillPathRename(filepath.Join(source, name), filepath.Join(destination, name)); err != nil {
			rollback()
			return fmt.Errorf("迁移 Skill 目录项失败 %s: %w", name, err)
		}
		moved = append(moved, name)
	}
	if err := skillPathChmod(destination, sourceMode); err != nil {
		rollback()
		return fmt.Errorf("恢复已发布 Skill 目录权限失败 %s: %w", destination, err)
	}
	return nil
}

// renameSkillFileNoReplaceFallback uses os.Link, which atomically fails if the
// destination exists, then removes the source to complete the move. If the
// source removal fails, the link just created is retracted so the failed
// publish leaves no untracked object at the destination (the caller has no
// publication record to roll back, and a backup restore would refuse the
// occupied path).
func renameSkillFileNoReplaceFallback(source, destination string) error {
	sourceInfo, err := skillPathLstat(source)
	if err != nil {
		return fmt.Errorf("读取源 Skill 身份失败 %s: %w", source, err)
	}
	sourceFileID := skillPathFileIdentity(source)
	if err := skillPathLink(source, destination); err != nil {
		if os.IsExist(err) {
			return &os.LinkError{Op: "rename", Old: source, New: destination, Err: os.ErrExist}
		}
		return fmt.Errorf("发布 Skill 文件失败 %s: %w", destination, err)
	}
	if err := skillPathRemove(source); err != nil {
		return errors.Join(
			fmt.Errorf("移动 Skill 文件失败（源 %s 与目标 %s 均尝试保留）: %w", source, destination, err),
			retractLinkedSkillFile(source, destination, sourceInfo, sourceFileID),
		)
	}
	return nil
}

// retractLinkedSkillFile removes the hard link the fallback just created at
// destination. The link shares the source inode, and only that proven
// identity may be removed: if destination no longer resolves to the linked
// object (a concurrent replacement won the path), it is left untouched and
// reported instead.
func retractLinkedSkillFile(source, destination string, sourceInfo os.FileInfo, sourceFileID string) error {
	destInfo, err := skillPathLstat(destination)
	if err != nil {
		return fmt.Errorf("确认待撤回 Skill 文件失败 %s: %w", destination, err)
	}
	if !skillPathIdentityProven(sourceInfo, destInfo, sourceFileID, skillPathFileIdentity(destination)) {
		return fmt.Errorf("待撤回 Skill 文件已被替换，保留目标 %s", destination)
	}
	if err := skillPathRemove(destination); err != nil {
		return fmt.Errorf("撤回已链接 Skill 文件失败 %s（源 %s 与目标 %s 均保留）: %w", destination, source, destination, err)
	}
	return nil
}
