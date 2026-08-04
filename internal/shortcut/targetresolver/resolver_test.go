// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package targetresolver

import (
	stderrors "errors"
	"reflect"
	"strings"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

type chatResolutionReader struct {
	responses []map[string]any
	calls     []map[string]any
}

func (r *chatResolutionReader) CallMCPData(product, tool string, params map[string]any) (map[string]any, error) {
	r.calls = append(r.calls, params)
	if product != "im" || tool != "search_groups" {
		return nil, stderrors.New("unexpected resolver tool")
	}
	if len(r.calls) > len(r.responses) {
		return nil, stderrors.New("unexpected resolver page")
	}
	return r.responses[len(r.calls)-1], nil
}

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

func TestOpenConversationIDIsNeverSearchedAsAGroupName(t *testing.T) {
	for _, value := range []string{"cid-fixture-chat-0001", " CIDO123456789 "} {
		if !LooksLikeOpenConversationID(value) {
			t.Fatalf("LooksLikeOpenConversationID(%q) = false", value)
		}
	}
	for _, value := range []string{"cid", "项目cid群", "conversation-1"} {
		if LooksLikeOpenConversationID(value) {
			t.Fatalf("LooksLikeOpenConversationID(%q) = true", value)
		}
	}

	reader := &chatResolutionReader{}
	_, err := ResolveChat(reader, "cid-fixture-chat-0001")
	var typed *apperrors.Error
	if err == nil || !stderrors.As(err, &typed) || typed.Reason != "target_type_mismatch" {
		t.Fatalf("ResolveChat(stable id) error = %v", err)
	}
	if strings.Contains(typed.Message, "看起来是") || !strings.Contains(typed.Message, "群目标参数类型不匹配") {
		t.Fatalf("ResolveChat(stable id) message = %q", typed.Message)
	}
	if typed.Details["providedType"] != "openConversationId" || typed.Details["expectedType"] != "chatName" {
		t.Fatalf("ResolveChat(stable id) details = %#v", typed.Details)
	}
	if len(reader.calls) != 0 {
		t.Fatalf("stable id unexpectedly reached search: %#v", reader.calls)
	}
}

func TestResolveChatTargetStableIDBypassesSearch(t *testing.T) {
	reader := &chatResolutionReader{}
	resolved, err := ResolveChatTarget(reader, " cid-fixture-chat-0001 ", "")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Selected.OpenConversationID != "cid-fixture-chat-0001" || resolved.MatchType != "stable_id" {
		t.Fatalf("resolved = %#v", resolved)
	}
	if len(reader.calls) != 0 {
		t.Fatalf("stable id unexpectedly reached search: %#v", reader.calls)
	}
}

func TestResolveChatTargetNaturalDirectValueUsesResolver(t *testing.T) {
	reader := &chatResolutionReader{responses: []map[string]any{{
		"result":  []any{map[string]any{"openConversationId": "cid-project-1", "title": "项目群"}},
		"hasMore": false,
	}}}
	resolved, err := ResolveChatTarget(reader, "项目群", "")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Selected.OpenConversationID != "cid-project-1" || len(reader.calls) != 1 {
		t.Fatalf("resolved = %#v calls = %#v", resolved, reader.calls)
	}
}

func TestResolveChatTargetQueryStableIDAlsoBypassesSearch(t *testing.T) {
	reader := &chatResolutionReader{}
	resolved, err := ResolveChatTarget(reader, "", "cid-query-123456")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Selected.OpenConversationID != "cid-query-123456" || len(reader.calls) != 0 {
		t.Fatalf("resolved = %#v calls = %#v", resolved, reader.calls)
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

func TestResolveChatPagesBeforeApplyingExactPreference(t *testing.T) {
	reader := &chatResolutionReader{responses: []map[string]any{
		{
			"result": []any{
				map[string]any{"openConversationId": "archive", "title": "项目群-归档"},
			},
			"hasMore":    true,
			"nextCursor": "page-2",
		},
		{
			"result": []any{
				map[string]any{"openConversationId": "active", "title": "项目群"},
			},
			"hasMore": false,
		},
	}}
	resolved, err := ResolveChat(reader, "项目群")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Selected.OpenConversationID != "active" || resolved.MatchType != "exact" {
		t.Fatalf("resolved = %#v", resolved)
	}
	wantCursors := []any{"0", "page-2"}
	gotCursors := []any{reader.calls[0]["cursor"], reader.calls[1]["cursor"]}
	if !reflect.DeepEqual(gotCursors, wantCursors) {
		t.Fatalf("cursors = %#v, want %#v", gotCursors, wantCursors)
	}
}

func TestResolveChatKeepsExactNamesakesAcrossPagesAmbiguous(t *testing.T) {
	reader := &chatResolutionReader{responses: []map[string]any{
		{
			"result": []any{
				map[string]any{"openConversationId": "c1", "title": "项目群"},
			},
			"hasMore":    true,
			"nextCursor": "page-2",
		},
		{
			"result": map[string]any{
				"items": []any{
					map[string]any{"openConversationId": "c2", "title": "项目群"},
				},
				"hasMore": false,
			},
		},
	}}
	_, err := ResolveChat(reader, "项目群")
	var typed *apperrors.Error
	if !stderrors.As(err, &typed) || typed.Reason != "resolution_ambiguous" {
		t.Fatalf("error = %#v", err)
	}
	candidates, ok := typed.Details["candidates"].([]Chat)
	if !ok || len(candidates) != 2 {
		t.Fatalf("details = %#v", typed.Details)
	}
}

func TestResolveChatFailsClosedWhenPaginationCannotAdvance(t *testing.T) {
	for _, tc := range []struct {
		name         string
		nextCursor   any
		wantFragment string
	}{
		{name: "missing cursor", wantFragment: "没有返回可继续"},
		{name: "stalled cursor", nextCursor: "0", wantFragment: "游标停滞"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := map[string]any{
				"result": []any{
					map[string]any{"openConversationId": "c1", "title": "项目群"},
				},
				"hasMore": true,
			}
			if tc.nextCursor != nil {
				response["nextCursor"] = tc.nextCursor
			}
			reader := &chatResolutionReader{responses: []map[string]any{response}}
			_, err := ResolveChat(reader, "项目群")
			var typed *apperrors.Error
			if !stderrors.As(err, &typed) || typed.Reason != "resolution_incomplete" || !typed.Retryable {
				t.Fatalf("error = %#v", err)
			}
			if typed.Details["subtype"] != StatusIncomplete {
				t.Fatalf("details = %#v", typed.Details)
			}
			if typed.Origin != "mcp_gateway" || typed.FailureStage != "target_resolution" || typed.ExecutionStarted == nil || *typed.ExecutionStarted {
				t.Fatalf("failure semantics = origin %q stage %q execution_started %v", typed.Origin, typed.FailureStage, typed.ExecutionStarted)
			}
			cause, _ := typed.Details["cause"].(string)
			if cause == "" || !strings.Contains(cause, tc.wantFragment) {
				t.Fatalf("cause = %q, want fragment %q", cause, tc.wantFragment)
			}
		})
	}
}

func TestResolveChatAcceptsShortLegacyPageWithoutPaginationMetadata(t *testing.T) {
	reader := &chatResolutionReader{responses: []map[string]any{{
		"result": []any{
			map[string]any{"openConversationId": "c1", "title": "项目群"},
		},
	}}}
	resolved, err := ResolveChat(reader, "项目群")
	if err != nil || resolved.Selected.OpenConversationID != "c1" {
		t.Fatalf("resolved = %#v, err = %v", resolved, err)
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
	if typed.Origin != "client" || typed.FailureStage != "target_resolution" || typed.ExecutionStarted == nil || *typed.ExecutionStarted {
		t.Fatalf("failure semantics = origin %q stage %q execution_started %v", typed.Origin, typed.FailureStage, typed.ExecutionStarted)
	}
	if typed.Details["type"] != "resolution" || typed.Details["subtype"] != StatusAmbiguous {
		t.Fatalf("details = %#v", typed.Details)
	}
	candidates, ok := typed.Details["candidates"].([]Chat)
	if !ok || len(candidates) != 2 {
		t.Fatalf("candidates = %#v", typed.Details["candidates"])
	}
}
