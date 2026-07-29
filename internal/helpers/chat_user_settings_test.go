package helpers

import (
	"strings"
	"testing"
)

func TestCrossPlatformCoverageChatGroupUserSettingsQueryValidation(t *testing.T) {
	caller := &guardedMutationCaller{}
	if err := executeGuardedMutationCommand(t, caller, newChatCommand,
		"group", "user-settings", "query", "--groups", ","); err == nil {
		t.Fatal("empty groups returned nil")
	}

	tooMany := make([]string, 101)
	for i := range tooMany {
		tooMany[i] = "cid"
	}
	if err := executeGuardedMutationCommand(t, caller, newChatCommand,
		"group", "user-settings", "query", "--groups", strings.Join(tooMany, ",")); err == nil {
		t.Fatal("101 groups returned nil")
	}
}

func TestCrossPlatformCoverageChatGroupUserSettingsSetValidation(t *testing.T) {
	caller := &guardedMutationCaller{}
	if err := executeGuardedMutationCommand(t, caller, newChatCommand,
		"group", "user-settings", "set", "--items", "[]"); err == nil {
		t.Fatal("empty items returned nil")
	}

	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 101; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"openConversationId":"cid"}`)
	}
	sb.WriteString("]")
	if err := executeGuardedMutationCommand(t, caller, newChatCommand,
		"group", "user-settings", "set", "--items", sb.String()); err == nil {
		t.Fatal("101 items returned nil")
	}
}

func TestCrossPlatformCoverageChatGroupUserSettingsSetHappyPath(t *testing.T) {
	caller := &guardedMutationCaller{}
	err := executeGuardedMutationCommand(t, caller, newChatCommand,
		"group", "user-settings", "set", "--items", `[{"openConversationId":"cid1","top":true}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("calls = %#v", caller.calls)
	}
	call := caller.calls[0]
	if call.productID != "im" || call.toolName != "batch_update_group_chat_settings" {
		t.Fatalf("call = %#v", call)
	}
}
