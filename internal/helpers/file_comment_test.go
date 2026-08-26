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
	want := map[string]string{
		"list":         "list_comments",
		"create":       "create_comment",
		"reply":        "reply_comment",
		"update":       "update_comment",
		"delete":       "delete_comment",
		"batch-query":  "batch_query_comments",
		"list-replies": "list_replies",
		"resolve":      "resolve_comment",
		"restore":      "restore_comment",
		"react-reply":  "reply_comment",
	}
	if len(group.Commands()) != len(want) {
		t.Fatalf("drive comment command count = %d, want %d", len(group.Commands()), len(want))
	}
	for leaf, rpc := range want {
		cmd, remaining, err := group.Find([]string{leaf})
		if err != nil || len(remaining) != 0 {
			t.Fatalf("drive comment %s lookup: cmd=%v remaining=%v err=%v", leaf, cmd, remaining, err)
		}
		final, ok := contractfinal.RuntimeContractFinal(cmd)
		if !ok || final.Identity == nil || final.Interface == nil || final.Interface.Ref == nil {
			t.Fatalf("drive comment %s incomplete ContractFinal: %#v", leaf, final)
		}
		if final.Identity.ProductID != "drive" || final.Identity.PrimaryCLIPath != "drive comment "+leaf {
			t.Fatalf("drive comment %s identity = %#v", leaf, final.Identity)
		}
		if final.Interface.Ref.ProductID != commentServer || final.Interface.Ref.RPCName != rpc {
			t.Fatalf("drive comment %s interface = %#v, want %s/%s", leaf, final.Interface.Ref, commentServer, rpc)
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
			args: []string{"comment", "create", "--node", "file-1", "--content", "new root"},
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

func TestCrossPlatformCoverageDriveCommentRemovesLegacyFlagsAndRejectsInvalidInput(t *testing.T) {
	group := newDriveFileCommentCmd()
	listCmd, _, _ := group.Find([]string{"list"})
	for _, legacy := range []string{"space-id", "scope", "all", "type", "page-size"} {
		if listCmd.Flags().Lookup(legacy) != nil {
			t.Errorf("drive comment list still exposes legacy --%s", legacy)
		}
	}
	createCmd, _, _ := group.Find([]string{"create"})
	for _, unsupported := range []string{"space-id", "mention", "mentioned-open-conversation-id", "anchor"} {
		if createCmd.Flags().Lookup(unsupported) != nil {
			t.Errorf("drive comment create exposes unsupported --%s", unsupported)
		}
	}

	caller := &docCommentMutationCaller{}
	if err := executeDriveCommentCommand(t, caller,
		"comment", "list", "--node", "file-1", "--limit", "51"); err == nil || !strings.Contains(err.Error(), "1-50") {
		t.Fatalf("invalid limit error = %v", err)
	}
	if err := executeDriveCommentCommand(t, caller,
		"comment", "create", "--node", "file-1", "--content", "   "); err == nil || !strings.Contains(err.Error(), "不能为空") {
		t.Fatalf("blank content error = %v", err)
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
