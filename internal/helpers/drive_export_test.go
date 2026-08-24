package helpers

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

// ── inferExportFilename：URL 推断本地文件名 ──

func TestCrossPlatformCoverageInferExportFilename(t *testing.T) {
	cases := []struct {
		name     string
		rawURL   string
		fallback string
		want     string
	}{
		{"query string stripped", "https://x.test/files/report.docx?sig=1", "fallback.docx", "report.docx"},
		{"percent-decoded path", "https://x.test/files/a%2Fb.xlsx", "fallback.xlsx", "b.xlsx"},
		{"backslash converted", `https://x.test/files\a\report.pdf`, "fallback.pdf", "report.pdf"},
		{"empty url uses fallback", "", "fallback.docx", "fallback.docx"},
		{"root only url uses fallback", "https://x.test/", "fallback.docx", "fallback.docx"},
		{"invalid escape keeps raw name", "https://x.test/%", "fallback.docx", "%"},
	}
	for _, tc := range cases {
		if got := inferExportFilename(tc.rawURL, tc.fallback); got != tc.want {
			t.Errorf("%s: inferExportFilename(%q) = %q, want %q", tc.name, tc.rawURL, got, tc.want)
		}
	}
}

// ── resolveDriveExportOutputPath：目录/文件路径与扩展名对齐 ──

func TestCrossPlatformCoverageResolveDriveExportOutputPath(t *testing.T) {
	dir := t.TempDir()

	cases := []struct {
		name        string
		outputPath  string
		downloadURL string
		fileName    string
		fileExt     string
		jobID       string
		want        string
	}{
		{
			name:       "non-directory path unchanged",
			outputPath: filepath.Join(dir, "custom.bin"),
			fileExt:    ".docx",
			want:       filepath.Join(dir, "custom.bin"),
		},
		{
			name:       "nonexistent path unchanged",
			outputPath: filepath.Join(dir, "missing", "target.docx"),
			fileExt:    ".docx",
			want:       filepath.Join(dir, "missing", "target.docx"),
		},
		{
			name:       "filename keeps matching extension",
			outputPath: dir, fileName: "report.docx", fileExt: ".docx",
			want: filepath.Join(dir, "report.docx"),
		},
		{
			name:       "mismatched extension realigned",
			outputPath: dir, fileName: "report.pdf", fileExt: ".docx",
			want: filepath.Join(dir, "report.docx"),
		},
		{
			name:       "missing extension appended",
			outputPath: dir, fileName: "report", fileExt: ".docx",
			want: filepath.Join(dir, "report.docx"),
		},
		{
			name:        "empty filename falls back to url",
			outputPath:  dir,
			downloadURL: "https://x.test/files/result.xlsx?sig=1",
			fileExt:     ".docx",
			want:        filepath.Join(dir, "result.docx"),
		},
		{
			name:       "unnamed filename falls back to url",
			outputPath: dir, fileName: "unnamed",
			downloadURL: "https://x.test/files/result.xlsx",
			fileExt:     ".docx",
			want:        filepath.Join(dir, "result.docx"),
		},
		{
			name:        "no filename and bare url use job id",
			outputPath:  dir,
			downloadURL: "https://x.test/",
			fileExt:     ".docx",
			jobID:       "job-42",
			want:        filepath.Join(dir, "drive-export-job-42.docx"),
		},
	}
	for _, tc := range cases {
		got := resolveDriveExportOutputPath(tc.outputPath, tc.downloadURL, tc.fileName, tc.fileExt, tc.jobID)
		if got != tc.want {
			t.Errorf("%s: resolveDriveExportOutputPath = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// ── inferExportFormatFromDocInfo：扩展名映射与 docx 兜底 ──

func TestCrossPlatformCoverageInferExportFormatFromDocInfo(t *testing.T) {
	cases := []struct {
		name  string
		steps []scriptedToolStep
		want  string
	}{
		{"call error falls back", []scriptedToolStep{{err: errors.New("boom")}}, "docx"},
		{"parse error falls back", []scriptedToolStep{{text: `{`}}, "docx"},
		{"flat adoc", []scriptedToolStep{{text: `{"extension":"adoc"}`}}, "docx"},
		{"flat axls", []scriptedToolStep{{text: `{"extension":"axls"}`}}, "xlsx"},
		{"flat appt", []scriptedToolStep{{text: `{"extension":"appt"}`}}, "pptx"},
		{"uppercase extension normalized", []scriptedToolStep{{text: `{"extension":" AXLS "}`}}, "xlsx"},
		{"unknown extension falls back", []scriptedToolStep{{text: `{"extension":"xyzw"}`}}, "docx"},
		{"wrapped extension", []scriptedToolStep{{text: `{"result":{"extension":"appt"}}`}}, "pptx"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			installScriptedCaller(t, &scriptedToolCaller{steps: tc.steps})
			if got := inferExportFormatFromDocInfo(context.Background(), "node-1"); got != tc.want {
				t.Fatalf("format = %q, want %q", got, tc.want)
			}
		})
	}
}

// ── pollDriveExportJob：取消 / 各终态 / 查询失败重试 / 轮询上限 ──

func TestCrossPlatformCoveragePollDriveExportJobTerminalStates(t *testing.T) {
	installImmediateTiming(t)
	installScriptedCaller(t, &scriptedToolCaller{steps: []scriptedToolStep{{text: `{"status":"PROCESSING"}`}}})

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := pollDriveExportJob(cancelled, "job-1"); err == nil || !strings.Contains(err.Error(), "导出轮询被取消") {
		t.Fatalf("cancelled error = %v", err)
	}

	steps := []struct {
		name  string
		steps []scriptedToolStep
		check func(t *testing.T, url, name string, err error)
	}{
		{"success", []scriptedToolStep{{text: `{"status":"SUCCESS","resultUrl":"https://x.test/f.docx","resultName":"f.docx"}`}},
			func(t *testing.T, url, name string, err error) {
				if err != nil || url != "https://x.test/f.docx" || name != "f.docx" {
					t.Fatalf("url=%q name=%q err=%v", url, name, err)
				}
			}},
		{"success after query failure", []scriptedToolStep{{err: errors.New("temporary")}, {text: `{"status":"SUCCESS","resultUrl":"https://x.test/f.docx"}`}},
			func(t *testing.T, url, name string, err error) {
				if err != nil || url != "https://x.test/f.docx" {
					t.Fatalf("url=%q err=%v", url, err)
				}
			}},
		{"success without url fails", []scriptedToolStep{{text: `{"status":"SUCCESS"}`}},
			func(t *testing.T, url, name string, err error) {
				if err == nil || !strings.Contains(err.Error(), "导出成功但 downloadUrl 为空") {
					t.Fatalf("error = %v", err)
				}
			}},
		{"failed with message", []scriptedToolStep{{text: `{"status":"FAILED","message":"export denied"}`}},
			func(t *testing.T, url, name string, err error) {
				if err == nil || err.Error() != "export denied" {
					t.Fatalf("error = %v", err)
				}
			}},
		{"failed without message", []scriptedToolStep{{text: `{"status":"FAILED"}`}},
			func(t *testing.T, url, name string, err error) {
				if err == nil || !strings.Contains(err.Error(), "导出任务失败 (taskId=job-1, status=FAILED)") {
					t.Fatalf("error = %v", err)
				}
			}},
		{"partial failed with message", []scriptedToolStep{{text: `{"status":"PARTIAL_FAILED","message":"partial"}`}},
			func(t *testing.T, url, name string, err error) {
				if err == nil || err.Error() != "partial" {
					t.Fatalf("error = %v", err)
				}
			}},
		{"partial failed without message", []scriptedToolStep{{text: `{"status":"PARTIAL_FAILED"}`}},
			func(t *testing.T, url, name string, err error) {
				if err == nil || !strings.Contains(err.Error(), "导出任务失败 (taskId=job-1, status=PARTIAL_FAILED)") {
					t.Fatalf("error = %v", err)
				}
			}},
		{"timeout", []scriptedToolStep{{text: `{"status":"TIMEOUT"}`}},
			func(t *testing.T, url, name string, err error) {
				if err == nil || !strings.Contains(err.Error(), "导出任务超时 (taskId=job-1)") {
					t.Fatalf("error = %v", err)
				}
			}},
		{"processing keeps polling", []scriptedToolStep{{text: `{"status":"PROCESSING"}`}, {text: `{"status":"SUCCESS","resultUrl":"https://x.test/f.docx"}`}},
			func(t *testing.T, url, name string, err error) {
				if err != nil || url != "https://x.test/f.docx" {
					t.Fatalf("url=%q err=%v", url, err)
				}
			}},
		{"poll cap still processing", []scriptedToolStep{{text: `{"status":"PENDING"}`}},
			func(t *testing.T, url, name string, err error) {
				if err == nil || !strings.Contains(err.Error(), "导出任务超时：已轮询 30 次仍在处理中") {
					t.Fatalf("error = %v", err)
				}
			}},
	}
	for _, tc := range steps {
		t.Run(tc.name, func(t *testing.T) {
			installScriptedCaller(t, &scriptedToolCaller{steps: tc.steps})
			url, name, err := pollDriveExportJob(context.Background(), "job-1")
			tc.check(t, url, name, err)
		})
	}
}

// ── runDriveExport 命令级：格式解析 / dry-run / 三步流程 ──

func TestCrossPlatformCoverageRunDriveExportFlow(t *testing.T) {
	installImmediateTiming(t)

	submitOK := scriptedToolStep{text: `{"jobId":"job-9"}`}
	queryOK := scriptedToolStep{text: `{"status":"SUCCESS","resultUrl":"https://x.test/report.docx","resultName":"report.docx"}`}

	t.Run("missing node flag", func(t *testing.T) {
		if err := executeDriveEdge(t, &scriptedToolCaller{}, "export"); err == nil {
			t.Fatal("missing node returned nil")
		}
	})

	t.Run("unsupported format", func(t *testing.T) {
		err := executeDriveEdge(t, &scriptedToolCaller{}, "export", "--node", "n1", "--export-format", "txt")
		if err == nil || !strings.Contains(err.Error(), "不支持的导出格式") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("md alias normalizes to markdown", func(t *testing.T) {
		caller := &scriptedToolCaller{steps: []scriptedToolStep{submitOK, queryOK}}
		if err := executeDriveEdge(t, caller, "export", "--node", "n1", "--export-format", "md"); err != nil {
			t.Fatal(err)
		}
		if caller.argsLog[0]["exportFormat"] != "markdown" {
			t.Fatalf("exportFormat = %v, want markdown", caller.argsLog[0]["exportFormat"])
		}
	})

	t.Run("legacy format flag as export format", func(t *testing.T) {
		caller := &scriptedToolCaller{steps: []scriptedToolStep{submitOK, queryOK}}
		if err := executeDriveEdge(t, caller, "export", "--node", "n1", "--format", "pdf"); err != nil {
			t.Fatal(err)
		}
		if caller.argsLog[0]["exportFormat"] != "pdf" {
			t.Fatalf("exportFormat = %v, want pdf", caller.argsLog[0]["exportFormat"])
		}
	})

	t.Run("output format value ignored for detection", func(t *testing.T) {
		caller := &scriptedToolCaller{steps: []scriptedToolStep{
			{text: `{"extension":"axls"}`}, // get_document_info auto-detect
			submitOK,
			queryOK,
		}}
		if err := executeDriveEdge(t, caller, "export", "--node", "n1", "--format", "json"); err != nil {
			t.Fatal(err)
		}
		if caller.toolLog[0] != "get_document_info" {
			t.Fatalf("first tool = %q, want get_document_info", caller.toolLog[0])
		}
		if caller.argsLog[1]["exportFormat"] != "xlsx" {
			t.Fatalf("exportFormat = %v, want xlsx", caller.argsLog[1]["exportFormat"])
		}
	})

	t.Run("dry run prints preview", func(t *testing.T) {
		for _, extra := range [][]string{{}, {"--output", "out.docx"}, {"--async"}, {"--output", "out.docx", "--async"}} {
			args := append([]string{"export", "--node", "n1", "--export-format", "docx", "--dry-run"}, extra...)
			if err := executeDriveEdge(t, &scriptedToolCaller{dry: true}, args...); err != nil {
				t.Fatalf("dry-run %v: %v", extra, err)
			}
		}
	})

	t.Run("submit error", func(t *testing.T) {
		err := executeDriveEdge(t, &scriptedToolCaller{steps: []scriptedToolStep{{err: errors.New("boom")}}},
			"export", "--node", "n1", "--export-format", "docx")
		if err == nil || !strings.Contains(err.Error(), "提交导出任务失败") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("submit response without job id", func(t *testing.T) {
		err := executeDriveEdge(t, &scriptedToolCaller{steps: []scriptedToolStep{{text: `{}`}}},
			"export", "--node", "n1", "--export-format", "docx")
		if err == nil || !strings.Contains(err.Error(), "jobId") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("async mode returns task id immediately", func(t *testing.T) {
		caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{submitOK}}
		if err := executeDriveEdge(t, caller, "export", "--node", "n1", "--export-format", "docx", "--async"); err != nil {
			t.Fatal(err)
		}
		if len(caller.toolLog) != 1 || caller.calls != 1 {
			t.Fatalf("calls = %v, want exactly the submit call", caller.toolLog)
		}
	})

	t.Run("poll failure surfaces", func(t *testing.T) {
		err := executeDriveEdge(t, &scriptedToolCaller{steps: []scriptedToolStep{submitOK, {text: `{"status":"FAILED","message":"denied"}`}}},
			"export", "--node", "n1", "--export-format", "docx")
		if err == nil || !strings.Contains(err.Error(), "denied") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("no output prints download url", func(t *testing.T) {
		caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{submitOK, queryOK}}
		if err := executeDriveEdge(t, caller, "export", "--node", "n1", "--export-format", "docx"); err != nil {
			t.Fatal(err)
		}
		if len(caller.toolLog) != 2 {
			t.Fatalf("calls = %v", caller.toolLog)
		}
	})

	t.Run("file output downloads", func(t *testing.T) {
		oldGet := httpGetFile
		httpGetFile = func(context.Context, string, map[string]string, string) error { return nil }
		t.Cleanup(func() { httpGetFile = oldGet })
		target := filepath.Join(t.TempDir(), "target.docx")
		if err := executeDriveEdge(t, &scriptedToolCaller{steps: []scriptedToolStep{submitOK, queryOK}},
			"export", "--node", "n1", "--export-format", "docx", "--output", target); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("directory output infers filename", func(t *testing.T) {
		oldGet := httpGetFile
		var savedPath string
		httpGetFile = func(_ context.Context, _ string, _ map[string]string, destination string) error {
			savedPath = destination
			return nil
		}
		t.Cleanup(func() { httpGetFile = oldGet })
		dir := t.TempDir()
		if err := executeDriveEdge(t, &scriptedToolCaller{steps: []scriptedToolStep{submitOK, queryOK}},
			"export", "--node", "n1", "--export-format", "docx", "--output", dir); err != nil {
			t.Fatal(err)
		}
		if filepath.Base(savedPath) != "report.docx" {
			t.Fatalf("savedPath = %q, want report.docx", savedPath)
		}
	})

	t.Run("download failure surfaces", func(t *testing.T) {
		oldGet := httpGetFile
		httpGetFile = func(context.Context, string, map[string]string, string) error { return errors.New("disk full") }
		t.Cleanup(func() { httpGetFile = oldGet })
		target := filepath.Join(t.TempDir(), "target.docx")
		err := executeDriveEdge(t, &scriptedToolCaller{steps: []scriptedToolStep{submitOK, queryOK}},
			"export", "--node", "n1", "--export-format", "docx", "--output", target)
		if err == nil || !strings.Contains(err.Error(), "文件下载失败") {
			t.Fatalf("error = %v", err)
		}
	})
}

// ── runDriveExportGet：参数校验 / dry-run / 查询链路 ──

func TestCrossPlatformCoverageDriveExportGetCommand(t *testing.T) {
	t.Run("missing task id", func(t *testing.T) {
		if err := executeDriveEdge(t, &scriptedToolCaller{}, "export", "get"); err == nil {
			t.Fatal("missing task-id returned nil")
		}
	})

	t.Run("dry run prints preview", func(t *testing.T) {
		err := executeDriveEdge(t, &scriptedToolCaller{dry: true}, "export", "get", "--task-id", "job-1", "--dry-run")
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("query success", func(t *testing.T) {
		caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: `{"status":"SUCCESS","resultUrl":"https://x.test/f.docx"}`}}}
		if err := executeDriveEdge(t, caller, "export", "get", "--task-id", "job-1", "--format", "json"); err != nil {
			t.Fatal(err)
		}
		if caller.server != "drive" || caller.tool != "query_task" {
			t.Fatalf("routed to %s/%s", caller.server, caller.tool)
		}
	})

	t.Run("query error surfaces", func(t *testing.T) {
		err := executeDriveEdge(t, &scriptedToolCaller{steps: []scriptedToolStep{{err: errors.New("boom")}}},
			"export", "get", "--task-id", "job-1")
		if err == nil || !strings.Contains(err.Error(), "查询任务失败") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("hidden job id alias works", func(t *testing.T) {
		caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: `{"status":"PROCESSING"}`}}}
		if err := executeDriveEdge(t, caller, "export", "get", "--job-id", "job-2"); err != nil {
			t.Fatal(err)
		}
		if caller.args["taskId"] != "job-2" {
			t.Fatalf("taskId arg = %v", caller.args["taskId"])
		}
	})
}
