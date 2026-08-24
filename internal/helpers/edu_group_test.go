// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");

package helpers

import (
	"io"
	"strings"
	"testing"
)

// withEduGroupCaller installs a dry-run capture caller so happy-path command
// execution exercises each RunE up to the callMCPToolOnServer dispatch without
// requiring a live MCP transport.
func withEduGroupCaller(t *testing.T) *recruitCaptureCaller {
	t.Helper()
	caller := &recruitCaptureCaller{dryRun: true}
	InitDepsForTest(t, caller)
	deps.Out.w = io.Discard
	return caller
}

func TestEduGroupHappyPaths(t *testing.T) {
	cases := [][]string{
		{"student-group", "info", "--dept-id", "123"},
		{"student-group", "exists", "--dept-id", "123"},
		{"student-group", "members", "--dept-id", "123"},
		{"student-group", "is-in", "--dept-id", "123"},
		{"student-group", "conversation", "--dept-id", "123"},
		{"student-group", "create", "--dept-id", "123"},
		{"student-group", "disband", "--dept-id", "123"},
		{"class-group", "conversation-id", "--dept-id", "123"},
		{"class-group", "conversation", "--dept-id", "123"},
		{"class-group", "exists", "--dept-id", "123"},
		{"class-group", "list-by-cids", "--conversation-ids", "cid1,cid2"},
		{"batch", "check-student-group", "--class-ids", "1,2"},
		{"batch", "get-class-groups", "--class-ids", "1,2"},
		{"batch", "create-student-groups"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			withEduGroupCaller(t)
			cmd := newEduGroupCommand()
			cmd.SetArgs(args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute(%v) = %v, want nil", args, err)
			}
		})
	}
}

func TestEduGroupMissingRequiredFlags(t *testing.T) {
	// Each dept-id command must reject an absent --dept-id, covering its own
	// error branch as well as the shared eduGroupRequiredIntFlag empty case.
	deptIDCommands := [][]string{
		{"student-group", "info"},
		{"student-group", "exists"},
		{"student-group", "members"},
		{"student-group", "is-in"},
		{"student-group", "conversation"},
		{"student-group", "create"},
		{"student-group", "disband"},
		{"class-group", "conversation-id"},
		{"class-group", "conversation"},
		{"class-group", "exists"},
	}
	for _, args := range deptIDCommands {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			withEduGroupCaller(t)
			cmd := newEduGroupCommand()
			cmd.SetArgs(args)
			if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "dept-id") {
				t.Fatalf("Execute(%v) error = %v, want dept-id required", args, err)
			}
		})
	}
}

func TestEduGroupFlagValidation(t *testing.T) {
	t.Run("non-integer dept-id", func(t *testing.T) {
		withEduGroupCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"student-group", "info", "--dept-id", "abc"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "整数") {
			t.Fatalf("non-integer dept-id error = %v", err)
		}
	})

	t.Run("list-by-cids missing conversation-ids", func(t *testing.T) {
		withEduGroupCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"class-group", "list-by-cids"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "conversation-ids") {
			t.Fatalf("missing conversation-ids error = %v", err)
		}
	})

	t.Run("list-by-cids only separators", func(t *testing.T) {
		withEduGroupCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"class-group", "list-by-cids", "--conversation-ids", " , , "})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "conversation-ids") {
			t.Fatalf("empty conversation-ids error = %v", err)
		}
	})

	t.Run("batch check missing class-ids", func(t *testing.T) {
		withEduGroupCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"batch", "check-student-group"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "class-ids") {
			t.Fatalf("missing class-ids error = %v", err)
		}
	})

	t.Run("batch check invalid integer class-ids", func(t *testing.T) {
		withEduGroupCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"batch", "check-student-group", "--class-ids", "x"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "整数") {
			t.Fatalf("invalid integer class-ids error = %v", err)
		}
	})

	t.Run("batch get missing class-ids", func(t *testing.T) {
		withEduGroupCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"batch", "get-class-groups"})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "class-ids") {
			t.Fatalf("missing class-ids error = %v", err)
		}
	})

	t.Run("batch get only separators", func(t *testing.T) {
		withEduGroupCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"batch", "get-class-groups", "--class-ids", " , , "})
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "class-ids") {
			t.Fatalf("empty class-ids error = %v", err)
		}
	})
}

func TestEduGroupParseHelpers(t *testing.T) {
	if got := eduGroupParseCSV(" a , , b "); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("eduGroupParseCSV = %#v", got)
	}
	ids, err := eduGroupParseIntCSV(" 1 , , 2 ")
	if err != nil || len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("eduGroupParseIntCSV = %#v, err = %v", ids, err)
	}
	if _, err := eduGroupParseIntCSV("1,bad"); err == nil {
		t.Fatalf("eduGroupParseIntCSV invalid = nil error")
	}
}

// withEduGroupDispatchCaller installs a non-dry-run capture caller so commands
// exercise the full dispatch path through deps.Caller.CallTool.
func withEduGroupDispatchCaller(t *testing.T) *recruitCaptureCaller {
	t.Helper()
	caller := &recruitCaptureCaller{}
	InitDepsForTest(t, caller)
	deps.Out.w = io.Discard
	return caller
}

func TestEduGroupDispatch(t *testing.T) {
	t.Run("student-group info dispatches get_class_group_info", func(t *testing.T) {
		caller := withEduGroupDispatchCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"student-group", "info", "--dept-id", "123"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute error = %v", err)
		}
		if caller.productID != "edu-group" {
			t.Fatalf("productID = %q, want %q", caller.productID, "edu-group")
		}
		if caller.tool != "get_class_group_info" {
			t.Fatalf("tool = %q, want %q", caller.tool, "get_class_group_info")
		}
		input, ok := caller.args["input"].(map[string]any)
		if !ok {
			t.Fatalf("args[\"input\"] = %#v, want map", caller.args["input"])
		}
		if input["deptId"] != int64(123) {
			t.Fatalf("input[\"deptId\"] = %#v, want int64(123)", input["deptId"])
		}
	})

	t.Run("student-group create dispatches create_class_group", func(t *testing.T) {
		caller := withEduGroupDispatchCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"student-group", "create", "--dept-id", "456"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute error = %v", err)
		}
		if caller.productID != "edu-group" {
			t.Fatalf("productID = %q, want %q", caller.productID, "edu-group")
		}
		if caller.tool != "create_class_group" {
			t.Fatalf("tool = %q, want %q", caller.tool, "create_class_group")
		}
		input, ok := caller.args["input"].(map[string]any)
		if !ok {
			t.Fatalf("args[\"input\"] = %#v, want map", caller.args["input"])
		}
		if input["deptId"] != int64(456) {
			t.Fatalf("input[\"deptId\"] = %#v, want int64(456)", input["deptId"])
		}
	})

	t.Run("student-group disband dispatches disband_class_group", func(t *testing.T) {
		caller := withEduGroupDispatchCaller(t)
		cmd := newEduGroupCommand()
		cmd.PersistentFlags().Bool("yes", false, "")
		cmd.PersistentFlags().Bool("dry-run", false, "")
		cmd.SetArgs([]string{"student-group", "disband", "--dept-id", "789", "--yes"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute error = %v", err)
		}
		if caller.productID != "edu-group" {
			t.Fatalf("productID = %q, want %q", caller.productID, "edu-group")
		}
		if caller.tool != "disband_class_group" {
			t.Fatalf("tool = %q, want %q", caller.tool, "disband_class_group")
		}
		input, ok := caller.args["input"].(map[string]any)
		if !ok {
			t.Fatalf("args[\"input\"] = %#v, want map", caller.args["input"])
		}
		if input["deptId"] != int64(789) {
			t.Fatalf("input[\"deptId\"] = %#v, want int64(789)", input["deptId"])
		}
	})

	t.Run("class-group list-by-cids dispatches list_groups_by_conversation_ids", func(t *testing.T) {
		caller := withEduGroupDispatchCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"class-group", "list-by-cids", "--conversation-ids", "cid1,cid2,cid3"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute error = %v", err)
		}
		if caller.productID != "edu-group" {
			t.Fatalf("productID = %q, want %q", caller.productID, "edu-group")
		}
		if caller.tool != "list_groups_by_conversation_ids" {
			t.Fatalf("tool = %q, want %q", caller.tool, "list_groups_by_conversation_ids")
		}
		input, ok := caller.args["input"].(map[string]any)
		if !ok {
			t.Fatalf("args[\"input\"] = %#v, want map", caller.args["input"])
		}
		cids, ok := input["conversationIds"].([]string)
		if !ok || len(cids) != 3 || cids[0] != "cid1" || cids[1] != "cid2" || cids[2] != "cid3" {
			t.Fatalf("input[\"conversationIds\"] = %#v, want [cid1 cid2 cid3]", input["conversationIds"])
		}
	})

	t.Run("batch check-student-group dispatches batch_check_class_group", func(t *testing.T) {
		caller := withEduGroupDispatchCaller(t)
		cmd := newEduGroupCommand()
		cmd.SetArgs([]string{"batch", "check-student-group", "--class-ids", "100,200"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute error = %v", err)
		}
		if caller.productID != "edu-group" {
			t.Fatalf("productID = %q, want %q", caller.productID, "edu-group")
		}
		if caller.tool != "batch_check_class_group" {
			t.Fatalf("tool = %q, want %q", caller.tool, "batch_check_class_group")
		}
		input, ok := caller.args["input"].(map[string]any)
		if !ok {
			t.Fatalf("args[\"input\"] = %#v, want map", caller.args["input"])
		}
		classIDs, ok := input["classIds"].([]int64)
		if !ok || len(classIDs) != 2 || classIDs[0] != 100 || classIDs[1] != 200 {
			t.Fatalf("input[\"classIds\"] = %#v, want [100 200]", input["classIds"])
		}
	})
}
