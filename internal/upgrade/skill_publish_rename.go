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
// even for an empty target) take the slow path instead — but only after
// proving the claim is still empty: a rename that failed because a
// concurrent writer landed inside the claim must abort with the destination
// retained, never fall through to child moves whose per-entry primitives
// would collide with (and rollback would delete) the foreign data. The slow
// path moves the source children into the claim one by one, each through an
// atomic no-clobber primitive (mkdir claim for directories, os.Link for
// files, os.Symlink for links), so a same-name concurrent entry aborts the
// move and a foreign entry of any name — detected by re-reading the claim —
// retains the destination instead of letting the rollback delete it. The
// destination claim is removed only while still empty, so a foreign entry
// always survives a failed publication.
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
	claimEntries, claimErr := skillPathReadDir(destination)
	if claimErr != nil {
		return fmt.Errorf("降级发布 Skill 目录失败 %s: %w", destination, claimErr)
	}
	if len(claimEntries) > 0 {
		return fmt.Errorf("降级发布中止，目标在声明后被并发写入 %s: %w", destination, os.ErrExist)
	}
	if err := moveSkillDirChildrenIntoClaim(source, destination, sourceInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("降级发布 Skill 目录失败 %s: %w", destination, err)
	}
	return nil
}

// moveSkillDirChildNoReplace moves one child of a staged directory into the
// claimed destination through an atomic no-clobber primitive: directories
// recursively claim with mkdir, regular files publish via os.Link, and
// symlinks are recreated with os.Symlink. Every primitive fails with EEXIST
// when the destination entry was taken concurrently, so no step can replace
// a foreign object.
func moveSkillDirChildNoReplace(source, destination string) error {
	info, err := skillPathLstat(source)
	if err != nil {
		return fmt.Errorf("读取源 Skill 目录项失败 %s: %w", source, err)
	}
	switch {
	case info.IsDir():
		if err := renameSkillDirNoReplaceFallback(source, destination); err != nil {
			return err
		}
		// The nested slow path leaves an emptied shell at the source (the
		// fast-path rename consumed it). os.Remove only deletes an empty
		// directory, so clearing the shell restores the single-rename
		// outcome the parent's rollback and shell signal expect; a
		// concurrent writer inside our own staging shell fails the removal
		// and surfaces through the parent's uncertain-state path instead of
		// being silently discarded.
		if err := skillPathRemove(source); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("清理已迁移 Skill 子目录失败 %s: %w", source, err)
		}
		return nil
	case info.Mode().IsRegular():
		return renameSkillFileNoReplaceFallback(source, destination)
	case info.Mode()&os.ModeSymlink != 0:
		return renameSkillSymlinkNoReplaceFallback(source, destination)
	default:
		return &os.LinkError{
			Op: "rename", Old: source, New: destination,
			Err: fmt.Errorf("%w: 源路径类型不支持降级发布", errNoReplaceRenameUnsupported),
		}
	}
}

// renameSkillSymlinkNoReplaceFallback recreates the link at the destination
// with os.Symlink — atomic, EEXIST when taken — and then removes the source.
// A failed source removal retracts the recreated link only while it still
// carries the same target, so a concurrent replacement survives and the
// object itself always remains reachable.
func renameSkillSymlinkNoReplaceFallback(source, destination string) error {
	target, err := skillPathReadlink(source)
	if err != nil {
		return fmt.Errorf("读取源 Skill 链接失败 %s: %w", source, err)
	}
	if err := skillPathSymlink(target, destination); err != nil {
		if os.IsExist(err) {
			return &os.LinkError{Op: "rename", Old: source, New: destination, Err: os.ErrExist}
		}
		return fmt.Errorf("发布 Skill 链接失败 %s: %w", destination, err)
	}
	if err := skillPathRemove(source); err != nil {
		if current, readErr := skillPathReadlink(destination); readErr == nil && current == target {
			_ = skillPathRemove(destination)
		}
		return fmt.Errorf("移动 Skill 链接失败（源 %s 与目标 %s 至少一者保留）: %w", source, destination, err)
	}
	return nil
}

// removeClaimIfEmpty deletes the claimed destination only while it still
// holds no entries. A foreign entry that appeared after the claim must never
// be removed: the rollback has no ownership proof for it.
func removeClaimIfEmpty(destination string) error {
	entries, err := skillPathReadDir(destination)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("检查降级发布目标失败 %s: %w", destination, err)
	}
	if len(entries) > 0 {
		return nil
	}
	if err := skillPathRemove(destination); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("清理降级发布目标失败 %s: %w", destination, err)
	}
	return nil
}

// moveSkillDirChildrenIntoClaim moves every top-level child of source into
// the claimed destination — each through moveSkillDirChildNoReplace, so a
// concurrently created same-name entry fails atomically — and restores the
// source mode on the claim. The claim is re-read after the moves: an entry
// count above the source's means a different-named foreign object landed
// mid-move, and the publication aborts with the destination retained. Any
// failure moves the already relocated children back and removes the claim
// only while it is empty, leaving the source intact and foreign entries in
// place. The emptied source shell is intentionally left behind: callers of
// renameSkillPathNoReplace use its presence to tell that the publication was
// a child move into a fresh claim (new identity) rather than a rename that
// consumed the source (identity preserved), and moveSkillPathRecoverably
// removes it once the move is confirmed.
func moveSkillDirChildrenIntoClaim(source, destination string, sourceMode os.FileMode) error {
	entries, err := skillPathReadDir(source)
	if err != nil {
		return errors.Join(
			fmt.Errorf("读取源 Skill 目录失败 %s: %w", source, err),
			removeClaimIfEmpty(destination),
		)
	}
	moved := make([]string, 0, len(entries))
	rollback := func() error {
		var rollbackErr error
		for i := len(moved) - 1; i >= 0; i-- {
			if err := skillPathRename(filepath.Join(destination, moved[i]), filepath.Join(source, moved[i])); err != nil {
				rollbackErr = errors.Join(rollbackErr, err)
			}
		}
		rollbackErr = errors.Join(rollbackErr, removeClaimIfEmpty(destination))
		return rollbackErr
	}
	for _, entry := range entries {
		name := entry.Name()
		if err := moveSkillDirChildNoReplace(filepath.Join(source, name), filepath.Join(destination, name)); err != nil {
			return errors.Join(fmt.Errorf("迁移 Skill 目录项失败 %s: %w", name, err), rollback())
		}
		moved = append(moved, name)
	}
	live, liveErr := skillPathReadDir(destination)
	if liveErr != nil {
		return errors.Join(
			fmt.Errorf("确认降级发布目标失败 %s: %w", destination, liveErr),
			rollback(),
		)
	}
	if len(live) > len(entries) {
		return errors.Join(
			fmt.Errorf("降级发布中止，目标出现并发目录项 %s: %w", destination, os.ErrExist),
			rollback(),
		)
	}
	if err := skillPathChmod(destination, sourceMode); err != nil {
		return errors.Join(
			fmt.Errorf("恢复已发布 Skill 目录权限失败 %s: %w", destination, err),
			rollback(),
		)
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
