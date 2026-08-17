// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package smart

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/helpers"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/output"
	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type calendarSmartTestStep struct {
	text string
	err  error
}

type calendarSmartTestCall struct {
	key  string
	args map[string]any
}

type calendarSmartTestCaller struct {
	steps  map[string][]calendarSmartTestStep
	counts map[string]int
	calls  []calendarSmartTestCall
}

func (c *calendarSmartTestCaller) CallTool(_ context.Context, product, tool string, args map[string]any) (*edition.ToolResult, error) {
	if c.counts == nil {
		c.counts = map[string]int{}
	}
	key := product + "/" + tool
	c.counts[key]++
	c.calls = append(c.calls, calendarSmartTestCall{key: key, args: args})
	steps := c.steps[key]
	index := c.counts[key] - 1
	if index >= len(steps) {
		return nil, errors.New("unexpected fixture call: " + key)
	}
	step := steps[index]
	if step.err != nil {
		return nil, step.err
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: step.text}}}, nil
}

func (c *calendarSmartTestCaller) CallReadTool(ctx context.Context, product, tool string, args map[string]any) (*edition.ToolResult, error) {
	return c.CallTool(ctx, product, tool, args)
}

func (*calendarSmartTestCaller) Format() string { return "json" }
func (*calendarSmartTestCaller) DryRun() bool   { return false }
func (*calendarSmartTestCaller) Fields() string { return "" }
func (*calendarSmartTestCaller) JQ() string     { return "" }

func runCalendarSmartCLI(t *testing.T, caller edition.ToolCaller, args ...string) (map[string]any, string, error) {
	t.Helper()
	helpers.InitDepsForTest(t, caller)
	root := newPlatformCoverageRoot()
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(io.Discard)
	root.SetArgs(args)
	executed, err := root.ExecuteC()
	if err == nil {
		if _, _, emitErr := output.EmitStoredResult(executed); emitErr != nil {
			err = emitErr
		}
	}
	if stdout.Len() == 0 {
		return nil, "", err
	}
	var payload map[string]any
	if decodeErr := json.Unmarshal(stdout.Bytes(), &payload); decodeErr != nil {
		t.Fatalf("calendar output is not exactly one JSON value: %q: %v", stdout.String(), decodeErr)
	}
	if data, ok := payload["data"].(map[string]any); ok {
		return data, stdout.String(), err
	}
	return payload, stdout.String(), err
}

func calendarSmartTestEvent(id string) map[string]any {
	return map[string]any{
		"id":      id,
		"summary": "fixture event",
		"start":   map[string]any{"dateTime": "2026-08-17T09:00:00+08:00"},
		"end":     map[string]any{"dateTime": "2026-08-17T10:00:00+08:00"},
	}
}

func TestCrossPlatformCoverageCalendarSmartResultContracts(t *testing.T) {
	shortcuts := []struct {
		name    string
		rollout output.RolloutState
		result  bool
	}{
		{"today", Today.OutputRollout, Today.Contract.Result != nil},
		{"tomorrow", Tomorrow.OutputRollout, Tomorrow.Contract.Result != nil},
		{"week", Week.OutputRollout, Week.Contract.Result != nil},
		{"next-event", NextEvent.OutputRollout, NextEvent.Contract.Result != nil},
		{"conflicts", Conflicts.OutputRollout, Conflicts.Contract.Result != nil},
		{"free-slots", FreeSlots.OutputRollout, FreeSlots.Contract.Result != nil},
		{"suggest-time", SuggestTime.OutputRollout, SuggestTime.Contract.Result != nil},
		{"book", Book.OutputRollout, Book.Contract.Result != nil},
		{"cancel-event", CancelEvent.OutputRollout, CancelEvent.Contract.Result != nil},
		{"invite", Invite.OutputRollout, Invite.Contract.Result != nil},
		{"reschedule", Reschedule.OutputRollout, Reschedule.Contract.Result != nil},
		{"free", FreeBusy.OutputRollout, FreeBusy.Contract.Result != nil},
		{"my-free", MyFree.OutputRollout, MyFree.Contract.Result != nil},
	}
	for _, item := range shortcuts {
		if item.rollout != output.RolloutUnifiedActive || !item.result {
			t.Errorf("%s rollout=%q result=%v", item.name, item.rollout, item.result)
		}
	}
}

func TestCrossPlatformCoverageCalendarSmartStrictCollections(t *testing.T) {
	valid := calendarSmartTestEvent("event-placeholder")
	items, hasMore, next, err := calendarSmartEventPage(map[string]any{
		"success": true,
		"result": map[string]any{
			"events":     []any{valid},
			"hasMore":    true,
			"nextCursor": "cursor-placeholder",
		},
	})
	if err != nil || len(items) != 1 || !hasMore || next != "cursor-placeholder" {
		t.Fatalf("valid page items=%#v hasMore=%v next=%q err=%v", items, hasMore, next, err)
	}
	items, hasMore, next, err = calendarSmartEventPage(map[string]any{
		"success": true,
		"result":  map[string]any{"events": []any{}, "hasMore": false},
	})
	if err != nil || items == nil || len(items) != 0 || hasMore || next != "" {
		t.Fatalf("explicit empty page items=%#v hasMore=%v next=%q err=%v", items, hasMore, next, err)
	}

	invalidPages := []map[string]any{
		{},
		{"success": false, "result": map[string]any{"events": []any{}, "hasMore": false}},
		{"success": true, "result": map[string]any{"hasMore": false}},
		{"success": true, "result": map[string]any{"events": map[string]any{}, "hasMore": false}},
		{"success": true, "result": map[string]any{"events": []any{}, "hasMore": true}},
		{"success": true, "result": map[string]any{"events": []any{}, "hasMore": false, "nextCursor": "unexpected"}},
		{"success": true, "result": map[string]any{"events": []any{map[string]any{"id": "event-placeholder"}}, "hasMore": false}},
		{"success": true, "result": map[string]any{"events": []any{"bad-item"}, "hasMore": false}},
	}
	for index, page := range invalidPages {
		if _, _, _, err := calendarSmartEventPage(page); err == nil {
			t.Errorf("invalid page %d accepted: %#v", index, page)
		}
	}

	allDay := map[string]any{
		"id":    "all-day-placeholder",
		"start": map[string]any{"date": "2026-08-17"},
		"end":   map[string]any{"date": "2026-08-18"},
	}
	if _, _, _, err := calendarSmartEventPage(map[string]any{
		"success": true,
		"result":  map[string]any{"events": []any{allDay}, "hasMore": false},
	}); err != nil {
		t.Fatalf("legitimate all-day event rejected: %v", err)
	}

	if slots, err := calendarSmartBusySlots(map[string]any{"success": true, "result": []any{}}); err != nil || slots == nil || len(slots) != 0 {
		t.Fatalf("explicit empty busy slots=%#v err=%v", slots, err)
	}
	if _, err := calendarSmartBusySlots(map[string]any{"success": true, "result": []any{map[string]any{}}}); err == nil {
		t.Fatal("busy result item without scheduleItems accepted")
	}
	if slots, err := calendarSmartSuggestedSlots(map[string]any{"success": true, "result": map[string]any{"recommendEventTimes": []any{}}}); err != nil || slots == nil || len(slots) != 0 {
		t.Fatalf("explicit empty suggestions=%#v err=%v", slots, err)
	}
	if _, err := calendarSmartSuggestedSlots(map[string]any{"success": true, "result": map[string]any{}}); err == nil {
		t.Fatal("missing recommendEventTimes accepted")
	}
	if _, err := calendarSmartSuggestedSlots(map[string]any{"success": true, "result": map[string]any{"recommendEventTimes": []any{map[string]any{"startTime": "x"}}}}); err == nil {
		t.Fatal("suggestion without endTime accepted")
	}
}

func TestCrossPlatformCoverageCalendarSmartPaginationIsCompleteOrFails(t *testing.T) {
	eventJSON := `{"id":"event-placeholder","summary":"fixture","start":{"dateTime":"2026-08-17T09:00:00+08:00"},"end":{"dateTime":"2026-08-17T10:00:00+08:00"}}`
	caller := &calendarSmartTestCaller{steps: map[string][]calendarSmartTestStep{
		"calendar/list_calendar_events": {
			{text: `{"success":true,"result":{"events":[` + eventJSON + `],"hasMore":true,"nextCursor":"cursor-placeholder"}}`},
			{text: `{"success":true,"result":{"events":[],"hasMore":false}}`},
		},
	}}
	payload, _, err := runCalendarSmartCLI(t, caller, "calendar", "+today")
	if err != nil || payload["complete"] != true || payload["count"] != float64(1) {
		t.Fatalf("today payload=%#v err=%v", payload, err)
	}
	if caller.counts["calendar/list_calendar_events"] != 2 {
		t.Fatalf("list calls=%#v", caller.counts)
	}
	if got := caller.calls[1].args["cursor"]; got != "cursor-placeholder" {
		t.Fatalf("second-page cursor=%#v calls=%#v", got, caller.calls)
	}

	for name, responses := range map[string][]calendarSmartTestStep{
		"missing pagination": {{text: `{"success":true,"result":{"events":[]}}`}},
		"stalled cursor": {
			{text: `{"success":true,"result":{"events":[],"hasMore":true,"nextCursor":"cursor-placeholder"}}`},
			{text: `{"success":true,"result":{"events":[],"hasMore":true,"nextCursor":"cursor-placeholder"}}`},
		},
	} {
		t.Run(name, func(t *testing.T) {
			bad := &calendarSmartTestCaller{steps: map[string][]calendarSmartTestStep{"calendar/list_calendar_events": responses}}
			payload, outputText, err := runCalendarSmartCLI(t, bad, "calendar", "+today")
			if err == nil || payload != nil || outputText != "" {
				t.Fatalf("incomplete list accepted payload=%#v output=%q err=%v", payload, outputText, err)
			}
		})
	}
}

func TestCrossPlatformCoverageCalendarSmartWriteReceiptsAndReadback(t *testing.T) {
	const start = "2026-08-17T09:00:00+08:00"
	const end = "2026-08-17T10:00:00+08:00"
	const eventID = "event-placeholder"

	book := &calendarSmartTestCaller{steps: map[string][]calendarSmartTestStep{
		"calendar/create_calendar_event": {{text: `{"success":true,"result":{"id":"` + eventID + `"}}`}},
		"calendar/get_calendar_detail":   {{text: `{"success":true,"result":{"id":"` + eventID + `","summary":"fixture title","start":{"dateTime":"` + start + `"},"end":{"dateTime":"` + end + `"}}}`}},
	}}
	payload, _, err := runCalendarSmartCLI(t, book, "calendar", "+book", "--title", "fixture title", "--start", start, "--end", end, "--yes")
	if err != nil || payload["success"] != true || payload["verified"] != true || payload["eventId"] != eventID {
		t.Fatalf("book payload=%#v err=%v", payload, err)
	}

	unknownWrite := &calendarSmartTestCaller{steps: map[string][]calendarSmartTestStep{
		"calendar/create_calendar_event": {{text: `{"result":{"id":"` + eventID + `"}}`}},
	}}
	payload, outputText, err := runCalendarSmartCLI(t, unknownWrite, "calendar", "+book", "--title", "fixture title", "--start", start, "--end", end, "--yes")
	if err == nil || payload != nil || outputText != "" || unknownWrite.counts["calendar/get_calendar_detail"] != 0 {
		t.Fatalf("unknown create receipt accepted payload=%#v output=%q err=%v counts=%#v", payload, outputText, err, unknownWrite.counts)
	}

	reschedule := &calendarSmartTestCaller{steps: map[string][]calendarSmartTestStep{
		"calendar/get_calendar_detail": {
			{text: `{"success":true,"result":{"id":"` + eventID + `"}}`},
			{text: `{"success":true,"result":{"id":"` + eventID + `","start":{"dateTime":"` + start + `"},"end":{"dateTime":"` + end + `"}}}`},
		},
		"calendar/update_calendar_event": {{text: `{"success":true}`}},
	}}
	payload, _, err = runCalendarSmartCLI(t, reschedule, "calendar", "+reschedule", "--event", eventID, "--start", start, "--end", end, "--yes")
	if err != nil || payload["success"] != true || payload["verified"] != true {
		t.Fatalf("reschedule payload=%#v err=%v", payload, err)
	}
	updates := reschedule.calls
	var updateArgs map[string]any
	for _, call := range updates {
		if call.key == "calendar/update_calendar_event" {
			updateArgs = call.args
		}
	}
	if updateArgs["eventId"] != eventID || updateArgs["startDateTime"] != start || updateArgs["endDateTime"] != end {
		t.Fatalf("reschedule request=%#v", updateArgs)
	}

	invite := &calendarSmartTestCaller{steps: map[string][]calendarSmartTestStep{
		"contact/search_contact_by_key_word": {{text: `{"result":[{"userId":"user-placeholder","name":"fixture person"}]}`}},
		"calendar/get_calendar_detail":       {{text: `{"success":true,"result":{"id":"` + eventID + `"}}`}},
		"calendar/add_calendar_participant":  {{text: `{"success":true}`}},
		"calendar/get_calendar_participants": {{text: `{"success":true,"result":[{"userId":"user-placeholder"}]}`}},
	}}
	payload, _, err = runCalendarSmartCLI(t, invite, "calendar", "+invite", "--event", eventID, "--with", "fixture person", "--yes")
	if err != nil || payload["success"] != true || payload["verified"] != true || payload["invitedCount"] != float64(1) {
		t.Fatalf("invite payload=%#v err=%v", payload, err)
	}
}

func TestCrossPlatformCoverageCalendarSmartDeleteAndRollbackVerifyAbsence(t *testing.T) {
	const eventID = "event-placeholder"
	const sensitiveTitle = "fixture-sensitive-title"
	cancel := &calendarSmartTestCaller{steps: map[string][]calendarSmartTestStep{
		"calendar/get_calendar_detail": {
			{text: `{"success":true,"result":{"id":"` + eventID + `","summary":"` + sensitiveTitle + `"}}`},
			{err: errors.New("event not found")},
		},
		"calendar/delete_calendar_event": {{text: `{"success":true}`}},
	}}
	payload, outputText, err := runCalendarSmartCLI(t, cancel, "calendar", "+cancel-event", "--event", eventID, "--yes")
	if err != nil || payload["success"] != true || payload["deleted"] != true || payload["verified"] != true {
		t.Fatalf("cancel payload=%#v err=%v", payload, err)
	}
	if strings.Contains(outputText, sensitiveTitle) {
		t.Fatalf("pre-delete PII leaked into output: %q", outputText)
	}
	if cancel.counts["calendar/get_calendar_detail"] != 2 {
		t.Fatalf("cancel did not verify absence: %#v", cancel.counts)
	}

	tombstone := &calendarSmartTestCaller{steps: map[string][]calendarSmartTestStep{
		"calendar/get_calendar_detail": {
			{text: `{"success":true,"result":{"id":"` + eventID + `"}}`},
			{text: `{"success":true,"result":{"id":"` + eventID + `","status":"cancelled"}}`},
		},
		"calendar/delete_calendar_event": {{text: `{"success":true}`}},
	}}
	payload, _, err = runCalendarSmartCLI(t, tombstone, "calendar", "+cancel-event", "--event", eventID, "--yes")
	if err != nil || payload["success"] != true || payload["verified"] != true {
		t.Fatalf("cancel tombstone payload=%#v err=%v", payload, err)
	}

	unknownReadback := &calendarSmartTestCaller{steps: map[string][]calendarSmartTestStep{
		"calendar/get_calendar_detail": {
			{text: `{"success":true,"result":{"id":"` + eventID + `"}}`},
			{err: errors.New("temporary readback failure")},
		},
		"calendar/delete_calendar_event": {{text: `{"success":true}`}},
	}}
	payload, outputText, err = runCalendarSmartCLI(t, unknownReadback, "calendar", "+cancel-event", "--event", eventID, "--yes")
	if err == nil || payload != nil || outputText != "" {
		t.Fatalf("unknown delete readback accepted payload=%#v output=%q err=%v", payload, outputText, err)
	}

	rollback := &calendarSmartTestCaller{steps: map[string][]calendarSmartTestStep{
		"contact/search_contact_by_key_word": {{text: `{"result":[{"userId":"user-placeholder","name":"fixture person"}]}`}},
		"calendar/create_calendar_event":     {{text: `{"success":true,"result":{"id":"` + eventID + `"}}`}},
		"calendar/add_calendar_participant":  {{text: `{"success":false,"message":"fixture rejection"}`}},
		"calendar/delete_calendar_event":     {{text: `{"success":true}`}},
		"calendar/get_calendar_detail":       {{err: errors.New("event not found")}},
	}}
	payload, outputText, err = runCalendarSmartCLI(t, rollback, "calendar", "+book", "--title", "fixture title", "--start", "2026-08-17T09:00:00+08:00", "--end", "2026-08-17T10:00:00+08:00", "--with", "fixture person", "--yes")
	if err == nil || payload != nil || outputText != "" || rollback.counts["calendar/delete_calendar_event"] != 1 || rollback.counts["calendar/get_calendar_detail"] != 1 {
		t.Fatalf("rollback was not verified payload=%#v output=%q err=%v counts=%#v", payload, outputText, err, rollback.counts)
	}
}
