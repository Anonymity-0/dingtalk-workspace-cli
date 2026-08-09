package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/spf13/cobra"
)

type pagedCommandCall struct {
	server string
	tool   string
	args   map[string]any
}

type pagedCommandCaller struct {
	steps  []scriptedToolStep
	calls  []pagedCommandCall
	format string
	dry    bool
}

func (c *pagedCommandCaller) CallTool(_ context.Context, serverID, toolName string, args map[string]any) (*edition.ToolResult, error) {
	copied := map[string]any{}
	for k, v := range args {
		copied[k] = v
	}
	c.calls = append(c.calls, pagedCommandCall{server: serverID, tool: toolName, args: copied})
	if len(c.steps) == 0 {
		return textToolResult(`{"result":{"messages":[],"hasMore":false}}`), nil
	}
	step := c.steps[len(c.calls)-1]
	if step.err != nil {
		return nil, step.err
	}
	return textToolResult(step.text), nil
}

func (c *pagedCommandCaller) Format() string { return c.format }
func (c *pagedCommandCaller) DryRun() bool   { return c.dry }
func (*pagedCommandCaller) Fields() string   { return "" }
func (*pagedCommandCaller) JQ() string       { return "" }

func runPagedCommandTest(t *testing.T, caller *pagedCommandCaller, cfg PagedMCPCommandConfig, args ...string) (map[string]any, string, error) {
	t.Helper()
	oldDeps := deps
	oldSleep := helperSleep
	t.Cleanup(func() {
		deps = oldDeps
		helperSleep = oldSleep
	})
	InitDeps(caller)
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	deps.Out.w = out
	deps.Out.errW = errOut
	helperSleep = func(d time.Duration) {}

	cmd := &cobra.Command{
		Use:          "paged",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return RunPagedMCPCommand(cmd, cfg)
		},
	}
	cmd.Flags().String("cursor", "0", "")
	AddPagedMCPFlags(cmd)
	cmd.SetErr(errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	if strings.TrimSpace(out.String()) == "" {
		return nil, errOut.String(), err
	}
	var parsed map[string]any
	if unmarshalErr := json.Unmarshal(out.Bytes(), &parsed); unmarshalErr != nil {
		t.Fatalf("stdout JSON = %q, err = %v", out.String(), unmarshalErr)
	}
	return parsed, errOut.String(), err
}

func pagedCommandMessagesConfig(fallback func(map[string]any) error) PagedMCPCommandConfig {
	if fallback == nil {
		fallback = func(map[string]any) error { return nil }
	}
	return PagedMCPCommandConfig{
		ServerID:    "chat",
		ToolName:    "search_messages_by_time_range",
		ItemPath:    "result.messages",
		CursorPath:  "result.nextCursor",
		HasMorePath: "result.hasMore",
		CursorArg:   "cursor",
		CursorKind:  PagedCursorString,
		BuildArgs: func(cmd *cobra.Command) (map[string]any, error) {
			cursor, _ := cmd.Flags().GetString("cursor")
			return map[string]any{"cursor": cursor, "limit": 2}, nil
		},
		Fallback: fallback,
	}
}

func TestPagedMCPCommandDefaultUsesFallbackOnly(t *testing.T) {
	caller := &pagedCommandCaller{}
	fallbackCalls := 0
	cfg := pagedCommandMessagesConfig(func(args map[string]any) error {
		fallbackCalls++
		if args["cursor"] != "0" {
			t.Fatalf("fallback args = %#v", args)
		}
		return nil
	})
	_, _, err := runPagedCommandTest(t, caller, cfg, "--page-limit", "2", "--max-items", "1", "--page-delay", "0")
	if err != nil {
		t.Fatal(err)
	}
	if fallbackCalls != 1 || len(caller.calls) != 0 {
		t.Fatalf("fallback=%d remote=%d, want fallback only", fallbackCalls, len(caller.calls))
	}
}

func TestPagedMCPCommandStringCursorAggregatesAndPageLimit(t *testing.T) {
	caller := &pagedCommandCaller{steps: []scriptedToolStep{
		{text: `{"result":{"messages":[{"id":"m1"}],"hasMore":true,"nextCursor":"c2"}}`},
		{text: `{"result":{"messages":[{"id":"m2"}],"hasMore":true,"nextCursor":"c3"}}`},
	}}
	got, _, err := runPagedCommandTest(t, caller, pagedCommandMessagesConfig(nil), "--page-all", "--page-limit", "2", "--page-delay", "0")
	if err != nil {
		t.Fatal(err)
	}
	items := got["result"].(map[string]any)["messages"].([]any)
	paging := got["paging"].(map[string]any)
	if len(items) != 2 || paging["truncated"] != true || paging["pages"].(float64) != 2 {
		t.Fatalf("result = %#v", got)
	}
	if caller.calls[0].args["cursor"] != "0" || caller.calls[1].args["cursor"] != "c2" {
		t.Fatalf("call args = %#v", caller.calls)
	}
}

func TestPagedMCPCommandMaxItemsTruncatesPrecisely(t *testing.T) {
	caller := &pagedCommandCaller{steps: []scriptedToolStep{
		{text: `{"result":{"messages":[{"id":"m1"},{"id":"m2"}],"hasMore":true,"nextCursor":"c2"}}`},
	}}
	got, _, err := runPagedCommandTest(t, caller, pagedCommandMessagesConfig(nil), "--page-all", "--max-items", "1", "--page-delay", "0")
	if err != nil {
		t.Fatal(err)
	}
	items := got["result"].(map[string]any)["messages"].([]any)
	paging := got["paging"].(map[string]any)
	if len(items) != 1 || paging["total"].(float64) != 1 || paging["truncated"] != true {
		t.Fatalf("result = %#v", got)
	}
}

func TestPagedMCPCommandInt64CursorAndItemsPath(t *testing.T) {
	caller := &pagedCommandCaller{steps: []scriptedToolStep{
		{text: `{"result":{"items":[{"id":"f1"}],"hasMore":true,"nextCursor":20}}`},
		{text: `{"result":{"items":[{"id":"f2"}],"hasMore":false,"nextCursor":0}}`},
	}}
	cfg := PagedMCPCommandConfig{
		ServerID:    "im",
		ToolName:    "list_message_favorites",
		ItemPath:    "result.items",
		CursorPath:  "result.nextCursor",
		HasMorePath: "result.hasMore",
		CursorArg:   "cursor",
		CursorKind:  PagedCursorInt64,
		BuildArgs: func(cmd *cobra.Command) (map[string]any, error) {
			return map[string]any{"cursor": int64(0), "size": "20"}, nil
		},
		Fallback: func(map[string]any) error { return nil },
	}
	got, _, err := runPagedCommandTest(t, caller, cfg, "--page-all", "--page-delay", "0")
	if err != nil {
		t.Fatal(err)
	}
	items := got["result"].(map[string]any)["items"].([]any)
	if len(items) != 2 || caller.calls[1].args["cursor"] != int64(20) {
		t.Fatalf("items=%#v calls=%#v", items, caller.calls)
	}
}

func TestPagedMCPCommandInt64CursorRejectsNonNumericNextCursor(t *testing.T) {
	caller := &pagedCommandCaller{steps: []scriptedToolStep{
		{text: `{"result":{"items":[{"id":"f1"}],"hasMore":true,"nextCursor":"not-a-number"}}`},
	}}
	cfg := PagedMCPCommandConfig{
		ServerID:    "im",
		ToolName:    "list_message_favorites",
		ItemPath:    "result.items",
		CursorPath:  "result.nextCursor",
		HasMorePath: "result.hasMore",
		CursorArg:   "cursor",
		CursorKind:  PagedCursorInt64,
		BuildArgs: func(cmd *cobra.Command) (map[string]any, error) {
			return map[string]any{"cursor": int64(0), "size": "20"}, nil
		},
		Fallback: func(map[string]any) error { return nil },
	}
	got, stderr, err := runPagedCommandTest(t, caller, cfg, "--page-all", "--page-delay", "0")
	if err == nil || !strings.Contains(err.Error(), "base-10 int64 string") {
		t.Fatalf("err=%v, want invalid int64 cursor error", err)
	}
	if !strings.Contains(stderr, "pagination stopped at page 2") {
		t.Fatalf("stderr=%q", stderr)
	}
	paging := got["paging"].(map[string]any)
	if paging["partial"] != true || paging["failedCursor"] != "not-a-number" || paging["pagesFetched"].(float64) != 1 {
		t.Fatalf("paging = %#v", paging)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("calls=%#v, want no second call with cursor 0", caller.calls)
	}
}

func TestPagedMCPCommandFirstPageFailureReturnsNoPartial(t *testing.T) {
	caller := &pagedCommandCaller{steps: []scriptedToolStep{{err: errors.New("boom")}}}
	got, _, err := runPagedCommandTest(t, caller, pagedCommandMessagesConfig(nil), "--page-all")
	if err == nil || got != nil {
		t.Fatalf("result=%#v err=%v, want first-page error without stdout", got, err)
	}
}

func TestPagedMCPCommandLaterFailureOutputsPartial(t *testing.T) {
	caller := &pagedCommandCaller{steps: []scriptedToolStep{
		{text: `{"result":{"messages":[{"id":"m1"}],"hasMore":true,"nextCursor":"c2"}}`},
		{err: errors.New("page failed")},
	}}
	got, stderr, err := runPagedCommandTest(t, caller, pagedCommandMessagesConfig(nil), "--page-all", "--page-delay", "0")
	if err == nil || !strings.Contains(stderr, "pagination stopped") {
		t.Fatalf("err=%v stderr=%q", err, stderr)
	}
	paging := got["paging"].(map[string]any)
	if paging["partial"] != true || paging["failedPage"].(float64) != 2 || paging["itemsFetched"].(float64) != 1 {
		t.Fatalf("paging = %#v", paging)
	}
}

func TestPagedMCPCommandCursorCycleOutputsPartial(t *testing.T) {
	caller := &pagedCommandCaller{steps: []scriptedToolStep{
		{text: `{"result":{"messages":[{"id":"m1"}],"hasMore":true,"nextCursor":"0"}}`},
	}}
	got, _, err := runPagedCommandTest(t, caller, pagedCommandMessagesConfig(nil), "--page-all", "--page-delay", "0")
	if err == nil {
		t.Fatal("cursor cycle should return error")
	}
	paging := got["paging"].(map[string]any)
	if paging["partial"] != true || paging["pagesFetched"].(float64) != 1 {
		t.Fatalf("paging = %#v", paging)
	}
}
