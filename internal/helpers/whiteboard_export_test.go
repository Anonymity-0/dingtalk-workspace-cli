package helpers

import (
	"context"
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
	oldGet := httpGetFile
	httpGetFile = func(_ context.Context, _ string, _ map[string]string, destination string) error {
		return os.WriteFile(destination, []byte("%PDF-test"), 0o644)
	}
	t.Cleanup(func() { httpGetFile = oldGet })

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
	oldAfter := whiteboardExportAfter
	whiteboardExportAfter = func(time.Duration) <-chan time.Time {
		ch := make(chan time.Time, 1)
		ch <- time.Now()
		return ch
	}
	t.Cleanup(func() { whiteboardExportAfter = oldAfter })

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
	oldGet := httpGetFile
	httpGetFile = func(_ context.Context, _ string, _ map[string]string, destination string) error {
		return os.WriteFile(destination, []byte("png"), 0o644)
	}
	t.Cleanup(func() { httpGetFile = oldGet })

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
	oldGet := httpGetFile
	httpGetFile = func(_ context.Context, _ string, _ map[string]string, destination string) error {
		return os.WriteFile(destination, []byte("png"), 0o644)
	}
	t.Cleanup(func() { httpGetFile = oldGet })

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
	oldGet := httpGetFile
	httpGetFile = func(_ context.Context, _ string, _ map[string]string, _ string) error {
		downloaded = true
		return nil
	}
	t.Cleanup(func() { httpGetFile = oldGet })

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
