// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0

package oa

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

type oaCoverageCaller struct {
	responses map[string][]string
	history   []string
	arguments []map[string]any
}

func (caller *oaCoverageCaller) CallTool(_ context.Context, _, tool string, arguments map[string]any) (*edition.ToolResult, error) {
	caller.history = append(caller.history, tool)
	caller.arguments = append(caller.arguments, arguments)
	queue := caller.responses[tool]
	if len(queue) == 0 {
		return nil, errors.New("missing OA fake response for " + tool)
	}
	caller.responses[tool] = queue[1:]
	if queue[0] == "__ERROR__" {
		return nil, errors.New("injected OA failure")
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: queue[0]}}}, nil
}

func (*oaCoverageCaller) Format() string { return "json" }
func (*oaCoverageCaller) DryRun() bool   { return false }
func (*oaCoverageCaller) Fields() string { return "" }
func (*oaCoverageCaller) JQ() string     { return "" }

func runOACoverage(t *testing.T, declaration shortcut.Shortcut, caller *oaCoverageCaller, args ...string) (*cobra.Command, error) {
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

func TestCrossPlatformCoverageOAContractsAreTypedAndUnified(t *testing.T) {
	for _, declaration := range []shortcut.Shortcut{
		ListPending, ListForms, SearchForms, ListExecuted, ListSubmitted,
		ListCc, PendingApprovals, DoneApprovals, Approve, MyInitiated,
	} {
		if declaration.Contract.Empty() || declaration.Contract.Result == nil {
			t.Errorf("%s lacks Contract.Result", declaration.Command)
		}
		if declaration.Safety.Effect == "" || declaration.Safety.Confirmation == "" {
			t.Errorf("%s lacks Safety", declaration.Command)
		}
		if declaration.OutputRollout != output.RolloutUnifiedActive {
			t.Errorf("%s rollout=%q", declaration.Command, declaration.OutputRollout)
		}
	}
	if Approve.Safety.Confirmation != "user_required" || Approve.Risk != shortcut.RiskHighWrite {
		t.Fatal("+approve-by must require explicit high-risk confirmation")
	}
	for _, declaration := range []shortcut.Shortcut{ListPending, ListForms, ListExecuted, ListSubmitted, ListCc, MyInitiated} {
		if declaration.Contract.Pagination == nil {
			t.Errorf("%s lacks Pagination", declaration.Command)
		}
	}
}

func TestCrossPlatformCoverageOAStrictResponseMatrix(t *testing.T) {
	validEmpty := map[string]any{"success": "true", "result": map[string]any{"values": []any{}}}
	items, err := oaProjectInstances(validEmpty, "oa/test", "result.values")
	if err != nil || len(items) != 0 {
		t.Fatalf("explicit empty must succeed: items=%v err=%v", items, err)
	}
	for name, fixture := range map[string]map[string]any{
		"empty":              {},
		"missing success":    {"result": map[string]any{"values": []any{}}},
		"false success":      {"success": false, "result": map[string]any{"values": []any{}}},
		"missing collection": {"success": true, "result": map[string]any{}},
		"wrong collection":   {"success": true, "result": map[string]any{"values": map[string]any{}}},
		"bad item":           {"success": true, "result": map[string]any{"values": []any{"bad"}}},
		"missing identity":   {"success": true, "result": map[string]any{"values": []any{map[string]any{"title": "fixture"}}}},
	} {
		if projected, err := oaProjectInstances(fixture, "oa/test", "result.values"); err == nil {
			t.Errorf("%s returned success: %#v", name, projected)
		}
	}
	forms := map[string]any{"success": true, "result": map[string]any{"processCodeList": []any{map[string]any{"processCode": "p", "processName": "fixture"}}}}
	if projected, err := oaProjectForms(forms, "oa/forms", "result.processCodeList"); err != nil || len(projected) != 1 {
		t.Fatalf("strict forms projection=%#v err=%v", projected, err)
	}
	badForm := map[string]any{"success": true, "result": map[string]any{"processCodeList": []any{map[string]any{"processName": "fixture"}}}}
	if projected, err := oaProjectForms(badForm, "oa/forms", "result.processCodeList"); err == nil {
		t.Fatalf("form without processCode returned success: %#v", projected)
	}
	search := map[string]any{"success": true, "result": []any{map[string]any{"processCode": "p", "processName": "fixture"}}}
	if projected, err := oaProjectForms(search, "oa/search", "result"); err != nil || len(projected) != 1 {
		t.Fatalf("strict search projection=%#v err=%v", projected, err)
	}
}

func TestCrossPlatformCoverageOAPaginationFailsClosed(t *testing.T) {
	if _, err := oaHasMorePage(map[string]any{}, "oa/page", 1); err == nil {
		t.Fatal("numbered page without hasMore was accepted")
	}
	if _, err := oaHasMorePage(map[string]any{"hasMore": "false"}, "oa/page", 1); err == nil {
		t.Fatal("wrong hasMore type was accepted")
	}
	page, err := oaHasMorePage(map[string]any{"hasMore": true}, "oa/page", 2)
	if err != nil || !page.HasMore || page.Next != "3" {
		t.Fatalf("numbered continuation=%+v err=%v", page, err)
	}
	if _, err := oaCursorPage(map[string]any{}, "oa/cursor", 0); err == nil {
		t.Fatal("cursor response without pagination was accepted")
	}
	if _, err := oaCursorPage(map[string]any{"hasMore": true}, "oa/cursor", 0); err == nil {
		t.Fatal("hasMore without nextCursor was accepted")
	}
	if _, err := oaCursorPage(map[string]any{"hasMore": true, "nextCursor": float64(1)}, "oa/cursor", 1); err == nil {
		t.Fatal("stalled cursor was accepted")
	}
	page, err = oaCursorPage(map[string]any{"hasMore": true, "nextCursor": float64(2)}, "oa/cursor", 1)
	if err != nil || page.Next != "2" {
		t.Fatalf("cursor continuation=%+v err=%v", page, err)
	}
}

func TestCrossPlatformCoverageMyInitiatedNeverFallsBackToRawResponse(t *testing.T) {
	caller := &oaCoverageCaller{responses: map[string][]string{
		"get_submitted_instances": {`{"success":"true","result":{"values":[],"hasMore":false}}`},
	}}
	cmd, err := runOACoverage(t, MyInitiated, caller, "--page", "1", "--limit", "20")
	if err != nil {
		t.Fatal(err)
	}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	code, emitted, err := output.EmitStoredResult(cmd)
	if err != nil || !emitted || code != 0 {
		t.Fatalf("emit code=%d emitted=%v err=%v", code, emitted, err)
	}
	var envelope struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatal(err)
	}
	if _, raw := envelope.Data["result"]; raw {
		t.Fatalf("raw result leaked: %s", stdout.String())
	}
	initiated, ok := envelope.Data["initiated"].([]any)
	if !ok || len(initiated) != 0 || envelope.Data["complete"] != true {
		t.Fatalf("strict empty initiated payload=%#v", envelope.Data)
	}
}

func TestCrossPlatformCoverageOAApproveConfirmationAndReadback(t *testing.T) {
	unconfirmed := &oaCoverageCaller{responses: map[string][]string{}}
	helpers.InitDepsForTest(t, unconfirmed)
	root := &cobra.Command{Use: "dws", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().Bool("yes", false, "")
	root.PersistentFlags().Bool("dry-run", false, "")
	root.PersistentFlags().String("format", "json", "")
	ctx, _ := output.WithResultStore(context.Background())
	root.SetContext(ctx)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	service := &cobra.Command{Use: "oa"}
	service.AddCommand(corecmd.New(shortcut.FromShortcut(Approve)))
	root.AddCommand(service)
	root.SetArgs([]string{"oa", "+approve-by", "--keyword", "fixture"})
	if err := root.Execute(); err == nil {
		t.Fatal("unconfirmed approval unexpectedly succeeded")
	}
	if len(unconfirmed.history) != 0 {
		t.Fatalf("unconfirmed approval made calls: %v", unconfirmed.history)
	}

	confirmed := &oaCoverageCaller{responses: map[string][]string{
		"list_pending_approvals": {`{"success":true,"result":{"hasMore":false,"values":[{"processInstanceId":"instance-1","title":"fixture"}]}}`},
		"list_pending_tasks": {
			`{"success":true,"result":{"taskIdList":[{"taskId":1}]}}`,
			`{"success":true,"result":{"taskIdList":[]}}`,
		},
		"approve_processInstance": {`{"success":true,"result":{"accepted":true}}`},
	}}
	helpers.InitDepsForTest(t, confirmed)
	confirmedRoot := &cobra.Command{Use: "dws", SilenceUsage: true, SilenceErrors: true}
	confirmedRoot.PersistentFlags().Bool("yes", false, "")
	confirmedRoot.PersistentFlags().Bool("dry-run", false, "")
	confirmedRoot.PersistentFlags().String("format", "json", "")
	confirmedCtx, _ := output.WithResultStore(context.Background())
	confirmedRoot.SetContext(confirmedCtx)
	confirmedRoot.SetOut(io.Discard)
	confirmedRoot.SetErr(io.Discard)
	confirmedService := &cobra.Command{Use: "oa"}
	confirmedService.AddCommand(corecmd.New(shortcut.FromShortcut(Approve)))
	confirmedRoot.AddCommand(confirmedService)
	confirmedRoot.SetArgs([]string{"oa", "+approve-by", "--keyword", "fixture", "--yes"})
	if err := confirmedRoot.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(confirmed.history, ","); got != "list_pending_approvals,list_pending_tasks,approve_processInstance,list_pending_tasks" {
		t.Fatalf("call history=%s", got)
	}
	if len(confirmed.arguments) != 4 || confirmed.arguments[2]["processInstanceId"] != "instance-1" || confirmed.arguments[3]["processInstanceId"] != "instance-1" {
		t.Fatalf("identity was not preserved: %#v", confirmed.arguments)
	}
}
