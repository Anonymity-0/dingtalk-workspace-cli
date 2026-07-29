// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package smart

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
)

func TestCrossPlatformCoverageMessageReadShortcutsPublishResourceDownloadPlans(t *testing.T) {
	message := `{"openMessageId":"msg","openConversationId":"cid","content":"{\"mediaId\":\"@image\"}"}`
	tests := []struct {
		name      string
		tool      string
		response  string
		args      []string
		resultKey string
	}{
		{
			name:      "chat messages",
			tool:      "chat/list_conversation_message_v2",
			response:  `{"result":{"messages":[` + message + `]}}`,
			args:      []string{"chat", "+chat-messages", "--group", "cid"},
			resultKey: "messages",
		},
		{
			name:      "search",
			tool:      "im/search_messages",
			response:  `{"result":{"messages":[` + message + `],"hasMore":false}}`,
			args:      []string{"chat", "+search-msg", "--query", "x", "--no-enrich"},
			resultKey: "messages",
		},
		{
			name:      "at me",
			tool:      "chat/search_at_me_message",
			response:  `{"result":{"conversationMessagesList":[{"openConversationId":"cid","messages":[` + message + `]}]}}`,
			args:      []string{"chat", "+at-me"},
			resultKey: "messages",
		},
		{
			name:      "thread replies",
			tool:      "chat/list_topic_replies",
			response:  `{"result":{"messages":[` + message + `]}}`,
			args:      []string{"chat", "+thread-replies", "--group", "cid", "--thread-id", "thread"},
			resultKey: "replies",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &smartCoverageCaller{responses: map[string][]string{
				tc.tool: {tc.response},
			}}
			helpers.InitDeps(caller)
			root := newPlatformCoverageRoot()
			var output bytes.Buffer
			root.SetOut(&output)
			args := append([]string{}, tc.args...)
			args = append(args, "--download-resources", "--output-dir", "./downloads", "--dry-run")
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if caller.counts[tc.tool] != 1 {
				t.Fatalf("lower calls = %#v", caller.counts)
			}
			var payload map[string]any
			if err := json.Unmarshal(output.Bytes(), &payload); err != nil {
				t.Fatalf("decode output: %v\n%s", err, output.String())
			}
			if _, ok := payload[tc.resultKey]; !ok {
				t.Fatalf("payload missing %s: %#v", tc.resultKey, payload)
			}
			ledger, _ := payload["resourceDownloads"].(map[string]any)
			if ledger["dryRun"] != true || ledger["requestedCount"] != float64(1) {
				t.Fatalf("resource plan = %#v", ledger)
			}
		})
	}
}

func TestCrossPlatformCoverageMessageReadShortcutResourceOutputValidation(t *testing.T) {
	for _, args := range [][]string{
		{"chat", "+chat-messages", "--group", "cid"},
		{"chat", "+search-msg", "--query", "x", "--no-enrich"},
		{"chat", "+at-me"},
		{"chat", "+thread-replies", "--group", "cid", "--thread-id", "thread"},
	} {
		helpers.InitDeps(&smartCoverageCaller{})
		root := newPlatformCoverageRoot()
		root.SetArgs(append(args, "--download-resources", "--output-dir", "../outside"))
		if err := root.Execute(); err == nil {
			t.Fatalf("unsafe output accepted: %v", args)
		}
	}
}

func TestCrossPlatformCoverageChatMessagesDefaultsToRecentHistory(t *testing.T) {
	caller := &platformCoverageCaller{}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	before := time.Now().Add(-2 * time.Second)
	root.SetArgs([]string{"chat", "+chat-messages", "--group", "cid", "--limit", "5"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	after := time.Now().Add(2 * time.Second)
	if len(caller.calls) != 1 {
		t.Fatalf("calls = %#v", caller.calls)
	}
	call := caller.calls[0]
	if call.args["forward"] != false {
		t.Fatalf("default forward = %#v, want false", call.args["forward"])
	}
	boundary, err := time.ParseInLocation("2006-01-02 15:04:05", call.args["time"].(string), time.Local)
	if err != nil {
		t.Fatal(err)
	}
	if boundary.Before(before) || boundary.After(after) {
		t.Fatalf("default time = %s, want current boundary", boundary)
	}
}

func TestCrossPlatformCoverageChatMessagesPreservesExplicitTime(t *testing.T) {
	caller := &platformCoverageCaller{}
	helpers.InitDeps(caller)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{
		"chat", "+chat-messages",
		"--group", "cid",
		"--time", "2026-07-01 12:34:56",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 1 ||
		caller.calls[0].args["time"] != "2026-07-01 12:34:56" {
		t.Fatalf("explicit time call = %#v", caller.calls)
	}
}

func TestCrossPlatformCoverageSearchMsgFlattensRealGroupedResponse(t *testing.T) {
	original := map[string]any{"openMessageId": "m1"}
	items := searchMsgItems(map[string]any{
		"result": map[string]any{
			"conversationMessagesList": []any{
				"invalid-group",
				map[string]any{"messages": "invalid"},
				map[string]any{
					"openConversationId": "cid",
					"title":              "会话",
					"singleChat":         true,
					"messages": []any{
						"invalid-message",
						original,
						map[string]any{
							"openMessageId":      "m2",
							"openConversationId": "own-cid",
							"conversationTitle":  "own-title",
							"singleChat":         false,
						},
					},
				},
			},
		},
	})
	if len(items) != 2 {
		t.Fatalf("grouped items = %#v", items)
	}
	if items[0]["openConversationId"] != "cid" || items[0]["conversationTitle"] != "会话" ||
		items[0]["singleChat"] != true {
		t.Fatalf("injected group context = %#v", items[0])
	}
	if items[1]["openConversationId"] != "own-cid" || items[1]["conversationTitle"] != "own-title" ||
		items[1]["singleChat"] != false {
		t.Fatalf("message context was overwritten: %#v", items[1])
	}
	if _, mutated := original["openConversationId"]; mutated {
		t.Fatalf("source message mutated: %#v", original)
	}
	if searchMsgChildMap(map[string]any{"result": "invalid"}, "result") != nil {
		t.Fatal("non-map child was accepted")
	}
}
