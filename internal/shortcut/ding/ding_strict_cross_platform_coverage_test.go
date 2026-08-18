// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package ding

import (
	"bytes"
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

type dingCoverageCaller struct {
	responses map[string][]string
	history   []string
}

func (caller *dingCoverageCaller) CallTool(_ context.Context, _, tool string, _ map[string]any) (*edition.ToolResult, error) {
	caller.history = append(caller.history, tool)
	queue := caller.responses[tool]
	if len(queue) == 0 {
		return nil, errors.New("missing DING fake response for " + tool)
	}
	caller.responses[tool] = queue[1:]
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: queue[0]}}}, nil
}

func (*dingCoverageCaller) Format() string { return "json" }
func (*dingCoverageCaller) DryRun() bool   { return false }
func (*dingCoverageCaller) Fields() string { return "" }
func (*dingCoverageCaller) JQ() string     { return "" }

func runDingCoverage(t *testing.T, declaration shortcut.Shortcut, caller *dingCoverageCaller, args ...string) (*cobra.Command, error) {
	t.Helper()
	helpers.InitDepsForTest(t, caller)
	cmd := corecmd.New(shortcut.FromShortcut(declaration))
	ctx, _ := output.WithResultStore(context.Background())
	cmd.SetContext(ctx)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	return cmd, cmd.Execute()
}

func TestCrossPlatformCoverageDINGContractsAreStrictTypedAndUnified(t *testing.T) {
	for _, declaration := range []shortcut.Shortcut{List, ReceiverStatus, SendPersonal, SendByMessage, RecallPersonal} {
		if declaration.Contract.Empty() {
			t.Errorf("%s lacks Contract", declaration.Command)
		}
		if declaration.Safety.Effect == "" || declaration.Safety.Confirmation == "" {
			t.Errorf("%s lacks Safety", declaration.Command)
		}
		if declaration.OutputRollout != output.RolloutUnifiedActive {
			t.Errorf("%s rollout=%q", declaration.Command, declaration.OutputRollout)
		}
	}
	for _, declaration := range []shortcut.Shortcut{List, ReceiverStatus} {
		if declaration.Contract.Result == nil {
			t.Errorf("%s lacks Result", declaration.Command)
		}
		if declaration.Contract.Interface == nil || declaration.Contract.Interface.Availability != "available" {
			t.Errorf("%s is not interface-available", declaration.Command)
		}
	}
	for _, declaration := range []shortcut.Shortcut{SendPersonal, SendByMessage, RecallPersonal} {
		if declaration.Safety.Confirmation != "user_required" || declaration.Contract.Interface == nil || declaration.Contract.Interface.Availability != "unavailable" {
			t.Errorf("%s write safety/interface drift", declaration.Command)
		}
	}
	if List.Contract.Pagination == nil {
		t.Fatal("+list lacks cursor pagination")
	}
}

func TestCrossPlatformCoverageDINGListResponseMatrix(t *testing.T) {
	valid := map[string]any{
		"success": true,
		"result": map[string]any{
			"dingMessages": []any{map[string]any{"openDingId": "ding-1", "dingContent": "fixture"}},
			"hasMore":      true,
			"nextCursor":   float64(2),
		},
	}
	messages, page, err := dingProjectMessages(valid, "im/list_ding_messages")
	if err != nil || len(messages) != 1 || !page.HasMore || page.Next != "2" || messages[0]["content"] != "fixture" {
		t.Fatalf("valid list projection messages=%#v page=%+v err=%v", messages, page, err)
	}
	empty := map[string]any{"success": true, "result": map[string]any{"dingMessages": []any{}, "hasMore": false, "nextCursor": nil}}
	if messages, page, err := dingProjectMessages(empty, "im/list_ding_messages"); err != nil || len(messages) != 0 || page.HasMore {
		t.Fatalf("explicit terminal empty page messages=%#v page=%+v err=%v", messages, page, err)
	}
	fixtures := map[string]map[string]any{
		"empty":                  {},
		"missing success":        {"result": map[string]any{"dingMessages": []any{}, "hasMore": false}},
		"wrong success":          {"success": "true", "result": map[string]any{"dingMessages": []any{}, "hasMore": false}},
		"false success":          {"success": false, "result": map[string]any{"dingMessages": []any{}, "hasMore": false}},
		"missing result":         {"success": true},
		"missing collection":     {"success": true, "result": map[string]any{"hasMore": false}},
		"wrong collection":       {"success": true, "result": map[string]any{"dingMessages": map[string]any{}, "hasMore": false}},
		"bad item":               {"success": true, "result": map[string]any{"dingMessages": []any{"bad"}, "hasMore": false}},
		"missing item id":        {"success": true, "result": map[string]any{"dingMessages": []any{map[string]any{"dingContent": "fixture"}}, "hasMore": false}},
		"duplicate item id":      {"success": true, "result": map[string]any{"dingMessages": []any{map[string]any{"openDingId": "same"}, map[string]any{"openDingId": "same"}}, "hasMore": false}},
		"missing pagination":     {"success": true, "result": map[string]any{"dingMessages": []any{}}},
		"wrong pagination":       {"success": true, "result": map[string]any{"dingMessages": []any{}, "hasMore": "false"}},
		"empty continuation":     {"success": true, "result": map[string]any{"dingMessages": []any{}, "hasMore": true, "nextCursor": float64(2)}},
		"missing next cursor":    {"success": true, "result": map[string]any{"dingMessages": []any{map[string]any{"openDingId": "ding-1"}}, "hasMore": true}},
		"wrong next cursor":      {"success": true, "result": map[string]any{"dingMessages": []any{map[string]any{"openDingId": "ding-1"}}, "hasMore": true, "nextCursor": "2"}},
		"conflicting terminal":   {"success": true, "result": map[string]any{"dingMessages": []any{}, "hasMore": false, "nextCursor": float64(2)}},
		"wrong optional content": {"success": true, "result": map[string]any{"dingMessages": []any{map[string]any{"openDingId": "ding-1", "dingContent": float64(1)}}, "hasMore": false}},
	}
	for name, fixture := range fixtures {
		if projected, _, projectErr := dingProjectMessages(fixture, "im/list_ding_messages"); projectErr == nil {
			t.Errorf("%s returned success: %#v", name, projected)
		}
	}
}

func TestCrossPlatformCoverageDINGReceiverResponseMatrix(t *testing.T) {
	valid := map[string]any{"success": true, "result": map[string]any{"receivers": []any{map[string]any{"openDingId": "ding-1", "confirmedStatus": float64(1), "receiverNick": "fixture"}}}}
	items, err := dingProjectReceivers(valid, "im/list_ding_receiver_status", "ding-1")
	if err != nil || len(items) != 1 || items[0]["confirmedStatus"] != int64(1) {
		t.Fatalf("valid receiver projection=%#v err=%v", items, err)
	}
	for name, fixture := range map[string]map[string]any{
		"empty":              {},
		"missing collection": {"success": true, "result": map[string]any{}},
		"empty collection":   {"success": true, "result": map[string]any{"receivers": []any{}}},
		"bad item":           {"success": true, "result": map[string]any{"receivers": []any{"bad"}}},
		"identity mismatch":  {"success": true, "result": map[string]any{"receivers": []any{map[string]any{"openDingId": "other", "confirmedStatus": float64(1), "receiverNick": "fixture"}}}},
		"wrong status":       {"success": true, "result": map[string]any{"receivers": []any{map[string]any{"openDingId": "ding-1", "confirmedStatus": "1", "receiverNick": "fixture"}}}},
		"missing receiver":   {"success": true, "result": map[string]any{"receivers": []any{map[string]any{"openDingId": "ding-1", "confirmedStatus": float64(1)}}}},
	} {
		if projected, projectErr := dingProjectReceivers(fixture, "im/list_ding_receiver_status", "ding-1"); projectErr == nil {
			t.Errorf("%s returned success: %#v", name, projected)
		}
	}
}

func TestCrossPlatformCoverageDINGExactReadShortcutsProjectUnifiedData(t *testing.T) {
	listCaller := &dingCoverageCaller{responses: map[string][]string{
		"list_ding_messages": {`{"success":true,"result":{"dingMessages":[{"openDingId":"ding-1","dingContent":"fixture"}],"hasMore":false,"nextCursor":null}}`},
	}}
	cmd, err := runDingCoverage(t, List, listCaller, "--type", "ALL")
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if code, emitted, emitErr := output.EmitStoredResult(cmd); emitErr != nil || !emitted || code != 0 {
		t.Fatalf("emit=(%d,%t,%v)", code, emitted, emitErr)
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	data := envelope["data"].(map[string]any)
	if _, leaked := data["result"]; leaked || data["count"] != float64(1) || data["complete"] != true {
		t.Fatalf("list unified projection=%#v", data)
	}
	if strings.Join(listCaller.history, ",") != "list_ding_messages" {
		t.Fatalf("list call history=%v", listCaller.history)
	}

	receiverCaller := &dingCoverageCaller{responses: map[string][]string{
		"list_ding_receiver_status": {`{"success":true,"result":{"receivers":[{"openDingId":"ding-1","confirmedStatus":1,"receiverNick":"fixture"}]}}`},
	}}
	if _, err := runDingCoverage(t, ReceiverStatus, receiverCaller, "--ding-id", "ding-1"); err != nil {
		t.Fatal(err)
	}
	if strings.Join(receiverCaller.history, ",") != "list_ding_receiver_status" {
		t.Fatalf("receiver call history=%v", receiverCaller.history)
	}
}

func TestCrossPlatformCoverageDINGUnavailableWritesNeverCallMCP(t *testing.T) {
	cases := []struct {
		declaration shortcut.Shortcut
		args        []string
	}{
		{SendPersonal, []string{"--users", "D-fixture", "--content", "fixture"}},
		{SendByMessage, []string{"--group", "cid-fixture", "--message-id", "mid-fixture", "--users", "D-fixture"}},
		{RecallPersonal, []string{"--id", "ding-fixture"}},
	}
	for _, test := range cases {
		t.Run(test.declaration.Command, func(t *testing.T) {
			unconfirmed := &dingCoverageCaller{responses: map[string][]string{}}
			err := runDingRoot(t, test.declaration, unconfirmed, false, test.args...)
			if err == nil || len(unconfirmed.history) != 0 {
				t.Fatalf("unconfirmed error=%v calls=%v", err, unconfirmed.history)
			}
			confirmed := &dingCoverageCaller{responses: map[string][]string{}}
			err = runDingRoot(t, test.declaration, confirmed, true, test.args...)
			if err == nil || !strings.Contains(err.Error(), "当前不可执行") || len(confirmed.history) != 0 {
				t.Fatalf("confirmed unavailable error=%v calls=%v", err, confirmed.history)
			}
		})
	}
}

func runDingRoot(t *testing.T, declaration shortcut.Shortcut, caller *dingCoverageCaller, yes bool, args ...string) error {
	t.Helper()
	helpers.InitDepsForTest(t, caller)
	root := &cobra.Command{Use: "dws", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().Bool("yes", false, "")
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().String("format", "json", "")
	ctx, _ := output.WithResultStore(context.Background())
	root.SetContext(ctx)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	service := &cobra.Command{Use: "ding"}
	service.AddCommand(corecmd.New(shortcut.FromShortcut(declaration)))
	root.AddCommand(service)
	argv := []string{"ding", declaration.Command}
	argv = append(argv, args...)
	if yes {
		argv = append(argv, "--yes")
	}
	root.SetArgs(argv)
	return root.Execute()
}
