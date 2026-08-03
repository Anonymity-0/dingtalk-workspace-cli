package helpers

import (
	"io"
	"os"
	"testing"
)

func executeOACommand(t *testing.T, caller *scriptedToolCaller, args ...string) error {
	t.Helper()
	previous := deps
	previousArgs := os.Args
	os.Args = []string{"dws", "oa"}
	InitDeps(caller)
	deps.Out.w = io.Discard
	deps.Out.errW = io.Discard
	t.Cleanup(func() {
		deps = previous
		os.Args = previousArgs
	})

	cmd := newOaCommand()
	cmd.PersistentFlags().Bool("yes", false, "跳过确认")
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestCrossPlatformCoverageOARemainingTimeAndRevertBranches(t *testing.T) {
	installScriptedCaller(t, &scriptedToolCaller{dry: true})
	for _, args := range [][]string{
		{"approval", "list-pending", "--start", "bad", "--end", "2030-01-01T10:00:00+08:00"},
		{"approval", "list-pending", "--start", "2030-01-01T09:00:00+08:00", "--end", "bad"},
		{"approval", "list-pending", "--start", "2030-01-01T10:00:00+08:00", "--end", "2030-01-01T09:00:00+08:00"},
		{"approval", "list-initiated", "--process-code", "code", "--start", "bad", "--end", "2030-01-01T10:00:00+08:00"},
		{"approval", "list-initiated", "--process-code", "code", "--start", "2030-01-01T09:00:00+08:00", "--end", "bad"},
		{"approval", "list-initiated", "--process-code", "code", "--start", "2030-01-01T10:00:00+08:00", "--end", "2030-01-01T09:00:00+08:00"},
	} {
		if err := executeFilterCoverage(t, newOaCommand(), args...); err == nil {
			t.Fatalf("args=%v returned nil", args)
		}
	}

	if err := executeFilterCoverage(t, newOaCommand(),
		"approval", "list-pending",
		"--start", "2030-01-01T09:00:00+08:00", "--end", "2030-01-01T10:00:00+08:00",
		"--page", "2", "--size", "20", "--query", "travel",
	); err != nil {
		t.Fatalf("pending options: %v", err)
	}
	if err := executeFilterCoverage(t, newOaCommand(),
		"approval", "revert-task", "--instance-id", "instance", "--task-id", "12",
		"--target-activity-id", "activity", "--action", "REVERT_FOR_APPROVAL", "--remark", "retry",
	); err != nil {
		t.Fatalf("revert task: %v", err)
	}
}

func TestOAApprovalCreateInstanceMapsInternalSimpleOptions(t *testing.T) {
	caller := &scriptedToolCaller{}
	err := executeOACommand(t, caller,
		"approval", "create-instance",
		"--process-code", "PROC",
		"--form-values", `{"事由":"测试"}`,
		"--originator-user-id", "originator",
		"--approvers", "approver-1,approver-2",
		"--approvers-action-type", "AND",
		"--cc-list", "cc-1,cc-2",
		"--cc-position", "FINISH",
		"--yes",
	)
	if err != nil {
		t.Fatalf("create instance: %v", err)
	}
	if caller.server != "oa" || caller.tool != "start_process_instance" {
		t.Fatalf("called %s/%s, want oa/start_process_instance", caller.server, caller.tool)
	}
	request, ok := caller.args["ProcessInstanceCreationPopRequest"].(map[string]any)
	if !ok {
		t.Fatalf("request payload = %#v", caller.args)
	}
	if got := request["originatorUserId"]; got != "originator" {
		t.Fatalf("originatorUserId = %#v", got)
	}
	approvers, ok := request["approvers"].([]map[string]any)
	if !ok || len(approvers) != 1 || approvers[0]["actionType"] != "AND" {
		t.Fatalf("approvers = %#v", request["approvers"])
	}
	if got := approvers[0]["userIds"]; len(got.([]string)) != 2 || got.([]string)[0] != "approver-1" || got.([]string)[1] != "approver-2" {
		t.Fatalf("approver userIds = %#v", got)
	}
	if got := request["ccList"]; len(got.([]string)) != 2 || got.([]string)[0] != "cc-1" || got.([]string)[1] != "cc-2" {
		t.Fatalf("ccList = %#v", got)
	}
	if got := request["ccPosition"]; got != "FINISH" {
		t.Fatalf("ccPosition = %#v", got)
	}
}

func TestOAApprovalCreateInstanceRejectsMixedRequestModes(t *testing.T) {
	caller := &scriptedToolCaller{}
	err := executeOACommand(t, caller,
		"approval", "create-instance",
		"--request", `{"processCode":"PROC"}`,
		"--process-code", "PROC",
		"--yes",
	)
	if err == nil {
		t.Fatal("mixed request modes returned nil")
	}
	if caller.calls != 0 {
		t.Fatalf("unexpected MCP call count: %d", caller.calls)
	}
}
