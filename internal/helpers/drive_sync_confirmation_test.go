package helpers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pull / push / sync 声明 Safety.Confirmation = user_required。本文件断言两件事：
//   1. 未确认（无 --yes、非 dry-run）时命令必须在任何写操作之前拒绝执行；
//   2. 明确确认（--yes）后才发出精确的工具调用并落盘。
// status 是只读叶子（not_required），不受确认门约束。

// ──────────────────────────────────────────────────────────
// 1. 未确认即拒绝，且一个写操作都不发生
// ──────────────────────────────────────────────────────────

func TestCrossPlatformCoverageDriveSyncFamily_refusesWithoutConfirmation(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		setup    func(t *testing.T, dir string)
		listJSON string
	}{
		{
			name: "pull",
			args: []string{"pull"},
			// 远端有文件、本地为空：即便是纯新增也必须先确认。
			listJSON: `{"result":{"items":[{"name":"a.txt","type":"file","fileId":"A","modifyTime":1}],"nextToken":""}}`,
		},
		{
			name:     "push",
			args:     []string{"push"},
			setup:    func(t *testing.T, dir string) { mustWrite(t, filepath.Join(dir, "a.txt"), "local") },
			listJSON: `{"result":{"items":[],"nextToken":""}}`,
		},
		{
			name:     "sync",
			args:     []string{"sync", "--quick", "--on-conflict", "remote-wins"},
			setup:    func(t *testing.T, dir string) { mustWrite(t, filepath.Join(dir, "a.txt"), "local") },
			listJSON: `{"result":{"items":[{"name":"b.txt","type":"file","fileId":"B","modifyTime":2}],"nextToken":""}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.setup != nil {
				tc.setup(t, dir)
			}

			caller := &driveScriptCaller{reply: func(tool string, _ map[string]any, nth int) (string, error) {
				switch tool {
				case "list_files":
					if nth > 0 {
						return `{"result":{"items":[],"nextToken":""}}`, nil
					}
					return tc.listJSON, nil
				case "download_file":
					return `{"result":{"downloadUrl":"https://oss.example.com/get","headers":{}}}`, nil
				case "get_upload_info":
					return `{"result":{"resourceUrls":[{"url":"https://oss.example.com/put","headers":{}}],"uploadId":"U1"}}`, nil
				}
				return `{"result":{"fileId":"NEW"},"success":true}`, nil
			}}
			SetHTTPGetFile(func(_ context.Context, _ string, _ map[string]string, dest string) error {
				t.Error("no download may happen before confirmation")
				return os.WriteFile(dest, []byte("leaked"), 0o644)
			})
			SetHTTPPutFile(func(context.Context, string, map[string]string, string, int64) error {
				t.Error("no upload may happen before confirmation")
				return nil
			})
			t.Cleanup(func() {
				SetHTTPGetFile(nil)
				SetHTTPPutFile(nil)
			})

			args := append(append([]string{}, tc.args...), "--local-folder", dir, "--remote-folder", "ROOT")
			err := runDriveCmdWithoutConfirm(t, caller, args...)
			if err == nil {
				t.Fatal("expected the command to refuse without --yes")
			}
			if !strings.Contains(err.Error(), "--yes") {
				t.Errorf("error must tell the user to confirm with --yes, got %q", err.Error())
			}
			// 拒绝必须发生在任何写工具调用之前。
			for _, tool := range []string{"download_file", "get_upload_info", "create_folder", "ln"} {
				if got := caller.callsFor(tool); len(got) != 0 {
					t.Errorf("%s must not be called before confirmation, got %v", tool, got)
				}
			}
			// 本地目录不得新增任何文件。
			entries, _ := os.ReadDir(dir)
			for _, e := range entries {
				if e.Name() == "b.txt" || e.Name() == "a.txt" && tc.name == "pull" {
					t.Errorf("%s must not be materialized before confirmation", e.Name())
				}
			}
		})
	}
}

// --dry-run 无需 --yes 也能预览：这是让用户看清将发生什么的必要出口，且不写任何东西。
func TestCrossPlatformCoverageDriveSyncFamily_dryRunNeedsNoConfirmation(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "local.txt"), "l")

	caller := syncCaller(map[string]string{
		"ROOT": `{"result":{"items":[{"name":"remote.txt","type":"file","fileId":"R","modifyTime":2}],"nextToken":""}}`,
	})
	caller.dryRun = true
	SetHTTPGetFile(func(context.Context, string, map[string]string, string) error {
		t.Error("dry-run must not download")
		return nil
	})
	SetHTTPPutFile(func(context.Context, string, map[string]string, string, int64) error {
		t.Error("dry-run must not upload")
		return nil
	})
	t.Cleanup(func() {
		SetHTTPGetFile(nil)
		SetHTTPPutFile(nil)
	})

	if err := runDriveCmdWithoutConfirm(t, caller, "sync",
		"--local-folder", dir, "--remote-folder", "ROOT", "--quick"); err != nil {
		t.Fatalf("dry-run must not require confirmation: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "remote.txt")); !os.IsNotExist(err) {
		t.Error("dry-run must not write remote files locally")
	}
}

// ──────────────────────────────────────────────────────────
// 2. 明确确认后才发出精确的工具调用
// ──────────────────────────────────────────────────────────

// sync --on-conflict remote-wins --yes：确认后只下载指定远端文件，且不上传。
func TestCrossPlatformCoverageDriveSync_confirmedRemoteWinsIssuesExactCalls(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	mustWrite(t, p, "local-version")
	local := readFileMTimeMillis(t, p)
	withSyncTransport(t, "remote-version")

	caller := syncCaller(map[string]string{
		"ROOT": `{"result":{"items":[{"name":"f.txt","type":"file","fileId":"F","modifyTime":` +
			differentMillis(local) + `}],"nextToken":""}}`,
	})
	if err := runDriveCmdWithoutConfirm(t, caller, "sync", "--local-folder", dir,
		"--remote-folder", "ROOT", "--quick", "--on-conflict", "remote-wins", "--yes"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dl := caller.callsFor("download_file")
	if len(dl) != 1 || dl[0].args["fileId"] != "F" {
		t.Fatalf("expected exactly one download_file(fileId=F), got %v", dl)
	}
	if up := caller.callsFor("get_upload_info"); len(up) != 0 {
		t.Errorf("remote-wins must not upload, got %v", up)
	}
	if b, _ := os.ReadFile(p); string(b) != "remote-version" {
		t.Errorf("remote-wins must replace the local content, got %q", string(b))
	}
}

// pull --if-exists overwrite --yes：确认后才 download_file(fileId=A) 并覆盖本地内容。
func TestCrossPlatformCoverageDrivePull_confirmedOverwriteIssuesExactCalls(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	mustWrite(t, p, "local-old")

	caller := pullListingCaller(`{"name":"a.txt","type":"file","fileId":"A","modifyTime":9}`)
	SetHTTPGetFile(func(_ context.Context, _ string, _ map[string]string, dest string) error {
		return os.WriteFile(dest, []byte("remote-new"), 0o644)
	})
	t.Cleanup(func() { SetHTTPGetFile(nil) })

	if err := runDriveCmdWithoutConfirm(t, caller, "pull", "--local-folder", dir,
		"--remote-folder", "ROOT", "--if-exists", "overwrite", "--yes"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dl := caller.callsFor("download_file")
	if len(dl) != 1 || dl[0].args["fileId"] != "A" {
		t.Fatalf("expected exactly one download_file(fileId=A), got %v", dl)
	}
	if b, _ := os.ReadFile(p); string(b) != "remote-new" {
		t.Errorf("confirmed overwrite must replace the local file, got %q", string(b))
	}
}

// push --if-exists overwrite --yes：确认后原地覆盖，必须带 overwriteFileId 且不带 parentId。
func TestCrossPlatformCoverageDrivePush_confirmedOverwriteIssuesExactCalls(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "a.txt"), "local")
	withNoopPut(t)

	caller := pushOKCaller(map[string]string{
		"ROOT": `{"result":{"items":[{"name":"a.txt","type":"file","fileId":"A","modifyTime":1}],"nextToken":""}}`,
	})
	if err := runDriveCmdWithoutConfirm(t, caller, "push", "--local-folder", dir,
		"--remote-folder", "ROOT", "--if-exists", "overwrite", "--yes"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	up := caller.callsFor("get_upload_info")
	if len(up) != 1 || up[0].args["overwriteFileId"] != "A" {
		t.Fatalf("expected get_upload_info(overwriteFileId=A), got %v", up)
	}
	if _, has := up[0].args["parentId"]; has {
		t.Error("in-place overwrite must not carry parentId")
	}
}

// sync --on-conflict local-wins --yes：确认后覆盖上传远端，且不下载。
func TestCrossPlatformCoverageDriveSync_confirmedLocalWinsIssuesExactCalls(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	mustWrite(t, p, "local-version")
	local := readFileMTimeMillis(t, p)
	withSyncTransport(t, "unused")

	caller := syncCaller(map[string]string{
		"ROOT": `{"result":{"items":[{"name":"f.txt","type":"file","fileId":"F","modifyTime":` +
			differentMillis(local) + `}],"nextToken":""}}`,
	})
	if err := runDriveCmdWithoutConfirm(t, caller, "sync", "--local-folder", dir,
		"--remote-folder", "ROOT", "--quick", "--on-conflict", "local-wins", "--yes"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	up := caller.callsFor("get_upload_info")
	if len(up) != 1 || up[0].args["overwriteFileId"] != "F" {
		t.Fatalf("expected get_upload_info(overwriteFileId=F), got %v", up)
	}
	if dl := caller.callsFor("download_file"); len(dl) != 0 {
		t.Errorf("local-wins must not download, got %v", dl)
	}
	if b, _ := os.ReadFile(p); string(b) != "local-version" {
		t.Errorf("local-wins must keep the local content, got %q", string(b))
	}
}

// ──────────────────────────────────────────────────────────
// 3. 安全默认值本身
// ──────────────────────────────────────────────────────────

// sync 默认 --on-conflict=skip：两侧都变更时两边内容都保留，且不发出任何传输调用。
func TestCrossPlatformCoverageDriveSync_defaultSkipsConflictsEntirely(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f.txt")
	mustWrite(t, p, "local-version")
	local := readFileMTimeMillis(t, p)
	withSyncTransport(t, "remote-version")

	caller := syncCaller(map[string]string{
		"ROOT": `{"result":{"items":[{"name":"f.txt","type":"file","fileId":"F","modifyTime":` +
			differentMillis(local) + `}],"nextToken":""}}`,
	})
	if err := runDriveCmd(t, caller, "sync", "--local-folder", dir, "--remote-folder", "ROOT", "--quick"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := len(caller.callsFor("download_file")) + len(caller.callsFor("get_upload_info")); got != 0 {
		t.Errorf("default skip must not transfer anything, got %d calls", got)
	}
	if b, _ := os.ReadFile(p); string(b) != "local-version" {
		t.Errorf("default skip must keep the local content, got %q", string(b))
	}
}

// pull 默认 --if-exists=skip：本地已存在的文件不被覆盖。
func TestCrossPlatformCoverageDrivePull_defaultSkipsExistingLocalFiles(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	mustWrite(t, p, "local-old")

	caller := pullListingCaller(`{"name":"a.txt","type":"file","fileId":"A","modifyTime":9}`)
	SetHTTPGetFile(func(context.Context, string, map[string]string, string) error {
		t.Error("default skip must not download over an existing local file")
		return nil
	})
	t.Cleanup(func() { SetHTTPGetFile(nil) })

	if err := runDriveCmd(t, caller, "pull", "--local-folder", dir, "--remote-folder", "ROOT"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b, _ := os.ReadFile(p); string(b) != "local-old" {
		t.Errorf("default skip must keep the local file, got %q", string(b))
	}
}
