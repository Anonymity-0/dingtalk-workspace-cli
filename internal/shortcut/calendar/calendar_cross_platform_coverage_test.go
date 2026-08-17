// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package calendar

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/shortcut"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
	"github.com/spf13/cobra"
)

type calendarCoverageCaller struct {
	responses map[string][]string
	history   []string
	arguments []map[string]any
}

func (caller *calendarCoverageCaller) CallTool(_ context.Context, _, tool string, arguments map[string]any) (*edition.ToolResult, error) {
	caller.history = append(caller.history, tool)
	caller.arguments = append(caller.arguments, arguments)
	queue := caller.responses[tool]
	if len(queue) == 0 {
		return nil, errors.New("missing fake response for " + tool)
	}
	caller.responses[tool] = queue[1:]
	if queue[0] == "__ERROR__" {
		return nil, errors.New("injected calendar failure")
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: queue[0]}}}, nil
}

func (*calendarCoverageCaller) Format() string { return "json" }
func (*calendarCoverageCaller) DryRun() bool   { return false }
func (*calendarCoverageCaller) Fields() string { return "" }
func (*calendarCoverageCaller) JQ() string     { return "" }

func runCalendarCoverage(t *testing.T, declaration shortcut.Shortcut, caller *calendarCoverageCaller, args ...string) error {
	t.Helper()
	helpers.InitDepsForTest(t, caller)
	declaration.OutputRollout = output.RolloutLegacyOnly
	root := &cobra.Command{Use: "dws", SilenceErrors: true, SilenceUsage: true}
	root.PersistentFlags().Bool("yes", false, "")
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().String("format", "json", "")
	service := &cobra.Command{Use: "calendar"}
	service.AddCommand(corecmd.New(shortcut.FromShortcut(declaration)))
	root.AddCommand(service)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"calendar", declaration.Command}, args...))
	return root.Execute()
}

func TestCrossPlatformCoverageCalendarListRequiresExplicitCollectionAndPagination(t *testing.T) {
	var explicitEmpty map[string]any
	if err := json.Unmarshal([]byte(`{"success":true,"result":{"events":[],"hasMore":false}}`), &explicitEmpty); err != nil {
		t.Fatal(err)
	}
	events, page, err := eventListProject(explicitEmpty)
	if err != nil || len(events) != 0 || !page.Known || page.HasMore {
		t.Fatalf("explicit empty: events=%v page=%+v err=%v", events, page, err)
	}

	for name, payload := range map[string]string{
		"missing collection":  `{"success":true,"result":{"hasMore":false}}`,
		"bad collection":      `{"success":true,"result":{"events":{}}}`,
		"bad item":            `{"success":true,"result":{"events":["bad"],"hasMore":false}}`,
		"empty item":          `{"success":true,"result":{"events":[{}],"hasMore":false}}`,
		"missing pagination":  `{"success":true,"result":{"events":[]}}`,
		"missing next cursor": `{"success":true,"result":{"events":[],"hasMore":true}}`,
	} {
		t.Run(name, func(t *testing.T) {
			var data map[string]any
			if err := json.Unmarshal([]byte(payload), &data); err != nil {
				t.Fatal(err)
			}
			if _, _, err := eventListProject(data); err == nil {
				t.Fatalf("payload unexpectedly accepted: %s", payload)
			}
		})
	}

	var nextPage map[string]any
	if err := json.Unmarshal([]byte(`{"success":true,"result":{"events":[{"id":"event-1","summary":"review"}],"hasMore":true,"nextCursor":"cursor-2"}}`), &nextPage); err != nil {
		t.Fatal(err)
	}
	events, page, err = eventListProject(nextPage)
	if err != nil || len(events) != 1 || events[0]["eventId"] != "event-1" || !page.HasMore || page.NextCursor != "cursor-2" {
		t.Fatalf("next page projection: events=%#v page=%+v err=%v", events, page, err)
	}
}

func TestCrossPlatformCoverageCalendarObjectsAndWritesFailClosed(t *testing.T) {
	for name, payload := range map[string]map[string]any{
		"empty":               {},
		"success false":       {"success": false, "errorMsg": "denied"},
		"nonterminal receipt": {"result": map[string]any{"id": "event-1"}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := requireCalendarWriteResponse(payload, "calendar/write"); err == nil {
				t.Fatalf("payload unexpectedly accepted: %#v", payload)
			}
		})
	}
	if _, err := requireCalendarWriteResponse(map[string]any{"success": true, "result": map[string]any{"id": "event-1"}}, "calendar/write"); err != nil {
		t.Fatalf("terminal write rejected: %v", err)
	}
	event, err := requireCalendarEvent(map[string]any{"success": true, "result": map[string]any{"id": "event-1", "summary": "review"}}, "calendar/get_calendar_detail", "event-1")
	if err != nil || normalizeCalendarEvent(event)["eventId"] != "event-1" {
		t.Fatalf("event normalization: event=%#v err=%v", event, err)
	}
	if _, err := requireCalendarEvent(map[string]any{"success": true, "result": map[string]any{"summary": "missing id"}}, "calendar/get_calendar_detail", "event-1"); err == nil {
		t.Fatal("event without id unexpectedly accepted")
	}
}

func TestCrossPlatformCoverageCalendarCreateReadsBackTerminalState(t *testing.T) {
	caller := &calendarCoverageCaller{responses: map[string][]string{
		"create_calendar_event": {`{"success":true,"result":{"id":"event-1"}}`},
		"get_calendar_detail":   {`{"success":true,"result":{"id":"event-1","summary":"review","start":{"dateTime":"2026-03-10T14:00:00+08:00"},"end":{"dateTime":"2026-03-10T15:00:00+08:00"}}}`},
	}}
	err := runCalendarCoverage(t, EventCreate, caller,
		"--title", "review", "--start", "2026-03-10T14:00:00+08:00", "--end", "2026-03-10T15:00:00+08:00", "--yes")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(caller.history, ","); got != "create_calendar_event,get_calendar_detail" {
		t.Fatalf("history=%s", got)
	}

	noReceipt := &calendarCoverageCaller{responses: map[string][]string{"create_calendar_event": {`{"result":{"id":"event-2"}}`}}}
	err = runCalendarCoverage(t, EventCreate, noReceipt,
		"--title", "review", "--start", "2026-03-10T14:00:00+08:00", "--end", "2026-03-10T15:00:00+08:00", "--yes")
	if err == nil || !strings.Contains(err.Error(), "success=true") {
		t.Fatalf("non-terminal create error=%v", err)
	}
}

func TestCrossPlatformCoverageCalendarAlignedContracts(t *testing.T) {
	for _, declaration := range []shortcut.Shortcut{EventGet, EventCreate, EventUpdate, RSVP, EventSearch, Suggestion, RoomFind} {
		if declaration.Contract.Empty() || declaration.Contract.Result == nil {
			t.Errorf("%s missing contract/result", declaration.Command)
		}
		if strings.TrimSpace(declaration.Safety.Effect) == "" || strings.TrimSpace(declaration.Safety.Confirmation) == "" {
			t.Errorf("%s missing safety", declaration.Command)
		}
		if declaration.OutputRollout != output.RolloutUnifiedActive {
			t.Errorf("%s output rollout=%q", declaration.Command, declaration.OutputRollout)
		}
	}
	if EventSearch.Contract.Pagination == nil || EventList.Contract.Pagination == nil {
		t.Fatal("cursor commands must publish pagination contracts")
	}
	if EventCreate.Safety.Confirmation != "user_required" || EventUpdate.Safety.Confirmation != "user_required" || RSVP.Safety.Confirmation != "user_required" {
		t.Fatal("calendar writes must require confirmation")
	}
}

func TestCrossPlatformCoverageCalendarRoomFindPreservesPublishedFlags(t *testing.T) {
	flags := make(map[string]shortcut.Flag, len(RoomFind.Flags))
	for _, flag := range RoomFind.Flags {
		flags[flag.Name] = flag
	}
	if flag := flags["available"]; flag.Type != shortcut.FlagBool || flag.Hidden {
		t.Fatalf("available flag=%+v, want visible bool", flag)
	}
	for _, name := range []string{"limit", "page"} {
		if flag := flags[name]; flag.Type != shortcut.FlagString || flag.Hidden {
			t.Fatalf("%s flag=%+v, want visible string", name, flag)
		}
	}

	caller := &calendarCoverageCaller{responses: map[string][]string{
		"query_available_meeting_room": {`{"success":true,"result":{"rooms":[],"hasMore":false}}`},
	}}
	err := runCalendarCoverage(t, RoomFind, caller,
		"--start", "2026-03-10T14:00:00+08:00",
		"--end", "2026-03-10T15:00:00+08:00",
		"--available", "--limit", "25", "--page", "2")
	if err != nil {
		t.Fatal(err)
	}
	if len(caller.arguments) != 1 {
		t.Fatalf("calls=%d, want 1", len(caller.arguments))
	}
	arguments := caller.arguments[0]
	if arguments["pageSize"] != "25" || arguments["pageIndex"] != "2" || arguments["needAvailable"] != true {
		t.Fatalf("arguments=%#v", arguments)
	}

	invalid := &calendarCoverageCaller{responses: map[string][]string{}}
	if err := runCalendarCoverage(t, RoomFind, invalid, "--limit", "101"); err == nil || !strings.Contains(err.Error(), "1-100") {
		t.Fatalf("invalid limit error=%v", err)
	}
	if len(invalid.history) != 0 {
		t.Fatalf("invalid input made calls: %v", invalid.history)
	}
}
