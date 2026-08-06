// Copyright 2026 Alibaba Group
// SPDX-License-Identifier: Apache-2.0

package aitable

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func writeAttachmentFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "report.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func attachmentRecordJSON(t *testing.T, fieldID string, attachments []any) string {
	t.Helper()
	return mustJSONText(t, map[string]any{"records": []any{map[string]any{
		"recordId": "record", "cells": map[string]any{fieldID: attachments},
	}}})
}

func TestCrossPlatformCoverageAttachmentPutPerformsHTTPAndReadBackE2E(t *testing.T) {
	filePath := writeAttachmentFixture(t, "actual attachment bytes")
	fileName := filepath.Base(filePath)
	uploaded := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		if request.Method != http.MethodPut || request.Header.Get("Content-Type") != "text/plain" || string(body) != "actual attachment bytes" {
			t.Errorf("upload request = method:%s type:%s body:%q", request.Method, request.Header.Get("Content-Type"), body)
			http.Error(w, "bad upload", http.StatusBadRequest)
			return
		}
		uploaded = true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	caller := &upsertByKeyCaller{steps: []upsertByKeyStep{
		{text: attachmentRecordJSON(t, "field", []any{})},
		{text: mustJSONText(t, map[string]any{"uploadUrl": server.URL + "/put", "fileToken": "ft1"})},
		{text: ""},
		{text: attachmentRecordJSON(t, "field", []any{map[string]any{"filename": fileName, "size": len("actual attachment bytes")}})},
	}}
	out, err := runAITableCompositeCLI(t, caller, "+attachment-put",
		"--base-id", "base", "--table-id", "table", "--record-id", "record", "--field-id", "field", "--file", filePath, "--mode", "replace", "--mime-type", "text/plain", "--yes")
	if err != nil || !uploaded {
		t.Fatalf("attachment put = output:%q err:%v uploaded:%v", out, err, uploaded)
	}
	for _, want := range []string{`"status": "recovered"`, `"fileToken": "ft1"`, `"attachmentCount": 1`, `"status": "verified"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("attachment output missing %s: %s", want, out)
		}
	}
	if len(caller.calls) != 4 || caller.calls[1].tool != "prepare_attachment_upload" || caller.calls[2].tool != "update_records" {
		t.Fatalf("attachment calls = %#v", caller.calls)
	}
}

func TestCrossPlatformCoverageAttachmentPutFailuresDoNotBecomeSuccessE2E(t *testing.T) {
	filePath := writeAttachmentFixture(t, "bytes")
	t.Run("append stops before upload when existing tokens unavailable", func(t *testing.T) {
		caller := &upsertByKeyCaller{steps: []upsertByKeyStep{
			{text: attachmentRecordJSON(t, "field", []any{map[string]any{"filename": "old.pdf", "url": "https://download"}})},
		}}
		out, err := runAITableCompositeCLI(t, caller, "+attachment-put",
			"--base-id", "base", "--table-id", "table", "--record-id", "record", "--field-id", "field", "--file", filePath, "--yes")
		if err == nil || out != "" || len(caller.calls) != 1 || !strings.Contains(err.Error(), "fileToken") {
			t.Fatalf("unsafe append = output:%q err:%v calls:%#v", out, err, caller.calls)
		}
	})

	t.Run("HTTP failure leaves no record write", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", http.StatusBadGateway) }))
		defer server.Close()
		caller := &upsertByKeyCaller{steps: []upsertByKeyStep{
			{text: attachmentRecordJSON(t, "field", []any{})},
			{text: mustJSONText(t, map[string]any{"uploadUrl": server.URL, "fileToken": "ft1"})},
		}}
		out, err := runAITableCompositeCLI(t, caller, "+attachment-put",
			"--base-id", "base", "--table-id", "table", "--record-id", "record", "--field-id", "field", "--file", filePath, "--mode", "replace", "--yes")
		if err == nil || out != "" || len(caller.calls) != 2 || !strings.Contains(err.Error(), "partial_success") {
			t.Fatalf("HTTP failure = output:%q err:%v calls:%#v", out, err, caller.calls)
		}
		var typed *apperrors.Error
		if !errors.As(err, &typed) || typed.Retryable {
			t.Fatalf("partially uploaded attachment must not be blindly retryable: %#v", err)
		}
	})

	t.Run("prepare response must carry both values", func(t *testing.T) {
		caller := &upsertByKeyCaller{steps: []upsertByKeyStep{
			{text: attachmentRecordJSON(t, "field", []any{})}, {text: `{"uploadUrl":"https://example.invalid"}`},
		}}
		out, err := runAITableCompositeCLI(t, caller, "+attachment-put",
			"--base-id", "base", "--table-id", "table", "--record-id", "record", "--field-id", "field", "--file", filePath, "--mode", "replace", "--yes")
		if err == nil || out != "" || len(caller.calls) != 2 {
			t.Fatalf("bad prepare = output:%q err:%v calls:%#v", out, err, caller.calls)
		}
	})
}

func TestCrossPlatformCoverageAttachmentRemoveClearAndSelectiveE2E(t *testing.T) {
	t.Run("clear empty write response recovered by empty read-back", func(t *testing.T) {
		caller := &upsertByKeyCaller{steps: []upsertByKeyStep{
			{text: attachmentRecordJSON(t, "field", []any{map[string]any{"filename": "a.pdf", "url": "https://download"}})},
			{text: ""},
			{text: attachmentRecordJSON(t, "field", []any{})},
		}}
		out, err := runAITableCompositeCLI(t, caller, "+attachment-remove",
			"--base-id", "base", "--table-id", "table", "--record-id", "record", "--field-id", "field", "--clear-all", "--yes")
		if err != nil || !strings.Contains(out, `"status": "recovered"`) || !strings.Contains(out, `"removedCount": 1`) {
			t.Fatalf("clear attachments = output:%q err:%v", out, err)
		}
	})

	t.Run("selective removal preserves tokenized remainder", func(t *testing.T) {
		caller := &upsertByKeyCaller{steps: []upsertByKeyStep{
			{text: attachmentRecordJSON(t, "field", []any{
				map[string]any{"fileToken": "remove-token", "filename": "remove.pdf"},
				map[string]any{"fileToken": "keep-token", "filename": "keep.pdf"},
			})},
			{text: `{"updatedCount":1}`},
			{text: attachmentRecordJSON(t, "field", []any{map[string]any{"filename": "keep.pdf", "size": 10}})},
		}}
		out, err := runAITableCompositeCLI(t, caller, "+attachment-remove",
			"--base-id", "base", "--table-id", "table", "--record-id", "record", "--field-id", "field", "--remove-name", "remove.pdf", "--yes")
		if err != nil || !strings.Contains(out, `"removedCount": 1`) {
			t.Fatalf("selective remove = output:%q err:%v", out, err)
		}
		written := caller.calls[1].args["records"].([]any)[0].(map[string]any)["cells"].(map[string]any)["field"].([]any)
		if len(written) != 1 || written[0].(map[string]any)["fileToken"] != "keep-token" {
			t.Fatalf("selective desired attachments = %#v", written)
		}
	})
}

func TestCrossPlatformCoverageAttachmentRemoveExplainsTokenBoundaryE2E(t *testing.T) {
	caller := &upsertByKeyCaller{steps: []upsertByKeyStep{{text: attachmentRecordJSON(t, "field", []any{
		map[string]any{"filename": "remove.pdf", "url": "https://one"},
		map[string]any{"filename": "keep.pdf", "url": "https://two"},
	})}}}
	out, err := runAITableCompositeCLI(t, caller, "+attachment-remove",
		"--base-id", "base", "--table-id", "table", "--record-id", "record", "--field-id", "field", "--remove-name", "remove.pdf", "--yes")
	if err == nil || out != "" || len(caller.calls) != 1 || !strings.Contains(err.Error(), "fileToken") || !strings.Contains(err.Error(), "增量删除") {
		t.Fatalf("token boundary = output:%q err:%v calls:%#v", out, err, caller.calls)
	}
}
