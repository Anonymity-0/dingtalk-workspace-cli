// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
)

func executeOAAttachmentCommandCapturingOutput(t *testing.T, caller *scriptedToolCaller, args ...string) (string, error) {
	t.Helper()
	testseam.Protect(t, &deps)
	testseam.Swap(t, &os.Args, []string{"dws", "oa"})
	InitDeps(caller)
	var stdout bytes.Buffer
	deps.Out.w = &stdout
	deps.Out.errW = io.Discard

	cmd := newOaCommand()
	cmd.PersistentFlags().Bool("yes", false, "跳过确认")
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(&stdout)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), err
}

func TestCrossPlatformCoverageOAAttachmentDownloadURLPreservesSignedQuery(t *testing.T) {
	const response = `{"result":{"downloadUri":"https://example.test/file?Expires=1&OSSAccessKeyId=2&Signature=3"},"success":true}`
	commandArgs := []string{
		"approval", "attachment", "download-url",
		"--instance-id", "instance-1",
		"--file-id", "file-1",
	}
	wantArgs := map[string]any{
		"processInstanceId": "instance-1",
		"fileId":            "file-1",
	}

	t.Run("json", func(t *testing.T) {
		caller := &scriptedToolCaller{format: "json", steps: []scriptedToolStep{{text: response}}}
		stdout, err := executeOAAttachmentCommandCapturingOutput(t, caller, commandArgs...)
		if err != nil {
			t.Fatalf("execute command: %v", err)
		}
		if caller.server != "oa" || caller.tool != "get_attachment_download_url" {
			t.Fatalf("called %s/%s, want oa/get_attachment_download_url", caller.server, caller.tool)
		}
		if !reflect.DeepEqual(caller.args, wantArgs) {
			t.Fatalf("args = %#v, want %#v", caller.args, wantArgs)
		}
		if !strings.Contains(stdout, "&OSSAccessKeyId=") || !strings.Contains(stdout, "&Signature=") {
			t.Fatalf("stdout does not preserve signed query separators: %q", stdout)
		}
		if strings.Contains(stdout, `\u0026`) {
			t.Fatalf("stdout contains escaped query separator: %q", stdout)
		}
	})

	t.Run("raw compatibility", func(t *testing.T) {
		caller := &scriptedToolCaller{format: "raw", steps: []scriptedToolStep{{text: response}}}
		stdout, err := executeOAAttachmentCommandCapturingOutput(t, caller, commandArgs...)
		if err != nil {
			t.Fatalf("execute command: %v", err)
		}
		if stdout != response+"\n" {
			t.Fatalf("stdout = %q, want %q", stdout, response+"\n")
		}
	})
}

func TestCrossPlatformCoverageOAAttachmentPayloads(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantTool string
		check    func(*testing.T, map[string]any)
	}{
		{
			name: "download URL",
			args: []string{
				"approval", "attachment", "download-url",
				"--instance-id", "instance-1",
				"--file-id", "file-1",
				"--with-comment-attachment=false",
			},
			wantTool: "get_attachment_download_url",
			check: func(t *testing.T, got map[string]any) {
				t.Helper()
				if got["processInstanceId"] != "instance-1" || got["fileId"] != "file-1" {
					t.Fatalf("download identifiers = %#v", got)
				}
				if value, ok := got["withCommentAttachment"].(bool); !ok || value {
					t.Fatalf("withCommentAttachment = %#v, want explicit false", got["withCommentAttachment"])
				}
			},
		},
		{
			name: "download authorization",
			args: []string{
				"approval", "attachment", "authorize-download",
				"--file-infos", `[{"spaceId":27827223951,"fileId":"file-2"}]`,
			},
			wantTool: "auth_download_file",
			check: func(t *testing.T, got map[string]any) {
				t.Helper()
				infos, ok := got["fileInfos"].([]map[string]any)
				if !ok || len(infos) != 1 {
					t.Fatalf("fileInfos = %#v", got["fileInfos"])
				}
				if infos[0]["spaceId"] != json.Number("27827223951") || infos[0]["fileId"] != "file-2" {
					t.Fatalf("fileInfos[0] = %#v", infos[0])
				}
			},
		},
		{
			name: "preview authorization",
			args: []string{
				"approval", "attachment", "authorize-preview",
				"--instance-id", "instance-2",
				"--file-ids", "file-3,file-4",
				"--with-comment-attachment",
			},
			wantTool: "auth_preview_attachment",
			check: func(t *testing.T, got map[string]any) {
				t.Helper()
				if got["processInstanceId"] != "instance-2" || got["withCommentAttachment"] != true {
					t.Fatalf("preview scalar payload = %#v", got)
				}
				ids, ok := got["fileIdList"].([]string)
				if !ok || len(ids) != 2 || ids[0] != "file-3" || ids[1] != "file-4" {
					t.Fatalf("fileIdList = %#v", got["fileIdList"])
				}
			},
		},
		{
			name: "optional boolean omitted",
			args: []string{
				"approval", "attachment", "download-url",
				"--instance-id", "instance-3",
				"--file-id", "file-5",
			},
			wantTool: "get_attachment_download_url",
			check: func(t *testing.T, got map[string]any) {
				t.Helper()
				if _, exists := got["withCommentAttachment"]; exists {
					t.Fatalf("optional withCommentAttachment should be omitted: %#v", got)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			if err := executeOACommand(t, caller, test.args...); err != nil {
				t.Fatalf("execute command: %v", err)
			}
			if caller.server != "oa" || caller.tool != test.wantTool {
				t.Fatalf("called %s/%s, want oa/%s", caller.server, caller.tool, test.wantTool)
			}
			test.check(t, caller.args)
		})
	}
}

func TestCrossPlatformCoverageOAAttachmentRejectsInvalidFileInfos(t *testing.T) {
	valid := `{"spaceId":27827223951,"fileId":"file-1"}`
	tests := []struct {
		name string
		raw  string
	}{
		{name: "malformed JSON", raw: `[{`},
		{name: "null", raw: `null`},
		{name: "not array", raw: `{}`},
		{name: "empty array", raw: `[]`},
		{name: "more than ten", raw: `[` + strings.Repeat(valid+`,`, 10) + valid + `]`},
		{name: "missing space ID", raw: `[{"fileId":"file-1"}]`},
		{name: "string space ID", raw: `[{"spaceId":"27827223951","fileId":"file-1"}]`},
		{name: "missing file ID", raw: `[{"spaceId":27827223951}]`},
		{name: "numeric file ID", raw: `[{"spaceId":27827223951,"fileId":232271651278}]`},
		{name: "blank file ID", raw: `[{"spaceId":27827223951,"fileId":"  "}]`},
		{name: "unknown property", raw: `[{"spaceId":27827223951,"fileId":"file-1","extra":true}]`},
		{name: "trailing JSON", raw: `[` + valid + `] {}`},
		{name: "malformed trailing JSON", raw: `[` + valid + `] {`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			err := executeOACommand(t, caller,
				"approval", "attachment", "authorize-download",
				"--file-infos", test.raw,
			)
			if err == nil {
				t.Fatalf("fileInfos %q unexpectedly succeeded", test.raw)
			}
			if caller.calls != 0 {
				t.Fatalf("invalid fileInfos made %d MCP call(s)", caller.calls)
			}
		})
	}
}

func TestCrossPlatformCoverageOAAttachmentRejectsInvalidPreviewFileIDs(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "separator only", raw: `,`},
		{name: "blank item", raw: `file-1, ,file-2`},
		{name: "more than twenty", raw: strings.Repeat("file,", 20) + "file"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			err := executeOACommand(t, caller,
				"approval", "attachment", "authorize-preview",
				"--instance-id", "instance-1",
				"--file-ids", test.raw,
			)
			if err == nil {
				t.Fatalf("file IDs %q unexpectedly succeeded", test.raw)
			}
			if caller.calls != 0 {
				t.Fatalf("invalid file IDs made %d MCP call(s)", caller.calls)
			}
		})
	}
}
