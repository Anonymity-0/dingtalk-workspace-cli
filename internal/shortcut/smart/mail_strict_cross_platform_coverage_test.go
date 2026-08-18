// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package smart

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestCrossPlatformCoverageSmartMailContractsAreUnifiedAndTyped(t *testing.T) {
	for _, declaration := range []struct {
		name    string
		rollout output.RolloutState
		result  bool
	}{
		{"search", SearchMail.OutputRollout, SearchMail.Contract.Result != nil},
		{"unread", UnreadMail.OutputRollout, UnreadMail.Contract.Result != nil},
		{"find-user", FindMailUser.OutputRollout, FindMailUser.Contract.Result != nil},
		{"recent", RecentMail.OutputRollout, RecentMail.Contract.Result != nil},
		{"triage", TriageMail.OutputRollout, TriageMail.Contract.Result != nil},
	} {
		if declaration.rollout != output.RolloutUnifiedActive || !declaration.result {
			t.Fatalf("%s is not unified with Result", declaration.name)
		}
	}
}

func TestCrossPlatformCoverageSmartMailZeroHitSentinelIsNarrow(t *testing.T) {
	fixture := map[string]any{
		"success": "true", "total": "0", "nextCursor": "$",
		"messages": []any{map[string]any{"ccRecipients": nil, "toRecipients": nil}},
	}
	items, err := smartMailSearchRows(fixture, "mail/search_emails")
	if err != nil || len(items) != 0 {
		t.Fatalf("reviewed zero-hit sentinel must normalize to empty: items=%v err=%v", items, err)
	}
	for name, bad := range map[string]map[string]any{
		"unexpected field": {"success": "true", "total": "0", "nextCursor": "$", "messages": []any{map[string]any{"subject": nil}}},
		"non-null field":   {"success": "true", "total": "0", "nextCursor": "$", "messages": []any{map[string]any{"ccRecipients": "x"}}},
		"multiple rows":    {"success": "true", "total": "0", "nextCursor": "$", "messages": []any{map[string]any{"ccRecipients": nil}, map[string]any{"toRecipients": nil}}},
	} {
		if _, err := smartMailSearchRows(bad, "mail/search_emails"); err == nil {
			t.Fatalf("%s must fail closed", name)
		}
	}
}

func TestCrossPlatformCoverageSmartMailMalformedCollectionsFail(t *testing.T) {
	for name, fixture := range map[string]map[string]any{
		"missing success":    {"messages": []any{}},
		"missing collection": {"success": "true", "total": "0", "nextCursor": "$"},
		"wrong type":         {"success": "true", "messages": map[string]any{}},
		"bad item":           {"success": "true", "messages": []any{"bad"}},
	} {
		if _, err := smartMailCollection(fixture, "mail/search_emails", "messages"); err == nil {
			t.Fatalf("%s must fail closed", name)
		}
	}
}

func TestCrossPlatformCoverageSmartMailNestedSendersFailClosed(t *testing.T) {
	if _, err := searchMailFrom(map[string]any{"from": []any{"bad"}}); err == nil {
		t.Fatal("malformed message from field must fail")
	}
	if _, err := recentMailSenders(map[string]any{"senders": []any{"bad"}}); err == nil {
		t.Fatal("non-object conversation sender must fail")
	}
	if _, err := recentMailSenders(map[string]any{"senders": []any{map[string]any{"name": ""}}}); err == nil {
		t.Fatal("sender without identity must fail")
	}
}
