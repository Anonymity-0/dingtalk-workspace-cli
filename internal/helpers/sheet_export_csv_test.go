// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package helpers

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSheetExportRejectsUnknownExportFormat(t *testing.T) {
	caller := &scriptedToolCaller{}
	err := executeSheetExportCoverage(t, caller, "node", "NODE", "export-format", "pdf")
	if err == nil || !strings.Contains(err.Error(), "--export-format 仅支持 xlsx 或 csv") {
		t.Fatalf("err = %v, want unsupported export format", err)
	}
	if caller.calls != 0 {
		t.Fatalf("calls = %d, want 0", caller.calls)
	}
}

func TestSheetExportCsvRequiresNode(t *testing.T) {
	caller := &scriptedToolCaller{}
	err := executeSheetExportCoverage(t, caller, "export-format", "csv")
	if err == nil || !strings.Contains(err.Error(), "--node is required") {
		t.Fatalf("err = %v, want required node", err)
	}
	if caller.calls != 0 {
		t.Fatalf("calls = %d, want 0", caller.calls)
	}
}

// --value-render-option 的枚举必须在发请求之前校验，否则非法取值会被静默透传。
func TestSheetExportCsvValidatesValueRenderOption(t *testing.T) {
	caller := &scriptedToolCaller{}
	err := executeSheetExportCoverage(t, caller,
		"node", "NODE", "export-format", "csv", "value-render-option", "pretty")
	if err == nil ||
		!strings.Contains(err.Error(), "--value-render-option 必须为 formatted_value / raw_value / formula") {
		t.Fatalf("err = %v, want enum rejection", err)
	}
	if caller.calls != 0 {
		t.Fatalf("calls = %d, want 0（校验必须在发请求之前）", caller.calls)
	}

	for _, option := range []string{"formatted_value", "raw_value", "formula", "FORMULA", " raw_value "} {
		ok := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{"csv":"a,b\n"}`}}}
		if err := executeSheetExportCoverage(t, ok,
			"node", "NODE", "export-format", "csv", "value-render-option", option); err != nil {
			t.Fatalf("value-render-option=%q 应被接受: %v", option, err)
		}
	}
}

func TestSheetExportCsvDryRunPreviewsSelectors(t *testing.T) {
	caller := &scriptedToolCaller{dry: true}
	if err := executeSheetExportCoverage(t, caller,
		"node", "NODE", "export-format", "csv",
		"sheet-id", "SHEET_1", "output", "out.csv"); err != nil {
		t.Fatalf("csv dry run: %v", err)
	}
	if caller.calls != 0 {
		t.Fatalf("dry run 不应发请求, calls = %d", caller.calls)
	}
}

func TestSheetExportCsvForwardsRangeAndWarnsOnTruncation(t *testing.T) {
	caller := &scriptedToolCaller{steps: []scriptedToolStep{
		{text: `{"csv":"a,b\n","hasMore":true}`},
	}}
	if err := executeSheetExportCoverage(t, caller,
		"node", "NODE", "export-format", "csv", "range", "A1:Z1000"); err != nil {
		t.Fatalf("csv export: %v", err)
	}
	if got := caller.args["range"]; got != "A1:Z1000" {
		t.Fatalf("range 未透传: %#v", caller.args["range"])
	}
	// csv 正文不带行号前缀，annotateRowNumbers 必须显式关掉。
	if caller.args["annotateRowNumbers"] != false {
		t.Fatalf("annotateRowNumbers = %#v, want false", caller.args["annotateRowNumbers"])
	}
}

func TestSheetExportCsvWritesIntoDirectory(t *testing.T) {
	dir := t.TempDir()
	caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{"csv":"a,b\n"}`}}}
	if err := executeSheetExportCoverage(t, caller,
		"node", "NODE", "export-format", "csv", "output", dir); err != nil {
		t.Fatalf("csv export: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "sheet-export.csv"))
	if err != nil {
		t.Fatalf("read exported csv: %v", err)
	}
	if string(body) != "a,b\n" {
		t.Fatalf("csv 内容 = %q", string(body))
	}
}

func TestSheetExportCsvReportsWriteFailure(t *testing.T) {
	caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{"csv":"a,b\n"}`}}}
	missing := filepath.Join(t.TempDir(), "no-such-dir", "out.csv")
	err := executeSheetExportCoverage(t, caller,
		"node", "NODE", "export-format", "csv", "output", missing)
	if err == nil || !strings.Contains(err.Error(), "写入 CSV 文件失败") {
		t.Fatalf("err = %v, want write failure", err)
	}
}

func TestSheetExportCsvReportsReadFailure(t *testing.T) {
	caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{"errorCode":"NoPermission"}`}}}
	err := executeSheetExportCoverage(t, caller, "node", "NODE", "export-format", "csv")
	if err == nil || !strings.Contains(err.Error(), "读取 CSV 失败") {
		t.Fatalf("err = %v, want read failure", err)
	}
}

func TestParseGetRangeAsCsvResult(t *testing.T) {
	// 兼容裸响应与 result 包装两种形状
	for _, body := range []string{
		`{"csv":"a,b\n","hasMore":true}`,
		`{"result":{"csv":"a,b\n","hasMore":true}}`,
	} {
		csv, hasMore, err := parseGetRangeAsCsvResult(body)
		if err != nil || csv != "a,b\n" || !hasMore {
			t.Fatalf("parseGetRangeAsCsvResult(%s) = (%q,%v,%v)", body, csv, hasMore, err)
		}
	}
	// 字段存在但为空串是合法的（真的空区域）
	csv, hasMore, err := parseGetRangeAsCsvResult(`{"csv":""}`)
	if err != nil || csv != "" || hasMore {
		t.Fatalf(`csv:"" = (%q,%v,%v), want ("",false,nil)`, csv, hasMore, err)
	}
}

// csv 字段缺失或类型不对必须报错。此前会被当成空表，配合 --output 会用
// 0 字节覆盖已有文件并打印"导出完成"，属于静默数据丢失。
func TestParseGetRangeAsCsvResultRejectsMissingOrBadCsvField(t *testing.T) {
	for _, tc := range []struct{ body, want string }{
		{"not json", "解析 get_range_as_csv 响应失败"},
		{`{"message":"something odd"}`, "缺少 csv 字段"},
		{`{"csv":123}`, "csv 字段不是字符串"},
		{`{"csv":null}`, "csv 字段不是字符串"},
		{`{"result":"not-an-object"}`, "result 不是对象"},
		{`{"result":{"message":"odd"}}`, "缺少 csv 字段"},
	} {
		if _, _, err := parseGetRangeAsCsvResult(tc.body); err == nil ||
			!strings.Contains(err.Error(), tc.want) {
			t.Errorf("parseGetRangeAsCsvResult(%s) err = %v, want contains %q", tc.body, err, tc.want)
		}
	}
}

// 端到端：响应缺 csv 字段时，绝不能覆盖 --output 指向的已有文件。
func TestSheetExportCsvNeverTruncatesOutputOnBadResponse(t *testing.T) {
	out := filepath.Join(t.TempDir(), "existing.csv")
	const original = "原有重要数据\n1,2,3\n"
	if err := os.WriteFile(out, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{"message":"something odd"}`}}}
	err := executeSheetExportCoverage(t, caller, "node", "NODE", "export-format", "csv", "output", out)
	if err == nil || !strings.Contains(err.Error(), "缺少 csv 字段") {
		t.Fatalf("err = %v, want missing csv field", err)
	}
	body, readErr := os.ReadFile(out)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(body) != original {
		t.Fatalf("已有文件被改写为 %q，原内容应保持不变", string(body))
	}
}

// CSV 正文走 stdout，截断警告必须只走 stderr：否则管道/重定向拿到的文件里会
// 混入 [WARN] 文本，不再是合法 RFC4180 CSV。而大表恰恰最容易触发这个分支。
func TestSheetExportCsvKeepsStdoutPureAndWarnsOnStderr(t *testing.T) {
	previousDeps := deps
	t.Cleanup(func() { deps = previousDeps })

	caller := &scriptedToolCaller{steps: []scriptedToolStep{{text: `{"csv":"a,b\nc,d\n","hasMore":true}`}}}
	installScriptedCaller(t, caller)
	installSheetProductArgs(t)
	var stdout, stderr bytes.Buffer
	deps.Out.w, deps.Out.errW = &stdout, &stderr

	cmd := newExportCmd()
	for _, kv := range [][2]string{{"node", "NODE"}, {"export-format", "csv"}} {
		if err := cmd.Flags().Set(kv[0], kv[1]); err != nil {
			t.Fatalf("set --%s: %v", kv[0], err)
		}
	}
	if err := runSheetExport(cmd, nil); err != nil {
		t.Fatalf("csv export: %v", err)
	}

	if got := stdout.String(); got != "a,b\nc,d\n" {
		t.Fatalf("stdout = %q, want 纯 CSV 正文", got)
	}
	if strings.Contains(stdout.String(), "WARN") || strings.Contains(stdout.String(), "截断") {
		t.Fatalf("stdout 混入了警告文本: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "CSV 已被截断") {
		t.Fatalf("stderr = %q, want 截断提示", stderr.String())
	}
}
