package helpers

import (
	"context"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCrossPlatformCoverageWhiteboardExportDownloadsUsingBoardName(t *testing.T) {
	caller := &whiteboardTestCaller{format: "json", response: func(call whiteboardTestCall, _ int) string {
		switch call.tool {
		case "export_whiteboard":
			return `{"jobId":"wb-1","success":true}`
		case "query_export_job":
			return `{"jobId":"wb-1","success":true,"status":"SUCCESS","downloadUrl":"https://example.test/%E6%96%B9%E6%A1%88%E7%99%BD%E6%9D%BF.pdf","logId":"log-1"}`
		default:
			return `{}`
		}
	}}
	output := installWhiteboardTestCaller(t, caller)
	testseam.Swap(t, &httpGetFile, func(_ context.Context, _ string, _ map[string]string, destination string) error {
		return os.WriteFile(destination, []byte("%PDF-test"), 0o644)
	})

	directory := t.TempDir()
	cmd := newWhiteboardCommand()
	cmd.SetArgs([]string{"export", "--node", "board-1", "--export-format", "pdf", "--output", directory})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "方案白板.pdf")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("exported file: %v", err)
	}
	if !strings.Contains(output.String(), filepath.Clean(path)) {
		t.Fatalf("output %q does not contain path %q", output.String(), path)
	}
	wantTools := []string{"export_whiteboard", "query_export_job"}
	gotTools := make([]string, 0, len(caller.calls))
	for _, call := range caller.calls {
		gotTools = append(gotTools, call.tool)
	}
	if !reflect.DeepEqual(gotTools, wantTools) {
		t.Fatalf("tools = %v, want %v", gotTools, wantTools)
	}
	if caller.calls[0].server != "whiteboard" || caller.calls[0].args["exportFormat"] != "pdf" {
		t.Fatalf("export call = %#v", caller.calls[0])
	}
}

func TestCrossPlatformCoverageWhiteboardExportPollsAndValidates(t *testing.T) {
	testseam.Swap(t, &whiteboardExportAfter, func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	})

	queryCount := 0
	caller := &whiteboardTestCaller{format: "json", response: func(call whiteboardTestCall, _ int) string {
		switch call.tool {
		case "export_whiteboard":
			return `{"jobId":"wb-2"}`
		case "query_export_job":
			queryCount++
			if queryCount == 1 {
				return `{"jobId":"wb-2","status":"PROCESSING"}`
			}
			return `{"jobId":"wb-2","status":"SUCCESS","downloadUrl":"https://example.test/board.png"}`
		}
		return `{}`
	}}
	installWhiteboardTestCaller(t, caller)
	testseam.Swap(t, &httpGetFile, func(_ context.Context, _ string, _ map[string]string, destination string) error {
		return os.WriteFile(destination, []byte("\x89PNG\r\n\x1a\nbody"), 0o644)
	})

	cmd := newWhiteboardCommand()
	cmd.SetArgs([]string{"export", "--node", "board-2", "--output", t.TempDir()})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if queryCount != 2 {
		t.Fatalf("query count = %d, want 2", queryCount)
	}

	bad := newWhiteboardCommand()
	bad.SetArgs([]string{"export", "--node", "board-2", "--output", t.TempDir(), "--export-format", "svg"})
	if err := bad.Execute(); err == nil || !strings.Contains(err.Error(), "png or pdf") {
		t.Fatalf("invalid format error = %v", err)
	}
}

func TestCrossPlatformCoverageWhiteboardExportGetUnwrapsResultJSON(t *testing.T) {
	caller := &whiteboardTestCaller{format: "json", response: func(call whiteboardTestCall, _ int) string {
		if call.tool != "query_export_job" {
			return `{}`
		}
		return `{"resultJson":"{\"jobId\":\"wb-wrapped\",\"status\":\"SUCCESS\",\"downloadUrl\":\"https://example.test/wrapped.png\"}"}`
	}}
	installWhiteboardTestCaller(t, caller)
	testseam.Swap(t, &httpGetFile, func(_ context.Context, _ string, _ map[string]string, destination string) error {
		return os.WriteFile(destination, []byte("\x89PNG\r\n\x1a\nbody"), 0o644)
	})

	directory := t.TempDir()
	cmd := newWhiteboardCommand()
	cmd.SetArgs([]string{"export-get", "--job-id", "wb-wrapped", "--export-format", "png", "--output", directory})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(directory, "wrapped.png")); err != nil {
		t.Fatalf("wrapped export: %v", err)
	}
}

func TestCrossPlatformCoverageWhiteboardExportGetRejectsFormatMismatchBeforeDownload(t *testing.T) {
	caller := &whiteboardTestCaller{format: "json", response: func(call whiteboardTestCall, _ int) string {
		if call.tool != "query_export_job" {
			return `{}`
		}
		return `{"result":{"jobId":"wb-pdf","status":"SUCCESS","downloadUrl":"https://example.test/board.pdf?Expires=1"}}`
	}}
	installWhiteboardTestCaller(t, caller)
	downloaded := false
	testseam.Swap(t, &httpGetFile, func(_ context.Context, _ string, _ map[string]string, _ string) error {
		downloaded = true
		return nil
	})

	outputDir := filepath.Join(t.TempDir(), "must-not-be-created")
	cmd := newWhiteboardCommand()
	cmd.SetArgs([]string{"export-get", "--job-id", "wb-pdf", "--export-format", "png", "--output", outputDir})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "任务格式不匹配") || !strings.Contains(err.Error(), "pdf") || !strings.Contains(err.Error(), "png") {
		t.Fatalf("format mismatch error = %v", err)
	}
	if downloaded {
		t.Fatal("format mismatch must stop before download")
	}
	if _, statErr := os.Stat(outputDir); !os.IsNotExist(statErr) {
		t.Fatalf("format mismatch created output directory: %v", statErr)
	}
}

func TestCrossPlatformCoverageWhiteboardExportDryRunDoesNotCallOrWrite(t *testing.T) {
	for _, leaf := range []string{"export", "export-get"} {
		t.Run(leaf, func(t *testing.T) {
			caller := &whiteboardTestCaller{dry: true, format: "json"}
			out := installWhiteboardTestCaller(t, caller)
			dir := filepath.Join(t.TempDir(), "absent")
			args := []string{leaf, "--output", dir}
			if leaf == "export" {
				args = append(args, "--node", "board")
			} else {
				args = append(args, "--job-id", "job")
			}
			cmd := newWhiteboardCommand()
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if len(caller.calls) != 0 {
				t.Fatalf("dry-run called service: %v", caller.calls)
			}
			if !strings.Contains(out.String(), "dry_run") {
				t.Fatalf("missing preview: %s", out)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("dry-run created directory: %v", err)
			}
		})
	}
}

func TestCrossPlatformCoverageWhiteboardExportRejectsInvalidDownloadAndAllowsRetry(t *testing.T) {
	for _, content := range []string{"", "<html>error</html>", "%PDF-1.7", "\x89PNG"} {
		t.Run(content, func(t *testing.T) {
			installWhiteboardTestCaller(t, &whiteboardTestCaller{format: "json"})
			body := content
			testseam.Swap(t, &httpGetFile, func(_ context.Context, _ string, _ map[string]string, path string) error {
				return os.WriteFile(path, []byte(body), 0600)
			})
			dir := t.TempDir()
			err := downloadWhiteboardExport(context.Background(), "job", "png", dir, "https://example.test/board.png", nil)
			if err == nil {
				t.Fatal("invalid download succeeded")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed download left files: %v %v", entries, err)
			}
			body = "\x89PNG\r\n\x1a\nbody"
			if err := downloadWhiteboardExport(context.Background(), "job", "png", dir, "https://example.test/board.png", nil); err != nil {
				t.Fatalf("retry failed: %v", err)
			}
		})
	}
}

func TestCrossPlatformCoverageWhiteboardExportSignedFilenameAndConflict(t *testing.T) {
	installWhiteboardTestCaller(t, &whiteboardTestCaller{format: "json"})
	calls := 0
	testseam.Swap(t, &httpGetFile, func(_ context.Context, _ string, _ map[string]string, path string) error {
		calls++
		return os.WriteFile(path, []byte("%PDF-1.7"), 0600)
	})
	dir := t.TempDir()
	url := "https://example.test/%E7%99%BD%E6%9D%BF.pdf?Signature=abc/def#fragment"
	if err := downloadWhiteboardExport(context.Background(), "job", "pdf", dir, url, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "白板.pdf")); err != nil {
		t.Fatal(err)
	}
	err := downloadWhiteboardExport(context.Background(), "job", "pdf", dir, url, nil)
	if err == nil || strings.Contains(err.Error(), "--overwrite") || !strings.Contains(err.Error(), "--output") {
		t.Fatalf("conflict error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("conflict downloaded again: %d", calls)
	}
}

func TestCrossPlatformCoverageWhiteboardExportRejectsFileDirectoryBeforeSubmission(t *testing.T) {
	for _, leaf := range []string{"export", "export-get"} {
		t.Run(leaf, func(t *testing.T) {
			caller := &whiteboardTestCaller{format: "json"}
			installWhiteboardTestCaller(t, caller)
			path := filepath.Join(t.TempDir(), "file")
			if err := os.WriteFile(path, []byte("existing"), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{leaf, "--output", path}
			if leaf == "export" {
				args = append(args, "--node", "board")
			} else {
				args = append(args, "--job-id", "recover-job", "--export-format", "pdf")
			}
			cmd := newWhiteboardCommand()
			cmd.SetArgs(args)
			err := cmd.Execute()
			if err == nil || len(caller.calls) != 0 {
				t.Fatalf("error=%v calls=%v", err, caller.calls)
			}
			if leaf == "export-get" && !strings.Contains(err.Error(), "--export-format pdf") {
				t.Fatalf("missing recovery: %v", err)
			}
		})
	}
}

func TestCrossPlatformCoverageWhiteboardExportPollingFailuresPreserveRecovery(t *testing.T) {
	for _, status := range []string{"PROCESSING", "FAILED", "UNEXPECTED", "SUCCESS", "cancelled-context"} {
		t.Run(status, func(t *testing.T) {
			caller := &whiteboardTestCaller{format: "json", response: func(whiteboardTestCall, int) string { return `{"jobId":"job","status":"` + status + `"}` }}
			installWhiteboardTestCaller(t, caller)
			testseam.Swap(t, &whiteboardExportAfter, func(time.Duration) <-chan time.Time { ch := make(chan time.Time, 1); ch <- time.Now(); return ch })
			cmd := newWhiteboardCommand()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if status == "cancelled-context" {
				cancel()
			}
			cmd.SetContext(ctx)
			dir := filepath.Join(t.TempDir(), "space ' directory")
			cmd.SetArgs([]string{"export-get", "--job-id", "job", "--export-format", "pdf", "--output", dir})
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected failure")
			}
			for _, part := range []string{"--job-id job", "--export-format pdf", ShellQuoteArg(dir)} {
				if !strings.Contains(err.Error(), part) {
					t.Fatalf("missing %q in %v", part, err)
				}
			}
			if status == "PROCESSING" && len(caller.calls) != 30 {
				t.Fatalf("poll count=%d", len(caller.calls))
			}
			if status == "cancelled-context" && len(caller.calls) != 0 {
				t.Fatalf("cancelled call=%v", caller.calls)
			}
			if _, err := os.Stat(dir); !os.IsNotExist(err) {
				t.Fatalf("failure created directory: %v", err)
			}
		})
	}
}
