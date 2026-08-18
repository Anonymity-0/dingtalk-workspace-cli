package upgrade

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"os"
	"path/filepath"
	"sort"
)

// SkillPathPublication records enough immutable identity to prove that a
// destination still belongs to the transaction that published it. The
// fingerprint is intentionally private so callers cannot forge records.
type SkillPathPublication struct {
	Destination string
	fingerprint [sha256.Size]byte
	identity    os.FileInfo
	incarnation string
	fileID      string
}

// PublishSkillPathNoReplace atomically publishes a staged path without ever
// replacing a destination created after the backup phase.
func PublishSkillPathNoReplace(staged, destination string) (SkillPathPublication, error) {
	identity, err := skillPathLstat(staged)
	if err != nil {
		return SkillPathPublication{}, fmt.Errorf("读取待发布 Skill 身份失败 %s: %w", staged, err)
	}
	fingerprint, err := fingerprintSkillPath(staged)
	if err != nil {
		return SkillPathPublication{}, fmt.Errorf("计算待发布 Skill 身份失败 %s: %w", staged, err)
	}
	stagedFileID := skillPathFileIdentity(staged)
	if err := skillPathRenameNoReplace(staged, destination); err != nil {
		return SkillPathPublication{}, fmt.Errorf("目标必须不存在的 Skill 发布失败 %s: %w", destination, err)
	}
	// Dest is now occupied by this transaction, but callers only record a
	// publication after this function returns. Capture identity immediately
	// so a later confirmation failure can retract dest instead of leaving an
	// untracked leftover that blocks backup restore.
	publication, recErr := recordSkillPathPublication(destination)
	if recErr != nil {
		if _, statErr := skillPathLstat(destination); os.IsNotExist(statErr) {
			return SkillPathPublication{}, fmt.Errorf("确认已发布 Skill 身份失败 %s: %w", destination, recErr)
		}
		return SkillPathPublication{}, fmt.Errorf("Skill 发布状态不确定：发布后无法登记目标 %s: %w；目标 %s 保留", destination, recErr, destination)
	}
	if publication.fingerprint != fingerprint {
		if _, stagedErr := skillPathLstat(staged); os.IsNotExist(stagedErr) &&
			!skillPathIdentityProven(identity, publication.identity, stagedFileID, publication.fileID) {
			// Dest is not the staged object: a concurrent writer won the path
			// after the rename returned. Leave that object in place.
			return SkillPathPublication{}, fmt.Errorf("确认已发布 Skill 身份失败 %s: staging 内容已变化", destination)
		}
		return SkillPathPublication{}, retractUnconfirmedSkillPublication(publication, fmt.Errorf("确认已发布 Skill 身份失败 %s: staging 内容已变化", destination))
	}
	// The rename consumed the staged path: the published object is the staged
	// object itself, which identity proves. The rename left the staged path
	// behind (the no-replace fallback moves children into a fresh claim):
	// identity cannot span that move, so the proof is the content equality
	// verified above.
	if _, stagedErr := skillPathLstat(staged); os.IsNotExist(stagedErr) {
		if !skillPathIdentityProven(identity, publication.identity, stagedFileID, publication.fileID) {
			return SkillPathPublication{}, fmt.Errorf("确认已发布 Skill 身份失败 %s: staging 身份已变化", destination)
		}
	} else if stagedErr != nil {
		return SkillPathPublication{}, retractUnconfirmedSkillPublication(publication, fmt.Errorf("确认已发布 Skill 身份失败 %s: 无法确认 staging 状态", destination))
	}
	return publication, nil
}

// retractUnconfirmedSkillPublication withdraws a dest that this transaction
// occupied but never handed to the caller. A failed retract names dest so a
// later restore is not mistaken for a clean original layout.
func retractUnconfirmedSkillPublication(publication SkillPathPublication, cause error) error {
	if retractErr := rollbackSkillPathPublication(publication); retractErr != nil {
		return fmt.Errorf("Skill 发布状态不确定：%v；撤回目标 %s 失败: %v；目标 %s 保留", cause, publication.Destination, retractErr, publication.Destination)
	}
	return fmt.Errorf("Skill 发布失败，目标已撤回: %w", cause)
}

// recordSkillPathPublication captures the live identity of a path that this
// transaction just published so a later retract can prove the object is still
// the one it created. Callers must not invent records for paths they do not own.
func recordSkillPathPublication(path string) (SkillPathPublication, error) {
	identity, err := skillPathLstat(path)
	if err != nil {
		return SkillPathPublication{}, fmt.Errorf("读取已发布 Skill 身份失败 %s: %w", path, err)
	}
	fingerprint, err := fingerprintSkillPath(path)
	if err != nil {
		return SkillPathPublication{}, fmt.Errorf("计算已发布 Skill 身份失败 %s: %w", path, err)
	}
	return recordSkillPathPublicationFrom(path, identity, fingerprint, skillPathFileIdentity(path)), nil
}

func recordSkillPathPublicationFrom(path string, identity os.FileInfo, fingerprint [sha256.Size]byte, fileID string) SkillPathPublication {
	return SkillPathPublication{
		Destination: path,
		fingerprint: fingerprint,
		identity:    identity,
		incarnation: skillPathFileIncarnation(identity),
		fileID:      fileID,
	}
}

// RollbackSkillPathPublications removes only objects that can still be proven
// to have been published by this transaction. Each live path is first claimed
// into a private sibling quarantine. A concurrent replacement is restored when
// possible, otherwise retained in quarantine and reported explicitly.
func RollbackSkillPathPublications(publications []SkillPathPublication) error {
	var rollbackErr error
	for i := len(publications) - 1; i >= 0; i-- {
		if err := rollbackSkillPathPublication(publications[i]); err != nil {
			rollbackErr = errors.Join(rollbackErr, err)
		}
	}
	return rollbackErr
}

func rollbackSkillPathPublication(publication SkillPathPublication) (err error) {
	destination := publication.Destination
	quarantineRoot, err := skillPathMkdirTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".rollback-")
	if err != nil {
		return fmt.Errorf("创建 Skill 回滚隔离目录失败 %s: %w", destination, err)
	}
	quarantine := filepath.Join(quarantineRoot, "payload")
	cleanupRoot := func() error {
		if cleanupErr := skillPathRemoveAll(quarantineRoot); cleanupErr != nil {
			return fmt.Errorf("清理 Skill 回滚隔离目录失败 %s: %w", quarantineRoot, cleanupErr)
		}
		return nil
	}

	liveIdentity, liveIdentityErr := skillPathLstat(destination)
	if os.IsNotExist(liveIdentityErr) {
		return cleanupRoot()
	}
	liveFingerprint, liveFingerprintErr := fingerprintSkillPath(destination)
	liveVerificationErr := errors.Join(liveIdentityErr, liveFingerprintErr)
	if liveVerificationErr != nil {
		return errors.Join(
			fmt.Errorf("拒绝删除无法验证的 Skill %s: %w", destination, liveVerificationErr),
			cleanupRoot(),
		)
	}
	if publication.identity == nil ||
		!skillPathIdentityProven(publication.identity, liveIdentity, publication.fileID, skillPathFileIdentity(destination)) ||
		publication.incarnation != skillPathFileIncarnation(liveIdentity) ||
		liveFingerprint != publication.fingerprint {
		return errors.Join(
			fmt.Errorf("拒绝删除非本事务 Skill %s: 发布对象身份已变化", destination),
			cleanupRoot(),
		)
	}

	if err := skillPathRename(destination, quarantine); err != nil {
		cleanupErr := cleanupRoot()
		if os.IsNotExist(err) {
			return cleanupErr
		}
		return errors.Join(fmt.Errorf("隔离待回滚 Skill 失败 %s: %w", destination, err), cleanupErr)
	}

	actualIdentity, identityErr := skillPathLstat(quarantine)
	actual, fingerprintErr := fingerprintSkillPath(quarantine)
	if identityErr == nil && fingerprintErr == nil &&
		skillPathIdentityProven(publication.identity, actualIdentity, publication.fileID, skillPathFileIdentity(quarantine)) &&
		actual == publication.fingerprint {
		if removeErr := removePublishedSkillSource(quarantine); removeErr != nil {
			return fmt.Errorf("移除事务发布的 Skill 失败 %s（隔离于 %s）: %w", destination, quarantine, removeErr)
		}
		return cleanupRoot()
	}

	verificationErr := errors.Join(identityErr, fingerprintErr)
	if verificationErr == nil {
		verificationErr = errors.New("发布对象身份已变化")
	}
	if restoreErr := skillPathRenameNoReplace(quarantine, destination); restoreErr != nil {
		return fmt.Errorf("拒绝删除非本事务 Skill %s: %v；并发对象保留于 %s，恢复原路径失败: %w", destination, verificationErr, quarantine, restoreErr)
	}
	return errors.Join(
		fmt.Errorf("拒绝删除非本事务 Skill %s: %w", destination, verificationErr),
		cleanupRoot(),
	)
}

func fingerprintSkillPath(path string) ([sha256.Size]byte, error) {
	h := sha256.New()
	if err := fingerprintSkillPathInto(h, path); err != nil {
		return [sha256.Size]byte{}, err
	}
	var result [sha256.Size]byte
	copy(result[:], h.Sum(nil))
	return result, nil
}

func fingerprintSkillPathInto(h hash.Hash, path string) error {
	info, err := skillPathLstat(path)
	if err != nil {
		return err
	}
	writeFingerprintField(h, info.Mode().Type().String())
	writeFingerprintField(h, info.Mode().Perm().String())
	switch {
	case info.IsDir():
		entries, err := skillPathReadDir(path)
		if err != nil {
			return err
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		writeFingerprintField(h, fmt.Sprintf("%d", len(entries)))
		for _, entry := range entries {
			writeFingerprintField(h, entry.Name())
			if err := fingerprintSkillPathInto(h, filepath.Join(path, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	case info.Mode()&os.ModeSymlink != 0:
		target, err := skillPathReadlink(path)
		if err != nil {
			return err
		}
		writeFingerprintField(h, target)
		return nil
	case info.Mode().IsRegular():
		digest, err := skillPathFileDigest(path)
		if err != nil {
			return err
		}
		writeFingerprintField(h, fmt.Sprintf("%d", info.Size()))
		_, err = h.Write(digest[:])
		return err
	default:
		return fmt.Errorf("不支持识别特殊 Skill 路径 %s（mode=%s）", path, info.Mode())
	}
}

func writeFingerprintField(h hash.Hash, value string) {
	_, _ = fmt.Fprintf(h, "%d:", len(value))
	_, _ = h.Write([]byte(value))
}
