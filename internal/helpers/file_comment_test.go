// Copyright 2026 Alibaba Group
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helpers

import (
	"io"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/internal/corecmd/contractfinal"
)

func executeDriveCommentCommand(t *testing.T, caller *docCommentMutationCaller, args ...string) error {
	t.Helper()
	previousDeps := deps
	previousArgs := os.Args
	t.Cleanup(func() {
		deps = previousDeps
		os.Args = previousArgs
	})

	InitDeps(caller)
	deps.Out.w = io.Discard
	os.Args = []string{"dws", "drive"}
	cmd := newDriveCommand()
	if cmd.PersistentFlags().Lookup("yes") == nil {
		cmd.PersistentFlags().Bool("yes", false, "skip confirmation")
	}
	cmd.SetIn(strings.NewReader(""))
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	return cmd.Execute()
}

func TestCrossPlatformCoverageDriveCommentRegistersAllTenNewCommentCommands(t *testing.T) {
	group := newDriveFileCommentCmd()
	type wantCommand struct {
		identity string
		rpc      string
		legacy   bool
	}
	want := map[string]wantCommand{
		"list":         {identity: "list_file_comments", rpc: "list_comments", legacy: true},
		"create":       {identity: "create_file_comment", rpc: "create_comment", legacy: true},
		"reply":        {identity: "reply_comment", rpc: "reply_comment"},
		"update":       {identity: "update_comment", rpc: "update_comment"},
		"delete":       {identity: "delete_comment", rpc: "delete_comment"},
		"batch-query":  {identity: "batch_query_comments", rpc: "batch_query_comments"},
		"list-replies": {identity: "list_replies", rpc: "list_replies"},
		"resolve":      {identity: "resolve_comment", rpc: "resolve_comment"},
		"restore":      {identity: "restore_comment", rpc: "restore_comment"},
		"react-reply":  {identity: "react_reply", rpc: "reply_comment"},
	}
	if len(group.Commands()) != len(want) {
		t.Fatalf("drive comment command count = %d, want %d", len(group.Commands()), len(want))
	}
	for leaf, expected := range want {
		cmd, remaining, err := group.Find([]string{leaf})
		if err != nil || len(remaining) != 0 {
			t.Fatalf("drive comment %s lookup: cmd=%v remaining=%v err=%v", leaf, cmd, remaining, err)
		}
		final, ok := contractfinal.RuntimeContractFinal(cmd)
		if !ok || final.Identity == nil || final.Interface == nil {
			t.Fatalf("drive comment %s incomplete ContractFinal: %#v", leaf, final)
		}
		if final.Identity.ProductID != "drive" || final.Identity.Name != expected.identity ||
			final.Identity.PrimaryCLIPath != "drive comment "+leaf {
			t.Fatalf("drive comment %s identity = %#v", leaf, final.Identity)
		}
		if expected.legacy {
			if final.Interface.Mode != "composite" || final.Interface.Ref != nil {
				t.Fatalf("drive comment %s historical interface = %#v", leaf, final.Interface)
			}
			continue
		}
		if final.Interface.Ref == nil || final.Interface.Ref.ProductID != commentServer ||
			final.Interface.Ref.RPCName != expected.rpc {
			t.Fatalf("drive comment %s interface = %#v, want %s/%s", leaf, final.Interface.Ref, commentServer, expected.rpc)
		}
	}
}

func TestCrossPlatformCoverageDriveCommentFirstFiveUseSharedNewCommentRPCs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		tool string
		want map[string]any
	}{
		{
			name: "list",
			args: []string{"comment", "list", "--node", "file-1", "--limit", "20", "--cursor", "opaque-2", "--resolve-status", "unresolved"},
			tool: "list_comments",
			want: map[string]any{
				"nodeId": "file-1", "pageSize": 20, "nextToken": "opaque-2",
				"resolveStatus": "unresolved", "commentType": driveCommentGlobalTopic,
			},
		},
		{
			name: "create",
			args: []string{"comment", "create", "--node", "file-1", "--content", "new root", "--yes"},
			tool: "create_comment",
			want: map[string]any{"nodeId": "file-1", "content": "new root"},
		},
		{
			name: "reply",
			args: []string{"comment", "reply", "--node", "file-1", "--comment-key", "comment-1", "--content", "reply"},
			tool: "reply_comment",
			want: map[string]any{
				"nodeId": "file-1", "replyCommentKey": "comment-1", "content": "reply", "emoji": false,
			},
		},
		{
			name: "update",
			args: []string{"comment", "update", "--node", "file-1", "--comment-key", "comment-1", "--content", "updated"},
			tool: "update_comment",
			want: map[string]any{"nodeId": "file-1", "commentKey": "comment-1", "content": "updated"},
		},
		{
			name: "delete",
			args: []string{"comment", "delete", "--node", "file-1", "--comment-key", "comment-1", "--yes"},
			tool: "delete_comment",
			want: map[string]any{"nodeId": "file-1", "commentKey": "comment-1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			caller := &docCommentMutationCaller{}
			if err := executeDriveCommentCommand(t, caller, tc.args...); err != nil {
				t.Fatal(err)
			}
			if len(caller.calls) != 1 {
				t.Fatalf("calls = %#v", caller.calls)
			}
			call := caller.calls[0]
			if call.productID != commentServer || call.toolName != tc.tool || !reflect.DeepEqual(call.args, tc.want) {
				t.Fatalf("call = %#v, want server=%q tool=%q args=%#v", call, commentServer, tc.tool, tc.want)
			}
			if call.toolName == "list_file_comments" || call.toolName == "create_file_comment" {
				t.Fatalf("new Drive command called legacy tool: %#v", call)
			}
		})
	}
}

func TestCrossPlatformCoverageDriveCommentKeepsLegacyFlagSurfaceAndRejectsInvalidInput(t *testing.T) {
	group := newDriveFileCommentCmd()
	listCmd, _, _ := group.Find([]string{"list"})
	for _, legacy := range []string{"space-id", "scope", "all", "page-size"} {
		if listCmd.Flags().Lookup(legacy) == nil {
			t.Errorf("drive comment list lost compatibility --%s", legacy)
		}
	}
	createCmd, _, _ := group.Find([]string{"create"})
	if createCmd.Flags().Lookup("space-id") == nil {
		t.Error("drive comment create lost compatibility --space-id")
	}
	for _, unsupported := range []string{"mention", "mentioned-open-conversation-id", "anchor"} {
		if createCmd.Flags().Lookup(unsupported) != nil {
			t.Errorf("drive comment create exposes unsupported --%s", unsupported)
		}
	}

	compatCaller := &docCommentMutationCaller{}
	if err := executeDriveCommentCommand(t, compatCaller,
		"comment", "list", "--node", "file-1", "--space-id", "123", "--scope", "whole", "--page-size", "20"); err != nil {
		t.Fatal(err)
	}
	wantCompatArgs := map[string]any{
		"nodeId": "file-1", "pageSize": 20, "commentType": driveCommentGlobalTopic,
	}
	if len(compatCaller.calls) != 1 || !reflect.DeepEqual(compatCaller.calls[0].args, wantCompatArgs) {
		t.Fatalf("compatibility flags leaked to new RPC: calls=%#v want=%#v", compatCaller.calls, wantCompatArgs)
	}
	if err := executeDriveCommentCommand(t, compatCaller,
		"comment", "list", "--node", "file-2", "--scope", "whole"); err != nil {
		t.Fatal(err)
	}
	wantDefaultArgs := map[string]any{
		"nodeId": "file-2", "pageSize": driveCommentMaxPageSize, "commentType": driveCommentGlobalTopic,
	}
	if len(compatCaller.calls) != 2 || !reflect.DeepEqual(compatCaller.calls[1].args, wantDefaultArgs) {
		t.Fatalf("legacy default page size was not capped for the new RPC: calls=%#v want=%#v", compatCaller.calls, wantDefaultArgs)
	}

	caller := &docCommentMutationCaller{}
	if err := executeDriveCommentCommand(t, caller,
		"comment", "list", "--node", "file-1", "--limit", "201"); err == nil || !strings.Contains(err.Error(), "1-200") {
		t.Fatalf("invalid limit error = %v", err)
	}
	if err := executeDriveCommentCommand(t, caller,
		"comment", "create", "--node", "file-1", "--content", "   "); err == nil || !strings.Contains(err.Error(), "不能为空") {
		t.Fatalf("blank content error = %v", err)
	}
	if err := executeDriveCommentCommand(t, caller,
		"comment", "list", "--node", "file-1", "--all"); err == nil || !strings.Contains(err.Error(), "--cursor") {
		t.Fatalf("legacy all error = %v", err)
	}
	if err := executeDriveCommentCommand(t, caller,
		"comment", "list", "--node", "file-1", "--scope", "partial"); err == nil || !strings.Contains(err.Error(), "全局评论") {
		t.Fatalf("legacy partial scope error = %v", err)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("invalid inputs reached MCP: %#v", caller.calls)
	}
}

func TestCrossPlatformCoverageDriveCommentDeleteRequiresConfirmation(t *testing.T) {
	caller := &docCommentMutationCaller{}
	err := executeDriveCommentCommand(t, caller,
		"comment", "delete", "--node", "file-1", "--comment-key", "comment-1")
	if err == nil || !strings.Contains(err.Error(), "用户确认") {
		t.Fatalf("delete without confirmation error = %v", err)
	}
	if len(caller.calls) != 0 {
		t.Fatalf("delete reached MCP before confirmation: %#v", caller.calls)
	}
}

func TestCrossPlatformCoverageDriveCommentCreateRequiresExplicitConfirmation(t *testing.T) {
	unconfirmed := &docCommentMutationCaller{}
	err := executeDriveCommentCommand(t, unconfirmed,
		"comment", "create", "--node", "file-1", "--content", "new root")
	if err == nil || !strings.Contains(err.Error(), "用户确认") {
		t.Fatalf("create without confirmation error = %v", err)
	}
	if len(unconfirmed.calls) != 0 {
		t.Fatalf("create reached MCP before confirmation: %#v", unconfirmed.calls)
	}

	confirmed := &docCommentMutationCaller{}
	if err := executeDriveCommentCommand(t, confirmed,
		"comment", "create", "--node", "123", "--space-id", "456",
		"--content", "new root", "--yes"); err != nil {
		t.Fatal(err)
	}
	want := docCommentMutationCall{
		productID: commentServer,
		toolName:  "create_comment",
		args:      map[string]any{"nodeId": "123", "content": "new root"},
	}
	if len(confirmed.calls) != 1 || !reflect.DeepEqual(confirmed.calls[0], want) {
		t.Fatalf("confirmed create calls = %#v, want %#v", confirmed.calls, want)
	}
}

func TestCrossPlatformCoverageDriveCommentLegacyValidationEdges(t *testing.T) {
	missingSpace := &docCommentMutationCaller{}
	if err := executeDriveCommentCommand(t, missingSpace,
		"comment", "list", "--node", "123"); err == nil || !strings.Contains(err.Error(), "--space-id") {
		t.Fatalf("numeric node without space error = %v", err)
	}
	if len(missingSpace.calls) != 0 {
		t.Fatalf("invalid numeric node reached MCP: %#v", missingSpace.calls)
	}

	tooLong := &docCommentMutationCaller{}
	if err := executeDriveCommentCommand(t, tooLong,
		"comment", "create", "--node", "file-1",
		"--content", strings.Repeat("a", legacyDriveCommentContentLength+1), "--yes"); err == nil || !strings.Contains(err.Error(), "UTF-16") {
		t.Fatalf("oversized content error = %v", err)
	}
	if len(tooLong.calls) != 0 {
		t.Fatalf("oversized content reached MCP: %#v", tooLong.calls)
	}

	if got := driveCommentUTF16Length("a😀"); got != 3 {
		t.Fatalf("UTF-16 length = %d, want 3", got)
	}
	if allDriveCommentASCIIDigits("") || allDriveCommentASCIIDigits("12a") ||
		!allDriveCommentASCIIDigits("123") {
		t.Fatal("ASCII digit validation returned an unexpected result")
	}
}
