// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package helpers

import (
	"errors"
	"strings"
	"testing"

	apperrors "github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/errors"
)

func TestCrossPlatformCoverageChatMessageWritesRequireConfirmationBeforeDispatch(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "current user send", args: []string{"message", "send", "--group", "cid-1", "--text", "hello"}},
		{name: "bot send", args: []string{"message", "send-by-bot", "--robot-code", "robot-1", "--group", "cid-1", "--title", "title", "--text", "hello"}},
		{name: "webhook send", args: []string{"message", "send-by-webhook", "--token", "token-1", "--title", "title", "--text", "hello"}},
		{name: "card send", args: []string{"message", "send-card", "--group", "cid-1"}},
		{name: "card update", args: []string{"message", "update-card", "--biz-id", "biz-1", "--content", "done", "--flow-status", "3"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			caller := &scriptedToolCaller{}
			InitDepsForTest(t, caller)
			cmd := newChatCommand()
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetIn(strings.NewReader(""))
			cmd.SetArgs(test.args)
			err := cmd.Execute()

			var appErr *apperrors.Error
			if !errors.As(err, &appErr) || appErr.Reason != "confirmation_required" {
				t.Fatalf("Execute() error = %#v, want confirmation_required", err)
			}
			if caller.calls != 0 {
				t.Fatalf("downstream calls = %d, want 0 before confirmation", caller.calls)
			}
		})
	}
}

func TestCrossPlatformCoverageChatMessageWriteLeavesInstallRuntimeConfirmation(t *testing.T) {
	chat := newChatCommand()
	for _, path := range [][]string{
		{"message", "send"},
		{"message", "send-by-bot"},
		{"message", "send-by-webhook"},
		{"message", "send-card"},
		{"message", "update-card"},
	} {
		leaf, _, err := chat.Find(path)
		if err != nil || leaf == nil {
			t.Fatalf("Find(%v) = %v, %v", path, leaf, err)
		}
		if !HasContractConfirmSafety(leaf) {
			t.Errorf("%s has no contract confirmation runtime guard", strings.Join(path, " "))
		}
	}
}
