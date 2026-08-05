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

package smart

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"os"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

// TestProjectChatMessageExpandsForwarded guards that a forwarded chat record
// ("聊天记录") exposes its nested messages under "forwarded" instead of
// collapsing to the lossy top-level "[卡片]" summary, recursing through nested
// forwards, and that the string-"null" sender is nulled out. The per-field
// behaviour (sender/text/encryption) is covered in the chatmsg package tests.
func TestCrossPlatformCoverageProjectChatMessageExpandsForwarded(t *testing.T) {
	row := projectChatMessage(map[string]any{
		"sender":             "hugozhu",
		"openMessageId":      "msg-top",
		"openConversationId": "cid-top",
		"openConvThreadId":   "thread-top",
		"msgType":            "text",
		"content":            "hugozhu与opencode-agent的聊天记录\nopencode-agent:[卡片]",
		"createTime":         "2026-07-20 21:41:21",
		"emotionReplyList": []any{
			map[string]any{"emoji": "赞", "replyUsers": []any{"D1"}},
		},
		"forwardMessages": []any{
			map[string]any{"sender": "null", "content": "读下冬翔发给我的最近两条消息", "createTime": "2026-07-20 09:30:33"},
			map[string]any{"sender": "冬翔", "content": "W29 工作总结", "createTime": "2026-07-19 23:35:40",
				// nested forward inside a forward — must expand recursively.
				"forwardMessages": []any{
					map[string]any{"sender": "念晨", "content": "收到", "createTime": "2026-07-19 23:36:00"},
				},
			},
		},
	})

	if row["sender"] != "hugozhu" {
		t.Fatalf("top sender = %v, want hugozhu", row["sender"])
	}
	if row["messageId"] != "msg-top" || row["conversationId"] != "cid-top" || row["messageType"] != "text" {
		t.Errorf("stable identity = %#v", row)
	}
	if row["threadId"] != "thread-top" {
		t.Errorf("thread identity = %#v", row)
	}
	if reactions, ok := row["reactions"].(map[string]any); !ok || len(reactions) == 0 {
		t.Errorf("reactions = %#v", row["reactions"])
	}
	forwarded, ok := row["forwarded"].([]map[string]any)
	if !ok || len(forwarded) != 2 {
		t.Fatalf("forwarded = %#v, want 2 entries", row["forwarded"])
	}
	if forwarded[0]["sender"] != nil {
		t.Errorf("forwarded[0].sender = %v, want nil (string \"null\")", forwarded[0]["sender"])
	}
	if forwarded[0]["text"] != "读下冬翔发给我的最近两条消息" {
		t.Errorf("forwarded[0].text = %v", forwarded[0]["text"])
	}
	nested, ok := forwarded[1]["forwarded"].([]map[string]any)
	if !ok || len(nested) != 1 || nested[0]["sender"] != "念晨" {
		t.Errorf("nested forwarded = %#v, want 1 entry from 念晨", forwarded[1]["forwarded"])
	}

	// A plain message must not grow a "forwarded" key.
	plain := projectChatMessage(map[string]any{"sender": "念晨", "content": "hi", "createTime": "t"})
	if _, has := plain["forwarded"]; has {
		t.Errorf("plain message unexpectedly has forwarded key: %#v", plain)
	}

	withoutReactions := projectChatMessageWithReactions(map[string]any{
		"content": "hi",
		"emotionReplyList": []any{
			map[string]any{"emoji": "赞", "replyUsers": []any{"D1"}},
		},
	}, false)
	if _, has := withoutReactions["reactions"]; has {
		t.Errorf("no-reactions projection leaked reactions: %#v", withoutReactions)
	}
}

type chatMessagesPagingCaller struct {
	responses []string
	args      []map[string]any
	failAt    int
}

type chatMessagesFailWriter struct{}

func (chatMessagesFailWriter) Write([]byte) (int, error) {
	return 0, stderrors.New("fixture output failure")
}

func (c *chatMessagesPagingCaller) CallTool(
	_ context.Context,
	_, _ string,
	args map[string]any,
) (*edition.ToolResult, error) {
	copied := make(map[string]any, len(args))
	for key, value := range args {
		copied[key] = value
	}
	c.args = append(c.args, copied)
	index := len(c.args) - 1
	if c.failAt > 0 && len(c.args) == c.failAt {
		return nil, stderrors.New("fixture read failure")
	}
	if index >= len(c.responses) {
		index = len(c.responses) - 1
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{
		Type: "text",
		Text: c.responses[index],
	}}}, nil
}

func (c *chatMessagesPagingCaller) CallReadTool(
	ctx context.Context,
	product, tool string,
	args map[string]any,
) (*edition.ToolResult, error) {
	return c.CallTool(ctx, product, tool, args)
}

func (*chatMessagesPagingCaller) Format() string { return "json" }
func (*chatMessagesPagingCaller) DryRun() bool   { return false }
func (*chatMessagesPagingCaller) Fields() string { return "" }
func (*chatMessagesPagingCaller) JQ() string     { return "" }

func TestCrossPlatformCoverageChatMessagesPageAllUsesTypedBoundaryAndDeduplicates(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"messages":[{"openMessageId":"m2","createTime":"2026-01-02 00:00:00"},{"openMessageId":"m1","createTime":"2026-01-01 00:00:00"}]}}`,
		`{"result":{"hasMore":false,"messages":[{"openMessageId":"m1","createTime":"2026-01-01 00:00:00"},{"openMessageId":"m0","createTime":"2025-12-31 00:00:00"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid",
		"--time", "2026-01-03 00:00:00", "--page-all", "--page-limit", "5",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(caller.args) != 2 || caller.args[1]["time"] != "2026-01-01 00:00:00" {
		t.Fatalf("pagination calls = %#v", caller.args)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["complete"] != true || payload["hasMore"] != false ||
		payload["count"] != float64(3) || payload["pagesFetched"] != float64(2) ||
		payload["stopReason"] != "source_complete" {
		t.Fatalf("all-page payload = %#v", payload)
	}
}

func TestCrossPlatformCoverageChatMessagesPageAllPublishesBoundedContinuation(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"messages":[{"openMessageId":"m1","createTime":"2026-01-01 00:00:00"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid",
		"--page-all", "--page-limit", "1",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	next, _ := payload["nextPage"].(map[string]any)
	if payload["complete"] != false || payload["hasMore"] != true ||
		payload["truncatedByPageLimit"] != true || payload["stopReason"] != "page_limit" ||
		next["time"] != "2026-01-01 00:00:00" {
		t.Fatalf("bounded payload = %#v", payload)
	}
}

func TestCrossPlatformCoverageChatMessagesPageAllFailsClosedOnStalledBoundary(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"messages":[{"openMessageId":"m1"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"chat", "+chat-messages", "--conversation-id", "cid", "--page-all"})
	err := root.Execute()
	var typed *apperrors.Error
	if !stderrors.As(err, &typed) || typed.Category != apperrors.CategoryAPI ||
		typed.Reason != "chat_messages_incomplete" || !typed.Retryable ||
		typed.ExecutionStarted == nil || !*typed.ExecutionStarted {
		t.Fatalf("error = %#v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["complete"] != false || payload["failedCount"] != float64(1) ||
		payload["stopReason"] != "pagination_error" {
		t.Fatalf("stalled payload = %#v", payload)
	}
}

func TestCrossPlatformCoverageChatMessagesFailedPageDoesNotExportPartialLedger(t *testing.T) {
	t.Chdir(t.TempDir())
	caller := &chatMessagesPagingCaller{
		responses: []string{
			`{"result":{"hasMore":true,"messages":[{"openMessageId":"m1","createTime":"2026-01-01 00:00:00"}]}}`,
		},
		failAt: 2,
	}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid", "--page-all",
		"--output", "exports/partial.json",
	})
	err := root.Execute()
	var typed *apperrors.Error
	if !stderrors.As(err, &typed) || typed.Reason != "chat_messages_incomplete" {
		t.Fatalf("error = %#v", err)
	}
	if _, statErr := os.Lstat("exports/partial.json"); !os.IsNotExist(statErr) {
		t.Fatalf("partial export exists: %v", statErr)
	}
	var ledger map[string]any
	if err := json.Unmarshal(output.Bytes(), &ledger); err != nil {
		t.Fatal(err)
	}
	if ledger["partial"] != true || ledger["failedCount"] != float64(1) || ledger["count"] != float64(1) {
		t.Fatalf("failure ledger = %#v", ledger)
	}
}

func TestCrossPlatformCoverageChatMessagesFailureLedgerOutputErrorIsNonZero(t *testing.T) {
	caller := &chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":true,"messages":[{"openMessageId":"m1"}]}}`,
	}}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	root.SetOut(chatMessagesFailWriter{})
	root.SetArgs([]string{"chat", "+chat-messages", "--conversation-id", "cid", "--page-all"})
	if err := root.Execute(); err == nil || err.Error() != "fixture output failure" {
		t.Fatalf("error = %v", err)
	}
}

func TestCrossPlatformCoverageChatMessagesExportIsAtomicAndNoClobber(t *testing.T) {
	t.Chdir(t.TempDir())
	newCaller := func() *chatMessagesPagingCaller {
		return &chatMessagesPagingCaller{responses: []string{
			`{"result":{"hasMore":false,"messages":[{"openMessageId":"m1","createTime":"2026-01-01 00:00:00"}]}}`,
		}}
	}
	run := func(overwrite bool) error {
		helpers.InitDeps(newCaller())
		root := newPlatformCoverageRoot()
		root.SetOut(&bytes.Buffer{})
		args := []string{"chat", "+chat-messages", "--conversation-id", "cid", "--page-all", "--output", "exports/messages.json"}
		if overwrite {
			args = append(args, "--overwrite")
		}
		root.SetArgs(args)
		return root.Execute()
	}
	if err := run(false); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("exports/messages.json")
	if err != nil {
		t.Fatal(err)
	}
	var exported map[string]any
	if err := json.Unmarshal(raw, &exported); err != nil {
		t.Fatal(err)
	}
	if exported["complete"] != true || exported["count"] != float64(1) {
		t.Fatalf("exported ledger = %#v", exported)
	}
	if err := run(false); err == nil {
		t.Fatal("existing export was overwritten without --overwrite")
	}
	if err := run(true); err != nil {
		t.Fatal(err)
	}
}

func TestCrossPlatformCoverageChatMessagesExportRejectsNonJSONPlaceholder(t *testing.T) {
	t.Chdir(t.TempDir())
	helpers.InitDeps(&chatMessagesPagingCaller{responses: []string{
		`{"result":{"hasMore":false,"messages":[]}}`,
	}})
	root := newPlatformCoverageRoot()
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{
		"chat", "+chat-messages", "--conversation-id", "cid", "--output", "{}",
	})
	if err := root.Execute(); err == nil {
		t.Fatal("non-JSON placeholder output unexpectedly succeeded")
	}
	if _, err := os.Lstat("{}"); !os.IsNotExist(err) {
		t.Fatalf("placeholder output was created: %v", err)
	}
}
