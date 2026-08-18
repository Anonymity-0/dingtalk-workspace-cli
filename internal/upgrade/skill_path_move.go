package upgrade

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

var (
	skillPathRename             = func(src, dst string) error { return upgradeRename(src, dst) }
	skillPathRenameNoReplace    = renameSkillPathNoReplace
	skillPathCopy               = copySkillPathLexically
	skillPathVerify             = verifySkillPathCopy
	skillPathRemoveAll          = os.RemoveAll
	skillPathMkdirAll           = os.MkdirAll
	skillPathMkdirTemp          = os.MkdirTemp
	skillPathChmod              = os.Chmod
	skillPathLstat              = os.Lstat
	skillPathMkdir              = os.Mkdir
	skillPathLink               = os.Link
	skillPathRemove             = os.Remove
	skillPathReadDir            = os.ReadDir
	skillPathFileIdentity       = skillPathFileIdentityImpl
	skillPathFileIncarnation    = skillPathFileIncarnationImpl
	skillPathSameFileIdentity   = skillPathSameFileIdentityImpl
	skillPathMarkPublication    = markSkillPublication
	skillPathPublicationHasMark = skillPublicationHasMark
	skillPathReadlink           = os.Readlink
	skillPathSymlink            = os.Symlink
	skillPathOpen               = os.Open
	skillPathOpenFile           = os.OpenFile
	skillPathCopyBytes          = io.Copy
	skillPathSync               = func(file *os.File) error { return file.Sync() }
	skillPathWalkDir            = filepath.WalkDir
)

// moveSkillPathRecoverably moves one managed Skill path without weakening the
// backup contract when source and destination are on different filesystems.
// The cross-device path publishes a fully copied and verified sibling staging
// path before removing the source. Any failure before the destination is
// published leaves the source intact. After the staging path has been renamed
// onto the destination, a later failure (mode restore, copy verification, or
// staging cleanup) retracts that identity-proven destination so the caller is
// not left with an unrecorded leftover that would block retries; a retract
// failure returns an uncertain-state error naming both retained locations. A
// source-removal failure after a verified publication deliberately leaves both
// copies.
func moveSkillPathRecoverably(src, dst string) (err error) {
	if _, statErr := skillPathLstat(dst); statErr == nil {
		return fmt.Errorf("目标已存在: %s", dst)
	} else if !os.IsNotExist(statErr) {
		return fmt.Errorf("检查移动目标失败 %s: %w", dst, statErr)
	}
	if err := skillPathMkdirAll(filepath.Dir(dst), dirPermShared); err != nil {
		return fmt.Errorf("创建移动目标目录失败 %s: %w", filepath.Dir(dst), err)
	}
	if renameErr := skillPathRenameNoReplace(src, dst); renameErr == nil {
		// The no-replace fallback may have published by moving the children
		// into a fresh claim, leaving an emptied source shell behind. Move
		// semantics require the source to be gone afterwards. If the shell
		// cannot be removed, the children are moved back into it and the
		// destination is retracted: reporting a plain failure while the data
		// sits at dst would leave the caller without a recorded backup and
		// the original path empty, breaking the contract that a failed move
		// keeps the source intact.
		if _, shellErr := skillPathLstat(src); shellErr == nil {
			if removeErr := skillPathRemove(src); removeErr != nil {
				if os.IsNotExist(removeErr) {
					return nil
				}
				if retractErr := retractSkillDirChildMove(src, dst); retractErr != nil {
					return fmt.Errorf("Skill 移动状态不确定：源空壳 %s 删除失败: %v；撤回目标 %s 也失败: %v；数据位于 %s", src, removeErr, dst, retractErr, dst)
				}
				return fmt.Errorf("Skill 移动失败，原路径已恢复 %s: %w", src, removeErr)
			}
		} else if !os.IsNotExist(shellErr) {
			return fmt.Errorf("检查已移动 Skill 源路径失败 %s: %w", src, shellErr)
		}
		return nil
	} else if !isCrossDeviceError(renameErr) {
		return fmt.Errorf("移动 Skill 路径失败 %s -> %s: %w", src, dst, renameErr)
	}

	stageRoot, err := skillPathMkdirTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".cross-device-")
	if err != nil {
		return fmt.Errorf("创建跨设备 Skill staging 失败 %s: %w", dst, err)
	}
	stage := filepath.Join(stageRoot, "payload")
	stageCleaned := false
	defer func() {
		if stageCleaned {
			return
		}
		makeSkillPathTreeWritable(stageRoot)
		if cleanupErr := skillPathRemoveAll(stageRoot); cleanupErr != nil {
			err = errors.Join(err, fmt.Errorf("清理跨设备 Skill staging 失败 %s: %w", stageRoot, cleanupErr))
		}
	}()

	if err := skillPathCopy(src, stage); err != nil {
		return fmt.Errorf("复制跨设备 Skill staging 失败 %s: %w", stage, err)
	}
	if err := skillPathVerify(src, stage); err != nil {
		return fmt.Errorf("校验跨设备 Skill staging 失败 %s: %w", stage, err)
	}
	stageInfo, err := skillPathLstat(stage)
	if err != nil {
		return fmt.Errorf("检查跨设备 Skill staging 失败 %s: %w", stage, err)
	}
	if stageInfo.IsDir() {
		// Darwin refuses to rename a read-only directory even when both parent
		// directories are writable. The exact source mode was verified above;
		// temporarily add owner access for publication, then restore it before
		// the final verification and before the source is removed.
		if err := skillPathChmod(stage, stageInfo.Mode().Perm()|0o700); err != nil {
			return fmt.Errorf("准备跨设备 Skill staging 发布失败 %s: %w", stage, err)
		}
	}
	if err := skillPathRenameNoReplace(stage, dst); err != nil {
		return fmt.Errorf("发布跨设备 Skill 备份失败 %s: %w", dst, err)
	}
	// The destination is now occupied by this transaction, but the caller has
	// not recorded it as a backup. Capture a verifiable publication identity
	// immediately so any later failure can retract dest instead of leaving an
	// untracked leftover that would make retries fail with "目标已存在".
	publication, recErr := recordSkillPathPublication(dst)
	if recErr != nil {
		return fmt.Errorf("Skill 移动状态不确定：跨设备发布后无法登记目标 %s: %w；源 %s 与目标 %s 均保留", dst, recErr, src, dst)
	}
	if stageInfo.IsDir() {
		if err := skillPathChmod(dst, stageInfo.Mode().Perm()); err != nil {
			return retractCrossDevicePublication(src, dst, publication, fmt.Errorf("恢复已发布 Skill 目录权限失败 %s: %w", dst, err))
		}
		// chmod updates mode (and ctime on some filesystems). Fingerprint and
		// incarnation include both, so refresh the record before later steps
		// can fail; a stale record would refuse to retract dest.
		refreshed, recErr := recordSkillPathPublication(dst)
		if recErr != nil {
			return fmt.Errorf("Skill 移动状态不确定：跨设备发布后无法登记已恢复权限的目标 %s: %w；源 %s 与目标 %s 均保留", dst, recErr, src, dst)
		}
		publication = refreshed
	}
	if err := skillPathVerify(src, dst); err != nil {
		return retractCrossDevicePublication(src, dst, publication, fmt.Errorf("校验已发布 Skill 备份失败 %s: %w", dst, err))
	}
	if err := skillPathRemoveAll(stageRoot); err != nil {
		return retractCrossDevicePublication(src, dst, publication, fmt.Errorf("清理已发布 Skill staging 失败 %s: %w", stageRoot, err))
	}
	stageCleaned = true
	if err := removePublishedSkillSource(src); err != nil {
		return fmt.Errorf("Skill 目标已发布但源路径删除失败（源 %s 与目标 %s 均保留）: %w", src, dst, err)
	}
	return nil
}

// retractCrossDevicePublication withdraws an identity-proven destination
// published by the cross-device copy path. The source is still intact and the
// caller has not recorded this destination as a backup. If retract fails, the
// error names both retained locations so a later retry is not mistaken for a
// clean original layout.
func retractCrossDevicePublication(src, dst string, publication SkillPathPublication, cause error) error {
	if retractErr := rollbackSkillPathPublication(publication); retractErr != nil {
		return fmt.Errorf("Skill 移动状态不确定：%v；撤回目标 %s 失败: %v；源 %s 与目标 %s 均保留", cause, dst, retractErr, src, dst)
	}
	return fmt.Errorf("Skill 移动失败，目标已撤回，原路径保留 %s: %w", src, cause)
}

// retractSkillDirChildMove undoes the no-replace fallback child move when the
// emptied source shell cannot be removed afterwards: every child of the
// published destination moves back into the shell, then the destination is
// withdrawn. The renames mirror the forward child move — each targets a path
// inside a directory this transaction owns — so no step replaces a foreign
// object, and a failed retraction surfaces with the data location reported
// instead of being dropped by a plain failure.
func retractSkillDirChildMove(src, dst string) error {
	entries, err := skillPathReadDir(dst)
	if err != nil {
		return fmt.Errorf("读取待撤回 Skill 目标失败 %s: %w", dst, err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if err := skillPathRename(filepath.Join(dst, name), filepath.Join(src, name)); err != nil {
			return fmt.Errorf("回迁 Skill 目录项失败 %s: %w", name, err)
		}
	}
	if err := skillPathRemove(dst); err != nil {
		return fmt.Errorf("撤回已清空 Skill 目标失败 %s: %w", dst, err)
	}
	return nil
}

type skillPathDirMode struct {
	path string
	mode os.FileMode
}

func removePublishedSkillSource(src string) error {
	dirModes, err := prepareSkillPathTreeRemoval(src)
	if err != nil {
		return err
	}
	removeErr := skillPathRemoveAll(src)
	if removeErr == nil {
		if _, statErr := skillPathLstat(src); os.IsNotExist(statErr) {
			return nil
		} else if statErr == nil {
			removeErr = errors.New("源路径仍存在")
		} else {
			removeErr = fmt.Errorf("无法确认源路径已删除: %w", statErr)
		}
	}
	return errors.Join(removeErr, restoreSkillPathDirModes(dirModes))
}

func prepareSkillPathTreeRemoval(root string) ([]skillPathDirMode, error) {
	var modes []skillPathDirMode
	err := skillPathWalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if mode&0o700 == 0o700 {
			return nil
		}
		if err := skillPathChmod(path, mode|0o700); err != nil {
			return err
		}
		modes = append(modes, skillPathDirMode{path: path, mode: mode})
		return nil
	})
	if err != nil {
		return nil, errors.Join(err, restoreSkillPathDirModes(modes))
	}
	return modes, nil
}

func restoreSkillPathDirModes(modes []skillPathDirMode) error {
	var restoreErr error
	for i := len(modes) - 1; i >= 0; i-- {
		item := modes[i]
		if _, err := skillPathLstat(item.path); os.IsNotExist(err) {
			continue
		} else if err != nil {
			restoreErr = errors.Join(restoreErr, err)
			continue
		}
		if err := skillPathChmod(item.path, item.mode); err != nil {
			restoreErr = errors.Join(restoreErr, fmt.Errorf("恢复源目录权限失败 %s: %w", item.path, err))
		}
	}
	return restoreErr
}

func makeSkillPathTreeWritable(root string) {
	// Best effort only: RemoveAll below remains the source of truth and its
	// error is returned. This preparation lets it traverse read-only staging
	// directories left by a copy or verification failure.
	_ = skillPathWalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr == nil && entry.IsDir() {
			_ = skillPathChmod(path, 0o700)
		}
		return nil
	})
}

// RestoreSkillPath restores a previously recorded backup using the same
// rename-first, verified cross-device fallback as backup publication.
func RestoreSkillPath(backup, original string) error {
	return moveSkillPathRecoverably(backup, original)
}

func copySkillPathLexically(src, dst string) error {
	info, err := skillPathLstat(src)
	if err != nil {
		return err
	}
	switch {
	case info.IsDir():
		// Staging must remain writable while children are copied. Restore the
		// source mode only after the directory is complete so read-only Skill
		// directories (for example 0555) can still be migrated lexically.
		if err := skillPathMkdir(dst, 0o700); err != nil {
			return err
		}
		entries, err := skillPathReadDir(src)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copySkillPathLexically(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
				return err
			}
		}
		return os.Chmod(dst, info.Mode().Perm())
	case info.Mode()&os.ModeSymlink != 0:
		target, err := skillPathReadlink(src)
		if err != nil {
			return err
		}
		return skillPathSymlink(target, dst)
	case info.Mode().IsRegular():
		return copyRegularSkillFile(src, dst, info.Mode().Perm())
	default:
		return fmt.Errorf("不支持复制特殊 Skill 路径 %s（mode=%s）", src, info.Mode())
	}
}

func copyRegularSkillFile(src, dst string, mode os.FileMode) (err error) {
	in, err := skillPathOpen(src)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, in.Close()) }()
	out, err := skillPathOpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, out.Close()) }()
	if _, err := skillPathCopyBytes(out, in); err != nil {
		return err
	}
	if err := skillPathSync(out); err != nil {
		return err
	}
	return os.Chmod(dst, mode)
}

func verifySkillPathCopy(src, dst string) error {
	srcInfo, err := skillPathLstat(src)
	if err != nil {
		return err
	}
	dstInfo, err := skillPathLstat(dst)
	if err != nil {
		return err
	}
	if srcInfo.Mode()&os.ModeType != dstInfo.Mode()&os.ModeType {
		return fmt.Errorf("路径类型不一致: %s (%s) != %s (%s)", src, srcInfo.Mode(), dst, dstInfo.Mode())
	}
	switch {
	case srcInfo.IsDir():
		if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
			return fmt.Errorf("目录权限不一致: %s (%s) != %s (%s)", src, srcInfo.Mode().Perm(), dst, dstInfo.Mode().Perm())
		}
		srcNames, err := skillPathEntryNames(src)
		if err != nil {
			return err
		}
		dstNames, err := skillPathEntryNames(dst)
		if err != nil {
			return err
		}
		if len(srcNames) != len(dstNames) {
			return fmt.Errorf("目录项数量不一致: %s (%d) != %s (%d)", src, len(srcNames), dst, len(dstNames))
		}
		for i, name := range srcNames {
			if name != dstNames[i] {
				return fmt.Errorf("目录项不一致: %s != %s", name, dstNames[i])
			}
			if err := verifySkillPathCopy(filepath.Join(src, name), filepath.Join(dst, name)); err != nil {
				return err
			}
		}
		return nil
	case srcInfo.Mode()&os.ModeSymlink != 0:
		srcTarget, err := skillPathReadlink(src)
		if err != nil {
			return err
		}
		dstTarget, err := skillPathReadlink(dst)
		if err != nil {
			return err
		}
		if srcTarget != dstTarget {
			return fmt.Errorf("符号链接目标不一致: %q != %q", srcTarget, dstTarget)
		}
		return nil
	case srcInfo.Mode().IsRegular():
		if srcInfo.Mode().Perm() != dstInfo.Mode().Perm() {
			return fmt.Errorf("文件权限不一致: %s (%s) != %s (%s)", src, srcInfo.Mode().Perm(), dst, dstInfo.Mode().Perm())
		}
		if srcInfo.Size() != dstInfo.Size() {
			return fmt.Errorf("文件大小不一致: %s (%d) != %s (%d)", src, srcInfo.Size(), dst, dstInfo.Size())
		}
		srcDigest, err := skillPathFileDigest(src)
		if err != nil {
			return err
		}
		dstDigest, err := skillPathFileDigest(dst)
		if err != nil {
			return err
		}
		if srcDigest != dstDigest {
			return fmt.Errorf("文件内容摘要不一致: %s != %s", src, dst)
		}
		return nil
	default:
		return fmt.Errorf("不支持校验特殊 Skill 路径 %s（mode=%s）", src, srcInfo.Mode())
	}
}

func skillPathEntryNames(dir string) ([]string, error) {
	entries, err := skillPathReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	return names, nil
}

func skillPathFileDigest(path string) ([sha256.Size]byte, error) {
	f, err := skillPathOpen(path)
	if err != nil {
		return [sha256.Size]byte{}, err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := skillPathCopyBytes(hash, f); err != nil {
		return [sha256.Size]byte{}, err
	}
	var digest [sha256.Size]byte
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}
