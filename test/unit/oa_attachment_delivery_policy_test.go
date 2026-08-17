// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package unit_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCrossPlatformCoverageOAAttachmentDeliveryPolicy(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))

	attachmentFile := filepath.Join(root, "internal", "helpers", "oa_attachment.go")
	if _, err := os.Stat(attachmentFile); !os.IsNotExist(err) {
		t.Fatalf("OA attachment commands must live in internal/helpers/oa.go; separate file still exists: %s", attachmentFile)
	}

	oaSourcePath := filepath.Join(root, "internal", "helpers", "oa.go")
	oaSource, err := os.ReadFile(oaSourcePath)
	if err != nil {
		t.Fatalf("read %s: %v", oaSourcePath, err)
	}
	for _, required := range []string{
		"func newOAAttachmentCommand()",
		`Tool:    "get_attachment_download_url"`,
		`Tool:    "auth_download_file"`,
		`Tool:    "auth_preview_attachment"`,
	} {
		if !strings.Contains(string(oaSource), required) {
			t.Errorf("%s missing attachment declaration %q", oaSourcePath, required)
		}
	}

	docPaths := []string{
		filepath.Join(root, "skills", "mono", "references", "products", "oa.md"),
		filepath.Join(root, "skills", "multi", "dingtalk-misc", "references", "oa.md"),
	}
	for _, docPath := range docPaths {
		doc, err := os.ReadFile(docPath)
		if err != nil {
			t.Fatalf("read %s: %v", docPath, err)
		}
		for _, required := range []string{
			"dws oa approval attachment download-url",
			"dws oa approval attachment authorize-download",
			"dws oa approval attachment authorize-preview",
			"临时下载链接",
			"最多 10",
			"最多 20",
		} {
			if !strings.Contains(string(doc), required) {
				t.Errorf("%s missing OA attachment Skill guidance %q", docPath, required)
			}
		}
	}
}
