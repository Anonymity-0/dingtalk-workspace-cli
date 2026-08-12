package helpers

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// ==========================================================
// drive sync — 本地与钉盘双向同步（本地 ⇄ Drive）
// ==========================================================
// ──────────────────────────────────────────────────────────
// dws drive sync — 把 --local-folder 与 --remote-folder 做文件级双向同步。
//
// 复用 status 的差异判定（exact MD5 / --quick modified_time）先算出 diff，再按方向执行：
//   new_local   仅本地存在   → push（缺失的远端目录按需创建，再上传）
//   new_remote  仅钉盘存在   → pull（下载到本地对应路径）
//   modified    两侧都变更   → 按 --on-conflict 解决：
//                 skip       默认：两侧都不动并保留两边内容
//                 remote-wins     ：拉取远端覆盖本地
//                 local-wins       ：上传本地覆盖远端
//                 keep-both        ：本地改名保留，再拉取远端到原路径
//                 ask              ：交互式逐个询问
//   unknown     exact 模式远端无可靠 md5、内容无法核对 → 跳过（记 skipped，提示改用 --quick）
//   unchanged   两侧一致                                → 不动
//
// 文件级同步——只新增/覆盖，不删除任何一侧的多余文件。summary.failed > 0 时以非零
// 退出码退出，结构化 summary + diff + items 仍打印在 stdout 上。
// ──────────────────────────────────────────────────────────

// --on-conflict 的五种冲突解决策略。
const (
	syncConflictLocalWins  = "local-wins"  // 上传本地覆盖远端
	syncConflictRemoteWins = "remote-wins" // 拉取远端覆盖本地
	syncConflictKeepBoth   = "keep-both"   // 本地改名保留，再拉取远端到原路径
	syncConflictAsk        = "ask"         // 交互式逐个询问
	syncConflictSkip       = "skip"        // 默认：两侧都变更时什么都不做，两边内容都保留
)

// sync 动作分类（direction 标注每条记录属于哪个方向）。
const (
	syncDirectionPull     = "pull"
	syncDirectionPush     = "push"
	syncDirectionConflict = "conflict"

	syncActionDownloaded    = "downloaded"
	syncActionUploaded      = "uploaded"
	syncActionOverwritten   = "overwritten"
	syncActionFolderCreated = "folder_created"
	syncActionRenamedLocal  = "renamed_local"
	syncActionSkipped       = "skipped"
	syncActionFailed        = "failed"
)

// driveSyncItem 是输出 items[] 中每条操作的明细。
type driveSyncItem struct {
	RelPath   string `json:"rel_path"`
	Action    string `json:"action"`
	Direction string `json:"direction,omitempty"`
	Error     string `json:"error,omitempty"`
}

// driveSyncSummary 是各方向的计数汇总。
type driveSyncSummary struct {
	Pulled  int `json:"pulled"`
	Pushed  int `json:"pushed"`
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
}

// driveSyncDiff 是本次同步前算出的五类差异（与 status 同源）。
type driveSyncDiff struct {
	NewLocal  []driveStatusEntry `json:"new_local"`
	NewRemote []driveStatusEntry `json:"new_remote"`
	Modified  []driveStatusEntry `json:"modified"`
	Unchanged []driveStatusEntry `json:"unchanged"`
	Unknown   []driveStatusEntry `json:"unknown"`
}

// driveSyncResult 是 sync 命令的输出 schema。
type driveSyncResult struct {
	Detection string           `json:"detection"`
	Diff      driveSyncDiff    `json:"diff"`
	Summary   driveSyncSummary `json:"summary"`
	Items     []driveSyncItem  `json:"items"`
}

// driveSyncFailure 在 summary.failed > 0 时返回：结构化结果已打印到 stdout，
// 这里只负责以 exit=1 退出并向 stderr 输出一行简短说明（与 drivePushFailure 一致）。
type driveSyncFailure struct{ failed int }

func (e *driveSyncFailure) Error() string {
	return fmt.Sprintf("drive sync: %d item(s) failed", e.failed)
}
func (e *driveSyncFailure) RawStderr() string { return e.Error() }
func (e *driveSyncFailure) ExitCode() int     { return 1 }

func runDriveSync(cmd *cobra.Command, _ []string) error {
	if err := validateRequiredFlags(cmd, "local-folder", "remote-folder"); err != nil {
		return err
	}
	localDir := mustGetFlag(cmd, "local-folder")
	remoteDirID := mustGetFlag(cmd, "remote-folder")
	// space-id 可选：不传则由底层使用「我的文件」对应的空间。
	spaceID := mustGetFlag(cmd, "space-id")

	onConflict, _ := cmd.Flags().GetString("on-conflict")
	if onConflict == "" {
		// 安全默认：两侧都变更时不擅自覆盖任何一侧。
		onConflict = syncConflictSkip
	}
	switch onConflict {
	case syncConflictSkip, syncConflictLocalWins, syncConflictRemoteWins, syncConflictKeepBoth, syncConflictAsk:
	default:
		return fmt.Errorf("--on-conflict 取值非法: %s（可选 skip|local-wins|remote-wins|keep-both|ask）", onConflict)
	}

	quick, _ := cmd.Flags().GetBool("quick")
	detection := "exact"
	if quick {
		detection = "quick"
	}

	absDir, err := validateLocalDirAbs(localDir)
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	// 远端现状：文件（含 hash/mtime/fileId）与目录（rel_path → fileId，"" 即根本身）。
	remoteFiles, remoteFolders, err := fetchRemoteTreeForPush(ctx, spaceID, remoteDirID)
	if err != nil {
		return err
	}
	// 本地现状：子目录（含空目录，父目录先于子目录）与常规文件。
	localDirs, localFiles, err := walkLocalForPush(absDir)
	if err != nil {
		return fmt.Errorf("扫描本地目录失败: %w", err)
	}
	localByRel := make(map[string]localPushFile, len(localFiles))
	for _, f := range localFiles {
		localByRel[f.RelPath] = f
	}

	// 计算 diff：双端都存在的文件复用 judgeFileMatch，保持与 status 完全一致的判定。
	diff := driveSyncDiff{
		NewLocal:  []driveStatusEntry{},
		NewRemote: []driveStatusEntry{},
		Modified:  []driveStatusEntry{},
		Unchanged: []driveStatusEntry{},
		Unknown:   []driveStatusEntry{},
	}
	var newLocal, newRemote, modified, unknown []string
	for rel, pf := range localByRel {
		rf, ok := remoteFiles[rel]
		if !ok {
			newLocal = append(newLocal, rel)
			continue
		}
		lf := &localFile{RelPath: pf.RelPath, AbsPath: pf.AbsPath, Size: pf.Size, ModTimeMillis: pf.ModTimeMillis}
		verdict, jerr := judgeFileMatch(lf, rf, quick)
		if jerr != nil {
			return jerr
		}
		switch verdict {
		case matchUnchanged:
			diff.Unchanged = append(diff.Unchanged, driveStatusEntry{RelPath: rel})
		case matchUnknown:
			unknown = append(unknown, rel)
		default:
			modified = append(modified, rel)
		}
	}
	for rel := range remoteFiles {
		if _, ok := localByRel[rel]; !ok {
			newRemote = append(newRemote, rel)
		}
	}
	sort.Strings(newLocal)
	sort.Strings(newRemote)
	sort.Strings(modified)
	sort.Strings(unknown)
	for _, rel := range newLocal {
		diff.NewLocal = append(diff.NewLocal, driveStatusEntry{RelPath: rel})
	}
	for _, rel := range newRemote {
		diff.NewRemote = append(diff.NewRemote, driveStatusEntry{RelPath: rel})
	}
	for _, rel := range modified {
		diff.Modified = append(diff.Modified, driveStatusEntry{RelPath: rel})
	}
	for _, rel := range unknown {
		diff.Unknown = append(diff.Unknown, driveStatusEntry{RelPath: rel})
	}
	sortEntries(diff.Unchanged)

	res := driveSyncResult{Detection: detection, Diff: diff, Items: []driveSyncItem{}}

	// --dry-run：只算差异、不执行任何同步动作。
	if deps.Caller.DryRun() {
		deps.Out.PrintInfo("dry-run: 仅计算差异，未执行任何同步操作")
		return deps.Out.PrintJSON(res)
	}

	// unknown：exact 模式下远端无可靠 md5、内容无法核对，不擅自覆盖任何一侧，记 skipped。
	for _, rel := range unknown {
		res.Summary.Skipped++
		res.Items = append(res.Items, driveSyncItem{
			RelPath: rel, Action: syncActionSkipped, Direction: syncDirectionConflict,
			Error: "远端无可靠 md5，内容无法核对，已跳过（可改用 --quick 按 modified_time 比对）",
		})
	}

	// ask 模式：先把所有 modified 的解决策略问全，再统一执行，避免边执行边交互。
	resolutions := make(map[string]string, len(modified))
	for _, rel := range modified {
		strategy := onConflict
		switch strategy {
		case syncConflictAsk:
			strategy, err = driveSyncAskConflict(rel)
			if err != nil {
				return err
			}
		case syncConflictSkip:
			strategy = "" // 与 ask 选择「跳过」同一条落地路径
		}
		resolutions[rel] = strategy
	}

	// 落盘前先确保本地根存在（供大小写探测与 pull 落盘），并探测目标文件系统大小写敏感性。
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}
	caseInsensitive := isCaseInsensitiveFS(absDir)

	// 远端 → 本地的大小写/规范化冲突：多个远端条目在当前文件系统上映射到同一本地路径时，
	// 命中的路径一律标记 failed、都不落盘，避免静默覆盖丢文件（与 pull 同策略）。
	remoteRels := make([]string, 0, len(remoteFiles))
	for rel := range remoteFiles {
		remoteRels = append(remoteRels, rel)
	}
	collided := detectTargetCollisions(absDir, remoteRels, caseInsensitive)

	// occupied：keep-both 生成不冲突的本地重命名目标时用，覆盖两侧全部已知路径。
	occupied := make(map[string]bool)
	for rel := range localByRel {
		occupied[rel] = true
	}
	for _, d := range localDirs {
		occupied[d] = true
	}
	for rel := range remoteFiles {
		occupied[rel] = true
	}
	for rel := range remoteFolders {
		if rel != "" {
			occupied[rel] = true
		}
	}

	// 阶段 1：镜像本地目录结构到远端（缺失则创建），保证空目录与 push 目标的父目录先存在。
	for _, dir := range localDirs {
		if _, ok := remoteFolders[dir]; ok {
			continue // 远端已存在，复用 fileId
		}
		parentRel, name := splitRel(dir)
		parentID, ok := remoteFolders[parentRel]
		if !ok || parentID == "" {
			res.Summary.Failed++
			res.Items = append(res.Items, driveSyncItem{RelPath: dir, Action: syncActionFailed, Direction: syncDirectionPush, Error: "父目录未能创建"})
			continue
		}
		fid, cerr := pushCreateFolder(ctx, spaceID, parentID, name)
		if cerr != nil || fid == "" {
			res.Summary.Failed++
			msg := "create_folder 未返回 fileId"
			if cerr != nil {
				msg = cerr.Error()
			}
			res.Items = append(res.Items, driveSyncItem{RelPath: dir, Action: syncActionFailed, Direction: syncDirectionPush, Error: msg})
			continue
		}
		remoteFolders[dir] = fid
		res.Summary.Pushed++
		res.Items = append(res.Items, driveSyncItem{RelPath: dir, Action: syncActionFolderCreated, Direction: syncDirectionPush})
	}

	// 阶段 2：new_remote 下载到本地。
	for _, rel := range newRemote {
		syncPullFile(&res, ctx, spaceID, remoteFiles[rel], absDir, rel, collided, syncDirectionPull)
	}

	// 阶段 3：new_local 上传到远端。
	for _, rel := range newLocal {
		pf := localByRel[rel]
		syncPushFile(&res, ctx, spaceID, remoteFolders, pf, rel, "")
	}

	// 阶段 4：modified 按 --on-conflict 解决。
	for _, rel := range modified {
		pf := localByRel[rel]
		rf := remoteFiles[rel]
		switch resolutions[rel] {
		case syncConflictRemoteWins:
			syncPullFile(&res, ctx, spaceID, rf, absDir, rel, collided, syncDirectionPull)
		case syncConflictLocalWins:
			syncPushFile(&res, ctx, spaceID, remoteFolders, pf, rel, rf.FileID)
		case syncConflictKeepBoth:
			syncKeepBoth(&res, ctx, spaceID, rf, absDir, rel, collided, occupied)
		default: // "" — ask 选择跳过
			res.Summary.Skipped++
			res.Items = append(res.Items, driveSyncItem{RelPath: rel, Action: syncActionSkipped, Direction: syncDirectionConflict})
		}
	}

	// 结构化结果始终打印到 stdout；有失败则额外以非零退出码退出。
	if perr := deps.Out.PrintJSON(res); perr != nil {
		return perr
	}
	if res.Summary.Failed > 0 {
		return &driveSyncFailure{failed: res.Summary.Failed}
	}
	return nil
}

// syncPullFile 下载单个远端文件到本地 rel 对应路径（总是覆盖，Drive 为该项权威源），
// 并把结果计入 res。命中大小写/规范化冲突或逃逸的路径记 failed、不落盘。
func syncPullFile(res *driveSyncResult, ctx context.Context, spaceID string, rf *remoteFile, absDir, rel string, collided map[string]bool, direction string) {
	if collided[rel] {
		res.Summary.Failed++
		res.Items = append(res.Items, driveSyncItem{
			RelPath: rel, Action: syncActionFailed, Direction: direction,
			Error: "多个远端条目在当前文件系统上映射到同一本地路径（大小写/规范化冲突），已跳过以避免覆盖丢失",
		})
		return
	}
	localPath, terr := resolveLocalTarget(absDir, rel)
	if terr != nil {
		res.Summary.Failed++
		res.Items = append(res.Items, driveSyncItem{RelPath: rel, Action: syncActionFailed, Direction: direction, Error: terr.Error()})
		return
	}
	action, perr := pullOneFile(ctx, spaceID, rf, localPath, ifExistsOverwrite)
	if action == pullActionDownloaded {
		res.Summary.Pulled++
		res.Items = append(res.Items, driveSyncItem{RelPath: rel, Action: syncActionDownloaded, Direction: direction})
		return
	}
	res.Summary.Failed++
	item := driveSyncItem{RelPath: rel, Action: syncActionFailed, Direction: direction}
	if perr != nil {
		item.Error = perr.Error()
	}
	res.Items = append(res.Items, item)
}

// syncPushFile 上传单个本地文件到远端；overwriteID 非空时走覆盖上传（原地覆盖同名远端文件）。
func syncPushFile(res *driveSyncResult, ctx context.Context, spaceID string, remoteFolders map[string]string, pf localPushFile, rel, overwriteID string) {
	parentRel, name := splitRel(rel)
	parentID, ok := remoteFolders[parentRel]
	if !ok || parentID == "" {
		res.Summary.Failed++
		res.Items = append(res.Items, driveSyncItem{RelPath: rel, Action: syncActionFailed, Direction: syncDirectionPush, Error: "父目录未能创建"})
		return
	}
	if err := pushUploadFile(ctx, spaceID, parentID, overwriteID, name, pf.AbsPath, pf.Size); err != nil {
		res.Summary.Failed++
		res.Items = append(res.Items, driveSyncItem{RelPath: rel, Action: syncActionFailed, Direction: syncDirectionPush, Error: err.Error()})
		return
	}
	res.Summary.Pushed++
	action := syncActionUploaded
	direction := syncDirectionPush
	if overwriteID != "" {
		action = syncActionOverwritten
		direction = syncDirectionConflict
	}
	res.Items = append(res.Items, driveSyncItem{RelPath: rel, Action: action, Direction: direction})
}

// syncKeepBoth 解决 modified 冲突的 keep-both 策略：先把本地文件改名保留（追加基于
// 远端 fileId 的不冲突后缀），再把远端文件拉取到原 rel 路径。拉取失败则回滚改名。
func syncKeepBoth(res *driveSyncResult, ctx context.Context, spaceID string, rf *remoteFile, absDir, rel string, collided, occupied map[string]bool) {
	if collided[rel] {
		res.Summary.Failed++
		res.Items = append(res.Items, driveSyncItem{
			RelPath: rel, Action: syncActionFailed, Direction: syncDirectionConflict,
			Error: "多个远端条目在当前文件系统上映射到同一本地路径（大小写/规范化冲突），已跳过以避免覆盖丢失",
		})
		return
	}
	oldAbs, e1 := resolveLocalTarget(absDir, rel)
	if e1 != nil {
		res.Summary.Failed++
		res.Items = append(res.Items, driveSyncItem{RelPath: rel, Action: syncActionFailed, Direction: syncDirectionConflict, Error: e1.Error()})
		return
	}
	// 以「不覆盖」语义原子占用一个本地重命名目标。occupied 的精确字符串查重覆盖不到
	// 大小写/NFC 等价的既有文件；用 O_CREATE|O_EXCL 让 OS 兜底判等价性，命中则换下一个
	// 后缀，杜绝 os.Rename 静默覆盖等价既有文件导致的数据丢失。
	suffixedRel, newAbs, e2 := reserveSyncKeepBothTarget(absDir, rel, rf.FileID, occupied)
	if e2 != nil {
		res.Summary.Failed++
		res.Items = append(res.Items, driveSyncItem{RelPath: rel, Action: syncActionFailed, Direction: syncDirectionConflict, Error: e2.Error()})
		return
	}
	// newAbs 此刻是我们刚用 O_EXCL 新建的空占位文件，覆盖它不会丢用户数据；且已确保它
	// 不与任何等价既有文件冲突。
	if err := os.Rename(oldAbs, newAbs); err != nil {
		_ = os.Remove(newAbs) // 清理空占位，避免残留
		res.Summary.Failed++
		res.Items = append(res.Items, driveSyncItem{RelPath: rel, Action: syncActionFailed, Direction: syncDirectionConflict, Error: fmt.Sprintf("本地改名保留失败: %v", err)})
		return
	}
	occupied[suffixedRel] = true

	action, perr := pullOneFile(ctx, spaceID, rf, oldAbs, ifExistsOverwrite)
	if action != pullActionDownloaded {
		// 拉取失败：回滚改名，把本地文件恢复回原名。pullOneFile 是原子的（下载写临时文件、
		// 成功才 rename，失败时绝不触碰 oldAbs），故此处 oldAbs 必不存在，rename 可直接复原、
		// 无需像参考实现那样先清残留。回滚本身失败时如实上报——本地版本仍以改名后的名字保留、
		// 未丢数据，但需让用户知道文件名已变。
		res.Summary.Failed++
		msg := ""
		if perr != nil {
			msg = perr.Error()
		}
		if rbErr := os.Rename(newAbs, oldAbs); rbErr != nil {
			if msg != "" {
				msg += "; "
			}
			msg += fmt.Sprintf("回滚改名失败，本地版本保留为 %s: %v", suffixedRel, rbErr)
		}
		res.Items = append(res.Items, driveSyncItem{RelPath: rel, Action: syncActionFailed, Direction: syncDirectionConflict, Error: msg})
		return
	}
	res.Summary.Pulled++
	res.Items = append(res.Items,
		driveSyncItem{RelPath: suffixedRel, Action: syncActionRenamedLocal, Direction: syncDirectionConflict},
		driveSyncItem{RelPath: rel, Action: syncActionDownloaded, Direction: syncDirectionPull},
	)
}

// syncKeepBothCandidate 生成 keep-both 的第 n 个候选相对路径（n=0 为首选）：在扩展名前
// 插入基于远端 fileId 末段的后缀（缺失时用 conflict），n>0 再追加序号消歧。
func syncKeepBothCandidate(rel, fileID string, n int) string {
	suffix := "conflict"
	if fileID != "" {
		s := fileID
		if len(s) > 8 {
			s = s[len(s)-8:]
		}
		suffix = "conflict-" + s
	}
	dir, base := splitRel(rel)
	ext := filepath.Ext(base) // base 无路径分隔符，取末尾扩展名安全
	stem := strings.TrimSuffix(base, ext)
	name := stem + "." + suffix + ext
	if n > 0 {
		name = fmt.Sprintf("%s.%s.%d%s", stem, suffix, n, ext)
	}
	if dir == "" {
		return name
	}
	return dir + "/" + name
}

// driveSyncSuffixedRel 返回首个不在 occupied（纯字符串查重）中的候选。仅用于快速路径与
// 单元测试；keep-both 实际执行走 reserveSyncKeepBothTarget，以 OS 兜底等价性。
func driveSyncSuffixedRel(rel, fileID string, occupied map[string]bool) string {
	for n := 0; ; n++ {
		if cand := syncKeepBothCandidate(rel, fileID, n); !occupied[cand] {
			return cand
		}
	}
}

// reserveSyncKeepBothTarget 生成并以「不覆盖」语义原子占用一个 keep-both 本地目标。
// 逐个候选用 O_CREATE|O_EXCL 尝试创建：成功即占住该名，返回其相对路径与绝对路径（调用方
// 随后可安全 os.Rename 覆盖这个空占位）；EEXIST（含大小写/NFC 等价的既有文件，由 OS 判定）
// 则记入 occupied 并试下一个后缀，绝不覆盖任何既有文件。
func reserveSyncKeepBothTarget(absDir, rel, fileID string, occupied map[string]bool) (string, string, error) {
	for n := 0; ; n++ {
		cand := syncKeepBothCandidate(rel, fileID, n)
		if occupied[cand] {
			continue // 本次运行内已知占用，快速跳过
		}
		abs, err := resolveLocalTarget(absDir, cand)
		if err != nil {
			return "", "", err // 逃逸/符号链接等，直接失败
		}
		f, oerr := os.OpenFile(abs, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if oerr == nil {
			_ = f.Close()
			return cand, abs, nil
		}
		if os.IsExist(oerr) {
			occupied[cand] = true // 等价目标已存在，记下避免重试，换下一个后缀
			continue
		}
		return "", "", oerr
	}
}

// syncAskStdin 是 --on-conflict=ask 交互提问的输入源，默认 os.Stdin；测试可替换它以
// 注入非交互（EOF）场景。
var syncAskStdin io.Reader = os.Stdin

// driveSyncAskConflict 在 --on-conflict=ask 下交互式询问单个冲突文件的解决策略。
// 返回四种具体策略之一，或 ""（跳过）。当 stdin 在给出选择前结束（管道/无 TTY 等非交互
// 环境）时，按文档约定等价于「跳过」，返回 ""——绝不因此中止整个同步，new_local/new_remote
// 等其余差异仍会照常处理。
func driveSyncAskConflict(rel string) (string, error) {
	fmt.Fprintf(os.Stderr, "冲突: 本地与远端都修改了 %q。请选择 [R]远端优先 / [L]本地优先 / [K]保留两者 / [S]跳过 (默认 R): ", rel)
	line, err := bufio.NewReader(syncAskStdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("读取冲突选择失败 (%s): %w", rel, err)
	}
	switch strings.TrimSpace(strings.ToLower(line)) {
	case "r", "remote", "remote-wins":
		return syncConflictRemoteWins, nil
	case "l", "local", "local-wins":
		return syncConflictLocalWins, nil
	case "k", "keep", "keep-both":
		return syncConflictKeepBoth, nil
	case "s", "skip":
		return "", nil
	case "":
		if errors.Is(err, io.EOF) {
			// 非交互（管道/无 TTY）：stdin 在给出选择前结束。按文档约定等价于跳过，
			// 返回 "" 而非报错——否则单个冲突会让 runDriveSync 直接中止，连
			// new_local/new_remote 都不再处理。
			return "", nil
		}
		return syncConflictRemoteWins, nil // 交互式回车 → 默认远端优先
	default:
		return "", fmt.Errorf("无效的冲突选择: %q（可选 remote/local/keep/skip）", strings.TrimSpace(line))
	}
}
