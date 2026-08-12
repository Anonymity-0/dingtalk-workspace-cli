package helpers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/text/unicode/norm"
)

// ==========================================================
// drive pull — 把钉盘文件夹镜像到本地（Drive → 本地）
// ==========================================================
// ──────────────────────────────────────────────────────────
// dws drive pull — 把钉盘文件夹单向、文件级镜像到本地（Drive → 本地）
//
// 递归列出 --remote-folder 指向的钉盘文件夹下所有 type=FILE 的文件，逐一下载到
// --local-folder 对应的相对路径。已存在的本地文件按 --if-exists 决定
// overwrite / smart / skip。结构化 summary + items 始终打印到 stdout；
// summary.failed > 0 时额外以非零退出码退出。
// ──────────────────────────────────────────────────────────

// --if-exists 的三种策略。
const (
	ifExistsOverwrite = "overwrite" // 总是下载覆盖（Drive 为权威源）
	ifExistsSmart     = "smart"     // 推荐增量：本地 mtime 已 ≥ 远端 modified_time 则跳过
	ifExistsSkip      = "skip"      // 默认：本地已存在则保持不动
)

// pullCreateTemp 仅作为文件系统失败分支的确定性测试 seam；测试必须通过
// testseam.Swap 替换并自动恢复。
var pullCreateTemp = os.CreateTemp

// pull 动作分类。
const (
	pullActionDownloaded = "downloaded"
	pullActionSkipped    = "skipped"
	pullActionFailed     = "failed"
)

// drivePullItem 是输出 items[] 中每个文件的明细。
type drivePullItem struct {
	RelPath string `json:"rel_path"`
	Action  string `json:"action"`
	Error   string `json:"error,omitempty"`
}

// drivePullSummary 是各动作的计数汇总。
type drivePullSummary struct {
	Downloaded int `json:"downloaded"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

// drivePullResult 是 pull 命令的输出 schema。
type drivePullResult struct {
	Summary drivePullSummary `json:"summary"`
	Items   []drivePullItem  `json:"items"`
}

type drivePullDryRunResult struct {
	DryRun      bool            `json:"dry_run"`
	Executed    bool            `json:"executed"`
	PreviewKind string          `json:"preview_kind"`
	Operation   string          `json:"operation"`
	IfExists    string          `json:"if_exists"`
	Plan        drivePullResult `json:"plan"`
}

// drivePartialFailure 在 summary.failed > 0 时返回：结构化结果已打印到 stdout，
// 这里只负责以 exit=1 退出并向 stderr 输出一行简短说明（与 push/sync 一致）。
type drivePartialFailure struct{ failed int }

func (e *drivePartialFailure) Error() string {
	return fmt.Sprintf("drive pull: %d file(s) failed", e.failed)
}
func (e *drivePartialFailure) RawStderr() string { return e.Error() }
func (e *drivePartialFailure) ExitCode() int     { return 1 }

// pathCollisionKey 把本地目标路径归一化成「目标文件系统下的等价键」，用于探测多个
// 远端条目是否会落到同一个本地文件。caseInsensitive 为真时（Windows / 默认 macOS）
// 折叠大小写并做 Unicode NFC 规范化，从而把 A.txt/a.txt、NFC/NFD 记法视为同一目标；
// 大小写敏感文件系统（如 Linux ext4）则按精确路径区分，避免误判合法的异名文件。
func pathCollisionKey(target string, caseInsensitive bool) string {
	p := filepath.Clean(target)
	if caseInsensitive {
		p = norm.NFC.String(strings.ToLower(p))
	}
	return p
}

// isCaseInsensitiveFS 是大小写探测的注入点（与 httpGetFile / httpPutFile 同样的 seam）：
// 生产路径始终是 detectCaseInsensitiveFS，测试可替换它，以便在大小写敏感的 CI 文件系统上
// 也能走到「等价路径冲突」分支。
var isCaseInsensitiveFS = detectCaseInsensitiveFS

// caseProbePattern 是探针文件名模板。抽成变量只为可测：纯数字模板的大写与原名相同，
// 可覆盖「名称无大小写差异、无法据此判定」的回退分支。
var caseProbePattern = "dws-caseprobe-*"

// detectCaseInsensitiveFS 探测 dir 所在文件系统是否大小写不敏感：在 dir 下创建一个随机
// 小写名探针文件，再用其大写名 stat；命中同一文件即为不敏感。dir 必须已存在。
// 探测失败时回退到平台默认（Windows / macOS 视为不敏感，其它敏感）。
func detectCaseInsensitiveFS(dir string) bool {
	platformDefault := runtime.GOOS == "windows" || runtime.GOOS == "darwin"
	f, err := os.CreateTemp(dir, caseProbePattern)
	if err != nil {
		return platformDefault
	}
	name := f.Name()
	_ = f.Close()
	defer os.Remove(name)
	base := filepath.Base(name)
	upper := strings.ToUpper(base)
	if upper == base {
		return platformDefault // 名称无大小写差异，无法据此判定
	}
	_, statErr := os.Stat(filepath.Join(dir, upper))
	return statErr == nil
}

// detectTargetCollisions 按 caseInsensitive 指定的等价规则，找出会映射到同一本地目标
// 的多个远端 rel_path，返回被判定冲突的 rel_path 集合（其中每个都不应落盘）。
func detectTargetCollisions(absDir string, rels []string, caseInsensitive bool) map[string]bool {
	groups := make(map[string][]string, len(rels))
	for _, rel := range rels {
		target := filepath.Join(absDir, filepath.FromSlash(rel))
		key := pathCollisionKey(target, caseInsensitive)
		groups[key] = append(groups[key], rel)
	}
	collided := make(map[string]bool)
	for _, g := range groups {
		if len(g) > 1 {
			for _, r := range g {
				collided[r] = true
			}
		}
	}
	return collided
}

func runDrivePull(cmd *cobra.Command, _ []string) error {
	if err := validateRequiredFlags(cmd, "local-folder", "remote-folder"); err != nil {
		return err
	}
	localDir := mustGetFlag(cmd, "local-folder")
	remoteDirID := mustGetFlag(cmd, "remote-folder")
	// space-id 可选：不传则由 fetchRemoteDriveTree 使用「我的文件」对应的空间。
	spaceID := mustGetFlag(cmd, "space-id")

	ifExists, _ := cmd.Flags().GetString("if-exists")
	if ifExists == "" {
		// 安全默认：不自动覆盖本地既有文件。
		ifExists = ifExistsSkip
	}
	switch ifExists {
	case ifExistsOverwrite, ifExistsSmart, ifExistsSkip:
	default:
		return fmt.Errorf("--if-exists 取值非法: %s（可选 overwrite|smart|skip）", ifExists)
	}

	absDir, err := validateLocalDirAbs(localDir)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	// 复用 status 的远端遍历：按 parentId 递归拿到所有 type=FILE 的文件（key 为 rel_path）。
	remote, err := fetchRemoteDriveTree(ctx, spaceID, remoteDirID, false)
	if err != nil {
		return err
	}

	// 稳定顺序输出（rel_path 升序）。
	relPaths := make([]string, 0, len(remote))
	for rel := range remote {
		relPaths = append(relPaths, rel)
	}
	sort.Strings(relPaths)
	if deps.Caller.DryRun() {
		caseInsensitive := runtime.GOOS == "windows" || runtime.GOOS == "darwin"
		return printDrivePullDryRun(absDir, ifExists, remote, relPaths, caseInsensitive)
	}

	// 落盘前先按目标文件系统的路径等价规则做一次全局冲突检查：大小写不敏感 FS 上
	// A.txt 与 a.txt（或 NFC/NFD 异写）会落到同一本地文件；overwrite 会顺序覆盖、
	// 却把两项都报 downloaded 而静默丢文件。冲突的条目一律标记 failed、都不写入。
	// 探针需要根目录存在，这里按 pull「自动创建目标根」的语义先建好。
	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return fmt.Errorf("创建本地目录失败: %w", err)
	}
	collided := detectTargetCollisions(absDir, relPaths, isCaseInsensitiveFS(absDir))

	res := drivePullResult{Items: make([]drivePullItem, 0, len(relPaths))}
	for _, rel := range relPaths {
		rf := remote[rel]
		// 多个远端条目映射到同一本地目标 → 全部 failed，不写入任何一个，避免静默覆盖丢文件。
		if collided[rel] {
			res.Summary.Failed++
			res.Items = append(res.Items, drivePullItem{
				RelPath: rel, Action: pullActionFailed,
				Error: "多个远端条目在当前文件系统上映射到同一本地路径（大小写/规范化冲突），已跳过以避免覆盖丢失",
			})
			continue
		}
		// 二次确认下载目标仍在本地根目录内，逃逸路径记为 failed 而非落盘。
		localPath, terr := resolveLocalTarget(absDir, rel)
		if terr != nil {
			res.Summary.Failed++
			res.Items = append(res.Items, drivePullItem{RelPath: rel, Action: pullActionFailed, Error: terr.Error()})
			continue
		}

		action, perr := pullOneFile(ctx, spaceID, rf, localPath, ifExists)
		item := drivePullItem{RelPath: rel, Action: action}
		switch action {
		case pullActionDownloaded:
			res.Summary.Downloaded++
		case pullActionSkipped:
			res.Summary.Skipped++
		case pullActionFailed:
			res.Summary.Failed++
			if perr != nil {
				item.Error = perr.Error()
			}
		}
		res.Items = append(res.Items, item)
	}

	// 结构化结果始终打印到 stdout；有失败则额外以非零退出码退出。
	if perr := deps.Out.PrintJSON(res); perr != nil {
		return perr
	}
	if res.Summary.Failed > 0 {
		return &drivePartialFailure{failed: res.Summary.Failed}
	}
	return nil
}

func printDrivePullDryRun(absDir, ifExists string, remote map[string]*remoteFile, relPaths []string, caseInsensitive bool) error {
	plan := drivePullResult{Items: make([]drivePullItem, 0, len(relPaths))}
	collided := detectTargetCollisions(absDir, relPaths, caseInsensitive)
	for _, rel := range relPaths {
		if collided[rel] {
			plan.Summary.Failed++
			plan.Items = append(plan.Items, drivePullItem{
				RelPath: rel, Action: pullActionFailed,
				Error: "多个远端条目在目标平台默认文件系统上映射到同一本地路径，计划已拒绝",
			})
			continue
		}
		localPath, err := resolveLocalTarget(absDir, rel)
		if err != nil {
			plan.Summary.Failed++
			plan.Items = append(plan.Items, drivePullItem{RelPath: rel, Action: pullActionFailed, Error: err.Error()})
			continue
		}
		action := pullActionDownloaded
		if fi, statErr := os.Stat(localPath); statErr == nil && fi.Mode().IsRegular() {
			switch ifExists {
			case ifExistsSkip:
				action = pullActionSkipped
			case ifExistsSmart:
				rf := remote[rel]
				if rf.ModifiedTimeValid && fi.ModTime().UnixMilli() >= rf.ModifiedTime {
					action = pullActionSkipped
				}
			}
		}
		if action == pullActionSkipped {
			plan.Summary.Skipped++
		} else {
			plan.Summary.Downloaded++
		}
		plan.Items = append(plan.Items, drivePullItem{RelPath: rel, Action: action})
	}
	return deps.Out.PrintJSON(drivePullDryRunResult{
		DryRun: true, Executed: false, PreviewKind: "plan", Operation: "drive pull",
		IfExists: ifExists, Plan: plan,
	})
}

// pullOneFile 处理单个远端文件：按 --if-exists 决定是否跳过，否则下载到 localPath。
// 返回动作分类（downloaded / skipped / failed）及失败时的 error。
func pullOneFile(ctx context.Context, spaceID string, rf *remoteFile, localPath, ifExists string) (string, error) {
	// 本地已存在常规文件时，按策略判断是否跳过下载。
	if fi, statErr := os.Stat(localPath); statErr == nil && fi.Mode().IsRegular() {
		switch ifExists {
		case ifExistsSkip:
			return pullActionSkipped, nil
		case ifExistsSmart:
			// 远端时间可信且本地 mtime 已 ≥ 远端 → 视为已对齐，跳过；
			// 时间缺失/非法时不盲跳，退回继续下载。
			if rf.ModifiedTimeValid && fi.ModTime().UnixMilli() >= rf.ModifiedTime {
				return pullActionSkipped, nil
			}
		case ifExistsOverwrite:
			// 总是下载覆盖。
		}
	}

	// download_file → 拿到带签名的下载 URL 与请求头，再 HTTP GET 落盘。
	args := map[string]any{"fileId": rf.FileID}
	if spaceID != "" {
		args["spaceId"] = spaceID
	}
	text, err := callMCPToolReturnText(ctx, "download_file", args)
	if err != nil {
		return pullActionFailed, err
	}
	resourceURL, headers, err := parseDriveDownloadInfo(text)
	if err != nil {
		return pullActionFailed, err
	}

	dir := filepath.Dir(localPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return pullActionFailed, fmt.Errorf("创建本地目录失败: %w", err)
	}

	// 先下载到同目录下的临时文件，完整落盘后再原子 rename 覆盖目标：任何中途失败
	// （网络中断、超时、写盘错误）都不会截断或破坏已存在的原文件。同目录保证
	// rename 在同一文件系统内、可原子替换；rename 会替换目标符号链接本身而非跟随它。
	tmp, err := pullCreateTemp(dir, ".dws-pull-*")
	if err != nil {
		return pullActionFailed, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmp.Name()
	_ = tmp.Close() // httpGetFile 会自行 os.Create 覆盖该临时文件
	// 除非成功 rename，否则始终清理临时文件，不留半成品。
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if err := httpGetFile(ctx, resourceURL, headers, tmpPath); err != nil {
		return pullActionFailed, err
	}
	// 对齐 mtime 放在 rename 之前，保证替换后的目标文件时间戳即为远端时间。
	if rf.ModifiedTimeValid {
		t := time.UnixMilli(rf.ModifiedTime)
		_ = os.Chtimes(tmpPath, t, t)
	}
	if err := driveReplaceFile(tmpPath, localPath); err != nil {
		return pullActionFailed, fmt.Errorf("替换目标文件失败: %w", err)
	}
	committed = true
	return pullActionDownloaded, nil
}
