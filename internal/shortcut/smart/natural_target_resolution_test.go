// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package smart

import (
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
)

func TestChatMessagesResolvesNaturalChatAndUserTargets(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantTool  string
		wantKey   string
		wantValue string
	}{
		{
			name:      "chat query",
			args:      []string{"chat", "+chat-messages", "--chat-query", "项目冲刺"},
			wantTool:  "list_conversation_message_v2",
			wantKey:   "openconversation_id",
			wantValue: "cid-1",
		},
		{
			name:      "natural group through group flag",
			args:      []string{"chat", "+chat-messages", "--group", "项目冲刺"},
			wantTool:  "list_conversation_message_v2",
			wantKey:   "openconversation_id",
			wantValue: "cid-1",
		},
		{
			name:      "user query",
			args:      []string{"chat", "+chat-messages", "--user-query", "张三"},
			wantTool:  "list_individual_chat_message",
			wantKey:   "openDingTalkId",
			wantValue: "open1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &platformCoverageCaller{}
			helpers.InitDeps(fake)
			root := newPlatformCoverageRoot()
			root.SetArgs(tt.args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if len(fake.calls) != 2 {
				t.Fatalf("calls = %#v, want resolve + read", fake.calls)
			}
			read := fake.calls[1]
			if read.tool != tt.wantTool || read.args[tt.wantKey] != tt.wantValue {
				t.Fatalf("read = %#v", read)
			}
		})
	}
}

func TestChatMessagesStableGroupBypassesNaturalResolution(t *testing.T) {
	fake := &platformCoverageCaller{}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{"chat", "+chat-messages", "--group", "cidACeQ0fCtKfLsFGvA47gXaQ=="})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 || fake.calls[0].tool != "list_conversation_message_v2" ||
		fake.calls[0].args["openconversation_id"] != "cidACeQ0fCtKfLsFGvA47gXaQ==" {
		t.Fatalf("calls = %#v", fake.calls)
	}
}

func TestChatMessagesNaturalUserAmbiguityStopsBeforeMessageRead(t *testing.T) {
	fake := &platformCoverageCaller{contactSearchResult: `{"result":[{"name":"张三","userId":"u1","openDingTalkId":"D1"},{"name":"张三","userId":"u2","openDingTalkId":"D2"}]}`}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{"chat", "+chat-messages", "--user-query", "张三"})
	if err := root.Execute(); err == nil {
		t.Fatal("ambiguous user unexpectedly reached message read")
	}
	if len(fake.calls) != 1 || fake.calls[0].tool != "search_contact_by_key_word" {
		t.Fatalf("calls = %#v", fake.calls)
	}
}

func TestChatMessagesRejectsConversationIDInPeerIdentityFlag(t *testing.T) {
	fake := &platformCoverageCaller{}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{"chat", "+chat-messages", "--open-dingtalk-id", "cidACeQ0fCtKfLsFGvA47gXaQ=="})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "--group") {
		t.Fatalf("error = %v", err)
	}
	if len(fake.calls) != 0 {
		t.Fatalf("invalid identity reached lower API: %#v", fake.calls)
	}
}

func TestAtMeResolvesNaturalGroupBeforeSearch(t *testing.T) {
	fake := &platformCoverageCaller{}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{"chat", "+at-me", "--chat-query", "项目冲刺"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 2 || fake.calls[1].tool != "search_at_me_message" || fake.calls[1].args["openConversationId"] != "cid-1" {
		t.Fatalf("calls = %#v", fake.calls)
	}
}

func TestAtMeStableIDInQueryBypassesNaturalResolution(t *testing.T) {
	fake := &platformCoverageCaller{}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{"chat", "+at-me", "--chat-query", "cidACeQ0fCtKfLsFGvA47gXaQ=="})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 || fake.calls[0].tool != "search_at_me_message" ||
		fake.calls[0].args["openConversationId"] != "cidACeQ0fCtKfLsFGvA47gXaQ==" {
		t.Fatalf("calls = %#v", fake.calls)
	}
}

func TestSendToGroupStableIDBypassesNaturalResolution(t *testing.T) {
	fake := &platformCoverageCaller{}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{"chat", "+send-to-group", "--group", "cidACeQ0fCtKfLsFGvA47gXaQ==", "--text", "评测", "--yes"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 || fake.calls[0].tool != "send_personal_message" ||
		fake.calls[0].args["openConversationId"] != "cidACeQ0fCtKfLsFGvA47gXaQ==" {
		t.Fatalf("calls = %#v", fake.calls)
	}
}

func TestSearchMsgResolvesNaturalChatAndSenderBeforeSearch(t *testing.T) {
	fake := &platformCoverageCaller{contactSearchResult: `{"result":[{"name":"张三","userId":"u1","openDingTalkId":"D1"}]}`}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{
		"chat", "+search-msg",
		"--chat-query", "项目冲刺",
		"--sender-query", "张三",
		"--no-enrich",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 3 {
		t.Fatalf("calls = %#v, want chat resolve + user resolve + search", fake.calls)
	}
	search := fake.calls[2]
	if search.product != "im" || search.tool != "search_messages" {
		t.Fatalf("search = %#v", search)
	}
	if got, want := search.args["openConversationIds"], []string{"cid-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("openConversationIds = %#v, want %#v", got, want)
	}
	if got, want := search.args["senderOpenDingTakIds"], []string{"D1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("senderOpenDingTakIds = %#v, want %#v", got, want)
	}
}

func TestSearchMsgAcceptsStableIDInChatQuery(t *testing.T) {
	fake := &platformCoverageCaller{}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{
		"chat", "+search-msg",
		"--chat-query", "cidRYtcWlwRzUKMD1/o2/J4PA==",
		"--text", "评测",
		"--no-enrich",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 || fake.calls[0].tool != "search_messages" {
		t.Fatalf("calls = %#v", fake.calls)
	}
	if got, want := fake.calls[0].args["openConversationIds"], []string{"cidRYtcWlwRzUKMD1/o2/J4PA=="}; !reflect.DeepEqual(got, want) {
		t.Fatalf("openConversationIds = %#v, want %#v", got, want)
	}
	if fake.calls[0].args["keyword"] != "评测" {
		t.Fatalf("keyword = %#v", fake.calls[0].args["keyword"])
	}
}

func TestChatMembersListGroupAcceptsNameAndStableID(t *testing.T) {
	for _, tt := range []struct {
		name      string
		args      []string
		wantCalls int
	}{
		{name: "name", args: []string{"chat", "+chat-members-list", "--group", "项目冲刺", "--member-types", "user"}, wantCalls: 2},
		{name: "stable id", args: []string{"chat", "+chat-members-list", "--group", "cidACeQ0fCtKfLsFGvA47gXaQ==", "--member-types", "user"}, wantCalls: 1},
		{name: "compat alias", args: []string{"chat", "+chat-members-list", "--open-conversation-id", "cid-short-placeholder", "--member-types", "user"}, wantCalls: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			fake := &platformCoverageCaller{}
			helpers.InitDeps(fake)
			root := newPlatformCoverageRoot()
			root.SetArgs(tt.args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if len(fake.calls) != tt.wantCalls || fake.calls[len(fake.calls)-1].tool != "get_group_members" {
				t.Fatalf("calls = %#v", fake.calls)
			}
		})
	}
}

func TestSearchMsgNaturalSenderAmbiguityStopsBeforeSearch(t *testing.T) {
	fake := &platformCoverageCaller{contactSearchResult: `{"result":[{"name":"张三","userId":"u1","openDingTalkId":"D1"},{"name":"张三","userId":"u2","openDingTalkId":"D2"}]}`}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{"chat", "+search-msg", "--sender-query", "张三", "--no-enrich"})
	if err := root.Execute(); err == nil {
		t.Fatal("ambiguous sender unexpectedly reached search")
	}
	if len(fake.calls) != 1 || fake.calls[0].tool != "search_contact_by_key_word" {
		t.Fatalf("calls = %#v", fake.calls)
	}
}
