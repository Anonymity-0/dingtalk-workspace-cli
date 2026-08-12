package helpers

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// failFolderCaller 让指定名字的 create_folder 失败，其余一切正常，
// 用于构造「父目录未能创建 → 子目录/子文件连锁失败」。
func failFolderCaller(failName string) *driveScriptCaller {
	return &driveScriptCaller{reply: func(tool string, args map[string]any, _ int) (string, error) {
		switch tool {
		case "list_files":
			return `{"result":{"items":[],"nextToken":""}}`, nil
		case "create_folder":
			if args["name"] == failName {
				return "", errors.New("create_folder rejected: " + failName)
			}
			return `{"result":{"fileId":"NEWDIR"},"success":true}`, nil
		case "get_upload_info":
			return `{"result":{"resourceUrls":[{"url":"https://oss.example.com/put","headers":{}}],"uploadId":"U1"}}`, nil
		case "download_file":
			return `{"result":{"downloadUrl":"https://oss.example.com/get","headers":{}}}`, nil
		}
		return `{"result":{"fileId":"NEWFILE"},"success":true}`, nil
	}}
}

// mkNested 建出 a/b/deep.txt 结构。
func mkNested(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "a", "b", "deep.txt"), "d")
	return dir
}

// 顶层目录 a 建失败 → 子目录 a/b 因父目录缺失也 failed，其中文件同样 failed。
func TestDrivePush_nestedDirFailsWhenParentMissing(t *testing.T) {
	dir := mkNested(t)
	withNoopPut(t)

	caller := failFolderCaller("a")
	err := runDriveCmd(t, caller, "push", "--local-folder", dir, "--remote-folder", "ROOT")
	var pf *drivePushFailure
	if !errors.As(err, &pf) {
		t.Fatalf("expected drivePushFailure, got %T %v", err, err)
	}
	// a（建失败）+ a/b（父缺失）+ a/b/deep.txt（父缺失）= 3
	if pf.failed != 3 {
		t.Errorf("failed = %d, want 3", pf.failed)
	}
	if got := caller.callsFor("get_upload_info"); len(got) != 0 {
		t.Errorf("nothing may upload without a parent folder, got %v", got)
	}
}

func TestDriveSync_nestedDirFailsWhenParentMissing(t *testing.T) {
	dir := mkNested(t)
	withNoopPut(t)

	caller := failFolderCaller("a")
	err := runDriveCmd(t, caller, "sync", "--local-folder", dir, "--remote-folder", "ROOT", "--quick")
	var sf *driveSyncFailure
	if !errors.As(err, &sf) {
		t.Fatalf("expected driveSyncFailure, got %T %v", err, err)
	}
	if sf.failed != 3 {
		t.Errorf("failed = %d, want 3", sf.failed)
	}
}

// new_remote 落盘目标经软链逃逸出本地根 → 记 failed，不写到根目录之外。
func TestDriveSync_newRemoteEscapeIsFailed(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "sub")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	withSyncTransport(t, "leak")

	caller := syncCaller(map[string]string{
		"ROOT": `{"result":{"items":[{"name":"sub","type":"folder","fileId":"SUB"}],"nextToken":""}}`,
		"SUB":  `{"result":{"items":[{"name":"leaked.txt","type":"file","fileId":"L","modifyTime":1}],"nextToken":""}}`,
	})
	err := runDriveCmd(t, caller, "sync", "--local-folder", root, "--remote-folder", "ROOT", "--quick")
	var sf *driveSyncFailure
	if !errors.As(err, &sf) {
		t.Fatalf("expected driveSyncFailure, got %T %v", err, err)
	}
	if _, statErr := os.Stat(filepath.Join(outside, "leaked.txt")); !os.IsNotExist(statErr) {
		t.Error("file escaped the local root")
	}
}

// ──────────────────────────────────────────────────────────
// keep-both 的失败与回滚分支
// ──────────────────────────────────────────────────────────

// syncKeepBoth 在 rel 逃逸时直接 failed（resolveLocalTarget 报错）。
func TestSyncKeepBoth_escapingRelIsFailed(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "sub")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	res := &driveSyncResult{}
	syncKeepBoth(res, context.Background(), "", &remoteFile{RelPath: "sub/f.txt", FileID: "FID12345678"},
		root, "sub/f.txt", map[string]bool{}, map[string]bool{})
	if res.Summary.Failed != 1 {
		t.Fatalf("failed = %d, want 1", res.Summary.Failed)
	}
}

// collided 命中时 keep-both 直接 failed，不做任何改名。
func TestSyncKeepBoth_collidedIsFailed(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "f.txt"), "local")
	res := &driveSyncResult{}
	syncKeepBoth(res, context.Background(), "", &remoteFile{RelPath: "f.txt", FileID: "FID12345678"},
		root, "f.txt", map[string]bool{"f.txt": true}, map[string]bool{})
	if res.Summary.Failed != 1 {
		t.Fatalf("failed = %d, want 1", res.Summary.Failed)
	}
	if b, _ := os.ReadFile(filepath.Join(root, "f.txt")); string(b) != "local" {
		t.Errorf("collided keep-both must not touch the local file, got %q", string(b))
	}
}

// 占用改名目标失败（目录不可写）→ failed。
func TestSyncKeepBoth_reserveFailureIsFailed(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("read-only dir enforcement needs POSIX perms and a non-root user")
	}
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "f.txt"), "local")
	if err := os.Chmod(root, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(root, 0o755) })

	res := &driveSyncResult{}
	syncKeepBoth(res, context.Background(), "", &remoteFile{RelPath: "f.txt", FileID: "FID12345678"},
		root, "f.txt", map[string]bool{}, map[string]bool{})
	if res.Summary.Failed != 1 {
		t.Fatalf("failed = %d, want 1", res.Summary.Failed)
	}
}

// 改名本身失败（本地原文件其实是目录）→ failed，并清理刚建的空占位。
func TestSyncKeepBoth_renameFailureCleansPlaceholder(t *testing.T) {
	root := t.TempDir()
	// rel 指向一个非空目录：os.Rename(dir, emptyFile) 会失败。
	if err := os.MkdirAll(filepath.Join(root, "f.txt", "inner"), 0o755); err != nil {
		t.Fatal(err)
	}
	res := &driveSyncResult{}
	occupied := map[string]bool{}
	syncKeepBoth(res, context.Background(), "", &remoteFile{RelPath: "f.txt", FileID: "FID12345678"},
		root, "f.txt", map[string]bool{}, occupied)

	if res.Summary.Failed != 1 {
		t.Fatalf("failed = %d, want 1", res.Summary.Failed)
	}
	// 空占位必须被清理，不留残留。
	if _, err := os.Stat(filepath.Join(root, syncKeepBothCandidate("f.txt", "FID12345678", 0))); !os.IsNotExist(err) {
		t.Error("empty placeholder must be removed after a failed rename")
	}
}

// 拉取失败 → 回滚改名，本地文件恢复原名与原内容。
func TestSyncKeepBoth_pullFailureRollsBackRename(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "f.txt")
	mustWrite(t, p, "local-version")

	prevDeps, prevArgs := deps, os.Args
	deps = &Deps{Caller: syncCaller(nil), Out: &Formatter{w: io.Discard}}
	os.Args = []string{"dws", "drive"}
	SetHTTPGetFile(func(context.Context, string, map[string]string, string) error { return errTestDownload })
	t.Cleanup(func() {
		deps, os.Args = prevDeps, prevArgs
		SetHTTPGetFile(nil)
	})

	res := &driveSyncResult{}
	syncKeepBoth(res, context.Background(), "", &remoteFile{RelPath: "f.txt", FileID: "FID12345678"},
		root, "f.txt", map[string]bool{}, map[string]bool{})

	if res.Summary.Failed != 1 {
		t.Fatalf("failed = %d, want 1", res.Summary.Failed)
	}
	if b, err := os.ReadFile(p); err != nil || string(b) != "local-version" {
		t.Fatalf("rename must be rolled back: %v / %q", err, string(b))
	}
	if _, err := os.Stat(filepath.Join(root, syncKeepBothCandidate("f.txt", "FID12345678", 0))); !os.IsNotExist(err) {
		t.Error("renamed copy must not survive a successful rollback")
	}
}

// 拉取失败且回滚也失败 → 如实上报“本地版本保留为 <改名>”，不谎称成功。
func TestSyncKeepBoth_rollbackFailureIsReported(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "f.txt")
	mustWrite(t, p, "local-version")

	prevDeps, prevArgs := deps, os.Args
	deps = &Deps{Caller: syncCaller(nil), Out: &Formatter{w: io.Discard}}
	os.Args = []string{"dws", "drive"}
	// 下载失败前先把原名占住一个目录，使回滚 rename 无法复原。
	SetHTTPGetFile(func(context.Context, string, map[string]string, string) error {
		if err := os.MkdirAll(filepath.Join(p, "blocker"), 0o755); err != nil {
			return err
		}
		return errTestDownload
	})
	t.Cleanup(func() {
		deps, os.Args = prevDeps, prevArgs
		SetHTTPGetFile(nil)
	})

	res := &driveSyncResult{}
	syncKeepBoth(res, context.Background(), "", &remoteFile{RelPath: "f.txt", FileID: "FID12345678"},
		root, "f.txt", map[string]bool{}, map[string]bool{})

	if res.Summary.Failed != 1 {
		t.Fatalf("failed = %d, want 1", res.Summary.Failed)
	}
	if len(res.Items) != 1 || res.Items[0].Action != syncActionFailed {
		t.Fatalf("items = %+v", res.Items)
	}
	// 数据没丢：本地版本仍以改名后的名字存在，且错误信息点明了这一点。
	suffixed := syncKeepBothCandidate("f.txt", "FID12345678", 0)
	if b, err := os.ReadFile(filepath.Join(root, suffixed)); err != nil || string(b) != "local-version" {
		t.Fatalf("local version must survive as %s: %v / %q", suffixed, err, string(b))
	}
	if !strings.Contains(res.Items[0].Error, "回滚改名失败") {
		t.Errorf("error must disclose the failed rollback, got %q", res.Items[0].Error)
	}
}
