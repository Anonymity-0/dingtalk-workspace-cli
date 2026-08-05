// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/testseam"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type chatBotRichMediaCall struct {
	server string
	tool   string
	args   map[string]any
}

type chatBotRichMediaCaller struct {
	calls  []chatBotRichMediaCall
	dryRun bool
}

func (c *chatBotRichMediaCaller) CallTool(_ context.Context, server, tool string, args map[string]any) (*edition.ToolResult, error) {
	copied := make(map[string]any, len(args))
	for key, value := range args {
		copied[key] = value
	}
	c.calls = append(c.calls, chatBotRichMediaCall{server: server, tool: tool, args: copied})

	switch tool {
	case "init_conversation_file_upload":
		return textToolResult(`{"resourceUrl":"https://upload.example/file","uploadKey":"upload-key"}`), nil
	case "commit_conversation_file_upload":
		return textToolResult(`{"result":{"downloadUrl":"https://download.example/report.pdf"}}`), nil
	case "send_robot_group_message", "batch_send_robot_msg_to_users":
		return textToolResult(`{"success":true}`), nil
	default:
		return nil, fmt.Errorf("unexpected tool call %s/%s", server, tool)
	}
}

func (*chatBotRichMediaCaller) Format() string { return "json" }
func (c *chatBotRichMediaCaller) DryRun() bool { return c.dryRun }
func (*chatBotRichMediaCaller) Fields() string { return "" }
func (*chatBotRichMediaCaller) JQ() string     { return "" }

func TestChatMessageSendByBotMapsMessageTypeByTarget(t *testing.T) {
	testseam.Protect(t, &deps)

	tests := []struct {
		name          string
		args          []string
		tool          string
		typeField     string
		typeValue     string
		unwantedField string
	}{
		{
			name:          "group markdown without title",
			args:          []string{"--group", "group-open-cid", "--text", "正文"},
			tool:          "send_robot_group_message",
			typeField:     "msgKey",
			typeValue:     "sampleMarkdownDX",
			unwantedField: "msgType",
		},
		{
			name:          "direct markdown",
			args:          []string{"--users", "318617", "--title", "标题", "--text", "正文"},
			tool:          "batch_send_robot_msg_to_users",
			typeField:     "msgType",
			typeValue:     "sampleMarkdown",
			unwantedField: "msgKey",
		},
		{
			name:          "group image",
			args:          []string{"--group", "group-open-cid", "--msg-type", "image", "--image-url", "https://example.com/image.png"},
			tool:          "send_robot_group_message",
			typeField:     "msgKey",
			typeValue:     "sampleImageMsg",
			unwantedField: "msgType",
		},
		{
			name:          "direct image",
			args:          []string{"--users", "318617", "--open-dingtalk-ids", "D-open-receiver", "--msg-type", "image", "--image-url", "https://example.com/image.png"},
			tool:          "batch_send_robot_msg_to_users",
			typeField:     "msgType",
			typeValue:     "sampleImageMsg",
			unwantedField: "msgKey",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &chatBotRichMediaCaller{}
			args := []string{"message", "send-by-bot", "--robot-code", "robot-code"}
			args = append(args, tt.args...)
			if err := runChatCoverageCommand(t, caller, args...); err != nil {
				t.Fatalf("send-by-bot: %v", err)
			}
			if len(caller.calls) != 1 {
				t.Fatalf("calls = %#v, want one bot call", caller.calls)
			}
			call := caller.calls[0]
			if call.server != "bot" || call.tool != tt.tool {
				t.Fatalf("call = %s/%s, want bot/%s", call.server, call.tool, tt.tool)
			}
			if got := call.args[tt.typeField]; got != tt.typeValue {
				t.Fatalf("%s = %v, want %s", tt.typeField, got, tt.typeValue)
			}
			if _, ok := call.args[tt.unwantedField]; ok {
				t.Fatalf("%s should not be set", tt.unwantedField)
			}
		})
	}
}

func TestChatMessageSendByBotRequiresMsgTypeForRichMedia(t *testing.T) {
	testseam.Protect(t, &deps)

	tests := []struct {
		name      string
		content   []string
		wantError string
	}{
		{
			name:      "markdown content",
			wantError: "missing required flag(s): --text",
		},
		{
			name:      "image",
			content:   []string{"--image-url", "https://example.com/image.png"},
			wantError: "--msg-type image is required when using --image-url",
		},
		{
			name:      "file",
			content:   []string{"--file-path", "/tmp/not-read-without-msg-type.pdf"},
			wantError: "--msg-type file is required when using --file-path",
		},
		{
			name:      "image content",
			content:   []string{"--msg-type", "image", "--text", "ignored"},
			wantError: "--image-url is required for --msg-type image",
		},
		{
			name:      "file content",
			content:   []string{"--msg-type", "file", "--text", "ignored"},
			wantError: "--file-path is required for --msg-type file",
		},
		{
			name:      "unsupported type",
			content:   []string{"--msg-type", "video", "--text", "ignored"},
			wantError: "unsupported --msg-type \"video\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &chatBotRichMediaCaller{}
			args := []string{
				"message", "send-by-bot",
				"--robot-code", "robot-code",
				"--group", "group-open-cid",
			}
			args = append(args, tt.content...)
			err := runChatCoverageCommand(t, caller, args...)
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("error = %v, want %q", err, tt.wantError)
			}
			if len(caller.calls) != 0 {
				t.Fatalf("validation made remote calls: %#v", caller.calls)
			}
		})
	}
}

func TestParseConversationFileDownloadURL(t *testing.T) {
	tests := []struct {
		name      string
		response  string
		wantURL   string
		wantError string
	}{
		{
			name:     "top-level URL",
			response: `{"downloadUrl":"https://download.example/report.pdf"}`,
			wantURL:  "https://download.example/report.pdf",
		},
		{
			name:      "invalid JSON",
			response:  `{`,
			wantError: "failed to parse uploaded file response JSON",
		},
		{
			name:      "missing URL",
			response:  `{"result":{}}`,
			wantError: "uploaded file response missing downloadUrl",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConversationFileDownloadURL(tt.response)
			if tt.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("error = %v, want %q", err, tt.wantError)
				}
				return
			}
			if err != nil || got != tt.wantURL {
				t.Fatalf("URL = %q, error = %v, want %q", got, err, tt.wantURL)
			}
		})
	}
}

func TestChatMessageSendByBotUploadsLocalFile(t *testing.T) {
	testseam.Protect(t, &deps)
	testseam.Swap(t, &httpPutFile, func(_ context.Context, resourceURL string, _ map[string]string, _ string, _ int64) error {
		if resourceURL != "https://upload.example/file" {
			t.Fatalf("resourceURL = %q", resourceURL)
		}
		return nil
	})

	filePath := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(filePath, []byte("report"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name            string
		target          []string
		uploadTargetKey string
		uploadTarget    string
		sendTool        string
		sendTypeField   string
	}{
		{
			name:            "group",
			target:          []string{"--group", "group-open-cid"},
			uploadTargetKey: "openConversationId",
			uploadTarget:    "group-open-cid",
			sendTool:        "send_robot_group_message",
			sendTypeField:   "msgKey",
		},
		{
			name:            "user",
			target:          []string{"--users", "318617"},
			uploadTargetKey: "userId",
			uploadTarget:    "318617",
			sendTool:        "batch_send_robot_msg_to_users",
			sendTypeField:   "msgType",
		},
		{
			name:            "openDingTalkId",
			target:          []string{"--open-dingtalk-ids", "D-open-receiver"},
			uploadTargetKey: "openDingTalkId",
			uploadTarget:    "D-open-receiver",
			sendTool:        "batch_send_robot_msg_to_users",
			sendTypeField:   "msgType",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &chatBotRichMediaCaller{}
			args := []string{
				"message", "send-by-bot",
				"--robot-code", "robot-code",
				"--msg-type", "file",
				"--file-path", filePath,
			}
			args = append(args, tt.target...)
			if err := runChatCoverageCommand(t, caller, args...); err != nil {
				t.Fatalf("send file: %v", err)
			}
			if len(caller.calls) != 3 {
				t.Fatalf("calls = %#v, want init, commit and bot send", caller.calls)
			}
			if got := caller.calls[0].args[tt.uploadTargetKey]; got != tt.uploadTarget {
				t.Fatalf("upload %s = %v, want %s", tt.uploadTargetKey, got, tt.uploadTarget)
			}
			send := caller.calls[2]
			if send.server != "bot" || send.tool != tt.sendTool {
				t.Fatalf("send call = %s/%s, want bot/%s", send.server, send.tool, tt.sendTool)
			}
			if got := send.args[tt.sendTypeField]; got != "sampleDingtalkDriveFile" {
				t.Fatalf("%s = %v, want sampleDingtalkDriveFile", tt.sendTypeField, got)
			}
			if got := send.args["fileUrl"]; got != "https://download.example/report.pdf" {
				t.Fatalf("fileUrl = %v, want committed download URL", got)
			}
		})
	}
}

func TestChatMessageSendByBotLocalFileRejectsMultipleDirectRecipients(t *testing.T) {
	testseam.Protect(t, &deps)

	caller := &chatBotRichMediaCaller{}
	err := runChatCoverageCommand(t, caller,
		"message", "send-by-bot",
		"--robot-code", "robot-code",
		"--users", "100001,100002",
		"--msg-type", "file",
		"--file-path", "/tmp/not-read-before-target-validation.pdf",
	)
	if err == nil || !strings.Contains(err.Error(), "--file-path requires exactly one") {
		t.Fatalf("error = %v, want single-recipient guidance", err)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("validation made remote calls: %#v", caller.calls)
	}
}

func TestChatMessageSendByBotLocalFileErrorsAndDryRun(t *testing.T) {
	testseam.Protect(t, &deps)

	t.Run("missing local file", func(t *testing.T) {
		caller := &chatBotRichMediaCaller{}
		err := runChatCoverageCommand(t, caller,
			"message", "send-by-bot",
			"--robot-code", "robot-code",
			"--group", "group-open-cid",
			"--msg-type", "file",
			"--file-path", filepath.Join(t.TempDir(), "missing.pdf"),
		)
		if err == nil {
			t.Fatal("missing local file should fail")
		}
		if len(caller.calls) != 0 {
			t.Fatalf("validation made remote calls: %#v", caller.calls)
		}
	})

	filePath := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(filePath, []byte("report"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Run("dry run", func(t *testing.T) {
		caller := &chatBotRichMediaCaller{dryRun: true}
		if err := runChatCoverageCommand(t, caller,
			"message", "send-by-bot",
			"--robot-code", "robot-code",
			"--group", "group-open-cid",
			"--msg-type", "file",
			"--file-path", filePath,
		); err != nil {
			t.Fatalf("dry run: %v", err)
		}
		if len(caller.calls) != 0 {
			t.Fatalf("dry run made remote calls: %#v", caller.calls)
		}
	})

	t.Run("upload failure", func(t *testing.T) {
		testseam.Swap(t, &httpPutFile, func(context.Context, string, map[string]string, string, int64) error {
			return fmt.Errorf("put failed")
		})
		caller := &chatBotRichMediaCaller{}
		err := runChatCoverageCommand(t, caller,
			"message", "send-by-bot",
			"--robot-code", "robot-code",
			"--group", "group-open-cid",
			"--msg-type", "file",
			"--file-path", filePath,
		)
		if err == nil || !strings.Contains(err.Error(), "put failed") {
			t.Fatalf("error = %v, want upload failure", err)
		}
	})
}
