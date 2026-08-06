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
	"context"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/DingTalk-Real-AI/dingtalk-workspace-cli/pkg/edition"
)

type toolbarCall struct {
	product string
	tool    string
	args    map[string]any
}

type toolbarTestCaller struct {
	product string
	tool    string
	args    map[string]any
	err     error
	calls   []toolbarCall
}

func (c *toolbarTestCaller) CallTool(_ context.Context, productID, toolName string, args map[string]any) (*edition.ToolResult, error) {
	c.calls = append(c.calls, toolbarCall{product: productID, tool: toolName, args: args})
	c.product = productID
	c.tool = toolName
	c.args = args
	if c.err != nil {
		return nil, c.err
	}
	return &edition.ToolResult{Content: []edition.ContentBlock{{Type: "text", Text: "{}"}}}, nil
}

func (c *toolbarTestCaller) Format() string { return "json" }
func (c *toolbarTestCaller) DryRun() bool   { return false }
func (c *toolbarTestCaller) Fields() string { return "" }
func (c *toolbarTestCaller) JQ() string     { return "" }

func executeToolbarCommand(t *testing.T, cmd *cobra.Command, caller *toolbarTestCaller, args ...string) error {
	t.Helper()
	InitDepsForTest(t, caller)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func requireToolbarCall(
	t *testing.T,
	caller *toolbarTestCaller,
	wantProduct, wantTool string,
	wantArgs map[string]any,
) {
	t.Helper()
	if caller.product != wantProduct || caller.tool != wantTool {
		t.Fatalf("tool call = %s/%s, want %s/%s", caller.product, caller.tool, wantProduct, wantTool)
	}
	if !reflect.DeepEqual(caller.args, wantArgs) {
		t.Fatalf("tool args = %#v, want %#v", caller.args, wantArgs)
	}
}

func toolbarCustomArgs(extra ...string) []string {
	args := []string{"--conversation-id", "cid", "--title", "Title", "--url", "https://example.com/mobile", "--icon-url", "https://example.com/icon.png", "--pc-url", "https://example.com/pc", "--org-id-list", "10"}
	return append(args, extra...)
}

func TestHasIntersection(t *testing.T) {
	tests := []struct {
		name string
		a, b []int64
		want bool
	}{
		{"both empty", nil, nil, false},
		{"a empty", nil, []int64{1}, false},
		{"b empty", []int64{1}, nil, false},
		{"no overlap", []int64{1, 2, 3}, []int64{4, 5, 6}, false},
		{"has overlap", []int64{1, 2, 3}, []int64{3, 4, 5}, true},
		{"single match", []int64{1}, []int64{1}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasIntersection(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("hasIntersection(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestIsSystemBusy(t *testing.T) {
	if isSystemBusy(nil) {
		t.Error("isSystemBusy(nil) should be false")
	}
	if !isSystemBusy(errors.New("error code: SYSTEM_BUSY")) {
		t.Error("isSystemBusy should detect SYSTEM_BUSY in error message")
	}
	if isSystemBusy(errors.New("some other error")) {
		t.Error("isSystemBusy should return false for non-SYSTEM_BUSY errors")
	}
}

func TestParseExtension(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().StringArray("extension", nil, "")

	ext, err := parseExtension(cmd)
	if err != nil {
		t.Fatalf("empty extension should not error: %v", err)
	}
	if ext != nil {
		t.Fatalf("empty extension should return nil, got %v", ext)
	}

	_ = cmd.Flags().Set("extension", "key1=val1")
	_ = cmd.Flags().Set("extension", "key2=val2")
	ext, err = parseExtension(cmd)
	if err != nil {
		t.Fatalf("valid extension should not error: %v", err)
	}
	if ext["key1"] != "val1" || ext["key2"] != "val2" {
		t.Fatalf("unexpected extension map: %v", ext)
	}

	cmd2 := &cobra.Command{Use: "test2"}
	cmd2.Flags().StringArray("extension", nil, "")
	_ = cmd2.Flags().Set("extension", "badformat")
	_, err = parseExtension(cmd2)
	if err == nil {
		t.Fatal("invalid extension format should error")
	}

	cmd3 := &cobra.Command{Use: "test3"}
	cmd3.Flags().StringArray("extension", nil, "")
	_ = cmd3.Flags().Set("extension", "color=blue")
	_ = cmd3.Flags().Set("extension", "color=red")
	_, err = parseExtension(cmd3)
	if err == nil {
		t.Fatal("duplicate extension keys should error")
	}

	cmd4 := &cobra.Command{Use: "test4"}
	cmd4.Flags().StringArray("extension", nil, "")
	_ = cmd4.Flags().Set("extension", " =value")
	_, err = parseExtension(cmd4)
	if err == nil {
		t.Fatal("blank extension key should error")
	}
}

func TestToolbarConversationID(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("conversation-id", "", "")

	_, err := toolbarConversationID(cmd)
	if err == nil {
		t.Fatal("missing conversation-id should error")
	}

	_ = cmd.Flags().Set("conversation-id", "cid123")
	cid, err := toolbarConversationID(cmd)
	if err != nil {
		t.Fatalf("valid conversation-id should not error: %v", err)
	}
	if cid != "cid123" {
		t.Fatalf("expected cid123, got %s", cid)
	}
}

func TestToolbarCommandsConvertSystemBusy(t *testing.T) {
	tests := []struct {
		name string
		cmd  func() *cobra.Command
		args []string
	}{
		{
			name: "add",
			cmd:  newToolbarAddCommand,
			args: []string{"--conversation-id", "cid", "--shortcut-ids", "1,2"},
		},
		{
			name: "hide",
			cmd:  newToolbarHideCommand,
			args: []string{"--conversation-id", "cid", "--shortcut-ids", "1,2"},
		},
		{
			name: "sort",
			cmd:  newToolbarSortCommand,
			args: []string{"--conversation-id", "cid", "--sorted-ids", "1,2"},
		},
		{
			name: "remove-custom",
			cmd:  newToolbarRemoveCustomCommand,
			args: []string{"--conversation-id", "cid", "--shortcut-id", "1", "--yes"},
		},
		{
			name: "create-custom",
			cmd:  newToolbarCreateCustomCommand,
			args: toolbarCustomArgs(),
		},
		{
			name: "update-custom",
			cmd:  newToolbarUpdateCustomCommand,
			args: toolbarCustomArgs("--shortcut-id", "99"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := executeToolbarCommand(t, tt.cmd(), &toolbarTestCaller{
				err: errors.New("remote SYSTEM_BUSY"),
			}, tt.args...)
			if err == nil || !strings.Contains(err.Error(), "SYSTEM_BUSY") {
				t.Fatalf("expected system_busy validation error, got %v", err)
			}
		})
	}
}

func TestToolbarCreateAndUpdateCustomIncludeOptionalFields(t *testing.T) {
	createCaller := &toolbarTestCaller{}
	err := executeToolbarCommand(t, newToolbarCreateCustomCommand(), createCaller, toolbarCustomArgs(
		"--org-id-list", "10,20",
		"--desc", "Description",
		"--tag", "Tag",
		"--sort-index", "7",
		"--extension", "color=blue",
	)...)
	if err != nil {
		t.Fatalf("create-custom returned error: %v", err)
	}
	requireToolbarCall(t, createCaller, "im", "create_chat_toolbar_custom_shortcut", map[string]any{
		"openCid":   "cid",
		"title":     "Title",
		"url":       "https://example.com/mobile",
		"iconUrl":   "https://example.com/icon.png",
		"pcUrl":     "https://example.com/pc",
		"orgIdList": []int64{10, 20},
		"desc":      "Description",
		"tag":       "Tag",
		"sortIndex": 7,
		"extension": map[string]string{"color": "blue"},
	})

	updateCaller := &toolbarTestCaller{}
	err = executeToolbarCommand(t, newToolbarUpdateCustomCommand(), updateCaller, toolbarCustomArgs(
		"--shortcut-id", "99",
		"--org-id-list", "10,20",
		"--desc", "Description",
		"--tag", "Tag",
		"--sort-index", "7",
		"--extension", "color=blue",
	)...)
	if err != nil {
		t.Fatalf("update-custom returned error: %v", err)
	}
	requireToolbarCall(t, updateCaller, "im", "update_chat_toolbar_custom_shortcut", map[string]any{
		"openCid":    "cid",
		"shortcutId": int64(99),
		"title":      "Title",
		"url":        "https://example.com/mobile",
		"iconUrl":    "https://example.com/icon.png",
		"pcUrl":      "https://example.com/pc",
		"orgIdList":  []int64{10, 20},
		"desc":       "Description",
		"tag":        "Tag",
		"sortIndex":  7,
		"extension":  map[string]string{"color": "blue"},
	})
}

func TestToolbarSortValidatesOptionalUnsortedIDs(t *testing.T) {
	intersectionErr := executeToolbarCommand(t, newToolbarSortCommand(), &toolbarTestCaller{},
		"--conversation-id", "cid",
		"--sorted-ids", "1,2",
		"--unsorted-ids", "2,3",
	)
	if intersectionErr == nil || !strings.Contains(intersectionErr.Error(), "不能有交集") {
		t.Fatalf("expected id_intersection error, got %v", intersectionErr)
	}

	caller := &toolbarTestCaller{}
	err := executeToolbarCommand(t, newToolbarSortCommand(), caller,
		"--conversation-id", "cid",
		"--sorted-ids", "1,2",
		"--unsorted-ids", "3,4",
	)
	if err != nil {
		t.Fatalf("sort returned error: %v", err)
	}
	requireToolbarCall(t, caller, "im", "sort_chat_toolbar_shortcuts", map[string]any{
		"openCid":             "cid",
		"sortedShortcutIds":   []int64{1, 2},
		"unsortedShortcutIds": []int64{3, 4},
	})
}

func TestCrossPlatformCoverageToolbarRemoveCustomRejectsWithoutYes(t *testing.T) {
	caller := &toolbarTestCaller{}
	err := executeToolbarCommand(t, newToolbarRemoveCustomCommand(), caller,
		"--conversation-id", "cidX", "--shortcut-id", "42")
	requireTypedConfirmationError(t, err)
	if len(caller.calls) != 0 {
		t.Fatalf("expected 0 MCP calls before --yes, got %d: %+v", len(caller.calls), caller.calls)
	}
	if caller.product != "" || caller.tool != "" || caller.args != nil {
		t.Fatalf("expected no MCP call recorded, got product=%q tool=%q args=%#v", caller.product, caller.tool, caller.args)
	}
}

func TestCrossPlatformCoverageToolbarRemoveCustomCallsMCPWithExactArgsWhenYes(t *testing.T) {
	caller := &toolbarTestCaller{}
	err := executeToolbarCommand(t, newToolbarRemoveCustomCommand(), caller,
		"--conversation-id", "cidX", "--shortcut-id", "42", "--yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(caller.calls) != 1 {
		t.Fatalf("expected exactly 1 MCP call, got %d: %+v", len(caller.calls), caller.calls)
	}
	call := caller.calls[0]
	if call.product != "im" {
		t.Fatalf("product = %q, want im", call.product)
	}
	if call.tool != "remove_chat_toolbar_custom_shortcut" {
		t.Fatalf("tool = %q, want remove_chat_toolbar_custom_shortcut", call.tool)
	}
	wantArgs := map[string]any{"openCid": "cidX", "shortcutId": int64(42)}
	if !reflect.DeepEqual(call.args, wantArgs) {
		t.Fatalf("tool args = %#v, want %#v", call.args, wantArgs)
	}
}
