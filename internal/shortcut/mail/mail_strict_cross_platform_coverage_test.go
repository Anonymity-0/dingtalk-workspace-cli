// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package mail

import (
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
)

func TestCrossPlatformCoverageMailContractsAreUnifiedAndTyped(t *testing.T) {
	declarations := []*struct {
		rollout output.RolloutState
		result  bool
	}{
		{ThreadList.OutputRollout, ThreadList.Contract.Result != nil},
		{FolderList.OutputRollout, FolderList.Contract.Result != nil},
		{TagList.OutputRollout, TagList.Contract.Result != nil},
		{UserSearch.OutputRollout, UserSearch.Contract.Result != nil},
		{TemplateList.OutputRollout, TemplateList.Contract.Result != nil},
		{ContactList.OutputRollout, ContactList.Contract.Result != nil},
		{Message.OutputRollout, Message.Contract.Result != nil},
		{Messages.OutputRollout, Messages.Contract.Result != nil},
		{Thread.OutputRollout, Thread.Contract.Result != nil},
		{DraftCreate.OutputRollout, DraftCreate.Contract.Result != nil},
		{DraftEdit.OutputRollout, DraftEdit.Contract.Result != nil},
		{TemplateCreate.OutputRollout, TemplateCreate.Contract.Result != nil},
		{TemplateUpdate.OutputRollout, TemplateUpdate.Contract.Result != nil},
	}
	for index, declaration := range declarations {
		if declaration.rollout != output.RolloutUnifiedActive || !declaration.result {
			t.Fatalf("mail declaration %d is not unified with Result", index)
		}
	}
}

func TestCrossPlatformCoverageMailWritesRequireConfirmation(t *testing.T) {
	for _, declaration := range []struct {
		name         string
		confirmation string
	}{
		{DraftCreate.Command, DraftCreate.Safety.Confirmation},
		{DraftEdit.Command, DraftEdit.Safety.Confirmation},
		{TemplateCreate.Command, TemplateCreate.Safety.Confirmation},
		{TemplateUpdate.Command, TemplateUpdate.Safety.Confirmation},
	} {
		if declaration.confirmation != "user_required" {
			t.Errorf("%s confirmation=%q, want user_required", declaration.name, declaration.confirmation)
		}
	}
}

func TestCrossPlatformCoverageMailStrictResponseMatrix(t *testing.T) {
	validEmpty := map[string]any{"success": "true", "folders": []any{}}
	items, err := mailRequireCollection(validEmpty, "mail/list_folders", "folders")
	if err != nil || len(items) != 0 {
		t.Fatalf("explicit empty collection must succeed: items=%v err=%v", items, err)
	}
	for name, fixture := range map[string]map[string]any{
		"missing success":    {"folders": []any{}},
		"missing collection": {"success": "true"},
		"wrong collection":   {"success": true, "folders": map[string]any{}},
		"bad item":           {"success": "true", "folders": []any{"bad"}},
	} {
		if _, err := mailRequireCollection(fixture, "mail/list_folders", "folders"); err == nil {
			t.Fatalf("%s must fail closed", name)
		}
	}
	if err := mailRequireSuccess(map[string]any{"success": "false"}, "mail/test"); err == nil {
		t.Fatal("string false must not be success")
	}
	if err := mailRequireSuccess(map[string]any{"success": true}, "mail/test"); err != nil {
		t.Fatalf("boolean true must be accepted: %v", err)
	}
}

func TestCrossPlatformCoverageMailPaginationAndIdentity(t *testing.T) {
	complete, next, err := mailPage(map[string]any{"nextCursor": "$"}, "mail/search", "")
	if err != nil || !complete || next != "" {
		t.Fatalf("terminal dollar cursor mismatch: complete=%v next=%q err=%v", complete, next, err)
	}
	complete, next, err = mailPage(map[string]any{"hasMore": "false", "nextCursor": ""}, "mail/list", "")
	if err != nil || !complete || next != "" {
		t.Fatalf("string false pagination mismatch: complete=%v next=%q err=%v", complete, next, err)
	}
	if _, _, err := mailPage(map[string]any{"hasMore": true, "nextCursor": ""}, "mail/list", ""); err == nil {
		t.Fatal("hasMore without a progressing cursor must fail")
	}
	if err := mailRequireIdentity(map[string]any{"id": "actual"}, "mail/get", "expected", "id"); err == nil {
		t.Fatal("identity mismatch must fail")
	}
}
