// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package targetresolver

import (
	stderrors "errors"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestExtractUsersKeepsUsableExternalContacts(t *testing.T) {
	users := ExtractUsers(map[string]any{
		"result": []any{
			map[string]any{"userId": "u1", "openDingTalkId": "D1", "name": "张三"},
			map[string]any{"openDingtalkId": "D2", "nick": "外部张三"},
			map[string]any{"name": "无 ID"},
			"garbage",
		},
	})
	if len(users) != 2 {
		t.Fatalf("users = %#v", users)
	}
	if users[1].OpenDingTalkID != "D2" || users[1].Name != "外部张三" {
		t.Fatalf("external user = %#v", users[1])
	}
}

func TestUserSelectionDedupesButDoesNotHideNamesakes(t *testing.T) {
	users := dedupeUsers([]User{
		{UserID: "u1", OpenDingTalkID: "D1", Name: "张三"},
		{UserID: "u1", OpenDingTalkID: "D1", Name: "duplicate"},
		{UserID: "u2", OpenDingTalkID: "D2", Name: "张三丰"},
	})
	selected, matchType := selectUsers(users, "  张三 ")
	if len(selected) != 2 || matchType != "ambiguous" {
		t.Fatalf("selected = %#v, matchType = %q", selected, matchType)
	}
}

func TestChatSelectionKeepsMultipleExactMatchesAmbiguous(t *testing.T) {
	chats := dedupeChats([]Chat{
		{OpenConversationID: "c1", Name: "项目群"},
		{OpenConversationID: "c1", Name: "项目群"},
		{OpenConversationID: "c2", Name: "项目群"},
		{OpenConversationID: "c3", Name: "项目群-归档"},
	})
	selected, matchType := preferExactChats(chats, "项目群")
	if len(selected) != 2 || matchType != "exact" {
		t.Fatalf("selected = %#v, matchType = %q", selected, matchType)
	}
}

func TestResolutionErrorCarriesStructuredCandidates(t *testing.T) {
	err := newResolutionError(StatusAmbiguous, "chat", "项目群", []Chat{
		{OpenConversationID: "c1", Name: "项目群"},
		{OpenConversationID: "c2", Name: "项目群"},
	})
	var typed *apperrors.Error
	if !stderrors.As(err, &typed) {
		t.Fatalf("error type = %T", err)
	}
	if typed.Reason != "resolution_ambiguous" {
		t.Fatalf("reason = %q", typed.Reason)
	}
	if typed.Details["type"] != "resolution" || typed.Details["subtype"] != StatusAmbiguous {
		t.Fatalf("details = %#v", typed.Details)
	}
	candidates, ok := typed.Details["candidates"].([]Chat)
	if !ok || len(candidates) != 2 {
		t.Fatalf("candidates = %#v", typed.Details["candidates"])
	}
}
