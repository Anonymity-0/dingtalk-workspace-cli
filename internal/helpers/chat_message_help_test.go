// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"bytes"
	"strings"
	"testing"
)

func TestCrossPlatformCoverageChatMessageHelpDocumentsPostSendIDChain(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		contains   []string
		notContain string
	}{
		{
			name:    "send returns task ID",
			command: "send",
			contains: []string{
				"openTaskId",
				"query-send-status --open-task-id <openTaskId>",
			},
		},
		{
			name:    "query returns message and conversation IDs",
			command: "query-send-status",
			contains: []string{
				"openTaskId",
				"openMessageId",
				"openConversationId",
				"chat message edit",
				"chat message recall",
			},
		},
		{
			name:    "edit includes post-send workflow",
			command: "edit",
			contains: []string{
				"send -> query-send-status -> edit",
				"query-send-status --open-task-id <上一步返回的openTaskId>",
				"edit --conversation-id <上一步返回的openConversationId> --message-id <上一步返回的openMessageId>",
			},
			notContain: "chat message list",
		},
		{
			name:    "recall includes post-send workflow",
			command: "recall",
			contains: []string{
				"send -> query-send-status -> recall",
				"query-send-status --open-task-id <上一步返回的openTaskId>",
				"recall --conversation-id <上一步返回的openConversationId> --message-id <上一步返回的openMessageId>",
			},
			notContain: "chat message list",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cmd := newChatCommand()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"message", test.command, "--help"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("chat message %s --help: %v\n%s", test.command, err, output.String())
			}

			help := output.String()
			for _, want := range test.contains {
				if !strings.Contains(help, want) {
					t.Errorf("chat message %s help missing %q:\n%s", test.command, want, help)
				}
			}
			if test.notContain != "" && strings.Contains(help, test.notContain) {
				t.Errorf("chat message %s help still contains %q:\n%s", test.command, test.notContain, help)
			}
		})
	}
}

func TestCrossPlatformCoverageChatReactionHelpHidesConversationAliases(t *testing.T) {
	for _, command := range []string{"add-emoji", "remove-emoji", "add-text-emotion", "remove-text-emotion"} {
		t.Run(command, func(t *testing.T) {
			cmd := newChatCommand()
			var output bytes.Buffer
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SetArgs([]string{"message", command, "--help"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("chat message %s --help: %v\n%s", command, err, output.String())
			}

			help := output.String()
			if !strings.Contains(help, "--conversation-id") {
				t.Fatalf("chat message %s help missing --conversation-id:\n%s", command, help)
			}
			for _, hidden := range []string{"--group", "--id", "--chat"} {
				if strings.Contains(help, hidden+" string") {
					t.Fatalf("chat message %s help exposes hidden alias %s:\n%s", command, hidden, help)
				}
			}
		})
	}
}

func TestCrossPlatformCoverageChatGroupBotsHelpSplitsGroupName(t *testing.T) {
	cmd := newChatCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"group", "bots", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("chat group bots --help: %v\n%s", err, output.String())
	}

	help := output.String()
	for _, visible := range []string{"--conversation-id", "--group-name"} {
		if !strings.Contains(help, visible) {
			t.Fatalf("chat group bots help missing %s:\n%s", visible, help)
		}
	}
	if strings.Contains(help, "--group string") {
		t.Fatalf("chat group bots help exposes hidden --group alias:\n%s", help)
	}
}

func TestCrossPlatformCoverageChatSendCardHelpUsesCanonicalIDFlags(t *testing.T) {
	cmd := newChatCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"message", "send-card", "--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("chat message send-card --help: %v\n%s", err, output.String())
	}

	help := output.String()
	for _, visible := range []string{"--conversation-id", "--open-dingtalk-id"} {
		if !strings.Contains(help, visible) {
			t.Fatalf("send-card help missing %s:\n%s", visible, help)
		}
	}
	for _, hidden := range []string{"--group", "--receiver"} {
		if strings.Contains(help, hidden+" string") {
			t.Fatalf("send-card help exposes hidden alias %s:\n%s", hidden, help)
		}
	}
}
