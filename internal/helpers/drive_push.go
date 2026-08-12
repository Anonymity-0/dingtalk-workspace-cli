package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// ==========================================================
// drive push — 把本地文件夹镜像到钉盘（本地 → Drive）
// ==========================================================
// ──────────────────────────────────────────────────────────
// dws drive push — 把本地目录单向、文件级镜像到钉盘（本地 → Drive）
//
// 递归遍历 --local-folder 下所有常规文件与子目录（含空目录），按相对路径在
// --remote-folder 指向的钉盘文件夹里新建/覆盖/跳过。已存在的远端目录复用其
// fileId、不重建；缺失的目录按需 create_folder。文件按 --if-exists 决定
// skip / smart / overwrite。summary.failed > 0 时以非零退出码退出（结构化
// summary + items 仍打印在 stdout 上）。
// ──────────────────────────────────────────────────────────

// push 动作分类。
const (
	pushActionUploaded      = "uploaded"       // 新建上传
	pushActionOverwritten   = "overwritten"    // 覆盖已存在的远端文件
	pushActionSkipped       = "skipped"        // 按 --if-exists 跳过
	pushActionFolderCreated = "folder_created" // 新建远端目录（不计入 uploaded）
	pushActionFailed        = "failed"
)

// localPushFile 描述一个待推送的本地常规文件。
type localPushFile struct {
	RelPath       string
	AbsPath       string
	ModTimeMillis int64
	Size          int64
}

// drivePushItem 是输出 items[] 中每个条目的明细。
type drivePushItem struct {
	RelPath   string `json:"rel_path"`
	Action    string `json:"action"`
	SizeBytes *int64 `json:"size_bytes,omitempty"`
	Error     string `json:"error,omitempty"`
}

// drivePushSummary 是各动作的计数汇总。uploaded 同时统计新建与覆盖。
type drivePushSummary struct {
	Uploaded int  `json:"uploaded"`
	Skipped  int  `json:"skipped"`
	Failed   int  `json:"failed"`
	Aborted  bool `json:"aborted"`
}

type drivePushResult struct {
	Summary drivePushSummary `json:"summary"`
	Items   []drivePushItem  `json:"items"`
}

// drivePushFailure 在 summary.failed > 0 时返回：结构化结果已打印到 stdout，
// 这里只负责以 exit=1 退出并向 stderr 输出一行简短说明。
type drivePushFailure struct{ failed int }

func (e *drivePushFailure) Error() string {
	return fmt.Sprintf("drive push: %d file(s) failed", e.failed)
}
func (e *drivePushFailure) RawStderr() string { return e.Error() }
func (e *drivePushFailure) ExitCode() int     { return 1 }

func runDrivePush(cmd *cobra.Command, _ []string) error {
	if err := validateRequiredFlags(cmd, "local-folder", "remote-folder"); err != nil {
		return err
	}
	localDir := mustGetFlag(cmd, "local-folder")
	remoteDirID := mustGetFlag(cmd, "remote-folder")
	spaceID := mustGetFlag(cmd, "space-id")

	// push 默认 skip（安全，只新增不覆盖）。
	ifExists, _ := cmd.Flags().GetString("if-exists")
	if ifExists == "" {
		ifExists = ifExistsSkip
	}
	switch ifExists {
	case ifExistsSkip, ifExistsSmart, ifExistsOverwrite:
	default:
		return fmt.Errorf("--if-exists 取值非法: %s（可选 skip|smart|overwrite）", ifExists)
	}

	absDir, err := validateLocalDirAbs(localDir)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	// 远端现状：已存在的文件（用于 --if-exists 判断）与目录（rel_path → fileId，用于复用/定位父目录）。
	remoteFiles, remoteFolders, err := fetchRemoteTreeForPush(ctx, spaceID, remoteDirID)
	if err != nil {
		return err
	}

	localDirs, localFiles, err := walkLocalForPush(absDir)
	if err != nil {
		return fmt.Errorf("扫描本地目录失败: %w", err)
	}

	res := drivePushResult{Items: make([]drivePushItem, 0, len(localDirs)+len(localFiles))}

	// 第一阶段：按需创建远端目录（浅层在前，保证父目录先于子目录存在）。
	for _, dir := range localDirs {
		if _, ok := remoteFolders[dir]; ok {
			continue // 远端已存在，复用 fileId，不留痕
		}
		parentRel, name := splitRel(dir)
		parentID, ok := remoteFolders[parentRel]
		if !ok || parentID == "" {
			res.Summary.Failed++
			res.Items = append(res.Items, drivePushItem{RelPath: dir, Action: pushActionFailed, Error: "父目录未能创建"})
			continue
		}
		fid, cerr := pushCreateFolder(ctx, spaceID, parentID, name)
		if cerr != nil || fid == "" {
			res.Summary.Failed++
			msg := "create_folder 未返回 fileId"
			if cerr != nil {
				msg = cerr.Error()
			}
			res.Items = append(res.Items, drivePushItem{RelPath: dir, Action: pushActionFailed, Error: msg})
			continue
		}
		remoteFolders[dir] = fid
		res.Items = append(res.Items, drivePushItem{RelPath: dir, Action: pushActionFolderCreated})
	}

	// 第二阶段：上传/覆盖/跳过文件。
	for i := range localFiles {
		lf := localFiles[i]
		size := lf.Size
		parentRel, name := splitRel(lf.RelPath)
		parentID, ok := remoteFolders[parentRel]
		if !ok || parentID == "" {
			res.Summary.Failed++
			res.Items = append(res.Items, drivePushItem{RelPath: lf.RelPath, Action: pushActionFailed, SizeBytes: &size, Error: "父目录未能创建"})
			continue
		}

		rf, exists := remoteFiles[lf.RelPath]
		action := pushActionUploaded
		overwriteID := ""
		if exists {
			switch ifExists {
			case ifExistsSkip:
				res.Summary.Skipped++
				res.Items = append(res.Items, drivePushItem{RelPath: lf.RelPath, Action: pushActionSkipped, SizeBytes: &size})
				continue
			case ifExistsSmart:
				// 远端时间可信且已 ≥ 本地 → 跳过；否则走覆盖路径。
				if rf.ModifiedTimeValid && rf.ModifiedTime >= lf.ModTimeMillis {
					res.Summary.Skipped++
					res.Items = append(res.Items, drivePushItem{RelPath: lf.RelPath, Action: pushActionSkipped, SizeBytes: &size})
					continue
				}
				action = pushActionOverwritten
			case ifExistsOverwrite:
				action = pushActionOverwritten
			}
			// 覆盖分支必须走覆盖上传（传 overwriteFileId、不传 parentId），
			// 否则会在同目录新建重名副本而非原地覆盖。
			if action == pushActionOverwritten {
				overwriteID = rf.FileID
			}
		}

		if err := pushUploadFile(ctx, spaceID, parentID, overwriteID, name, lf.AbsPath, size); err != nil {
			res.Summary.Failed++
			res.Items = append(res.Items, drivePushItem{RelPath: lf.RelPath, Action: pushActionFailed, SizeBytes: &size, Error: err.Error()})
			continue
		}
		res.Summary.Uploaded++ // uploaded 同时统计新建与覆盖
		res.Items = append(res.Items, drivePushItem{RelPath: lf.RelPath, Action: action, SizeBytes: &size})
	}

	// 结构化结果始终打印到 stdout；有失败则额外以非零退出码退出。
	if perr := deps.Out.PrintJSON(res); perr != nil {
		return perr
	}
	if res.Summary.Failed > 0 {
		return &drivePushFailure{failed: res.Summary.Failed}
	}
	return nil
}

// fetchRemoteTreeForPush 递归拉取 rootID 下的远端现状：files（rel_path → *remoteFile，
// 用于 --if-exists 判断）与 folders（rel_path → fileId，rel_path "" 即 rootID 本身）。
func fetchRemoteTreeForPush(ctx context.Context, spaceID, rootID string) (map[string]*remoteFile, map[string]string, error) {
	files := make(map[string]*remoteFile)
	folders := map[string]string{"": rootID}
	if err := walkRemoteForPush(ctx, spaceID, rootID, "", files, folders, 0); err != nil {
		return nil, nil, err
	}
	return files, folders, nil
}

func walkRemoteForPush(ctx context.Context, spaceID, parentID, relBase string, files map[string]*remoteFile, folders map[string]string, depth int) error {
	if depth > remoteMaxDepth {
		return fmt.Errorf("drive 目录层级超过 %d 层，疑似循环引用，已中止", remoteMaxDepth)
	}
	nextToken := ""
	for {
		args := map[string]any{"maxResults": float64(driveListPageSize)}
		if spaceID != "" {
			args["spaceId"] = spaceID
		}
		if parentID != "" {
			args["parentId"] = parentID
		}
		if nextToken != "" {
			args["nextToken"] = nextToken
		}

		text, err := callMCPToolReturnText(ctx, "list_files", args)
		if err != nil {
			return err
		}
		items, token, err := parseDriveList(text)
		if err != nil {
			return err
		}

		for _, it := range items {
			name := it.name()
			if name == "" {
				continue
			}
			// 与 walkRemoteDir 一致：非规范名称跳过，避免逃逸性 rel_path 进入索引。
			if !isSafeRemoteSegment(name) {
				slog.Warn("overlay: 跳过含非法路径成分的远端条目", "name", name, "relBase", relBase)
				continue
			}
			childRel := name
			if relBase != "" {
				childRel = relBase + "/" + name
			}
			if it.isFolder() {
				folders[childRel] = it.id()
				if err := walkRemoteForPush(ctx, spaceID, it.id(), childRel, files, folders, depth+1); err != nil {
					return err
				}
				continue
			}
			if !it.isFile() {
				continue
			}
			modMillis, modValid := it.modifiedMillis()
			files[childRel] = &remoteFile{
				RelPath:           childRel,
				FileID:            it.id(),
				Hash:              it.hash(),
				ModifiedTime:      modMillis,
				ModifiedTimeValid: modValid,
			}
		}

		if token == "" || token == nextToken {
			break
		}
		nextToken = token
	}
	return nil
}

// walkLocalForPush 遍历本地根目录，返回所有子目录 rel_path（不含根本身，浅层在前）
// 与所有常规文件。dirs 升序排序保证父目录先于子目录被创建。
func walkLocalForPush(root string) ([]string, []localPushFile, error) {
	var dirs []string
	var files []localPushFile
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil // 根目录本身不处理
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			dirs = append(dirs, relSlash)
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return ierr
		}
		// 只推送常规文件；符号链接、设备文件等忽略。
		if !info.Mode().IsRegular() {
			return nil
		}
		files = append(files, localPushFile{
			RelPath:       relSlash,
			AbsPath:       path,
			ModTimeMillis: info.ModTime().UnixMilli(),
			Size:          info.Size(),
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	// 字典序即浅层在前（"a" < "a/b" < "a/b/c"），父目录先于子目录。
	sort.Strings(dirs)
	return dirs, files, nil
}

// pushCreateFolder 在 parentID 下创建名为 name 的文件夹，返回新目录的 fileId。
func pushCreateFolder(ctx context.Context, spaceID, parentID, name string) (string, error) {
	args := map[string]any{"name": name}
	if spaceID != "" {
		args["spaceId"] = spaceID
	}
	if parentID != "" {
		args["parentId"] = parentID
	}
	text, err := callMCPToolReturnText(ctx, "create_folder", args)
	if err != nil {
		return "", err
	}
	return parseNodeID(text), nil
}

// pushUploadFile 走 get_upload_info → OSS PUT → commit_upload 三步，把本地文件
// 上传到 parentID 目录下。复用 drive upload 的凭证解析与 HTTP PUT。
//
// overwriteFileID 非空时走覆盖流程（与 uploadToDrive 一致）：get_upload_info 与
// commit_upload 两个阶段都传 overwriteFileId、都不传 parentId（服务端据此设置
// conflictStrategy=OVERWRITE，原地覆盖而非在同目录新建重名副本）。
func pushUploadFile(ctx context.Context, spaceID, parentID, overwriteFileID, fileName, filePath string, fileSize int64) error {
	step1 := map[string]any{"fileName": fileName, "fileSize": float64(fileSize)}
	if spaceID != "" {
		step1["spaceId"] = spaceID
	}
	if overwriteFileID != "" {
		step1["overwriteFileId"] = overwriteFileID
	} else if parentID != "" {
		step1["parentId"] = parentID
	}
	text, err := callMCPToolReturnText(ctx, "get_upload_info", step1)
	if err != nil {
		return err
	}
	resourceURL, uploadID, ossHeaders, err := parseDriveUploadInfo(text)
	if err != nil {
		return err
	}
	if err := httpPutFile(ctx, resourceURL, ossHeaders, filePath, fileSize); err != nil {
		return err
	}
	commit := map[string]any{"fileName": fileName, "fileSize": float64(fileSize), "uploadId": uploadID}
	if spaceID != "" {
		commit["spaceId"] = spaceID
	}
	if overwriteFileID != "" {
		commit["overwriteFileId"] = overwriteFileID
	} else if parentID != "" {
		commit["parentId"] = parentID
	}
	_, err = callMCPToolReturnText(ctx, "commit_upload", commit)
	return err
}

// parseNodeID 从 create_folder / commit 等返回里抽出节点 fileId（带 fallback）。
func parseNodeID(text string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(text), &data); err != nil {
		return ""
	}
	if r, ok := data["result"].(map[string]any); ok {
		data = r
	}
	for _, k := range []string{"fileId", "dentryUuid", "dentryId", "id", "nodeId"} {
		if s, ok := data[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// splitRel 把 rel_path 拆成父路径与末段名："a/b/c" → ("a/b","c")，"c" → ("","c")。
func splitRel(rel string) (string, string) {
	if i := strings.LastIndex(rel, "/"); i >= 0 {
		return rel[:i], rel[i+1:]
	}
	return "", rel
}

// checkFileNotDingTalkDoc 通过 get_file_info 检查文件类型，
// 若为钉钉在线文档 (adoc/axls/amind/adraw) 则返回错误，提示使用对应服务命令。
// 探测失败（如无效 fileId）不阻断流程，让后续 MCP 工具自行报错。
