// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package minutes

import (
	"encoding/json"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCrossPlatformCoverageMinutesExportSummarySignedLinks(t *testing.T) {
	for _, tc := range []struct{ input, want string }{
		{"plain text", "plain text"},
		{"[link](https://example.test/a)", "[link](https://example.test/a)"},
		{"[link](https://example.test/a?page=2#part)", "[link](https://example.test/a?page=2#part)"},
		{"![图](https://example.test/a.png?OSSAccessKeyId=secret&Expires=1&Signature=secret)", "![图]([signed-url-removed])"},
		{"<img src=\"https://example.test/a?foo=1&amp;Signature=secret\">", "<img src=\"[signed-url-removed]\">"},
		{"https://example.test/a?%53ignature=secret", exportRemovedURL},
		{"https://example.test/a?X-Amz-Credential=secret", exportRemovedURL},
		{"https://example.test/a?X-Oss-Signature=secret", exportRemovedURL},
		{"https://example.test/a?bad%zz=value", exportRemovedURL},
	} {
		if got := (&exportRedactions{}).text(tc.input); got != tc.want {
			t.Errorf("sanitizeExportSummary(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestCrossPlatformCoverageMinutesExportSummarySanitizedOnDisk(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	work := t.TempDir()
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
	caller := &minutesE2ECaller{responses: map[string][]string{
		"minutes/get_minutes_ai_summary": {`{"success":true,"result":{"fullSummary":"纪要\n![图](https://example.test/a.png?OSSAccessKeyId=secret&Signature=secret)\n[文档](https://example.test/doc?id=1)"}}`},
	}}
	payload, _, err := runMinutesAlignmentCLI(t, caller, "minutes", "+export-pack", "--id", "u1", "--output", "pack", "--artifacts", "summary")
	if err != nil || payload["published"] != true {
		t.Fatalf("payload=%v err=%v", payload, err)
	}
	data, err := os.ReadFile(filepath.Join(work, "pack", "summary.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "纪要\n![图]([signed-url-removed])\n[文档](https://example.test/doc?id=1)" {
		t.Fatalf("unexpected archive content: %s", data)
	}
}

func TestCrossPlatformCoverageMinutesExportNestedCredentials(t *testing.T) {
	original := map[string]any{"id": int64(9007199254740993), "nested": []map[string]any{{"text": `{"url":"https:\/\/example.test\/a?Signature=CANARY"}`, "Signature": "CANARY"}}, "plain": "https://example.test/doc?id=7"}
	clean, count, err := sanitizeExportArtifact(original)
	if err != nil || count != 2 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	raw, _ := json.Marshal(clean)
	if strings.Contains(string(raw), "CANARY") || !strings.Contains(string(raw), "9007199254740993") || containsExportCredentials(clean) {
		t.Fatalf("unsafe/changed result: %s", raw)
	}
	if original["nested"].([]map[string]any)[0]["Signature"] != "CANARY" {
		t.Fatal("mutated original payload")
	}
	if _, _, err := sanitizeExportArtifact(make(chan int)); err == nil {
		t.Fatal("unencodable value accepted")
	}
}

func TestCrossPlatformCoverageMinutesExportScanRejectsResidual(t *testing.T) {
	for _, body := range []string{`{"nested":["https://example.test/a?Signature=CANARY"]}`, `{"Signature":"CANARY"}`, `{"https://example.test/?Signature=CANARY":0}`, `{broken`} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "basic.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := scanExportCredentials(dir, []string{"basic"}, false); err == nil || strings.Contains(err.Error(), "CANARY") {
			t.Fatalf("scan error=%v", err)
		}
	}
}

func TestCrossPlatformCoverageMinutesExportAllTextAndNoPublishOnLeak(t *testing.T) {
	for _, inject := range []bool{false, true} {
		t.Run(map[bool]string{false: "clean", true: "injected"}[inject], func(t *testing.T) {
			old, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			work := t.TempDir()
			if err := os.Chdir(work); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chdir(old) })
			if inject {
				write := minutesWriteFile
				testseam.Swap(t, &minutesWriteFile, func(path string, raw []byte, mode os.FileMode) error {
					if filepath.Base(path) == "basic.json" {
						raw = []byte(`{"url":"https://example.test/?Signature=CANARY"}`)
					}
					return write(path, raw, mode)
				})
			}
			caller := &minutesE2ECaller{responses: map[string][]string{
				"minutes/get_minutes_basic_info": {`{"success":true,"result":{"taskUuid":"u1","title":"https://example.test/?Signature=CANARY"}}`},
				"minutes/list_minutes_todos":     {`{"success":true,"result":{"actions":["https://example.test/?OSSAccessKeyId=CANARY&Signature=CANARY"]}}`},
			}}
			payload, _, err := runMinutesAlignmentCLI(t, caller, "minutes", "+export-pack", "--id", "u1", "--output", "pack", "--artifacts", "basic,todos")
			if inject {
				if err == nil || payload["published"] != false {
					t.Fatalf("leak published: %v %v", payload, err)
				}
				if _, err := os.Stat("pack"); !os.IsNotExist(err) {
					t.Fatal("target exists on failure")
				}
				entries, _ := os.ReadDir(work)
				if len(entries) != 0 {
					t.Fatal("temporary export not cleaned")
				}
				return
			}
			if err != nil || payload["sanitized"] != true || payload["redactionCount"].(float64) < 2 {
				t.Fatalf("payload=%v err=%v", payload, err)
			}
			entries, _ := os.ReadDir("pack")
			for _, entry := range entries {
				raw, err := os.ReadFile(filepath.Join("pack", entry.Name()))
				if err != nil || strings.Contains(string(raw), "CANARY") {
					t.Fatal("secret in archive")
				}
			}
		})
	}
}
