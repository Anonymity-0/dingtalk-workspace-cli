package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type chatMessagePageAllCaller struct {
	steps []scriptedToolStep
	dry   bool
	calls []pagedCommandCall
}

func (c *chatMessagePageAllCaller) CallTool(_ context.Context, serverID, toolName string, args map[string]any) (*edition.ToolResult, error) {
	copied := map[string]any{}
	for k, v := range args {
		copied[k] = v
	}
	c.calls = append(c.calls, pagedCommandCall{server: serverID, tool: toolName, args: copied})
	if len(c.steps) == 0 {
		return textToolResult(`{"result":{"messages":[],"hasMore":false}}`), nil
	}
	index := len(c.calls) - 1
	if index >= len(c.steps) {
		index = len(c.steps) - 1
	}
	step := c.steps[index]
	if step.err != nil {
		return nil, step.err
	}
	return textToolResult(step.text), nil
}

func (*chatMessagePageAllCaller) Format() string { return "json" }
func (c *chatMessagePageAllCaller) DryRun() bool { return c.dry }
func (*chatMessagePageAllCaller) Fields() string { return "" }
func (*chatMessagePageAllCaller) JQ() string     { return "" }

func executeChatMessagePageAllCommand(t *testing.T, caller *chatMessagePageAllCaller, args ...string) (map[string]any, error) {
	t.Helper()
	oldDeps := deps
	t.Cleanup(func() { deps = oldDeps })
	InitDeps(caller)
	out := &bytes.Buffer{}
	deps.Out.w = out
	deps.Out.errW = io.Discard

	root := newChatCommand()
	root.SilenceErrors = true
	root.SilenceUsage = true
	root.SetOut(out)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	err := root.ExecuteContext(context.Background())
	if out.Len() == 0 {
		return nil, err
	}
	var parsed map[string]any
	if unmarshalErr := json.Unmarshal(out.Bytes(), &parsed); unmarshalErr != nil {
		t.Fatalf("stdout JSON = %q, err = %v", out.String(), unmarshalErr)
	}
	return parsed, err
}

func pageAllMessages(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	rows, ok := payload["messages"].([]any)
	if !ok {
		t.Fatalf("messages = %#v, want array", payload["messages"])
	}
	messages := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		message, ok := row.(map[string]any)
		if !ok {
			t.Fatalf("message row = %#v, want object", row)
		}
		messages = append(messages, message)
	}
	return messages
}

func pageAllMessageIDs(t *testing.T, payload map[string]any) []string {
	t.Helper()
	ids := make([]string, 0)
	for _, message := range pageAllMessages(t, payload) {
		ids = append(ids, fmt.Sprint(message["messageId"]))
	}
	return ids
}

func TestCrossPlatformCoverageChatMessageListPageAllSinglePageUnchanged(t *testing.T) {
	response := `{"result":{"messages":[{"openMessageId":"m1","content":"hello"}],"hasMore":false}}`
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "no paging flags",
			args: []string{"message", "list", "--conversation-id", "cidAAAAAAAAAA1", "--time", "2025-03-01 00:00:00", "--direction", "older"},
		},
		{
			name: "paging flags without page-all stay single page",
			args: []string{"message", "list", "--conversation-id", "cidAAAAAAAAAA1", "--time", "2025-03-01 00:00:00", "--direction", "older", "--page-limit", "20", "--max-items", "5", "--page-delay", "0"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			caller := &chatMessagePageAllCaller{steps: []scriptedToolStep{{text: response}}}
			got, err := executeChatMessagePageAllCommand(t, caller, tt.args...)
			if err != nil {
				t.Fatal(err)
			}
			if len(caller.calls) != 1 {
				t.Fatalf("calls = %d, want exactly one single-page call", len(caller.calls))
			}
			call := caller.calls[0]
			if call.server != "chat" || call.tool != "list_conversation_message_v2" {
				t.Fatalf("call = %s/%s, want chat/list_conversation_message_v2", call.server, call.tool)
			}
			if call.args["openconversation_id"] != "cidAAAAAAAAAA1" || call.args["time"] != "2025-03-01 00:00:00" || call.args["forward"] != false {
				t.Fatalf("args = %#v", call.args)
			}
			if _, exists := call.args["page-all"]; exists {
				t.Fatalf("single-page request leaked paging args: %#v", call.args)
			}
			for _, key := range []string{"stopReason", "pagesFetched", "truncatedByPageLimit", "truncatedByResultLimit", "paging", "complete"} {
				if _, exists := got[key]; exists {
					t.Fatalf("single-page output gained aggregate field %q: %#v", key, got[key])
				}
			}
			ids := pageAllMessageIDs(t, got)
			if len(ids) != 1 || ids[0] != "m1" {
				t.Fatalf("messages = %#v, want [m1]", ids)
			}
		})
	}
}

func TestCrossPlatformCoverageChatMessageListPageAllAggregatesAndDedups(t *testing.T) {
	steps := []scriptedToolStep{
		{text: `{"result":{"messages":[{"openMessageId":"m1"},{"openMessageId":"m2"}],"hasMore":true,"nextCursor":1787000000123}}`},
		{text: `{"result":{"messages":[{"openMessageId":"m2"},{"openMessageId":"m3"}],"hasMore":false}}`},
	}
	caller := &chatMessagePageAllCaller{steps: steps}
	got, err := executeChatMessagePageAllCommand(t, caller,
		"message", "list", "--conversation-id", "cidAAAAAAAAAA1", "--time", "2025-03-01 00:00:00", "--direction", "older", "--page-all", "--page-delay", "0")
	if err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("calls = %d, want two pages", len(caller.calls))
	}
	first := caller.calls[0]
	if first.server != "chat" || first.tool != "list_conversation_message_v2" {
		t.Fatalf("first call = %s/%s", first.server, first.tool)
	}
	if first.args["openconversation_id"] != "cidAAAAAAAAAA1" || first.args["time"] != "2025-03-01 00:00:00" || first.args["forward"] != false {
		t.Fatalf("first args = %#v", first.args)
	}
	wantBoundary := time.UnixMilli(1787000000123).UTC().Format(time.RFC3339Nano)
	if caller.calls[1].args["time"] != wantBoundary {
		t.Fatalf("second call time = %#v, want %q", caller.calls[1].args["time"], wantBoundary)
	}
	if caller.calls[1].args["openconversation_id"] != "cidAAAAAAAAAA1" || caller.calls[1].args["forward"] != false {
		t.Fatalf("second args = %#v", caller.calls[1].args)
	}
	ids := pageAllMessageIDs(t, got)
	if len(ids) != 3 || ids[0] != "m1" || ids[1] != "m2" || ids[2] != "m3" {
		t.Fatalf("messages = %#v, want [m1 m2 m3] with m2 deduped", ids)
	}
	if got["count"].(float64) != 3 || got["pagesFetched"].(float64) != 2 {
		t.Fatalf("count/pagesFetched = %#v/%#v, want 3/2", got["count"], got["pagesFetched"])
	}
	if got["complete"] != true || got["hasMore"] != false || got["stopReason"] != "source_complete" {
		t.Fatalf("complete/hasMore/stopReason = %#v/%#v/%#v", got["complete"], got["hasMore"], got["stopReason"])
	}
	if got["truncatedByPageLimit"] != false || got["truncated"] != false || got["failedCount"].(float64) != 0 {
		t.Fatalf("truncation fields = %#v/%#v/%#v", got["truncatedByPageLimit"], got["truncated"], got["failedCount"])
	}
	if _, exists := got["nextPage"]; exists {
		t.Fatalf("nextPage = %#v, want absent when source complete", got["nextPage"])
	}
}

func TestCrossPlatformCoverageChatMessageListPageAllPageLimitStops(t *testing.T) {
	caller := &chatMessagePageAllCaller{steps: []scriptedToolStep{
		{text: `{"result":{"messages":[{"openMessageId":"m1"}],"hasMore":true,"nextCursor":1787000000123}}`},
	}}
	got, err := executeChatMessagePageAllCommand(t, caller,
		"message", "list", "--conversation-id", "cidAAAAAAAAAA1", "--time", "2025-03-01 00:00:00", "--direction", "older", "--page-all", "--page-limit", "1", "--page-delay", "0")
	if err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("calls = %d, want one page under --page-limit 1", len(caller.calls))
	}
	if got["stopReason"] != "page_limit" || got["truncatedByPageLimit"] != true {
		t.Fatalf("stopReason/truncatedByPageLimit = %#v/%#v", got["stopReason"], got["truncatedByPageLimit"])
	}
	if got["complete"] != false || got["hasMore"] != true {
		t.Fatalf("complete/hasMore = %#v/%#v", got["complete"], got["hasMore"])
	}
	nextPage, ok := got["nextPage"].(map[string]any)
	if !ok || nextPage["time"] != time.UnixMilli(1787000000123).UTC().Format(time.RFC3339Nano) {
		t.Fatalf("nextPage = %#v, want boundary time", got["nextPage"])
	}
	if nextPage["direction"] != "older" {
		t.Fatalf("nextPage.direction = %#v, want older", nextPage["direction"])
	}
}

func TestCrossPlatformCoverageChatMessageListPageAllMaxItemsTruncates(t *testing.T) {
	t.Run("overshoot within one page", func(t *testing.T) {
		rows := make([]string, 0, 50)
		for i := 1; i <= 50; i++ {
			rows = append(rows, fmt.Sprintf(`{"openMessageId":"m%d"}`, i))
		}
		response := `{"result":{"messages":[` + joinCommas(rows) + `],"hasMore":false}}`
		caller := &chatMessagePageAllCaller{steps: []scriptedToolStep{{text: response}}}
		got, err := executeChatMessagePageAllCommand(t, caller,
			"message", "list", "--conversation-id", "cidAAAAAAAAAA1", "--time", "2025-03-01 00:00:00", "--direction", "older", "--page-all", "--max-items", "30", "--page-delay", "0")
		if err != nil {
			t.Fatal(err)
		}
		if len(caller.calls) != 1 {
			t.Fatalf("calls = %d, want one page", len(caller.calls))
		}
		ids := pageAllMessageIDs(t, got)
		if len(ids) != 30 || ids[29] != "m30" {
			t.Fatalf("messages = %d rows, last = %v, want 30 rows ending m30", len(ids), lastOr(ids))
		}
		if got["truncated"] != true || got["truncatedByResultLimit"] != true {
			t.Fatalf("truncated/truncatedByResultLimit = %#v/%#v", got["truncated"], got["truncatedByResultLimit"])
		}
		if got["hasMore"] != true || got["complete"] != false {
			t.Fatalf("hasMore/complete = %#v/%#v, want true/false for truncated output", got["hasMore"], got["complete"])
		}
	})
	t.Run("accumulated across pages stops at limit", func(t *testing.T) {
		page := func(id string) string {
			return fmt.Sprintf(`{"result":{"messages":[{"openMessageId":%q}],"hasMore":true,"nextCursor":%d}}`, id, len(id)+1700000000000)
		}
		caller := &chatMessagePageAllCaller{steps: []scriptedToolStep{{text: page("m1")}, {text: page("m2")}}}
		got, err := executeChatMessagePageAllCommand(t, caller,
			"message", "list", "--conversation-id", "cidAAAAAAAAAA1", "--time", "2025-03-01 00:00:00", "--direction", "older", "--page-all", "--max-items", "1", "--page-delay", "0")
		if err != nil {
			t.Fatal(err)
		}
		if len(caller.calls) != 1 {
			t.Fatalf("calls = %d, want sweep to stop after reaching --max-items", len(caller.calls))
		}
		ids := pageAllMessageIDs(t, got)
		if len(ids) != 1 || ids[0] != "m1" {
			t.Fatalf("messages = %#v, want [m1]", ids)
		}
		if got["truncatedByResultLimit"] != true || got["stopReason"] != "result_limit" {
			t.Fatalf("truncatedByResultLimit/stopReason = %#v/%#v", got["truncatedByResultLimit"], got["stopReason"])
		}
		if got["hasMore"] != true || got["complete"] != false {
			t.Fatalf("hasMore/complete = %#v/%#v", got["hasMore"], got["complete"])
		}
	})
}

func TestCrossPlatformCoverageChatMessageListPageAllDirectionNewer(t *testing.T) {
	caller := &chatMessagePageAllCaller{steps: []scriptedToolStep{
		{text: `{"result":{"messages":[{"openMessageId":"m1"}],"hasMore":true,"nextCursor":1787000000100}}`},
		{text: `{"result":{"messages":[{"openMessageId":"m2"}],"hasMore":true,"nextCursor":1787000000200}}`},
	}}
	got, err := executeChatMessagePageAllCommand(t, caller,
		"message", "list", "--conversation-id", "cidAAAAAAAAAA1", "--time", "2025-03-01 00:00:00", "--direction", "newer", "--page-all", "--page-limit", "2", "--page-delay", "0")
	if err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("calls = %d, want two pages", len(caller.calls))
	}
	if caller.calls[0].args["forward"] != true || caller.calls[1].args["forward"] != true {
		t.Fatalf("forward args = %#v / %#v, want true", caller.calls[0].args["forward"], caller.calls[1].args["forward"])
	}
	if got["stopReason"] != "page_limit" || got["truncatedByPageLimit"] != true {
		t.Fatalf("stopReason/truncatedByPageLimit = %#v/%#v", got["stopReason"], got["truncatedByPageLimit"])
	}
	nextPage, ok := got["nextPage"].(map[string]any)
	if !ok {
		t.Fatalf("nextPage = %#v, want present", got["nextPage"])
	}
	if nextPage["direction"] != "newer" {
		t.Fatalf("nextPage.direction = %#v, want newer", nextPage["direction"])
	}
	if nextPage["time"] != time.UnixMilli(1787000000200).UTC().Format(time.RFC3339Nano) {
		t.Fatalf("nextPage.time = %#v, want boundary of the last page cursor", nextPage["time"])
	}
}

func TestCrossPlatformCoverageChatMessageListPageAllIndividualChatPath(t *testing.T) {
	caller := &chatMessagePageAllCaller{steps: []scriptedToolStep{
		{text: `{"result":{"messages":[{"openMessageId":"m1"}],"hasMore":true,"nextCursor":1787000000123}}`},
		{text: `{"result":{"messages":[{"openMessageId":"m2"}],"hasMore":false}}`},
	}}
	got, err := executeChatMessagePageAllCommand(t, caller,
		"message", "list", "--user", "u1", "--time", "2025-03-01 00:00:00", "--direction", "older", "--page-all", "--page-delay", "0")
	if err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 2 {
		t.Fatalf("calls = %d, want two pages", len(caller.calls))
	}
	for _, call := range caller.calls {
		if call.server != "chat" || call.tool != "list_individual_chat_message" {
			t.Fatalf("call = %s/%s, want chat/list_individual_chat_message", call.server, call.tool)
		}
		if call.args["userId"] != "u1" {
			t.Fatalf("args = %#v, want userId=u1", call.args)
		}
		if _, exists := call.args["openconversation_id"]; exists {
			t.Fatalf("individual-chat args leaked group id: %#v", call.args)
		}
	}
	ids := pageAllMessageIDs(t, got)
	if len(ids) != 2 || ids[0] != "m1" || ids[1] != "m2" {
		t.Fatalf("messages = %#v, want [m1 m2]", ids)
	}
	if got["pagesFetched"].(float64) != 2 || got["stopReason"] != "source_complete" || got["complete"] != true {
		t.Fatalf("pagesFetched/stopReason/complete = %#v/%#v/%#v", got["pagesFetched"], got["stopReason"], got["complete"])
	}
}

func TestCrossPlatformCoverageChatMessageListPageAllCursorStallStops(t *testing.T) {
	page := `{"result":{"messages":[{"openMessageId":"m1"}],"hasMore":true,"nextCursor":1787000000123}}`
	caller := &chatMessagePageAllCaller{steps: []scriptedToolStep{{text: page}, {text: page}}}
	got, err := executeChatMessagePageAllCommand(t, caller,
		"message", "list", "--conversation-id", "cidAAAAAAAAAA1", "--time", "2025-03-01 00:00:00", "--direction", "older", "--page-all", "--page-delay", "0")
	if err == nil {
		t.Fatal("expected non-zero exit for cursor stall, got nil")
	}
	if len(caller.calls) != 2 {
		t.Fatalf("calls = %d, want exactly two pages before stall detection (not page-limit iterations)", len(caller.calls))
	}
	if got["stopReason"] != "pagination_error" {
		t.Fatalf("stopReason = %#v, want pagination_error", got["stopReason"])
	}
	if got["failedCount"].(float64) != 1 {
		t.Fatalf("failedCount = %#v, want 1", got["failedCount"])
	}
	failures, ok := got["failures"].([]any)
	if !ok || len(failures) != 1 {
		t.Fatalf("failures = %#v, want one diagnostic", got["failures"])
	}
	if got["partial"] != true {
		t.Fatalf("partial = %#v, want true", got["partial"])
	}
	ids := pageAllMessageIDs(t, got)
	if len(ids) != 1 || ids[0] != "m1" {
		t.Fatalf("messages = %#v, want deduped [m1] from the repeated page", ids)
	}
}

func TestCrossPlatformCoverageChatMessageListPageAllMidSweepFailurePartial(t *testing.T) {
	caller := &chatMessagePageAllCaller{steps: []scriptedToolStep{
		{text: `{"result":{"messages":[{"openMessageId":"m1"}],"hasMore":true,"nextCursor":1787000000123}}`},
		{err: fmt.Errorf("gateway timeout")},
	}}
	got, err := executeChatMessagePageAllCommand(t, caller,
		"message", "list", "--conversation-id", "cidAAAAAAAAAA1", "--time", "2025-03-01 00:00:00", "--direction", "older", "--page-all", "--page-delay", "0")
	if err == nil {
		t.Fatal("expected non-zero exit for mid-sweep failure, got nil")
	}
	if got["stopReason"] != "read_failure" || got["partial"] != true || got["failedCount"].(float64) != 1 {
		t.Fatalf("stopReason/partial/failedCount = %#v/%#v/%#v", got["stopReason"], got["partial"], got["failedCount"])
	}
	if got["pagesFetched"].(float64) != 1 {
		t.Fatalf("pagesFetched = %#v, want 1", got["pagesFetched"])
	}
	ids := pageAllMessageIDs(t, got)
	if len(ids) != 1 || ids[0] != "m1" {
		t.Fatalf("messages = %#v, want page-one rows preserved", ids)
	}
}

func TestCrossPlatformCoverageChatMessageListPageAllDryRun(t *testing.T) {
	caller := &chatMessagePageAllCaller{dry: true}
	got, err := executeChatMessagePageAllCommand(t, caller,
		"message", "list", "--conversation-id", "cidAAAAAAAAAA1", "--time", "2025-03-01 00:00:00", "--direction", "older", "--page-all", "--page-limit", "10", "--max-items", "200", "--page-delay", "0")
	if err != nil {
		t.Fatal(err)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("calls = %d, want zero tool calls in dry-run", len(caller.calls))
	}
	if got["dry_run"] != true {
		t.Fatalf("dry_run = %#v, want true", got["dry_run"])
	}
	request, ok := got["request"].(map[string]any)
	if !ok {
		t.Fatalf("request = %#v, want object", got["request"])
	}
	if request["server"] != "chat" || request["name"] != "list_conversation_message_v2" {
		t.Fatalf("request = %#v", request)
	}
	args, ok := request["args"].(map[string]any)
	if !ok || args["openconversation_id"] != "cidAAAAAAAAAA1" || args["time"] != "2025-03-01 00:00:00" || args["forward"] != false {
		t.Fatalf("request args = %#v", request["args"])
	}
	paging, ok := got["paging"].(map[string]any)
	if !ok || paging["pageAll"] != true || paging["pageLimit"].(float64) != 10 || paging["maxItems"].(float64) != 200 {
		t.Fatalf("paging = %#v, want page-all sweep plan", got["paging"])
	}
}

func joinCommas(parts []string) string {
	out := ""
	for i, part := range parts {
		if i > 0 {
			out += ","
		}
		out += part
	}
	return out
}

func lastOr(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	return ids[len(ids)-1]
}
