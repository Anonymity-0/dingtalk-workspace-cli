// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package chat

import (
	stderrors "errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
)

func TestMessagesSendResolvesNaturalUserAndChatTargets(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		responses  map[string]string
		wantTarget string
		wantValue  string
	}{
		{
			name: "user query",
			args: []string{
				"chat", "+messages-send", "--as", "user",
				"--user-query", "张三", "--text", "你好", "--yes",
			},
			responses: map[string]string{
				"contact/search_contact_by_key_word": `{"result":[{"name":"张三","userId":"u1","openDingTalkId":"D1"}]}`,
			},
			wantTarget: "receiverOpenDingTalkId",
			wantValue:  "D1",
		},
		{
			name: "chat query exact wins",
			args: []string{
				"chat", "+messages-send", "--as", "user",
				"--chat-query", "项目群", "--text", "你好", "--yes",
			},
			responses: map[string]string{
				"im/search_groups": `{"result":[{"title":"项目群-归档","openConversationId":"c2"},{"title":"项目群","openConversationId":"c1"}]}`,
			},
			wantTarget: "openConversationId",
			wantValue:  "c1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &larkAlignmentCaller{responses: tt.responses}
			helpers.InitDeps(fake)
			root := newPlatformCoverageRoot()
			root.SetArgs(tt.args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			if len(fake.calls) != 2 {
				t.Fatalf("calls = %#v, want resolve + send", fake.calls)
			}
			send := fake.calls[1]
			if send.product != "chat" || send.tool != "send_personal_message" {
				t.Fatalf("send = %#v", send)
			}
			if send.args[tt.wantTarget] != tt.wantValue {
				t.Fatalf("%s = %#v, want %q", tt.wantTarget, send.args[tt.wantTarget], tt.wantValue)
			}
		})
	}
}

func TestMessagesSendNaturalTargetAmbiguityHasNoWrite(t *testing.T) {
	fake := &larkAlignmentCaller{responses: map[string]string{
		"contact/search_contact_by_key_word": `{"result":[{"name":"张三","userId":"u1","openDingTalkId":"D1"},{"name":"张三","userId":"u2","openDingTalkId":"D2"}]}`,
	}}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{
		"chat", "+messages-send", "--as", "user",
		"--user-query", "张三", "--text", "你好", "--yes",
	})
	err := root.Execute()
	if err == nil {
		t.Fatal("ambiguous user unexpectedly sent")
	}
	if len(fake.calls) != 1 || fake.calls[0].tool != "search_contact_by_key_word" {
		t.Fatalf("ambiguous resolution reached write: %#v", fake.calls)
	}
	var typed *apperrors.Error
	if !stderrors.As(err, &typed) {
		t.Fatalf("error type = %T", err)
	}
	if typed.Reason != "resolution_ambiguous" || typed.Details["type"] != "resolution" {
		t.Fatalf("structured error = %#v", typed)
	}
}

func TestMessagesSendDryRunUsesRealNaturalTargetResolution(t *testing.T) {
	fake := &larkAlignmentCaller{responses: map[string]string{
		"im/search_groups": `{"result":[{"title":"项目群","openConversationId":"c1"}]}`,
	}}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{
		"chat", "+messages-send", "--as", "user",
		"--chat-query", "项目群", "--text", "你好", "--dry-run", "--yes",
	})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("dry-run calls = %#v, want one read-only resolution", fake.calls)
	}
	if fake.calls[0].product != "im" || fake.calls[0].tool != "search_groups" {
		t.Fatalf("dry-run resolution = %#v", fake.calls[0])
	}
}

func TestMessagesSendRejectsNaturalTargetForUnsupportedIdentity(t *testing.T) {
	fake := &larkAlignmentCaller{}
	helpers.InitDeps(fake)
	root := newPlatformCoverageRoot()
	root.SetArgs([]string{
		"chat", "+messages-send", "--as", "bot", "--robot-code", "r",
		"--chat-query", "项目群", "--text", "你好", "--yes",
	})
	if err := root.Execute(); err == nil {
		t.Fatal("bot natural target unexpectedly accepted")
	}
	if len(fake.calls) != 0 {
		t.Fatalf("invalid identity reached lower service: %#v", fake.calls)
	}
}
